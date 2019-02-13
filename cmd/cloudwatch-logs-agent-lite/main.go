package main

import (
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	agent "github.com/shogo82148/cloudwatch-logs-agent-lite"
)

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}

	w := &agent.Writer{
		Config:        cfg,
		LogGroupName:  "log-group",
		LogStreamName: "log-stream",
	}
	_, err = io.Copy(w, os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}
