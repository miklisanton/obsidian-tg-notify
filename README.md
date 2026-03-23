# obsidian-tg-notify

## CI/CD and compose deploy

This repo now supports a GHCR-backed Docker Compose deploy flow. CI tests and builds the Go binaries. CD builds one app image, pushes it to GHCR, then the server pulls that image and runs both app and Postgres via Compose. Deploy also runs `seed-default-rules` as a one-off Compose service.

### 1. One-time server bootstrap

Run once as root on the server:

```bash
./deploy/scripts/bootstrap-server-root.sh obsidian
```

This creates the `obsidian` user if missing, prepares app dirs, copies the prod `compose.yaml`, adds the deploy user to the `docker` group, and disables the old systemd-based app setup if it exists.

Then switch to that user and run once from a checked out copy of this repo:

```bash
./deploy/scripts/bootstrap-server-user.sh
```

That creates:

- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`
- `~/apps/obsidian-tg-notify/compose.yaml`

Edit those before first deploy.

### 2. Server config

`shared/config.yaml` should point at your real vault path and Compose Postgres host.

`shared/.env` should contain values for:

- `APP_TIMEZONE`
- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_DB`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_SSLMODE`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_ALLOWED_CHAT_ID`
- `PERSONAL_VAULT_HOST_PATH`
- `PERSONAL_VAULT_PATH`

Make sure the `obsidian` user can read the vault path.
If you use the bundled Postgres container, set `POSTGRES_HOST=postgres`.

### 3. GitHub Actions secrets

Add these repository secrets:

- `SSH_HOST`
- `SSH_PORT`
- `SSH_USER`
- `SSH_PRIVATE_KEY`
- `SSH_KNOWN_HOSTS`
- `GHCR_USERNAME`
- `GHCR_TOKEN`

`SSH_USER` should be the non-root deploy user, eg `obsidian`.
`GHCR_TOKEN` should be a token that can read this package on GHCR.

You can build `SSH_KNOWN_HOSTS` with:

```bash
ssh-keyscan -p 22 your-server-host
```

### 4. Pipelines

- `.github/workflows/ci.yml` runs tests and builds both Go binaries on push/PR.
- `.github/workflows/deploy.yml` tests, builds and pushes the app image to GHCR, uploads `deploy/compose.yaml`, pulls the new image on the server, runs `seed-default-rules`, and starts the app via Compose.

Deploy runs on pushes to `main` and on manual dispatch.

### 5. Server layout

The deploy job manages:

- `~/apps/obsidian-tg-notify/compose.yaml`
- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`

### 6. Runtime notes

- App migrations still run on startup.
- Deploy also runs `seed-default-rules` before app update.
- `.env` is now optional; process env alone also works.
- App and Postgres both run in Docker Compose.
- Postgres is bound to `127.0.0.1:${POSTGRES_PORT}` on the host.
- App reaches Postgres over Compose DNS with `POSTGRES_HOST=postgres`.

### 7. First deploy checklist

1. Bootstrap server root side.
2. Bootstrap server user side.
3. Fill `shared/config.yaml`.
4. Fill `shared/.env`.
5. Log out and back in so docker group applies.
6. Ensure vault path permissions.
7. Add GitHub secrets.
8. Push to `main`.
