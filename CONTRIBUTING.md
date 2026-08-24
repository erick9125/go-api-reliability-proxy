# Contributing

Thanks for considering a contribution to Go API Reliability Proxy.

## Development

Requirements:

- Go 1.24 or newer

Useful commands:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/reliability-proxy
```

## Pull requests

- Keep changes focused.
- Run `gofmt`, `go vet`, `go test ./...`, and `go test -race ./...` before opening a PR.
- Prefer table-driven tests.
- Do not log Authorization, Cookie, request bodies, or full query strings.
- Do not add an open-proxy feature such as choosing the upstream from request headers.

## Style

This project follows standard Go conventions:

- small interfaces
- explicit error returns
- `context.Context` as the first argument
- no panic for expected configuration or request errors
