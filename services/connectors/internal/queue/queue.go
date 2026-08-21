// Package queue publishes normalized jobs to the ingestion queue.
//
// Connectors publish the already-normalized connector.Job, not the raw
// source payload — see plan/active/week-2.md's design note. The worker is a
// separate Go module and cannot import a connector's internal Normalize, so
// normalization stays here, where it already lives.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

// Publisher sends a normalized job to the ingestion queue.
type Publisher interface {
	Publish(ctx context.Context, job connector.Job) error
}

// sqsAPI is the subset of *sqs.Client that Publisher needs, so tests can
// fake it without a network call or a running ElasticMQ.
type sqsAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// SQSPublisher publishes jobs as SQS message bodies (JSON-encoded Job).
type SQSPublisher struct {
	client   sqsAPI
	queueURL string
}

// NewSQSPublisher wraps an existing SQS-compatible client. Exported so tests
// can pass a fake sqsAPI.
func NewSQSPublisher(client sqsAPI, queueURL string) *SQSPublisher {
	return &SQSPublisher{client: client, queueURL: queueURL}
}

// NewSQSPublisherFromEnv builds a real SQS client. When endpointURL is set
// (ElasticMQ locally, via SQS_ENDPOINT_URL), requests go there instead of
// AWS; region and credentials come from the standard AWS env vars, which
// ElasticMQ does not validate but the SDK still requires to sign requests.
func NewSQSPublisherFromEnv(ctx context.Context, endpointURL, region, queueURL string) (*SQSPublisher, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	})
	return NewSQSPublisher(client, queueURL), nil
}

func (p *SQSPublisher) Publish(ctx context.Context, job connector.Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if _, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
