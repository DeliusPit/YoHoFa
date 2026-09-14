#!/usr/bin/env sh
set -eu

: "${S3_ENDPOINT:?Set S3_ENDPOINT, for example: garage.example.nl:3900}"
: "${S3_ACCESS_KEY:?Set S3_ACCESS_KEY}"
: "${S3_SECRET_KEY:?Set S3_SECRET_KEY}"
: "${S3_BUCKET:?Set S3_BUCKET}"

export S3_REGION="${S3_REGION:-garage}"
export S3_PATH_STYLE="${S3_PATH_STYLE:-true}"
export S3_CREATE_BUCKET="${S3_CREATE_BUCKET:-false}"
export S3_USE_TLS="${S3_USE_TLS:-true}"

if ! command -v go >/dev/null 2>&1; then
  printf '%s\n' 'Go is niet geinstalleerd; Garage/S3-check kan niet worden uitgevoerd.' >&2
  exit 127
fi

go test ./internal/objectstore -run TestLiveS3 -count=1
