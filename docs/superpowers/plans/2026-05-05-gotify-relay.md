# Gotify Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Dockerized Gotify relay that authenticates external callers and fans alerts out to configured Gotify application tokens.

**Architecture:** A small Go HTTP service loads YAML config, exposes `POST /alert/{group}`, checks bearer-token authorization, and sends Gotify `/message` requests in parallel. Gotify remains the source of user accounts, client connections, and app tokens.

**Tech Stack:** Go, standard `net/http`, `gopkg.in/yaml.v3`, Docker multi-stage build.

---

## File Structure

- `go.mod`: module and YAML dependency.
- `cmd/gotify-relay/main.go`: process entrypoint, config path/env handling, server startup.
- `internal/config/config.go`: YAML types, loading, validation, defaults.
- `internal/config/config_test.go`: config behavior tests.
- `internal/relay/relay.go`: HTTP handler, auth, fan-out orchestration.
- `internal/relay/gotify.go`: Gotify HTTP client.
- `internal/relay/relay_test.go`: handler and Gotify client behavior tests.
- `config.example.yaml`: documented placeholder configuration.
- `Dockerfile`: production image.
- `.dockerignore`: image hygiene.
- `README.md`: architecture, setup, API, deploy, and security notes.

## Tasks

### Task 1: Config

- [ ] Write failing config tests for valid config loading, missing Gotify URL, duplicate/empty targets, and caller groups that do not exist.
- [ ] Run `go test ./internal/config` and confirm the expected compile/failure state.
- [ ] Implement config structs, `Load`, and `Validate`.
- [ ] Re-run `go test ./internal/config` and confirm pass.

### Task 2: Relay Handler

- [ ] Write failing handler tests for unauthorized, forbidden group, unknown group, invalid JSON, success fan-out, and partial failure.
- [ ] Run `go test ./internal/relay` and confirm expected failure.
- [ ] Implement handler with bearer-token lookup, group authorization, request validation, and parallel target pushes.
- [ ] Re-run `go test ./internal/relay` and confirm pass.

### Task 3: Gotify Client

- [ ] Write failing test proving Gotify requests use `POST /message?token=<app-token>` with the expected JSON body.
- [ ] Run the targeted test and confirm failure.
- [ ] Implement the Gotify HTTP client.
- [ ] Re-run the targeted test and confirm pass.

### Task 4: Entrypoint And Packaging

- [ ] Add `cmd/gotify-relay/main.go`.
- [ ] Add `Dockerfile`, `.dockerignore`, and `config.example.yaml`.
- [ ] Add README sections for architecture, Gotify setup, relay config, API examples, Docker run, Coolify-style deploy notes, and security.
- [ ] Run `gofmt`, `go test ./...`, and `go build ./cmd/gotify-relay`.
