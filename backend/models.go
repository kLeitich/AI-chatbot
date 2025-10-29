package main

import (
	"time"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Appointment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PatientName string    `gorm:"size:255;not null" json:"patient_name"`
	Doctor      string    `gorm:"size:255;not null" json:"doctor"`
	Date        string    `gorm:"size:10;not null" json:"date"`
	Time        string    `gorm:"size:5;not null" json:"time"`
	Reason      string    `gorm:"size:500" json:"reason"`
	Status      string    `gorm:"size:50;default:pending" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

type ChatResponse struct {
	Message     string       `json:"message"`
	Appointment *Appointment `json:"appointment,omitempty"`
}

// In-memory conversation state per session
// This is non-persistent and intended for short-lived conversational context.
// For production, consider Redis or a DB table to persist context across replicas.
type ConversationState struct {
	LastUserMessage string
	LastAIMessage   string
	Draft           Appointment
	UpdatedAt       time.Time
}
