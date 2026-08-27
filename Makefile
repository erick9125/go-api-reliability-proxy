.PHONY: test race vet lint fmt build run docker

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

# Requires golangci-lint:
#   go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

build:
	go build -o bin/reliability-proxy ./cmd/reliability-proxy

run:
	go run ./cmd/reliability-proxy

docker:
	docker build -t reliability-proxy .
