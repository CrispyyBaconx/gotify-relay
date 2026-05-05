package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gotify-relay/internal/config"
)

func TestHandlerRejectsMissingBearerToken(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "infra", "", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerAuthenticatesBeforeRevealingGroupExistence(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "missing", "", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsUnknownBearerToken(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "infra", "wrong-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsCallerWithoutGroupAccess(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "personal", "infra-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsUnknownGroup(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "missing", "infra-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsInvalidJSON(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "infra", "infra-token", `{"message":`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsEmptyMessage(t *testing.T) {
	rr := postAlert(t, testHandler(nil), "infra", "infra-token", `{"title":"No body"}`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerFansOutToGroupTargets(t *testing.T) {
	pusher := &recordingPusher{}
	rr := postAlert(t, testHandler(pusher), "infra", "infra-token", `{"title":"Disk","message":"disk full","priority":8}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(pusher.calls) != 2 {
		t.Fatalf("expected 2 pushes, got %d", len(pusher.calls))
	}
	assertPushedTo(t, pusher.calls, "bacon-phone", "gotify-token-a")
	assertPushedTo(t, pusher.calls, "bow-desktop", "gotify-token-b")
	if pusher.calls[0].message.Message != "disk full" {
		t.Fatalf("unexpected pushed message: %#v", pusher.calls[0].message)
	}

	var body AlertResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Group != "infra" || len(body.Results) != 2 {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestHandlerReturnsMultiStatusForPartialFailure(t *testing.T) {
	pusher := &recordingPusher{failTarget: "bow-desktop"}
	rr := postAlert(t, testHandler(pusher), "infra", "infra-token", `{"message":"disk full"}`)

	if rr.Code != StatusMultiStatus {
		t.Fatalf("expected 207, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "bow-desktop") {
		t.Fatalf("expected failed target name in response, got %s", body)
	}
	if strings.Contains(body, "gotify-token-b") {
		t.Fatalf("response leaked app token: %s", body)
	}
}

func testHandler(pusher Pusher) http.Handler {
	if pusher == nil {
		pusher = &recordingPusher{}
	}

	return NewHandler(&config.Config{
		Gotify: config.GotifyConfig{URL: "https://gotify.example.com"},
		Callers: map[string]config.Caller{
			"infra-app": {
				Token:  "infra-token",
				Groups: []string{"infra"},
			},
		},
		Groups: map[string]config.Group{
			"infra": {
				Targets: []config.Target{
					{Name: "bacon-phone", AppToken: "gotify-token-a"},
					{Name: "bow-desktop", AppToken: "gotify-token-b"},
				},
			},
			"personal": {
				Targets: []config.Target{
					{Name: "bacon-phone", AppToken: "gotify-token-a"},
				},
			},
		},
	}, pusher)
}

func postAlert(t *testing.T, handler http.Handler, group string, token string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/alert/"+group, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

type recordingPusher struct {
	failTarget string
	calls      []pushCall
}

type pushCall struct {
	target  config.Target
	message Message
}

func (p *recordingPusher) Push(_ context.Context, target config.Target, message Message) error {
	p.calls = append(p.calls, pushCall{target: target, message: message})
	if target.Name == p.failTarget {
		return errors.New("push failed")
	}
	return nil
}

func assertPushedTo(t *testing.T, calls []pushCall, name string, token string) {
	t.Helper()

	for _, call := range calls {
		if call.target.Name == name && call.target.AppToken == token {
			return
		}
	}
	t.Fatalf("expected push to %q with token %q, got %#v", name, token, calls)
}
