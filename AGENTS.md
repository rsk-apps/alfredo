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
  webhook/                  — HTTP emitter (no-op when base_url is empty)
  petcare/                  — pet-care domain
    domain/                 — Pet, Vaccine types
    port/                   — repository interfaces only
    service/                — pure CRUD services (no side-effects to other domains)
    adapters/primary/http/  — HTTP handlers: pet, vaccine_handler
    adapters/secondary/sqlite/ — SQLite repositories + migrations (001)
  app/                      — Application Services (Use Cases) — cross-domain orchestration
    vaccine_usecase.go      — vaccine → webhook events
    pet_usecase.go          — pet CRUD pass-through
    health_aggregator.go    — unified /api/v1/health
    ports.go                — narrow interfaces for use cases
```

## Key Design Decisions

- **No EventPublisher**: Pet-care services are pure CRUD. Cross-domain side-effects (webhook events to n8n) happen only in Use Cases in `internal/app/`.
- **Fire-and-forget webhooks**: Webhook failures are logged and swallowed. Pet-care data always saves.
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

## API Collection

The `bruno/` directory at repo root contains a [Bruno](https://www.usebruno.com/) importable collection covering all 13 routes. It is the **source of truth for route documentation** — keep it in sync whenever routes are added or removed.

```
bruno/
├── bruno.json              — collection metadata
├── environments/Local.bru  — baseUrl + sample UUIDs for local dev
├── Healthcheck.bru
├── pets/                   — 5 requests (CRUD)
├── care/vaccines/          — 3 requests
└── treatments/             — 4 requests (CRUD)
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
- **Use case tests** (app/*_test.go): mock domain services, test cross-domain orchestration and webhook emission
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
| `webhook.base_url` | `` | `APP_WEBHOOK_BASE_URL` |
| `webhook.api_key` | `` | `APP_WEBHOOK_API_KEY` |
| `auth.api_key` | `` | `APP_AUTH_API_KEY` |
| `log.level` | `info` | `APP_LOG_LEVEL` |

## Webhook (n8n integration)

Alfredo emits fire-and-forget domain events to n8n on every mutation. Set `webhook.base_url`
(`APP_WEBHOOK_BASE_URL`) to your n8n instance's webhook base URL (e.g. `http://localhost:5678/webhook`).
Leave empty to disable — pet-care data always saves regardless.

All events are posted to `POST {base_url}/events`. The n8n workflow uses a Switch node on
`{{ $json.event }}` to route to independent sub-workflows.

### Event Envelope

```json
{
  "event": "vaccine.taken",
  "occurred_at": "2026-03-27T10:00:00Z",
  "domain": "petcare",
  "payload": { ...event-specific fields... }
}
```

### Event Catalog

| Event | Trigger | Key payload fields |
|---|---|---|
| `pet.created` | `POST /api/v1/pets` | `id`, `name`, `species`, `breed`, `birth_date` |
| `pet.deleted` | `DELETE /api/v1/pets/:id` | `pet_id`, `pet_name` |
| `vaccine.taken` | `POST /api/v1/pets/:id/vaccines` | `pet_id`, `pet_name`, `vaccine_id`, `vaccine_name`, `date`, `recurrence_days` (omitted if not set) |
| `vaccine.deleted` | `DELETE /api/v1/pets/:id/vaccines/:vid` | `pet_id`, `pet_name`, `vaccine_id` |
| `treatment.doses_scheduled` | `POST /api/v1/pets/:id/treatments` and daily extender job | `pet_id`, `pet_name`, `treatment_id`, `treatment_name`, `dosage_amount`, `dosage_unit`, `route`, `interval_hours`, `doses` (array of `{dose_id, scheduled_for}`) |
| `treatment.stopped` | `DELETE /api/v1/pets/:id/treatments/:tid` | `pet_id`, `pet_name`, `treatment_id`, `treatment_name`, `stopped_at`, `deleted_dose_ids` |
