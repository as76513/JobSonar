package jooble

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
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if got := r.URL.Path; got != "/api/test-key" {
			t.Errorf("path=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New("test-key", WithBaseURL(srv.URL), WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{
		Query: "DevOps",
		Where: "Amsterdam",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(raws) != 2 {
		t.Fatalf("Fetch got %d jobs, want 2", len(raws))
	}

	first, err := c.Normalize(raws[0])
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != "jooble" || first.Title != "DevOps Engineer" || first.Company != "Example Labs" {
		t.Fatalf("first=%+v", first)
	}
	if first.Location != "Amsterdam" || first.SourceURL != "https://jooble.org/desc/1001" {
		t.Fatalf("first loc/url=%+v", first)
	}
	if first.SalaryMin == nil || *first.SalaryMin != 70000 || first.Currency != "EUR" {
		t.Fatalf("salary=%v %v %q", first.SalaryMin, first.SalaryMax, first.Currency)
	}

	remote, err := c.Normalize(raws[1])
	if err != nil {
		t.Fatal(err)
	}
	if remote.RemoteType != "remote" {
		t.Errorf("remote_type=%q", remote.RemoteType)
	}
}

func TestFetchBackoffOn429(t *testing.T) {
	body := fixture(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	c := New("key", WithBaseURL(srv.URL), WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() < 2 || len(raws) != 2 {
		t.Fatalf("hits=%d jobs=%d", hits.Load(), len(raws))
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"denied"}`)
	}))
	t.Cleanup(srv.Close)

	c := New("key", WithBaseURL(srv.URL), WithMinInterval(0))
	if _, err := c.Fetch(context.Background(), connector.SearchParams{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMissingKey(t *testing.T) {
	if _, err := New("").Fetch(context.Background(), connector.SearchParams{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRateLimit(t *testing.T) {
	rl := New("key").RateLimit()
	if rl.Requests != 1 || rl.Window != time.Second {
		t.Fatalf("%+v", rl)
	}
}

func TestParseSalarySkipsJunk(t *testing.T) {
	min, max, cur := parseSalary("Competitive")
	if min != nil || max != nil || cur != "" {
		t.Fatalf("invented salary from junk: %v %v %q", min, max, cur)
	}
}
