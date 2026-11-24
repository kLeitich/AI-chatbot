package main

import (
	"bytes"
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

	req := httptest.NewRequest("POST", "/auth/clerk/sync", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing token, got %d", resp.StatusCode)
	}
}
