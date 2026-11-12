package main

import (
	"time"
)

// Tenant represents a logical clinic / organization in a multitenant setup.
// All users, appointments and chat sessions are scoped to a tenant.
type Tenant struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Slug             string    `gorm:"uniqueIndex;size:100;not null" json:"slug"` // e.g. "default", "clinic-a"
	Name             string    `gorm:"size:255;not null" json:"name"`
	EmergencyNumber  string    `gorm:"size:100" json:"emergency_number,omitempty"`
	EmergencyMessage string    `gorm:"size:500" json:"emergency_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type User struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Email              string    `gorm:"uniqueIndex:idx_user_email_tenant;size:255;not null" json:"email"`
	Password           string    `json:"-"`
	Role               string    `gorm:"size:100;default:staff" json:"role"`
	Status             string    `gorm:"size:50;default:active" json:"status"` // "active", "disabled"
	MustChangePassword bool      `gorm:"default:false" json:"must_change_password"`
	TokenVersion       int       `gorm:"default:1" json:"token_version"`
	TenantID           uint      `gorm:"index" json:"tenant_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	RolePlatformAdmin = "platform_admin"
	RoleTenantAdmin   = "tenant_admin"
	RoleStaff         = "staff"
)

type Appointment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	// See comment on User.TenantID – nullable at DB level, required in app logic.
	TenantID    uint      `gorm:"index" json:"tenant_id"`
	PatientName string    `gorm:"size:255;not null" json:"patient_name"`
	Doctor      string    `gorm:"size:255;not null" json:"doctor"`
	DoctorID    *uint     `gorm:"index" json:"doctor_id,omitempty"`
	Date        string    `gorm:"size:10;not null" json:"date"`
	Time        string    `gorm:"size:5;not null" json:"time"`
	Reason      string    `gorm:"size:500" json:"reason"`
	Status      string    `gorm:"size:50;default:pending" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Appointment status constants
const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
	StatusCompleted = "completed"
	StatusNoShow    = "no_show"
)

func isValidStatus(status string) bool {
	switch status {
	case StatusPending, StatusConfirmed, StatusCancelled, StatusCompleted, StatusNoShow:
		return true
	default:
		return false
	}
}

type Doctor struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"index" json:"tenant_id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Specialty   string    `gorm:"size:255" json:"specialty"`
	Bio         string    `gorm:"size:500" json:"bio"`
	WorkDays    string    `gorm:"size:100;default:'Mon,Tue,Wed,Thu,Fri,Sat'" json:"work_days"`
	WorkStart   string    `gorm:"size:5;default:'09:00'" json:"work_start"`
	WorkEnd     string    `gorm:"size:5;default:'17:00'" json:"work_end"`
	SlotMinutes int       `gorm:"default:30" json:"slot_minutes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	At          time.Time `gorm:"not null;index" json:"at"`
	ActorUserID uint      `gorm:"index" json:"actor_user_id"`
	ActorRole   string    `gorm:"size:100;index" json:"actor_role"`
	TenantID    uint      `gorm:"index" json:"tenant_id"` // 0 if platform-wide
	Action      string    `gorm:"size:100;not null;index" json:"action"`
	Entity      string    `gorm:"size:100;not null;index" json:"entity"`
	EntityID    uint      `gorm:"index" json:"entity_id"`
	IP          string    `gorm:"size:100" json:"ip"`
}

type ReasonOption struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index" json:"tenant_id"`
	Label     string    `gorm:"size:255;not null" json:"label"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

type ChatResponse struct {
	Message           string       `json:"message,omitempty"`
	Reply             string       `json:"reply,omitempty"`
	SessionID         string       `json:"session_id,omitempty"`
	Appointment       *Appointment `json:"appointment,omitempty"`
	Emergency         bool         `json:"emergency,omitempty"`
	EmergencyCategory string       `json:"emergency_category,omitempty"`
	EmergencyNumber   string       `json:"emergency_number,omitempty"`
}

// In-memory conversation state per session
// This is non-persistent and intended for short-lived conversational context.
// For production, consider Redis or a DB table to persist context across replicas.
type ConversationState struct {
	LastUserMessage       string
	LastAIMessage         string
	Draft                 Appointment
	AwaitingConfirmation  bool
	Expecting             string // "name" | "doctor" | "date" | "time" | "reason" | ""
	UpdatedAt             time.Time
}
