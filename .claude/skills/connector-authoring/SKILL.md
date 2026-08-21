---
name: connector-authoring
description: Author or repair a JobSonar source connector. Use when adding a new job source (aggregator API or ATS board) or fixing an existing one. Do NOT use for scoring or infra work.
---
# Authoring a connector
1. Create `services/connectors/internal/<source>/`.
2. Implement `Connector`: Name, Fetch, Normalize, RateLimit.
3. Map the source payload to the unified `Job` schema (TRD section 3). Never invent fields.
4. Do not compute dedup here — the worker owns dedup_hash.
5. Respect rate limits; back off on HTTP 429.
6. Register the connector; add a fixture-based test using a recorded response.
7. Confirm a failing source cannot block others (isolate errors).

Aggregator APIs and ATS endpoints only. The scraper connector stays disabled by default and must respect robots.txt.
