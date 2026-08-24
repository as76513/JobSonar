# Week 5 — Embeddings + resume parsing

Status: completed
Branch: `week-5`
Headline outcome (from [docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-5--embeddings--resume-parsing)): upload a resume, parse skills locally, store embeddings, and show an explainable match (keyword coverage + optional similarity).

## Outcome
P0 shipped and verified. Python `services/agent` parses PDF/DOCX against a checked-in lexicon, writes `profiles.skills`, and embeds the profile plus jobs (Ollama `nomic-embed-text` when the model is present; `EMBED_BACKEND=fake` otherwise). Go owns `POST /profile/resume` (store file only, 202) and `GET /jobs` rank. `0004_embeddings.sql` adds `profiles.embedding`, `resumes`, and `job_embeddings`. Demo: upload → agent parse → 28 skills on the profile; list ranks by **job-ask skill coverage**, with embedding cosine as a tiebreak and a `sim` chip. `make test` covers Go API score/handlers/store plus agent pytest (live Ollama test skipped unless `RUN_OLLAMA_EMBED_TEST=1`). No raw resume text in logs. No HTTP between Go and Python. No secrets. No auto-apply. No Bedrock.

**P1 leftover (not blocking close):** Rancher Desktop Ollama cannot `ollama pull nomic-embed-text` on corp MITM (`x509: unknown authority`). Use `EMBED_BACKEND=fake` or pull on a host Ollama and copy blobs into `ollama_data`. Years/titles/seniority extract, SQL hard gates, and the composite `scores` row wait for Week 6.

Builds on Week 4 ([plan/completed/week-4.md](week-4.md)): keyword rank, tracker UI, `profiles.skills`.

## Design notes
- **Language split is the seam.** Python owns embeddings and resume parsing. Go owns upload + the similarity query. They share **Postgres only** (Architecture: embedder picks up rows without vectors). No HTTP from API → agent or agent → API.
- **Local embeddings only.** Default model: Ollama `nomic-embed-text` (768-d, matches [TRD §3](../../docs/TRD.md#3-data-model-core-tables) `vector(768)`). Never send resume text to Bedrock. Never log raw resume text (golden rule 5).
- **Parse is deterministic first.** P0: PDF/DOCX → text → skill lexicon extract + profile embedding. Full FR-1 (years, titles, seniority via LLM) is Week 6.
- **Rank.** Headline % is **keyword coverage** of skills the *job* asks for (`matched / job skills`). Extra resume skills are not gaps and do not dilute coverage. When both embeddings exist, cosine (`1 - (job <=> profile)`) is a tiebreak and `score.semantic` on the payload. Missing vectors → keyword-only rank, `semantic` omitted.
- **Schema** → `db/migrations/0004_embeddings.sql` and [TRD §3](../../docs/TRD.md#3-data-model-core-tables) / [§4.3](../../docs/TRD.md#43-key-rest-endpoints-go-api) in the same change.

## P0
- Python `services/agent`: `Embedder` protocol + Ollama implementation + FakeEmbedder.
- `POST /profile/resume` (Go): store file, insert `resumes` row, do not parse in Go.
- Agent: parse pending resumes → update `profiles.skills` + `profiles.embedding`; embed jobs missing vectors into `job_embeddings`.
- `GET /jobs` returns keyword coverage + optional `score.semantic`; ranks by coverage then similarity.

## Definition of done
- `Embedder` protocol + Ollama impl; unit tests pass without a live model.
- Upload → parse → `profiles.embedding` + `job_embeddings`; `GET /jobs` exposes similarity when vectors exist.
- Resume text stays local. No HTTP between Go and Python. No secrets. No auto-apply. No Bedrock.

All P0 items satisfied. Milestone M2 still belongs to Week 6 (explainable composite + hard gates).

Next: Week 6 — explainable sub-scores + hard gates ([docs/WEEKLY_PLAN.md](../../docs/WEEKLY_PLAN.md#week-6--explainable-sub-scores--hard-gates)).
