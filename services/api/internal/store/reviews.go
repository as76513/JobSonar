package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type ReviewSnippet struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Source  string `json:"source,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type ReviewLinks struct {
	Glassdoor string `json:"glassdoor"`
	Mouthshut string `json:"mouthshut"`
	WebSearch string `json:"web_search"`
}

// CompanyReview is one cached lookup for a company + role. Nil Rating
// means we only have outbound links / snippets, not a parsed score.
type CompanyReview struct {
	Company     string          `json:"company"`
	RoleTitle   string          `json:"role_title,omitempty"`
	Rating      *float64        `json:"rating,omitempty"`
	ReviewCount int             `json:"review_count,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Snippets    []ReviewSnippet `json:"snippets,omitempty"`
	Links       ReviewLinks     `json:"links"`
	Provider    string          `json:"provider"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	FetchedAt   time.Time       `json:"fetched_at"`
}

type CompanyRole struct {
	Company string
	Title   string
}

func CompanyKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (s *Store) UpsertCompanyReview(ctx context.Context, r CompanyReview) (CompanyReview, error) {
	if r.Snippets == nil {
		r.Snippets = []ReviewSnippet{}
	}
	snips, err := json.Marshal(r.Snippets)
	if err != nil {
		return CompanyReview{}, err
	}
	links, err := json.Marshal(r.Links)
	if err != nil {
		return CompanyReview{}, err
	}
	companyKey := CompanyKey(r.Company)
	roleKey := CompanyKey(r.RoleTitle)
	err = s.pool.QueryRow(ctx, `
		INSERT INTO company_reviews (
			company_key, role_key, company, role_title, rating, review_count,
			summary, snippets, links, provider, status, error, fetched_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), now())
		ON CONFLICT (company_key, role_key) DO UPDATE SET
			company = EXCLUDED.company,
			role_title = EXCLUDED.role_title,
			rating = EXCLUDED.rating,
			review_count = EXCLUDED.review_count,
			summary = EXCLUDED.summary,
			snippets = EXCLUDED.snippets,
			links = EXCLUDED.links,
			provider = EXCLUDED.provider,
			status = EXCLUDED.status,
			error = EXCLUDED.error,
			fetched_at = now(),
			updated_at = now()
		RETURNING rating, review_count, summary, snippets, links, provider, status, error, fetched_at
	`, companyKey, roleKey, r.Company, r.RoleTitle, r.Rating, r.ReviewCount,
		r.Summary, snips, links, r.Provider, r.Status, r.Error,
	).Scan(&r.Rating, &r.ReviewCount, &r.Summary, &snips, &links, &r.Provider, &r.Status, &r.Error, &r.FetchedAt)
	if err != nil {
		return CompanyReview{}, err
	}
	if err := unmarshalJSON(snips, &r.Snippets); err != nil {
		return CompanyReview{}, err
	}
	if err := unmarshalJSON(links, &r.Links); err != nil {
		return CompanyReview{}, err
	}
	return r, nil
}

// ListSalaryJobCompanies returns distinct company+title pairs for jobs
// that already have a posted salary (the high-pay shortlist).
func (s *Store) ListSalaryJobCompanies(ctx context.Context) ([]CompanyRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT company, title
		FROM jobs
		WHERE salary_min IS NOT NULL OR salary_max IS NOT NULL
		ORDER BY company, title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CompanyRole
	for rows.Next() {
		var cr CompanyRole
		if err := rows.Scan(&cr.Company, &cr.Title); err != nil {
			return nil, err
		}
		out = append(out, cr)
	}
	if out == nil {
		out = []CompanyRole{}
	}
	return out, rows.Err()
}
