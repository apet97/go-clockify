# Local Artifact Review

Status: **accepted as non-candidate workstation artifacts**.

This document records ignored local/full-tree findings from the
public-content audit. It is not a request to keep these files forever
and it is not a substitute for a clean candidate-tag scan. It only
documents why the current full working-tree findings do not change the
candidate branch content result.

Rules:

- Do not print or commit any local secret values from these files.
- `make public-content-audit` closes the local artifact bucket only
  when every local finding path is listed here and `git check-ignore`
  classifies it as ignored rather than tracked.
- Candidate branch file content must still be `0 open, 0 unknown`.
- Destructive cleanup, including `make clean-deep CONFIRM=1`, remains
  maintainer-approved only.

Current accepted local findings:

| Path | State | Disposition |
| --- | --- | --- |
| `.local/http-test.env` | ignored | Local HTTP smoke/development env file; not candidate branch content. Do not print values. |
| `.serena/cache/go/document_symbols.pkl` | ignored | Local IDE/assistant cache; not candidate branch content. |
| `go-clockify/.env.example` | ignored via nested duplicate checkout | Nested duplicate checkout artifact; not candidate branch content. |
| `go-clockify/.github/workflows/docker-image.yml` | ignored via nested duplicate checkout | Nested duplicate checkout artifact; not candidate branch content. |
| `go-clockify/internal/authn/authn_test.go` | ignored via nested duplicate checkout | Nested duplicate checkout artifact; not candidate branch content. |

Closure rule:

This review closes only the local ignored-artifact bucket for the
current workstation snapshot. If any new full-tree finding appears, or
any listed path becomes tracked/unignored, `make public-content-audit`
must fail open until the finding is cleaned or explicitly reviewed.
