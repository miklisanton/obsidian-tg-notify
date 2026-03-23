#!/usr/bin/env bash
set -euo pipefail

app_root="${HOME}/apps/obsidian-tg-notify"
shared_dir="${app_root}/shared"
unit_dir="${HOME}/.config/systemd/user"

mkdir -p "${shared_dir}" "${unit_dir}"

if [ ! -f "${shared_dir}/config.yaml" ]; then
  cp config.example.yaml "${shared_dir}/config.yaml"
  printf 'created %s from template\n' "${shared_dir}/config.yaml"
fi

if [ ! -f "${shared_dir}/.env" ]; then
  touch "${shared_dir}/.env"
  chmod 600 "${shared_dir}/.env"
  printf 'created empty %s\n' "${shared_dir}/.env"
fi

install -m 0644 deploy/systemd/obsidian-tg-notify.service "${unit_dir}/obsidian-tg-notify.service"
systemctl --user daemon-reload
printf 'user bootstrap done\n'
