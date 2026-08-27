# Changelog

All notable changes to this project are documented in this file.

## 0.1.0 - unreleased

Initial release of Go API Reliability Proxy.

- Reverse HTTP proxy with streaming request and response bodies
- YAML configuration and CLI flags (`--config`, `--target`, `--listen`, `--seed`,
  `--log-level`, `--log-format`, `--version`)
- Path matching with exact paths and trailing `/*` prefix wildcards
- HTTP method matching; omitting `methods` matches every method, while a blank
  entry is rejected at startup
- First matching rule wins
- Fixed and random latency injection with context cancellation
- Probabilistic failure injection
- Synthetic HTTP responses with a body and headers that may carry several values
- Timeout simulation that returns 504 if the client is still connected
- HTTP/1.x connection reset via hijacking, emitting a real TCP RST
- Configuration validation that rejects unreachable effect combinations,
  non-token HTTP methods, and headers managed by the server
- Fixed upstream dial, TLS, response header and idle timeouts
- Reserved internal endpoints: `/__reliability/health` and `/__reliability/status`
- In-process atomic metrics separating injected effects from affected requests
- Structured logs with `log/slog`, text or JSON, that omit Authorization,
  Cookie, bodies, and query strings
- Graceful shutdown on SIGINT and SIGTERM that drains in-flight requests
- Unit tests, integration tests, race detector, and golangci-lint
- Docker image, GoReleaser binary matrix, and a tag-triggered release workflow
