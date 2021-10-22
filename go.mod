module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.17

require (
	github.com/aws/aws-sdk-go-v2 v1.10.0
	github.com/aws/aws-sdk-go-v2/config v1.8.3
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.7.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.8.0
	github.com/google/go-cmp v0.5.6
	github.com/hashicorp/logutils v1.0.0
	github.com/shogo82148/go-tail v0.0.6
	golang.org/x/sys v0.0.0-20210818153620-00dd8d7831e7 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.4.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.2.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.3.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.4.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.7.2 // indirect
	github.com/aws/smithy-go v1.8.1 // indirect
	github.com/fsnotify/fsnotify v1.5.0 // indirect
)
