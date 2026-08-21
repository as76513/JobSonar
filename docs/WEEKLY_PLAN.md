# Weekly plan — JobSonar

A 10-week plan sized for evenings/weekends (~8–12 hrs/week). Sequenced **data path first, agent second, cloud third, polish last** — because most job-tracker projects die on sourcing, not matching. Each week has a single headline outcome you can demo.

Legend: **P0** = must land this week · **P1** = if time allows.

---

## Phase 1 — The boring, real data path (Stripe one)

### Week 1 — Foundations & one connector ✅ complete
- **P0** Repo scaffold (structure from `PROJECT_STRUCTURE.md`), `docker-compose.yml` with Postgres + SQS emulator (LocalStack/ElasticMQ) + Ollama.
- **P0** DB migrations for `jobs`, `job_sources` (TRD §3 subset).
- **P0** Go `Connector` interface + **Adzuna** connector with a recorded-fixture test.
- **Demo:** `make up`; running the Adzuna connector prints normalised jobs.
- **Detail:** [plan/completed/week-1.md](../plan/completed/week-1.md)

### Week 2 — Queue + worker + dedup 🚧 in progress
- **P0** Connector emits `RawJob` to SQS; **Go worker** drains it, normalises, computes `dedup_hash`, upserts.
- **P0** Dedup property test (same role, two sources → one job, two URLs).
- **P1** `first_seen`/`last_seen` + closure marking.
- **Demo:** one command ingests Adzuna → deduped rows in Postgres.
- **Detail:** [plan/active/week-2.md](../plan/active/week-2.md)

### Week 3 — More sources + the API skeleton
- **P0** Add **Jooble** + one **ATS** connector (Greenhouse or Lever) behind the same interface.
- **P0** Target-company list (`POST /companies`) driving ATS fetches.
- **P0** Go **Fiber API**: `GET /jobs`, `GET /jobs/{id}`.
- **Demo:** jobs from 3 source types, deduped, served over REST.

### Week 4 — UI + naive scoring
- **P0** React UI: job list + job detail. Deploy as static build (local nginx for now).
- **P0** A **dumb keyword score** (skill overlap only) so the list ranks.
- **P0** Application tracking: `applications` + `application_events`, Kanban board (saved → applied → … → closed).
- **Demo (Stripe one complete):** browse ranked jobs, move one through the pipeline — all local, no AI, no cloud.

---

## Phase 2 — The AI agent (Stripe two)

### Week 5 — Embeddings + resume parsing
- **P0** Python `agent` service; `Embedder` protocol + **local model** (bge/nomic via Ollama).
- **P0** Resume upload → parse → structured profile (`profiles` table) + profile embedding.
- **P0** `job_embeddings` populated; pgvector similarity query working.
- **Demo:** upload resume → semantic similarity ranks jobs better than keywords.

### Week 6 — Explainable sub-scores + hard gates
- **P0** Composite score from named sub-scores: skill coverage, semantic, seniority, location, recency.
- **P0** **Hard gates in SQL** (must-have skills, seniority, location).
- **P0** Matched vs missing skills lists; confidence bands; **golden scoring test**.
- **P1** Tunable weights in the UI.
- **Demo:** each job shows a score breakdown and a skill gap.

### Week 7 — Tiered deep dive
- **P0** LangGraph graph: `first_pass (local) → shortlist → deep_dive (Bedrock or local)` behind the `LLM` abstraction.
- **P0** Deep dive writes `analyses` (justification + tailoring hints) for shortlisted jobs only.
- **P0** Cost guard: assert premium calls ≤ shortlist size (NFR-1 test).
- **Demo:** shortlisted jobs carry a written "why you fit / what to close"; others don't incur premium cost.

### Week 8 — Funnel analytics
- **P0** `GET /analytics/funnel`: apply → response → interview → offer, sliced by role type and band.
- **P0** Ghosting nudge + duplicate-apply guard.
- **Demo:** "you convert on 40% of strong matches, 2% of stretch" — the differentiator feature.

---

## Phase 3 — Cloud & hardening

### Week 9 — Lift onto AWS (hybrid)
- **P0** Terraform: RDS + pgvector, SQS, S3/CloudFront, EKS.
- **P0** Manifests: connectors as **CronJobs**, worker/agent/api Deployments; UI to S3/CloudFront.
- **P0** **External Secrets Operator** ← Secrets Manager; **IRSA** per pod.
- **P0** Enable **Bedrock** for the deep-dive node via config (local path still works).
- **Demo:** the same repo running on EKS; flip one env var to go fully-local again.

### Week 10 — Observability, cost, polish
- **P0** OpenTelemetry across all services → CloudWatch (and a local Grafana option).
- **P0** Right-size: scale-to-zero idle workers, `t4g.micro` RDS, Fargate for CronJobs; confirm near-free-at-rest.
- **P1** Digest, company brief, assisted-apply (open + tailored resume, never auto-submit).
- **P0** README polish + one architecture screenshot for the portfolio.
- **Demo (done):** end-to-end on EKS, cost dashboard, funnel insight, explainable matches.

---

## Milestones

| Milestone | End of | Proves |
|---|---|---|
| M1 — Data path works | Week 4 | Sourcing won't collapse; tracker usable |
| M2 — Explainable matching | Week 6 | The score you can trust |
| M3 — Tiered agent + funnel | Week 8 | The differentiator |
| M4 — Production on AWS | Week 10 | The DevSecOps/cloud story |

## Risk notes

- **Sourcing is the risky 80%** — that's why it's Phase 1. If a source flakes, the connector interface contains the blast radius; move on.
- **Don't gold-plate the agent before M1.** A working boring pipeline beats a clever agent with no data.
- **Watch premium-model spend** from Week 7 — the cost-guard test is not optional.
- **Keep the local path alive** every week; if `make up` ever needs cloud, you've broken portability.
