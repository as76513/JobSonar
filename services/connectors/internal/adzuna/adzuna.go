package adzuna

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

const (
	defaultBaseURL = "https://api.adzuna.com"
	defaultCountry = "us"
	defaultPerPage = 20
	maxRetries     = 4
	maxBackoff     = 30 * time.Second
)

// Client fetches and normalises Adzuna search results.
type Client struct {
	appID      string
	appKey     string
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
}

func New(appID, appKey string, opts ...Option) *Client {
	c := &Client{
		appID:       appID,
		appKey:      appKey,
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		minInterval: time.Second, // conservative; 429 backoff is the hard guard
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

func WithMinInterval(d time.Duration) Option {
	return func(c *Client) { c.minInterval = d }
}

func (c *Client) Name() string { return "adzuna" }

func (c *Client) RateLimit() connector.RateLimit {
	return connector.RateLimit{Requests: 1, Window: time.Second}
}

func (c *Client) Fetch(ctx context.Context, q connector.SearchParams) ([]connector.RawJob, error) {
	if c.appID == "" || c.appKey == "" {
		return nil, fmt.Errorf("ADZUNA_APP_ID and ADZUNA_APP_KEY are required")
	}
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}

	country := q.Country
	if country == "" {
		country = defaultCountry
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	perPage := q.PerPage
	if perPage < 1 {
		perPage = defaultPerPage
	}

	u, err := url.Parse(fmt.Sprintf("%s/v1/api/jobs/%s/search/%d", c.baseURL, url.PathEscape(country), page))
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	qs.Set("app_id", c.appID)
	qs.Set("app_key", c.appKey)
	qs.Set("results_per_page", strconv.Itoa(perPage))
	qs.Set("content-type", "application/json")
	if q.Query != "" {
		qs.Set("what", q.Query)
	}
	if q.Where != "" {
		qs.Set("where", q.Where)
	}
	u.RawQuery = qs.Encode()

	var lastErr error
	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read body: %w", readErr)
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			wait := retryAfter(resp.Header, backoff)
			lastErr = fmt.Errorf("HTTP 429")
			if err := sleep(ctx, wait); err != nil {
				return nil, err
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(body, 256))
		}

		var parsed searchResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		out := make([]connector.RawJob, 0, len(parsed.Results))
		for _, raw := range parsed.Results {
			out = append(out, connector.RawJob{Source: c.Name(), Payload: raw})
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries")
	}
	return nil, lastErr
}

func (c *Client) Normalize(raw connector.RawJob) (connector.Job, error) {
	var ad ad
	if err := json.Unmarshal(raw.Payload, &ad); err != nil {
		return connector.Job{}, fmt.Errorf("normalize: %w", err)
	}
	if ad.Title == "" || ad.RedirectURL == "" {
		return connector.Job{}, fmt.Errorf("missing title or redirect_url")
	}

	job := connector.Job{
		Source:        c.Name(),
		SourceURL:     ad.RedirectURL,
		Title:         ad.Title,
		Company:       ad.Company.DisplayName,
		Location:      ad.Location.DisplayName,
		RemoteType:    remoteType(ad),
		DescriptionMD: ad.Description,
		SalaryMin:     ad.SalaryMin,
		SalaryMax:     ad.SalaryMax,
	}
	if ad.Created != "" {
		if t, err := time.Parse(time.RFC3339, ad.Created); err == nil {
			job.PostedAt = t
		}
	}
	return job, nil
}

func (c *Client) throttle(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.minInterval <= 0 || c.lastCall.IsZero() {
		c.lastCall = time.Now()
		return nil
	}
	wait := c.minInterval - time.Since(c.lastCall)
	if wait > 0 {
		if err := sleep(ctx, wait); err != nil {
			return err
		}
	}
	c.lastCall = time.Now()
	return nil
}

type searchResponse struct {
	Results []json.RawMessage `json:"results"`
}

type ad struct {
	ID          json.RawMessage `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Created     string          `json:"created"`
	RedirectURL string          `json:"redirect_url"`
	SalaryMin   *float64        `json:"salary_min"`
	SalaryMax   *float64        `json:"salary_max"`
	Company     struct {
		DisplayName string `json:"display_name"`
	} `json:"company"`
	Location struct {
		DisplayName string `json:"display_name"`
	} `json:"location"`
}

func remoteType(ad ad) string {
	blob := strings.ToLower(strings.Join([]string{
		ad.Title,
		ad.Location.DisplayName,
		ad.Description,
	}, " "))
	if strings.Contains(blob, "remote") {
		return "remote"
	}
	return ""
}

func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(v); err == nil {
		d := time.Duration(secs) * time.Second
		if d > maxBackoff {
			return maxBackoff
		}
		if d < 0 {
			return fallback
		}
		return d
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > maxBackoff {
			return maxBackoff
		}
		if d < 0 {
			return fallback
		}
		return d
	}
	return fallback
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
