package callers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gotify-relay/internal/config"
)

type CallerInfo struct {
	Name     string   `json:"name"`
	Token    string   `json:"token"`
	Channels []string `json:"channels"`
}

type JSONStore struct {
	path    string
	callers map[string]config.Caller
	mu      sync.Mutex
	tokens  map[string]string
}

type stateFile struct {
	Tokens map[string]string `json:"tokens"`
}

func NewJSONStore(path string, cfg *config.Config) (*JSONStore, error) {
	if path == "" {
		return nil, errors.New("caller token store path is required")
	}

	store := &JSONStore{
		path:    path,
		callers: cfg.Callers,
		tokens:  map[string]string{},
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	if err := store.reconcile(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *JSONStore) Token(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.tokens[name]
	return token, ok
}

func (s *JSONStore) ByToken(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, t := range s.tokens {
		if t == token {
			return name, true
		}
	}
	return "", false
}

func (s *JSONStore) List() []CallerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.callers))
	for name := range s.callers {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]CallerInfo, 0, len(names))
	for _, name := range names {
		out = append(out, CallerInfo{
			Name:     name,
			Token:    s.tokens[name],
			Channels: s.callers[name].Channels,
		})
	}
	return out
}

func (s *JSONStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read caller tokens: %w", err)
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parse caller tokens: %w", err)
	}
	if state.Tokens != nil {
		s.tokens = state.Tokens
	}
	return nil
}

func (s *JSONStore) reconcile() error {
	changed := false

	for name := range s.callers {
		if _, ok := s.tokens[name]; !ok {
			token, err := generateToken()
			if err != nil {
				return fmt.Errorf("generate token for caller %q: %w", name, err)
			}
			s.tokens[name] = token
			changed = true
		}
	}

	for name := range s.tokens {
		if _, ok := s.callers[name]; !ok {
			delete(s.tokens, name)
			changed = true
		}
	}

	if changed {
		return s.persist()
	}
	return nil
}

func (s *JSONStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create caller token directory: %w", err)
	}

	data, err := json.MarshalIndent(stateFile{Tokens: s.tokens}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode caller tokens: %w", err)
	}
	data = append(data, '\n')

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write caller tokens: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace caller tokens: %w", err)
	}
	return nil
}

func generateToken() (string, error) {
	b := make([]byte, 22)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
