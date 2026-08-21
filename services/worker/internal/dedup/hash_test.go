package dedup

import (
	"testing"

	"github.com/as76513/JobSonar/services/worker/internal/job"
)

func TestHash_SameRoleTwoSources_SameHash(t *testing.T) {
	a := job.Job{Company: "Acme", Title: "Senior SWE", Location: "Remote", Source: "adzuna", SourceURL: "https://adzuna.example/1"}
	b := job.Job{Company: "  ACME  ", Title: "senior   swe", Location: "remote", Source: "jooble", SourceURL: "https://jooble.example/2"}

	if Hash(a) != Hash(b) {
		t.Fatalf("expected same dedup_hash for the same role from two sources, got %q and %q", Hash(a), Hash(b))
	}
}

func TestHash_DifferentRole_DifferentHash(t *testing.T) {
	a := job.Job{Company: "Acme", Title: "Senior SWE", Location: "Remote"}
	b := job.Job{Company: "Acme", Title: "Staff SWE", Location: "Remote"}

	if Hash(a) == Hash(b) {
		t.Fatalf("expected different dedup_hash for different titles, got the same %q", Hash(a))
	}
}
