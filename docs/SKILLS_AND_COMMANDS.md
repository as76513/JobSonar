# Skills & commands — JobRadar (Claude Code)

Two kinds of reusable automation live in `.claude/`:

- **Skills** (`.claude/skills/<name>/SKILL.md`) — instructions Claude Code loads when a task matches, encoding *how we do a recurring kind of work* (authoring a connector, adding a scoring signal, deploying infra). They make the work repeatable and consistent with the golden rules in `CLAUDE.md`.
- **Slash commands** (`.claude/commands/<name>.md`) — short, invocable prompts for frequent chores (`/add-connector`, `/score-check`, `/db-migrate`, `/ship-slice`).

Below are the ones worth creating, with starter content.

---

## Skills

### 1. `connector-authoring`
**When:** adding or fixing a job source.
**Encodes:** implement the `Connector` interface only; never touch the pipeline; aggregator/ATS sources only (scraper stays opt-in); add a recorded-fixture test; respect rate limits and 429 backoff.

```markdown
---
name: connector-authoring
description: Author or repair a JobRadar source connector. Use when adding a new
  job source (aggregator API or ATS board) or fixing an existing one. Do NOT use
  for scoring or infra work.
---
# Authoring a connector
1. Create `services/connectors/internal/<source>/`.
2. Implement `Connector`: Name, Fetch, Normalize, RateLimit.
3. Map the source payload to the unified `Job` schema (TRD §3). Never invent fields.
4. Compute nothing about dedup here — the worker owns dedup_hash.
5. Respect rate limits; back off on HTTP 429.
6. Register the connector; add a fixture-based test using a recorded response.
7. Confirm a failing source cannot block others (isolate errors).
Aggregator APIs and ATS endpoints only. The scraper connector stays disabled by default.
```

### 2. `scoring-model`
**When:** changing match scoring.
**Encodes:** sub-scores are explainable and named; hard gates stay in SQL; add a golden test; never let the LLM override deterministic gates; keep first-pass local.

```markdown
---
name: scoring-model
description: Add or modify a JobRadar match sub-score or the scoring cascade. Use
  when touching skill coverage, semantic similarity, seniority/location/recency, or
  the shortlist threshold. Do NOT use for connectors or infra.
---
# Scoring changes
1. Sub-scores live in services/agent/jobradar_agent/score/. Each is named and surfaced in the breakdown.
2. Hard gates (must-have skills, seniority, location) are evaluated in SQL, not the model.
3. First pass uses the local LLM/embeddings and touches every job.
4. Deep dive (Bedrock) runs only above the shortlist threshold.
5. Add/After change: a golden test (fixed resume+job -> expected sub-scores within tolerance).
6. Never send all jobs to the premium model (cost model NFR-1).
```

### 3. `infra-deploy`
**When:** infra or deployment changes.
**Encodes:** portability (local kind/k3s must still work); secrets via ESO + IRSA, never in manifests; one image per service; OTel wired.

```markdown
---
name: infra-deploy
description: Modify JobRadar infrastructure or deployment (Terraform, Helm/k8s
  manifests, CronJobs, External Secrets, IRSA). Use for cloud or local cluster
  changes. Do NOT use for application logic.
---
# Infra & deploy
1. Every change must keep the local (kind/k3s + Ollama, no cloud) path working.
2. No secrets in manifests: source from Secrets Manager via External Secrets Operator.
3. Each pod gets least-privilege AWS access via IRSA — no node-wide/long-lived keys.
4. Same container image runs local and cloud; only config/manifests differ.
5. New services wire the OpenTelemetry exporter and a ServiceMonitor.
6. Keep AWS SDK calls behind the adapter layer.
```

---

## Slash commands

### `/add-connector`
`.claude/commands/add-connector.md`
```markdown
Add a new source connector named "$ARGUMENTS".
Use the connector-authoring skill. Implement the Connector interface, map to the
unified Job schema, add a recorded-fixture test, register it, and confirm error
isolation. Aggregator/ATS only. Show me the diff and the test.
```

### `/score-check`
`.claude/commands/score-check.md`
```markdown
Run the golden scoring tests and summarise any drift. If a sub-score changed,
explain which signal moved and whether a hard gate was (incorrectly) delegated to
the model. Do not "fix" by loosening a gate without asking.
```

### `/db-migrate`
`.claude/commands/db-migrate.md`
```markdown
Create a migration for "$ARGUMENTS" in db/migrations/, apply it locally, and update
docs/TRD.md §3 to match in the same change. Show the up/down SQL.
```

### `/ship-slice`
`.claude/commands/ship-slice.md`
```markdown
Build the next vertical slice end to end for "$ARGUMENTS": connector -> SQS -> worker
-> Postgres/pgvector -> scoring -> API/UI. Keep it runnable via `make up` locally with
Ollama and no cloud. Report what works end to end and what's stubbed.
```

---

## Relationship to Endor's Agent Kit (if you reuse the pattern)

The skill+command layout mirrors how Endor Labs' open-source Agent Kit packages AppSec workflows for Claude Code — small, opinionated, guardrailed recipes invoked by name. If you later add a security stripe to this project, that kit is a good reference for structuring SCA/remediation skills the same way.
