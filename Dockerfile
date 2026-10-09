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
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/hohoho-api ./cmd/api

FROM alpine:3.23

RUN apk add --no-cache \
    ca-certificates \
    ocrmypdf \
    poppler-utils \
    rclone \
    restic \
    sqlite \
    tesseract-ocr \
    tesseract-ocr-data-deu \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-fra \
    tesseract-ocr-data-nld
WORKDIR /app
COPY --from=api-build /out/hohoho-api /app/hohoho-api
COPY --from=web-build /src/web/dist /app/web/dist
COPY config/banks /app/config/banks
COPY config/translations /app/config/translations
COPY config/ai-docs.yaml /app/config/ai-docs.yaml

EXPOSE 8080
ENTRYPOINT ["/app/hohoho-api"]
