package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

func TestAgent(t *testing.T) {
	// replace STDIN with a dummy pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin = r
	defer func() {
		stdin = os.Stdin
		w.Close()
		r.Close()
	}()

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

	// start logging
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

	// write some log messages and close
	if _, err := w.WriteString("testtest\n"); err != nil {
		t.Error(err)
	}
	if err := w.Close(); err != nil {
		t.Error(err)
	}
	a.Wait()

	// we will get the messages wrote
	e := <-ch
	if aws.ToString(e.Message) != "testtest" {
		t.Errorf("want %q, got %q", "testtest", aws.ToString(e.Message))
	}
}

func TestAgent_FlushInterval(t *testing.T) {
	// replace STDIN with a dummy pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin = r
	defer func() {
		stdin = os.Stdin
		w.Close()
		r.Close()
	}()

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

	// start logging with the FlushInterval option
	a := &Agent{
		Writer: &Writer{
			LogGroupName:  testLogGroup,
			LogStreamName: testLogStream,
			logs:          mockCloudWatch,
		},
		Files:         []string{},
		FlushInterval: time.Second, // periodic flushing is enabled
	}
	if err := a.Start(); err != nil {
		t.Error(err)
	}

	// write some log messages
	if _, err := w.WriteString("testtest\n"); err != nil {
		t.Error(err)
	}

	// do not a.Close(), so the log buffer is not flushed here
	// the log buffer will be flushed by periodic flushing

	// we will get the messages wrote
	e := <-ch
	if aws.ToString(e.Message) != "testtest" {
		t.Errorf("want %q, got %q", "testtest", aws.ToString(e.Message))
	}

	if err := w.Close(); err != nil {
		t.Error(err)
	}
	if err := a.Close(); err != nil {
		t.Error(err)
	}
	a.Wait()
}

func TestAgent_FlushTimeout(t *testing.T) {
	t.Run("The first error stops log forwarding", func(t *testing.T) {
		// replace STDIN with a dummy pipe
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdin = r
		defer func() {
			stdin = os.Stdin
			w.Close()
			r.Close()
		}()

		mockCloudWatch := &cloudwatchlogsiface.Mock{
			// PutLogEventsFunc never succeed
			PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
				<-ctx.Done() // wait for timeout or canceling
				return nil, ctx.Err()
			},
		}

		// start logging with the FlushInterval option
		a := &Agent{
			Writer: &Writer{
				LogGroupName:  testLogGroup,
				LogStreamName: testLogStream,
				logs:          mockCloudWatch,
			},
			Files:         []string{},
			FlushInterval: time.Second, // periodic flushing is enabled
			FlushTimeout:  time.Second,
		}
		if err := a.Start(); err != nil {
			t.Error(err)
		}

		if _, err := w.WriteString("testtest\n"); err != nil {
			t.Error(err)
		}

		// a.Wait() will return without a.Close()
		// because flushing is failed
		a.Wait()
	})

	t.Run("If it succeeds once, the error doesn't stop log forwarding", func(t *testing.T) {
		// replace STDIN with a dummy pipe
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdin = r
		defer func() {
			stdin = os.Stdin
			w.Close()
			r.Close()
		}()

		var flushCount int
		mockCloudWatch := &cloudwatchlogsiface.Mock{
			// PutLogEventsFunc never succeed
			PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
				flushCount++
				if flushCount == 1 {
					return &cloudwatchlogs.PutLogEventsOutput{}, nil
				}
				<-ctx.Done() // wait for timeout or canceling
				return nil, ctx.Err()
			},
		}

		// start logging with the FlushInterval option
		a := &Agent{
			Writer: &Writer{
				LogGroupName:  testLogGroup,
				LogStreamName: testLogStream,
				logs:          mockCloudWatch,
			},
			Files:         []string{},
			FlushInterval: time.Second, // periodic flushing is enabled
			FlushTimeout:  time.Second,
		}
		if err := a.Start(); err != nil {
			t.Error(err)
		}

		ch1 := make(chan struct{})
		go func() {
			defer close(ch1)
			if _, err := w.WriteString("testtest\n"); err != nil {
				t.Error(err)
			}
			if _, err := w.WriteString("testtest\n"); err != nil {
				t.Error(err)
			}
			time.Sleep(3 * time.Second)
			w.Close()
		}()

		ch2 := make(chan struct{})
		go func() {
			defer close(ch2)
			a.Wait()
		}()

		select {
		case <-ch1:
			<-ch2
		case <-ch2:
			t.Error("Log forwarding stopped unexpectedly")
		}
	})
}
