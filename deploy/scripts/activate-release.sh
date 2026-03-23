#!/usr/bin/env bash
set -euo pipefail

release_sha="${RELEASE_SHA:-}"
if [ -z "${release_sha}" ]; then
  printf 'RELEASE_SHA missing\n' >&2
  exit 1
fi

app_root="${HOME}/apps/obsidian-tg-notify"
releases_dir="${app_root}/releases"
release_dir="${releases_dir}/${release_sha}"
archive_path="${releases_dir}/${release_sha}.tar.gz"
shared_dir="${app_root}/shared"

if [ ! -f "${shared_dir}/.env" ]; then
  printf 'missing %s\n' "${shared_dir}/.env" >&2
  exit 1
fi

set -a
. "${shared_dir}/.env"
set +a

if [ ! -f "${archive_path}" ]; then
  printf 'missing %s\n' "${archive_path}" >&2
  exit 1
fi

if [ ! -f "${shared_dir}/config.yaml" ]; then
  printf 'missing %s\n' "${shared_dir}/config.yaml" >&2
  exit 1
fi

rm -rf "${release_dir}"
mkdir -p "${release_dir}"
tar -xzf "${archive_path}" -C "${release_dir}" --strip-components=1
ln -sfn "${release_dir}" "${app_root}/current"

sudo systemctl restart obsidian-tg-notify-postgres.service

postgres_host="${POSTGRES_HOST:-127.0.0.1}"
postgres_port="${POSTGRES_PORT:-5432}"
postgres_ready=false
for _ in $(seq 1 60); do
	if bash -c "</dev/tcp/${postgres_host}/${postgres_port}" >/dev/null 2>&1; then
		postgres_ready=true
		break
	fi
	sleep 1
done

if [ "${postgres_ready}" != "true" ]; then
	printf 'postgres not ready on %s:%s\n' "${postgres_host}" "${postgres_port}" >&2
	exit 1
fi

APP_CONFIG="${shared_dir}/config.yaml" APP_ENV_FILE="${shared_dir}/.env" "${app_root}/current/seed-default-rules"
sudo systemctl restart obsidian-tg-notify.service
sudo systemctl is-active obsidian-tg-notify.service
