module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.20

require (
	github.com/aws/aws-sdk-go-v2 v1.22.0
	github.com/aws/aws-sdk-go-v2/config v1.20.0
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.25.0
	github.com/google/go-cmp v0.6.0
	github.com/hashicorp/logutils v1.0.0
	github.com/shogo82148/go-tail v1.0.2
	golang.org/x/sys v0.2.0 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.16.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.18.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.24.0 // indirect
	github.com/aws/smithy-go v1.16.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
)
