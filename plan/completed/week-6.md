# Week 6 — Explainable sub-scores + hard gates

Status: completed
Branch: `week-6` (merged to `main` via PR #6)
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-6--explainable-sub-scores--hard-gates)): each job shows a score breakdown and a skill gap. Closes milestone **M2 — Explainable matching**.

## Outcome
P0 shipped and verified live. The agent writes named sub-scores (`skill_cov`, `semantic`, `seniority_fit`, `location_fit`, `recency`) plus composite/band into `scores`; the Go API only reads that table and ranks by `composite`. Hard gates (must-have skills, seniority, location) are SQL `CASE` expressions in `Store.upsert_score`, not Python/`if`. `GET /jobs` hides `excluded`; `GET /jobs/{id}` still explains the gate. Golden scoring test plus live-DB gate tests are in `services/agent/tests/`. Demo: 54 scored jobs, list banded strong/possible/stretch, detail shows all five sub-scores and skill gaps. No Bedrock. No auto-apply.

**P1 leftover (not blocking close):** tunable weights in the UI.

## Design note before starting: scoring moves from Go to Python

Today, "match score" is computed **live, in Go, on every `GET /jobs` request**: `score.Overlap()` (`services/api/internal/score/keyword.go`) does keyword matching against a hardcoded lexicon, and semantic similarity is a pgvector cosine computed inline in `store.go`'s SQL. There is no `scores` table — `docs/TRD.md`'s schema for one was never migrated.

That was the right shortcut for Week 4 (no agent existed yet). It's the wrong shape for Week 6: `CLAUDE.md`'s language split says Python owns "scoring reasoning"; the `scoring-model` skill says sub-scores live in `services/agent/jobsonar_agent/score/`, named and surfaced in the breakdown, with hard gates in SQL. So this week:
- Scoring becomes a **batch pass in the agent** (same shape as the existing embed pass) that writes one row per (job, profile) to a new `scores` table.
- The Go API stops computing scores and just **reads** `scores`, joined onto `jobs`.
- `services/api/internal/score/keyword.go` and `lexicon.go` become redundant once the API reads from `scores` — delete them at the end of the week rather than keeping two implementations of "skill coverage" alive in two languages.

Also missing today, needed for gates: `profiles` has only `skills` and `embedding` — no `seniority`, `location`, `remote_pref`, or a separate `must_have_skills` list (hard gates are conceptually distinct from the ranking skill list). `jobs.skills_extracted` exists in the schema but has **never been written** by anything.

## Day 1 — Schema
- `db/migrations/0005_scores.sql`: the `scores` table per [TRD §3](../../docs/TRD.md#3-data-model-core-tables) — `job_id, profile_id, composite, skill_cov, semantic, seniority_fit, location_fit, recency, band, matched_skills jsonb, missing_skills jsonb, scored_at`, FKs to `jobs`/`profiles`, `unique(job_id, profile_id)`.
- `db/migrations/0006_profile_preferences.sql`: add `seniority text`, `location text`, `remote_pref text`, and `must_have_skills jsonb default '[]'` to `profiles`. All nullable/empty-default — an unset preference means that gate doesn't filter anything, so the current demo profile (which has none of these) keeps working.
- Update `docs/TRD.md` §3 in the same change (schema-change rule in `CLAUDE.md`).

## Day 2 — Skill coverage + extraction, in Python ✅ done
- No lexicon port needed: `jobsonar_agent.resume.parse.extract_skills()` already exists (Week 5, for resumes) and already implements the identical normalize+substring-match semantics as the Go `Extract()` it was documented to mirror (`services/api/internal/score/lexicon.go:3-5` says as much). `services/agent/jobsonar_agent/score/skill_coverage.py` reuses it directly for job-side extraction instead of adding a second lexicon.
- `skill_coverage.py`: `extract_job_skills(title, description_md)` + `coverage(profile_skills, job_skills) -> (skill_cov, matched, missing)` — same semantics as `keyword.go`'s `Overlap`. Writing `skills_extracted` back to the `jobs` row happens when this is wired into the scoring pass (Day 5), not here.
- `services/agent/tests/test_score_skill_coverage.py`: ports every case from `services/api/internal/score/keyword_test.go` verbatim (same fixtures) so the Go implementation being deleted (Day 6) and its Python replacement provably agree, not just "should."

## Day 3 — Seniority, location, recency sub-scores + composite ✅ done
- `score/seniority.py`: infer a job's band from title keywords (`intern/junior/mid/senior/lead/principal`, defaulting to `mid` when no keyword matches); `seniority_fit` decays by band-distance from `profile.seniority` (1.0 exact match, 0.0 unset or ≥3 bands away).
- `score/location.py`: `location_fit` averages whichever of `remote_pref`/`location` the profile actually set against whatever `job.remote_type`/`job.location` data exists — an axis with nothing to compare against is dropped, not counted as a mismatch (most connectors don't populate `remote_type` today).
- `score/recency.py`: linear decay to 0 over a 60-day window from `posted_at` (falling back to `first_seen_at`), chosen over exponential for the same reason skill matching is substring-based, not ML: linear is easier to explain ("half the window old = half credit").
- `score/composite.py`: named `WEIGHTS` dict (`skill_cov 0.40, semantic 0.20, seniority_fit 0.15, location_fit 0.15, recency 0.10`) — skill_cov weighted highest since it's the primary signal the UI already surfaces (semantic is explicitly a secondary tiebreak per `Jobs.jsx`'s own copy). Missing `semantic` renormalises over the sub-scores that exist rather than counting as 0. Bands are **`strong` / `possible` / `stretch`** per `docs/FRD.md` FR-14 (not invented here) — thresholds `≥0.70` / `≥0.45` / else.
- 24 new tests across the four modules (`services/agent/tests/test_score_{seniority,location,recency,composite}.py`).

## Day 4 — Hard gates in SQL ✅ done
- `Store.upsert_score()` (`services/agent/jobsonar_agent/store.py`) persists every sub-score via one `INSERT ... SELECT ... CASE ... ON CONFLICT DO UPDATE`, where the `CASE` **is** the hard gate: `NOT (profile.must_have_skills = '[]' OR must_have_skills <@ jobs.skills_extracted)` (jsonb containment), OR a clear seniority mismatch (`seniority_fit < 0.3` once a preference is set), OR a clear location mismatch (`location_fit = 0.0` once a preference is set) forces `band = 'excluded'` — decided by Postgres via SQL boolean expressions, never Python `if` statements, per `CLAUDE.md` golden rule 3.
- Jobs failing a gate still get a full `scores` row (composite/sub-scores untouched, so the UI can explain *why* it was excluded) — never silently dropped, just excluded from the default ranked list (Day 6).
- `services/agent/tests/test_store_gates.py`: 6 tests against a **live Postgres** (skips if unreachable — gated on an actual connection attempt, not merely `POSTGRES_DSN` being set, learning from the SQS-integration-test mistake fixed earlier this project). Proves, concretely: a deliberately high composite (0.99) + perfect semantic (1.0) still gets excluded on a seniority mismatch — the gate cannot be talked past by a good score (FRD FR-12).
- Nothing in this week touches an LLM at all; that's Week 7.

## Day 5 — Wire the scoring pass + golden test ✅ done
- `run.score_jobs()`: same shape as `embed_jobs` — drains `store.jobs_for_scoring()` (missing/stale a `scores` row) in `SCORE_BATCH` batches, computes all four sub-scores + composite + band, writes `skills_extracted` and the `scores` row. Wired into `once()` alongside the existing parse/embed passes.
- `services/agent/tests/test_scoring_golden.py`: fixed resume (5 skills) + fixed job fixture → exact expected `skill_cov` (5/7), `seniority_fit` (1.0), `location_fit` (1.0), `recency` (0.5 at half the window), and `composite`/`band`, all within tolerance — matches the filename `docs/PROJECT_STRUCTURE.md` already reserved for this.
- **Verified against real data, not just fixtures**: restarted the live `make agent` process, it scored all 54 ingested jobs against the real (user-uploaded, 30-skill) profile — 36 strong / 8 possible / 10 stretch. Spot-checked: "Senior Java Developer" correctly landed `skill_cov=0`, `missing_skills=["java"]`, `stretch`; top "strong" jobs all had `skill_cov=1`, `location_fit=1` (Pune match), `seniority_fit=1` (no seniority preference set yet — neutral).

## Day 6 — API reads from `scores` ✅ done
- `store.Job` gained a `Score *store.Score` field (nil = not yet scored, never "scored zero"); `jobSelect` now joins `scores` via a `LEFT JOIN LATERAL` keyed to the current profile, replacing the old `job_embeddings`/profile-embedding join entirely (semantic now comes pre-computed from `scores.semantic`, written by the agent).
- **Real bug caught by the new live-DB test** (`scores_test.go`): a `LEFT JOIN LATERAL` with the `band <> 'excluded'` filter *inside* it still returns the outer job row with all-NULL score columns when the filter excludes the only match — `LEFT JOIN` guarantees the outer row survives. The exclusion has to be a `WHERE` clause on the *outer* query (`WHERE sc.band IS DISTINCT FROM 'excluded'`), not inside the lateral. Would have shipped broken (`excluded` jobs still appearing, just unscored-looking) without a test that hits real SQL — the fake-store handler tests could never have caught it.
- `handlers.go`'s `decorate()`/`scoredJob`/`semOr()` and the Go-side `sort.SliceStable` are all deleted — ranking is 100% `ORDER BY sc.composite DESC NULLS LAST, j.last_seen_at DESC` in `store.go` now; the handler only strips `description_md` for the list view.
- Deleted `services/api/internal/score/` (`keyword.go`, `lexicon.go`, their tests) entirely — nothing imports it anymore.
- `GetJob` (unlike `ListJobs`) does **not** filter out `excluded` — Day 4's "never silently dropped" principle: a direct link to a gated job still explains why via `score.band`.
- Verified live: restarted `make api`, `GET /jobs` now returns real persisted sub-scores/band/matched-missing-skills, ordered correctly, matching the direct-Postgres inspection from Day 5.

## Day 7 — UI, demo, buffer ✅ done
- `Jobs.jsx`: cards now show composite % (was `coverage`), a `data-band` badge colored by `strong`/`possible`/`stretch`/`unscored` (was a hardcoded 0.5/0.25 threshold on coverage), and a band-name chip. Pipeline step renamed "Jobs scored" (was "Jobs re-ranked" — that language was semantic-vs-keyword framing that no longer matches: ranking is always by composite now). All "keyword rank vs semantic rank" copy replaced with composite/band language.
- `JobDetail.jsx`: full breakdown — composite % + band label, then all five named sub-scores as chips (skill coverage, semantic, seniority, location, recency), matched/missing skills. An `excluded` band gets an explicit "excluded by a hard gate" explanation line (Day 4's "never silently dropped" principle, actually surfaced now, not just true in the data).
- `styles.css`: `.score[data-band]` selectors renamed from generic `high/mid/low` to the real band names, plus `excluded` (danger color) and `unscored` (muted color).
- **Demo verified live** (Playwright screenshots, not just `npm run build`): Jobs list shows 54 real jobs, correctly banded and colored (green/yellow/red), composite-ordered; clicking into the top job shows "85% · strong match" with all five sub-score chips and matched skills — sourced from the persisted `scores` row end-to-end (Adzuna/Jooble/Greenhouse → worker → agent scoring pass → API → UI).
- **Bug found (pre-existing since Week 4, out of Week 6's scope) and fixed**: direct navigation or a page refresh on `/jobs/<uuid>` was hitting the Go API directly instead of the React app, because both `vite.config.js`'s dev proxy and `nginx.conf`'s prod proxy match `/jobs` as a prefix, which also swallows `/jobs/:id` — the exact path the React Router detail route *and* its own data-fetching `fetch()` call both use. Fixed by distinguishing on the `Accept` header (only a real browser navigation sends `text/html`; `fetch()` doesn't) — Vite's proxy gets a `bypass()` that serves `index.html` for navigations, nginx gets an equivalent `map`+`if`. Verified with a real Playwright direct-navigation screenshot: the exact URL that used to return raw JSON now renders the full page.

## P1 (if time allows)
Tunable weights in the UI — expose `score/composite.py`'s weight dict via a `PATCH /profile/weights`-style endpoint and a simple form, rather than editing Python to change ranking behavior.

## Definition of done
Matches [CLAUDE.md](../../CLAUDE.md)'s checklist — `make test` passes including the new golden scoring test, hard gates are verifiably SQL (not model) logic, `docs/TRD.md` updated in the same change as the schema migration, runs fully local via `make up` (no Bedrock/cloud — that's Week 7).

All P0 items satisfied. Milestone M2 (explainable matching) is closed.

Next: Week 7 — tiered deep dive ([docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-7--tiered-deep-dive)).
