package agent

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	tail "github.com/shogo82148/go-tail"
)

//go:generate mockgen -destination mock_cloudwatchlogsiface/mock.go "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/cloudwatchlogsiface" ClientAPI

// Agent is a CloudWatch Logs Agent Lite.
type Agent struct {
	*Writer

	// Files are target file names for tailing.
	// If the length of Files is zero, the standard input is used.
	Files []string

	// FlushInterval specifies the flush interval
	// to flush to the logs.
	// If zero, no periodic flushing is done.
	FlushInterval time.Duration

	wg     sync.WaitGroup
	tails  []*tail.Tail
	lines  chan *tail.Line
	errors chan error

	closeOnce sync.Once
	closeErr  error
}

// Start starts log forwarding.
func (a *Agent) Start() error {
	a.lines = make(chan *tail.Line, 16)
	a.errors = make(chan error, 1)
	files := a.Files
	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, f := range files {
		var t *tail.Tail
		var err error
		if f == "-" {
			t, err = tail.NewTailReader(os.Stdin)
		} else {
			t, err = tail.NewTailFile(f)
		}
		if err != nil {
			for _, t := range a.tails {
				t.Close()
			}
			return err
		}
		a.wg.Add(1)
		go a.runTail(t)
		a.tails = append(a.tails, t)
	}
	a.wg.Add(1)
	go a.runForward()
	return nil
}

// Close stops log forwarding.
func (a *Agent) Close() error {
	a.closeOnce.Do(func() {
		var ferr error
		for _, t := range a.tails {
			err := t.Close()
			if err != nil && ferr == nil {
				ferr = err
			}
		}
		a.wg.Wait()
	})
	return a.closeErr
}

// Wait waits for all readers are closed.
func (a *Agent) Wait() {
	a.wg.Wait()
}

func (a *Agent) runTail(t *tail.Tail) {
	defer a.wg.Done()
	defer close(a.errors)
	defer close(a.lines)
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				return
			}
			a.lines <- line
		case err, ok := <-t.Errors:
			if ok {
				a.errors <- err
			}
		}
	}
}

func (a *Agent) runForward() {
	defer a.wg.Done()

	var flush <-chan time.Time
	if a.FlushInterval > 0 {
		ticker := time.NewTicker(a.FlushInterval)
		defer ticker.Stop()
		flush = ticker.C
	}

LOOP:
	for {
		select {
		case line, ok := <-a.lines:
			if !ok {
				break LOOP
			}
			text := strings.TrimSpace(line.Text)
			_, err := a.WriteEvent(line.Time, text)
			if err != nil {
				log.Println("Error: ", err)
			}
		case err, ok := <-a.errors:
			if ok {
				log.Println("Error: ", err)
			}
		case <-flush:
			err := a.Flush()
			if err != nil {
				log.Println("Error: ", err)
			}
		}
	}

	if err := a.Writer.Close(); err != nil {
		a.closeErr = err
	}
}
