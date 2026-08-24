# Security

## Reporting a vulnerability

Please report security issues privately through GitHub Security Advisories on this repository. Do not open a public issue for vulnerabilities.

## Operational guidance

`reliability-proxy` is a local testing tool. Treat it like any other proxy:

- It binds to `127.0.0.1:8080` by default so it is not exposed on every interface.
- In Docker, bind explicitly with `--listen :8080` only on trusted networks.
- The upstream target comes from trusted configuration (`--target` or `proxy.target`). Requests cannot choose a different origin.
- Logs omit Authorization, Cookie, request bodies, and query strings.
- Traffic is not persisted.
- `/__reliability/*` is reserved for proxy internals and is not forwarded upstream.

Connection reset simulation uses HTTP/1.x hijacking. Do not assume it works for HTTP/2.
