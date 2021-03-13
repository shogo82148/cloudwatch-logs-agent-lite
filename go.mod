module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.2.1
	github.com/aws/aws-sdk-go-v2/config v1.1.2
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.0.3
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.1.2
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/shogo82148/go-tail v0.0.3
	golang.org/x/sys v0.0.0-20210313202042-bd2e13477e9c // indirect
)
