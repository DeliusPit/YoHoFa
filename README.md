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

PostgreSQL-data staat onder `data/postgres`; documentobjecten staan onder `data/minio`.

## Documenten

Documentmetadata en koppelingen staan in PostgreSQL. De originele bestanden staan in de configureerbare MinIO-bucket. Compose gebruikt standaard `MINIO_BUCKET=geldzaken-documents`; voor lokaal starten buiten Compose zijn daarnaast `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY` en optioneel `MINIO_USE_SSL` nodig.

Documenten zijn zelfstandige objecten. Een koppeling met een transactie verwijderen verwijdert het document of het MinIO-object niet.

## Bezittingen

Bezittingen zijn zelfstandige domeinobjecten in PostgreSQL. Ze kunnen via generieke relaties aan transacties en via de bestaande documentlaag aan documenten worden gekoppeld. Waarden worden intern als centen opgeslagen; de valuta staat per bezitting vastgelegd. `assets.current_value` is uitsluitend een transactioneel bijgewerkte cache van de nieuwste regel in `asset_valuations` en wordt niet rechtstreeks via assetbeheer aangepast.

## Schulden

Schulden zijn zelfstandige PostgreSQL-objecten met type, schuldeiser, looptijd, oorspronkelijk bedrag en actuele balans. Ze kunnen aan transacties en bewijsdocumenten worden gekoppeld. Een asset kan met `financed_by` aan de liability worden gekoppeld die de asset financiert.

## Dossiers

Dossiers (`cases`) groeperen een vrij gekozen, niet-lege classificatie, titel, beschrijving en optionele looptijd. De exacte statussen zijn `planned`, `active`, `completed` en `archived`. Een dossier kan met `relates_to` aan transacties, assets en liabilities worden gekoppeld; bewijsdocumenten wijzen met `evidence_for` naar het dossier. De gebruikersgerichte tijdlijn komt uit `domain_events` en is geen workflow.

## Domeinrelaties

`entity_relations` legt polymorfe relaties vast tussen cases, assets, liabilities, transacties en documenten. `relates_to` loopt van een case naar een transactie, asset of liability, of van een asset/liability naar een transactie. `evidence_for` loopt van een document naar een case, asset, transactie of liability. `financed_by` loopt van een asset naar een liability. PostgreSQL kan voor een type/id-paar geen normale foreign key naar meerdere mogelijke tabellen afdwingen; daarom controleert de repository het bestaan van beide objecten en blokkeren delete-operaties relaties waar nodig.

## Domeingebeurtenissen

`domain_events` is een automatisch bijgewerkte tijdlijnprojectie voor assets, liabilities en cases. Het is geen event-sourcingmodel, broker of technische auditlog. Mutaties en hun tijdlijnregel worden in dezelfde PostgreSQL-transactie opgeslagen. De API biedt alleen `GET /api/v1/assets/{id}/events`, `GET /api/v1/liabilities/{id}/events` en `GET /api/v1/cases/{id}/events`; gebeurtenissen kunnen niet rechtstreeks via de API worden aangemaakt, gewijzigd of verwijderd.

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
