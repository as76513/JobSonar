---
name: scoring-model
description: Add or modify a JobRadar match sub-score or the scoring cascade. Use when touching skill coverage, semantic similarity, seniority/location/recency, or the shortlist threshold. Do NOT use for connectors or infra.
---
# Scoring changes
1. Sub-scores live in services/agent/jobradar_agent/score/. Each is named and surfaced in the breakdown.
2. Hard gates (must-have skills, seniority, location) are evaluated in SQL, not the model.
3. First pass uses the local LLM/embeddings and touches every job.
4. Deep dive (Bedrock) runs only above the shortlist threshold.
5. After any change, add/update a golden test (fixed resume+job -> expected sub-scores within tolerance).
6. Never send all jobs to the premium model (cost model NFR-1).
