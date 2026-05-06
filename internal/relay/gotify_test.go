package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotify-relay/internal/config"
)

func TestGotifyClientPushPostsMessageWithAppToken(t *testing.T) {
	var gotPath string
	var gotToken string
	var gotMessage Message

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.URL.Query().Get("token")
		if err := json.NewDecoder(r.Body).Decode(&gotMessage); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewGotifyClient(server.URL, server.Client())
	err := client.Push(t.Context(), "bacon", config.Member{
		AppToken: "gotify-app-token",
	}, Message{
		Title:    "Disk",
		Message:  "disk full",
		Priority: 8,
	})
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	if gotPath != "/message" {
		t.Fatalf("expected /message path, got %q", gotPath)
	}
	if gotToken != "gotify-app-token" {
		t.Fatalf("unexpected token query: %q", gotToken)
	}
	if gotMessage.Message != "disk full" || gotMessage.Title != "Disk" || gotMessage.Priority != 8 {
		t.Fatalf("unexpected message body: %#v", gotMessage)
	}
}

func TestGotifyClientPushReturnsErrorForNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token", http.StatusForbidden)
	}))
	defer server.Close()

	client := NewGotifyClient(server.URL, server.Client())
	err := client.Push(t.Context(), "bacon", config.Member{
		AppToken: "secret-gotify-token",
	}, Message{Message: "disk full"})
	if err == nil {
		t.Fatal("expected error")
	}
}
