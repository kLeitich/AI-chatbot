package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type TenantSummary struct {
	ID               uint      `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	EmergencyNumber  string    `json:"emergency_number,omitempty"`
	EmergencyMessage string    `json:"emergency_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UserCount        int64     `json:"user_count"`
	AppointmentCount int64     `json:"appointment_count"`
}

func recordAuditLog(actorUserID uint, actorRole string, tenantID uint, action, entity string, entityID uint, ip string) {
	entry := AuditLog{
		At:          time.Now(),
		ActorUserID: actorUserID,
		ActorRole:   actorRole,
		TenantID:    tenantID,
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		IP:          ip,
	}
	_ = db.Create(&entry).Error
}

func getActorInfo(c *fiber.Ctx) (uint, string) {
	uid, _ := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)
	return uid, role
}

// listPlatformTenants returns all tenants with user and appointment counts
func listPlatformTenants(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	defer recordAuditLog(actorID, actorRole, 0, "read", "tenants", 0, c.IP())

	var tenants []Tenant
	if err := db.Order("created_at DESC").Find(&tenants).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query tenants")
	}

	summaries := make([]TenantSummary, len(tenants))
	for i, t := range tenants {
		var uCount, aCount int64
		db.Model(&User{}).Where("tenant_id = ?", t.ID).Count(&uCount)
		db.Model(&Appointment{}).Where("tenant_id = ?", t.ID).Count(&aCount)

		summaries[i] = TenantSummary{
			ID:               t.ID,
			Slug:             t.Slug,
			Name:             t.Name,
			EmergencyNumber:  t.EmergencyNumber,
			EmergencyMessage: t.EmergencyMessage,
			CreatedAt:        t.CreatedAt,
			UserCount:        uCount,
			AppointmentCount: aCount,
		}
	}

	return c.JSON(summaries)
}

// listPlatformUsers returns users scoped by optional tenant_id, excluding platform admins
func listPlatformUsers(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)

	var targetTenantID uint
	query := db.Model(&User{}).Where("role <> ?", RolePlatformAdmin)
	if tParam := c.Query("tenant_id"); tParam != "" {
		if tid, err := strconv.ParseUint(tParam, 10, 64); err == nil && tid > 0 {
			targetTenantID = uint(tid)
			query = query.Where("tenant_id = ?", targetTenantID)
		}
	}

	defer recordAuditLog(actorID, actorRole, targetTenantID, "read", "users", 0, c.IP())

	var users []User
	if err := query.Order("created_at DESC").Find(&users).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query users")
	}

	return c.JSON(users)
}

// listPlatformDoctors returns doctors across clinics with optional tenant_id filter
func listPlatformDoctors(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)

	var targetTenantID uint
	query := db.Model(&Doctor{})
	if tParam := c.Query("tenant_id"); tParam != "" {
		if tid, err := strconv.ParseUint(tParam, 10, 64); err == nil && tid > 0 {
			targetTenantID = uint(tid)
			query = query.Where("tenant_id = ?", targetTenantID)
		}
	}

	defer recordAuditLog(actorID, actorRole, targetTenantID, "read", "doctors", 0, c.IP())

	var doctors []Doctor
	if err := query.Order("name ASC").Find(&doctors).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query doctors")
	}

	return c.JSON(doctors)
}

type PaginatedAppointmentsResponse struct {
	Appointments []Appointment `json:"appointments"`
	Total        int64         `json:"total"`
	Page         int           `json:"page"`
	Limit        int           `json:"limit"`
}

// listPlatformAppointments returns appointments across tenants with query filters
func listPlatformAppointments(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)

	query := db.Model(&Appointment{})
	var targetTenantID uint

	if tParam := c.Query("tenant_id"); tParam != "" {
		if tid, err := strconv.ParseUint(tParam, 10, 64); err == nil && tid > 0 {
			targetTenantID = uint(tid)
			query = query.Where("tenant_id = ?", targetTenantID)
		}
	}

	if dParam := c.Query("doctor_id"); dParam != "" {
		if did, err := strconv.ParseUint(dParam, 10, 64); err == nil && did > 0 {
			query = query.Where("doctor_id = ?", uint(did))
		}
	}

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		query = query.Where("date >= ?", startDate)
	}

	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		query = query.Where("date <= ?", endDate)
	}

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	defer recordAuditLog(actorID, actorRole, targetTenantID, "read", "appointments", 0, c.IP())

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count appointments")
	}

	page := 1
	if pStr := c.Query("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			if l > 100 {
				limit = 100
			} else {
				limit = l
			}
		}
	}

	offset := (page - 1) * limit
	var appointments []Appointment
	if err := query.Order("date DESC, time DESC").Offset(offset).Limit(limit).Find(&appointments).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query appointments")
	}

	return c.JSON(PaginatedAppointmentsResponse{
		Appointments: appointments,
		Total:        total,
		Page:         page,
		Limit:        limit,
	})
}

type PaginatedAuditResponse struct {
	Logs  []AuditLog `json:"logs"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

// listPlatformAuditLogs returns system audit logs with filtering
func listPlatformAuditLogs(c *fiber.Ctx) error {
	actorID, actorRole := getActorInfo(c)
	defer recordAuditLog(actorID, actorRole, 0, "read", "audit_logs", 0, c.IP())

	query := db.Model(&AuditLog{})

	if tParam := c.Query("tenant_id"); tParam != "" {
		if tid, err := strconv.ParseUint(tParam, 10, 64); err == nil {
			query = query.Where("tenant_id = ?", uint(tid))
		}
	}

	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}

	if entity := strings.TrimSpace(c.Query("entity")); entity != "" {
		query = query.Where("entity = ?", entity)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count audit logs")
	}

	page := 1
	if pStr := c.Query("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			if l > 100 {
				limit = 100
			} else {
				limit = l
			}
		}
	}

	offset := (page - 1) * limit
	var logs []AuditLog
	if err := query.Order("at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query audit logs")
	}

	return c.JSON(PaginatedAuditResponse{
		Logs:  logs,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// listTenantAuditLogs returns audit logs strictly scoped to the tenant
func listTenantAuditLogs(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid tenant context")
	}

	query := db.Model(&AuditLog{}).Where("tenant_id = ?", tenantID)

	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}

	if entity := strings.TrimSpace(c.Query("entity")); entity != "" {
		query = query.Where("entity = ?", entity)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count audit logs")
	}

	page := 1
	if pStr := c.Query("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			if l > 100 {
				limit = 100
			} else {
				limit = l
			}
		}
	}

	offset := (page - 1) * limit
	var logs []AuditLog
	if err := query.Order("at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to query audit logs")
	}

	return c.JSON(PaginatedAuditResponse{
		Logs:  logs,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

