# Gotify Relay Design

## Goal

Build a small HTTP relay that lets external applications authenticate once, post an alert to a named group, and have that alert delivered to the matching Gotify application tokens for individual users.

## Architecture

The relay runs as a separate service in front of an existing Gotify server. Callers send `POST /alert/{group}` with a relay bearer token. The relay validates that the caller token is allowed to send to the requested group, then pushes the message to all Gotify application tokens configured for that group.

Gotify remains the notification server and user/device registry. Each Gotify user keeps their own account and creates one Gotify application token per channel they want to receive. The relay owns only routing, caller authentication, and fan-out.

## Configuration

Configuration is a local YAML file:

- `gotify.url` points at the main Gotify server.
- `callers` maps friendly caller names to bearer tokens and allowed groups.
- `groups` maps alert group names to Gotify app-token targets.

Secrets must not be committed. The repository includes an example config with placeholder values.

## HTTP API

`POST /alert/{group}` accepts JSON compatible with Gotify's message API:

```json
{
  "title": "Alert title",
  "message": "Alert body",
  "priority": 5,
  "extras": {}
}
```

`Authorization: Bearer <relay-token>` is required. The relay returns `401` for missing or unknown tokens, `403` for a token that cannot send to the group, `404` for unknown groups, and `207` when fan-out partially succeeds.

## Error Handling

The relay validates configuration at startup. Runtime push failures are isolated per target so one user's broken Gotify app token does not block the rest of the group. Responses include target names and status only, never secret tokens.

## Testing

Tests cover config validation, auth behavior, group authorization, JSON validation, Gotify request shape, full fan-out success, and partial fan-out failure.
