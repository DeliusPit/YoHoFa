# Datamodel v3 — de eenvoudige kern

Status: **doelontwerp, nog niet geïmplementeerd**.

Dit document legt het complete vereenvoudigde doelmodel vast. De kern bestaat uit **negen tabellen**. Het [SQLite-referentieschema](target-schema-v3.sql) vertaalt de aangeleverde tabeldefinities naar de gekozen database, met dezelfde velden en relaties. Het [huidige datamodel](data-model.md) beschrijft de bestaande applicatie.

De [technische doelarchitectuur v3](technical-architecture-v3.md) gebruikt uitsluitend SQLite voor gestructureerde data en Garage/S3 voor documenten. Eén Go-service levert de browserfrontend en API, gedeployed met Docker Compose.

Eén HoHoHo-deployment/database vertegenwoordigt één huishouden. Het doelmodel bevat geen `households`-tabel of `household_id`-velden.

**Uitvoeringsbesluit:** alle bestaande administratiegegevens mogen vervallen. V3 begint met een nieuwe, lege SQLite-database en zonder oude documentkoppelingen. Dataconversie, ID-mapping en een migratiearchief zijn niet nodig. Het daadwerkelijk verwijderen van oude opslag is een aparte uitvoeringsstap.

## 1. Het financiële object is de kern

Een financieel object heeft alleen een ID, type, naam en omschrijving.

| `object_type` | Betekenis | Voorbeelden |
| --- | --- | --- |
| `asset` | Bezitting | Woning, auto, bankrekening |
| `debt` | Schuld | Hypotheek, lening |
| `reserve` | Reservering | Nieuw dak, toekomstige onderhoudskosten |
| `expense` | Last | Boodschappen, benzine, zorgverzekering |

De applicatie valideert deze vier typewaarden; de aangeleverde SQL gebruikt hiervoor een vrij tekstveld.

Transacties en documenten worden aan één of meer financiële objecten gekoppeld. Een nieuw object aanmaken vereist geen nieuwe domeintabel.

`financial_objects` bevat uitsluitend de identiteit: geen bank-, woning-, hypotheek-, verzekerings- of contractgegevens en geen actuele waarde. Waarden staan in `object_values`.

## 2. De negen tabellen

```text
financial_objects
    ├─ object_values
    ├─ transaction_financial_objects ── transactions
    ├─ document_financial_objects ── documents ── object_key → S3/Garage
    └─ object_relations ── financial_objects

bank_import_profiles
    └─ bank_import_rows ── transactions
                              └─ transaction_financial_objects ── financial_objects
```

### `financial_objects`

- `id`: identificatie.
- `object_type`: `asset`, `debt`, `reserve` of `expense`.
- `name`: naam van het object.
- `description`: optionele toelichting.

### `transactions`

- `id`: identificatie.
- `transaction_date`: datum.
- `amount_cents`: bedrag in integer centen.
- `description`: optionele omschrijving.
- `counterparty`: optionele tegenpartij.

Een transactie is een genormaliseerde financiële mutatie: negatief betekent dat geld eruit gaat, positief dat geld binnenkomt. De transactie bevat geen categorie en geen direct `financial_object_id`. Classificatie en context komen uitsluitend uit de koppeltabel. Ook ontvangen bedragen blijven transacties; er komt geen vijfde objecttype bij.

### `transaction_financial_objects`

- `transaction_id`: de transactie.
- `financial_object_id`: het financiële object.
- `amount_cents`: optioneel aan het object toegewezen bedrag.

De samengestelde primary key staat per transactie/object-paar één koppeling toe. Meerdere objecten per transactie maken zowel gedeelde context als splitsing van bedragen mogelijk.

Voor de applicatie wordt het bedrag op de koppeling gebruikt voor een expliciete toewijzing; NULL betekent een koppeling zonder expliciete bedragstoewijzing. Rapportages mogen een gekoppelde transactie niet automatisch voor ieder object volledig meetellen. Negatieve bedragen blijven uitgaven en positieve bedragen ontvangsten, zoals in de huidige applicatie.

Bij volledige classificatie is de som van de toegewezen bedragen gelijk aan `transactions.amount_cents`. Een transactie mag tijdelijk geen koppeling hebben: deze is nog niet geclassificeerd. Een contextkoppeling zonder bedrag maakt een transactie op zichzelf niet volledig financieel geclassificeerd.

Voorbeelden:

- ALDI, -83,42 euro → Boodschappen (`expense`), -8342 cent.
- Hypotheekbetaling, -1000 euro → Hypotheek Sparda (`debt`), -70000 cent én Hypotheekrente (`expense`), -30000 cent.

Deze somregel wordt door de applicatie gecontroleerd; de referentie-SQL bevat geen aggregate constraint.

### `documents`

- `id`: identificatie.
- `filename`: oorspronkelijke bestandsnaam.
- `object_key`: verwijzing naar het originele bestand in S3/Garage.
- `document_date`: optionele documentdatum.
- `description`: optionele omschrijving.
- `extracted_text`: tekst uit het document.
- `translated_text`: Nederlandse vertaling van de documenttekst.
- `analysis_json`: gestructureerde analyse van de externe PDF-AI-service.

SQLite bewaart metadata en tekst/analyses; S3/Garage bewaart de fysieke bestanden. De S3-configuratie bepaalt de opslaglocatie. Objectkeys bevatten geen classificatie of andere businessbetekenis.

De externe PDF-AI-service verzorgt OCR indien nodig, tekstextractie, vertaling en AI-analyse. De resultaten komen rechtstreeks in de drie inhoudsvelden op `documents`. De tabel heeft geen direct `financial_object_id`.

### `document_financial_objects`

- `document_id`: het document.
- `financial_object_id`: het financiële object.

De samengestelde primary key voorkomt dubbele koppelingen. Eén document kan bij meerdere objecten horen zonder het bestand te kopiëren. Ontkoppelen verwijdert het originele bestand niet.

Voorbeelden:

- `wartungsvertrag.pdf` → Onderhoud Kleinkläranlage (`expense`) én Woning Sögel (`asset`).
- `hypotheekakte.pdf` → Hypotheek Sparda (`debt`) én Woning Sögel (`asset`).

Een relatietype is hier niet nodig: de koppeling betekent alleen dat het document bij het object hoort.

### `object_values`

- `id`: identificatie.
- `financial_object_id`: het financiële object.
- `value_date`: waardedatum.
- `value_cents`: waarde in integer centen.

Alle waarden worden als historie opgeslagen. De actuele waarde is de waarde met de meest recente `value_date`; er staat geen actuele-waarde-cache op `financial_objects`. Bij gelijke datums kan de applicatie `id DESC` als vaste tie-break gebruiken.

| Object | Waardedatum | `value_cents` |
| --- | --- | ---: |
| Woning Sögel | 2026-03-01 | 59850000 |
| Woning Sögel | 2027-01-01 | 62500000 |
| Hypotheek Sparda | 2026-03-01 | 14000000 |
| Hypotheek Sparda | 2026-10-01 | 13400000 |

Hetzelfde mechanisme dient voor woningwaarde, banksaldo, beleggingswaarde, restschuld en reserveringsbedrag. De SQL legt geen niet-negativiteitsconstraint op, zodat bijvoorbeeld ook een negatief banksaldo kan worden vastgelegd. Een waardesnapshot wordt niet automatisch bij alle historische transacties opgeteld.

### `bank_import_profiles`

- `id`: identificatie.
- `name`: menselijke profielnaam, bijvoorbeeld ABN CSV.
- `source_format`: bronformaat, bijvoorbeeld `csv`.
- `configuration`: JSON-configuratie voor parsing en veldmapping.

Bankspecifieke verschillen staan in configuratie, niet in vertakkingen zoals `if bank == "ABN"` of `if bank == "Sparda"`. Formaatparsers mogen generieke CSV/PDF/etc.-mechaniek implementeren; banknamen en bronkolommen bepalen niet de rest van de applicatie.

ABN-voorbeeld:

```json
{
  "delimiter": ";",
  "date_field": "Datum",
  "amount_field": "Bedrag",
  "description_field": "Omschrijving",
  "counterparty_field": "Naam tegenpartij"
}
```

Sparda gebruikt hetzelfde mechanisme met een ander profiel:

```json
{
  "delimiter": ";",
  "date_field": "Buchungstag",
  "amount_field": "Umsatz",
  "description_field": "Verwendungszweck",
  "counterparty_field": "Auftraggeber/Empfänger"
}
```

Elk profiel levert dezelfde canonieke velden: `transaction_date`, `amount_cents`, `description` en `counterparty`. Een bedrag zoals `-83,42` wordt exact -8342 integer centen.

### `bank_import_rows`

- `id`: identificatie.
- `profile_id`: het gebruikte importprofiel.
- `raw_data`: de originele bankregel als JSON.
- `transaction_id`: optionele verwijzing naar de uiteindelijke transactie.

Flow: bankbestand → importprofiel → ruwe regel → `bank_import_rows` → genormaliseerde transactie. Zolang nog geen transactie is aangemaakt kan `transaction_id` NULL zijn. Brongegevens blijven behouden als de normalisatie niet lukt.

Voorbeeld van ruwe input:

```json
{
  "Buchungstag": "01.10.2026",
  "Umsatz": "-83,42",
  "Verwendungszweck": "ALDI NORD SOEGEL",
  "Auftraggeber/Empfänger": "ALDI"
}
```

Dit levert `transaction_date=2026-10-01`, `amount_cents=-8342`, `description=ALDI NORD SOEGEL` en `counterparty=ALDI`. De bronregel verwijst vervolgens naar die transactie. De rest van de applicatie gebruikt uitsluitend de genormaliseerde velden.

### `object_relations`

- `source_object_id`: bronobject.
- `target_object_id`: doelobject.
- `relation_type`: vrije tekst die de relatie beschrijft.

De samengestelde primary key voorkomt dubbele relaties met dezelfde bron, hetzelfde doel en hetzelfde type. Alleen financiële objecten zijn eindpunten; documenten en transacties gebruiken hun eigen koppeltabellen.

Voorbeelden:

- Hypotheek Sparda → `finances` → Woning Sögel.
- Nieuw dak → `reserved_for` → Woning Sögel.
- Benzine → `used_for` → Subaru.

Maak geen enum of aparte relatietypetabel. Nieuwe relatietekst introduceert ook geen nieuw objecttype: de eindpunten blijven binnen de vier financiële objecttypes.

## 3. Belangrijke ontwerpregels

1. Geen aparte categorieëntabel.
2. Het financiële object is zelf de classificatie.
3. `transactions` bevat geen `financial_object_id`.
4. Transactie-objectkoppelingen lopen uitsluitend via `transaction_financial_objects`.
5. `documents` bevat geen `financial_object_id`.
6. Document-objectkoppelingen lopen uitsluitend via `document_financial_objects`.
7. Eén transactie kan meerdere financiële objecten raken.
8. Eén document kan meerdere financiële objecten raken.
9. `financial_objects` bevat alleen generieke identiteit: type, naam, omschrijving.
10. Geen woning-, bank-, hypotheek- of verzekeringsspecifieke velden op `financial_objects`.
11. Waarden altijd via `object_values` opslaan.
12. Bankspecifieke input altijd via `bank_import_profiles` configureren.
13. Ruwe bankdata bewaren in `bank_import_rows`.
14. De rest van de applicatie werkt uitsluitend met genormaliseerde `transactions`.
15. `object_relations` bevat alleen relaties tussen financiële objecten onderling.
16. Geen multi-household-schema: één deployment/database is één huishouden.
17. Houd het model klein: alleen tabellen toevoegen als functionaliteit aantoonbaar niet met deze negen tabellen kan worden gerealiseerd.

De kern heeft geen parent-hiërarchie, attributen-JSON, object-kind of lifecycle-model. Er zijn geen aparte dossiers, contracten, partijen, analyse- of importbatchtabellen. API en frontend gebruiken de kernbegrippen en mogen objecten per type tonen.

## 4. Wat moet in de repository veranderen?

| Onderdeel | Nodige verandering |
| --- | --- |
| Database | Een nieuwe, lege SQLite-database met de negen kerntabellen initialiseren; geen bestaande tabellen converteren |
| Store-laag | Financiële objecten, transacties, documenten, waarden, importprofielen/-regels en de drie soorten koppelingen ondersteunen |
| Classificatie | `transactions.category_id` vervangen door `transaction_financial_objects` |
| Bankimport | Bankspecifieke hard-coded profielen en branches vervangen door configuratie; ruwe regels bewaren en aan canonieke transacties koppelen; deduplicatie expliciet afhandelen |
| Waarden en relaties | Waardehistorie en object-objectrelaties generiek opslaan; actuele waarden op meest recente datum lezen |
| Documentopslag | S3/Garage gebruiken voor nieuwe uploads; metadata en koppelingen in SQLite |
| PDF-AI | Response expliciet mappen naar `extracted_text`, `translated_text`, `analysis_json` |
| HTTP API | Objectgerichte routes en koppelingen aanbieden; bestaande domeinroutes vervangen zodra de UI is omgezet |
| Frontend | Beheer van de vier objecttypes, waarden, relaties, importprofielen, transactie-/documentkoppeling en bedragstoewijzing; filters en rapporten daarop aanpassen |
| Startup | `ensureSchema` en `seedDefaults` aanpassen zodat het oude model niet opnieuw wordt aangemaakt |
| Opslag | Eén SQLite-implementatie en Garage/S3-documentstore; database-/objectbackendkeuzes en opslagprofielen vervangen |
| Herstel en checks | Backup/herstel, startup en bestaande verificatiescripts op het vereenvoudigde schema afstemmen |

De belangrijkste code zit in `internal/store/`, `internal/httpserver/`, `internal/bankimport/`, `internal/documenttranslation/` en `web/src/main.js`. De huidige runtime heeft schema-initialisatie per databasebackend, waaronder `internal/store/postgres.go`. V3 krijgt één SQLite-migratielijn; alleen een bestand aan de huidige PostgreSQL-map `migrations/` toevoegen volstaat niet.

## 5. Nieuwe, lege administratie

De overstap is een vervanging van de applicatiestructuur, geen dataconversie. Bestaande transacties, documenten, waarden, dossiers en overige administratiegegevens worden niet meegenomen. Ook oude bankprofielen en seeds worden niet automatisch ingevoerd.

Na het bouwen van de opslagbasis bestaan alle negen tabellen en hun foreign keys, maar bevatten ze geen records. Bij herstart blijft het nieuwe SQLite-bestand behouden. De oude startupcode mag daar geen v2-tabellen of voorbeeldgegevens aan toevoegen.

De administratie wordt vervolgens vanuit de browser gevuld met financiële objecten, waarden, relaties, importprofielen, transacties en documenten. De nieuwe documentopslag krijgt een eigen lege bucket voor HoHoHo; het wissen van oude objecten richt zich uitsluitend op de oude HoHoHo-opslag, niet op andere Garage-buckets.

Het SQL-model bepaalt geen valuta; de applicatie moet één expliciete bedrag-/valutaconventie gebruiken. Een reserveringsbedrag is geen vrij beschikbaar bezit; een positief opgeslagen schuld wordt bij nettovermogen afgetrokken.

De foreign keys bepalen nog geen cascadegedrag: verwijderen van een gekoppeld object, document of transactie wordt standaard geblokkeerd. De applicatie moet ontkoppelen en verwijderen als expliciete acties behandelen.

Een gewijzigde importconfiguratie verandert de reeds opgeslagen canonieke transacties niet. Het model bevat geen profielsnapshot: leg bij de implementatie vast hoe wijzigingen en opnieuw normaliseren worden behandeld. De SQL voorkomt ook niet zelfstandig dubbele imports; deduplicatiegedrag moet expliciet worden geïmplementeerd zonder het oude fingerprint-/batchmodel automatisch terug te brengen.

## 6. Uitvoeringsvolgorde

1. Leg de huidige code vast en ontwikkel v3 op `v3-simplification`; het besluit om leeg te starten staat hierboven.
2. Bouw de nieuwe SQLite-schema-/storelaag, schemaversiebeheer en basis voor objecten, waarden en relaties. Start zonder records of oude seeds.
3. Zet API, configureerbare import, PDF-AI en frontend om naar de negen tabellen.
4. Verifieer met zelf aangemaakte testgegevens: allocaties, waardehistorie, relaties, importherkomst, documentinhoud, herstart en herstel.
5. Deploy v3 met een lege administratie en ruim de vervangen code en oude HoHoHo-opslag als expliciete uitvoeringsstappen op.

Acceptatie: de applicatie gebruikt operationeel uitsluitend deze negen domeintabellen in SQLite, valideert de vier objecttypes, ondersteunt meerdere koppelingen per transactie/document, controleert volledige bedragstoewijzing en telt bedragen in rapportages niet dubbel. Waarden zijn historisch, objectrelatietypes zijn vrije tekst en bankverschillen komen uit profielen. Beschikbare originele bronregels blijven herleidbaar naar transacties. Eén database vertegenwoordigt één huishouden. Documenten staan in Garage/S3; de browserapp, startup en herstel werken via de nieuwe Compose-deployment.
