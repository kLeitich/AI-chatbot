package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestPublicConfigEndpoint(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "public_config_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "pubconf_test.db")
	_ = initDatabase(dbPath, "development")

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	tenantGroup := app.Group("/t/:tenant")
	tenantGroup.Get("/public-config", getTenantPublicConfigHandler)

	// Create test tenant with specific settings
	t1 := Tenant{
		Slug:                "public-clinic",
		Name:                "Public Health Centre",
		EmergencyNumber:     "+254 700 123 456",
		EmergencyMessage:    "Sensitive internal emergency guidance", // should NOT leak
		BookingLinkEnabled:  true,
		PrivacyURL:          "https://example.com/privacy-policy",
	}
	t1.SetAllowedEmbedOrigins([]string{"https://mysite.com", "https://*.mysite.com"})
	if err := db.Create(&t1).Error; err != nil {
		t.Fatalf("Failed to create tenant: %v", err)
	}

	// Create tenant with empty emergency number to test fallback
	t2 := Tenant{
		Slug:               "empty-emerg-clinic",
		Name:               "Empty Emerg Clinic",
		EmergencyNumber:    "",
		BookingLinkEnabled: true,
	}
	t2.SetAllowedEmbedOrigins([]string{})
	if err := db.Create(&t2).Error; err != nil {
		t.Fatalf("Failed to create second tenant: %v", err)
	}
	if err := db.Model(&t2).Update("booking_link_enabled", false).Error; err != nil {
		t.Fatalf("Failed to update booking_link_enabled to false: %v", err)
	}

	t.Run("Returns exact set of allowed JSON keys and no sensitive fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/t/public-clinic/public-config", nil)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		// Exact expected keys
		expectedKeys := []string{
			"allowed_embed_origins",
			"booking_enabled",
			"emergency_number",
			"name",
			"privacy_url",
			"slug",
		}
		sort.Strings(expectedKeys)

		actualKeys := make([]string, 0, len(rawMap))
		for k := range rawMap {
			actualKeys = append(actualKeys, k)
		}
		sort.Strings(actualKeys)

		if len(actualKeys) != len(expectedKeys) {
			t.Fatalf("Expected exactly %d keys, got %d: %v", len(expectedKeys), len(actualKeys), actualKeys)
		}

		for i, expected := range expectedKeys {
			if actualKeys[i] != expected {
				t.Errorf("Key mismatch at index %d: expected %s, got %s", i, expected, actualKeys[i])
			}
		}

		// Verify sensitive keys are strictly absent
		sensitiveKeys := []string{"id", "emergency_message", "created_at", "updated_at", "password", "token"}
		for _, sk := range sensitiveKeys {
			if _, exists := rawMap[sk]; exists {
				t.Errorf("Sensitive key '%s' leaked in public-config response!", sk)
			}
		}

		// Verify values
		if rawMap["name"] != "Public Health Centre" {
			t.Errorf("Expected name 'Public Health Centre', got %v", rawMap["name"])
		}
		if rawMap["slug"] != "public-clinic" {
			t.Errorf("Expected slug 'public-clinic', got %v", rawMap["slug"])
		}
		if rawMap["booking_enabled"] != true {
			t.Errorf("Expected booking_enabled true, got %v", rawMap["booking_enabled"])
		}
		if rawMap["emergency_number"] != "+254 700 123 456" {
			t.Errorf("Expected emergency_number '+254 700 123 456', got %v", rawMap["emergency_number"])
		}
		if rawMap["privacy_url"] != "https://example.com/privacy-policy" {
			t.Errorf("Expected privacy_url 'https://example.com/privacy-policy', got %v", rawMap["privacy_url"])
		}

		origins, ok := rawMap["allowed_embed_origins"].([]interface{})
		if !ok || len(origins) != 2 {
			t.Fatalf("Expected 2 origins, got %v", rawMap["allowed_embed_origins"])
		}
		if origins[0] != "https://mysite.com" || origins[1] != "https://*.mysite.com" {
			t.Errorf("Unexpected origins: %v", origins)
		}
	})

	t.Run("Falls back to 999 when emergency number is empty and reflects booking_enabled false", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/t/empty-emerg-clinic/public-config", nil)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var rawMap map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&rawMap)

		if rawMap["emergency_number"] != "999" {
			t.Errorf("Expected fallback emergency_number '999', got %v", rawMap["emergency_number"])
		}
		if rawMap["booking_enabled"] != false {
			t.Errorf("Expected booking_enabled false, got %v", rawMap["booking_enabled"])
		}
	})

	t.Run("Returns 404 NOT_FOUND for unknown tenant slug", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/t/non-existent-tenant/public-config", nil)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("Expected status 404, got %d", resp.StatusCode)
		}

		var errBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody["code"] != ErrCodeNotFound {
			t.Errorf("Expected error code %s, got %v", ErrCodeNotFound, errBody["code"])
		}
	})
}
