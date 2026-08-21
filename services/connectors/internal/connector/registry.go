package connector

import (
	"context"
	"fmt"
)

// Result is the outcome of running a single connector. Err set means that
// source produced no jobs; other sources in the same Run are unaffected.
type Result struct {
	Name string
	Jobs []Job
	Err  error
}

// Registry holds connectors and runs them in isolation.
type Registry struct {
	connectors []Connector
}

func NewRegistry(cs ...Connector) *Registry {
	return &Registry{connectors: append([]Connector(nil), cs...)}
}

func (r *Registry) Register(c Connector) {
	r.connectors = append(r.connectors, c)
}

// Run fetches and normalises every registered source. A failing Fetch is
// recorded on that Result and the loop continues. A failing Normalize skips
// that one record rather than failing the source.
func (r *Registry) Run(ctx context.Context, q SearchParams) []Result {
	out := make([]Result, 0, len(r.connectors))
	for _, c := range r.connectors {
		res := Result{Name: c.Name()}
		raws, err := c.Fetch(ctx, q)
		if err != nil {
			res.Err = fmt.Errorf("%s: fetch: %w", c.Name(), err)
			out = append(out, res)
			continue
		}
		jobs := make([]Job, 0, len(raws))
		for _, raw := range raws {
			job, err := c.Normalize(raw)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}
		res.Jobs = jobs
		out = append(out, res)
	}
	return out
}
