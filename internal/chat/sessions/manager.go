package sessions

import (
	"crypto/rand"
	"fmt"
	"io"
	"smith/internal/chat"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*chat.Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*chat.Session),
	}
}

func (m *Manager) CreateSession(sType chat.SessionType, context map[string]string) (*chat.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return nil, err
	}

	session := &chat.Session{
		ID:        id,
		Type:      sType,
		Context:   context,
		Messages:  []chat.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.sessions[id] = session
	return session, nil
}

func (m *Manager) GetSession(id string) (*chat.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) AddMessage(sessionID string, msg chat.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
	return nil
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return fmt.Sprintf("sess_%x", b), nil
}
