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
	PatientName string `json:"patient_name"`
	Doctor      string `json:"doctor"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Reason      string `json:"reason"`
}

func buildPrompt(message string, prev ConversationState) string {
	var b strings.Builder
	b.WriteString("You extract doctor appointment details from natural text and output ONLY JSON.\n")
	b.WriteString("Schema: {\"patient_name\": string, \"doctor\": string, \"date\": YYYY-MM-DD, \"time\": HH:MM(24h), \"reason\": string}.\n")
	b.WriteString("Rules: No prose. No markdown. No extra keys. If a field is missing, leave it empty string.\n")
	b.WriteString("Examples:\n")
	b.WriteString("Input: 'Book me with Dr. Kim tomorrow at 10am'\n")
	b.WriteString("Output: {\"patient_name\": \"\", \"doctor\": \"Dr. Kim\", \"date\": \"" + time.Now().Add(24*time.Hour).Format("2006-01-02") + "\", \"time\": \"10:00\", \"reason\": \"\"}\n")
	b.WriteString("Input: '11am on 30th October with doctor Mercy for checkup'\n")
	b.WriteString("Output: {\"patient_name\": \"\", \"doctor\": \"Dr. Mercy\", \"date\": \"2025-10-30\", \"time\": \"11:00\", \"reason\": \"checkup\"}\n")
	if prev.Draft.PatientName != "" || prev.Draft.Doctor != "" || prev.Draft.Date != "" || prev.Draft.Time != "" || prev.Draft.Reason != "" {
		b.WriteString("Previous context (partial):\n")
		prevJSON, _ := json.Marshal(prev.Draft)
		b.Write(prevJSON)
		b.WriteString("\n")
	}
	b.WriteString("Text: ")
	b.WriteString(message)
	b.WriteString("\nJSON:")
	return b.String()
}

func runOllama(prompt string) (ollamaExtraction, error) {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "phi3"
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
			return ollamaExtraction{}, errors.New(strings.TrimSpace(stderr.String()))
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return ollamaExtraction{}, errors.New("ollama timed out")
	}
	text := out.String()
	start := strings.LastIndex(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return ollamaExtraction{}, errors.New("no JSON found in model output")
	}
	var res ollamaExtraction
	if err := json.Unmarshal([]byte(text[start:end+1]), &res); err != nil {
		return ollamaExtraction{}, err
	}
	return res, nil
}
