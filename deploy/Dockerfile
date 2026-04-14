# syntax=docker/dockerfile:1

# Stage 1: Build the MCP server
#
# Base image is pinned by digest, not just by tag. The human-readable tag is
# kept inline for review ergonomics; the `@sha256:` is what Docker actually
# resolves. Bumping this digest is a deliberate supply-chain action and should
# happen in its own commit alongside a note about what changed upstream
# (toolchain bump, CVE fix, etc.). See docs/runbooks/w2-12-digest-pinning.md.
#
# Digest resolved from `golang:1.25-bookworm` on 2026-04-14 via
#   docker buildx imagetools inspect golang:1.25-bookworm
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm@sha256:29e59af995c51a5bf63d072eca973b918e0e7af4db0e4667aa73f1b8da1a6d8c AS builder
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
#
# Runtime base is pinned by digest for the same reasons as the builder. The
# `nonroot` tag is a moving pointer that Google retags on every distroless
# update; without a digest pin, two back-to-back releases could ship different
# base layers with no repo diff, defeating the reproducibility proofs in
# reproducibility.yml.
#
# Digest resolved from `gcr.io/distroless/static-debian12:nonroot` on
# 2026-04-14 via
#   docker buildx imagetools inspect gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1

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
