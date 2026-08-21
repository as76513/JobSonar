package queue

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeSQS struct {
	receiveOut    *sqs.ReceiveMessageOutput
	receiveErr    error
	deletedHandle string
	deleteErr     error
}

func (f *fakeSQS) ReceiveMessage(ctx context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return f.receiveOut, f.receiveErr
}

func (f *fakeSQS) DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.deletedHandle = *in.ReceiptHandle
	return &sqs.DeleteMessageOutput{}, f.deleteErr
}

func TestConsumer_Receive_ReturnsBodyAndReceiptHandle(t *testing.T) {
	fake := &fakeSQS{
		receiveOut: &sqs.ReceiveMessageOutput{
			Messages: []types.Message{
				{Body: aws.String(`{"title":"Senior SWE"}`), ReceiptHandle: aws.String("handle-1")},
			},
		},
	}
	c := NewConsumer(fake, "http://localhost:9324/000000000000/raw-jobs")

	msgs, err := c.Receive(context.Background(), 1)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != `{"title":"Senior SWE"}` || msgs[0].ReceiptHandle != "handle-1" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestConsumer_Receive_EmptyIsNotAnError(t *testing.T) {
	fake := &fakeSQS{receiveOut: &sqs.ReceiveMessageOutput{}}
	c := NewConsumer(fake, "http://localhost:9324/000000000000/raw-jobs")

	msgs, err := c.Receive(context.Background(), 1)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages, got %d", len(msgs))
	}
}

func TestConsumer_Delete_PassesReceiptHandle(t *testing.T) {
	fake := &fakeSQS{}
	c := NewConsumer(fake, "http://localhost:9324/000000000000/raw-jobs")

	if err := c.Delete(context.Background(), "handle-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if fake.deletedHandle != "handle-1" {
		t.Fatalf("deleted handle = %q, want handle-1", fake.deletedHandle)
	}
}
