.PHONY: build test clean docker

GO=CGO_ENABLED=0 GO111MODULE=on go
GOCGO=CGO_ENABLED=1 GO111MODULE=on go

MICROSERVICES=cmd/device-gpiod/device-gpiod
.PHONY: $(MICROSERVICES)

DOCKER_TAG=$(VERSION)-dev

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-gpiod.Version=$(VERSION)"
GOTESTFLAGS?=-race

GIT_SHA=$(shell git rev-parse HEAD)

ifdef version
	VERSION=$(version)
else
	VERSION=0.0.0
endif

ifdef arch
	ARCH=$(arch)
else
	ARCH=amd64
endif

ifdef os
	OS=$(os)
else
	OS=linux
endif

build: $(MICROSERVICES)
	$(GOCGO) install -tags=safe

cmd/device-gpiod/device-gpiod:
	go mod tidy
	$(GOCGO) build $(GOFLAGS) -o $@ ./cmd/device-gpiod

docker:
	docker buildx build \
		-f cmd/device-gpiod/Dockerfile \
		-t gufiregistry.cloud.reply.eu/comosyme/device-gpiod:$(VERSION) \
		--platform=$(OS)/$(ARCH) \
		--build-arg TARGETOS=$(OS) \
		--build-arg TARGETARCH=$(ARCH) \
		.

test:
	go mod tidy
	GO111MODULE=on go test $(GOTESTFLAGS) -coverprofile=coverage.out ./...
	GO111MODULE=on go vet ./...
	gofmt -l .
	[ "`gofmt -l .`" = "" ]
	./bin/test-attribution-txt.sh
	./bin/test-go-mod-tidy.sh

clean:
	rm -f $(MICROSERVICES)
