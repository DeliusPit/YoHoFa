#!/usr/bin/env sh
set -eu

export APP_ENV="${APP_ENV:-development}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export STORE_BACKEND="${STORE_BACKEND:-postgres}"
export DATABASE_PATH="${DATABASE_PATH:-data/geldzaken.db}"
export CORS_ORIGIN="${CORS_ORIGIN:-http://localhost:5173}"

if [ "$STORE_BACKEND" = "sqlite" ]; then
  mkdir -p "$(dirname "$DATABASE_PATH")"
fi

go run ./cmd/api
