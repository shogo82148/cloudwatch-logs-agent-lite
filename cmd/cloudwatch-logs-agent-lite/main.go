package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	agent "github.com/shogo82148/cloudwatch-logs-agent-lite"
)

// these variable is set by goreleaser
var version = "main" // .Version
var commit = "HEAD"  // .ShortCommit

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var groupName, streamName string
	var region string
	var logRetentionDays int
	var interval time.Duration
	var showVersion bool
	flag.StringVar(&groupName, "log-group-name", "", "log group name")
	flag.StringVar(&streamName, "log-stream-name", "", "log stream name")
	flag.StringVar(&region, "region", "", "aws region")
	flag.IntVar(nil, "log-retention-days", 0, "If set to a number greater than zero, and newly create log group's retention policy is set to this many days.")
	flag.DurationVar(&interval, "flush-interval", time.Second, "flush interval to flush the logs")
	flag.BoolVar(&showVersion, "version", false, "show the version")
	flag.Parse()

	if showVersion {
		fmt.Printf("cloudwatch-logs-agent-lite version %s (rev %s) %s/%s (built by %s)\n", version, commit, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if groupName == "" {
		log.Fatal("-log-group-name is required.")
	}
	if streamName == "" {
		streamName = generateStreamName(ctx)
	}

	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Fatal("fail to load aws config: ", err)
	}

	a := &agent.Agent{
		Writer: &agent.Writer{
			Config:           cfg,
			LogGroupName:     groupName,
			LogStreamName:    streamName,
			LogRetentionDays: logRetentionDays,
		},
		Files:         flag.Args(),
		FlushInterval: interval,
	}
	if err := a.Start(); err != nil {
		log.Fatal("fail to start: ", err)
	}

	chwait := make(chan struct{}, 0)
	go func() {
		a.Wait()
		close(chwait)
	}()

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
		case <-chwait:
			if chshutdown == nil {
				return
			}
		}
	}
}

func generateStreamName(ctx context.Context) string {
	// default: AWS EC2 Instance ID
	if name := getAWSInstanceID(ctx); name != "" {
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

func getAWSInstanceID(ctx context.Context) string {
	// use a shorter timeout than default because the metadata
	// service is local if it is running, and to fail faster
	// if not running on an ec2 instance.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ""
	}

	svc := imds.NewFromConfig(cfg)
	out, err := svc.GetMetadata(ctx, &imds.GetMetadataInput{Path: "instance-id"})
	if err != nil {
		return ""
	}
	defer out.Content.Close()
	id, err := ioutil.ReadAll(out.Content)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(id))
}
