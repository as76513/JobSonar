// Package dedup computes the TRD §3 dedup_hash: the same role posted by two
// sources must land as one jobs row, distinguished only by job_sources.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/as76513/JobSonar/services/worker/internal/job"
)

// Hash returns dedup_hash = sha256(normalize(company) + "|" + normalize(title) + "|" + normalize(location)),
// hex-encoded. normalize() lowercases, trims, and collapses internal
// whitespace so formatting differences between sources don't defeat dedup.
func Hash(j job.Job) string {
	sum := sha256.Sum256([]byte(normalize(j.Company) + "|" + normalize(j.Title) + "|" + normalize(j.Location)))
	return hex.EncodeToString(sum[:])
}

func normalize(s string) string {
	fields := strings.FieldsFunc(s, unicode.IsSpace)
	return strings.ToLower(strings.Join(fields, " "))
}
