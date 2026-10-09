# YoHoFa

YoHoFa (Your Own Household Financial Assistant) is een huishoudadministratie voor een enkel huishouden. De applicatie brengt bezittingen, schulden, lasten, reserveringen, inkomsten, uitgaven, bankimporten en documenten samen in een browserinterface. Een installatie bevat precies een administratie: er zijn geen gebruikers-, huishouden- of tenanttabellen.

De applicatie bestaat uit een Go-service die zowel de API als de gebouwde webinterface serveert. SQLite bewaart alle gestructureerde gegevens. Originele documentbestanden staan in een S3-compatibele objectstore, normaal gesproken Garage. De browser ontvangt nooit S3-sleutels.

## Wat de applicatie doet

Financiële objecten vormen de kern van de administratie. Elk object is een bezit, schuld, last of reservering. Waarden zijn historisch: de meest recente waardering op of voor de gekozen peildatum bepaalt de actuele waarde. Transacties kunnen aan een of meer objecten worden toegewezen, ook gedeeltelijk.

Het dashboard toont de balans op een gekozen periode-einde en de inkomsten, uitgaven en het saldo binnen die periode. Reserveringen en lasten staan bewust apart van het vermogen.

CSV-bankbestanden worden met configureerbare YAML-profielen gelezen. De import bewaart bronregels, maakt genormaliseerde transacties en slaat herhaalde regels per importprofiel over. AI-classificatie is een afzonderlijke, optionele stap: zij koppelt nog niet-geclassificeerde transacties aan bestaande financiële objecten.

Documenten worden naar S3/Garage geupload terwijl SQLite metadata, tekst, analyse en objectkoppelingen bewaart. YoHoFa haalt tekst lokaal uit PDF's en afbeeldingen met `pdftotext`, `ocrmypdf` en Tesseract; een OpenAI-compatible taalmodel wordt alleen gebruikt voor vertaling, analyse en objectkoppeling.

De ingebouwde backupfunctie maakt consistente snapshots van SQLite, documentobjecten en runtimeconfiguratie met restic en rclone. Backups hebben planning en retentie, worden gecontroleerd met `restic check` en krijgen een niet-destructieve restore-proef voordat zij als geslaagd gelden.

## Vereisten

- Linux-host of vergelijkbare shellomgeving waarop `sh`-scripts kunnen draaien.
- `git` om de repository op te halen en te updaten.
- Docker Engine met de Docker Compose plugin (`docker compose`).
- Een gebruiker die Docker mag gebruiken, of toegang via `sudo` buiten dit script om.
- Voor de standaardinstallatie: geen aparte objectstore; `setup.sh` start Garage als onderdeel van de stack.
- Optioneel: een bestaande S3-compatibele objectstore als je geen lokale Garage-container wilt gebruiken.
- In productie: een reverse proxy of load balancer die TLS afhandelt voor poort 8080.

De standaardinstallatie start YoHoFa met een lokale Garage/S3-container. Een reverse proxy en een eventuele OpenAI-compatible AI-server blijven externe voorzieningen. Gebruik één app-proces per SQLite-bestand; plaats het bestand niet op een gedeeld netwerkfilesystem.

## Installatie

1. Haal de repository op en ga naar de projectmap.

   ```sh
   git clone <repository-url> geldzaken
   cd geldzaken
   ```

   Bij een bestaande checkout:

   ```sh
   git pull
   cd geldzaken
   ```

2. Draai de setup.

   ```sh
   ./setup.sh
   ```

   `setup.sh` vraagt eerst alle invoer uit. Standaard kiest het script voor een complete stack met lokale Garage/S3. Je kunt ook kiezen voor bestaande S3/Garage-storage; dan vraagt het script om endpoint, bucket en credentials.

   Na de vragen schrijft het script een lokaal installatieplan naar `.install/install.sh` en de gekozen waarden naar `.install/install.env`. Pas na de vraag `Wilt u nu installeren?` wordt Docker echt gestart. Kies je `nee`, dan kun je het gegenereerde installscript eerst controleren of later uitvoeren.

3. Voor een productieinstallatie vraagt `setup.sh` minimaal om authenticatie en S3-credentials. Bij de standaardkeuze worden Garage-credentials gebruikt om de lokale Garage-container te initialiseren. Bij bestaande storage gebruik je de endpoint-, bucket- en toegangssleutels daarvan.

   Vereiste instellingen voor productie:

   ```text
   AUTH_USERNAME=
   AUTH_PASSWORD=
   S3_ENDPOINT=
   S3_ACCESS_KEY=
   S3_SECRET_KEY=
   S3_BUCKET=hohoho-v3-documents
   ```

   Voeg waar nodig `S3_REGION`, `S3_PATH_STYLE`, `S3_USE_TLS`, `PDF_AI_URL`, `PDF_AI_MODEL`, `TRANSACTION_AI_URL` en `TRANSACTION_AI_MODEL` toe. Garage gebruikt doorgaans path-style S3-toegang. Commit `.env`, `config/.env`, `.install/` en `config/garage/` nooit.

4. Als je bij de bevestiging `nee` kiest, start je later met het gegenereerde installscript.

   ```sh
   ./.install/install.sh
   ```

5. Open `http://localhost:8080` en meld aan met de ingestelde Basic-authenticatie.

De SQLite-database staat standaard op de host in `data/v3/hohoho.db` en is in de container beschikbaar als `/app/data/hohoho.db`. De configuratiepagina kan databasebestanden binnen `/app/data` selecteren, bijvoorbeeld later `example.db` of `hohoho.db`; een andere keuze wordt actief na herstart. Bewaar de volledige directory `data/v3`, inclusief eventuele WAL- en SHM-bestanden. Een nieuwe database bevat geen voorbeeldgegevens.

De runtime-image bevat de documentverwerkingssoftware die nodig is voor lokale tekstextractie en OCR: `poppler-utils`, `ocrmypdf`, `tesseract-ocr` en Tesseract-taaldata voor Nederlands, Engels, Duits en Frans. Deze software wordt automatisch meegenomen wanneer `setup.sh` of `docker compose up --build` de image bouwt.

## Update

Een update is dezelfde flow als installatie:

```sh
git pull
./setup.sh
```

Maak vóór een upgrade een geslaagde backup. De SQLite-migraties gebruiken `PRAGMA user_version`; de huidige applicatie kan een database met een nieuwere schema-versie niet openen. Oude v2-databases worden niet geconverteerd of automatisch verwijderd.

## Beschikbaarheid en beheer

- `GET /healthz` controleert of SQLite beschikbaar is.
- `GET /readyz` controleert SQLite en de geconfigureerde S3-bucket.
- De configuratiepagina toont de status van SQLite, S3, document-AI, transactie-AI en backup.
- De backupsectie configureert de resticrepository, planning en retentie. Herstel vereist de bevestiging `HERSTEL` en wordt bij de volgende start uitgevoerd.

Handige beheercommando's:

```sh
docker compose ps
docker compose logs -f backend
docker compose restart backend
docker compose down
```

Als `./setup.sh` meldt dat Docker niet bereikbaar is, controleer dan of de Docker daemon draait en of de huidige gebruiker Docker mag gebruiken. Als `readyz` faalt maar `healthz` werkt, is SQLite bereikbaar maar S3/Garage waarschijnlijk niet correct geconfigureerd of niet bereikbaar.

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
