# Scripts

Actueel voor de v3-basis:

- `check-local.sh`: start de v3-API met een tijdelijke lege SQLite-database, controleert objecten/waarden/relaties, transacties, dashboard en persistentie na een herstart. Het gebruikt bewust geen Garage, externe AI of backuprepository.
- `check-garage-s3.sh`: controleert een live S3-compatible/Garage-configuratie via de objectstore-client.
- `dev-api.sh`: start de API lokaal voor ontwikkeling.
- `build.sh`: bouwt lokale Go/frontend artifacts wanneer Go en Node lokaal beschikbaar zijn.

De scripts controleren uitsluitend de actieve SQLite/S3-runtime; v2-runtime- en multi-backendcontroles zijn verwijderd.

Gebruik voor de actuele centrale deployment:

```sh
docker compose -f compose.yml up --build -d
```
