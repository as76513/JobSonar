-- +goose Up
-- Week 7: deep-dive prose for the shortlist only (TRD §3). Written by the
-- Python agent; the Go API only reads it. Never populated for jobs below
-- the shortlist band — that is the NFR-1 cost model.

CREATE TABLE analyses (
    job_id           UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    profile_id       UUID NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    justification_md TEXT NOT NULL DEFAULT '',
    tailoring_md     TEXT NOT NULL DEFAULT '',
    model            TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, profile_id)
);

CREATE INDEX analyses_profile_created ON analyses (profile_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS analyses;
