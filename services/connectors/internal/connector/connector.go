package connector

import (
	"context"
	"encoding/json"
	"time"
)

// Connector is the source contract (TRD §4.1). New sources implement this
// interface only; Fetch failures are isolated by Registry so one down source
// cannot take the ingest path down.
type Connector interface {
	Name() string
	Fetch(ctx context.Context, q SearchParams) ([]RawJob, error)
	Normalize(raw RawJob) (Job, error)
	RateLimit() RateLimit
}

// SearchParams is the query a connector receives. Sources ignore fields they
// do not support.
type SearchParams struct {
	Query   string
	Where   string
	Country string
	Page    int
	PerPage int
}

// RawJob is an opaque source payload. The worker (Week 2) persists the
// normalised Job; connectors must not compute dedup_hash.
type RawJob struct {
	Source  string          `json:"source"`
	Payload json.RawMessage `json:"payload"`
}

// Job is the unified in-memory schema (TRD §3), minus worker-owned fields
// (id, dedup_hash, first_seen_at, last_seen_at, status, skills_extracted).
type Job struct {
	Source        string    `json:"source"`
	SourceURL     string    `json:"source_url"`
	Title         string    `json:"title"`
	Company       string    `json:"company"`
	Location      string    `json:"location"`
	RemoteType    string    `json:"remote_type"`
	DescriptionMD string    `json:"description_md"`
	SalaryMin     *float64  `json:"salary_min,omitempty"`
	SalaryMax     *float64  `json:"salary_max,omitempty"`
	Currency      string    `json:"currency,omitempty"`
	PostedAt      time.Time `json:"posted_at,omitempty"`
}

// RateLimit is the source's documented (or conservative) request budget.
// Connectors must also back off on HTTP 429 (NFR-7).
type RateLimit struct {
	Requests int
	Window   time.Duration
}
