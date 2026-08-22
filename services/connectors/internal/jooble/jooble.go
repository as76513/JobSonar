package jooble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

const (
	defaultBaseURL = "https://jooble.org"
	maxRetries     = 4
	maxBackoff     = 30 * time.Second
)

// Client fetches and normalises Jooble search results.
// JOOBLE_BASE_URL must match the region the API key was issued for
// (jooble.org = US; nl.jooble.org = NL).
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
}

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:      apiKey,
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		minInterval: time.Second,
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

func (c *Client) Name() string { return "jooble" }

func (c *Client) RateLimit() connector.RateLimit {
	return connector.RateLimit{Requests: 1, Window: time.Second}
}

func (c *Client) Fetch(ctx context.Context, q connector.SearchParams) ([]connector.RawJob, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("JOOBLE_API_KEY is required")
	}
	var (
		out     []connector.RawJob
		seen    = map[string]struct{}{}
		lastErr error
		anyOK   bool
	)
	for _, p := range connector.ExpandSearches(q) {
		raws, err := c.fetchOne(ctx, p)
		if err != nil {
			lastErr = err
			continue
		}
		anyOK = true
		for _, raw := range raws {
			k := string(raw.Payload)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, raw)
		}
	}
	if !anyOK {
		if lastErr == nil {
			lastErr = fmt.Errorf("no searches ran")
		}
		return nil, lastErr
	}
	return out, nil
}

func (c *Client) fetchOne(ctx context.Context, q connector.SearchParams) ([]connector.RawJob, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	location := q.Where
	if location == "" {
		location = q.Country
	}

	body, err := json.Marshal(map[string]any{
		"keywords":      q.Query,
		"location":      location,
		"page":          strconv.Itoa(page),
		"companysearch": "false",
	})
	if err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/api/" + c.apiKey
	var lastErr error
	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request: %w", err)
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read body: %w", readErr)
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("HTTP 429")
			if err := sleep(ctx, retryAfter(resp.Header, backoff)); err != nil {
				return nil, err
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(raw, 256))
		}

		var parsed searchResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		out := make([]connector.RawJob, 0, len(parsed.Jobs))
		for _, job := range parsed.Jobs {
			out = append(out, connector.RawJob{Source: c.Name(), Payload: job})
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries")
	}
	return nil, lastErr
}

func (c *Client) Normalize(raw connector.RawJob) (connector.Job, error) {
	var ad listing
	if err := json.Unmarshal(raw.Payload, &ad); err != nil {
		return connector.Job{}, fmt.Errorf("normalize: %w", err)
	}
	if ad.Title == "" || ad.Link == "" {
		return connector.Job{}, fmt.Errorf("missing title or link")
	}
	job := connector.Job{
		Source:        c.Name(),
		SourceURL:     ad.Link,
		Title:         ad.Title,
		Company:       ad.Company,
		Location:      ad.Location,
		RemoteType:    remoteType(ad),
		DescriptionMD: ad.Snippet,
	}
	if t, ok := parseTime(ad.Updated); ok {
		job.PostedAt = t
	}
	job.SalaryMin, job.SalaryMax, job.Currency = parseSalary(ad.Salary)
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
	Jobs []json.RawMessage `json:"jobs"`
}

type listing struct {
	Title    string `json:"title"`
	Location string `json:"location"`
	Snippet  string `json:"snippet"`
	Salary   string `json:"salary"`
	Link     string `json:"link"`
	Company  string `json:"company"`
	Updated  string `json:"updated"`
	Type     string `json:"type"`
}

func remoteType(ad listing) string {
	blob := strings.ToLower(ad.Title + " " + ad.Location + " " + ad.Snippet)
	if strings.Contains(blob, "remote") {
		return "remote"
	}
	return ""
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseSalary only accepts clearly numeric ranges like "50000 - 80000 EUR"
// or "50000". Anything else is left unset — do not invent fields.
func parseSalary(s string) (min *float64, max *float64, currency string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil, ""
	}
	fields := strings.Fields(s)
	nums := make([]float64, 0, 2)
	for _, f := range fields {
		f = strings.Trim(f, "-–,")
		f = strings.ReplaceAll(f, ",", "")
		if f == "" {
			continue
		}
		n, err := strconv.ParseFloat(f, 64)
		if err != nil {
			if currency == "" && isCurrencyToken(f) {
				currency = strings.ToUpper(f)
			}
			continue
		}
		nums = append(nums, n)
	}
	if len(nums) == 0 {
		return nil, nil, ""
	}
	min = &nums[0]
	if len(nums) > 1 {
		max = &nums[1]
	}
	return min, max, currency
}

func isCurrencyToken(s string) bool {
	switch strings.ToUpper(s) {
	case "EUR", "USD", "GBP", "INR", "AUD", "CAD", "CHF", "PLN":
		return true
	}
	return false
}

func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil {
		d := time.Duration(secs) * time.Second
		if d > 0 && d < maxBackoff {
			return d
		}
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
