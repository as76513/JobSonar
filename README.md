# JobSonar

An agentic, resume-driven job tracker. It sources jobs from free aggregator APIs and company ATS boards, scores each opening against your resume with an explainable match model, tracks applications as a personal ATS, and gives you a conversion funnel so you learn which applications actually turn into interviews.

---

## Why this project exists

Job boards optimise for applications sent, not offers received. JobSonar inverts that: it tells you **why** each job is a fit, **what gap** to close, and — over time — **which kinds of applications convert for you**. That feedback loop is the differentiator; matching and tracking are commodity.

Two design commitments carried through the whole project:

1. **Sourcing that won't collapse.** Primary layer is legitimate free APIs (Adzuna, Jooble) and company ATS JSON endpoints (Greenhouse, Lever, Ashby). Scraping is an optional, last-resort connector — never the foundation.
2. **Explainable, tiered matching.** A cheap local model scores every job; a premium model does deep analysis only on the shortlist. Cost tracks value, not volume.

---

## The plan in one paragraph

Build in two vertical stripes. **Stripe one** is the boring-but-real data path — one connector (Adzuna) → queue → normalise/dedup → Postgres+pgvector → a dumb keyword score → a tracker UI — end to end on a laptop with docker-compose. **Stripe two** adds the tiered AI agent (local first pass + Bedrock deep dive), the funnel analytics, and lifts the whole thing onto EKS with the hybrid AWS/self-hosted split. Most people build the fancy agent first and never get sourcing working. We do it backwards.

---

## Tech stack (hybrid)

| Layer | Choice | Buy / Build |
|---|---|---|
| Connectors | Go services, k8s CronJobs | Build (self-host) |
| Queue | Amazon SQS | Buy (managed, free tier) |
| Normalise / dedup | Go workers (goroutines) | Build (self-host) |
| Storage | RDS Postgres + pgvector | Buy (managed) |
| Embeddings | local `bge`/`nomic` model | Build (self-host) |
| Matching agent | tiered: Ollama first pass → Bedrock Claude on shortlist | Both |
| Agent orchestration | LangGraph (Python) | Build (portable) |
| API | Go (Fiber) | Build |
| UI | React on S3 + CloudFront | Buy (hosting) |
| Secrets | Secrets Manager → External Secrets Operator | Buy (custody) + Build (consume) |
| Identity | IRSA (least-privilege per pod) | Build |
| Observability | OpenTelemetry → CloudWatch / Grafana | Portable seam |

**Language split:** Go owns the I/O-bound plumbing (connectors, queue consumers, workers, API); Python owns the AI (embeddings, LLM orchestration, resume parsing). They communicate only via the queue and the database — no tight coupling.

---

## Status

- ✅ **Week 1** — Foundations & one connector: local stack (Postgres+pgvector, ElasticMQ, Ollama), `jobs`/`job_sources` schema, Adzuna connector with a fixture test. See [`plan/completed/week-1.md`](plan/completed/week-1.md).
- 🚧 **Week 2** — Queue + worker + dedup, in progress. See [`plan/active/week-2.md`](plan/active/week-2.md).
- Full roadmap: [`docs/WEEKLY_PLAN.md`](docs/WEEKLY_PLAN.md).

---

## Document index

- [`docs/FRD.md`](docs/FRD.md) — Functional Requirements
- [`docs/TRD.md`](docs/TRD.md) — Technical Requirements
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Architecture, data flow, diagram
- [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md) — Repo layout
- [`docs/WEEKLY_PLAN.md`](docs/WEEKLY_PLAN.md) — 10-week build plan
- [`docs/SKILLS_AND_COMMANDS.md`](docs/SKILLS_AND_COMMANDS.md) — Claude Code skills + slash commands
- [`CLAUDE.md`](CLAUDE.md) — Working agreement for Claude Code

---

## Non-goals (deliberately excluded)

- **Auto-apply / automated form submission.** Violates most portals' ToS, produces low-quality applications, risks account bans. JobSonar *assists* a human to apply fast (tailored resume, pre-filled draft, one-click open) — it never submits on your behalf unseen.
- **Scraping LinkedIn / Indeed as a primary source.** Retired APIs and aggressive bot management. Optional scraper connector only, behind the same interface, respecting robots.txt and rate limits.
