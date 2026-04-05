# usdt-rates

gRPC-сервис: тянет стакан USDT с Grinex, считает bid/ask (topN, avg по уровням), пишет снимок в PostgreSQL на **каждый** вызов `GetRates`. HTTP: health + Prometheus metrics. Опционально: OTLP-трейсы (Jaeger), логи в Docker → Loki через Promtail.

## Запуск (Docker)

1. `cp postgres.env.example postgres.env`
2. `cp app.env.example app.env` — в `app.env`
3. `docker compose up -d`
4. `docker compose --profile tools run --rm migrator` если миграции не прошли то  `make install-goose` `make migrate-up`

Локальная сборка: `make build` · тесты: `make test` · линтер: `make lint` · образ: `make docker-build` · локальный run: `make run` (нужен свой Postgres и те же env). **Kubernetes / Helm** — в отдельном репозитории **usdt-rates-helm** https://github.com/srg206/usdt-rates-helm

x## Функционал

- **GetRates** — один gRPC-метод: запрос к Grinex (resty), расчёт topN / avgNM, INSERT в `rate_snapshots`, ответ с ценами и `exchange_time`.
- **Health** — `GET /healthz/live` (процесс), `GET /healthz/ready` (Postgres + доступность depth API).
- **Graceful shutdown** — SIGTERM/SIGINT, остановка gRPC и HTTP с таймаутом `SHUTDOWN_TIMEOUT`.
- **Конфиг** — переменные окружения (обязательные см. `app.env.example`) и флаги CLI с теми же смыслами (`-grpc-addr`, `-postgres-url`, …); флаги перекрывают env после `flag.Parse`. Параллель к бирже: `GRINEX_MAX_CONCURRENT` / `-grinex-max-concurrent` (семафор). **Circuit breaker** (`pkg/circuitbreaker`, Sony gobreaker) вокруг HTTP к Grinex: `GRINEX_CB_ENABLED`, `GRINEX_CB_CONSECUTIVE_FAILURES`, `GRINEX_CB_OPEN_TIMEOUT`, `GRINEX_CB_HALF_OPEN_MAX`, `GRINEX_CB_INTERVAL` — см. `app.env.example` и флаги `-grinex-cb-*`.

## Где смотреть

| Что | Адрес / команда |
|-----|-----------------|
| **gRPC** | `localhost:50051`, сервис `rates.v1.RatesService`, метод `GetRates`, тело `{}`. Reflection нет — нужен `-proto`. Пример:<br><br>`grpcurl -plaintext -import-path proto -import-path "$(brew --prefix protobuf)/include" -proto rates/v1/rates.proto -d '{}' localhost:50051 rates.v1.RatesService/GetRates`<br>(Linux: второй путь часто `/usr/include`). |
| **Метрики** | `http://localhost:8080/metrics` (Prometheus scrape в `deploy/prometheus/prometheus.yml` → UI `http://localhost:9090`). |
| **Логи** | `docker logs -f app` (JSON); при полном compose — Grafana `http://localhost:3000`, datasource Loki. |
| **Grafana (PostgreSQL)** | Дашборд [**USDT Rates — database (snapshots)**](http://localhost:3000/d/usdt-rates-db) — bid/ask из таблицы `rate_snapshots` (история с биржи Grinex). Данные появляются **только после** вызовов gRPC `GetRates`: каждый запрос пишет снимок; если к приложению ещё не обращались, графики пустые. Есть ещё техдашборд Prometheus: `USDT Rates — technical` (http://localhost:3000/d/usdt-rates-technical). |
| **Трейсы** | Jaeger UI `http://localhost:16686` при заданном `OTEL_COLLECTOR_URL` (в compose по умолчанию `jaeger:4318` внутри сети). Без коллектора трейсы не шлются. |
| **Health** | `http://localhost:8080/healthz/live`, `http://localhost:8080/healthz/ready`. |
| **Переменные и флаги** | в `app.env.example` и `config/config.go` / `-help` у бинарника. |
