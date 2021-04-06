package agent

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsiface "github.com/shogo82148/cloudwatch-logs-agent-lite/internal/cloudwatchlogs"
)

const (
	// See: http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	perEventBytes          = 26
	maximumBytesPerPut     = 1048576
	maximumLogEventsPerPut = 10000
)

// Writer is a wrapper CloudWatch Logs that provids io.Writer interface.
type Writer struct {
	Config           aws.Config
	LogGroupName     string
	LogStreamName    string
	LogRetentionDays int

	logs              cloudwatchlogsiface.Interface
	nextSequenceToken *string
	remain            string
	events            []types.InputLogEvent
	currentByteLength int
}

func (w *Writer) logsClient() cloudwatchlogsiface.Interface {
	if w.logs == nil {
		w.logs = cloudwatchlogs.NewFromConfig(w.Config)
	}
	return w.logs
}

// Write implements io.Writer interface.
func (w *Writer) Write(p []byte) (int, error) {
	return w.WriteWithTime(time.Now(), p)
}

// WriteContext is same as Write, and it supports the context.
func (w *Writer) WriteContext(ctx context.Context, p []byte) (int, error) {
	return w.WriteWithTimeContext(ctx, time.Now(), p)
}

// WriteWithTime writes data with timestamp.
func (w *Writer) WriteWithTime(now time.Time, p []byte) (int, error) {
	return w.WriteWithTimeContext(context.Background(), now, p)
}

// WriteWithTimeContext writes data with timestamp.
func (w *Writer) WriteWithTimeContext(ctx context.Context, now time.Time, p []byte) (int, error) {
	var m int

	// concat the remain and the first line
	if w.remain != "" {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			w.remain += string(p)
			return len(p), nil
		}
		line := w.remain + string(p[:idx])
		p = p[idx+1:]
		w.remain = ""
		n, err := w.WriteEventContext(ctx, now, line)
		if err != nil {
			return m, err
		}
		m += n
		m++ // for '\n'
	}

	n, err := w.WriteStringWithTimeContext(ctx, now, string(p))
	if err != nil {
		return m, err
	}
	return m + n, nil
}

// WriteString writes a string.
func (w *Writer) WriteString(s string) (int, error) {
	return w.WriteStringWithTimeContext(context.Background(), time.Now(), s)
}

// WriteStringContext writes a string.
func (w *Writer) WriteStringContext(ctx context.Context, s string) (int, error) {
	return w.WriteStringWithTimeContext(ctx, time.Now(), s)
}

// WriteStringWithTime writes data with timestamp.
func (w *Writer) WriteStringWithTime(now time.Time, s string) (int, error) {
	return w.WriteStringWithTimeContext(context.Background(), now, s)
}

// WriteStringWithTimeContext writes data with timestamp.
func (w *Writer) WriteStringWithTimeContext(ctx context.Context, now time.Time, s string) (int, error) {
	var m int

	// concat the remain and the first line
	if w.remain != "" {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			w.remain += s
			return len(s), nil
		}
		line := w.remain + s[:idx]
		s = s[idx+1:]
		w.remain = ""
		n, err := w.WriteEvent(now, line)
		if err != nil {
			return m, err
		}
		m += n
		m++ // for '\n'
	}

	for len(s) > 0 {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			w.remain = s
			m += len(s)
			break
		}
		line := s[:idx]
		s = s[idx+1:]
		n, err := w.WriteEvent(now, line)
		if err != nil {
			return m, err
		}
		m += n
		m++ // for '\n'
	}
	return m, nil
}

// WriteEvent writes an log event.
func (w *Writer) WriteEvent(now time.Time, message string) (int, error) {
	return w.WriteEventContext(context.Background(), now, message)
}

// WriteEventContext writes an log event.
func (w *Writer) WriteEventContext(ctx context.Context, now time.Time, message string) (int, error) {
	if message == "" {
		return 0, nil
	}

	if w.currentByteLength+cloudwatchLen(message) >= maximumBytesPerPut {
		// the byte length will be over the limit
		// need flush before adding the new event.
		if err := w.FlushContext(ctx); err != nil {
			return 0, err
		}
	}

	w.events = append(w.events, types.InputLogEvent{
		Message:   aws.String(message),
		Timestamp: aws.Int64(now.Unix()*1000 + int64(now.Nanosecond()/1000000)),
	})
	if len(w.events) == maximumLogEventsPerPut {
		// the count of events reaches the limit, need flush.
		if err := w.FlushContext(ctx); err != nil {
			return 0, err
		}
	}
	return len(message), nil
}

// Flush is same as FlushContext, but it doesn't support the context.
func (w *Writer) Flush() error {
	return w.FlushContext(context.Background())
}

// FlushContext flushes the logs to the AWS CloudWatch Logs.
func (w *Writer) FlushContext(ctx context.Context) error {
	events := w.events
	w.events = nil
	w.currentByteLength = 0
	if len(events) == 0 {
		return nil
	}

	err := w.putEvents(ctx, events)
	if err != nil {
		if awsErr := (*types.ResourceNotFoundException)(nil); errors.As(err, &awsErr) {
			// Maybe our log stream doesn't exist yet.
			if err := w.createStream(ctx, true); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		}
		if awsErr := (*types.DataAlreadyAcceptedException)(nil); errors.As(err, &awsErr) {
			// This batch was already sent
			return nil
		}
		if awsErr := (*types.InvalidSequenceTokenException)(nil); errors.As(err, &awsErr) {
			if err := w.getNextSequenceToken(ctx); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		}
		return err
	}
	return nil
}

func (w *Writer) putEvents(ctx context.Context, events []types.InputLogEvent) error {
	logs := w.logsClient()
	resp, err := logs.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  &w.LogGroupName,
		LogStreamName: &w.LogStreamName,
		SequenceToken: w.nextSequenceToken,
	})
	if err != nil {
		w.nextSequenceToken = nil
		return err
	}
	w.nextSequenceToken = resp.NextSequenceToken
	return nil
}

func (w *Writer) createStream(ctx context.Context, tryToCreateGroup bool) error {
	logs := w.logsClient()
	_, err := logs.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &w.LogGroupName,
		LogStreamName: &w.LogStreamName,
	})
	if err != nil {
		if awsErr := (*types.ResourceAlreadyExistsException)(nil); errors.As(err, &awsErr) {
			// already created, just ignore
			return nil
		}
		if awsErr := (*types.ResourceNotFoundException)(nil); errors.As(err, &awsErr) {
			// Maybe our log group doesn't exist yet.
			if !tryToCreateGroup {
				return err
			}
			if err := w.createGroup(ctx); err != nil {
				return err
			}
			return w.createStream(ctx, false)
		}
		return err
	}
	return nil
}

func (w *Writer) createGroup(ctx context.Context) error {
	logs := w.logsClient()
	_, err := logs.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &w.LogGroupName,
	})
	if err != nil {
		if awsErr := (*types.ResourceAlreadyExistsException)(nil); errors.As(err, &awsErr) {
			// already created, just ignore
			return nil
		}
		return err
	}
	return w.setLogGroupRetention(ctx)
}

func (w *Writer) setLogGroupRetention(ctx context.Context) error {
	if w.LogRetentionDays == 0 {
		return nil
	}

	logs := w.logsClient()
	_, err := logs.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    &w.LogGroupName,
		RetentionInDays: aws.Int32(int32(w.LogRetentionDays)),
	})
	if err != nil {
		return err
	}
	return nil
}

func (w *Writer) getNextSequenceToken(ctx context.Context) error {
	logs := w.logsClient()
	resp, err := logs.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        &w.LogGroupName,
		LogStreamNamePrefix: &w.LogStreamName,
		Limit:               aws.Int32(1),
	})
	if err != nil {
		return err
	}
	w.nextSequenceToken = resp.LogStreams[0].UploadSequenceToken
	return nil
}

// Close closes the Writer.
func (w *Writer) Close() error {
	return w.Flush()
}

// steal from https://github.com/aws/amazon-cloudwatch-logs-for-fluent-bit/blob/b5dc2e67047da375dd5327e5a2d9cf5a2436219a/cloudwatch/cloudwatch.go#L494-L509
// effectiveLen counts the effective number of bytes in the string, after
// UTF-8 normalization.  UTF-8 normalization includes replacing bytes that do
// not constitute valid UTF-8 encoded Unicode codepoints with the Unicode
// replacement codepoint U+FFFD (a 3-byte UTF-8 sequence, represented in Go as
// utf8.RuneError)
func effectiveLen(line string) int {
	effectiveBytes := 0
	for _, rune := range line {
		effectiveBytes += utf8.RuneLen(rune)
	}
	return effectiveBytes
}

func cloudwatchLen(event string) int {
	return effectiveLen(event) + perEventBytes
}
