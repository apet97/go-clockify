import { existsSync } from "node:fs";
import { join } from "node:path";

export interface PlatformKey {
  os: NodeJS.Platform;
  arch: string;
}

// Maps Node's process.platform/arch to the Go release asset basename.
// Mirrors the asset names produced by .github/workflows/release.yml.
export function binaryFilename(p: PlatformKey): string {
  const table: Record<string, string> = {
    "darwin/arm64": "clockify-mcp_darwin_arm64",
    "darwin/x64": "clockify-mcp_darwin_amd64",
    "linux/x64": "clockify-mcp_linux_amd64",
    "linux/arm64": "clockify-mcp_linux_arm64",
    "win32/x64": "clockify-mcp_windows_amd64.exe",
  };
  const key = `${p.os}/${p.arch}`;
  const name = table[key];
  if (!name) {
    throw new Error(
      `unsupported platform: ${key}. Supported: ${Object.keys(table).join(", ")}`,
    );
  }
  return name;
}

export interface ResolveOptions {
  platform: PlatformKey;
  packageRoot: string;
  envBinary?: string | undefined;
  fileExists?: (p: string) => boolean;
}

export function resolveBinaryPath(opts: ResolveOptions): string {
  const exists = opts.fileExists ?? existsSync;

  if (opts.envBinary && opts.envBinary.length > 0) {
    if (exists(opts.envBinary)) return opts.envBinary;
    throw new Error(
      `CLOCKIFY_MCP_BINARY is set to "${opts.envBinary}" but no file exists there`,
    );
  }

  const filename = binaryFilename(opts.platform);
  const vendored = join(opts.packageRoot, "vendor", filename);
  if (exists(vendored)) return vendored;

  throw new Error(
    [
      `Could not find the clockify-mcp Go binary.`,
      `Looked for: ${vendored}`,
      ``,
      `Fix one of the following:`,
      `  - Set CLOCKIFY_MCP_BINARY to an absolute path of a built clockify-mcp binary.`,
      `  - Build it with: go build -o <path> ./cmd/clockify-mcp`,
      `  - Install a release package that includes vendor/ binaries.`,
    ].join("\n"),
  );
}
