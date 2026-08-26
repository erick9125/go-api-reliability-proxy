# Go API Reliability Proxy

A fault-injection HTTP proxy for testing API resilience with latency, timeouts, failures, rate limits, connection resets, and configurable traffic rules.

Go API Reliability Proxy sits between your application and an upstream HTTP API and intentionally introduces latency, server failures, timeouts, connection resets, and other adverse conditions to help validate retry, timeout, idempotency, fallback, and recovery behavior.

[Español](README.es.md)

**Topics:** `go` `golang` `http` `proxy` `reliability` `resilience` `fault-injection` `chaos-engineering` `api-testing` `developer-tools` `networking` `cli` `observability`

## Problem

When you develop an HTTP client you usually test this:

```
request → API → 200 OK
```

Production looks more like:

```
request → latency → 429 → retry → timeout → 503 → network reset → eventual success
```

The proxy sits between the client and the API:

```
Application → Reliability Proxy → Real API
```

It can manipulate traffic without changing the application or the backend.

```
mobile app → localhost:8080 → proxy → localhost:3000
```

## Features

- Reverse HTTP proxy with streaming bodies
- CLI (`flag`, no extra framework)
- YAML configuration
- Path matching: exact and trailing `/*` prefix
- HTTP method matching (empty list means all methods)
- First matching rule wins
- Fixed and random latency
- Probabilistic failures
- Synthetic HTTP responses, headers, and body
- Timeout simulation
- Connection reset (HTTP/1.x)
- Passthrough when no rule matches
- In-process metrics
- Structured logs
- Graceful shutdown
- Unit tests, integration tests, and race detector

The proxy does **not** retry. It injects conditions that force *your* client to retry.

## Installation

```bash
go install github.com/erick9125/go-api-reliability-proxy/cmd/reliability-proxy@latest
```

Or download a release binary for linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, or windows-amd64. Verify downloads with `checksums.txt`.

Build from source:

```bash
go build -o reliability-proxy ./cmd/reliability-proxy
```

## Quick Start

Upstream API:

```
http://localhost:3000
```

Start the proxy:

```bash
reliability-proxy \
  --target http://localhost:3000 \
  --config reliability.yaml \
  --listen :8080
```

Point the client at:

```
http://localhost:8080
```

Minimal flags:

```bash
reliability-proxy --target http://localhost:3000 --listen :8080
```

Example rule:

```yaml
rules:
  - name: flaky-payments
    match:
      path: "/payments/*"
    effects:
      failure:
        probability: 0.2
        status: 503
```

Then:

```bash
curl http://localhost:8080/payments/1
```

may receive a synthetic `503` without touching the backend.

## Architecture

```
HTTP Request
     ↓
Proxy Handler
     ↓
Rule Matcher
     ↓
Matched Rule (first match)
     ↓
Fault Engine
     ↓
Effects: delay / error / timeout / reset / synthetic response
     ↓
Reverse Proxy (if the request was not stopped)
     ↓
Target API
```

`internal/` packages are not a public Go API. Other modules cannot import them.

## Configuration

CLI flags override YAML, which overrides defaults.

| Source | Example |
| --- | --- |
| CLI | `--listen :9090` |
| YAML | `proxy.listen: ":8080"` |
| Default listen | `127.0.0.1:8080` |

```yaml
version: 1

proxy:
  listen: ":8080"
  target: "http://localhost:3000"

rules:
  - name: payments-latency
    match:
      path: "/payments/*"
      methods:
        - POST
    effects:
      latency:
        fixed: 1200ms
```

See `examples/basic.yaml`, `examples/flaky-api.yaml`, and `examples/rate-limit.yaml`.

Invalid configuration fails at startup. Examples: missing target, non-http(s) target, probability outside `[0, 1]`, HTTP status `999`, `latency.min > latency.max`, unnamed rules, duplicate rule names, and rules whose effects can never run (see [Effect order](#rate-limit-simulation)).

### CLI

```
Usage:

  reliability-proxy [options]

Options:

  --config string
      configuration file

  --target string
      upstream API URL

  --listen string
      proxy listen address (default "127.0.0.1:8080")

  --seed int
      optional RNG seed for deterministic fault injection

  --version
      print version and exit
```

`--version` prints `reliability-proxy v0.1.0`.

`--seed` replays the same probabilistic decisions for **sequential** traffic, which is what makes a scripted run repeatable in CI. It cannot make concurrent traffic deterministic: parallel requests draw from the generator in whatever order the scheduler hands them the lock, so the same seed with the same load assigns different decisions to different requests. Passing an explicit random source in code overrides the seed, and the proxy logs a warning when both are set.

## Rule Matching

Rules are evaluated in declaration order. The first matching rule is applied.

- `/users` is an exact path match
- `/users/*` matches `/users` and any path under `/users/`
- Regex matching is not supported in 0.1
- If `methods` is omitted or empty, all HTTP methods match
- Methods are compared case-insensitively

## Latency Injection

Fixed:

```yaml
latency:
  fixed: 1200ms
```

Random range, chosen per request:

```yaml
latency:
  min: 100ms
  max: 1500ms
```

Sleep respects `context.Context`. If the client cancels, the goroutine exits instead of ignoring cancellation.

## Failure Injection

```yaml
failure:
  probability: 0.25
  status: 503
```

25% of matching requests receive the synthetic status and never reach upstream. Probability is tested with an injectable random source (`random < probability`).

## Timeout Simulation

Latency waits, then still forwards (unless another stopping effect runs). Timeout waits, then returns `504 Gateway Timeout` if the client is still connected:

```yaml
timeout:
  duration: 30s
```

## Connection Reset

```yaml
reset: {}
```

Optional probability:

```yaml
reset:
  probability: 1.0
```

Connection reset simulation is currently intended for HTTP/1.x. It uses `http.Hijacker` and closes the connection. HTTP/2 behavior is not promised.

## Rate Limit Simulation

0.1 does not implement a token bucket. Simulate the client-visible symptom:

```yaml
response:
  status: 429
  headers:
    Retry-After: "10"
  body: |
    {"error":"rate limited"}
```

If you set `Content-Type`, it is preserved and sent as-is. If you omit it, the proxy does not add one — but `net/http` then sniffs the body and fills it in, so a JSON body without an explicit `Content-Type` goes out as `text/plain; charset=utf-8`. Set it explicitly whenever the body's type matters to the client under test.

`failure` is probabilistic. `response` always fires when the rule matches (after latency, and after a failure that did not inject).

Effect order: latency → timeout → reset → failure → response.

The first effect that ends the request wins, so a rule cannot declare an effect that could never run. Configurations where one effect permanently shadows another are rejected at startup rather than silently ignored:

- `timeout` combined with `reset`, `failure`, or `response` — `timeout` always ends the request.
- `reset` without a probability (or with `probability: 1.0`) combined with `failure` or `response` — that reset always fires.

Combinations with a reachable path stay valid: `latency` composes with everything because it never ends the request, `failure` + `response` is the documented pairing above, and a `reset` with `probability` below 1 leaves the effects behind it reachable. Split anything else into separate rules.

## Metrics

`GET /__reliability/status`

```json
{
  "requests": 1024,
  "matched": 311,
  "faultsInjected": 73,
  "proxied": 951
}
```

`GET /__reliability/health`

```json
{
  "status": "ok"
}
```

`/__reliability/*` is reserved for the proxy and is not forwarded upstream.

Full configuration is not exposed (it may contain operational details you do not want on a status endpoint).

## Security

- Default listen address is `127.0.0.1:8080`, not `0.0.0.0`
- Target is fixed per process from trusted configuration. Requests cannot set an upstream with headers such as `X-Target-URL`
- Logs do not include Authorization, Cookie, request bodies, or full query strings
- Traffic is not recorded or persisted
- Do not expose the proxy on untrusted networks

## Upstream Limits

The connection to the upstream uses fixed limits rather than Go's defaults, so a hung upstream cannot pin connections and goroutines indefinitely:

| Limit | Value |
| --- | --- |
| Dial timeout | 10s |
| TLS handshake timeout | 10s |
| Response header timeout | 30s |
| Idle connection timeout | 90s |
| Idle connections per host | 100 |

Injected latency runs *before* the request is forwarded, so it never counts against the response header timeout — only genuine upstream slowness does. An upstream that legitimately takes longer than 30s to send response headers will surface as `502`. These are not configurable in 0.1.

## Limitations

- HTTP only (no gRPC, WebSockets, generic TCP, or Kafka)
- One proxy instance, one target
- No regex, header, or query matching
- No response mutation of real upstream responses
- No config hot reload
- No Prometheus endpoint in 0.1
- Connection reset is HTTP/1.x oriented
- No TLS MITM

## Docker

```bash
docker build -t reliability-proxy .
docker run --rm -p 8080:8080 \
  -v ./reliability.yaml:/config.yaml \
  reliability-proxy \
  --config /config.yaml \
  --listen :8080
```

Inside a container you must listen on `:8080` (or another container-visible address). The process default of `127.0.0.1:8080` would only accept connections from inside the container. The image is based on distroless static with CA certificates so HTTPS upstreams work.

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Integration tests live in `tests/integration` and use `httptest` for both the proxy and a fake upstream.

## Benchmarks

```bash
go test -bench=. -benchmem ./internal/rules
```

Run that command on your machine and publish the numbers you actually measured. This repository does not invent overhead figures.

## Roadmap

**0.2.0:** config hot reload, real rate limiter, response fault injection, regex / header / query matching, per-rule counters, Prometheus, deterministic seed already present in 0.1 via `--seed`.

**0.3.0:** bandwidth throttling, partial responses, response corruption, upstream response delay, scenario sequences (500 then timeout then 200).

**0.4.0:** multiple upstreams, rule groups, profiles, runtime rule API, fault schedules, OpenTelemetry.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
