# HoHoHo v3

House Hold Hosting: huishoudadministratie rond financiële objecten, met uitsluitend SQLite voor data en Garage/S3 voor originele documenten. Eén deployment is één huishouden; de gebruiker werkt in een browser.

## Huidige stap: de lege basis

Stappen 1 en 2 zijn gerealiseerd:

- Bestaande data hoeft niet behouden of geconverteerd te worden.
- Startup maakt een nieuwe SQLite-database met precies negen lege domeintabellen, zonder seeds.
- Onder **Balans & Planning** zijn financiële objecten (`asset`, `debt`, `expense`, `reserve`), waardehistorie en objectrelaties via de browser te beheren; dezelfde functies zijn via de API beschikbaar.
- Balans & Planning heeft een submenu per objectsoort en vier aanmaakknoppen. De objecten staan in vier gelijke kaders: bezittingen linksboven, schulden linksonder, lasten rechtsboven en reserveringen rechtsonder. Het submenu brengt het betreffende kader in beeld; aanmaken/wijzigen opent het gedeelde formulier eronder.
- Wanneer een financieel object is geselecteerd toont Balans & Planning ook de nieuwste gekoppelde transacties met bedrag en objecttoewijzing.
- Onder **Configuratie** staat een v3-overzicht van SQLite, Garage/S3, applicatiestatus en het backup/recovery-blok uit de vorige versie. Garage/S3-instellingen kunnen daar worden opgeslagen; secrets worden niet teruggegeven aan de browser en wijzigingen gelden na een herstart. Databasekeuzes worden niet live gewisseld: v3 gebruikt de vaste SQLite/Garage-combinatie.
- Onder **Applicatie Configuratie** staan twee AI-koppelingen: een PDF/OCR-AI URL voor documentverwerking en een Transactieclassificatie-AI URL voor automatische classificatie van banktransacties.
- Backup en recovery gebruiken opnieuw restic/rclone. De runtime image bevat deze tools; backups omvatten SQLite, Garage/S3-objecten en configuratie. Recovery wordt gepland met bevestiging `HERSTEL` en uitgevoerd bij de volgende start.
- **Bankimport Configuratie** beheert YAML-bestanden onder `config/banks/` zoals `abn.yaml`, `sparda.yaml` en `triodos.yaml`. Elk bestand beschrijft bronformaat, mapping naar `transaction_date`, `amount_cents`, `description` en `counterparty`, plus datum-/bedragformaten. Opslaan valideert het YAML-bestand en synchroniseert generiek naar `bank_import_profiles`; er is geen bankspecifieke Go-vertakking.
- Het tabblad **Import** kan een bankconfiguratie selecteren, een CSV-export uploaden, een preview tonen en de genormaliseerde transacties opslaan. Ruwe CSV-regels worden per geïmporteerde transactie in `bank_import_rows` bewaard.
- Het tabblad **Inkomsten & Uitgaven** bevat handmatige transacties: aanmaken, wijzigen en verwijderen, met datum, bedrag, omschrijving en tegenpartij. Bedragen kunnen over financiële objecten worden verdeeld; ongeclassificeerde en gedeeltelijk toegewezen transacties zijn toegestaan.
- Transacties worden vanuit de browser verborgen in plaats van fysiek verwijderd. Onder **Toon** kunnen verborgen transacties apart worden bekeken en weer zichtbaar worden gemaakt. Bij categoriseren kan dezelfde categorisatie automatisch worden toegepast op zichtbare transacties met dezelfde tegenpartij die nog niet geclassificeerd zijn.
- Het hoofdmenu staat in deze volgorde: **Dashboard**, **Balans & Planning**, **Inkomsten & Uitgaven**, **Documenten**, **Import**, **Configuratie**. Documenten, Import en Configuratie zijn voorbereid met een melding dat hun functies nog volgen. Navigatie, browsertitel en directe hashlinks volgen het actieve tabblad; wisselen tussen tabs bewaart de ingevulde formulieren.
- Het **Dashboard** gebruikt de afgeronde panelen en jaar-/maandknoppen uit de oude vormgeving. Links staan balans en planning op de laatste dag van de geselecteerde periode, rechts inkomsten, uitgaven, saldo en aantallen binnen die periode. Waarderingen worden historisch gelezen; reserveringen en lasten staan apart van de netto balans.
- Het transactieoverzicht toont inkomsten, uitgaven en saldo van de selectie, plus paginering en een filter op richting. Totalen tellen elke transactie één keer, onafhankelijk van het aantal koppelingen.
- SQLite gebruikt foreign keys, WAL, een lock-wachttijd en `PRAGMA user_version` voor schema-evolutie.
- Eén Go-service serveert frontend en API. Opslagprofielen en `config/storage.json` worden door deze runtime niet gelezen.

Bankimportconfiguratie, CSV-preview/import met deduplicatie, backup/herstel en handmatige transacties zijn aangesloten. `cmd/api` gebruikt uitsluitend de SQLite/S3-kern.

## Docker Compose

Maak lokale configuratie vanuit `.env.example` en vul authenticatie en Garage-toegang in. Commit de ingevulde bestanden niet.

```sh
cp .env.example .env
docker compose up --build -d
```

- Browser: `http://localhost:8080`.
- Productie vereist `AUTH_USERNAME` en `AUTH_PASSWORD`.
- De database staat op de host in `data/v3/hohoho.db`, gemount als `/app/data/hohoho.db`.
- De huidige Compose-definitie bevat alleen de applicatieservice en sluit aan op een bestaande Garage-installatie via `S3_*`. Een beheerde Garage-service/bootstrap volgt in de deploymentstap.
- `config/.env` wordt tijdelijk nog als tweede env-file ingelezen voor bestaande installatiecredentials; die waarden gaan vóór `.env`. Gebruik voor nieuwe inrichting de root-`.env` en voorkom dubbele instellingen.
- `/healthz` controleert de SQLite-kern; `/readyz` controleert ook de S3-bucket. Zonder Garage blijft objectbeheer bruikbaar, maar is `/readyz` niet gereed.

Een oude database wordt niet automatisch gewist of omgezet. Als een expliciet `DATABASE_PATH` naar een oud schema wijst, weigert startup dat bestand. De nieuwe default en Compose-volume gebruiken een afzonderlijke v3-map. Oude databasebestanden of HoHoHo-documentobjecten verwijderen is een aparte uitvoeringsstap.

## Lokaal ontwikkelen

Vereisten: Go 1.25+ en Node.js 24+.

```sh
./scripts/dev-api.sh
```

Dit start de API met `data/v3/hohoho.db`. In een tweede terminal, vanuit `web/`:

```sh
npm ci
npm run dev
```

Vite proxy't de API naar poort 8080. Voor frontendassets die de Go-service zelf serveert: voer `npm run build` uit vanuit `web/`.

## Basis-API

```sh
curl http://localhost:8080/api/v1/financial-objects
curl -H 'Content-Type: application/json' \
  -d '{"objectType":"asset","name":"Woning Sögel"}' \
  http://localhost:8080/api/v1/financial-objects
```

Zie [API v3](docs/api-v3.md) voor objectbeheer, waarden, relaties en foutafhandeling. De voorbeelden hierboven gaan uit van lokale ontwikkeling zonder authenticatie.

## Verificatie

```sh
go build ./...
go test ./...
./scripts/check-local.sh
```

De lokale check gebruikt een tijdelijk databasebestand, controleert CRUD en herstart/persistentie en ruimt alleen die testgegevens op. De tests bewaken onder meer negen lege tabellen, foreign keys op nieuwe verbindingen, JSON-validatie, actuele waarden en het weigeren van oude/nieuwere schema's.

## Structuur en documentatie

- `cmd/api`: enige actieve backend-entrypoint.
- `internal/config/runtime.go`: vaste v3-runtimeconfiguratie.
- `internal/store/core.go` en `core_schema.sql`: actieve SQLite-kern en canoniek schema.
- `internal/httpserver/core.go`: actieve v3-routes.
- `web/src/main.js` en `core.css`: browserinterface voor de basis.
- `internal/objectstore/s3.go`: S3-client voor Garage.
- [Datamodel v3](docs/data-model-v3.md).
- [Technische architectuur en vervolgstappen](docs/technical-architecture-v3.md).
- [Documentatie-index](docs/index.md).

De actieve applicatie heeft geen PostgreSQL- of MySQL-migratielijn; SQLite-schema-upgrades verlopen via `PRAGMA user_version`.
