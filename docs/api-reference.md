# API-overzicht

Dit document beschrijft de huidige HTTP API op hoofdlijnen. Alle paden hieronder vallen onder dezelfde backend en gebruiken JSON, behalve upload-endpoints met multipart form-data en documentcontent-downloads.

## Algemene conventies

- Basispad: `/api/v1`.
- Succesvolle JSON-responses gebruiken HTTP 200 of 201.
- Verwijderen en link/unlink-acties gebruiken vaak HTTP 204 zonder body.
- Ongeldige JSON geeft `400 invalid_json`.
- Ongeldige IDs geven `400 invalid_id` of een vergelijkbare foutcode.
- Niet gevonden geeft meestal 404 met domeinspecifieke foutcode.
- Interne fouten geven `500 internal_error`.
- Documentroutes geven `501 document_storage_unavailable` wanneer documentopslag niet actief is.

## Dashboard en samenvatting

### `GET /api/v1/summary`

Geeft financiële samenvatting voor dashboard en transacties.

### `GET /api/v1/reports/category-monthly`

Rapportage per categorie per maand.

Queryparameters volgen de transactiefilters waar relevant.

### `GET /api/v1/reports/main-category-period`

Rapportage per hoofdcategorie over een periode.

## Accounts

### `GET /api/v1/accounts`

Lijst bankrekeningen.

### `POST /api/v1/accounts`

Maakt rekening aan.

Belangrijke velden:

- `name`
- `iban`
- `accountType`
- `owner`
- `bankName`
- `bankProfile`
- `openingBalanceCents`

### `PATCH /api/v1/accounts/{id}`

Wijzigt rekeningmetadata.

### `DELETE /api/v1/accounts/{id}`

Verwijdert rekening als deze niet door transacties wordt gebruikt.

## Categorieen

### `GET /api/v1/main-categories`

Geeft hoofdcategorieen.

### `POST /api/v1/main-categories`

Maakt hoofdcategorie aan.

### `PATCH /api/v1/main-categories/{id}`

Wijzigt hoofdcategorie.

### `DELETE /api/v1/main-categories/{id}`

Verwijdert hoofdcategorie als geen subcategorieen bestaan.

### `GET /api/v1/categories`

Geeft subcategorieen.

### `POST /api/v1/categories`

Maakt subcategorie aan.

Belangrijke velden:

- `mainId`
- `name`
- `kind`
- `purpose`

### `PATCH /api/v1/categories/{id}`

Wijzigt subcategorie.

### `DELETE /api/v1/categories/{id}`

Verwijdert subcategorie als deze niet door transacties wordt gebruikt.

## Transacties

### `GET /api/v1/transactions`

Geeft transacties met filters.

Veelgebruikte queryparameters:

- `accountId`
- `categoryId`
- `mainCategoryId`
- `year`
- `month`
- `q`
- `uncategorized`
- `limit`
- `offset`

### `POST /api/v1/transactions`

Maakt handmatige transactie aan.

Belangrijke velden:

- `accountId`
- `bookedOn`
- `description`
- `counterparty`
- `counterpartyIban`
- `amountCents`
- `categoryId`

### `GET /api/v1/transactions/years`

Geeft jaren waarvoor transacties bestaan.

### `GET /api/v1/transactions/summary`

Geeft count/som voor transactiefilter.

### `PATCH /api/v1/transactions/{id}/category`

Categoriseert transactie.

Belangrijke velden:

- `categoryId`
- `applyToSimilar`

Bij `applyToSimilar` gebruikt de store party-koppeling als die bestaat, anders raw counterparty fallback.

### `PATCH /api/v1/transactions/{id}/hidden`

Verbergt transactie.

### `DELETE /api/v1/transactions/{id}/hidden`

Maakt transactie weer zichtbaar.

## Bankimport

### `GET /api/v1/imports/bank-profiles`

Geeft ondersteunde bankprofielen.

### `POST /api/v1/imports/preview`

Multipart upload voor importpreview.

Form fields:

- `profile`
- `file`

Schrijft niets naar de database.

### `POST /api/v1/imports`

Multipart upload voor definitieve import.

Form fields:

- `accountId`
- `profile`
- `file`

Slaat nieuwe transacties op en rapporteert imported/duplicate counts.

## Documenten

### `GET /api/v1/documents`

Geeft alle documentmetadata.

### `POST /api/v1/documents`

Multipart upload voor document.

Form fields:

- `file`
- `documentDate`
- `description`

Start optioneel documentanalyse wanneer LLM-documentmetadata actief is.

### `GET /api/v1/documents/{id}/content`

Downloadt originele documentinhoud.

### `GET /api/v1/transactions/{id}/documents`

Geeft documenten gekoppeld aan transactie.

### `POST /api/v1/transactions/{id}/documents`

Koppelt bestaand document aan transactie.

Body:

```json
{ "documentId": 1 }
```

### `DELETE /api/v1/transactions/{id}/documents/{documentId}`

Ontkoppelt document van transactie.

## Bezittingen

### `GET /api/v1/assets`

Geeft bezittingen.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/assets`

Maakt bezitting aan.

### `GET /api/v1/assets/{id}`

Geeft één bezitting.

### `PATCH /api/v1/assets/{id}`

Wijzigt bezitting.

### `DELETE /api/v1/assets/{id}`

Verwijdert bezitting als er geen blokkerende relaties/documentlinks bestaan.

### `GET /api/v1/assets/{id}/valuations`

Geeft waarderingen.

### `POST /api/v1/assets/{id}/valuations`

Maakt waardering aan en werkt actuele waarde bij.

### `PATCH /api/v1/assets/{id}/valuations/{valuationId}`

Wijzigt waardering.

### `DELETE /api/v1/assets/{id}/valuations/{valuationId}`

Verwijdert waardering en herberekent actuele waarde.

### `GET /api/v1/assets/{id}/transactions`

Geeft gekoppelde transacties.

### `POST /api/v1/assets/{id}/transactions`

Koppelt transactie.

### `DELETE /api/v1/assets/{id}/transactions/{transactionId}`

Ontkoppelt transactie.

### `GET /api/v1/assets/{id}/relations`

Geeft domeinrelaties.

### `POST /api/v1/assets/{id}/relations`

Maakt domeinrelatie zoals `financed_by`.

### `DELETE /api/v1/assets/{id}/relations/{relationId}`

Verwijdert domeinrelatie.

### Documentroutes

- `GET /api/v1/assets/{id}/documents`
- `POST /api/v1/assets/{id}/documents`
- `DELETE /api/v1/assets/{id}/documents/{documentId}`

### `GET /api/v1/assets/{id}/events`

Geeft tijdlijngebeurtenissen.

## Schulden

### `GET /api/v1/liabilities`

Geeft schulden.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/liabilities`

Maakt schuld aan.

### `GET /api/v1/liabilities/{id}`

Geeft één schuld.

### `PATCH /api/v1/liabilities/{id}`

Wijzigt schuld en werkt `creditor_party_id` bij wanneer mogelijk.

### `DELETE /api/v1/liabilities/{id}`

Verwijdert schuld als er geen blokkerende relaties/documentlinks bestaan.

### Relatie-, transactie-, document- en eventroutes

- `GET /api/v1/liabilities/{id}/transactions`
- `POST /api/v1/liabilities/{id}/transactions`
- `DELETE /api/v1/liabilities/{id}/transactions/{transactionId}`
- `GET /api/v1/liabilities/{id}/relations`
- `POST /api/v1/liabilities/{id}/relations`
- `DELETE /api/v1/liabilities/{id}/relations/{relationId}`
- `GET /api/v1/liabilities/{id}/documents`
- `POST /api/v1/liabilities/{id}/documents`
- `DELETE /api/v1/liabilities/{id}/documents/{documentId}`
- `GET /api/v1/liabilities/{id}/events`

## Reserveringen

### `GET /api/v1/reservations`

Geeft reserveringen.

Queryparameters:

- `type`

### `POST /api/v1/reservations`

Maakt reservering aan.

### `GET /api/v1/reservations/{id}`

Geeft één reservering.

### `PATCH /api/v1/reservations/{id}`

Wijzigt reservering.

### `DELETE /api/v1/reservations/{id}`

Verwijdert reservering als er geen blokkerende relaties/documentlinks bestaan.

### Documentroutes

- `GET /api/v1/reservations/{id}/documents`
- `POST /api/v1/reservations/{id}/documents`
- `DELETE /api/v1/reservations/{id}/documents/{documentId}`

### `GET /api/v1/reservations/{id}/events`

Geeft tijdlijngebeurtenissen.

## Dossiers

### `GET /api/v1/cases`

Geeft dossiers.

Queryparameters:

- `type`
- `status`

### `POST /api/v1/cases`

Maakt dossier aan.

### `GET /api/v1/cases/{id}`

Geeft één dossier.

### `PATCH /api/v1/cases/{id}`

Wijzigt dossier.

### `DELETE /api/v1/cases/{id}`

Verwijdert dossier als het niet beheerd is door een hoofdcategorie en geen blokkerende relaties/documentlinks heeft.

### Relatie-, transactie-, document- en eventroutes

- `GET /api/v1/cases/{id}/transactions`
- `POST /api/v1/cases/{id}/transactions`
- `DELETE /api/v1/cases/{id}/transactions/{transactionId}`
- `GET /api/v1/cases/{id}/relations`
- `POST /api/v1/cases/{id}/relations`
- `DELETE /api/v1/cases/{id}/relations/{relationId}`
- `GET /api/v1/cases/{id}/documents`
- `POST /api/v1/cases/{id}/documents`
- `DELETE /api/v1/cases/{id}/documents/{documentId}`
- `GET /api/v1/cases/{id}/events`

## Domeingebeurtenissen

Events zijn alleen-lezen via objectroutes:

- `GET /api/v1/assets/{id}/events`
- `GET /api/v1/liabilities/{id}/events`
- `GET /api/v1/cases/{id}/events`
- `GET /api/v1/reservations/{id}/events`

Er zijn geen publieke create/update/delete endpoints voor events.

## Systeemconfiguratie

### `GET /api/v1/system/configuration`

Geeft veilige configuratieweergave. Secrets worden gemaskeerd.

### `PUT /api/v1/system/configuration`

Slaat configuratiewijziging op als configuratie bewerkbaar is.

Belangrijke groepen:

- `profile`
- `database`
- `objects`
- `llm`

Wijzigingen gelden na herstart.

## Systeembackup

### `GET /api/v1/system/backup`

Geeft backupconfiguratie en runtime-status.

### `POST /api/v1/system/backup/validate`

Valideert backupconfiguratie.

### `POST /api/v1/system/backup/runs`

Start backup.

### `GET /api/v1/system/backup/snapshots`

Geeft beschikbare snapshots.

### `POST /api/v1/system/backup/recovery`

Start herstelprocedure.

## Toekomstige AI-bankimport API

Nog niet geïmplementeerd. Zie [AI bij bankimport](ai-bankimport.md) voor het voorgestelde endpointontwerp.
