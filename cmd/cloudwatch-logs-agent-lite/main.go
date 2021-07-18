package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/hashicorp/logutils"
	agent "github.com/shogo82148/cloudwatch-logs-agent-lite"
)

var defaultLogLevel string = "info"

func init() {
	if level := os.Getenv("CLOUDWATCH_LOG_LEVEL"); level != "" {
		defaultLogLevel = level
	}
}

// we want to collect as many logs as possible, so do more retries.
const maxAttempts = 10

// api calls should not take more defaultTimeout, even if they retry.
const defaultTimeout = retry.DefaultMaxBackoff * (maxAttempts - 1)

// the version is set by goreleaser
var version = "" // .Version

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var groupName, streamName string
	var region string
	var logRetentionDays int
	var interval time.Duration
	var timeout time.Duration
	var logLevel string
	var showVersion bool
	flag.StringVar(&groupName, "log-group-name", "", "log group name")
	flag.StringVar(&streamName, "log-stream-name", "", "log stream name")
	flag.StringVar(&region, "region", "", "aws region")
	flag.IntVar(&logRetentionDays, "log-retention-days", 0, "Specifies the number of days you want to retain log events in the specified log group. Possible values are: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653")
	flag.DurationVar(&interval, "flush-interval", 5*time.Second, "interval to flush the logs")
	flag.DurationVar(&timeout, "flush-timeout", defaultTimeout, "timeout to flush the logs")
	flag.StringVar(&logLevel, "log-level", defaultLogLevel, "minimum log level. Possible values are: debug, info, warn, error")
	flag.BoolVar(&showVersion, "version", false, "show the version")
	flag.Parse()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(strings.ToUpper(logLevel)),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	if showVersion {
		fmt.Printf("cloudwatch-logs-agent-lite version %s %s/%s (built by %s)\n", getVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if groupName == "" {
		log.Fatal("[ERROR] -log-group-name is required.")
	}
	if !isValidLogRetentionDays(logRetentionDays) {
		log.Fatalf("[ERROR] invalid log retention days. Possible values are: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653: %d", logRetentionDays)
	}
	if streamName == "" {
		streamName = generateStreamName(ctx)
	}

	opts := make([]func(*config.LoadOptions) error, 0, 2)
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	// overwrite max attempts of retry
	opts = append(opts, config.WithRetryer(func() aws.Retryer {
		return retry.AddWithMaxAttempts(retry.NewStandard(), maxAttempts)
	}))

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Fatal("[ERROR] fail to load aws config: ", err)
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
		FlushTimout:   timeout,
	}
	if err := a.Start(); err != nil {
		log.Fatal("[ERROR] fail to start: ", err)
	}
	log.Printf("[DEBUG] cloudwatch-logs-agent-lite version %s %s/%s (built by %s) is started",
		getVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version())

	chwait := make(chan struct{})
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
				log.Printf("[INFO] received signal %s, shutting down...", s)
				chshutdown = make(chan error, 1)
				go func() {
					chshutdown <- a.Close()
				}()
			} else {
				log.Printf("[WARN] received signal %s, force to close", s)
				return
			}
		case err := <-chshutdown:
			if err != nil {
				log.Print("[ERROR] fail to close: ", err)
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
	id, err := io.ReadAll(out.Content)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(id))
}

func isValidLogRetentionDays(days int) bool {
	validValues := [...]int{
		0, // the default value: do not set log retention days
		1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653,
	}
	for _, v := range validValues {
		if days == v {
			return true
		}
	}
	return false
}

func getVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return info.Main.Version
}
