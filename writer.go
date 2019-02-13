package agent

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/cloudwatchlogsiface"
)

// Writer is a wrapper CloudWatch Logs that provids io.Writer interface.
type Writer struct {
	Config        aws.Config
	LogGroupName  string
	LogStreamName string

	logs              cloudwatchlogsiface.CloudWatchLogsAPI
	nextSequenceToken *string
	remain            string
	events            []cloudwatchlogs.InputLogEvent
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
	w.events = append(w.events, cloudwatchlogs.InputLogEvent{
		Message:   aws.String(message),
		Timestamp: aws.Int64(now.Unix()*1000 + int64(now.Nanosecond()/1000)),
	})
	return len(message), nil
}

// Flush flushes the logs to the AWS CloudWatch Logs.
func (w *Writer) Flush() error {
	events := w.events
	w.events = nil
	return w.putEvents(context.Background(), events)
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
	req.SetContext(ctx)
	resp, err := req.Send()
	if err != nil {
		w.nextSequenceToken = nil
		return err
	}
	w.nextSequenceToken = resp.NextSequenceToken
	return nil
}

// Close closes the Writer.
func (w *Writer) Close() error {
	return w.Flush()
}
