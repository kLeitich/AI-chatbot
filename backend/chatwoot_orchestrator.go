package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
)

type groqChatMessage struct {
	Role       string          `json:"role"`
	Content    interface{}     `json:"content"`
	ToolCalls  []groqToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type groqToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// loadOrCreatePatient resolves a Patient row for this tenant by phone or Chatwoot contact ID
func loadOrCreatePatient(tenantID uint, phone, name string, contactID int) (*Patient, error) {
	cleanPhone := strings.TrimSpace(phone)
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = "WhatsApp Patient"
	}

	var p Patient
	if contactID > 0 {
		if err := db.Where("tenant_id = ? AND chatwoot_contact_id = ?", tenantID, contactID).First(&p).Error; err == nil {
			if cleanPhone != "" && p.Phone == "" {
				p.Phone = cleanPhone
				db.Save(&p)
			}
			return &p, nil
		}
	}

	if cleanPhone != "" {
		if err := db.Where("tenant_id = ? AND phone = ?", tenantID, cleanPhone).First(&p).Error; err == nil {
			if contactID > 0 && p.ChatwootContactID == nil {
				cID := contactID
				p.ChatwootContactID = &cID
				db.Save(&p)
			}
			return &p, nil
		}
	}

	var cIDPtr *int
	if contactID > 0 {
		cID := contactID
		cIDPtr = &cID
	}
	newPatient := Patient{
		TenantID:          tenantID,
		Name:              cleanName,
		Phone:             cleanPhone,
		ChatwootContactID: cIDPtr,
	}
	if err := db.Create(&newPatient).Error; err != nil {
		return nil, err
	}
	return &newPatient, nil
}

// loadOrCreateChatSession retrieves or creates persistent conversation state in DB
func loadOrCreateChatSession(tenant Tenant, convID, inboxID, accountID, contactID int, phone, senderName string) (*ChatSession, error) {
	sessionKey := fmt.Sprintf("chatwoot-%d-%d", tenant.ID, convID)

	var sess ChatSession
	err := db.Where("session_id = ?", sessionKey).First(&sess).Error
	if err == nil {
		updated := false
		if sess.PatientPhone == "" && phone != "" {
			sess.PatientPhone = phone
			updated = true
		}
		if sess.PatientName == "" && senderName != "" {
			sess.PatientName = senderName
			updated = true
		}
		if updated {
			_ = db.Save(&sess).Error
		}
		return &sess, nil
	}

	patient, _ := loadOrCreatePatient(tenant.ID, phone, senderName, contactID)
	var patientID *uint
	if patient != nil {
		patientID = &patient.ID
	}

	sess = ChatSession{
		SessionID:              sessionKey,
		TenantID:               tenant.ID,
		ProviderID:             tenant.ID,
		PatientID:              patientID,
		PatientPhone:           phone,
		PatientName:            senderName,
		ChatwootAccountID:      accountID,
		ChatwootInboxID:        inboxID,
		ChatwootConversationID: convID,
		ChatwootContactID:      contactID,
		WorkflowState:          "initial",
		AIEnabled:              tenant.AIReceptionistEnabled,
	}
	if err := db.Create(&sess).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

// callGroqWithTools executes a single step of Groq Chat Completions with OpenAI-style function calling
func callGroqWithTools(messages []groqChatMessage, tools []map[string]interface{}) (*groqChatMessage, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GROQ_API_KEY not set")
	}

	model := os.Getenv("GROQ_MODEL")
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"tools":       tools,
		"tool_choice": "auto",
		"temperature": 0.3,
		"max_tokens":  700,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Groq tool API error %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Choices []struct {
			Message groqChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, errors.New("no choices returned from Groq")
	}

	return &result.Choices[0].Message, nil
}

// RunChatwootAIAgent orchestrates the WhatsApp AI receptionist using backend tools and persistent session state
func RunChatwootAIAgent(tenant Tenant, session *ChatSession, messageText string) (string, *Appointment, bool, string) {
	todayStr := nowInAppLocation().Format("2006-01-02 (Monday)")
	tz := tenant.Timezone
	if tz == "" {
		tz = "Africa/Nairobi"
	}

	marketplaceNote := "You are scoped strictly to this clinic/hospital. Do NOT recommend other hospitals."
	if tenant.MarketplaceEnabled {
		marketplaceNote = "By default search within this clinic/hospital. If the patient explicitly asks for alternative hospitals or cross-provider comparisons, you may use search_providers or search_doctors with cross_provider=true."
	}

	systemPrompt := fmt.Sprintf(`You are the friendly, professional AI WhatsApp Receptionist for %s (%s).
Today's date (%s): %s.

PATIENT PROFILE:
- Name: %s
- Phone: %s

PROVIDER DETAILS:
- Business Hours: %s
- Address: %s
- Cancellation Policy: %s
- Rescheduling Policy: %s

STRICT RULES & SECURITY MANDATE:
1. You are strictly an administrative scheduling assistant. NEVER diagnose conditions, prescribe medication, or offer medical advice. If a patient seeks medical advice or expresses a complaint, call 'transfer_to_human'.
2. NEVER fabricate doctors, branches, appointment slots, consultation fees, or booking references. ALWAYS use the provided tools to query real data.
3. %s
4. Before calling 'create_appointment', you must confirm: doctor name, date (YYYY-MM-DD), time (24h HH:MM), patient name, and visit reason.
5. Keep WhatsApp responses clear, warm, concise, and easy to read on mobile.
6. If the patient explicitly requests to speak to a human, receptionist, or agent, immediately call 'transfer_to_human'.`,
		tenant.Name, tenant.ProviderType, tz, todayStr,
		session.PatientName, session.PatientPhone,
		tenant.BusinessHours, tenant.Address, tenant.CancellationPolicy, tenant.ReschedulingPolicy,
		marketplaceNote)

	messages := []groqChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	if session.ConversationSummary != "" {
		messages = append(messages, groqChatMessage{
			Role:    "system",
			Content: "Previous conversation summary: " + session.ConversationSummary,
		})
	}
	if session.LastUserMessage != "" && session.LastAIMessage != "" {
		messages = append(messages,
			groqChatMessage{Role: "user", Content: session.LastUserMessage},
			groqChatMessage{Role: "assistant", Content: session.LastAIMessage},
		)
	}
	messages = append(messages, groqChatMessage{
		Role:    "user",
		Content: messageText,
	})

	tools := GetBookingToolsSchema()
	var confirmedApp *Appointment
	var handoverTriggered bool
	var handoverReason string

	// Up to 5 tool-calling hops per patient turn
	for hop := 0; hop < 5; hop++ {
		assistantMsg, err := callGroqWithTools(messages, tools)
		if err != nil {
			log.Printf("[CHATWOOT AGENT] Groq tool execution failed (%v); falling back to deterministic engine", err)
			reply, app := processChatwootFallback(tenant, session, messageText)
			return reply, app, handoverTriggered, handoverReason
		}

		messages = append(messages, *assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			finalReply := ""
			if contentStr, ok := assistantMsg.Content.(string); ok {
				finalReply = strings.TrimSpace(contentStr)
			}
			return finalReply, confirmedApp, handoverTriggered, handoverReason
		}

		// Execute each tool requested by the AI
		for _, tc := range assistantMsg.ToolCalls {
			toolResult := ExecuteBookingTool(tenant, session, tc.Function.Name, tc.Function.Arguments)
			if toolResult.ConfirmedBooking != nil {
				confirmedApp = toolResult.ConfirmedBooking
			}
			if toolResult.HandoverTriggered {
				handoverTriggered = true
				handoverReason = toolResult.HandoverReasonText
			}

			messages = append(messages, groqChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolResult.Payload,
			})
		}
	}

	return "I'm still working on checking that for you. Could you please clarify what doctor or date you prefer?", confirmedApp, handoverTriggered, handoverReason
}

// processChatwootFallback provides deterministic state-machine scheduling when LLM is unavailable (e.g. offline/tests)
func processChatwootFallback(tenant Tenant, session *ChatSession, messageText string) (string, *Appointment) {
	lowerMessage := strings.ToLower(strings.TrimSpace(messageText))
	tenantDoctors := tenantDoctorNames(tenant.ID)

	// Restore ConversationState from DB session
	conv := ConversationState{
		LastUserMessage:      session.LastUserMessage,
		LastAIMessage:        session.LastAIMessage,
		AwaitingConfirmation: session.AwaitingConfirmation,
		Expecting:            session.WorkflowState,
		Draft: Appointment{
			TenantID:     tenant.ID,
			PatientName:  session.PatientName,
			PatientPhone: session.PatientPhone,
			Doctor:       session.SelectedDoctorName,
			Date:         session.SelectedDate,
			Time:         session.SelectedTime,
			Reason:       session.SelectedReason,
		},
	}

	if conv.Expecting == "initial" || conv.Expecting == "completed" {
		conv.Expecting = ""
	}

	// Confirmation handling
	if conv.AwaitingConfirmation {
		if isAffirmative(lowerMessage) {
			finalReason := conv.Draft.Reason
			if strings.TrimSpace(finalReason) == "" {
				finalReason = "general consultation"
			}
			finalTime := normalizeTime(conv.Draft.Time)
			sourceWA := "whatsapp"
			finalApp := Appointment{
				TenantID:               tenant.ID,
				PatientID:              session.PatientID,
				PatientName:            conv.Draft.PatientName,
				PatientPhone:           session.PatientPhone,
				Doctor:                 conv.Draft.Doctor,
				Date:                   conv.Draft.Date,
				Time:                   finalTime,
				Reason:                 finalReason,
				Status:                 StatusConfirmed,
				Source:                 &sourceWA,
				ChatwootConversationID: &session.ChatwootConversationID,
			}

			bErr := db.Transaction(func(tx *gorm.DB) error {
				_, err := bookSlot(tx, tenant.ID, &finalApp)
				return err
			})
			if bErr != nil {
				slots, _ := availableTimeSlots(tenant.ID, conv.Draft.Date, conv.Draft.Doctor)
				session.SelectedTime = ""
				session.WorkflowState = "time"
				session.AwaitingConfirmation = false
				return fmt.Sprintf("Sorry, %s at %s is not available (%v). Available slots: %s",
					conv.Draft.Date, finalTime, bErr, strings.Join(slots, ", ")), nil
			}

			session.SelectedAppointmentID = &finalApp.ID
			session.WorkflowState = "completed"
			session.AwaitingConfirmation = false
			return fmt.Sprintf("✅ Appointment Confirmed!\nProvider: %s\nDoctor: %s\nDate: %s at %s\nReference: BK-%d-%d\nPatient: %s",
				tenant.Name, finalApp.Doctor, finalApp.Date, finalApp.Time, tenant.ID, finalApp.ID, finalApp.PatientName), &finalApp
		}

		if isNegative(lowerMessage) {
			session.AwaitingConfirmation = false
			session.WorkflowState = "initial"
			return "No problem! What would you like to change (doctor, date, or time)?", nil
		}
		conv.AwaitingConfirmation = false
	}

	if conv.Expecting != "" {
		if handled, reply := applyExpectedAnswer(&conv, messageText, tenantDoctors); handled {
			session.SelectedDoctorName = conv.Draft.Doctor
			session.SelectedDate = conv.Draft.Date
			session.SelectedTime = conv.Draft.Time
			session.SelectedReason = conv.Draft.Reason
			session.WorkflowState = conv.Expecting

			if conv.Draft.PatientName != "" && conv.Draft.Doctor != "" &&
				isValidDate(conv.Draft.Date) && isValidTime(conv.Draft.Time) {
				if conv.Draft.Reason == "" {
					session.WorkflowState = "reason"
					return fmt.Sprintf("Got it! What is the reason for your visit with %s on %s at %s?",
						conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time), nil
				}
				session.AwaitingConfirmation = true
				session.WorkflowState = "awaiting_confirmation"
				return fmt.Sprintf("Please confirm your booking at %s:\nDoctor: %s\nDate: %s\nTime: %s\nReason: %s\nReply 'Yes' to confirm.",
					tenant.Name, conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, conv.Draft.Reason), nil
			}
			return reply, nil
		}
	}

	docList := strings.Join(tenantDoctors, ", ")
	if docList != "" {
		session.WorkflowState = "doctor"
		return fmt.Sprintf("Welcome to %s! Which doctor would you like to book with? Our available doctors are: %s.", tenant.Name, docList), nil
	}
	return fmt.Sprintf("Welcome to %s! How can I help you with your appointment today?", tenant.Name), nil
}
