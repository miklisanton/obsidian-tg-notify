FROM golang:1.25.6 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/obsidian-notify ./cmd/app

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --system --gid 10001 app \
    && useradd --system --uid 10001 --gid 10001 --home-dir /app --shell /usr/sbin/nologin app

WORKDIR /app

COPY --from=build --chown=app:app /out/obsidian-notify /app/obsidian-notify
COPY --chown=app:app migrations /app/migrations

ENV APP_CONFIG=/app/config.yaml

USER app:app

CMD ["/app/obsidian-notify"]
