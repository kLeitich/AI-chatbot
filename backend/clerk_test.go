package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestParseRSAPublicKey(t *testing.T) {
	// Sample small valid modulus and exponent
	nBase64 := "u1W1x6q_z-pL9iLz"
	eBase64 := "AQAB" // 65537

	pubKey, err := parseRSAPublicKey(nBase64, eBase64)
	if err != nil {
		t.Fatalf("Failed to parse RSA public key: %v", err)
	}
	if pubKey.E != 65537 {
		t.Errorf("Expected exponent 65537, got %d", pubKey.E)
	}
	if pubKey.N.BitLen() == 0 {
		t.Errorf("Expected non-zero modulus bit length")
	}
}

func TestClerkSyncHandlerMissingToken(t *testing.T) {
	setupTestDB(t)

	app := fiber.New()
	app.Post("/auth/clerk/sync", clerkSyncHandler)
	app.Post("/t/:tenant/auth/clerk/sync", clerkSyncHandler)

	// 1. Missing token
	req := httptest.NewRequest("POST", "/auth/clerk/sync", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing token, got %d", resp.StatusCode)
	}

	// 2. Tenant-scoped route with invalid token
	reqTenant := httptest.NewRequest("POST", "/t/default/auth/clerk/sync", bytes.NewReader([]byte(`{"token": "invalid.jwt.token"}`)))
	reqTenant.Header.Set("Content-Type", "application/json")

	respTenant, err := app.Test(reqTenant)
	if err != nil {
		t.Fatalf("Tenant test failed: %v", err)
	}

	if respTenant.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for invalid clerk token, got %d", respTenant.StatusCode)
	}
}

func TestCreateUniqueTenantSlug(t *testing.T) {
	setupTestDB(t)

	// 1. Normal name
	slug1 := createUniqueTenantSlug("Dr. Smith Clinic")
	if slug1 != "dr-smith-clinic" {
		t.Errorf("Expected 'dr-smith-clinic', got '%s'", slug1)
	}

	// 2. Reserved slug (e.g. "admin")
	slugReserved := createUniqueTenantSlug("Admin")
	if slugReserved == "admin" || slugReserved != "admin-1" {
		t.Errorf("Expected 'admin-1' for reserved slug, got '%s'", slugReserved)
	}

	// 3. Duplicate handling
	t1 := Tenant{Name: "Alpha Dental", Slug: "alpha-dental"}
	if err := db.Create(&t1).Error; err != nil {
		t.Fatalf("Failed to create tenant: %v", err)
	}

	slugDup := createUniqueTenantSlug("Alpha Dental")
	if slugDup != "alpha-dental-1" {
		t.Errorf("Expected 'alpha-dental-1' for duplicate, got '%s'", slugDup)
	}
}

func TestFetchClerkUserProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v1/users/user_abc123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"first_name": "Jane",
			"last_name": "Wanjiku",
			"primary_email_address_id": "idn_2",
			"email_addresses": [
				{"id": "idn_1", "email_address": "old@example.com"},
				{"id": "idn_2", "email_address": "jane.wanjiku@gmail.com"}
			]
		}`))
	}))
	defer srv.Close()

	prevBase := clerkAPIBaseURL
	clerkAPIBaseURL = srv.URL
	defer func() { clerkAPIBaseURL = prevBase }()

	email, fullName, err := fetchClerkUserProfile("user_abc123", "sk_test_secret123")
	if err != nil {
		t.Fatalf("Expected no error fetching profile, got %v", err)
	}
	if email != "jane.wanjiku@gmail.com" {
		t.Errorf("Expected primary email 'jane.wanjiku@gmail.com', got '%s'", email)
	}
	if fullName != "Jane Wanjiku" {
		t.Errorf("Expected full name 'Jane Wanjiku', got '%s'", fullName)
	}
}


