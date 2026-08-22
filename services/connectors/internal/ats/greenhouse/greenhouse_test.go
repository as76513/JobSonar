package greenhouse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/as76513/JobSonar/services/connectors/internal/companies"
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
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Query().Get("content") != "true" {
			t.Error("expected content=true")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	src := companies.Static{{Name: "Stripe", ATS: "greenhouse", Token: "stripe"}}
	c := New(src, WithBaseURL(srv.URL), WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/boards/stripe/jobs" {
		t.Fatalf("path=%q", gotPath)
	}
	if len(raws) != 3 {
		t.Fatalf("jobs=%d", len(raws))
	}

	first, err := c.Normalize(raws[0])
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != "greenhouse" || first.Company != "Stripe" {
		t.Fatalf("%+v", first)
	}
	if first.Title != "Software Engineer, Infrastructure" || first.Location != "Amsterdam" {
		t.Fatalf("%+v", first)
	}
	if first.SourceURL != "https://boards.greenhouse.io/stripe/jobs/4034987002" {
		t.Fatalf("url=%s", first.SourceURL)
	}
	wantPosted := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	if !first.PostedAt.Equal(wantPosted) {
		t.Fatalf("posted_at=%s want first_published %s", first.PostedAt, wantPosted)
	}

	remote, err := c.Normalize(raws[1])
	if err != nil {
		t.Fatal(err)
	}
	if remote.RemoteType != "remote" {
		t.Errorf("remote_type=%q", remote.RemoteType)
	}
}

func TestFetchFiltersByRoleQuery(t *testing.T) {
	body := fixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	src := companies.Static{{Name: "Stripe", ATS: "greenhouse", Token: "stripe"}}
	c := New(src, WithBaseURL(srv.URL), WithMinInterval(0))

	cases := []struct {
		query string
		where string
		want  int
		title string
	}{
		{query: "Software Engineer", want: 1, title: "Software Engineer, Infrastructure"},
		{query: "DevOps", want: 2},
		{query: "DevOps Lead, DevSecOps Engineer", want: 2},
		{query: "DevOps", where: "Amsterdam", want: 1, title: "Software Engineer, Infrastructure"},
		{query: "DevOps", where: "Pune", want: 0},
	}
	for _, tc := range cases {
		raws, err := c.Fetch(context.Background(), connector.SearchParams{Query: tc.query, Where: tc.where})
		if err != nil {
			t.Fatalf("%q where=%q: %v", tc.query, tc.where, err)
		}
		if len(raws) != tc.want {
			t.Fatalf("%q where=%q: jobs=%d want %d", tc.query, tc.where, len(raws), tc.want)
		}
		if tc.want == 1 {
			got, err := c.Normalize(raws[0])
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != tc.title {
				t.Fatalf("%q: title=%q want %q", tc.query, got.Title, tc.title)
			}
		}
		for _, raw := range raws {
			got, err := c.Normalize(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Title == "Account Executive" {
				t.Fatalf("%q matched sales role from description", tc.query)
			}
		}
	}
}

func TestFetchEmptyBoards(t *testing.T) {
	c := New(companies.Static{}, WithMinInterval(0))
	raws, err := c.Fetch(context.Background(), connector.SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(raws) != 0 {
		t.Fatalf("got %d", len(raws))
	}
}

func TestNormalizeRejectsIncomplete(t *testing.T) {
	c := New(nil)
	_, err := c.Normalize(connector.RawJob{
		Payload: []byte(`{"company":"X","job":{"title":"Y"}}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
