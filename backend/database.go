package main

import (
	"log"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

const defaultTenantSlug = "default"

// initDatabase connects to SQLite, runs migrations, and conditionally seeds data in development
func initDatabase(dbPath string, appEnv string) uint {
	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&Tenant{}, &User{}, &Doctor{}, &ReasonOption{}, &Appointment{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	if strings.EqualFold(appEnv, "development") {
		defaultTenantID := ensureDefaultTenant()
		// Backfill any legacy rows (from before multitenancy) to the default tenant.
		backfillTenantIDs(defaultTenantID)
		seedSampleData(defaultTenantID)
		return defaultTenantID
	}

	// In non-development, do not auto-seed sample appointments or default tenant
	var t Tenant
	if err := db.Where("slug = ?", defaultTenantSlug).First(&t).Error; err == nil {
		backfillTenantIDs(t.ID)
		return t.ID
	}
	return 0
}

// ensureDefaultTenant creates (or loads) the default tenant used for local dev.
func ensureDefaultTenant() uint {
	var t Tenant
	if err := db.Where("slug = ?", defaultTenantSlug).First(&t).Error; err == nil {
		return t.ID
	}

	t = Tenant{
		Slug: defaultTenantSlug,
		Name: "Default Clinic",
	}
	if err := db.Create(&t).Error; err != nil {
		log.Fatalf("failed to create default tenant: %v", err)
	}
	return t.ID
}

// backfillTenantIDs assigns the default tenant to any existing users/appointments
// that don't yet have a tenant_id (from pre-multitenant deployments).
func backfillTenantIDs(defaultTenantID uint) {
	// Users
	if err := db.Model(&User{}).
		Where("tenant_id IS NULL OR tenant_id = 0").
		Update("tenant_id", defaultTenantID).Error; err != nil {
		log.Printf("failed to backfill user tenant IDs: %v", err)
	}

	// Appointments
	if err := db.Model(&Appointment{}).
		Where("tenant_id IS NULL OR tenant_id = 0").
		Update("tenant_id", defaultTenantID).Error; err != nil {
		log.Printf("failed to backfill appointment tenant IDs: %v", err)
	}
}

func seedSampleData(defaultTenantID uint) {
	var count int64
	db.Model(&Appointment{}).Where("tenant_id = ?", defaultTenantID).Count(&count)
	if count > 0 {
		return
	}
	now := time.Now()
	samples := []Appointment{
		{TenantID: defaultTenantID, PatientName: "John Doe", Doctor: "Dr. Kim", Date: now.Format("2006-01-02"), Time: "10:00", Reason: "checkup", Status: "confirmed"},
		{TenantID: defaultTenantID, PatientName: "Jane Smith", Doctor: "Dr. Mercy", Date: now.AddDate(0, 0, 1).Format("2006-01-02"), Time: "11:00", Reason: "consultation", Status: "pending"},
		{TenantID: defaultTenantID, PatientName: "Alex Johnson", Doctor: "Dr. Lee", Date: now.AddDate(0, 0, 2).Format("2006-01-02"), Time: "15:30", Reason: "follow-up", Status: "pending"},
	}
	for _, ap := range samples {
		_ = db.Create(&ap).Error
	}

	// Seed admin-managed doctor and reason options for initial tenant
	doctorCount := int64(0)
	db.Model(&Doctor{}).Where("tenant_id = ?", defaultTenantID).Count(&doctorCount)
	if doctorCount == 0 {
		doctors := []Doctor{
			{TenantID: defaultTenantID, Name: "Dr. Kim", Specialty: "Primary Care", Bio: "Experienced general practitioner for family care."},
			{TenantID: defaultTenantID, Name: "Dr. Mercy", Specialty: "Pediatrics", Bio: "Child health and wellness specialist."},
			{TenantID: defaultTenantID, Name: "Dr. Lee", Specialty: "Cardiology", Bio: "Heart and vascular care with 10+ years of experience."},
		}
		for _, d := range doctors {
			_ = db.Create(&d).Error
		}
	}

	reasonCount := int64(0)
	db.Model(&ReasonOption{}).Where("tenant_id = ?", defaultTenantID).Count(&reasonCount)
	if reasonCount == 0 {
		reasons := []ReasonOption{
			{TenantID: defaultTenantID, Label: "checkup"},
			{TenantID: defaultTenantID, Label: "consultation"},
			{TenantID: defaultTenantID, Label: "follow-up"},
		}
		for _, r := range reasons {
			_ = db.Create(&r).Error
		}
	}
}
