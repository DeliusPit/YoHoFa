# API-overzicht

Dit document beschrijft de HTTP API van HoHoHo en verwijst per endpointgroep naar de fysieke Go-broncode waarin de handlers staan.

## Broncodekaart

- Router en kernroutes: `internal/httpserver/server.go`
- Documentroutes: `internal/httpserver/documents.go`
- Bezittingen: `internal/httpserver/assets.go`
- Schulden: `internal/httpserver/liabilities.go`
- Reserveringen: `internal/httpserver/reservations.go`
- Dossiers: `internal/httpserver/cases.go`
- Tijdlijnen: `internal/httpserver/domain_events.go`
- Systeemconfiguratie: `internal/httpserver/configuration.go`
- Systeembackup: `internal/httpserver/system_backup.go`
- Store-modellen en interfaces: `internal/store/*.go`
- Bankimport-parsing: `internal/bankimport`

## Algemene conventies

- Basispad voor applicatie-API: `/api/v1`.
- Healthcheck: `/healthz`.
- JSON is de standaard request/response-vorm, behalve multipart upload en documentcontent-downloads.
- Succesvolle JSON-responses gebruiken meestal `200 OK` of `201 Created`.
- Link/unlink en delete-acties gebruiken meestal `204 No Content`.
- Ongeldige JSON geeft `400` met `invalid_json`.
- Ongeldige path IDs geven `400` met `invalid_id` of een specifieker equivalent.
- Interne fouten geven `500` met `internal_error`.
- Niet beschikbare subsystemen geven `501 Not Implemented` of `503 Service Unavailable`.
- Als productieauthenticatie actief is, beschermt Basic Auth alle routes.

## Healthcheck

Broncode: `internal/httpserver/server.go`

### `GET /healthz`

Controleert de store en, wanneer vereist of beschikbaar, objectopslag.

Response:

```json
{ "status": "ok" }
```

Fouten:

- `503 { "status": "unhealthy" }`

## Dashboard en rapportage

Broncode: `internal/httpserver/server.go`

### `GET /api/v1/summary`

Geeft globale aantallen.

Responsevelden:

- `households`
- `accounts`
- `transactions`

### `GET /api/v1/reports/category-monthly`

Geeft maandtotalen voor een categorie of hoofdcategorie.

Queryparameters:

- `mainCategoryId`
- `categoryId`

Response-item:

```json
{ "month": "2026-09", "amountCents": -12345 }
```

### `GET /api/v1/reports/main-category-period`

Geeft totalen per hoofdcategorie/categorie over een periode.

Queryparameters:

- `mainCategoryId`
- `from`
- `to`

Response-item:

```json
{
  "mainCategoryId": 1,
  "mainCategoryName": "Wonen",
  "categoryId": 2,
  "categoryName": "Onderhoud",
  "amountCents": -5000
}
```

## Accounts

Broncode: `internal/httpserver/server.go`

### `GET /api/v1/accounts`

Geeft bankrekeningen.

Response-item:

```json
{
  "id": 1,
  "name": "Betaalrekening",
  "iban": "NL00BANK0123456789",
  "type": "checking",
  "owner": "Huishouden",
  "bankProfile": "triodos",
  "currency": "EUR",
  "sortOrder": 10,
  "balanceCents": 12345
}
```

### `POST /api/v1/accounts`

Maakt een rekening aan.

Request:

```json
{
  "name": "Betaalrekening",
  "iban": "NL00BANK0123456789",
  "type": "checking",
  "owner": "Huishouden",
  "bankProfile": "triodos"
}
```

### `PATCH /api/v1/accounts/{id}`

Wijzigt een rekening.

Request: zelfde vorm als `POST /api/v1/accounts`.

### `DELETE /api/v1/accounts/{id}`

Verwijdert een rekening als deze niet in gebruik is.

Fouten:

- `400` bij gebruik door transacties of ongeldig ID.

## Hoofdcategorieen

Broncode: `internal/httpserver/server.go`

### `GET /api/v1/main-categories`

Geeft hoofdcategorieen.

Response-item:

```json
{ "id": 1, "name": "Wonen", "sortOrder": 20 }
```

### `POST /api/v1/main-categories`

Maakt hoofdcategorie aan. De store maakt ook een beheerd dossier voor deze hoofdcategorie.

Request:

```json
{ "name": "Belastingen" }
```

Fouten:

- `400 name_is_required`

### `PATCH /api/v1/main-categories/{id}`

Wijzigt hoofdcategorie.

Request:

```json
{ "name": "Belastingen" }
```

### `DELETE /api/v1/main-categories/{id}`

Verwijdert hoofdcategorie als er geen subcategorieen onder hangen.

## Subcategorieen

Broncode: `internal/httpserver/server.go`

### `GET /api/v1/categories`

Geeft subcategorieen.

Response-item:

```json
{
  "id": 10,
  "mainCategoryId": 1,
  "name": "Boodschappen",
  "kind": "expense",
  "purpose": "daily",
  "sortOrder": 10
}
```

### `POST /api/v1/categories`

Maakt subcategorie aan.

Request:

```json
{
  "mainCategoryId": 1,
  "name": "Boodschappen",
  "kind": "expense",
  "purpose": "daily"
}
```

### `PATCH /api/v1/categories/{id}`

Wijzigt subcategorie.

Request: zelfde vorm als `POST /api/v1/categories`.

### `DELETE /api/v1/categories/{id}`

Verwijdert subcategorie als deze niet door transacties wordt gebruikt.

## Transacties

Broncode: `internal/httpserver/server.go`, documentkoppelingen in `internal/httpserver/documents.go`

### `GET /api/v1/transactions`

Geeft transacties.

Queryparameters:

- `accountId`
- `categoryId`
- `mainCategoryId`
- `from`
- `to`
- `q`
- `uncategorized`
- `hiddenOnly`
- `limit`
- `offset`

Response-item:

```json
{
  "id": 1,
  "accountId": 1,
  "categoryId": 10,
  "bookedOn": "2026-09-24",
  "description": "Omschrijving",
  "counterparty": "Voorbeeld BV",
  "counterpartyIban": "NL00BANK0123456789",
  "amountCents": -1234,
  "externalReference": "ref",
  "importFingerprint": "fingerprint",
  "hidden": false,
  "documentCount": 0
}
```

### `POST /api/v1/transactions`

Maakt handmatige transactie aan.

Request:

```json
{
  "accountId": 1,
  "bookedOn": "2026-09-24",
  "description": "Handmatige transactie",
  "counterparty": "Voorbeeld BV",
  "counterpartyIban": "NL00BANK0123456789",
  "amountCents": -1234,
  "categoryId": 10
}
```

Fouten:

- `501 manual_transactions_unavailable` als de store dit niet ondersteunt.

### `GET /api/v1/transactions/years`

Geeft beschikbare transactiejaren als array van getallen.

### `GET /api/v1/transactions/summary`

Geeft aantal en bedrag voor dezelfde filters als `GET /api/v1/transactions`.

Response:

```json
{ "count": 12, "amountCents": -123456 }
```

### `PATCH /api/v1/transactions/{id}/category`

Categoriseert transactie.

Request:

```json
{ "categoryId": 10, "applyToSameCounterparty": true }
```

Response:

```json
{ "updated": 3 }
```

### `PATCH /api/v1/transactions/{id}/hidden`

Verbergt transactie.

Response:

```json
{ "status": "hidden" }
```

### `DELETE /api/v1/transactions/{id}/hidden`

Maakt transactie weer zichtbaar.

Response:

```json
{ "status": "visible" }
```

### `GET /api/v1/transactions/{id}/documents`

Geeft documenten gekoppeld aan transactie.

Broncode: `internal/httpserver/documents.go`

### `POST /api/v1/transactions/{id}/documents`

Koppelt bestaand document aan transactie.

Request:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/transactions/{id}/documents/{documentId}`

Ontkoppelt document van transactie.

## Bankimport

Broncode: routes in `internal/httpserver/server.go`, parsing in `internal/bankimport`

### `GET /api/v1/imports/bank-profiles`

Geeft beschikbare importprofielen.

Response:

```json
{ "profiles": ["generic", "ing", "rabobank"] }
```

### `POST /api/v1/imports/preview`

Multipart upload voor preview. Schrijft niets naar de database.

Form fields:

- `profile`: optioneel, default `generic`.
- `file`: verplicht.

Fouten:

- `400 invalid_multipart_form`
- `400 missing_file`
- `400 <parser error>`

### `POST /api/v1/imports`

Multipart upload voor definitieve import. Het account bepaalt het bankprofiel; het `profile` form field wordt hier niet gebruikt.

Form fields:

- `accountId`: verplicht.
- `file`: verplicht.

Response:

```json
{
  "profile": "triodos",
  "account": {},
  "result": { "imported": 10, "duplicates": 2, "total": 12 },
  "warnings": []
}
```

## Documenten

Broncode: `internal/httpserver/documents.go`

### `GET /api/v1/documents`

Geeft documentmetadata.

Response-item bevat onder andere:

- `id`
- `originalFilename`
- `mimeType`
- `sizeBytes`
- `sha256`
- `documentDate`
- `description`
- `translationText`
- `translationStatus`
- `createdAt`

### `POST /api/v1/documents`

Multipart upload voor document. Maximale documentgrootte is 25 MiB.

Form fields:

- `file`: verplicht.
- `documentDate`: optioneel, formaat `YYYY-MM-DD`.
- `description`: optioneel.

Gedrag:

- Schrijft objectbytes naar filesystem/S3.
- Schrijft metadata naar relationele store.
- Start documentanalyse als `LLMDocumentMetadata` aan staat en de service beschikbaar is.

Fouten:

- `400 invalid_or_too_large_upload`
- `400 missing_file`
- `400 invalid_document_date`
- `502 object_storage_failed`
- `500 metadata_storage_failed`

### `GET /api/v1/documents/{id}/content`

Streamt originele documentinhoud.

Headers:

- `Content-Type`
- `Content-Length`
- `Content-Disposition`
- `Content-Security-Policy: sandbox; default-src 'none'`
- `X-Content-Type-Options: nosniff`

Inline content-types:

- `application/pdf`
- `image/gif`
- `image/jpeg`
- `image/png`
- `image/webp`

## Bezittingen

Broncode: `internal/httpserver/assets.go`, events in `internal/httpserver/domain_events.go`

### `GET /api/v1/assets`

Geeft bezittingen.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/assets`

Maakt bezitting aan.

Requestvelden:

- `assetType`
- `name`
- `description`
- `acquiredAt`
- `disposedAt`
- `purchaseValueCents`
- `currency`
- `status`

### `GET /api/v1/assets/{id}`

Geeft één bezitting.

### `PATCH /api/v1/assets/{id}`

Wijzigt bezitting.

### `DELETE /api/v1/assets/{id}`

Verwijdert bezitting als relaties en documentlinks dit toelaten.

### `GET /api/v1/assets/{id}/valuations`

Geeft waarderingen.

### `POST /api/v1/assets/{id}/valuations`

Maakt waardering aan.

Requestvelden:

- `valuedOn`
- `valueCents`
- `source`
- `notes`

### `PATCH /api/v1/assets/{id}/valuations/{valuationId}`

Wijzigt waardering.

### `DELETE /api/v1/assets/{id}/valuations/{valuationId}`

Verwijdert waardering.

### `GET /api/v1/assets/{id}/transactions`

Geeft gekoppelde transacties.

### `POST /api/v1/assets/{id}/transactions`

Koppelt transactie.

Request:

```json
{ "transactionId": 1 }
```

### `DELETE /api/v1/assets/{id}/transactions/{transactionId}`

Ontkoppelt transactie.

### `GET /api/v1/assets/{id}/relations`

Geeft generieke relaties.

### `POST /api/v1/assets/{id}/relations`

Maakt generieke relatie.

Request:

```json
{
  "targetType": "liability",
  "targetId": 1,
  "relationType": "financed_by",
  "validFrom": "2026-01-01",
  "validTo": "",
  "notes": ""
}
```

### `DELETE /api/v1/assets/{id}/relations/{relationId}`

Verwijdert relatie wanneer die bij deze asset hoort.

### `GET /api/v1/assets/{id}/documents`

Geeft gekoppelde documenten.

### `POST /api/v1/assets/{id}/documents`

Koppelt bestaand document.

Request:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/assets/{id}/documents/{documentId}`

Ontkoppelt document.

### `GET /api/v1/assets/{id}/events`

Geeft tijdlijngebeurtenissen.

## Schulden

Broncode: `internal/httpserver/liabilities.go`, events in `internal/httpserver/domain_events.go`

### `GET /api/v1/liabilities`

Geeft schulden.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/liabilities`

Maakt schuld aan.

Requestvelden:

- `liabilityType`
- `name`
- `description`
- `creditor`
- `startDate`
- `endDate`
- `originalAmountCents`
- `currentBalanceCents`
- `currency`
- `status`

### `GET /api/v1/liabilities/{id}`

Geeft één schuld.

### `PATCH /api/v1/liabilities/{id}`

Wijzigt schuld.

### `DELETE /api/v1/liabilities/{id}`

Verwijdert schuld als relaties en documentlinks dit toelaten.

### `GET /api/v1/liabilities/{id}/transactions`

Geeft gekoppelde transacties.

### `POST /api/v1/liabilities/{id}/transactions`

Koppelt transactie.

Request:

```json
{ "transactionId": 1 }
```

### `DELETE /api/v1/liabilities/{id}/transactions/{transactionId}`

Ontkoppelt transactie.

### `GET /api/v1/liabilities/{id}/relations`

Geeft generieke relaties.

### `POST /api/v1/liabilities/{id}/relations`

Maakt generieke relatie.

Requestvelden: `targetType`, `targetId`, `relationType`, `validFrom`, `validTo`, `notes`.

### `DELETE /api/v1/liabilities/{id}/relations/{relationId}`

Verwijdert relatie.

### `GET /api/v1/liabilities/{id}/documents`

Geeft gekoppelde documenten.

### `POST /api/v1/liabilities/{id}/documents`

Koppelt bestaand document.

Request:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/liabilities/{id}/documents/{documentId}`

Ontkoppelt document.

### `GET /api/v1/liabilities/{id}/events`

Geeft tijdlijngebeurtenissen.

## Reserveringen

Broncode: `internal/httpserver/reservations.go`, events in `internal/httpserver/domain_events.go`

### `GET /api/v1/reservations`

Geeft reserveringen.

Queryparameters:

- `type`

### `POST /api/v1/reservations`

Maakt reservering aan.

Requestvelden:

- `reserveType`
- `name`
- `description`
- `dueDate`
- `amountCents`
- `currency`

### `GET /api/v1/reservations/{id}`

Geeft één reservering.

### `PATCH /api/v1/reservations/{id}`

Wijzigt reservering.

### `DELETE /api/v1/reservations/{id}`

Verwijdert reservering als relaties en documentlinks dit toelaten.

### `GET /api/v1/reservations/{id}/documents`

Geeft gekoppelde documenten.

### `POST /api/v1/reservations/{id}/documents`

Koppelt bestaand document.

Request:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/reservations/{id}/documents/{documentId}`

Ontkoppelt document.

### `GET /api/v1/reservations/{id}/events`

Geeft tijdlijngebeurtenissen.

## Dossiers

Broncode: `internal/httpserver/cases.go`, events in `internal/httpserver/domain_events.go`

### `GET /api/v1/cases`

Geeft dossiers.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/cases`

Maakt dossier aan.

Requestvelden:

- `caseType`
- `title`
- `description`
- `startDate`
- `endDate`
- `termMonths`
- `status`

### `GET /api/v1/cases/{id}`

Geeft één dossier.

### `PATCH /api/v1/cases/{id}`

Wijzigt dossier.

### `DELETE /api/v1/cases/{id}`

Verwijdert dossier als het niet door een hoofdcategorie beheerd wordt en relaties/documentlinks dit toelaten.

### `GET /api/v1/cases/{id}/transactions`

Geeft gekoppelde transacties.

### `POST /api/v1/cases/{id}/transactions`

Koppelt transactie.

Request:

```json
{ "transactionId": 1 }
```

### `DELETE /api/v1/cases/{id}/transactions/{transactionId}`

Ontkoppelt transactie.

### `GET /api/v1/cases/{id}/relations`

Geeft generieke relaties.

### `POST /api/v1/cases/{id}/relations`

Maakt generieke relatie.

Requestvelden: `targetType`, `targetId`, `relationType`, `validFrom`, `validTo`, `notes`.

### `DELETE /api/v1/cases/{id}/relations/{relationId}`

Verwijdert relatie.

### `GET /api/v1/cases/{id}/documents`

Geeft gekoppelde documenten.

### `POST /api/v1/cases/{id}/documents`

Koppelt bestaand document.

Request:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/cases/{id}/documents/{documentId}`

Ontkoppelt document.

### `GET /api/v1/cases/{id}/events`

Geeft tijdlijngebeurtenissen.

## Domeingebeurtenissen

Broncode: `internal/httpserver/domain_events.go`

Events zijn alleen-lezen en worden via objectroutes ontsloten:

- `GET /api/v1/assets/{id}/events`
- `GET /api/v1/liabilities/{id}/events`
- `GET /api/v1/reservations/{id}/events`
- `GET /api/v1/cases/{id}/events`

Als eventrepository niet beschikbaar is:

- `501 domain_events_require_postgres`

## Relaties

Broncode: relation endpoints in `internal/httpserver/assets.go`, `internal/httpserver/liabilities.go`, `internal/httpserver/cases.go`; validatie en opslag in `internal/store/object_relations.go`, `internal/store/sqlite_relations.go`, `internal/store/mysql_relations.go`.

Relaties worden niet via een los top-level endpoint beheerd. Ze hangen aan assets, liabilities en cases.

Requestvelden voor relatiecreatie:

- `targetType`
- `targetId`
- `relationType`
- `validFrom`
- `validTo`
- `notes`

Belangrijke relation types:

- `evidence_for`
- `relates_to`
- `financed_by`

## Systeemconfiguratie

Broncode: `internal/httpserver/configuration.go`; configuratiemodel in `internal/config`.

### `GET /api/v1/system/configuration`

Geeft veilige configuratieweergave. Secrets worden niet teruggegeven; alleen statusvelden zoals `passwordSet`, `accessKeySet` en `secretKeySet`.

Fouten:

- `503 configuration_unavailable`

### `PUT /api/v1/system/configuration`

Slaat configuratiewijziging op als configuratie bewerkbaar is.

Requestgroepen:

- `profile`
- `database`
- `objects`
- `llm`

Fouten:

- `400 invalid_json`
- `400 invalid_configuration`
- `403 configuration_read_only`
- `503 configuration_unavailable`

Wijzigingen gelden na herstart.

## Systeembackup

Broncode: `internal/httpserver/system_backup.go`; service in `internal/systembackup`.

### `GET /api/v1/system/backup`

Geeft backupconfiguratie en runtime-status.

Fouten:

- `503 backup_unavailable`

### `PUT /api/v1/system/backup`

Slaat backupinstellingen op en valideert ze direct.

Requestvelden:

- `repository`
- `rcloneConfig`
- `resticPassword`

Fouten:

- `400 invalid_json`
- `400 <backup error code>`
- `409 <backup error code>` bij actieve backup/herstelactie.
- `502 <backup error code>` bij externe storage/restic/rclone-fout.

### `POST /api/v1/system/backup/validate`

Valideert opgeslagen backupinstellingen opnieuw.

### `POST /api/v1/system/backup/runs`

Start een backup.

Responses:

- `202 Accepted` met actuele backupview.

Fouten:

- `412 backup_not_ready`
- `409 backup_running`

### `GET /api/v1/system/backup/snapshots`

Geeft beschikbare snapshots.

Fouten:

- `409` bij actieve taak.
- `412` bij ongeldige of ontbrekende configuratie.
- `502` bij repositoryfout.

### `POST /api/v1/system/backup/recovery`

Plant herstel vanuit snapshot.

Requestvelden:

- `snapshotId`
- `confirmation`
- optionele herstelparameters zoals door `internal/systembackup` gevalideerd.

Responses:

- `202 Accepted` met actuele backupview.

Fouten:

- `400` bij ongeldige request of ontbrekende bevestiging.
- `404` bij onbekende snapshot.
- `409` bij actieve taak.
- `502` bij externe fout.

## Niet-beschikbare features

Broncode: fallbackregistratie in `internal/httpserver/server.go`

Wanneer dependencies bij startup ontbreken, registreert de server expliciete fallbackroutes:

- Documentroutes: `501 documents_require_relational_store_and_object_storage`
- Assetroutes: `501 assets_require_postgres`
- Liabilityroutes: `501 liabilities_require_postgres`
- Reservationroutes: `501 reservations_require_postgres`
- Caseroutes: `501 cases_require_postgres`
- Eventroutes: `501 domain_events_require_postgres`

De foutnamen zijn historisch en betekenen praktisch: de actieve store/dependency ondersteunt deze feature niet.

## Toekomstige AI-bankimport API

Nog niet geïmplementeerd. Zie [AI bij bankimport](ai-bankimport.md) voor het voorgestelde endpointontwerp.
