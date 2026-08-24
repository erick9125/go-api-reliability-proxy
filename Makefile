.PHONY: test race lint fmt build run docker

test:
	go test ./...

race:
	go test -race ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

build:
	go build -o bin/reliability-proxy ./cmd/reliability-proxy

run:
	go run ./cmd/reliability-proxy

docker:
	docker build -t reliability-proxy .
