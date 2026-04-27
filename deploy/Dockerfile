# syntax=docker/dockerfile:1

# Stage 1: Build the MCP server
#
# Base image is pinned by digest, not just by tag. The human-readable tag is
# kept inline for review ergonomics; the `@sha256:` is what Docker actually
# resolves. Bumping this digest is a deliberate supply-chain action and should
# happen in its own commit alongside a note about what changed upstream
# (toolchain bump, CVE fix, etc.). See docs/runbooks/image-digest-pinning.md.
#
# Digest resolved from `golang:1.25-bookworm` on 2026-04-14 via
#   docker buildx imagetools inspect golang:1.25-bookworm
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm@sha256:29e59af995c51a5bf63d072eca973b918e0e7af4db0e4667aa73f1b8da1a6d8c AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build

# Copy the workspace manifests first so the layer cache survives source
# edits. The default (untagged) build uses only the root go.mod (stdlib
# only, no external deps), but `--build-arg GO_TAGS=grpc[,postgres]`
# triggers blank-imports of the workspace sub-modules
# (internal/transport/grpc, internal/controlplane/postgres,
# internal/tracing/otel), each of which has its own go.mod and go.sum.
# The Go toolchain resolves them via go.work — without go.work and the
# matching sub-module manifests in the build context the build either
# fails offline ("module lookup disabled by GOPROXY=off") or silently
# downloads a stale remote version of the sub-module. Either outcome
# breaks reproducibility for custom GO_TAGS images. Copying the
# manifests up-front is cheap (a handful of files) and keeps the
# default build untouched.
COPY go.mod go.work go.work.sum ./
COPY internal/tracing/otel/go.mod internal/tracing/otel/go.sum internal/tracing/otel/
COPY internal/transport/grpc/go.mod internal/transport/grpc/go.sum internal/transport/grpc/
COPY internal/controlplane/postgres/go.mod internal/controlplane/postgres/go.sum internal/controlplane/postgres/
COPY cmd/ cmd/
COPY internal/ internal/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
# GO_TAGS opts a custom image build into a tagged binary (e.g. grpc,
# postgres, fips). The default image build leaves it empty, which keeps
# the published `ghcr.io/apet97/go-clockify` image stdlib-only per
# ADR 0001. Operators rolling out the private-network gRPC profile can
# build a custom image with:
#   docker build --build-arg GO_TAGS=grpc,postgres -f deploy/Dockerfile -t clockify-mcp:grpc-postgres .
# An empty `-tags=""` is a no-op as far as the Go toolchain is
# concerned, so the default image continues to build the stdlib-only
# binary even with this build-arg present.
ARG GO_TAGS=""
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -tags="${GO_TAGS}" \
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
      org.opencontainers.image.description="Go MCP server for Clockify" \
      org.opencontainers.image.source="https://github.com/apet97/go-clockify" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

COPY --from=builder /build/clockify-mcp /usr/local/bin/clockify-mcp

# Run as non-root (distroless "nonroot" resolves to uid/gid 65532).
USER 65532:65532

# Default to the spec-strict streamable HTTP transport. Legacy http is
# still selectable via MCP_TRANSPORT=http but is deprecated for new
# deployments — it drops server-initiated notifications and is unsuitable
# for shared-service hosting. See docs/adr/ for the rationale.
ENV MCP_TRANSPORT=streamable_http
ENV MCP_HTTP_BIND=0.0.0.0:8080
ENV MCP_LOG_FORMAT=json
# Reject Host headers that don't match the bound address; mitigates
# DNS-rebinding when the server is exposed beyond loopback.
ENV MCP_STRICT_HOST_CHECK=1

EXPOSE 8080

STOPSIGNAL SIGTERM

# Liveness probe: verify the binary can execute. Distroless has no shell or
# curl, so we use the exec form with --version, which exits 0 on success.
# Real HTTP health is exposed at /health and /ready for orchestrator probes.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD ["/usr/local/bin/clockify-mcp", "--version"]

ENTRYPOINT ["clockify-mcp"]
