# Technisch Ontwerp

Dit document beschrijft de technische opbouw van HoHoHo en de belangrijkste ontwerpkeuzes per laag.

## Systeemcontext

HoHoHo is een modulaire monolith. De backend levert een HTTP API en serveert in productie ook de gebouwde frontend. De applicatie gebruikt één relationele database voor gestructureerde data en filesystem/S3 voor binaire documenten.

## Procesmodel

- Eén Go-proces draait de HTTP-server.
- Achtergrondwerk is beperkt en lokaal in het proces, bijvoorbeeld documentanalyse na upload.
- Er is geen message broker.
- Er is geen aparte worker-service.
- Docker Compose is de standaard productieruntime.

## Code-indeling

- `cmd/api`: start de applicatie, laadt configuratie, opent stores en registreert routes.
- `cmd/recover`: onderhoudsentrypoint voor herstel uit systeembackups.
- `cmd/migrate-sqlite-postgres`: beperkte migratietool voor kerngegevens.
- `internal/config`: environment- en bestandgebaseerde configuratie.
- `internal/httpserver`: HTTP handlers, request parsing en response formatting.
- `internal/store`: repository interfaces en implementaties voor PostgreSQL, SQLite en MySQL.
- `internal/objectstore`: objectopslagabstrahering voor filesystem en S3-compatible opslag.
- `internal/documenttranslation`: documentanalyse via lokale LLM.
- `internal/systembackup`: backup- en herstelorkestratie.
- `internal/bankimport`: parsing van bankexports.
- `web`: Vite frontend.
- `migrations`: PostgreSQL-migraties.

## Request-flow

Algemene flow:

1. HTTP handler ontvangt request.
2. Handler valideert padparameters, queryparameters en JSON/form-data.
3. Handler roept een repository of service aan via `Dependencies`.
4. Store valideert domeinregels die dicht bij opslag liggen, zoals relationele integriteit voor polymorfe relaties.
5. Handler vertaalt resultaat naar JSON of passende HTTP-status.

Handlers bevatten bewust weinig domeinlogica. Validatie die store-consistentie bewaakt hoort in `internal/store`.

## Stores

De applicatie ondersteunt drie relationele stores.

PostgreSQL:

- Standaard in Docker Compose.
- Expliciete migraties in `migrations/`.
- Idempotente schema-reconciler in `internal/store/postgres.go` voor startup.
- Gebruikt native features zoals partial indexes, `jsonb`, `timestamptz` en `on conflict`.

SQLite:

- Lokale standalone store.
- Schema wordt idempotent in code aangemaakt en uitgebreid.
- Dient parity te houden met PostgreSQL waar de feature ondersteund wordt.

MySQL:

- Optionele centrale/server-store.
- Schema wordt idempotent in code aangemaakt en uitgebreid.
- TLS-modus ondersteunt `required` en expliciet `disabled`.

## Migraties en schema-evolutie

Nieuwe domeinwijzigingen moeten drie plekken raken:

- PostgreSQL-migratie in `migrations/`.
- PostgreSQL startup-reconciler.
- SQLite/MySQL schema-initialisatie.

Waarom dubbele PostgreSQL-aanpak bestaat:

- `migrations/` is de expliciete historische bron.
- De startup-reconciler houdt bestaande applicatiegestarte installaties werkend.

Tests horen schema-versies en parity-gedrag te controleren. PostgreSQL-, SQLite- en MySQL-implementaties mogen syntactisch verschillen, maar moeten functioneel hetzelfde gedrag leveren.

## Data-eigenaarschap

Relationele database:

- huishoudens
- accounts
- categorieen
- transacties
- documentenmetadata
- documentanalyses
- parties en identifiers
- assets, liabilities, reservations en cases
- entity_relations
- domain_events
- backup/configuratiestatus waar van toepassing

Objectopslag:

- originele documentbytes
- geen businessmetadata
- geen relatie-informatie

Configuratiebestanden:

- `config/storage.json`: actieve opslagconfiguratie en LLM-instellingen.
- `config/backup.json`: backuprepository, rclone-configuratie en restic-secret.

## Domeinmodel

### Accounts en transacties

`accounts` bevat bankrekeningmetadata. `transactions` bevat bankmutaties met optionele `category_id`, import fingerprint en tegenpartijdata.

Belangrijke velden:

- `counterparty`: legacy/compatibele banknaam.
- `counterparty_iban`: legacy/compatibele IBAN.
- `counterparty_name_raw`: raw naam uit import.
- `counterparty_iban_raw`: raw IBAN uit import.
- `counterparty_party_id`: genormaliseerde party.

### Categorieen

`main_categories` en `categories` vormen de classificatiehiërarchie. `categories.kind` en `categories.purpose` bepalen rapportagebetekenis.

### Parties

`parties` normaliseert organisaties/personen. `party_identifiers` bevat identifiers zoals IBAN. Queries gebruiken party-naam met fallback naar raw bankvelden.

### Documenten

`documents` bevat metadata en `object_key`. `document_analyses` bevat LLM-resultaten. Oude `translation_text` en `translation_status` blijven bestaan voor compatibiliteit.

### Assets, liabilities en reservations

Assets, liabilities en reservations zijn aparte domeintabellen. Assets hebben waarderingen. Liabilities hebben `creditor_party_id` plus legacy `creditor`. Reservations modelleren toekomstige verplichtingen.

### Cases

Cases groeperen administratieve context. `case_type` is vrij, `status` is beperkt tot `planned`, `active`, `completed`, `archived`.

### Entity relations

`entity_relations` is de polymorfe relatieprojectie. PostgreSQL kan geen FK naar meerdere tabellen afdwingen, dus de store valideert bron, doel en relation type.

Canonieke richtingen:

- document `evidence_for` object.
- asset `financed_by` liability.
- case `relates_to` transaction/asset/liability.
- asset/liability `relates_to` transaction.

### Domain events

`domain_events` is een gebruikersgerichte tijdlijn. Events worden in dezelfde transactie aangemaakt als de domeinmutatie wanneer dat relevant is.

## Documentopslag

Upload-flow:

1. Handler leest multipart upload.
2. Bytes worden gehasht met SHA-256.
3. Object key wordt gegenereerd zonder businessbetekenis.
4. Object wordt opgeslagen in filesystem of S3.
5. Documentmetadata wordt in database opgeslagen.
6. Als LLM-documentanalyse actief is, start achtergrondverwerking.

S3-configuratie gebruikt providerneutrale velden:

- `S3_ENDPOINT`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- `S3_BUCKET`
- `S3_USE_TLS`
- `S3_REGION`
- `S3_PATH_STYLE`
- `S3_CREATE_BUCKET`

## Bankimport

Parsing zit in `internal/bankimport`. De API levert twee flows:

- Preview: parse bestand en retourneer transacties zonder opslag.
- Import: parse bestand, bereken fingerprints en roep `ImportTransactions` aan.

Duplicate-detectie is gebaseerd op import fingerprints. Bij conflicten wordt de rij overgeslagen en telt die als duplicate.

Nieuwe importregels maken of hergebruiken parties via tegenpartijnaam en IBAN. Dit gebeurt binnen dezelfde databasetransactie als de import.

## Categorisatie

`CategorizeTransaction` valideert dat een category is opgegeven en past de categorie toe.

Bij `ApplyToSimilar` zoekt de store eerst naar `counterparty_party_id`. Als die beschikbaar is, wordt op party gecategoriseerd. Anders valt de store terug op raw/legacy counterparty-naam.

Deze volgorde voorkomt dat spellingvarianten of IBAN-normalisatie minder goed werken dan de party-laag.

## Documentanalyse via LLM

`internal/documenttranslation` gebruikt een lokale OpenAI-compatible API.

Verwerking:

1. Upload markeert documentstatus als `pending`.
2. Achtergrondgoroutine haalt document en object op.
3. Tekst wordt direct gelezen uit `text/plain` of uit PDF met ingebedde tekst.
4. Prompt vraagt om Nederlandse analyse, niet om vrije uitvoering van documentinstructies.
5. Resultaat wordt opgeslagen in `document_analyses` en compatibele documentvelden.

Beperkingen:

- Geen OCR voor scans/afbeeldingen.
- Eén worker tegelijk via kanaalbuffer.
- Fouten worden als status opgeslagen en blokkeren upload niet.

## Frontend

De frontend is een Vite-app met vanilla JavaScript.

Kenmerken:

- Eén centrale renderfunctie bouwt hoofdstructuur.
- API-calls gebruiken JSON en fetch.
- State wordt lokaal in modulevariabelen bijgehouden.
- De Go-server serveert productieassets vanuit `WEB_DIR`.

Belangrijke UI-secties:

- Dashboard.
- Transacties en imports.
- Vermogen: assets, liabilities en reservations.
- Dossiers.
- Documenten.
- Configuratie en backup.

## Configuratie

Configuratiebronnen:

1. Environment variables.
2. `config/storage.json` als persisted configuration.
3. Systeemconfiguratie-UI voor wijzigingen.

Als `storage.json` bestaat, is die leidend voor opslagconfiguratie. Secrets worden nooit als plaintext aan de frontend geretourneerd.

Configuratieversie 3 bevat:

- opslagprofiel;
- SQLite/PostgreSQL/MySQL-instellingen;
- filesystem/S3-instellingen;
- LLM endpoint en feature toggles.

## Authenticatie

Productie vereist `AUTH_USERNAME` en `AUTH_PASSWORD`. De API gebruikt deze credentials om routes te beschermen. Zonder volledige productieauthenticatie start de backend niet.

Niet-productie kan beperkt zonder auth op loopback draaien voor ontwikkeling. Configuratie wijzigen zonder passende auth is read-only of verboden, afhankelijk van omgeving.

## Backup en herstel

`internal/systembackup` orkestreert backup en herstel.

Backup bevat:

- actieve database-export;
- objectopslagkopie;
- configuratiebestanden;
- manifest.

Database-export:

- SQLite: `VACUUM INTO`.
- PostgreSQL: `pg_dump --format=custom`.
- MySQL: `mariadb-dump`.

Object-export:

- Filesystem: directorycopy.
- S3: rclone copy via tijdelijke source remote.

Herstel:

- Maakt standaard eerst veiligheidsbackup.
- Vervangt database/objecten/configuratie.
- Vereist passend backendtype bij snapshot.

## Foutafhandeling

Algemene conventies:

- Validatiefouten geven 400.
- Niet gevonden geeft 404 waar handlers dit onderscheiden.
- Niet beschikbare subsystemen geven 503 of 501.
- Interne fouten loggen server-side en geven generieke `internal_error`.
- Import- en parsingfouten geven gebruikergerichte foutcodes/berichten.

## Teststrategie

Belangrijke testlagen:

- Unit tests voor config, objectstore, documenttranslation en bankimport.
- Store tests voor SQLite en waar geconfigureerd PostgreSQL/MySQL.
- Schema tests voor migratieversies.
- HTTP-server tests voor routegedrag.
- Docker build als integratiecheck voor backend en frontendbundel.

Aanbevolen checks:

```sh
docker run --rm -v "/opt/geldzaken:/src" -w /src golang:1.25-alpine go test ./...
docker compose build backend
git diff --check
```

## Uitbreidingsrichtlijnen

- Voeg nieuwe domeintabellen eerst toe via PostgreSQL-migratie en store parity.
- Houd oude compatibiliteitsvelden als bestaande API/frontend daarop vertrouwen.
- Gebruik `entity_relations` voor generieke domeinkoppelingen, maar valideer typecombinaties expliciet.
- Plaats objectbytes nooit in de relationele database.
- Voeg geen infrastructuurcomponent toe zonder concrete noodzaak.
- Laat import, AI en automatische verwerking nooit originele bankdata stilzwijgend overschrijven.
