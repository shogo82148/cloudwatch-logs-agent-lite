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
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/ec2imds"
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

	cfg, err := config.LoadDefaultConfig()
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
	cfg, err := config.LoadDefaultConfig()
	if err != nil {
		return ""
	}

	// use a shorter timeout than default because the metadata
	// service is local if it is running, and to fail faster
	// if not running on an ec2 instance.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	svc := ec2imds.NewFromConfig(cfg)
	out, err := svc.GetMetadata(ctx, &ec2imds.GetMetadataInput{Path: "instance-id"})
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
