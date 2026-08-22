# Week 2 — Queue + worker + dedup

Status: completed
Branch: `week-2`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-2--queue--worker--dedup)): one command ingests Adzuna → deduped rows in Postgres.

## Outcome
P0 shipped and verified live. Connectors publish a normalised `Job` to ElasticMQ (`raw-jobs` + DLQ); the Go worker drains, computes `dedup_hash`, and upserts `jobs` / `job_sources`. Dedup, idempotency, and poison-message tests pass. `make ingest` against live Adzuna persisted 20 jobs. No secrets committed; connector and worker share only the queue and Postgres.

**P1 leftover (not blocking close):** `first_seen_at` / `last_seen_at` work via upsert. Closure (`status='closed'` for stale listings) is deferred until ingest runs often enough for staleness to mean something — revisit after Week 3 adds more sources.

## Progress
- **Days 1–3 (all P0): done and verified live**, not just unit-tested. Published two synthetic messages for the same role from two sources plus one malformed message directly onto the real `raw-jobs` ElasticMQ queue, ran the real `services/worker` binary against them, and confirmed in Postgres: one `jobs` row, two `job_sources` rows. Separately confirmed the malformed message survives exactly `maxReceiveCount=3` redeliveries before landing in `raw-jobs-dlq`, per the redrive policy in `elasticmq.conf`.
- Fixed a test-hygiene issue found along the way: the connector's SQS integration test was gated on `SQS_ENDPOINT_URL`/`SQS_QUEUE_URL`, which `.env` sets for every dev (needed for `make publish`) — so it would silently run, and could race a concurrently-draining worker on the same queue, on every plain `make test`. Re-gated behind an explicit `RUN_QUEUE_INTEGRATION_TEST=1`.
- `docs/TRD.md` and `docs/ARCHITECTURE.md` updated to match: connectors normalise and publish `Job`; workers only hash + persist (was previously worded as "worker normalises `RawJob`").
- **Day 4 (P1) deferred**: `first_seen_at`/`last_seen_at` already work correctly as a side effect of the `Upsert` SQL (verified by the idempotency test). Closure marking (flip `status='closed'` for stale listings) not implemented — no real ingestion cadence yet to make staleness meaningful; revisit once Week 3 adds more sources/runs.
- **Day 5: done and verified with real data.** `make ingest` (connector → SQS → worker → Postgres) run against live Adzuna with real `ADZUNA_APP_ID`/`ADZUNA_APP_KEY` — 20 real jobs fetched, normalized, queued, drained, and persisted with no decode/upsert errors (title, company, location, salary, posted_at all populated correctly from the live API).

Builds on Week 1 ([plan/completed/week-1.md](week-1.md)): the `Connector`/`Registry` types, the Adzuna connector, and the `jobs`/`job_sources` schema already exist and pass `go test ./...`.

## Design note: what goes on the queue

`docs/WEEKLY_PLAN.md` originally said "connector emits `RawJob` to SQS; worker normalises." The `Connector` interface already puts `Normalize` on the connector, and `services/worker` cannot import `services/connectors/internal/adzuna`. Normalisation stays in the connector; the queue carries a unified `Job`. The worker only hashes and upserts. TRD §4.1 was updated to match.

## Day 1 — Queue publishing from the connector
- `services/connectors/internal/queue/queue.go`: a small `Publisher` interface (`Publish(ctx context.Context, job connector.Job) error`) — keeps the AWS SDK behind a thin adapter (CLAUDE.md golden rule 7 / NFR-2).
- `sqsPublisher` implementation using `aws-sdk-go-v2/service/sqs`, pointed at an endpoint from env (`SQS_ENDPOINT_URL`, defaulting to ElasticMQ locally; unset in cloud so the real SQS endpoint is used).
- `docker-compose.yml`: mount an `elasticmq.conf` declaring a `raw-jobs` queue plus a `raw-jobs-dlq` (redrive policy), per NFR-4 — ElasticMQ needs queues declared, it won't auto-create them like real SQS can.
- `cmd/connector/main.go`: add a `-sink=stdout|sqs` flag (default `stdout`, so the Week 1 demo command is unchanged). In `sqs` mode, publish each normalized `Job` instead of printing it.

## Day 2 — Worker scaffold
- New Go module `services/worker/` (own `go.mod`, per [PROJECT_STRUCTURE.md](../../docs/PROJECT_STRUCTURE.md)'s "Go modules per service").
- `internal/dedup/hash.go`: `dedup_hash = sha256(lower(company) || '|' || normalize(title) || '|' || normalize(location))` per [TRD §3](../../docs/TRD.md#3-data-model-core-tables). `normalize()` = lowercase, trim, collapse internal whitespace, so `"Senior SWE"` and `"  senior swe"` hash the same.
- `internal/store/store.go`: pgx v5 pool + two upserts —
  - `jobs`: `INSERT ... ON CONFLICT (dedup_hash) DO UPDATE SET last_seen_at = now(), ...` (never overwrite `first_seen_at`).
  - `job_sources`: `INSERT ... ON CONFLICT (job_id, source, source_url) DO NOTHING`.
- `cmd/worker/main.go`: receive-message loop against the `raw-jobs` SQS/ElasticMQ queue, decode `Job`, delete the message only after a successful upsert (at-least-once, not at-most-once — matters for Day 3's idempotency test).

## Day 3 — Dedup + idempotency tests
- **Dedup property test:** two `Job`s, same `company`/`title`/`location`, different `source`/`source_url` → assert exactly one `jobs` row, two `job_sources` rows. This is the P0 test named in the weekly plan.
- **Idempotency test:** feed the identical message twice (simulating SQS at-least-once redelivery) → still one `jobs` row, `last_seen_at` advances, no duplicate `job_sources` row (NFR-4).
- **Poison-message test:** an undecodable message → worker logs and skips (or routes to DLQ) without crashing the loop — one bad message can't take the worker down, same isolation principle as the connector registry.

## Day 4 — first_seen/last_seen + closure (P1 — only if Days 1–3 land early)
- Confirm the upsert semantics from Day 2 already handle `first_seen_at`/`last_seen_at` correctly (set-once vs always-bump).
- A closure pass: jobs with `last_seen_at` older than N days and not re-seen in the current run get `status = 'closed'`. Simplest version: one SQL statement run at the end of a worker batch — not its own scheduled service yet, that can wait.

## Day 5 — Wire the one-command demo
- `Makefile`: new `ingest` target — run the connector once in `-sink=sqs` mode, then run the worker in a drain-and-exit mode (not long-running) against the same compose stack.
- `make up && make migrate && make ingest`, then a `psql` one-liner (or a tiny `make show-jobs` helper) to display deduped rows — this is the week's demo.

## Day 6–7 — Tests, docs, buffer
- `make test` runs both `services/connectors` and `services/worker` module test suites.
- Update `docs/TRD.md` §1 if you agree with the Day-1 design note (clarify that connectors publish normalized `Job`s, not raw payloads).
- Buffer day for ElasticMQ/SQS SDK friction — endpoint-style vs path-style addressing, and queue URL differences between ElasticMQ and real SQS are the likeliest snag.

## Definition of done
Matches [CLAUDE.md](../../CLAUDE.md)'s checklist — `make test` passes (including the new dedup/idempotency tests), nothing secret is committed, runs fully local via `make up`, no direct HTTP coupling introduced between the connector and worker (they only share the queue and Postgres). All P0 items satisfied.

Next: [plan/completed/week-3.md](week-3.md). Week 4 is UI + naive scoring.
