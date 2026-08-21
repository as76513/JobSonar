package queue

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

// TestIntegration_SQSPublisher_AgainstLiveQueue publishes a job to a real
// SQS-compatible queue (ElasticMQ locally, via `make up`) and reads it back.
// Gated on RUN_QUEUE_INTEGRATION_TEST rather than just SQS_ENDPOINT_URL/
// SQS_QUEUE_URL: those are set in every developer's .env (needed for `make
// publish`/`make ingest`), so gating on them alone would make this test run
// — and potentially race a concurrently-draining worker on the same shared
// queue — on every plain `make test`. This stays opt-in and offline by
// default.
func TestIntegration_SQSPublisher_AgainstLiveQueue(t *testing.T) {
	if os.Getenv("RUN_QUEUE_INTEGRATION_TEST") == "" {
		t.Skip("set RUN_QUEUE_INTEGRATION_TEST=1 (plus SQS_ENDPOINT_URL, SQS_QUEUE_URL, AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY) to run this against a live queue — see .env.example")
	}
	endpoint := os.Getenv("SQS_ENDPOINT_URL")
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if endpoint == "" || queueURL == "" {
		t.Fatal("RUN_QUEUE_INTEGRATION_TEST is set but SQS_ENDPOINT_URL/SQS_QUEUE_URL are not")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pub, err := NewSQSPublisherFromEnv(ctx, endpoint, os.Getenv("AWS_REGION"), queueURL)
	if err != nil {
		t.Fatalf("NewSQSPublisherFromEnv: %v", err)
	}

	want := connector.Job{
		Source:    "adzuna",
		SourceURL: "https://example.com/job/integration-test",
		Title:     "Integration Test Job",
		Company:   "Acme",
		Location:  "Remote",
	}
	if err := pub.Publish(ctx, want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	out, err := pub.client.(*sqs.Client).ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 5,
		WaitTimeSeconds:     3,
	})
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}

	for _, msg := range out.Messages {
		var got connector.Job
		if err := json.Unmarshal([]byte(*msg.Body), &got); err != nil {
			continue
		}
		if got == want {
			_, _ = pub.client.(*sqs.Client).DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			return
		}
	}
	t.Fatalf("published job not found among %d received message(s)", len(out.Messages))
}
