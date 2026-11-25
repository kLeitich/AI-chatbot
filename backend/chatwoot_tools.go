package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// ToolCallDefinition defines an OpenAI/Groq-compatible tool schema for the AI agent
func GetBookingToolsSchema() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "search_providers",
				"description": "Search participating clinics/hospitals in the cross-hospital marketplace when the patient explicitly asks for alternative providers or other clinics.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query":    map[string]interface{}{"type": "string", "description": "Hospital name or keyword"},
						"location": map[string]interface{}{"type": "string", "description": "City, neighborhood, or branch location"},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_provider_details",
				"description": "Get current hospital or clinic details including address, phone, business hours, cancellation policy, and rescheduling policy.",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "search_doctors",
				"description": "Search doctors by specialty, name, or branch within the clinic (or across marketplace providers if cross_provider is true).",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"specialty":      map[string]interface{}{"type": "string", "description": "Medical specialty e.g. Cardiology, Pediatrics, Dental"},
						"query":          map[string]interface{}{"type": "string", "description": "Doctor name search query"},
						"branch_name":    map[string]interface{}{"type": "string", "description": "Branch or facility name"},
						"cross_provider": map[string]interface{}{"type": "boolean", "description": "Set true ONLY if the patient explicitly asks for doctors at other hospitals"},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_doctor_details",
				"description": "Get detailed info about a doctor including bio, specialty, working days/hours, branch, and consultation fee.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctor_name": map[string]interface{}{"type": "string", "description": "Doctor's name"},
					},
					"required": []string{"doctor_name"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "search_specialties",
				"description": "List all available medical specialties offered at this clinic or hospital.",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_provider_branches",
				"description": "Get the list of physical branches/locations for the healthcare provider.",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_available_slots",
				"description": "Retrieve real-time available appointment time slots for a specific doctor and date (YYYY-MM-DD).",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctor_name": map[string]interface{}{"type": "string", "description": "Name of the doctor"},
						"date":        map[string]interface{}{"type": "string", "description": "Date in YYYY-MM-DD format"},
					},
					"required": []string{"doctor_name", "date"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_consultation_fee",
				"description": "Check the consultation fee for a specific doctor.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctor_name": map[string]interface{}{"type": "string", "description": "Name of the doctor"},
					},
					"required": []string{"doctor_name"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "create_appointment",
				"description": "Book and persist a verified appointment in the database after confirming doctor, date, time, patient name, and reason with the patient.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"doctor_name":  map[string]interface{}{"type": "string", "description": "Exact doctor name"},
						"date":         map[string]interface{}{"type": "string", "description": "YYYY-MM-DD date"},
						"time":         map[string]interface{}{"type": "string", "description": "24-hour HH:MM time"},
						"patient_name": map[string]interface{}{"type": "string", "description": "Full name of the patient"},
						"reason":       map[string]interface{}{"type": "string", "description": "Reason for the visit"},
						"tenant_slug":  map[string]interface{}{"type": "string", "description": "Optional target clinic slug if booking across marketplace"},
					},
					"required": []string{"doctor_name", "date", "time", "patient_name"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_patient_appointments",
				"description": "List existing upcoming appointments for the current patient by their phone number.",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "cancel_appointment",
				"description": "Cancel an existing patient appointment by appointment ID.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"appointment_id": map[string]interface{}{"type": "integer", "description": "Appointment reference ID"},
						"reason":         map[string]interface{}{"type": "string", "description": "Optional reason for cancelling"},
					},
					"required": []string{"appointment_id"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "reschedule_appointment",
				"description": "Reschedule an existing patient appointment to a new date (YYYY-MM-DD) and time (HH:MM).",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"appointment_id": map[string]interface{}{"type": "integer", "description": "Appointment reference ID"},
						"new_date":       map[string]interface{}{"type": "string", "description": "New YYYY-MM-DD date"},
						"new_time":       map[string]interface{}{"type": "string", "description": "New HH:MM time"},
					},
					"required": []string{"appointment_id", "new_date", "new_time"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_provider_directions",
				"description": "Get physical address and location instructions for the clinic or specific branch.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"branch_name": map[string]interface{}{"type": "string", "description": "Optional branch name"},
					},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "transfer_to_human",
				"description": "Transfer/hand over the conversation immediately to a live clinic receptionist or human agent.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"reason": map[string]interface{}{"type": "string", "description": "Concise summary of why human help is requested"},
					},
					"required": []string{"reason"},
				},
			},
		},
	}
}

// ToolExecutionResult captures side effects of a tool execution
type ToolExecutionResult struct {
	Payload            string
	ConfirmedBooking   *Appointment
	HandoverTriggered  bool
	HandoverReasonText string
}

// ExecuteBookingTool runs a validated, tenant-scoped backend tool invocation
func ExecuteBookingTool(tenant Tenant, session *ChatSession, toolName string, rawArgs string) ToolExecutionResult {
	var args map[string]interface{}
	if strings.TrimSpace(rawArgs) != "" {
		_ = json.Unmarshal([]byte(rawArgs), &args)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	getString := func(key string) string {
		if v, ok := args[key].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	getBool := func(key string) bool {
		if v, ok := args[key].(bool); ok {
			return v
		}
		return false
	}
	getUint := func(key string) uint {
		if v, ok := args[key].(float64); ok && v > 0 {
			return uint(v)
		}
		return 0
	}

	res := ToolExecutionResult{}
	toJSON := func(data interface{}) string {
		b, _ := json.Marshal(data)
		return string(b)
	}

	switch toolName {
	case "search_providers":
		q := strings.ToLower(getString("query"))
		loc := strings.ToLower(getString("location"))
		var providers []Tenant
		db.Where("marketplace_enabled = ?", true).Find(&providers)
		var out []map[string]interface{}
		for _, p := range providers {
			if q != "" && !strings.Contains(strings.ToLower(p.Name), q) && !strings.Contains(strings.ToLower(p.Description), q) {
				continue
			}
			if loc != "" && !strings.Contains(strings.ToLower(p.Address), loc) {
				continue
			}
			out = append(out, map[string]interface{}{
				"slug":           p.Slug,
				"name":           p.Name,
				"provider_type":  p.ProviderType,
				"address":        p.Address,
				"phone":          p.Phone,
				"business_hours": p.BusinessHours,
			})
		}
		res.Payload = toJSON(map[string]interface{}{"providers": out, "count": len(out)})

	case "get_provider_details":
		res.Payload = toJSON(map[string]interface{}{
			"name":                tenant.Name,
			"provider_type":       tenant.ProviderType,
			"description":         tenant.Description,
			"address":             tenant.Address,
			"phone":               tenant.Phone,
			"whatsapp_number":     tenant.WhatsAppNumber,
			"business_hours":      tenant.BusinessHours,
			"cancellation_policy": tenant.CancellationPolicy,
			"rescheduling_policy": tenant.ReschedulingPolicy,
			"timezone":            tenant.Timezone,
		})

	case "search_doctors":
		specialty := strings.ToLower(getString("specialty"))
		queryStr := strings.ToLower(getString("query"))
		branchStr := strings.ToLower(getString("branch_name"))
		crossProvider := getBool("cross_provider")

		var docs []Doctor
		if crossProvider && tenant.MarketplaceEnabled {
			db.Joins("JOIN tenants ON tenants.id = doctors.tenant_id").
				Where("doctors.active = ? AND tenants.marketplace_enabled = ?", true, true).
				Find(&docs)
		} else {
			db.Where("tenant_id = ? AND active = ?", tenant.ID, true).Order("name ASC").Find(&docs)
		}

		var out []map[string]interface{}
		for _, d := range docs {
			if specialty != "" && !strings.Contains(strings.ToLower(d.Specialty), specialty) {
				continue
			}
			if queryStr != "" && !strings.Contains(strings.ToLower(d.Name), queryStr) {
				continue
			}
			if branchStr != "" && !strings.Contains(strings.ToLower(d.BranchName), branchStr) {
				continue
			}
			item := map[string]interface{}{
				"id":               d.ID,
				"name":             d.Name,
				"specialty":        d.Specialty,
				"branch":           d.BranchName,
				"consultation_fee": d.ConsultationFee,
				"work_days":        d.WorkDays,
				"work_start":       d.WorkStart,
				"work_end":         d.WorkEnd,
			}
			if d.TenantID != tenant.ID {
				var p Tenant
				if err := db.First(&p, d.TenantID).Error; err == nil {
					item["provider_name"] = p.Name
					item["provider_slug"] = p.Slug
				}
			}
			out = append(out, item)
		}
		res.Payload = toJSON(map[string]interface{}{"doctors": out, "count": len(out)})

	case "get_doctor_details", "get_consultation_fee":
		docName := getString("doctor_name")
		doc, err := resolveDoctor(tenant.ID, docName)
		if err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": err.Error()})
			break
		}
		res.Payload = toJSON(map[string]interface{}{
			"id":               doc.ID,
			"name":             doc.Name,
			"specialty":        doc.Specialty,
			"bio":              doc.Bio,
			"branch":           doc.BranchName,
			"consultation_fee": doc.ConsultationFee,
			"work_days":        doc.WorkDays,
			"work_start":       doc.WorkStart,
			"work_end":         doc.WorkEnd,
			"slot_minutes":     doc.SlotMinutes,
		})

	case "search_specialties":
		var docs []Doctor
		db.Where("tenant_id = ? AND active = ?", tenant.ID, true).Find(&docs)
		specSet := map[string]bool{}
		var specs []string
		for _, d := range docs {
			s := strings.TrimSpace(d.Specialty)
			if s != "" && !specSet[s] {
				specSet[s] = true
				specs = append(specs, s)
			}
		}
		res.Payload = toJSON(map[string]interface{}{"specialties": specs})

	case "get_provider_branches":
		var branches []Branch
		db.Where("tenant_id = ? AND active = ?", tenant.ID, true).Order("name ASC").Find(&branches)
		res.Payload = toJSON(map[string]interface{}{"branches": branches})

	case "get_available_slots":
		docName := getString("doctor_name")
		date := getString("date")
		slots, err := availableTimeSlots(tenant.ID, date, docName)
		if err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": err.Error()})
			break
		}
		res.Payload = toJSON(map[string]interface{}{
			"doctor":          docName,
			"date":            date,
			"available_slots": slots,
			"count":           len(slots),
		})

	case "create_appointment":
		docName := getString("doctor_name")
		date := getString("date")
		rawTime := getString("time")
		pName := getString("patient_name")
		reason := getString("reason")
		targetSlug := getString("tenant_slug")

		targetTenant := tenant
		if targetSlug != "" && targetSlug != tenant.Slug {
			var dest Tenant
			if err := db.Where("slug = ? AND marketplace_enabled = ?", targetSlug, true).First(&dest).Error; err != nil {
				res.Payload = toJSON(map[string]interface{}{"error": "Target provider not available in marketplace"})
				break
			}
			targetTenant = dest
		}

		normTime := normalizeTime(rawTime)
		if normTime == "" {
			normTime = rawTime
		}
		if reason == "" {
			reason = "general consultation"
		}
		if pName == "" {
			pName = session.PatientName
		}

		sourceWA := "whatsapp"
		app := Appointment{
			TenantID:               targetTenant.ID,
			PatientID:              session.PatientID,
			PatientName:            pName,
			PatientPhone:           session.PatientPhone,
			Doctor:                 docName,
			Date:                   date,
			Time:                   normTime,
			Reason:                 reason,
			Status:                 StatusConfirmed,
			Source:                 &sourceWA,
			ChatwootConversationID: &session.ChatwootConversationID,
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			_, bErr := bookSlot(tx, targetTenant.ID, &app)
			return bErr
		})
		if err != nil {
			var conflictErr *BookingConflictError
			if errors.As(err, &conflictErr) {
				res.Payload = toJSON(map[string]interface{}{
					"error":        conflictErr.Message,
					"alternatives": conflictErr.Alternatives,
				})
				break
			}
			res.Payload = toJSON(map[string]interface{}{"error": err.Error()})
			break
		}

		// Sync session state & record audit
		session.SelectedDoctorID = app.DoctorID
		session.SelectedDoctorName = app.Doctor
		session.SelectedDate = app.Date
		session.SelectedTime = app.Time
		session.SelectedReason = app.Reason
		session.SelectedAppointmentID = &app.ID
		session.WorkflowState = "completed"
		session.AwaitingConfirmation = false
		recordAuditLog(0, "ai_assistant", targetTenant.ID, "create", "appointment", app.ID, "chatwoot-webhook")

		res.ConfirmedBooking = &app
		res.Payload = toJSON(map[string]interface{}{
			"status":           "confirmed",
			"appointment_id":   app.ID,
			"reference":        fmt.Sprintf("BK-%d-%d", targetTenant.ID, app.ID),
			"provider_name":    targetTenant.Name,
			"doctor":           app.Doctor,
			"branch":           app.BranchName,
			"date":             app.Date,
			"time":             app.Time,
			"consultation_fee": app.ConsultationFee,
			"patient_name":     app.PatientName,
		})

	case "get_patient_appointments":
		if strings.TrimSpace(session.PatientPhone) == "" {
			res.Payload = toJSON(map[string]interface{}{"appointments": []interface{}{}, "note": "No phone number identified"})
			break
		}
		var appts []Appointment
		todayStr := nowInAppLocation().Format("2006-01-02")
		db.Where("tenant_id = ? AND patient_phone = ? AND date >= ? AND status <> ?",
			tenant.ID, session.PatientPhone, todayStr, StatusCancelled).
			Order("date ASC, time ASC").Find(&appts)

		res.Payload = toJSON(map[string]interface{}{"appointments": appts, "count": len(appts)})

	case "cancel_appointment":
		apptID := getUint("appointment_id")
		reason := getString("reason")
		if apptID == 0 {
			res.Payload = toJSON(map[string]interface{}{"error": "appointment_id is required"})
			break
		}
		var ap Appointment
		if err := db.Where("id = ? AND tenant_id = ?", apptID, tenant.ID).First(&ap).Error; err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": "Appointment not found for this clinic"})
			break
		}
		if session.PatientPhone != "" && ap.PatientPhone != "" && ap.PatientPhone != session.PatientPhone {
			res.Payload = toJSON(map[string]interface{}{"error": "Permission denied: phone number does not match patient record"})
			break
		}

		ap.Status = StatusCancelled
		if reason != "" {
			ap.CancellationReason = reason
		}
		if err := db.Save(&ap).Error; err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": "Failed to cancel appointment"})
			break
		}

		if tenant.GoogleCalendarEnabled && ap.GoogleEventID != "" {
			var docCalID string
			if ap.DoctorID != nil {
				var doc Doctor
				if err := db.First(&doc, *ap.DoctorID).Error; err == nil {
					docCalID = doc.GoogleCalendarID
				}
			}
			go func(evID, calID string) {
				_ = deleteGoogleCalendarEvent(tenant, evID, calID)
			}(ap.GoogleEventID, docCalID)
		}

		recordAuditLog(0, "ai_assistant", tenant.ID, "cancel", "appointment", ap.ID, "chatwoot-webhook")
		res.Payload = toJSON(map[string]interface{}{
			"status":         "cancelled",
			"appointment_id": ap.ID,
			"doctor":         ap.Doctor,
			"date":           ap.Date,
			"time":           ap.Time,
		})

	case "reschedule_appointment":
		apptID := getUint("appointment_id")
		newDate := getString("new_date")
		newTime := normalizeTime(getString("new_time"))
		if apptID == 0 || newDate == "" || newTime == "" {
			res.Payload = toJSON(map[string]interface{}{"error": "appointment_id, new_date, and new_time are required"})
			break
		}
		var ap Appointment
		if err := db.Where("id = ? AND tenant_id = ?", apptID, tenant.ID).First(&ap).Error; err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": "Appointment not found for this clinic"})
			break
		}
		if session.PatientPhone != "" && ap.PatientPhone != "" && ap.PatientPhone != session.PatientPhone {
			res.Payload = toJSON(map[string]interface{}{"error": "Permission denied: phone number does not match patient record"})
			break
		}

		ap.Date = newDate
		ap.Time = newTime
		err := db.Transaction(func(tx *gorm.DB) error {
			_, bErr := bookSlot(tx, tenant.ID, &ap)
			return bErr
		})
		if err != nil {
			res.Payload = toJSON(map[string]interface{}{"error": err.Error()})
			break
		}

		if tenant.GoogleCalendarEnabled {
			go func() {
				_ = syncAppointmentToGoogleCalendar(tenant, &ap)
			}()
		}

		recordAuditLog(0, "ai_assistant", tenant.ID, "reschedule", "appointment", ap.ID, "chatwoot-webhook")
		res.ConfirmedBooking = &ap
		res.Payload = toJSON(map[string]interface{}{
			"status":         "rescheduled",
			"appointment_id": ap.ID,
			"doctor":         ap.Doctor,
			"date":           ap.Date,
			"time":           ap.Time,
		})

	case "get_provider_directions":
		branchName := strings.ToLower(getString("branch_name"))
		var branches []Branch
		db.Where("tenant_id = ? AND active = ?", tenant.ID, true).Find(&branches)
		if branchName != "" {
			for _, b := range branches {
				if strings.Contains(strings.ToLower(b.Name), branchName) {
					res.Payload = toJSON(map[string]interface{}{"branch": b.Name, "address": b.Address, "phone": b.Phone})
					return res
				}
			}
		}
		res.Payload = toJSON(map[string]interface{}{
			"provider_name": tenant.Name,
			"main_address":  tenant.Address,
			"phone":         tenant.Phone,
			"branches":      branches,
		})

	case "transfer_to_human":
		reason := getString("reason")
		if reason == "" {
			reason = "Patient requested human assistance"
		}
		session.HumanHandover = true
		session.WorkflowState = "handed_off"
		session.HandoverReason = reason
		res.HandoverTriggered = true
		res.HandoverReasonText = reason
		res.Payload = toJSON(map[string]interface{}{"status": "handed_off", "reason": reason})

	default:
		res.Payload = toJSON(map[string]interface{}{"error": fmt.Sprintf("unknown tool: %s", toolName)})
		log.Printf("[CHATWOOT TOOL ERROR] Attempted unknown tool '%s'", toolName)
	}

	return res
}
