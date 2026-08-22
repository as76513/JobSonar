-- +goose Up
-- Target-company ATS list (FR-5 / Week 3). board_token is the Greenhouse
-- (or later Lever) public board slug, not a secret.

CREATE TABLE companies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    ats         TEXT NOT NULL,
    board_token TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ats, board_token)
);

-- +goose Down
DROP TABLE IF EXISTS companies;
