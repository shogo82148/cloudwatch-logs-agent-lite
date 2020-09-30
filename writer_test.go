package agent

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

const (
	testLogGroup  = "my-logs"
	testLogStream = "my-stream"
)

func TestWriter_WriteEvent(t *testing.T) {
	var events []*types.InputLogEvent
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			events = append(events, params.LogEvents...)
			if got, want := aws.ToString(params.LogGroupName), testLogGroup; got != want {
				t.Errorf("unexpected log group name: want %q, got %q", want, got)
			}
			if got, want := aws.ToString(params.LogStreamName), testLogStream; got != want {
				t.Errorf("unexpected log stream name: want %q, got %q", want, got)
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}
	now := time.Unix(1234567890, 0)
	n, err := w.WriteEvent(now, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello") {
		t.Errorf("want %d, got %d", len("hello"), n)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("invalid length: want 1, got %d", len(events))
	}
	if got, want := aws.ToString(events[0].Message), "hello"; got != want {
		t.Errorf("unexpected message: want %q, got %q", want, got)
	}
	if got, want := aws.ToInt64(events[0].Timestamp), int64(1234567890000); got != want {
		t.Errorf("unexpected timestamp: want %d, got %d", want, got)
	}
}
