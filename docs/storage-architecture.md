# Opslagarchitectuur

> **Historische v2-opslagarchitectuur.** V3 gebruikt één vaste opslagcombinatie: SQLite voor gestructureerde data en Garage/S3 voor documenten. Zie [Technische architectuur v3](technical-architecture-v3.md). Onderstaande opslagprofielen worden niet meer door de actieve runtime gebruikt.

Deze applicatie ondersteunt drie opslagprofielen:

- `local`: SQLite met filesystem-objectopslag voor standalone gebruik.
- `server`: PostgreSQL of MySQL met filesystem- of S3-compatible objectopslag.
- `central`: PostgreSQL of MySQL met filesystem- of S3-compatible objectopslag voor gedeeld webgebruik.

## Schema's en migraties

PostgreSQL heeft twee elkaar aanvullende mechanismen:

- `migrations/*.sql` zijn expliciete, uitvoerbare upgrades voor beheerde PostgreSQL-installaties.
- `internal/store/postgres.go` bevat de idempotente startup-reconciler (`postgresSchema`). Die brengt applicatiegestarte installaties naar hetzelfde schema en registreert dezelfde migratieversies.

SQLite en MySQL hebben eigen idempotente schema-initialisatie in code. Elke domeinwijziging moet daarom in alle actieve relationele stores worden doorgevoerd en met parity-tests worden gecontroleerd.

## Parity-controle

Gebruik `scripts/check-storage-parity.sh` om de Go-tests voor SQLite, optionele PostgreSQL en optionele MySQL te draaien. PostgreSQL- en MySQL-tests worden alleen geactiveerd wanneer respectievelijk `TEST_DATABASE_URL` en `TEST_MYSQL_DSN` zijn gezet.

De belangrijkste parity-gebieden zijn:

- accounts, categorieen, transacties en rapporten;
- documentenmetadata en documentrelaties;
- assets, liabilities en cases;
- `entity_relations`, `domain_events` en reservations;
- importdeduplicatie en handmatige transacties.

## Objectopslag

Documentmetadata staat altijd in de relationele database. Originele documentobjecten staan in filesystem-opslag of S3-compatible opslag.

Voor S3-compatible opslag gebruikt de applicatie uitsluitend de provider-neutrale `S3_*`-instellingen. Garage hoort met path-style lookup te draaien en meestal met vooraf aangemaakte buckets:

```text
S3_ENDPOINT=garage.example.nl:3900
S3_REGION=garage
S3_PATH_STYLE=true
S3_CREATE_BUCKET=false
```

Gebruik `scripts/check-garage-s3.sh` om een actieve Garage/S3-configuratie tegen de applicatieclient te controleren.

## Migratiepaden

Configuratiewijzigingen migreren geen data. Ze wijzigen alleen welke backend bij een volgende start actief is.

Ondersteund of gepland:

- SQLite naar PostgreSQL: `cmd/migrate-sqlite-postgres` migreert alleen kerngegevens en weigert een bron met documenten of andere domeingegevens, zodat geen data stilzwijgend verloren gaat.
- SQLite naar MySQL: nog niet geimplementeerd.
- PostgreSQL naar MySQL of MySQL naar PostgreSQL: nog niet geimplementeerd.
- Filesystem naar S3/Garage of S3/Garage naar filesystem: nog niet geimplementeerd.

Een toekomstige migratietool moet databasegegevens en objecten samen migreren, met checksums en behoud van documentmetadata.

## Standalone

Standalone distributies horen standaard `local` te gebruiken: SQLite met filesystem-objectopslag. De filesystem-objectstore heeft een Unix-implementatie met extra symlinkbescherming en een Windows-compatible implementatie zodat Windows-builds niet meer door Unix-only imports worden geblokkeerd.
