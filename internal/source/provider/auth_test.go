package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type authEventCollector struct{ events []AuthEvent }

func (a *authEventCollector) RecordAuthEvent(_ context.Context, event AuthEvent) error {
	a.events = append(a.events, event)
	return nil
}

func TestAuthManagerConnectStatusRefreshDisconnect(t *testing.T) {
	dir := t.TempDir()
	store := NewFileTokenStore(filepath.Join(dir, "tokens.json"))
	client := NewMockDeviceAuthClient()
	audit := &authEventCollector{}
	mgr := NewAuthManager(ProviderCodex, store, client, audit)
	mgr.refreshSkew = 30 * time.Second

	ctx := context.Background()
	status, err := mgr.Status(ctx)
	if err != nil {
		t.Fatalf("status before connect: %v", err)
	}
	if status.Connected {
		t.Fatal("expected disconnected status before connect")
	}

	session, err := mgr.StartConnect(ctx, "operator-a")
	if err != nil {
		t.Fatalf("start connect: %v", err)
	}
	if session.DeviceCode == "" || session.UserCode == "" {
		t.Fatal("expected non-empty device auth session")
	}

	token, err := mgr.CompleteConnect(ctx, "operator-a", session.DeviceCode)
	if err != nil {
		t.Fatalf("complete connect: %v", err)
	}
	if token.AccessToken == "" {
		t.Fatal("expected access token")
	}

	status, err = mgr.Status(ctx)
	if err != nil {
		t.Fatalf("status after connect: %v", err)
	}
	if !status.Connected {
		t.Fatal("expected connected=true")
	}
	if status.AccountID == "" {
		t.Fatal("expected account id in status")
	}
	if status.LastRefreshAt.IsZero() {
		t.Fatal("expected last refresh metadata in status")
	}

	stored, found, err := store.Get(ctx, ProviderCodex)
	if err != nil || !found {
		t.Fatalf("store get found=%v err=%v", found, err)
	}
	stored.ExpiresAt = time.Now().UTC().Add(10 * time.Second)
	if err := store.Put(ctx, ProviderCodex, stored); err != nil {
		t.Fatalf("store put expiring token: %v", err)
	}

	refreshed, err := mgr.EnsureValidToken(ctx, "operator-a")
	if err != nil {
		t.Fatalf("ensure valid token: %v", err)
	}
	if refreshed.AccessToken != "access-refreshed" {
		t.Fatalf("expected refreshed access token, got %q", refreshed.AccessToken)
	}

	if err := mgr.Disconnect(ctx, "operator-a"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	status, err = mgr.Status(ctx)
	if err != nil {
		t.Fatalf("status after disconnect: %v", err)
	}
	if status.Connected {
		t.Fatal("expected connected=false after disconnect")
	}

	if len(audit.events) < 3 {
		t.Fatalf("expected auth audit events, got %d", len(audit.events))
	}
}

func TestAuthManagerRequiresConnection(t *testing.T) {
	store := NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	mgr := NewAuthManager(ProviderCodex, store, NewMockDeviceAuthClient(), nil)

	_, err := mgr.EnsureValidToken(context.Background(), "operator-b")
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got %v", err)
	}
}

func TestFileTokenStoreUsesRestrictedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := NewFileTokenStore(path)

	err := store.Put(context.Background(), ProviderCodex, Token{
		AccessToken:  "a",
		RefreshToken: "r",
		ExpiresAt:    time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("put token: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", st.Mode().Perm())
	}
}
