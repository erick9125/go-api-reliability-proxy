FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=0.1.0" \
    -o /reliability-proxy \
    ./cmd/reliability-proxy

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /reliability-proxy /reliability-proxy

USER nonroot:nonroot
ENTRYPOINT ["/reliability-proxy"]
