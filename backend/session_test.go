package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSessionIDGenerationAndValidation(t *testing.T) {
	// 1. Fresh generation generates valid 128-bit hex IDs (32 hex characters)
	generated := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sid, err := generateSessionID()
		if err != nil {
			t.Fatalf("Unexpected error generating session ID: %v", err)
		}
		if len(sid) != 32 {
			t.Fatalf("Expected 32 hex chars for 128-bit session ID, got length %d (%s)", len(sid), sid)
		}
		if !isValidSessionID(sid) {
			t.Fatalf("Generated session ID failed validation: %s", sid)
		}
		if generated[sid] {
			t.Fatalf("Collision detected on generated session ID: %s", sid)
		}
		generated[sid] = true
	}

	// 2. Validation checks
	validCases := []string{
		"1234567890123456",
		"session_id_with_underscores_and_hyphens-123456",
		"a1b2c3d4e5f678901234567890123456",
	}
	for _, tc := range validCases {
		if !isValidSessionID(tc) {
			t.Errorf("Expected valid session ID for: %s", tc)
		}
	}

	invalidCases := []string{
		"",                          // Empty
		"short",                     // Too short (<16 chars)
		"123456789012345",          // 15 chars (too short)
		"invalid with spaces 12345", // Contains spaces
		"invalid@symbols!123456789", // Contains special symbols
		string(make([]byte, 65)),    // Too long (>64 chars)
	}
	for _, tc := range invalidCases {
		if isValidSessionID(tc) {
			t.Errorf("Expected invalid session ID for: %q", tc)
		}
	}
}

func TestTwoSessionsNeverShareADraft(t *testing.T) {
	sessionA := "session_alpha_123456789012345"
	sessionB := "session_beta__123456789012345"

	// Ensure clean initial state
	setConversation(sessionA, ConversationState{})
	setConversation(sessionB, ConversationState{})

	// Session A records a booking draft for Alice with Dr. Kim
	draftA := Appointment{
		PatientName: "Alice Mutua",
		Doctor:      "Dr. Kim",
		Date:        "2026-11-20",
		Time:        "10:00",
		Reason:      "Consultation",
	}
	setConversation(sessionA, ConversationState{
		AwaitingConfirmation: true,
		Draft:                draftA,
	})

	// Verify Session B does NOT see Session A's draft
	stateB := getConversation(sessionB)
	if stateB.Draft.PatientName != "" || stateB.Draft.Doctor != "" {
		t.Fatalf("Session B leaked draft from Session A: %+v", stateB.Draft)
	}

	// Session B records a booking draft for Bob with Dr. Wangechi
	draftB := Appointment{
		PatientName: "Bob Otieno",
		Doctor:      "Dr. Wangechi",
		Date:        "2026-11-21",
		Time:        "14:00",
		Reason:      "Dental Checkup",
	}
	setConversation(sessionB, ConversationState{
		AwaitingConfirmation: false,
		Draft:                draftB,
	})

	// Verify Session A still has Alice's details intact
	stateA := getConversation(sessionA)
	if stateA.Draft.PatientName != "Alice Mutua" || stateA.Draft.Doctor != "Dr. Kim" {
		t.Fatalf("Session A state was corrupted by Session B: %+v", stateA.Draft)
	}

	// Verify Session B has Bob's details intact
	stateB = getConversation(sessionB)
	if stateB.Draft.PatientName != "Bob Otieno" || stateB.Draft.Doctor != "Dr. Wangechi" {
		t.Fatalf("Session B state incorrect: %+v", stateB.Draft)
	}
}

func TestSessionLockSerialization(t *testing.T) {
	convKey := "default:test_session_lock_123456"

	var counter int
	var wg sync.WaitGroup
	numWorkers := 20

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := acquireSessionLock(convKey)
			defer unlock()

			// Critical section
			current := counter
			// simulate small read-modify-write delay
			for j := 0; j < 1000; j++ {
			}
			counter = current + 1
		}()
	}

	wg.Wait()

	if counter != numWorkers {
		t.Fatalf("Race detected in acquireSessionLock: expected counter = %d, got %d", numWorkers, counter)
	}
}

type failingRandReader struct{}

func (f *failingRandReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated crypto/rand failure")
}

func TestGenerateSessionIDFailurePath(t *testing.T) {
	// 1. Mock randReader to fail
	origReader := randReader
	randReader = &failingRandReader{}
	defer func() {
		randReader = origReader
	}()

	sid, err := generateSessionID()
	if err == nil {
		t.Fatalf("Expected generateSessionID to fail when entropy source fails, got sid: %q", sid)
	}
	if sid != "" {
		t.Fatalf("Expected empty sid on failure, got: %q", sid)
	}

	// 2. Test chatHandler returns 500 Internal Server Error when session ID generation fails
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_session_fail.db")
	_ = initDatabase(dbPath, "development")

	app := fiber.New()
	app.Post("/t/:tenant/chat", chatHandler)

	reqBody, _ := json.Marshal(ChatRequest{
		Message:   "hi",
		SessionID: "", // triggers session ID generation
	})
	req := httptest.NewRequest(http.MethodPost, "/t/default/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("Expected HTTP 500 Internal Server Error when session ID generation fails, got: %d", resp.StatusCode)
	}
}
