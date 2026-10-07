# HoHoHo-documentatie

## Actieve v3-basis

Stappen 1 en 2 en de handmatige transactieflow zijn gerealiseerd: negen lege SQLite-tabellen, schema-versiebeheer en beheer van financiële objecten, waarden, relaties en inkomsten/uitgaven via API en browser. Nieuwe databases hebben geen seeds; v3-data blijft na herstart behouden.

- [Datamodel v3](data-model-v3.md): de negen domeintabellen en ontwerpregels.
- [Technische architectuur v3](technical-architecture-v3.md): vaste SQLite/Garage-stack, gerealiseerde basis en vervolgstappen.
- [API v3](api-v3.md): actieve object-, transactie-, waarde- en relatieroutes.
- [Canoniek SQLite-schema](../internal/store/core_schema.sql): ingebed door de actieve store.
- [README](../README.md): starten, Compose, configuratie en verificatie.

Configureerbare bankimport, documenten/PDF-AI en backup/herstel worden in de volgende stappen aangesloten. Beheerde Garage/bootstrap volgt later; de huidige Compose gebruikt een bestaand endpoint.

## Historische v2-documentatie

Onderstaande documenten beschrijven de vervangen multi-backendapplicatie en tijdelijk behouden legacycode, niet de actieve v3-routes of opslagkeuzes:

- [Functioneel ontwerp v2](functional-design.md).
- [Technisch ontwerp v2](technical-design.md).
- [Datamodel v2](data-model.md).
- [API-overzicht v2](api-reference.md).
- [Bankimport v2](bankimport.md).
- [Opslagarchitectuur v2](storage-architecture.md).
- [AI-bankimportvoorstel v2](ai-bankimport.md).

## Ontwerpdiscipline

- Eén database/deployment is één huishouden.
- SQLite is de enige structured-data-store; originele documenten staan in Garage/S3.
- Financiële objecten zijn de classificatie. Geen extra domeintabellen zonder concrete behoefte die niet met de negen tabellen kan worden gerealiseerd.
- Beschrijf gerealiseerde functies en plannen apart; nieuwe startup mag geen legacytabellen of seeds toevoegen.
