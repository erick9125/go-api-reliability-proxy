# Changelog

All notable changes to this project are documented in this file.

## 0.1.0 - 2026-08-24

Initial release of Go API Reliability Proxy.

- Reverse HTTP proxy with streaming request and response bodies
- YAML configuration and CLI flags (`--config`, `--target`, `--listen`, `--seed`, `--version`)
- Path matching with exact paths and trailing `/*` prefix wildcards
- HTTP method matching, with empty methods meaning all methods
- First matching rule wins
- Fixed and random latency injection with context cancellation
- Probabilistic failure injection
- Synthetic HTTP responses, including headers and body
- Timeout simulation that returns 504 if the client is still connected
- HTTP/1.x connection reset via hijacking
- Reserved internal endpoints: `/__reliability/health` and `/__reliability/status`
- Structured logs with `log/slog` that omit Authorization, Cookie, bodies, and query strings
- In-process atomic metrics
- Graceful shutdown on SIGINT and SIGTERM
- Unit tests, integration tests, and race detector coverage
- Docker image and GoReleaser binary matrix
