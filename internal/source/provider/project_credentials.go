package provider

import (
	"context"
	"strings"
	"time"
)

type ProjectCredential struct {
	GitHubUser string    `json:"github_user,omitempty"`
	PAT        string    `json:"pat"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type ProjectCredentialStore interface {
	GetProjectCredential(ctx context.Context, projectID string) (ProjectCredential, bool, error)
	PutProjectCredential(ctx context.Context, projectID string, credential ProjectCredential) error
	DeleteProjectCredential(ctx context.Context, projectID string) error
}

func normalizeProjectCredentialID(projectID string) string {
	return strings.ToLower(strings.TrimSpace(projectID))
}
