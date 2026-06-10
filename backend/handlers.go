package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

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
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	// Use session ID or generate a default one for conversation tracking,
	// but namespace by tenant so sessions are isolated per tenant.
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	convKey := fmt.Sprintf("%s:%s", tenantSlug, sessionID)

	// Get conversation history
	conv := getConversation(convKey)
	messageText := strings.TrimSpace(req.Message)
	lowerMessage := strings.ToLower(messageText)

	if conv.AwaitingConfirmation {
		if isAffirmative(lowerMessage) {
			finalReason := conv.Draft.Reason
			if strings.TrimSpace(finalReason) == "" {
				finalReason = "general consultation"
			}
			finalTime := normalizeTime(conv.Draft.Time)
			finalApp := Appointment{
				PatientName: conv.Draft.PatientName,
				Doctor:      conv.Draft.Doctor,
				Date:        conv.Draft.Date,
				Time:        finalTime,
				Reason:      finalReason,
				Status:      "pending",
			}

			finalApp.TenantID = tenant.ID
			if err := db.Create(&finalApp).Error; err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, "failed to create appointment")
			}

			reply = fmt.Sprintf("Great! Your appointment with %s on %s at %s for %s is confirmed, %s.",
				finalApp.Doctor, finalApp.Date, finalApp.Time, finalApp.Reason, finalApp.PatientName)
			setConversation(convKey, ConversationState{})
			return c.JSON(ChatResponse{Message: reply, Appointment: &finalApp})
		}

		if isNegative(lowerMessage) && len(strings.Fields(lowerMessage)) <= 3 {
			reply = "No problem. What would you like to change?"
			conv.AwaitingConfirmation = false
			conv.LastAIMessage = reply
			setConversation(convKey, conv)
			return c.JSON(fiber.Map{"reply": reply})
		}

		// If the user provides more details instead of a simple yes/no,
		// clear the confirmation flag and continue parsing.
		conv.AwaitingConfirmation = false
	}

	ap, reply, err := AskForAppointmentFromMessage("", req.Message, conv)
	if err != nil {
		log.Printf("[Chat Error] %v", err)
	}

	// Update conversation state with partial information
	if ap.Doctor != "" || ap.PatientName != "" || ap.Date != "" || ap.Time != "" || ap.Reason != "" {
		conv.Draft.PatientName = choose(ap.PatientName, conv.Draft.PatientName)
		conv.Draft.Doctor = choose(ap.Doctor, conv.Draft.Doctor)
		conv.Draft.Date = choose(ap.Date, conv.Draft.Date)
		conv.Draft.Time = choose(ap.Time, conv.Draft.Time)
		conv.Draft.Reason = choose(ap.Reason, conv.Draft.Reason)
		conv.LastUserMessage = req.Message
		conv.LastAIMessage = reply
		setConversation(convKey, conv)
	}

	// Check if the current response has information to update the conversation state
	if strings.TrimSpace(ap.Reason) != "" {
		conv.Draft.Reason = ap.Reason
	}
	if strings.TrimSpace(ap.PatientName) != "" {
		conv.Draft.PatientName = ap.PatientName
	}
	if strings.TrimSpace(ap.Doctor) != "" {
		conv.Draft.Doctor = ap.Doctor
	}
	if strings.TrimSpace(ap.Date) != "" && isValidDate(ap.Date) {
		conv.Draft.Date = ap.Date
	}
	if strings.TrimSpace(ap.Time) != "" {
		// Normalize time before saving to ensure proper format
		normalizedTime := normalizeTime(ap.Time)
		if normalizedTime != "" && isValidTime(normalizedTime) {
			conv.Draft.Time = normalizedTime
			ap.Time = normalizedTime // Also update the appointment object
		}
	}
	setConversation(convKey, conv)

	// If the user provided doctor and date but not time, suggest available slots
	if conv.Draft.Doctor != "" && conv.Draft.Date != "" && conv.Draft.Time == "" {
		slots, err := availableTimeSlots(tenant.ID, conv.Draft.Date, conv.Draft.Doctor)
		if err == nil && len(slots) > 0 {
			reply = fmt.Sprintf("I found these available times with %s on %s: %s. Which one works best for you?", conv.Draft.Doctor, conv.Draft.Date, strings.Join(slots, ", "))
			setConversation(convKey, conv)
			return c.JSON(fiber.Map{"reply": reply})
		}
	}

	// Check if we now have all required fields
	updatedHasAll := conv.Draft.Doctor != "" &&
		conv.Draft.PatientName != "" &&
		isValidDate(conv.Draft.Date) &&
		isValidTime(conv.Draft.Time)

	if updatedHasAll {
		finalReason := choose(ap.Reason, conv.Draft.Reason)
		if strings.TrimSpace(finalReason) == "" {
			reasonReply := fmt.Sprintf("Perfect! I have all the details. What is the reason for your appointment with %s on %s at %s?",
				conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time)
			conv.LastAIMessage = reasonReply
			setConversation(convKey, conv)
			return c.JSON(fiber.Map{"reply": reasonReply})
		}

		confirmationReply := fmt.Sprintf("I have your appointment with %s on %s at %s for %s. Does that look right? Please reply with yes to confirm or no to change.",
			conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time, finalReason)
		conv.AwaitingConfirmation = true
		conv.LastAIMessage = confirmationReply
		setConversation(convKey, conv)
		return c.JSON(fiber.Map{"reply": confirmationReply})
	}

	if strings.TrimSpace(reply) == "" {
		reply = "Hi! I can help you book an appointment. Which doctor and date work for you?"
	}
	return c.JSON(fiber.Map{"reply": strings.TrimSpace(reply)})
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
		in.Status = "pending"
	}
	in.TenantID = tenantID
	if in.PatientName == "" || in.Doctor == "" || !isValidDate(in.Date) || !isValidTime(in.Time) {
		return fiber.NewError(fiber.StatusBadRequest, "patient, doctor, valid date and time required")
	}
	if err := db.Create(&in).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create")
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

	if in.Date != "" && !isValidDate(in.Date) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid date")
	}
	if in.Time != "" && !isValidTime(in.Time) {
		return fiber.NewError(fiber.StatusBadRequest, "invalid time")
	}

	ap.PatientName = choose(in.PatientName, ap.PatientName)
	ap.Doctor = choose(in.Doctor, ap.Doctor)
	ap.Date = choose(in.Date, ap.Date)
	ap.Time = choose(in.Time, ap.Time)
	ap.Reason = choose(in.Reason, ap.Reason)
	if in.Status != "" {
		ap.Status = in.Status
	}

	if err := db.Save(&ap).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update")
	}
	return c.JSON(ap)
}

func deleteAppointment(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	id := c.Params("id")
	if err := db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&Appointment{}).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type createDoctorRequest struct {
	Name      string `json:"name"`
	Specialty string `json:"specialty"`
	Bio       string `json:"bio"`
}

type createReasonRequest struct {
	Label string `json:"label"`
}

func listDoctors(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	var doctors []Doctor
	if err := db.Where("tenant_id = ?", tenant.ID).Order("name ASC").Find(&doctors).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list doctors")
	}
	return c.JSON(doctors)
}

func createDoctor(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	var req createDoctorRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "doctor name is required")
	}

	doctor := Doctor{
		TenantID:  tenantID,
		Name:      strings.TrimSpace(req.Name),
		Specialty: strings.TrimSpace(req.Specialty),
		Bio:       strings.TrimSpace(req.Bio),
	}

	if err := db.Create(&doctor).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create doctor")
	}
	return c.Status(fiber.StatusCreated).JSON(doctor)
}

func listReasons(c *fiber.Ctx) error {
	tenantSlug := c.Params("tenant")
	if tenantSlug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing tenant")
	}

	var tenant Tenant
	if err := db.Where("slug = ?", tenantSlug).First(&tenant).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	var reasons []ReasonOption
	if err := db.Where("tenant_id = ?", tenant.ID).Order("label ASC").Find(&reasons).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list reasons")
	}
	return c.JSON(reasons)
}

func createReason(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(uint)
	if !ok || tenantID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "tenant not found in token")
	}

	var req createReasonRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Label) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "reason label is required")
	}

	reason := ReasonOption{
		TenantID: tenantID,
		Label:    strings.TrimSpace(req.Label),
	}

	if err := db.Create(&reason).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create reason")
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
		return fiber.NewError(fiber.StatusNotFound, "tenant not found")
	}

	date := strings.TrimSpace(c.Query("date"))
	doctorName := strings.TrimSpace(c.Query("doctor"))
	if date == "" || !isValidDate(date) {
		return fiber.NewError(fiber.StatusBadRequest, "valid date is required")
	}

	slots, err := availableTimeSlots(tenant.ID, date, doctorName)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to compute availability")
	}

	return c.JSON(availabilityResponse{
		Date:           date,
		Doctor:         doctorName,
		AvailableTimes: slots,
	})
}

func availableTimeSlots(tenantID uint, date string, doctorName string) ([]string, error) {
	query := db.Where("tenant_id = ? AND date = ?", tenantID, date)
	if doctorName != "" {
		query = query.Where("doctor = ?", doctorName)
	}

	var booked []Appointment
	if err := query.Find(&booked).Error; err != nil {
		return nil, err
	}

	taken := map[string]bool{}
	for _, ap := range booked {
		if isValidTime(ap.Time) {
			taken[ap.Time] = true
		}
	}

	slots := []string{}
	for hour := 9; hour < 17; hour++ {
		for minute := 0; minute < 60; minute += 30 {
			timeStr := formatTwo(hour) + ":" + formatTwo(minute)
			if !taken[timeStr] {
				slots = append(slots, timeStr)
			}
		}
	}
	return slots, nil
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
