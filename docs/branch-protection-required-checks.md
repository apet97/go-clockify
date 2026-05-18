# Required status checks for `main`

`main` must require all of these CI checks to pass before a PR can merge. The names
below are the GitHub Actions **job names** (the `name:` field), which is what branch
protection matches.

- Format
- Vet
- Lint
- Vulncheck
- Build
- Test
- Stdio smoke
- Tool-catalog drift
- API parity matrix drift
- OpenAPI drift
- Raw allowlist drift
- Self-inspection drift
- Shellcheck
- Actionlint
- Secret scan (gitleaks)

## Applying it (maintainer / repo admin only)

Run this once, from an account with admin rights on `apet97/go-clockify`:

```sh
gh api -X PUT repos/apet97/go-clockify/branches/main/protection \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "Format", "Vet", "Lint", "Vulncheck", "Build", "Test", "Stdio smoke",
      "Tool-catalog drift", "API parity matrix drift", "OpenAPI drift",
      "Raw allowlist drift", "Self-inspection drift",
      "Shellcheck", "Actionlint", "Secret scan (gitleaks)"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null
}
JSON
```

Verify afterwards:

```sh
gh api repos/apet97/go-clockify/branches/main/protection/required_status_checks/contexts
```
