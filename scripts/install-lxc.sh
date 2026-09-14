#!/usr/bin/env sh
set -eu

printf '%s\n' 'Let op: scripts/install-lxc.sh is de legacy SQLite/LXC-deployflow. Gebruik voor de actuele centrale deployment: docker compose -f compose.yml up --build -d' >&2

: "${LXC_HOST:?Set LXC_HOST, for example: LXC_HOST=192.168.1.50}"
: "${LXC_USER:=root}"
: "${AUTH_USERNAME:=geldzaken}"
: "${AUTH_PASSWORD:?Set AUTH_PASSWORD}"

npm --prefix web ci
npm --prefix web run build

auth_username_b64=$(printf '%s' "$AUTH_USERNAME" | base64 | tr -d '\n')
auth_password_b64=$(printf '%s' "$AUTH_PASSWORD" | base64 | tr -d '\n')

tar \
  --exclude='./.git' \
  --exclude='./.env' \
  --exclude='./bin' \
  --exclude='./dist' \
  --exclude='./data' \
  --exclude='./web/node_modules' \
  -czf - . | ssh "${LXC_USER}@${LXC_HOST}" "
    set -eu
    apt-get update
    apt-get install -y ca-certificates sqlite3
    command -v go >/dev/null || { echo 'Go is niet geinstalleerd in de LXC' >&2; exit 1; }

    if ! getent group geldzaken >/dev/null; then
      groupadd --system geldzaken
    fi
    if ! id geldzaken >/dev/null 2>&1; then
      useradd --system --gid geldzaken --home /opt/geldzaken --shell /usr/sbin/nologin geldzaken
    fi

    rm -rf /opt/geldzaken.new
    mkdir -p /opt/geldzaken.new /var/lib/geldzaken /etc/geldzaken
    tar -xzf - -C /opt/geldzaken.new

    cd /opt/geldzaken.new
    go build -o geldzaken-api ./cmd/api

    if [ -d /opt/geldzaken ]; then
      rm -rf /opt/geldzaken.old
      mv /opt/geldzaken /opt/geldzaken.old
    fi
    mv /opt/geldzaken.new /opt/geldzaken
    chown -R root:root /opt/geldzaken
    chown -R geldzaken:geldzaken /var/lib/geldzaken

    cat >/etc/geldzaken/api.env <<EOF
APP_ENV=production
HTTP_ADDR=:8080
STORE_BACKEND=sqlite
DATABASE_PATH=/var/lib/geldzaken/geldzaken.db
CORS_ORIGIN=http://${LXC_HOST}:8080
WEB_DIR=/opt/geldzaken/web/dist
AUTH_USERNAME_BASE64=${auth_username_b64}
AUTH_PASSWORD_BASE64=${auth_password_b64}
EOF

    chmod 0600 /etc/geldzaken/api.env
    cp /opt/geldzaken/deploy/systemd/geldzaken-api.service /etc/systemd/system/geldzaken-api.service
    systemctl daemon-reload
    systemctl enable geldzaken-api
    systemctl restart geldzaken-api
    systemctl --no-pager --full status geldzaken-api
  "
