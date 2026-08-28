package reviews

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/as76513/JobSonar/services/api/internal/store"
)

func TestLinksAndQuery(t *testing.T) {
	links := Links("Dkatalis Labs", "DevOps Engineer")
	if !strings.Contains(links.Glassdoor, "Dkatalis") {
		t.Fatalf("glassdoor: %s", links.Glassdoor)
	}
	if !strings.Contains(links.Mouthshut, "Dkatalis") {
		t.Fatalf("mouthshut: %s", links.Mouthshut)
	}
	if !strings.Contains(links.WebSearch, "reviews") {
		t.Fatalf("web: %s", links.WebSearch)
	}
	q := Query("Acme", "SRE")
	if !strings.Contains(q, "Acme") || !strings.Contains(q, "SRE") || !strings.Contains(q, "reviews") {
		t.Fatalf("query: %s", q)
	}
}

func TestLinkOnlyDoesNotNeedNetwork(t *testing.T) {
	rev, err := LinkOnly{}.Search(context.Background(), "Acme", "SRE")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Provider != "links" || rev.Status != "done" || rev.Links.Glassdoor == "" {
		t.Fatalf("%+v", rev)
	}
}

func TestFakeSearch(t *testing.T) {
	rev, err := Fake{Rating: 4.2}.Search(context.Background(), "Acme", "SRE")
	if err != nil || rev.Rating == nil || *rev.Rating != 4.2 {
		t.Fatalf("%+v %v", rev, err)
	}
}

func TestParseBraveAndRating(t *testing.T) {
	body := []byte(`{
		"web": {"results": [
			{"title": "Acme reviews 4.3 / 5", "url": "https://www.glassdoor.com/Reviews/acme", "description": "Employees rate Acme 4.3 out of 5 for the SRE team."},
			{"title": "Other", "url": "https://news.example/acme", "description": "Hiring news"}
		]}
	}`)
	rev, err := parseBrave("Acme", "SRE", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rev.Snippets) != 2 || rev.Snippets[0].Source != "glassdoor.com" {
		t.Fatalf("snippets: %+v", rev.Snippets)
	}
	if rev.Rating == nil || *rev.Rating != 4.3 {
		t.Fatalf("rating=%v", rev.Rating)
	}
}

func TestBraveUsesHTTPFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "test-key" {
			t.Errorf("missing token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Ok","url":"https://example.test/r","description":"snippet"}]}}`))
	}))
	defer srv.Close()
	b := NewBrave("test-key")
	b.base = srv.URL
	rev, err := b.Search(context.Background(), "Acme", "SRE")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Provider != "brave" || len(rev.Snippets) != 1 {
		t.Fatalf("%+v", rev)
	}
}

func TestStale(t *testing.T) {
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	if !Stale(nil, now) {
		t.Fatal("nil should be stale")
	}
	fresh := &store.CompanyReview{Status: "done", FetchedAt: now.Add(-time.Hour)}
	if Stale(fresh, now) {
		t.Fatal("hour-old done row should be fresh")
	}
	old := &store.CompanyReview{Status: "done", FetchedAt: now.Add(-8 * 24 * time.Hour)}
	if !Stale(old, now) {
		t.Fatal("8-day row should be stale")
	}
}
