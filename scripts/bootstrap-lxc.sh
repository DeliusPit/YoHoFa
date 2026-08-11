#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' 'Run this script as root inside the LXC container.' >&2
  exit 1
fi

: "${HTTP_ADDR:=:8080}"
: "${DATABASE_PATH:=/var/lib/geldzaken/geldzaken.db}"
: "${CORS_ORIGIN:=http://localhost:5173}"

apt-get update
apt-get install -y ca-certificates curl sqlite3

if ! getent group geldzaken >/dev/null; then
  groupadd --system geldzaken
fi

if ! id geldzaken >/dev/null 2>&1; then
  useradd --system --gid geldzaken --home /opt/geldzaken --shell /usr/sbin/nologin geldzaken
fi

install -d -o geldzaken -g geldzaken -m 0755 /opt/geldzaken
install -d -o geldzaken -g geldzaken -m 0750 /var/lib/geldzaken
install -d -o root -g root -m 0755 /etc/geldzaken

cat >/etc/geldzaken/api.env <<EOF
APP_ENV=production
HTTP_ADDR=${HTTP_ADDR}
STORE_BACKEND=sqlite
DATABASE_PATH=${DATABASE_PATH}
CORS_ORIGIN=${CORS_ORIGIN}
EOF

if [ -f /tmp/geldzaken-api.service ]; then
  install -o root -g root -m 0644 /tmp/geldzaken-api.service /etc/systemd/system/geldzaken-api.service
elif [ -f ./deploy/systemd/geldzaken-api.service ]; then
  install -o root -g root -m 0644 ./deploy/systemd/geldzaken-api.service /etc/systemd/system/geldzaken-api.service
else
  cat >/etc/systemd/system/geldzaken-api.service <<'EOF'
[Unit]
Description=Geldzaken API
After=network-online.target
Wants=network-online.target

[Service]
User=geldzaken
Group=geldzaken
EnvironmentFile=/etc/geldzaken/api.env
ExecStart=/opt/geldzaken/geldzaken-api
WorkingDirectory=/opt/geldzaken
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/opt/geldzaken /var/lib/geldzaken

[Install]
WantedBy=multi-user.target
EOF
fi

systemctl daemon-reload
systemctl enable geldzaken-api

printf '%s\n' 'LXC bootstrap complete.'
printf '%s\n' 'Deploy the binary with: LXC_HOST=<ip> LXC_USER=root ./scripts/deploy-lxc.sh'
printf '%s\n' 'Then check: systemctl status geldzaken-api'
