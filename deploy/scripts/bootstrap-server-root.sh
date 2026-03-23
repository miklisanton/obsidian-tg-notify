#!/usr/bin/env bash
set -euo pipefail

app_user="${1:-obsidian}"
app_home="/home/${app_user}"

if ! id "${app_user}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${app_user}"
fi

if getent group docker >/dev/null 2>&1; then
  usermod -aG docker "${app_user}"
fi

install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps"
install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps/obsidian-tg-notify"
install -d -o "${app_user}" -g "${app_user}" -m 0700 "${app_home}/apps/obsidian-tg-notify/shared"

printf 'root bootstrap done for %s\n' "${app_user}"
