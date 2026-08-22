package companies

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Board is one ATS career-site token the Greenhouse (or later Lever)
// connector should fetch.
type Board struct {
	Name  string
	ATS   string
	Token string
}

// Store reads the companies table. Connectors use this instead of calling
// the HTTP API (Go services share only the queue and Postgres).
type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required to list ATS boards")
	}
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
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) ListByATS(ctx context.Context, ats string) ([]Board, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT name, ats, board_token FROM companies WHERE ats = $1 ORDER BY name
	`, ats)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.Name, &b.ATS, &b.Token); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Static is an in-memory BoardSource for tests (and a missing-DSN no-op).
type Static []Board

func (s Static) ListByATS(_ context.Context, ats string) ([]Board, error) {
	var out []Board
	for _, b := range s {
		if b.ATS == ats || ats == "" {
			out = append(out, b)
		}
	}
	return out, nil
}
