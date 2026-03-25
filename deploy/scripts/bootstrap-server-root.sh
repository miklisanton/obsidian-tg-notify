#!/usr/bin/env bash
set -euo pipefail

app_user="${1:-obsidian}"

if ! id "${app_user}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${app_user}"
fi

if ! grep -q "^${app_user}:" /etc/subuid; then
  next_subuid_start="$(( $(awk -F: 'BEGIN { max = 99999 } { end = $2 + $3 - 1; if (end > max) max = end } END { print max }' /etc/subuid 2>/dev/null) + 1 ))"
  printf '%s:%s:65536\n' "${app_user}" "${next_subuid_start}" >> /etc/subuid
fi

if ! grep -q "^${app_user}:" /etc/subgid; then
  next_subgid_start="$(( $(awk -F: 'BEGIN { max = 99999 } { end = $2 + $3 - 1; if (end > max) max = end } END { print max }' /etc/subgid 2>/dev/null) + 1 ))"
  printf '%s:%s:65536\n' "${app_user}" "${next_subgid_start}" >> /etc/subgid
fi

app_home="$(getent passwd "${app_user}" | cut -d: -f6)"

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y uidmap dbus-user-session docker-ce-rootless-extras

install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps"
install -d -o "${app_user}" -g "${app_user}" -m 0755 "${app_home}/apps/obsidian-tg-notify"
install -d -o "${app_user}" -g "${app_user}" -m 0700 "${app_home}/apps/obsidian-tg-notify/shared"

cp deploy/compose.yaml "${app_home}/apps/obsidian-tg-notify/compose.yaml"
chown "${app_user}:${app_user}" "${app_home}/apps/obsidian-tg-notify/compose.yaml"
chmod 0644 "${app_home}/apps/obsidian-tg-notify/compose.yaml"

su - "${app_user}" -c 'dockerd-rootless-setuptool.sh install --skip-iptables'
loginctl enable-linger "${app_user}"
systemctl disable --now obsidian-tg-notify.service obsidian-tg-notify-postgres.service >/dev/null 2>&1 || true
rm -f /etc/sudoers.d/obsidian-tg-notify

printf 'root bootstrap done for %s\n' "${app_user}"
