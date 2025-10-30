package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Remove duplicate type declarations here. models.go defines structs.

func chatHandler(c *fiber.Ctx) error {
	var req ChatRequest
	if err := c.BodyParser(&req); err != nil {
		fmt.Printf("[DEBUG] Invalid request body: %v\n", err)
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	userMsg := strings.TrimSpace(req.Message)
	if userMsg == "" {
		fmt.Println("[DEBUG] Empty message payload")
		return c.JSON(fiber.Map{"reply": "Please type a message to continue."})
	}
	fmt.Printf("\n[DEBUG] Incoming chat: session=%s message=%q\n", req.SessionID, userMsg)

	ap, reply, err := AskForAppointmentFromMessage("", userMsg, req.SessionID)
	if err != nil {
		fmt.Printf("[DEBUG] AI error/fallback: %v\n", err)
		return c.JSON(fiber.Map{"reply": "Sorry, I had trouble processing that. Could you try again?"})
	}

	if ap.Doctor != "" && ap.Date != "" && ap.Time != "" && ap.PatientName != "" {
		fmt.Printf("[DEBUG] Booking appointment: %+v\n", ap)
		if err := db.Create(&ap).Error; err != nil {
			fmt.Printf("[DEBUG] DB create failed: %v\n", err)
			return fiber.NewError(fiber.StatusInternalServerError, "failed to save appointment")
		}
		fmt.Println("[DEBUG] Appointment booked and returned to frontend")
		return c.JSON(ChatResponse{
			Message:     "Appointment booked successfully.",
			Reply:       reply,
			Appointment: &ap,
		})
	}

	fmt.Printf("[DEBUG] Fallback reply returned: %s\n", strings.TrimSpace(reply))
	return c.JSON(fiber.Map{"reply": strings.TrimSpace(reply)})
}

// -------------------- CRUD Handlers --------------------

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
		return fiber.NewError(fiber.StatusNotFound, "appointment not found")
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
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete appointment")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// -------------------- Helpers --------------------

func choose(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}
