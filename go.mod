module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.15

require (
	github.com/aws/aws-sdk-go-v2 v1.0.0
	github.com/aws/aws-sdk-go-v2/config v1.0.0
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.0.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.0.0
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/shogo82148/go-tail v0.0.3
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930 // indirect
)
