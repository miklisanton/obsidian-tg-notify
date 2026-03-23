#!/usr/bin/env bash
set -euo pipefail

release_sha="${1:?release sha required}"
app_root="${HOME}/apps/obsidian-tg-notify"
releases_dir="${app_root}/releases"
release_dir="${releases_dir}/${release_sha}"
archive_path="${releases_dir}/${release_sha}.tar.gz"
shared_dir="${app_root}/shared"
systemd_unit="obsidian-tg-notify.service"

mkdir -p "${shared_dir}"

if [ ! -f "${shared_dir}/config.yaml" ]; then
  printf 'missing %s\n' "${shared_dir}/config.yaml" >&2
  exit 1
fi

if [ ! -f "${archive_path}" ]; then
  printf 'missing %s\n' "${archive_path}" >&2
  exit 1
fi

rm -rf "${release_dir}"
mkdir -p "${release_dir}"
tar -xzf "${archive_path}" -C "${release_dir}" --strip-components=1
ln -sfn "${release_dir}" "${app_root}/current"

APP_CONFIG="${shared_dir}/config.yaml" APP_ENV_FILE="${shared_dir}/.env" "${app_root}/current/seed-default-rules"

systemctl --user daemon-reload
systemctl --user enable --now "${systemd_unit}"
systemctl --user restart "${systemd_unit}"
systemctl --user is-active "${systemd_unit}"
