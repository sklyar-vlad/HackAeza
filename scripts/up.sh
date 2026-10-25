#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

compose_files=(-f infra/compose.yaml)
case "${1:-local}" in
    local) ;;
    vps) compose_files+=(-f infra/compose.vps.yaml) ;;
    *) printf 'Usage: %s [local|vps]\n' "$0" >&2; exit 2 ;;
esac

bash scripts/setup-env.sh
exec docker compose --env-file infra/.env "${compose_files[@]}" -p hackaeza \
    up -d --build --wait --wait-timeout 180
