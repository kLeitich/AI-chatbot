package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ChatwootWebhookPayload represents incoming webhook payloads from Chatwoot
type ChatwootWebhookPayload struct {
	Event        string               `json:"event"`
	MessageType  interface{}          `json:"message_type"` // Can be string ("incoming"/"outgoing") or int (0/1)
	Content      string               `json:"content"`
	Private      bool                 `json:"private"`
	Conversation ChatwootConversation `json:"conversation"`
	Sender       ChatwootSender       `json:"sender"`
	Account      ChatwootAccount      `json:"account"`
	Inbox        ChatwootInbox        `json:"inbox"`
}

type ChatwootConversation struct {
	ID           int                 `json:"id"`
	AccountID    int                 `json:"account_id"`
	InboxID      int                 `json:"inbox_id"`
	Status       string              `json:"status"`
	ContactInbox struct {
		SourceID string `json:"source_id"` // WhatsApp phone number e.g. "+254712345678"
	} `json:"contact_inbox"`
}

type ChatwootSender struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Type        string `json:"type"` // "contact" or "user"
}

type ChatwootAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ChatwootInbox struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"`
}

var chatwootHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// isIncomingMessage checks whether the message is from a customer/contact (not from our bot or staff)
func (p *ChatwootWebhookPayload) isIncomingMessage() bool {
	if p.Private {
		return false
	}
	if p.Sender.Type == "user" {
		return false
	}

	switch v := p.MessageType.(type) {
	case string:
		return strings.EqualFold(v, "incoming")
	case float64:
		return int(v) == 0
	case int:
		return v == 0
	default:
		return false
	}
}

// SendChatwootMessage sends an outgoing reply into the Chatwoot conversation
func SendChatwootMessage(baseURL, token string, accountID, conversationID int, content string) error {
	if baseURL == "" || token == "" || accountID == 0 || conversationID == 0 {
		return fmt.Errorf("missing chatwoot credentials: baseURL=%s accountID=%d convID=%d", baseURL, accountID, conversationID)
	}

	baseURL = strings.TrimRight(baseURL, "/")
	apiURL := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", baseURL, accountID, conversationID)

	body := map[string]interface{}{
		"content":      content,
		"message_type": "outgoing",
		"private":      false,
	}
	payloadBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_access_token", token)

	resp, err := chatwootHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to dispatch chatwoot message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chatwoot API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendChatwootPrivateNote sends an internal note visible only to clinic agents in Chatwoot
func SendChatwootPrivateNote(baseURL, token string, accountID, conversationID int, note string) error {
	if baseURL == "" || token == "" || accountID == 0 || conversationID == 0 {
		return nil
	}

	baseURL = strings.TrimRight(baseURL, "/")
	apiURL := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", baseURL, accountID, conversationID)

	body := map[string]interface{}{
		"content":      note,
		"message_type": "outgoing",
		"private":      true,
	}
	payloadBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_access_token", token)

	resp, err := chatwootHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ToggleChatwootConversationStatus changes conversation status to open (for human handoff)
func ToggleChatwootConversationStatus(baseURL, token string, accountID, conversationID int, status string) error {
	if baseURL == "" || token == "" || accountID == 0 || conversationID == 0 {
		return nil
	}

	baseURL = strings.TrimRight(baseURL, "/")
	apiURL := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/toggle_status", baseURL, accountID, conversationID)

	body := map[string]string{"status": status}
	payloadBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_access_token", token)

	resp, err := chatwootHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// chatwootWebhookHandler handles incoming webhooks from Chatwoot for WhatsApp messages
func chatwootWebhookHandler(c *fiber.Ctx) error {
	tenantSlug := strings.TrimSpace(c.Params("tenant"))
	if tenantSlug == "" {
		tenantSlug = "default"
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	var payload ChatwootWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid webhook payload")
	}

	// 1. Only process message_created events
	if payload.Event != "message_created" {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "unhandled event type"})
	}

	// 2. Only process incoming customer messages (ignore bot's own replies to avoid infinite loop)
	if !payload.isIncomingMessage() {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "not incoming contact message"})
	}

	messageText := strings.TrimSpace(payload.Content)
	if messageText == "" {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "empty message"})
	}

	convID := payload.Conversation.ID
	accountID := payload.Account.ID
	if accountID == 0 {
		accountID = tenant.ChatwootAccountID
	}
	baseURL := tenant.ChatwootBaseURL
	if baseURL == "" {
		baseURL = getEnv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	}
	apiToken := tenant.ChatwootAPIToken
	if apiToken == "" {
		apiToken = os.Getenv("CHATWOOT_API_TOKEN")
	}

	// Session ID based on Chatwoot conversation ID for state continuity
	sessionID := fmt.Sprintf("chatwoot-%d", convID)

	// Extract phone number from sender or WhatsApp contact inbox source_id
	phone := strings.TrimSpace(payload.Sender.PhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(payload.Conversation.ContactInbox.SourceID)
	}
	senderName := strings.TrimSpace(payload.Sender.Name)

	// Check for human agent handoff request
	lowerText := strings.ToLower(messageText)
	if strings.Contains(lowerText, "human") || strings.Contains(lowerText, "agent") ||
		strings.Contains(lowerText, "receptionist") || strings.Contains(lowerText, "speak to someone") {
		handoffMsg := "I have notified our clinic reception team. An agent will respond to you here on WhatsApp shortly."
		_ = SendChatwootMessage(baseURL, apiToken, accountID, convID, handoffMsg)
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID, fmt.Sprintf("👋 [Booka Bot] Patient requested human handoff. Phone: %s Name: %s", phone, senderName))
		_ = ToggleChatwootConversationStatus(baseURL, apiToken, accountID, convID, "open")
		return c.JSON(fiber.Map{"status": "handoff_initiated"})
	}

	// Deterministic Emergency Check before LLM
	if isEmergency, category := DetectEmergency(messageText); isEmergency {
		LogEmergencyDetected(tenantSlug, sessionID, category)
		emergMsg := EmergencyResponse(tenant, category)
		_ = SendChatwootMessage(baseURL, apiToken, accountID, convID, emergMsg)
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID, fmt.Sprintf("🚨 [EMERGENCY DETECTED] Category: %s. Handing over to emergency staff.", category))
		_ = ToggleChatwootConversationStatus(baseURL, apiToken, accountID, convID, "open")
		return c.JSON(fiber.Map{"status": "emergency_handled"})
	}

	// Process message using Booka's conversation engine
	replyText, confirmedApp := processChatwootMessage(tenant, sessionID, messageText, senderName, phone, convID)

	if replyText != "" {
		if err := SendChatwootMessage(baseURL, apiToken, accountID, convID, replyText); err != nil {
			log.Printf("[Chatwoot Error] Failed to send reply: %v", err)
		}
	}

	// If appointment was confirmed:
	if confirmedApp != nil {
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID,
			fmt.Sprintf("✅ [Booka Bot] Appointment booked for %s with %s on %s at %s. Patient Phone: %s",
				confirmedApp.PatientName, confirmedApp.Doctor, confirmedApp.Date, confirmedApp.Time, phone))

		// If Google Calendar is configured, sync event
		if tenant.GoogleCalendarEnabled {
			go func() {
				_ = syncAppointmentToGoogleCalendar(tenant, confirmedApp)
			}()
		}
	}

	return c.JSON(fiber.Map{"status": "processed", "reply": replyText})
}

// processChatwootMessage encapsulates the core conversation state engine for WhatsApp
func processChatwootMessage(tenant Tenant, sessionID, messageText, senderName, phone string, convID int) (string, *Appointment) {
	convKey := fmt.Sprintf("%s:%s", tenant.Slug, sessionID)
	unlock := acquireSessionLock(convKey)
	defer unlock()

	conv := getConversation(convKey)
	if conv.Draft.PatientName == "" && senderName != "" {
		conv.Draft.PatientName = senderName
	}
	sourceWhatsApp := "whatsapp"
	conv.Draft.Source = &sourceWhatsApp

	lowerMessage := strings.ToLower(messageText)
	tenantDoctors := tenantDoctorNames(tenant.ID)

	// Confirmation flow
	if conv.AwaitingConfirmation {
		if isAffirmative(lowerMessage) {
			finalReason := conv.Draft.Reason
			if strings.TrimSpace(finalReason) == "" {
				finalReason = "general consultation"
			}
			finalTime := normalizeTime(conv.Draft.Time)
			finalApp := Appointment{
				TenantID:               tenant.ID,
				PatientName:            conv.Draft.PatientName,
				PatientPhone:           phone,
				Doctor:                 conv.Draft.Doctor,
				Date:                   conv.Draft.Date,
				Time:                   finalTime,
				Reason:                 finalReason,
				Status:                 StatusPending,
				Source:                 &sourceWhatsApp,
				ChatwootConversationID: &convID,
			}

			// Validate slot still free
			slots, err := availableTimeSlots(tenant.ID, conv.Draft.Date, conv.Draft.Doctor)
			slotAvailable := false
			if err == nil {
				for _, s := range slots {
					if s == finalTime {
						slotAvailable = true
						break
					}
				}
			}

			if !slotAvailable {
				reply := fmt.Sprintf("Sorry, %s at %s is no longer available. Available slots: %s. What time would you prefer?",
					conv.Draft.Date, finalTime, strings.Join(slots, ", "))
				conv.Draft.Time = ""
				conv.Expecting = "time"
				conv.AwaitingConfirmation = false
				setConversation(convKey, conv)
				return reply, nil
			}

			bErr := db.Transaction(func(tx *gorm.DB) error {
				_, bErr := bookSlot(tx, tenant.ID, &finalApp)
				return bErr
			})
			if bErr != nil {
				reply := fmt.Sprintf("Sorry, that slot was just taken. Please suggest another time.")
				conv.Draft.Time = ""
				conv.Expecting = "time"
				conv.AwaitingConfirmation = false
				setConversation(convKey, conv)
				return reply, nil
			}

			reply := fmt.Sprintf("Great! Your appointment with %s on %s at %s for %s is confirmed, %s. See you then!",
				finalApp.Doctor, finalApp.Date, finalApp.Time, finalApp.Reason, finalApp.PatientName)
			setConversation(convKey, ConversationState{})
			return reply, &finalApp
		}

		if isNegative(lowerMessage) {
			conv.AwaitingConfirmation = false
			setConversation(convKey, conv)
			return "No problem! What would you like to change? (doctor, date, or time)", nil
		}

		conv.AwaitingConfirmation = false
	}

	// Extract answers based on expectation
	if conv.Expecting != "" {
		if handled, reply := applyExpectedAnswer(&conv, messageText, tenantDoctors); handled {
			if conv.Draft.PatientName != "" && conv.Draft.Doctor != "" &&
				isValidDate(conv.Draft.Date) && isValidTime(conv.Draft.Time) {
				if conv.Draft.Reason == "" {
					conv.Expecting = "reason"
					setConversation(convKey, conv)
					return fmt.Sprintf("Perfect! What is the reason for your visit with %s on %s at %s?",
						conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time), nil
				}
				conv.AwaitingConfirmation = true
				conv.Expecting = ""
				setConversation(convKey, conv)
				return fmt.Sprintf("I have your appointment with %s on %s at %s for %s. Does that look right? Please reply Yes to confirm.",
					conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, conv.Draft.Reason), nil
			}
			setConversation(convKey, conv)
			return reply, nil
		}
	}

	// Freeform intent extraction
	draftApp, reply, err := AskForAppointmentFromMessage(getEnv("GROQ_MODEL", "openai/gpt-oss-120b"), messageText, conv)
	if err == nil && draftApp.Doctor != "" {
		if draftApp.Doctor != "" {
			conv.Draft.Doctor = draftApp.Doctor
		}
		if draftApp.Date != "" {
			conv.Draft.Date = draftApp.Date
		}
		if draftApp.Time != "" {
			conv.Draft.Time = draftApp.Time
		}
		if draftApp.PatientName != "" {
			conv.Draft.PatientName = draftApp.PatientName
		}
		if draftApp.Reason != "" {
			conv.Draft.Reason = draftApp.Reason
		}

		if conv.Draft.PatientName != "" && conv.Draft.Doctor != "" &&
			isValidDate(conv.Draft.Date) && isValidTime(conv.Draft.Time) {
			conv.AwaitingConfirmation = true
			setConversation(convKey, conv)
			return fmt.Sprintf("I have your appointment with %s on %s at %s for %s. Shall I confirm this for you?",
				conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, conv.Draft.Reason), nil
		}
	}

	// Conversational fallback
	if reply == "" {
		reply, _ = AskConversationalReply(getEnv("GROQ_MODEL", "openai/gpt-oss-120b"), messageText, conv)
	}
	if reply == "" {
		reply = "Hello! I can help you schedule a doctor appointment. Which doctor or specialty are you looking for?"
	}

	conv.LastUserMessage = messageText
	conv.LastAIMessage = reply
	setConversation(convKey, conv)
	return reply, nil
}
