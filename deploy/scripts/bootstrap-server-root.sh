#!/usr/bin/env bash
set -euo pipefail

app_user="${1:-obsidian}"

if ! id "${app_user}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${app_user}"
fi

app_home="$(getent passwd "${app_user}" | cut -d: -f6)"

install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps"
install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps/obsidian-tg-notify"
install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps/obsidian-tg-notify/releases"
install -d -o "${app_user}" -g "${app_user}" -m 0700 "${app_home}/apps/obsidian-tg-notify/shared"

sed "s|__APP_HOME__|${app_home}|g" deploy/postgres-compose.yaml > "${app_home}/apps/obsidian-tg-notify/postgres-compose.yaml"
chown "${app_user}:${app_user}" "${app_home}/apps/obsidian-tg-notify/postgres-compose.yaml"
chmod 0644 "${app_home}/apps/obsidian-tg-notify/postgres-compose.yaml"

sed "s|__APP_USER__|${app_user}|g; s|__APP_HOME__|${app_home}|g" deploy/systemd/obsidian-tg-notify.service > /etc/systemd/system/obsidian-tg-notify.service
sed "s|__APP_HOME__|${app_home}|g" deploy/systemd/obsidian-tg-notify-postgres.service > /etc/systemd/system/obsidian-tg-notify-postgres.service
install -m 0440 deploy/sudoers/obsidian-tg-notify /etc/sudoers.d/obsidian-tg-notify
sed -i.bak "s|__APP_USER__|${app_user}|g" /etc/sudoers.d/obsidian-tg-notify
rm -f /etc/sudoers.d/obsidian-tg-notify.bak
visudo -cf /etc/sudoers.d/obsidian-tg-notify
systemctl daemon-reload
systemctl enable obsidian-tg-notify-postgres.service
systemctl enable obsidian-tg-notify.service

printf 'root bootstrap done for %s\n' "${app_user}"
