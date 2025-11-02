package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// QueryGroq sends a chat completion request to Groq API
func QueryGroq(model, prompt string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", errors.New("GROQ_API_KEY not set")
	}

	if model == "" {
		// Default to a current Groq model (configurable via GROQ_MODEL env var)
		model = os.Getenv("GROQ_MODEL")
		if model == "" {
			model = "llama-3.3-70b-versatile" // Updated to current model
		}
	}

	payload := map[string]interface{}{
		"model":                 model,
		"messages":              []map[string]string{{"role": "user", "content": prompt}},
		"temperature":           0.8,
		"max_completion_tokens": 512,
		"top_p":                 1,
		"stream":                false,
	}
	// Only add reasoning_effort for models that support it (like llama-3.2-11b-vision-preview)
	// For most models, this parameter is not needed

	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse Groq response: %w (response: %s)", err, string(data))
	}

	if len(result.Choices) == 0 {
		return "", errors.New("no response from Groq model")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// AskForAppointmentFromMessage processes natural input and extracts intent
func AskForAppointmentFromMessage(model, userMessage string, conv ConversationState) (Appointment, string, error) {
	systemPrompt := `
You are a friendly and conversational assistant that helps patients book doctor appointments.
Your goal is to collect: doctor, date, time, patient name, and reason for appointment.

IMPORTANT RULES:
1. Extract information from the current message AND combine with any previous context provided
2. If previous context has information (like doctor, date, time, patient name), USE IT - don't ignore it
3. Extract patient names from:
   - "Kevin Leitich, i want to see..." → extract "Kevin Leitich" 
   - "my name is X" or "I'm X"
   - Simple two capitalized words at start: "John Doe wants..."
4. Extract doctor names from patterns like "Dr. Kim", "doctor Kim", "with Dr. Smith", "i want to see Dr. Angela"
5. Extract dates from patterns like "4 nov", "november 4th", "tomorrow" - convert to YYYY-MM-DD format
6. Extract times from patterns like "11am", "2pm", "4pm", "14:30" - MUST convert to 24-hour HH:MM format:
   - "4pm" -> "16:00" (4 + 12 = 16)
   - "11am" -> "11:00"
   - "2:30pm" -> "14:30" (2 + 12 = 14)
   - "12pm" -> "12:00"
   - "12am" -> "00:00"
7. Extract reason from phrases like "for checkup", "because of headache", "I need a checkup"

CRITICAL: If you have doctor, date, time, and patient_name (from current message OR previous context), return JSON immediately.

If all required details (doctor, date, time, patient_name) are provided, return JSON:
{
  "intent": "book",
  "doctor": "Dr. Kim",
  "date": "2025-11-04",
  "time": "11:00",
  "patient_name": "John Doe",
  "reason": "checkup",
  "reply": "Perfect! I've booked your appointment with Dr. Kim on November 4th at 11:00 AM for checkup. Thank you!"
}

If any REQUIRED information (doctor, date, time, patient_name) is missing, DO NOT return JSON. 
ABSOLUTE RULE: If previous context has some fields, DO NOT ask for them again - only ask for what's missing.
Check the "You ALREADY have" section - NEVER ask for anything listed there.
`

	// Build context from conversation history
	contextStr := ""
	if conv.Draft.Doctor != "" {
		contextStr += fmt.Sprintf("You ALREADY have: Doctor = %s. ", conv.Draft.Doctor)
	}
	if conv.Draft.Date != "" {
		contextStr += fmt.Sprintf("You ALREADY have: Date = %s. ", conv.Draft.Date)
	}
	if conv.Draft.Time != "" {
		contextStr += fmt.Sprintf("You ALREADY have: Time = %s. ", conv.Draft.Time)
	}
	if conv.Draft.PatientName != "" {
		contextStr += fmt.Sprintf("You ALREADY have: Patient name = %s. ", conv.Draft.PatientName)
	}
	if conv.Draft.Reason != "" {
		contextStr += fmt.Sprintf("You ALREADY have: Reason = %s. ", conv.Draft.Reason)
	}
	if conv.LastUserMessage != "" {
		contextStr += fmt.Sprintf("Previous user message: %s. ", conv.LastUserMessage)
	}

	fullPrompt := systemPrompt
	if contextStr != "" {
		fullPrompt += "\n\n" + contextStr
	}
	fullPrompt += "\n\nCurrent user message: " + userMessage

	raw, err := QueryGroq(model, fullPrompt)
	if err != nil {
		return Appointment{}, "Sorry, I couldn’t reach the Groq API.", err
	}

	raw = strings.TrimSpace(raw)

	// Try to parse JSON if found
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end > start {
		jsonStr := raw[start : end+1]

		var tmp struct {
			Intent      string `json:"intent"`
			Doctor      string `json:"doctor"`
			Date        string `json:"date"`
			Time        string `json:"time"`
			PatientName string `json:"patient_name"`
			Reason      string `json:"reason"`
			Reply       string `json:"reply"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &tmp); err == nil && tmp.Intent == "book" {
			// Normalize time from AI response (might be "4pm" instead of "16:00")
			normalizedTime := normalizeTime(tmp.Time)
			if normalizedTime != "" {
				tmp.Time = normalizedTime
			}
			
			// Merge with conversation draft (use draft values if extracted ones are empty)
			app := Appointment{
				PatientName: choose(tmp.PatientName, conv.Draft.PatientName),
				Doctor:      choose(tmp.Doctor, conv.Draft.Doctor),
				Date:        choose(tmp.Date, conv.Draft.Date),
				Time:        choose(tmp.Time, conv.Draft.Time),
				Reason:      choose(tmp.Reason, conv.Draft.Reason),
				Status:      "pending",
			}
			
			// Also normalize the final time in case it came from conversation draft
			if app.Time != "" {
				app.Time = normalizeTime(app.Time)
			}

			if err := SaveAppointment(app); err != nil {
				fmt.Println("[DB Error]", err)
			}

			reply := tmp.Reply
			if reply == "" {
				if app.Reason != "" {
					reply = fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s for %s. Thank you!", 
						app.Doctor, app.Date, app.Time, app.Reason)
				} else {
					reply = fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s. Thank you!", 
						app.Doctor, app.Date, app.Time)
				}
			}
			return app, reply, nil
		}
	}

	// Try local parsing to extract partial information
	partialApp, hasPartial := tryLocalParse(userMessage)
	if hasPartial {
		// Merge with conversation draft
		app := Appointment{
			PatientName: choose(partialApp.PatientName, conv.Draft.PatientName),
			Doctor:      choose(partialApp.Doctor, conv.Draft.Doctor),
			Date:        choose(partialApp.Date, conv.Draft.Date),
			Time:        choose(partialApp.Time, conv.Draft.Time),
			Reason:      choose(partialApp.Reason, conv.Draft.Reason),
			Status:      "pending",
		}
		// Return partial appointment so handler can update conversation state
		reply, err := AskConversationalReply(model, userMessage, conv)
		if err != nil {
			return Appointment{}, "I'm here to help! Could you please rephrase that?", err
		}
		return app, reply, nil
	}

	// Extract reason from patterns like "for Dentist", "for checkup", etc.
	reasonRe := regexp.MustCompile(`(?i)\b(?:for|because of|reason is|need)\s+([a-zA-Z]+(?:\s+[a-zA-Z]+)?)\b`)
	var extractedReason string
	if matches := reasonRe.FindStringSubmatch(userMessage); len(matches) > 1 {
		extractedReason = strings.TrimSpace(matches[1])
	}

	// Extract patient name from patterns like "Kevin Leitich, i want to see..." or just "Kevin Leitich"
	var extractedName string
	// Pattern 1: Name at the start followed by comma (e.g., "Kevin Leitich, i want...")
	nameCommaRe := regexp.MustCompile(`^([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+)?)\s*[,]`)
	if matches := nameCommaRe.FindStringSubmatch(userMessage); len(matches) > 1 {
		extractedName = strings.TrimSpace(matches[1])
	} else {
		// Pattern 2: "my name is X"
		nameIsRe := regexp.MustCompile(`(?i)(?:my name is|i'm|i am|this is)\s+([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+)?)`)
		if matches := nameIsRe.FindStringSubmatch(userMessage); len(matches) > 1 {
			extractedName = strings.TrimSpace(matches[1])
		} else {
			// Pattern 3: Simple two-word name at start if context suggests it (when we have doctor but no name)
			if conv.Draft.Doctor != "" && conv.Draft.PatientName == "" {
				words := strings.Fields(userMessage)
				if len(words) >= 2 && len(words) <= 3 {
					// Check if first two words look like a name (both capitalized, no common words)
					firstWord := words[0]
					secondWord := words[1]
					if len(firstWord) > 1 && len(secondWord) > 1 &&
						strings.ToUpper(firstWord[:1]) == firstWord[:1] &&
						strings.ToUpper(secondWord[:1]) == secondWord[:1] {
						// Exclude common words
						excluded := []string{"Doctor", "Dr", "Want", "See", "Book", "Appointment"}
						isExcluded := false
						for _, ex := range excluded {
							if strings.EqualFold(firstWord, ex) || strings.EqualFold(secondWord, ex) {
								isExcluded = true
								break
							}
						}
						if !isExcluded {
							extractedName = firstWord + " " + secondWord
						}
					}
				}
			}
		}
	}

	// Even if local parsing fails, try to extract doctor name from common patterns
	// Pattern 1: "Dr. X" or "doctor X"
	doctorRe := regexp.MustCompile(`(?i)\b(?:dr\.?|doctor)\s+([a-zA-Z]+)\b`)
	// Pattern 2: "i want to see X" or "i would like to see X" (where X might be a doctor name)
	seeDoctorRe := regexp.MustCompile(`(?i)(?:want to see|would like to see|like to see|see|see dr\.?|see doctor)\s+([A-Z][a-zA-Z]+)`)
	
	var doctorName string
	if matches := doctorRe.FindStringSubmatch(userMessage); len(matches) > 1 {
		doctorName = strings.ToLower(matches[1])
		// Capitalize first letter
		if len(doctorName) > 0 {
			doctorName = strings.ToUpper(doctorName[:1]) + doctorName[1:]
		}
	} else if matches := seeDoctorRe.FindStringSubmatch(userMessage); len(matches) > 1 {
		// Extract from "see Wangechi" pattern
		doctorName = strings.TrimSpace(matches[1])
	}
	
	if doctorName != "" {
		partialApp := Appointment{
			Doctor:      "Dr. " + doctorName,
			Reason:      extractedReason,
			PatientName: extractedName,
		}
		// Merge with conversation draft - always return partial info so state gets updated
		app := Appointment{
			PatientName: choose(partialApp.PatientName, conv.Draft.PatientName),
			Doctor:      choose(partialApp.Doctor, conv.Draft.Doctor),
			Date:        conv.Draft.Date,
			Time:        conv.Draft.Time,
			Reason:      choose(partialApp.Reason, conv.Draft.Reason),
			Status:      "pending",
		}
		reply, err := AskConversationalReply(model, userMessage, conv)
		if err != nil {
			return Appointment{}, "I'm here to help! Could you please rephrase that?", err
		}
		return app, reply, nil
	}

	// If we extracted a reason or name but no doctor, still save them
	if extractedReason != "" || extractedName != "" {
		app := Appointment{
			PatientName: choose(extractedName, conv.Draft.PatientName),
			Doctor:      conv.Draft.Doctor,
			Date:        conv.Draft.Date,
			Time:        conv.Draft.Time,
			Reason:      choose(extractedReason, conv.Draft.Reason),
			Status:      "pending",
		}
		reply, err := AskConversationalReply(model, userMessage, conv)
		if err != nil {
			return Appointment{}, "I'm here to help! Could you please rephrase that?", err
		}
		return app, reply, nil
	}

	// Always try to extract any partial information, even if extraction fails
	// At minimum, return the conversation draft so state is preserved
	if conv.Draft.Doctor != "" || conv.Draft.Date != "" || conv.Draft.Time != "" || conv.Draft.PatientName != "" {
		app := Appointment{
			PatientName: conv.Draft.PatientName,
			Doctor:      conv.Draft.Doctor,
			Date:        conv.Draft.Date,
			Time:        conv.Draft.Time,
			Reason:      conv.Draft.Reason,
			Status:      "pending",
		}
		reply, err := AskConversationalReply(model, userMessage, conv)
		if err != nil {
			return Appointment{}, "I'm here to help! Could you please rephrase that?", err
		}
		return app, reply, nil
	}

	// Check if user is providing just a name (when we have doctor, date, time)
	if conv.Draft.Doctor != "" && conv.Draft.Date != "" && conv.Draft.Time != "" && conv.Draft.PatientName == "" {
		// We have doctor, date, time but no name - check if user is providing name
		nameRe := regexp.MustCompile(`(?i)(?:my name is|i'm|i am|this is)\s+([a-zA-Z]+(?:\s+[a-zA-Z]+)?)`)
		if matches := nameRe.FindStringSubmatch(userMessage); len(matches) > 1 {
			app := Appointment{
				PatientName: strings.TrimSpace(matches[1]),
				Doctor:      conv.Draft.Doctor,
				Date:        conv.Draft.Date,
				Time:        conv.Draft.Time,
				Reason:      conv.Draft.Reason,
				Status:      "pending",
			}
			// We have all required fields now (reason is optional or will be asked later)
			if err := SaveAppointment(app); err != nil {
				fmt.Println("[DB Error]", err)
			}
			reasonText := ""
			if app.Reason != "" {
				reasonText = fmt.Sprintf(" for %s", app.Reason)
			}
			return app, fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s%s. Thank you, %s!", 
				app.Doctor, app.Date, app.Time, reasonText, app.PatientName), nil
		}
		// Or if it's just a simple name (single word or two words)
		// Exclude common medical/reason words that might be confused as names
		excludedWords := map[string]bool{
			"dentist": true, "doctor": true, "checkup": true, "consultation": true,
			"appointment": true, "visit": true, "examination": true,
		}
		nameParts := strings.Fields(strings.TrimSpace(userMessage))
		if len(nameParts) <= 2 && len(nameParts) > 0 {
			firstWord := strings.ToLower(nameParts[0])
			if firstWord != "yes" && firstWord != "no" && !excludedWords[firstWord] {
				app := Appointment{
					PatientName: strings.Join(nameParts, " "),
					Doctor:      conv.Draft.Doctor,
					Date:        conv.Draft.Date,
					Time:        conv.Draft.Time,
					Reason:      conv.Draft.Reason,
					Status:      "pending",
				}
				if err := SaveAppointment(app); err != nil {
					fmt.Println("[DB Error]", err)
				}
				reasonText := ""
				if app.Reason != "" {
					reasonText = fmt.Sprintf(" for %s", app.Reason)
				}
				return app, fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s%s. Thank you, %s!", 
					app.Doctor, app.Date, app.Time, reasonText, app.PatientName), nil
			}
		}
	}

	// Check if we already have all required fields - if so, complete booking regardless
	if conv.Draft.Doctor != "" && conv.Draft.Date != "" && conv.Draft.Time != "" && conv.Draft.PatientName != "" {
		// We have all required fields! Use reason from draft or current message
		finalReason := choose(strings.TrimSpace(userMessage), conv.Draft.Reason)
		
		// If current message looks like a reason, use it
		msgLower := strings.ToLower(strings.TrimSpace(userMessage))
		reasonKeywords := []string{
			"dentist", "dental", "checkup", "consultation", "examination", "exam",
			"headache", "pain", "injury", "follow-up", "followup", "surgery",
			"treatment", "therapy", "routine", "annual", "physical", "screening",
		}
		isReasonKeyword := false
		for _, keyword := range reasonKeywords {
			if strings.Contains(msgLower, keyword) {
				isReasonKeyword = true
				break
			}
		}
		
		// If it's a short message (1-3 words), treat it as reason if it matches keywords
		words := strings.Fields(userMessage)
		if isReasonKeyword || (len(words) <= 3 && len(words) > 0 && finalReason == "") {
			finalReason = strings.TrimSpace(userMessage)
		}
		
		// If we still don't have reason, use default
		if finalReason == "" {
			finalReason = "general consultation"
		}
		
		app := Appointment{
			PatientName: conv.Draft.PatientName,
			Doctor:      conv.Draft.Doctor,
			Date:        conv.Draft.Date,
			Time:        conv.Draft.Time,
			Reason:      finalReason,
			Status:      "pending",
		}
		// We have everything now!
		if err := SaveAppointment(app); err != nil {
			fmt.Println("[DB Error]", err)
		}
		return app, fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s for %s. Thank you, %s!", 
			app.Doctor, app.Date, app.Time, app.Reason, app.PatientName), nil
	}

	// Fallback to conversational reply
	reply, err := AskConversationalReply(model, userMessage, conv)
	if err != nil {
		return Appointment{}, "I'm here to help! Could you please rephrase that?", err
	}

	return Appointment{}, reply, nil
}

// AskConversationalReply creates friendly follow-up messages
func AskConversationalReply(model, message string, conv ConversationState) (string, error) {
	// Build context about what we already know
	contextInfo := ""
	missingFields := []string{}
	
	if conv.Draft.Doctor != "" {
		contextInfo += fmt.Sprintf("You already have the doctor: %s. ", conv.Draft.Doctor)
	} else {
		missingFields = append(missingFields, "doctor")
	}
	if conv.Draft.Date != "" {
		contextInfo += fmt.Sprintf("You already have the date: %s. ", conv.Draft.Date)
	} else {
		missingFields = append(missingFields, "date")
	}
	if conv.Draft.Time != "" {
		contextInfo += fmt.Sprintf("You already have the time: %s. ", conv.Draft.Time)
	} else {
		missingFields = append(missingFields, "time")
	}
	if conv.Draft.PatientName != "" {
		contextInfo += fmt.Sprintf("You already have the patient name: %s. ", conv.Draft.PatientName)
	} else {
		missingFields = append(missingFields, "patient name")
	}
	if conv.Draft.Reason != "" {
		contextInfo += fmt.Sprintf("You already have the reason: %s. ", conv.Draft.Reason)
	} else {
		missingFields = append(missingFields, "reason")
	}

	missingStr := ""
	if len(missingFields) > 0 {
		missingStr = " You still need: " + strings.Join(missingFields, ", ") + "."
	}

	// Build what we have vs what we need
	hasFields := []string{}
	if conv.Draft.Doctor != "" {
		hasFields = append(hasFields, "doctor ("+conv.Draft.Doctor+")")
	}
	if conv.Draft.PatientName != "" {
		hasFields = append(hasFields, "patient name ("+conv.Draft.PatientName+")")
	}
	if conv.Draft.Date != "" {
		hasFields = append(hasFields, "date ("+conv.Draft.Date+")")
	}
	if conv.Draft.Time != "" {
		hasFields = append(hasFields, "time ("+conv.Draft.Time+")")
	}
	if conv.Draft.Reason != "" {
		hasFields = append(hasFields, "reason ("+conv.Draft.Reason+")")
	}

	hasStr := ""
	if len(hasFields) > 0 {
		hasStr = "You ALREADY HAVE: " + strings.Join(hasFields, ", ") + ". "
	}

	prompt := `
You are a warm, friendly assistant helping patients book appointments.

` + hasStr + missingStr + `

ABSOLUTE CRITICAL RULES - YOU MUST FOLLOW THESE:
1. NEVER EVER ask for information that is listed in "You ALREADY HAVE" above
2. ONLY ask for what is in the "still need" list - nothing else
3. If the "still need" list is empty, you have everything - confirm the booking or ask only for reason
4. DO NOT mention or reference information you already have in your questions
5. If you have doctor, date, time, and patient name - you MUST complete the booking (ask for reason if missing, but don't re-ask for other fields)

EXAMPLE:
- If you ALREADY HAVE: patient name (Kevin Leitich), doctor (Dr. Wangechi)
- And user says "3 nov"
- You should respond: "Great! What time works best for you?" (ONLY ask for time, NOT name or doctor)

Keep responses short and natural.

Current conversation state:
- Doctor: ` + func() string {
		if conv.Draft.Doctor != "" {
			return conv.Draft.Doctor
		}
		return "NOT PROVIDED YET"
	}() + `
- Patient Name: ` + func() string {
		if conv.Draft.PatientName != "" {
			return conv.Draft.PatientName
		}
		return "NOT PROVIDED YET"
	}() + `
- Date: ` + func() string {
		if conv.Draft.Date != "" {
			return conv.Draft.Date
		}
		return "NOT PROVIDED YET"
	}() + `
- Time: ` + func() string {
		if conv.Draft.Time != "" {
			return conv.Draft.Time
		}
		return "NOT PROVIDED YET"
	}() + `
- Reason: ` + func() string {
		if conv.Draft.Reason != "" {
			return conv.Draft.Reason
		}
		return "NOT PROVIDED YET"
	}() + `

User just said: ` + message

	resp, err := QueryGroq(model, prompt)
	if err != nil {
		return "", err
	}

	reply := strings.TrimSpace(resp)
	if reply == "" {
		reply = "Sure! Could you tell me which doctor and date you’d like?"
	}
	return reply, nil
}

// SaveAppointment simulates DB persistence
func SaveAppointment(a Appointment) error {
	fmt.Printf("✅ [Saved Appointment] %+v\n", a)
	return nil
}
