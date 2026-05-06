# Gotify Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Dockerized Gotify relay that authenticates external callers, lets static members manage channel subscriptions, and fans alerts out to subscribed members.

**Architecture:** A small Go HTTP service loads YAML config, persists member subscription state in JSON, exposes `POST /alert/{channel}` for callers, and exposes subscription endpoints for members. Gotify remains the source of user accounts, client connections, and app tokens.

**Tech Stack:** Go, standard `net/http`, `gopkg.in/yaml.v3`, Docker multi-stage build.

---

## File Structure

- `go.mod`: module and YAML dependency.
- `cmd/gotify-relay/main.go`: process entrypoint, config path/env handling, subscription store setup, server startup.
- `internal/config/config.go`: YAML types, loading, validation, defaults.
- `internal/config/config_test.go`: config behavior tests.
- `internal/subscriptions/store.go`: JSON subscription persistence.
- `internal/subscriptions/store_test.go`: subscription state tests.
- `internal/relay/relay.go`: HTTP handler, caller/member auth, subscription endpoints, fan-out orchestration.
- `internal/relay/gotify.go`: Gotify HTTP client.
- `internal/relay/relay_test.go`: handler behavior tests.
- `internal/relay/gotify_test.go`: Gotify client behavior tests.
- `config.example.yaml`: documented placeholder configuration.
- `Dockerfile`: production image.
- `.dockerignore`: image hygiene.
- `README.md`: architecture, setup, API, deploy, and security notes.

## Tasks

### Task 1: Config

- [x] Write failing config tests for valid member/channel config, environment expansion, missing Gotify URL, unknown caller channels, missing member app tokens, and duplicate tokens.
- [x] Implement config structs, `Load`, and `Validate`.
- [x] Re-run `go test ./internal/config` and confirm pass.

### Task 2: Subscription Store

- [x] Write failing store tests for channel defaults, persistence, subscribed-member lookup, unknown member/channel errors, and parent directory creation.
- [x] Implement JSON subscription store.
- [x] Re-run `go test ./internal/subscriptions` and confirm pass.

### Task 3: Relay Handler

- [x] Write failing handler tests for caller auth, member auth, channel authorization, alert validation, subscribed-member fan-out, empty-channel success, partial failure, and subscription endpoints.
- [x] Implement handler with separate caller/member token lookup and subscription-backed fan-out.
- [x] Re-run `go test ./internal/relay` and confirm pass.

### Task 4: Entrypoint And Packaging

- [x] Wire `cmd/gotify-relay/main.go` to create the subscription store.
- [x] Add Docker writable `/data` directory.
- [x] Update `config.example.yaml` and `README.md`.
- [x] Run `gofmt`, `go test ./...`, `go build ./cmd/gotify-relay`, and Docker build.
