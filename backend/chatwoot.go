package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ChatwootWebhookPayload represents incoming webhook payloads from Chatwoot
type ChatwootWebhookPayload struct {
	ID           int                  `json:"id"`
	Event        string               `json:"event"`
	MessageType  interface{}          `json:"message_type"` // Can be string ("incoming"/"outgoing") or int (0/1)
	Content      string               `json:"content"`
	Private      bool                 `json:"private"`
	Status       string               `json:"status"` // For conversation_status_changed / conversation_updated
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

// resolveTenantForChatwootWebhook identifies the exact healthcare provider from Chatwoot InboxID or URL slug
func resolveTenantForChatwootWebhook(inboxID, accountID int, urlSlug string) (Tenant, error) {
	var tenant Tenant

	// 1. Trusted inbox mapping (highest priority)
	if inboxID > 0 {
		if accountID > 0 {
			if err := db.Where("chatwoot_inbox_id = ? AND chatwoot_account_id = ?", inboxID, accountID).First(&tenant).Error; err == nil {
				return tenant, nil
			}
		}
		if err := db.Where("chatwoot_inbox_id = ?", inboxID).First(&tenant).Error; err == nil {
			return tenant, nil
		}
	}

	// 2. Fallback to URL parameter (/t/:tenant/webhooks/chatwoot)
	if urlSlug != "" {
		if err := db.Where("slug = ?", urlSlug).First(&tenant).Error; err == nil {
			if tenant.ChatwootInboxID > 0 && inboxID > 0 && tenant.ChatwootInboxID != inboxID {
				return Tenant{}, fmt.Errorf("inbox mismatch: url tenant '%s' requires inbox %d, got %d", urlSlug, tenant.ChatwootInboxID, inboxID)
			}
			return tenant, nil
		}
	}

	return Tenant{}, fmt.Errorf("no clinic/hospital mapped to inbox_id=%d account_id=%d slug=%s", inboxID, accountID, urlSlug)
}

// verifyChatwootWebhookSecret validates HMAC signature or secret token when configured
func verifyChatwootWebhookSecret(c *fiber.Ctx, tenant Tenant, rawBody []byte) bool {
	secret := strings.TrimSpace(tenant.ChatwootWebhookSecret)
	if secret == "" {
		return true // Secret verification not enabled for this tenant
	}

	// Check explicit header or query token
	if hdr := c.Get("X-Webhook-Secret"); hdr != "" {
		return hmac.Equal([]byte(hdr), []byte(secret))
	}
	if q := c.Query("secret"); q != "" {
		return hmac.Equal([]byte(q), []byte(secret))
	}

	// Check X-Chatwoot-Signature (HMAC-SHA256)
	sig := strings.TrimSpace(c.Get("X-Chatwoot-Signature"))
	if sig == "" {
		return false
	}
	sig = strings.TrimPrefix(sig, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedMAC))
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

// ToggleChatwootConversationStatus changes conversation status to open or resolved
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

// SendAppointmentNotificationWhatsApp dispatches appointment notifications (booking/reminder/cancel) via Chatwoot
func SendAppointmentNotificationWhatsApp(tenant Tenant, app *Appointment, message string) error {
	if app == nil || app.ChatwootConversationID == nil || *app.ChatwootConversationID == 0 {
		return nil
	}
	baseURL := tenant.ChatwootBaseURL
	if baseURL == "" {
		baseURL = getEnv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	}
	apiToken := tenant.ChatwootAPIToken
	if apiToken == "" {
		apiToken = os.Getenv("CHATWOOT_API_TOKEN")
	}
	if tenant.ChatwootAccountID == 0 || apiToken == "" {
		return nil
	}
	return SendChatwootMessage(baseURL, apiToken, tenant.ChatwootAccountID, *app.ChatwootConversationID, message)
}

// testChatwootConnectionHandler verifies Chatwoot API access token & inbox connectivity
func testChatwootConnectionHandler(c *fiber.Ctx) error {
	tenantSlug := strings.TrimSpace(c.Params("tenant"))
	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	baseURL := strings.TrimRight(tenant.ChatwootBaseURL, "/")
	if baseURL == "" {
		baseURL = getEnv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	}
	apiToken := tenant.ChatwootAPIToken
	if apiToken == "" {
		apiToken = os.Getenv("CHATWOOT_API_TOKEN")
	}

	if apiToken == "" || tenant.ChatwootAccountID == 0 {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "Chatwoot API Token and Account ID must be saved before testing connectivity")
	}

	apiURL := fmt.Sprintf("%s/api/v1/accounts/%d/inboxes", baseURL, tenant.ChatwootAccountID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, err.Error())
	}
	req.Header.Set("api_access_token", apiToken)

	resp, err := chatwootHTTPClient.Do(req)
	if err != nil {
		return sendError(c, fiber.StatusBadGateway, ErrCodeExternalError, fmt.Sprintf("Failed to reach Chatwoot server: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return sendError(c, fiber.StatusBadGateway, ErrCodeExternalError, fmt.Sprintf("Chatwoot returned HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var inboxesResp struct {
		Payload []ChatwootInbox `json:"payload"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&inboxesResp)

	inboxFound := false
	matchedInboxName := ""
	for _, inb := range inboxesResp.Payload {
		if inb.ID == tenant.ChatwootInboxID {
			inboxFound = true
			matchedInboxName = inb.Name
			break
		}
	}

	return c.JSON(fiber.Map{
		"status":             "connected",
		"account_id":         tenant.ChatwootAccountID,
		"configured_inbox":   tenant.ChatwootInboxID,
		"inbox_verified":     inboxFound,
		"matched_inbox_name": matchedInboxName,
		"total_inboxes":      len(inboxesResp.Payload),
	})
}

// chatwootWebhookHandler handles incoming webhooks from Chatwoot for WhatsApp messages
func chatwootWebhookHandler(c *fiber.Ctx) error {
	rawBody := c.Body()
	var payload ChatwootWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid webhook payload")
	}

	inboxID := payload.Inbox.ID
	if inboxID == 0 {
		inboxID = payload.Conversation.InboxID
	}
	accountID := payload.Account.ID
	if accountID == 0 {
		accountID = payload.Conversation.AccountID
	}
	urlSlug := strings.TrimSpace(c.Params("tenant"))

	tenant, err := resolveTenantForChatwootWebhook(inboxID, accountID, urlSlug)
	if err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, err.Error())
	}

	// Verify signature / webhook secret if configured
	if !verifyChatwootWebhookSecret(c, tenant, rawBody) {
		return sendError(c, fiber.StatusUnauthorized, ErrCodePermissionDenied, "invalid webhook signature")
	}

	convID := payload.Conversation.ID

	// Handle conversation status transitions (e.g. human resolved -> resume AI)
	if payload.Event == "conversation_status_changed" || payload.Event == "conversation_updated" {
		status := payload.Status
		if status == "" {
			status = payload.Conversation.Status
		}
		if strings.EqualFold(status, "resolved") && convID > 0 {
			sessionKey := fmt.Sprintf("chatwoot-%d-%d", tenant.ID, convID)
			db.Model(&ChatSession{}).Where("session_id = ?", sessionKey).Updates(map[string]interface{}{
				"human_handover": false,
				"ai_enabled":     tenant.AIReceptionistEnabled,
				"workflow_state": "initial",
			})
		}
		return c.JSON(fiber.Map{"status": "ignored", "reason": "conversation event processed"})
	}

	// 1. Only process message_created events beyond this point
	if payload.Event != "message_created" {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "unhandled event type"})
	}

	// 2. Only process incoming customer messages (ignore bot's own replies or private notes)
	if !payload.isIncomingMessage() {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "not incoming contact message"})
	}

	messageText := strings.TrimSpace(payload.Content)
	if messageText == "" {
		return c.JSON(fiber.Map{"status": "ignored", "reason": "empty message"})
	}

	// 3. Idempotency Guard: Deduplicate repeated webhooks for the same message ID
	if payload.ID > 0 {
		var existing int64
		db.Model(&ChatwootProcessedMessage{}).Where("message_id = ?", payload.ID).Count(&existing)
		if existing > 0 {
			return c.JSON(fiber.Map{"status": "ignored", "reason": "duplicate_message"})
		}
		_ = db.Create(&ChatwootProcessedMessage{
			MessageID:      payload.ID,
			ConversationID: convID,
			InboxID:        inboxID,
			TenantID:       tenant.ID,
			ProcessedAt:    time.Now(),
		}).Error
	}

	// 4. Synchronous processing in test runs, non-blocking background execution in production
	isTestMode := flag.Lookup("test.v") != nil
	if isTestMode {
		resMap := executeChatwootWebhookMessage(tenant, payload, inboxID, accountID, convID, messageText)
		return c.JSON(resMap)
	}

	go func() {
		_ = executeChatwootWebhookMessage(tenant, payload, inboxID, accountID, convID, messageText)
	}()

	return c.JSON(fiber.Map{"status": "accepted", "tenant": tenant.Slug})
}

func executeChatwootWebhookMessage(tenant Tenant, payload ChatwootWebhookPayload, inboxID, accountID, convID int, messageText string) fiber.Map {
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

	phone := strings.TrimSpace(payload.Sender.PhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(payload.Conversation.ContactInbox.SourceID)
	}
	senderName := strings.TrimSpace(payload.Sender.Name)

	convKey := fmt.Sprintf("cw:%d:%d", tenant.ID, convID)
	unlock := acquireSessionLock(convKey)
	defer unlock()

	session, err := loadOrCreateChatSession(tenant, convID, inboxID, accountID, payload.Sender.ID, phone, senderName)
	if err != nil {
		log.Printf("[CHATWOOT ERROR] Failed to load session: %v", err)
		return fiber.Map{"status": "error"}
	}

	// If AI receptionist is disabled for this tenant or conversation is handed off to a human agent:
	if !tenant.AIReceptionistEnabled || session.HumanHandover {
		session.LastUserMessage = messageText
		_ = db.Save(session).Error
		return fiber.Map{"status": "skipped_human_active"}
	}

	// Deterministic Emergency Check before LLM
	if isEmergency, category := DetectEmergency(messageText); isEmergency {
		LogEmergencyDetected(tenant.Slug, session.SessionID, category)
		emergMsg := EmergencyResponse(tenant, category)
		session.HumanHandover = true
		session.WorkflowState = "handed_off"
		session.HandoverReason = "Emergency detected: " + string(category)
		session.LastUserMessage = messageText
		session.LastAIMessage = emergMsg
		_ = db.Save(session).Error

		_ = SendChatwootMessage(baseURL, apiToken, accountID, convID, emergMsg)
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID,
			fmt.Sprintf("🚨 [EMERGENCY DETECTED] Category: %s. Phone: %s Name: %s. Immediate staff intervention required.", category, phone, senderName))
		_ = ToggleChatwootConversationStatus(baseURL, apiToken, accountID, convID, "open")
		return fiber.Map{"status": "emergency_handled", "reply": emergMsg}
	}

	// Deterministic Handover Keywords Check
	lowerText := strings.ToLower(messageText)
	keywords := strings.Split(tenant.HandoffKeywords, ",")
	if len(keywords) == 0 || (len(keywords) == 1 && keywords[0] == "") {
		keywords = []string{"human", "agent", "receptionist", "speak to someone"}
	}
	for _, kw := range keywords {
		cleanKW := strings.TrimSpace(strings.ToLower(kw))
		if cleanKW != "" && strings.Contains(lowerText, cleanKW) {
			handoffMsg := fmt.Sprintf("I have notified our team at %s. A human representative will assist you here on WhatsApp shortly.", tenant.Name)
			session.HumanHandover = true
			session.WorkflowState = "handed_off"
			session.HandoverReason = "Patient keyword match: " + cleanKW
			session.LastUserMessage = messageText
			session.LastAIMessage = handoffMsg
			_ = db.Save(session).Error

			_ = SendChatwootMessage(baseURL, apiToken, accountID, convID, handoffMsg)
			_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID,
				fmt.Sprintf("👋 [Booka Bot Handover] Patient requested human support. Phone: %s Name: %s\nLast message: %s", phone, senderName, messageText))
			_ = ToggleChatwootConversationStatus(baseURL, apiToken, accountID, convID, "open")
			return fiber.Map{"status": "handoff_initiated", "reply": handoffMsg}
		}
	}

	// Execute AI Agent & Tool-Calling Orchestration
	replyText, confirmedApp, aiHandover, handoverReason := RunChatwootAIAgent(tenant, session, messageText)

	if payload.ID > 0 {
		session.LastProcessedMessageID = payload.ID
	}
	session.LastUserMessage = messageText
	if replyText != "" {
		session.LastAIMessage = replyText
	}
	_ = db.Save(session).Error

	if replyText != "" {
		if err := SendChatwootMessage(baseURL, apiToken, accountID, convID, replyText); err != nil {
			log.Printf("[Chatwoot Error] Failed to send reply: %v", err)
		}
	}

	if aiHandover {
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID,
			fmt.Sprintf("👋 [Booka AI Escalation] Reason: %s\nPhone: %s Name: %s", handoverReason, phone, senderName))
		_ = ToggleChatwootConversationStatus(baseURL, apiToken, accountID, convID, "open")
	}

	if confirmedApp != nil {
		_ = SendChatwootPrivateNote(baseURL, apiToken, accountID, convID,
			fmt.Sprintf("✅ [Booka Booking #%d] Confirmed for %s with %s on %s at %s. Phone: %s",
				confirmedApp.ID, confirmedApp.PatientName, confirmedApp.Doctor, confirmedApp.Date, confirmedApp.Time, phone))

		// Trigger Google Calendar Sync
		if tenant.GoogleCalendarEnabled {
			go func(t Tenant, ap Appointment) {
				_ = syncAppointmentToGoogleCalendar(t, &ap)
			}(tenant, *confirmedApp)
		}
	}

	return fiber.Map{"status": "processed", "reply": replyText}
}

// listChatwootSessionsHandler lists active WhatsApp/Chatwoot sessions for the clinic admin dashboard
func listChatwootSessionsHandler(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}
	var sessions []ChatSession
	if err := db.Where("tenant_id = ?", tenantID).Order("updated_at DESC").Limit(50).Find(&sessions).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to load WhatsApp sessions")
	}
	return c.JSON(sessions)
}

// resumeChatwootAISessionHandler allows an authorized clinic agent to re-enable AI assistance after human handover
func resumeChatwootAISessionHandler(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}
	sessionID := strings.TrimSpace(c.Params("sessionID"))
	if sessionID == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "missing sessionID")
	}
	var session ChatSession
	if err := db.Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).First(&session).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "session not found")
	}

	session.HumanHandover = false
	session.AIEnabled = true
	session.WorkflowState = "initial"
	session.HandoverReason = ""
	if err := db.Save(&session).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to resume AI session")
	}

	return c.JSON(session)
}

