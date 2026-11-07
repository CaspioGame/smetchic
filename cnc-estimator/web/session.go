package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

const (
	sessionLifetime  = 12 * time.Hour
	sessionSweepRate = time.Minute
)

type serverSession struct {
	ID        string
	UserID    string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type sessionStore struct {
	mu          sync.Mutex
	sessions    map[string]serverSession
	lastCleanup time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]serverSession)}
}

func (store *sessionStore) create(userID string) (serverSession, error) {
	sessionID, err := randomToken(32)
	if err != nil {
		return serverSession{}, fmt.Errorf("generate session ID: %w", err)
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return serverSession{}, fmt.Errorf("generate CSRF token: %w", err)
	}
	now := time.Now().UTC()
	session := serverSession{
		ID: sessionID, UserID: userID, CSRFToken: csrfToken,
		CreatedAt: now, ExpiresAt: now.Add(sessionLifetime),
	}
	store.mu.Lock()
	if now.Sub(store.lastCleanup) >= sessionSweepRate {
		for id, existing := range store.sessions {
			if !now.Before(existing.ExpiresAt) {
				delete(store.sessions, id)
			}
		}
		store.lastCleanup = now
	}
	store.sessions[session.ID] = session
	store.mu.Unlock()
	return session, nil
}

func (store *sessionStore) get(sessionID string) (serverSession, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, exists := store.sessions[sessionID]
	if !exists {
		return serverSession{}, false
	}
	if !time.Now().Before(session.ExpiresAt) {
		delete(store.sessions, sessionID)
		return serverSession{}, false
	}
	return session, true
}

func (store *sessionStore) delete(sessionID string) {
	store.mu.Lock()
	delete(store.sessions, sessionID)
	store.mu.Unlock()
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
