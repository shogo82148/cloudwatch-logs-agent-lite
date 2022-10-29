package agent

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/google/go-cmp/cmp"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

const (
	testLogGroup  = "my-logs"
	testLogStream = "my-stream"
)

var _ io.WriteCloser = (*Writer)(nil)
var _ io.StringWriter = (*Writer)(nil)

var inputs = []string{
	"single line\n",
	"multi line 1\nmulti line 2\nmulti line 3\n",
	"continuous line 1", "continuous line 2", "continuous line 3\n",
}

var output = []string{
	"single line",
	"multi line 1",
	"multi line 2",
	"multi line 3",
	"continuous line 1continuous line 2continuous line 3",
}

func TestWriter_Write(t *testing.T) {
	var events []string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, event := range params.LogEvents {
				events = append(events, aws.ToString(event.Message))
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	for _, input := range inputs {
		n, err := w.Write([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if n != len(input) {
			t.Errorf("unexpected wrote bytes: input: %q, want %d, got %d", input, len(input), n)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(output, events); diff != "" {
		t.Errorf("unexpected events (-want/+got):\n%s", diff)
	}
}

func TestWriter_WriteString(t *testing.T) {
	var events []string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, event := range params.LogEvents {
				events = append(events, aws.ToString(event.Message))
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	for _, input := range inputs {
		n, err := w.WriteString(input)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(input) {
			t.Errorf("unexpected wrote bytes: input: %q, want %d, got %d", input, len(input), n)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(output, events); diff != "" {
		t.Errorf("unexpected evenets (-want/+got):\n%s", diff)
	}
}

func TestWriter_WriteEvent(t *testing.T) {
	var events []types.InputLogEvent
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

func TestWriter_LastFlushedTime(t *testing.T) {
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	// check initial value of LastFlushedTime()
	if !w.LastFlushedTime().IsZero() {
		t.Errorf("initial value of LastFlushedTime() should be zero, but got %s", w.LastFlushedTime())
	}

	now := time.Unix(1234567890, 0)
	if _, err := w.WriteEvent(now, "hello"); err != nil {
		t.Fatal(err)
	}
	// LastFlushedTime() isn't updated because "hello" is just buffered.
	if !w.LastFlushedTime().IsZero() {
		t.Errorf("unexpected LastFlushedTime(), want zero, got %s", w.LastFlushedTime())
	}

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	// LastFlushedTime() is updated because "hello" is flushed.
	if !w.LastFlushedTime().Equal(now) {
		t.Errorf("unexpected LastFlushedTime(), want %s, got %s", now, w.LastFlushedTime())
	}

	prev, now := now, now.Add(time.Second)
	if _, err := w.WriteEvent(now, "world"); err != nil {
		t.Fatal(err)
	}
	// LastFlushedTime() isn't updated because "world" is just buffered.
	if !w.LastFlushedTime().Equal(prev) {
		t.Errorf("unexpected LastFlushedTime(), want %s, got %s", prev, w.LastFlushedTime())
	}

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	// LastFlushedTime() is updated because "world" is flushed.
	if !w.LastFlushedTime().Equal(now) {
		t.Errorf("unexpected LastFlushedTime(), want %s, got %s", now, w.LastFlushedTime())
	}
}

func TestWriter_createGroup(t *testing.T) {
	var events []types.InputLogEvent
	var logGroupName, logStreamName string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			if logGroupName != aws.ToString(params.LogGroupName) || logStreamName != aws.ToString(params.LogStreamName) {
				return nil, &types.ResourceNotFoundException{}
			}
			events = append(events, params.LogEvents...)
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
		CreateLogStreamFunc: func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			if logGroupName != aws.ToString(params.LogGroupName) {
				return nil, &types.ResourceNotFoundException{}
			}
			logStreamName = *params.LogStreamName
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		CreateLogGroupFunc: func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			logGroupName = aws.ToString(params.LogGroupName)
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
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

func TestWriter_setLogGroupRetention(t *testing.T) {
	var events []types.InputLogEvent
	var logGroupName, logStreamName string
	var retentionInDays int32
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			if logGroupName != aws.ToString(params.LogGroupName) || logStreamName != aws.ToString(params.LogStreamName) {
				return nil, &types.ResourceNotFoundException{}
			}
			events = append(events, params.LogEvents...)
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
		CreateLogStreamFunc: func(ctx context.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			if logGroupName != aws.ToString(params.LogGroupName) {
				return nil, &types.ResourceNotFoundException{}
			}
			logStreamName = *params.LogStreamName
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		CreateLogGroupFunc: func(ctx context.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
			logGroupName = aws.ToString(params.LogGroupName)
			return &cloudwatchlogs.CreateLogGroupOutput{}, nil
		},
		PutRetentionPolicyFunc: func(ctx context.Context, params *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
			if aws.ToString(params.LogGroupName) != logGroupName {
				return nil, &types.ResourceNotFoundException{}
			}
			retentionInDays = aws.ToInt32(params.RetentionInDays)
			return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
		},
	}

	w := &Writer{
		LogGroupName:     testLogGroup,
		LogStreamName:    testLogStream,
		LogRetentionDays: 90,
		logs:             mockCloudWatch,
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

	if got, want := retentionInDays, int32(90); got != want {
		t.Errorf("unexpected timestamp: want %d, got %d", want, got)
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

func TestWriter_WriteEventContext(t *testing.T) {
	count := 0
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			count = len(params.LogEvents)
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	for i := 0; i < maximumLogEventsPerPut; i++ {
		n, err := w.WriteEventContext(context.Background(), time.Now(), "a")
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("unexpected wrote bytes: want %d, got %d", 4, n)
		}
	}

	// Here is no FlushContext, but it is done in WriteEventContext.
	// Because the events count reaches maximumLogEventsPerPut.
	if count != maximumLogEventsPerPut {
		t.Errorf("want flushed, but not: %d", count)
	}
}

func TestWriter_WriteEventContext_LongLongLine(t *testing.T) {
	var events []string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, event := range params.LogEvents {
				events = append(events, aws.ToString(event.Message))
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	// "あ" has 3 bytes (len("あ") = 3)
	// and PutLogEvents can put an event up to 1048550 bytes at a time.
	// (1048550 bytes = maximumBytesPerPut-perEventBytes)
	//
	// WriteEvent separates the message that has more than 349516 あ.
	// len("あ") x 349516 = 1048548 bytes < 1048550 bytes
	// len("あ") x 349517 = 1048551 bytes > 1048550 bytes
	line := strings.Repeat("あ", 349517)

	n, err := w.WriteEvent(time.Now(), line)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(line) {
		t.Errorf("unexpected wrote bytes: want %d, got %d", 4, n)
	}
	w.Flush()

	if len(events) != 2 {
		t.Errorf("unexpected events count: %d", len(events))
	}
	want := strings.Repeat("あ", 349516)
	if events[0] != want {
		t.Errorf("unexpected event: %s", events[0])
	}
	if events[1] != "あ" {
		t.Errorf("unexpected event: %s", events[1])
	}
}

func TestWriter_WriteEventContext_ReplacementChar(t *testing.T) {
	var events []string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, event := range params.LogEvents {
				events = append(events, aws.ToString(event.Message))
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	// "\x80" has 1 bytes (len("\x80") = 1),
	// however it is replaced by U+FFFD � replacement character,
	// and U+FFFD has 3 bytes (len("\uFFFD") = 3).
	// and PutLogEvents can put an event up to 1048550 bytes at a time.
	// (1048550 bytes = maximumBytesPerPut-perEventBytes)
	//
	// WriteEvent separates the message that has more than 349516 "\x80".
	// len("\uFFFD") x 349516 = 1048548 bytes < 1048550 bytes
	// len("\uFFFD") x 349517 = 1048551 bytes > 1048550 bytes
	line := strings.Repeat("\x80", 349517)

	n, err := w.WriteEvent(time.Now(), line)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(line) {
		t.Errorf("unexpected wrote bytes: want %d, got %d", 4, n)
	}
	w.Flush()

	if len(events) != 2 {
		t.Errorf("unexpected events count: %d", len(events))
	}
	want := strings.Repeat("\uFFFD", 349516)
	if events[0] != want {
		t.Errorf("unexpected event: %s", events[0])
	}
	if events[1] != "\uFFFD" {
		t.Errorf("unexpected event: %s", events[1])
	}
}

func TestWriter_Write_LongLongLine(t *testing.T) {
	var events []string
	mockCloudWatch := &cloudwatchlogsiface.Mock{
		PutLogEventsFunc: func(ctx context.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			for _, event := range params.LogEvents {
				events = append(events, aws.ToString(event.Message))
			}
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}

	input := bytes.Repeat([]byte("😀"), 1<<20)
	input = append(input, '\n')
	n, err := w.Write(input)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Errorf("unexpected wrote bytes: input: %q, want %d, got %d", input, len(input), n)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(output, events); diff != "" {
		t.Errorf("unexpected events (-want/+got):\n%s", diff)
	}
}
