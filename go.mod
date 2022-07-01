module github.com/awslabs/aws-cloudmap-prometheus-sd

go 1.13

require (
	github.com/aws/aws-sdk-go v1.44.46
	github.com/go-kit/kit v0.12.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.35.0
	github.com/prometheus/prometheus v0.0.0-20190818123050-43acd0e2e93f
	github.com/stretchr/testify v1.7.5
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
