// Package reviews looks up employer reputation without scraping.
//
// Glassdoor shut its public partner API (no self-serve keys). Mouthshut
// only publishes write-review widgets, not a read API. Third-party
// scrapers exist; JobSonar does not use them (CLAUDE.md: no scraping as
// a default source).
//
// Instead we always emit outbound search links (Glassdoor, Mouthshut,
// DuckDuckGo) and, when BRAVE_SEARCH_API_KEY is set, pull snippets from
// Brave's official Search API for "{company} {title} employee reviews".
package reviews

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/as76513/JobSonar/services/api/internal/store"
)

const defaultStale = 7 * 24 * time.Hour

// Searcher returns a company+role review brief. Implementations must not
// fetch Glassdoor or Mouthshut HTML.
type Searcher interface {
	Name() string
	Search(ctx context.Context, company, title string) (store.CompanyReview, error)
}

// NewFromEnv picks Brave when a key is set, otherwise link-only.
func NewFromEnv() Searcher {
	if key := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")); key != "" {
		return NewBrave(key)
	}
	return LinkOnly{}
}

// Links builds the three outbound URLs a human (or ATS-safe UI) can open.
func Links(company, title string) store.ReviewLinks {
	qCompany := strings.TrimSpace(company)
	qRole := strings.TrimSpace(title)
	webQ := strings.TrimSpace(qCompany + " " + qRole + " employee reviews Glassdoor OR Mouthshut OR AmbitionBox")
	return store.ReviewLinks{
		Glassdoor: "https://www.glassdoor.com/Search/results.htm?keyword=" + url.QueryEscape(qCompany),
		Mouthshut: "https://www.mouthshut.com/search/prodsrch.aspx?data=" + url.QueryEscape(qCompany),
		WebSearch: "https://duckduckgo.com/?q=" + url.QueryEscape(webQ),
	}
}

func Query(company, title string) string {
	parts := []string{strings.TrimSpace(company)}
	if t := strings.TrimSpace(title); t != "" {
		parts = append(parts, t)
	}
	parts = append(parts, "employee reviews Glassdoor OR Mouthshut OR AmbitionBox")
	return strings.Join(parts, " ")
}

// LinkOnly never hits the network. Used when no search API key is set
// and as the portable local default.
type LinkOnly struct{}

func (LinkOnly) Name() string { return "links" }

func (LinkOnly) Search(_ context.Context, company, title string) (store.CompanyReview, error) {
	return store.CompanyReview{
		Company:   company,
		RoleTitle: title,
		Links:     Links(company, title),
		Provider:  "links",
		Status:    "done",
		Summary:   "No public Glassdoor or Mouthshut API. Open the links to read reviews in a browser.",
		FetchedAt: time.Now().UTC(),
	}, nil
}

// Fake is a deterministic Searcher for tests.
type Fake struct {
	Rating   float64
	Snippets []store.ReviewSnippet
	Err      error
}

func (Fake) Name() string { return "fake" }

func (f Fake) Search(_ context.Context, company, title string) (store.CompanyReview, error) {
	if f.Err != nil {
		return store.CompanyReview{}, f.Err
	}
	rating := f.Rating
	snips := f.Snippets
	if len(snips) == 0 {
		snips = []store.ReviewSnippet{{
			Title:   company + " reviews",
			URL:     "https://example.test/reviews/" + url.PathEscape(company),
			Source:  "example.test",
			Snippet: "Employees rate " + company + " well for the " + title + " role.",
		}}
	}
	return store.CompanyReview{
		Company:   company,
		RoleTitle: title,
		Rating:    &rating,
		Summary:   snips[0].Snippet,
		Snippets:  snips,
		Links:     Links(company, title),
		Provider:  "fake",
		Status:    "done",
		FetchedAt: time.Now().UTC(),
	}, nil
}

const braveURL = "https://api.search.brave.com/res/v1/web/search"

// Brave calls Brave Search (official API, not a scraper).
type Brave struct {
	key    string
	client *http.Client
	base   string
}

func NewBrave(apiKey string) *Brave {
	return &Brave{
		key:    apiKey,
		client: &http.Client{Timeout: 8 * time.Second},
		base:   braveURL,
	}
}

func (b *Brave) Name() string { return "brave" }

type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func (b *Brave) Search(ctx context.Context, company, title string) (store.CompanyReview, error) {
	if b == nil || b.key == "" {
		return LinkOnly{}.Search(ctx, company, title)
	}
	q := url.Values{}
	q.Set("q", Query(company, title))
	q.Set("count", "8")
	q.Set("text_decorations", "false")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.base+"?"+q.Encode(), nil)
	if err != nil {
		return store.CompanyReview{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.key)
	resp, err := b.client.Do(req)
	if err != nil {
		return store.CompanyReview{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return store.CompanyReview{}, err
	}
	if resp.StatusCode >= 300 {
		return store.CompanyReview{}, fmt.Errorf("brave search: HTTP %d", resp.StatusCode)
	}
	return parseBrave(company, title, body)
}

func parseBrave(company, title string, body []byte) (store.CompanyReview, error) {
	var parsed braveResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.CompanyReview{}, err
	}
	snips := make([]store.ReviewSnippet, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		if strings.TrimSpace(r.URL) == "" {
			continue
		}
		snips = append(snips, store.ReviewSnippet{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Source:  hostOf(r.URL),
			Snippet: strings.TrimSpace(r.Description),
		})
	}
	rev := store.CompanyReview{
		Company:   company,
		RoleTitle: title,
		Snippets:  snips,
		Links:     Links(company, title),
		Provider:  "brave",
		Status:    "done",
		FetchedAt: time.Now().UTC(),
	}
	if len(snips) > 0 {
		rev.Summary = snips[0].Snippet
		rev.Rating = extractRating(snips)
	} else {
		rev.Summary = "Search returned no review snippets. Use the Glassdoor / Mouthshut / web links."
	}
	return rev, nil
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

var ratingRe = regexp.MustCompile(`(?i)(\d(?:\.\d)?)\s*(?:/|out of)\s*5`)

func extractRating(snips []store.ReviewSnippet) *float64 {
	for _, s := range snips {
		m := ratingRe.FindStringSubmatch(s.Title + " " + s.Snippet)
		if len(m) < 2 {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(m[1], "%f", &v); err != nil || v < 1 || v > 5 {
			continue
		}
		return &v
	}
	return nil
}

// Stale reports whether a cached row should be refreshed.
func Stale(r *store.CompanyReview, now time.Time) bool {
	if r == nil || r.Status != "done" {
		return true
	}
	if r.FetchedAt.IsZero() {
		return true
	}
	return now.Sub(r.FetchedAt) > defaultStale
}
