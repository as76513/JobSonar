package connector_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

type stub struct {
	name      string
	fetchErr  error
	raws      []connector.RawJob
	normErrOn string
}

func (s stub) Name() string { return s.name }

func (s stub) Fetch(context.Context, connector.SearchParams) ([]connector.RawJob, error) {
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.raws, nil
}

func (s stub) Normalize(raw connector.RawJob) (connector.Job, error) {
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw.Payload, &payload); err != nil {
		return connector.Job{}, err
	}
	if s.normErrOn != "" && payload.Title == s.normErrOn {
		return connector.Job{}, errors.New("normalize failed")
	}
	return connector.Job{Source: s.name, Title: payload.Title, SourceURL: "https://example.test/" + payload.Title}, nil
}

func (s stub) RateLimit() connector.RateLimit { return connector.RateLimit{} }

func TestRegistryIsolatesFetchFailure(t *testing.T) {
	good := stub{
		name: "good",
		raws: []connector.RawJob{{Source: "good", Payload: []byte(`{"title":"ok"}`)}},
	}
	bad := stub{name: "bad", fetchErr: errors.New("source down")}

	results := connector.NewRegistry(bad, good).Run(context.Background(), connector.SearchParams{})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	var sawErr, sawJobs bool
	for _, r := range results {
		switch r.Name {
		case "bad":
			if r.Err == nil {
				t.Fatal("expected fetch error from bad source")
			}
			if len(r.Jobs) != 0 {
				t.Fatalf("failing source should not return jobs, got %d", len(r.Jobs))
			}
			sawErr = true
		case "good":
			if r.Err != nil {
				t.Fatalf("good source should not fail: %v", r.Err)
			}
			if len(r.Jobs) != 1 || r.Jobs[0].Title != "ok" {
				t.Fatalf("good source jobs = %+v", r.Jobs)
			}
			sawJobs = true
		}
	}
	if !sawErr || !sawJobs {
		t.Fatalf("missing expected results: err=%v jobs=%v", sawErr, sawJobs)
	}
}

func TestRegistrySkipsBadRecords(t *testing.T) {
	s := stub{
		name:      "src",
		normErrOn: "skip-me",
		raws: []connector.RawJob{
			{Source: "src", Payload: []byte(`{"title":"keep"}`)},
			{Source: "src", Payload: []byte(`{"title":"skip-me"}`)},
		},
	}
	results := connector.NewRegistry(s).Run(context.Background(), connector.SearchParams{})
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("source should succeed: %v", results[0].Err)
	}
	if len(results[0].Jobs) != 1 || results[0].Jobs[0].Title != "keep" {
		t.Fatalf("jobs = %+v", results[0].Jobs)
	}
}
