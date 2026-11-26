.PHONY: build test lint check

build:
	go build -trimpath -o bin/mlib ./cmd/mlib

test:
	go test -race -shuffle=on ./...

lint:
	golangci-lint run

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	$(MAKE) test
	$(MAKE) lint
