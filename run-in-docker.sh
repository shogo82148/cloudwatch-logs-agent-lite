#!/bin/sh

CURRENT=$(cd "$(dirname "$0")" && pwd)
docker run --rm -it \
    -e GO111MODULE=on \
    -v "$CURRENT":/go/src/github.com/shogo82148/cloudwatch-logs-agent-lite \
    -w /go/src/github.com/shogo82148/cloudwatch-logs-agent-lite golang:1.11.5 "$@"
