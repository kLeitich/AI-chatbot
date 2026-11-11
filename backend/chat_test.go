package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupTestApp(t *testing.T) (*fiber.App, string) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, fmt.Sprintf("test_chat_%d.db", time.Now().UnixNano()))
	_ = initDatabase(dbPath, "development")

	app := fiber.New()
	app.Post("/t/:tenant/chat", chatHandler)

	sessionID, _ := generateSessionID()
	return app, sessionID
}

func TestChatSlotFillingRegression(t *testing.T) {
	app, sessionID := setupTestApp(t)

	type step struct {
		name        string
		userMessage string
		assertReply func(t *testing.T, reply string)
		assertDraft func(t *testing.T, conv ConversationState)
	}

	steps := []step{
		{
			name:        "Turn 1: Greeting",
			userMessage: "hi",
			assertReply: func(t *testing.T, reply string) {
				if !strings.Contains(strings.ToLower(reply), "name") {
					t.Errorf("Expected bot to ask for name on initial greeting, got: %s", reply)
				}
			},
			assertDraft: func(t *testing.T, conv ConversationState) {
				if conv.Expecting != "name" {
					t.Errorf("Expected Expecting='name', got: %s", conv.Expecting)
				}
			},
		},
		{
			name:        "Turn 2: Patient Name",
			userMessage: "kevin",
			assertReply: func(t *testing.T, reply string) {
				// after "kevin": bot asks for the doctor
				lower := strings.ToLower(reply)
				if !strings.Contains(lower, "doctor") {
					t.Errorf("Expected bot to ask for doctor, got: %s", reply)
				}
				if !strings.Contains(reply, "Dr. Kim") {
					t.Errorf("Expected bot to list available doctors (Dr. Kim), got: %s", reply)
				}
			},
			assertDraft: func(t *testing.T, conv ConversationState) {
				// after "kevin": name = "Kevin"
				if conv.Draft.PatientName != "Kevin" {
					t.Errorf("Expected Draft.PatientName='Kevin', got: %s", conv.Draft.PatientName)
				}
				if conv.Expecting != "doctor" {
					t.Errorf("Expected Expecting='doctor', got: %s", conv.Expecting)
				}
			},
		},
		{
			name:        "Turn 3: Invalid Doctor",
			userMessage: "Alice",
			assertReply: func(t *testing.T, reply string) {
				// after "Alice": reply contains "couldn't find"
				lower := strings.ToLower(reply)
				if !strings.Contains(lower, "couldn't find") && !strings.Contains(lower, "could not find") {
					t.Errorf("Expected reply to contain \"couldn't find\", got: %s", reply)
				}
				if !strings.Contains(reply, "Alice") {
					t.Errorf("Expected reply to mention unrecognized doctor name 'Alice', got: %s", reply)
				}
			},
			assertDraft: func(t *testing.T, conv ConversationState) {
				// after "Alice": doctor still empty, name still "Kevin"
				if conv.Draft.PatientName != "Kevin" {
					t.Errorf("Expected Draft.PatientName to remain 'Kevin', got: %s", conv.Draft.PatientName)
				}
				if conv.Draft.Doctor != "" {
					t.Errorf("Expected Draft.Doctor to remain empty, got: %s", conv.Draft.Doctor)
				}
				if conv.Expecting != "doctor" {
					t.Errorf("Expected Expecting='doctor', got: %s", conv.Expecting)
				}
			},
		},
		{
			name:        "Turn 4: Valid Doctor",
			userMessage: "kim",
			assertReply: func(t *testing.T, reply string) {
				// after "kim": bot asks for the date
				lower := strings.ToLower(reply)
				if !strings.Contains(lower, "date") {
					t.Errorf("Expected bot to ask for date, got: %s", reply)
				}
			},
			assertDraft: func(t *testing.T, conv ConversationState) {
				// after "kim": doctor = "Dr. Kim"
				if conv.Draft.Doctor != "Dr. Kim" {
					t.Errorf("Expected Draft.Doctor='Dr. Kim', got: %s", conv.Draft.Doctor)
				}
				if conv.Draft.PatientName != "Kevin" {
					t.Errorf("Expected Draft.PatientName='Kevin', got: %s", conv.Draft.PatientName)
				}
				if conv.Expecting != "date" {
					t.Errorf("Expected Expecting='date', got: %s", conv.Expecting)
				}
			},
		},
	}

	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(ChatRequest{
				Message:   s.userMessage,
				SessionID: sessionID,
			})
			req := httptest.NewRequest(http.MethodPost, "/t/default/chat", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, 5000)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			var chatResp ChatResponse
			if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
				t.Fatalf("Failed to parse JSON response: %v (raw: %s)", err, string(bodyBytes))
			}

			if chatResp.SessionID != sessionID {
				t.Errorf("Expected session_id=%s, got %s", sessionID, chatResp.SessionID)
			}

			s.assertReply(t, chatResp.Reply)

			conv := getConversation("default:" + sessionID)
			s.assertDraft(t, conv)
		})
	}
}

func TestChatValidateAtConfirmTime(t *testing.T) {
	app, sessionID := setupTestApp(t)

	// Pre-set conversation to awaiting confirmation for a specific slot
	targetDate, _ := getFutureWeekday(time.Tuesday, 7)
	targetTime := "10:00"

	convKey := "default:" + sessionID
	setConversation(convKey, ConversationState{
		AwaitingConfirmation: true,
		Draft: Appointment{
			TenantID:    1,
			PatientName: "John Doe",
			Doctor:      "Dr. Kim",
			Date:        targetDate,
			Time:        targetTime,
			Reason:      "Checkup",
		},
	})

	// Before user confirms, simulate another patient taking the exact same slot
	existingApp := &Appointment{
		TenantID:    1,
		Doctor:      "Dr. Kim",
		Date:        targetDate,
		Time:        targetTime,
		PatientName: "Other Patient",
		Reason:      "Other Checkup",
		Status:      StatusPending,
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		_, bErr := bookSlot(tx, 1, existingApp)
		return bErr
	})
	if err != nil {
		t.Fatalf("Failed to seed conflicting appointment: %v", err)
	}

	// Now user sends "yes" to confirm their draft
	reqBody, _ := json.Marshal(ChatRequest{
		Message:   "yes",
		SessionID: sessionID,
	})
	req := httptest.NewRequest(http.MethodPost, "/t/default/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var chatResp ChatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v (raw: %s)", err, string(bodyBytes))
	}

	// Must reject confirmation, notify user slot is no longer available, and show free slots
	if !strings.Contains(strings.ToLower(chatResp.Reply), "no longer available") {
		t.Errorf("Expected reply to indicate slot is no longer available, got: %s", chatResp.Reply)
	}
	if chatResp.Appointment != nil {
		t.Errorf("Expected Appointment to be nil on conflict, got: %+v", chatResp.Appointment)
	}

	// Verify state in session store: Draft.Time is cleared, Expecting="time", AwaitingConfirmation=false
	conv := getConversation(convKey)
	if conv.Draft.Time != "" {
		t.Errorf("Expected Draft.Time to be cleared, got: %s", conv.Draft.Time)
	}
	if conv.Expecting != "time" {
		t.Errorf("Expected Expecting='time', got: %s", conv.Expecting)
	}
	if conv.AwaitingConfirmation {
		t.Errorf("Expected AwaitingConfirmation to be false, got true")
	}
}

