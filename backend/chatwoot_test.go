package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

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
