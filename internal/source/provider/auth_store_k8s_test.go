package provider

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewSecretTokenStoreValidation(t *testing.T) {
	_, err := NewSecretTokenStore(nil, "ns", "name", "tokens.json")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	_, err = NewSecretTokenStore(fake.NewSimpleClientset(), "", "name", "tokens.json")
	if err == nil {
		t.Fatal("expected error for empty namespace")
	}
	_, err = NewSecretTokenStore(fake.NewSimpleClientset(), "ns", "", "tokens.json")
	if err == nil {
		t.Fatal("expected error for empty secret name")
	}
}

func TestSecretTokenStoreCRUD(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	store, err := NewSecretTokenStore(client, "smith-system", "smith-auth-store", "")
	if err != nil {
		t.Fatalf("new secret token store: %v", err)
	}

	_, found, err := store.Get(ctx, ProviderCodex)
	if err != nil {
		t.Fatalf("get missing token: %v", err)
	}
	if found {
		t.Fatal("expected missing token before put")
	}

	_, err = client.CoreV1().Secrets("smith-system").Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "smith-auth-store"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{defaultSecretTokenStoreKey: []byte(`{"tokens":{}}`)},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create backing secret: %v", err)
	}

	want := Token{
		AccessToken: "sk-test-123",
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		AccountID:   "acct-api",
		AuthMethod:  "api_key",
	}
	if err := store.Put(ctx, ProviderCodex, want); err != nil {
		t.Fatalf("put token: %v", err)
	}

	got, found, err := store.Get(ctx, ProviderCodex)
	if err != nil {
		t.Fatalf("get token: %v", err)
	}
	if !found {
		t.Fatal("expected token to be present after put")
	}
	if got.AccessToken != want.AccessToken {
		t.Fatalf("expected access token %q, got %q", want.AccessToken, got.AccessToken)
	}
	if got.AccountID != want.AccountID {
		t.Fatalf("expected account id %q, got %q", want.AccountID, got.AccountID)
	}

	secret, err := client.CoreV1().Secrets("smith-system").Get(ctx, "smith-auth-store", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get backing secret: %v", err)
	}
	if len(secret.Data[defaultSecretTokenStoreKey]) == 0 {
		t.Fatal("expected backing secret token payload")
	}

	if err := store.Delete(ctx, ProviderCodex); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	_, found, err = store.Get(ctx, ProviderCodex)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if found {
		t.Fatal("expected token to be missing after delete")
	}
}

func TestSecretTokenStorePutRequiresExistingSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	store, err := NewSecretTokenStore(client, "smith-system", "missing-auth-store", "")
	if err != nil {
		t.Fatalf("new secret token store: %v", err)
	}
	err = store.Put(ctx, ProviderCodex, Token{
		AccessToken: "sk-test-123",
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected put to fail when backing secret is missing")
	}
}

func TestSecretTokenStoreProjectCredentialCRUD(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	store, err := NewSecretTokenStore(client, "smith-system", "smith-auth-store", "")
	if err != nil {
		t.Fatalf("new secret token store: %v", err)
	}
	_, err = client.CoreV1().Secrets("smith-system").Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "smith-auth-store"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{defaultSecretTokenStoreKey: []byte(`{"tokens":{}}`)},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create backing secret: %v", err)
	}

	projectID := "proj-alpha"
	cred := ProjectCredential{
		GitHubUser: "smith-bot",
		PAT:        "ghp_test_123",
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
		t.Fatal("expected project credential to exist")
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
		t.Fatal("expected project credential to be deleted")
	}
}
