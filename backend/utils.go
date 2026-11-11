package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	dateRegex     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex     = regexp.MustCompile(`^\d{2}:\d{2}$`)
	monthNameRe   = regexp.MustCompile(`(?i)\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b`)
	ordinalDayRe  = regexp.MustCompile(`(?i)\b(\d{1,2})(st|nd|rd|th)?\b`)
	time24Re      = regexp.MustCompile(`\b([01]?\d|2[0-3]):([0-5]\d)\b`)
	timeAmPmRe    = regexp.MustCompile(`(?i)\b(?:at\s*)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)\b`)
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

func isAffirmative(message string) bool {
	msg := strings.TrimSpace(strings.ToLower(message))
	return regexp.MustCompile(`(?i)^(yes|yeah|yep|sure|confirm|correct|that sounds good|go ahead|please do|ok|okay)$`).MatchString(msg)
}

func isNegative(message string) bool {
	msg := strings.TrimSpace(strings.ToLower(message))
	return regexp.MustCompile(`(?i)^(no|nah|nope|not yet|change|wrong|don'?t|do not|dont)$`).MatchString(msg)
}

// conversation memory store with safe concurrent access and TTL eviction
const sessionTTL = 2 * time.Hour

var sessionIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,64}$`)

func isValidSessionID(sid string) bool {
	return sessionIDRegex.MatchString(sid)
}

func generateSessionID() string {
	b := make([]byte, 16) // 128-bit
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().Unix())
	}
	return hex.EncodeToString(b)
}

var (
	convMuRW sync.RWMutex
	sessionM = map[string]ConversationState{}

	sessionLocksMu sync.Mutex
	sessionLocks   = map[string]*sessionLockEntry{}
)

type sessionLockEntry struct {
	mu       sync.Mutex
	refCount int
}

// acquireSessionLock serialises handling per convKey so read-modify-write cannot race
func acquireSessionLock(convKey string) func() {
	sessionLocksMu.Lock()
	entry, exists := sessionLocks[convKey]
	if !exists {
		entry = &sessionLockEntry{}
		sessionLocks[convKey] = entry
	}
	entry.refCount++
	sessionLocksMu.Unlock()

	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()
		sessionLocksMu.Lock()
		entry.refCount--
		if entry.refCount <= 0 {
			delete(sessionLocks, convKey)
		}
		sessionLocksMu.Unlock()
	}
}

func init() {
	// Background worker to evict expired sessions and prevent memory leaks
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			cleanupExpiredSessions()
		}
	}()
}

func cleanupExpiredSessions() {
	convMuRW.Lock()
	defer convMuRW.Unlock()
	cutoff := time.Now().Add(-sessionTTL)
	for id, s := range sessionM {
		if !s.UpdatedAt.IsZero() && s.UpdatedAt.Before(cutoff) {
			delete(sessionM, id)
		}
	}
}

func getConversation(sessionID string) ConversationState {
	if sessionID == "" {
		return ConversationState{}
	}
	convMuRW.RLock()
	s, ok := sessionM[sessionID]
	convMuRW.RUnlock()

	if !ok {
		return ConversationState{}
	}
	// Expire if older than sessionTTL
	if !s.UpdatedAt.IsZero() && time.Since(s.UpdatedAt) > sessionTTL {
		convMuRW.Lock()
		delete(sessionM, sessionID)
		convMuRW.Unlock()
		return ConversationState{}
	}
	return s
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

// extractTime extracts a valid 24-hour HH:MM time from free text
func extractTime(message string) string {
	msg := strings.ToLower(message)
	candidates := []string{}

	if matches := time24Re.FindAllStringSubmatch(msg, -1); len(matches) > 0 {
		for _, m := range matches {
			hour, _ := strconv.Atoi(m[1])
			min, _ := strconv.Atoi(m[2])
			if hour >= 0 && hour <= 23 && min >= 0 && min < 60 {
				candidates = append(candidates, formatTwo(hour)+":"+formatTwo(min))
			}
		}
	}

	if matches := timeAmPmRe.FindAllStringSubmatch(msg, -1); len(matches) > 0 {
		for _, m := range matches {
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
			if hour >= 0 && hour <= 23 && min >= 0 && min < 60 {
				candidates = append(candidates, formatTwo(hour)+":"+formatTwo(min))
			}
		}
	}

	if len(candidates) > 0 {
		return normalizeTime(candidates[len(candidates)-1])
	}
	return ""
}

// extractDate extracts a YYYY-MM-DD date from free text
func extractDate(message string) string {
	msg := strings.ToLower(message)
	now := nowInAppLocation()

	if d := dateRegex.FindString(strings.TrimSpace(message)); d != "" && isValidDate(d) {
		return d
	}

	if tomorrowRe.MatchString(msg) {
		return now.Add(24 * time.Hour).Format("2006-01-02")
	} else if todayRe.MatchString(msg) {
		return now.Format("2006-01-02")
	} else if monthNameRe.MatchString(msg) && ordinalDayRe.MatchString(msg) {
		// Extract day and month
		dayMatch := ordinalDayRe.FindStringSubmatch(msg)
		monthMatch := monthNameRe.FindStringSubmatch(msg)
		day, _ := strconv.Atoi(dayMatch[1])
		monName := strings.ToLower(monthMatch[1])
		mon := monthNameToNumber(monName)
		year := now.Year()
		loc := getAppLocation()
		// If month has passed this year and no explicit year, assume next year
		cand := time.Date(year, time.Month(mon), day, 0, 0, 0, 0, loc)
		if cand.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)) {
			year++
			cand = time.Date(year, time.Month(mon), day, 0, 0, 0, 0, loc)
		}
		return cand.Format("2006-01-02")
	}
	return ""
}

// tryLocalParse extracts simple date/time/doctor from free text.
// Handles examples like:
// - "book me for 11am on 30th october with doctor Mercy"
// - "tomorrow at 14:30 with doctor Kim"
// Returns (draft appointment, true) if confident.
func tryLocalParse(message string) (Appointment, bool) {
	hhmm := extractTime(message)
	dateStr := extractDate(message)

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

	// Reason extraction (from patterns like "for Dentist", "for checkup", or standalone medical terms)
	msg := strings.ToLower(message)
	reason := ""
	reasonPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:for|because of|reason is|need)\s+([a-zA-Z]+(?:\s+[a-zA-Z]+)?)\b`),
		regexp.MustCompile(`(?i)\b(dentist|dental|checkup|consultation|examination|exam|headache|pain|injury|surgery|treatment|therapy|routine|annual|physical|screening)\b`),
	}
	for _, pattern := range reasonPatterns {
		if m := pattern.FindStringSubmatch(msg); len(m) > 1 {
			reason = strings.TrimSpace(m[1])
			break
		}
	}

	ap := Appointment{PatientName: patient, Doctor: doctor, Date: dateStr, Time: hhmm, Reason: reason, Status: "pending"}
	if ap.Date != "" && isValidDate(ap.Date) && ap.Time != "" && isValidTime(ap.Time) {
		return ap, true
	}
	return Appointment{}, false
}

func monthNameToNumber(m string) int {
	switch m[:3] {
	case "jan":
		return 1
	case "feb":
		return 2
	case "mar":
		return 3
	case "apr":
		return 4
	case "may":
		return 5
	case "jun":
		return 6
	case "jul":
		return 7
	case "aug":
		return 8
	case "sep":
		return 9
	case "oct":
		return 10
	case "nov":
		return 11
	case "dec":
		return 12
	}
	return int(nowInAppLocation().Month())
}

func formatTwo(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// normalizeTime converts various time formats to HH:MM (24-hour)
// Handles: "4pm" -> "16:00", "11am" -> "11:00", "2:30pm" -> "14:30", "14:30" -> "14:30"
func normalizeTime(timeStr string) string {
	if timeStr == "" {
		return ""
	}

	timeStr = strings.TrimSpace(strings.ToLower(timeStr))

	// Already in HH:MM format?
	if timeRegex.MatchString(timeStr) {
		return timeStr
	}

	// Try to extract time from patterns like "4pm", "2:30pm", "11am"
	// Pattern 1: Simple "4pm" or "11am"
	simpleRe := regexp.MustCompile(`^(\d{1,2})\s*(am|pm)$`)
	if m := simpleRe.FindStringSubmatch(timeStr); len(m) == 3 {
		hour, _ := strconv.Atoi(m[1])
		ampm := m[2]
		if ampm == "pm" && hour < 12 {
			hour += 12
		}
		if ampm == "am" && hour == 12 {
			hour = 0
		}
		if hour >= 0 && hour <= 23 {
			return formatTwo(hour) + ":00"
		}
	}

	// Pattern 2: "2:30pm" or "11:45am"
	withMinRe := regexp.MustCompile(`^(\d{1,2}):(\d{2})\s*(am|pm)$`)
	if m := withMinRe.FindStringSubmatch(timeStr); len(m) == 4 {
		hour, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		ampm := m[3]
		if ampm == "pm" && hour < 12 {
			hour += 12
		}
		if ampm == "am" && hour == 12 {
			hour = 0
		}
		if hour >= 0 && hour <= 23 && min >= 0 && min < 60 {
			return formatTwo(hour) + ":" + formatTwo(min)
		}
	}

	// If no match, return original (might already be in correct format)
	return timeStr
}
