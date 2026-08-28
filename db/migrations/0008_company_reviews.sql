-- +goose Up
-- Company + role review cache. Glassdoor and Mouthshut have no public
-- read APIs; we store outbound search links plus optional Brave Search
-- snippets (an official search API). Never scrape those review sites.

CREATE TABLE company_reviews (
    company_key  TEXT NOT NULL,
    role_key     TEXT NOT NULL DEFAULT '',
    company      TEXT NOT NULL,
    role_title   TEXT NOT NULL DEFAULT '',
    rating       NUMERIC,
    review_count INTEGER NOT NULL DEFAULT 0,
    summary      TEXT NOT NULL DEFAULT '',
    snippets     JSONB NOT NULL DEFAULT '[]'::jsonb,
    links        JSONB NOT NULL DEFAULT '{}'::jsonb,
    provider     TEXT NOT NULL DEFAULT 'links',
    status       TEXT NOT NULL DEFAULT 'pending',
    error        TEXT NOT NULL DEFAULT '',
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_key, role_key)
);

CREATE INDEX company_reviews_fetched ON company_reviews (fetched_at DESC);

-- +goose Down
DROP TABLE IF EXISTS company_reviews;
