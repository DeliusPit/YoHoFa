# API v3 — objecten, transacties, waarden en relaties

De actieve API van `cmd/api` gebruikt uitsluitend de SQLite-kern. Objectbeheer, waarden, relaties, samenvatting, handmatige transacties met objecttoewijzingen en CSV-bankimport zijn beschikbaar. Documenten volgen later.

Alle bedragen zijn integer centen. De browserinterface gebruikt EUR als huidige weergaveconventie. Objecttypes zijn exact `asset`, `debt`, `expense`, `reserve`. Relatietypes zijn vrije niet-lege tekst.

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
- `objectGroups`: per asset/debt/reserve/expense het aantal huidige objecten, `valuedCount`, de som van bekende `valueCents` en `periodAmountCents` (expliciet toegewezen mutaties in de periode).
- `netWorthCents`: bekende bezitwaarden minus bekende schuldsaldi. Reserveringen en lasten worden niet afgetrokken als actuele schuld.
- `transactions`: aantal posten, aantallen inkomsten/uitgaven, `incomeCents`, negatief `expenseCents`, `balanceCents` en `unallocatedCount` (nog niet volledig toegewezen transacties).

Waarderingen gebruiken de laatste `value_date <= to`, daarna hoogste ID bij gelijke datum. Oudere waarden blijven geldig tot een volgende waardering. Waarden na de peildatum tellen niet mee. Een object zonder waardering is herkenbaar via `valuedCount`; het wordt niet als bekend nulbedrag gepresenteerd. Het model bevat geen aanmaak-/sluitdatum van objectidentiteiten, dus objectaantallen beschrijven de huidige objecten, niet een historische inventaris.

Transactiebedragen worden uitsluitend binnen `from` t/m `to` opgeteld, één keer per transactie. Splitsingen en contextlinks veroorzaken geen dubbeltelling. `periodAmountCents` gebruikt alleen expliciete bedragen op koppelingen en staat los van waardesnapshots. Er worden geen tabel, waarde-cache of bankimportflow toegevoegd.

De browser toont jaar- en maandknoppen boven beide panelen. Een andere jaarselectie kiest eerst het hele jaar; klikken op een geselecteerde maand kiest weer het hele jaar. Het dashboard houdt een eigen periode bij en verandert geen ingevoerde gegevens of de filters van het transactiebeheer.

## Financiële objecten

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/financial-objects` | Alle objecten; optioneel `?type=asset` (of een van de andere drie types) |
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

## Bankimport

| Methode en pad | Werking |
| --- | --- |
| `GET /api/v1/bank-configs` | Beschikbare YAML-bankconfiguraties |
| `GET /api/v1/bank-configs/{slug}` | Een bankconfiguratie ophalen |
| `PUT /api/v1/bank-configs/{slug}` | YAML-bankconfiguratie opslaan en synchroniseren als importprofiel |
| `DELETE /api/v1/bank-configs/{slug}` | YAML-bankconfiguratie verwijderen |
| `POST /api/v1/bank-imports/preview` | CSV lokaal parsen via gekozen bankconfiguratie |
| `POST /api/v1/bank-imports` | CSV importeren en transacties via `TRANSACTION_AI_URL` aan objecten koppelen |

Preview en import gebruiken `multipart/form-data` met `bankConfig` en `file`. Definitieve import stuurt de genormaliseerde regels plus bestaande financiële objecten naar de transactie-AI. Het AI-contract staat in `docs/ai-bankimport.md`.

## Fouten en invoer

- 400: ongeldige JSON, onbekende velden, meerdere JSON-waarden, ongeldig ID, type, naam, datum of ontbrekend bedrag.
- 401: ontbrekende of onjuiste authenticatie.
- 404: record of API-route bestaat niet. Oude rekening-, categorie-, dossier- en configuratieroutes retourneren JSON-404, geen frontend-HTML.
- 409: dubbele relatie of een verwijdering die door foreign keys wordt geblokkeerd. Verwijder eerst waarden/koppelingen voordat een financieel object definitief wordt verwijderd.
- 500: interne databasefout; details blijven uit de response.

Mutatiebodies zijn begrensd op 64 KiB. Objecten, transacties, waarden en relaties worden persistent opgeslagen; lege lijsten zijn `[]`, niet `null`.
