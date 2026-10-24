# Frontend

Панель рисков оттока: React 19, TypeScript, Vite. Внешний вид повторяет личный кабинет my.aeza.net.

## Запуск

```bash
npm ci
npm run dev            # http://localhost:5173
```

Dev-сервер проксирует запросы так же, как `nginx.conf` в проде:

| Путь                              | Куда                                                                    |
| --------------------------------- | ----------------------------------------------------------------------- |
| `/api/*`                          | бэкенд, `API_URL` (по умолчанию `http://localhost:8081`)                |
| `/api/v1/docs`                    | редирект на `/docs/swagger.html`                                        |
| `/docs/*`, `/api/v1/openapi.yaml` | контейнер swagger, `SWAGGER_URL` (по умолчанию `http://localhost:8080`) |

Адреса меняются переменными окружения: `API_URL=http://host:8081 npm run dev`. Бэкенд и swagger поднимаются из корня репозитория командой `task up`. Если они не запущены, Vite пишет в консоль `ECONNREFUSED`, это ожидаемо.

## Скрипты

| Команда                              | Что делает                                        |
| ------------------------------------ | ------------------------------------------------- |
| `npm run dev`                        | dev-сервер                                        |
| `npm run build`                      | проверка типов и production-сборка в `dist/`      |
| `npm run api`                        | перегенерировать клиент из `../docs/openapi.yaml` |
| `npm run lint` / `npm run typecheck` | проверки TypeScript                               |
| `npm run test:ci`                    | тесты Vitest                                      |
| `npm run format`                     | Prettier                                          |

## API

Клиент генерируется [orval](https://orval.dev) из контракта `docs/openapi.yaml` в `src/shared/api/generated/`: типы, хуки TanStack Query и zod-схемы. Эти файлы руками не правят. После изменения контракта нужно выполнить `npm run api` и закоммитить результат.

Запросы проходят через `src/shared/api/http.ts`: он добавляет Bearer-токен сессии и при ответе 401 сбрасывает её. Изменение настроек и загрузка снимка отвечают `202` с `jobId`. Хук `useJob` (`src/entities/job`) опрашивает `/api/v1/jobs/{jobId}` и после публикации обновляет данные. Поток `/api/v1/events` (SSE) необязателен: если его нет, данные обновляются при возврате на вкладку.

## Структура

Облегчённый Feature-Sliced Design:

```
src/
  app/       точка входа, роутер (экраны грузятся лениво), глобальные стили и токены
  pages/     экраны: login, overview, customers, customer, model, source
  widgets/   каркас с навигацией, плашка cookies
  entities/  уровни риска, фоновые задания
  shared/    api (http + generated), ui (иконки, dropdown, логотип), lib (форматирование), model (сессия, тема)
```

Цвета, радиусы и анимации заданы CSS-переменными в `src/app/styles.css` (светлая и тёмная тема через `data-theme`). Тема, согласие на cookies и сессия хранятся в Zustand с `persist`.
