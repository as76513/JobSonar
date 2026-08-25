-- +goose Up
-- Week 6: persisted sub-scores (TRD §3), one row per (job, profile), written
-- by the agent's scoring pass. The API reads this table; it never computes
-- scores itself.

CREATE TABLE scores (
    job_id         UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    profile_id     UUID NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    composite      DOUBLE PRECISION NOT NULL,
    skill_cov      DOUBLE PRECISION NOT NULL,
    semantic       DOUBLE PRECISION,
    seniority_fit  DOUBLE PRECISION NOT NULL,
    location_fit   DOUBLE PRECISION NOT NULL,
    recency        DOUBLE PRECISION NOT NULL,
    band           TEXT NOT NULL,
    matched_skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    missing_skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    scored_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, profile_id)
);

CREATE INDEX scores_profile_band ON scores (profile_id, band, composite DESC);

-- +goose Down
DROP TABLE IF EXISTS scores;
