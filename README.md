# obsidian-tg-notify

## CI/CD and non-root deploy

This repo now supports a Dockerised GitHub Actions deploy flow. CI builds and tests the app, CD pushes an image to GHCR, then the server pulls that image and runs it with Docker Compose as a non-root user.

### 1. One-time server bootstrap

Run once as root on the server:

```bash
./deploy/scripts/bootstrap-server-root.sh obsidian
```

This creates the `obsidian` user if missing, adds it to the `docker` group if present, and prepares app dirs.

Then switch to that user and run once from a checked out copy of this repo:

```bash
./deploy/scripts/bootstrap-server-user.sh
```

That installs the Compose file and creates:

- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`
- `~/apps/obsidian-tg-notify/compose.yaml`

Edit those before first deploy.

### 2. Server config

`shared/config.yaml` should point at your real vault path and Postgres host.

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
`GHCR_TOKEN` should be a token that can read packages from `ghcr.io`.

You can build `SSH_KNOWN_HOSTS` with:

```bash
ssh-keyscan -p 22 your-server-host
```

### 4. Pipelines

- `.github/workflows/ci.yml` runs tests and a Docker build on push/PR.
- `.github/workflows/deploy.yml` tests, builds a Docker image, pushes it to GHCR, uploads `compose.yaml`, pulls the image on the server, runs `seed-default-rules`, and starts the app with Docker Compose.

Deploy runs on pushes to `main` and on manual dispatch.

### 5. Server layout

The deploy job manages:

- `~/apps/obsidian-tg-notify/compose.yaml`
- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`

### 6. Runtime notes

- App migrations still run on startup.
- Deploy also runs `seed-default-rules` inside the image before app restart.
- `.env` is now optional; process env alone also works.
- Docker image runs as a non-root user.
- Server user needs Docker and Docker Compose access.

### 7. First deploy checklist

1. Bootstrap server root side.
2. Bootstrap server user side.
3. Fill `shared/config.yaml`.
4. Fill `shared/.env`.
5. Ensure vault path permissions.
6. Add GitHub secrets.
7. Re-login if `docker` group membership was just added.
8. Push to `main`.
