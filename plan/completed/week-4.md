# Week 4 — UI + naive scoring

Status: completed
Branch: `week-4`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-4--ui--naive-scoring)): browse ranked jobs, move one through the pipeline — all local, no AI, no cloud.

## Outcome
P0 shipped and verified. Keyword overlap ranks `GET /jobs` in Go (no LLM). `profiles` + `applications` + `application_events` land in `0003_applications.sql`. Fiber serves profile + application CRUD; React list / detail / Kanban track saved → applied → … → closed. Demo: 179 ingested jobs ranked (top: Tech Holding Senior DevOps Engineer, 90% coverage); one job saved and moved to **applied**. `go test ./...` in `services/api` covers `keyword` + `TestApplicationsPipeline`. No secrets. No auto-apply. No Bedrock.

**P1 leftover (not blocking close):** Lever connector and stale-job closure remain from Week 3. Tunable score weights wait for Week 6. Vite binds `localhost` (IPv6); `127.0.0.1:5173` can refuse.

Builds on Week 3 ([plan/completed/week-3.md](week-3.md)): Adzuna + Jooble + Greenhouse ingest, Fiber `GET /jobs` / `GET /jobs/{id}`, `POST /companies`.

## Design notes
- **Dumb keyword score lives in Go.** Week 5–6 own the Python agent and real sub-scores (`scoring-model` skill). This week ranks by skill-token overlap against a seeded `profiles.skills` list — no LLM, no embeddings.
- **Hard gates stay out.** No SQL must-have filters yet (Week 6). The score is explainable: `matched` / `missing` skill lists and a 0–1 coverage number.
- **No auto-apply.** Tracker records status and opens `source_url`. It never submits a form.
- **Schema** → `db/migrations/0003_applications.sql` (`profiles`, `applications`, `application_events`) and [TRD §3](../../docs/TRD.md#3-data-model-core-tables) / [§4.3](../../docs/TRD.md#43-key-rest-endpoints-go-api) in the same change.
- **UI is a new Vite + React app** in `web/`. Dev server proxies `/jobs`, `/profile`, `/applications` to the host API (`:8080`). Optional nginx compose service (`profiles: ["ui"]`) serves `web/dist` on `:3000` so default `make up` does not require a build.

## P0
- React UI: job list (ranked) + job detail + Kanban tracker.
- Keyword overlap score on `GET /jobs` and `GET /jobs/{id}`.
- `POST /applications`, `PATCH /applications/{id}`, `GET /applications`.
- Pipeline: saved → applied → screen → interview → offer → closed.

## Day 1 — Schema + seeded profile
- Migration `0003_applications.sql`:
  - `profiles(id uuid pk, skills jsonb not null default '[]', updated_at)` — one row is enough for a single-user app.
  - `applications(..., unique(job_id))` — one tracker card per job. Status default `saved`. `applied_at` set only when status becomes `applied`.
  - `application_events(application_id, from_status, to_status, at)` — append-only history for every `PATCH`.
- `db/seed/profile.sql`: insert one profile with a DevOps skill list (`kubernetes`, `terraform`, `aws`, `azure`, `docker`, `devops`, `ci/cd`, `linux`, `python`, `go`).
- `make seed` applies greenhouse boards **and** the profile seed.
- Update TRD §3 (new tables) and §4.3 (`GET`/`PUT /profile`, ranked `GET /jobs`, applications endpoints).

## Day 2 — Keyword overlap score (Go only)
- `services/api/internal/score`: tokenize title + description against `profiles.skills`.
- Normalize (lowercase, trim). Match padded tokens so `"go"` does not hit `"golang"`. Treat `ci/cd` as a phrase, not two tokens.
- Return `{coverage, matched_skills, missing_skills}`. Coverage = matched / len(skills), 0 when the profile is empty.
- Unit tests in `keyword_test.go`: overlap, phrase match, short-token false positive (`go` vs `golang`).
- Do **not** add Python, Ollama scoring, or Bedrock. Do **not** let a model override rank.

## Day 3 — Profile + applications API
- `GET /profile` / `PUT /profile` — read/replace the skill list (JSON array of strings).
- Rank `GET /jobs` by `score.coverage` desc. Omit `description_md` on the list (keep payload small). Include `score` and optional `application` when a tracker row exists.
- `GET /jobs/{id}` — same score plus full description.
- `GET /applications`, `POST /applications` `{job_id}` (default `saved`), `PATCH /applications/:id` `{status}`.
- Valid statuses only: `saved` → `applied` → `screen` → `interview` → `offer` → `closed`. `applied` sets `applied_at`. Append an event in the same transaction.
- CORS for `http://localhost:5173` (Vite) and `http://localhost:3000` (nginx).
- Handler tests for rank order, create, status patch, reject unknown status.
- `make test` includes `services/api` score + application tests.

## Day 4 — React job list + detail
- Scaffold `web/` (Vite + React). `vite.config.js` proxies API paths to `:8080`.
- **Jobs** page: ranked list (title, company, location, coverage %, matched/missing chips). Skill box + **Re-rank** (`PUT /profile` then reload list).
- **Job detail**: description, score breakdown, **Open posting** (`source_url`, new tab — never submit). **Save to tracker** → `POST /applications`.
- Dark green/teal styling consistent across pages. No auth (single-user local).

## Day 5 — Kanban tracker + demo
- **Tracker** page: columns for each pipeline status. Selecting a card / dropdown calls `PATCH /applications/:id`.
- `Makefile`: `web` (`npm install && npm run dev` → `:5173`), `web-build` (`web/dist`).
- Optional compose `web` service behind `profiles: ["ui"]` + `web/nginx.conf` proxying API to `host.docker.internal:8080`.
- Demo: `make up && make migrate && make seed && make api` + `make web`.
  - List ranks (Pune DevOps should surface first against the seeded skills).
  - Open one job, Save to tracker, move saved → applied on the board.
- Bind/verify on `http://localhost:5173/` (Vite may be IPv6-only; `127.0.0.1:5173` can refuse).

## Day 6–7 — Buffer / P1
- Buffer for CORS, Vite bind (localhost vs 127.0.0.1), and empty-profile / zero-coverage edge cases.
- **P1 leftover (not this week's gate):** Lever connector and stale-job closure (Week 3 leftovers). Tunable score weights wait for Week 6.
- Do not start resume upload, embeddings, or hard gates — those are Weeks 5–6.

## Definition of done
- `make test` includes API score + application handler tests.
- `make migrate && make seed && make api` + UI: rank jobs, open one, move it on the board.
- Local only. No secrets. No auto-apply. No Bedrock.

All P0 items satisfied. Milestone M1 (data path works) is closed.

Next: [plan/completed/week-5.md](week-5.md).
