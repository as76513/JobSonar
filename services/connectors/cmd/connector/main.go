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
)

func main() {
	what := flag.String("what", env("ADZUNA_WHAT", "software engineer"), "search keywords")
	where := flag.String("where", env("ADZUNA_WHERE", ""), "location filter")
	country := flag.String("country", env("ADZUNA_COUNTRY", "us"), "Adzuna country code (us, gb, in, ...)")
	page := flag.Int("page", envInt("ADZUNA_PAGE", 1), "result page (1-indexed)")
	perPage := flag.Int("per-page", envInt("ADZUNA_PER_PAGE", 20), "results per page")
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

	enc := json.NewEncoder(os.Stdout)
	anyOK := false
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", r.Name, r.Err)
			continue
		}
		anyOK = true
		for _, job := range r.Jobs {
			if err := enc.Encode(job); err != nil {
				fmt.Fprintf(os.Stderr, "encode: %v\n", err)
				os.Exit(1)
			}
		}
	}
	if !anyOK {
		os.Exit(1)
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
