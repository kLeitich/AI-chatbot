package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupPermissionsTestApp(t *testing.T) (*fiber.App, uint, string, string, User) {
	t.Helper()
	_ = validateConfig("development", "")
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "perm_test.db")
	tenantID := initDatabase(dbPath, "development")

	app := fiber.New()

	tenant := app.Group("/t/:tenant")
	tenant.Get("/me", jwtMiddleware, tenantUserMiddleware, getTenantMeHandler)

	admin := tenant.Group("/admin", jwtMiddleware, tenantUserMiddleware)
	admin.Get("/me", getTenantMeHandler)
	admin.Get("/appointments", requirePermission(PermAppointmentsView), listAppointments)
	admin.Post("/appointments", requirePermission(PermAppointmentsCreate), createAppointment)
	admin.Put("/appointments/:id", updateAppointment)
	admin.Delete("/appointments/:id", requirePermission(PermAppointmentsDelete), deleteAppointment)
	admin.Get("/doctors", requirePermission(PermDoctorsView), listDoctors)
	admin.Post("/doctors", requirePermission(PermDoctorsManage), createDoctor)

	admin.Get("/permissions", tenantAdminMiddleware, listPermissionsHandler)
	admin.Get("/users", tenantAdminMiddleware, listTenantUsersHandler)
	admin.Post("/users", tenantAdminMiddleware, createTenantUserHandler)
	admin.Patch("/users/:id/permissions", tenantAdminMiddleware, updateUserPermissionsHandler)
	admin.Patch("/users/:id/status", tenantAdminMiddleware, updateUserStatusHandler)
	admin.Post("/users/:id/reset-password", tenantAdminMiddleware, resetUserPasswordHandler)

	// Create test tenant admin
	adminHash, _ := hashPassword("AdminSecretPassword123!")
	adminUser := User{
		Email:        "admin@testclinic.internal",
		Password:     adminHash,
		Role:         RoleTenantAdmin,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	db.Create(&adminUser)
	adminTok, _ := createTenantJWTToken(adminUser.ID, adminUser.Email, tenantID, RoleTenantAdmin, 1)

	return app, tenantID, defaultTenantSlug, adminTok, adminUser
}

func TestPermissionsCatalogueAndPresets(t *testing.T) {
	app, _, tenantSlug, adminTok, _ := setupPermissionsTestApp(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/permissions", tenantSlug), nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	resp, err := app.Test(req, 10000)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch permissions catalogue: %v, status: %d", err, resp.StatusCode)
	}

	var data struct {
		Permissions []string            `json:"permissions"`
		Presets     map[string][]string `json:"presets"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	if len(data.Permissions) != 9 {
		t.Errorf("Expected 9 permissions, got %d", len(data.Permissions))
	}
	if len(data.Presets["viewer"]) != 2 {
		t.Errorf("Expected 2 permissions in viewer preset, got %d", len(data.Presets["viewer"]))
	}
	if len(data.Presets["receptionist"]) != 5 {
		t.Errorf("Expected 5 permissions in receptionist preset, got %d", len(data.Presets["receptionist"]))
	}
	if len(data.Presets["manager"]) != 9 {
		t.Errorf("Expected 9 permissions in manager preset, got %d", len(data.Presets["manager"]))
	}
}

func TestUserManagementCRUDAndRevocation(t *testing.T) {
	app, tenantID, tenantSlug, adminTok, adminUser := setupPermissionsTestApp(t)

	// 1. Create a staff user with "receptionist" preset
	body := map[string]interface{}{
		"email":  "reception@testclinic.internal",
		"preset": "receptionist",
	}
	b, _ := json.Marshal(body)
	req1 := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/admin/users", tenantSlug), bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+adminTok)
	resp1, err := app.Test(req1, 10000)
	if err != nil || resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create user with preset: %v, status: %d", err, resp1.StatusCode)
	}

	var created struct {
		ID                 uint     `json:"id"`
		Email              string   `json:"email"`
		Role               string   `json:"role"`
		Status             string   `json:"status"`
		MustChangePassword bool     `json:"must_change_password"`
		TemporaryPassword  string   `json:"temporary_password"`
		Permissions        []string `json:"permissions"`
	}
	json.NewDecoder(resp1.Body).Decode(&created)

	if created.Role != RoleStaff || !created.MustChangePassword {
		t.Fatalf("Created user properties incorrect: %+v", created)
	}
	if len(created.Permissions) != 5 {
		t.Fatalf("Expected 5 permissions for receptionist, got %d", len(created.Permissions))
	}

	// 2. Fetch /t/:tenant/admin/users
	reqList := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/users", tenantSlug), nil)
	reqList.Header.Set("Authorization", "Bearer "+adminTok)
	respList, _ := app.Test(reqList, 10000)
	var usersList []UserWithPermissions
	json.NewDecoder(respList.Body).Decode(&usersList)
	if len(usersList) < 2 {
		t.Fatalf("Expected at least 2 users in tenant, got %d", len(usersList))
	}

	// 3. Update permissions to viewer preset
	bodyUpdatePerm := map[string]interface{}{
		"preset": "viewer",
	}
	bUp, _ := json.Marshal(bodyUpdatePerm)
	reqUp := httptest.NewRequest("PATCH", fmt.Sprintf("/t/%s/admin/users/%d/permissions", tenantSlug, created.ID), bytes.NewReader(bUp))
	reqUp.Header.Set("Content-Type", "application/json")
	reqUp.Header.Set("Authorization", "Bearer "+adminTok)
	respUp, _ := app.Test(reqUp, 10000)
	if respUp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to update user permissions, status: %d", respUp.StatusCode)
	}

	permsAfter, _ := getUserPermissions(created.ID)
	if len(permsAfter) != 2 {
		t.Errorf("Expected 2 permissions after switching to viewer, got %d", len(permsAfter))
	}

	// 4. Status update: disable user
	bodyStatus := map[string]interface{}{
		"status": "disabled",
	}
	bSt, _ := json.Marshal(bodyStatus)
	reqSt := httptest.NewRequest("PATCH", fmt.Sprintf("/t/%s/admin/users/%d/status", tenantSlug, created.ID), bytes.NewReader(bSt))
	reqSt.Header.Set("Content-Type", "application/json")
	reqSt.Header.Set("Authorization", "Bearer "+adminTok)
	respSt, _ := app.Test(reqSt, 10000)
	if respSt.StatusCode != http.StatusOK {
		t.Fatalf("Failed to update user status to disabled, status: %d", respSt.StatusCode)
	}

	// 5. Cannot disable yourself
	reqSelf := httptest.NewRequest("PATCH", fmt.Sprintf("/t/%s/admin/users/%d/status", tenantSlug, adminUser.ID), bytes.NewReader(bSt))
	reqSelf.Header.Set("Content-Type", "application/json")
	reqSelf.Header.Set("Authorization", "Bearer "+adminTok)
	respSelf, _ := app.Test(reqSelf, 10000)
	if respSelf.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 when disabling own account, got %d", respSelf.StatusCode)
	}

	// 6. Reset password
	bodyReset := map[string]interface{}{
		"password": "NewResetPassword123!",
	}
	bRes, _ := json.Marshal(bodyReset)
	reqRes := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/admin/users/%d/reset-password", tenantSlug, created.ID), bytes.NewReader(bRes))
	reqRes.Header.Set("Content-Type", "application/json")
	reqRes.Header.Set("Authorization", "Bearer "+adminTok)
	respRes, _ := app.Test(reqRes, 10000)
	if respRes.StatusCode != http.StatusOK {
		t.Fatalf("Failed to reset password, status: %d", respRes.StatusCode)
	}

	var userInDB User
	db.First(&userInDB, created.ID)
	if !userInDB.MustChangePassword {
		t.Errorf("Expected must_change_password=true after password reset")
	}

	// 7. GET /t/:tenant/me returns current user details and permissions
	reqMe := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/me", tenantSlug), nil)
	reqMe.Header.Set("Authorization", "Bearer "+adminTok)
	respMe, _ := app.Test(reqMe, 10000)
	if respMe.StatusCode != http.StatusOK {
		t.Fatalf("Failed to call /t/:tenant/me, status: %d", respMe.StatusCode)
	}
	var meData map[string]interface{}
	json.NewDecoder(respMe.Body).Decode(&meData)
	if meData["role"] != RoleTenantAdmin {
		t.Errorf("Expected tenant_admin role in /me, got %v", meData["role"])
	}
	permsList := meData["permissions"].([]interface{})
	if len(permsList) != len(AllPermissions) {
		t.Errorf("Expected tenant admin to have all %d permissions, got %d", len(AllPermissions), len(permsList))
	}

	// 8. Staff user calling /t/:tenant/me
	staffUser := User{
		Email:        "staff_me@testclinic.internal",
		Role:         RoleStaff,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	db.Create(&staffUser)
	_ = db.Create(&UserPermission{UserID: staffUser.ID, Permission: PermAppointmentsView}).Error
	staffTok, _ := createTenantJWTToken(staffUser.ID, staffUser.Email, tenantID, RoleStaff, 1)

	reqStaffMe := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/me", tenantSlug), nil)
	reqStaffMe.Header.Set("Authorization", "Bearer "+staffTok)
	respStaffMe, _ := app.Test(reqStaffMe, 10000)
	if respStaffMe.StatusCode != http.StatusOK {
		t.Fatalf("Staff failed to call /t/:tenant/me: %d", respStaffMe.StatusCode)
	}
	var staffMeData map[string]interface{}
	json.NewDecoder(respStaffMe.Body).Decode(&staffMeData)
	if staffMeData["role"] != RoleStaff {
		t.Errorf("Expected staff role in /me, got %v", staffMeData["role"])
	}
	staffPerms := staffMeData["permissions"].([]interface{})
	if len(staffPerms) != 1 || staffPerms[0] != PermAppointmentsView {
		t.Errorf("Expected staff to have exactly [appointments.view], got %+v", staffPerms)
	}
}

func TestNonAdminPermissionMatrix(t *testing.T) {
	app, tenantID, tenantSlug, _, _ := setupPermissionsTestApp(t)

	// 1. Create a staff user with ONLY appointments.view
	viewerUser := User{
		Email:        "viewer@clinic.internal",
		Role:         RoleStaff,
		Status:       "active",
		TokenVersion: 1,
		TenantID:     tenantID,
	}
	db.Create(&viewerUser)
	db.Create(&UserPermission{UserID: viewerUser.ID, Permission: PermAppointmentsView})
	viewerTok, _ := createTenantJWTToken(viewerUser.ID, viewerUser.Email, tenantID, RoleStaff, 1)

	// Create an appointment for update/delete tests
	ap := Appointment{
		TenantID:    tenantID,
		PatientName: "Patient One",
		Doctor:      "Dr. Kim",
		Date:        "2026-11-20",
		Time:        "10:00",
		Reason:      "Checkup",
		Status:      StatusConfirmed,
	}
	db.Create(&ap)

	// a) CAN view appointments (200 OK)
	reqView := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), nil)
	reqView.Header.Set("Authorization", "Bearer "+viewerTok)
	respView, _ := app.Test(reqView, 10000)
	if respView.StatusCode != http.StatusOK {
		t.Fatalf("Viewer should be allowed to view appointments, got status %d", respView.StatusCode)
	}

	// b) CANNOT create appointment (403 Forbidden)
	createBody := map[string]string{
		"patient_name": "New Patient",
		"doctor":       "Dr. Kim",
		"date":         "2026-11-22",
		"time":         "11:00",
		"reason":       "Consultation",
	}
	bCr, _ := json.Marshal(createBody)
	reqCr := httptest.NewRequest("POST", fmt.Sprintf("/t/%s/admin/appointments", tenantSlug), bytes.NewReader(bCr))
	reqCr.Header.Set("Content-Type", "application/json")
	reqCr.Header.Set("Authorization", "Bearer "+viewerTok)
	respCr, _ := app.Test(reqCr, 10000)
	if respCr.StatusCode != http.StatusForbidden {
		t.Fatalf("Viewer should be forbidden from creating appointments, got %d", respCr.StatusCode)
	}

	// c) CANNOT cancel appointment (403 Forbidden)
	cancelBody := map[string]string{
		"status": "cancelled",
	}
	bCa, _ := json.Marshal(cancelBody)
	reqCa := httptest.NewRequest("PUT", fmt.Sprintf("/t/%s/admin/appointments/%d", tenantSlug, ap.ID), bytes.NewReader(bCa))
	reqCa.Header.Set("Content-Type", "application/json")
	reqCa.Header.Set("Authorization", "Bearer "+viewerTok)
	respCa, _ := app.Test(reqCa, 10000)
	if respCa.StatusCode != http.StatusForbidden {
		t.Fatalf("Viewer should be forbidden from cancelling appointments, got %d", respCa.StatusCode)
	}

	// d) CANNOT edit appointment (403 Forbidden)
	editBody := map[string]string{
		"reason": "Updated reason",
	}
	bEd, _ := json.Marshal(editBody)
	reqEd := httptest.NewRequest("PUT", fmt.Sprintf("/t/%s/admin/appointments/%d", tenantSlug, ap.ID), bytes.NewReader(bEd))
	reqEd.Header.Set("Content-Type", "application/json")
	reqEd.Header.Set("Authorization", "Bearer "+viewerTok)
	respEd, _ := app.Test(reqEd, 10000)
	if respEd.StatusCode != http.StatusForbidden {
		t.Fatalf("Viewer should be forbidden from editing appointments, got %d", respEd.StatusCode)
	}

	// e) CANNOT delete appointment (403 Forbidden)
	reqDel := httptest.NewRequest("DELETE", fmt.Sprintf("/t/%s/admin/appointments/%d", tenantSlug, ap.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+viewerTok)
	respDel, _ := app.Test(reqDel, 10000)
	if respDel.StatusCode != http.StatusForbidden {
		t.Fatalf("Viewer should be forbidden from deleting appointments, got %d", respDel.StatusCode)
	}

	// f) CANNOT access user management (403 Forbidden)
	reqUsers := httptest.NewRequest("GET", fmt.Sprintf("/t/%s/admin/users", tenantSlug), nil)
	reqUsers.Header.Set("Authorization", "Bearer "+viewerTok)
	respUsers, _ := app.Test(reqUsers, 10000)
	if respUsers.StatusCode != http.StatusForbidden {
		t.Fatalf("Viewer should be forbidden from accessing user management, got %d", respUsers.StatusCode)
	}
}

func TestDataMigrationToPermissions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "migration_perm_test.db")
	tenantID := initDatabase(dbPath, "development")

	// Insert legacy users with role "user"
	u1 := User{Email: "legacy1@clinic.com", Role: "user", TenantID: tenantID}
	u2 := User{Email: "legacy2@clinic.com", Role: "user", TenantID: tenantID}
	db.Create(&u1)
	db.Create(&u2)

	// Run migration
	migrateUsersToPermissions()

	// Verify users were converted to role "staff"
	var checkU1, checkU2 User
	db.First(&checkU1, u1.ID)
	db.First(&checkU2, u2.ID)

	if checkU1.Role != RoleStaff || checkU2.Role != RoleStaff {
		t.Errorf("Expected role 'staff' after migration, got u1=%s, u2=%s", checkU1.Role, checkU2.Role)
	}

	// Verify "viewer" permissions were seeded
	permsU1, _ := getUserPermissions(u1.ID)
	if len(permsU1) != 2 {
		t.Errorf("Expected 2 permissions (viewer) for migrated user, got %d", len(permsU1))
	}
}

