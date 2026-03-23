#!/usr/bin/env bash
set -euo pipefail

app_root="${HOME}/apps/obsidian-tg-notify"
shared_dir="${app_root}/shared"
compose_file="${app_root}/compose.yaml"
app_image="${APP_IMAGE:-}"
ghcr_username="${GHCR_USERNAME:-}"
ghcr_token="${GHCR_TOKEN:-}"

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

if [ -n "${ghcr_username}" ] && [ -n "${ghcr_token}" ]; then
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin
fi

docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" pull app seed
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" up -d postgres
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" run --rm --no-deps seed
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" up -d app
APP_IMAGE="${app_image}" docker compose --env-file "${shared_dir}/.env" -f "${compose_file}" ps
