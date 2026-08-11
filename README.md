# Geldzaken

Applicatie-opbouw voor persoonlijke geldzaken van een huishouden.

## Doel

Een zo veel mogelijk in Go geschreven applicatie met een kleine JavaScript frontend, SQLite als standaard database en deployment naar een LXC-container op een Proxmox-cluster.

## Stack

- Backend: Go HTTP API
- Frontend: Vite + vanilla JavaScript
- Data tijdens lokale ontwikkeling en productie: PostgreSQL
- Vorige opslaglaag: SQLite, alleen nog aanwezig voor migratie/fallback zolang cleanup niet is afgerond
- PostgreSQL schema: SQL-bestanden in `migrations/` en schema-check bij starten
- Lokaal draaien: Go API + Vite frontend
- Deployment: build artifact + systemd service in LXC

## Structuur

- `cmd/api`: startpunt van de backend
- `internal/config`: configuratie via environment variables
- `internal/httpserver`: HTTP routes en handlers
- `internal/store`: data-opslag via SQLite, memory-store of Postgres
- `migrations`: database schema migraties
- `web`: frontend
- `deploy`: systemd en Proxmox/LXC deployment-opbouw
- `scripts`: hulpscripts voor lokale en remote taken

## Lokaal ontwikkelen met PostgreSQL

Vereisten: Go en Node.js.

De standaardconfiguratie gebruikt `STORE_BACKEND=postgres` met `DATABASE_URL`. Via Docker Compose draait PostgreSQL naast de applicatie.

```sh
cp .env.example .env
docker compose up -d postgres
./scripts/dev-api.sh
```

Frontend:

```sh
cd web
npm install
npm run dev
```

API healthcheck:

```sh
curl http://localhost:8080/healthz
```

Samenvatting:

```sh
curl http://localhost:8080/api/v1/summary
```

Snelle lokale check met tijdelijke SQLite database:

```sh
./scripts/check-local.sh
```

## Docker Compose

De applicatiecontainer bouwt de Go API uit `cmd/api` en de Vite frontend uit `web/`. De Go-server serveert de frontend en gebruikt PostgreSQL als actieve databasebackend.

```sh
docker compose -f compose.yml up --build -d
```

PostgreSQL-data staat onder `data/postgres`. MinIO blijft beschikbaar in Compose maar wordt nog niet door de applicatie gebruikt.

## Eerste domeinen

De initiële database-opbouw bevat:

- huishoudens
- rekeningen
- hoofdcategorieen
- subcategorieen
- transacties

SQLite bewaart dit standaard in `data/geldzaken.db`. De belangrijkste tabellen zijn:

- `accounts`: bankrekeningen met naam, IBAN, type, eigenaar en bankprofiel
- `main_categories`: hoofdcategorieen zoals Wonen, Huishouden en Vervoer
- `categories`: subcategorieen met hoofdcategorie, soort en doel
- `transactions`: transacties met rekening, optionele categorie, datum, bedrag, tegenpartij en importreferentie

De eerste hoofdcategorieen zijn: Inkomsten, Wonen, Huishouden, Medische kosten, Verzekeringen, Telecom, Vervoer, Educatie, Kleding, Vrije tijd en Overige uitgaven.

Categorieen hebben twee aparte eigenschappen:

- `kind`: boekhoudkundige richting, zoals `income`, `expense` of `transfer`
- `purpose`: huishoudelijk doel, zoals `daily`, `reservation` of `investment`

Daardoor blijven dagelijkse uitgaven, reserveringen en huishoudelijke investeringen vanaf het begin gescheiden zonder dat dit transacties of budgetten later ingewikkelder maakt. Investeringen zijn hier bijvoorbeeld woningverbetering, verduurzaming of duurzame aankopen, niet primair vermogensopbouw.

## Bankimport

De applicatie heeft een eerste import-preview voor CSV-bestanden van banken. Uploaden slaat nog niets op; het leest alleen transacties zodat het formaat gecontroleerd kan worden.

Beschikbare profielen: `generic`, `ing`, `rabobank`, `abnamro`, `asn_sns_regiobank`, `sparda`, `triodos` en `ics`. ICS gebruikt het PDF-afschrift; de overige profielen gebruiken CSV.

Testbestand:

```text
samples/bankimport/generic.csv
```

Meer details staan in `docs/bankimport.md`.

## Deployment richting LXC/Proxmox

Zie `deploy/proxmox/README.md` voor de beoogde flow. De basis is:

1. Maak een Debian/Ubuntu LXC aan in Proxmox.
2. Installeer Go in de container en zorg voor SSH-toegang.
3. Draai lokaal `LXC_HOST=<ip> AUTH_PASSWORD='<wachtwoord>' ./scripts/install-lxc.sh`.
4. De applicatie draait daarna met SQLite, systemd en HTTP Basic Authentication op poort 8080.

De scripts zijn templates. Vul hostnamen, container-ID's, gebruikers en paden in via environment variables of CI secrets.
