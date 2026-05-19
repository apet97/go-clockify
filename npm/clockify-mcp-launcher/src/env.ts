const REQUIRED = ["CLOCKIFY_API_KEY", "CLOCKIFY_WORKSPACE_ID"] as const;

export function missingEnv(env: NodeJS.ProcessEnv): string[] {
  return REQUIRED.filter((k) => {
    const v = env[k];
    return v === undefined || v.trim() === "";
  });
}

export function envWarning(env: NodeJS.ProcessEnv): string | null {
  const missing = missingEnv(env);
  if (missing.length === 0) return null;
  return (
    `[clockify-mcp] warning: ${missing.join(", ")} not set in environment. ` +
    `The Go server will reject startup if it requires them.`
  );
}
