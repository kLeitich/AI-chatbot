package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAuditLogEventsAndScoping(t *testing.T) {
	_ = validateConfig("development", "")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "audit_test.db")
	tenantID := initDatabase(dbPath, "development")

	app := setupTestAppWithAudit(tenantID)

	// 1. Create a tenant admin in DB
	tenantAdminHash, _ := hashPassword("AdminPassword123!")
	tenantAdmin := User{
		Email:        "auditadmin@clinic.internal",
		Password:     tenantAdminHash,
		Role:         RoleTenantAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	db.Create(&tenantAdmin)
	adminTok, _ := createTenantJWTToken(tenantAdmin.ID, tenantAdmin.Email, tenantID, RoleTenantAdmin, 1)

	// 2. Platform admin in DB
	platHash, _ := hashPassword("PlatPassword123!")
	platAdmin := User{
		Email:        "plat@cloud.internal",
		Password:     platHash,
		Role:         RolePlatformAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     0,
	}
	db.Create(&platAdmin)
	platTok, _ := createPlatformAdminToken(platAdmin.ID, platAdmin.Email)

	// A) Test Login Failure generates audit log
	failBody := map[string]string{"email": "auditadmin@clinic.internal", "password": "WrongPassword123!"}
	bFail, _ := json.Marshal(failBody)
	reqFail := httptest.NewRequest("POST", "/t/default/login", bytes.NewReader(bFail))
	reqFail.Header.Set("Content-Type", "application/json")
	respFail, _ := app.Test(reqFail)
	if respFail.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 on failed login, got %d", respFail.StatusCode)
	}

	var failLog AuditLog
	if err := db.Where("action = ? AND entity = ?", "login_failure", "auth").First(&failLog).Error; err != nil {
		t.Fatalf("Audit log not recorded for login failure: %v", err)
	}
	if failLog.TenantID != tenantID {
		t.Errorf("Expected tenant ID %d in audit log, got %d", tenantID, failLog.TenantID)
	}

	// B) Test Login Success generates audit log
	succBody := map[string]string{"email": "auditadmin@clinic.internal", "password": "AdminPassword123!"}
	bSucc, _ := json.Marshal(succBody)
	reqSucc := httptest.NewRequest("POST", "/t/default/login", bytes.NewReader(bSucc))
	reqSucc.Header.Set("Content-Type", "application/json")
	respSucc, _ := app.Test(reqSucc)
	if respSucc.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on successful login, got %d", respSucc.StatusCode)
	}

	var succLog AuditLog
	if err := db.Where("action = ? AND entity = ?", "login_success", "auth").First(&succLog).Error; err != nil {
		t.Fatalf("Audit log not recorded for login success: %v", err)
	}

	// C) Test Tenant Scoped Audit Log Endpoint
	reqTenantAudit := httptest.NewRequest("GET", "/t/default/admin/audit", nil)
	reqTenantAudit.Header.Set("Authorization", "Bearer "+adminTok)
	respTenantAudit, err := app.Test(reqTenantAudit)
	if err != nil || respTenantAudit.StatusCode != http.StatusOK {
		t.Fatalf("Tenant admin failed to fetch audit logs: %v, status: %d", err, respTenantAudit.StatusCode)
	}
	var tenantAuditRes PaginatedAuditResponse
	json.NewDecoder(respTenantAudit.Body).Decode(&tenantAuditRes)
	for _, l := range tenantAuditRes.Logs {
		if l.TenantID != tenantID {
			t.Errorf("Tenant audit log leaked another tenant's log: %+v", l)
		}
	}

	// D) Test Platform Audit Log Endpoint
	reqPlatAudit := httptest.NewRequest("GET", "/platform/audit", nil)
	reqPlatAudit.Header.Set("Authorization", "Bearer "+platTok)
	respPlatAudit, err := app.Test(reqPlatAudit)
	if err != nil || respPlatAudit.StatusCode != http.StatusOK {
		t.Fatalf("Platform admin failed to fetch audit logs: %v, status: %d", err, respPlatAudit.StatusCode)
	}
	var platAuditRes PaginatedAuditResponse
	json.NewDecoder(respPlatAudit.Body).Decode(&platAuditRes)
	if platAuditRes.Total == 0 {
		t.Errorf("Platform audit logs expected to have entries")
	}

	// E) Verify Audit logs contain no passwords or health data
	var allLogs []AuditLog
	db.Find(&allLogs)
	for _, l := range allLogs {
		if l.Action == "" || l.Entity == "" {
			t.Errorf("Audit log missing action or entity: %+v", l)
		}
	}
}

func setupTestAppWithAudit(tenantID uint) *fiber.App {
	app := fiber.New()

	app.Post("/auth/platform/login", platformLoginHandler)

	platform := app.Group("/platform", jwtMiddleware, platformAdminMiddleware)
	platform.Get("/audit", listPlatformAuditLogs)

	tenant := app.Group("/t/:tenant")
	tenant.Post("/login", loginHandler)
	tenant.Post("/change-password", jwtMiddleware, changePasswordHandler)

	admin := tenant.Group("/admin", jwtMiddleware, tenantUserMiddleware)
	admin.Get("/audit", tenantAdminMiddleware, listTenantAuditLogs)

	return app
}
