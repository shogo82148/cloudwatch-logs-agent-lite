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
	flag.StringVar(&groupName, "log-group-name", "", "log group name")
	flag.StringVar(&streamName, "log-stream-name", "", "log stream name")
	flag.Parse()

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
		Files: flag.Args(),
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
	for {
		select {
		case s := <-chsig:
			log.Printf("received signal %s, shutting down...", s)
			go func() {
				if err := a.Close(); err != nil {
					log.Fatal("fail to close: ", err)
				}
			}()
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
	defer req.Body.Close()
	var builder strings.Builder
	if _, err := io.Copy(&builder, req.Body); err != nil {
		return ""
	}
	return strings.TrimSpace(builder.String())
}
