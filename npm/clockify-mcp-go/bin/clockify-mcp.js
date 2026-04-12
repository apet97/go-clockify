#!/usr/bin/env node
// Dispatcher shim for @apet97/clockify-mcp-go. Resolves the platform-
// specific sibling package (@apet97/clockify-mcp-go-{platform}) via
// node's require.resolve and exec's its bundled Go binary. The five
// platform packages are listed under optionalDependencies in
// package.json so npm only installs the one matching the user's os/cpu.
"use strict";

const { spawnSync } = require("child_process");

const PLATFORM_MAP = {
  "darwin-arm64": "@apet97/clockify-mcp-go-darwin-arm64",
  "darwin-x64": "@apet97/clockify-mcp-go-darwin-x64",
  "linux-x64": "@apet97/clockify-mcp-go-linux-x64",
  "linux-arm64": "@apet97/clockify-mcp-go-linux-arm64",
  "win32-x64": "@apet97/clockify-mcp-go-windows-x64",
};

const key = `${process.platform}-${process.arch}`;
const pkgName = PLATFORM_MAP[key];
if (!pkgName) {
  console.error(
    `clockify-mcp: no prebuilt binary available for ${key}. ` +
      `Install a tarball from the GitHub release or run ` +
      `'go install github.com/apet97/go-clockify/cmd/clockify-mcp@latest'.`
  );
  process.exit(1);
}

const binName = process.platform === "win32" ? "clockify-mcp.exe" : "clockify-mcp";
let binPath;
try {
  binPath = require.resolve(`${pkgName}/bin/${binName}`);
} catch (err) {
  console.error(
    `clockify-mcp: could not resolve '${pkgName}/bin/${binName}'. ` +
      `The optional dependency was not installed for this platform. ` +
      `Re-run 'npm install' or check 'npm ls ${pkgName}'.`
  );
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`clockify-mcp: failed to exec ${binPath}: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
