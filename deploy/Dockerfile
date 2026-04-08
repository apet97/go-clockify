# syntax=docker/dockerfile:1

# Stage 1: Build the MCP server
FROM golang:1.25-bookworm AS builder
WORKDIR /build

COPY go.mod ./
# No go.sum needed — stdlib only, zero external dependencies
COPY cmd/ cmd/
COPY internal/ internal/

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o clockify-mcp \
    ./cmd/clockify-mcp

# Stage 2: Minimal runtime image (~10MB)
FROM gcr.io/distroless/static-debian12

COPY --from=builder /build/clockify-mcp /usr/local/bin/clockify-mcp

# Run as non-root
USER nonroot:nonroot

ENV MCP_TRANSPORT=http
ENV MCP_HTTP_BIND=0.0.0.0:8080
ENV MCP_LOG_FORMAT=json

EXPOSE 8080

# Liveness probe: verify the binary can execute. Distroless has no shell or
# curl, so we use the exec form with --version, which exits 0 on success.
# Real HTTP health is exposed at /health and /ready for orchestrator probes.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/clockify-mcp", "--version"]

ENTRYPOINT ["clockify-mcp"]
