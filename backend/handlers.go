package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func chatHandler(c *fiber.Ctx) error {
	var req ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	prev := getConversation(req.SessionID)
	prompt := buildPrompt(req.Message, prev)
	extract, err := runOllama(prompt)

	// Prepare appointment from AI if available
	ap := Appointment{}
	if err == nil {
		ap = Appointment{
			PatientName: strings.TrimSpace(extract.PatientName),
			Doctor:      strings.TrimSpace(extract.Doctor),
			Date:        strings.TrimSpace(extract.Date),
			Time:        strings.TrimSpace(extract.Time),
			Reason:      strings.TrimSpace(extract.Reason),
			Status:      "pending",
		}
	}

	// Fallback to local parsing if AI failed or missing critical fields
	if ap.Date == "" || !isValidDate(ap.Date) || ap.Time == "" || !isValidTime(ap.Time) {
		if guess, ok := tryLocalParse(req.Message); ok {
			if ap.PatientName == "" {
				ap.PatientName = guess.PatientName
			}
			if ap.Doctor == "" {
				ap.Doctor = guess.Doctor
			}
			ap.Date = guess.Date
			ap.Time = guess.Time
		}
	}

	state := prev
	state.LastUserMessage = req.Message
	state.Draft = ap
	setConversation(req.SessionID, state)

	// validate extracted fields; only create if all valid
	if ap.PatientName == "" || ap.Doctor == "" || !isValidDate(ap.Date) || !isValidTime(ap.Time) {
		return c.Status(fiber.StatusOK).JSON(ChatResponse{Message: "I couldn't parse that. Could you rephrase with patient, doctor, date (YYYY-MM-DD), and time (HH:MM)?"})
	}

	if err := db.Create(&ap).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create appointment")
	}
	state.LastAIMessage = "Appointment booked"
	setConversation(req.SessionID, state)
	return c.JSON(ChatResponse{Message: "Appointment booked", Appointment: &ap})
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
	if in.Status == "" { in.Status = "pending" }
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
	// Allow edits; validate date/time when provided
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
