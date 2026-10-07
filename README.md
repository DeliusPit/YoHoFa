# HoHoHo

HoHoHo is een huishoudadministratie voor een enkel huishouden. De applicatie brengt bezittingen, schulden, lasten, reserveringen, inkomsten, uitgaven, bankimporten en documenten samen in een browserinterface. Een installatie bevat precies een administratie: er zijn geen gebruikers-, huishouden- of tenanttabellen.

De applicatie bestaat uit een Go-service die zowel de API als de gebouwde webinterface serveert. SQLite bewaart alle gestructureerde gegevens. Originele documentbestanden staan in een S3-compatibele objectstore, normaal gesproken Garage. De browser ontvangt nooit S3-sleutels.

## Wat de applicatie doet

Financiële objecten vormen de kern van de administratie. Elk object is een bezit, schuld, last of reservering. Waarden zijn historisch: de meest recente waardering op of voor de gekozen peildatum bepaalt de actuele waarde. Transacties kunnen aan een of meer objecten worden toegewezen, ook gedeeltelijk.

Het dashboard toont de balans op een gekozen periode-einde en de inkomsten, uitgaven en het saldo binnen die periode. Reserveringen en lasten staan bewust apart van het vermogen.

CSV-bankbestanden worden met configureerbare YAML-profielen gelezen. De import bewaart bronregels, maakt genormaliseerde transacties en slaat herhaalde regels per importprofiel over. AI-classificatie is een afzonderlijke, optionele stap: zij koppelt nog niet-geclassificeerde transacties aan bestaande financiële objecten.

Documenten worden naar S3/Garage geupload terwijl SQLite metadata, tekst, analyse en objectkoppelingen bewaart. Een optionele externe PDF-AI-service kan tekst, vertaling en analyse leveren.

De ingebouwde backupfunctie maakt consistente snapshots van SQLite, documentobjecten en runtimeconfiguratie met restic en rclone. Backups hebben planning en retentie, worden gecontroleerd met `restic check` en krijgen een niet-destructieve restore-proef voordat zij als geslaagd gelden.

## Vereisten

- Docker Engine met Docker Compose.
- Een bereikbare S3-compatibele objectstore. Garage is de beoogde provider.
- In productie: een reverse proxy of load balancer die TLS afhandelt voor poort 8080.

De Compose-definitie start alleen HoHoHo. Garage, een reverse proxy en een eventuele PDF-AI-service zijn externe voorzieningen. Gebruik één app-proces per SQLite-bestand; plaats het bestand niet op een gedeeld netwerkfilesystem.

## Installatie

1. Maak de runtimeconfiguratie vanuit het voorbeeldbestand.

   ```sh
   cp .env.example .env
   ```

2. Vul in `.env` ten minste de productieauthenticatie en S3-instellingen in. Gebruik voor een bestaande Garage-installatie de endpoint-, bucket- en toegangssleutels daarvan.

   Vereiste instellingen voor productie:

   ```text
   AUTH_USERNAME=
   AUTH_PASSWORD=
   S3_ENDPOINT=
   S3_ACCESS_KEY=
   S3_SECRET_KEY=
   S3_BUCKET=hohoho-v3-documents
   ```

   Voeg waar nodig `S3_REGION`, `S3_PATH_STYLE`, `S3_USE_TLS`, `PDF_AI_URL`, `TRANSACTION_AI_URL` en `TRANSACTION_AI_MODEL` toe. Garage gebruikt doorgaans path-style S3-toegang. Commit `.env` en `config/.env` nooit.

3. Start de applicatie.

   ```sh
   docker compose up --build -d
   ```

4. Open `http://localhost:8080` en meld aan met de ingestelde Basic-authenticatie.

De SQLite-database staat standaard op de host in `data/v3/hohoho.db` en is in de container beschikbaar als `/app/data/hohoho.db`. Bewaar de volledige directory `data/v3`, inclusief eventuele WAL- en SHM-bestanden. Een nieuwe database bevat geen voorbeeldgegevens.

## Beschikbaarheid en beheer

- `GET /healthz` controleert of SQLite beschikbaar is.
- `GET /readyz` controleert SQLite en de geconfigureerde S3-bucket.
- De configuratiepagina toont de status van SQLite, S3, PDF-AI, transactie-AI en backup.
- De backupsectie configureert de resticrepository, planning en retentie. Herstel vereist de bevestiging `HERSTEL` en wordt bij de volgende start uitgevoerd.

Maak vóór een upgrade een geslaagde backup. De SQLite-migraties gebruiken `PRAGMA user_version`; de huidige applicatie kan een database met een nieuwere schema-versie niet openen. Oude v2-databases worden niet geconverteerd of automatisch verwijderd.

## Ontwikkeling en verificatie

Voor lokaal ontwikkelen zijn Go 1.25+ en Node.js 24+ nodig.

```sh
./scripts/dev-api.sh
```

Start voor frontendontwikkeling in een tweede terminal:

```sh
cd web
npm ci
npm run dev
```

De belangrijkste controles zijn:

```sh
go build ./...
go test ./...
./scripts/check-local.sh
docker build -t hohoho-check .
```

`check-local.sh` gebruikt een tijdelijke SQLite-database en controleert onder andere objecten, waarden, transacties, bankimportdeduplicatie, dashboard en persistentie na herstart. De scriptcheck gebruikt geen echte S3-, AI- of backupcredentials.

## Ontwerp en API

- [Datamodel v3](docs/data-model-v3.md): de negen domeintabellen en ontwerpregels.
- [Technische architectuur v3](docs/technical-architecture-v3.md): runtime, opslag, backup en deployment.
- [API v3](docs/api-v3.md): actieve HTTP-routes en invoerregels.
- [Documentatie-index](docs/index.md): actieve en historische documentatie.

Historische v2-documenten zijn alleen naslagwerk. De actieve runtime gebruikt uitsluitend de v3 SQLite/S3-architectuur.
