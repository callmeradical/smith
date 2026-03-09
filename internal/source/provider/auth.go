package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAuthRequired     = errors.New("provider authentication required")
	ErrTokenExpired     = errors.New("provider token expired")
	ErrTokenRefresh     = errors.New("provider token refresh failed")
	ErrAuthNotConnected = errors.New("provider is not connected")
)

type Token struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	AccountID     string    `json:"account_id"`
	AuthMethod    string    `json:"auth_method,omitempty"`
	ConnectedAt   time.Time `json:"connected_at,omitempty"`
	LastRefreshAt time.Time `json:"last_refresh_at,omitempty"`
}

type AuthStatus struct {
	Connected     bool
	ExpiresAt     time.Time
	AccountID     string
	AuthMethod    string
	ConnectedAt   time.Time
	LastRefreshAt time.Time
}

type StoredCredential struct {
	Connected  bool
	AccountID  string
	AuthMethod string
	APIKey     string
}

type DeviceAuthSession struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
	IntervalSeconds int       `json:"interval_seconds"`
}

type AuthEvent struct {
	ProviderID string            `json:"provider_id"`
	Actor      string            `json:"actor"`
	Action     string            `json:"action"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type TokenStore interface {
	Get(ctx context.Context, providerID string) (Token, bool, error)
	Put(ctx context.Context, providerID string, token Token) error
	Delete(ctx context.Context, providerID string) error
}

type AuthClient interface {
	StartDeviceAuth(ctx context.Context) (DeviceAuthSession, error)
	ExchangeDeviceCode(ctx context.Context, deviceCode string) (Token, error)
	Refresh(ctx context.Context, refreshToken string) (Token, error)
}

type AuditSink interface {
	RecordAuthEvent(ctx context.Context, event AuthEvent) error
}

type AuthManager struct {
	providerID  string
	store       TokenStore
	client      AuthClient
	audit       AuditSink
	clock       func() time.Time
	refreshSkew time.Duration
}

func NewAuthManager(providerID string, store TokenStore, client AuthClient, audit AuditSink) *AuthManager {
	return &AuthManager{
		providerID:  normalize(providerID),
		store:       store,
		client:      client,
		audit:       audit,
		clock:       time.Now,
		refreshSkew: 2 * time.Minute,
	}
}

func (m *AuthManager) StartConnect(ctx context.Context, actor string) (DeviceAuthSession, error) {
	session, err := m.client.StartDeviceAuth(ctx)
	if err != nil {
		return DeviceAuthSession{}, err
	}
	_ = m.record(ctx, actor, "connect-started", map[string]string{"verification_uri": session.VerificationURI})
	return session, nil
}

func (m *AuthManager) CompleteConnect(ctx context.Context, actor string, deviceCode string) (Token, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return Token{}, errors.New("device_code is required")
	}
	token, err := m.client.ExchangeDeviceCode(ctx, deviceCode)
	if err != nil {
		return Token{}, err
	}
	if err := validateToken(token); err != nil {
		return Token{}, err
	}
	now := m.clock().UTC()
	token.AuthMethod = "openai_sign_in"
	token.ConnectedAt = now
	token.LastRefreshAt = now
	if err := m.store.Put(ctx, m.providerID, token); err != nil {
		return Token{}, err
	}
	_ = m.record(ctx, actor, "connected", map[string]string{"account_id": token.AccountID})
	return token, nil
}

func (m *AuthManager) Status(ctx context.Context) (AuthStatus, error) {
	token, found, err := m.store.Get(ctx, m.providerID)
	if err != nil {
		return AuthStatus{}, err
	}
	if !found || strings.TrimSpace(token.AccessToken) == "" {
		return AuthStatus{}, nil
	}
	if token.LastRefreshAt.IsZero() {
		token.LastRefreshAt = token.ConnectedAt
	}
	return AuthStatus{
		Connected:     true,
		ExpiresAt:     token.ExpiresAt,
		AccountID:     token.AccountID,
		AuthMethod:    token.AuthMethod,
		ConnectedAt:   token.ConnectedAt,
		LastRefreshAt: token.LastRefreshAt,
	}, nil
}

func (m *AuthManager) ConnectAPIKey(ctx context.Context, actor string, apiKey string, accountID string) (Token, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return Token{}, errors.New("api_key is required")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		accountID = "api-key"
	}
	now := m.clock().UTC()
	token := Token{
		AccessToken:   apiKey,
		RefreshToken:  "",
		ExpiresAt:     now.Add(10 * 365 * 24 * time.Hour),
		AccountID:     accountID,
		AuthMethod:    "api_key",
		ConnectedAt:   now,
		LastRefreshAt: now,
	}
	if err := m.store.Put(ctx, m.providerID, token); err != nil {
		return Token{}, err
	}
	_ = m.record(ctx, actor, "connected-api-key", map[string]string{"account_id": token.AccountID})
	return token, nil
}

func (m *AuthManager) StoredCredential(ctx context.Context) (StoredCredential, error) {
	token, found, err := m.store.Get(ctx, m.providerID)
	if err != nil {
		return StoredCredential{}, err
	}
	if !found || strings.TrimSpace(token.AccessToken) == "" {
		return StoredCredential{}, nil
	}
	credential := StoredCredential{
		Connected:  true,
		AccountID:  token.AccountID,
		AuthMethod: token.AuthMethod,
	}
	if strings.EqualFold(token.AuthMethod, "api_key") {
		credential.APIKey = token.AccessToken
	}
	return credential, nil
}

func (m *AuthManager) Disconnect(ctx context.Context, actor string) error {
	if err := m.store.Delete(ctx, m.providerID); err != nil {
		return err
	}
	_ = m.record(ctx, actor, "disconnected", nil)
	return nil
}

func (m *AuthManager) EnsureValidToken(ctx context.Context, actor string) (Token, error) {
	token, found, err := m.store.Get(ctx, m.providerID)
	if err != nil {
		return Token{}, err
	}
	if !found || strings.TrimSpace(token.AccessToken) == "" {
		return Token{}, fmt.Errorf("%w: run connect flow", ErrAuthRequired)
	}

	now := m.clock().UTC()
	if token.ExpiresAt.After(now.Add(m.refreshSkew)) {
		return token, nil
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return Token{}, fmt.Errorf("%w: reconnect required", ErrTokenExpired)
	}

	refreshed, err := m.client.Refresh(ctx, token.RefreshToken)
	if err != nil {
		_ = m.record(ctx, actor, "refresh-failed", map[string]string{"reason": "refresh_error"})
		return Token{}, fmt.Errorf("%w: reconnect required", ErrTokenRefresh)
	}
	if err := validateToken(refreshed); err != nil {
		return Token{}, err
	}
	refreshed.ConnectedAt = token.ConnectedAt
	refreshed.LastRefreshAt = m.clock().UTC()
	if refreshed.ConnectedAt.IsZero() {
		refreshed.ConnectedAt = refreshed.LastRefreshAt
	}
	if err := m.store.Put(ctx, m.providerID, refreshed); err != nil {
		return Token{}, err
	}
	_ = m.record(ctx, actor, "refreshed", map[string]string{"account_id": refreshed.AccountID})
	return refreshed, nil
}

func validateToken(token Token) error {
	if strings.TrimSpace(token.AccessToken) == "" {
		return errors.New("access token is required")
	}
	if token.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	return nil
}

func (m *AuthManager) record(ctx context.Context, actor, action string, metadata map[string]string) error {
	if m.audit == nil {
		return nil
	}
	return m.audit.RecordAuthEvent(ctx, AuthEvent{
		ProviderID: m.providerID,
		Actor:      strings.TrimSpace(actor),
		Action:     action,
		Timestamp:  m.clock().UTC(),
		Metadata:   metadata,
	})
}
