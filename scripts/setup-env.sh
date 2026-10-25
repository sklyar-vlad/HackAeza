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

if ! grep -Eq '^POSTGRES_PASSWORD=.+$' infra/.env; then
    auth_db_password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
    auth_env_tmp=$(mktemp infra/.env.tmp.XXXXXX)
    trap 'rm -f "$auth_env_tmp"' EXIT
    while IFS= read -r auth_env_line || [[ -n "$auth_env_line" ]]; do
        if [[ "$auth_env_line" != POSTGRES_PASSWORD=* ]]; then
            printf '%s\n' "$auth_env_line"
        fi
    done < infra/.env > "$auth_env_tmp"
    printf 'POSTGRES_PASSWORD=%s\n' "$auth_db_password" >> "$auth_env_tmp"
    mv "$auth_env_tmp" infra/.env
    trap - EXIT
fi
printf 'infra/.env готов. Пароль PostgreSQL хранится только в локальном файле.\n'
