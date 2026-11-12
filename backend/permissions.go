package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Permission constants
const (
	PermAppointmentsView   = "appointments.view"
	PermAppointmentsCreate = "appointments.create"
	PermAppointmentsEdit   = "appointments.edit"
	PermAppointmentsCancel = "appointments.cancel"
	PermAppointmentsDelete = "appointments.delete"
	PermDoctorsView        = "doctors.view"
	PermDoctorsManage      = "doctors.manage"
	PermReasonsManage      = "reasons.manage"
	PermSettingsManage     = "settings.manage"
)

var AllPermissions = []string{
	PermAppointmentsView,
	PermAppointmentsCreate,
	PermAppointmentsEdit,
	PermAppointmentsCancel,
	PermAppointmentsDelete,
	PermDoctorsView,
	PermDoctorsManage,
	PermReasonsManage,
	PermSettingsManage,
}

var ValidPermissionsMap = map[string]bool{
	PermAppointmentsView:   true,
	PermAppointmentsCreate: true,
	PermAppointmentsEdit:   true,
	PermAppointmentsCancel: true,
	PermAppointmentsDelete: true,
	PermDoctorsView:        true,
	PermDoctorsManage:      true,
	PermReasonsManage:      true,
	PermSettingsManage:     true,
}

var PermissionPresets = map[string][]string{
	"viewer": {
		PermAppointmentsView,
		PermDoctorsView,
	},
	"receptionist": {
		PermAppointmentsView,
		PermAppointmentsCreate,
		PermAppointmentsEdit,
		PermAppointmentsCancel,
		PermDoctorsView,
	},
	"manager": {
		PermAppointmentsView,
		PermAppointmentsCreate,
		PermAppointmentsEdit,
		PermAppointmentsCancel,
		PermAppointmentsDelete,
		PermDoctorsView,
		PermDoctorsManage,
		PermReasonsManage,
		PermSettingsManage,
	},
}

type UserPermission struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:idx_user_perm;not null" json:"user_id"`
	Permission string    `gorm:"uniqueIndex:idx_user_perm;size:100;not null" json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserWithPermissions struct {
	ID                 uint      `json:"id"`
	Email              string    `json:"email"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	MustChangePassword bool      `json:"must_change_password"`
	TokenVersion       int       `json:"token_version"`
	TenantID           uint      `json:"tenant_id"`
	Permissions        []string  `json:"permissions"`
	CreatedAt          time.Time `json:"created_at"`
}

// getUserPermissions returns the permissions slice for a user
func getUserPermissions(userID uint) ([]string, error) {
	var perms []UserPermission
	if err := db.Where("user_id = ?", userID).Find(&perms).Error; err != nil {
		return nil, err
	}
	result := make([]string, len(perms))
	for i, p := range perms {
		result[i] = p.Permission
	}
	return result, nil
}

// hasPermission checks if the authenticated user has a specific permission
func hasPermission(c *fiber.Ctx, perm string) bool {
	role, _ := c.Locals("role").(string)
	if role == RoleTenantAdmin {
		return true // tenant_admin implicitly has all permissions within tenant
	}

	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return false
	}

	var count int64
	db.Model(&UserPermission{}).Where("user_id = ? AND permission = ?", userID, perm).Count(&count)
	return count > 0
}

// requirePermission ensures the caller has the required permission for the current tenant route
func requirePermission(perm string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := validateTenantUser(c); err != nil {
			return err
		}

		role, _ := c.Locals("role").(string)
		// tenant_admin implicitly has all permissions within their tenant
		if role == RoleTenantAdmin {
			return c.Next()
		}

		if !hasPermission(c, perm) {
			return fiber.NewError(fiber.StatusForbidden, fmt.Sprintf("permission denied: missing '%s'", perm))
		}

		return c.Next()
	}
}

// listPermissionsHandler returns catalogue of all permissions and presets
func listPermissionsHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"permissions": AllPermissions,
		"presets":     PermissionPresets,
	})
}

// listTenantUsersHandler returns all users belonging to this tenant with their permissions
func listTenantUsersHandler(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(uint)

	var users []User
	if err := db.Where("tenant_id = ? AND role <> ?", tenantID, RolePlatformAdmin).Order("created_at DESC").Find(&users).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query users")
	}

	result := make([]UserWithPermissions, len(users))
	for i, u := range users {
		var perms []string
		if u.Role == RoleTenantAdmin {
			perms = AllPermissions
		} else {
			perms, _ = getUserPermissions(u.ID)
		}

		result[i] = UserWithPermissions{
			ID:                 u.ID,
			Email:              u.Email,
			Role:               u.Role,
			Status:             u.Status,
			MustChangePassword: u.MustChangePassword,
			TokenVersion:       u.TokenVersion,
			TenantID:           u.TenantID,
			Permissions:        perms,
			CreatedAt:          u.CreatedAt,
		}
	}

	return c.JSON(result)
}

type createTenantUserReq struct {
	Email       string   `json:"email"`
	Name        string   `json:"name,omitempty"`
	Password    string   `json:"password,omitempty"`
	Preset      string   `json:"preset,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

func generateTempPassword() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "TempPass!" + hex.EncodeToString(b)
}

// createTenantUserHandler creates a new staff user with permissions and temporary password
func createTenantUserHandler(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	tenantID := c.Locals("tenant_id").(uint)

	var req createTenantUserReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email is required")
	}
	if _, err := mail.ParseAddress(cleanEmail); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid email address")
	}

	tempPassword := strings.TrimSpace(req.Password)
	if tempPassword == "" {
		tempPassword = generateTempPassword()
	} else {
		if err := validatePasswordPolicy(tempPassword); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
	}

	// Resolve permissions
	var permsToAssign []string
	presetKey := strings.ToLower(strings.TrimSpace(req.Preset))
	if presetKey != "" {
		presetPerms, ok := PermissionPresets[presetKey]
		if !ok {
			return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid preset '%s'", req.Preset))
		}
		permsToAssign = presetPerms
	} else if len(req.Permissions) > 0 {
		permsToAssign = req.Permissions
	} else {
		permsToAssign = PermissionPresets["viewer"]
	}

	// Validate permissions
	cleanPerms := make([]string, 0, len(permsToAssign))
	seen := make(map[string]bool)
	for _, p := range permsToAssign {
		p = strings.TrimSpace(p)
		if ValidPermissionsMap[p] && !seen[p] {
			cleanPerms = append(cleanPerms, p)
			seen[p] = true
		}
	}

	hashed, err := hashPassword(tempPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to hash password")
	}

	newUser := User{
		Email:              cleanEmail,
		Password:           hashed,
		Role:               RoleStaff,
		Status:             "active",
		MustChangePassword: true,
		TokenVersion:       1,
		TenantID:           tenantID,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}
		for _, p := range cleanPerms {
			up := UserPermission{
				UserID:     newUser.ID,
				Permission: p,
			}
			if err := tx.Create(&up).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return fiber.NewError(fiber.StatusConflict, "user already exists or database error")
	}

	defer recordAuditLog(actorID, actorRole, tenantID, "create", "user", newUser.ID, c.IP())

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":                   newUser.ID,
		"email":                newUser.Email,
		"role":                 newUser.Role,
		"status":               newUser.Status,
		"must_change_password": newUser.MustChangePassword,
		"temporary_password":   tempPassword,
		"permissions":          cleanPerms,
	})
}

type updatePermissionsReq struct {
	Preset      string   `json:"preset,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// updateUserPermissionsHandler replaces user's permissions and bumps token_version
func updateUserPermissionsHandler(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	tenantID := c.Locals("tenant_id").(uint)

	idStr := c.Params("id")
	targetID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || targetID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	var targetUser User
	if err := db.Where("id = ? AND tenant_id = ?", targetID, tenantID).First(&targetUser).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found in tenant")
	}

	if targetUser.Role == RolePlatformAdmin {
		return fiber.NewError(fiber.StatusForbidden, "cannot modify platform admin")
	}

	if targetUser.Role == RoleTenantAdmin {
		return fiber.NewError(fiber.StatusBadRequest, "tenant admin has full access; cannot set permissions")
	}

	var req updatePermissionsReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	var permsToAssign []string
	presetKey := strings.ToLower(strings.TrimSpace(req.Preset))
	if presetKey != "" {
		presetPerms, ok := PermissionPresets[presetKey]
		if !ok {
			return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid preset '%s'", req.Preset))
		}
		permsToAssign = presetPerms
	} else if req.Permissions != nil {
		permsToAssign = req.Permissions
	} else {
		return fiber.NewError(fiber.StatusBadRequest, "preset or permissions array required")
	}

	cleanPerms := make([]string, 0, len(permsToAssign))
	seen := make(map[string]bool)
	for _, p := range permsToAssign {
		p = strings.TrimSpace(p)
		if ValidPermissionsMap[p] && !seen[p] {
			cleanPerms = append(cleanPerms, p)
			seen[p] = true
		}
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", targetUser.ID).Delete(&UserPermission{}).Error; err != nil {
			return err
		}
		for _, p := range cleanPerms {
			up := UserPermission{
				UserID:     targetUser.ID,
				Permission: p,
			}
			if err := tx.Create(&up).Error; err != nil {
				return err
			}
		}
		targetUser.TokenVersion++
		return tx.Model(&targetUser).Update("token_version", targetUser.TokenVersion).Error
	})

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update permissions")
	}

	defer recordAuditLog(actorID, actorRole, tenantID, "update_permissions", "user", targetUser.ID, c.IP())

	return c.JSON(fiber.Map{
		"message":       "permissions updated successfully",
		"id":            targetUser.ID,
		"token_version": targetUser.TokenVersion,
		"permissions":   cleanPerms,
	})
}

type updateStatusReq struct {
	Status string `json:"status"`
}

// updateUserStatusHandler enables or disables a tenant user, bumping token_version
func updateUserStatusHandler(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	tenantID := c.Locals("tenant_id").(uint)

	idStr := c.Params("id")
	targetID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || targetID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	var req updateStatusReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	newStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if newStatus != "active" && newStatus != "disabled" {
		return fiber.NewError(fiber.StatusBadRequest, "status must be 'active' or 'disabled'")
	}

	var targetUser User
	if err := db.Where("id = ? AND tenant_id = ?", targetID, tenantID).First(&targetUser).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found in tenant")
	}

	if targetUser.Role == RolePlatformAdmin {
		return fiber.NewError(fiber.StatusForbidden, "cannot modify platform admin")
	}

	// Cannot disable yourself
	if targetUser.ID == actorID && newStatus == "disabled" {
		return fiber.NewError(fiber.StatusBadRequest, "cannot disable your own account")
	}

	// Cannot disable the last active tenant_admin
	if targetUser.Role == RoleTenantAdmin && newStatus == "disabled" {
		var activeAdminCount int64
		db.Model(&User{}).Where("tenant_id = ? AND role = ? AND status = ?", tenantID, RoleTenantAdmin, "active").Count(&activeAdminCount)
		if activeAdminCount <= 1 {
			return fiber.NewError(fiber.StatusBadRequest, "cannot disable the last active tenant admin")
		}
	}

	targetUser.Status = newStatus
	targetUser.TokenVersion++
	if err := db.Save(&targetUser).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update user status")
	}

	defer recordAuditLog(actorID, actorRole, tenantID, "update_status", "user", targetUser.ID, c.IP())

	return c.JSON(fiber.Map{
		"message":       "user status updated successfully",
		"id":            targetUser.ID,
		"status":        targetUser.Status,
		"token_version": targetUser.TokenVersion,
	})
}

type resetPasswordReq struct {
	Password string `json:"password,omitempty"`
}

// resetUserPasswordHandler resets user password, setting must_change_password and bumping token_version
func resetUserPasswordHandler(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	tenantID := c.Locals("tenant_id").(uint)

	idStr := c.Params("id")
	targetID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || targetID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}

	var targetUser User
	if err := db.Where("id = ? AND tenant_id = ?", targetID, tenantID).First(&targetUser).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found in tenant")
	}

	if targetUser.Role == RolePlatformAdmin {
		return fiber.NewError(fiber.StatusForbidden, "cannot modify platform admin")
	}

	var req resetPasswordReq
	_ = c.BodyParser(&req)

	tempPassword := strings.TrimSpace(req.Password)
	if tempPassword == "" {
		tempPassword = generateTempPassword()
	} else {
		if err := validatePasswordPolicy(tempPassword); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
	}

	hashed, err := hashPassword(tempPassword)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to hash password")
	}

	targetUser.Password = hashed
	targetUser.MustChangePassword = true
	targetUser.TokenVersion++
	if err := db.Save(&targetUser).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to reset password")
	}

	defer recordAuditLog(actorID, actorRole, tenantID, "reset_password", "user", targetUser.ID, c.IP())

	return c.JSON(fiber.Map{
		"message":            "password reset successfully",
		"id":                 targetUser.ID,
		"temporary_password": tempPassword,
		"token_version":      targetUser.TokenVersion,
	})
}

// getTenantMeHandler returns current user's role, permissions, and must_change_password
func getTenantMeHandler(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	}

	var perms []string
	if user.Role == RoleTenantAdmin {
		perms = AllPermissions
	} else {
		perms, _ = getUserPermissions(user.ID)
	}

	return c.JSON(fiber.Map{
		"id":                   user.ID,
		"email":                user.Email,
		"role":                 user.Role,
		"status":               user.Status,
		"must_change_password": user.MustChangePassword,
		"permissions":          perms,
	})
}

