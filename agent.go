package agent

import (
	"log"
	"os"
	"strings"
	"sync"

	tail "github.com/shogo82148/go-tail"
)

// Agent is a CloudWatch Logs Agent Lite.
type Agent struct {
	*Writer
	Files []string

	wg     sync.WaitGroup
	tails  []*tail.Tail
	lines  chan *tail.Line
	errors chan error

	closeOnce sync.Once
	closeErr  error
}

// Start starts log fowarding.
func (a *Agent) Start() error {
	a.lines = make(chan *tail.Line, 16)
	a.errors = make(chan error, 1)
	for _, f := range a.Files {
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
	go a.runFoward()
	return nil
}

// Close stops log fowarding.
func (a *Agent) Close() error {
	a.closeOnce.Do(func() {
		var ferr error
		for _, t := range a.tails {
			err := t.Close()
			if err != nil && ferr == nil {
				ferr = err
			}
		}
		if err := a.Writer.Close(); err != nil && ferr == nil {
			ferr = err
		}
		a.closeErr = ferr
		a.wg.Wait()
	})
	return a.closeErr
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

func (a *Agent) runFoward() {
	defer a.wg.Done()
	for {
		select {
		case line, ok := <-a.lines:
			if !ok {
				return
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
		}
	}
}
