-- +goose Up
-- Week 5: local embeddings + resume rows. Parse/embed happen in Python;
-- Go only stores the file and queries pgvector.

ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS embedding vector(768);

CREATE TABLE resumes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_name TEXT NOT NULL DEFAULT 'default',
    storage_uri  TEXT NOT NULL,
    parsed       JSONB,
    status       TEXT NOT NULL DEFAULT 'pending',
    error        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX resumes_status_created ON resumes (status, created_at DESC);

CREATE TABLE job_embeddings (
    job_id     UUID PRIMARY KEY REFERENCES jobs (id) ON DELETE CASCADE,
    embedding  vector(768) NOT NULL,
    model      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS job_embeddings;
DROP TABLE IF EXISTS resumes;
ALTER TABLE profiles DROP COLUMN IF EXISTS embedding;
