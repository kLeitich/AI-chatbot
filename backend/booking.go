package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"gorm.io/gorm"
)

// BookingConflictError is returned when a requested slot is already taken
type BookingConflictError struct {
	Message      string
	Alternatives []string
}

func (e *BookingConflictError) Error() string {
	return e.Message
}

// getAppLocation returns the configured app timezone, defaulting to Africa/Nairobi
func getAppLocation() *time.Location {
	tz := os.Getenv("APP_TIMEZONE")
	if tz == "" {
		tz = "Africa/Nairobi"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.FixedZone("EAT", 3*3600)
	}
	return loc
}

// nowInAppLocation returns the current wall-clock time in the configured app timezone
func nowInAppLocation() time.Time {
	return time.Now().In(getAppLocation())
}

func dayIndex(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "sun", "sunday":
		return 0
	case "mon", "monday":
		return 1
	case "tue", "tuesday":
		return 2
	case "wed", "wednesday":
		return 3
	case "thu", "thursday":
		return 4
	case "fri", "friday":
		return 5
	case "sat", "saturday":
		return 6
	default:
		return -1
	}
}

// isDoctorWorkingOnDay checks if a doctor is scheduled to work on the given date
func isDoctorWorkingOnDay(doc Doctor, t time.Time) bool {
	workDays := strings.TrimSpace(doc.WorkDays)
	if workDays == "" {
		workDays = "Mon,Tue,Wed,Thu,Fri,Sat"
	}

	dayName := t.Format("Mon") // e.g. "Mon", "Tue"

	// If specified as a range like "Mon-Sat" or "Mon-Fri"
	if strings.Contains(workDays, "-") {
		parts := strings.Split(workDays, "-")
		if len(parts) == 2 {
			startDay := dayIndex(parts[0])
			endDay := dayIndex(parts[1])
			currDay := int(t.Weekday())
			if startDay >= 0 && endDay >= 0 && startDay <= endDay {
				return currDay >= startDay && currDay <= endDay
			}
		}
	}

	// Comma-separated list e.g. "Mon,Tue,Wed,Thu,Fri,Sat"
	days := strings.Split(workDays, ",")
	for _, d := range days {
		if strings.EqualFold(strings.TrimSpace(d), dayName) {
			return true
		}
	}
	return false
}

// generateDoctorSlots produces all valid slot times (HH:MM) for a doctor on a specific date
func generateDoctorSlots(doc Doctor, dateStr string) ([]string, error) {
	loc := getAppLocation()
	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	if !isDoctorWorkingOnDay(doc, t) {
		return []string{}, nil
	}

	start := strings.TrimSpace(doc.WorkStart)
	if start == "" {
		start = "09:00"
	}
	end := strings.TrimSpace(doc.WorkEnd)
	if end == "" {
		end = "17:00"
	}
	slotMins := doc.SlotMinutes
	if slotMins <= 0 {
		slotMins = 30
	}

	startTime, err := time.Parse("15:04", start)
	if err != nil {
		startTime, _ = time.Parse("15:04", "09:00")
	}
	endTime, err := time.Parse("15:04", end)
	if err != nil {
		endTime, _ = time.Parse("15:04", "17:00")
	}

	var slots []string
	curr := startTime
	for {
		next := curr.Add(time.Duration(slotMins) * time.Minute)
		if next.After(endTime) {
			break
		}
		slots = append(slots, curr.Format("15:04"))
		curr = next
	}

	return slots, nil
}

// resolveDoctor finds a Doctor row for a tenant case-insensitively, or returns error with valid doctor list
func resolveDoctor(tenantID uint, doctorName string) (Doctor, error) {
	cleanName := strings.TrimSpace(doctorName)
	if cleanName == "" {
		return Doctor{}, errors.New("doctor name is required")
	}

	// 1. Exact match (case-insensitive) on name for this tenant (active doctors only)
	var doc Doctor
	err := db.Where("tenant_id = ? AND LOWER(TRIM(name)) = LOWER(TRIM(?)) AND active = ?", tenantID, cleanName, true).First(&doc).Error
	if err == nil {
		return doc, nil
	}

	// 2. Match without common doctor prefixes ("Dr.", "Dr ", "Doctor ")
	stripped := cleanName
	prefixes := []string{"Dr. ", "Dr.", "Dr ", "Doctor ", "doctor "}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(stripped), strings.ToLower(p)) {
			stripped = strings.TrimSpace(stripped[len(p):])
			break
		}
	}

	err = db.Where("tenant_id = ? AND (LOWER(TRIM(name)) = LOWER(TRIM(?)) OR LOWER(name) LIKE LOWER(?)) AND active = ?",
		tenantID, stripped, "%"+stripped+"%", true).First(&doc).Error
	if err == nil {
		return doc, nil
	}

	// List all valid active doctors for this tenant to help the user choose
	var allDocs []Doctor
	_ = db.Where("tenant_id = ? AND active = ?", tenantID, true).Order("name ASC").Find(&allDocs).Error
	var names []string
	for _, d := range allDocs {
		names = append(names, d.Name)
	}

	if len(names) == 0 {
		return Doctor{}, fmt.Errorf("doctor '%s' not recognized. No doctors are currently registered for this clinic", doctorName)
	}
	return Doctor{}, fmt.Errorf("doctor '%s' not recognized. Available doctors: %s", doctorName, strings.Join(names, ", "))
}

// availableTimeSlots returns available slot times on a date for a specific doctor
func availableTimeSlots(tenantID uint, date string, doctorName string) ([]string, error) {
	if strings.TrimSpace(doctorName) == "" {
		return nil, errors.New("please specify which doctor you would like to see to check availability")
	}

	doc, err := resolveDoctor(tenantID, doctorName)
	if err != nil {
		return nil, err
	}

	slots, err := generateDoctorSlots(doc, date)
	if err != nil {
		return nil, err
	}

	// Exclude cancelled appointments only (active appointments are taken)
	var booked []Appointment
	if err := db.Where("tenant_id = ? AND doctor_id = ? AND date = ? AND status <> ?",
		tenantID, doc.ID, date, StatusCancelled).Find(&booked).Error; err != nil {
		return nil, err
	}

	taken := make(map[string]bool)
	for _, ap := range booked {
		taken[ap.Time] = true
	}

	now := nowInAppLocation()
	todayStr := now.Format("2006-01-02")
	nowHHMM := now.Format("15:04")

	var available []string
	for _, s := range slots {
		if taken[s] {
			continue
		}
		// Exclude past times for today in Africa/Nairobi
		if date == todayStr && s <= nowHHMM {
			continue
		}
		available = append(available, s)
	}

	return available, nil
}

// bookSlot performs atomic validation and creation/update of an appointment slot within a transaction
func bookSlot(tx *gorm.DB, tenantID uint, app *Appointment) ([]string, error) {
	if app.TenantID == 0 {
		app.TenantID = tenantID
	}
	if app.TenantID != tenantID {
		return nil, errors.New("tenant mismatch on appointment")
	}

	// 1. Resolve doctor to a Doctor row of that tenant
	doc, err := resolveDoctor(tenantID, app.Doctor)
	if err != nil {
		return nil, err
	}
	app.DoctorID = &doc.ID
	app.Doctor = doc.Name // Standardise doctor name to DB record

	// 2. Reject past date/time (Africa/Nairobi)
	if !isValidDate(app.Date) {
		return nil, fmt.Errorf("invalid date format '%s', expected YYYY-MM-DD", app.Date)
	}
	if !isValidTime(app.Time) {
		return nil, fmt.Errorf("invalid time format '%s', expected HH:MM", app.Time)
	}

	loc := getAppLocation()
	slotDateTime, err := time.ParseInLocation("2006-01-02 15:04", app.Date+" "+app.Time, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid date/time: %w", err)
	}

	now := nowInAppLocation()
	if slotDateTime.Before(now) {
		return nil, errors.New("cannot schedule an appointment in the past")
	}

	// 3. Require a time on the slot grid within the doctor's working hours
	validSlots, err := generateDoctorSlots(doc, app.Date)
	if err != nil {
		return nil, err
	}
	if len(validSlots) == 0 {
		return nil, fmt.Errorf("%s is not scheduled to work on %s", doc.Name, app.Date)
	}

	onGrid := false
	for _, s := range validSlots {
		if s == app.Time {
			onGrid = true
			break
		}
	}
	if !onGrid {
		return nil, fmt.Errorf("time %s is outside %s's working hours (%s - %s) or not on a valid %d-minute slot",
			app.Time, doc.Name, doc.WorkStart, doc.WorkEnd, doc.SlotMinutes)
	}

	// 4. Check no active appointment exists for that tenant, doctor, date and time
	var count int64
	q := tx.Model(&Appointment{}).Where("tenant_id = ? AND doctor_id = ? AND date = ? AND time = ? AND status <> ?",
		tenantID, doc.ID, app.Date, app.Time, StatusCancelled)
	if app.ID > 0 {
		q = q.Where("id <> ?", app.ID)
	}
	if err := q.Count(&count).Error; err != nil {
		return nil, fmt.Errorf("database slot check error: %w", err)
	}

	if count > 0 {
		// Conflict: find up to 3 alternative slots
		alts, _ := availableTimeSlots(tenantID, app.Date, doc.Name)
		if len(alts) > 3 {
			alts = alts[:3]
		}
		return alts, &BookingConflictError{
			Message:      fmt.Sprintf("Slot at %s on %s with %s is already booked", app.Time, app.Date, doc.Name),
			Alternatives: alts,
		}
	}

	// 5. Insert or update
	if app.Status == "" {
		app.Status = StatusPending
	}

	if app.ID == 0 {
		if err := tx.Create(app).Error; err != nil {
			return nil, fmt.Errorf("failed to insert appointment: %w", err)
		}
	} else {
		if err := tx.Save(app).Error; err != nil {
			return nil, fmt.Errorf("failed to update appointment: %w", err)
		}
	}

	return nil, nil
}
