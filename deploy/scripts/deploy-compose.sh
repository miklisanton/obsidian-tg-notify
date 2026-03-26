#!/usr/bin/env bash
set -euo pipefail

app_root="${HOME}/apps/obsidian-tg-notify"
shared_dir="${app_root}/shared"
compose_file="${app_root}/compose.yaml"
app_image="${APP_IMAGE:-}"
ghcr_username="${GHCR_USERNAME:-}"
ghcr_token="${GHCR_TOKEN:-}"
uid="$(id -u)"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/${uid}}"
export DOCKER_HOST="${DOCKER_HOST:-unix://${XDG_RUNTIME_DIR}/docker.sock}"

if [ -z "${app_image}" ]; then
  printf 'APP_IMAGE missing\n' >&2
  exit 1
fi

if [ ! -f "${shared_dir}/.env" ]; then
  printf 'missing %s\n' "${shared_dir}/.env" >&2
  exit 1
fi

if [ ! -f "${shared_dir}/config.yaml" ]; then
  printf 'missing %s\n' "${shared_dir}/config.yaml" >&2
  exit 1
fi

if [ ! -f "${compose_file}" ]; then
  printf 'missing %s\n' "${compose_file}" >&2
  exit 1
fi

if [ ! -S "${XDG_RUNTIME_DIR}/docker.sock" ]; then
  printf 'rootless docker socket missing: %s\n' "${XDG_RUNTIME_DIR}/docker.sock" >&2
  exit 1
fi

if [ -n "${ghcr_username}" ] && [ -n "${ghcr_token}" ]; then
  printf '== ghcr login ==\n'
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin
fi

printf '== pull ==\n'
docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" pull app seed
printf '== seed ==\n'
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" run --rm --no-deps -T seed </dev/null
printf '== up app ==\n'
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" up -d app
printf '== ps ==\n'
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" ps
