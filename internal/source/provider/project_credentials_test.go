package provider

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFileTokenStoreProjectCredentialCRUD(t *testing.T) {
	store := NewFileTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	ctx := context.Background()
	projectID := "proj-beta"
	cred := ProjectCredential{
		GitHubUser: "beta-bot",
		PAT:        "ghp_beta_123",
		UpdatedAt:  time.Now().UTC(),
	}

	if err := store.PutProjectCredential(ctx, projectID, cred); err != nil {
		t.Fatalf("put project credential: %v", err)
	}
	got, found, err := store.GetProjectCredential(ctx, projectID)
	if err != nil {
		t.Fatalf("get project credential: %v", err)
	}
	if !found {
		t.Fatal("expected project credential to be found")
	}
	if got.PAT != cred.PAT {
		t.Fatalf("expected pat %q, got %q", cred.PAT, got.PAT)
	}
	if got.GitHubUser != cred.GitHubUser {
		t.Fatalf("expected github user %q, got %q", cred.GitHubUser, got.GitHubUser)
	}

	if err := store.DeleteProjectCredential(ctx, projectID); err != nil {
		t.Fatalf("delete project credential: %v", err)
	}
	_, found, err = store.GetProjectCredential(ctx, projectID)
	if err != nil {
		t.Fatalf("get project credential after delete: %v", err)
	}
	if found {
		t.Fatal("expected deleted project credential")
	}
}
