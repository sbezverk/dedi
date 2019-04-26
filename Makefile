REGISTRY_NAME = docker.io/sbezverk
IMAGE_VERSION = 0.0.1

.PHONY: all dispatcher client server container push clean test

ifdef V
TESTARGS = -v -args -alsologtostderr -v 5
else
TESTARGS =
endif

all: dispatcher client server

dispatcher:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/dispatcher ./cmd/dispatcher

mac-dispatcher:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/dispatcher.mac ./cmd/dispatcher

client:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/client ./cmd/client

mac-client:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/client.mac ./cmd/client

server:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/server ./cmd/server

mac-server:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin GO111MODULE=on go build -a -ldflags '-extldflags "-static"' -o ./bin/server.mac ./cmd/server

container: dispatcher client server
	docker build -t $(REGISTRY_NAME)/dispatch:$(IMAGE_VERSION) -f ./build/Dockerfile.dispatcher .
	docker build -t $(REGISTRY_NAME)/connect:$(IMAGE_VERSION) -f ./build/Dockerfile.client .
	docker build -t $(REGISTRY_NAME)/listen:$(IMAGE_VERSION) -f ./build/Dockerfile.server .

push: container
	docker push $(REGISTRY_NAME)/dispatch:$(IMAGE_VERSION)
	docker push $(REGISTRY_NAME)/connect:$(IMAGE_VERSION)
	docker push $(REGISTRY_NAME)/listen:$(IMAGE_VERSION)

clean:
	rm -rf bin

test:
	GO111MODULE=on go test `go list ./... | grep -v 'vendor'` $(TESTARGS)
	GO111MODULE=on go vet `go list ./... | grep -v vendor`
