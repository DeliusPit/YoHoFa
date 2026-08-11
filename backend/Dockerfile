FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /geldzaken-api .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /geldzaken-api /geldzaken-api

EXPOSE 8080

ENTRYPOINT ["/geldzaken-api"]
