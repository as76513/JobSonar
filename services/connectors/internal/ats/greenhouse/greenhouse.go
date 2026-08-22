package greenhouse

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

	"github.com/as76513/JobSonar/services/connectors/internal/companies"
	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

const (
	defaultBaseURL = "https://boards-api.greenhouse.io"
	maxRetries     = 4
	maxBackoff     = 30 * time.Second
)

// BoardSource lists Greenhouse boards to fetch. Production uses the
// companies table; tests inject a static list.
type BoardSource interface {
	ListByATS(ctx context.Context, ats string) ([]companies.Board, error)
}

// Client fetches public Greenhouse Job Board JSON. No API key.
type Client struct {
	boards     BoardSource
	baseURL    string
	httpClient *http.Client

	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
}

func New(boards BoardSource, opts ...Option) *Client {
	c := &Client{
		boards:      boards,
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
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

func (c *Client) Name() string { return "greenhouse" }

func (c *Client) RateLimit() connector.RateLimit {
	return connector.RateLimit{Requests: 1, Window: time.Second}
}

func (c *Client) Fetch(ctx context.Context, q connector.SearchParams) ([]connector.RawJob, error) {
	if c.boards == nil {
		return nil, nil
	}
	list, err := c.boards.ListByATS(ctx, "greenhouse")
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}

	var out []connector.RawJob
	for _, board := range list {
		jobs, err := c.fetchBoardFiltered(ctx, board, q)
		if err != nil {
			return nil, err
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func (c *Client) fetchBoardFiltered(ctx context.Context, board companies.Board, q connector.SearchParams) ([]connector.RawJob, error) {
	needFilter := strings.TrimSpace(q.Query) != "" || strings.TrimSpace(q.Where) != ""
	if !needFilter {
		return c.fetchBoard(ctx, board, true)
	}
	listed, err := c.fetchBoard(ctx, board, false)
	if err != nil {
		return nil, err
	}
	kept := filterJobs(listed, q.Query, q.Where)
	if len(kept) == 0 {
		return nil, nil
	}
	full, err := c.fetchBoard(ctx, board, true)
	if err != nil {
		return nil, err
	}
	want := map[int64]struct{}{}
	for _, raw := range kept {
		if ad, ok := envelopePost(raw.Payload); ok {
			want[ad.ID] = struct{}{}
		}
	}
	var out []connector.RawJob
	for _, raw := range full {
		ad, ok := envelopePost(raw.Payload)
		if !ok {
			continue
		}
		if _, ok := want[ad.ID]; ok {
			out = append(out, raw)
		}
	}
	return out, nil
}

func (c *Client) fetchBoard(ctx context.Context, board companies.Board, withContent bool) ([]connector.RawJob, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	u, err := url.Parse(fmt.Sprintf("%s/v1/boards/%s/jobs", c.baseURL, url.PathEscape(board.Token)))
	if err != nil {
		return nil, err
	}
	qs := u.Query()
	if withContent {
		qs.Set("content", "true")
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
			lastErr = fmt.Errorf("HTTP 429")
			if err := sleep(ctx, retryAfter(resp.Header, backoff)); err != nil {
				return nil, err
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("board %s: HTTP %d: %s", board.Token, resp.StatusCode, truncate(body, 256))
		}

		var parsed boardResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		out := make([]connector.RawJob, 0, len(parsed.Jobs))
		for _, raw := range parsed.Jobs {
			envelope, err := json.Marshal(rawJob{
				Company: board.Name,
				Job:     raw,
			})
			if err != nil {
				return nil, err
			}
			out = append(out, connector.RawJob{Source: c.Name(), Payload: envelope})
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries")
	}
	return nil, lastErr
}

func (c *Client) Normalize(raw connector.RawJob) (connector.Job, error) {
	var env rawJob
	if err := json.Unmarshal(raw.Payload, &env); err != nil {
		return connector.Job{}, fmt.Errorf("normalize: %w", err)
	}
	var ad post
	if err := json.Unmarshal(env.Job, &ad); err != nil {
		return connector.Job{}, fmt.Errorf("normalize job: %w", err)
	}
	if ad.Title == "" || ad.AbsoluteURL == "" {
		return connector.Job{}, fmt.Errorf("missing title or absolute_url")
	}
	job := connector.Job{
		Source:        c.Name(),
		SourceURL:     ad.AbsoluteURL,
		Title:         ad.Title,
		Company:       env.Company,
		Location:      ad.Location.Name,
		RemoteType:    remoteType(ad),
		DescriptionMD: ad.Content,
	}
	if t, ok := postedAt(ad); ok {
		job.PostedAt = t
	}
	return job, nil
}

func postedAt(ad post) (time.Time, bool) {
	for _, s := range []string{ad.FirstPublished, ad.UpdatedAt} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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

type boardResponse struct {
	Jobs []json.RawMessage `json:"jobs"`
}

type rawJob struct {
	Company string          `json:"company"`
	Job     json.RawMessage `json:"job"`
}

type post struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	AbsoluteURL    string `json:"absolute_url"`
	FirstPublished string `json:"first_published"`
	UpdatedAt      string `json:"updated_at"`
	Content        string `json:"content"`
	Location       struct {
		Name string `json:"name"`
	} `json:"location"`
	Departments []struct {
		Name string `json:"name"`
	} `json:"departments"`
	Offices []struct {
		Name     string `json:"name"`
		Location string `json:"location"`
	} `json:"offices"`
}

func remoteType(ad post) string {
	blob := strings.ToLower(ad.Title + " " + ad.Location.Name)
	if strings.Contains(blob, "remote") {
		return "remote"
	}
	return ""
}

func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	if secs, err := strconv.Atoi(h.Get("Retry-After")); err == nil {
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
