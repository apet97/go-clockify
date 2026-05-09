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
# The production binary links gRPC only when built with `-tags=grpc`,
# but the transport implementation lives in its own submodule and its
# auth tests intentionally run directly without root build tags. The
# script uses `-run` to scope to the auth/mTLS regression tests; running
# the whole package would still catch them but a focused run keeps this
# job's failure output actionable when the broader test job is also red.

set -euo pipefail

go test -count=1 \
  -run 'TestAuthInterceptor_.*MTLS|TestAuthInterceptor_AuthenticatorError|TestAuthInterceptor_StaticBearer_RequiresAuthorizationMetadata' \
  ./internal/transport/grpc

echo "smoke-grpc-auth: OK"
