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

read_env_value() {
	local key="$1"
	local file="$2"
	local line
	local value

	line="$(grep -E "^${key}=" "$file" || true)"
	if [ -z "$line" ]; then
		return 1
	fi

	value="${line#*=}"
	value="${value%$'\r'}"
	if [[ "$value" == \"*\" && "$value" == *\" ]]; then
		value="${value:1:-1}"
	elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
		value="${value:1:-1}"
	fi

	printf '%s' "$value"
}

if [ ! -f "${shared_dir}/.env" ]; then
  printf 'missing %s\n' "${shared_dir}/.env" >&2
  exit 1
fi

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

postgres_host="$(read_env_value POSTGRES_HOST "${shared_dir}/.env" || true)"
postgres_port="$(read_env_value POSTGRES_PORT "${shared_dir}/.env" || true)"
if [ -z "${postgres_host}" ]; then
	postgres_host="127.0.0.1"
fi
if [ -z "${postgres_port}" ]; then
	postgres_port="5432"
fi
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
