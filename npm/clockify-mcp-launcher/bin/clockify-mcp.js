#!/usr/bin/env node
"use strict";

const path = require("node:path");
const { resolveBinaryPath } = require("../dist/resolve-binary.js");
const { envWarning } = require("../dist/env.js");
const { launch } = require("../dist/launcher.js");

const packageRoot = path.join(__dirname, "..");

const warning = envWarning(process.env);
if (warning) process.stderr.write(warning + "\n");

let binaryPath;
try {
  binaryPath = resolveBinaryPath({
    platform: { os: process.platform, arch: process.arch },
    packageRoot,
    envBinary: process.env.CLOCKIFY_MCP_BINARY,
  });
} catch (err) {
  process.stderr.write("[clockify-mcp] " + err.message + "\n");
  process.exit(1);
}

launch(binaryPath, process.argv.slice(2));
