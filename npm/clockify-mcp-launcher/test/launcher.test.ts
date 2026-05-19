import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSpawnPlan, exitCodeFor } from "../src/launcher.ts";

test("buildSpawnPlan forwards command and argv", () => {
  const args = ["doctor", "--live"];
  assert.deepEqual(buildSpawnPlan("/bin/clockify-mcp", args), {
    command: "/bin/clockify-mcp",
    args,
  });
});

test("buildSpawnPlan forwards empty argv", () => {
  assert.deepEqual(buildSpawnPlan("/bin/clockify-mcp", []), {
    command: "/bin/clockify-mcp",
    args: [],
  });
});

test("exitCodeFor returns numeric exit code", () => {
  assert.equal(exitCodeFor(0, null), 0);
  assert.equal(exitCodeFor(3, null), 3);
});

test("exitCodeFor maps SIGTERM to conventional shell code", () => {
  assert.equal(exitCodeFor(null, "SIGTERM"), 143);
});

test("exitCodeFor falls back to 1 when code and signal are absent", () => {
  assert.equal(exitCodeFor(null, null), 1);
});
