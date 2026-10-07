#!/usr/bin/env sh
set -eu

export APP_ENV="${APP_ENV:-development}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export DATABASE_PATH="${DATABASE_PATH:-data/v3/hohoho.db}"

go run ./cmd/api
