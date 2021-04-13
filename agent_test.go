package agent

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

func TestAgent(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin = r
	defer func() { stdin = os.Stdin }()

	ch := make(chan types.InputLogEvent, 1)
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, e := range params.LogEvents {
				ch <- e
			}
			if got, want := aws.ToString(params.LogGroupName), testLogGroup; got != want {
				t.Errorf("unexpected log group name: want %q, got %q", want, got)
			}
			if got, want := aws.ToString(params.LogStreamName), testLogStream; got != want {
				t.Errorf("unexpected log stream name: want %q, got %q", want, got)
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	a := &Agent{
		Writer: &Writer{
			LogGroupName:  testLogGroup,
			LogStreamName: testLogStream,
			logs:          mockCloudWatch,
		},
		Files: []string{},
	}
	if err := a.Start(); err != nil {
		t.Error(err)
	}

	if _, err := w.WriteString("testtest\n"); err != nil {
		t.Error(err)
	}
	if err := w.Close(); err != nil {
		t.Error(err)
	}
	if err := a.Close(); err != nil {
		t.Error(err)
	}

	e := <-ch
	if aws.ToString(e.Message) != "testtest" {
		t.Errorf("want %q, got %q", "testtest", aws.ToString(e.Message))
	}

	a.Wait()
}
