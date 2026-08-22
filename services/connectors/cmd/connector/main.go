package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/as76513/JobSonar/services/connectors/internal/adzuna"
	"github.com/as76513/JobSonar/services/connectors/internal/ats/greenhouse"
	"github.com/as76513/JobSonar/services/connectors/internal/companies"
	"github.com/as76513/JobSonar/services/connectors/internal/connector"
	"github.com/as76513/JobSonar/services/connectors/internal/jooble"
	"github.com/as76513/JobSonar/services/connectors/internal/queue"
)

func main() {
	what := flag.String("what", env("ADZUNA_WHAT", "software engineer"), "comma-separated roles (Adzuna/Jooble search; Greenhouse title OR)")
	where := flag.String("where", env("ADZUNA_WHERE", ""), "comma-separated cities; zip with -country when counts match")
	country := flag.String("country", env("ADZUNA_COUNTRY", "us"), "comma-separated Adzuna country codes (us, gb, nl, in, ...)")
	page := flag.Int("page", envInt("ADZUNA_PAGE", 1), "result page (1-indexed)")
	perPage := flag.Int("per-page", envInt("ADZUNA_PER_PAGE", 20), "results per page")
	sink := flag.String("sink", env("CONNECTOR_SINK", "stdout"), "where normalized jobs go: stdout or sqs")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	conns, closeBoards := buildConnectors(ctx)
	defer closeBoards()
	reg := connector.NewRegistry(conns...)

	results := reg.Run(ctx, connector.SearchParams{
		Query:   *what,
		Where:   *where,
		Country: *country,
		Page:    *page,
		PerPage: *perPage,
	})

	publish, err := newPublishFunc(ctx, *sink)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sink %q: %v\n", *sink, err)
		os.Exit(1)
	}

	anyOK := false
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", r.Name, r.Err)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %d job(s)\n", r.Name, len(r.Jobs))
		for _, job := range r.Jobs {
			if err := publish(job); err != nil {
				fmt.Fprintf(os.Stderr, "publish: %v\n", err)
				os.Exit(1)
			}
			anyOK = true
		}
	}
	if !anyOK {
		os.Exit(1)
	}
}

func buildConnectors(ctx context.Context) ([]connector.Connector, func()) {
	closeFn := func() {}
	out := []connector.Connector{
		adzuna.New(os.Getenv("ADZUNA_APP_ID"), os.Getenv("ADZUNA_APP_KEY")),
		jooble.New(os.Getenv("JOOBLE_API_KEY"), jooble.WithBaseURL(env("JOOBLE_BASE_URL", "https://jooble.org"))),
	}

	var boards greenhouse.BoardSource = companies.Static{}
	dsn := env("POSTGRES_DSN", "postgres://jobsonar:jobsonar@localhost:5432/jobsonar?sslmode=disable")
	if st, err := companies.New(ctx, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "companies: %v (greenhouse will fetch nothing)\n", err)
	} else {
		closeFn = st.Close
		boards = st
	}
	out = append(out, greenhouse.New(boards))
	return out, closeFn
}

func newPublishFunc(ctx context.Context, sink string) (func(connector.Job) error, error) {
	switch sink {
	case "", "stdout":
		enc := json.NewEncoder(os.Stdout)
		return func(job connector.Job) error { return enc.Encode(job) }, nil
	case "sqs":
		queueURL := os.Getenv("SQS_QUEUE_URL")
		if queueURL == "" {
			return nil, fmt.Errorf("SQS_QUEUE_URL is required for -sink=sqs")
		}
		pub, err := queue.NewSQSPublisherFromEnv(ctx, os.Getenv("SQS_ENDPOINT_URL"), os.Getenv("AWS_REGION"), queueURL)
		if err != nil {
			return nil, err
		}
		return func(job connector.Job) error { return pub.Publish(ctx, job) }, nil
	default:
		return nil, fmt.Errorf("unknown sink %q (want stdout or sqs)", sink)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
