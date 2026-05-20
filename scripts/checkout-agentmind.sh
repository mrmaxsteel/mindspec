#!/usr/bin/env bash
# checkout-agentmind.sh — Spec 083 Bead 2 sibling-checkout helper.
#
# Ensures a usable agentmind sibling repository exists at the path the
# mindspec `replace github.com/mrmaxsteel/agentmind => ../agentmind`
# directive expects (relative to mindspec's repo root). Idempotent: if
# the sibling is already present, exits 0 without modification (after
# optionally checking out the requested tag).
#
# Lookup order (mirrors the Bead 1 / Test G deferral pattern):
#
#   1. If `../agentmind` already exists as a directory with a `go.mod`
#      naming `github.com/mrmaxsteel/agentmind`, treat it as the sibling
#      and (when a tag was requested) try to check it out.
#   2. Otherwise, clone the upstream repo to `../agentmind` and check
#      out the requested tag.
#   3. If upstream is unreachable AND the sibling is absent, emit the
#      Bead-1-style deferral message and exit non-zero so the caller
#      can decide whether to proceed.
#
# Spec reference:
#   .mindspec/docs/specs/083-agentmind-extraction-v2/spec.md
#   - Bead 2 step 4 ("clones agentmind at the tag pinned in go.mod to
#     a sibling directory so CI can resolve the replace directive").
#
# Exit codes:
#   0  — sibling repo is in place at $SIBLING_PATH (cloned or
#        already-present). On success the path is printed to stdout.
#   2  — upstream reachable but the requested tag is absent (expected
#        today: v0.1.0 not yet published; deferral mode).
#   3  — upstream unreachable AND no usable sibling already present.
#   4  — invocation error (bad args, missing git).
#
# Usage:
#   scripts/checkout-agentmind.sh                 # default tag: v0.1.0
#   scripts/checkout-agentmind.sh v0.3.0          # different tag
#   AGENTMIND_REPO_URL=... scripts/checkout-agentmind.sh
#   SIBLING_PATH=/abs/path scripts/checkout-agentmind.sh

set -euo pipefail

export GIT_TERMINAL_PROMPT=0

REPO_URL="${AGENTMIND_REPO_URL:-https://github.com/mrmaxsteel/agentmind}"

usage() {
    cat <<'EOF'
checkout-agentmind.sh — Spec 083 Bead 2 sibling-checkout helper.

Ensures a usable agentmind sibling repository exists at the path the
mindspec `replace ... => ../agentmind` directive points at. Idempotent.

Exit codes:
  0  — sibling repo is in place at $SIBLING_PATH (cloned or already-present).
  2  — upstream reachable but the requested tag is absent (deferral mode).
  3  — upstream unreachable AND no usable sibling already present.
  4  — invocation error (bad arguments / missing git).

Usage:
  scripts/checkout-agentmind.sh                 # default tag: v0.1.0
  scripts/checkout-agentmind.sh v0.3.0          # different tag

Environment:
  AGENTMIND_REPO_URL  Override the upstream URL (default:
                      https://github.com/mrmaxsteel/agentmind).
  SIBLING_PATH        Override the sibling-repo path (default:
                      ../agentmind, relative to mindspec repo root).
EOF
}

for arg in "$@"; do
    case "$arg" in
        --help|-h)
            usage
            exit 0
            ;;
    esac
done

if [ "$#" -gt 0 ]; then
    case "${1:-}" in
        --*)
            echo "checkout-agentmind.sh: first argument looks like a flag ('$1')" >&2
            usage >&2
            exit 4
            ;;
    esac
fi

TAG="${1:-v0.1.0}"

if ! command -v git >/dev/null 2>&1; then
    echo "checkout-agentmind.sh: git not found on PATH" >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SIBLING_PATH="${SIBLING_PATH:-$REPO_ROOT/../agentmind}"

# Normalize to an absolute path; works whether or not the parent exists.
SIBLING_PARENT="$(cd "$(dirname "$SIBLING_PATH")" && pwd)"
SIBLING_BASE="$(basename "$SIBLING_PATH")"
SIBLING_ABS="$SIBLING_PARENT/$SIBLING_BASE"

is_agentmind_repo() {
    local dir="$1"
    [ -d "$dir" ] && [ -f "$dir/go.mod" ] && \
        grep -q '^module github\.com/mrmaxsteel/agentmind$' "$dir/go.mod"
}

if is_agentmind_repo "$SIBLING_ABS"; then
    # Already present. If a tag was requested AND the directory is a git
    # checkout, try to check it out; otherwise leave as-is (local
    # development against an unversioned working tree is the normal case
    # during Phases 2-5).
    if [ -d "$SIBLING_ABS/.git" ]; then
        if git -C "$SIBLING_ABS" rev-parse --verify "$TAG" >/dev/null 2>&1; then
            git -C "$SIBLING_ABS" checkout --quiet "$TAG"
        fi
    fi
    echo "$SIBLING_ABS"
    exit 0
fi

# Probe upstream reachability. Same accounting as verify-agentmind-tag.sh.
ERR_FILE="$(mktemp -t checkout-agentmind.err.XXXXXX)"
trap 'rm -f "$ERR_FILE"' EXIT

LS_REMOTE_OUT="$(git ls-remote --tags "$REPO_URL" 2>"$ERR_FILE")" && LS_REMOTE_RC=0 || LS_REMOTE_RC=$?

if [ "$LS_REMOTE_RC" -ne 0 ]; then
    echo "checkout-agentmind.sh: upstream unreachable: $REPO_URL" >&2
    echo "  (git ls-remote exited $LS_REMOTE_RC)" >&2
    if [ -s "$ERR_FILE" ]; then
        sed 's/^/  /' "$ERR_FILE" >&2
    fi
    echo "" >&2
    echo "  Expected during the parallel mindspec-side migration:" >&2
    echo "  the github.com/mrmaxsteel/agentmind repository has not yet been" >&2
    echo "  published. For local development, create a sibling working tree" >&2
    echo "  at $SIBLING_ABS with a valid go.mod naming" >&2
    echo "  github.com/mrmaxsteel/agentmind, and this script will detect it" >&2
    echo "  on the next invocation." >&2
    exit 3
fi

# Match exactly "refs/tags/<TAG>" (no peeled-tag suffix "^{}").
SHA="$(printf '%s\n' "$LS_REMOTE_OUT" | awk -v tag="refs/tags/$TAG" '$2 == tag { print $1; exit }')"

if [ -z "$SHA" ]; then
    echo "checkout-agentmind.sh: tag $TAG NOT found at $REPO_URL" >&2
    echo "" >&2
    echo "  Expected during the parallel mindspec-side migration:" >&2
    echo "  agentmind $TAG has not yet been published upstream." >&2
    echo "  Once upstream publishes the tag, re-run this script to clone." >&2
    exit 2
fi

echo "checkout-agentmind.sh: cloning $REPO_URL@$TAG -> $SIBLING_ABS" >&2
git clone --depth 1 --branch "$TAG" "$REPO_URL" "$SIBLING_ABS"

echo "$SIBLING_ABS"
exit 0
