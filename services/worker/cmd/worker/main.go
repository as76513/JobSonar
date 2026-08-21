package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/as76513/JobSonar/services/worker/internal/dedup"
	"github.com/as76513/JobSonar/services/worker/internal/job"
	"github.com/as76513/JobSonar/services/worker/internal/queue"
	"github.com/as76513/JobSonar/services/worker/internal/store"
)

func main() {
	queueURL := flag.String("queue-url", os.Getenv("SQS_QUEUE_URL"), "SQS/ElasticMQ queue URL to drain")
	endpointURL := flag.String("endpoint-url", os.Getenv("SQS_ENDPOINT_URL"), "SQS endpoint override (ElasticMQ locally; empty for real AWS)")
	region := flag.String("region", os.Getenv("AWS_REGION"), "AWS region")
	dsn := flag.String("dsn", env("POSTGRES_DSN", "postgres://jobsonar:jobsonar@localhost:5432/jobsonar?sslmode=disable"), "Postgres DSN")
	idleExitAfter := flag.Int("idle-exit-after", envInt("WORKER_IDLE_EXIT_AFTER", 3), "exit after this many consecutive empty polls; <=0 runs forever")
	flag.Parse()

	if *queueURL == "" {
		fmt.Fprintln(os.Stderr, "SQS_QUEUE_URL (or -queue-url) is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	consumer, err := queue.NewConsumerFromEnv(ctx, *endpointURL, *region, *queueURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "queue: %v\n", err)
		os.Exit(1)
	}

	db, err := store.New(ctx, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	processed, idle := 0, 0
	for {
		msgs, err := consumer.Receive(ctx, 2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "receive: %v\n", err)
			os.Exit(1)
		}
		if len(msgs) == 0 {
			idle++
			if *idleExitAfter > 0 && idle >= *idleExitAfter {
				break
			}
			continue
		}
		idle = 0

		for _, msg := range msgs {
			var j job.Job
			if err := json.Unmarshal([]byte(msg.Body), &j); err != nil {
				// Leave undeleted: the queue's redrive policy moves it to
				// the DLQ after maxReceiveCount redeliveries.
				fmt.Fprintf(os.Stderr, "decode: %v\n", err)
				continue
			}

			if _, err := db.Upsert(ctx, j, dedup.Hash(j)); err != nil {
				fmt.Fprintf(os.Stderr, "upsert %s: %v\n", j.SourceURL, err)
				continue
			}
			if err := consumer.Delete(ctx, msg.ReceiptHandle); err != nil {
				fmt.Fprintf(os.Stderr, "delete: %v\n", err)
				continue
			}
			processed++
		}
	}

	fmt.Fprintf(os.Stderr, "processed %d job(s)\n", processed)
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
