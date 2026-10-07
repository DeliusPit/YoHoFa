# Scripts

Actueel voor de v3-basis:

- `check-local.sh`: start de v3-API met een tijdelijke lege SQLite-database, controleert objecten/waarden/relaties, handmatige transacties en herstart/persistentie.
- `check-garage-s3.sh`: controleert een live S3-compatible/Garage-configuratie via de objectstore-client.
- `dev-api.sh`: start de API lokaal voor ontwikkeling.
- `build.sh`: bouwt lokale Go/frontend artifacts wanneer Go en Node lokaal beschikbaar zijn.

`check-runtime.sh` en `check-storage-parity.sh` beschrijven nog v2-routes/backends. Ze worden in de vervolgstappen aangepast of opgeruimd; gebruik ze niet als v3-acceptatiecheck.

Gebruik voor de actuele centrale deployment:

```sh
docker compose -f compose.yml up --build -d
```
