package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const defaultSecretTokenStoreKey = "tokens.json"

type SecretTokenStore struct {
	client     kubernetes.Interface
	namespace  string
	secretName string
	dataKey    string
}

func NewSecretTokenStore(client kubernetes.Interface, namespace, secretName, dataKey string) (*SecretTokenStore, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, errors.New("kubernetes namespace is required")
	}
	secretName = strings.TrimSpace(secretName)
	if secretName == "" {
		return nil, errors.New("kubernetes secret name is required")
	}
	dataKey = strings.TrimSpace(dataKey)
	if dataKey == "" {
		dataKey = defaultSecretTokenStoreKey
	}
	return &SecretTokenStore{
		client:     client,
		namespace:  namespace,
		secretName: secretName,
		dataKey:    dataKey,
	}, nil
}

func (s *SecretTokenStore) Get(ctx context.Context, providerID string) (Token, bool, error) {
	f, err := s.read(ctx)
	if err != nil {
		return Token{}, false, err
	}
	tok, ok := f.Tokens[normalize(providerID)]
	return tok, ok, nil
}

func (s *SecretTokenStore) Put(ctx context.Context, providerID string, token Token) error {
	id := normalize(providerID)
	return s.update(ctx, func(f tokenFile) tokenFile {
		if f.Tokens == nil {
			f.Tokens = map[string]Token{}
		}
		f.Tokens[id] = token
		return f
	})
}

func (s *SecretTokenStore) Delete(ctx context.Context, providerID string) error {
	id := normalize(providerID)
	return s.update(ctx, func(f tokenFile) tokenFile {
		if f.Tokens == nil {
			f.Tokens = map[string]Token{}
			return f
		}
		delete(f.Tokens, id)
		return f
	})
}

func (s *SecretTokenStore) read(ctx context.Context) (tokenFile, error) {
	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, s.secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return tokenFile{Tokens: map[string]Token{}}, nil
		}
		return tokenFile{}, err
	}
	return decodeTokenFile(secret.Data[s.dataKey])
}

func (s *SecretTokenStore) update(ctx context.Context, mutator func(tokenFile) tokenFile) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		secrets := s.client.CoreV1().Secrets(s.namespace)
		secret, err := secrets.Get(ctx, s.secretName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("auth store secret %s/%s not found", s.namespace, s.secretName)
			}
			return err
		}

		f, err := decodeTokenFile(secret.Data[s.dataKey])
		if err != nil {
			return fmt.Errorf("decode token data: %w", err)
		}
		f = mutator(f)
		payload, err := json.Marshal(f)
		if err != nil {
			return err
		}

		next := secret.DeepCopy()
		if next.Data == nil {
			next.Data = map[string][]byte{}
		}
		next.Data[s.dataKey] = payload
		_, err = secrets.Update(ctx, next, metav1.UpdateOptions{})
		return err
	})
}

func decodeTokenFile(data []byte) (tokenFile, error) {
	if len(data) == 0 {
		return tokenFile{Tokens: map[string]Token{}}, nil
	}
	var f tokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return tokenFile{}, err
	}
	if f.Tokens == nil {
		f.Tokens = map[string]Token{}
	}
	return f, nil
}
