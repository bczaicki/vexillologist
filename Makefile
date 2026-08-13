VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/phillips-connect/vexillologist/internal/cli.Version=$(VERSION)

.PHONY: build test lint run clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/vex ./cmd/vex

test:
	go test ./...

lint:
	golangci-lint run

run: build
	./bin/vex

clean:
	rm -rf bin
