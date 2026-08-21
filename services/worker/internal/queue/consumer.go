// Package queue is the worker's SQS consumer. It is a separate, small
// implementation from the connectors module's publisher (see
// services/connectors/internal/queue) rather than a shared package —
// worker and connectors are independent Go modules, and Go's internal/
// visibility rule is by import-path ancestry, not by repo, so worker cannot
// import connectors' internal packages at all.
package queue

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Message is one received SQS message: the raw body plus the receipt
// handle needed to delete it after a successful upsert.
type Message struct {
	Body          string
	ReceiptHandle string
}

// sqsAPI is the subset of *sqs.Client the Consumer needs, so tests can fake
// it without a network call or a running ElasticMQ.
type sqsAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

type Consumer struct {
	client   sqsAPI
	queueURL string
}

func NewConsumer(client sqsAPI, queueURL string) *Consumer {
	return &Consumer{client: client, queueURL: queueURL}
}

// NewConsumerFromEnv builds a real SQS client. When endpointURL is set
// (ElasticMQ locally, via SQS_ENDPOINT_URL), requests go there instead of
// AWS.
func NewConsumerFromEnv(ctx context.Context, endpointURL, region, queueURL string) (*Consumer, error) {
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
	return NewConsumer(client, queueURL), nil
}

// Receive long-polls for up to 10 messages. An empty, error-free result
// means the queue was drained at the time of the call, not that it will
// stay empty.
func (c *Consumer) Receive(ctx context.Context, waitSeconds int32) ([]Message, error) {
	out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     waitSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}
	msgs := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, Message{Body: aws.ToString(m.Body), ReceiptHandle: aws.ToString(m.ReceiptHandle)})
	}
	return msgs, nil
}

// Delete removes a message after it has been durably persisted. Messages
// that fail to decode or persist are deliberately left undeleted: after
// maxReceiveCount redeliveries (raw-jobs' redrive policy, elasticmq.conf),
// the queue itself moves them to raw-jobs-dlq — the worker does not need to
// special-case poison messages itself.
func (c *Consumer) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}
