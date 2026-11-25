package main

import (
	"fmt"
	"log"
	"time"
)

// startAppointmentReminderWorker runs periodically in the background to dispatch upcoming WhatsApp appointment reminders
func startAppointmentReminderWorker() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			processPendingAppointmentReminders()
			<-ticker.C
		}
	}()
}

// processPendingAppointmentReminders checks upcoming appointments and dispatches 24h and 3h WhatsApp reminders
func processPendingAppointmentReminders() {
	if db == nil {
		return
	}

	loc := getAppLocation()
	now := time.Now().In(loc)

	var appts []Appointment
	if err := db.Where("status IN (?, ?) AND chatwoot_conversation_id IS NOT NULL AND (reminder_24h_sent = ? OR reminder_3h_sent = ?)",
		StatusPending, StatusConfirmed, false, false).Find(&appts).Error; err != nil {
		return
	}

	for _, ap := range appts {
		appTime, err := time.ParseInLocation("2006-01-02 15:04", ap.Date+" "+ap.Time, loc)
		if err != nil {
			continue
		}

		diff := appTime.Sub(now)
		if diff <= 0 {
			continue // Already in the past
		}

		var tenant Tenant
		if err := db.First(&tenant, ap.TenantID).Error; err != nil || !tenant.ChatwootEnabled {
			continue
		}

		// 3-hour reminder window (between 0 and 3.5 hours before)
		if !ap.Reminder3hSent && diff <= 3*time.Hour+30*time.Minute {
			msg := fmt.Sprintf("⏰ Reminder from %s: Your appointment with %s is in about %d minutes/hours today at %s (%s). Please arrive 10 minutes early.",
				tenant.Name, ap.Doctor, int(diff.Hours()), ap.Time, ap.Date)
			if err := SendAppointmentNotificationWhatsApp(tenant, &ap, msg); err == nil {
				db.Model(&Appointment{}).Where("id = ?", ap.ID).Updates(map[string]interface{}{
					"reminder_3h_sent":  true,
					"reminder_24h_sent": true,
				})
				log.Printf("[REMINDER] Dispatched 3h WhatsApp reminder for appointment #%d", ap.ID)
			}
			continue
		}

		// 24-hour reminder window (between 20 and 24.5 hours before)
		if !ap.Reminder24hSent && diff <= 24*time.Hour+30*time.Minute && diff > 20*time.Hour {
			msg := fmt.Sprintf("📅 Appointment Reminder from %s: You have an upcoming visit with %s tomorrow on %s at %s. Reply here if you need to reschedule or cancel.",
				tenant.Name, ap.Doctor, ap.Date, ap.Time)
			if err := SendAppointmentNotificationWhatsApp(tenant, &ap, msg); err == nil {
				db.Model(&Appointment{}).Where("id = ?", ap.ID).Update("reminder_24h_sent", true)
				log.Printf("[REMINDER] Dispatched 24h WhatsApp reminder for appointment #%d", ap.ID)
			}
		}
	}
}
