package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

type fakeSQS struct {
	gotInput *sqs.SendMessageInput
	err      error
}

func (f *fakeSQS) SendMessage(ctx context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.gotInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &sqs.SendMessageOutput{}, nil
}

func TestSQSPublisher_Publish_SendsQueueURLAndJobBody(t *testing.T) {
	fake := &fakeSQS{}
	pub := NewSQSPublisher(fake, "http://localhost:9324/000000000000/raw-jobs")

	job := connector.Job{
		Source:    "adzuna",
		SourceURL: "https://example.com/job/1",
		Title:     "Senior SWE",
		Company:   "Acme",
		Location:  "Remote",
	}

	if err := pub.Publish(context.Background(), job); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if fake.gotInput == nil {
		t.Fatal("SendMessage was not called")
	}
	if got := *fake.gotInput.QueueUrl; got != "http://localhost:9324/000000000000/raw-jobs" {
		t.Errorf("QueueUrl = %q, want the configured queue URL", got)
	}

	var gotJob connector.Job
	if err := json.Unmarshal([]byte(*fake.gotInput.MessageBody), &gotJob); err != nil {
		t.Fatalf("MessageBody did not decode as a Job: %v", err)
	}
	if gotJob != job {
		t.Errorf("decoded job = %+v, want %+v", gotJob, job)
	}
}

func TestSQSPublisher_Publish_WrapsSendError(t *testing.T) {
	fake := &fakeSQS{err: errors.New("boom")}
	pub := NewSQSPublisher(fake, "http://localhost:9324/000000000000/raw-jobs")

	err := pub.Publish(context.Background(), connector.Job{})
	if err == nil {
		t.Fatal("expected an error when SendMessage fails")
	}
}
