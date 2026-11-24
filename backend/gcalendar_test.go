package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGoogleCalendarEventSerialization(t *testing.T) {
	loc := appLocation()
	startTime, err := time.ParseInLocation("2006-01-02 15:04", "2026-12-01 14:00", loc)
	if err != nil {
		t.Fatalf("Failed to parse start time: %v", err)
	}
	endTime := startTime.Add(30 * time.Minute)

	event := GoogleCalendarEvent{
		Summary:     "Medical Appointment: Jane Doe with Dr. Kim",
		Description: "Patient Jane Doe booked for General Consultation",
		Start: GoogleCalendarEventTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: loc.String(),
		},
		End: GoogleCalendarEventTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: loc.String(),
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal Google Calendar event: %v", err)
	}

	var parsed GoogleCalendarEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal Google Calendar event: %v", err)
	}

	if parsed.Summary != event.Summary {
		t.Errorf("Expected summary %q, got %q", event.Summary, parsed.Summary)
	}
	if parsed.Start.DateTime != startTime.Format(time.RFC3339) {
		t.Errorf("Expected start dateTime %q, got %q", startTime.Format(time.RFC3339), parsed.Start.DateTime)
	}
}

func TestGoogleFreeBusyResponseParsing(t *testing.T) {
	rawJSON := `{
		"calendars": {
			"primary": {
				"busy": [
					{
						"start": "2026-12-01T09:00:00+03:00",
						"end": "2026-12-01T09:30:00+03:00"
					},
					{
						"start": "2026-12-01T14:30:00+03:00",
						"end": "2026-12-01T15:00:00+03:00"
					}
				]
			}
		}
	}`

	var resp GoogleFreeBusyResponse
	if err := json.Unmarshal([]byte(rawJSON), &resp); err != nil {
		t.Fatalf("Failed to parse freeBusy response: %v", err)
	}

	cal, exists := resp.Calendars["primary"]
	if !exists {
		t.Fatalf("Expected primary calendar in freeBusy response")
	}

	if len(cal.Busy) != 2 {
		t.Fatalf("Expected 2 busy slots, got %d", len(cal.Busy))
	}

	t1, _ := time.Parse(time.RFC3339, cal.Busy[0].Start)
	t2, _ := time.Parse(time.RFC3339, cal.Busy[1].Start)

	loc := appLocation()
	slot1 := t1.In(loc).Format("15:04")
	slot2 := t2.In(loc).Format("15:04")

	if slot1 != "09:00" || slot2 != "14:30" {
		t.Errorf("Expected busy slots 09:00 and 14:30, got %s and %s", slot1, slot2)
	}
}
