import { spawn } from "node:child_process";

export interface SpawnPlan {
  command: string;
  args: string[];
}

export function buildSpawnPlan(binaryPath: string, argv: string[]): SpawnPlan {
  return { command: binaryPath, args: argv };
}

const SIGNAL_NUMBERS: Record<string, number> = {
  SIGHUP: 1,
  SIGINT: 2,
  SIGQUIT: 3,
  SIGKILL: 9,
  SIGTERM: 15,
};

export function exitCodeFor(
  code: number | null,
  signal: NodeJS.Signals | null,
): number {
  if (code !== null) return code;
  if (signal && SIGNAL_NUMBERS[signal] !== undefined) {
    return 128 + SIGNAL_NUMBERS[signal];
  }
  return 1;
}

export function launch(binaryPath: string, argv: string[]): void {
  const plan = buildSpawnPlan(binaryPath, argv);
  const child = spawn(plan.command, plan.args, {
    stdio: ["inherit", "inherit", "inherit"],
    env: process.env,
  });

  child.on("error", (err) => {
    process.stderr.write(`[clockify-mcp] failed to start Go binary: ${err.message}\n`);
    process.exit(1);
  });

  child.on("exit", (code, signal) => {
    process.exit(exitCodeFor(code, signal));
  });

  for (const sig of ["SIGINT", "SIGTERM"] as NodeJS.Signals[]) {
    process.on(sig, () => {
      if (!child.killed) child.kill(sig);
    });
  }
}
