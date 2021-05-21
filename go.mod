module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.6.0
	github.com/aws/aws-sdk-go-v2/config v1.2.0
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.1.1
	github.com/aws/aws-sdk-go-v2/internal/ini v1.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.3.0
	github.com/shogo82148/go-tail v0.0.5
)
