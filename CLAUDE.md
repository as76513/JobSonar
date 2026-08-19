# CLAUDE.md

Working agreement for Claude Code on the JobRadar repo. Read this before making changes.

## What this project is

A single-user, resume-driven job tracker. Sources jobs from **legitimate free APIs and ATS endpoints**, scores them against a resume with an **explainable, tiered** model, and tracks applications with a **conversion funnel**. See `docs/ARCHITECTURE.md` and `docs/TRD.md` for the full picture.

## Golden rules (do not violate)

1. **No auto-apply.** Never write code that submits an application form on the user's behalf. Assist-to-apply only (draft, tailor, open link).
2. **No scraping as a default source.** New sources should be aggregator APIs or ATS JSON endpoints. The scraper connector stays disabled by default and must respect robots.txt and rate limits.
3. **Hard gates live in SQL, not the LLM.** Must-have skills, seniority band, and location are deterministic filters. Never let the model override them.
4. **Tiered inference is mandatory.** The local model (Ollama) scores every job. The premium model (Bedrock) runs only on the shortlist. Do not send all jobs to Bedrock — it breaks the cost model (NFR-1).
5. **Resume is PII.** Default to local embeddings. Only shortlisted job descriptions + derived profile fields may go to Bedrock, and only when opted in. Never log raw resume text.
6. **No secrets in code or manifests.** Use env vars locally and External Secrets / IRSA in cloud.
7. **Portability.** Keep AWS SDK calls behind a thin adapter. The stack must still run fully local on kind/k3s + Ollama with no cloud.

## Language split

- **Go** — connectors, worker, api. I/O-bound, concurrent, compiled. New plumbing goes here.
- **Python** — the `agent` service only (embeddings, scoring reasoning, resume parsing, LangGraph).
- They communicate **only** via SQS and Postgres. Do not add direct HTTP coupling between Go and Python services.

## Conventions

- New source → implement the `Connector` interface in `services/connectors/internal/<source>/`, register it, add a fixture-based test. Nothing else in the pipeline should change. Use the `connector-authoring` skill.
- New scoring signal → add a sub-score in `services/agent/jobradar_agent/score/`, expose it in the breakdown, add a golden test. Use the `scoring-model` skill.
- Schema changes → a migration in `db/migrations/`; update `docs/TRD.md` §3 in the same change.
- Every service emits OpenTelemetry traces/metrics. New services must wire the OTel exporter.

## Definition of done

- Tests pass (`make test`), including golden scoring tests within tolerance.
- New connector has a recorded-fixture test and isolates its own failures.
- No secret material added to the repo.
- If schema or endpoints changed, the relevant doc in `docs/` is updated in the same PR.
- Runs locally via `make up` with Ollama, no cloud required.

## Common commands

```
make up        # local stack (postgres, ollama, sqs emulator, services)
make test      # all unit + golden + contract tests
make migrate   # apply db migrations
make deploy    # build images + apply manifests to the current kube context
```

## When unsure

Prefer the smallest change that keeps the portability seams intact. If a change would send more data to a paid model, couple Go and Python directly, or put a secret in the repo — stop and flag it instead.
