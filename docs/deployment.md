# Deployment

## Current deployment

- **Platform:** Coolify
- **Build:** Dockerfile (multi-stage Go build, Alpine runtime)
- **Source:** branch `master`

## Storage

| Mount path | Type | Purpose |
|-----------|------|---------|
| `/data` | persistent volume | Subscription state and auto-generated caller tokens |
| `/etc/gotify-relay/config.yaml` | file mount | YAML config with `${ENV_VAR}` placeholders |

Files in `/data`:
- `subscriptions.json` — per-member channel subscription state
- `caller-tokens.json` — caller tokens (generated once per caller, persisted across restarts)

## Environment variables

Set in Coolify. The config.yaml references these via `${...}` syntax — they are expanded at application startup, not at build time.

| Variable | Purpose |
|----------|---------|
| `GOTIFY_URL` | Gotify server base URL (use internal Docker address when on same server) |
| `<MEMBER>_GOTIFY_APP_TOKEN` | Gotify app token for a member |
| `<MEMBER>_MEMBER_TOKEN` | Relay auth token for a member |

Caller tokens are **not** configured as env vars. They are auto-generated when a caller is first added to config and persisted to `/data/caller-tokens.json`. Tokens are stable across restarts and redeploys. Retrieve them via `GET /callers` with a member token.

## Common operations

### Redeploy after code changes

```bash
coolify deploy uuid <app-uuid>
```

### Update an environment variable

```bash
coolify app env update <app-uuid> <env-key> --value "new-value"
coolify deploy uuid <app-uuid>
```

### Sync all env vars from a .env file

```bash
coolify app env sync <app-uuid> --file .env --is-literal
coolify deploy uuid <app-uuid>
```

### Check status

```bash
coolify app get <app-uuid>
```

### View logs

```bash
coolify app logs <app-uuid>
```

### Restart without rebuilding

```bash
coolify app restart <app-uuid>
```

### Retrieve caller tokens

```bash
curl -H "Authorization: Bearer $MEMBER_TOKEN" https://relay.example.com/callers
```

## Adding a new member

1. Create a Gotify application in the new member's Gotify account. Copy the app token.
2. Generate a relay member token (any random string, e.g. `python3 -c "import secrets; print(secrets.token_urlsafe(16))"`).
3. Add the member to the config.yaml file mount in Coolify (or update the file storage content).
4. Add two env vars: `<NAME>_GOTIFY_APP_TOKEN` and `<NAME>_MEMBER_TOKEN`.
5. Redeploy.

## Adding a new channel

1. Add the channel to the config.yaml under `channels:` with a `default_subscribed` value.
2. Add or update a caller entry under `callers:` that includes the new channel.
3. Redeploy. The caller token is auto-generated — retrieve it via `GET /callers`.

## Adding a new caller

1. Add the caller to config.yaml under `callers:` with its allowed channels.
2. Redeploy. A token is generated for the new caller and persisted.
3. Retrieve the token via `GET /callers` with a member token.
