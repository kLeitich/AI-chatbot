package main

import (
	"regexp"
	"sync"
	"time"
)

var (
	dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex = regexp.MustCompile(`^\d{2}:\d{2}$`)
)

func isValidDate(date string) bool {
	if !dateRegex.MatchString(date) {
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func isValidTime(t string) bool {
	if !timeRegex.MatchString(t) {
		return false
	}
	_, err := time.Parse("15:04", t)
	return err == nil
}

// conversation memory store
var (
	convMu   sync.RWMutex
	sessionM = map[string]ConversationState{}
)

func getConversation(sessionID string) ConversationState {
	if sessionID == "" {
		return ConversationState{}
	}
	convMu.RLock()
	defer convMu.RUnlock()
	return sessionM[sessionID]
}

func setConversation(sessionID string, s ConversationState) {
	if sessionID == "" {
		return
	}
	convMu.Lock()
	defer convMu.Unlock()
	s.UpdatedAt = time.Now()
	sessionM[sessionID] = s
}
