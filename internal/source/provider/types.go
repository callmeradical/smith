package provider

import (
	"context"
	"time"
)

type SessionRequest struct {
	LoopID  string
	Model   string
	Options map[string]string
}

type TurnInput struct {
	Role    string
	Content string
}

type Session struct {
	ID         string
	ProviderID string
	Model      string
	CreatedAt  time.Time
}

type TurnResult struct {
	Output string
}

type Event struct {
	Type      string
	Message   string
	Timestamp time.Time
}

type Config struct {
	Model   string
	Options map[string]string
}

type Adapter interface {
	CreateSession(ctx context.Context, request SessionRequest) (Session, error)
	SendTurn(ctx context.Context, session Session, input TurnInput) (TurnResult, error)
	StreamEvents(ctx context.Context, session Session) (<-chan Event, error)
	CloseSession(ctx context.Context, session Session) error
	ValidateConfig(config Config) error
}

type Registration struct {
	ProviderID   string
	DefaultModel string
	Models       []string
	Adapter      Adapter
}

type Selection struct {
	ProviderID string
	Model      string
	Adapter    Adapter
}
