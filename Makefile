BINARY := bin/herdr-kibitzr
GOBIN  := $(shell go env GOPATH)/bin

.PHONY: all build test race smoke mutate fmt fmt-check lint vuln qa clean tools

all: qa build

build:
	go build -o $(BINARY) ./cmd/herdr-kibitzr

test:
	go test ./...

race:
	go test -race ./...

smoke:
	bash herdr/smoke.sh

mutate:
	bash herdr/mutate.sh

fmt:
	$(GOBIN)/golangci-lint fmt

fmt-check:
	$(GOBIN)/golangci-lint fmt --diff

lint:
	$(GOBIN)/golangci-lint run

vuln:
	$(GOBIN)/govulncheck ./...

qa: fmt-check lint race mutate smoke vuln

tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

clean:
	rm -rf bin coverage.out
