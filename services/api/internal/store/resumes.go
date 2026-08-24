package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateResume(ctx context.Context, storageURI string) (Resume, error) {
	var r Resume
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resumes (storage_uri, status)
		VALUES ($1, 'pending')
		RETURNING id, status, error, created_at
	`, storageURI).Scan(&r.ID, &r.Status, &r.Error, &r.CreatedAt)
	return r, err
}

func (s *Store) LatestResume(ctx context.Context) (Resume, error) {
	var r Resume
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, error, created_at
		FROM resumes
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&r.ID, &r.Status, &r.Error, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resume{}, ErrNotFound
	}
	return r, err
}
