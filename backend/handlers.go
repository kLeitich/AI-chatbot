package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var sourceRegex = regexp.MustCompile(`^[a-z0-9_-]{1,30}$`)

func sanitizeSource(raw string) *string {
	s := strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	res := b.String()
	if len(res) > 30 {
		res = res[:30]
	}
	if len(res) == 0 {
		return nil
	}
	return &res
}

func chatHandler(c *fiber.Ctx) error {
	var req ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	// Resolve tenant from URL for conversation + appointment scoping
	tenantSlug := c.Params("tenant")
	if strings.TrimSpace(tenantSlug) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	if !tenant.BookingLinkEnabled {
		return sendError(c, fiber.StatusForbidden, ErrCodeBookingDisabled, "Online booking is currently disabled for this clinic.")
	}

	// Validate session ID or generate a cryptographically secure 128-bit ID
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" || !isValidSessionID(sessionID) {
		var err error
		sessionID, err = generateSessionID()
		if err != nil {
			log.Printf("[Session Error] Failed to generate secure session ID: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "failed to generate secure session ID")
		}
	}

	convKey := fmt.Sprintf("%s:%s", tenantSlug, sessionID)

	// Serialise handling per convKey so read-modify-write cannot race
	unlockSession := acquireSessionLock(convKey)
	defer unlockSession()

	// Get conversation history
	conv := getConversation(convKey)
	messageText := strings.TrimSpace(req.Message)
	if messageText == "" {
		return fiber.NewError(fiber.StatusBadRequest, "message cannot be empty")
	}
	if len(messageText) > 1000 {
		messageText = messageText[:1000]
	}

	// Seed draft context deterministically on first message only
	if conv.LastUserMessage == "" && req.Context != nil {
		if req.Context.DoctorID != nil {
			var doc Doctor
			if err := db.Where("id = ? AND tenant_id = ? AND active = ?", *req.Context.DoctorID, tenant.ID, true).First(&doc).Error; err == nil {
				conv.Draft.Doctor = doc.Name
			}
		}
		if req.Context.ReasonID != nil {
			var r ReasonOption
			if err := db.Where("id = ? AND tenant_id = ? AND active = ?", *req.Context.ReasonID, tenant.ID, true).First(&r).Error; err == nil {
				conv.Draft.Reason = r.Label
			}
		}
		if req.Context.Source != nil {
			conv.Draft.Source = sanitizeSource(*req.Context.Source)
		}
	}

	// Emergency detector runs deterministic checks BEFORE any LLM call or state modification
	if isEmergency, category := DetectEmergency(messageText); isEmergency {
		LogEmergencyDetected(tenantSlug, sessionID, category)
		// Clear session state and do not save draft
		setConversation(convKey, ConversationState{})
		emergencyNum := tenant.EmergencyNumber
		if strings.TrimSpace(emergencyNum) == "" {
			emergencyNum = "999"
		}
		return c.JSON(ChatResponse{
			Reply:             EmergencyResponse(tenant, category),
			SessionID:         sessionID,
			Emergency:         true,
			EmergencyCategory: string(category),
			EmergencyNumber:   emergencyNum,
		})
	}

	lowerMessage := strings.ToLower(messageText)
	tenantDoctors := tenantDoctorNames(tenant.ID)

	// respond helper saves LastUserMessage, LastAIMessage, Expecting, AwaitingConfirmation,
	// persists state via setConversation, and returns a JSON ChatResponse with SessionID.
	respond := func(replyMsg string, expecting string, awaitingConfirm bool, finalApp *Appointment) error {
		conv.LastUserMessage = messageText
		conv.LastAIMessage = replyMsg
		conv.Expecting = expecting
		conv.AwaitingConfirmation = awaitingConfirm
		setConversation(convKey, conv)

		if finalApp != nil {
			return c.JSON(ChatResponse{
				Message:     replyMsg,
				Appointment: finalApp,
				SessionID:   sessionID,
			})
		}
		return c.JSON(ChatResponse{
			Reply:     replyMsg,
			SessionID: sessionID,
		})
	}

	if conv.AwaitingConfirmation {
		if isAffirmative(lowerMessage) {
			finalReason := conv.Draft.Reason
			if strings.TrimSpace(finalReason) == "" {
				finalReason = "general consultation"
			}
			finalTime := normalizeTime(conv.Draft.Time)
			finalApp := Appointment{
				TenantID:    tenant.ID,
				PatientName: conv.Draft.PatientName,
				Doctor:      conv.Draft.Doctor,
				Date:        conv.Draft.Date,
				Time:        finalTime,
				Reason:      finalReason,
				Status:      StatusPending,
				Source:      conv.Draft.Source,
			}

			// Validate at confirm time:
			// Re-check date is not in the past, time is on grid, and slot is still free
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
				altMsg := ""
				if len(slots) > 0 {
					altMsg = fmt.Sprintf(" Available times on %s with %s are: %s.", conv.Draft.Date, conv.Draft.Doctor, strings.Join(slots, ", "))
				} else {
					altMsg = fmt.Sprintf(" %s has no available slots on %s.", conv.Draft.Doctor, conv.Draft.Date)
				}
				reply := fmt.Sprintf("Sorry, that slot is no longer available.%s What time would you prefer instead?", altMsg)
				conv.Draft.Time = ""
				return respond(reply, "time", false, nil)
			}

			var alts []string
			bErr := db.Transaction(func(tx *gorm.DB) error {
				var err error
				alts, err = bookSlot(tx, tenant.ID, &finalApp)
				return err
			})
			if bErr != nil {
				altMsg := ""
				if len(alts) > 0 {
					altMsg = fmt.Sprintf(" Available times on %s with %s are: %s.", finalApp.Date, finalApp.Doctor, strings.Join(alts, ", "))
				}
				reply := fmt.Sprintf("Sorry, that slot is no longer available.%s What time would you prefer instead?", altMsg)
				conv.Draft.Time = ""
				return respond(reply, "time", false, nil)
			}

			reply := fmt.Sprintf("Great! Your appointment with %s on %s at %s for %s is confirmed, %s.",
				finalApp.Doctor, finalApp.Date, finalApp.Time, finalApp.Reason, finalApp.PatientName)
			// Clear conversation state on successful booking
			conv = ConversationState{}
			setConversation(convKey, conv)

			if tenant.GoogleCalendarEnabled {
				go func() {
					_ = syncAppointmentToGoogleCalendar(tenant, &finalApp)
				}()
			}

			return c.JSON(ChatResponse{Message: reply, Appointment: &finalApp, SessionID: sessionID})
		}

		if isNegative(lowerMessage) && len(strings.Fields(lowerMessage)) <= 3 {
			reply := "No problem. What would you like to change?"
			return respond(reply, "", false, nil)
		}

		// If user provides more details instead of yes/no, clear confirmation flag and continue
		conv.AwaitingConfirmation = false
	}

	// 1. applyExpectedAnswer runs FIRST: if handled, return immediately
	if conv.Expecting != "" {
		if handled, expectedReply := applyExpectedAnswer(&conv, messageText, tenantDoctors); handled {
			// Check if all required fields are now present
			if conv.Draft.PatientName != "" && conv.Draft.Doctor != "" &&
				isValidDate(conv.Draft.Date) && isValidTime(conv.Draft.Time) {
				if conv.Draft.Reason == "" {
					reasonQ := fmt.Sprintf("Perfect! I have all the details. What is the reason for your appointment with %s on %s at %s?",
						conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time)
					return respond(reasonQ, "reason", false, nil)
				}
				confirmQ := fmt.Sprintf("I have your appointment with %s on %s at %s for %s. Does that look right? Please reply with yes to confirm or no to change.",
					conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, conv.Draft.Reason)
				return respond(confirmQ, "", true, nil)
			}
			return respond(expectedReply, conv.Expecting, false, nil)
		}
	}

	// 2. LLM path only runs if applyExpectedAnswer didn't handle the message
	ap, _, err := AskForAppointmentFromMessage("", messageText, conv)
	if err != nil {
		log.Printf("[Chat Info] LLM fallback notice: %v", err)
	}

	// Any extracted doctor is validated against tenantDoctorNames; unknown doctors are dropped with the "couldn't find" reply
	if ap.Doctor != "" {
		if matchedDoc, ok := matchDoctor(ap.Doctor, tenantDoctors); ok {
			conv.Draft.Doctor = matchedDoc
		} else {
			docList := strings.Join(tenantDoctors, ", ")
			reply := fmt.Sprintf("I couldn't find a doctor called %s. Our doctors are: %s.", ap.Doctor, docList)
			return respond(reply, "doctor", false, nil)
		}
	}

	// Draft merge fills fields from the extracted appointment (valid dates/times only)
	if ap.PatientName != "" && isValidPatientNameCandidate(ap.PatientName) {
		conv.Draft.PatientName = formatPatientName(ap.PatientName)
	}
	if ap.Date != "" && isValidDate(ap.Date) {
		conv.Draft.Date = ap.Date
	}
	if ap.Time != "" {
		normalizedTime := normalizeTime(ap.Time)
		if isValidTime(normalizedTime) {
			conv.Draft.Time = normalizedTime
		}
	}
	if ap.Reason != "" {
		conv.Draft.Reason = strings.TrimSpace(ap.Reason)
	}

	// If the user provided doctor and date but not time, suggest available slots
	if conv.Draft.Doctor != "" && conv.Draft.Date != "" && conv.Draft.Time == "" {
		slots, err := availableTimeSlots(tenant.ID, conv.Draft.Date, conv.Draft.Doctor)
		if err == nil && len(slots) > 0 {
			reply := fmt.Sprintf("I found these available times with %s on %s: %s. Which one works best for you?",
				conv.Draft.Doctor, conv.Draft.Date, strings.Join(slots, ", "))
			return respond(reply, "time", false, nil)
		}
	}

	// When a patient states a time directly in chat, validate it against availability before moving to confirmation
	if conv.Draft.Doctor != "" && isValidDate(conv.Draft.Date) && isValidTime(conv.Draft.Time) {
		slots, err := availableTimeSlots(tenant.ID, conv.Draft.Date, conv.Draft.Doctor)
		slotAvailable := false
		if err == nil {
			for _, s := range slots {
				if s == conv.Draft.Time {
					slotAvailable = true
					break
				}
			}
		}
		if !slotAvailable {
			altMsg := ""
			if err != nil {
				altMsg = fmt.Sprintf(" (%v)", err)
			} else if len(slots) > 0 {
				altMsg = fmt.Sprintf(" Available times on %s with %s are: %s.", conv.Draft.Date, conv.Draft.Doctor, strings.Join(slots, ", "))
			} else {
				altMsg = fmt.Sprintf(" %s has no available slots on %s.", conv.Draft.Doctor, conv.Draft.Date)
			}
			reply := fmt.Sprintf("The time %s is not available.%s Which time works best for you?", conv.Draft.Time, altMsg)
			conv.Draft.Time = ""
			return respond(reply, "time", false, nil)
		}
	}

	// Check if we now have all required fields
	updatedHasAll := conv.Draft.Doctor != "" &&
		conv.Draft.PatientName != "" &&
		isValidDate(conv.Draft.Date) &&
		isValidTime(conv.Draft.Time)

	if updatedHasAll {
		if strings.TrimSpace(conv.Draft.Reason) == "" {
			reasonReply := fmt.Sprintf("Perfect! I have all the details. What is the reason for your appointment with %s on %s at %s?",
				conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time)
			return respond(reasonReply, "reason", false, nil)
		}

		confirmationReply := fmt.Sprintf("I have your appointment with %s on %s at %s for %s. Does that look right? Please reply with yes to confirm or no to change.",
			conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, conv.Draft.Reason)
		return respond(confirmationReply, "", true, nil)
	}

	// Slot questions come from nextSlotQuestion (Go), not the LLM
	nextSlot, nextQ := nextSlotQuestion(conv.Draft, tenantDoctors)
	return respond(nextQ, nextSlot, false, nil)
}

func listAppointments(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	var apps []Appointment
	if err := db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&apps).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list appointments")
	}
	return c.JSON(apps)
}

func createAppointment(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	var in Appointment
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	in.PatientName = strings.TrimSpace(in.PatientName)
	in.Doctor = strings.TrimSpace(in.Doctor)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Status == "" {
		in.Status = StatusPending
	}
	if !isValidStatus(in.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid appointment status")
	}
	in.TenantID = tenantID
	if in.PatientName == "" || in.Doctor == "" || !isValidDate(in.Date) || !isValidTime(in.Time) {
		return fiber.NewError(fiber.StatusBadRequest, "patient, doctor, valid date and time required")
	}

	var alts []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var bErr error
		alts, bErr = bookSlot(tx, tenantID, &in)
		return bErr
	})
	if err != nil {
		if len(alts) > 3 {
			alts = alts[:3]
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message":      err.Error(),
			"code":         ErrCodeSlotTaken,
			"alternatives": alts,
		})
	}
	if in.TenantID > 0 {
		var tenant Tenant
		if err := db.First(&tenant, in.TenantID).Error; err == nil && tenant.GoogleCalendarEnabled {
			go func() {
				_ = syncAppointmentToGoogleCalendar(tenant, &in)
			}()
		}
	}

	return c.Status(fiber.StatusCreated).JSON(in)
}

func updateAppointment(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	id := c.Params("id")
	var ap Appointment
	if err := db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&ap).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	var in Appointment
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}

	isCancelOnly := in.Status == StatusCancelled && in.Date == "" && in.Time == "" && in.Doctor == "" && in.PatientName == "" && in.Reason == ""
	if isCancelOnly {
		if !hasPermission(c, PermAppointmentsCancel) && !hasPermission(c, PermAppointmentsEdit) {
			return fiber.NewError(fiber.StatusForbidden, "permission denied: missing 'appointments.cancel'")
		}
	} else {
		if !hasPermission(c, PermAppointmentsEdit) {
			return fiber.NewError(fiber.StatusForbidden, "permission denied: missing 'appointments.edit'")
		}
	}

	if in.Status != "" {
		if !isValidStatus(in.Status) {
			return fiber.NewError(fiber.StatusBadRequest, "invalid appointment status")
		}
		ap.Status = in.Status
	}

	slotChanged := false
	if in.Date != "" && in.Date != ap.Date {
		if !isValidDate(in.Date) {
			return fiber.NewError(fiber.StatusBadRequest, "invalid date")
		}
		ap.Date = in.Date
		slotChanged = true
	}
	if in.Time != "" && in.Time != ap.Time {
		if !isValidTime(in.Time) {
			return fiber.NewError(fiber.StatusBadRequest, "invalid time")
		}
		ap.Time = in.Time
		slotChanged = true
	}
	if in.Doctor != "" && in.Doctor != ap.Doctor {
		ap.Doctor = in.Doctor
		slotChanged = true
	}

	ap.PatientName = choose(in.PatientName, ap.PatientName)
	ap.Reason = choose(in.Reason, ap.Reason)

	if slotChanged {
		var alts []string
		err := db.Transaction(func(tx *gorm.DB) error {
			var bErr error
			alts, bErr = bookSlot(tx, tenantID, &ap)
			return bErr
		})
		if err != nil {
			if len(alts) > 3 {
				alts = alts[:3]
			}
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message":      err.Error(),
				"code":         ErrCodeSlotTaken,
				"alternatives": alts,
			})
		}
	} else {
		if err := db.Save(&ap).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to update")
		}
	}

	if ap.Status == StatusCancelled && ap.GoogleEventID != "" {
		var tenant Tenant
		if err := db.First(&tenant, tenantID).Error; err == nil {
			go func(eID string) {
				_ = deleteGoogleCalendarEvent(tenant, eID, "")
			}(ap.GoogleEventID)
		}
	}

	return c.JSON(ap)
}

func deleteAppointment(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	id := c.Params("id")
	var ap Appointment
	if err := db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&ap).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "appointment not found")
	}

	if ap.GoogleEventID != "" {
		var tenant Tenant
		if tErr := db.First(&tenant, tenantID).Error; tErr == nil {
			go func(eID string) {
				_ = deleteGoogleCalendarEvent(tenant, eID, "")
			}(ap.GoogleEventID)
		}
	}

	if err := db.Delete(&ap).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type createDoctorRequest struct {
	Name             string `json:"name"`
	Specialty        string `json:"specialty"`
	Bio              string `json:"bio"`
	WorkDays         string `json:"work_days"`
	WorkStart        string `json:"work_start"`
	WorkEnd          string `json:"work_end"`
	SlotMinutes      int    `json:"slot_minutes"`
	GoogleCalendarID string `json:"google_calendar_id"`
}

type createReasonRequest struct {
	Label string `json:"label"`
}

func listDoctors(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	query := db.Where("tenant_id = ?", tenant.ID)
	// Public routes only show active doctors; admin routes show all
	if !strings.Contains(c.Path(), "/admin/") {
		query = query.Where("active = ?", true)
	}

	var doctors []Doctor
	if err := query.Order("name ASC").Find(&doctors).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to list doctors")
	}
	return c.JSON(doctors)
}

type patchDoctorRequest struct {
	Name             *string `json:"name"`
	Specialty        *string `json:"specialty"`
	Bio              *string `json:"bio"`
	WorkDays         *string `json:"work_days"`
	WorkStart        *string `json:"work_start"`
	WorkEnd          *string `json:"work_end"`
	SlotMinutes      *int    `json:"slot_minutes"`
	Active           *bool   `json:"active"`
	GoogleCalendarID *string `json:"google_calendar_id"`
}

type patchReasonRequest struct {
	Name   *string `json:"name"`
	Label  *string `json:"label"`
	Active *bool   `json:"active"`
}

func parseHHMMToMinutes(s string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, false
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func patchDoctor(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}

	docID, err := c.ParamsInt("id")
	if err != nil || docID <= 0 {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid doctor id")
	}

	var doc Doctor
	if err := db.Where("id = ? AND tenant_id = ?", docID, tenantID).First(&doc).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "doctor not found")
	}

	var req patchDoctorRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid request body")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "name cannot be empty")
		}
		doc.Name = name
	}
	if req.Specialty != nil {
		doc.Specialty = strings.TrimSpace(*req.Specialty)
	}
	if req.Bio != nil {
		doc.Bio = strings.TrimSpace(*req.Bio)
	}
	if req.WorkDays != nil {
		wd := strings.TrimSpace(*req.WorkDays)
		if wd == "" {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "work_days cannot be empty")
		}
		doc.WorkDays = wd
	}

	newStart := doc.WorkStart
	newEnd := doc.WorkEnd
	if req.WorkStart != nil {
		newStart = strings.TrimSpace(*req.WorkStart)
	}
	if req.WorkEnd != nil {
		newEnd = strings.TrimSpace(*req.WorkEnd)
	}

	startMins, okStart := parseHHMMToMinutes(newStart)
	endMins, okEnd := parseHHMMToMinutes(newEnd)
	if !okStart || !okEnd {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid working hours format (HH:MM required)")
	}
	if startMins >= endMins {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "work_start must be earlier than work_end")
	}
	doc.WorkStart = newStart
	doc.WorkEnd = newEnd

	if req.SlotMinutes != nil {
		if *req.SlotMinutes < 10 || *req.SlotMinutes > 120 {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "slot_minutes must be between 10 and 120")
		}
		doc.SlotMinutes = *req.SlotMinutes
	}

	if req.Active != nil {
		doc.Active = *req.Active
	}

	if req.GoogleCalendarID != nil {
		doc.GoogleCalendarID = strings.TrimSpace(*req.GoogleCalendarID)
	}

	if err := db.Save(&doc).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to update doctor")
	}

	return c.JSON(doc)
}

func createDoctor(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}

	var req createDoctorRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid request body")
	}
	if strings.TrimSpace(req.Name) == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "doctor name is required")
	}

	workDays := strings.TrimSpace(req.WorkDays)
	if workDays == "" {
		workDays = "Mon,Tue,Wed,Thu,Fri,Sat"
	}
	workStart := strings.TrimSpace(req.WorkStart)
	if workStart == "" {
		workStart = "09:00"
	}
	workEnd := strings.TrimSpace(req.WorkEnd)
	if workEnd == "" {
		workEnd = "17:00"
	}
	slotMins := req.SlotMinutes
	if slotMins <= 0 {
		slotMins = 30
	}

	startMins, okStart := parseHHMMToMinutes(workStart)
	endMins, okEnd := parseHHMMToMinutes(workEnd)
	if !okStart || !okEnd {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid working hours format (HH:MM required)")
	}
	if startMins >= endMins {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "work_start must be earlier than work_end")
	}
	if slotMins < 10 || slotMins > 120 {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "slot_minutes must be between 10 and 120")
	}

	doctor := Doctor{
		TenantID:    tenantID,
		Name:        strings.TrimSpace(req.Name),
		Specialty:   strings.TrimSpace(req.Specialty),
		Bio:         strings.TrimSpace(req.Bio),
		WorkDays:    workDays,
		WorkStart:   workStart,
		WorkEnd:     workEnd,
		SlotMinutes:      slotMins,
		Active:           true,
		GoogleCalendarID: strings.TrimSpace(req.GoogleCalendarID),
	}

	if err := db.Create(&doctor).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to create doctor")
	}
	return c.Status(fiber.StatusCreated).JSON(doctor)
}

func listReasons(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	query := db.Where("tenant_id = ?", tenant.ID)
	// Public routes only show active reasons; admin routes show all
	if !strings.Contains(c.Path(), "/admin/") {
		query = query.Where("active = ?", true)
	}

	var reasons []ReasonOption
	if err := query.Order("label ASC").Find(&reasons).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to list reasons")
	}
	return c.JSON(reasons)
}

func patchReason(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}

	reasonID, err := c.ParamsInt("id")
	if err != nil || reasonID <= 0 {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid reason id")
	}

	var reason ReasonOption
	if err := db.Where("id = ? AND tenant_id = ?", reasonID, tenantID).First(&reason).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "reason not found")
	}

	var req patchReasonRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid request body")
	}

	if req.Label != nil {
		lbl := strings.TrimSpace(*req.Label)
		if lbl == "" {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "label cannot be empty")
		}
		reason.Label = lbl
	} else if req.Name != nil {
		lbl := strings.TrimSpace(*req.Name)
		if lbl == "" {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "name cannot be empty")
		}
		reason.Label = lbl
	}

	if req.Active != nil {
		reason.Active = *req.Active
	}

	if err := db.Save(&reason).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to update reason")
	}

	return c.JSON(reason)
}

func createReason(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return sendError(c, fiber.StatusUnauthorized, ErrCodeTokenExpired, "tenant not found in token")
	}

	var req createReasonRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "invalid request body")
	}
	if strings.TrimSpace(req.Label) == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "reason label is required")
	}

	reason := ReasonOption{
		TenantID: tenantID,
		Label:    strings.TrimSpace(req.Label),
		Active:   true,
	}

	if err := db.Create(&reason).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to create reason")
	}
	return c.Status(fiber.StatusCreated).JSON(reason)
}

type availabilityResponse struct {
	Date          string   `json:"date"`
	Doctor        string   `json:"doctor,omitempty"`
	AvailableTimes []string `json:"available_times"`
}

func getAvailability(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	date := strings.TrimSpace(c.Query("date"))
	doctorName := strings.TrimSpace(c.Query("doctor"))
	if date == "" || !isValidDate(date) {
		return fiber.NewError(fiber.StatusBadRequest, "valid date is required (YYYY-MM-DD)")
	}
	if doctorName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "doctor parameter is required to check availability")
	}

	slots, err := availableTimeSlots(tenant.ID, date, doctorName)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(availabilityResponse{
		Date:           date,
		Doctor:         doctorName,
		AvailableTimes: slots,
	})
}

func listCompanies(c *fiber.Ctx) error {
	var companies []Tenant
	if err := db.Order("created_at DESC").Find(&companies).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list companies")
	}
	return c.JSON(companies)
}

func listUsers(c *fiber.Ctx) error {
	var users []User
	if err := db.Order("created_at DESC").Find(&users).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list users")
	}
	return c.JSON(users)
}

func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}

type TenantSettingsRequest struct {
	EmergencyNumber     *string   `json:"emergency_number"`
	EmergencyMessage    *string   `json:"emergency_message"`
	BookingLinkEnabled  *bool     `json:"booking_link_enabled"`
	AllowedEmbedOrigins *[]string `json:"allowed_embed_origins"`
	PrivacyURL          *string   `json:"privacy_url"`

	// Chatwoot & WhatsApp
	ChatwootEnabled       *bool   `json:"chatwoot_enabled"`
	ChatwootBaseURL       *string `json:"chatwoot_base_url"`
	ChatwootAPIToken      *string `json:"chatwoot_api_token"`
	ChatwootAccountID     *int    `json:"chatwoot_account_id"`
	ChatwootInboxID       *int    `json:"chatwoot_inbox_id"`
	ChatwootWebhookSecret *string `json:"chatwoot_webhook_secret"`

	// Google Calendar
	GoogleCalendarEnabled *bool   `json:"google_calendar_enabled"`
	GoogleCalendarID      *string `json:"google_calendar_id"`
	GoogleCalendarToken   *string `json:"google_calendar_token"`
	GoogleClientID        *string `json:"google_client_id"`
	GoogleClientSecret    *string `json:"google_client_secret"`

	// Clerk SSO
	ClerkEnabled        *bool   `json:"clerk_enabled"`
	ClerkPublishableKey *string `json:"clerk_publishable_key"`
	ClerkSecretKey      *string `json:"clerk_secret_key"`
}

type TenantSettingsResponse struct {
	EmergencyNumber     string   `json:"emergency_number"`
	EmergencyMessage    string   `json:"emergency_message"`
	BookingLinkEnabled  bool     `json:"booking_link_enabled"`
	AllowedEmbedOrigins []string `json:"allowed_embed_origins"`
	PrivacyURL          string   `json:"privacy_url,omitempty"`

	// Chatwoot & WhatsApp
	ChatwootEnabled       bool   `json:"chatwoot_enabled"`
	ChatwootBaseURL       string `json:"chatwoot_base_url,omitempty"`
	ChatwootAPIToken      string `json:"chatwoot_api_token,omitempty"`
	ChatwootAccountID     int    `json:"chatwoot_account_id,omitempty"`
	ChatwootInboxID       int    `json:"chatwoot_inbox_id,omitempty"`
	ChatwootWebhookSecret string `json:"chatwoot_webhook_secret,omitempty"`

	// Google Calendar
	GoogleCalendarEnabled bool   `json:"google_calendar_enabled"`
	GoogleCalendarID      string `json:"google_calendar_id,omitempty"`
	GoogleCalendarToken   string `json:"google_calendar_token,omitempty"`
	GoogleClientID        string `json:"google_client_id,omitempty"`
	GoogleClientSecret    string `json:"google_client_secret,omitempty"`

	// Clerk SSO
	ClerkEnabled        bool   `json:"clerk_enabled"`
	ClerkPublishableKey string `json:"clerk_publishable_key,omitempty"`
	ClerkSecretKey      string `json:"clerk_secret_key,omitempty"`
}

type TenantPublicConfigResponse struct {
	Name                string   `json:"name"`
	Slug                string   `json:"slug"`
	BookingEnabled      bool     `json:"booking_enabled"`
	EmergencyNumber     string   `json:"emergency_number"`
	PrivacyURL          string   `json:"privacy_url"`
	AllowedEmbedOrigins []string `json:"allowed_embed_origins"`
	ChatwootEnabled     bool     `json:"chatwoot_enabled"`
	ChatwootBaseURL     string   `json:"chatwoot_base_url,omitempty"`
	ChatwootInboxID     int      `json:"chatwoot_inbox_id,omitempty"`
	ClerkEnabled        bool     `json:"clerk_enabled"`
	ClerkPublishableKey string   `json:"clerk_publishable_key,omitempty"`
}

var emergencyNumberRegex = regexp.MustCompile(`^[0-9+ ]{0,20}$`)

func stripControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isValidDomainLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}
	for i, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		if r == '-' && i > 0 && i < len(label)-1 {
			continue
		}
		return false
	}
	return true
}

func validateOrigin(raw string, isDev bool) (string, error) {
	orig := strings.ToLower(strings.TrimSpace(raw))
	if orig == "" {
		return "", errors.New("origin cannot be empty")
	}

	// Must not have trailing slash, query, or path
	if strings.HasSuffix(orig, "/") || strings.Contains(orig, "?") || strings.Contains(orig, "#") {
		return "", errors.New("origin must not contain paths, queries, fragments or trailing slashes")
	}

	// In development only: http://localhost[:port]
	if strings.HasPrefix(orig, "http://localhost") {
		if !isDev {
			return "", errors.New("http://localhost is only allowed in development environment")
		}
		rest := strings.TrimPrefix(orig, "http://localhost")
		if rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", errors.New("invalid localhost origin")
			}
			portStr := rest[1:]
			port, err := strconv.Atoi(portStr)
			if err != nil || port <= 0 || port > 65535 {
				return "", errors.New("invalid port number on localhost")
			}
		}
		return orig, nil
	}

	if !strings.HasPrefix(orig, "https://") {
		return "", errors.New("origin must use https://")
	}

	hostPart := strings.TrimPrefix(orig, "https://")
	if hostPart == "" {
		return "", errors.New("missing host in origin")
	}

	if strings.Contains(hostPart, "/") {
		return "", errors.New("origin must not contain a path")
	}

	host := hostPart
	portStr := ""
	if idx := strings.LastIndex(hostPart, ":"); idx != -1 {
		host = hostPart[:idx]
		portStr = hostPart[idx+1:]
		if portStr == "" {
			return "", errors.New("invalid port specification")
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return "", errors.New("invalid port number")
		}
	}

	if strings.Contains(host, "*") {
		if !strings.HasPrefix(host, "*.") {
			return "", errors.New("wildcard '*' is only allowed as the leading label, e.g. https://*.example.com")
		}
		if strings.Count(host, "*") > 1 {
			return "", errors.New("multiple wildcards are not allowed")
		}
		remainingDomain := host[2:]
		if remainingDomain == "" || strings.Contains(remainingDomain, "*") || strings.HasPrefix(remainingDomain, ".") || strings.HasSuffix(remainingDomain, ".") {
			return "", errors.New("invalid wildcard host pattern")
		}
		labels := strings.Split(remainingDomain, ".")
		for _, label := range labels {
			if !isValidDomainLabel(label) {
				return "", fmt.Errorf("invalid domain label '%s'", label)
			}
		}
	} else {
		labels := strings.Split(host, ".")
		for _, label := range labels {
			if !isValidDomainLabel(label) {
				return "", fmt.Errorf("invalid domain label '%s'", label)
			}
		}
	}

	if portStr != "" {
		return fmt.Sprintf("https://%s:%s", host, portStr), nil
	}
	return fmt.Sprintf("https://%s", host), nil
}

func getTenantSettingsHandler(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	return c.JSON(TenantSettingsResponse{
		EmergencyNumber:     tenant.EmergencyNumber,
		EmergencyMessage:    tenant.EmergencyMessage,
		BookingLinkEnabled:  tenant.BookingLinkEnabled,
		AllowedEmbedOrigins: tenant.GetAllowedEmbedOrigins(),
		PrivacyURL:          tenant.PrivacyURL,

		ChatwootEnabled:       tenant.ChatwootEnabled,
		ChatwootBaseURL:       tenant.ChatwootBaseURL,
		ChatwootAPIToken:      tenant.ChatwootAPIToken,
		ChatwootAccountID:     tenant.ChatwootAccountID,
		ChatwootInboxID:       tenant.ChatwootInboxID,
		ChatwootWebhookSecret: tenant.ChatwootWebhookSecret,

		GoogleCalendarEnabled: tenant.GoogleCalendarEnabled,
		GoogleCalendarID:      tenant.GoogleCalendarID,
		GoogleCalendarToken:   tenant.GoogleCalendarToken,
		GoogleClientID:        tenant.GoogleClientID,
		GoogleClientSecret:    tenant.GoogleClientSecret,

		ClerkEnabled:        tenant.ClerkEnabled,
		ClerkPublishableKey: tenant.ClerkPublishableKey,
		ClerkSecretKey:      tenant.ClerkSecretKey,
	})
}

func updateTenantSettingsHandler(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	// Reject unknown JSON fields with 400 VALIDATION_ERROR
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()

	var req TenantSettingsRequest
	if err := dec.Decode(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, fmt.Sprintf("invalid settings request: %v", err))
	}

	if req.EmergencyNumber != nil {
		cleanNum := strings.TrimSpace(*req.EmergencyNumber)
		if len(cleanNum) > 20 || !emergencyNumberRegex.MatchString(cleanNum) {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "emergency_number may only contain digits, '+', and spaces, max 20 characters")
		}
		tenant.EmergencyNumber = cleanNum
	}

	if req.EmergencyMessage != nil {
		cleanMsg := stripControlChars(*req.EmergencyMessage)
		if len(cleanMsg) > 500 {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "emergency_message must be at most 500 characters")
		}
		tenant.EmergencyMessage = cleanMsg
	}

	if req.BookingLinkEnabled != nil {
		tenant.BookingLinkEnabled = *req.BookingLinkEnabled
	}

	if req.PrivacyURL != nil {
		pURL := strings.TrimSpace(*req.PrivacyURL)
		if pURL != "" {
			if len(pURL) > 300 {
				return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "privacy_url must be at most 300 characters")
			}
			parsed, err := url.Parse(pURL)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
				return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "privacy_url must be a valid https URL")
			}
		}
		tenant.PrivacyURL = pURL
	}

	if req.AllowedEmbedOrigins != nil {
		isDev := isDevelopmentEnv()
		seen := make(map[string]bool)
		deduped := make([]string, 0, len(*req.AllowedEmbedOrigins))

		for _, raw := range *req.AllowedEmbedOrigins {
			norm, err := validateOrigin(raw, isDev)
			if err != nil {
				return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, fmt.Sprintf("invalid allowed_embed_origin: %v", err))
			}
			if !seen[norm] {
				seen[norm] = true
				deduped = append(deduped, norm)
			}
		}

		if len(deduped) > 10 {
			return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "allowed_embed_origins exceeds maximum limit of 10")
		}

		tenant.SetAllowedEmbedOrigins(deduped)
	}

	// Chatwoot & WhatsApp settings
	if req.ChatwootEnabled != nil {
		tenant.ChatwootEnabled = *req.ChatwootEnabled
	}
	if req.ChatwootBaseURL != nil {
		tenant.ChatwootBaseURL = strings.TrimSpace(*req.ChatwootBaseURL)
	}
	if req.ChatwootAPIToken != nil {
		tenant.ChatwootAPIToken = strings.TrimSpace(*req.ChatwootAPIToken)
	}
	if req.ChatwootAccountID != nil {
		tenant.ChatwootAccountID = *req.ChatwootAccountID
	}
	if req.ChatwootInboxID != nil {
		tenant.ChatwootInboxID = *req.ChatwootInboxID
	}
	if req.ChatwootWebhookSecret != nil {
		tenant.ChatwootWebhookSecret = strings.TrimSpace(*req.ChatwootWebhookSecret)
	}

	// Google Calendar settings
	if req.GoogleCalendarEnabled != nil {
		tenant.GoogleCalendarEnabled = *req.GoogleCalendarEnabled
	}
	if req.GoogleCalendarID != nil {
		tenant.GoogleCalendarID = strings.TrimSpace(*req.GoogleCalendarID)
	}
	if req.GoogleCalendarToken != nil {
		tenant.GoogleCalendarToken = strings.TrimSpace(*req.GoogleCalendarToken)
	}
	if req.GoogleClientID != nil {
		tenant.GoogleClientID = strings.TrimSpace(*req.GoogleClientID)
	}
	if req.GoogleClientSecret != nil {
		tenant.GoogleClientSecret = strings.TrimSpace(*req.GoogleClientSecret)
	}

	// Clerk SSO settings
	if req.ClerkEnabled != nil {
		tenant.ClerkEnabled = *req.ClerkEnabled
	}
	if req.ClerkPublishableKey != nil {
		tenant.ClerkPublishableKey = strings.TrimSpace(*req.ClerkPublishableKey)
	}
	if req.ClerkSecretKey != nil {
		tenant.ClerkSecretKey = strings.TrimSpace(*req.ClerkSecretKey)
	}

	if err := db.Save(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusInternalServerError, ErrCodeServerError, "failed to update settings")
	}

	userID, _ := c.Locals("user_id").(uint)
	role, _ := c.Locals("role").(string)
	recordAuditLog(userID, role, tenant.ID, "update", "settings", tenant.ID, c.IP())

	return c.JSON(TenantSettingsResponse{
		EmergencyNumber:     tenant.EmergencyNumber,
		EmergencyMessage:    tenant.EmergencyMessage,
		BookingLinkEnabled:  tenant.BookingLinkEnabled,
		AllowedEmbedOrigins: tenant.GetAllowedEmbedOrigins(),
		PrivacyURL:          tenant.PrivacyURL,

		ChatwootEnabled:       tenant.ChatwootEnabled,
		ChatwootBaseURL:       tenant.ChatwootBaseURL,
		ChatwootAPIToken:      tenant.ChatwootAPIToken,
		ChatwootAccountID:     tenant.ChatwootAccountID,
		ChatwootInboxID:       tenant.ChatwootInboxID,
		ChatwootWebhookSecret: tenant.ChatwootWebhookSecret,

		GoogleCalendarEnabled: tenant.GoogleCalendarEnabled,
		GoogleCalendarID:      tenant.GoogleCalendarID,
		GoogleCalendarToken:   tenant.GoogleCalendarToken,
		GoogleClientID:        tenant.GoogleClientID,
		GoogleClientSecret:    tenant.GoogleClientSecret,

		ClerkEnabled:        tenant.ClerkEnabled,
		ClerkPublishableKey: tenant.ClerkPublishableKey,
		ClerkSecretKey:      tenant.ClerkSecretKey,
	})
}

func getTenantPublicConfigHandler(c *fiber.Ctx) error {
	tenantSlug := strings.TrimSpace(c.Params("tenant"))
	if tenantSlug == "" {
		return sendError(c, fiber.StatusBadRequest, ErrCodeValidationError, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return sendError(c, fiber.StatusNotFound, ErrCodeNotFound, "tenant not found")
	}

	emergNum := tenant.EmergencyNumber
	if strings.TrimSpace(emergNum) == "" {
		emergNum = "999"
	}

	return c.JSON(TenantPublicConfigResponse{
		Name:                tenant.Name,
		Slug:                tenant.Slug,
		BookingEnabled:      tenant.BookingLinkEnabled,
		EmergencyNumber:     emergNum,
		PrivacyURL:          tenant.PrivacyURL,
		AllowedEmbedOrigins: tenant.GetAllowedEmbedOrigins(),
		ChatwootEnabled:     tenant.ChatwootEnabled,
		ChatwootBaseURL:     tenant.ChatwootBaseURL,
		ChatwootInboxID:     tenant.ChatwootInboxID,
		ClerkEnabled:        tenant.ClerkEnabled,
		ClerkPublishableKey: tenant.ClerkPublishableKey,
	})
}
