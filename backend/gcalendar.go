package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var gcalendarHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// GoogleOAuthTokenResponse represents token response from Google OAuth endpoint
type GoogleOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// GoogleCalendarEvent represents a Google Calendar event payload
type GoogleCalendarEvent struct {
	ID          string                  `json:"id,omitempty"`
	Summary     string                  `json:"summary"`
	Description string                  `json:"description,omitempty"`
	Start       GoogleCalendarEventTime `json:"start"`
	End         GoogleCalendarEventTime `json:"end"`
	Attendees   []GoogleCalendarAttendee `json:"attendees,omitempty"`
}

type GoogleCalendarEventTime struct {
	DateTime string `json:"dateTime"` // RFC3339 format, e.g. "2026-11-25T10:00:00+03:00"
	TimeZone string `json:"timeZone,omitempty"`
}

type GoogleCalendarAttendee struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
}

type GoogleFreeBusyRequest struct {
	TimeMin string                         `json:"timeMin"`
	TimeMax string                         `json:"timeMax"`
	Items   []GoogleFreeBusyRequestItem    `json:"items"`
}

type GoogleFreeBusyRequestItem struct {
	ID string `json:"id"`
}

type GoogleFreeBusyResponse struct {
	Calendars map[string]struct {
		Busy []struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"busy"`
	} `json:"calendars"`
}

// getGoogleAccessToken exchanges a refresh token for a fresh short-lived access token
func getGoogleAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	if refreshToken == "" {
		refreshToken = os.Getenv("GOOGLE_CALENDAR_REFRESH_TOKEN")
	}
	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}

	if refreshToken == "" {
		return "", fmt.Errorf("google calendar refresh token not configured")
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := gcalendarHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to contact Google OAuth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google oauth token error %d: %s", resp.StatusCode, string(body))
	}

	var tokResp GoogleOAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil {
		return "", err
	}

	return tokResp.AccessToken, nil
}

// resolveCalendarID returns the target calendar ID for an appointment
func resolveCalendarID(tenant Tenant, app *Appointment) string {
	// 1. Check doctor-specific Google Calendar ID
	if app.DoctorID != nil && *app.DoctorID > 0 {
		var doc Doctor
		if err := db.First(&doc, *app.DoctorID).Error; err == nil && doc.GoogleCalendarID != "" {
			return doc.GoogleCalendarID
		}
	}

	// 2. Fall back to clinic-wide calendar ID
	if tenant.GoogleCalendarID != "" {
		return tenant.GoogleCalendarID
	}
	if envCal := os.Getenv("GOOGLE_CALENDAR_ID"); envCal != "" {
		return envCal
	}
	return "primary"
}

// syncAppointmentToGoogleCalendar creates an event in Google Calendar when booked
func syncAppointmentToGoogleCalendar(tenant Tenant, app *Appointment) error {
	calID := resolveCalendarID(tenant, app)

	loc := nowInAppLocation().Location()
	slotDuration := 30 * time.Minute
	if app.DoctorID != nil && *app.DoctorID > 0 {
		var doc Doctor
		if err := db.First(&doc, *app.DoctorID).Error; err == nil && doc.SlotMinutes > 0 {
			slotDuration = time.Duration(doc.SlotMinutes) * time.Minute
		}
	}

	// Parse appointment start time
	startStr := fmt.Sprintf("%sT%s:00", app.Date, app.Time)
	startTime, err := time.ParseInLocation("2006-01-02T15:04:05", startStr, loc)
	if err != nil {
		log.Printf("[GCAL] Failed to parse appointment time: %v", err)
		return err
	}
	endTime := startTime.Add(slotDuration)

	event := GoogleCalendarEvent{
		Summary:     fmt.Sprintf("Medical Appointment: %s with %s", app.PatientName, app.Doctor),
		Description: fmt.Sprintf("Booka Healthcare Booking #%d\nDoctor: %s\nPatient: %s\nReason: %s\nPhone: %s",
			app.ID, app.Doctor, app.PatientName, app.Reason, app.PatientPhone),
		Start: GoogleCalendarEventTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: loc.String(),
		},
		End: GoogleCalendarEventTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: loc.String(),
		},
	}

	if app.PatientEmail != "" {
		event.Attendees = append(event.Attendees, GoogleCalendarAttendee{
			Email:       app.PatientEmail,
			DisplayName: app.PatientName,
		})
	}

	accessToken, err := getGoogleAccessToken(tenant.GoogleClientID, tenant.GoogleClientSecret, tenant.GoogleCalendarToken)
	if err != nil {
		log.Printf("[GCAL NOTICE] Google Calendar access token unavailable (%v). Event logged for sync: %+v", err, event)
		return nil
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", url.PathEscape(calID))
	payloadBytes, _ := json.Marshal(event)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := gcalendarHTTPClient.Do(req)
	if err != nil {
		log.Printf("[GCAL ERROR] Failed to create Google Calendar event: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[GCAL ERROR] API error %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("google calendar error: %s", string(body))
	}

	var createdEvent GoogleCalendarEvent
	if err := json.NewDecoder(resp.Body).Decode(&createdEvent); err == nil && createdEvent.ID != "" {
		app.GoogleEventID = createdEvent.ID
		db.Model(app).Update("google_event_id", createdEvent.ID)
		log.Printf("[GCAL] Successfully synced appointment #%d to Google Calendar ID '%s', event '%s'", app.ID, calID, createdEvent.ID)
	}

	return nil
}

// deleteGoogleCalendarEvent removes an appointment event from Google Calendar on cancellation
func deleteGoogleCalendarEvent(tenant Tenant, googleEventID string, calID string) error {
	if googleEventID == "" {
		return nil
	}
	if calID == "" {
		calID = tenant.GoogleCalendarID
	}
	if calID == "" {
		calID = "primary"
	}

	accessToken, err := getGoogleAccessToken(tenant.GoogleClientID, tenant.GoogleClientSecret, tenant.GoogleCalendarToken)
	if err != nil {
		return nil
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events/%s",
		url.PathEscape(calID), url.PathEscape(googleEventID))

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := gcalendarHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("[GCAL] Deleted Google Calendar event '%s' from calendar '%s'", googleEventID, calID)
	return nil
}

// fetchGoogleCalendarBusySlots queries Google Calendar FreeBusy API to find blocked time slots
func fetchGoogleCalendarBusySlots(tenantID uint, doctorName, dateStr string) ([]string, error) {
	var tenant Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil || !tenant.GoogleCalendarEnabled {
		return nil, nil // Google Calendar sync not enabled for this tenant
	}

	var doc Doctor
	calID := tenant.GoogleCalendarID
	if err := db.Where("tenant_id = ? AND LOWER(TRIM(name)) = LOWER(TRIM(?))", tenantID, doctorName).First(&doc).Error; err == nil {
		if doc.GoogleCalendarID != "" {
			calID = doc.GoogleCalendarID
		}
	}
	if calID == "" {
		calID = getEnv("GOOGLE_CALENDAR_ID", "primary")
	}

	accessToken, err := getGoogleAccessToken(tenant.GoogleClientID, tenant.GoogleClientSecret, tenant.GoogleCalendarToken)
	if err != nil {
		return nil, nil // Silently ignore if credentials not active
	}

	loc := nowInAppLocation().Location()
	dayStart, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return nil, err
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	fbReq := GoogleFreeBusyRequest{
		TimeMin: dayStart.Format(time.RFC3339),
		TimeMax: dayEnd.Format(time.RFC3339),
		Items:   []GoogleFreeBusyRequestItem{{ID: calID}},
	}
	reqBody, _ := json.Marshal(fbReq)

	req, err := http.NewRequest("POST", "https://www.googleapis.com/calendar/v3/freeBusy", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := gcalendarHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var fbResp GoogleFreeBusyResponse
	if err := json.NewDecoder(resp.Body).Decode(&fbResp); err != nil {
		return nil, err
	}

	var busySlots []string
	if calData, exists := fbResp.Calendars[calID]; exists {
		for _, b := range calData.Busy {
			bStart, sErr := time.Parse(time.RFC3339, b.Start)
			if sErr == nil {
				localTime := bStart.In(loc)
				busySlots = append(busySlots, localTime.Format("15:04"))
			}
		}
	}

	return busySlots, nil
}
