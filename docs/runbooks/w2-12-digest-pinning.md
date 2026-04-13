# W2-12 — Image digest pinning at deploy time

## Why this runbook exists

The production Kustomize overlay intentionally does **not** pin an image
tag. Pinning in-tree drifts out of sync with every release: prior to
wave-a A5, `deploy/k8s/overlays/prod/kustomization.yaml` was stuck at
`newTag: "0.7.0"` while the base pointed at `v1.0.0`. Applying the prod
overlay as-committed would have silently downgraded production.

The fix is structural: overlays own environment differences (replicas,
strict-host-check, policy mode, labels), release automation owns image
identity. `scripts/check-overlay-structure.sh` fails CI if any overlay
re-introduces an `images:` block.

This runbook documents the deploy-time workflow that replaces the
removed in-tree pin.

## The workflow

At the moment of deploy, take one of two paths:

### Path A — digest-pin via `kustomize edit set image`

```sh
# 1. Resolve the digest for the tag you want to ship.
docker buildx imagetools inspect ghcr.io/apet97/go-clockify:v1.0.0 \
  --format '{{json .Manifest.Digest}}'
# -> "sha256:abc123…"

# 2. Pin the overlay in a working directory (NOT in the repo).
cp -r deploy/k8s /tmp/clockify-mcp-deploy
cd /tmp/clockify-mcp-deploy/overlays/prod
kustomize edit set image \
  ghcr.io/apet97/go-clockify=ghcr.io/apet97/go-clockify@sha256:abc123…

# 3. Render and apply.
kubectl kustomize . | kubectl apply -f -
```

The `kustomize edit set image` command writes an `images:` block to the
overlay in `/tmp/` — exactly what the structural guard forbids in the
repo. That's the point: the pin lives in the transient build, never in
git.

### Path B — GitOps tooling (Argo CD / Flux)

If Argo CD or Flux renders the overlay, configure the image parameter
override in the ApplicationSet / Kustomization resource rather than in
the tree. Both tools support this via an `images:` field at the tool
config layer, which is evaluated at render time without touching the
checked-in YAML.

Argo CD example:

```yaml
spec:
  source:
    kustomize:
      images:
        - ghcr.io/apet97/go-clockify:v1.0.0
```

Flux example:

```yaml
spec:
  images:
    - name: ghcr.io/apet97/go-clockify
      newTag: v1.0.0
```

Both render correctly against the in-tree overlay because the tool
applies its own `images:` overlay on top.

## What to pin — tag or digest?

- **Digest** is safer but less readable. Use digests for regulated
  environments where the pin must be auditable and tamper-evident.
- **Tag** is readable and simpler. Use tags when the registry guarantees
  immutability (ghcr.io does not by default, but we never overwrite
  tags in our release pipeline — see `.github/workflows/release.yml`).

For this project, prefer digest pins in prod and tag pins in staging.

## What to do if the structural guard fails CI

If `scripts/check-overlay-structure.sh` fails on your PR, it means an
`images:` / `newTag:` / `newName:` / `newDigest:` entry landed in one of
the overlays. Two fixes:

1. **Remove the pin** from the overlay YAML and do the pinning at
   deploy time per Path A or Path B above.
2. **Pin in `deploy/k8s/base/`** if the tag is genuinely a project-wide
   default. The base is allowed to pin; overlays are not. This is rare —
   the base tag moves with every release and the release workflow has
   the access to update it.

The guard is a structural check, not a value check — it doesn't care
*which* tag is pinned, only that overlays don't do the pinning. That
catches drift at the source instead of playing whack-a-mole with
individual stale tags.

## Related

- `deploy/k8s/overlays/prod/kustomization.yaml` (header comment restates
  the policy)
- `scripts/check-overlay-structure.sh` (the CI-enforced guard)
- `.github/workflows/ci.yml` (deploy-render job calls
  `scripts/check-k8s-render.sh`, which calls the guard)
