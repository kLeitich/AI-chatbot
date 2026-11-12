package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupPlatformTestApp(t *testing.T) (*fiber.App, uint, string, string) {
	t.Helper()
	_ = validateConfig("development", "")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "platform_test.db")
	tenantID := initDatabase(dbPath, "development")

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})

	// Platform admin endpoints (strictly read-only views)
	platform := app.Group("/platform", jwtMiddleware, platformAdminMiddleware)
	platform.Get("/tenants", listPlatformTenants)
	platform.Get("/companies", listPlatformTenants)
	platform.Get("/users", listPlatformUsers)
	platform.Get("/doctors", listPlatformDoctors)
	platform.Get("/appointments", listPlatformAppointments)
	platform.Get("/audit", listPlatformAuditLogs)

	// Tenant admin endpoints
	tenant := app.Group("/t/:tenant")
	admin := tenant.Group("/admin", jwtMiddleware, tenantAdminMiddleware)
	admin.Get("/appointments", listAppointments)
	admin.Post("/appointments", createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", deleteAppointment)
	admin.Post("/doctors", createDoctor)

	// Create test platform admin
	platUser := User{
		Email:        "superadmin@platform.internal",
		Role:         RolePlatformAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     0,
	}
	db.Create(&platUser)
	platTok, _ := createPlatformAdminToken(platUser.ID, platUser.Email)

	return app, tenantID, defaultTenantSlug, platTok
}

func TestPlatformViewTenantsAndCounts(t *testing.T) {
	app, tenantID, _, platTok := setupPlatformTestApp(t)

	// Seed second tenant
	t2 := Tenant{
		Slug: "eldoret-clinic",
		Name: "Eldoret Clinic",
	}
	db.Create(&t2)

	// Add users
	db.Create(&User{Email: "staff1@default.com", TenantID: tenantID, Role: RoleStaff, Status: "active"})
	db.Create(&User{Email: "staff2@eldoret.com", TenantID: t2.ID, Role: RoleStaff, Status: "active"})

	// Add appointments
	db.Create(&Appointment{TenantID: tenantID, PatientName: "P1", Doctor: "Dr. Kim", Date: "2026-11-20", Time: "09:00", Status: StatusConfirmed})
	db.Create(&Appointment{TenantID: tenantID, PatientName: "P2", Doctor: "Dr. Kim", Date: "2026-11-20", Time: "10:00", Status: StatusConfirmed})
	db.Create(&Appointment{TenantID: t2.ID, PatientName: "P3", Doctor: "Dr. A", Date: "2026-11-21", Time: "11:00", Status: StatusConfirmed})

	req := httptest.NewRequest("GET", "/platform/tenants", nil)
	req.Header.Set("Authorization", "Bearer "+platTok)
	resp, err := app.Test(req, 10000)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch /platform/tenants: %v, status: %d", err, resp.StatusCode)
	}

	var summaries []TenantSummary
	if err := json.NewDecoder(resp.Body).Decode(&summaries); err != nil {
		t.Fatalf("Failed to decode tenant summaries: %v", err)
	}

	if len(summaries) < 2 {
		t.Fatalf("Expected at least 2 tenants, got %d", len(summaries))
	}

	// Verify audit log entry was created
	var audit AuditLog
	if err := db.Where("action = ? AND entity = ?", "read", "tenants").First(&audit).Error; err != nil {
		t.Fatalf("Expected audit log for tenants read, got err: %v", err)
	}
	if audit.ActorRole != RolePlatformAdmin {
		t.Errorf("Expected actor role 'platform_admin', got %s", audit.ActorRole)
	}
}

func TestPlatformViewAppointmentsWithFilters(t *testing.T) {
	app, _, _, platTok := setupPlatformTestApp(t)

	t1 := Tenant{Slug: "filter-clinic-1", Name: "Filter Clinic 1"}
	t2 := Tenant{Slug: "nakuru-care", Name: "Nakuru Care"}
	db.Create(&t1)
	db.Create(&t2)

	doc1 := Doctor{TenantID: t1.ID, Name: "Dr. Otieno"}
	doc2 := Doctor{TenantID: t2.ID, Name: "Dr. Wanjiku"}
	db.Create(&doc1)
	db.Create(&doc2)

	db.Create(&Appointment{TenantID: t1.ID, DoctorID: &doc1.ID, Doctor: doc1.Name, PatientName: "Alice", Date: "2026-11-15", Time: "09:00", Status: StatusConfirmed})
	db.Create(&Appointment{TenantID: t1.ID, DoctorID: &doc1.ID, Doctor: doc1.Name, PatientName: "Bob", Date: "2026-11-20", Time: "10:00", Status: StatusCancelled})
	db.Create(&Appointment{TenantID: t2.ID, DoctorID: &doc2.ID, Doctor: doc2.Name, PatientName: "Charlie", Date: "2026-11-25", Time: "11:00", Status: StatusConfirmed})

	// 1. Filter by tenant_id
	urlTenant := fmt.Sprintf("/platform/appointments?tenant_id=%d", t1.ID)
	req1 := httptest.NewRequest("GET", urlTenant, nil)
	req1.Header.Set("Authorization", "Bearer "+platTok)
	resp1, err := app.Test(req1, 10000)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("Failed with tenant filter: %v, status: %d", err, resp1.StatusCode)
	}
	var res1 PaginatedAppointmentsResponse
	json.NewDecoder(resp1.Body).Decode(&res1)
	if res1.Total != 2 {
		t.Errorf("Expected 2 appointments for tenant %d, got %d", t1.ID, res1.Total)
	}

	// 2. Filter by status on this tenant
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/platform/appointments?tenant_id=%d&status=confirmed", t1.ID), nil)
	req2.Header.Set("Authorization", "Bearer "+platTok)
	resp2, _ := app.Test(req2, 10000)
	var res2 PaginatedAppointmentsResponse
	json.NewDecoder(resp2.Body).Decode(&res2)
	if res2.Total != 1 {
		t.Errorf("Expected 1 confirmed appointment for tenant %d, got %d", t1.ID, res2.Total)
	}

	// 3. Filter by date range across clinics
	req3 := httptest.NewRequest("GET", "/platform/appointments?start_date=2026-11-18&end_date=2026-11-22", nil)
	req3.Header.Set("Authorization", "Bearer "+platTok)
	resp3, _ := app.Test(req3, 10000)
	var res3 PaginatedAppointmentsResponse
	json.NewDecoder(resp3.Body).Decode(&res3)
	if res3.Total != 1 {
		t.Errorf("Expected 1 appointment between Nov 18 and Nov 22, got %d", res3.Total)
	}
}

func TestPlatformAdminBlockedFromTenantAdminWrites(t *testing.T) {
	app, _, tenantSlug, platTok := setupPlatformTestApp(t)

	// Attempt to create an appointment on tenant admin endpoint
	req1 := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	req1.Header.Set("Authorization", "Bearer "+platTok)
	resp1, _ := app.Test(req1, 10000)
	if resp1.StatusCode != http.StatusForbidden {
		t.Fatalf("Platform admin should be blocked from tenant admin appointment creation with 403, got %d", resp1.StatusCode)
	}

	// Attempt to create a doctor on tenant admin endpoint
	req2 := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/admin/doctors", tenantSlug), nil)
	req2.Header.Set("Authorization", "Bearer "+platTok)
	resp2, _ := app.Test(req2, 10000)
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("Platform admin should be blocked from tenant admin doctor creation with 403, got %d", resp2.StatusCode)
	}
}
