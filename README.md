# obsidian-tg-notify

## CI/CD and non-root deploy

This repo now supports a Go binary GitHub Actions deploy flow. CI tests and builds the binaries, CD uploads a Linux release tarball over SSH, then the server activates it behind a system-wide `systemd` service that runs as a non-root user. Postgres runs in a local Docker container managed by its own systemd unit.

### 1. One-time server bootstrap

Run once as root on the server:

```bash
./deploy/scripts/bootstrap-server-root.sh obsidian
```

This creates the `obsidian` user if missing, prepares app dirs, installs the app and Postgres systemd units, and grants limited `sudo systemctl` access for deploy restarts.

Then switch to that user and run once from a checked out copy of this repo:

```bash
./deploy/scripts/bootstrap-server-user.sh
```

That creates:

- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`
- `~/apps/obsidian-tg-notify/releases/`
- `~/apps/obsidian-tg-notify/postgres-compose.yaml`

Edit those before first deploy.

### 2. Server config

`shared/config.yaml` should point at your real vault path and local Postgres host.

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
- `PERSONAL_VAULT_PATH`

Make sure the `obsidian` user can read the vault path.
If you use the bundled Postgres container, set `POSTGRES_HOST=127.0.0.1`.

### 3. GitHub Actions secrets

Add these repository secrets:

- `SSH_HOST`
- `SSH_PORT`
- `SSH_USER`
- `SSH_PRIVATE_KEY`
- `SSH_KNOWN_HOSTS`

`SSH_USER` should be the non-root deploy user, eg `obsidian`.

You can build `SSH_KNOWN_HOSTS` with:

```bash
ssh-keyscan -p 22 your-server-host
```

### 4. Pipelines

- `.github/workflows/ci.yml` runs tests and builds both Go binaries on push/PR.
- `.github/workflows/deploy.yml` tests, builds a Linux release tarball, uploads it over SSH, makes sure the Postgres container is up, runs `seed-default-rules`, and restarts the systemd service.

Deploy runs on pushes to `main` and on manual dispatch.

### 5. Server layout

The deploy job manages:

- `~/apps/obsidian-tg-notify/releases/<git-sha>`
- `~/apps/obsidian-tg-notify/current`
- `~/apps/obsidian-tg-notify/postgres-compose.yaml`
- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`

### 6. Runtime notes

- App migrations still run on startup.
- Deploy also runs `seed-default-rules` before service restart.
- `.env` is now optional; process env alone also works.
- `systemd` is system-wide, but app process runs as the non-root deploy user.
- Postgres is provided by `obsidian-tg-notify-postgres.service`, which runs a Docker `postgres:17` container bound to `127.0.0.1:${POSTGRES_PORT}`.

### 7. First deploy checklist

1. Bootstrap server root side.
2. Bootstrap server user side.
3. Fill `shared/config.yaml`.
4. Fill `shared/.env`.
5. Ensure vault path permissions.
6. Add GitHub secrets.
7. Push to `main`.

If deploy fails on `sudo systemctl`, rerun root bootstrap after pulling latest repo so `/etc/sudoers.d/obsidian-tg-notify` and both systemd units are refreshed:

```bash
sudo ./deploy/scripts/bootstrap-server-root.sh <deploy-user>
```
