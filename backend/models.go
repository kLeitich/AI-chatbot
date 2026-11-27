package main

import (
	"encoding/json"
	"time"
)

// Tenant represents a logical clinic / organization in a multitenant setup.
// All users, appointments and chat sessions are scoped to a tenant.
type Tenant struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Slug                string    `gorm:"uniqueIndex;size:100;not null" json:"slug"` // e.g. "default", "clinic-a"
	Name                string    `gorm:"size:255;not null" json:"name"`
	EmergencyNumber     string    `gorm:"size:100" json:"emergency_number,omitempty"`
	EmergencyMessage    string    `gorm:"size:500" json:"emergency_message,omitempty"`
	BookingLinkEnabled  bool      `gorm:"default:true" json:"booking_link_enabled"`
	AllowedEmbedOrigins string    `gorm:"type:text;default:'[]'" json:"allowed_embed_origins"`
	PrivacyURL          string    `gorm:"size:300" json:"privacy_url,omitempty"`

	// Chatwoot & WhatsApp Integration
	ChatwootEnabled       bool   `gorm:"default:false" json:"chatwoot_enabled"`
	ChatwootBaseURL       string `gorm:"size:255" json:"chatwoot_base_url,omitempty"`
	ChatwootAPIToken      string `gorm:"size:255" json:"chatwoot_api_token,omitempty"`
	ChatwootAccountID     int    `json:"chatwoot_account_id,omitempty"`
	ChatwootInboxID       int    `json:"chatwoot_inbox_id,omitempty"`
	ChatwootWebhookSecret string `gorm:"size:255" json:"chatwoot_webhook_secret,omitempty"`

	// Provider Metadata & Omnichannel Configuration
	ProviderType          string `gorm:"size:100;default:'clinic'" json:"provider_type"` // "hospital", "clinic", "medical_center", "specialist_practice"
	Description           string `gorm:"size:1000" json:"description,omitempty"`
	LogoURL               string `gorm:"size:500" json:"logo_url,omitempty"`
	Address               string `gorm:"size:500" json:"address,omitempty"`
	Phone                 string `gorm:"size:50" json:"phone,omitempty"`
	WhatsAppNumber        string `gorm:"size:50" json:"whatsapp_number,omitempty"`
	BusinessHours         string `gorm:"size:500" json:"business_hours,omitempty"`
	CancellationPolicy    string `gorm:"size:1000" json:"cancellation_policy,omitempty"`
	ReschedulingPolicy    string `gorm:"size:1000" json:"rescheduling_policy,omitempty"`
	Timezone              string `gorm:"size:100;default:'Africa/Nairobi'" json:"timezone,omitempty"`
	AIReceptionistEnabled bool   `gorm:"default:true" json:"ai_receptionist_enabled"`
	MarketplaceEnabled    bool   `gorm:"default:false" json:"marketplace_enabled"`
	HandoffKeywords       string `gorm:"size:255;default:'human,agent,receptionist,speak to someone'" json:"handoff_keywords,omitempty"`

	// Google Calendar Integration
	GoogleCalendarEnabled bool   `gorm:"default:false" json:"google_calendar_enabled"`
	GoogleCalendarID      string `gorm:"size:255" json:"google_calendar_id,omitempty"`
	GoogleCalendarToken   string `gorm:"type:text" json:"google_calendar_token,omitempty"`
	GoogleClientID        string `gorm:"size:255" json:"google_client_id,omitempty"`
	GoogleClientSecret    string `gorm:"size:255" json:"google_client_secret,omitempty"`

	// Clerk SSO Integration
	ClerkEnabled        bool   `gorm:"default:false" json:"clerk_enabled"`
	ClerkPublishableKey string `gorm:"size:255" json:"clerk_publishable_key,omitempty"`
	ClerkSecretKey      string `gorm:"size:255" json:"clerk_secret_key,omitempty"`

	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (t *Tenant) GetAllowedEmbedOrigins() []string {
	if t.AllowedEmbedOrigins == "" || t.AllowedEmbedOrigins == "[]" {
		return []string{}
	}
	var origins []string
	if err := json.Unmarshal([]byte(t.AllowedEmbedOrigins), &origins); err != nil {
		return []string{}
	}
	return origins
}

func (t *Tenant) SetAllowedEmbedOrigins(origins []string) {
	if origins == nil {
		origins = []string{}
	}
	data, _ := json.Marshal(origins)
	t.AllowedEmbedOrigins = string(data)
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
	ClerkUserID        string    `gorm:"index;size:255" json:"clerk_user_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	RolePlatformAdmin = "platform_admin"
	RoleTenantAdmin   = "tenant_admin"
	RoleStaff         = "staff"
)

type Appointment struct {
	ID                     uint      `gorm:"primaryKey" json:"id"`
	TenantID               uint      `gorm:"index" json:"tenant_id"`
	PatientID              *uint     `gorm:"index" json:"patient_id,omitempty"`
	PatientName            string    `gorm:"size:255;not null" json:"patient_name"`
	PatientPhone           string    `gorm:"size:50" json:"patient_phone,omitempty"`
	PatientEmail           string    `gorm:"size:255" json:"patient_email,omitempty"`
	Doctor                 string    `gorm:"size:255;not null" json:"doctor"`
	DoctorID               *uint     `gorm:"index" json:"doctor_id,omitempty"`
	BranchID               *uint     `gorm:"index" json:"branch_id,omitempty"`
	BranchName             string    `gorm:"size:255" json:"branch_name,omitempty"`
	Date                   string    `gorm:"size:10;not null" json:"date"`
	Time                   string    `gorm:"size:5;not null" json:"time"`
	Reason                 string    `gorm:"size:500" json:"reason"`
	Status                 string    `gorm:"size:50;default:pending" json:"status"`
	ConsultationFee        float64   `json:"consultation_fee,omitempty"`
	CancellationReason     string    `gorm:"size:500" json:"cancellation_reason,omitempty"`
	Source                 *string   `gorm:"size:30" json:"source,omitempty"`
	GoogleEventID          string    `gorm:"size:255" json:"google_event_id,omitempty"`
	ChatwootConversationID *int      `json:"chatwoot_conversation_id,omitempty"`
	ChatwootMessageID      *int      `json:"chatwoot_message_id,omitempty"`
	Reminder24hSent        bool      `gorm:"column:reminder_24h_sent;default:false" json:"reminder_24h_sent"`
	Reminder3hSent         bool      `gorm:"column:reminder_3h_sent;default:false" json:"reminder_3h_sent"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
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
	ID               uint      `gorm:"primaryKey" json:"id"`
	TenantID         uint      `gorm:"index" json:"tenant_id"`
	Name             string    `gorm:"size:255;not null" json:"name"`
	Specialty        string    `gorm:"size:255" json:"specialty"`
	Bio              string    `gorm:"size:500" json:"bio"`
	WorkDays         string    `gorm:"size:100;default:'Mon,Tue,Wed,Thu,Fri,Sat'" json:"work_days"`
	WorkStart        string    `gorm:"size:5;default:'09:00'" json:"work_start"`
	WorkEnd          string    `gorm:"size:5;default:'17:00'" json:"work_end"`
	SlotMinutes      int       `gorm:"default:30" json:"slot_minutes"`
	Active           bool      `gorm:"default:true" json:"active"`
	ConsultationFee  float64   `gorm:"default:0" json:"consultation_fee"`
	BranchID         *uint     `gorm:"index" json:"branch_id,omitempty"`
	BranchName       string    `gorm:"size:255" json:"branch_name,omitempty"`
	GoogleCalendarID string    `gorm:"size:255" json:"google_calendar_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Branch represents a physical hospital/clinic branch or facility location
type Branch struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Address   string    `gorm:"size:500" json:"address,omitempty"`
	Phone     string    `gorm:"size:50" json:"phone,omitempty"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Patient represents an identified patient record linked to phone and Chatwoot contact
type Patient struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TenantID          uint      `gorm:"index:idx_patient_phone_tenant;not null" json:"tenant_id"`
	Name              string    `gorm:"size:255;not null" json:"name"`
	Phone             string    `gorm:"index:idx_patient_phone_tenant;size:50;not null" json:"phone"`
	Email             string    `gorm:"size:255" json:"email,omitempty"`
	ChatwootContactID *int      `gorm:"index" json:"chatwoot_contact_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ChatSession represents persistent conversation state across messages and server restarts
type ChatSession struct {
	ID                     uint      `gorm:"primaryKey" json:"id"`
	SessionID              string    `gorm:"uniqueIndex;size:120;not null" json:"session_id"`
	TenantID               uint      `gorm:"index;not null" json:"tenant_id"`
	ProviderID             uint      `gorm:"index" json:"provider_id"`
	PatientID              *uint     `gorm:"index" json:"patient_id,omitempty"`
	PatientPhone           string    `gorm:"size:50" json:"patient_phone,omitempty"`
	PatientName            string    `gorm:"size:255" json:"patient_name,omitempty"`
	ChatwootAccountID      int       `json:"chatwoot_account_id,omitempty"`
	ChatwootInboxID        int       `gorm:"index" json:"chatwoot_inbox_id,omitempty"`
	ChatwootConversationID int       `gorm:"index" json:"chatwoot_conversation_id,omitempty"`
	ChatwootContactID      int       `json:"chatwoot_contact_id,omitempty"`
	CurrentIntent          string    `gorm:"size:100" json:"current_intent,omitempty"`
	SelectedDoctorID       *uint     `json:"selected_doctor_id,omitempty"`
	SelectedDoctorName     string    `gorm:"size:255" json:"selected_doctor_name,omitempty"`
	SelectedBranchID       *uint     `json:"selected_branch_id,omitempty"`
	SelectedBranchName     string    `gorm:"size:255" json:"selected_branch_name,omitempty"`
	SelectedDate           string    `gorm:"size:20" json:"selected_date,omitempty"`
	SelectedTime           string    `gorm:"size:20" json:"selected_time,omitempty"`
	SelectedReason         string    `gorm:"size:500" json:"selected_reason,omitempty"`
	SelectedAppointmentID  *uint     `json:"selected_appointment_id,omitempty"`
	WorkflowState          string    `gorm:"size:50;default:'initial'" json:"workflow_state"`
	AwaitingConfirmation   bool      `gorm:"default:false" json:"awaiting_confirmation"`
	AIEnabled              bool      `gorm:"default:true" json:"ai_enabled"`
	HumanHandover          bool      `gorm:"default:false" json:"human_handover"`
	HandoverReason         string    `gorm:"size:255" json:"handover_reason,omitempty"`
	ConversationSummary    string    `gorm:"type:text" json:"conversation_summary,omitempty"`
	LastUserMessage        string    `gorm:"type:text" json:"last_user_message,omitempty"`
	LastAIMessage          string    `gorm:"type:text" json:"last_ai_message,omitempty"`
	LastProcessedMessageID int       `json:"last_processed_message_id,omitempty"`
	UpdatedAt              time.Time `json:"updated_at"`
	CreatedAt              time.Time `json:"created_at"`
}

// ChatwootProcessedMessage stores idempotency keys for webhook deduplication
type ChatwootProcessedMessage struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	MessageID      int       `gorm:"uniqueIndex:idx_chatwoot_msg;not null" json:"message_id"`
	ConversationID int       `gorm:"index" json:"conversation_id"`
	InboxID        int       `gorm:"index" json:"inbox_id"`
	TenantID       uint      `gorm:"index" json:"tenant_id"`
	ProcessedAt    time.Time `gorm:"not null" json:"processed_at"`
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
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Email     string    `gorm:"index;size:255;not null" json:"email"`
	TokenHash string    `gorm:"uniqueIndex;size:64;not null" json:"token_hash"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	Used      bool      `gorm:"default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatContext struct {
	DoctorID *uint   `json:"doctor_id"`
	ReasonID *uint   `json:"reason_id"`
	Source   *string `json:"source"`
}

type ChatRequest struct {
	Message   string       `json:"message"`
	SessionID string       `json:"session_id"`
	Context   *ChatContext `json:"context,omitempty"`
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
