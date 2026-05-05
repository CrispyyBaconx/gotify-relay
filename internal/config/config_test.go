package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"

callers:
  uptime-kuma:
    token: "relay-token"
    groups: ["infra"]

groups:
  infra:
    targets:
      - name: "bacon-phone"
        app_token: "gotify-app-token"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.ListenAddr != ":8080" {
		t.Fatalf("expected default listen addr :8080, got %q", cfg.Server.ListenAddr)
	}
	if cfg.Gotify.URL != "https://gotify.example.com" {
		t.Fatalf("unexpected gotify url: %q", cfg.Gotify.URL)
	}
	if cfg.Callers["uptime-kuma"].Token != "relay-token" {
		t.Fatalf("caller token was not loaded")
	}
	if got := cfg.Groups["infra"].Targets[0].Name; got != "bacon-phone" {
		t.Fatalf("unexpected target name: %q", got)
	}
}

func TestLoadExpandsEnvironmentVariables(t *testing.T) {
	t.Setenv("GOTIFY_URL", "https://gotify.example.com")
	t.Setenv("RELAY_TOKEN", "relay-token")
	t.Setenv("APP_TOKEN", "gotify-app-token")
	path := writeConfig(t, `
gotify:
  url: "${GOTIFY_URL}"

callers:
  uptime-kuma:
    token: "${RELAY_TOKEN}"
    groups: ["infra"]

groups:
  infra:
    targets:
      - name: "bacon-phone"
        app_token: "${APP_TOKEN}"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Gotify.URL != "https://gotify.example.com" {
		t.Fatalf("expected env-expanded Gotify URL, got %q", cfg.Gotify.URL)
	}
	if cfg.Callers["uptime-kuma"].Token != "relay-token" {
		t.Fatalf("expected env-expanded relay token")
	}
	if cfg.Groups["infra"].Targets[0].AppToken != "gotify-app-token" {
		t.Fatalf("expected env-expanded app token")
	}
}

func TestLoadRejectsMissingGotifyURL(t *testing.T) {
	path := writeConfig(t, `
callers:
  app:
    token: "relay-token"
    groups: ["infra"]
groups:
  infra:
    targets:
      - name: "bacon"
        app_token: "gotify-token"
`)

	_, err := Load(path)
	assertErrorContains(t, err, "gotify.url")
}

func TestLoadRejectsCallerGroupThatDoesNotExist(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
callers:
  app:
    token: "relay-token"
    groups: ["missing"]
groups:
  infra:
    targets:
      - name: "bacon"
        app_token: "gotify-token"
`)

	_, err := Load(path)
	assertErrorContains(t, err, "unknown group")
}

func TestLoadRejectsTargetWithoutAppToken(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
callers:
  app:
    token: "relay-token"
    groups: ["infra"]
groups:
  infra:
    targets:
      - name: "bacon"
        app_token: ""
`)

	_, err := Load(path)
	assertErrorContains(t, err, "app_token")
}

func TestLoadRejectsDuplicateTargetNamesInGroup(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
callers:
  app:
    token: "relay-token"
    groups: ["infra"]
groups:
  infra:
    targets:
      - name: "bacon"
        app_token: "gotify-token-1"
      - name: "bacon"
        app_token: "gotify-token-2"
`)

	_, err := Load(path)
	assertErrorContains(t, err, "duplicate target")
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}
