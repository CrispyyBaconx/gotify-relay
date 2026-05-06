package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"

	"gotify-relay/internal/config"
	"gotify-relay/internal/subscriptions"
)

const StatusMultiStatus = 207

type Pusher interface {
	Push(ctx context.Context, memberName string, member config.Member, message Message) error
}

type SubscriptionStore interface {
	List(member string) ([]subscriptions.Subscription, error)
	Set(member string, channel string, subscribed bool) error
	SubscribedMembers(channel string) ([]string, error)
}

type Message struct {
	Title    string         `json:"title,omitempty"`
	Message  string         `json:"message"`
	Priority int            `json:"priority,omitempty"`
	Extras   map[string]any `json:"extras,omitempty"`
}

type AlertResponse struct {
	Channel string         `json:"channel"`
	Results []TargetResult `json:"results"`
}

type TargetResult struct {
	Target string `json:"target"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

type ChannelResponse struct {
	Name              string `json:"name"`
	DefaultSubscribed bool   `json:"default_subscribed"`
}

type SubscriptionsResponse struct {
	Subscriptions []subscriptions.Subscription `json:"subscriptions"`
}

type Handler struct {
	cfg          *config.Config
	store        SubscriptionStore
	pusher       Pusher
	callersByKey map[string]callerAccess
	membersByKey map[string]string
}

type callerAccess struct {
	name     string
	channels map[string]struct{}
}

func NewHandler(cfg *config.Config, store SubscriptionStore, pusher Pusher) http.Handler {
	h := &Handler{
		cfg:          cfg,
		store:        store,
		pusher:       pusher,
		callersByKey: map[string]callerAccess{},
		membersByKey: map[string]string{},
	}
	for name, caller := range cfg.Callers {
		access := callerAccess{name: name, channels: map[string]struct{}{}}
		for _, channel := range caller.Channels {
			access.channels[channel] = struct{}{}
		}
		h.callersByKey[caller.Token] = access
	}
	for name, member := range cfg.Members {
		h.membersByKey[member.Token] = name
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")

	if channel, ok := strings.CutPrefix(path, "alert/"); ok {
		h.handleAlert(w, r, channel)
		return
	}
	if path == "channels" {
		h.handleChannels(w, r)
		return
	}
	if path == "subscriptions" {
		h.handleSubscriptions(w, r)
		return
	}
	if channel, ok := strings.CutPrefix(path, "subscriptions/"); ok {
		h.handleSubscription(w, r, channel)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (h *Handler) handleAlert(w http.ResponseWriter, r *http.Request, channelName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if channelName == "" || strings.Contains(channelName, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	caller, ok := h.authenticateCaller(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if _, ok := h.cfg.Channels[channelName]; !ok {
		writeError(w, http.StatusNotFound, "unknown channel")
		return
	}

	if _, ok := caller.channels[channelName]; !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var message Message
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(message.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	memberNames, err := h.store.SubscribedMembers(channelName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "subscription lookup failed")
		return
	}

	results := h.pushToMembers(r.Context(), memberNames, message)
	status := fanoutStatus(results)
	writeJSON(w, status, AlertResponse{Channel: channelName, Results: results})
}

func (h *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := h.authenticateMember(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	names := make([]string, 0, len(h.cfg.Channels))
	for name := range h.cfg.Channels {
		names = append(names, name)
	}
	sort.Strings(names)

	channels := make([]ChannelResponse, 0, len(names))
	for _, name := range names {
		channels = append(channels, ChannelResponse{
			Name:              name,
			DefaultSubscribed: h.cfg.Channels[name].DefaultSubscribed,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]ChannelResponse{"channels": channels})
}

func (h *Handler) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	memberName, ok := h.authenticateMember(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, err := h.store.List(memberName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "subscription lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, SubscriptionsResponse{Subscriptions: items})
}

func (h *Handler) handleSubscription(w http.ResponseWriter, r *http.Request, channelName string) {
	if channelName == "" || strings.Contains(channelName, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	memberName, ok := h.authenticateMember(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, ok := h.cfg.Channels[channelName]; !ok {
		writeError(w, http.StatusNotFound, "unknown channel")
		return
	}

	var subscribed bool
	switch r.Method {
	case http.MethodPut:
		subscribed = true
	case http.MethodDelete:
		subscribed = false
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.store.Set(memberName, channelName, subscribed); err != nil {
		writeError(w, http.StatusInternalServerError, "subscription update failed")
		return
	}

	writeJSON(w, http.StatusOK, subscriptions.Subscription{
		Channel:    channelName,
		Subscribed: subscribed,
	})
}

func (h *Handler) authenticateCaller(r *http.Request) (callerAccess, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return callerAccess{}, false
	}
	caller, ok := h.callersByKey[token]
	return caller, ok
}

func (h *Handler) authenticateMember(r *http.Request) (string, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return "", false
	}
	member, ok := h.membersByKey[token]
	return member, ok
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

func (h *Handler) pushToMembers(ctx context.Context, memberNames []string, message Message) []TargetResult {
	results := make([]TargetResult, len(memberNames))
	var wg sync.WaitGroup

	for i, memberName := range memberNames {
		i, memberName := i, memberName
		member := h.cfg.Members[memberName]
		wg.Add(1)
		go func() {
			defer wg.Done()

			result := TargetResult{Target: memberName, OK: true}
			if err := h.pusher.Push(ctx, memberName, member, message); err != nil {
				result.OK = false
				result.Error = err.Error()
			}
			results[i] = result
		}()
	}

	wg.Wait()
	return results
}

func fanoutStatus(results []TargetResult) int {
	if len(results) == 0 {
		return http.StatusOK
	}

	failures := 0
	for _, result := range results {
		if !result.OK {
			failures++
		}
	}
	if failures == 0 {
		return http.StatusOK
	}
	if failures == len(results) {
		return http.StatusBadGateway
	}
	return StatusMultiStatus
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
