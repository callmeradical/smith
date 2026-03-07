package provider

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type codexAdapter struct {
	sessionCounter uint64
	auth           *AuthManager
}

func NewCodexRegistration() Registration {
	return NewCodexRegistrationWithAuth(nil)
}

func NewCodexRegistrationWithAuth(auth *AuthManager) Registration {
	return Registration{
		ProviderID:   ProviderCodex,
		DefaultModel: DefaultCodexModel,
		Models: []string{
			DefaultCodexModel,
			CodexMiniModel,
		},
		Adapter: &codexAdapter{auth: auth},
	}
}

func (a *codexAdapter) CreateSession(ctx context.Context, request SessionRequest) (Session, error) {
	if a.auth != nil {
		if _, err := a.auth.EnsureValidToken(ctx, "runtime"); err != nil {
			return Session{}, err
		}
	}
	model := normalize(request.Model)
	if model == "" {
		model = DefaultCodexModel
	}
	if err := a.ValidateConfig(Config{Model: model, Options: request.Options}); err != nil {
		return Session{}, err
	}

	id := atomic.AddUint64(&a.sessionCounter, 1)
	return Session{
		ID:         fmt.Sprintf("codex-%d", id),
		ProviderID: ProviderCodex,
		Model:      model,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (a *codexAdapter) SendTurn(ctx context.Context, session Session, input TurnInput) (TurnResult, error) {
	if a.auth != nil {
		if _, err := a.auth.EnsureValidToken(ctx, "runtime"); err != nil {
			return TurnResult{}, err
		}
	}
	if err := a.ValidateConfig(Config{Model: session.Model}); err != nil {
		return TurnResult{}, err
	}
	if strings.TrimSpace(input.Content) == "" {
		return TurnResult{}, fmt.Errorf("input content is required")
	}
	return TurnResult{Output: fmt.Sprintf("codex:%s", strings.TrimSpace(input.Content))}, nil
}

func (a *codexAdapter) StreamEvents(ctx context.Context, session Session) (<-chan Event, error) {
	if err := a.ValidateConfig(Config{Model: session.Model}); err != nil {
		return nil, err
	}
	ch := make(chan Event, 1)
	ch <- Event{Type: "session_started", Message: session.ID, Timestamp: time.Now().UTC()}
	close(ch)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return ch, nil
	}
}

func (a *codexAdapter) CloseSession(context.Context, Session) error {
	return nil
}

func (a *codexAdapter) ValidateConfig(config Config) error {
	model := normalize(config.Model)
	switch model {
	case DefaultCodexModel, CodexMiniModel:
		return nil
	default:
		return fmt.Errorf("%w: provider=%s model=%s", ErrUnsupportedModel, ProviderCodex, model)
	}
}
