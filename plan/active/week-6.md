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
