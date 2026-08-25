# Week 6 — Explainable sub-scores + hard gates

Status: active
Branch: (create off `main`, e.g. `week-6`)
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-6--explainable-sub-scores--hard-gates)): each job shows a score breakdown and a skill gap. Closes milestone **M2 — Explainable matching**.

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

## Day 2 — Skill coverage + extraction, in Python
- `services/agent/jobsonar_agent/score/lexicon.py`: port the ~60-term skill lexicon from `services/api/internal/score/lexicon.go` into Python — deliberately duplicated (Go and Python are separate runtimes; this mirrors the connector/worker `Job` duplication pattern from Week 2), not shared.
- `services/agent/jobsonar_agent/score/skill_coverage.py`: extract `skills_extracted` for a job from title+description via the lexicon (write it back to `jobs.skills_extracted`, once, so it's a persisted explainable fact, not a re-derived one), then `skill_cov = |matched ∩ profile.skills| / |skills_extracted|`, plus `matched_skills`/`missing_skills` lists. This is a straight port of `keyword.go`'s existing logic, not a new algorithm.

## Day 3 — Seniority, location, recency sub-scores + composite
- `score/seniority.py`: infer a job's seniority band from title keywords (junior/mid/senior/lead/staff/principal — same style of heuristic as the existing lexicon matching); `seniority_fit` = 1.0 if it matches `profile.seniority` (or unset), a partial score for adjacent bands, 0 for a clear mismatch.
- `score/location.py`: `location_fit` from `job.remote_type`/`job.location` vs. `profile.remote_pref`/`profile.location` (string/remote-type compatibility; unset profile fields = neutral 1.0, not a penalty).
- `score/recency.py`: decay `recency` from `posted_at` (or `first_seen_at` if null) — e.g. linear or exponential falloff over ~60 days.
- `score/composite.py`: named weights (a module-level dict, not hardcoded inline — Week 6's P1 is making these tunable in the UI, so keep them one place), `composite = Σ weight_i * subscore_i`, and a `band` from fixed thresholds (e.g. `strong ≥ 0.7`, `good ≥ 0.5`, else `stretch`).

## Day 4 — Hard gates in SQL
- A SQL predicate (view or reusable WHERE fragment) evaluated by the agent's scoring pass, per `scoring-model` skill rule 2 ("hard gates... in SQL, not the model"): job passes iff `profile.must_have_skills ⊆ jobs.skills_extracted` (vacuously true if `must_have_skills` is empty) AND seniority band is not a hard mismatch (if `profile.seniority` set) AND location/remote is not a hard mismatch (if `profile.location`/`remote_pref` set).
- Jobs failing a gate still get a `scores` row (so the UI can explain *why* excluded) but with `band = 'excluded'` and are omitted from the default ranked list — never silently dropped.
- **This logic must live in SQL, called by the agent — never a judgment call left to an LLM**, per `CLAUDE.md` golden rule 3. Nothing in this week touches an LLM at all; that's Week 7.

## Day 5 — Wire the scoring pass + golden test
- Extend `services/agent/jobsonar_agent/run.py`'s existing loop (same cadence as the Week 5 embed pass) to also score any `(job, profile)` pair missing a `scores` row, or whose `jobs.last_seen_at`/`profiles.updated_at` moved since `scored_at`.
- `services/agent/tests/test_scoring_golden.py`: a fixed resume + fixed job fixture → expected sub-scores within tolerance, per the `scoring-model` skill's mandatory rule. Add/update this test with every future sub-score change, not just this week.

## Day 6 — API reads from `scores`
- `services/api`: `GET /jobs` and `GET /jobs/{id}` join `scores` instead of calling `score.Overlap()`; response includes the named sub-scores, `band`, and `matched_skills`/`missing_skills` from the table.
- Delete `services/api/internal/score/keyword.go` and `lexicon.go` once nothing references them — per CLAUDE.md, no parallel implementation left "just in case."
- Jobs with no `scores` row yet (freshly ingested, agent hasn't caught up) need a UX state — reuse the existing "waiting on the agent" pattern from Jobs.jsx's resume-upload flow rather than inventing a new one.

## Day 7 — UI, demo, buffer
- `web/src/pages/Jobs.jsx` / `JobDetail.jsx`: replace the Coverage/Semantic-only display with the full breakdown — composite %, band, and each named sub-score, plus the skill gap (`missing_skills`).
- **Demo:** open a job, see a score breakdown (five named sub-scores + band) and a skill gap list, sourced from a persisted `scores` row, not a live computation.
- Buffer for whatever slipped — the Go→Python scoring move (Day 1–2) is the likeliest source of friction, not the sub-score math itself.

## P1 (if time allows)
Tunable weights in the UI — expose `score/composite.py`'s weight dict via a `PATCH /profile/weights`-style endpoint and a simple form, rather than editing Python to change ranking behavior.

## Definition of done
Matches [CLAUDE.md](../../CLAUDE.md)'s checklist — `make test` passes including the new golden scoring test, hard gates are verifiably SQL (not model) logic, `docs/TRD.md` updated in the same change as the schema migration, runs fully local via `make up` (no Bedrock/cloud — that's Week 7).

## Move to completed
When the week is done, move this file to `plan/completed/week-6.md` and update `Status:` above.
