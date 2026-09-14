FROM node:24-alpine AS web-build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS api-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/geldzaken-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/geldzaken-recover ./cmd/recover

FROM rclone/rclone:1.75.0 AS rclone

FROM postgres:18

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates mariadb-client restic \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=rclone /usr/local/bin/rclone /usr/local/bin/rclone
COPY --from=api-build /out/geldzaken-api /app/geldzaken-api
COPY --from=api-build /out/geldzaken-recover /app/geldzaken-recover
COPY --from=web-build /src/web/dist /app/web/dist

RUN pg_dump --version && mariadb-dump --version && rclone version && restic version

EXPOSE 8080
ENTRYPOINT ["/app/geldzaken-api"]
