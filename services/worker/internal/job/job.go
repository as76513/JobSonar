// Package job is the worker's copy of the unified Job schema (TRD §3),
// deliberately duplicated from the connectors module rather than shared:
// worker and connectors are independent Go modules (PROJECT_STRUCTURE.md),
// and the wire format (JSON on the SQS message body) is the real contract
// between them, not a shared Go type.
package job

import "time"

// Job mirrors connector.Job's JSON shape exactly. The worker never
// recomputes source-specific normalization — it only hashes and persists
// what the connector already normalized.
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
