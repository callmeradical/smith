package provider

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestCodexAdapterRequiresAuthWhenConfigured(t *testing.T) {
	store := NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	auth := NewAuthManager(ProviderCodex, store, NewMockDeviceAuthClient(), nil)
	reg := NewCodexRegistrationWithAuth(auth)

	session, err := reg.Adapter.CreateSession(context.Background(), SessionRequest{LoopID: "loop-1", Model: DefaultCodexModel})
	if err == nil || !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got session=%+v err=%v", session, err)
	}
}

func TestCodexAdapterSessionWorksAfterConnect(t *testing.T) {
	store := NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	auth := NewAuthManager(ProviderCodex, store, NewMockDeviceAuthClient(), nil)

	s, err := auth.StartConnect(context.Background(), "operator")
	if err != nil {
		t.Fatalf("start connect: %v", err)
	}
	if _, err := auth.CompleteConnect(context.Background(), "operator", s.DeviceCode); err != nil {
		t.Fatalf("complete connect: %v", err)
	}

	reg := NewCodexRegistrationWithAuth(auth)
	session, err := reg.Adapter.CreateSession(context.Background(), SessionRequest{LoopID: "loop-1", Model: DefaultCodexModel})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	result, err := reg.Adapter.SendTurn(context.Background(), session, TurnInput{Role: "user", Content: "ping"})
	if err != nil {
		t.Fatalf("send turn: %v", err)
	}
	if result.Output == "" {
		t.Fatal("expected non-empty output")
	}
}
