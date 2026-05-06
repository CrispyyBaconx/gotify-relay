package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gotify-relay/internal/config"
	"gotify-relay/internal/subscriptions"
)

func TestHandlerRejectsMissingBearerToken(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "infra", "", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerAuthenticatesBeforeRevealingChannelExistence(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "missing", "", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsUnknownBearerToken(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "infra", "wrong-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsCallerWithoutChannelAccess(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "personal", "infra-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsUnknownChannel(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "missing", "infra-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsInvalidJSON(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "infra", "infra-token", `{"message":`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsEmptyMessage(t *testing.T) {
	rr := postAlert(t, testHandler(t, nil), "infra", "infra-token", `{"title":"No body"}`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlerFansOutToSubscribedMembersOnly(t *testing.T) {
	pusher := &recordingPusher{}
	store := testStore(t)
	if err := store.Set("bow", "infra", false); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	rr := postAlert(t, testHandlerWithStore(t, pusher, store), "infra", "infra-token", `{"title":"Disk","message":"disk full","priority":8}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(pusher.calls) != 1 {
		t.Fatalf("expected 1 push, got %d", len(pusher.calls))
	}
	assertPushedTo(t, pusher.calls, "bacon", "gotify-token-a")
	if pusher.calls[0].message.Message != "disk full" {
		t.Fatalf("unexpected pushed message: %#v", pusher.calls[0].message)
	}

	var body AlertResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Channel != "infra" || len(body.Results) != 1 {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestHandlerReturnsOKWhenChannelHasNoSubscribers(t *testing.T) {
	pusher := &recordingPusher{}
	store := testStore(t)
	if err := store.Set("bacon", "infra", false); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := store.Set("bow", "infra", false); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	rr := postAlert(t, testHandlerWithStore(t, pusher, store), "infra", "infra-token", `{"message":"disk full"}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(pusher.calls) != 0 {
		t.Fatalf("expected no pushes, got %d", len(pusher.calls))
	}
}

func TestHandlerReturnsMultiStatusForPartialFailure(t *testing.T) {
	pusher := &recordingPusher{failTarget: "bow"}
	rr := postAlert(t, testHandler(t, pusher), "infra", "infra-token", `{"message":"disk full"}`)

	if rr.Code != StatusMultiStatus {
		t.Fatalf("expected 207, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "bow") {
		t.Fatalf("expected failed member name in response, got %s", body)
	}
	if strings.Contains(body, "gotify-token-b") {
		t.Fatalf("response leaked app token: %s", body)
	}
}

func TestHandlerListsMemberSubscriptions(t *testing.T) {
	rr := get(t, testHandler(t, nil), "/subscriptions", "bacon-member-token")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"channel":"infra"`) || !strings.Contains(body, `"subscribed":true`) {
		t.Fatalf("unexpected response: %s", body)
	}
	if !strings.Contains(body, `"channel":"deploys"`) || !strings.Contains(body, `"subscribed":false`) {
		t.Fatalf("unexpected response: %s", body)
	}
}

func TestHandlerListsChannels(t *testing.T) {
	rr := get(t, testHandler(t, nil), "/channels", "bacon-member-token")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"name":"deploys"`) || !strings.Contains(body, `"default_subscribed":false`) {
		t.Fatalf("unexpected response: %s", body)
	}
	if !strings.Contains(body, `"name":"infra"`) || !strings.Contains(body, `"default_subscribed":true`) {
		t.Fatalf("unexpected response: %s", body)
	}
}

func TestHandlerUpdatesMemberSubscription(t *testing.T) {
	store := testStore(t)
	handler := testHandlerWithStore(t, nil, store)

	put := httptest.NewRequest(http.MethodPut, "/subscriptions/deploys", nil)
	put.Header.Set("Authorization", "Bearer bacon-member-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, put)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	list, err := store.List("bacon")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	assertSubscription(t, list, "deploys", true)
}

func TestHandlerDeletesMemberSubscription(t *testing.T) {
	store := testStore(t)
	handler := testHandlerWithStore(t, nil, store)

	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/infra", nil)
	req.Header.Set("Authorization", "Bearer bacon-member-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	list, err := store.List("bacon")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	assertSubscription(t, list, "infra", false)
}

func TestHandlerRejectsCallerTokenForSubscriptionRoutes(t *testing.T) {
	rr := get(t, testHandler(t, nil), "/subscriptions", "infra-token")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func testHandler(t *testing.T, pusher Pusher) http.Handler {
	t.Helper()
	return testHandlerWithStore(t, pusher, testStore(t))
}

func testHandlerWithStore(t *testing.T, pusher Pusher, store SubscriptionStore) http.Handler {
	t.Helper()
	if pusher == nil {
		pusher = &recordingPusher{}
	}

	return NewHandler(testConfig(), store, pusher)
}

func testConfig() *config.Config {
	return &config.Config{
		Gotify: config.GotifyConfig{URL: "https://gotify.example.com"},
		Callers: map[string]config.Caller{
			"infra-app": {
				Token:    "infra-token",
				Channels: []string{"infra"},
			},
		},
		Members: map[string]config.Member{
			"bacon": {Token: "bacon-member-token", AppToken: "gotify-token-a"},
			"bow":   {Token: "bow-member-token", AppToken: "gotify-token-b"},
		},
		Channels: map[string]config.Channel{
			"deploys": {DefaultSubscribed: false},
			"infra":   {DefaultSubscribed: true},
			"personal": {
				DefaultSubscribed: false,
			},
		},
	}
}

func testStore(t *testing.T) *subscriptions.JSONStore {
	t.Helper()

	store, err := subscriptions.NewJSONStore(t.TempDir()+"/subscriptions.json", testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore returned error: %v", err)
	}
	return store
}

func postAlert(t *testing.T, handler http.Handler, channel string, token string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/alert/"+channel, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func get(t *testing.T, handler http.Handler, path string, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

type recordingPusher struct {
	failTarget string
	mu         sync.Mutex
	calls      []pushCall
}

type pushCall struct {
	member  config.Member
	name    string
	message Message
}

func (p *recordingPusher) Push(_ context.Context, memberName string, member config.Member, message Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, pushCall{name: memberName, member: member, message: message})
	if memberName == p.failTarget {
		return errors.New("push failed")
	}
	return nil
}

func assertPushedTo(t *testing.T, calls []pushCall, name string, token string) {
	t.Helper()

	for _, call := range calls {
		if call.name == name && call.member.AppToken == token {
			return
		}
	}
	t.Fatalf("expected push to %q with token %q, got %#v", name, token, calls)
}

func assertSubscription(t *testing.T, list []subscriptions.Subscription, channel string, subscribed bool) {
	t.Helper()

	for _, item := range list {
		if item.Channel == channel {
			if item.Subscribed != subscribed {
				t.Fatalf("expected %s subscribed=%v, got %v", channel, subscribed, item.Subscribed)
			}
			return
		}
	}
	t.Fatalf("missing subscription for %s in %#v", channel, list)
}
