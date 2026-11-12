package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func setupAuthTestApp(t *testing.T) (*fiber.App, uint, string) {
	t.Helper()
	_ = validateConfig("development", "")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "auth_test.db")
	tenantID := initDatabase(dbPath, "development")

	app := fiber.New()

	app.Post("/auth/register", companyRegisterHandler)
	app.Post("/auth/platform/login", platformLoginHandler)
	app.Post("/platform/login", platformLoginHandler)
	app.Post("/auth/change-password", jwtMiddleware, changePasswordHandler)

	platform := app.Group("/platform", jwtMiddleware, platformAdminMiddleware)
	platform.Get("/companies", listCompanies)
	platform.Get("/users", listUsers)

	tenant := app.Group("/t/:tenant")
	tenant.Post("/register", registerHandler)
	tenant.Post("/login", loginHandler)
	tenant.Post("/change-password", jwtMiddleware, changePasswordHandler)

	admin := tenant.Group("/admin", jwtMiddleware, tenantAdminMiddleware)
	admin.Get("/appointments", listAppointments)
	admin.Post("/users", createUserHandler)

	return app, tenantID, defaultTenantSlug
}

func TestPasswordPolicy(t *testing.T) {
	cases := []struct {
		password string
		valid    bool
	}{
		{"short", false},
		{"123456789", false}, // 9 chars
		{"password123", false}, // common
		{"admin12345", false},  // common
		{"platform123", false}, // common
		{"StrongPassw0rd!", true},
		{"Correct-Horse-Battery-Staple", true},
	}

	for _, tc := range cases {
		err := validatePasswordPolicy(tc.password)
		if tc.valid && err != nil {
			t.Errorf("Expected password %q to be valid, got: %v", tc.password, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("Expected password %q to be invalid, but got nil error", tc.password)
		}
	}
}

func TestTokenClaimsAndIsolation(t *testing.T) {
	_ = setupTestDB(t)
	_ = validateConfig("development", "")

	// Platform admin token
	platTok, err := createPlatformAdminToken(99, "platform@example.com")
	if err != nil {
		t.Fatalf("Failed to create platform admin token: %v", err)
	}

	secret := getJWTSecret()
	tok, err := jwt.Parse(platTok, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("Failed to parse platform token: %v", err)
	}
	claims := tok.Claims.(jwt.MapClaims)
	if claims["aud"] != "platform_admin" {
		t.Errorf("Expected aud 'platform_admin', got %v", claims["aud"])
	}
	if claims["role"] != RolePlatformAdmin {
		t.Errorf("Expected role 'platform_admin', got %v", claims["role"])
	}
	if _, ok := claims["tenantId"]; ok {
		t.Errorf("Expected no tenantId in platform admin token, got %v", claims["tenantId"])
	}
	expFloat := claims["exp"].(float64)
	expiryTime := time.Unix(int64(expFloat), 0)
	duration := time.Until(expiryTime)
	if duration > 2*time.Hour+time.Minute || duration < 1*time.Hour+50*time.Minute {
		t.Errorf("Expected ~2h expiry for platform admin token, got duration: %v", duration)
	}

	// Tenant user token
	tenantTok, err := createTenantJWTToken(10, "staff@clinic.com", 2, RoleStaff, 1)
	if err != nil {
		t.Fatalf("Failed to create tenant token: %v", err)
	}
	tTok, err := jwt.Parse(tenantTok, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !tTok.Valid {
		t.Fatalf("Failed to parse tenant token: %v", err)
	}
	tClaims := tTok.Claims.(jwt.MapClaims)
	if tClaims["aud"] != "tenant_user" {
		t.Errorf("Expected aud 'tenant_user', got %v", tClaims["aud"])
	}
	if tClaims["role"] != RoleStaff {
		t.Errorf("Expected role 'staff', got %v", tClaims["role"])
	}
	if tClaims["tenantId"] != float64(2) {
		t.Errorf("Expected tenantId 2, got %v", tClaims["tenantId"])
	}
}

func TestBootstrapPlatformAdmin(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "bootstrap_test.db")
	_ = initDatabase(dbPath, "production")

	// Set env vars
	os.Setenv("PLATFORM_ADMIN_EMAIL", "bootstrap@platform.internal")
	os.Setenv("PLATFORM_ADMIN_PASSWORD", "SuperSecureBootstrap123!")
	defer func() {
		os.Unsetenv("PLATFORM_ADMIN_EMAIL")
		os.Unsetenv("PLATFORM_ADMIN_PASSWORD")
	}()

	// 1. Zero platform admins exist; bootstrap should create one
	bootstrapPlatformAdmin("production")

	var count int64
	db.Model(&User{}).Where("role = ?", RolePlatformAdmin).Count(&count)
	if count != 1 {
		t.Fatalf("Expected 1 platform admin after bootstrap, got %d", count)
	}

	var admin User
	db.Where("email = ?", "bootstrap@platform.internal").First(&admin)
	if admin.Role != RolePlatformAdmin || admin.TenantID != 0 {
		t.Errorf("Bootstrap admin properties unexpected: %+v", admin)
	}

	// 2. Calling again when 1 exists should be a no-op
	os.Setenv("PLATFORM_ADMIN_EMAIL", "another@platform.internal")
	bootstrapPlatformAdmin("production")

	db.Model(&User{}).Where("role = ?", RolePlatformAdmin).Count(&count)
	if count != 1 {
		t.Fatalf("Expected platform admin count to remain 1, got %d", count)
	}
}

func TestRoleInjectionPrevented(t *testing.T) {
	app, _, tenantSlug := setupAuthTestApp(t)

	// 1. Company registration cannot set role to platform_admin
	bodyComp := map[string]string{
		"company_name":   "Shield Clinic",
		"admin_email":    "doc@shieldclinic.com",
		"admin_password": "ValidPassword123!",
		"role":           "platform_admin", // Malicious injection
	}
	b, _ := json.Marshal(bodyComp)
	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("Company registration request failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Company registration returned status: %d", resp.StatusCode)
	}

	var registeredAdmin User
	if err := db.Where("email = ?", "doc@shieldclinic.com").First(&registeredAdmin).Error; err != nil {
		t.Fatalf("Registered admin not found in DB: %v", err)
	}
	if registeredAdmin.Role != RoleTenantAdmin {
		t.Fatalf("Expected RoleTenantAdmin, got %q", registeredAdmin.Role)
	}

	// 2. Tenant self-registration cannot set role
	bodyUser := map[string]string{
		"email":    "malicious@example.com",
		"password": "Password12345!",
		"role":     "platform_admin", // Malicious injection
	}
	b2, _ := json.Marshal(bodyUser)
	req2 := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/register", tenantSlug), bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, 10000)
	if err != nil {
		t.Fatalf("Tenant user register request failed: %v", err)
	}
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("Tenant user register status: %d", resp2.StatusCode)
	}

	var registeredStaff User
	if err := db.Where("email = ?", "malicious@example.com").First(&registeredStaff).Error; err != nil {
		t.Fatalf("Registered staff not found: %v", err)
	}
	if registeredStaff.Role != RoleStaff {
		t.Fatalf("Expected RoleStaff, got %q", registeredStaff.Role)
	}

	// 3. Tenant admin user creation cannot set role
	tenantAdminTok, err := createTenantJWTToken(registeredAdmin.ID, registeredAdmin.Email, registeredAdmin.TenantID, RoleTenantAdmin, registeredAdmin.TokenVersion)
	if err != nil {
		t.Fatalf("Failed to create tenant admin token: %v", err)
	}
	bodyCreate := map[string]string{
		"email":    "assistant@example.com",
		"password": "Password12345!",
		"role":     "platform_admin", // Malicious injection
	}
	b3, _ := json.Marshal(bodyCreate)
	req3 := httptest.NewRequest("POST", "/t/shield-clinic/admin/users", bytes.NewReader(b3))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+tenantAdminTok)
	resp3, err := app.Test(req3, 10000)
	if err != nil {
		t.Fatalf("Admin create user request failed: %v", err)
	}
	if resp3.StatusCode != http.StatusCreated {
		t.Fatalf("Admin create user status: %d", resp3.StatusCode)
	}

	var createdUser User
	if err := db.Where("email = ?", "assistant@example.com").First(&createdUser).Error; err != nil {
		t.Fatalf("Created user not found: %v", err)
	}
	if createdUser.Role != RoleStaff {
		t.Fatalf("Expected RoleStaff, got %q", createdUser.Role)
	}
}

func TestTokenBoundariesAndRevocation(t *testing.T) {
	app, tenantID, tenantSlug := setupAuthTestApp(t)

	// Create test platform admin in DB
	platHash, _ := hashPassword("PlatformSecurePassword123!")
	platUser := User{
		Email:        "plat@cloud.internal",
		Password:     platHash,
		Role:         RolePlatformAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     0,
	}
	if err := db.Create(&platUser).Error; err != nil {
		t.Fatalf("Failed to create platform user: %v", err)
	}
	platTok, err := createPlatformAdminToken(platUser.ID, platUser.Email)
	if err != nil {
		t.Fatalf("Failed to create platform token: %v", err)
	}

	// Create test tenant admin in DB
	tenantAdminHash, _ := hashPassword("TenantAdmin12345!")
	tenantAdminUser := User{
		Email:        "admin@clinic.internal",
		Password:     tenantAdminHash,
		Role:         RoleTenantAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	if err := db.Create(&tenantAdminUser).Error; err != nil {
		t.Fatalf("Failed to create tenant admin user: %v", err)
	}
	tenantAdminTok, err := createTenantJWTToken(tenantAdminUser.ID, tenantAdminUser.Email, tenantID, RoleTenantAdmin, 1)
	if err != nil {
		t.Fatalf("Failed to create tenant admin token: %v", err)
	}

	// 1. Platform admin token CAN access /platform/companies
	req1 := httptest.NewRequest("GET", "/platform/companies", nil)
	req1.Header.Set("Authorization", "Bearer "+platTok)
	resp1, err := app.Test(req1, 10000)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("Platform admin should access /platform/companies, got code %d, err: %v", resp1.StatusCode, err)
	}

	// 2. Platform admin token CANNOT access /t/:tenant/admin/appointments (403 Forbidden)
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	req2.Header.Set("Authorization", "Bearer "+platTok)
	resp2, err := app.Test(req2, 10000)
	if err != nil || resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("Platform admin token should be forbidden on tenant admin route, got code %d", resp2.StatusCode)
	}

	// 3. Tenant admin token CANNOT access /platform/companies (403 Forbidden)
	req3 := httptest.NewRequest("GET", "/platform/companies", nil)
	req3.Header.Set("Authorization", "Bearer "+tenantAdminTok)
	resp3, err := app.Test(req3, 10000)
	if err != nil || resp3.StatusCode != http.StatusForbidden {
		t.Fatalf("Tenant admin token should be forbidden on platform route, got code %d", resp3.StatusCode)
	}

	// 4. Tenant admin CAN access their own tenant admin route
	req4 := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	req4.Header.Set("Authorization", "Bearer "+tenantAdminTok)
	resp4, err := app.Test(req4, 10000)
	if err != nil || resp4.StatusCode != http.StatusOK {
		t.Fatalf("Tenant admin should access their tenant admin route, got code %d", resp4.StatusCode)
	}

	// 5. Disabled account is immediately rejected (403 Forbidden)
	db.Model(&User{}).Where("id = ?", tenantAdminUser.ID).Update("status", "disabled")
	req5 := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	req5.Header.Set("Authorization", "Bearer "+tenantAdminTok)
	resp5, err := app.Test(req5, 10000)
	if err != nil || resp5.StatusCode != http.StatusForbidden {
		t.Fatalf("Disabled user token should be rejected with 403 Forbidden, got code %d", resp5.StatusCode)
	}

	// Re-enable user, but bump token_version -> token is revoked (401 Unauthorized)
	db.Model(&User{}).Where("id = ?", tenantAdminUser.ID).Updates(map[string]interface{}{
		"status":        "active",
		"token_version": 2,
	})
	req6 := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	req6.Header.Set("Authorization", "Bearer "+tenantAdminTok) // Token has token_version = 1
	resp6, err := app.Test(req6, 10000)
	if err != nil || resp6.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Stale token_version should be rejected with 401 Unauthorized, got code %d", resp6.StatusCode)
	}
}
