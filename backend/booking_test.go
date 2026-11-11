package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) uint {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_appointments.db")
	tenantID := initDatabase(dbPath, "development")
	return tenantID
}

func getFutureWeekday(day time.Weekday, minDaysAhead int) (string, time.Time) {
	now := nowInAppLocation()
	target := now.AddDate(0, 0, minDaysAhead)
	for target.Weekday() != day {
		target = target.AddDate(0, 0, 1)
	}
	return target.Format("2006-01-02"), target
}

func TestBookingDoubleBookingRejected(t *testing.T) {
	tenantID := setupTestDB(t)
	dateStr, _ := getFutureWeekday(time.Wednesday, 7)

	// 1. Initial booking should succeed
	app1 := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        "10:00",
		PatientName: "First Patient",
		Reason:      "Checkup",
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, app1)
		return err
	})
	if err != nil {
		t.Fatalf("First booking failed: %v", err)
	}

	// 2. Second booking for the exact same slot should be rejected with BookingConflictError
	app2 := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        "10:00",
		PatientName: "Second Patient",
		Reason:      "Consultation",
	}

	var conflictAlts []string
	err = db.Transaction(func(tx *gorm.DB) error {
		var bErr error
		conflictAlts, bErr = bookSlot(tx, tenantID, app2)
		return bErr
	})

	if err == nil {
		t.Fatalf("Expected double booking to fail, but it succeeded")
	}

	conflictErr, ok := err.(*BookingConflictError)
	if !ok {
		t.Fatalf("Expected BookingConflictError, got: %T (%v)", err, err)
	}

	if !strings.Contains(conflictErr.Message, "already booked") {
		t.Errorf("Expected conflict message to mention 'already booked', got: %s", conflictErr.Message)
	}

	// Conflict error must provide up to 3 alternative slots
	if len(conflictAlts) == 0 {
		t.Errorf("Expected alternative slots to be offered on conflict, got none")
	}
	if len(conflictAlts) > 3 {
		t.Errorf("Expected at most 3 alternative slots, got %d", len(conflictAlts))
	}
}

func TestBookingConcurrentGoroutinesOneSuccess(t *testing.T) {
	tenantID := setupTestDB(t)
	dateStr, _ := getFutureWeekday(time.Thursday, 7)
	slotTime := "11:00"

	numGoroutines := 20
	var successCount int32
	var conflictCount int32
	var wg sync.WaitGroup

	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		patientName := fmt.Sprintf("Concurrent Patient %d", i)

		go func(pName string) {
			defer wg.Done()
			<-startSignal // synchronize start

			app := &Appointment{
				TenantID:    tenantID,
				Doctor:      "Dr. Kim",
				Date:        dateStr,
				Time:        slotTime,
				PatientName: pName,
				Reason:      "Routine check",
			}

			err := db.Transaction(func(tx *gorm.DB) error {
				_, bErr := bookSlot(tx, tenantID, app)
				return bErr
			})

			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&conflictCount, 1)
			}
		}(patientName)
	}

	close(startSignal) // trigger all goroutines at once
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("Expected exactly 1 successful booking out of %d goroutines, got %d (conflicts: %d)",
			numGoroutines, successCount, conflictCount)
	}

	if conflictCount != int32(numGoroutines-1) {
		t.Fatalf("Expected %d conflicts, got %d", numGoroutines-1, conflictCount)
	}

	// Verify database record count is exactly 1
	var count int64
	db.Model(&Appointment{}).Where("tenant_id = ? AND date = ? AND time = ? AND status <> ?",
		tenantID, dateStr, slotTime, StatusCancelled).Count(&count)
	if count != 1 {
		t.Fatalf("Expected database count to be 1, got %d", count)
	}
}

func TestBookingCancelledSlotReopens(t *testing.T) {
	tenantID := setupTestDB(t)
	dateStr, _ := getFutureWeekday(time.Friday, 7)
	slotTime := "14:00"

	app := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        slotTime,
		PatientName: "Patient A",
		Reason:      "Checkup",
	}

	// 1. Initial booking
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, app)
		return err
	})
	if err != nil {
		t.Fatalf("Initial booking failed: %v", err)
	}

	// 2. Cancel the appointment
	if err := db.Model(app).Update("status", StatusCancelled).Error; err != nil {
		t.Fatalf("Failed to cancel appointment: %v", err)
	}

	// 3. New booking for the same slot should now succeed
	app2 := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        slotTime,
		PatientName: "Patient B",
		Reason:      "New Checkup",
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, app2)
		return err
	})
	if err != nil {
		t.Fatalf("Booking cancelled slot failed, expected slot to reopen: %v", err)
	}
}

func TestBookingUnknownDoctorRejected(t *testing.T) {
	tenantID := setupTestDB(t)
	dateStr, _ := getFutureWeekday(time.Monday, 7)

	app := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Nonexistent",
		Date:        dateStr,
		Time:        "10:00",
		PatientName: "Test Patient",
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, app)
		return err
	})

	if err == nil {
		t.Fatalf("Expected booking with unknown doctor to fail, but got nil")
	}

	if !strings.Contains(err.Error(), "not recognized") || !strings.Contains(err.Error(), "Dr. Kim") {
		t.Errorf("Expected error to list available doctors, got: %v", err)
	}
}

func TestBookingPastDateRejected(t *testing.T) {
	tenantID := setupTestDB(t)

	app := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        "2020-01-15",
		Time:        "10:00",
		PatientName: "Test Patient",
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, app)
		return err
	})

	if err == nil {
		t.Fatalf("Expected booking in past date to be rejected, but it succeeded")
	}

	if !strings.Contains(err.Error(), "past") {
		t.Errorf("Expected error to mention past, got: %v", err)
	}
}

func TestBookingOutsideWorkingHoursAndDaysRejected(t *testing.T) {
	tenantID := setupTestDB(t)

	// 1. Outside hours on a working day (e.g. 21:00, Dr. Kim works 09:00 - 17:00)
	dateStr, _ := getFutureWeekday(time.Monday, 7)
	appOutHours := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        "21:00",
		PatientName: "Test Patient",
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, appOutHours)
		return err
	})
	if err == nil {
		t.Fatalf("Expected booking outside working hours (21:00) to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "working hours") {
		t.Errorf("Expected error mentioning working hours, got: %v", err)
	}

	// 2. Off-grid time (e.g. 10:15 when slotMinutes is 30)
	appOffGrid := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        "10:15",
		PatientName: "Test Patient",
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, appOffGrid)
		return err
	})
	if err == nil {
		t.Fatalf("Expected booking on off-grid time (10:15) to fail, but it succeeded")
	}

	// 3. Sunday booking (Dr. Kim works Mon-Sat)
	sundayStr, _ := getFutureWeekday(time.Sunday, 7)
	appSunday := &Appointment{
		TenantID:    tenantID,
		Doctor:      "Dr. Kim",
		Date:        sundayStr,
		Time:        "10:00",
		PatientName: "Test Patient",
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenantID, appSunday)
		return err
	})
	if err == nil {
		t.Fatalf("Expected Sunday booking for Dr. Kim to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "not scheduled to work") {
		t.Errorf("Expected error stating doctor is not scheduled to work on Sunday, got: %v", err)
	}
}

func TestBookingCrossTenantIsolation(t *testing.T) {
	tenant1ID := setupTestDB(t)

	// Create Tenant 2
	tenant2 := Tenant{
		Name: "Apex Medical Clinic",
		Slug: "apex-clinic",
	}
	if err := db.Create(&tenant2).Error; err != nil {
		t.Fatalf("Failed to create tenant 2: %v", err)
	}

	// Create Dr. Kim in Tenant 2
	doc2 := Doctor{
		TenantID:    tenant2.ID,
		Name:        "Dr. Kim",
		Specialty:   "General",
		WorkDays:    "Mon,Tue,Wed,Thu,Fri,Sat",
		WorkStart:   "09:00",
		WorkEnd:     "17:00",
		SlotMinutes: 30,
	}
	if err := db.Create(&doc2).Error; err != nil {
		t.Fatalf("Failed to create doctor in tenant 2: %v", err)
	}

	dateStr, _ := getFutureWeekday(time.Tuesday, 7)
	slotTime := "15:00"

	// Book slot in Tenant 1
	app1 := &Appointment{
		TenantID:    tenant1ID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        slotTime,
		PatientName: "Tenant 1 Patient",
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenant1ID, app1)
		return err
	})
	if err != nil {
		t.Fatalf("Tenant 1 booking failed: %v", err)
	}

	// Booking the SAME slot (date, time, doctor name) in Tenant 2 MUST SUCCEED
	app2 := &Appointment{
		TenantID:    tenant2.ID,
		Doctor:      "Dr. Kim",
		Date:        dateStr,
		Time:        slotTime,
		PatientName: "Tenant 2 Patient",
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := bookSlot(tx, tenant2.ID, app2)
		return err
	})
	if err != nil {
		t.Fatalf("Cross-tenant isolation failure: Tenant 2 booking was blocked by Tenant 1: %v", err)
	}
}

