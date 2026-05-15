#!/usr/bin/env bash
set -euo pipefail

ruby <<'RUBY'
require "fileutils"
require "json"
require "tmpdir"
require "yaml"

def assert(condition, message)
  abort("[test-openapi-live-overrides] #{message}") unless condition
end

doc = YAML.load_file("docs/openapi/clockify-openapi.yaml")
paths = doc.fetch("paths")
schemas = doc.fetch("components").fetch("schemas")
generation = doc.fetch("x-clockify-generation")

assert(!paths.key?("/workspaces/{workspaceId}/scheduling/capacity"), "phantom scheduling capacity path must stay quarantined")
assert(generation.dig("sourceManifest", "path") == "manifest.json", "OpenAPI artifact must record the repo-local source manifest")
assert(generation.dig("sourceManifest", "files").to_i > 0, "OpenAPI source manifest must pin input files")
assert(File.exist?("docs/openapi/sources/manifest.json"), "repo-local OpenAPI source manifest must exist")

stable_ids = {
  ["/file/image", "post"] => "uploadImage",
  ["/workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals", "get"] => "getUserCapacityTotal",
  ["/workspaces/{workspaceId}/time-off/balance/policy/{policyId}", "patch"] => "updateBalance",
  ["/workspaces/{workspaceId}/users/{userId}/roles", "post"] => "giveUserManagerRole",
  ["/workspaces/{workspaceId}/users/{userId}/roles", "delete"] => "removeUserManagerRole"
}
stable_ids.each do |(path, method), operation_id|
  assert(paths.dig(path, method, "operationId") == operation_id, "#{method.upcase} #{path} operationId must stay stable")
end

summary = schemas.fetch("SummaryReportResponse").fetch("properties")
assert(!summary.key?("chart"), "SummaryReportResponse must not expose stale chart")
assert(summary.dig("donutChart", "type") == "array", "SummaryReportResponse.donutChart must be an array")
assert(summary.dig("groupTotals", "type") == "object", "SummaryReportResponse.groupTotals must be an object")

working_days = schemas.fetch("MemberProfileUpdateRequest").fetch("properties").fetch("workingDays")
assert(working_days["type"] == "array", "MemberProfileUpdateRequest.workingDays must be an array")
assert(working_days.dig("items", "enum")&.include?("MONDAY"), "workingDays must enumerate day strings")

user_roles = schemas.fetch("UserDtoV1").fetch("properties").fetch("roles")
assert(user_roles["type"] == "array", "UserDtoV1.roles must be an open array")

webhook_props = schemas.fetch("WebhookDtoV1").fetch("properties")
%w[entityType feature payloadType validSourceTypes].each do |field|
  assert(!webhook_props.key?(field), "WebhookDtoV1 must not expose live-absent #{field}")
end

invoice_status = schemas.fetch("InvoiceStatus")
assert(invoice_status["enum"] == %w[UNSENT SENT PAID PARTIALLY_PAID VOID OVERDUE], "InvoiceStatus enum must match live Clockify")
assert(schemas.fetch("InvoiceStatusRequest").dig("properties", "invoiceStatus", "$ref") == "#/components/schemas/InvoiceStatus", "InvoiceStatusRequest must use invoiceStatus body key")

export_params = paths.fetch("/workspaces/{workspaceId}/invoices/{invoiceId}/export").fetch("get").fetch("parameters")
user_locale = export_params.find { |param| param["in"] == "query" && param["name"] == "userLocale" }
assert(user_locale && user_locale["required"] == true, "invoice export must require userLocale")
assert(user_locale.dig("schema", "default") == "en-US", "invoice export userLocale default must be en-US")

workspace_required = schemas.fetch("CreateWorkspaceRequest").fetch("required")
assert(workspace_required.include?("organizationId"), "CreateWorkspaceRequest must require organizationId")

tasks_params = paths.fetch("/workspaces/{workspaceId}/projects/{projectId}/tasks").fetch("get").fetch("parameters")
task_query_names = tasks_params.select { |param| param["in"] == "query" }.map { |param| param["name"] }
assert(task_query_names.include?("is-active"), "task list must use is-active query parameter")
assert(!task_query_names.include?("task-status"), "task list must not use dead task-status query parameter")

holiday_params = paths.fetch("/workspaces/{workspaceId}/holidays/in-period").fetch("get").fetch("parameters")
assigned_to = holiday_params.find { |param| param["in"] == "query" && param["name"] == "assigned-to" }
assert(assigned_to && assigned_to["required"] == true, "holidays in-period assigned-to must be required")

expense_status = schemas.fetch("ExpenseCategoryDtoV1").fetch("properties").fetch("status")
assert(expense_status["nullable"] == true, "ExpenseCategoryDtoV1.status must allow null")

makefile = File.read("Makefile")
assert(makefile.include?("gen-coverage-matrix:"), "Makefile must expose gen-coverage-matrix")
assert(makefile.include?("coverage-matrix-drift:"), "Makefile must expose coverage-matrix-drift")

matrix = JSON.parse(File.read("docs/openapi/coverage-matrix.json"))
# no_mcp_tool_count is the number of OpenAPI operations with no typed MCP
# tool. It is intentionally non-zero: the typed surface is 152 curated
# tools, and the remaining documented operations are reachable through
# the clockify_api_get / clockify_api_request raw fallback rather than a
# bespoke tool. The matrix records the count; coverage-matrix-drift keeps
# it honest. Assert only that the field is present and well-formed.
assert(matrix.dig("summary", "no_mcp_tool_count").is_a?(Integer), "coverage matrix must record an integer no_mcp_tool_count")
assert(matrix.dig("summary", "no_evidence_count") == 0, "coverage matrix must have zero no-evidence operations")
assert(matrix["tool_catalog"] == {"domain" => 133, "raw" => 2, "total" => 152, "workflow" => 17}, "coverage matrix must use workflow/domain/raw tool catalog counts")

coverage_contract = IO.popen(
  ["python3", "-c", %q{
import runpy

mod = runpy.run_path("scripts/gen-coverage-matrix", run_name="coverage_matrix_contract_test")

try:
    mod["catalog_tools"]({"tier1": [], "tier2": []})
except ValueError as exc:
    assert "catalog must not use legacy tier1/tier2 top-level shape" in str(exc), exc
else:
    raise AssertionError("legacy tier1/tier2 catalog shape must fail validation")

try:
    mod["catalog_tools"]({"generator": "fixture"})
except ValueError as exc:
    assert "catalog must use top-level tools array" in str(exc), exc
else:
    raise AssertionError("catalog without tools[] must fail validation")

tools = mod["catalog_tools"]({"tools": [{"name": "clockify_status"}]})
assert tools == [{"name": "clockify_status"}], tools
}],
  err: [:child, :out],
  &:read
)
assert($?.success?, "coverage matrix generator must reject legacy catalog shapes: #{coverage_contract}")

Dir.mktmpdir("openapi-catalog-contract") do |repo|
  FileUtils.mkdir_p(File.join(repo, "docs"))
  File.write(File.join(repo, "docs/tool-catalog.json"), JSON.pretty_generate({"tier1" => [], "tier2" => []}))

  output = IO.popen(
    ["scripts/gen-clockify-openapi", "--validate-only", "--repo-root", repo, "--out", "docs/openapi/clockify-openapi.yaml"],
    err: [:child, :out],
    &:read
  )
  assert(!$?.success?, "legacy tier1/tier2 catalog shape must fail validation")
  assert(output.include?("catalog must not use legacy tier1/tier2 top-level shape"), "legacy catalog failure must name tier1/tier2 shape")
end
RUBY

echo "[test-openapi-live-overrides] OK"
