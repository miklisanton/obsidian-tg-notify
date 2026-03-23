# obsidian-tg-notify

## CI/CD and non-root deploy

This repo now supports a simple GitHub Actions pipeline and a remote server deploy flow where the app runs as a regular user via `systemd --user`.

### 1. One-time server bootstrap

Run once as root on the server:

```bash
./deploy/scripts/bootstrap-server-root.sh obsidian
```

This creates the `obsidian` user if missing, enables user services after reboot, and prepares app dirs.

Then switch to that user and run once from a checked out copy of this repo:

```bash
./deploy/scripts/bootstrap-server-user.sh
```

That installs the user service and creates:

- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`

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
- `PERSONAL_VAULT_PATH`

Make sure the `obsidian` user can read the vault path.

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

- `.github/workflows/ci.yml` runs tests and a Docker build on push/PR.
- `.github/workflows/deploy.yml` builds a Linux release tarball, uploads it over SSH, activates it, and restarts the user service.
- Deploy also runs the bundled `seed-default-rules` binary before restart.

Deploy runs on pushes to `main` and on manual dispatch.

### 5. Release layout on server

The deploy job manages:

- `~/apps/obsidian-tg-notify/releases/<git-sha>`
- `~/apps/obsidian-tg-notify/current`
- `~/apps/obsidian-tg-notify/shared/config.yaml`
- `~/apps/obsidian-tg-notify/shared/.env`

### 6. Runtime notes

- App migrations still run on startup.
- `.env` is now optional; process env alone also works.
- Docker image now runs as a non-root user too.

### 7. First deploy checklist

1. Bootstrap server root side.
2. Bootstrap server user side.
3. Fill `shared/config.yaml`.
4. Fill `shared/.env`.
5. Ensure vault path permissions.
6. Add GitHub secrets.
7. Push to `main`.
