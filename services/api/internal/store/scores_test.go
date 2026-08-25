package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newTestStore connects to a real Postgres (the docker-compose one, via
// make up && make migrate) and skips if POSTGRES_DSN isn't set, so `go
// test ./...` stays offline by default -- same convention as
// services/worker/internal/store's newTestStore.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_DSN to run store tests against a live Postgres (see .env.example)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// seedJobWithScore inserts a throwaway job and a scores row for it against
// a fresh profile, cleaned up by t.Cleanup.
func seedJobWithScore(t *testing.T, s *Store, profileID uuid.UUID, composite float64, band string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	jobID := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (id, dedup_hash, source, source_url, title, company, location)
		VALUES ($1, $2, 'test', $3, 'Test Job', 'Acme Score Test', 'Pune')
	`, jobID, "score-test-"+jobID.String(), "https://example.com/"+jobID.String())
	if err != nil {
		t.Fatalf("seed job: %v", err)
	}
	t.Cleanup(func() { _, _ = s.pool.Exec(context.Background(), "DELETE FROM jobs WHERE id = $1", jobID) })

	_, err = s.pool.Exec(ctx, `
		INSERT INTO scores (job_id, profile_id, composite, skill_cov, seniority_fit, location_fit, recency, band)
		VALUES ($1, $2, $3, $3, 1, 1, 1, $4)
	`, jobID, profileID, composite, band)
	if err != nil {
		t.Fatalf("seed score: %v", err)
	}
	return jobID
}

func seedProfile(t *testing.T, s *Store) uuid.UUID {
	t.Helper()
	profileID := uuid.New()
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO profiles (id, skills, updated_at) VALUES ($1, '[]'::jsonb, now())
	`, profileID)
	if err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	t.Cleanup(func() { _, _ = s.pool.Exec(context.Background(), "DELETE FROM profiles WHERE id = $1", profileID) })
	return profileID
}

func TestListJobs_OrdersByCompositeAndExcludesGatedBand(t *testing.T) {
	s := newTestStore(t)
	profileID := seedProfile(t, s) // most-recently-updated profile -> "current" for scoring

	strongID := seedJobWithScore(t, s, profileID, 0.90, "strong")
	stretchID := seedJobWithScore(t, s, profileID, 0.20, "stretch")
	excludedID := seedJobWithScore(t, s, profileID, 0.99, "excluded") // high composite, still gated

	jobs, err := s.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}

	var seen []uuid.UUID
	for _, j := range jobs {
		if j.ID == strongID || j.ID == stretchID || j.ID == excludedID {
			seen = append(seen, j.ID)
		}
	}
	if len(seen) != 2 || seen[0] != strongID || seen[1] != stretchID {
		t.Fatalf("want [strong, stretch] (excluded omitted) in that order, got %v", seen)
	}

	// GetJob, unlike ListJobs, must still surface the excluded job -- Week
	// 6 Day 4: gated jobs are never silently dropped, only hidden from
	// the default ranked list.
	excluded, err := s.GetJob(context.Background(), excludedID)
	if err != nil {
		t.Fatalf("GetJob(excluded): %v", err)
	}
	if excluded.Score == nil || excluded.Score.Band != "excluded" {
		t.Fatalf("want excluded job's Score.Band = excluded, got %+v", excluded.Score)
	}
}

func TestListJobs_UnscoredJobHasNilScore(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	jobID := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (id, dedup_hash, source, source_url, title, company, location)
		VALUES ($1, $2, 'test', $3, 'Unscored Job', 'Acme Score Test', 'Pune')
	`, jobID, "score-test-unscored-"+jobID.String(), "https://example.com/"+jobID.String())
	if err != nil {
		t.Fatalf("seed job: %v", err)
	}
	t.Cleanup(func() { _, _ = s.pool.Exec(context.Background(), "DELETE FROM jobs WHERE id = $1", jobID) })

	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Score != nil {
		t.Fatalf("want nil Score for an unscored job, got %+v", job.Score)
	}
}
