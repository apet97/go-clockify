# syntax=docker/dockerfile:1

# Stage 1: Build the MCP server
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build

COPY go.mod ./
# No go.sum needed — stdlib only, zero external dependencies
COPY cmd/ cmd/
COPY internal/ internal/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath \
    -ldflags "-s -w \
        -X main.version=${VERSION} \
        -X main.commit=${COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o clockify-mcp \
    ./cmd/clockify-mcp

# Stage 2: Minimal runtime image (~10MB)
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# OCI image labels for registry UX and supply-chain tooling. org.opencontainers.*
# is the standard; GitHub + GHCR + Grafana + other scanners index these.
LABEL org.opencontainers.image.title="clockify-mcp" \
      org.opencontainers.image.description="Production-grade Go MCP server for Clockify" \
      org.opencontainers.image.source="https://github.com/apet97/go-clockify" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

COPY --from=builder /build/clockify-mcp /usr/local/bin/clockify-mcp

# Run as non-root (distroless "nonroot" resolves to uid/gid 65532).
USER 65532:65532

ENV MCP_TRANSPORT=http
ENV MCP_HTTP_BIND=0.0.0.0:8080
ENV MCP_LOG_FORMAT=json

EXPOSE 8080

STOPSIGNAL SIGTERM

# Liveness probe: verify the binary can execute. Distroless has no shell or
# curl, so we use the exec form with --version, which exits 0 on success.
# Real HTTP health is exposed at /health and /ready for orchestrator probes.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/clockify-mcp", "--version"]

ENTRYPOINT ["clockify-mcp"]
