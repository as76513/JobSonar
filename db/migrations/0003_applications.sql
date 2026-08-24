-- +goose Up
-- Week 4: single-user profile (skill list) + application pipeline.

CREATE TABLE profiles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skills     JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE applications (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id         UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    resume_variant TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'saved',
    applied_at     TIMESTAMPTZ,
    notes          TEXT NOT NULL DEFAULT '',
    contacts       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id)
);

CREATE TABLE application_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    from_status    TEXT,
    to_status      TEXT NOT NULL,
    at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX application_events_app_id_at ON application_events (application_id, at);

-- +goose Down
DROP TABLE IF EXISTS application_events;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS profiles;
