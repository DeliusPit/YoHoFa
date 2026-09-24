# Bankimport

Bankgegevens importeren bestaat meestal uit twee stappen.

1. In de bankomgeving exporteer je transacties als CSV of, voor ICS, als PDF-afschrift.
2. In HoHoHo upload je het bestand en controleer je eerst een preview voordat gegevens worden opgeslagen.

De applicatie ondersteunt preview, definitieve import en duplicate-detectie.

## Ondersteunde profielen

- `generic`: generiek CSV-formaat voor testen
- `ing`: Nederlandse ING CSV-export
- `rabobank`: Rabobank CSV-export
- `abnamro`: ABN AMRO CSV-export
- `asn_sns_regiobank`: ASN, SNS en RegioBank-achtige CSV-exporten
- `sparda`: Duitse Sparda CSV-export
- `triodos`: Triodos CSV-export zonder headerregel
- `ics`: PDF-afschrift van International Card Services

De profielen zijn tolerant opgezet en zoeken naar bekende kolomnamen voor datum, bedrag, omschrijving, tegenpartij, IBAN en referentie. Echte bankexports kunnen per bank en taalinstelling verschillen; daarom is preview de eerste stap.

Het ICS-profiel gebruikt de boekingsdatum uit het PDF-afschrift. `Af` wordt als negatief bedrag en `Bij` als positief bedrag geïmporteerd. Het jaar wordt afgeleid van de afschriftdatum, inclusief een jaarovergang van januari naar december.

## Lokaal testen

Start de API en frontend:

```sh
./scripts/dev-api.sh
cd web
npm run dev
```

Gebruik daarna in de frontend het profiel `generic` met:

```text
samples/bankimport/generic.csv
```

Voor Triodos kun je het fictieve voorbeeldbestand gebruiken met profiel `triodos`:

```text
samples/bankimport/triodos-example.csv
```

Of test direct via curl:

```sh
curl -F profile=generic -F file=@samples/bankimport/generic.csv http://localhost:8080/api/v1/imports/preview
```

## Nog niet actief

- CAMT.053 of MT940 parser
- PSD2/Open Banking koppeling

Voor Nederlandse en Duitse banken is handmatige export/import meestal het simpelste begin. PSD2-koppelingen kunnen later, maar vragen registratie, consent-flow en vaak een provider of bank-specifieke toegang.
