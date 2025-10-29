package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	dateRegex     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex     = regexp.MustCompile(`^\d{2}:\d{2}$`)
	monthNameRe   = regexp.MustCompile(`(?i)\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b`)
	ordinalDayRe  = regexp.MustCompile(`(?i)\b(\d{1,2})(st|nd|rd|th)?\b`)
	timePhraseRe  = regexp.MustCompile(`(?i)\b(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\b`)
	tomorrowRe    = regexp.MustCompile(`(?i)\btomorrow\b`)
	todayRe       = regexp.MustCompile(`(?i)\btoday\b`)
	doctorNameRe  = regexp.MustCompile(`(?i)\bdoctor\s+([a-zA-Z]+)\b`)
	patientNameRe = regexp.MustCompile(`(?i)\bmy\s+name\s+is\s+([a-zA-Z]+(?:\s+[a-zA-Z]+)?)\b`)
)

func isValidDate(date string) bool {
	if !dateRegex.MatchString(date) {
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func isValidTime(t string) bool {
	if !timeRegex.MatchString(t) {
		return false
	}
	_, err := time.Parse("15:04", t)
	return err == nil
}

// conversation memory store
var (
	convMuRW syncWrapper
	sessionM = map[string]ConversationState{}
)

type syncWrapper struct{}

func (syncWrapper) Lock()   {}
func (syncWrapper) Unlock() {}
func (syncWrapper) RLock()  {}
func (syncWrapper) RUnlock() {}

func getConversation(sessionID string) ConversationState {
	if sessionID == "" {
		return ConversationState{}
	}
	convMuRW.RLock()
	defer convMuRW.RUnlock()
	return sessionM[sessionID]
}

func setConversation(sessionID string, s ConversationState) {
	if sessionID == "" {
		return
	}
	convMuRW.Lock()
	defer convMuRW.Unlock()
	s.UpdatedAt = time.Now()
	sessionM[sessionID] = s
}

// tryLocalParse extracts simple date/time/doctor from free text.
// Handles examples like:
// - "book me for 11am on 30th october with doctor Mercy"
// - "tomorrow at 14:30 with doctor Kim"
// Returns (draft appointment, true) if confident.
func tryLocalParse(message string) (Appointment, bool) {
	msg := strings.ToLower(message)

	// Time
	hhmm := ""
	if m := timePhraseRe.FindStringSubmatch(msg); len(m) > 0 {
		hour, _ := strconv.Atoi(m[1])
		min := 0
		if m[2] != "" {
			min, _ = strconv.Atoi(m[2])
		}
		ampm := strings.ToLower(m[3])
		if ampm == "pm" && hour < 12 {
			hour += 12
		}
		if ampm == "am" && hour == 12 {
			hour = 0
		}
		if hour >= 0 && hour <= 23 {
			hhmm = formatTwo(hour) + ":" + formatTwo(min)
		}
	}

	// Date
	var dateStr string
	now := time.Now()
	if tomorrowRe.MatchString(msg) {
		dateStr = now.Add(24 * time.Hour).Format("2006-01-02")
	} else if todayRe.MatchString(msg) {
		dateStr = now.Format("2006-01-02")
	} else if monthNameRe.MatchString(msg) && ordinalDayRe.MatchString(msg) {
		// Extract day and month
		dayMatch := ordinalDayRe.FindStringSubmatch(msg)
		monthMatch := monthNameRe.FindStringSubmatch(msg)
		day, _ := strconv.Atoi(dayMatch[1])
		monName := strings.ToLower(monthMatch[1])
		mon := monthNameToNumber(monName)
		year := now.Year()
		// If month has passed this year and no explicit year, assume next year
		cand := time.Date(year, time.Month(mon), day, 0, 0, 0, 0, time.Local)
		if cand.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)) {
			year++
			cand = time.Date(year, time.Month(mon), day, 0, 0, 0, 0, time.Local)
		}
		dateStr = cand.Format("2006-01-02")
	}

	// Doctor
	doctor := ""
	if m := doctorNameRe.FindStringSubmatch(message); len(m) > 0 {
		doctor = strings.TrimSpace("Dr. " + m[1])
	}

	// Patient (optional)
	patient := ""
	if m := patientNameRe.FindStringSubmatch(message); len(m) > 0 {
		patient = strings.TrimSpace(m[1])
	}

	ap := Appointment{PatientName: patient, Doctor: doctor, Date: dateStr, Time: hhmm, Status: "pending"}
	if ap.Date != "" && isValidDate(ap.Date) && ap.Time != "" && isValidTime(ap.Time) {
		return ap, true
	}
	return Appointment{}, false
}

func monthNameToNumber(m string) int {
	switch m[:3] {
	case "jan": return 1
	case "feb": return 2
	case "mar": return 3
	case "apr": return 4
	case "may": return 5
	case "jun": return 6
	case "jul": return 7
	case "aug": return 8
	case "sep": return 9
	case "oct": return 10
	case "nov": return 11
	case "dec": return 12
	}
	return int(time.Now().Month())
}

func formatTwo(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
