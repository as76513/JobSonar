package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type JobSource struct {
	Source    string `json:"source"`
	SourceURL string `json:"source_url"`
}

type Job struct {
	ID         uuid.UUID   `json:"id"`
	Title      string      `json:"title"`
	Company    string      `json:"company"`
	Location   string      `json:"location"`
	Source     string      `json:"source"`
	SourceURL  string      `json:"source_url"`
	PostedAt   *time.Time  `json:"posted_at,omitempty"`
	Status     string      `json:"status"`
	LastSeenAt time.Time   `json:"last_seen_at"`
	Sources    []JobSource `json:"sources"`
}

type Company struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ATS        string    `json:"ats"`
	BoardToken string    `json:"board_token"`
	CreatedAt  time.Time `json:"created_at"`
}

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

func (s *Store) Close() { s.pool.Close() }

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.title, j.company, j.location, j.source, j.source_url,
		       j.posted_at, j.status, j.last_seen_at,
		       COALESCE(
		         json_agg(json_build_object('source', s.source, 'source_url', s.source_url))
		         FILTER (WHERE s.source IS NOT NULL),
		         '[]'
		       )
		FROM jobs j
		LEFT JOIN job_sources s ON s.job_id = j.id
		GROUP BY j.id
		ORDER BY j.last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (Job, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT j.id, j.title, j.company, j.location, j.source, j.source_url,
		       j.posted_at, j.status, j.last_seen_at,
		       COALESCE(
		         json_agg(json_build_object('source', s.source, 'source_url', s.source_url))
		         FILTER (WHERE s.source IS NOT NULL),
		         '[]'
		       )
		FROM jobs j
		LEFT JOIN job_sources s ON s.job_id = j.id
		WHERE j.id = $1
		GROUP BY j.id
	`, id)
	j, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return j, err
}

func (s *Store) CreateCompany(ctx context.Context, name, ats, token string) (Company, error) {
	var c Company
	err := s.pool.QueryRow(ctx, `
		INSERT INTO companies (name, ats, board_token)
		VALUES ($1, $2, $3)
		ON CONFLICT (ats, board_token) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, ats, board_token, created_at
	`, name, ats, token).Scan(&c.ID, &c.Name, &c.ATS, &c.BoardToken, &c.CreatedAt)
	return c, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var j Job
	var posted *time.Time
	var sources []byte
	if err := row.Scan(&j.ID, &j.Title, &j.Company, &j.Location, &j.Source, &j.SourceURL,
		&posted, &j.Status, &j.LastSeenAt, &sources); err != nil {
		return Job{}, err
	}
	j.PostedAt = posted
	if err := jsonUnmarshalSources(sources, &j.Sources); err != nil {
		return Job{}, err
	}
	if j.Sources == nil {
		j.Sources = []JobSource{}
	}
	return j, nil
}

func scanJobs(rows pgx.Rows) ([]Job, error) {
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if out == nil {
		out = []Job{}
	}
	return out, rows.Err()
}

func jsonUnmarshalSources(b []byte, dest *[]JobSource) error {
	if len(b) == 0 {
		*dest = []JobSource{}
		return nil
	}
	return unmarshalJSON(b, dest)
}
