import { test } from "node:test";
import assert from "node:assert/strict";
import { envWarning, missingEnv } from "../src/env.ts";

test("missingEnv returns empty when required vars are present", () => {
  assert.deepEqual(
    missingEnv({
      CLOCKIFY_API_KEY: "key",
      CLOCKIFY_WORKSPACE_ID: "workspace",
    }),
    [],
  );
});

test("missingEnv returns both names when both are absent", () => {
  assert.deepEqual(missingEnv({}), ["CLOCKIFY_API_KEY", "CLOCKIFY_WORKSPACE_ID"]);
});

test("missingEnv treats whitespace-only values as missing", () => {
  assert.deepEqual(
    missingEnv({
      CLOCKIFY_API_KEY: "   ",
      CLOCKIFY_WORKSPACE_ID: "\t",
    }),
    ["CLOCKIFY_API_KEY", "CLOCKIFY_WORKSPACE_ID"],
  );
});

test("envWarning returns null when nothing is missing", () => {
  assert.equal(
    envWarning({
      CLOCKIFY_API_KEY: "key",
      CLOCKIFY_WORKSPACE_ID: "workspace",
    }),
    null,
  );
});

test("envWarning mentions missing var names", () => {
  const warning = envWarning({ CLOCKIFY_API_KEY: "key" });
  assert.match(warning ?? "", /CLOCKIFY_WORKSPACE_ID/);
});
