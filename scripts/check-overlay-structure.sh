#!/usr/bin/env bash
# Guards against the drift that prompted wave-a A5: a production overlay
# that pinned an older image tag than the base (`newTag: "0.7.0"` while
# base was `v1.0.0`) and would have silently downgraded prod on apply.
#
# The rule: NO overlay under deploy/k8s/overlays/ may set a kustomize
# `images:` override (newTag / newName / newDigest). Operators pin the
# image digest at deploy time via `kustomize edit set image`, per
# docs/runbooks/w2-12-digest-pinning.md. That pinning never lives in the
# tree; it's built into the manifests immediately before `kubectl apply`.
#
# A structural check is safer than a value check (e.g. "overlay tag >=
# base tag") because it doesn't rely on semver parsing in bash and catches
# the class of bug — overlay-owned pinning drift — at its source.
#
# Called from CI (deploy-render job, via check-k8s-render.sh) and from
# `make verify-k8s`.

set -euo pipefail

OVERLAYS_DIR="deploy/k8s/overlays"

if [ ! -d "$OVERLAYS_DIR" ]; then
    echo "ERROR: $OVERLAYS_DIR not found (run from repo root)" >&2
    exit 1
fi

violations=0
for overlay in "$OVERLAYS_DIR"/*/kustomization.yaml; do
    [ -e "$overlay" ] || continue
    name=$(basename "$(dirname "$overlay")")

    # Match top-level `images:` block or any newTag/newName/newDigest
    # value under it. Comment lines (starting with # after optional
    # whitespace) are ignored so documentation about the policy doesn't
    # trip the guard.
    stripped=$(grep -v '^\s*#' "$overlay")

    if printf '%s\n' "$stripped" | grep -qE '^\s*images:'; then
        echo "FAIL: $name overlay has an 'images:' block at $overlay" >&2
        echo "      Per policy, overlays must not pin image tags/digests." >&2
        echo "      Pin at deploy time via 'kustomize edit set image' instead." >&2
        violations=$((violations + 1))
        continue
    fi
    if printf '%s\n' "$stripped" | grep -qE '^\s*newTag:|^\s*newName:|^\s*newDigest:'; then
        echo "FAIL: $name overlay sets newTag/newName/newDigest in $overlay" >&2
        echo "      Per policy, overlays must not pin image tags/digests." >&2
        violations=$((violations + 1))
    fi
done

if [ "$violations" -gt 0 ]; then
    printf '\n%d overlay(s) violate the image-pinning policy\n' "$violations" >&2
    echo "See docs/runbooks/w2-12-digest-pinning.md for the deploy-time workflow." >&2
    exit 1
fi

echo "OK: no overlay under $OVERLAYS_DIR pins image tags/digests"
