#!/usr/bin/env bash
set -euo pipefail

# Usage: bash scripts/check-docs.sh [frontend URL] [standalone docs URL]
frontend_url="${1:-http://localhost:3000}"
docs_url="${2:-http://localhost:8080}"

check() {
    local method="$1" url="$2" expected_status="$3" expected_body="$4"
    local response status body
    response=$(curl --silent --show-error --location --max-time 20 \
        --request "$method" --write-out $'\n%{http_code}' "$url")
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    if [[ "$status" != "$expected_status" || "$body" != *"$expected_body"* ]]; then
        printf 'FAIL %s %s: expected HTTP %s and body containing %s; got HTTP %s\n%s\n' \
            "$method" "$url" "$expected_status" "$expected_body" "$status" "$body" >&2
        return 1
    fi
    printf 'OK %s %s -> %s\n' "$method" "$url" "$status"
}

check GET "${frontend_url%/}/" 200 '<html'
for origin in "${frontend_url%/}" "${docs_url%/}"; do
    if [[ "$origin" == "${frontend_url%/}" ]]; then
        docs_path=/docs
    else
        docs_path=
    fi
    check GET "$origin$docs_path/" 200 'reference.js'
    check GET "$origin$docs_path/swagger.html" 200 'swagger-ui-bundle.js'
    check GET "$origin$docs_path/openapi.yaml" 200 'openapi: 3.0.3'
    check GET "$origin$docs_path/spec.json" 200 'getLiveness'
    check GET "$origin/api/v1/docs" 200 'swagger-ui-bundle.js'
    check GET "$origin/api/v1/openapi.yaml" 200 'openapi: 3.0.3'
    check GET "$origin/healthz" 200 '"status":"ok"'
    # Missing API routes must reach Go and return its 404, never the React page.
    check GET "$origin/api/v1/__docs_smoke_missing__?page=2" 404 '404 page not found'
    check POST "$origin/api/v1/__docs_smoke_missing__" 404 '404 page not found'
done
printf '\nDocumentation HTTP checks passed. Also run the browser checks in README.md.\n'
