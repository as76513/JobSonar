package adzuna

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "response.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFetchNormalizeRecordedFixture(t *testing.T) {
	body := fixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("app_id") == "" || r.URL.Query().Get("app_key") == "" {
			t.Error("expected app_id and app_key query params")
		}
		if got := r.URL.Query().Get("what"); got != "javascript developer" {
			t.Errorf("what=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New("test-id", "test-key", WithBaseURL(srv.URL), WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{
		Query:   "javascript developer",
		Country: "gb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(raws) != 3 {
		t.Fatalf("Fetch got %d jobs, want 3", len(raws))
	}

	jobs := make([]connector.Job, 0, len(raws))
	for _, raw := range raws {
		job, err := c.Normalize(raw)
		if err != nil {
			t.Fatalf("Normalize: %v", err)
		}
		jobs = append(jobs, job)
	}

	first := jobs[0]
	if first.Source != "adzuna" {
		t.Errorf("source=%q", first.Source)
	}
	if first.Title != "Javascript Developer" {
		t.Errorf("title=%q", first.Title)
	}
	if first.Company != "Corporate Project Solutions" {
		t.Errorf("company=%q", first.Company)
	}
	if first.Location != "Marlow, Buckinghamshire" {
		t.Errorf("location=%q", first.Location)
	}
	if first.SourceURL != "http://adzuna.co.uk/jobs/land/ad/129698749" {
		t.Errorf("source_url=%q", first.SourceURL)
	}
	if first.RemoteType != "" {
		t.Errorf("remote_type=%q, want empty", first.RemoteType)
	}
	if first.SalaryMin == nil || *first.SalaryMin != 50000 {
		t.Errorf("salary_min=%v", first.SalaryMin)
	}
	wantPosted := time.Date(2013, 11, 8, 18, 7, 39, 0, time.UTC)
	if !first.PostedAt.Equal(wantPosted) {
		t.Errorf("posted_at=%s, want %s", first.PostedAt, wantPosted)
	}
	if first.DescriptionMD == "" {
		t.Error("expected description_md")
	}

	remote := jobs[2]
	if remote.Title != "Staff Platform Engineer (Remote)" {
		t.Errorf("remote title=%q", remote.Title)
	}
	if remote.RemoteType != "remote" {
		t.Errorf("remote_type=%q, want remote", remote.RemoteType)
	}
}

func TestFetchBackoffOn429(t *testing.T) {
	body := fixture(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New("id", "key", WithBaseURL(srv.URL), WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected retry after 429, hits=%d", hits.Load())
	}
	if len(raws) != 3 {
		t.Fatalf("got %d jobs after retry", len(raws))
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"denied"}`)
	}))
	t.Cleanup(srv.Close)

	c := New("id", "key", WithBaseURL(srv.URL), WithMinInterval(0))
	_, err := c.Fetch(context.Background(), connector.SearchParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeRejectsIncomplete(t *testing.T) {
	c := New("id", "key")
	_, err := c.Normalize(connector.RawJob{Source: "adzuna", Payload: []byte(`{"title":"x"}`)})
	if err == nil {
		t.Fatal("expected error for missing redirect_url")
	}
}

func TestRateLimit(t *testing.T) {
	rl := New("id", "key").RateLimit()
	if rl.Requests != 1 || rl.Window != time.Second {
		t.Fatalf("RateLimit() = %+v", rl)
	}
}
