module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.19

require (
	github.com/aws/aws-sdk-go-v2 v1.17.4
	github.com/aws/aws-sdk-go-v2/config v1.18.11
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.21
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.20.2
	github.com/google/go-cmp v0.5.9
	github.com/hashicorp/logutils v1.0.0
	github.com/shogo82148/go-tail v1.0.2
	golang.org/x/sys v0.2.0 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.28 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.2 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
)
