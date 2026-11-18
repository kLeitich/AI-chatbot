package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestOriginValidationTable(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		isDev     bool
		wantValid bool
		wantNorm  string
	}{
		// Valid HTTPS origins
		{"valid standard https", "https://example.com", false, true, "https://example.com"},
		{"valid standard https uppercase", "HTTPS://EXAMPLE.COM", false, true, "https://example.com"},
		{"valid https with port", "https://example.com:8443", false, true, "https://example.com:8443"},
		{"valid https subdomain", "https://portal.clinic.co.ke", false, true, "https://portal.clinic.co.ke"},
		{"valid https wildcard leading", "https://*.example.com", false, true, "https://*.example.com"},
		{"valid https wildcard nested", "https://*.portal.example.com", false, true, "https://*.portal.example.com"},

		// Localhost rules
		{"localhost dev http", "http://localhost:3000", true, true, "http://localhost:3000"},
		{"localhost dev http no port", "http://localhost", true, true, "http://localhost"},
		{"localhost prod http blocked", "http://localhost:3000", false, false, ""},
		{"localhost prod http no port blocked", "http://localhost", false, false, ""},

		// Invalid schemes
		{"http on remote dev blocked", "http://example.com", true, false, ""},
		{"http on remote prod blocked", "http://example.com", false, false, ""},
		{"ftp blocked", "ftp://example.com", false, false, ""},
		{"javascript scheme blocked", "javascript:alert(1)", false, false, ""},

		// Invalid paths, queries, fragments, slashes
		{"trailing slash blocked", "https://example.com/", false, false, ""},
		{"path blocked", "https://example.com/embed", false, false, ""},
		{"query blocked", "https://example.com?foo=bar", false, false, ""},
		{"fragment blocked", "https://example.com#section", false, false, ""},

		// Invalid wildcards
		{"wildcard not leading", "https://sub.*.example.com", false, false, ""},
		{"wildcard within label", "https://*example.com", false, false, ""},
		{"bare wildcard", "https://*", false, false, ""},
		{"wildcard trailing dot", "https://*.example.", false, false, ""},

		// Malformed hosts and ports
		{"invalid port non-numeric", "https://example.com:abc", false, false, ""},
		{"empty origin", "", false, false, ""},
		{"whitespace only", "   ", false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm, err := validateOrigin(tt.origin, tt.isDev)
			if tt.wantValid {
				if err != nil {
					t.Errorf("validateOrigin(%q, %v) unexpected error: %v", tt.origin, tt.isDev, err)
				}
				if norm != tt.wantNorm {
					t.Errorf("validateOrigin(%q, %v) = %q, want %q", tt.origin, tt.isDev, norm, tt.wantNorm)
				}
			} else {
				if err == nil {
					t.Errorf("validateOrigin(%q, %v) expected error, got nil (norm=%q)", tt.origin, tt.isDev, norm)
				}
			}
		})
	}
}

func TestSourceSanitizationTable(t *testing.T) {
	tests := []struct {
		input    string
		expected *string
	}{
		{"instagram", strPtr("instagram")},
		{"INSTAGRAM", strPtr("instagram")},
		{"google-ads_2026", strPtr("google-ads_2026")},
		{"facebook! promo#123", strPtr("facebookpromo123")},
		{"this_is_a_very_long_tracking_source_identifier_that_exceeds_thirty_characters", strPtr("this_is_a_very_long_tracking_s")},
		{"!@#$%^&*()", nil},
		{"", nil},
		{"   ", nil},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			got := sanitizeSource(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("sanitizeSource(%q) = %q, want nil", tt.input, *got)
				}
			} else {
				if got == nil {
					t.Errorf("sanitizeSource(%q) = nil, want %q", tt.input, *tt.expected)
				} else if *got != *tt.expected {
					t.Errorf("sanitizeSource(%q) = %q, want %q", tt.input, *got, *tt.expected)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func TestChatContextValidationAndCrossTenantIsolation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, fmt.Sprintf("test_ctx_%d.db", time.Now().UnixNano()))
	_ = initDatabase(dbPath, "development")

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	tenantGroup := app.Group("/t/:tenant")
	tenantGroup.Post("/chat", chatHandler)

	// Create Tenant A
	tA := Tenant{Slug: "clinic-a", Name: "Clinic A", BookingLinkEnabled: true}
	db.Create(&tA)
	docAActive := Doctor{TenantID: tA.ID, Name: "Dr. Alice", Active: true}
	docAInactive := Doctor{TenantID: tA.ID, Name: "Dr. Inactive", Active: true}
	db.Create(&docAActive)
	db.Create(&docAInactive)
	db.Model(&docAInactive).Update("active", false)
	reasonA := ReasonOption{TenantID: tA.ID, Label: "Dental Cleaning", Active: true}
	db.Create(&reasonA)

	// Create Tenant B
	tB := Tenant{Slug: "clinic-b", Name: "Clinic B", BookingLinkEnabled: true}
	db.Create(&tB)
	docBActive := Doctor{TenantID: tB.ID, Name: "Dr. Bob", Active: true}
	reasonB := ReasonOption{TenantID: tB.ID, Label: "Cardiology Consult", Active: true}
	db.Create(&docBActive)
	db.Create(&reasonB)

	// Test 1: Cross-tenant doctor ID from Tenant B passed to Tenant A must be silently ignored
	t.Run("Cross-tenant doctor ID is silently ignored", func(t *testing.T) {
		sessID, _ := generateSessionID()
		reqBody, _ := json.Marshal(ChatRequest{
			Message:   "hi",
			SessionID: sessID,
			Context: &ChatContext{
				DoctorID: &docBActive.ID,
			},
		})
		req := httptest.NewRequest("POST", "/t/clinic-a/chat", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 5000)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("Chat request failed: %v, status: %d", err, resp.StatusCode)
		}

		conv := getConversation(fmt.Sprintf("clinic-a:%s", sessID))
		if conv.Draft.Doctor == "Dr. Bob" {
			t.Errorf("Cross-tenant doctor 'Dr. Bob' was leaked into draft: %v", conv.Draft.Doctor)
		}
	})

	// Test 2: Inactive doctor ID must be silently ignored
	t.Run("Inactive doctor ID is silently ignored", func(t *testing.T) {
		sessID, _ := generateSessionID()
		reqBody, _ := json.Marshal(ChatRequest{
			Message:   "hi",
			SessionID: sessID,
			Context: &ChatContext{
				DoctorID: &docAInactive.ID,
			},
		})
		req := httptest.NewRequest("POST", "/t/clinic-a/chat", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		conv := getConversation(fmt.Sprintf("clinic-a:%s", sessID))
		if conv.Draft.Doctor == "Dr. Inactive" {
			t.Errorf("Inactive doctor was seeded into draft: %v", conv.Draft.Doctor)
		}
	})

	// Test 3: Cross-tenant reason ID must be silently ignored
	t.Run("Cross-tenant reason ID is silently ignored", func(t *testing.T) {
		sessID, _ := generateSessionID()
		reqBody, _ := json.Marshal(ChatRequest{
			Message:   "hi",
			SessionID: sessID,
			Context: &ChatContext{
				ReasonID: &reasonB.ID,
			},
		})
		req := httptest.NewRequest("POST", "/t/clinic-a/chat", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		conv := getConversation(fmt.Sprintf("clinic-a:%s", sessID))
		if conv.Draft.Reason == "Cardiology Consult" {
			t.Errorf("Cross-tenant reason was seeded into draft: %v", conv.Draft.Reason)
		}
	})

	// Test 4: Valid doctor, reason, and source on first message correctly seeds draft
	t.Run("Valid active doctor, reason, and source seeds draft on first message", func(t *testing.T) {
		sessID, _ := generateSessionID()
		src := "INSTAGRAM_AD_2026!"
		reqBody, _ := json.Marshal(ChatRequest{
			Message:   "hello",
			SessionID: sessID,
			Context: &ChatContext{
				DoctorID: &docAActive.ID,
				ReasonID: &reasonA.ID,
				Source:   &src,
			},
		})
		req := httptest.NewRequest("POST", "/t/clinic-a/chat", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		conv := getConversation(fmt.Sprintf("clinic-a:%s", sessID))
		if conv.Draft.Doctor != "Dr. Alice" {
			t.Errorf("Expected draft doctor 'Dr. Alice', got %q", conv.Draft.Doctor)
		}
		if conv.Draft.Reason != "Dental Cleaning" {
			t.Errorf("Expected draft reason 'Dental Cleaning', got %q", conv.Draft.Reason)
		}
		if conv.Draft.Source == nil || *conv.Draft.Source != "instagram_ad_2026" {
			t.Errorf("Expected sanitized draft source 'instagram_ad_2026', got %v", conv.Draft.Source)
		}

		// Subsequent message cannot overwrite context
		otherDocID := docBActive.ID
		otherSrc := "malicious-takeover"
		reqBody2, _ := json.Marshal(ChatRequest{
			Message:   "my name is David",
			SessionID: sessID,
			Context: &ChatContext{
				DoctorID: &otherDocID,
				Source:   &otherSrc,
			},
		})
		req2 := httptest.NewRequest("POST", "/t/clinic-a/chat", bytes.NewReader(reqBody2))
		req2.Header.Set("Content-Type", "application/json")
		app.Test(req2, 5000)

		conv2 := getConversation(fmt.Sprintf("clinic-a:%s", sessID))
		if conv2.Draft.Doctor != "Dr. Alice" {
			t.Errorf("Draft doctor overwritten on second message: %q", conv2.Draft.Doctor)
		}
		if conv2.Draft.Source == nil || *conv2.Draft.Source != "instagram_ad_2026" {
			t.Errorf("Draft source overwritten on second message: %v", conv2.Draft.Source)
		}
	})
}

func TestSettingsAuditEntryNoSensitiveValues(t *testing.T) {
	_ = validateConfig("development", "")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, fmt.Sprintf("test_audit_%d.db", time.Now().UnixNano()))
	tenantID := initDatabase(dbPath, "development")

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	tenant := app.Group("/t/:tenant")
	admin := tenant.Group("/admin", jwtMiddleware, tenantUserMiddleware)
	admin.Patch("/settings", requirePermission(PermSettingsManage), updateTenantSettingsHandler)

	adminHash, _ := hashPassword("AdminPassword123!")
	adminUser := User{
		Email:        "admin@auditclinic.internal",
		Password:     adminHash,
		Role:         RoleTenantAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	db.Create(&adminUser)
	tok, _ := createTenantJWTToken(adminUser.ID, adminUser.Email, tenantID, RoleTenantAdmin, 1)

	sensitiveMessage := "Internal secret: Code 1234 on door. Do not broadcast."
	patchPayload := map[string]interface{}{
		"emergency_number":      "+254 711 999 888",
		"emergency_message":     sensitiveMessage,
		"booking_link_enabled":  true,
		"privacy_url":           "https://clinic.example.com/privacy",
		"allowed_embed_origins": []string{"https://clinic.example.com"},
	}
	b, _ := json.Marshal(patchPayload)

	req := httptest.NewRequest("PATCH", "/t/default/admin/settings", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := app.Test(req, 5000)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Settings patch failed: %v, status: %d", err, resp.StatusCode)
	}

	// Verify AuditLog entry was recorded
	var audit AuditLog
	if err := db.Where("action = ? AND entity = ?", "update", "settings").First(&audit).Error; err != nil {
		t.Fatalf("Audit log not found: %v", err)
	}

	if audit.ActorUserID != adminUser.ID {
		t.Errorf("Expected actor ID %d, got %d", adminUser.ID, audit.ActorUserID)
	}
	if audit.ActorRole != RoleTenantAdmin {
		t.Errorf("Expected actor role %s, got %s", RoleTenantAdmin, audit.ActorRole)
	}
	if audit.TenantID != tenantID {
		t.Errorf("Expected tenant ID %d, got %d", tenantID, audit.TenantID)
	}

	// Confirm that sensitive data (e.g. emergency message text) is NOT in the database audit log table
	// Let's inspect all text fields of audit
	auditString := fmt.Sprintf("%+v", audit)
	if strings.Contains(auditString, sensitiveMessage) {
		t.Errorf("Audit log leaked sensitive message content!")
	}
	if strings.Contains(auditString, "1234") {
		t.Errorf("Audit log leaked sensitive numbers from message!")
	}
}
