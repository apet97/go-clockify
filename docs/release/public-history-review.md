# Public History Review

Status: **technical review complete for current keyword matches**.

This document records the public-content history sanity check from the
May 8 launch-readiness review. It is not permission to rewrite history
or flip repository visibility. It only records why the current
`secret|token|password|key=` commit-message matches are not leaked
credentials.

Verification command:

```sh
git log --all --format='%h%x09%ad%x09%s' --date=short -200 | grep -Ei 'secret|token|password|key='
```

Current accepted matches:

| Commit | Date | Match reason | Review disposition |
| --- | --- | --- | --- |
| `5eeaa46` | 2026-04-29 | Subject/body mention confirmation tokens as a proposed MCP safety design. | Accepted false positive; no secret token value is present. |
| `7da957c` | 2026-04-28 | Subject/body mention per-token rate limits for authenticated principals. | Accepted false positive; "token" refers to rate-limit subject scoping, not credential material. |
| `20368d4` | 2026-04-28 | Subject/body mention per-token rate-limit environment variables. | Accepted false positive; "token" refers to rate-limit subject scoping, not credential material. |

Closure rule:

- `make public-content-audit` closes the public-history bucket only
  when every matching recent commit SHA is listed here.
- Any new matching commit must be reviewed and added with a concise
  disposition, or the audit remains open.
- History rewrite remains a maintainer-only decision.
