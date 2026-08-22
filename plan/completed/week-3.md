# Week 3 — More sources + the API skeleton

Status: completed
Branch: `week-3`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-3--more-sources--the-api-skeleton)): jobs from 3 source types, deduped, served over REST.

## Outcome
P0 shipped and verified live. Adzuna, Jooble, and Greenhouse run through the same registry; a down source does not block the others (Jooble 403 / Adzuna 503 observed live). `companies` + `POST /companies` drive Greenhouse. Fiber serves `GET /jobs`, `GET /jobs/{id}`. Last successful ingest: Adzuna 61 / Jooble 120 / Greenhouse 5 (Pune DevOps boards), upserted and deduped. `make test` covers all three fixture suites plus API handler tests. No secrets in git. No auto-apply. No scraper.

**P1 leftover (not blocking close):** Lever connector and job-closure (`status='closed'` when `last_seen_at` is stale) were not done. Greenhouse is company-list + local role/location filter — it is not a market search like Adzuna/Jooble. Seed is Energy Exemplar + Tech Holding (Pune DevOps); Stripe has no Pune DevOps listings.

## The three source types
1. **Adzuna** — aggregator API (already shipped).
2. **Jooble** — second aggregator API (`POST https://{region}.jooble.org/api/{key}`).
3. **Greenhouse Job Board API** — one ATS JSON endpoint (`GET https://boards-api.greenhouse.io/v1/boards/{token}/jobs?content=true`). Public read, no API key. Lever/Ashby wait until a later week unless Day 6–7 has slack.

Same `Connector` interface. Registry already isolates a failing source. Worker and `dedup_hash` do not change.

Builds on Week 2 ([plan/completed/week-2.md](week-2.md)): queue publish, worker upsert, `make ingest`.

## Design notes
- **Jooble keys are regional.** A `jooble.org` key is US-only; NL needs a key from `nl.jooble.org`. Store `JOOBLE_API_KEY` + `JOOBLE_BASE_URL` in `.env` (placeholders in `.env.example`). Never commit a real key.
- **Greenhouse board token, not company name.** Stripe’s board is `https://boards.greenhouse.io/stripe` → token `stripe`. `POST /companies` stores `name`, `ats=greenhouse`, `board_token`.
- **ATS fetch is driven by Postgres, not flags.** The API writes `companies`; the Greenhouse connector reads that table on `Fetch`. No HTTP from connector → API (golden language-split rule).
- **API is a new Go module** `services/api/` (Fiber), own `go.mod`. Week 3 endpoints only: `POST /companies`, `GET /jobs`, `GET /jobs/{id}`. Do not implement profile, applications, funnel, or `?band=` — those need scoring (Weeks 4–8).
- **Schema change** → `db/migrations/0002_companies.sql` and update [TRD §3](../../docs/TRD.md#3-data-model-core-tables) in the same change.

## Day 1 — Jooble connector
- `services/connectors/internal/jooble/`: `Connector` implementation. `Fetch` POSTs `{keywords, location, page}` (map `SearchParams.Query` / `Where` / `Page`). `Normalize` maps `title`, `company`, `location`, `snippet` → `description_md`, `link` → `source_url`, `updated` → `posted_at`. Parse salary string only when it is clearly numeric; otherwise leave null — do not invent fields.
- Rate limit + 429 backoff (NFR-7).
- Recorded fixture: `testdata/response.json` + `httptest` test (connector-authoring skill).
- Register Jooble next to Adzuna in `cmd/connector`. A down Jooble must not block Adzuna.

## Day 2 — Companies table + Greenhouse connector
- Migration `0002_companies.sql`: `companies(id uuid pk, name text not null, ats text not null, board_token text not null, created_at timestamptz default now(), unique(ats, board_token))`.
- `services/connectors/internal/ats/greenhouse/`: `GET .../boards/{token}/jobs?content=true`. `Normalize` maps `title`, `location.name`, `absolute_url`, `updated_at`, company from the token/row name, `content` (HTML) → `description_md` as-is. `remote_type` only if location/title contains "remote".
- `Fetch` loads Greenhouse rows from `companies` via pgx (DSN from env). Empty list → empty result, not an error.
- Recorded fixture from a real public board (e.g. a well-known token) with secrets stripped — board JSON is public.
- Register Greenhouse in the CLI registry.

## Day 3 — Fiber API skeleton
- New module `services/api/` with Fiber.
- `GET /jobs` — list from Postgres (`id`, `title`, `company`, `location`, `source`, `source_url`, `posted_at`, `status`), newest `last_seen_at` first. Include `job_sources` as a nested array so the demo can show two URLs on a deduped row.
- `GET /jobs/{id}` — one job + its sources; 404 if missing.
- `POST /companies` — JSON `{name, ats, board_token}`; persist; 201 with the row. Reject unknown `ats` (Week 3: `greenhouse` only).
- Contract tests against a test DB or a transactional pgx mock — at least one happy-path test per endpoint.
- `Makefile`: `api` target. Optional compose service so `make up` starts the API; otherwise `make api` against compose Postgres is enough.

## Day 4 — Wire ingest across three sources
- CLI runs **all registered** connectors through the existing registry (Adzuna, Jooble, Greenhouse). Keep `-sink=stdout|sqs`.
- `make ingest` unchanged in shape: publish from every source, then drain the worker. One Jooble/Greenhouse failure still yields Adzuna jobs (registry isolation).
- Seed one Greenhouse company (via `POST /companies` or `db/seed/`) so the ATS path is not empty on the first demo. Pick a public board that actually has jobs.
- `.env.example`: `JOOBLE_API_KEY`, `JOOBLE_BASE_URL`, `API_ADDR` (e.g. `:8080`).

## Day 5 — Demo
- `make up && make migrate && make ingest && make api`
- `curl` `POST /companies`, `GET /jobs`, `GET /jobs/{id}` — show rows from ≥2 source types if Jooble key is present, and Greenhouse if a company was added. Dedup: same role from Adzuna + Greenhouse → one job, two `job_sources`.
- `make test` includes connectors (Adzuna + Jooble + Greenhouse fixtures) and `services/api`.

## Day 6–7 — Buffer / P1
- Jooble key approval and regional-endpoint surprises are the likeliest snag.
- **P1 if time:** Lever connector behind the same ATS table (`ats='lever'`, `GET https://api.lever.co/v0/postings/{token}`).
- **P1 if time:** Week 2 leftover — mark jobs `closed` when `last_seen_at` is older than N days. Only if ingest is being run more than once.

## Definition of done
- Three connector types registered; each has a recorded-fixture test; one source failing does not block the others.
- `companies` in schema + TRD §3; `POST /companies` persists a Greenhouse board.
- `GET /jobs` and `GET /jobs/{id}` return persisted, deduped jobs over REST.
- `make ingest` still local-only (compose + Ollama unused). No secrets in git. No auto-apply. No scraper.

## Move to completed
Moved to `plan/completed/week-3.md`.
