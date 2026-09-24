# Scripts

Actueel voor de centrale Docker Compose-runtime:

- `check-runtime.sh`: controleert een draaiende webapp/API op de belangrijkste endpoints.
- `check-storage-parity.sh`: draait store/config/objectstore/httpserver/systembackup-tests; PostgreSQL en MySQL integratietests draaien alleen met `TEST_DATABASE_URL` en `TEST_MYSQL_DSN`.
- `check-garage-s3.sh`: controleert een live S3-compatible/Garage-configuratie via de objectstore-client.
- `dev-api.sh`: start de API lokaal voor ontwikkeling.
- `build.sh`: bouwt lokale Go/frontend artifacts wanneer Go en Node lokaal beschikbaar zijn.

Gebruik voor de actuele centrale deployment:

```sh
docker compose -f compose.yml up --build -d
```
