package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

// Score mirrors one `scores` row (Week 6). Written by the agent's scoring
// pass (services/agent/jobsonar_agent/run.py:score_jobs); the API only
// reads it -- see docs/TRD.md §4.1's note on where normalisation/scoring
// reasoning lives. Nil on a Job means the agent hasn't scored it yet, not
// that it scored zero.
type Score struct {
	Composite     float64  `json:"composite"`
	SkillCov      float64  `json:"skill_cov"`
	Semantic      *float64 `json:"semantic,omitempty"`
	SeniorityFit  float64  `json:"seniority_fit"`
	LocationFit   float64  `json:"location_fit"`
	Recency       float64  `json:"recency"`
	Band          string   `json:"band"`
	MatchedSkills []string `json:"matched_skills"`
	MissingSkills []string `json:"missing_skills"`
}

type Job struct {
	ID            uuid.UUID   `json:"id"`
	Title         string      `json:"title"`
	Company       string      `json:"company"`
	Location      string      `json:"location"`
	Source        string      `json:"source"`
	SourceURL     string      `json:"source_url"`
	DescriptionMD string      `json:"description_md,omitempty"`
	PostedAt      *time.Time  `json:"posted_at,omitempty"`
	Status        string      `json:"status"`
	LastSeenAt    time.Time   `json:"last_seen_at"`
	Sources       []JobSource `json:"sources"`
	Application   *AppBrief   `json:"application,omitempty"`
	Score         *Score      `json:"score,omitempty"`
}

type AppBrief struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

type Resume struct {
	ID         uuid.UUID `json:"id"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	StorageURI string    `json:"-"`
}

type Profile struct {
	ID           uuid.UUID `json:"id"`
	Skills       []string  `json:"skills"`
	HasEmbedding bool      `json:"has_embedding"`
	Embedding    []float64 `json:"-"`
	LatestResume *Resume   `json:"latest_resume,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Application struct {
	ID            uuid.UUID `json:"id"`
	JobID         uuid.UUID `json:"job_id"`
	Title         string    `json:"title"`
	Company       string    `json:"company"`
	Location      string    `json:"location"`
	SourceURL     string    `json:"source_url"`
	ResumeVariant string    `json:"resume_variant"`
	Status        string    `json:"status"`
	AppliedAt     *time.Time `json:"applied_at,omitempty"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
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

// currentProfileID names the same "single profile, most recently updated"
// selection used throughout (upsert_skills/upsert_profile in the agent;
// the one profile this single-user project targets, per CLAUDE.md).
const currentProfileID = `(SELECT id FROM profiles ORDER BY updated_at DESC LIMIT 1)`

func (s *Store) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.pool.Query(ctx, jobSelect+`
		FROM jobs j
		LEFT JOIN applications a ON a.job_id = j.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(json_build_object('source', src.source, 'source_url', src.source_url)), '[]') AS sources
			FROM job_sources src WHERE src.job_id = j.id
		) srcs ON true
		LEFT JOIN LATERAL (
			SELECT * FROM scores sc
			WHERE sc.job_id = j.id AND sc.profile_id = `+currentProfileID+`
		) sc ON true
		WHERE sc.band IS DISTINCT FROM 'excluded'
		ORDER BY sc.composite DESC NULLS LAST, j.last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (Job, error) {
	// Unlike ListJobs, an excluded (hard-gated) score is still returned
	// here -- a job someone links to directly should explain why it was
	// excluded, not disappear (Week 6 Day 4: never silently dropped).
	row := s.pool.QueryRow(ctx, jobSelect+`
		FROM jobs j
		LEFT JOIN applications a ON a.job_id = j.id
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(json_build_object('source', src.source, 'source_url', src.source_url)), '[]') AS sources
			FROM job_sources src WHERE src.job_id = j.id
		) srcs ON true
		LEFT JOIN LATERAL (
			SELECT * FROM scores sc
			WHERE sc.job_id = j.id AND sc.profile_id = `+currentProfileID+`
		) sc ON true
		WHERE j.id = $1
	`, id)
	j, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return j, err
}

const jobSelect = `
		SELECT j.id, j.title, j.company, j.location, j.source, j.source_url,
		       j.description_md, j.posted_at, j.status, j.last_seen_at,
		       srcs.sources,
		       a.id, a.status,
		       sc.composite, sc.skill_cov, sc.semantic, sc.seniority_fit,
		       sc.location_fit, sc.recency, sc.band, sc.matched_skills, sc.missing_skills
`

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
	var appID *uuid.UUID
	var appStatus *string
	var composite, skillCov, seniorityFit, locationFit, recency *float64
	var semantic *float64
	var band *string
	var matchedRaw, missingRaw []byte
	if err := row.Scan(&j.ID, &j.Title, &j.Company, &j.Location, &j.Source, &j.SourceURL,
		&j.DescriptionMD, &posted, &j.Status, &j.LastSeenAt, &sources, &appID, &appStatus,
		&composite, &skillCov, &semantic, &seniorityFit, &locationFit, &recency, &band, &matchedRaw, &missingRaw); err != nil {
		return Job{}, err
	}
	j.PostedAt = posted
	if err := jsonUnmarshalSources(sources, &j.Sources); err != nil {
		return Job{}, err
	}
	if j.Sources == nil {
		j.Sources = []JobSource{}
	}
	if appID != nil && appStatus != nil {
		j.Application = &AppBrief{ID: *appID, Status: *appStatus}
	}
	if composite != nil && band != nil {
		sc := &Score{
			Composite: *composite, SkillCov: *skillCov, Semantic: semantic,
			SeniorityFit: *seniorityFit, LocationFit: *locationFit, Recency: *recency, Band: *band,
		}
		if err := unmarshalJSON(matchedRaw, &sc.MatchedSkills); err != nil {
			return Job{}, err
		}
		if err := unmarshalJSON(missingRaw, &sc.MissingSkills); err != nil {
			return Job{}, err
		}
		if sc.MatchedSkills == nil {
			sc.MatchedSkills = []string{}
		}
		if sc.MissingSkills == nil {
			sc.MissingSkills = []string{}
		}
		j.Score = sc
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

func FormatVector(v []float64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}

func ParseVector(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		x, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, nil
}
