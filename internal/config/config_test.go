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

members:
  bacon:
    token: "bacon-member-token"
    app_token: "bacon-gotify-token"

channels:
  infra:
    default_subscribed: true

callers:
  uptime-kuma:
    token: "relay-token"
    channels: ["infra"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.ListenAddr != ":8080" {
		t.Fatalf("expected default listen addr :8080, got %q", cfg.Server.ListenAddr)
	}
	if cfg.Subscriptions.Path != "/data/subscriptions.json" {
		t.Fatalf("expected default subscription path, got %q", cfg.Subscriptions.Path)
	}
	if cfg.Gotify.URL != "https://gotify.example.com" {
		t.Fatalf("unexpected gotify url: %q", cfg.Gotify.URL)
	}
	if cfg.Members["bacon"].Token != "bacon-member-token" {
		t.Fatalf("member token was not loaded")
	}
	if !cfg.Channels["infra"].DefaultSubscribed {
		t.Fatalf("channel default was not loaded")
	}
	if cfg.Callers["uptime-kuma"].Token != "relay-token" {
		t.Fatalf("caller token was not loaded")
	}
}

func TestLoadExpandsEnvironmentVariables(t *testing.T) {
	t.Setenv("GOTIFY_URL", "https://gotify.example.com")
	t.Setenv("RELAY_TOKEN", "relay-token")
	t.Setenv("MEMBER_TOKEN", "member-token")
	t.Setenv("APP_TOKEN", "gotify-app-token")
	path := writeConfig(t, `
gotify:
  url: "${GOTIFY_URL}"

members:
  bacon:
    token: "${MEMBER_TOKEN}"
    app_token: "${APP_TOKEN}"

channels:
  infra:
    default_subscribed: true

callers:
  uptime-kuma:
    token: "${RELAY_TOKEN}"
    channels: ["infra"]
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
	if cfg.Members["bacon"].Token != "member-token" {
		t.Fatalf("expected env-expanded member token")
	}
	if cfg.Members["bacon"].AppToken != "gotify-app-token" {
		t.Fatalf("expected env-expanded app token")
	}
}

func TestLoadRejectsMissingGotifyURL(t *testing.T) {
	path := writeConfig(t, `
members:
  bacon:
    token: "member-token"
    app_token: "gotify-token"
channels:
  infra:
    default_subscribed: true
callers:
  app:
    token: "relay-token"
    channels: ["infra"]
`)

	_, err := Load(path)
	assertErrorContains(t, err, "gotify.url")
}

func TestLoadRejectsCallerChannelThatDoesNotExist(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
members:
  bacon:
    token: "member-token"
    app_token: "gotify-token"
channels:
  infra:
    default_subscribed: true
callers:
  app:
    token: "relay-token"
    channels: ["missing"]
`)

	_, err := Load(path)
	assertErrorContains(t, err, "unknown channel")
}

func TestLoadRejectsMemberWithoutAppToken(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
members:
  bacon:
    token: "member-token"
    app_token: ""
channels:
  infra:
    default_subscribed: true
callers:
  app:
    token: "relay-token"
    channels: ["infra"]
`)

	_, err := Load(path)
	assertErrorContains(t, err, "app_token")
}

func TestLoadRejectsDuplicateTokensAcrossMembersAndCallers(t *testing.T) {
	path := writeConfig(t, `
gotify:
  url: "https://gotify.example.com"
members:
  bacon:
    token: "shared-token"
    app_token: "gotify-token"
channels:
  infra:
    default_subscribed: true
callers:
  app:
    token: "shared-token"
    channels: ["infra"]
`)

	_, err := Load(path)
	assertErrorContains(t, err, "duplicates")
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
