# Gotify Relay

Gotify Relay is a small HTTP service that sits in front of a Gotify server. External applications authenticate once to the relay, post to a named alert group, and the relay fans that alert out to the Gotify application tokens configured for that group.

This is useful when Gotify users keep separate accounts and devices, but external senders should not need to know every user's Gotify app token.

## Architecture

Gotify already handles users, clients, messages, and device streams. This relay only handles routing:

1. Each user has their own Gotify account.
2. Each user creates one Gotify application for each channel/group they want to receive.
3. The relay config maps groups, such as `infra` or `personal`, to those Gotify app tokens.
4. External applications receive one relay token.
5. External applications call `POST /alert/{group}`.
6. The relay checks the relay token, verifies it may send to that group, and pushes to Gotify's `/message` API for every configured target.

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

callers:
  uptime-kuma:
    token: "${UPTIME_KUMA_RELAY_TOKEN}"
    groups: ["infra"]

groups:
  infra:
    targets:
      - name: "bacon-infra"
        app_token: "${BACON_INFRA_GOTIFY_APP_TOKEN}"
      - name: "bow-infra"
        app_token: "${BOW_INFRA_GOTIFY_APP_TOKEN}"
```

Do not commit real relay tokens or Gotify app tokens.

## Gotify Setup

For each user and group:

1. Log in to Gotify as that user.
2. Create an application named after the group, such as `infra`.
3. Copy that application's token.
4. Add it under the matching relay group as a target.

If a user wants to stop receiving a group, remove their target from the relay config or delete their Gotify application token.

## API

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

Responses:

- `200 OK`: all targets accepted the message.
- `207 Multi-Status`: some targets failed and some succeeded.
- `400 Bad Request`: invalid JSON or missing `message`.
- `401 Unauthorized`: missing or unknown bearer token.
- `403 Forbidden`: caller token is valid but cannot send to the group.
- `404 Not Found`: group or route does not exist.
- `502 Bad Gateway`: every Gotify push failed.

Response bodies include target names and failure status, but never app tokens.

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

Run with a mounted config:

```bash
docker run --rm -p 8080:8080 \
  --env-file .env \
  -v "$PWD/config.yaml:/etc/gotify-relay/config.yaml:ro" \
  gotify-relay
```

For Coolify or similar platforms, deploy this repo as a Dockerfile service, set the environment variables referenced by `config.yaml`, and mount/provide the config at `/etc/gotify-relay/config.yaml`. If the relay and Gotify share an internal Docker network, `gotify.url` can be the internal service URL instead of the public domain.

## Security Notes

- Treat relay tokens like passwords.
- Treat Gotify app tokens like passwords.
- Use HTTPS for public relay endpoints.
- Give each external application its own relay token.
- Restrict each caller to only the groups it needs.
- Rotate a caller token by changing the relay config and restarting the service.
- Do not expose Gotify app tokens to external applications; only the relay needs them.

## Development

```bash
go test ./...
go build ./cmd/gotify-relay
```
