package agent

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/cloudwatchlogsiface"
)

const (
	// See: http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	perEventBytes          = 26
	maximumBytesPerPut     = 1048576
	maximumLogEventsPerPut = 10000
)

// Writer is a wrapper CloudWatch Logs that provids io.Writer interface.
type Writer struct {
	Config        aws.Config
	LogGroupName  string
	LogStreamName string

	logs              cloudwatchlogsiface.ClientAPI
	nextSequenceToken *string
	remain            string
	events            []cloudwatchlogs.InputLogEvent
	currentByteLength int
}

// Write implements io.Writer interface.
func (w *Writer) Write(p []byte) (int, error) {
	return w.WriteWithTime(time.Now(), p)
}

// WriteWithTime writes data with timestamp.
func (w *Writer) WriteWithTime(now time.Time, p []byte) (int, error) {
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
		n, err := w.WriteEvent(now, line)
		if err != nil {
			return m, err
		}
		m += n
		m++ // for '\n'
	}

	n, err := w.WriteStringWithTime(now, string(p))
	if err != nil {
		return m, err
	}
	return m + n, nil
}

// WriteString writes a string.
func (w *Writer) WriteString(s string) (int, error) {
	return w.WriteStringWithTime(time.Now(), s)
}

// WriteStringWithTime writes data with timestamp.
func (w *Writer) WriteStringWithTime(now time.Time, s string) (int, error) {
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
	if message == "" {
		return 0, nil
	}

	if w.currentByteLength+cloudwatchLen(message) >= maximumBytesPerPut {
		// the byte length will be over the limit
		// need flush before adding the new event.
		if err := w.Flush(); err != nil {
			return 0, err
		}
	}

	w.events = append(w.events, cloudwatchlogs.InputLogEvent{
		Message:   aws.String(message),
		Timestamp: aws.Int64(now.Unix()*1000 + int64(now.Nanosecond()/1000000)),
	})
	if len(w.events) == maximumLogEventsPerPut {
		// the count of events reaches the limit, need flush.
		if err := w.Flush(); err != nil {
			return 0, err
		}
	}
	return len(message), nil
}

// Flush flushes the logs to the AWS CloudWatch Logs.
func (w *Writer) Flush() error {
	events := w.events
	w.events = nil
	w.currentByteLength = 0
	if len(events) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := w.putEvents(ctx, events)
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case cloudwatchlogs.ErrCodeResourceNotFoundException:
			// Maybe our log stream doesn't exist yet.
			if err := w.createStream(ctx, true); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		case cloudwatchlogs.ErrCodeDataAlreadyAcceptedException:
			// This batch was already sent
			return nil
		case cloudwatchlogs.ErrCodeInvalidSequenceTokenException:
			if err := w.getNextSequenceToken(ctx); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		}
	}
	return err
}

func (w *Writer) putEvents(ctx context.Context, events []cloudwatchlogs.InputLogEvent) error {
	if w.logs == nil {
		w.logs = cloudwatchlogs.New(w.Config)
	}
	req := w.logs.PutLogEventsRequest(&cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  &w.LogGroupName,
		LogStreamName: &w.LogStreamName,
		SequenceToken: w.nextSequenceToken,
	})
	resp, err := req.Send(ctx)
	if err != nil {
		w.nextSequenceToken = nil
		return err
	}
	w.nextSequenceToken = resp.NextSequenceToken
	return nil
}

func (w *Writer) createStream(ctx context.Context, tryToCreateGroup bool) error {
	if w.logs == nil {
		w.logs = cloudwatchlogs.New(w.Config)
	}
	req := w.logs.CreateLogStreamRequest(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &w.LogGroupName,
		LogStreamName: &w.LogStreamName,
	})
	_, err := req.Send(ctx)
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case cloudwatchlogs.ErrCodeResourceAlreadyExistsException:
			// already created, just ignore
			return nil
		case cloudwatchlogs.ErrCodeResourceNotFoundException:
			// Maybe our log group doesn't exist yet.
			if !tryToCreateGroup {
				return err
			}
			if err := w.createGroup(ctx); err != nil {
				return err
			}
			return w.createStream(ctx, false)
		}
	}
	return err
}

func (w *Writer) createGroup(ctx context.Context) error {
	if w.logs == nil {
		w.logs = cloudwatchlogs.New(w.Config)
	}
	req := w.logs.CreateLogGroupRequest(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &w.LogGroupName,
	})
	_, err := req.Send(ctx)
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		switch awsErr.Code() {
		case cloudwatchlogs.ErrCodeResourceAlreadyExistsException:
			// already created, just ignore
			return nil
		}
	}
	return err
}

func (w *Writer) getNextSequenceToken(ctx context.Context) error {
	if w.logs == nil {
		w.logs = cloudwatchlogs.New(w.Config)
	}
	req := w.logs.DescribeLogStreamsRequest(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        &w.LogGroupName,
		LogStreamNamePrefix: &w.LogStreamName,
		Limit:               aws.Int64(1),
	})
	resp, err := req.Send(ctx)
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
