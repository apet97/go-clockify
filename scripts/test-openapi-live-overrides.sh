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
