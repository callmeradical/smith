package sessions

import (
	"smith/internal/chat"
	"testing"
	"time"
)

func TestManager_CreateSession(t *testing.T) {
	m := NewManager()
	sType := chat.SessionTypePRDRefinement
	ctx := map[string]string{"foo": "bar"}

	s, err := m.CreateSession(sType, ctx)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if s.ID == "" {
		t.Error("expected session ID to be set")
	}
	if s.Type != sType {
		t.Errorf("expected type %s, got %s", sType, s.Type)
	}
	if s.Context["foo"] != "bar" {
		t.Errorf("expected context foo=bar, got %v", s.Context)
	}
	if len(s.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(s.Messages))
	}

	// Verify it's in the manager
	s2, ok := m.GetSession(s.ID)
	if !ok {
		t.Error("expected session to be found in manager")
	}
	if s2 != s {
		t.Error("expected retrieved session to be the same instance")
	}
}

func TestManager_GetSession(t *testing.T) {
	m := NewManager()

	t.Run("non-existent session", func(t *testing.T) {
		_, ok := m.GetSession("non-existent")
		if ok {
			t.Error("expected ok to be false for non-existent session")
		}
	})

	t.Run("existing session", func(t *testing.T) {
		s, _ := m.CreateSession(chat.SessionTypePRDRefinement, nil)
		s2, ok := m.GetSession(s.ID)
		if !ok {
			t.Error("expected session to be found")
		}
		if s2.ID != s.ID {
			t.Errorf("expected ID %s, got %s", s.ID, s2.ID)
		}
	})
}

func TestManager_AddMessage(t *testing.T) {
	m := NewManager()

	t.Run("session not found", func(t *testing.T) {
		err := m.AddMessage("non-existent", chat.Message{Content: "hello"})
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})

	t.Run("add message success", func(t *testing.T) {
		s, _ := m.CreateSession(chat.SessionTypePRDRefinement, nil)
		initialUpdateAt := s.UpdatedAt

		// Wait a tiny bit to ensure UpdatedAt changes
		time.Sleep(1 * time.Millisecond)

		msg := chat.Message{
			ID:      "msg1",
			Role:    chat.RoleUser,
			Content: "hello",
		}

		err := m.AddMessage(s.ID, msg)
		if err != nil {
			t.Fatalf("failed to add message: %v", err)
		}

		if len(s.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(s.Messages))
		}
		if s.Messages[0].Content != "hello" {
			t.Errorf("expected message content 'hello', got %s", s.Messages[0].Content)
		}
		if !s.UpdatedAt.After(initialUpdateAt) {
			t.Error("expected UpdatedAt to be updated")
		}
	})
}
