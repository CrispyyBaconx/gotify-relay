package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogRequestsSkipsHealthChecks(t *testing.T) {
	var logs bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})

	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(httptest.NewRecorder(), health)
	if strings.Contains(logs.String(), "/healthz") {
		t.Fatalf("expected health check to be omitted from logs, got %q", logs.String())
	}

	other := httptest.NewRequest(http.MethodPost, "/alerts/infra", nil)
	handler.ServeHTTP(httptest.NewRecorder(), other)
	if !strings.Contains(logs.String(), "POST /alerts/infra") {
		t.Fatalf("expected non-health request to be logged, got %q", logs.String())
	}
}
