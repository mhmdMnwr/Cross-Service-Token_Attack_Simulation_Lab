package main

import (
	"sync"
	"time"
)

// Session represents a UE session managed by the AMF.
type Session struct {
	UEID            string            `json:"ueId"`
	State           string            `json:"state"`
	AuthStatus      string            `json:"authStatus"`
	RegistrationTime string           `json:"registrationTime"`
	TokensUsed      map[string]string `json:"tokensUsed"` // service -> token
}

// SessionStore is an in-memory store of UE sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// CreateSession creates a new UE session.
func (ss *SessionStore) CreateSession(ueID string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session := &Session{
		UEID:            ueID,
		State:           "REGISTERED",
		AuthStatus:      "AUTHENTICATED",
		RegistrationTime: time.Now().UTC().Format(time.RFC3339),
		TokensUsed:      make(map[string]string),
	}
	ss.sessions[ueID] = session
	return session
}

// GetSession retrieves a session by UE ID.
func (ss *SessionStore) GetSession(ueID string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.sessions[ueID]
	return s, ok
}

// AddTokenToSession records a token used for a session.
func (ss *SessionStore) AddTokenToSession(ueID, service, token string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if s, ok := ss.sessions[ueID]; ok {
		s.TokensUsed[service] = token
	}
}

// ListSessions returns all active sessions.
func (ss *SessionStore) ListSessions() []*Session {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	sessions := make([]*Session, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}
