# ADR 008 — Runtime JSON-schema validation at the enforcement boundary

**Status**: Accepted, 2026-04-11.

## Context

Phase H (`W1-10`, commit `869dd81`) rewrote every Tier 1 and Tier 2 tool
input schema to advertise a strict shape: `additionalProperties: false`
on every object, `minimum`/`maximum` bounds on pagination integers,
`format: "date-time"` on RFC3339 string fields, and `^#[0-9a-fA-F]{6}$`
on colour fields. The walker that applies these constraints lives in
`internal/tools/common.go` as `tightenInputSchema` and runs inside
`normalizeDescriptors` at tool registration time. Two property tests
(`TestRegistrySchemasAllHaveAdditionalPropertiesFalse` and its Tier 2
sibling) assert the walker processed every tool.

The gap that survived Wave 1: nothing on the wire actually validates
against those schemas. A client that sends `{"bogus":"x","start":"..."}`
to `clockify_log_time` reaches the handler with the extra field intact.
Clients that send a string where a boolean is declared reach the
handler with the wrong type. The schema is advertised in `tools/list`
but is not enforced at dispatch. That contradicts the "schema as
contract" promise made by ADR 003 (enforcement pipeline) and ADR 007
(streamable HTTP rewrite) — both of which assume the protocol surface
is the authoritative contract between server and client.

Two alternatives were considered before landing on the current design:

1. **Validate inside each handler.** Rejected because the 33 Tier 1 +
   91 Tier 2 handlers would each need the same boilerplate, with no
   central guarantee that tightenings propagate. Drift is inevitable.
2. **Vendor a third-party validator** (e.g.
   `github.com/santhosh-tekuri/jsonschema`). Rejected because
   `go build` must link zero third-party runtime deps (ADR 001).
   Adding one just for the keyword subset our tools actually use
   would be a regression of the stdlib-only promise.

## Decision

Introduce a tiny stdlib-only JSON-schema validator at
`internal/jsonschema/validator.go` and wire it into
`enforcement.Pipeline.BeforeCall` as the first gate — before policy,
before rate-limiting, before dry-run. Failures return
`*mcp.InvalidParamsError` (a new typed error in `internal/mcp/types.go`)
which the `tools/call` dispatch case in `server.go` detects via
`errors.As` and renders as a JSON-RPC `-32602 invalid params`
response. The RFC 6901 JSON Pointer to the offending field is placed
in `error.data.pointer` so clients can locate the failing field
without string parsing.

Supported keyword subset (the strict minimum that the existing tool
fleet uses):

- `type`: `object`, `string`, `integer`, `number`, `boolean`, `array`
- `required` (array of strings on objects)
- `additionalProperties: false` (explicit `true` is a no-op)
- `properties` (recursive)
- `items` (array element schema)
- `minimum` / `maximum` (integer and number)
- `minLength` / `maxLength` (string)
- `pattern` (regexp, anchored via `^...$` if not already)
- `format: "date"` / `format: "date-time"` (lenient `time.Parse`)
- `enum` (array of any; exact JSON-equal match, with numeric coercion)

Explicitly out of scope: `$ref`, `$defs`, `allOf`/`anyOf`/`oneOf`,
`not`, `if`/`then`/`else`, `dependentSchemas`, `const`,
`exclusiveMinimum`/`exclusiveMaximum`, `multipleOf`, `propertyNames`,
`patternProperties`. None of those keywords appear in Tier 1 or
Tier 2 and each would add validator complexity with no caller.

The protocol-core guard lives in `internal/mcp/types.go` (not
`internal/enforcement/`) so `internal/mcp/` keeps its zero-domain-
imports invariant. Enforcement imports mcp; mcp never imports
enforcement. `Enforcement.BeforeCall` grows a new `schema map[string]any`
parameter (between `hints` and `lookupHandler`); `internal/mcp/server.go`
passes the already-looked-up `d.Tool.InputSchema` through.

Validation happens **before** rate-limiting so malformed calls never
consume a window slot — a noisy client cannot exhaust the budget with
garbage input. It happens before dry-run so a destructive intercept
never fires on invalid args.

## Consequences

- **BREAKING:** every incoming `tools/call` is now validated against
  the advertised schema. Clients that previously relied on silent
  extra-key acceptance, loose-type coercion, or RFC3339-format
  laxness will see `-32602` responses. Migration path:
  inspect the advertised `inputSchema` returned by `tools/list`
  and align payloads field-by-field. The pointer in
  `error.data.pointer` identifies the first offender.
- **Metric**: `clockify_mcp_tool_calls_total` gains a new `outcome`
  label value `invalid_params`, distinct from `tool_error`,
  `rate_limited`, `policy_denied`, `timeout`, `dry_run`, and
  `cancelled`. Dashboards that `sum by (outcome)` automatically pick
  up the new dimension.
- **Coverage**: new per-package floor `internal/jsonschema >= 85%`
  in `.github/workflows/ci.yml`. The validator landed at 86.4%.
- **Stdlib-only default-build promise holds.** The validator imports
  `fmt`, `reflect`, `regexp`, `sync`, `time` — all stdlib. No OTel
  symbols introduced (verified by the existing nm gate).
- **Drift surface.** Future schema tightenings in the walker
  (`internal/tools/common.go`) must be paired with a corresponding
  validator update. The new property test
  `TestRegistrySchemasAcceptHappyPathArgs` walks every Tier 1 + Tier 2
  descriptor and feeds a synthesized happy-path argument map through
  the validator; if the walker and validator disagree the test fails
  with the tool name + JSON Pointer.
- **Interface change**: `mcp.Enforcement.BeforeCall` grew a new
  parameter. The only production implementation (`enforcement.Pipeline`)
  was updated, and every test-only stub (`integration_test.go`'s
  `testEnforcement`, plus three schema-specific stubs in the new
  `internal/mcp/schema_validation_dispatch_test.go`) received the
  same signature. External consumers that embedded the interface would
  need a matching signature update — no such consumers exist today
  (`internal/enforcement` and the test files are the only users).

## Status

Landed on `main` in the W2-01 commit of the 2026-04-11 session.
Wave 2 first item closed; subsequent Wave 2 items move in priority
order per `docs/wave2-backlog.md`.
