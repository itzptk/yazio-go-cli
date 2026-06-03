BINARY := bin/yazio
GO := /usr/local/go/bin/go
DOCKER_UID := $(shell id -u)
DOCKER_GID := $(shell id -g)

.PHONY: test build docker-test docker-build fmt

test:
	$(GO) test ./...

build:
	mkdir -p bin
	$(GO) build -o $(BINARY) ./cmd/yazio

fmt:
	$(GO) fmt ./...

docker-test:
	docker run --rm -u $(DOCKER_UID):$(DOCKER_GID) -v $(CURDIR):/src -w /src --entrypoint sh golang:1.24-bookworm -c '/usr/local/go/bin/go test ./...'

docker-build:
	docker run --rm -u $(DOCKER_UID):$(DOCKER_GID) -v $(CURDIR):/src -w /src --entrypoint sh golang:1.24-bookworm -c 'mkdir -p bin && /usr/local/go/bin/go build -o bin/yazio ./cmd/yazio'
