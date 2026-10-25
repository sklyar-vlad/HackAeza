#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

if [[ ! -f infra/.env ]]; then
    if [[ -f .env ]]; then
        mv .env infra/.env
    else
        cp infra/.env.example infra/.env
    fi
fi

ensure_env() {
    local auth_env_key="$1" auth_env_value="${2:-}"
    if grep -Eq "^${auth_env_key}=.+$" infra/.env; then
        return
    fi
    if [[ -z "$auth_env_value" ]]; then
        auth_env_value=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
    fi
    auth_env_tmp=$(mktemp infra/.env.tmp.XXXXXX)
    trap 'rm -f "$auth_env_tmp"' EXIT
    while IFS= read -r auth_env_line || [[ -n "$auth_env_line" ]]; do
        if [[ "$auth_env_line" != "${auth_env_key}="* ]]; then
            printf '%s\n' "$auth_env_line"
        fi
    done < infra/.env > "$auth_env_tmp"
    printf '%s=%s\n' "$auth_env_key" "$auth_env_value" >> "$auth_env_tmp"
    mv "$auth_env_tmp" infra/.env
    trap - EXIT
}

ensure_env POSTGRES_PASSWORD
ensure_env ADMIN_USERNAME admin
ensure_env ADMIN_PASSWORD
printf 'infra/.env готов. Данные входа: ADMIN_USERNAME и ADMIN_PASSWORD.\n'
