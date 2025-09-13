package agent

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	tail "github.com/shogo82148/go-tail"
)

// for testing
var stdin = os.Stdin

// Agent is a CloudWatch Logs Agent Lite.
type Agent struct {
	*Writer

	// Files are target file names for tailing.
	// If the length of Files is zero, the standard input is used.
	Files []string

	// FlushInterval specifies the interval
	// to flush to the logs.
	// If zero, no periodic flushing is done.
	FlushInterval time.Duration

	// FlushTimeout specifies the timeout
	// to flush to the logs.
	// If zero, flushing is never timeout.
	FlushTimeout time.Duration

	wgTails        sync.WaitGroup // for waiting all tails are closed
	wg             sync.WaitGroup // for waiting runForward is finished
	tails          []*tail.Tail
	lines          chan *tail.Line
	errors         chan error
	closeLinesOnce sync.Once

	closeOnce sync.Once
	closeErr  error
}

// Start starts log forwarding.
func (a *Agent) Start() error {
	a.lines = make(chan *tail.Line, 16*maximumLogEventsPerPut)
	a.errors = make(chan error, 1)
	files := a.Files
	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, f := range files {
		var t *tail.Tail
		var err error
		opts := tail.Options{
			MaxBytesLine: maximumBytesPerPut,
		}
		if f == "-" {
			t, err = tail.NewTailReaderWithOptions(stdin, opts)
		} else {
			t, err = tail.NewTailFileWithOptions(f, opts)
		}
		if err != nil {
			for _, t := range a.tails {
				t.Close()
			}
			return err
		}
		a.wgTails.Go(func() {
			a.runTail(t)
		})
		a.tails = append(a.tails, t)
	}
	a.wg.Go(a.runForward)
	return nil
}

// Close stops log forwarding.
func (a *Agent) Close() error {
	err := a.closeTails()
	a.wg.Wait()
	return err
}

func (a *Agent) closeTails() error {
	a.closeOnce.Do(func() {
		var ferr error
		for _, t := range a.tails {
			err := t.Close()
			if err != nil && ferr == nil {
				ferr = err
			}
		}
		a.closeErr = ferr

		a.wgTails.Wait()
		a.closeLines()
	})
	return a.closeErr
}

func (a *Agent) closeLines() {
	a.closeLinesOnce.Do(func() {
		close(a.errors)
		close(a.lines)
	})
}

// Wait waits for all readers are closed.
func (a *Agent) Wait() {
	a.wgTails.Wait()
	a.closeLines()
	a.wg.Wait()
}

func (a *Agent) runTail(t *tail.Tail) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var dropped uint64
	errors := t.Errors
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				return
			}
			select {
			case a.lines <- line:
			default:
				dropped++
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
				break
			}
			a.errors <- err
		case <-ticker.C:
			if dropped != 0 {
				log.Printf("[WARN] %d line(s) are dropped", dropped)
				dropped = 0
			}
		}
	}
}

func (a *Agent) runForward() {
LOOP1:
	for {
		select {
		case line, ok := <-a.lines:
			if !ok {
				break LOOP1
			}
			text := strings.TrimSpace(line.Text)
			err := a.writeEventWithTimeout(line.Time, text)
			if err == nil {
				err = a.flushWithTimeout()
			}
			if err != nil {
				log.Println("[ERROR] writing the first log failed:", err)
				log.Println("[ERROR] Your configuration might be wrong. So I can't continue to forward logs.")
				log.Println("[ERROR] Please check it.")
				_ = a.closeTails()
			}
			break LOOP1
		case err, ok := <-a.errors:
			if ok {
				log.Println("[ERROR] reading files failed:", err)
			}
		}
	}

	var flush <-chan time.Time
	if a.FlushInterval > 0 {
		ticker := time.NewTicker(a.FlushInterval)
		defer ticker.Stop()
		flush = ticker.C
	}

LOOP2:
	for {
		select {
		case line, ok := <-a.lines:
			if !ok {
				break LOOP2
			}
			text := strings.TrimSpace(line.Text)
			err := a.writeEventWithTimeout(line.Time, text)
			if err != nil {
				log.Println("[ERROR] writing logs failed:", err)
				// We guess it is a temporary error, because writing logs have succeeded at least once.
				// so continue to continue to forward logs.
			}
		case err, ok := <-a.errors:
			if ok {
				log.Println("[ERROR] reading files failed:", err)
			}
		case <-flush:
			// check whether the logs are flushed recently
			flushed := a.LastFlushedTime()
			if flushed.IsZero() || time.Since(flushed) >= a.FlushInterval {
				// the logs aren't flushed recently, we need to flush.
				log.Println("[DEBUG] periodic flushing")
				err := a.flushWithTimeout()
				if err != nil {
					log.Println("[ERROR] periodic flushing failed:", err)
					// We guess it is a temporary error, because writing logs have succeeded at least once.
					// so continue to continue to forward logs.
				}
			}
		}
	}

	log.Println("[DEBUG] closing the writer")
	if err := a.closeWithTimeout(); err != nil {
		log.Println("[ERROR] closing the writer failed:", err)
	}
}

func (a *Agent) timeoutContext() (context.Context, context.CancelFunc) {
	if a.FlushTimeout > 0 {
		return context.WithTimeout(context.Background(), a.FlushTimeout)
	}
	return context.Background(), func() {}
}

func (a *Agent) writeEventWithTimeout(now time.Time, text string) error {
	ctx, cancel := a.timeoutContext()
	defer cancel()
	_, err := a.WriteEventContext(ctx, now, text)
	return err
}

func (a *Agent) flushWithTimeout() error {
	ctx, cancel := a.timeoutContext()
	defer cancel()
	return a.FlushContext(ctx)
}

func (a *Agent) closeWithTimeout() error {
	ctx, cancel := a.timeoutContext()
	defer cancel()
	return a.CloseContext(ctx)
}
