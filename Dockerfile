FROM golang:1.24-bookworm AS builder

# Version metadata is passed in rather than hardcoded, so the image cannot keep
# reporting an old version after the code moves on. Mirrors what GoReleaser
# injects for release binaries.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /reliability-proxy \
    ./cmd/reliability-proxy

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /reliability-proxy /reliability-proxy

USER nonroot:nonroot
ENTRYPOINT ["/reliability-proxy"]
