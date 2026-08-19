Build the next vertical slice end to end for "$ARGUMENTS": connector -> SQS -> worker
-> Postgres/pgvector -> scoring -> API/UI. Keep it runnable via `make up` locally with
Ollama and no cloud. Report what works end to end and what's stubbed.
