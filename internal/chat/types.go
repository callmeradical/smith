package chat

import (
	"time"
)

type SessionType string

const (
	SessionTypePRDRefinement  SessionType = "prd-refinement"
	SessionTypeLoopAssist     SessionType = "loop-assist"
	SessionTypeDocumentAssist SessionType = "document-assist"
	SessionTypeRuntimeAssist  SessionType = "runtime-assist"
)

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

type Message struct {
	ID        string      `json:"id"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
}

type Session struct {
	ID        string            `json:"sessionId"`
	Type      SessionType       `json:"type"`
	Context   map[string]string `json:"context"`
	Messages  []Message         `json:"messages"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type EventType string

const (
	EventMessageDelta     EventType = "message.delta"
	EventMessageCompleted EventType = "message.completed"
	EventToolStarted      EventType = "tool.started"
	EventToolCompleted    EventType = "tool.completed"
	EventStructuredResult EventType = "structured.result"
	EventError            EventType = "error"
)

type ChatEvent struct {
	Event EventType `json:"event"`
	Data  any       `json:"data"`
}

type MessageDelta struct {
	Delta string `json:"delta"`
}

type ToolEvent struct {
	Tool string `json:"tool"`
}

type StructuredResult struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}
