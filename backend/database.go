package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

const defaultTenantSlug = "default"

// backupDatabaseFile creates a timestamped copy of the database before migrations
func backupDatabaseFile(dbPath string) {
	if _, err := os.Stat(dbPath); err == nil {
		backupPath := fmt.Sprintf("%s.bak.%d", dbPath, time.Now().Unix())
		data, err := os.ReadFile(dbPath)
		if err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err == nil {
				log.Printf("[database] Created backup of %s at %s", dbPath, backupPath)
			}
		}
	}
}

// backfillDoctorIDs assigns doctor_id to legacy appointment rows matching doctor names
func backfillDoctorIDs() {
	var apps []Appointment
	if err := db.Where("doctor_id IS NULL OR doctor_id = 0").Find(&apps).Error; err != nil {
		log.Printf("[migration] Error querying appointments for doctor_id backfill: %v", err)
		return
	}

	for _, ap := range apps {
		docName := strings.TrimSpace(ap.Doctor)
		if docName == "" {
			log.Printf("[MIGRATION WARNING] Appointment ID %d has empty doctor field; cannot backfill doctor_id", ap.ID)
			continue
		}

		var matches []Doctor
		db.Where("tenant_id = ? AND LOWER(TRIM(name)) = LOWER(TRIM(?))", ap.TenantID, docName).Find(&matches)

		if len(matches) == 0 {
			// Strip prefix e.g. "Dr. "
			stripped := docName
			prefixes := []string{"Dr. ", "Dr.", "Dr ", "Doctor ", "doctor "}
			for _, p := range prefixes {
				if strings.HasPrefix(strings.ToLower(stripped), strings.ToLower(p)) {
					stripped = strings.TrimSpace(stripped[len(p):])
					break
				}
			}
			db.Where("tenant_id = ? AND (LOWER(TRIM(name)) = LOWER(TRIM(?)) OR LOWER(name) LIKE LOWER(?))",
				ap.TenantID, stripped, "%"+stripped+"%").Find(&matches)
		}

		if len(matches) == 1 {
			docID := matches[0].ID
			db.Model(&Appointment{}).Where("id = ?", ap.ID).Update("doctor_id", docID)
		} else if len(matches) == 0 {
			log.Printf("[MIGRATION WARNING] Appointment ID %d (Tenant %d) has unmatchable doctor '%s'. Cannot backfill doctor_id.", ap.ID, ap.TenantID, ap.Doctor)
		} else {
			log.Printf("[MIGRATION WARNING] Appointment ID %d (Tenant %d) has ambiguous doctor '%s' matching %d records. Leaving doctor_id unset.", ap.ID, ap.TenantID, ap.Doctor, len(matches))
		}
	}
}

// checkDuplicatesAndCreateActiveSlotIndex checks existing rows for conflicts and establishes the unique active slot index
func checkDuplicatesAndCreateActiveSlotIndex() {
	type dupResult struct {
		TenantID uint
		DoctorID uint
		Date     string
		Time     string
		Cnt      int
	}
	var dups []dupResult
	db.Raw(`
		SELECT tenant_id, doctor_id, date, time, COUNT(*) as cnt 
		FROM appointments 
		WHERE status <> 'cancelled' AND doctor_id IS NOT NULL AND doctor_id > 0
		GROUP BY tenant_id, doctor_id, date, time 
		HAVING cnt > 1
	`).Scan(&dups)

	if len(dups) > 0 {
		for _, d := range dups {
			log.Printf("[MIGRATION WARNING] Duplicate active appointment slot found: tenant=%d doctor=%d date=%s time=%s (count=%d). Please resolve manually.",
				d.TenantID, d.DoctorID, d.Date, d.Time, d.Cnt)
		}
	}

	// Create partial unique index
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_appointments_active_slot ON appointments(tenant_id, doctor_id, date, time) WHERE status <> 'cancelled'`).Error; err != nil {
		log.Printf("[migration] Error creating active slot unique index: %v", err)
	}
}

// initDatabase connects to SQLite with WAL mode and busy timeout, runs migrations, and conditionally seeds data
func initDatabase(dbPath string, appEnv string) uint {
	// 1. Back up database before migrations
	backupDatabaseFile(dbPath)

	// 2. Configure SQLite with WAL mode and busy timeout via DSN options
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000"
	} else {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000"
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&Tenant{}, &User{}, &Doctor{}, &ReasonOption{}, &Appointment{}, &AuditLog{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	if strings.EqualFold(appEnv, "development") {
		defaultTenantID := ensureDefaultTenant()
		backfillTenantIDs(defaultTenantID)
		seedSampleData(defaultTenantID)
		backfillDoctorIDs()
		checkDuplicatesAndCreateActiveSlotIndex()
		return defaultTenantID
	}

	// In non-development, do not auto-seed sample appointments or default tenant
	var t Tenant
	if err := db.Where("slug = ?", defaultTenantSlug).First(&t).Error; err == nil {
		backfillTenantIDs(t.ID)
		backfillDoctorIDs()
		checkDuplicatesAndCreateActiveSlotIndex()
		return t.ID
	}

	backfillDoctorIDs()
	checkDuplicatesAndCreateActiveSlotIndex()
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
	// Seed admin-managed doctor and reason options for initial tenant first
	doctorCount := int64(0)
	db.Model(&Doctor{}).Where("tenant_id = ?", defaultTenantID).Count(&doctorCount)
	if doctorCount == 0 {
		doctors := []Doctor{
			{TenantID: defaultTenantID, Name: "Dr. Kim", Specialty: "Primary Care", Bio: "Experienced general practitioner for family care.", WorkDays: "Mon,Tue,Wed,Thu,Fri,Sat", WorkStart: "09:00", WorkEnd: "17:00", SlotMinutes: 30},
			{TenantID: defaultTenantID, Name: "Dr. Mercy", Specialty: "Pediatrics", Bio: "Child health and wellness specialist.", WorkDays: "Mon,Tue,Wed,Thu,Fri,Sat", WorkStart: "09:00", WorkEnd: "17:00", SlotMinutes: 30},
			{TenantID: defaultTenantID, Name: "Dr. Lee", Specialty: "Cardiology", Bio: "Heart and vascular care with 10+ years of experience.", WorkDays: "Mon,Tue,Wed,Thu,Fri,Sat", WorkStart: "09:00", WorkEnd: "17:00", SlotMinutes: 30},
		}
		for _, d := range doctors {
			_ = db.Create(&d).Error
		}
	}

	var count int64
	db.Model(&Appointment{}).Where("tenant_id = ?", defaultTenantID).Count(&count)
	if count > 0 {
		return
	}

	var kim, mercy, lee Doctor
	_ = db.Where("tenant_id = ? AND name = ?", defaultTenantID, "Dr. Kim").First(&kim).Error
	_ = db.Where("tenant_id = ? AND name = ?", defaultTenantID, "Dr. Mercy").First(&mercy).Error
	_ = db.Where("tenant_id = ? AND name = ?", defaultTenantID, "Dr. Lee").First(&lee).Error

	now := time.Now()
	samples := []Appointment{
		{TenantID: defaultTenantID, DoctorID: &kim.ID, PatientName: "John Doe", Doctor: "Dr. Kim", Date: now.Format("2006-01-02"), Time: "10:00", Reason: "checkup", Status: StatusConfirmed},
		{TenantID: defaultTenantID, DoctorID: &mercy.ID, PatientName: "Jane Smith", Doctor: "Dr. Mercy", Date: now.AddDate(0, 0, 1).Format("2006-01-02"), Time: "11:00", Reason: "consultation", Status: StatusPending},
		{TenantID: defaultTenantID, DoctorID: &lee.ID, PatientName: "Alex Johnson", Doctor: "Dr. Lee", Date: now.AddDate(0, 0, 2).Format("2006-01-02"), Time: "15:30", Reason: "follow-up", Status: StatusPending},
	}
	for _, ap := range samples {
		_ = db.Create(&ap).Error
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
