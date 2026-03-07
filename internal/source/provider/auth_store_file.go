package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type FileTokenStore struct {
	path string
	mu   sync.Mutex
}

type tokenFile struct {
	Tokens map[string]Token `json:"tokens"`
}

func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{path: path}
}

func (s *FileTokenStore) Get(_ context.Context, providerID string) (Token, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return Token{}, false, err
	}
	tok, ok := f.Tokens[normalize(providerID)]
	return tok, ok, nil
}

func (s *FileTokenStore) Put(_ context.Context, providerID string, token Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	if f.Tokens == nil {
		f.Tokens = map[string]Token{}
	}
	f.Tokens[normalize(providerID)] = token
	return s.write(f)
}

func (s *FileTokenStore) Delete(_ context.Context, providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	delete(f.Tokens, normalize(providerID))
	return s.write(f)
}

func (s *FileTokenStore) read() (tokenFile, error) {
	if s.path == "" {
		return tokenFile{}, errors.New("token store path is required")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return tokenFile{Tokens: map[string]Token{}}, nil
		}
		return tokenFile{}, err
	}
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

func (s *FileTokenStore) write(f tokenFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	payload, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	return os.Chmod(s.path, 0o600)
}
