#!/usr/bin/env sh
set -eu

printf '%s\n' 'Let op: scripts/package-runtime.sh maakt een legacy SQLite runtimepakket. Gebruik Docker Compose voor de actuele centrale deployment.' >&2

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname "$script_dir")

: "${GOOS:=linux}"
: "${GOARCH:=amd64}"
: "${DATABASE_PATH:=${project_dir}/data/geldzaken.db}"
: "${OUTPUT:=${project_dir}/dist/geldzaken-backend-${GOOS}-${GOARCH}.zip}"

if [ ! -f "$DATABASE_PATH" ]; then
  printf 'Database niet gevonden: %s\n' "$DATABASE_PATH" >&2
  exit 1
fi

command -v zip >/dev/null 2>&1 || {
  printf '%s\n' 'zip is niet geinstalleerd.' >&2
  exit 1
}

stage_root="/tmp/opencode/geldzaken-runtime-$$"
stage_dir="${stage_root}/backend"
trap 'rm -rf "$stage_root"' EXIT INT TERM

mkdir -p "$stage_dir/data" "$stage_dir/web" "$(dirname "$OUTPUT")"

(cd "$project_dir" && CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$stage_dir/geldzaken-api" ./cmd/api)
(cd "$project_dir/web" && npm run build)
cp -R "$project_dir/web/dist" "$stage_dir/web/dist"
(cd "$project_dir" && go run ./cmd/dbbackup "$DATABASE_PATH" "$stage_dir/data/geldzaken.db")

cat >"$stage_dir/run.sh" <<'EOF'
#!/usr/bin/env sh
set -eu

runtime_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -f "$runtime_dir/runtime.env" ]; then
  set -a
  . "$runtime_dir/runtime.env"
  set +a
fi

export APP_ENV="${APP_ENV:-production}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export STORE_BACKEND=sqlite
export DATABASE_PATH="${DATABASE_PATH:-$runtime_dir/data/geldzaken.db}"
export WEB_DIR="${WEB_DIR:-$runtime_dir/web/dist}"
export CORS_ORIGIN="${CORS_ORIGIN:-http://localhost:8080}"

exec "$runtime_dir/geldzaken-api"
EOF

cat >"$stage_dir/runtime.env" <<'EOF'
HTTP_ADDR=:8080
# AUTH_USERNAME=geldzaken
# AUTH_PASSWORD=vervang-dit-wachtwoord
EOF

cat >"$stage_dir/geldzaken-backend.service" <<'EOF'
[Unit]
Description=Geldzaken
After=network-online.target
Wants=network-online.target

[Service]
User=geldzaken
Group=geldzaken
WorkingDirectory=/opt/geldzaken/backend
EnvironmentFile=-/opt/geldzaken/backend/runtime.env
ExecStart=/opt/geldzaken/backend/run.sh
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/opt/geldzaken/backend/data

[Install]
WantedBy=multi-user.target
EOF

chmod 0755 "$stage_dir/geldzaken-api" "$stage_dir/run.sh"
rm -f "$OUTPUT"
(cd "$stage_dir" && zip -qr "$OUTPUT" .)

printf 'Runtimepakket gemaakt: %s\n' "$OUTPUT"
