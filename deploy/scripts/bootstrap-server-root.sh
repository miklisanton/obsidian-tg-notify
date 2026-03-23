#!/usr/bin/env bash
set -euo pipefail

app_user="${1:-obsidian}"

if ! id "${app_user}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${app_user}"
fi

app_home="$(getent passwd "${app_user}" | cut -d: -f6)"

install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps"
install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps/obsidian-tg-notify"
install -d -o "${app_user}" -g "${app_user}" -m 0700 "${app_home}/apps/obsidian-tg-notify/shared"

cp deploy/compose.yaml "${app_home}/apps/obsidian-tg-notify/compose.yaml"
chown "${app_user}:${app_user}" "${app_home}/apps/obsidian-tg-notify/compose.yaml"
chmod 0644 "${app_home}/apps/obsidian-tg-notify/compose.yaml"

usermod -aG docker "${app_user}"
systemctl disable --now obsidian-tg-notify.service obsidian-tg-notify-postgres.service >/dev/null 2>&1 || true
rm -f /etc/sudoers.d/obsidian-tg-notify

printf 'root bootstrap done for %s\n' "${app_user}"
