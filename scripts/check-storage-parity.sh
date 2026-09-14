#!/usr/bin/env sh
set -eu

if ! command -v go >/dev/null 2>&1; then
  printf '%s\n' 'Go is niet geinstalleerd; storage parity tests kunnen niet worden uitgevoerd.' >&2
  exit 127
fi

go test ./internal/store ./internal/config ./internal/objectstore ./internal/httpserver ./internal/systembackup

if [ -n "${TEST_DATABASE_URL:-}" ]; then
  go test ./internal/store -run 'Postgres|Parity|Domain|Case|Liability'
else
  printf '%s\n' 'TEST_DATABASE_URL is niet gezet; PostgreSQL-integratietests zijn overgeslagen.'
fi

if [ -n "${TEST_MYSQL_DSN:-}" ]; then
  go test ./internal/store -run 'MySQL|Parity|Domain|Case|Liability'
else
  printf '%s\n' 'TEST_MYSQL_DSN is niet gezet; MySQL-integratietests zijn overgeslagen.'
fi
