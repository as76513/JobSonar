# Week 7 — Tiered deep dive

Status: implementation complete (live compose demo blocked — Docker daemon was not running)
Branch: `week-7`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-7--tiered-deep-dive)): shortlisted jobs carry a written "why you fit / what to close"; others don't incur premium cost.

Builds on Week 6 ([plan/completed/week-6.md](../completed/week-6.md)): composite `scores` + SQL hard gates. **Do not re-score in the LLM.** First pass stays the existing `score_jobs()` pass. Deep dive is new prose on the shortlist only (FR-15, NFR-1).

## Design note before starting: the graph wraps Week 6, it does not replace it

Today the agent loop is linear: parse → embed → `score_jobs()` → sleep. There is an `LLM` **protocol** (`jobsonar_agent/llm/__init__.py`) but no `OllamaLLM` / `BedrockLLM`, no LangGraph, and no `analyses` table.

Architecture (and the weekly plan) say `first_pass (local) → shortlist → deep_dive`. After Week 6, **first pass is already done** — deterministic sub-scores + SQL gates, not an LLM. Using a local LLM to re-rank would violate golden rule 3 (hard gates in SQL) and NFR-1 (don't send every job to a model). So the graph is:

1. **`first_pass`** — call existing `score_jobs()` (and parse/embed if needed). Touches 100% of jobs. Zero LLM completions.
2. **`shortlist`** — SQL: `scores.band = 'strong'` (composite ≥ 0.70) and not `excluded`. ~5% in the architecture sketch; we reuse the Week 6 band instead of inventing a second threshold.
3. **`deep_dive`** — `LLM.complete()` **once per shortlisted job missing an `analyses` row**. Writes `justification_md` + `tailoring_md`. Backend is config: `fake` (tests / corp TLS) → `ollama` (local demo) → `bedrock` (opt-in only).

Both LLM impls sit behind the existing `LLM` protocol (TRD §4.2). Switching deep-dive from Ollama to Bedrock is an env var, not a rewrite.

**PII (golden rule 5):** never send raw resume text. Prompt inputs are derived profile fields (`skills`, `seniority`, `location`, `remote_pref`) + job title/company + truncated description + `matched_skills` / `missing_skills`. Bedrock only when `DEEP_DIVE_BACKEND=bedrock` **and** `DEEP_DIVE_OPT_IN=1`.

**No auto-apply.** Analysis is assist-to-apply copy only.

**Python:** LangGraph current releases want 3.10+. Agent venv is already 3.11 (`make agent-install`). Keep `make test` off the network: FakeLLM, no live Ollama/Bedrock.

## P0
- LangGraph: `first_pass → shortlist → deep_dive`.
- `analyses` table + agent upsert; Go API reads it on `GET /jobs/{id}`.
- Cost-guard test: premium `complete()` count ≤ shortlist size (NFR-1).
- Demo: a **strong** job shows written justification; a stretch job has none and caused zero premium calls.

## Day 1 — Schema + LLM stubs ✅ done
- `db/migrations/0007_analyses.sql` per [TRD §3](../../docs/TRD.md#3-data-model-core-tables): `job_id`, `profile_id`, `justification_md`, `tailoring_md`, `model`, `created_at`; FKs to `jobs`/`profiles`; `unique(job_id, profile_id)`.
- Update TRD §3 / §4.3 in the same change (`GET /jobs/{id}` includes optional `analysis`).
- `OllamaLLM` (`POST {OLLAMA_HOST}/api/generate` or chat, `LLM_MODEL` default e.g. `llama3.2`) and `FakeLLM` (deterministic markdown, no network) implementing `LLM.complete`.
- `BedrockLLM` thin adapter (`bedrock-runtime` Converse or InvokeModel) — **no calls unless backend is bedrock + opt-in**. No keys in repo; AWS SDK stays in this one file (portability seam).
- `.env.example`: `LLM_MODEL`, `DEEP_DIVE_BACKEND=fake|ollama|bedrock`, `DEEP_DIVE_OPT_IN=0`, `SHORTLIST_BAND=strong`.
- Tests: FakeLLM round-trip; Bedrock/Ollama skipped unless `RUN_LLM_TEST=1`.

## Day 2 — Shortlist query + prompt (no graph yet) ✅ done
- `Store.jobs_for_deep_dive(profile_id, band)`: strong scores with no `analyses` row (or stale `scores.scored_at` > `analyses.created_at`).
- Prompt builder: structured markdown request → JSON or two fenced sections (`justification`, `tailoring`). Cap job description length. Log model name + job id, **never** prompt body if it could contain resume text (it shouldn't; still don't log the prompt).
- Parser: tolerate slightly messy model output; on failure write a short error into `analyses` or skip and leave the job for retry — do not crash the agent loop.
- Unit tests with a fixture JD + profile skills: FakeLLM returns fixed text; parser extracts both fields.

## Day 3 — LangGraph wiring ✅ done
- `jobsonar_agent/graph/`: state `{premium_calls, shortlist_ids, ...}`; nodes `first_pass`, `shortlist`, `deep_dive`.
- `first_pass` node: existing `once()` scoring slice (`score_jobs` after parse/embed), **no** `LLM.complete`.
- Conditional edge: empty shortlist → skip `deep_dive`.
- `deep_dive` node: for each id, `complete()` on the **deep-dive** LLM only; increment `premium_calls` only when `DEEP_DIVE_BACKEND=bedrock` (local Ollama is not premium; FakeLLM is not premium). Still count FakeLLM completions in a `deep_dive_calls` counter so the test can assert `calls == len(shortlist)` and `premium_calls == 0` unless backend is bedrock.
- Wire `once()` / `make agent` to `graph.invoke` instead of calling `score_jobs` as a dangling extra (scoring stays inside `first_pass`).
- Add `langgraph` to `services/agent/pyproject.toml`. `make test` must not require a live graph LLM.

## Day 4 — NFR-1 cost guard (the week's test) ✅ done
- `CountingLLM` wrapper: increments on every `complete()`.
- Fixture: N jobs (e.g. 10), M of which are `strong` (M << N). Run the graph with FakeLLM (or a stub BedrockLLM).
- Assert: `deep_dive_calls == M` and `deep_dive_calls <= len(shortlist)` and **stretch/possible/excluded jobs never appear in the prompt job-id list**.
- Second case: `DEEP_DIVE_BACKEND=bedrock` + opt-in **unset** → zero Bedrock invocations (fail closed).
- This is not optional (WEEKLY_PLAN risk note). Filename: `services/agent/tests/test_nfr1_cost_guard.py`.

## Day 5 — API + UI ✅ done
- Go: join `analyses` on `GET /jobs/{id}` (and omit from list payload to keep it small). `analysis` null if none.
- Handler test: job with analysis row vs without.
- `JobDetail.jsx`: sections **Why you fit** / **What to close** when `job.analysis` exists; muted "Deep dive runs on strong matches only" when band is not strong; nothing invented by the UI.
- List chip `analyzed` on strong jobs that have a row (optional, keep cards uncluttered if noisy).
- No Bedrock from the browser. No resume text in API logs.

## Day 6–7 — Demo, buffer ✅ code complete; live compose demo pending Docker
- `make migrate && EMBED_BACKEND=fake DEEP_DIVE_BACKEND=fake make embed` (or `make agent --once`) → strong jobs get FakeLLM prose; stretch jobs stay empty.
- If a local Ollama **chat** model is available (nomic-embed-text is not a chat model — need a generate model), demo `DEEP_DIVE_BACKEND=ollama`. Corp MITM may block `ollama pull`; FakeLLM is an acceptable demo for close-out, same as Week 5 embeddings.
- Confirm: `make test` includes the cost-guard test; no secrets; no HTTP Go↔Python; no auto-apply; no Bedrock unless opted in.
- **Still out:** funnel (Week 8), tunable weights (Week 6 P1), Lever, real Bedrock in AWS (Week 9 flips the env).

## P1 (if time)
- Truncate/refresh analysis when `scores.scored_at` is newer than `analyses.created_at`.
- `?band=strong` already unused on `GET /jobs` — leave list unfiltered; deep dive is agent-side.

## Definition of done
- Graph runs locally with FakeLLM; shortlisted jobs only get `analyses` rows.
- NFR-1 test fails if deep-dive would run on every job.
- TRD updated with `analyses` + `GET /jobs/{id}` analysis field.
- Resume text never leaves the box; Bedrock stays opt-in. No auto-apply.

## Move to completed
When the week is done, move this file to `plan/completed/week-7.md` and update `Status:` above.
