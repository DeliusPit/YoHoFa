# Datamodel

Dit document beschrijft het totale datamodel van HoHoHo. Het is geschreven als logisch model, met technische verwijzingen naar de huidige relationele tabellen. PostgreSQL is de primaire Compose-store; SQLite en MySQL houden functionele parity waar de feature ondersteund wordt.

## Ontwerpprincipes

- De relationele database is de bron van waarheid voor gestructureerde data, metadata, relaties en applicatiestatus.
- Originele documentbestanden staan buiten de database in filesystem- of S3-compatible objectopslag.
- Domeinobjecten zijn first-class tabellen: transacties, documenten, bezittingen, schulden, reserveringen en dossiers.
- Koppelingen tussen domeinen worden expliciet gemodelleerd in `entity_relations`.
- Raw importdata blijft bewaard naast genormaliseerde data.
- AI-output wordt apart opgeslagen en overschrijft originele documenten of bankdata niet.
- Polymorfe relaties worden door applicatiecode gevalideerd, omdat relationele databases geen normale foreign key naar meerdere mogelijke tabellen kunnen afdwingen.

## Hoofdgroepen

Het datamodel bestaat uit deze clusters:

- Huishouden en rekeningen.
- Categorieen en rapportageclassificatie.
- Transacties en bankimport.
- Parties en identifiers.
- Documenten en analyses.
- Bezittingen en waarderingen.
- Schulden.
- Reserveringen.
- Dossiers.
- Generieke relaties.
- Domeingebeurtenissen.
- Migratiestatus en technische compatibiliteit.

## Entiteitsoverzicht

```text
households
  ├─ accounts
  │    └─ transactions ── category_id ── categories ── main_categories
  │             │
  │             └─ counterparty_party_id ── parties ── party_identifiers
  │
  ├─ main_categories ── managed case via cases.main_category_id
  ├─ assets ── asset_valuations
  ├─ liabilities ── creditor_party_id ── parties
  ├─ reservations
  ├─ cases
  ├─ documents ── document_analyses
  ├─ entity_relations
  └─ domain_events
```

## Households

Tabel: `households`

Doel:

- Logische eigenaar van de administratie.
- Basis voor toekomstige multi-household of multi-user ondersteuning.

Belangrijke velden:

- `id`: primary key.
- `name`: huishoudnaam.
- `created_at`: aanmaakmoment.

Relaties:

- Veel tabellen hebben `household_id` met default `1`.
- De huidige applicatie gebruikt functioneel één household, maar nieuwe normalisaties zoals parties worden wel per household gescoped.

## Accounts

Tabel: `accounts`

Doel:

- Bankrekening of kasbron waarop transacties worden geboekt.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `name`: gebruikersnaam van rekening.
- `iban`: optionele IBAN.
- `account_type`: bijvoorbeeld `checking` of `savings`.
- `owner`: eigenaarlabel.
- `bank_name`: banknaam.
- `bank_profile`: importprofiel voor bankexports.
- `currency`: valuta, meestal `EUR`.
- `opening_balance_cents`: openingssaldo.
- `sort_order`: weergavevolgorde.
- `created_at`: aanmaakmoment.

Relaties:

- `transactions.account_id` verwijst naar `accounts.id`.
- Import van transacties gebruikt `account_id` om counterparty parties per household te scopen.

Gedrag:

- Account verwijderen wordt geblokkeerd wanneer transacties bestaan.
- Savings accounts kunnen functioneel in het vermogensoverzicht verschijnen.

## Main Categories

Tabel: `main_categories`

Doel:

- Hoofdniveau voor financiële classificatie.

Belangrijke velden:

- `id`: primary key.
- `name`: unieke naam.
- `sort_order`: volgorde.
- `created_at`: aanmaakmoment.

Relaties:

- `categories.main_category_id` verwijst naar `main_categories.id`.
- `cases.main_category_id` kan een beheerd dossier aan de hoofdcategorie koppelen.

Gedrag:

- Bij aanmaken van een hoofdcategorie wordt ook een managed case aangemaakt.
- Verwijderen wordt geblokkeerd wanneer subcategorieen bestaan.

## Categories

Tabel: `categories`

Doel:

- Detailclassificatie van transacties.

Belangrijke velden:

- `id`: primary key.
- `main_category_id`: hoofdcategorie.
- `name`: categorienaam.
- `kind`: `income`, `expense` of `transfer`.
- `purpose`: `daily`, `reservation` of `investment`.
- `sort_order`: volgorde binnen hoofdcategorie.
- `created_at`: aanmaakmoment.

Relaties:

- `transactions.category_id` verwijst optioneel naar `categories.id`.

Gedrag:

- Categorie verwijderen wordt geblokkeerd wanneer transacties eraan gekoppeld zijn.
- `kind` en `purpose` sturen rapportage en interpretatie, maar vervangen geen domeinrelaties.

## Transactions

Tabel: `transactions`

Doel:

- Feitelijke bankmutaties, geïmporteerd of handmatig ingevoerd.

Belangrijke velden:

- `id`: primary key.
- `account_id`: rekening.
- `category_id`: optionele categorie.
- `booked_on`: boekingsdatum.
- `description`: bankomschrijving of handmatige omschrijving.
- `counterparty`: legacy/compatibele tegenpartijnaam.
- `counterparty_iban`: legacy/compatibele tegenpartij-IBAN.
- `counterparty_party_id`: genormaliseerde party.
- `counterparty_name_raw`: raw tegenpartijnaam uit import.
- `counterparty_iban_raw`: raw IBAN uit import.
- `amount_cents`: bedrag in centen; negatieve bedragen zijn uitgaven.
- `external_reference`: bankreferentie.
- `import_fingerprint`: deduplicatiesleutel.
- `hidden`: markering om transactie uit normale overzichten te houden.
- `created_at`: aanmaakmoment.

Relaties:

- Naar `accounts` via `account_id`.
- Naar `categories` via `category_id`.
- Naar `parties` via `counterparty_party_id`.
- Naar documenten en domeinobjecten via `entity_relations`.

Indexes en constraints:

- Index op rekening en boekingsdatum.
- Index op category.
- Unieke import fingerprint wanneer niet leeg.
- Index op `counterparty_party_id` voor categorisatie op dezelfde tegenpartij.

Gedrag:

- Import vult raw velden en maakt/hergebruikt parties.
- Categorisatie met `applyToSimilar` gebruikt eerst `counterparty_party_id`, daarna raw/legacy tegenpartijnaam.
- Raw bankdata blijft beschikbaar voor controle.

## Parties

Tabel: `parties`

Doel:

- Genormaliseerde personen, organisaties, banken, overheden, verzekeraars of leveranciers.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `party_type`: `person`, `organization`, `bank`, `government`, `insurer`, `supplier` of `other`.
- `name`: genormaliseerde naam.
- `notes`: vrije notities.
- `created_at`, `updated_at`: timestamps.

Relaties:

- `party_identifiers.party_id` verwijst naar party.
- `transactions.counterparty_party_id` verwijst naar party.
- `liabilities.creditor_party_id` verwijst naar party.

Gedrag:

- Parties worden per household en naam hergebruikt.
- Bij transacties kan IBAN sterker matchen dan naam.

## Party Identifiers

Tabel: `party_identifiers`

Doel:

- Externe identifiers van parties vastleggen.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `party_id`: party.
- `identifier_type`: onder andere `iban`, `bic`, `creditor_id`, `vat_id`, `tax_id`, `customer_number`, `email`, `phone`, `other`.
- `identifier_value`: originele identifier.
- `normalized_value`: genormaliseerde waarde voor matching.
- `created_at`: aanmaakmoment.

Constraints:

- Uniek op `(household_id, identifier_type, normalized_value)`.

Gedrag:

- IBAN wordt genormaliseerd door spaties te verwijderen en uppercase te gebruiken.
- Identifiers helpen bij transactiematching en toekomstige AI-context.

## Documents

Tabel: `documents`

Doel:

- Metadata voor originele documenten.

Belangrijke velden:

- `id`: primary key.
- `original_filename`: oorspronkelijke bestandsnaam.
- `mime_type`: MIME-type.
- `size_bytes`: grootte.
- `sha256`: checksum.
- `object_key`: sleutel in filesystem/S3-objectopslag.
- `document_date`: optionele documentdatum.
- `description`: omschrijving.
- `translation_text`: legacy/compatibiliteitsveld voor analyse/vertaling.
- `translation_status`: legacy/compatibiliteitsstatus.
- `created_at`: aanmaakmoment.

Relaties:

- `document_analyses.document_id` verwijst naar document.
- Documentrelaties lopen via `entity_relations` als document `evidence_for` doelobject.

Gedrag:

- Object key bevat geen businessbetekenis.
- Metadata en relaties blijven relationeel.
- Originele objecten worden niet verwijderd bij alleen ontkoppelen.

## Document Analyses

Tabel: `document_analyses`

Doel:

- Resultaten van LLM-documentanalyse of vertaling opslaan.

Belangrijke velden:

- `id`: primary key.
- `document_id`: document.
- `analysis_type`: `translation` of `analysis`.
- `status`: `not_requested`, `pending`, `completed`, `unsupported` of `failed`.
- `language`: taalcode, momenteel vooral `nl`.
- `model`: gebruikt LLM-model, optioneel.
- `content_text`: tekstuele analyse.
- `content_json`: gestructureerde analyse voor toekomstige uitbreidingen.
- `error_message`: foutmelding, indien aanwezig.
- `created_at`, `updated_at`: timestamps.

Constraints:

- Uniek op `(document_id, analysis_type)`.

Gedrag:

- Nieuwe code leest primair uit `document_analyses`.
- Compatibiliteit met oude documentvelden blijft behouden.

## Assets

Tabel: `assets`

Doel:

- Bezittingen zoals woning, voertuig of andere waardevolle objecten.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `asset_type`: type bezitting.
- `name`: naam.
- `description`: omschrijving.
- `acquired_at`: aanschafdatum.
- `disposed_at`: afvoerdatum.
- `purchase_value`: aankoopwaarde in centen.
- `current_value`: actuele waarde-cache in centen.
- `currency`: valuta.
- `status`: lifecycle-status.
- `created_at`, `updated_at`: timestamps.

Relaties:

- Naar waarderingen via `asset_valuations.asset_id`.
- Naar documenten, transacties, liabilities en cases via `entity_relations`.

Gedrag:

- `current_value` wordt bijgewerkt vanuit de nieuwste waardering.
- Asset delete wordt geblokkeerd bij relevante relaties/documentlinks.

## Asset Valuations

Tabel: `asset_valuations`

Doel:

- Historie van waarderingen voor assets.

Belangrijke velden:

- `id`: primary key.
- `asset_id`: bezitting.
- `valued_on`: waarderingsdatum.
- `value`: waarde in centen.
- `source`: bron van waardering.
- `notes`: toelichting.
- `created_at`: aanmaakmoment.

Relaties:

- Verwijst naar `assets.id`.

Gedrag:

- Mutaties aan waarderingen herberekenen `assets.current_value`.
- Waarderingen genereren domeingebeurtenissen.

## Liabilities

Tabel: `liabilities`

Doel:

- Schulden, leningen, hypotheken en andere verplichtingen.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `liability_type`: type schuld.
- `name`: naam.
- `description`: omschrijving.
- `creditor`: legacy/compatibele schuldeisernaam.
- `creditor_party_id`: genormaliseerde party.
- `start_date`: startdatum.
- `end_date`: einddatum.
- `original_amount`: oorspronkelijk bedrag in centen.
- `current_balance`: huidig saldo in centen.
- `currency`: valuta.
- `status`: `planned`, `active`, `completed` of `archived`.
- `created_at`, `updated_at`: timestamps.

Relaties:

- Naar `parties` via `creditor_party_id`.
- Naar assets, transactions, cases en documents via `entity_relations`.

Gedrag:

- Create/update maakt of hergebruikt party voor creditor.
- `creditor` blijft bestaan voor API/frontendcompatibiliteit.
- Delete wordt geblokkeerd bij relevante relaties/documentlinks.

## Reservations

Tabel: `reservations`

Doel:

- Toekomstige verplichtingen of geplande reserveringen modelleren zonder ze als actuele schuld te behandelen.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `reserve_type`: `mortgage`, `loan`, `tax_debt`, `credit` of `other`.
- `name`: naam.
- `description`: omschrijving.
- `due_date`: verwachte datum.
- `amount`: bedrag in centen.
- `currency`: valuta.
- `created_at`, `updated_at`: timestamps.

Relaties:

- Naar documenten via `entity_relations`.
- Naar events via `domain_events`.

Gedrag:

- Reserveringen staan apart van liabilities.
- Delete wordt geblokkeerd bij relevante documentlinks of relaties.

## Cases

Tabel: `cases`

Doel:

- Administratieve context of dossier.

Belangrijke velden:

- `id`: primary key.
- `household_id`: eigenaar.
- `main_category_id`: optionele koppeling voor managed category-case.
- `case_type`: vrije classificatie.
- `title`: titel.
- `description`: omschrijving.
- `start_date`: startdatum.
- `end_date`: einddatum.
- `term_months`: optionele looptijd.
- `status`: `planned`, `active`, `completed` of `archived`.
- `created_at`, `updated_at`: timestamps.

Relaties:

- Naar main category via `main_category_id`.
- Naar documenten, transacties, assets en liabilities via `entity_relations`.

Gedrag:

- Cases met `main_category_id` zijn managed en mogen niet als normale case verwijderd of vrij gewijzigd worden waar dat geblokkeerd is.
- Timelines komen uit `domain_events`.

## Entity Relations

Tabel: `entity_relations`

Doel:

- Canonieke polymorfe relaties tussen domeinobjecten.

Belangrijke velden:

- `id`: primary key.
- `source_type`: brontype.
- `source_id`: bron-ID.
- `relation_type`: semantiek.
- `target_type`: doeltype.
- `target_id`: doel-ID.
- `created_at`: aanmaakmoment.

Ondersteunde types:

- `document`
- `transaction`
- `asset`
- `liability`
- `case`
- `reservation`

Ondersteunde semantiek:

- `evidence_for`: document bewijst of hoort bij object.
- `relates_to`: administratieve relatie.
- `financed_by`: asset is gefinancierd door liability.

Canonieke richtingen:

- Document `evidence_for` case/asset/transaction/liability/reservation.
- Asset `financed_by` liability.
- Case `relates_to` transaction/asset/liability.
- Asset/liability `relates_to` transaction.

Constraints en gedrag:

- Typecombinaties worden in applicatiecode gevalideerd.
- Self-relations worden geblokkeerd.
- Duplicates worden voorkomen via unieke relationele constraints/indexes.
- Delete-operaties tellen relaties en blokkeren waar nodig.

## Domain Events

Tabel: `domain_events`

Doel:

- Gebruikersgerichte tijdlijn voor domeinobjecten.

Belangrijke velden:

- `id`: primary key.
- `entity_type`: `asset`, `liability`, `case` of `reservation`.
- `entity_id`: domeinobject-ID.
- `event_type`: gebeurtenistype.
- `event_date`: gebruikersdatum.
- `description`: tekst.
- `related_entity_type`: optioneel gerelateerd type.
- `related_entity_id`: optioneel gerelateerd ID.
- `created_at`: aanmaakmoment.

Gedrag:

- Aanmaak-events zijn uniek per object.
- Events worden door store-mutaties aangemaakt.
- Events zijn geen auditlog en geen event-sourcingbron.
- De API biedt events alleen-lezen aan.

## Legacy en Compatibiliteit

Enkele tabellen/velden bestaan bewust voor compatibiliteit of migratiehistorie.

### Document translation fields

Velden:

- `documents.translation_text`
- `documents.translation_status`

Nieuwe analyse staat in `document_analyses`, maar de velden blijven beschikbaar voor bestaande API/frontendcontracten.

### Liability creditor

Veld:

- `liabilities.creditor`

Nieuwe normalisatie gebruikt `creditor_party_id`, maar de tekstwaarde blijft beschikbaar voor compatibiliteit.

### Transaction counterparty fields

Velden:

- `transactions.counterparty`
- `transactions.counterparty_iban`

Nieuwe normalisatie gebruikt `counterparty_party_id`, `counterparty_name_raw` en `counterparty_iban_raw`, maar oude velden blijven behouden.

### document_links en object_relations

PostgreSQL-schema bevat historische relationele structuren zoals `document_links` en `object_relations`. De canonieke relationele laag is `entity_relations`. Nieuwe functionaliteit hoort `entity_relations` te gebruiken.

## Schema Migrations

Tabel: `schema_migrations`

Doel:

- Bijhouden welke PostgreSQL-migraties zijn toegepast.

Huidige logische migratielijn:

- `001_initial_schema`: huishoudens, accounts, categorieen, transacties.
- `002_documents`: documentmetadata en documentkoppelingen.
- `003_assets_and_object_relations`: assets en oudere objectrelaties.
- `004_asset_valuations`: waarderingshistorie.
- `005_entity_relations`: canonieke polymorfe relaties.
- `006_liabilities`: schulden.
- `007_cases`: dossiers.
- `008_domain_events`: tijdlijnen.
- `009_main_category_cases`: beheerde category-cases.
- `010_case_contract_fields`: contract/looptijdvelden voor cases.
- `011_reservations`: reserveringen.
- `012_document_translations`: documentanalyse-compatibiliteitsvelden.
- `013_document_analyses_parties`: documentanalyses, parties, creditor backfill.
- `014_transaction_counterparties`: transaction party/raw counterparty backfill.

## Delete- en Integriteitsregels

Belangrijke regels:

- Accounts met transacties kunnen niet verwijderd worden.
- Categories met transacties kunnen niet verwijderd worden.
- Main categories met subcategorieen kunnen niet verwijderd worden.
- Assets, liabilities, cases en reservations worden beschermd tegen verwijderen wanneer relaties of documentlinks bestaan.
- Managed cases voor hoofdcategorieen mogen niet als normale cases worden verwijderd.
- Document ontkoppelen verwijdert het documentobject niet.

## Bedragen en Valuta

Bedragen worden opgeslagen als integer centen:

- `transactions.amount_cents`
- `assets.purchase_value`
- `assets.current_value`
- `asset_valuations.value`
- `liabilities.original_amount`
- `liabilities.current_balance`
- `reservations.amount`

Valuta wordt per object bewaard waar relevant. Er is geen automatische valutaconversie.

## Datums en Timestamps

Gebruikersdatums:

- `booked_on`
- `document_date`
- `acquired_at`
- `disposed_at`
- `valued_on`
- `start_date`
- `end_date`
- `due_date`
- `event_date`

Technische timestamps:

- `created_at`
- `updated_at`

SQLite bewaart timestamps als tekst; PostgreSQL gebruikt timestamp/timestamptz; MySQL gebruikt datetime/timestamp-equivalenten. De API presenteert datumvelden als strings.

## Toekomstige AI-bankimport

Het voorgestelde AI-bankimportmodel voegt later `transaction_classification_suggestions` toe. Die tabel is nog niet geïmplementeerd en hoort suggesties gescheiden te houden van feitelijke transacties.

Zie [AI bij bankimport](ai-bankimport.md) voor het ontwerp.
