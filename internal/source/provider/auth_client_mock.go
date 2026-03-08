package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type MockDeviceAuthClient struct {
	mu       sync.Mutex
	sessions map[string]DeviceAuthSession
	counter  int
}

func NewMockDeviceAuthClient() *MockDeviceAuthClient {
	return &MockDeviceAuthClient{sessions: map[string]DeviceAuthSession{}}
}

func (c *MockDeviceAuthClient) StartDeviceAuth(_ context.Context) (DeviceAuthSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counter++
	now := time.Now().UTC()
	s := DeviceAuthSession{
		DeviceCode:      fmt.Sprintf("device-%d", c.counter),
		UserCode:        fmt.Sprintf("USER-%04d", c.counter),
		VerificationURI: "https://codex.example.com/device",
		ExpiresAt:       now.Add(10 * time.Minute),
		IntervalSeconds: 5,
	}
	c.sessions[s.DeviceCode] = s
	return s, nil
}

func (c *MockDeviceAuthClient) ExchangeDeviceCode(_ context.Context, deviceCode string) (Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[deviceCode]
	if !ok {
		return Token{}, errors.New("invalid device code")
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		return Token{}, errors.New("device code expired")
	}
	return Token{
		AccessToken:  "access-" + deviceCode,
		RefreshToken: "refresh-" + deviceCode,
		ExpiresAt:    time.Now().UTC().Add(15 * time.Minute),
		AccountID:    "codex-account-demo",
	}, nil
}

func (c *MockDeviceAuthClient) Refresh(_ context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, errors.New("missing refresh token")
	}
	if refreshToken == "invalid" {
		return Token{}, errors.New("invalid refresh token")
	}
	return Token{
		AccessToken:  "access-refreshed",
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().UTC().Add(15 * time.Minute),
		AccountID:    "codex-account-demo",
	}, nil
}
