# Geldzaken

Applicatie-opbouw voor persoonlijke geldzaken van een huishouden.

## Doel

Een zo veel mogelijk in Go geschreven applicatie met een kleine JavaScript frontend, configureerbare opslag en deployment naar een LXC-container op een Proxmox-cluster.

## Stack

- Backend: Go HTTP API
- Frontend: Vite + vanilla JavaScript
- Data tijdens lokale ontwikkeling en productie: configureerbaar per opslagprofiel
- Geconfigureerde databasekeuzes: PostgreSQL, SQLite en MySQL
- PostgreSQL schema: SQL-bestanden in `migrations/` en schema-check bij starten
- Lokaal draaien: Go API + Vite frontend
- Deployment: build artifact + systemd service in LXC

## Structuur

- `cmd/api`: startpunt van de backend
- `internal/config`: configuratie via environment variables of een beveiligd configuratiebestand
- `internal/httpserver`: HTTP routes en handlers
- `internal/store`: data-opslag via SQLite, PostgreSQL, MySQL of memory-store
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

PostgreSQL-data staat onder `data/postgres`; de lokale MinIO-service gebruikt `data/minio`; SQLite-bestanden staan onder `data/sqlite` en objecten van de filesystem-backend onder `data/objects`. MinIO is hier alleen een lokale S3-compatible provider. Voor productie kan dezelfde S3-laag ook naar Garage of een andere S3-compatible dienst wijzen. SQLite-paden moeten absoluut zijn, mogen geen URI of `/` zijn en blijven in Compose beperkt tot `/app/data/sqlite` via `DATABASE_DATA_ROOT`.

Let op: `docker-compose.yml` is een oude PostgreSQL-only helper. Gebruik voor de actuele applicatiestack `compose.yml`.

De optionele MySQL 8.4-service start niet standaard. Vul de vier `MYSQL_*`-waarden uit `.env.example` in en start hem alleen met:

```sh
docker compose -f compose.yml --profile mysql up -d mysql
```

De persistente MySQL-data staat onder `data/mysql`. Poort 3306 is uitsluitend op `127.0.0.1` van de host gepubliceerd. De lokale Compose-service configureert geen TLS; kies voor deze service expliciet MySQL TLS-modus `disabled`, uitsluitend omdat applicatie en database over het afgeschermde Compose-netwerk communiceren.

## Documenten

Documentmetadata en koppelingen staan in de geconfigureerde relationele database. De originele bestanden kunnen in S3-compatibele objectopslag of op het lokale bestandssysteem staan. Compose koppelt het bestandssysteem standaard aan `/app/data/objects`; dit pad moet absoluut zijn en mag niet `/` zijn. Voor S3 gebruikt de applicatie `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_USE_TLS`, `S3_REGION`, `S3_PATH_STYLE` en `S3_CREATE_BUCKET`. MinIO blijft als lokale Compose-service beschikbaar om de S3-laag lokaal te testen.

Voor Garage bevat `S3_ENDPOINT` alleen `host:poort`, zonder `http://` of pad. Stel `S3_REGION` in op de regio van de Garage-key, gebruik normaal `S3_PATH_STYLE=true` en zet `S3_CREATE_BUCKET=false` wanneer de bucket vooraf in Garage is aangemaakt.

### Opslagconfiguratie

`config/storage.json` op de host, gekoppeld als `/app/config/storage.json`, is de gezaghebbende opslagconfiguratie. Het bestand bevat database- en objectopslagcredentials en is daarom geheim: maak geen commit of backup zonder passende versleuteling en toegangsbeperking. Als het bestand bestaat, wint het volledig van alle legacy opslagvariabelen; velden worden nooit afzonderlijk met environment variables samengevoegd. De root `.env` bevat alleen Compose-credentials voor de lokale PostgreSQL-, optionele MySQL- en MinIO-services. Productie-authenticatie en optionele providerneutrale bootstrapwaarden staan lokaal in `config/.env`, gebaseerd op `config/.env.example`, met bestandsmodus `0600`. Zonder zowel `AUTH_USERNAME` als `AUTH_PASSWORD` start de productiebackend niet.

Voor een eenmalige omzetting van de huidige Compose-variabelen naar het bestand gebruikt u exact:

```sh
CONFIG_BOOTSTRAP_FROM_ENV=true docker compose -f compose.yml up -d
```

Daarna gebruikt elke normale herstart alleen het bestand:

```sh
docker compose -f compose.yml up -d
```

Zonder bestand blijft de bestaande environment-configuratie alleen-lezen werken wanneer opslagvariabelen aanwezig zijn. In productie zijn zowel `AUTH_USERNAME` als `AUTH_PASSWORD` verplicht; bij een ontbrekende waarde start de backend niet. Met volledige authenticatie staat `CONFIG_EDITABLE=true` wijzigingen via `Configuratie > Systeemconfiguratie` toe. Alleen niet-productieontwikkeling op een loopback-adres mag zonder authenticatie wijzigen; andere niet-productieomgevingen worden zonder volledige authenticatie alleen-lezen. Secrets worden uitsluitend als ingesteld/niet ingesteld getoond. Wijzigingen worden atomisch met bestandsmodus `0600` opgeslagen en gelden na een herstart.

Een succesvolle wijziging schrijft opslagconfiguratieversie 2. Daarin blijven de afzonderlijke SQLite-, PostgreSQL- en MySQL-instellingen bewaard wanneer een andere backend wordt gekozen; hetzelfde geldt voor filesystem- en S3-instellingen. Een bestaand ongenummerd bestand met `database.backend` en `database.url` blijft exact compatibel en wordt pas bij een succesvolle wijziging naar versie 2 geschreven. De veilige GET-API retourneert alleen of wachtwoorden/sleutels zijn ingesteld, nooit de geheimen zelf.

De profielen `local`, `server` en `central` zijn ondersteund. `local` combineert SQLite met filesystem-opslag; `server` en `central` combineren PostgreSQL of MySQL met filesystem- of S3-opslag. Alle drie databases implementeren documentenmetadata, assets, liabilities, dossiers, relaties, waarderingen en tijdlijnen. MySQL TLS is beperkt tot `required` (standaard, zonder plaintext fallback) en expliciet `disabled` voor een vertrouwd lokaal netwerk zoals Compose. Eerdere `preferred`-instellingen worden veilig als `required` behandeld. CA-instellingen zijn nog niet geïmplementeerd. Een andere selectie wijzigt alleen de actieve backend en migreert geen bestaande databasegegevens of objecten.

### Systeembackup

`Configuratie > Systeembackup` maakt handmatig een versleutelde restic-snapshot van de actieve database, documenten en configuratie. Rclone verzorgt de verbinding met de gekozen opslagprovider. Vul als doel een bestaande rclone-remote en pad in, bijvoorbeeld `backup:geldzaken`, samen met de bijbehorende rclone-configuratie en een afzonderlijk restic-wachtwoord. Opslaan valideert de opslag en initialiseert uitsluitend een lege doelmap; de knop `Nu backup maken` wordt pas daarna actief.

De backupinstellingen staan met bestandsmodus `0600` in `config/backup.json`. Dit bestand bevat de rclone-configuratie en het restic-wachtwoord en mag niet ongecodeerd buiten het systeem worden bewaard. Bewaar het restic-wachtwoord daarnaast in een afzonderlijke herstelkluis: zonder dat wachtwoord kunnen snapshots niet worden hersteld. Gebruik voor S3 altijd een andere bucket dan de live documentenbucket; de applicatie blokkeert een rechtstreeks geconfigureerde overlap.

Onder `Configuratie > Systeembackup > Backup herstellen` kan een bestaande snapshot worden gekozen. De applicatie maakt eerst een extra veiligheidsbackup, herstart in herstelmodus en vervangt daarna database, documenten en `storage.json`. Tijdens de herstart is de applicatie tijdelijk niet bereikbaar. De gekozen snapshot moet hetzelfde database- en objectopslagtype gebruiken als de actieve configuratie.

Als de webinterface niet start, is hetzelfde herstel als onderhoudscommando beschikbaar:

```sh
docker compose stop backend
docker compose run --rm --no-deps --entrypoint /app/geldzaken-recover backend --list
docker compose run --rm --no-deps --entrypoint /app/geldzaken-recover backend --snapshot SNAPSHOT_ID
docker compose up -d backend
```

Voer het herstelcommando alleen uit terwijl de backend is gestopt. Ook het noodcommando maakt standaard eerst een veiligheidsbackup; `--no-safety-backup` is uitsluitend bedoeld voor situaties waarin de actieve opslag niet meer leesbaar is.

Documenten zijn zelfstandige objecten. Een koppeling met een transactie verwijderen verwijdert het document of het onderliggende object in filesystem/S3 niet.

## Bezittingen

Bezittingen zijn zelfstandige domeinobjecten in de relationele store. Ze kunnen via generieke relaties aan transacties en via de bestaande documentlaag aan documenten worden gekoppeld. Waarden worden intern als centen opgeslagen; de valuta staat per bezitting vastgelegd. `assets.current_value` is uitsluitend een transactioneel bijgewerkte cache van de nieuwste regel in `asset_valuations` en wordt niet rechtstreeks via assetbeheer aangepast.

## Schulden

Schulden zijn zelfstandige domeinobjecten met type, schuldeiser, looptijd, oorspronkelijk bedrag en actuele balans. Ze kunnen aan transacties en bewijsdocumenten worden gekoppeld. Een asset kan met `financed_by` aan de liability worden gekoppeld die de asset financiert.

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
