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

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=api-build /out/geldzaken-api /app/geldzaken-api
COPY --from=web-build /src/web/dist /app/web/dist

EXPOSE 8080
ENTRYPOINT ["/app/geldzaken-api"]
