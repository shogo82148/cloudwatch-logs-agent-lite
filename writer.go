package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
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
	maximumBytesPerEvent   = maximumBytesPerPut - perEventBytes
)

// Writer is a wrapper CloudWatch Logs that provides io.Writer interface.
type Writer struct {
	Config           aws.Config
	LogGroupName     string
	LogStreamName    string
	LogRetentionDays int

	logs              cloudwatchlogsiface.Interface
	nextSequenceToken *string

	remain bytes.Buffer

	// events buffered
	events []types.InputLogEvent

	// current byte length in events
	currentByteLength int

	lastWroteTime   time.Time
	lastFlushedTime time.Time
}

func (w *Writer) logsClient() cloudwatchlogsiface.Interface {
	if w.logs == nil {
		w.logs = cloudwatchlogs.NewFromConfig(w.Config)
	}
	return w.logs
}

// Write implements io.Writer interface.
func (w *Writer) Write(p []byte) (int, error) {
	return w.WriteWithTimeContext(context.Background(), time.Now(), p)
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
	// concat the remain and the first line
	// if w.remain.Len() != 0 {
	// 	idx := bytes.IndexByte(p, '\n')
	// 	if idx < 0 {
	// 		return w.remain.Write(p)
	// 	}

	// 	// bytes.Buffer never return any error.
	// 	// so we don't need to check it.
	// 	n, _ := w.remain.Write(p[:idx])
	// 	m += n
	// 	m++ // for '\n'

	// 	line := w.remain.String()
	// 	p = p[idx+1:]
	// 	w.remain.Reset()
	// 	_, err := w.WriteEventContext(ctx, now, line)
	// 	if err != nil {
	// 		return m, err
	// 	}
	// }

	var m int
	for m < len(p) {
		r, n := utf8.DecodeLastRune(p[m:])
		m += n
		utf8.RuneLen(r)
		w.remain.WriteRune(r)
	}
	return m, nil
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
	if w.remain.Len() != 0 {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			return w.remain.WriteString(s)
		}

		// bytes.Buffer never return any error.
		// so we don't need to check it.
		n, _ := w.remain.WriteString(s[:idx])
		m += n
		m++ // for '\n'

		line := w.remain.String()
		s = s[idx+1:]
		w.remain.Reset()
		_, err := w.WriteEventContext(ctx, now, line)
		if err != nil {
			return m, err
		}
	}

	for len(s) > 0 {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			w.remain.WriteString(s)
			m += len(s)
			break
		}
		line := s[:idx]
		s = s[idx+1:]
		n, err := w.WriteEventContext(ctx, now, line)
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

// WriteEventContext writes log events.
// Long message might be separated into multiple events.
func (w *Writer) WriteEventContext(ctx context.Context, now time.Time, message string) (int, error) {
	// pre-allocate buffer
	var buf strings.Builder
	grow := len(message)
	if grow > maximumBytesPerEvent {
		grow = maximumBytesPerEvent
	}
	buf.Grow(grow)

	n := 0
	for n < len(message) {
		r, m := utf8.DecodeLastRuneInString(message[n:])
		n += m

		l := utf8.RuneLen(r)
		if buf.Len()+l > maximumBytesPerEvent {
			// message is too long; we need to split it.
			if _, err := w.writeEventContext(ctx, now, buf.String()); err != nil {
				return 0, err
			}

			// reset and pre-allocate buffer
			buf.Reset()
			grow := len(message) - n
			if grow > maximumBytesPerEvent {
				grow = maximumBytesPerEvent
			}
			buf.Grow(grow)
		}
		buf.WriteRune(r)
	}
	if _, err := w.writeEventContext(ctx, now, buf.String()); err != nil {
		return 0, err
	}
	return n, nil
}

// writeEventContext writes an log event.
// message must valid utf8 string and len(message) <= maximumBytesPerEvent.
func (w *Writer) writeEventContext(ctx context.Context, now time.Time, message string) (int, error) {
	if message == "" {
		return 0, nil
	}

	if w.lastWroteTime.IsZero() || now.After(w.lastWroteTime) {
		w.lastWroteTime = now
	}

	l := len(message) + perEventBytes
	if l > maximumBytesPerPut {
		return 0, fmt.Errorf("agent: internal error: too long event message: %d", l)
	}
	if w.currentByteLength+l > maximumBytesPerPut {
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
	w.currentByteLength += l
	if len(w.events) == maximumLogEventsPerPut || // the count of events reaches the limit
		w.currentByteLength >= maximumBytesPerEvent { // byte length reaches the limit

		// we need to flush
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

	// there is no event; nothing to do
	if len(events) == 0 {
		return nil
	}

	if w.lastFlushedTime.IsZero() || w.lastWroteTime.After(w.lastFlushedTime) {
		w.lastFlushedTime = w.lastWroteTime
	}
	w.events = events[:0]
	w.currentByteLength = 0

	err := w.putEvents(ctx, events)
	if err != nil {
		if _, ok := errorsAs[*types.ResourceNotFoundException](err); ok {
			// Maybe our log stream doesn't exist yet.
			// try to create new one.
			if err := w.createStream(ctx, true); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		}
		if _, ok := errorsAs[*types.DataAlreadyAcceptedException](err); ok {
			// This batch was already sent
			return nil
		}
		if _, ok := errorsAs[*types.InvalidSequenceTokenException](err); ok {
			if err := w.getNextSequenceToken(ctx); err != nil {
				return err
			}
			return w.putEvents(ctx, events)
		}
		return err
	}
	return nil
}

func errorsAs[T error](err error) (errT T, ok bool) {
	ok = errors.As(err, &errT)
	return
}

// LastFlushedTime returns the timestamp of the event most recently putted.
func (w *Writer) LastFlushedTime() time.Time {
	return w.lastFlushedTime
}

func (w *Writer) putEvents(ctx context.Context, events []types.InputLogEvent) error {
	log.Printf("[DEBUG] putting %d events", len(events))
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
	log.Printf("[DEBUG] try to create stream: group name: %s, stream name: %s", w.LogGroupName, w.LogStreamName)
	logs := w.logsClient()
	_, err := logs.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &w.LogGroupName,
		LogStreamName: &w.LogStreamName,
	})
	if err != nil {
		if _, ok := errorsAs[*types.ResourceAlreadyExistsException](err); ok {
			// already created, just ignore
			return nil
		}
		if _, ok := errorsAs[*types.ResourceNotFoundException](err); ok {
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
	log.Printf("[DEBUG] try to create log group: group name: %s", w.LogGroupName)
	logs := w.logsClient()
	_, err := logs.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: &w.LogGroupName,
	})
	if err != nil {
		if _, ok := errorsAs[*types.ResourceAlreadyExistsException](err); ok {
			// already created, just ignore
			log.Printf("[DEBUG] group name %s is already created", w.LogGroupName)
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

	log.Printf("[DEBUG] putting log retention policy: group name: %s, retention in days: %d", w.LogGroupName, w.LogRetentionDays)
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
	log.Printf("[DEBUG] getting next sequence token: group name: %s, stream name: %s", w.LogGroupName, w.LogStreamName)
	logs := w.logsClient()
	resp, err := logs.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        &w.LogGroupName,
		LogStreamNamePrefix: &w.LogStreamName,
		Limit:               aws.Int32(1),
	})
	if err != nil {
		return err
	}
	log.Printf(
		"[DEBUG] next sequence token is %q : group name: %s, stream name: %s",
		aws.ToString(w.nextSequenceToken), w.LogGroupName, w.LogStreamName,
	)
	w.nextSequenceToken = resp.LogStreams[0].UploadSequenceToken
	return nil
}

// Close closes the Writer.
func (w *Writer) Close() error {
	return w.CloseContext(context.Background())
}

// CloseContexts closes the Writer.
func (w *Writer) CloseContext(ctx context.Context) error {
	return w.FlushContext(ctx)
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
