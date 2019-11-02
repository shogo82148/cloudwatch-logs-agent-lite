#!/bin/sh

CURRENT=$(cd "$(dirname "$0")" && pwd)
docker run --rm -it \
    -e GO111MODULE=on \
    -e "GOOS=${GOOS:-linux}" -e "GOARCH=${GOARCH:-amd64}" -e "CGO_ENABLED=${CGO_ENABLED:-1}" \
    -v "$CURRENT/.mod":/go/pkg/mod \
    -v "$CURRENT":/go/src/github.com/shogo82148/cloudwatch-logs-agent-lite \
    -w /go/src/github.com/shogo82148/cloudwatch-logs-agent-lite golang:1.13.4 "$@"
