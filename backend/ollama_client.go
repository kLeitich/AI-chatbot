package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ollamaExtraction struct {
	Intent      string `json:"intent"`
	PatientName string `json:"patient_name"`
	Doctor      string `json:"doctor"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Reason      string `json:"reason"`
}

// QueryOllama runs the model with the given prompt and returns raw stdout text.
func QueryOllama(model, prompt string) (string, error) {
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
		if model == "" {
			model = "phi3"
		}
	}
	cmd := exec.Command("ollama", "run", model)
	cmd.Stdin = bytes.NewBufferString(prompt)
	cmd.Env = os.Environ()
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	cmdErr := make(chan error, 1)
	go func() { cmdErr <- cmd.Run() }()
	select {
	case err := <-cmdErr:
		if err != nil {
			return "", errors.New(strings.TrimSpace(stderr.String()))
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return "", errors.New("ollama timed out")
	}
	return out.String(), nil
}

// AskForAppointmentFromMessage tries JSON extraction first.
// Returns (Appointment, friendlyReply, error). If an Appointment is complete/valid, friendlyReply can be empty.
func AskForAppointmentFromMessage(model, userMessage string, prev ConversationState) (Appointment, string, error) {
	// System prompt: personable, JSON extraction if possible
	var b strings.Builder
	b.WriteString("You are a friendly medical assistant that helps people book doctor appointments.\n")
	b.WriteString("You respond politely, naturally, and conversationally.\n")
	b.WriteString("If the user wants to book an appointment, extract structured info in JSON format:\n")
	b.WriteString("{\"intent\": \"book\", \"patient_name\": \"...\", \"doctor\": \"...\", \"date\": \"YYYY-MM-DD\", \"time\": \"HH:MM\", \"reason\": \"...\"}\n")
	b.WriteString("If you cannot extract this info confidently, still output a JSON object with {\"intent\": \"chat\"}.\n")
	if prev.Draft.PatientName != "" || prev.Draft.Doctor != "" || prev.Draft.Date != "" || prev.Draft.Time != "" || prev.Draft.Reason != "" {
		b.WriteString("Previous context (may be partial): ")
		prevJSON, _ := json.Marshal(prev.Draft)
		b.Write(prevJSON)
		b.WriteString("\n")
	}
	b.WriteString("User: ")
	b.WriteString(userMessage)
	b.WriteString("\nAssistant JSON: ")

	raw, err := QueryOllama(model, b.String())
	if err != nil {
		return Appointment{}, "", err
	}
	text := strings.TrimSpace(raw)
	// Extract last JSON object
	start := strings.LastIndex(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return Appointment{}, "", errors.New("no json")
	}
	var ex ollamaExtraction
	if json.Unmarshal([]byte(text[start:end+1]), &ex) != nil {
		return Appointment{}, "", errors.New("bad json")
	}

	if strings.ToLower(strings.TrimSpace(ex.Intent)) == "book" {
		ap := Appointment{
			PatientName: strings.TrimSpace(ex.PatientName),
			Doctor:      strings.TrimSpace(ex.Doctor),
			Date:        strings.TrimSpace(ex.Date),
			Time:        strings.TrimSpace(ex.Time),
			Reason:      strings.TrimSpace(ex.Reason),
			Status:      "pending",
		}
		return ap, "", nil
	}
	// Not a book intent; ask convo follow-up separately
	return Appointment{}, "", errors.New("not book intent")
}

// AskConversationalReply asks the model for a short, friendly plain-text reply.
func AskConversationalReply(model, message string, prev ConversationState) (string, error) {
	var b strings.Builder
	b.WriteString("You are a friendly assistant helping with doctor appointments.\n")
	b.WriteString("Respond politely, conversationally, and help the user clarify date/time or doctor details if missing.\n")
	b.WriteString("Keep responses short, natural, and avoid repeating the same phrasing.\n")
	if prev.Draft.PatientName != "" || prev.Draft.Doctor != "" || prev.Draft.Date != "" || prev.Draft.Time != "" || prev.Draft.Reason != "" {
		b.WriteString("Known so far: ")
		prevJSON, _ := json.Marshal(prev.Draft)
		b.Write(prevJSON)
		b.WriteString("\n")
	}
	b.WriteString("User: ")
	b.WriteString(message)
	b.WriteString("\nAssistant (plain text): ")
	raw, err := QueryOllama(model, b.String())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}
