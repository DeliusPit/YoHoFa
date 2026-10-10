# Datamodel v3 — de eenvoudige kern

Status: **stappen 1 en 2 plus handmatige transacties geïmplementeerd**. Schema, versiebeheer, object-/waarde-/relatiebeheer en inkomsten/uitgaven met objecttoewijzingen werken via API en browser. Bankimport en documentflows zijn vervolgstappen.

Dit document legt het complete vereenvoudigde doelmodel vast. De kern bestaat uit **negen tabellen**. Het [canonieke SQLite-schema](../internal/store/core_schema.sql) vertaalt de tabeldefinities naar SQLite en wordt door de actieve store ingebed.

De [technische architectuur v3](technical-architecture-v3.md) gebruikt uitsluitend SQLite voor gestructureerde data en Garage/S3 voor documenten. Eén Go-service levert de browserfrontend en API, gedeployed met Docker Compose. De [actieve API](api-v3.md) beschrijft het object-, waarde- en relatiebeheer.

Eén YoHoFa-deployment/database vertegenwoordigt één huishouden. Het doelmodel bevat geen `households`-tabel of `household_id`-velden.

**Uitvoeringsbesluit:** alle bestaande administratiegegevens mogen vervallen. V3 begint met een nieuwe, lege SQLite-database en zonder oude documentkoppelingen. Dataconversie, ID-mapping en een migratiearchief zijn niet nodig. Het daadwerkelijk verwijderen van oude opslag is een aparte uitvoeringsstap.

## 1. Het financiële object is de kern

Een financieel object heeft alleen een ID, type, naam en omschrijving.

| `object_type` | Betekenis | Voorbeelden |
| --- | --- | --- |
| `asset` | Bezitting | Woning, auto, bankrekening |
| `debt` | Schuld | Hypotheek, lening |
| `reserve` | Reservering | Nieuw dak, toekomstige onderhoudskosten |
| `expense` | Last | Boodschappen, benzine, zorgverzekering |
| `financial` | Financieel | Inkomsten, contantgeldopnames, interne overboekingen |

De applicatie valideert deze vijf typewaarden; de aangeleverde SQL gebruikt hiervoor een vrij tekstveld.

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
- `object_type`: `asset`, `debt`, `reserve`, `expense` of `financial`.
- `name`: naam van het object.
- `description`: optionele toelichting.

### `transactions`

- `id`: identificatie.
- `transaction_date`: datum.
- `amount_cents`: bedrag in integer centen.
- `description`: optionele omschrijving.
- `counterparty`: optionele tegenpartij.
- `hidden`: 0/1-vlag voor verborgen transacties; verborgen transacties blijven bewaard maar tellen niet mee in standaardlijsten en dashboardtotalen.

Een transactie is een genormaliseerde financiële mutatie: negatief betekent dat geld eruit gaat, positief dat geld binnenkomt. De transactie bevat geen categorie en geen direct `financial_object_id`. Classificatie en context komen uitsluitend uit de koppeltabel. Ook ontvangen bedragen blijven transacties; er komt geen vijfde objecttype bij.

### `transaction_financial_objects`

- `transaction_id`: de transactie.
- `financial_object_id`: het financiële object.
- `amount_cents`: optioneel aan het object toegewezen bedrag.

De samengestelde primary key staat per transactie/object-paar één koppeling toe. Meerdere objecten per transactie maken zowel gedeelde context als splitsing van bedragen mogelijk.

Voor de applicatie wordt het bedrag op de koppeling gebruikt voor een expliciete toewijzing; NULL betekent een koppeling zonder expliciete bedragstoewijzing. Rapportages mogen een gekoppelde transactie niet automatisch voor ieder object volledig meetellen. Negatieve bedragen blijven uitgaven en positieve bedragen ontvangsten, zoals in de huidige applicatie.

Bij volledige classificatie is de som van de toegewezen bedragen gelijk aan `transactions.amount_cents`. Een transactie mag tijdelijk geen koppeling hebben: deze is nog niet geclassificeerd. Een contextkoppeling zonder bedrag maakt een transactie op zichzelf niet volledig financieel geclassificeerd. De handmatige transactieflow is geïmplementeerd, inclusief gedeeltelijke toewijzing en het getekende restant.

Voorbeelden:

- ALDI, -83,42 euro → Boodschappen (`expense`), -8342 cent.
- Hypotheekbetaling, -1000 euro → Hypotheek Sparda (`debt`), -70000 cent én Hypotheekrente (`expense`), -30000 cent.

De applicatie controleert gelijke tekens, unieke objectkoppelingen en het voorkomen van overtoewijzing. Transactiegegevens en koppelingen worden samen opgeslagen/vervangen in één korte SQLite-transactie; de schema-SQL bevat geen aggregate constraint. Bij categoriseren kan dezelfde tegenpartij automatisch worden toegepast op zichtbare transacties zonder bestaande objectkoppeling.

### `documents`

- `id`: identificatie.
- `filename`: oorspronkelijke bestandsnaam.
- `object_key`: verwijzing naar het originele bestand in S3/Garage.
- `document_date`: optionele documentdatum.
- `description`: optionele omschrijving.
- `extracted_text`: tekst uit het document.
- `translated_text`: Nederlandse vertaling van de documenttekst.
- `analysis_json`: gestructureerde documentanalyse uit lokale tekstextractie/OCR plus OpenAI-compatible LLM.

SQLite bewaart metadata en tekst/analyses; S3/Garage bewaart de fysieke bestanden. De S3-configuratie bepaalt de opslaglocatie. Objectkeys bevatten geen classificatie of andere businessbetekenis.

YoHoFa verzorgt tekstextractie en OCR lokaal met de runtime-tools in de app-container. De geconfigureerde OpenAI-compatible LLM verzorgt vertaling en analyse op basis van die tekst en krijgt bestaande financiële objecten aangeboden. De resultaten komen in de drie inhoudsvelden op `documents`; objectkoppelingen worden apart opgeslagen via `document_financial_objects`. De tabel heeft geen direct `financial_object_id`.

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

Alle waarden worden als historie opgeslagen. De actuele waarde is de waarde met de meest recente `value_date`; er staat geen actuele-waarde-cache op `financial_objects`. Bij gelijke datums gebruikt de applicatie `id DESC` als vaste tie-break.

| Object | Waardedatum | `value_cents` |
| --- | --- | ---: |
| Woning | 2026-03-01 | 59850000 |
| Woning | 2027-01-01 | 62500000 |
| Hypotheek | 2026-03-01 | 14000000 |
| Hypotheek | 2026-10-01 | 13400000 |

Hetzelfde mechanisme dient voor woningwaarde, banksaldo, beleggingswaarde, restschuld en reserveringsbedrag. De SQL legt geen niet-negativiteitsconstraint op, zodat bijvoorbeeld ook een negatief banksaldo kan worden vastgelegd. Een waardesnapshot wordt niet automatisch bij alle historische transacties opgeteld.

### `bank_import_profiles`

- `id`: identificatie.
- `name`:  profielnaam, bijvoorbeeld ABN CSV.
- `source_format`: bronformaat, bijvoorbeeld `csv`.
- `configuration`: JSON-weergave van het YAML-configuratiebestand uit `config/banks/` voor parsing en veldmapping.

Bankspecifieke verschillen staan in configuratie, niet in vertakkingen zoals `if bank == "ABN"` of `if bank == "Sparda"`. Formaatparsers mogen generieke CSV/PDF/etc.-mechaniek implementeren; banknamen en bronkolommen bepalen niet de rest van de applicatie. De beheerpagina leest en schrijft bestanden zoals `config/banks/abn.yaml`, `config/banks/sparda.yaml` en `config/banks/triodos.yaml`.

Configuratievoorbeeld:

```yaml
bank:
  name: "Bank West"

source:
  format: csv
  delimiter: ";"
  encoding: utf-8

mapping:
  transaction_date: Buchungstag
  amount_cents: Betrag
  description: Verwendungszweck
  counterparty: Auftraggeber/Empfänger

formats:
  transaction_date: "02.01.2006"
  decimal_separator: ","
```

Elk profiel levert dezelfde canonieke velden: `transaction_date`, `amount_cents`, `description` en `counterparty`. Een bedrag zoals `-83,42` wordt exact -8342 integer centen.

### `bank_import_rows`

- `id`: identificatie.
- `profile_id`: het gebruikte importprofiel.
- `raw_data`: de originele bankregel als JSON.
- `transaction_id`: optionele verwijzing naar de uiteindelijke transactie.

Flow: bankbestand → importprofiel → preview → ruwe regel → `bank_import_rows` → genormaliseerde transactie. De eerste importversie ondersteunt CSV-bestanden via de generieke bankconfiguratie en schrijft succesvolle imports direct naar `transactions`. Zolang nog geen transactie is aangemaakt kan `transaction_id` NULL zijn. Brongegevens blijven behouden als de normalisatie niet lukt.

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

- Hypotheek Bank → `finances` → Woning .
- Nieuw dak → `reserved_for` → Woning .
