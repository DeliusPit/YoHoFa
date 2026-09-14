#!/usr/bin/env sh
set -eu

: "${BASE_URL:=http://127.0.0.1:8080}"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname "$script_dir")

if [ -z "${AUTH_USERNAME:-}" ] || [ -z "${AUTH_PASSWORD:-}" ]; then
  if [ -f "$project_dir/config/.env" ]; then
    set -a
    . "$project_dir/config/.env"
    set +a
  fi
fi

request() {
  path="$1"
  expected_content="$2"
  response_file=$(mktemp)
  if [ -n "${AUTH_USERNAME:-}" ] && [ -n "${AUTH_PASSWORD:-}" ]; then
    status=$(curl -sS -u "$AUTH_USERNAME:$AUTH_PASSWORD" -o "$response_file" -w '%{http_code}' "$BASE_URL$path")
  else
    status=$(curl -sS -o "$response_file" -w '%{http_code}' "$BASE_URL$path")
  fi
  if [ "$status" != "200" ]; then
    printf 'Runtimecheck faalde: %s gaf HTTP %s\n' "$path" "$status" >&2
    if [ "$status" = "401" ]; then
      printf 'Controleer AUTH_USERNAME en AUTH_PASSWORD in %s/config/.env of exporteer ze voor het starten.\n' "$project_dir" >&2
    fi
    rm -f "$response_file"
    exit 1
  fi
  if [ -n "$expected_content" ] && ! grep -q "$expected_content" "$response_file"; then
    printf 'Runtimecheck faalde: %s bevatte niet %s\n' "$path" "$expected_content" >&2
    rm -f "$response_file"
    exit 1
  fi
  rm -f "$response_file"
  printf 'ok %s\n' "$path"
}

request /healthz '"status":"ok"'
request /api/v1/summary 'households'
request /api/v1/accounts ''
request /api/v1/categories ''
request /api/v1/system/configuration 'capabilities'

printf '%s\n' 'Runtimecheck voltooid.'
