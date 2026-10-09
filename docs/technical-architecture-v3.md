# YoHoFa v3 — eenvoudige browserapp met SQLite en Garage

Status: **v3-runtime, bankimport, documenten, document-AI en backup/herstel geïmplementeerd**. De actieve Go-service gebruikt SQLite, Garage/S3 en een browserinterface. Compose bouwt het app-image; `setup.sh` kan daarnaast een lokale Garage/S3-container bootstrappen of bestaande S3/Garage-storage gebruiken. Dit document beschrijft de gerealiseerde basis en verdere technische richting naast het [kernmodel met negen tabellen](data-model-v3.md).

**Uitvoeringsbesluit:** bestaande data mag vervallen. V3 start op `v3-simplification` met een nieuwe, lege SQLite-database en lege YoHoFa-documentopslag. Er is geen datamigrator, ID-mapping of migratiearchief nodig. Oude data verwijderen blijft een aparte uitvoeringsstap.

## 1. De vaste stack

| Onderdeel | Keuze |
| --- | --- |
| Gebruikersinterface | Browser, bestaande Vite/vanilla-JavaScript frontend |
| Applicatie | Eén Go-service vanuit `cmd/api`, met HTTP API en gebouwde frontend |
| Gestructureerde data | Alleen SQLite, als persistent bestand bij de Go-service |
| Originele documenten | Alleen S3-compatible opslag, met Garage als deploymentprovider |
| Documentverwerking | Lokale tekstextractie/OCR in de app-container plus OpenAI-compatible LLM |
| Deployment | Docker Compose |
| Administratie | Eén database/deployment per huishouden |

De gebruiker heeft alleen een browser nodig. SQLite draait ingebed in de Go-service, niet in de browser. De browser benadert de applicatie via HTTP; de Go-service leest en schrijft SQLite en S3. Er is één applicatie-origin voor zowel frontend als API. S3-credentials blijven aan de serverkant.

De bestaande frontend en Go-backend blijven de basis. Er is geen aparte frontend-runtimecontainer, databaseserver, worker-service of message broker nodig.

## 2. Samenhang en Compose

```text
Browser
   │ HTTP(S): frontend, API, upload en download
   ▼
YoHoFa (één Go-service)
   ├─ gebouwde frontend
   ├─ SQLite → persistent lokaal volume
   ├─ S3-client → Garage → persistente metadata- en objectvolumes
   └─ Document-AI → pdftotext/ocrmypdf/Tesseract + OpenAI-compatible LLM
```

De standaardinstallatie bestaat uit YoHoFa en optioneel een lokale Garage/S3-container die door `setup.sh` wordt gegenereerd. De basis-`compose.yml` bevat alleen YoHoFa en kan ook op een bestaande Garage/S3-installatie aansluiten:

- **YoHoFa:** één replica, bouw vanuit de repository-root, publiceert poort 8080 en mount een persistente datamap voor SQLite.
- **Garage:** S3-endpoint op het interne Compose-netwerk, eigen persistente metadata- en objectopslag en eigen configuratie. S3/RPC/admin-poorten worden niet naar buiten gepubliceerd voor applicatiegebruik.

De document-AI gebruikt lokale commandline-tools in de app-container voor tekstextractie en OCR. De geconfigureerde `PDF_AI_URL` is een OpenAI-compatible chat-completions endpoint voor vertaling en analyse; YoHoFa introduceert geen eigen LLM-runtime.

Compose-builds gebruiken een multi-stage aanpak: Node/Vite voor de frontendbuild, Go voor de binary en een Alpine-runtime-image met CA-certificaten, `restic`, `rclone`, `sqlite`, `poppler-utils`, `ocrmypdf`, `tesseract-ocr` en taaldata voor Nederlands, Engels, Duits en Frans. PostgreSQL-/MariaDB-clients en hun dumpcontroles zijn verwijderd.

Starten: `docker compose up --build -d` of interactief via `./setup.sh`. De huidige `compose.yml` start de SQLite-kern op poort 8080, mount `data/v3`, `config/translations` en `config/ai-docs.yaml` en bevat geen PostgreSQL-/MySQL-services.

### Garage-initialisatie

Een lege Garage-node vereist initialisatie: node/layout configureren, de layout toepassen, een bucket en applicatiesleutel aanmaken en bucketrechten toekennen. `setup.sh` genereert hiervoor een lokale Compose-overlay en installatieplan. Deze bootstrap mag bestaande Garage-data niet resetten.

De app start pas documentopslag wanneer bucket en toegang beschikbaar zijn. Een interne single-node Garage is het eenvoudige uitgangspunt; persistente volumes en backups verzorgen databehoud.

## 3. SQLite als enige structured-data-store

Het [canonieke SQLite-schema](../internal/store/core_schema.sql) bevat de negen domeintabellen en wordt ingebed door `internal/store/core.go`. De eerdere PostgreSQL-notatie is vertaald zonder het logische model uit te breiden:

| Aangeleverde PostgreSQL-notatie | SQLite |
| --- | --- |
| `BIGSERIAL PRIMARY KEY` | `INTEGER PRIMARY KEY`, met automatisch gegenereerde IDs |
| `BIGINT` | `INTEGER`, met 64-bit bedragen in centen |
| `DATE` | `TEXT`, door de applicatie gevalideerd als `YYYY-MM-DD` |
| `JSONB` | `TEXT`, met `json_valid`-checks |

`object_type` blijft applicatievalidatie voor de vijf toegestane types. Relatietypes blijven vrije tekst. JSON-configuratie en AI-response worden als JSON geparseerd, niet als ongevalideerde willekeurige strings gebruikt.

### Openen, concurrency en migraties

- SQLite staat op een lokaal persistent volume: Compose gebruikt `/app/data/hohoho.db`, lokaal is de default `data/v3/hohoho.db`. De volledige map wordt gemount, zodat eventuele `-wal`- en `-shm`-bestanden op dezelfde opslag staan.
- Eén Go-proces beheert het bestand; geen horizontale app-replicatie op hetzelfde bestand en geen gedeelde netwerkfilesystem-database.
- Foreign keys worden **per verbinding** aangezet en getest. Alleen de PRAGMA in een los SQL-bestand is hiervoor niet genoeg.
- Gebruik korte databasetransacties, een begrensde wachttijd voor locks en WAL-mode. Begin met één databaseverbinding, zoals de huidige SQLite-store, voor eenvoudig voorspelbaar schrijfgedrag.
- Wacht niet op S3-, OCR- of AI-calls terwijl een schrijftransactie open staat.
- Eén versieerbare SQLite-migratielijn is de bron voor schema-evolutie. `PRAGMA user_version` houdt momenteel versie 7 bij zonder een tiende domeintabel toe te voegen.
- Nieuwe en bestaande databases worden door hetzelfde migratiepad voorbereid. Parallelle PostgreSQL/MySQL-schema-implementaties zijn verwijderd.

SQLite is de bron van waarheid voor alle gestructureerde domeingegevens, inclusief documenttekst, analyse, bronregels, waarden en relaties. Browseropslag is geen primaire database of backup.

## 4. Documenten en Document-AI

Upload:

1. Browser uploadt het bestand naar de Go-API.
2. De API genereert een objectkey zonder businessbetekenis en schrijft de originele bytes naar Garage.
3. De API slaat het documentrecord en gekozen objectkoppelingen op in SQLite.
4. De Go-service zet het document in de analysequeue wanneer document-AI beschikbaar is.
5. De analyzer haalt bytes uit S3, extraheert tekst lokaal met `pdftotext` of OCR met `ocrmypdf`/Tesseract, vertaalt/analyseert via de OpenAI-compatible LLM en valideert object-ID's tegen bestaande financiële objecten.
6. Het resultaat wordt gemapt naar `extracted_text`, `translated_text`, `analysis_json` en koppelingen in `document_financial_objects`.

Download loopt via de Go-API, die de bijbehorende objectkey uit SQLite leest en het object uit Garage streamt. De browser kent geen permanente S3-toegangssleutels.

SQLite en S3 delen geen transactie. Als opslag van metadata na upload mislukt, moet de applicatie het nieuwe ongekoppelde object opruimen of als te reconciliëren object registreren buiten het domeinmodel. Document ontkoppelen verwijdert geen bestand. Definitief verwijderen krijgt een expliciete flow voor metadata en objectbytes.

Documentanalyse blijft beperkt achtergrondwerk in dezelfde Go-service. Er is een statusveld op `documents`, maar geen aparte jobtabel. Fouten zetten het document op foutstatus; batch- en selectieacties kunnen documenten opnieuw in de queue plaatsen zonder het oude workflowmodel terug te brengen.

## 5. Eenvoudige configuratie

In v3 kiest de gebruiker geen databasebackend, objectbackend of opslagprofiel. SQLite en S3 staan vast. Eén duidelijke deploymentconfiguratie uit Compose/environment vervangt het huidige bewaren en omschakelen van alternatieve opslagconfiguraties.

De actieve runtime leest alleen environment; `config/storage.json` wordt niet geopend. Compose leest de root-`.env` en tijdelijk daarna `config/.env` om bestaande credentials bereikbaar te houden. De laatste env-file heeft voorrang. De overgang naar uitsluitend de root-`.env` hoort bij de verdere deploymentopruiming.

Benodigde operationele instellingen:

| Instelling | Doel |
| --- | --- |
| `DATABASE_PATH` | Pad naar het persistente SQLite-bestand |
| `S3_ENDPOINT` | Garage-endpoint; intern bijvoorbeeld `garage:3900` |
| `S3_BUCKET` | Documentbucket |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | Server-side buckettoegang |
| `S3_REGION`, `S3_PATH_STYLE`, `S3_USE_TLS` | Bestaand S3-clientcontract; Garage gebruikt path-style, TLS passend bij het endpoint |
| `PDF_AI_URL`, `PDF_AI_MODEL` | OpenAI-compatible LLM voor documentvertaling en analyse |
| `TRANSACTION_AI_URL`, `TRANSACTION_AI_MODEL` | OpenAI-compatible transactieclassificatie voor bankimport |
| `AI_DOCS_CONFIG_PATH` | YAML-configuratie voor documentprompts, OCR-talen en tekstlimieten |
| `AUTH_USERNAME`, `AUTH_PASSWORD` | Bestaande productieauthenticatie behouden |

`HTTP_ADDR` en `WEB_DIR` kunnen vaste defaults in het app-image krijgen. `STORE_BACKEND`, `DATABASE_URL`, `MYSQL_*`, `OBJECT_BACKEND` en `STORAGE_PROFILE` verdwijnen als gebruikerskeuzes. De deployment gebruikt één bron voor instellingen; oude `config/storage.json`-waarden mogen nieuwe instellingen niet ongemerkt overschrijven. Secrets blijven buiten Git en buiten frontendresponses.

De UI toont alleen noodzakelijke instellingen/status voor de vaste stack. De databasebackend staat vast; binnen `/app/data` kan wel een fysiek `.db`-bestand worden gekozen, bijvoorbeeld `hohoho.db` of later `example.db`. De bestaande productieauthenticatie blijft bij frontend, API en documentdownloads van toepassing.

## 6. Backup en herstel

Een complete administratie bestaat uit SQLite **en** de bijbehorende Garage-objecten. De backupflow moet beide en de benodigde deploymentconfiguratie kunnen herstellen.

- Maak een consistente SQLite-snapshot via een SQLite-backupmechanisme of `VACUUM INTO`; kopieer een live database met WAL niet zomaar als één bestand.
- Voor een eenvoudige consistente database/object-backup: pauzeer mutaties, laat lopend werk afronden, maak de snapshot en bewaar de bijbehorende objectset. Daarna kunnen writes hervatten.
- Herstel het SQLite-bestand en de Garage-objecten samen; controleer ontbrekende objectsleutels voordat de applicatie weer schrijft.
- Test backup en herstel ook na containervervanging en na schemamigraties.

- De actieve implementatie houdt muterende API-verzoeken aan tijdens een snapshot. Zij maakt een SQLite `VACUUM INTO`-snapshot en een bijbehorende S3-kopie, bewaart beide met restic en schrijft een manifest.

- Standaard draait dagelijks om 02:30 een backup; de beheerder kan tijdstip en retentie aanpassen. De standaardretentie is 14 dagelijkse, 8 wekelijkse en 24 maandelijkse snapshots.

- Elke geslaagde backup voert `restic check` uit en herstelt de nieuwe snapshot tijdelijk. De restore-proef controleert het manifest en `PRAGMA integrity_check` op SQLite voordat de snapshot als geslaagd telt.

De backupservice en de operator-CLI gebruiken uitsluitend SQLite en S3. `restic` en `rclone` zijn onderdeel van het runtime-image; PostgreSQL/MySQL-backupbranches zijn verwijderd.

## 7. Gerealiseerde wijzigingen

| Onderdeel | Actieve v3-implementatie |
| --- | --- |
| `internal/store/` | Eén SQLite-implementatie van het negen-tabellenmodel |
| `internal/config/` | Alleen environment- en runtimeconfiguratie voor SQLite/S3 |
| `cmd/api/main.go` en `cmd/api/documentai.go` | SQLite en S3 openen; HTTP, shutdown, lokale OCR/document-AI beheren |
| `internal/objectstore/` | S3/Garage als enige documentstore |
| `compose.yml` | Eén app-service met persistent SQLite-volume, configuratiemounts en S3/Garage-configuratie via environment |
| `Dockerfile` | Runtime-image met SQLite, backuptools en documentverwerkingstools; zonder PostgreSQL/MariaDB-tooling |
| `web/src/main.js` | Objectgerichte UI, documenten, import en configuratie voor de vaste stack |
| `internal/systembackup` | Consistente SQLite/S3-snapshots, retentie, validatie en herstel |
| `go.mod`, tests en scripts | Alleen SQLite-, S3- en actieve end-to-endchecks |

V3 verwijdert geen bestaande databases of S3-objecten automatisch. Oude YoHoFa-data verwijderen blijft een aparte, expliciete uitvoeringsstap.

## 8. Acceptatiecriteria

- De gebruiker werkt volledig in een browser; frontend en API worden door één app-service geleverd.
- Alleen SQLite bewaart gestructureerde data, volgens het negen-tabellenmodel.
- Originele documenten staan in Garage/S3 en blijven na herstart bereikbaar.
- Eén deployment/database vertegenwoordigt één huishouden; opslagbackendselectie is verdwenen.
- Een nieuwe Compose-installatie kan na gedocumenteerde Garage-bootstrap starten zonder PostgreSQL/MySQL.
- Containerrebuild/vervanging behoudt SQLite en Garage-data.
- Foreign keys, allocaties, waardehistorie, importherkomst en objectrelaties werken op SQLite.
- Een backup/herstelproef levert dezelfde administratie en documentbytes op.

## 9. Verificatie van de gerealiseerde basis

`go build ./...` en `go test ./...` controleren de v3-runtime. `scripts/check-local.sh` start de echte API op een geïsoleerde tijdelijke database, controleert objecten/waarden/relaties en herstart/persistentie. De frontend wordt met Vite gebouwd. Productie-image en HTTP-startup worden afzonderlijk gecontroleerd.

De actieve `/healthz` meet SQLite; `/readyz` controleert ook S3. Ontbrekende/onbereikbare S3 maakt readiness 503 en blokkeert documentupload. Documentupload, analyse, download, import en gepland herstel zijn actieve v3-routes.
