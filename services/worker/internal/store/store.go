// Package store persists normalized jobs (db/migrations/0001_jobs.sql:
// jobs + job_sources).
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/as76513/JobSonar/services/worker/internal/job"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// Upsert persists a normalized job under dedupHash. A first sighting inserts
// a jobs row (first_seen_at = last_seen_at = now(), per the column
// defaults); a repeat sighting only advances last_seen_at, leaving
// first_seen_at untouched — the ON CONFLICT branch never mentions it. The
// source/source_url pair is recorded in job_sources so the same role seen
// from two sources collapses to one jobs row (NFR-4: idempotent upserts).
func (s *Store) Upsert(ctx context.Context, j job.Job, dedupHash string) (uuid.UUID, error) {
	var jobID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO jobs (dedup_hash, source, source_url, title, company, location, remote_type, description_md, salary_min, salary_max, currency, posted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (dedup_hash) DO UPDATE SET last_seen_at = now()
		RETURNING id
	`, dedupHash, j.Source, j.SourceURL, j.Title, j.Company, j.Location, j.RemoteType, j.DescriptionMD,
		j.SalaryMin, j.SalaryMax, nullableString(j.Currency), nullableTime(j.PostedAt),
	).Scan(&jobID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert job: %w", err)
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO job_sources (job_id, source, source_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, source, source_url) DO NOTHING
	`, jobID, j.Source, j.SourceURL); err != nil {
		return uuid.Nil, fmt.Errorf("upsert job_source: %w", err)
	}

	return jobID, nil
}

// CountJobSources reports how many job_sources rows exist for a job — used
// by the dedup property test to confirm two sources collapse to one job.
func (s *Store) CountJobSources(ctx context.Context, jobID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM job_sources WHERE job_id = $1`, jobID).Scan(&n)
	return n, err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
