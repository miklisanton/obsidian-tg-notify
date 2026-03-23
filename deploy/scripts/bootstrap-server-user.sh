#!/usr/bin/env bash
set -euo pipefail

app_root="${HOME}/apps/obsidian-tg-notify"
shared_dir="${app_root}/shared"

mkdir -p "${shared_dir}" "${app_root}/releases"

chmod 700 "${shared_dir}"

if [ ! -f "${shared_dir}/config.yaml" ]; then
  cp config.example.yaml "${shared_dir}/config.yaml"
  printf 'created %s from template\n' "${shared_dir}/config.yaml"
fi

if [ ! -f "${shared_dir}/.env" ]; then
  touch "${shared_dir}/.env"
  chmod 600 "${shared_dir}/.env"
  printf 'created empty %s\n' "${shared_dir}/.env"
fi

printf 'user bootstrap done\n'
