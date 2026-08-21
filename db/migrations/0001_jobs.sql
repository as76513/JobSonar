-- +goose Up
-- Week-1 subset of TRD §3: jobs + job_sources.
-- pgvector is enabled now so Week 5 does not need an image/extension swap.
-- dedup_hash is unique here; the worker (Week 2) is what populates it.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dedup_hash       TEXT NOT NULL UNIQUE,
    source           TEXT NOT NULL,
    source_url       TEXT NOT NULL,
    title            TEXT NOT NULL,
    company          TEXT NOT NULL,
    location         TEXT NOT NULL DEFAULT '',
    remote_type      TEXT NOT NULL DEFAULT '',
    description_md   TEXT NOT NULL DEFAULT '',
    skills_extracted JSONB NOT NULL DEFAULT '[]'::jsonb,
    salary_min       NUMERIC,
    salary_max       NUMERIC,
    currency         TEXT,
    posted_at        TIMESTAMPTZ,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    status           TEXT NOT NULL DEFAULT 'open'
);

CREATE TABLE job_sources (
    job_id     UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    source     TEXT NOT NULL,
    source_url TEXT NOT NULL,
    PRIMARY KEY (job_id, source, source_url)
);

-- +goose Down
DROP TABLE IF EXISTS job_sources;
DROP TABLE IF EXISTS jobs;
DROP EXTENSION IF EXISTS vector;
