# aws-cloudmap-prometheus-sd

A custom service discovery adapter for Prometheus that integrates with [AWS Cloud Map](https://aws.amazon.com/cloud-map/). This leverages [custom sd](https://github.com/prometheus/prometheus/tree/master/documentation/examples/custom-sd) to output a file that can be passed as `file_sd` in `prometheus.yaml`. This will allow you to pass your targets registered under Cloud Map service to Prometheus for scraping without having to use a static config.

AWS Cloud Map is a cloud resource discovery service. With Cloud Map, you can define custom names for your application resources, and it maintains the updated location of these dynamically changing resources. This increases your application availability because your web service always discovers the most up-to-date locations of its resources.

## Usage

1. Clone this repository
```
git clone https://github.com/awslabs/aws-cloudmap-prometheus-sd
```
2. Build
```
make image

>> awslabs/aws-cloudmap-prometheus-sd
```
3. Run
```
mkdir -p /tmp/output

docker run -v /tmp/output:/output awslabs/aws-cloudmap-prometheus-sd  --help
usage: aws-cloudmap-prometheus-sd usage [<flags>]

Tool to generate file_sd target files for AWS Cloud Map services.

Flags:
  -h, --help                   Show context-sensitive help (also try --help-long
                               and --help-man).
      --output.file="cloudmap_sd.json"
                               Output file for file_sd compatible file.
      --aws.region=AWS.REGION  AWS Region to use. If none provided, region will
                               be auto-discovered by AWS SDK using environment.
      --cloudmap.namespace=CLOUDMAP.NAMESPACE
                               CloudMap namespace to discovery services. If none
                               provided all namespaces will be discovered
      --target.refresh=60      The refresh interval (in seconds).

e.g.
docker run -v /tmp/output:/output awslabs/aws-cloudmap-prometheus-sd  \
    --output.file=/output/cloudmap_sd.json \
    --target.refresh=30 \
    --aws.region=us-east-2
    --cloudmap.namespace=howto-k8s-cloudmap.pvt.aws.local \
```
4. Verify
```
sudo cat /tmp/output/cloudmap_sd.json
```

## Sample file_sd output
```
[
    {
        "targets": [
            "192.168.34.115"
        ],
        "labels": {
            "__meta_cloudmap_namespace_name": "howto-k8s-cloudmap.pvt.aws.local",
            "__meta_cloudmap_service_name": "front"
        }
    },
    {
        "targets": [
            "192.168.35.13",
            "192.168.78.132"
        ],
        "labels": {
            "__meta_cloudmap_namespace_name": "howto-k8s-cloudmap.pvt.aws.local",
            "__meta_cloudmap_service_name": "colorapp"
        }
    }
]
```

## License

This project is licensed under the Apache-2.0 License.


