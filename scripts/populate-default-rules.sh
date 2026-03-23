#!/usr/bin/env bash
set -euo pipefail

APP_CONFIG="${APP_CONFIG:-config.yaml}"
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"

export APP_CONFIG
export POSTGRES_HOST

go run ./cmd/seed-default-rules
