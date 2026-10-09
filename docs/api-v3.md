# API v3 — objecten, transacties, documenten en import

De actieve API van `cmd/api` gebruikt uitsluitend de SQLite-kern met Garage/S3 voor documentbytes. Objectbeheer, waarden, relaties, samenvatting, handmatige transacties met objecttoewijzingen, CSV-bankimport, documenten en systeembackup zijn beschikbaar.

Alle bedragen zijn integer centen. De browserinterface gebruikt EUR als huidige weergaveconventie. Objecttypes zijn exact `asset`, `debt`, `expense`, `reserve`, `financial`. Relatietypes zijn vrije niet-lege tekst.

## Beschikbaarheid en authenticatie

- `GET /healthz`: SQLite bereikbaar → 200, anders 503.
- `GET /readyz`: SQLite én S3-bucket bereikbaar → 200, anders 503. Zonder S3-configuratie retourneert dit endpoint 503; de basis-API blijft bruikbaar.
- `GET /api/v1/summary`: schemaVersion, aantallen objecten, waarden, relaties, transacties, documenten en importprofielen.

De bestaande HTTP Basic-authenticatie beschermt zowel frontend als alle API-/healthroutes wanneer credentials zijn ingesteld. In productie zijn beide credentials verplicht.

## Dashboard en periodes

`GET /api/v1/dashboard?year=2026&month=10` levert beide dashboardpanelen vanuit één consistente SQLite-leestransactie. Weglaten van `year` kiest het huidige jaar; weglaten van `month` of `month=0` kiest het hele jaar. Geldige jaren zijn 1–9999, maanden 0–12; ongeldige invoer geeft 400.

De response bevat:

- `year`, `month`, `from`, `to`: gekozen jaar/maand en inclusieve kalendergrenzen.
- `availableYears`: jaren uit transacties en waarderingen, plus het huidige jaar.
- `objectGroups`: per asset/debt/reserve/expense/financial het aantal huidige objecten, `valuedCount`, de som van bekende `valueCents` en `periodAmountCents` (expliciet toegewezen mutaties in de periode).
- `netWorthCents`: bekende bezitwaarden minus bekende schuldsaldi. Reserveringen en lasten worden niet afgetrokken als actuele schuld.
- `transactions`: aantal posten, aantallen inkomsten/uitgaven, `incomeCents`, negatief `expenseCents`, `balanceCents` en `unallocatedCount` (nog niet volledig toegewezen transacties).

Waarderingen gebruiken de laatste `value_date <= to`, daarna hoogste ID bij gelijke datum. Oudere waarden blijven geldig tot een volgende waardering. Waarden na de peildatum tellen niet mee. Een object zonder waardering is herkenbaar via `valuedCount`; het wordt niet als bekend nulbedrag gepresenteerd. Het model bevat geen aanmaak-/sluitdatum van objectidentiteiten, dus objectaantallen beschrijven de huidige objecten, niet een historische inventaris.

Transactiebedragen worden uitsluitend binnen `from` t/m `to` opgeteld, één keer per transactie. Splitsingen en contextlinks veroorzaken geen dubbeltelling. `periodAmountCents` gebruikt alleen expliciete bedragen op koppelingen en staat los van waardesnapshots. Er worden geen tabel, waarde-cache of bankimportflow toegevoegd.

De browser toont jaar- en maandknoppen boven beide panelen. Een andere jaarselectie kiest eerst het hele jaar; klikken op een geselecteerde maand kiest weer het hele jaar. Het dashboard houdt een eigen periode bij en verandert geen ingevoerde gegevens of de filters van het transactiebeheer.

`GET /api/v1/budget?year=2026&month=10` gebruikt dezelfde periode-invoer en levert alleen financiële objecten met gekoppelde, zichtbare transacties in die periode. Per object bevat de response alle bijbehorende posten en de expliciet toegewezen bedragen. Bij een last is `monthlyMaximumCents` de laatste objectwaarde op of voor het einde van de periode, als maandelijks maximum. De browser vergelijkt dit met de werkelijke uitgaven; voor een heel jaar wordt het maandmaximum met twaalf vermenigvuldigd.

## Financiële objecten

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/financial-objects` | Alle objecten; optioneel `?type=asset` of een van de andere objecttypes |
| `POST /api/v1/financial-objects` | Object aanmaken; 201 |
| `GET /api/v1/financial-objects/{id}` | Eén object; 200 |
| `PUT /api/v1/financial-objects/{id}` | Type, naam en omschrijving vervangen; 200 |
| `DELETE /api/v1/financial-objects/{id}` | Niet-gekoppeld object verwijderen; 204 |

POST/PUT-body:

```json
{"objectType":"asset","name":"Woning Sögel","description":"Eigen woning"}
```

De naam is niet leeg na trimmen; de omschrijving is optioneel. Namen hoeven niet uniek te zijn. Een GET-object bevat naast identiteit optioneel `currentValue`: een afgeleide waarderegel, geen extra veld in `financial_objects`.

## Waarden

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/financial-objects/{id}/values` | Historie, aflopend op datum en ID |
| `POST /api/v1/financial-objects/{id}/values` | Waarde toevoegen; 201 |
| `DELETE /api/v1/financial-objects/{id}/values/{valueId}` | Waarde van dit object verwijderen; 204 |

POST-body:

```json
{"valueDate":"2026-10-01","valueCents":59850000}
```

Beide velden zijn vereist. `valueDate` moet een geldige kalenderdatum in `YYYY-MM-DD` zijn. Nul en negatieve bedragen zijn toegestaan. De actuele waarde is de regel met de hoogste `value_date`; bij gelijke datum de hoogste ID. Een waarde verwijderen laat automatisch de vorige actuele regel zichtbaar worden. Een waarde van een ander object kan niet via dit pad worden verwijderd.

## Objectrelaties

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/financial-objects/{id}/relations` | Inkomende en uitgaande relaties van een object |
| `POST /api/v1/object-relations` | Relatie aanmaken; 201 |
| `DELETE /api/v1/object-relations` | Relatie verwijderen op basis van de drie sleutelvelden in de body; 204 |

POST/DELETE-body:

```json
{"sourceObjectId":2,"targetObjectId":1,"relationType":"finances"}
```

Beide objecten moeten bestaan, de eindpunten moeten verschillend zijn en het relatietype mag niet leeg zijn. Er is geen enum. De combinatie bron/doel/type is uniek.

## Handmatige inkomsten en uitgaven

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/transactions` | Transactieoverzicht met totalen en paginering |
| `POST /api/v1/transactions` | Transactie met optionele objecttoewijzingen aanmaken; 201 |
| `POST /api/v1/transactions/bulk-classify` | Geselecteerde zichtbare transacties volledig aan één financieel object toewijzen; 204 |
| `GET /api/v1/transactions/{id}` | Eén transactie inclusief koppelingen en toewijzingsstatus |
| `PUT /api/v1/transactions/{id}` | Gegevens en alle koppelingen atomair vervangen; 200 |
| `DELETE /api/v1/transactions/{id}` | Transactie en eigen objectkoppelingen atomair verwijderen; 204 |

POST/PUT-body, bijvoorbeeld een hypotheekbetaling:

```json
{
  "transactionDate": "2026-10-03",
  "amountCents": -100000,
  "description": "Hypotheekbetaling",
  "counterparty": "Sparda",
  "allocations": [
    {"financialObjectId": 2, "amountCents": -70000},
    {"financialObjectId": 3, "amountCents": -30000}
  ]
}
```

Datum en bedrag zijn vereist. Bedragen zijn niet nul; negatief is uitgaand, positief inkomend. Omschrijving/tegenpartij zijn optioneel. `transactions` bevat geen categorie of directe object-ID.

- `allocations` mag ontbreken of leeg zijn: nog niet geclassificeerd.
- Elk object mag één keer voorkomen en moet bestaan.
- NULL/ontbrekend `allocations[].amountCents` betekent alleen context; dit wijst geen geld toe.
- Expliciete toewijzingen hebben hetzelfde teken als de transactie en mogen samen het bedrag niet overschrijden. Gedeeltelijke toewijzing is toegestaan.
- Bij een som gelijk aan het transactiebedrag is `fullyAllocated=true`. `unallocatedAmountCents` toont het getekende restant.
- Een PUT vervangt het gehele record en alle koppelingen; ontbrekende/leeggegeven `allocations` ontkoppelt de objecten.
- Een mislukte mutatie laat zowel gegevens als koppelingen intact. Objectwaarden veranderen niet door een transactie.

GET-lijst accepteert `direction=income` of `direction=expense` (weglaten voor beide), `limit` (1–100, default 25) en `offset` (vanaf 0). De response bevat `items`, `total`, `incomeCents`, `expenseCents` en `balanceCents`. Uitgaventotalen zijn negatief in de API. Totalen gelden voor de volledige gefilterde selectie, niet alleen de pagina, en zijn gebaseerd op transactiebedragen zonder join-dubbeltelling. Sortering is transactiedatum aflopend, daarna ID aflopend.

De browser laat een positief bedrag invoeren en bepaalt het teken uit Inkomst/Uitgave. Het formulier ondersteunt meerdere objectkoppelingen, splitsingen en een knop om het volledige bedrag aan een object toe te wijzen.

## Documenten

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/documents` | Alle documenten, inclusief status, analyse en gekoppelde object-ID's |
| `POST /api/v1/documents` | Document uploaden naar S3/Garage en metadata opslaan; zet analyse in de queue als document-AI beschikbaar is |
| `POST /api/v1/documents/analyze-batch` | Alle documenten met status `I` of `F` opnieuw in de analysequeue zetten |
| `POST /api/v1/documents/analyze-selected` | Opgegeven document-ID's opnieuw in de analysequeue zetten |
| `GET /api/v1/documents/{id}` | Eén document |
| `GET /api/v1/documents/{id}/content` | Originele documentbytes via de API streamen |
| `POST /api/v1/documents/{id}/analyze` | Eén document direct analyseren en resultaat opslaan |
| `POST /api/v1/documents/{id}/links` | Document handmatig aan een financieel object koppelen |
| `DELETE /api/v1/documents/{id}/links/{objectId}` | Document-objectkoppeling verwijderen |
| `POST /api/v1/documents/{id}/classify/manual` | Documentstatus op handmatig geclassificeerd zetten |
| `DELETE /api/v1/documents/{id}` | Metadata en objectbytes verwijderen |

Upload gebruikt `multipart/form-data` met veld `file`; documenten zijn maximaal 25 MiB. De originele bytes staan uitsluitend in S3/Garage. SQLite bewaart metadata, `extractedText`, `translatedText`, `analysis` en `financialObjectIds`.

`POST /api/v1/documents/analyze-selected` accepteert:

```json
{"documentIds":[1,2,3]}
```

Ongeldige, dubbele of verdwenen IDs worden genegeerd. De response is `202 Accepted` met `{ "queued": n }`.

Document-AI haalt tekst lokaal uit PDF's en afbeeldingen met de runtime-tools in de app-container. De OpenAI-compatible LLM krijgt bestaande financiële objecten mee en mag alleen bestaande object-ID's teruggeven. De backend valideert deze IDs opnieuw voordat koppelingen in `document_financial_objects` worden opgeslagen.

## Bankimport

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/bank-configs` | Beschikbare YAML-bankconfiguraties |
| `GET /api/v1/bank-configs/{slug}` | Een bankconfiguratie ophalen |
| `PUT /api/v1/bank-configs/{slug}` | YAML-bankconfiguratie opslaan en synchroniseren als importprofiel |
| `DELETE /api/v1/bank-configs/{slug}` | YAML-bankconfiguratie verwijderen |
| `POST /api/v1/bank-imports/preview` | CSV lokaal parsen via gekozen bankconfiguratie |
| `POST /api/v1/bank-imports` | CSV importeren als niet-geclassificeerde transacties |

Preview en import gebruiken `multipart/form-data` met `bankConfig` en `file`. Bestanden zijn maximaal 8 MiB; een definitieve import bevat maximaal 5.000 transactieregels. De response bevat `total`, `imported` en `skipped`. Dezelfde genormaliseerde datum, bedrag, omschrijving en tegenpartij binnen hetzelfde importprofiel wordt als duplicaat overgeslagen, ook bij een tweede upload van hetzelfde bestand. Classificatie is een afzonderlijke actie via `POST /api/v1/transactions/classify-ai`; het AI-contract staat in [Bankimport en AI](ai-bankimport.md).

## Fouten en invoer

- 400: ongeldige JSON, onbekende velden, meerdere JSON-waarden, ongeldig ID, type, naam, datum of ontbrekend bedrag.
- 413: document of bankimportbestand is groter dan de endpointlimiet.
- 401: ontbrekende of onjuiste authenticatie.
- 404: record of API-route bestaat niet. Oude rekening-, categorie-, dossier- en configuratieroutes retourneren JSON-404, geen frontend-HTML.
- 409: dubbele relatie of een verwijdering die door foreign keys wordt geblokkeerd. Verwijder eerst waarden/koppelingen voordat een financieel object definitief wordt verwijderd.
- 500: interne databasefout; details blijven uit de response.

Mutatiebodies zijn begrensd op 64 KiB. Objecten, transacties, waarden en relaties worden persistent opgeslagen; lege lijsten zijn `[]`, niet `null`.

## Systeemconfiguratie

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/system/configuration` | Runtimeconfiguratie, databasekeuze, S3-status, AI-status en beschikbare talen |
| `PUT /api/v1/system/configuration` | S3-, taal-, database- en AI-instellingen opslaan |
| `GET /api/v1/system/transaction-ai-models?url=...` | Modellen ophalen via OpenAI-compatible `/models` endpoint |
| `GET /api/v1/system/ai-docs-config` | `config/ai-docs.yaml` lezen |
| `PUT /api/v1/system/ai-docs-config` | Document-AI YAML-configuratie opslaan |

De databasebackend is vast SQLite. `databasePath` mag alleen naar een `.db`-bestand binnen dezelfde datamap wijzen, standaard `/app/data`. Een gewijzigde databasekeuze wordt actief na herstart. Document-AI gebruikt `pdfAiUrl` en `pdfAiModel`; wanneer de UI dat aangeeft kunnen dezelfde URL en model ook voor transactie-AI worden gebruikt.

## Systeembackup

`GET /api/v1/system/backup` geeft naast de configuratiestatus het backupbeleid en de tijden van de laatste controle en restore-proef terug.

`PUT /api/v1/system/backup` accepteert optioneel `policy`:

```json
{
  "policy": {
    "schedule": "02:30",
    "keepDaily": 14,
    "keepWeekly": 8,
    "keepMonthly": 24
  }
}
```

Na iedere backup controleert de service de repository en voert zij een niet-destructieve restore-proef uit. Herstel synchroniseert de objectbucket exact voordat SQLite wordt vervangen.
