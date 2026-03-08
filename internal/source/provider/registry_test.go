package provider

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultRegistryResolvesCodexByDefault(t *testing.T) {
	registry := NewDefaultRegistry()

	selection, err := registry.Resolve("", "")
	if err != nil {
		t.Fatalf("expected default resolution, got %v", err)
	}
	if selection.ProviderID != ProviderCodex {
		t.Fatalf("expected provider %q, got %q", ProviderCodex, selection.ProviderID)
	}
	if selection.Model != DefaultCodexModel {
		t.Fatalf("expected model %q, got %q", DefaultCodexModel, selection.Model)
	}
}

func TestRegistryResolveRejectsUnknownProvider(t *testing.T) {
	registry := NewDefaultRegistry()
	_, err := registry.Resolve("missing", "")
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("expected ErrUnknownProvider, got %v", err)
	}
}

func TestRegistryResolveRejectsUnsupportedModel(t *testing.T) {
	registry := NewDefaultRegistry()
	_, err := registry.Resolve(ProviderCodex, "unknown-model")
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Fatalf("expected ErrUnsupportedModel, got %v", err)
	}
}

func TestCodexAdapterLifecycle(t *testing.T) {
	registry := NewDefaultRegistry()
	selection, err := registry.Resolve(ProviderCodex, DefaultCodexModel)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	session, err := selection.Adapter.CreateSession(context.Background(), SessionRequest{LoopID: "loop-1", Model: selection.Model})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if session.ProviderID != ProviderCodex {
		t.Fatalf("expected provider %q, got %q", ProviderCodex, session.ProviderID)
	}

	result, err := selection.Adapter.SendTurn(context.Background(), session, TurnInput{Role: "user", Content: "status"})
	if err != nil {
		t.Fatalf("send turn failed: %v", err)
	}
	if result.Output == "" {
		t.Fatal("expected non-empty output")
	}

	events, err := selection.Adapter.StreamEvents(context.Background(), session)
	if err != nil {
		t.Fatalf("stream events failed: %v", err)
	}
	count := 0
	for range events {
		count++
	}
	if count == 0 {
		t.Fatal("expected at least one event")
	}

	if err := selection.Adapter.CloseSession(context.Background(), session); err != nil {
		t.Fatalf("close session failed: %v", err)
	}
}
