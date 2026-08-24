package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GetProfile(ctx context.Context) (Profile, error) {
	var p Profile
	var raw []byte
	var embedText *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, skills, embedding IS NOT NULL, embedding::text, updated_at
		FROM profiles ORDER BY updated_at DESC LIMIT 1
	`).Scan(&p.ID, &raw, &p.HasEmbedding, &embedText, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{Skills: []string{}}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	if err := json.Unmarshal(raw, &p.Skills); err != nil {
		return Profile{}, err
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if embedText != nil && *embedText != "" {
		v, err := ParseVector(*embedText)
		if err != nil {
			return Profile{}, err
		}
		p.Embedding = v
		p.HasEmbedding = len(v) > 0
	}
	return p, nil
}

func (s *Store) UpsertProfile(ctx context.Context, skills []string) (Profile, error) {
	if skills == nil {
		skills = []string{}
	}
	raw, err := json.Marshal(skills)
	if err != nil {
		return Profile{}, err
	}
	existing, err := s.GetProfile(ctx)
	if err == nil && existing.ID != uuid.Nil {
		var p Profile
		err = s.pool.QueryRow(ctx, `
			UPDATE profiles SET skills = $2, embedding = NULL, updated_at = now() WHERE id = $1
			RETURNING id, skills, updated_at
		`, existing.ID, raw).Scan(&p.ID, &raw, &p.UpdatedAt)
		if err != nil {
			return Profile{}, err
		}
		p.Skills = skills
		return p, nil
	}
	var p Profile
	err = s.pool.QueryRow(ctx, `
		INSERT INTO profiles (skills) VALUES ($1)
		RETURNING id, skills, updated_at
	`, raw).Scan(&p.ID, &raw, &p.UpdatedAt)
	if err != nil {
		return Profile{}, err
	}
	p.Skills = skills
	return p, nil
}
