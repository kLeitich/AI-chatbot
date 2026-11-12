package main

import (
	"fmt"
	"strings"
	"unicode"
)

// tenantDoctorNames returns all active doctor names for a given tenant
func tenantDoctorNames(tenantID uint) []string {
	var docs []Doctor
	if err := db.Where("tenant_id = ? AND active = ?", tenantID, true).Order("id ASC").Find(&docs).Error; err != nil {
		return nil
	}
	var names []string
	for _, d := range docs {
		n := strings.TrimSpace(d.Name)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

// cleanDocName strips common prefixes ("Dr.", "Doctor") and punctuation
func cleanDocName(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	prefixes := []string{"dr. ", "dr.", "dr ", "doctor "}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			lower = strings.TrimSpace(lower[len(p):])
			break
		}
	}
	return strings.Trim(lower, " .,!?:;")
}

// matchDoctor performs forgiving matching of user input against a list of tenant doctors.
// Handles "Kim", "dr kim", "Dr. Kim" -> "Dr. Kim", accepts a unique token match,
// and returns ("", false) if ambiguous or unknown.
func matchDoctor(input string, doctors []string) (string, bool) {
	trimmed := strings.Trim(strings.TrimSpace(input), " .,!?:;")
	if trimmed == "" || len(doctors) == 0 {
		return "", false
	}

	cleanInput := cleanDocName(trimmed)
	if cleanInput == "" {
		return "", false
	}

	// 1. Exact full-string match (case-insensitive)
	for _, doc := range doctors {
		if strings.EqualFold(strings.TrimSpace(doc), trimmed) {
			return doc, true
		}
	}

	// 2. Exact match on cleaned names (e.g. "kim" == "kim" for "Dr. Kim")
	var cleanMatches []string
	for _, doc := range doctors {
		if cleanDocName(doc) == cleanInput {
			cleanMatches = append(cleanMatches, doc)
		}
	}
	if len(cleanMatches) == 1 {
		return cleanMatches[0], true
	} else if len(cleanMatches) > 1 {
		return "", false // Ambiguous
	}

	// 3. Unique token match (e.g. single word matching a doctor's surname or first name)
	inputTokens := strings.Fields(cleanInput)
	var tokenMatches []string
	for _, doc := range doctors {
		docTokens := strings.Fields(cleanDocName(doc))
		matched := false
		for _, it := range inputTokens {
			for _, dt := range docTokens {
				if it == dt {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			tokenMatches = append(tokenMatches, doc)
		}
	}

	if len(tokenMatches) == 1 {
		return tokenMatches[0], true
	}

	return "", false
}

// nextSlotQuestion returns (slot, question) for the first missing field in order:
// name -> doctor -> date -> time -> reason
func nextSlotQuestion(draft Appointment, doctors []string) (string, string) {
	if strings.TrimSpace(draft.PatientName) == "" {
		return "name", "What is your name, please?"
	}

	if strings.TrimSpace(draft.Doctor) == "" {
		docList := strings.Join(doctors, ", ")
		if docList != "" {
			return "doctor", fmt.Sprintf("Which doctor would you like to see? Our available doctors are: %s.", docList)
		}
		return "doctor", "Which doctor would you like to see?"
	}

	if strings.TrimSpace(draft.Date) == "" || !isValidDate(draft.Date) {
		return "date", fmt.Sprintf("What date would you like to book with %s?", draft.Doctor)
	}

	if strings.TrimSpace(draft.Time) == "" || !isValidTime(draft.Time) {
		return "time", fmt.Sprintf("What time works best for you on %s?", draft.Date)
	}

	if strings.TrimSpace(draft.Reason) == "" {
		return "reason", fmt.Sprintf("What is the reason for your appointment with %s on %s at %s?", draft.Doctor, draft.Date, draft.Time)
	}

	return "", ""
}

// formatPatientName capitalizes each word in a patient's name
func formatPatientName(raw string) string {
	raw = strings.Trim(strings.TrimSpace(raw), " .,!?:;")
	words := strings.Fields(raw)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

// isValidPatientNameCandidate checks if a string could be a valid patient name
func isValidPatientNameCandidate(msg string) bool {
	trimmed := strings.Trim(strings.TrimSpace(msg), " .,!?:;")
	if trimmed == "" {
		return false
	}

	// Reject affirmative/negative words
	if isAffirmative(trimmed) || isNegative(trimmed) {
		return false
	}

	// Reject if contains digits or @
	for _, r := range trimmed {
		if unicode.IsDigit(r) || r == '@' {
			return false
		}
	}

	// Reject if starts with "dr" or "doctor"
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "dr.") || strings.HasPrefix(lower, "dr ") || strings.HasPrefix(lower, "doctor ") || lower == "dr" || lower == "doctor" {
		return false
	}

	// Reject if more than 4 words
	words := strings.Fields(trimmed)
	if len(words) > 4 {
		return false
	}

	return true
}

// applyExpectedAnswer checks if an expected slot can be fulfilled directly by the user's message
func applyExpectedAnswer(conv *ConversationState, msg string, doctors []string) (bool, string) {
	if conv.Expecting == "" {
		return false, ""
	}

	trimmed := strings.TrimSpace(msg)
	words := strings.Fields(trimmed)

	// If the user sends a long message (>4 words) and we are not expecting a reason,
	// let it fall through to full extraction so complex multi-slot answers are not misparsed.
	if len(words) > 4 && conv.Expecting != "reason" {
		return false, ""
	}

	switch conv.Expecting {
	case "name":
		if !isValidPatientNameCandidate(trimmed) {
			return true, "Please provide your name (without titles or numbers)."
		}
		conv.Draft.PatientName = formatPatientName(trimmed)
		conv.Expecting = ""
		nextSlot, nextQ := nextSlotQuestion(conv.Draft, doctors)
		conv.Expecting = nextSlot
		return true, nextQ

	case "doctor":
		matched, ok := matchDoctor(trimmed, doctors)
		if !ok {
			docList := strings.Join(doctors, ", ")
			return true, fmt.Sprintf("I couldn't find a doctor called %s. Our doctors are: %s.", trimmed, docList)
		}
		conv.Draft.Doctor = matched
		conv.Expecting = ""
		nextSlot, nextQ := nextSlotQuestion(conv.Draft, doctors)
		conv.Expecting = nextSlot
		return true, nextQ

	case "date":
		d := extractDate(trimmed)
		if d != "" && isValidDate(d) {
			conv.Draft.Date = d
			conv.Expecting = ""
			nextSlot, nextQ := nextSlotQuestion(conv.Draft, doctors)
			conv.Expecting = nextSlot
			return true, nextQ
		}
		return true, "Please provide a valid date (e.g. tomorrow, 3 nov, or YYYY-MM-DD)."

	case "time":
		t := extractTime(trimmed)
		if t != "" && isValidTime(t) {
			conv.Draft.Time = t
			conv.Expecting = ""
			nextSlot, nextQ := nextSlotQuestion(conv.Draft, doctors)
			conv.Expecting = nextSlot
			return true, nextQ
		}
		return true, "Please provide a valid time (e.g. 10:00, 2pm, or 14:30)."

	case "reason":
		if trimmed != "" {
			conv.Draft.Reason = trimmed
			conv.Expecting = ""
			nextSlot, nextQ := nextSlotQuestion(conv.Draft, doctors)
			conv.Expecting = nextSlot
			return true, nextQ
		}
		return true, "Please let us know the reason for your appointment."
	}

	return false, ""
}

