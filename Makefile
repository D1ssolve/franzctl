VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test fmt vet smoke install snapshot

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/franzctl ./cmd/franzctl

test:
	go test -race ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

smoke: build
	./scripts/smoke.sh ./bin/franzctl

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/franzctl

snapshot:
	goreleaser release --snapshot --clean --skip=publish
