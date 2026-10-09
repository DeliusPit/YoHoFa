#!/bin/sh
set -eu

info() { printf '%s\n' "$*"; }
die() { printf 'Fout: %s\n' "$*" >&2; exit 1; }
command_exists() { command -v "$1" >/dev/null 2>&1; }

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

[ -f compose.yml ] || die "voer dit script uit vanuit de repository-root of laat upgrade.sh daar staan"
command_exists docker || die "Docker is niet geinstalleerd of staat niet in PATH"
docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is niet beschikbaar; installeer docker compose"

COMPOSE_FILES="-f compose.yml"
if [ -f .install/compose.garage.yml ]; then
  COMPOSE_FILES="$COMPOSE_FILES -f .install/compose.garage.yml"
fi

if [ ! -f .env ] && [ ! -f config/.env ]; then
  die "geen .env of config/.env gevonden; draai eerst ./setup.sh"
fi

info "YoHoFa upgrade"
info ""
info "Controleer voor een upgrade of er een recente backup is."
info "De update bouwt de container opnieuw en start bestaande services met bestaande configuratie."
info ""

if [ -d data/v3 ]; then
  for db in data/v3/*.db; do
    [ -e "$db" ] || continue
    version=$(docker run --rm -v "$PWD:/work" -w /work alpine:3.23 sh -c "apk add --no-cache sqlite >/dev/null && sqlite3 '$db' 'PRAGMA user_version;'" 2>/dev/null || printf '?')
    info "Database: $db (schema user_version: $version)"
  done
fi

printf 'Doorgaan met upgrade? [j/N] ' >&2
IFS= read -r answer || answer=
case "$answer" in
  j|J|y|Y|ja|Ja|yes|Yes) ;;
  *) die "upgrade afgebroken" ;;
esac

info "Containers bouwen en starten..."
docker compose $COMPOSE_FILES up --build -d

info "Wachten op healthcheck..."
tries=0
until docker compose $COMPOSE_FILES exec -T backend sh -c 'auth=$(printf "%s:%s" "$AUTH_USERNAME" "$AUTH_PASSWORD" | base64); wget -q -O- --header "Authorization: Basic $auth" http://127.0.0.1:8080/healthz' >/dev/null 2>&1; do
  tries=$((tries + 1))
  [ "$tries" -lt 30 ] || die "healthz reageert niet op tijd"
  sleep 1
done

if docker compose $COMPOSE_FILES exec -T backend sh -c 'auth=$(printf "%s:%s" "$AUTH_USERNAME" "$AUTH_PASSWORD" | base64); wget -q -O- --header "Authorization: Basic $auth" http://127.0.0.1:8080/readyz' >/dev/null 2>&1; then
  info "Readiness: ok"
else
  info "Readiness: nog niet ok; SQLite werkt, maar S3/Garage of externe opslag is mogelijk nog niet bereikbaar."
fi

docker compose $COMPOSE_FILES ps
info "Upgrade klaar."
