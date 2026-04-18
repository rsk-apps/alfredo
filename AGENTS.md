# Alfredo

Modular monolith combining **pet-care** (SQLite) into a single Go binary. Replaces independent `pet-care` microservice plus `jarvis-agent` orchestrator.

## Architecture

Hexagonal architecture with strict domain isolation enforced by package boundaries.

```
cmd/server/main.go          — single entry point, wires all dependencies
internal/
  config/                   — unified Viper config
  logger/                   — Zap logger helpers for Echo context
  shared/health/            — shared HealthResult, DependencyStatus types
  gcalendar/                — Google Calendar adapter plus no-op local adapter
  petcare/                  — pet-care domain
    domain/                 — Pet, Vaccine types
    port/                   — repository interfaces only
    service/                — pure CRUD services (no side-effects to other domains)
    adapters/primary/http/  — HTTP handlers: pet, vaccine_handler
    adapters/secondary/sqlite/ — SQLite repositories + migrations (001)
  app/                      — Application Services (Use Cases) — cross-domain orchestration
    vaccine_usecase.go      — vaccine → calendar events
    pet_usecase.go          — pet CRUD + per-pet calendar lifecycle
    treatment_usecase.go    — treatment dose scheduling + calendar events
    health_aggregator.go    — unified /api/v1/health
    ports.go                — narrow interfaces for use cases
```

## Key Design Decisions

- **No EventPublisher**: Pet-care services are pure CRUD. Cross-domain side-effects (Google Calendar writes) happen only in Use Cases in `internal/app/`.
- **Transactional calendar writes**: Calendar failures return an error and roll back pet-care data writes, so reminder state is not lost silently.
- **No-op calendar adapter for local dev**: When Google Calendar credentials are absent, Alfredo logs calendar calls and returns deterministic fake IDs.
- **Handler interfaces unchanged**: HTTP handlers define narrow interfaces. Use Cases implement the same interfaces, so handlers don't change — only the injected dependency changes (service → use case for mutations).
- **Domain isolation**: petcare domain must not import from app/; app/ imports and orchestrates services. This enforces unidirectional dependency.

## Tech Lead Gate

Alfredo has two advisor skills that govern how work flows from idea to merge:

- **`/pm` (Jinx)** — product authority. Sole writer of `docs/stories/`, `docs/VISION.md`, and `docs/pm/MEMORY.md`.
- **`/tl` (Vex)** — tech lead authority. Sole writer of `docs/tl/` and of the `## Tech Lead Review` section on story files.

**No story in `docs/stories/backlog/` enters execution until it has a `## Tech Lead Review` section with `Verdict: APPROVED`.** Before any executing agent begins work on a story, it must read the story file and check that section. If it is missing or the verdict is `CHANGES REQUESTED` / `REJECTED`, the agent stops and invokes `/tl` in Mode A (Story Review). Vex walks `docs/tl/checklists/STORY_REVIEW_DOD.md` out loud, then appends the review block using `docs/tl/templates/STORY_REVIEW_BLOCK.md` and updates the story's `tech_lead_review:` frontmatter field.

Vex also serves as an on-demand advisor. Agents and the user can invoke `/tl` mid-work for architecture review, Go-idiom review, test-quality review, or security review. Vex's memory lives at `docs/tl/MEMORY.md` (index) and `docs/tl/adr/ADR-*.md` (full ADRs). Both are append-only; supersede prior entries by adding new ones, never by editing.

## Routes

| Route | Handler |
|---|---|
| `GET /api/v1/health` | HealthAggregator (sqlite) |
| `GET /api/v1/pets` | PetHandler |
| `GET /api/v1/pets/summary` | SummaryHandler |
| `POST /api/v1/pets` | PetHandler |
| `GET /api/v1/pets/:id` | PetHandler |
| `PUT /api/v1/pets/:id` | PetHandler |
| `DELETE /api/v1/pets/:id` | PetHandler |
| `GET /api/v1/pets/:id/vaccines` | VaccineHandler |
| `POST /api/v1/pets/:id/vaccines` | VaccineHandler |
| `DELETE /api/v1/pets/:id/vaccines/:vid` | VaccineHandler |
| `POST /api/v1/pets/:id/treatments` | TreatmentHandler |
| `GET /api/v1/pets/:id/treatments` | TreatmentHandler |
| `GET /api/v1/pets/:id/treatments/:tid` | TreatmentHandler |
| `DELETE /api/v1/pets/:id/treatments/:tid` | TreatmentHandler |
| `POST /api/v1/pets/:id/observations` | ObservationHandler |
| `GET /api/v1/pets/:id/observations` | ObservationHandler |
| `GET /api/v1/pets/:id/observations/:oid` | ObservationHandler |
| `POST /api/v1/pets/:id/supplies` | SupplyHandler |
| `GET /api/v1/pets/:id/supplies` | SupplyHandler |
| `GET /api/v1/pets/:id/supplies/:sid` | SupplyHandler |
| `PATCH /api/v1/pets/:id/supplies/:sid` | SupplyHandler |
| `DELETE /api/v1/pets/:id/supplies/:sid` | SupplyHandler |

## API Collection

The `bruno/` directory at repo root contains a [Bruno](https://www.usebruno.com/) importable collection covering all 21 routes. It is the **source of truth for route documentation** — keep it in sync whenever routes are added or removed.

```
bruno/
├── bruno.json              — collection metadata
├── environments/Local.bru  — baseUrl + sample UUIDs for local dev
├── Healthcheck.bru
├── pets/                   — 6 requests (CRUD + summary)
├── vaccines/               — 3 requests
├── treatments/             — 4 requests (CRUD)
├── observations/           — 3 requests (create + read)
└── supplies/               — 5 requests (CRUD)
```

**Import**: Open Bruno → Import Collection → select `bruno/` folder → set environment to **Local**.

## Authentication

All routes except `GET /api/v1/health` require an API key. Accepted headers (first match wins):
- `Authorization: Bearer <key>`
- `X-Api-Key: <key>`

> **Gotcha**: The server refuses to start if `APP_AUTH_API_KEY` is empty.

## Development

```bash
make build          # compile ./alfredo binary
make run            # build + run server in background (writes alfredo.pid)
make stop           # kill server using alfredo.pid
make test           # go test ./internal/...
make tidy           # go mod tidy
make generate       # mockery
```

`make run` auto-sources `.env` from the project root if present — use it to set `APP_AUTH_API_KEY` and other vars locally without modifying `config.yaml`.

### Testing

- **Domain service tests** (petcare/service): mock repository interfaces, test CRUD logic in isolation
- **Use case tests** (app/*_test.go): mock domain services, test cross-domain orchestration and calendar operations
- **Handlers**: tested through use case layer; keep handler tests minimal (just wire-up verification)
- **Run tests**: `make test` runs all tests in internal/

### Production

```bash
docker compose -f docker-compose.prod.yml up -d   # uses ghcr.io/rafaelsoares/alfredo
```

## Prerequisites

- Go 1.26+

## Configuration

`config.yaml` at project root, or env vars with `APP_` prefix:

| Key | Default | Env |
|---|---|---|
| `server.host` | `0.0.0.0` | `APP_SERVER_HOST` |
| `server.port` | `8080` | `APP_SERVER_PORT` |
| `database.path` | `./data/alfredo.db` | `APP_DATABASE_PATH` |
| `app.timezone` | `America/Sao_Paulo` | `APP_TIMEZONE` |
| `gcalendar.client_id` | `` | `APP_GCALENDAR_CLIENT_ID` |
| `gcalendar.client_secret` | `` | `APP_GCALENDAR_CLIENT_SECRET` |
| `gcalendar.refresh_token` | `` | `APP_GCALENDAR_REFRESH_TOKEN` |
| `telegram.bot_token` | `` | `APP_TELEGRAM_BOT_TOKEN` |
| `telegram.chat_id` | `` | `APP_TELEGRAM_CHAT_ID` |
| `auth.api_key` | `` | `APP_AUTH_API_KEY` |
| `log.level` | `info` | `APP_LOG_LEVEL` |

## Google Calendar Integration

Alfredo writes pet-care reminder state directly to Google Calendar. Set `APP_GCALENDAR_CLIENT_ID`,
`APP_GCALENDAR_CLIENT_SECRET`, and `APP_GCALENDAR_REFRESH_TOKEN` to enable the real adapter.
When any credential is empty, Alfredo uses the no-op adapter for local development.

Each pet gets its own Google Calendar. Vaccine reminders, finite treatment dose events, and ongoing
treatment recurrence series are written to that pet's calendar. If a calendar write fails, the
corresponding pet-care write is rolled back and the endpoint returns an error.

All user-facing pet-care time fields use `APP_TIMEZONE` for naive datetimes such as
`2026-04-12T12:00:00`. RFC3339 values with an explicit offset keep that offset exactly.
Date-only values are rejected for vaccine and treatment time fields; pet `birth_date` remains
date-only.

### Calendar Operations

| Operation | Trigger | Calendar behavior |
|---|---|---|
| Pet created | `POST /api/v1/pets` | Create pet calendar and store `google_calendar_id` |
| Pet deleted | `DELETE /api/v1/pets/:id` | Delete pet calendar |
| Vaccine recorded | `POST /api/v1/pets/:id/vaccines` | Create vaccine event and store `google_calendar_event_id` |
| Vaccine deleted | `DELETE /api/v1/pets/:id/vaccines/:vid` | Delete vaccine event |
| Finite treatment started | `POST /api/v1/pets/:id/treatments` with `ended_at` | Create one event per dose |
| Finite treatment stopped | `DELETE /api/v1/pets/:id/treatments/:tid` | Delete future dose events |
| Ongoing treatment started | `POST /api/v1/pets/:id/treatments` without `ended_at` | Create recurring event series |
| Ongoing treatment stopped | `DELETE /api/v1/pets/:id/treatments/:tid` | Stop recurring event series |

## Telegram Integration

Alfredo sends pet-care notifications directly to Telegram. Set `APP_TELEGRAM_BOT_TOKEN` and
`APP_TELEGRAM_CHAT_ID` to enable the real Bot API adapter. When either value is empty, Alfredo
uses the no-op adapter for local development and logs calls with `"telegram noop"`.

Telegram notifications are best-effort: failures are logged and swallowed, and the pet-care write
still succeeds. This is deliberately different from Google Calendar because Telegram does not store
integration state that Alfredo must preserve.

Messages are Portuguese, HTML-formatted, and sent for vaccine and treatment create/delete flows.
Pet create/delete does not send Telegram notifications.
