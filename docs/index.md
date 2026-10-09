# YoHoFa-documentatie

## Actieve v3

De actieve runtime bevat de negen SQLite-tabellen, schema-versiebeheer, beheer van financiële objecten, waarden, relaties, inkomsten/uitgaven, configureerbare CSV-bankimport, S3-documenten, lokale OCR/documentanalyse met OpenAI-compatible LLM en backup/herstel. Nieuwe databases hebben geen seeds; v3-data blijft na herstart behouden.

- [Datamodel v3](data-model-v3.md): de negen domeintabellen en ontwerpregels.
- [Technische architectuur v3](technical-architecture-v3.md): vaste SQLite/Garage-stack, gerealiseerde basis en vervolgstappen.
- [API v3](api-v3.md): actieve object-, transactie-, document-, import- en backuproutes.
- [Bankimport en AI](ai-bankimport.md): classificatie van opgeslagen, niet-geclassificeerde transacties.
- [Canoniek SQLite-schema](../internal/store/core_schema.sql): ingebed door de actieve store.
- [README](../README.md): starten, Compose, configuratie en verificatie.

Bankimport slaat dubbele genormaliseerde regels per importprofiel over en begrenst uploads. `setup.sh` kan een complete stack met lokale Garage/S3 initialiseren; Compose kan ook met bestaande S3/Garage-storage werken.

## Ontwerpdiscipline

- Eén database/deployment is één huishouden.
- SQLite is de enige structured-data-store; originele documenten staan in Garage/S3.
- Financiële objecten zijn de classificatie. Geen extra domeintabellen zonder concrete behoefte die niet met de negen tabellen kan worden gerealiseerd.
- Beschrijf gerealiseerde functies en plannen apart; nieuwe startup mag geen legacytabellen of seeds toevoegen.
