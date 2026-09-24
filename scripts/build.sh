#!/usr/bin/env sh
set -eu

mkdir -p bin dist
go build -o bin/hohoho-api ./cmd/api

if [ -d web/node_modules ]; then
  (cd web && npm run build)
  rm -rf dist/web
  cp -R web/dist dist/web
fi
