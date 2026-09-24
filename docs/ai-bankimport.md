# AI bij bankimport

Dit document beschrijft een gefaseerd ontwerp voor AI-ondersteuning bij het inlezen en verwerken van banktransacties. Het uitgangspunt is dat de bestaande import betrouwbaar blijft: AI mag helpen met voorstellen, normalisatie en uitleg, maar mag in de eerste fases geen onzichtbare boekingen of categorieen aanpassen.

## Doelen

- Nieuwe transacties sneller categoriseren tijdens of direct na import.
- Tegenpartijen beter normaliseren naar bestaande `parties`.
- Omschrijvingen samenvatten zonder originele bankdata te overschrijven.
- Gebruikers reviewbare voorstellen tonen met confidence en reden.
- Later veilige automatische toepassing toestaan voor duidelijke gevallen.

## Niet-doelen voor de eerste fase

- Geen PSD2/Open Banking-koppeling.
- Geen externe cloud-LLM als standaardpad.
- Geen automatische categoriewijziging zonder expliciete review of hoge confidence-instelling.
- Geen vervanging van bestaande duplicate-detectie of import-preview.
- Geen verwijdering van ruwe bankvelden zoals omschrijving, tegenpartij en IBAN.

## Bestaande basis

- Bankimport ondersteunt preview en definitieve import via profielen in `internal/bankimport` en store-methodes zoals `ImportTransactions`.
- Transacties hebben `category_id`, `counterparty`, `counterparty_iban`, raw counterparty-velden en `counterparty_party_id`.
- Tegenpartijen worden al genormaliseerd naar `parties` en `party_identifiers`.
- Categoriseren kan handmatig via `CategorizeTransaction`, inclusief toepassen op dezelfde tegenpartij.
- Lokale LLM-configuratie bestaat in de systeemconfiguratie.
- Documentanalyse gebruikt al een lokale OpenAI-compatible `/v1` API en kan als implementatiepatroon dienen.

## Ontwerpprincipe

AI-output is een suggestie, geen bron van waarheid.

De transaction record blijft de feitelijke banktransactie. AI-resultaten worden apart opgeslagen als herleidbare voorstellen met inputcontext, modelinformatie, confidence, status en eventuele foutmelding. Toepassen van een voorstel is een afzonderlijke handeling die normale validatie gebruikt.

## Fase 1: Suggesties na import

De eerste implementatie draait na definitieve import en verwerkt alleen nieuw ingevoegde transacties zonder categorie.

### Functioneel gedrag

- Import-preview blijft ongewijzigd.
- Definitieve import slaat transacties op zoals nu.
- Als `LLM_TRANSACTION_CLASSIFY=true` en een LLM-endpoint is ingesteld, start een achtergrondtaak.
- Per nieuwe transactie wordt een suggestie gemaakt voor:
  - `categoryId`
  - genormaliseerde tegenpartijnaam
  - optionele `partyId` als bestaande party duidelijk matcht
  - korte Nederlandse reden
  - confidence tussen `0` en `1`
  - optionele korte omschrijving/samenvatting
- De bestaande transactie wordt niet direct aangepast.

### Waarom na import en niet in preview

- Preview moet snel en deterministisch blijven.
- Suggesties kunnen aan echte transaction IDs gekoppeld worden.
- Fouten in AI-verwerking blokkeren de import niet.
- Retry en reviewstatus zijn eenvoudiger te beheren.

## Fase 2: Review UI

Gebruikers krijgen in de transactielijst of een aparte reviewweergave AI-voorstellen te zien.

### UI-acties

- Voorstel toepassen op deze transactie.
- Voorstel negeren.
- Voorstel aanpassen en toepassen.
- Categorie toepassen op alle ongecategoriseerde transacties van dezelfde party.

### UI-informatie

- Voorgestelde categorie met hoofdgroep.
- Confidence-label, bijvoorbeeld laag, middel, hoog.
- Reden in gewone taal.
- Waarschuwing als de voorgestelde categorie niet meer bestaat.
- Originele bankomschrijving blijft zichtbaar.

## Fase 3: Leren van correcties

Na review kan de app betere context meegeven aan toekomstige prompts.

### Brondata voor context

- Bestaande categorieen en hoofdgroepen.
- Eerder handmatig gecategoriseerde transacties van dezelfde party.
- Eerder geaccepteerde AI-suggesties.
- Party identifiers zoals IBAN.

### Bewust niet doen

- Geen model finetuning in de app.
- Geen verborgen zelflerende wijzigingen aan categorieen.
- Geen automatische category creation door AI.

## Fase 4: Voorzichtige automatische toepassing

Automatisch toepassen wordt pas toegevoegd als suggesties reviewbaar en betrouwbaar zijn.

### Mogelijke regels

- Alleen als de gebruiker dit expliciet aanzet.
- Alleen boven een instelbare confidence, bijvoorbeeld `0.90`.
- Alleen als dezelfde party eerder meermaals bevestigd is voor dezelfde categorie.
- Alleen voor bestaande categorieen.
- Altijd met auditbare status: automatisch toegepast door AI-suggestie.

## Data-model voorstel

Nieuwe tabel `transaction_classification_suggestions`.

```sql
id bigserial primary key
transaction_id bigint not null references transactions(id) on delete cascade
status text not null -- pending, completed, failed, accepted, rejected, superseded
provider text not null default 'local_openai_compatible'
model text
suggested_category_id bigint references categories(id) on delete set null
suggested_party_id bigint references parties(id) on delete set null
suggested_party_name text
suggested_description text
confidence numeric(4,3)
reason text
input_hash text not null
error_message text
created_at timestamptz not null default now()
updated_at timestamptz not null default now()
unique (transaction_id, input_hash)
```

Voor SQLite en MySQL komt een equivalent schema in de store-initialisatie, zodat gedrag consistent blijft.

## Repository interfaces

Minimale uitbreiding in `internal/store`:

- `CreateTransactionClassificationSuggestion`
- `TransactionClassificationSuggestions(filter)`
- `UpdateTransactionClassificationSuggestionStatus`
- `AcceptTransactionClassificationSuggestion`

Acceptatie moet dezelfde validatie gebruiken als bestaande categorisatie en mag niet rechtstreeks om businessregels heen schrijven.

## Service ontwerp

Nieuwe package: `internal/transactionclassification`.

### Verantwoordelijkheden

- Transactiecontext ophalen.
- Categorieen en relevante historische voorbeelden ophalen.
- Prompt bouwen.
- OpenAI-compatible LLM aanroepen.
- JSON-response strikt valideren.
- Suggestie opslaan.

### Niet verantwoordelijk

- Bankbestanden parsen.
- Transacties importeren.
- Categorieen muteren.
- Suggestions toepassen zonder store-validatie.

## Promptstrategie

De prompt moet klein, strikt en Nederlandstalig zijn.

Input bevat:

- datum
- bedrag
- omschrijving
- tegenpartijnaam
- IBAN, indien aanwezig
- lijst met toegestane categorieen inclusief IDs
- enkele voorbeelden van dezelfde party of vergelijkbare omschrijvingen

Output moet JSON zijn:

```json
{
  "categoryId": 123,
  "partyName": "Voorbeeld BV",
  "confidence": 0.82,
  "reason": "Terugkerende betaling aan dezelfde leverancier, eerder als energie geboekt.",
  "description": "Maandelijkse energierekening"
}
```

Validatie:

- `categoryId` moet bestaan in de meegegeven lijst.
- `confidence` moet tussen `0` en `1` liggen.
- Vrije tekstvelden worden getrimd en begrensd.
- Ongeldige output wordt als `failed` opgeslagen, niet toegepast.

## Privacy en veiligheid

- Alleen lokaal geconfigureerde LLM gebruiken in eerste implementatie.
- Geen secrets loggen.
- Geen volledige rekeninghistorie meesturen; alleen compacte context.
- IBAN meesturen is optioneel te maken als privacy-instelling.
- Prompt-injection in bankomschrijvingen expliciet neutraliseren: banktekst is data, geen instructie.
- LLM-fouten mogen import niet blokkeren.

## API voorstel

Nieuwe endpoints:

- `GET /api/v1/transaction-suggestions?status=pending`
- `POST /api/v1/transactions/{id}/suggest-classification`
- `POST /api/v1/transaction-suggestions/{id}/accept`
- `POST /api/v1/transaction-suggestions/{id}/reject`

Later mogelijk:

- `POST /api/v1/imports/{id}/suggest-classifications` als imports een eigen batch-id krijgen.

## Implementatiestappen

1. Schema toevoegen voor `transaction_classification_suggestions` in PostgreSQL-migratie en SQLite/MySQL initialisatie.
2. Store-interface en tests toevoegen voor suggesties en accept/reject-flow.
3. LLM-service `internal/transactionclassification` toevoegen met JSON-validatie en unit tests op parsing/validatie.
4. Importpad uitbreiden zodat nieuw ingevoegde transaction IDs optioneel naar de service gaan.
5. API endpoints toevoegen voor losse suggestie, lijst, accepteren en negeren.
6. Frontend review UI toevoegen in transactiescherm.
7. Context verbeteren met historische voorbeelden per party.
8. Optionele automatische toepassing toevoegen achter expliciete instelling en confidence-threshold.

## Eerste kleine PR/commit

De kleinste veilige eerste stap is alleen backend-fundering:

- migratie `015_transaction_classification_suggestions.sql`;
- store-modellen en repository-methodes;
- tests voor PostgreSQL schema en SQLite gedrag;
- nog geen LLM-aanroep en nog geen UI.

Daarna kan de LLM-service in een tweede stap worden toegevoegd zonder het data-model tegelijk te veranderen.

## Open keuzes

- Suggesties direct per transactie draaien of eerst per party clusteren.
- IBAN wel of niet standaard meesturen naar de lokale LLM.
- Confidence-threshold voor latere automatische toepassing.
- Of accepted/rejected suggesties permanent bewaard blijven of later opgeschoond worden.
- Of importbatches expliciet gemodelleerd moeten worden om suggesties per upload te groeperen.
