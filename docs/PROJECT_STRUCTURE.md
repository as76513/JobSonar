# Project structure — JobRadar

A polyglot monorepo. Go owns the plumbing, Python owns the AI, and they share only the queue and the database. Infra and deploy manifests keep the local/cloud split honest.

```
jobradar/
├── README.md
├── CLAUDE.md                        # working agreement for Claude Code
├── docker-compose.yml               # local stack: postgres, ollama, sqs-emulator, services
├── Makefile                         # dev, test, up, deploy targets
│
├── docs/
│   ├── FRD.md
│   ├── TRD.md
│   ├── ARCHITECTURE.md
│   ├── PROJECT_STRUCTURE.md
│   ├── WEEKLY_PLAN.md
│   └── SKILLS_AND_COMMANDS.md
│
├── .claude/
│   ├── commands/                    # slash commands (see SKILLS_AND_COMMANDS.md)
│   │   ├── add-connector.md
│   │   ├── score-check.md
│   │   ├── db-migrate.md
│   │   └── ship-slice.md
│   └── skills/                      # project skills (SKILL.md folders)
│       ├── connector-authoring/SKILL.md
│       ├── scoring-model/SKILL.md
│       └── infra-deploy/SKILL.md
│
├── services/
│   ├── connectors/                  # Go — one binary, many connectors
│   │   ├── cmd/connector/main.go
│   │   ├── internal/
│   │   │   ├── connector/           # Connector interface + registry
│   │   │   ├── adzuna/
│   │   │   ├── jooble/
│   │   │   ├── ats/                 # greenhouse, lever, ashby
│   │   │   └── scraper/             # optional, disabled by default
│   │   ├── go.mod
│   │   └── connector_test.go
│   │
│   ├── worker/                      # Go — normalise + dedup consumer
│   │   ├── cmd/worker/main.go
│   │   ├── internal/normalize/
│   │   ├── internal/dedup/
│   │   └── internal/store/          # pgx upserts
│   │
│   ├── api/                         # Go Fiber
│   │   ├── cmd/api/main.go
│   │   ├── internal/handlers/
│   │   ├── internal/store/
│   │   └── internal/analytics/      # funnel queries
│   │
│   └── agent/                       # Python — the AI layer
│       ├── jobradar_agent/
│       │   ├── llm/                 # LLM + Embedder protocols; ollama & bedrock impls
│       │   ├── embed/               # local embedding pipeline
│       │   ├── score/               # sub-scores + hard gates (gates call SQL)
│       │   ├── graph/               # LangGraph: first_pass -> shortlist -> deep_dive
│       │   ├── resume/              # resume parsing -> profile
│       │   └── config.py            # cascade config (which node uses which LLM)
│       ├── tests/
│       │   ├── test_scoring_golden.py
│       │   └── fixtures/
│       └── pyproject.toml
│
├── web/                             # React UI
│   ├── src/
│   │   ├── pages/{Jobs,Tracker,Funnel,Profile}.tsx
│   │   └── components/{ScoreBreakdown,SkillGap,KanbanBoard}.tsx
│   └── package.json
│
├── db/
│   ├── migrations/                  # sql migrations (goose or atlas)
│   └── seed/
│
├── infra/
│   ├── terraform/                   # RDS, SQS, S3/CloudFront, EKS, IAM/IRSA
│   └── helm/ or k8s/                # per-service manifests, CronJobs, ESO, ServiceMonitors
│       ├── connectors-cronjob.yaml
│       ├── worker-deploy.yaml
│       ├── agent-deploy.yaml
│       ├── api-deploy.yaml
│       └── external-secrets.yaml
│
└── .github/workflows/               # CI: lint, test, build images, deploy
```

## Conventions

- **Go modules** per service under `services/` (independent build/deploy); shared types via a small `pkg/` or duplicated deliberately to avoid coupling.
- **Python** single package `jobradar_agent`; `LLM`/`Embedder` behind protocols so the cascade is config-driven.
- **Hard gates live in SQL** (`services/api` or a shared query file), never in the Python LLM path.
- **One image per service**, same image local and cloud; environment differences live only in manifests/config.
- **Migrations are the source of truth** for the schema in TRD §3.
