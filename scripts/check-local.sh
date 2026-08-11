#!/usr/bin/env sh
set -eu

mkdir -p bin
go build -o bin/geldzaken-api ./cmd/api

export APP_ENV=development
export HTTP_ADDR=127.0.0.1:18080
export STORE_BACKEND=sqlite
export DATABASE_PATH="/tmp/opencode/geldzaken-check-$$.db"
export CORS_ORIGIN=http://localhost:5173

bin/geldzaken-api &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT INT TERM

sleep 1
curl -fsS http://127.0.0.1:18080/healthz >/dev/null
curl -fsS http://127.0.0.1:18080/api/v1/summary >/dev/null
curl -fsS http://127.0.0.1:18080/api/v1/accounts >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"name":"Spaarrekening","iban":"NL00TEST0123456789","type":"savings","owner":"Huishouden","bankProfile":"triodos"}' http://127.0.0.1:18080/api/v1/accounts >/dev/null
curl -fsS -X PATCH -H 'Content-Type: application/json' -d '{"name":"Spaarpot","iban":"NL00TEST0123456789","type":"savings","owner":"Huishouden","bankProfile":"triodos"}' http://127.0.0.1:18080/api/v1/accounts/2 >/dev/null
curl -fsS -X DELETE http://127.0.0.1:18080/api/v1/accounts/2 >/dev/null
curl -fsS http://127.0.0.1:18080/api/v1/main-categories >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"name":"Testcategorie"}' http://127.0.0.1:18080/api/v1/main-categories >/dev/null
curl -fsS -X PATCH -H 'Content-Type: application/json' -d '{"name":"Overig"}' http://127.0.0.1:18080/api/v1/main-categories/11 >/dev/null
curl -fsS -X DELETE http://127.0.0.1:18080/api/v1/main-categories/11 >/dev/null
curl -fsS http://127.0.0.1:18080/api/v1/categories >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"mainCategoryId":2,"name":"Test subcategorie","kind":"expense","purpose":"daily"}' http://127.0.0.1:18080/api/v1/categories >/dev/null
curl -fsS -X PATCH -H 'Content-Type: application/json' -d '{"mainCategoryId":2,"name":"Woonlasten","kind":"expense","purpose":"reservation"}' http://127.0.0.1:18080/api/v1/categories/2 >/dev/null
curl -fsS -X DELETE http://127.0.0.1:18080/api/v1/categories/1 >/dev/null
curl -fsS -F profile=generic -F file=@samples/bankimport/generic.csv http://127.0.0.1:18080/api/v1/imports/preview >/dev/null
curl -fsS -F profile=triodos -F file=@samples/bankimport/triodos-example.csv http://127.0.0.1:18080/api/v1/imports/preview >/dev/null
curl -fsS -F accountId=1 -F file=@samples/bankimport/triodos-example.csv http://127.0.0.1:18080/api/v1/imports >/dev/null
curl -fsS -F accountId=1 -F file=@samples/bankimport/triodos-example.csv http://127.0.0.1:18080/api/v1/imports >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?accountId=1&from=2026-01-01&to=2026-12-31&limit=10&offset=0' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions/years' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions/summary?accountId=1&from=2026-01-01&to=2026-12-31' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?accountId=1&search=triodos&limit=10&offset=0' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?accountId=1&from=2026-01-01&to=2026-12-31&limit=10&offset=10' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?accountId=1&uncategorized=true' >/dev/null
curl -fsS -X PATCH -H 'Content-Type: application/json' -d '{"categoryId":2,"applyToSameCounterparty":true}' http://127.0.0.1:18080/api/v1/transactions/1/category >/dev/null
curl -fsS -X PATCH http://127.0.0.1:18080/api/v1/transactions/1/hidden >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?hidden=true&limit=10' >/dev/null
curl -fsS -X DELETE http://127.0.0.1:18080/api/v1/transactions/1/hidden >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?mainCategoryId=2' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/transactions?categoryId=2' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/reports/category-monthly?mainCategoryId=2' >/dev/null
curl -fsS 'http://127.0.0.1:18080/api/v1/reports/main-category-period?from=2026-01-01&to=2026-12-31' >/dev/null
