package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestChatwootIsIncomingMessage(t *testing.T) {
	// Incoming contact message
	incomingPayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "incoming",
		Private:     false,
		Sender: ChatwootSender{
			Type: "contact",
		},
	}
	if !incomingPayload.isIncomingMessage() {
		t.Fatalf("Expected incoming contact message to return true")
	}

	// Incoming message with numeric message_type 0
	incomingNum := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: float64(0),
		Private:     false,
		Sender: ChatwootSender{
			Type: "contact",
		},
	}
	if !incomingNum.isIncomingMessage() {
		t.Fatalf("Expected numeric 0 message type to be identified as incoming")
	}

	// Outgoing message (bot's own reply or agent reply)
	outgoingPayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "outgoing",
		Private:     false,
		Sender: ChatwootSender{
			Type: "user",
		},
	}
	if outgoingPayload.isIncomingMessage() {
		t.Fatalf("Expected outgoing agent message to return false")
	}

	// Private note
	privatePayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "incoming",
		Private:     true,
		Sender: ChatwootSender{
			Type: "contact",
		},
	}
	if privatePayload.isIncomingMessage() {
		t.Fatalf("Expected private note to return false")
	}
}

func TestChatwootWebhookHandlerRouting(t *testing.T) {
	tenantID := setupTestDB(t)

	var tenant Tenant
	db.First(&tenant, tenantID)

	app := fiber.New()
	app.Post("/t/:tenant/webhooks/chatwoot", chatwootWebhookHandler)

	// 1. Ignore non-message_created events
	nonMessagePayload := ChatwootWebhookPayload{
		Event: "conversation_updated",
	}
	b, _ := json.Marshal(nonMessagePayload)
	req := httptest.NewRequest("POST", "/t/"+tenant.Slug+"/webhooks/chatwoot", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK for ignored event, got status %d, err %v", resp.StatusCode, err)
	}

	// 2. Ignore bot outgoing message (prevent loops)
	outgoingPayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "outgoing",
		Sender: ChatwootSender{
			Type: "user",
		},
		Content: "Hello from agent",
	}
	b, _ = json.Marshal(outgoingPayload)
	req = httptest.NewRequest("POST", "/t/"+tenant.Slug+"/webhooks/chatwoot", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK for ignored outgoing message, got status %d", resp.StatusCode)
	}

	// 3. Human handoff detection
	handoffPayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "incoming",
		Content:     "I want to speak to a human receptionist",
		Sender: ChatwootSender{
			Name:        "John Doe",
			PhoneNumber: "+254700000000",
			Type:        "contact",
		},
		Conversation: ChatwootConversation{
			ID: 101,
		},
	}
	b, _ = json.Marshal(handoffPayload)
	req = httptest.NewRequest("POST", "/t/"+tenant.Slug+"/webhooks/chatwoot", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK for human handoff request, got %d", resp.StatusCode)
	}

	// 4. Emergency handoff detection
	emergencyPayload := ChatwootWebhookPayload{
		Event:       "message_created",
		MessageType: "incoming",
		Content:     "I have severe chest pain and cannot breathe",
		Sender: ChatwootSender{
			Name:        "Jane Doe",
			PhoneNumber: "+254711111111",
			Type:        "contact",
		},
		Conversation: ChatwootConversation{
			ID: 102,
		},
	}
	b, _ = json.Marshal(emergencyPayload)
	req = httptest.NewRequest("POST", "/t/"+tenant.Slug+"/webhooks/chatwoot", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Expected 200 OK for emergency escalation, got %d", resp.StatusCode)
	}
}

func TestChatwootMultiTenantInboxResolutionAndIdempotency(t *testing.T) {
	setupTestDB(t)

	// Create 2 distinct healthcare providers with distinct Chatwoot Inbox IDs
	clinicA := Tenant{
		Name:                  "Nairobi Medical Centre",
		Slug:                  "nairobi-med",
		ChatwootEnabled:       true,
		ChatwootAccountID:     1,
		ChatwootInboxID:       1001,
		AIReceptionistEnabled: true,
	}
	clinicB := Tenant{
		Name:                  "Mombasa Hospital",
		Slug:                  "mombasa-hosp",
		ChatwootEnabled:       true,
		ChatwootAccountID:     1,
		ChatwootInboxID:       2002,
		AIReceptionistEnabled: true,
	}
	if err := db.Create(&clinicA).Error; err != nil {
		t.Fatalf("Failed creating Clinic A: %v", err)
	}
	if err := db.Create(&clinicB).Error; err != nil {
		t.Fatalf("Failed creating Clinic B: %v", err)
	}

	// Verify Inbox A resolves exclusively to Clinic A
	resolvedA, err := resolveTenantForChatwootWebhook(1001, 1, "")
	if err != nil || resolvedA.ID != clinicA.ID {
		t.Fatalf("Expected Clinic A (%d), got %v (err=%v)", clinicA.ID, resolvedA.ID, err)
	}

	// Verify Inbox B resolves exclusively to Clinic B
	resolvedB, err := resolveTenantForChatwootWebhook(2002, 1, "")
	if err != nil || resolvedB.ID != clinicB.ID {
		t.Fatalf("Expected Clinic B (%d), got %v (err=%v)", clinicB.ID, resolvedB.ID, err)
	}

	// Verify cross-tenant inbox mismatch protection on tenant-scoped route
	_, mismatchErr := resolveTenantForChatwootWebhook(2002, 1, "nairobi-med")
	if mismatchErr == nil {
		t.Fatalf("Expected mismatch error when Inbox 2002 hits Clinic A route")
	}

	// Test global /webhooks/chatwoot route idempotency
	app := fiber.New()
	app.Post("/webhooks/chatwoot", chatwootWebhookHandler)

	msgPayload := ChatwootWebhookPayload{
		ID:          99001,
		Event:       "message_created",
		MessageType: "incoming",
		Content:     "Hello Clinic A",
		Account:     ChatwootAccount{ID: 1},
		Inbox:       ChatwootInbox{ID: 1001},
		Sender: ChatwootSender{
			ID:          55,
			Name:        "Alice Test",
			PhoneNumber: "+254722111222",
			Type:        "contact",
		},
		Conversation: ChatwootConversation{ID: 5001, InboxID: 1001},
	}

	b, _ := json.Marshal(msgPayload)
	req1 := httptest.NewRequest("POST", "/webhooks/chatwoot", bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := app.Test(req1)
	if err != nil || resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("First webhook call failed: status=%d err=%v", resp1.StatusCode, err)
	}

	// Second duplicate webhook with same Message ID 99001
	req2 := httptest.NewRequest("POST", "/webhooks/chatwoot", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	if err != nil || resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("Duplicate webhook call failed: status=%d err=%v", resp2.StatusCode, err)
	}
	bodyBytes, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(bodyBytes), "duplicate_message") {
		t.Errorf("Expected duplicate_message response, got %s", string(bodyBytes))
	}

	// Verify persistent ChatSession was stored in DB for Clinic A
	var sess ChatSession
	if err := db.Where("chatwoot_conversation_id = ? AND tenant_id = ?", 5001, clinicA.ID).First(&sess).Error; err != nil {
		t.Fatalf("Expected persistent ChatSession for Clinic A, got %v", err)
	}
	if sess.PatientPhone != "+254722111222" {
		t.Errorf("Expected patient phone '+254722111222', got '%s'", sess.PatientPhone)
	}
}

func TestChatwootControlledBookingToolsAndIsolation(t *testing.T) {
	setupTestDB(t)

	clinicA := Tenant{Name: "Clinic Alpha", Slug: "alpha", AIReceptionistEnabled: true, MarketplaceEnabled: false}
	clinicB := Tenant{Name: "Clinic Beta", Slug: "beta", AIReceptionistEnabled: true, MarketplaceEnabled: false}
	db.Create(&clinicA)
	db.Create(&clinicB)

	branchA := Branch{TenantID: clinicA.ID, Name: "Westlands Wing", Address: "Westlands Rd", Phone: "+254700111222", Active: true}
	db.Create(&branchA)

	docA := Doctor{
		TenantID:        clinicA.ID,
		Name:            "Dr. Achieng",
		Specialty:       "Pediatrics",
		WorkDays:        "Mon,Tue,Wed,Thu,Fri,Sat,Sun",
		WorkStart:       "08:00",
		WorkEnd:         "17:00",
		SlotMinutes:     30,
		Active:          true,
		ConsultationFee: 2500,
		BranchID:        &branchA.ID,
		BranchName:      branchA.Name,
	}
	docB := Doctor{
		TenantID:    clinicB.ID,
		Name:        "Dr. BetaOnly",
		Specialty:   "Cardiology",
		WorkDays:    "Mon,Tue,Wed,Thu,Fri",
		WorkStart:   "09:00",
		WorkEnd:     "16:00",
		SlotMinutes: 30,
		Active:      true,
	}
	db.Create(&docA)
	db.Create(&docB)

	sessionA := &ChatSession{
		SessionID:              "test-sess-a",
		TenantID:               clinicA.ID,
		ProviderID:             clinicA.ID,
		PatientPhone:           "+254799888777",
		PatientName:            "Test Patient",
		ChatwootConversationID: 888,
	}

	// 1. Search doctors scoped strictly to Clinic A (Dr. BetaOnly must NOT appear)
	docSearchRes := ExecuteBookingTool(clinicA, sessionA, "search_doctors", `{"cross_provider": true}`)
	if strings.Contains(docSearchRes.Payload, "Dr. BetaOnly") {
		t.Fatalf("Tenant isolation breach! Marketplace search leaked Clinic B doctor when marketplace_enabled=false")
	}
	if !strings.Contains(docSearchRes.Payload, "Dr. Achieng") {
		t.Fatalf("Expected Dr. Achieng in search results, got %s", docSearchRes.Payload)
	}

	// 2. Book appointment for tomorrow
	tomorrow := nowInAppLocation().Add(24 * time.Hour).Format("2006-01-02")
	createArgs := `{"doctor_name": "Dr. Achieng", "date": "` + tomorrow + `", "time": "10:00", "patient_name": "Test Patient", "reason": "Annual Checkup"}`
	createRes := ExecuteBookingTool(clinicA, sessionA, "create_appointment", createArgs)
	if createRes.ConfirmedBooking == nil {
		t.Fatalf("Expected confirmed booking, got payload %s", createRes.Payload)
	}
	if createRes.ConfirmedBooking.ConsultationFee != 2500 || createRes.ConfirmedBooking.BranchName != "Westlands Wing" {
		t.Errorf("Expected fee=2500 and branch='Westlands Wing', got fee=%v branch=%s",
			createRes.ConfirmedBooking.ConsultationFee, createRes.ConfirmedBooking.BranchName)
	}

	// 3. Verify Clinic B cannot cancel Clinic A's appointment
	sessionB := &ChatSession{TenantID: clinicB.ID, PatientPhone: "+254799888777"}
	cancelCrossRes := ExecuteBookingTool(clinicB, sessionB, "cancel_appointment", `{"appointment_id": 1}`)
	if !strings.Contains(cancelCrossRes.Payload, "error") {
		t.Fatalf("Expected permission/not found error on cross-tenant cancel, got %s", cancelCrossRes.Payload)
	}

	// 4. Verify Clinic A patient can reschedule and cancel their own appointment
	reschedArgs := `{"appointment_id": ` + formatUint(createRes.ConfirmedBooking.ID) + `, "new_date": "` + tomorrow + `", "new_time": "11:30"}`
	reschedRes := ExecuteBookingTool(clinicA, sessionA, "reschedule_appointment", reschedArgs)
	if !strings.Contains(reschedRes.Payload, `"status":"rescheduled"`) {
		t.Fatalf("Expected rescheduled confirmation, got %s", reschedRes.Payload)
	}

	// 5. Transfer to human
	handoffRes := ExecuteBookingTool(clinicA, sessionA, "transfer_to_human", `{"reason": "Insurance inquiry"}`)
	if !handoffRes.HandoverTriggered || !sessionA.HumanHandover {
		t.Fatalf("Expected HumanHandover=true after transfer_to_human tool call")
	}
}

func formatUint(v uint) string {
	b, _ := json.Marshal(v)
	return string(b)
}

