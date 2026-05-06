# Gotify Relay Design

## Goal

Build a small HTTP relay that lets external applications authenticate once, post an alert to a named channel, and have that alert delivered only to static members currently subscribed to that channel.

## Architecture

The relay runs as a separate service in front of an existing Gotify server. Callers send `POST /alert/{channel}` with a caller bearer token. The relay validates that the caller token is allowed to send to the requested channel, reads subscription state, then pushes the message to the Gotify app token for each subscribed member.

Gotify remains the notification server and user/device registry. Each member keeps their own Gotify account and creates one Gotify application token for relay-delivered notifications. The relay owns only routing, caller authentication, member subscription state, and fan-out.

## Configuration

Configuration is a local YAML file:

- `gotify.url` points at the main Gotify server.
- `members` maps static member names to member tokens and Gotify app tokens.
- `channels` maps channel names to default subscription state.
- `callers` maps friendly caller names to bearer tokens and allowed channels.
- `subscriptions.path` points at a JSON file used for dynamic member subscription state.

Secrets must not be committed. The repository includes an example config with placeholder values.

## HTTP API

`POST /alert/{channel}` accepts JSON compatible with Gotify's message API:

```json
{
  "title": "Alert title",
  "message": "Alert body",
  "priority": 5,
  "extras": {}
}
```

Member tokens can call:

- `GET /channels`
- `GET /subscriptions`
- `PUT /subscriptions/{channel}`
- `DELETE /subscriptions/{channel}`

Caller tokens cannot manage subscriptions.

## Error Handling

The relay validates configuration at startup. Runtime push failures are isolated per member so one user's broken Gotify app token does not block the rest of the channel. Responses include member names and status only, never secret tokens.

## Testing

Tests cover config validation, subscription persistence, auth behavior, channel authorization, JSON validation, Gotify request shape, subscribed-member fan-out, empty-channel success, partial fan-out failure, and member subscription endpoints.
