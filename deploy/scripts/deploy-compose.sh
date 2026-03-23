#!/usr/bin/env bash
set -euo pipefail

app_root="${HOME}/apps/obsidian-tg-notify"
compose_file="${app_root}/compose.yaml"
shared_dir="${app_root}/shared"

if ! command -v docker >/dev/null 2>&1; then
  printf 'docker missing\n' >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  printf 'docker compose missing\n' >&2
  exit 1
fi

if [ -z "${APP_IMAGE:-}" ]; then
  printf 'APP_IMAGE missing\n' >&2
  exit 1
fi

if [ ! -f "${compose_file}" ]; then
  printf 'missing %s\n' "${compose_file}" >&2
  exit 1
fi

if [ ! -f "${shared_dir}/config.yaml" ]; then
  printf 'missing %s\n' "${shared_dir}/config.yaml" >&2
  exit 1
fi

if [ ! -f "${shared_dir}/.env" ]; then
  printf 'missing %s\n' "${shared_dir}/.env" >&2
  exit 1
fi

mkdir -p "${HOME}/.docker"

if [ -n "${GHCR_USERNAME:-}" ] && [ -n "${GHCR_TOKEN:-}" ]; then
  printf '%s' "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin
fi

APP_IMAGE="${APP_IMAGE}" docker compose -f "${compose_file}" --env-file "${shared_dir}/.env" pull app seed-default-rules
APP_IMAGE="${APP_IMAGE}" docker compose -f "${compose_file}" --env-file "${shared_dir}/.env" run --rm seed-default-rules
APP_IMAGE="${APP_IMAGE}" docker compose -f "${compose_file}" --env-file "${shared_dir}/.env" up -d app
