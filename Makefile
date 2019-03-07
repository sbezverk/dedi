REGISTRY_NAME = docker.io/sbezverk
IMAGE_VERSION = 0.0.0

.PHONY: all dispatcher container push clean test

ifdef V
TESTARGS = -v -args -alsologtostderr -v 5
else
TESTARGS =
endif

all: dispatcher

dispatcher:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ./bin/dispatcher ./cmd/dispatcher

mac-dispatcher:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin go build -a -ldflags '-extldflags "-static"' -o ./bin/dispatcher.mac ./cmd/dispatcher

container: sriov-controller
	docker build -t $(REGISTRY_NAME)/sriov-controller:$(IMAGE_VERSION) -f ./Dockerfile .

push: container
	docker push $(REGISTRY_NAME)/sriov-controller:$(IMAGE_VERSION)

clean:
	rm -rf bin

test:
	go test `go list ./... | grep -v 'vendor'` $(TESTARGS)
	go vet `go list ./... | grep -v vendor`
