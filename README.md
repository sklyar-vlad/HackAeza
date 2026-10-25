# HackAeza

Локальный запуск: `bash scripts/up.sh` → http://aeza.localhost.

Запуск на VPS: `bash scripts/up.sh vps` → https://aeza.fabulosus.online.

Команда создаёт `infra/.env`, собирает контейнеры, применяет миграции и дожидается готовности сервисов. Существующие настройки и данные сохраняются.

Документация: `/docs/`, Swagger UI: `/api/v1/docs`, OpenAPI: `/api/v1/openapi.yaml`.

Для VPS DNS-записи домена должны указывать на сервер, порты 80 и 443 должны быть доступны. Traefik выпускает и обновляет сертификат Let's Encrypt; сертификаты сохраняются в Docker volume. Email для уведомлений можно указать в `ACME_EMAIL` в `infra/.env`.
