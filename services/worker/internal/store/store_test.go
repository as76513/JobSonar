package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/as76513/JobSonar/services/worker/internal/dedup"
	"github.com/as76513/JobSonar/services/worker/internal/job"
)

// newTestStore connects to a real Postgres (the docker-compose one, via
// make up && make migrate) and skips if POSTGRES_DSN isn't set, so `go test
// ./...` stays offline by default.
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

// TestUpsert_SameRoleTwoSources_OneJobTwoSources is the Week 2 P0 dedup
// property test: the same role from two different sources/URLs collapses
// to one jobs row with two job_sources rows.
func TestUpsert_SameRoleTwoSources_OneJobTwoSources(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := job.Job{Company: "Acme Dedup Test", Title: "Senior SWE", Location: "Remote"}

	a := base
	a.Source, a.SourceURL = "adzuna", "https://adzuna.example/dedup-test-1"
	b := base
	b.Source, b.SourceURL = "jooble", "https://jooble.example/dedup-test-2"

	hash := dedup.Hash(base) // same company/title/location => same hash for both

	idA, err := s.Upsert(ctx, a, hash)
	if err != nil {
		t.Fatalf("Upsert a: %v", err)
	}
	idB, err := s.Upsert(ctx, b, hash)
	if err != nil {
		t.Fatalf("Upsert b: %v", err)
	}
	if idA != idB {
		t.Fatalf("expected both sources to resolve to the same job, got %s and %s", idA, idB)
	}

	n, err := s.CountJobSources(ctx, idA)
	if err != nil {
		t.Fatalf("CountJobSources: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 job_sources rows, got %d", n)
	}
}

// TestUpsert_Redelivery_IsIdempotent simulates SQS at-least-once delivery:
// processing the identical message twice must not duplicate job_sources.
func TestUpsert_Redelivery_IsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	j := job.Job{Company: "Acme Idempotency Test", Title: "Staff SWE", Location: "Remote", Source: "adzuna", SourceURL: "https://adzuna.example/idempotency-test"}
	hash := dedup.Hash(j)

	id1, err := s.Upsert(ctx, j, hash)
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	id2, err := s.Upsert(ctx, j, hash) // redelivery of the same message
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("redelivery produced a different job id: %s vs %s", id1, id2)
	}

	n, err := s.CountJobSources(ctx, id1)
	if err != nil {
		t.Fatalf("CountJobSources: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected redelivery to not duplicate job_sources, got %d rows", n)
	}
}
