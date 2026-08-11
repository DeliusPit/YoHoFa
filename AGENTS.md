# AGENTS.md

Personal-finance app ("Geldzaken"). Early scaffold: a Docker Compose stack (postgres 18 + MinIO + Go API) with an empty `frontend/`. Not a git repo yet; no tests, CI, or lint setup exist.

## Run the stack

- `.env` is required for compose (variable interpolation). It holds real DB/MinIO credentials in plaintext — never log or commit it; a `.gitignore` for it still needs creating.
- Full stack: `docker compose up --build` (add `-d` for detached).
- Inside compose, the API talks to `postgres:5432` and `minio:9000` via the `geldzaken-backend` network — use these hostnames, not `localhost`, in code.
- MinIO console is exposed on `http://localhost:9001`. The API is on `:8080`. Postgres and MinIO API ports are NOT published to the host; only reachable from other containers.
- Persisted data lives in `./data/postgres` and `./data/minio`. Resetting data = wiping these dirs.

## Backend (`backend/`)

- Go 1.25, module name `geldzaken`, standard library `net/http` only (no framework, no deps in `go.mod` yet).
- Single entrypoint `main.go`; only route is `GET /api/health` returning `{"status":"ok"}`.
- Builds in Docker as a static binary (`CGO_ENABLED=0`) running on distroless nonroot. Local `go run`/`go build` works for the stub but won't have working DB/MinIO without env vars.
- UI-facing strings and log messages are in Dutch (e.g. `log.Println("... luistert op :8080")`) — keep that convention.

## Frontend (`frontend/`)

Empty. No framework or tooling chosen yet; any new frontend work must decide this first.
