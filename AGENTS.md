# AGENTS.md

Personal-finance app ("Geldzaken").

The active application is the root Go module with `cmd/api` as the backend entrypoint and `web/` as the Vite frontend.

The application currently runs with SQLite. Docker Compose also starts PostgreSQL and MinIO, but the application does not use them yet.

## Current architecture

- `cmd/api` is the real backend entrypoint.
- `internal/` contains the application logic, HTTP server, configuration, imports and stores.
- `web/` is the real frontend.
- `migrations/` contains PostgreSQL migrations.
- `compose.yml` is the current Docker Compose runtime definition.
- `backend/` is an older temporary stub and is not the current application backend.
- `frontend/` is not used by the current application.
- SQLite is the current working database backend.
- PostgreSQL is available but not yet functionally complete.
- MinIO is available but not yet used by application code.

## Run the stack

- `.env` is required for Compose variable interpolation and may contain secrets. Never log or commit it.
- Start or rebuild the full stack with:

  `docker compose up --build`

- Run detached with:

  `docker compose up --build -d`

- Compose builds the real application from the repository-root `Dockerfile`, not `backend/Dockerfile`.
- The API serves the built frontend from `WEB_DIR` on port `8080`.
- PostgreSQL and MinIO are available on the `geldzaken-backend` Docker network.
- While `STORE_BACKEND=sqlite`, PostgreSQL and MinIO are not used by the application.
- Persistent runtime data is stored outside the application container.

## Backend

- `cmd/api` is the real backend entrypoint.
- Keep application logic under `internal/`.
- Preserve existing package boundaries unless there is a concrete reason to change them.
- Keep UI-facing strings and log messages in Dutch where that convention already exists.
- Do not introduce a second application backend.
- Do not move active application logic into `backend/`.

## Frontend

- `web/` is the real frontend.
- The frontend is built with Vite.
- Production frontend assets are served by the Go application.
- Do not create a separate frontend application under `frontend/`.
- Do not change frontend behavior unless required by the task.

## Database

- SQLite is the current working baseline.
- The application must remain functional with SQLite until the PostgreSQL migration is complete.
- PostgreSQL is the target relational database.
- Do not remove SQLite support until PostgreSQL provides equivalent functionality and the migration has been verified.
- PostgreSQL schema changes belong in `migrations/`.
- Keep application store behavior consistent across database implementations while both backends exist.
- Do not silently change database semantics, constraints or seed behavior.

## Documents and object storage

- MinIO is the target object store for documents and other binary objects.
- PostgreSQL must remain the system of record for document metadata, relationships and application state.
- Do not encode business meaning in MinIO paths.
- Store object identifiers and metadata in PostgreSQL.
- Do not introduce MinIO application logic until a concrete document-storage requirement is being implemented.

## Architecture direction

- Migrate incrementally.
- Leave the application in a working state after every migration step.
- Separate deployment changes from functional/data-model migrations where practical.
- Prefer a modular monolith over microservices.
- Do not introduce additional infrastructure components unless they solve a concrete requirement.
- Prefer PostgreSQL capabilities before introducing additional databases, search engines or queues.
- Use application events conceptually where useful, but do not introduce a message broker without a concrete need.
- Keep PostgreSQL as the authoritative structured-data store.
- Keep MinIO as binary object storage, not as a metadata store.

## Domain direction

The application is intended to become an integrated household administration system.

Core domains include:

- assets;
- liabilities;
- transactions;
- documents;
- bank imports and categorisation;
- tax-related data;
- depreciation;
- reservations and future obligations.

The distinguishing principle is integration between these domains.

Documents, transactions, assets, liabilities, obligations and tax records should be linkable to each other where relevant.

Avoid designing the system purely around bank transactions or bookkeeping accounts.

Prefer domain objects and explicit relationships over folder-based document organisation.

A document may relate to multiple domain objects.

A transaction may relate to multiple relevant objects or administrative contexts.

## Docker and deployment

- `compose.yml` is the current Docker Compose definition.
- Build application images from the repository root.
- Runtime containers should not contain build toolchains unless required.
- Keep persistent data outside disposable application containers.
- Do not expose PostgreSQL or MinIO externally unless explicitly required.
- Prefer Docker internal networking between application services.
- Do not install application runtimes or databases directly on the Debian Docker host when they belong in containers.

## Secrets

- Never commit `.env`.
- Never commit credentials, API keys, database passwords or tokens.
- Never print secrets in logs, diffs, command output or responses.
- Use environment variables or the existing runtime configuration mechanism for secrets.
- Treat Gitea, OpenAI and other service credentials as secrets.

## Git policy

- Inspect `git status` and `git diff` before committing changes.
- Commit only files that belong to the implemented change.
- Do not make commits unless explicitly instructed.
- Do not push unless explicitly instructed.
- Never use force push.
- Do not rewrite existing history unless explicitly instructed.
- Do not delete branches or tags unless explicitly instructed.
- Do not change Git remotes unless explicitly instructed.

## Change discipline

Before making changes:

- inspect the relevant existing implementation;
- identify the current runtime behavior;
- determine which files actually participate in that behavior;
- avoid modifying unrelated files.

During changes:

- prefer the smallest coherent change;
- preserve working behavior outside the requested scope;
- do not perform opportunistic refactoring unless necessary;
- do not remove temporary compatibility code until its replacement is complete.

After changes:

- run relevant build or test commands;
- verify application startup;
- verify the changed functionality;
- inspect `git diff`;
- report any remaining limitations or known incomplete migration work.

## Safety constraints for migrations

For SQLite-to-PostgreSQL migration work:

- do not remove the SQLite implementation first;
- first achieve functional parity in PostgreSQL;
- keep schema migration separate from data migration where practical;
- verify transaction import, categorisation, reports and existing queries before switching the default backend;
- preserve data during migration;
- do not assume PostgreSQL schema parity with SQLite without checking both implementations.

For document-storage migration work:

- do not move documents to MinIO without retaining metadata and referential information;
- avoid duplicate object storage when a relationship can be represented in PostgreSQL;
- preserve checksums and immutable original files where applicable;
- separate original documents from generated derivatives such as previews, OCR text or thumbnails.
