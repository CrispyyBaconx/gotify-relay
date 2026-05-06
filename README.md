# Gotify Relay

Gotify Relay is a small HTTP service that sits in front of a Gotify server. External applications authenticate once to the relay, post to a named channel, and the relay fans that alert out only to members currently subscribed to that channel.

This is useful when Gotify users keep separate accounts and devices, but external senders should not need to know every user's Gotify app token or subscription preferences.

## Architecture

Gotify already handles users, clients, messages, and device streams. This relay handles routing and lightweight subscription state:

1. Each member has their own Gotify account.
2. Each member creates one Gotify application for relay-delivered notifications.
3. The relay config keeps the static member list, channel list, and external caller tokens.
4. The relay stores per-member channel subscriptions in a JSON file.
5. External applications call `POST /alert/{channel}` with a caller token.
6. Members call subscription endpoints with their member token to opt in or out.
7. The relay pushes alerts to Gotify only for subscribed members.

The relay is not a Gotify plugin and does not replace Gotify auth. It is a separate service you can run beside Gotify, behind the same reverse proxy if desired.

## Configuration

Copy the example config:

```bash
cp config.example.yaml config.yaml
```

The loader expands environment variables before parsing YAML, so secrets can stay in your deployment environment:

```yaml
gotify:
  url: "${GOTIFY_URL}"

subscriptions:
  path: "/data/subscriptions.json"

members:
  bacon:
    token: "${BACON_MEMBER_TOKEN}"
    app_token: "${BACON_GOTIFY_APP_TOKEN}"

channels:
  infra:
    default_subscribed: true
  deploys:
    default_subscribed: false

callers:
  uptime-kuma:
    token: "${UPTIME_KUMA_RELAY_TOKEN}"
    channels: ["infra"]
```

Do not commit real relay tokens, member tokens, or Gotify app tokens.

## Gotify Setup

For each member:

1. Log in to Gotify as that member.
2. Create one application, such as `relay`.
3. Copy that application's token.
4. Add the token under `members.<name>.app_token`.
5. Give the member their private `members.<name>.token` for subscription management.

If a member should no longer receive relay notifications at all, remove them from `members` and restart the relay.

## Subscription State

Subscriptions are stored in the JSON file configured by `subscriptions.path`. Missing member/channel entries are initialized from each channel's `default_subscribed` value when the relay starts.

Mount `/data` as persistent storage in Docker or Coolify so subscription changes survive redeploys.

## Alert API

Send an alert:

```bash
curl -X POST "https://relay.example.com/alert/infra" \
  -H "Authorization: Bearer $UPTIME_KUMA_RELAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Disk space",
    "message": "root filesystem is above 90%",
    "priority": 8
  }'
```

The JSON body matches Gotify's message shape:

```json
{
  "title": "Optional title",
  "message": "Required message",
  "priority": 5,
  "extras": {}
}
```

Alert responses:

- `200 OK`: all subscribed members accepted the message, or no members are subscribed.
- `207 Multi-Status`: some subscribed members failed and some succeeded.
- `400 Bad Request`: invalid JSON or missing `message`.
- `401 Unauthorized`: missing or unknown caller token.
- `403 Forbidden`: caller token is valid but cannot send to the channel.
- `404 Not Found`: channel or route does not exist.
- `502 Bad Gateway`: every Gotify push failed.

Response bodies include member names and failure status, but never app tokens.

## Member API

Member endpoints require a member token:

```bash
curl -H "Authorization: Bearer $BACON_MEMBER_TOKEN" \
  "https://relay.example.com/channels"
```

List current subscriptions:

```bash
curl -H "Authorization: Bearer $BACON_MEMBER_TOKEN" \
  "https://relay.example.com/subscriptions"
```

Subscribe:

```bash
curl -X PUT \
  -H "Authorization: Bearer $BACON_MEMBER_TOKEN" \
  "https://relay.example.com/subscriptions/infra"
```

Unsubscribe:

```bash
curl -X DELETE \
  -H "Authorization: Bearer $BACON_MEMBER_TOKEN" \
  "https://relay.example.com/subscriptions/infra"
```

Caller tokens cannot use member endpoints.

## Running Locally

```bash
go test ./...
go run ./cmd/gotify-relay -config config.yaml
```

By default the service listens on `:8080`. You can set `server.listen_addr` in the config or override it with `LISTEN_ADDR`.

Health check:

```bash
curl -i http://localhost:8080/healthz
```

## Docker

Build:

```bash
docker build -t gotify-relay .
```

Run with a mounted config and persistent subscription storage:

```bash
docker run --rm -p 8080:8080 \
  --env-file .env \
  -v "$PWD/config.yaml:/etc/gotify-relay/config.yaml:ro" \
  -v gotify-relay-data:/data \
  gotify-relay
```

For Coolify or similar platforms, deploy this repo as a Dockerfile service, set the environment variables referenced by `config.yaml`, mount/provide the config at `/etc/gotify-relay/config.yaml`, and add persistent storage at `/data`. If the relay and Gotify share an internal Docker network, `gotify.url` can be the internal service URL instead of the public domain.

## Security Notes

- Treat caller tokens like passwords.
- Treat member tokens like passwords.
- Treat Gotify app tokens like passwords.
- Use HTTPS for public relay endpoints.
- Give each external application its own caller token.
- Restrict each caller to only the channels it needs.
- Rotate a token by changing the relay config and restarting the service.
- Do not expose Gotify app tokens to external applications; only the relay needs them.

## Development

```bash
go test ./...
go build ./cmd/gotify-relay
```
