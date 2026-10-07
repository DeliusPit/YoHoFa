#!/usr/bin/env sh
set -eu

work=$(mktemp -d "${TMPDIR:-/tmp}/hohoho-v3-check.XXXXXX")
pid=""
cleanup() {
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

go build -buildvcs=false -o "$work/hohoho-api" ./cmd/api
export APP_ENV=development
export HTTP_ADDR=127.0.0.1:18080
export DATABASE_PATH="$work/hohoho.db"
unset AUTH_USERNAME AUTH_PASSWORD AUTH_USERNAME_BASE64 AUTH_PASSWORD_BASE64
unset S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY
base=http://127.0.0.1:18080
bank_csv="$work/sparda.csv"
printf '%s\n' 'Buchungstag;Betrag;Verwendungszweck;Auftraggeber/Empfänger' '03.10.2026;-42,50;Boodschappen;Supermarkt' >"$bank_csv"

start() {
  "$work/hohoho-api" &
  pid=$!
  attempt=0
  until curl -fsS "$base/healthz" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 50 ] || ! kill -0 "$pid" 2>/dev/null; then
      exit 1
    fi
    sleep 0.1
  done
}

start
test "$(curl -fsS "$base/api/v1/financial-objects")" = "[]"
curl -fsS -H 'Content-Type: application/json' -d '{"objectType":"asset","name":"Woning Sögel"}' "$base/api/v1/financial-objects" >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"objectType":"debt","name":"Hypotheek Sparda"}' "$base/api/v1/financial-objects" >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"valueDate":"2026-10-01","valueCents":59850000}' "$base/api/v1/financial-objects/1/values" >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"sourceObjectId":2,"targetObjectId":1,"relationType":"finances"}' "$base/api/v1/object-relations" >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"transactionDate":"2026-10-01","amountCents":-100000,"description":"Hypotheekbetaling","allocations":[{"financialObjectId":2,"amountCents":-100000}]}' "$base/api/v1/transactions" >/dev/null
curl -fsS -H 'Content-Type: application/json' -d '{"transactionDate":"2026-10-02","amountCents":250000,"description":"Pensioen"}' "$base/api/v1/transactions" >/dev/null
first_import=$(curl -fsS -F 'bankConfig=sparda' -F "file=@$bank_csv" "$base/api/v1/bank-imports")
case "$first_import" in *'"imported":1'*'"skipped":0'*) ;; *) exit 1 ;; esac
second_import=$(curl -fsS -F 'bankConfig=sparda' -F "file=@$bank_csv" "$base/api/v1/bank-imports")
case "$second_import" in *'"imported":0'*'"skipped":1'*) ;; *) exit 1 ;; esac
curl -fsS "$base/api/v1/financial-objects/1/values" >/dev/null
curl -fsS "$base/api/v1/financial-objects/1/relations" >/dev/null
test "$(curl -s -o /dev/null -w '%{http_code}' "$base/api/v1/accounts")" = "404"
kill "$pid"
wait "$pid"
pid=""
start
summary=$(curl -fsS "$base/api/v1/summary")
case "$summary" in *'"financialObjects":2'*) ;; *) exit 1 ;; esac
case "$summary" in *'"objectValues":1'*) ;; *) exit 1 ;; esac
case "$summary" in *'"objectRelations":1'*) ;; *) exit 1 ;; esac
case "$summary" in *'"transactions":3'*) ;; *) exit 1 ;; esac
transactions=$(curl -fsS "$base/api/v1/transactions")
case "$transactions" in *'"balanceCents":150000'*) ;; *) exit 1 ;; esac
dashboard=$(curl -fsS "$base/api/v1/dashboard?year=2026&month=10")
case "$dashboard" in *'"netWorthCents":59850000'*) ;; *) exit 1 ;; esac
case "$dashboard" in *'"balanceCents":150000'*) ;; *) exit 1 ;; esac
curl -fsS "$base/api/v1/transactions?direction=expense&limit=1" >/dev/null
curl -fsS -X PUT -H 'Content-Type: application/json' -d '{"transactionDate":"2026-10-02","amountCents":260000,"description":"Pensioen aangepast"}' "$base/api/v1/transactions/2" >/dev/null
curl -fsS -X DELETE "$base/api/v1/transactions/2" >/dev/null
curl -fsS -X DELETE -H 'Content-Type: application/json' -d '{"sourceObjectId":2,"targetObjectId":1,"relationType":"finances"}' "$base/api/v1/object-relations" >/dev/null
# Verwijder in FK-volgorde: toewijzing (PUT zonder allocaties), waarde, dan objecten.
curl -fsS -X PUT -H 'Content-Type: application/json' -d '{"transactionDate":"2026-10-01","amountCents":-100000,"description":"Hypotheekbetaling","allocations":[]}' "$base/api/v1/transactions/1" >/dev/null
curl -fsS -X DELETE "$base/api/v1/financial-objects/1/values/1" >/dev/null
curl -fsS -X DELETE "$base/api/v1/financial-objects/1" >/dev/null
curl -fsS -X DELETE "$base/api/v1/financial-objects/2" >/dev/null
test "$(curl -fsS "$base/api/v1/financial-objects")" = "[]"
