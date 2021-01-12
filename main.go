package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/awslabs/aws-cloudmap-prometheus-sd/pkg/adapter"
	"github.com/awslabs/aws-cloudmap-prometheus-sd/pkg/discovery"
	"github.com/go-kit/kit/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	a                     = kingpin.New("aws-cloudmap-prometheus-sd usage", "Tool to generate file_sd target files for AWS Cloud Map services.")
	outputFileFlag        = a.Flag("output.file", "Output file for file_sd compatible file.").Default("cloudmap_sd.json").String()
	awsRegionFlag         = a.Flag("aws.region", "AWS Region to use. If none provided, region will be auto-discovered by AWS SDK using environment.").String()
	cloudmapNamespaceFlag = a.Flag("cloudmap.namespace", "CloudMap namespace to discovery services. If none provided all namespaces will be discovered").String()
	refreshIntervalFlag   = a.Flag("target.refresh", "The refresh interval (in seconds).").Default("60").Int()
	logger                log.Logger
)

func main() {
	a.HelpFlag.Short('h')

	_, err := a.Parse(os.Args[1:])
	if err != nil {
		fmt.Println("err: ", err)
		return
	}
	logger = log.NewSyncLogger(log.NewLogfmtLogger(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	ctx := context.Background()

	cfg := discovery.SDConfig{
		RefreshInterval: *refreshIntervalFlag,
		CloudmapNamespace: cloudmapNamespaceFlag,
	}
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("err: ", err)
		return
	}
	if aws.StringValue(awsRegionFlag) != "" {
		cfg.Region = aws.StringValue(awsRegionFlag)
	} else {
		metadata := ec2metadata.New(sess)
		region, err := metadata.Region()
		if err != nil {
			fmt.Println("err: SD configuration requires a region")
			return
		}
		cfg.Region = region
	}

	disc, err := discovery.NewDiscovery(logger, cfg)
	if err != nil {
		fmt.Println("err: ", err)
	}
	sdAdapter := adapter.NewAdapter(ctx, *outputFileFlag, "aws-cloudmap-prometheus-sd", disc, logger)
	sdAdapter.Run()

	<-ctx.Done()
}
