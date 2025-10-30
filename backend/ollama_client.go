package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// SessionInfo tracks extracted appointment details per session
type SessionInfo struct {
	Doctor  string
	Date    string
	Time    string
	Patient string
	Reason  string
}

// In-memory session-based conversation history
var conversationMemory = make(map[string][]string)

// Track extracted info per session
var sessionInfo = make(map[string]map[string]string)

//
// -------------------- Query Ollama --------------------
//
func QueryOllama(model, prompt string) (string, error) {
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "phi3"
		}
	}
	fmt.Printf("\n[DEBUG] [OLLAMA REQUEST] model=%s\nPrompt:\n%s\n", model, prompt)

	body, _ := json.Marshal(map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": true,
	})
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("[DEBUG] Ollama API call failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()
	
	fmt.Printf("[DEBUG] Ollama HTTP Status: %d\n", resp.StatusCode)
	if resp.StatusCode != 200 {
		fmt.Printf("[DEBUG] Non-200 response from Ollama!\n")
	}

	var output strings.Builder
	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var chunk map[string]interface{}
		if err := dec.Decode(&chunk); err == nil {
			if txt, ok := chunk["response"].(string); ok {
				output.WriteString(txt)
			}
		}
	}
	result := strings.TrimSpace(output.String())
	fmt.Printf("[DEBUG] Ollama raw output:\n%s\n", result)
	return result, nil
}

//
// -------------------- Extract Info from Message --------------------
//
func extractInfoFromMessage(message string, info map[string]string) {
	msg := strings.ToLower(message)
	
	// Extract doctor name (Dr. + name)
	if strings.Contains(msg, "dr.") || strings.Contains(msg, "dr ") || strings.Contains(msg, "doctor") {
		re := regexp.MustCompile(`(?i)dr\.?\s+(\w+)`)
		if matches := re.FindStringSubmatch(message); len(matches) > 1 {
			info["doctor"] = "Dr. " + strings.Title(matches[1])
			fmt.Printf("[DEBUG] Extracted doctor: %s\n", info["doctor"])
		}
	}
	
	// Extract date patterns (31 oct, oct 31, 2025-10-31)
	monthMap := map[string]string{
		"jan": "01", "feb": "02", "mar": "03", "apr": "04",
		"may": "05", "jun": "06", "jul": "07", "aug": "08",
		"sep": "09", "oct": "10", "nov": "11", "dec": "12",
	}
	
	// Pattern: "31 oct" or "oct 31"
	dateRe := regexp.MustCompile(`(?i)(\d{1,2})\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)|(?i)(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+(\d{1,2})`)
	if matches := dateRe.FindStringSubmatch(msg); len(matches) > 0 {
		var day, month string
		if matches[1] != "" {
			day = matches[1]
			month = monthMap[strings.ToLower(matches[2])]
		} else {
			month = monthMap[strings.ToLower(matches[3])]
			day = matches[4]
		}
		if len(day) == 1 {
			day = "0" + day
		}
		year := time.Now().Year()
		info["date"] = fmt.Sprintf("%d-%s-%s", year, month, day)
		fmt.Printf("[DEBUG] Extracted date: %s\n", info["date"])
	}
	
	// Extract time (2pm, 14:00, 2:30pm)
	timeRe := regexp.MustCompile(`(?i)(\d{1,2}):?(\d{2})?\s*(am|pm)?`)
	if matches := timeRe.FindStringSubmatch(msg); len(matches) > 1 {
		hour := matches[1]
		minute := "00"
		if matches[2] != "" {
			minute = matches[2]
		}
		
		// Convert to 24-hour format if am/pm specified
		if matches[3] != "" {
			h := 0
			fmt.Sscanf(hour, "%d", &h)
			if strings.ToLower(matches[3]) == "pm" && h < 12 {
				h += 12
			} else if strings.ToLower(matches[3]) == "am" && h == 12 {
				h = 0
			}
			hour = fmt.Sprintf("%02d", h)
		} else if len(hour) == 1 {
			hour = "0" + hour
		}
		
		info["time"] = fmt.Sprintf("%s:%s", hour, minute)
		fmt.Printf("[DEBUG] Extracted time: %s\n", info["time"])
	}
	
	// Extract patient name (look for capitalized names, but not if it's part of doctor)
	if !strings.Contains(msg, "dr.") && !strings.Contains(msg, "dr ") && !strings.Contains(msg, "doctor") {
		nameRe := regexp.MustCompile(`\b([A-Z][a-z]+)\s+([A-Z][a-z]+)\b`)
		if matches := nameRe.FindStringSubmatch(message); len(matches) > 2 {
			info["patient"] = matches[1] + " " + matches[2]
			fmt.Printf("[DEBUG] Extracted patient name: %s\n", info["patient"])
		}
	}
	
	// Extract reason - common medical reasons
	reasons := []string{"checkup", "check-up", "check up", "follow-up", "followup", 
		"consultation", "examination", "test", "review", "surgery", "procedure"}
	for _, reason := range reasons {
		if strings.Contains(msg, reason) {
			info["reason"] = reason
			fmt.Printf("[DEBUG] Extracted reason: %s\n", info["reason"])
			break
		}
	}
}

//
// -------------------- AskForAppointmentFromMessage --------------------
//
func AskForAppointmentFromMessage(model, userMessage, sessionID string) (Appointment, string, error) {
	systemPrompt := `
You are a helpful medical assistant.

IMPORTANT RULES:
- If the user wants to book an appointment AND provides all required details, ONLY respond with a pure JSON object in this schema:
  {"intent": "book", "doctor": "<Doctor Name>", "date": "YYYY-MM-DD", "time": "HH:MM", "patient_name": "<Name>", "reason": "<Reason>", "reply": "short friendly confirmation"}
- DO NOT say anything before or after the JSON. DO NOT use markdown. DO NOT wrap the JSON in code blocks. DO NOT explain or apologize. DO NOT say 'Here is your appointment:' or similar. ONLY the raw JSON.
- If ANY required field (doctor, date, time, patient name) is missing, DO NOT send JSON. Reply in *plain text* and ask for only the missing info politely and naturally.
- NEVER reply with both JSON and text; one or the other.
- Confirmation should sound human: e.g., "Got it, John! I've booked you with Dr. Mercy on October 31st at 11am."
`

	history := strings.Join(conversationMemory[sessionID], "\n")
	fullPrompt := fmt.Sprintf("%s\nConversation so far:\n%s\nUser: %s", systemPrompt, history, userMessage)

	raw, err := QueryOllama(model, fullPrompt)
	if err != nil {
		fmt.Printf("[DEBUG] Model unreachable or Query error: %v\n", err)
		return Appointment{}, "Sorry, I couldn’t reach the AI model.", err
	}
	raw = strings.TrimSpace(raw)
	conversationMemory[sessionID] = append(conversationMemory[sessionID], "User: "+userMessage, "AI: "+raw)

	// Extract JSON
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		jsonStr := raw[start : end+1]
		var tmp struct {
			Intent      string `json:"intent"`
			PatientName string `json:"patient_name"`
			Doctor      string `json:"doctor"`
			Date        string `json:"date"`
			Time        string `json:"time"`
			Reason      string `json:"reason"`
			Reply       string `json:"reply"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &tmp); err == nil && tmp.Intent == "book" {
			app := Appointment{
				PatientName: tmp.PatientName,
				Doctor:      tmp.Doctor,
				Date:        tmp.Date,
				Time:        tmp.Time,
				Reason:      tmp.Reason,
				Status:      "pending",
			}
			if app.PatientName == "" {
				fmt.Println("[DEBUG] Appointment missing patient name. Prompt user for name.")
				return Appointment{}, "Could I please have your full name to confirm the booking?", nil
			}
			fmt.Printf("[DEBUG] Extracted appointment: %+v\n", app)
			if err := SaveAppointment(app); err != nil {
				fmt.Println("[DB DEBUG] Save err:", err)
			}
			reply := tmp.Reply
			if reply == "" {
				reply = fmt.Sprintf("Got it, %s! I've booked you with %s on %s at %s.", app.PatientName, app.Doctor, app.Date, app.Time)
			}
			return app, reply, nil
		} else if err != nil {
			fmt.Printf("[DEBUG] JSON unmarshal error: %v\nraw: %s\n", err, jsonStr)
		}
	}
	// fallback if no JSON but valid text
	if len(strings.Fields(raw)) > 3 {
		fmt.Printf("[DEBUG] Ollama fallback (no JSON, text reply): %s\n", raw)
		return Appointment{}, raw, nil
	}
	// Fallback ask
	reply, err2 := AskConversationalReply(model, userMessage)
	if err2 != nil {
		fmt.Printf("[DEBUG] AskConversationalReply error: %v\n", err2)
		return Appointment{}, "I’m here to help! Could you please rephrase that?", err2
	}
	fmt.Printf("[DEBUG] Conversational fallback reply: %s\n", reply)
	return Appointment{}, reply, nil
}

//
// -------------------- Build Smart Response --------------------
//
func buildSmartResponse(info map[string]string, lastMessage string) string {
	// Acknowledge what we just received
	var acknowledgment string
	
	if info["doctor"] != "" && info["date"] != "" {
		acknowledgment = fmt.Sprintf("Great! %s on %s. ", info["doctor"], info["date"])
	} else if info["doctor"] != "" {
		acknowledgment = fmt.Sprintf("Got it, %s. ", info["doctor"])
	} else if info["date"] != "" {
		acknowledgment = fmt.Sprintf("Okay, %s. ", info["date"])
	}
	
	// Ask for next missing piece
	if info["doctor"] == "" {
		return acknowledgment + "Which doctor would you like to see?"
	}
	if info["date"] == "" {
		return acknowledgment + "What date works for you?"
	}
	if info["time"] == "" {
		return acknowledgment + "What time would you prefer?"
	}
	if info["patient"] == "" {
		return acknowledgment + "Could I get your full name for the booking?"
	}
	if info["reason"] == "" {
		return acknowledgment + "What's the reason for your visit?"
	}
	
	return "Could you provide more details about your appointment?"
}

//
// -------------------- Conversational fallback --------------------
//
func AskConversationalReply(model, message string) (string, error) {
	prompt := `You are a friendly assistant helping patients schedule doctor appointments.
Respond conversationally and naturally – use warm, human language.
If unclear, ask follow-up questions like:
"Which doctor would you like to see?" or "What date works best for you?"
Avoid formal or robotic tone. Be brief, kind, and engaging.
User: ` + message

	resp, err := QueryOllama(model, prompt)
	if err != nil {
		return "", err
	}

	reply := strings.TrimSpace(resp)
	if reply == "" {
		reply = "Sure! Could you tell me which doctor and date you'd like?"
	}
	return reply, nil
}

//
// -------------------- DB Save (stub) --------------------
//
func SaveAppointment(a Appointment) error {
	fmt.Printf("✅ [Saved Appointment] %+v\n", a)
	return nil
}