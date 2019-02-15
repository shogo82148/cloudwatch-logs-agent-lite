package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	agent "github.com/shogo82148/cloudwatch-logs-agent-lite"
)

func main() {
	var groupName, streamName string
	var interval time.Duration
	var version bool
	flag.StringVar(&groupName, "log-group-name", "", "log group name")
	flag.StringVar(&streamName, "log-stream-name", "", "log stream name")
	flag.DurationVar(&interval, "flush-interval", time.Second, "flush interval to flush the logs")
	flag.BoolVar(&version, "version", false, "show the version")
	flag.Parse()

	if version {
		fmt.Printf("cloudwatch-logs-agent-lite v%s\n", agent.Version)
		return
	}
	if groupName == "" {
		log.Fatal("-log-group-name is required.")
	}
	if streamName == "" {
		streamName = generateStreamName()
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal("fail to load aws config: ", err)
	}

	a := &agent.Agent{
		Writer: &agent.Writer{
			Config:        cfg,
			LogGroupName:  groupName,
			LogStreamName: streamName,
		},
		Files:         flag.Args(),
		FlushInterval: interval,
	}
	if err := a.Start(); err != nil {
		log.Fatal("fail to start: ", err)
	}

	// handle signals
	chsig := make(chan os.Signal, 1)
	signal.Notify(
		chsig,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	var chshutdown chan error
	for {
		select {
		case s := <-chsig:
			if chshutdown == nil {
				log.Printf("received signal %s, shutting down...", s)
				chshutdown = make(chan error, 1)
				go func() {
					chshutdown <- a.Close()
				}()
			} else {
				log.Fatalf("received signal %s, force to close", s)
			}
		case err := <-chshutdown:
			if err != nil {
				log.Fatal("fail to close: ", err)
			}
			return
		}
	}
}

func generateStreamName() string {
	// default: AWS EC2 Instance ID
	if name := getAWSInstanceID(); name != "" {
		return name
	}
	// fall back to hostname
	if name, err := os.Hostname(); err == nil {
		return name
	}
	// fall back to /dev/random
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	// fall back to timestamp
	return fmt.Sprintf("%09d", time.Now().Nanosecond())
}

func getAWSInstanceID() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/instance-id", nil)
	if err != nil {
		return ""
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var builder strings.Builder
	if _, err := io.Copy(&builder, resp.Body); err != nil {
		return ""
	}
	return strings.TrimSpace(builder.String())
}
