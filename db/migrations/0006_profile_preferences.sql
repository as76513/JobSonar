-- +goose Up
-- Week 6: hard-gate preferences on profiles. All optional — an unset value
-- means that gate does not filter anything, so the existing single-profile
-- demo data keeps working without a backfill.
-- must_have_skills is deliberately separate from the ranking `skills` list:
-- `skills` drives skill_cov (a ranking signal), must_have_skills drives a
-- hard gate (an exclusion), and conflating them would let a "nice to have"
-- silently become a requirement.

ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS seniority        TEXT,
    ADD COLUMN IF NOT EXISTS location         TEXT,
    ADD COLUMN IF NOT EXISTS remote_pref      TEXT,
    ADD COLUMN IF NOT EXISTS must_have_skills JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE profiles
    DROP COLUMN IF EXISTS must_have_skills,
    DROP COLUMN IF EXISTS remote_pref,
    DROP COLUMN IF EXISTS location,
    DROP COLUMN IF EXISTS seniority;
