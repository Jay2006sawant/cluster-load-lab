#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go build -o bin/cluster-load-lab ./cmd/cluster-load-lab

exec ./bin/cluster-load-lab manifest \
  --namespace "${NAMESPACE:-default}" \
  --host "${DB_HOST:?set DB_HOST}" \
  --user "${DB_USER:?set DB_USER}" \
  --driver "${DB_DRIVER:-pgsql}" \
  --threads "${THREADS:-8}" \
  --duration "${DURATION:-30}"
