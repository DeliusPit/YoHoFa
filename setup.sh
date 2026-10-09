#!/bin/sh
set -eu

info() { printf '%s\n' "$*"; }
die() { printf 'Fout: %s\n' "$*" >&2; exit 1; }
command_exists() { command -v "$1" >/dev/null 2>&1; }

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

[ -f compose.yml ] || die "voer dit script uit vanuit de repository-root of laat setup.sh daar staan"

get_env() {
  key=$1
  file=${2:-.env}
  [ -f "$file" ] || return 0
  sed -n "s/^${key}=//p" "$file" | tail -n 1
}

random_value() {
  if command_exists openssl; then
    openssl rand -hex 24
  else
    od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

prompt_default() {
  label=$1
  default=$2
  if [ -n "$default" ]; then
    printf '%s [%s]: ' "$label" "$default" >&2
  else
    printf '%s: ' "$label" >&2
  fi
  IFS= read -r value || value=
  if [ -z "$value" ]; then
    value=$default
  fi
  printf '%s' "$value"
}

prompt_secret() {
  label=$1
  default=$2
  if [ -n "$default" ]; then
    printf '%s [enter = standaard/bestaand]: ' "$label" >&2
  else
    printf '%s: ' "$label" >&2
  fi
  if [ -t 0 ]; then
    stty -echo
    IFS= read -r value || value=
    stty echo
    printf '\n' >&2
  else
    IFS= read -r value || value=
  fi
  if [ -z "$value" ]; then
    value=$default
  fi
  printf '%s' "$value"
}

yes_no() {
  label=$1
  default=${2:-n}
  if [ "$default" = "y" ]; then
    suffix='[J/n]'
  else
    suffix='[j/N]'
  fi
  printf '%s %s ' "$label" "$suffix" >&2
  IFS= read -r answer || answer=
  [ -n "$answer" ] || answer=$default
  case "$answer" in
    j|J|y|Y|ja|Ja|yes|Yes) return 0 ;;
    *) return 1 ;;
  esac
}

shell_quote() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\''/g")"
}

write_var() {
  key=$1
  value=$2
  printf '%s=' "$key" >> .install/install.env
  shell_quote "$value" >> .install/install.env
  printf '\n' >> .install/install.env
}

is_valid_s3_bucket() {
  bucket=$1
  length=${#bucket}
  [ "$length" -ge 3 ] && [ "$length" -le 63 ] || return 1
  if ! printf '%s' "$bucket" | grep -Eq '^[a-z0-9][a-z0-9.-]*[a-z0-9]$'; then
    return 1
  fi
  if printf '%s' "$bucket" | grep -Eq '\.\.'; then
    return 1
  fi
  if printf '%s' "$bucket" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
    return 1
  fi
  return 0
}

validate_s3_bucket() {
  is_valid_s3_bucket "$1" || die "S3 bucket moet 3-63 tekens zijn, alleen kleine letters/cijfers/punten/streepjes bevatten, met letter/cijfer beginnen en eindigen, en geen IP-adres zijn"
}

require_tools() {
  command_exists docker || die "Docker is niet geinstalleerd of staat niet in PATH"
  docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is niet beschikbaar; installeer docker compose"
}

require_tools

info "YoHoFa setup"
info "Deze setup verzamelt eerst alle invoer en schrijft daarna .install/install.sh."
info "Er wordt pas geinstalleerd nadat u bevestigt."
info ""
info "Opslagkeuze:"
info "  1) Complete YoHoFa-stack met lokale Garage/S3 container (standaard)"
info "  2) Bestaande S3/Garage storage gebruiken"

storage_choice=$(prompt_default "Kies 1 of 2" "1")
case "$storage_choice" in
  1) STORAGE_MODE=local-garage ;;
  2) STORAGE_MODE=external-s3 ;;
  *) die "ongeldige opslagkeuze" ;;
esac

AUTH_USERNAME=$(prompt_default "Gebruikersnaam voor YoHoFa" "$(get_env AUTH_USERNAME)")
AUTH_PASSWORD=$(prompt_secret "Wachtwoord voor YoHoFa" "$(get_env AUTH_PASSWORD)")
[ -n "$AUTH_USERNAME" ] || die "AUTH_USERNAME is verplicht"
[ -n "$AUTH_PASSWORD" ] || die "AUTH_PASSWORD is verplicht"

default_bucket=$(get_env S3_BUCKET)
if [ -z "$default_bucket" ] || ! is_valid_s3_bucket "$default_bucket"; then
  default_bucket=yohofa-docs
fi
S3_BUCKET=$(prompt_default "S3 bucket" "$default_bucket")
validate_s3_bucket "$S3_BUCKET"

if [ "$STORAGE_MODE" = "local-garage" ]; then
  S3_ENDPOINT=http://garage:3900
  S3_REGION=garage
  S3_USE_TLS=false
  S3_PATH_STYLE=true
  S3_CREATE_BUCKET=true
  default_access=$(get_env S3_ACCESS_KEY)
  default_secret=$(get_env S3_SECRET_KEY)
  [ -n "$default_access" ] || default_access=yohofa-$(random_value | cut -c1-16)
  [ -n "$default_secret" ] || default_secret=$(random_value)
  S3_ACCESS_KEY=$(prompt_default "Garage access key" "$default_access")
  S3_SECRET_KEY=$(prompt_secret "Garage secret key" "$default_secret")
  GARAGE_IMAGE=$(prompt_default "Garage image" "dxflrs/garage:v2.0.0")
  GARAGE_RPC_SECRET=$(random_value)
  GARAGE_ADMIN_TOKEN=$(random_value)
else
  S3_ENDPOINT=$(prompt_default "S3 endpoint URL" "$(get_env S3_ENDPOINT)")
  S3_ACCESS_KEY=$(prompt_default "S3 access key" "$(get_env S3_ACCESS_KEY)")
  S3_SECRET_KEY=$(prompt_secret "S3 secret key" "$(get_env S3_SECRET_KEY)")
  S3_REGION=$(prompt_default "S3 region" "$(get_env S3_REGION)")
  [ -n "$S3_REGION" ] || S3_REGION=garage
  S3_USE_TLS=$(prompt_default "S3 TLS gebruiken (true/false)" "$(get_env S3_USE_TLS)")
  [ -n "$S3_USE_TLS" ] || S3_USE_TLS=false
  S3_PATH_STYLE=$(prompt_default "S3 path-style requests (true/false)" "$(get_env S3_PATH_STYLE)")
  [ -n "$S3_PATH_STYLE" ] || S3_PATH_STYLE=true
  S3_CREATE_BUCKET=$(prompt_default "Bucket aanmaken indien nodig (true/false)" "$(get_env S3_CREATE_BUCKET)")
  [ -n "$S3_CREATE_BUCKET" ] || S3_CREATE_BUCKET=false
  GARAGE_IMAGE=
  GARAGE_RPC_SECRET=
  GARAGE_ADMIN_TOKEN=
fi

[ -n "$S3_ENDPOINT" ] || die "S3 endpoint is verplicht"
[ -n "$S3_ACCESS_KEY" ] || die "S3 access key is verplicht"
[ -n "$S3_SECRET_KEY" ] || die "S3 secret key is verplicht"

PDF_AI_URL=$(prompt_default "PDF/OCR AI URL (optioneel)" "$(get_env PDF_AI_URL)")
PDF_AI_MODEL=$(prompt_default "Document-AI model" "$(get_env PDF_AI_MODEL)")
[ -n "$PDF_AI_MODEL" ] || PDF_AI_MODEL=Qwen3.8-Flash-Next-UD-Q4_K_XL
TRANSACTION_AI_URL=$(prompt_default "Transactie-AI URL (optioneel)" "$(get_env TRANSACTION_AI_URL)")
TRANSACTION_AI_MODEL=$(prompt_default "Transactie-AI model" "$(get_env TRANSACTION_AI_MODEL)")
[ -n "$TRANSACTION_AI_MODEL" ] || TRANSACTION_AI_MODEL=Qwen3.6-35B-A3B-Q4_K_M

mkdir -p .install
chmod 700 .install || true
: > .install/install.env
chmod 600 .install/install.env || true

write_var STORAGE_MODE "$STORAGE_MODE"
write_var AUTH_USERNAME "$AUTH_USERNAME"
write_var AUTH_PASSWORD "$AUTH_PASSWORD"
write_var S3_ENDPOINT "$S3_ENDPOINT"
write_var S3_ACCESS_KEY "$S3_ACCESS_KEY"
write_var S3_SECRET_KEY "$S3_SECRET_KEY"
write_var S3_BUCKET "$S3_BUCKET"
write_var S3_USE_TLS "$S3_USE_TLS"
write_var S3_REGION "$S3_REGION"
write_var S3_PATH_STYLE "$S3_PATH_STYLE"
write_var S3_CREATE_BUCKET "$S3_CREATE_BUCKET"
write_var PDF_AI_URL "$PDF_AI_URL"
write_var PDF_AI_MODEL "$PDF_AI_MODEL"
write_var TRANSACTION_AI_URL "$TRANSACTION_AI_URL"
write_var TRANSACTION_AI_MODEL "$TRANSACTION_AI_MODEL"
write_var GARAGE_IMAGE "$GARAGE_IMAGE"
write_var GARAGE_RPC_SECRET "$GARAGE_RPC_SECRET"
write_var GARAGE_ADMIN_TOKEN "$GARAGE_ADMIN_TOKEN"

cat > .install/install.sh <<'INSTALL'
#!/bin/sh
set -eu

info() { printf '%s\n' "$*"; }
die() { printf 'Fout: %s\n' "$*" >&2; exit 1; }

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$SCRIPT_DIR"

. ./.install/install.env

docker compose version >/dev/null 2>&1 || die "Docker Compose plugin is niet beschikbaar"
docker info >/dev/null 2>&1 || die "Docker daemon is niet bereikbaar"

mkdir -p data/v3 config/banks config/translations config

cat > .env <<ENV
AUTH_USERNAME=$AUTH_USERNAME
AUTH_PASSWORD=$AUTH_PASSWORD
S3_ENDPOINT=$S3_ENDPOINT
S3_ACCESS_KEY=$S3_ACCESS_KEY
S3_SECRET_KEY=$S3_SECRET_KEY
S3_BUCKET=$S3_BUCKET
S3_USE_TLS=$S3_USE_TLS
S3_REGION=$S3_REGION
S3_PATH_STYLE=$S3_PATH_STYLE
S3_CREATE_BUCKET=$S3_CREATE_BUCKET
PDF_AI_URL=$PDF_AI_URL
PDF_AI_MODEL=$PDF_AI_MODEL
TRANSACTION_AI_URL=$TRANSACTION_AI_URL
TRANSACTION_AI_MODEL=$TRANSACTION_AI_MODEL
ENV
chmod 600 .env || true

COMPOSE_FILES="-f compose.yml"

if [ "$STORAGE_MODE" = "local-garage" ]; then
  mkdir -p data/garage/meta data/garage/data config/garage
  cat > config/garage/garage.toml <<GARAGE
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"
replication_mode = "none"
rpc_bind_addr = "[::]:3901"
rpc_public_addr = "garage:3901"
rpc_secret = "$GARAGE_RPC_SECRET"

[s3_api]
s3_region = "$S3_REGION"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage.localhost"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "$GARAGE_ADMIN_TOKEN"
GARAGE

  cat > .install/compose.garage.yml <<GARAGECOMPOSE
services:
  garage:
    image: $GARAGE_IMAGE
    container_name: hohoho-garage
    restart: unless-stopped
    volumes:
      - ./config/garage/garage.toml:/etc/garage.toml:ro
      - ./data/garage/meta:/var/lib/garage/meta
      - ./data/garage/data:/var/lib/garage/data
    networks:
      - hohoho-backend
GARAGECOMPOSE
  COMPOSE_FILES="$COMPOSE_FILES -f .install/compose.garage.yml"

  info "Garage container starten..."
  docker compose $COMPOSE_FILES up -d garage
  info "Garage initialiseren..."
  tries=0
  until docker compose $COMPOSE_FILES exec -T garage /garage status >/dev/null 2>&1; do
    tries=$((tries + 1))
    [ "$tries" -lt 30 ] || die "Garage reageert niet op tijd"
    sleep 1
  done
  NODE_ID=$(docker compose $COMPOSE_FILES exec -T garage /garage node id 2>/dev/null | sed -n 's/.*\([0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f].*\)/\1/p' | head -n 1)
  if [ -n "$NODE_ID" ]; then
    docker compose $COMPOSE_FILES exec -T garage /garage layout assign -z dc1 -c 1 "$NODE_ID" >/dev/null 2>&1 || true
    docker compose $COMPOSE_FILES exec -T garage /garage layout apply --version 1 >/dev/null 2>&1 || true
  fi
  docker compose $COMPOSE_FILES exec -T garage /garage bucket create "$S3_BUCKET" >/dev/null 2>&1 || true
  docker compose $COMPOSE_FILES exec -T garage /garage key import --yes -n yohofa "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null 2>&1 || true
  docker compose $COMPOSE_FILES exec -T garage /garage bucket allow --read --write --owner "$S3_BUCKET" --key "$S3_ACCESS_KEY" >/dev/null 2>&1 || true
fi

info "YoHoFa bouwen en starten..."
docker compose $COMPOSE_FILES up --build -d
docker compose $COMPOSE_FILES ps
info "Klaar. Open http://localhost:8080"
INSTALL
chmod 700 .install/install.sh

info ""
info "Installatieplan geschreven naar .install/install.sh"
info "Configuratie geschreven naar .install/install.env"
info "Opslagmodus: $STORAGE_MODE"
if [ "$STORAGE_MODE" = "local-garage" ]; then
  info "Garage wordt lokaal als container gestart; poorten worden niet naar de host gepubliceerd."
else
  info "Bestaande S3 endpoint: $S3_ENDPOINT"
fi
info ""

if yes_no "Wilt u nu installeren?" "n"; then
  ./.install/install.sh
else
  info "Niet geinstalleerd. Test/controleer eventueel .install/install.sh en voer later uit met:"
  info "  ./.install/install.sh"
fi
