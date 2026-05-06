package subscriptions

import (
	"os"
	"path/filepath"
	"testing"

	"gotify-relay/internal/config"
)

func TestStoreInitializesChannelDefaultsForMembers(t *testing.T) {
	store := newTestStore(t)

	list, err := store.List("bacon")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	assertSubscription(t, list, "infra", true)
	assertSubscription(t, list, "deploys", false)
}

func TestStorePersistsSubscriptionChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	cfg := testConfig()

	store, err := NewJSONStore(path, cfg)
	if err != nil {
		t.Fatalf("NewJSONStore returned error: %v", err)
	}
	if err := store.Set("bacon", "deploys", true); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	reloaded, err := NewJSONStore(path, cfg)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	list, err := reloaded.List("bacon")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	assertSubscription(t, list, "deploys", true)
}

func TestStoreReturnsSubscribedMembersForChannel(t *testing.T) {
	store := newTestStore(t)

	if err := store.Set("bow", "infra", false); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := store.Set("bacon", "deploys", true); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	infra, err := store.SubscribedMembers("infra")
	if err != nil {
		t.Fatalf("SubscribedMembers returned error: %v", err)
	}
	if len(infra) != 1 || infra[0] != "bacon" {
		t.Fatalf("expected only bacon subscribed to infra, got %#v", infra)
	}

	deploys, err := store.SubscribedMembers("deploys")
	if err != nil {
		t.Fatalf("SubscribedMembers returned error: %v", err)
	}
	if len(deploys) != 1 || deploys[0] != "bacon" {
		t.Fatalf("expected only bacon subscribed to deploys, got %#v", deploys)
	}
}

func TestStoreRejectsUnknownMemberOrChannel(t *testing.T) {
	store := newTestStore(t)

	if err := store.Set("missing", "infra", true); err == nil {
		t.Fatal("expected unknown member error")
	}
	if err := store.Set("bacon", "missing", true); err == nil {
		t.Fatal("expected unknown channel error")
	}
	if _, err := store.List("missing"); err == nil {
		t.Fatal("expected unknown member error")
	}
	if _, err := store.SubscribedMembers("missing"); err == nil {
		t.Fatal("expected unknown channel error")
	}
}

func TestStoreCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subscriptions.json")

	store, err := NewJSONStore(path, testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore returned error: %v", err)
	}
	if err := store.Set("bacon", "deploys", true); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted subscriptions file: %v", err)
	}
}

func newTestStore(t *testing.T) *JSONStore {
	t.Helper()

	store, err := NewJSONStore(filepath.Join(t.TempDir(), "subscriptions.json"), testConfig())
	if err != nil {
		t.Fatalf("NewJSONStore returned error: %v", err)
	}
	return store
}

func testConfig() *config.Config {
	return &config.Config{
		Members: map[string]config.Member{
			"bacon": {Token: "bacon-member-token", AppToken: "bacon-gotify-token"},
			"bow":   {Token: "bow-member-token", AppToken: "bow-gotify-token"},
		},
		Channels: map[string]config.Channel{
			"infra":   {DefaultSubscribed: true},
			"deploys": {DefaultSubscribed: false},
		},
	}
}

func assertSubscription(t *testing.T, list []Subscription, channel string, subscribed bool) {
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
