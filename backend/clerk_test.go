package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
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

func TestSignupAndClerkAutoProvisionPermissions(t *testing.T) {
	setupTestDB(t)
	_ = os.Setenv("JWT_SECRET", "test-jwt-secret-key-at-least-32-chars!!")
	defer os.Unsetenv("JWT_SECRET")

	// Generate ephemeral RSA keypair for mock Clerk JWKS
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	nEnc := base64.RawURLEncoding.EncodeToString(privKey.N.Bytes())
	eEnc := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.E)).Bytes())

	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ClerkJWKS{
			Keys: []ClerkJSONWebKey{{
				Kty: "RSA",
				Use: "sig",
				Kid: "test-kid-perm",
				N:   nEnc,
				E:   eEnc,
			}},
		})
	}))
	defer jwksSrv.Close()

	prevJWKS := os.Getenv("CLERK_JWKS_URL")
	_ = os.Setenv("CLERK_JWKS_URL", jwksSrv.URL)
	defer func() {
		if prevJWKS == "" {
			_ = os.Unsetenv("CLERK_JWKS_URL")
		} else {
			_ = os.Setenv("CLERK_JWKS_URL", prevJWKS)
		}
	}()

	signClerkJWT := func(sub, email, name string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":   sub,
			"email": email,
			"name":  name,
			"exp":   time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = "test-kid-perm"
		signed, signErr := tok.SignedString(privKey)
		if signErr != nil {
			t.Fatalf("Failed to sign mock Clerk JWT: %v", signErr)
		}
		return signed
	}

	app := fiber.New()
	app.Post("/auth/register", companyRegisterHandler)
	app.Post("/auth/clerk/sync", clerkSyncHandler)
	app.Post("/t/:tenant/auth/clerk/sync", clerkSyncHandler)
	app.Get("/t/:tenant/me", jwtMiddleware, tenantUserMiddleware, getTenantMeHandler)

	// 1. Standard company signup assigns RoleTenantAdmin and all 9 permissions
	regBody := []byte(`{"company_name":"Nairobi Heart Care","admin_email":"founder@nairobiheart.test","admin_password":"SecurePassword123!"}`)
	regReq := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq, 10000)
	if err != nil || regResp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 on /auth/register, got status=%d err=%v", regResp.StatusCode, err)
	}
	var regData struct {
		TenantID    string   `json:"tenant_id"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	_ = json.NewDecoder(regResp.Body).Decode(&regData)
	if regData.Role != RoleTenantAdmin || len(regData.Permissions) != len(AllPermissions) {
		t.Fatalf("Expected role=tenant_admin and %d permissions on /auth/register, got role=%s perms=%v",
			len(AllPermissions), regData.Role, regData.Permissions)
	}
	var regUser User
	db.Where("email = ?", "founder@nairobiheart.test").First(&regUser)
	regDBPerms, _ := getUserPermissions(regUser.ID)
	if len(regDBPerms) != len(AllPermissions) {
		t.Fatalf("Expected %d DB UserPermission rows for registered admin, got %d", len(AllPermissions), len(regDBPerms))
	}

	// 2. Global Google/Clerk signup auto-creates clinic and assigns RoleTenantAdmin + all 9 permissions
	clerkJWT := signClerkJWT("user_clerk_owner_1", "owner.google@westlandsclinic.test", "Dr. Ochieng")
	syncBody, _ := json.Marshal(map[string]string{
		"token":        clerkJWT,
		"company_name": "Westlands Care Clinic",
	})
	syncReq := httptest.NewRequest("POST", "/auth/clerk/sync", bytes.NewReader(syncBody))
	syncReq.Header.Set("Content-Type", "application/json")
	syncResp, err := app.Test(syncReq, 10000)
	if err != nil || syncResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on global /auth/clerk/sync, got status=%d err=%v", syncResp.StatusCode, err)
	}
	var syncData struct {
		Token string `json:"token"`
		User  struct {
			ID          uint     `json:"id"`
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
		Tenant struct {
			Slug string `json:"slug"`
		} `json:"tenant"`
	}
	_ = json.NewDecoder(syncResp.Body).Decode(&syncData)
	if syncData.User.Role != RoleTenantAdmin || len(syncData.User.Permissions) != len(AllPermissions) {
		t.Fatalf("Expected tenant_admin + %d permissions on Google signup, got role=%s perms=%v",
			len(AllPermissions), syncData.User.Role, syncData.User.Permissions)
	}
	clerkDBPerms, _ := getUserPermissions(syncData.User.ID)
	if len(clerkDBPerms) != len(AllPermissions) {
		t.Fatalf("Expected %d DB UserPermission rows for Google signup owner, got %d", len(AllPermissions), len(clerkDBPerms))
	}

	// Verify /t/:tenant/me returns all 9 permissions using issued Booka JWT
	meReq := httptest.NewRequest("GET", "/t/"+syncData.Tenant.Slug+"/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+syncData.Token)
	meResp, err := app.Test(meReq, 10000)
	if err != nil || meResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on /t/%s/me, got status=%d err=%v", syncData.Tenant.Slug, meResp.StatusCode, err)
	}
	var meData struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	_ = json.NewDecoder(meResp.Body).Decode(&meData)
	if meData.Role != RoleTenantAdmin || len(meData.Permissions) != len(AllPermissions) {
		t.Fatalf("Expected /me role=tenant_admin and %d permissions, got role=%s perms=%v",
			len(AllPermissions), meData.Role, meData.Permissions)
	}

	// 3. Tenant-scoped Clerk auto-provisioning creates RoleStaff with viewer permissions (not empty)
	staffJWT := signClerkJWT("user_clerk_staff_1", "frontdesk@westlandsclinic.test", "Grace Frontdesk")
	staffSyncBody, _ := json.Marshal(map[string]string{"token": staffJWT})
	staffReq := httptest.NewRequest("POST", "/t/"+syncData.Tenant.Slug+"/auth/clerk/sync", bytes.NewReader(staffSyncBody))
	staffReq.Header.Set("Content-Type", "application/json")
	staffResp, err := app.Test(staffReq, 10000)
	if err != nil || staffResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on tenant /auth/clerk/sync, got status=%d err=%v", staffResp.StatusCode, err)
	}
	var staffSyncData struct {
		User struct {
			ID          uint     `json:"id"`
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	_ = json.NewDecoder(staffResp.Body).Decode(&staffSyncData)
	if staffSyncData.User.Role != RoleStaff {
		t.Fatalf("Expected tenant auto-provisioned user role=staff, got %s", staffSyncData.User.Role)
	}
	joined := strings.Join(staffSyncData.User.Permissions, ",")
	if len(staffSyncData.User.Permissions) != len(PermissionPresets["viewer"]) ||
		!strings.Contains(joined, PermAppointmentsView) || !strings.Contains(joined, PermDoctorsView) {
		t.Fatalf("Expected viewer permissions [appointments.view, doctors.view] for auto-provisioned staff, got %v",
			staffSyncData.User.Permissions)
	}
}


