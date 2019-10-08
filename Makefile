PKG=github.com/awslabs/aws-cloudmap-prometheus-sd
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
# Docker
IMAGE=awslabs/aws-cloudmap-prometheus-sd
REPO=$(AWS_ACCOUNT).dkr.ecr.$(AWS_REGION).amazonaws.com/$(IMAGE)
VERSION=v0.0.1

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/aws-cloudmap-prometheus-sd

.PHONY: darwin
darwin:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin go build -ldflags ${LDFLAGS} -o bin/aws-cloudmap-prometheus-sd-osx

.PHONY: image
image:
	docker build -t $(IMAGE):latest .

.PHONY: image-release
image-release:
	docker build -t $(IMAGE):$(VERSION) .

.PHONY: push
push:
ifeq ($(AWS_ACCOUNT),)
	$(error AWS_ACCOUNT is not set)
endif
	docker tag $(IMAGE):latest $(REPO):latest
	docker push $(REPO):latest

.PHONY: push-release
push-release:
	docker tag $(IMAGE):$(VERSION) $(REPO):$(VERSION)
	docker push $(REPO):$(VERSION)

PACKAGES:=$(shell go list ./... | sed -n '1!p' | grep ${PKG}/pkg)
test:
	echo "mode: count" > coverage-all.out
	$(foreach pkg,$(PACKAGES), \
		go test -p=1 -cover -covermode=count -coverprofile=coverage.out ${pkg}; \
		tail -n +2 coverage.out >> coverage-all.out;)

cover: test
	go tool cover -html=coverage-all.out

.PHONY: clean
clean:
	rm -rf ./bin

