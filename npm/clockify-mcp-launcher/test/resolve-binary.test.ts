import { test } from "node:test";
import assert from "node:assert/strict";
import { join } from "node:path";
import { binaryFilename, resolveBinaryPath } from "../src/resolve-binary.ts";

test("binaryFilename returns release asset names for supported platforms", () => {
  assert.equal(binaryFilename({ os: "darwin", arch: "arm64" }), "clockify-mcp_darwin_arm64");
  assert.equal(binaryFilename({ os: "darwin", arch: "x64" }), "clockify-mcp_darwin_amd64");
  assert.equal(binaryFilename({ os: "linux", arch: "x64" }), "clockify-mcp_linux_amd64");
  assert.equal(binaryFilename({ os: "linux", arch: "arm64" }), "clockify-mcp_linux_arm64");
  assert.equal(binaryFilename({ os: "win32", arch: "x64" }), "clockify-mcp_windows_amd64.exe");
});

test("binaryFilename throws for unsupported platforms", () => {
  assert.throws(
    () => binaryFilename({ os: "linux", arch: "ia32" }),
    /unsupported platform: linux\/ia32/,
  );
});

test("resolveBinaryPath returns envBinary when it exists", () => {
  assert.equal(
    resolveBinaryPath({
      platform: { os: "darwin", arch: "arm64" },
      packageRoot: "/pkg",
      envBinary: "/custom/clockify-mcp",
      fileExists: (p) => p === "/custom/clockify-mcp",
    }),
    "/custom/clockify-mcp",
  );
});

test("resolveBinaryPath throws when envBinary is missing", () => {
  assert.throws(
    () =>
      resolveBinaryPath({
        platform: { os: "darwin", arch: "arm64" },
        packageRoot: "/pkg",
        envBinary: "/missing/clockify-mcp",
        fileExists: () => false,
      }),
    /CLOCKIFY_MCP_BINARY is set to "\/missing\/clockify-mcp" but no file exists there/,
  );
});

test("resolveBinaryPath returns vendored binary when no env override is set", () => {
  const expected = join("/pkg", "vendor", "clockify-mcp_linux_amd64");

  assert.equal(
    resolveBinaryPath({
      platform: { os: "linux", arch: "x64" },
      packageRoot: "/pkg",
      fileExists: (p) => p === expected,
    }),
    expected,
  );
});

test("resolveBinaryPath throws when no binary exists", () => {
  assert.throws(
    () =>
      resolveBinaryPath({
        platform: { os: "linux", arch: "x64" },
        packageRoot: "/pkg",
        fileExists: () => false,
      }),
    /Could not find the clockify-mcp Go binary\./,
  );
});
