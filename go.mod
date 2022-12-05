module github.com/shogo82148/cloudwatch-logs-agent-lite

go 1.19

require (
	github.com/aws/aws-sdk-go-v2 v1.17.2
	github.com/aws/aws-sdk-go-v2/config v1.18.4
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.20
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.17.1
	github.com/google/go-cmp v0.5.9
	github.com/hashicorp/logutils v1.0.0
	github.com/shogo82148/go-tail v1.0.2
	golang.org/x/sys v0.2.0 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.26 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.17.6 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
)
