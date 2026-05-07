package callers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gotify-relay/internal/config"
)

func TestNewStoreGeneratesTokensForNewCallers(t *testing.T) {
	store := testStore(t, nil)

	info := store.List()
	if len(info) != 1 {
		t.Fatalf("expected 1 caller, got %d", len(info))
	}
	if info[0].Name != "test-app" {
		t.Fatalf("expected caller name test-app, got %q", info[0].Name)
	}
	if info[0].Token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestNewStoreLoadsExistingTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	existing := stateFile{Tokens: map[string]string{"test-app": "pre-existing-token"}}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	token, ok := store.Token("test-app")
	if !ok || token != "pre-existing-token" {
		t.Fatalf("expected pre-existing-token, got %q (ok=%v)", token, ok)
	}
}

func TestNewStorePrunesRemovedCallers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	existing := stateFile{Tokens: map[string]string{
		"test-app": "keep-this",
		"removed":  "prune-this",
	}}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	if _, ok := store.Token("removed"); ok {
		t.Fatal("expected removed caller to be pruned")
	}
	if token, ok := store.Token("test-app"); !ok || token != "keep-this" {
		t.Fatalf("expected keep-this, got %q", token)
	}
}

func TestByTokenFindsCallerByToken(t *testing.T) {
	store := testStore(t, nil)

	info := store.List()
	token := info[0].Token

	name, ok := store.ByToken(token)
	if !ok || name != "test-app" {
		t.Fatalf("expected test-app, got %q (ok=%v)", name, ok)
	}

	_, ok = store.ByToken("nonexistent")
	if ok {
		t.Fatal("expected ByToken to return false for unknown token")
	}
}

func TestStorePersistsTokensToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	store1, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("first NewJSONStore: %v", err)
	}
	token1, _ := store1.Token("test-app")

	store2, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("second NewJSONStore: %v", err)
	}
	token2, _ := store2.Token("test-app")

	if token1 != token2 {
		t.Fatalf("token changed across restarts: %q != %q", token1, token2)
	}
}

func testStore(t *testing.T, existing *stateFile) *JSONStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	if existing != nil {
		data, _ := json.Marshal(existing)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	store, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	return store
}

func testConfig() *config.Config {
	return &config.Config{
		Gotify:   config.GotifyConfig{URL: "http://localhost"},
		Members:  map[string]config.Member{"alice": {Token: "m1", AppToken: "a1"}},
		Channels: map[string]config.Channel{"alerts": {DefaultSubscribed: true}},
		Callers:  map[string]config.Caller{"test-app": {Channels: []string{"alerts"}}},
	}
}
