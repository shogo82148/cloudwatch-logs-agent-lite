module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.15

require (
	github.com/aws/aws-sdk-go-v2 v0.27.0
	github.com/aws/aws-sdk-go-v2/config v0.1.1
	github.com/aws/aws-sdk-go-v2/ec2imds v0.1.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v0.27.0
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/shogo82148/go-tail v0.0.3
	golang.org/x/sys v0.0.0-20200929083018-4d22bbb62b3c // indirect
)
