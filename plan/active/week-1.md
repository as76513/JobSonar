# Week 1 — Foundations & one connector

Status: active
Branch: `week-1`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-1--foundations--one-connector)): `make up`; running the Adzuna connector prints normalised jobs.

## Day 1 — Repo scaffold + local stack
- Create the skeleton per [PROJECT_STRUCTURE.md](../../docs/PROJECT_STRUCTURE.md): `services/connectors/{cmd/connector,internal/connector,internal/adzuna}`, `db/migrations/`, `db/seed/`.
- `docker-compose.yml`: Postgres 16 **with pgvector** (image `pgvector/pgvector:pg16`, even though vectors aren't used until Week 5 — cheaper to bake it in now than migrate later), an SQS emulator (ElasticMQ is lighter than LocalStack for just queues), and Ollama.
- `Makefile` targets: `up`, `down`, `migrate`, `test` (stubs are fine where later weeks fill in behavior).
- `.env.example` with `ADZUNA_APP_ID` / `ADZUNA_APP_KEY` placeholders — never a real key committed (CLAUDE.md golden rule 6).

## Day 2 — DB migrations
- Migration tool: **goose** (single binary, plain `.sql` up/down files, no extra runtime) over atlas, absent a reason to prefer atlas's declarative diffing.
- `db/migrations/0001_jobs.sql`: just the Week-1 subset of TRD §3 — `jobs` and `job_sources` only (no `profiles`/`scores`/`analyses` yet — those land with the agent in Phase 2). Include `dedup_hash unique` now even though the worker doesn't populate it until Week 2, so the schema doesn't need a second migration for it.
- `make migrate` applies it against the compose Postgres.

## Day 3–4 — Connector interface + Adzuna
- `services/connectors/internal/connector/connector.go`: the `Connector` interface (`Name`, `Fetch`, `Normalize`, `RateLimit`) and a small registry, per [TRD §4.1](../../docs/TRD.md#41-connector-contract-go) and the `connector-authoring` skill.
- `services/connectors/internal/adzuna/adzuna.go`: calls Adzuna's search endpoint, `Fetch` returns `RawJob`s, `Normalize` maps to the unified `Job` struct (title, company, location, remote_type, description_md, posted_at, source_url — **no dedup_hash**, that's the worker's job next week per the skill's rule 4).
- Respect Adzuna's rate limit in `RateLimit()`; back off on HTTP 429 (NFR-7).
- `services/connectors/cmd/connector/main.go`: CLI that runs the Adzuna connector and prints normalized jobs as JSON lines to stdout — no SQS/DB wiring yet, that's Week 2.

## Day 5 — Fixture test + error isolation
- Record one real Adzuna response into `internal/adzuna/testdata/response.json`.
- `adzuna_test.go`: spin up an `httptest.Server` serving the fixture, assert `Fetch`+`Normalize` produce the expected `Job` fields — this is the "recorded-fixture test" the skill and `/add-connector` command both require.
- Wrap connector execution in the registry so one source's failure can't take down others, even with only one connector registered today (sets the pattern before Week 3 adds more).

## Day 6–7 — Wire it together, demo, buffer
- Confirm `make up` boots Postgres + ElasticMQ + Ollama with no cloud dependency.
- `make test` runs the Go fixture test.
- Demo script: `make up && make migrate && go run ./services/connectors/cmd/connector` → normalized Adzuna jobs printed to stdout.
- Buffer day for whichever of the above slipped — Adzuna API key approval / rate-limit surprises are the likeliest snag.

## Definition of done
Matches [CLAUDE.md](../../CLAUDE.md)'s checklist — `make test` passes, the connector has an isolated fixture test, nothing secret is committed, runs fully local via `make up`.

## Move to completed
When the week is done, move this file to `plan/completed/week-1.md` and update `Status:` above.
