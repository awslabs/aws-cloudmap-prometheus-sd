FROM golang:1.13-stretch as builder
WORKDIR /go/src/github.com/awslabs/aws-cloudmap-prometheus-sd

# go.mod and go.sum go into their own layers.
COPY go.mod .
COPY go.sum .

# This ensures `go mod download` happens only when go.mod and go.sum change.
RUN go mod download

COPY . .
RUN make

FROM amazonlinux:2
RUN yum install -y ca-certificates
COPY --from=builder /go/src/github.com/awslabs/aws-cloudmap-prometheus-sd/bin/aws-cloudmap-prometheus-sd /bin/aws-cloudmap-prometheus-sd

ENTRYPOINT ["/bin/aws-cloudmap-prometheus-sd"]
