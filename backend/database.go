package main

import (
	"log"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func initDatabase(dbPath string) {
	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(&User{}, &Appointment{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	seedSampleData()
}

func seedSampleData() {
	var count int64
	db.Model(&Appointment{}).Count(&count)
	if count > 0 {
		return
	}
	now := time.Now()
	samples := []Appointment{
		{PatientName: "John Doe", Doctor: "Dr. Kim", Date: now.Format("2006-01-02"), Time: "10:00", Reason: "checkup", Status: "confirmed"},
		{PatientName: "Jane Smith", Doctor: "Dr. Mercy", Date: now.AddDate(0, 0, 1).Format("2006-01-02"), Time: "11:00", Reason: "consultation", Status: "pending"},
		{PatientName: "Alex Johnson", Doctor: "Dr. Lee", Date: now.AddDate(0, 0, 2).Format("2006-01-02"), Time: "15:30", Reason: "follow-up", Status: "pending"},
	}
	for _, ap := range samples {
		_ = db.Create(&ap).Error
	}
}
