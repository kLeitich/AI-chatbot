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

	// Use session ID or generate a default one for conversation tracking
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Get conversation history
	conv := getConversation(sessionID)
	
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
		setConversation(sessionID, conv)
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
	setConversation(sessionID, conv)

	// Check if we now have all required fields
	updatedHasAll := conv.Draft.Doctor != "" && 
		conv.Draft.PatientName != "" && 
		isValidDate(conv.Draft.Date) && 
		isValidTime(conv.Draft.Time)

	// CRITICAL: If we have all required fields, complete booking immediately - don't ask again
	if updatedHasAll {
		// Check if reason is missing - ask for it before completing booking
		finalReason := choose(ap.Reason, conv.Draft.Reason)
		if strings.TrimSpace(finalReason) == "" {
			// We have all required fields except reason - ask for it ONLY
			reasonReply := fmt.Sprintf("Perfect! I have all the details. What is the reason for your appointment with %s on %s at %s?", 
				conv.Draft.Doctor, conv.Draft.Date, conv.Draft.Time)
			return c.JSON(fiber.Map{"reply": reasonReply})
		}

		// We have everything including reason - complete the booking immediately
		if strings.TrimSpace(finalReason) == "" {
			finalReason = "general consultation" // Default if still empty
		}
		
		// Normalize time to ensure it's in correct format
		finalTime := normalizeTime(conv.Draft.Time)
		
		finalApp := Appointment{
			PatientName: conv.Draft.PatientName,
			Doctor:      conv.Draft.Doctor,
			Date:        conv.Draft.Date,
			Time:        finalTime,
			Reason:      finalReason,
			Status:      "pending",
		}

		// Generate confirmation message
		reply = fmt.Sprintf("Perfect! I've booked your appointment with %s on %s at %s for %s. Thank you, %s!", 
			finalApp.Doctor, finalApp.Date, finalApp.Time, finalApp.Reason, finalApp.PatientName)

		if err := db.Create(&finalApp).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to create appointment")
		}
		// Clear conversation state after successful booking
		setConversation(sessionID, ConversationState{})
		return c.JSON(ChatResponse{Message: reply, Appointment: &finalApp})
	}

	if strings.TrimSpace(reply) == "" {
		reply = "Hi! I can help you book an appointment. Which doctor and date work for you?"
	}
	return c.JSON(fiber.Map{"reply": strings.TrimSpace(reply)})
}

func listAppointments(c *fiber.Ctx) error {
	var apps []Appointment
	if err := db.Order("created_at DESC").Find(&apps).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list appointments")
	}
	return c.JSON(apps)
}

func createAppointment(c *fiber.Ctx) error {
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
	if in.PatientName == "" || in.Doctor == "" || !isValidDate(in.Date) || !isValidTime(in.Time) {
		return fiber.NewError(fiber.StatusBadRequest, "patient, doctor, valid date and time required")
	}
	if err := db.Create(&in).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create")
	}
	return c.Status(fiber.StatusCreated).JSON(in)
}

func updateAppointment(c *fiber.Ctx) error {
	id := c.Params("id")
	var ap Appointment
	if err := db.First(&ap, id).Error; err != nil {
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
	id := c.Params("id")
	if err := db.Delete(&Appointment{}, id).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}
