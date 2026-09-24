# HoHoHo Documentatie

Deze directory bevat de ontwerp- en technische documentatie voor HoHoHo, de huishoudadministratie-applicatie.

## Startpunten

- [Functioneel ontwerp](functional-design.md): beschrijft alle gebruikersfuncties, schermen, domeinen en gewenste werking.
- [Technisch ontwerp](technical-design.md): beschrijft architectuur, lagen, stores, objectopslag, configuratie, migraties en betrouwbaarheid.
- [API-overzicht](api-reference.md): beschrijft de HTTP API per functiegroep.
- [AI bij bankimport](ai-bankimport.md): ontwerp voor AI-ondersteunde transactiecategorisatie.
- [Bankimport](bankimport.md): bestaande importprofielen en gebruik.
- [Opslagarchitectuur](storage-architecture.md): opslagprofielen, schema-aanpak en migratiepaden.

## Applicatiefuncties

HoHoHo ondersteunt momenteel deze hoofdgebieden:

- Dashboard en financiële samenvatting.
- Bankrekeningen.
- Hoofd- en subcategorieen.
- Banktransactie-import en duplicate-detectie.
- Transactiebeheer, filteren, verbergen en categoriseren.
- Rapportages per categorie en periode.
- Documentbeheer met metadata, objectopslag en documentanalyse.
- Bezittingen inclusief waarderingen, contracten, documenten, transacties en tijdlijn.
- Schulden inclusief schuldeiser-normalisatie, documenten, transacties en tijdlijn.
- Reserveringen voor toekomstige verplichtingen.
- Dossiers voor administratieve contexten.
- Generieke relaties tussen documenten, transacties, bezittingen, schulden, dossiers en reserveringen.
- Domeingebeurtenissen als gebruikersgerichte tijdlijn.
- Systeemconfiguratie voor database, objectopslag en lokale LLM.
- Systeembackup en herstel met restic/rclone.

## Architectuur in het kort

- Backend: Go HTTP API vanuit `cmd/api`.
- Applicatielogica: `internal/`.
- Stores: PostgreSQL, SQLite en MySQL via `internal/store`.
- Frontend: Vite met vanilla JavaScript in `web/`.
- PostgreSQL-migraties: `migrations/`.
- Objectopslag: filesystem of S3-compatible provider via `internal/objectstore`.
- Productieruntime: Docker Compose met PostgreSQL en optionele S3-compatible objectopslag.

## Documentatieregels

- De relationele database blijft de bron van waarheid voor metadata, relaties en applicatiestatus.
- Originele documenten blijven immutable objecten in filesystem/S3.
- Nieuwe domeinfuncties moeten in PostgreSQL, SQLite en MySQL consistent blijven zolang die stores ondersteund worden.
- UI-compatibiliteit is belangrijk: raw bank- en documentgegevens worden niet stilzwijgend overschreven.
