module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.8.0
	github.com/aws/aws-sdk-go-v2/config v1.6.0
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.4.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.5.1
	github.com/google/go-cmp v0.5.6
	github.com/hashicorp/logutils v1.0.0
	github.com/shogo82148/go-tail v0.0.5
)
