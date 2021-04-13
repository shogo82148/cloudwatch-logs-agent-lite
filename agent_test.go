package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

func TestAgent(t *testing.T) {
	dir, err := os.MkdirTemp("", "cwlogs-lite-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	filename := filepath.Join(dir, "hoge.log")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}

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
		Files: []string{filename},
	}
	if err := a.Start(); err != nil {
		t.Error(err)
	}

	// wait for starting tail
	time.Sleep(time.Second)

	if _, err := file.WriteString("testtest\n"); err != nil {
		t.Error(err)
	}
	if err := file.Close(); err != nil {
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
