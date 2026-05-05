package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"gotify-relay/internal/config"
)

const StatusMultiStatus = 207

type Pusher interface {
	Push(ctx context.Context, target config.Target, message Message) error
}

type Message struct {
	Title    string         `json:"title,omitempty"`
	Message  string         `json:"message"`
	Priority int            `json:"priority,omitempty"`
	Extras   map[string]any `json:"extras,omitempty"`
}

type AlertResponse struct {
	Group   string         `json:"group"`
	Results []TargetResult `json:"results"`
}

type TargetResult struct {
	Target string `json:"target"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

type Handler struct {
	cfg          *config.Config
	pusher       Pusher
	callersByKey map[string]callerAccess
}

type callerAccess struct {
	name   string
	groups map[string]struct{}
}

func NewHandler(cfg *config.Config, pusher Pusher) http.Handler {
	h := &Handler{
		cfg:          cfg,
		pusher:       pusher,
		callersByKey: map[string]callerAccess{},
	}
	for name, caller := range cfg.Callers {
		access := callerAccess{name: name, groups: map[string]struct{}{}}
		for _, group := range caller.Groups {
			access.groups[group] = struct{}{}
		}
		h.callersByKey[caller.Token] = access
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	groupName, ok := strings.CutPrefix(strings.Trim(r.URL.Path, "/"), "alert/")
	if !ok || groupName == "" || strings.Contains(groupName, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	caller, ok := h.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	group, ok := h.cfg.Groups[groupName]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown group")
		return
	}

	if _, ok := caller.groups[groupName]; !ok {
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

	results := h.pushToTargets(r.Context(), group.Targets, message)
	status := fanoutStatus(results)
	writeJSON(w, status, AlertResponse{Group: groupName, Results: results})
}

func (h *Handler) authenticate(r *http.Request) (callerAccess, bool) {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return callerAccess{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if token == "" {
		return callerAccess{}, false
	}
	caller, ok := h.callersByKey[token]
	return caller, ok
}

func (h *Handler) pushToTargets(ctx context.Context, targets []config.Target, message Message) []TargetResult {
	results := make([]TargetResult, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		i, target := i, target
		wg.Add(1)
		go func() {
			defer wg.Done()

			result := TargetResult{Target: target.Name, OK: true}
			if err := h.pusher.Push(ctx, target, message); err != nil {
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
