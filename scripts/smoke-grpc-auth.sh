#!/usr/bin/env bash
#
# smoke-grpc-auth.sh
#
# Behavioural smoke for the gRPC auth interceptor. The CI build-tag
# matrix covers compile-time wiring; this script targets the actual
# auth/mTLS *behaviour* so a regression that re-introduces one of the
# locked-down failure modes — Authorization metadata required for mTLS,
# detailed auth errors leaking to clients — trips a dedicated job
# rather than hiding inside the broader test run.
#
# The gRPC transport currently always builds (no `-tags=grpc` wall) so
# a default `go test ./internal/transport/grpc` suffices. The script
# uses `-run` to scope to the auth/mTLS regression tests; running the
# whole package would still catch them but a focused run keeps this
# job's failure output actionable when the broader test job is also
# red.
#
# If a future refactor moves the gRPC suite behind a build tag, add
# `-tags=grpc` to the invocation below and update the CI job.

set -euo pipefail

go test -count=1 \
  -run 'TestAuthInterceptor_.*MTLS|TestAuthInterceptor_AuthenticatorError|TestAuthInterceptor_StaticBearer_RequiresAuthorizationMetadata' \
  ./internal/transport/grpc

echo "smoke-grpc-auth: OK"
