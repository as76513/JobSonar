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
	"github.com/as76513/JobSonar/services/connectors/internal/connector"
	"github.com/as76513/JobSonar/services/connectors/internal/queue"
)

func main() {
	what := flag.String("what", env("ADZUNA_WHAT", "software engineer"), "search keywords")
	where := flag.String("where", env("ADZUNA_WHERE", ""), "location filter")
	country := flag.String("country", env("ADZUNA_COUNTRY", "us"), "Adzuna country code (us, gb, in, ...)")
	page := flag.Int("page", envInt("ADZUNA_PAGE", 1), "result page (1-indexed)")
	perPage := flag.Int("per-page", envInt("ADZUNA_PER_PAGE", 20), "results per page")
	sink := flag.String("sink", env("CONNECTOR_SINK", "stdout"), "where normalized jobs go: stdout or sqs")
	flag.Parse()

	client := adzuna.New(os.Getenv("ADZUNA_APP_ID"), os.Getenv("ADZUNA_APP_KEY"))
	reg := connector.NewRegistry(client)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

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

// newPublishFunc returns a function that delivers one normalized Job to the
// configured sink: "stdout" (Week 1 behavior, JSON lines) or "sqs" (Week 2,
// publishes to the queue named by SQS_QUEUE_URL).
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
