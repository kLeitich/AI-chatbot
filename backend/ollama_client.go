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
	// Personable, JSON extraction if possible with explicit schema
	var b strings.Builder
	b.WriteString("You are a friendly medical assistant that helps people book doctor appointments.\n")
	b.WriteString("You respond politely, naturally, and conversationally.\n")
	b.WriteString("When the user wants to book an appointment, output ONLY JSON with keys: intent, patient_name, doctor, date (YYYY-MM-DD), time (HH:MM 24h), reason.\n")
	b.WriteString("If you cannot extract confidently, still output JSON with intent=chat.\n")
	if prev.Draft.PatientName != "" || prev.Draft.Doctor != "" || prev.Draft.Date != "" || prev.Draft.Time != "" || prev.Draft.Reason != "" {
		b.WriteString("Known so far (may be partial, prefer reusing unless contradicted): ")
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
		// Merge with memory: fill missing from previous draft
		if ex.PatientName == "" {
			ex.PatientName = prev.Draft.PatientName
		}
		if ex.Doctor == "" {
			ex.Doctor = prev.Draft.Doctor
		}
		if ex.Date == "" {
			ex.Date = prev.Draft.Date
		}
		if ex.Time == "" {
			ex.Time = prev.Draft.Time
		}
		if ex.Reason == "" {
			ex.Reason = prev.Draft.Reason
		}
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
	return Appointment{}, "", errors.New("not book intent")
}

// AskConversationalReply asks the model for a short, friendly plain-text reply, leveraging memory if present.
func AskConversationalReply(model, message string, prev ConversationState) (string, error) {
	var b strings.Builder
	b.WriteString("You are a friendly assistant helping with doctor appointments.\n")
	b.WriteString("Respond politely and conversationally. If details are missing, ask a brief follow-up.\n")
	b.WriteString("Use a warm human tone. Avoid repetitive phrasing.\n")
	if prev.Draft.Doctor != "" || prev.Draft.Date != "" || prev.Draft.Time != "" {
		b.WriteString("You may reference prior details to confirm, e.g., 'You mentioned ")
		if prev.Draft.Doctor != "" { b.WriteString(prev.Draft.Doctor) }
		if prev.Draft.Date != "" || prev.Draft.Time != "" { b.WriteString(" on ") }
		if prev.Draft.Date != "" { b.WriteString(prev.Draft.Date) }
		if prev.Draft.Time != "" { b.WriteString(" at "+prev.Draft.Time) }
		b.WriteString(" earlier — should I use that?'\n")
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
