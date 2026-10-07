# HoHoHo v3 — eenvoudige browserapp met SQLite en Garage

Status: **v3-runtime, bankimport, documenten en backup/herstel geïmplementeerd**. De actieve Go-service gebruikt SQLite, Garage/S3 en een browserinterface. Compose bouwt een klein app-image en gebruikt een bestaande Garage-configuratie. Beheerde Garage/bootstrap volgt later. Dit document beschrijft de gerealiseerde basis en verdere technische richting naast het [kernmodel met negen tabellen](data-model-v3.md).

**Uitvoeringsbesluit:** bestaande data mag vervallen. V3 start op `v3-simplification` met een nieuwe, lege SQLite-database en lege HoHoHo-documentopslag. Er is geen datamigrator, ID-mapping of migratiearchief nodig. Oude data verwijderen blijft een aparte uitvoeringsstap.

## 1. De vaste stack

| Onderdeel | Keuze |
| --- | --- |
| Gebruikersinterface | Browser, bestaande Vite/vanilla-JavaScript frontend |
| Applicatie | Eén Go-service vanuit `cmd/api`, met HTTP API en gebouwde frontend |
| Gestructureerde data | Alleen SQLite, als persistent bestand bij de Go-service |
| Originele documenten | Alleen S3-compatible opslag, met Garage als deploymentprovider |
| Documentverwerking | Bestaande externe PDF-AI-service |
| Deployment | Docker Compose |
| Administratie | Eén database/deployment per huishouden |

De gebruiker heeft alleen een browser nodig. SQLite draait ingebed in de Go-service, niet in de browser. De browser benadert de applicatie via HTTP; de Go-service leest en schrijft SQLite en S3. Er is één applicatie-origin voor zowel frontend als API. S3-credentials blijven aan de serverkant.

De bestaande frontend en Go-backend blijven de basis. Er is geen aparte frontend-runtimecontainer, databaseserver, worker-service of message broker nodig.

## 2. Samenhang en Compose

```text
Browser
   │ HTTP(S): frontend, API, upload en download
   ▼
HoHoHo (één Go-service)
   ├─ gebouwde frontend
   ├─ SQLite → persistent lokaal volume
   ├─ S3-client → Garage → persistente metadata- en objectvolumes
   └─ PDF-AI-client → bestaande externe PDF-AI-service
```

De uiteindelijke zelfstandige Compose-deployment bestaat uit twee langlopende services. De huidige Compose-definitie bevat alleen HoHoHo en sluit aan op een bestaande Garage-installatie:

- **HoHoHo:** één replica, bouw vanuit de repository-root, publiceert poort 8080 en mount een persistente datamap voor SQLite.
- **Garage:** S3-endpoint op het interne Compose-netwerk, eigen persistente metadata- en objectopslag en eigen configuratie. S3/RPC/admin-poorten worden niet naar buiten gepubliceerd voor applicatiegebruik.

De bestaande externe PDF-AI-service wordt via `PDF_AI_URL` benaderd. De HoHoHo-stack introduceert daarvoor geen extra AI-runtime. Een bestaande Garage-installatie kan via hetzelfde S3-contract worden aangesloten; de applicatieopslag blijft dezelfde vaste combinatie.

Compose-builds behouden de multi-stage aanpak: Node/Vite voor de frontendbuild, Go voor de binary, een klein runtime-image met CA-certificaten en uitsluitend benodigde runtimehulpmiddelen. De huidige `FROM postgres:18`, PostgreSQL-/MariaDB-clients en bijbehorende dumpcontroles vervallen in het doelimage. Backuphulpmiddelen blijven alleen aanwezig als de gekozen backupflow ze daadwerkelijk gebruikt.

Starten: `docker compose up --build -d`. De huidige `compose.yml` start de SQLite-kern op poort 8080, mount `data/v3` en bevat geen PostgreSQL-/MySQL-services. Garage zelf wordt nog niet door deze Compose-definitie gestart.

### Garage-initialisatie

Een lege Garage-node vereist initialisatie: node/layout configureren, de layout toepassen, een bucket en applicatiesleutel aanmaken en bucketrechten toekennen. Compose verzorgt containers en volumes, maar maakt die S3-configuratie niet vanzelf.

Bij implementatie hoort daarom een gedocumenteerde, herhaalbare eenmalige bootstrapstap. Deze mag bestaande Garage-data niet resetten. De app start pas documentopslag wanneer bucket en toegang beschikbaar zijn. Een interne single-node Garage is het eenvoudige uitgangspunt; persistente volumes en backups verzorgen databehoud.

## 3. SQLite als enige structured-data-store

Het [canonieke SQLite-schema](../internal/store/core_schema.sql) bevat de negen domeintabellen en wordt ingebed door `internal/store/core.go`. De eerdere PostgreSQL-notatie is vertaald zonder het logische model uit te breiden:

| Aangeleverde PostgreSQL-notatie | SQLite |
| --- | --- |
| `BIGSERIAL PRIMARY KEY` | `INTEGER PRIMARY KEY`, met automatisch gegenereerde IDs |
| `BIGINT` | `INTEGER`, met 64-bit bedragen in centen |
| `DATE` | `TEXT`, door de applicatie gevalideerd als `YYYY-MM-DD` |
| `JSONB` | `TEXT`, met `json_valid`-checks |

`object_type` blijft applicatievalidatie voor de vier toegestane types. Relatietypes blijven vrije tekst. JSON-configuratie en AI-response worden als JSON geparseerd, niet als ongevalideerde willekeurige strings gebruikt.

### Openen, concurrency en migraties

- SQLite staat op een lokaal persistent volume: Compose gebruikt `/app/data/hohoho.db`, lokaal is de default `data/v3/hohoho.db`. De volledige map wordt gemount, zodat eventuele `-wal`- en `-shm`-bestanden op dezelfde opslag staan.
- Eén Go-proces beheert het bestand; geen horizontale app-replicatie op hetzelfde bestand en geen gedeelde netwerkfilesystem-database.
- Foreign keys worden **per verbinding** aangezet en getest. Alleen de PRAGMA in een los SQL-bestand is hiervoor niet genoeg.
- Gebruik korte databasetransacties, een begrensde wachttijd voor locks en WAL-mode. Begin met één databaseverbinding, zoals de huidige SQLite-store, voor eenvoudig voorspelbaar schrijfgedrag.
- Wacht niet op S3- of PDF-AI-calls terwijl een schrijftransactie open staat.
- Eén versieerbare SQLite-migratielijn is de bron voor schema-evolutie. `PRAGMA user_version` houdt momenteel versie 1 bij zonder een tiende domeintabel toe te voegen.
- Nieuwe en bestaande databases worden door hetzelfde migratiepad voorbereid. De huidige PostgreSQL-reconciler en drie parallelle schema-implementaties vervallen na migratie.

SQLite is de bron van waarheid voor alle gestructureerde domeingegevens, inclusief documenttekst, analyse, bronregels, waarden en relaties. Browseropslag is geen primaire database of backup.

## 4. Documenten en PDF-AI

Upload:

1. Browser uploadt het bestand naar de Go-API.
2. De API genereert een objectkey zonder businessbetekenis en schrijft de originele bytes naar Garage.
3. De API slaat het documentrecord en gekozen objectkoppelingen op in SQLite.
4. De Go-service biedt het document aan de externe PDF-AI-service aan.
5. Het resultaat wordt gemapt naar `extracted_text`, `translated_text` en `analysis_json`.

Download loopt via de Go-API, die de bijbehorende objectkey uit SQLite leest en het object uit Garage streamt. De browser kent geen permanente S3-toegangssleutels.

SQLite en S3 delen geen transactie. Als opslag van metadata na upload mislukt, moet de applicatie het nieuwe ongekoppelde object opruimen of als te reconciliëren object registreren buiten het domeinmodel. Document ontkoppelen verwijdert geen bestand. Definitief verwijderen krijgt een expliciete flow voor metadata en objectbytes.

Documentanalyse blijft beperkt achtergrondwerk in dezelfde Go-service. Het negen-tabellenmodel bevat geen analyse-statuskolom of jobtabel. De implementatie moet fouten en opnieuw analyseren begrijpelijk afhandelen zonder stilzwijgend het oude workflowmodel terug te brengen.

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
| `PDF_AI_URL` | Externe documentverwerking, zonder hardcoded LAN-adres |
| `TRANSACTION_AI_URL`, `TRANSACTION_AI_MODEL` | OpenAI-compatible transactieclassificatie voor bankimport |
| `AUTH_USERNAME`, `AUTH_PASSWORD` | Bestaande productieauthenticatie behouden |

`HTTP_ADDR` en `WEB_DIR` kunnen vaste defaults in het app-image krijgen. `STORE_BACKEND`, `DATABASE_URL`, `MYSQL_*`, `OBJECT_BACKEND` en `STORAGE_PROFILE` verdwijnen als gebruikerskeuzes. De deployment gebruikt één bron voor instellingen; oude `config/storage.json`-waarden mogen nieuwe instellingen niet ongemerkt overschrijven. Secrets blijven buiten Git en buiten frontendresponses.

De UI toont alleen noodzakelijke instellingen/status voor de vaste stack. Databaseselectie, backendvalidatie en opslagprofielwissels verdwijnen. De bestaande productieauthenticatie blijft bij frontend, API en documentdownloads van toepassing.

## 6. Backup en herstel

Een complete administratie bestaat uit SQLite **en** de bijbehorende Garage-objecten. De backupflow moet beide en de benodigde deploymentconfiguratie kunnen herstellen.

- Maak een consistente SQLite-snapshot via een SQLite-backupmechanisme of `VACUUM INTO`; kopieer een live database met WAL niet zomaar als één bestand.
- Voor een eenvoudige consistente database/object-backup: pauzeer mutaties, laat lopend werk afronden, maak de snapshot en bewaar de bijbehorende objectset. Daarna kunnen writes hervatten.
- Herstel het SQLite-bestand en de Garage-objecten samen; controleer ontbrekende objectsleutels voordat de applicatie weer schrijft.
- Test backup en herstel ook na containervervanging en na schemamigraties.

- De actieve implementatie houdt muterende API-verzoeken aan tijdens een snapshot. Zij maakt een SQLite `VACUUM INTO`-snapshot en een bijbehorende S3-kopie, bewaart beide met restic en schrijft een manifest.

- Standaard draait dagelijks om 02:30 een backup; de beheerder kan tijdstip en retentie aanpassen. De standaardretentie is 14 dagelijkse, 8 wekelijkse en 24 maandelijkse snapshots.

- Elke geslaagde backup voert `restic check` uit en herstelt de nieuwe snapshot tijdelijk. De restore-proef controleert het manifest en `PRAGMA integrity_check` op SQLite voordat de snapshot als geslaagd telt.

De bestaande backup-/recovercode kan hiervoor worden versmald naar SQLite en S3. Of restic/rclone hiervoor behouden blijven wordt aan deze concrete flow getoetst; PostgreSQL/MySQL-backupbranches zijn geen onderdeel van v3.

## 7. Concrete wijzigingen en volgorde

| Huidig | V3-werk |
| --- | --- |
| `internal/store/` met drie SQL-backends | Eén SQLite-implementatie van het negen-tabellenmodel; backendselectie en serverdrivers vervangen |
| `internal/config/manager.go` | Profielregels verwijderen; huidige regels blokkeren SQLite+S3 (`local` vereist filesystem, `server`/`central` vereisen PostgreSQL/MySQL) |
| `cmd/api/main.go` | SQLite en S3 rechtstreeks openen; bestaande HTTP-, shutdown- en PDF-AI-werking behouden |
| `internal/objectstore/` | S3/Garage als enige operationele documentstore; filesystem-runtime vervangen |
| `compose.yml` | PostgreSQL/MySQL-services vervangen door app+Garage, met persistente volumes en readiness/bootstrap |
| `Dockerfile` | Klein runtime-image zonder PostgreSQL/MariaDB-tooling |
| `web/src/main.js` | Objectgerichte UI en configuratiescherm voor de vaste stack |
| `internal/systembackup` | Consistente SQLite/S3-snapshots, retentie, validatie en herstel |
| `go.mod`, tests en scripts | Niet meer benodigde drivers/dependencies verwijderen; SQLite-, S3- en end-to-endchecks behouden |

Uitvoeringsvolgorde:

1. Leg de bestaande code vast; ontwikkel de nieuwe structuur op `v3-simplification`.
2. Bouw het negen-tabellenmodel, schemaversiebeheer en nieuwe repositorycontracten op een lege SQLite-database, zonder oude seeds.
3. Bouw objecten, waarden en relaties; daarna transacties/classificatie, configureerbare bankimport en documenten/PDF-AI.
4. Versmal configuratie, startup, backup en UI; bouw de app+Garage Compose-deployment met een eigen lege HoHoHo-bucket.
5. Verifieer de volledige browserflow, import, waarden, relaties, upload/download, PDF-AI en herstel met nieuwe testgegevens.
6. Deploy met een lege administratie. Ruim vervangen PostgreSQL-/MySQL-implementaties, filesystem-runtime, profielkeuzes, dependencies en scripts op zodra de nieuwe flows werken. Verwijder oude HoHoHo-data gericht als aparte uitvoeringsstap en stem repository-instructies op v3 af.

De technische omschakeling en nieuwe domeinfuncties krijgen afzonderlijke controles. Code vastleggen of een branch aanmaken verwijdert geen databases of S3-objecten.

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

De actieve `/healthz` meet SQLite; `/readyz` controleert ook S3. Ontbrekende/onbereikbare S3 maakt readiness 503, maar blokkeert de huidige objectbasis niet. Documentupload, analyse en herstel zijn nog niet beschikbaar als v3-routes.
