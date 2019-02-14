package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	agent "github.com/shogo82148/cloudwatch-logs-agent-lite"
)

func main() {
	var groupName, streamName string
	flag.StringVar(&groupName, "log-group-name", "", "log group name")
	flag.StringVar(&streamName, "log-stream-name", "", "log stream name")
	flag.Parse()

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
