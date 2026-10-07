# Functioneel Ontwerp

> **Historische v2-functies.** De actieve v3-basis gebruikt [negen kerntabellen](data-model-v3.md) en de [SQLite/Garage-browserarchitectuur](technical-architecture-v3.md). Objecten, waarden en relaties werken; de overige v3-flows volgen later. Eén deployment/database is één huishouden.

Dit document beschrijft de functionele opzet van HoHoHo. Het doel is een geïntegreerde huishoudadministratie waarin banktransacties, documenten, bezittingen, schulden, reserveringen en dossiers met elkaar verbonden kunnen worden.

## Productvisie

HoHoHo is geen klassieke boekhoudapplicatie en ook geen simpele documentenmap. De kern is integratie: een belastingdossier kan bewijsdocumenten, transacties, schulden en bezittingen bij elkaar brengen; een woningbezitting kan waarderingen, contracten, financiering, onderhoudstransacties en documenten tonen; een schuld kan gekoppeld zijn aan een schuldeiser, transacties en bewijsstukken.

## Gebruikersrollen

De huidige applicatie gaat functioneel uit van één huishouden en één beheeromgeving. Authenticatie beschermt de productieomgeving, maar er zijn nog geen aparte rollen zoals beheerder, lezer of gezinslid.

Toekomstige multi-user-functionaliteit moet rekening houden met bestaande `household_id`-velden, maar is nog geen functioneel onderdeel.

## Dashboard

Het dashboard geeft een compacte financiële status.

Functionele doelen:

- Snel inzicht in totale bezittingen, schulden en nettovermogen.
- Direct doorklikken naar de vermogenssecties.
- Savings accounts als vermogenscomponent tonen.
- Reserveringen apart zichtbaar maken als toekomstige verplichtingen.
- Transacties kunnen bekijken en filteren vanuit de dashboardcontext.

Belangrijke keuzes:

- Bedragen worden per valuta getoond; er is geen automatische valutaconversie.
- Alleen actieve bezittingen en schulden tellen mee in het nettovermogen.
- Reserveringen verminderen het nettovermogen niet automatisch; ze zijn een aparte planningcategorie.

## Bankrekeningen

Bankrekeningen leggen de bron van transacties vast.

Gebruikersfuncties:

- Rekeningen aanmaken, wijzigen en verwijderen.
- Naam, IBAN, eigenaar, banknaam, rekeningtype en importprofiel vastleggen.
- Opening balance vastleggen.
- Rekeningtype gebruiken voor filtering en dashboardinterpretatie.

Validatie:

- Een rekening die transacties bevat kan niet zomaar verwijderd worden.
- IBAN is functioneel optioneel, omdat niet iedere bron of testimport een IBAN bevat.
- Het importprofiel bepaalt hoe bankbestanden geïnterpreteerd worden.

## Categorieen

Categorieen structureren inkomsten en uitgaven.

Functionele structuur:

- Hoofdcategorieen groeperen subcategorieen.
- Subcategorieen hebben `kind`: `income`, `expense` of `transfer`.
- Subcategorieen hebben `purpose`: bijvoorbeeld `daily`, `reservation` of `investment`.
- Hoofdcategorieen krijgen automatisch een gekoppeld dossier voor administratieve samenhang.

Gebruikersfuncties:

- Hoofdcategorieen beheren.
- Subcategorieen beheren.
- Categorieen aan transacties koppelen.
- Categorieen gebruiken in rapportages.

Belangrijke keuzes:

- Categorieen zijn huishoudelijke betekenis, geen grootboekrekeningen.
- `kind` voorkomt dat inkomsten per ongeluk als uitgaven behandeld worden.
- `purpose` maakt onderscheid tussen dagelijkse uitgaven, reserveringen en investeringen zonder aparte transactietabellen.

## Bankimport

Bankimport verwerkt handmatige bankexports.

Gebruikersflow:

1. Gebruiker kiest een rekening en importprofiel.
2. Gebruiker uploadt CSV of, voor ICS, PDF.
3. De applicatie toont een preview.
4. Gebruiker controleert aantallen, datums, bedragen en omschrijvingen.
5. Definitieve import slaat nieuwe transacties op.
6. Duplicates worden op basis van import fingerprint overgeslagen.

Ondersteunde profielen:

- `generic`
- `ing`
- `rabobank`
- `abnamro`
- `asn_sns_regiobank`
- `sparda`
- `triodos`
- `ics`

Belangrijke keuzes:

- Preview schrijft niets naar de database.
- Definitieve import mag bestaande transacties niet dubbel invoeren.
- Ruwe bankvelden blijven bewaard voor controle en latere verbetering.
- Tegenpartijen worden genormaliseerd naar parties, maar raw waarden blijven beschikbaar.

## Transacties

Transacties zijn feitelijke bankmutaties.

Gebruikersfuncties:

- Transacties bekijken en filteren.
- Filteren op rekening, categorie, hoofdcategorie, jaar, maand, zoektekst en ongecategoriseerd.
- Handmatige transacties aanmaken.
- Transacties categoriseren.
- Categorie toepassen op één transactie of op dezelfde tegenpartij.
- Transacties verbergen uit overzichten.
- Verborgen transacties herstellen.
- Documenten koppelen als bewijs.
- Transacties koppelen aan bezittingen, schulden en dossiers.

Belangrijke keuzes:

- Een transactie kan aan meerdere domeinobjecten gerelateerd zijn.
- Categorisatie is niet hetzelfde als relatiebeheer: een onderhoudsbetaling kan categorie `Onderhoud` hebben en daarnaast aan een woningbezitting gekoppeld zijn.
- Verbergen verwijdert transacties niet.

## Rapportages

Rapportages geven inzicht in uitgaven en inkomsten.

Huidige rapporten:

- Categorie per maand.
- Hoofdcategorie per periode.
- Transactiesamenvatting voor filters.

Belangrijke keuzes:

- Rapportages gebruiken gecategoriseerde transacties.
- Verborgen transacties worden buiten normale rapportage gehouden waar de store dat filter toepast.
- Rapportages zijn informatief en schrijven geen data.

## Documenten

Documenten bestaan uit metadata en een origineel object.

Gebruikersfuncties:

- Document uploaden.
- Documentmetadata vastleggen: bestandsnaam, MIME-type, grootte, checksum, datum en omschrijving.
- Documentinhoud openen/downloaden.
- Documenten koppelen aan transacties, bezittingen, schulden, dossiers en reserveringen.
- Automatische PDF-documentanalyse bekijken wanneer de centrale PDF-AI-service is geconfigureerd.

Belangrijke keuzes:

- De relationele database is bron van waarheid voor metadata en relaties.
- Het originele document blijft in objectopslag en wordt niet vervangen door AI-output.
- Documentanalyse wordt apart opgeslagen in `document_analyses` en compatibel getoond via bestaande documentvelden.
- PDF-analyse loopt via de centrale PDF-AI-service; die verzorgt tekstextractie en OCR voor gewone en gescande PDF's.

## Parties en Tegenpartijen

Parties zijn genormaliseerde personen of organisaties.

Functionele doelen:

- Banktegenpartijen over transacties heen herkennen.
- Schuldeisers van liabilities normaliseren.
- IBAN's als identifiers aan parties koppelen.
- Later betere AI-classificatie en rapportage op partijen mogelijk maken.

Belangrijke keuzes:

- Raw banknaam en raw IBAN blijven beschikbaar.
- Party-koppeling is ondersteunend en mag originele importdata niet overschrijven.
- Parties worden per household gescoped.

## Bezittingen

Bezittingen zijn zelfstandige domeinobjecten zoals woning, voertuig, spaarrekening-achtige representatie of andere waardevolle objecten.

Gebruikersfuncties:

- Bezitting aanmaken, wijzigen en verwijderen.
- Type, status, aanschafdatum, afvoerdatum, aankoopwaarde, valuta en omschrijving vastleggen.
- Waarderingen registreren.
- Contracten koppelen of openen.
- Documenten koppelen.
- Transacties koppelen.
- Relatie leggen met schulden via `financed_by`.
- Tijdlijn bekijken.

Belangrijke keuzes:

- Huidige waarde is cache van de nieuwste waardering.
- Valuta wordt per bezitting bewaard en niet automatisch geconverteerd.
- Een bezitting kan bestaan zonder transacties of documenten.

## Schulden

Schulden leggen verplichtingen en financieringen vast.

Gebruikersfuncties:

- Schuld aanmaken, wijzigen en verwijderen.
- Type, naam, schuldeiser, looptijd, oorspronkelijk bedrag, huidig saldo, valuta en status vastleggen.
- Schuldeiser normaliseren naar `parties`.
- Documenten koppelen.
- Transacties koppelen.
- Relaties met bezittingen en dossiers leggen.
- Tijdlijn bekijken.

Belangrijke keuzes:

- De oude tekstuele schuldeiser blijft bestaan voor compatibiliteit.
- `creditor_party_id` geeft de genormaliseerde partij weer.
- Geplande schulden blijven zichtbaar in de lijst wanneer de gebruiker alle statussen bekijkt.

## Reserveringen

Reserveringen leggen toekomstige verplichtingen vast zonder ze al als echte schuld te behandelen.

Gebruikersfuncties:

- Reservering aanmaken, wijzigen en verwijderen.
- Type, naam, verwachte datum, bedrag, valuta en omschrijving vastleggen.
- Documenten koppelen.
- Tijdlijn bekijken.
- Reserveringen op dashboard apart tonen.

Belangrijke keuzes:

- Reserveringen zijn planning, geen geboekte banktransactie.
- Reserveringen tellen niet automatisch als liability.
- Een reservering kan later handmatig worden omgezet of gekoppeld aan daadwerkelijke transacties via toekomstige functionaliteit.

## Dossiers

Dossiers groeperen administratieve context.

Gebruikersfuncties:

- Dossier aanmaken, wijzigen en verwijderen.
- Vrije case type-classificatie gebruiken.
- Status gebruiken: `planned`, `active`, `completed`, `archived`.
- Startdatum, einddatum en looptijd vastleggen.
- Documenten koppelen.
- Transacties koppelen.
- Relaties met bezittingen, schulden en transacties leggen.
- Tijdlijn bekijken.

Belangrijke keuzes:

- Case type is vrij en niet beperkt tot vaste workflowtypes.
- Status is eenvoudige lifecycle-status, geen workflow engine.
- Hoofdcategorieen kunnen een beheerd dossier hebben dat niet als normaal handmatig dossier verwijderd mag worden.

## Domeinrelaties

Domeinrelaties maken integratie tussen objecten mogelijk.

Ondersteunde semantiek:

- Document `evidence_for` transactie, bezitting, schuld, dossier of reservering.
- Asset `financed_by` liability.
- Case `relates_to` transactie, asset of liability.
- Asset/liability `relates_to` transactie.

Belangrijke keuzes:

- Relaties worden expliciet gevalideerd door applicatiecode.
- Delete-operaties blokkeren wanneer relevante relaties of documentlinks bestaan.
- Relaties vervangen categorieen niet; ze voegen context toe.

## Domeingebeurtenissen

Domeingebeurtenissen vormen een gebruikersgerichte tijdlijn.

Gebeurtenissen ontstaan bij:

- Aanmaken en wijzigen van bezittingen.
- Waarderingen van bezittingen.
- Aanmaken en wijzigen van schulden.
- Statuswijzigingen van schulden.
- Aanmaken en wijzigen van dossiers.
- Statuswijzigingen van dossiers.
- Aanmaken en wijzigen van reserveringen.

Belangrijke keuzes:

- Geen technische auditlog.
- Geen event sourcing.
- Geen handmatige API om events te muteren.

## Systeemconfiguratie

Systeemconfiguratie beheert runtimekeuzes.

Gebruikersfuncties:

- Opslagprofiel kiezen: `local`, `server`, `central`.
- Databasebackend configureren: SQLite, PostgreSQL of MySQL.
- Objectopslag configureren: filesystem of S3-compatible.
- PDF-AI-endpoint configureren voor documentanalyse.
- Lokale LLM-endpoint configureren voor toekomstige transactieclassificatie.
- Opties voor documentanalyse en toekomstige transactieclassificatie beheren.

Belangrijke keuzes:

- Secrets worden niet teruggegeven aan de frontend, alleen status `set/not set`.
- Wijzigingen worden opgeslagen in `config/storage.json` en gelden na herstart.
- Configuratiewijzigingen migreren geen data.

## Systeembackup en Herstel

Systeembackup maakt een herstelbare snapshot.

Gebruikersfuncties:

- Backupconfiguratie opslaan en valideren.
- Handmatig backup draaien.
- Snapshots bekijken.
- Herstel starten via webinterface.
- Noodherstel via `/app/hohoho-recover` wanneer de webinterface niet start.

Belangrijke keuzes:

- Restic verzorgt encryptie.
- Rclone verzorgt opslagtoegang.
- Backup bevat actieve database, objecten en configuratie.
- Herstel maakt standaard eerst een veiligheidsbackup.

## AI-functies

Huidig actief:

- PDF-documentanalyse via de centrale PDF-AI-service.

Ontworpen maar nog niet actief:

- AI-suggesties voor banktransactieclassificatie. Zie [AI bij bankimport](ai-bankimport.md).

Belangrijke keuzes:

- AI-output is ondersteuning, geen bron van waarheid.
- AI mag originele data niet stilzwijgend overschrijven.
- Lokale LLM is het uitgangspunt.
