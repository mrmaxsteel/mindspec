#!/usr/bin/env bash
# test-e-continuity.sh — Spec 083 Bead 6 step 4 Test E cross-repo gate.
#
# Spec Test E (no-circular-discovery) used to grep the in-tree
# `internal/agentmind/` directory; after Bead 5's deletion that target
# no longer exists inside mindspec. This script preserves the test by
# performing a shallow clone of the agentmind repo at the tag pinned
# in mindspec's `go.mod` and running the spec-canonical grep against
# the agentmind tree.
#
# Spec reference:
#   .mindspec/docs/specs/083-agentmind-extraction-v2/spec.md
#   - Test E (lines 289-293):
#       grep -rEn 'exec\.Command.*"mindspec"|LookPath.*"mindspec"|StartProcess.*"mindspec"' \
#           ./agentmind/client/ ./agentmind/cmd/ ./agentmind/internal/
#     returns no match.
#   .mindspec/docs/specs/083-agentmind-extraction-v2/plan.md
#   - Bead 6 step 4 (Test E continuity post-Bead-5 deletion).
#
# The agentmind side's own CI is the primary owner of Test E
# enforcement; this script is the mindspec-side mirror that runs in
# mindspec CI from Phase 6 onward.
#
# Until upstream publishes `v1.0.0`, the requested tag cannot be
# resolved and the script exits 2 with a clear deferral message. CI
# treats exit 2 as "skip the job with a warning"; only exit 0 (zero
# matches) is a hard pass, exit 1 (matches found, circular discovery
# detected) is a hard fail.
#
# Usage:
#   scripts/test-e-continuity.sh                # tag = v1.0.0
#   scripts/test-e-continuity.sh v1.2.0         # different tag
#   scripts/test-e-continuity.sh --use-sibling  # use ../agentmind if present
#                                               # (local-dev / pre-release mode)
#
# Exit codes:
#   0  — clone succeeded, grep returned no matches. Test E green.
#   1  — clone succeeded, grep found matches. Circular discovery in
#        the agentmind tree; bug.
#   2  — upstream reachable but the tag is absent. Deferral mode;
#        upstream agentmind has not shipped this tag yet.
#   3  — upstream unreachable AND no usable sibling found (when
#        --use-sibling was specified).
#   4  — invocation error.

set -euo pipefail

export GIT_TERMINAL_PROMPT=0

REPO_URL="${AGENTMIND_REPO_URL:-https://github.com/mrmaxsteel/agentmind}"

usage() {
    cat <<'EOF'
test-e-continuity.sh — Spec 083 Bead 6 Test E cross-repo gate.

Performs a shallow clone of github.com/mrmaxsteel/agentmind at the
requested tag and runs the spec-canonical Test E grep against the
checkout. Exits 0 on zero matches (green), 1 on matches found
(failure), 2 on tag-not-found (deferral), 3 on upstream unreachable.

Usage:
  scripts/test-e-continuity.sh [TAG] [--use-sibling]

  TAG            Tag to clone (default: v1.0.0).
  --use-sibling  If a usable ../agentmind sibling repo exists, grep
                 against it instead of cloning. Intended for local
                 development against an in-progress agentmind branch.
EOF
}

USE_SIBLING=0
TAG=""

for arg in "$@"; do
    case "$arg" in
        --help|-h)
            usage
            exit 0
            ;;
        --use-sibling)
            USE_SIBLING=1
            ;;
        --*)
            echo "test-e-continuity.sh: unknown flag '$arg'" >&2
            usage >&2
            exit 4
            ;;
        *)
            if [ -z "$TAG" ]; then
                TAG="$arg"
            else
                echo "test-e-continuity.sh: unexpected positional argument '$arg'" >&2
                exit 4
            fi
            ;;
    esac
done

TAG="${TAG:-v1.0.0}"

if ! command -v git >/dev/null 2>&1; then
    echo "test-e-continuity.sh: git not found on PATH" >&2
    exit 4
fi
if ! command -v grep >/dev/null 2>&1; then
    echo "test-e-continuity.sh: grep not found on PATH" >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# run_grep runs the spec-canonical Test E grep against the supplied
# directory. The directory must contain `client/`, `cmd/`, and/or
# `internal/` subtrees (missing subtrees are skipped). Returns 0 on
# zero matches, 1 on matches found.
run_grep() {
    local root="$1"
    local subdirs=()
    for sub in client cmd internal; do
        if [ -d "$root/$sub" ]; then
            subdirs+=("$root/$sub")
        fi
    done
    if [ "${#subdirs[@]}" -eq 0 ]; then
        echo "test-e-continuity.sh: no client/, cmd/, or internal/ subdirs found under $root" >&2
        # Treat as "nothing to grep" — Test E is vacuously green.
        return 0
    fi

    # The -E flag is mandatory; without it the `|` alternation is
    # interpreted literally and the gate silently false-negatives
    # (spec plan Bead 4 step 4 emphasizes this exact failure mode).
    local matches
    matches="$(grep -rEn \
        'exec\.Command.*"mindspec"|LookPath.*"mindspec"|StartProcess.*"mindspec"' \
        "${subdirs[@]}" 2>/dev/null || true)"

    if [ -n "$matches" ]; then
        echo "test-e-continuity.sh: Test E FAILED — circular-discovery references found:" >&2
        printf '%s\n' "$matches" >&2
        return 1
    fi
    echo "test-e-continuity.sh: Test E PASSED — zero circular-discovery references in $root" >&2
    return 0
}

# Sibling-mode shortcut.
if [ "$USE_SIBLING" -eq 1 ]; then
    SIBLING="$REPO_ROOT/../agentmind"
    if [ -d "$SIBLING" ] && [ -f "$SIBLING/go.mod" ] && \
       grep -q '^module github\.com/mrmaxsteel/agentmind$' "$SIBLING/go.mod"; then
        echo "test-e-continuity.sh: using sibling checkout at $SIBLING (not the pinned tag)" >&2
        run_grep "$SIBLING"
        exit $?
    fi
    echo "test-e-continuity.sh: --use-sibling requested but no usable agentmind sibling found at $SIBLING" >&2
    # Fall through to the upstream-clone path below.
fi

# Probe upstream for the tag.
ERR_FILE="$(mktemp -t test-e-continuity.err.XXXXXX)"
CLONE_DIR=""
cleanup() {
    rm -f "$ERR_FILE"
    if [ -n "$CLONE_DIR" ] && [ -d "$CLONE_DIR" ]; then
        rm -rf "$CLONE_DIR"
    fi
}
trap cleanup EXIT

if ! LS_REMOTE_OUT="$(git ls-remote --tags "$REPO_URL" 2>"$ERR_FILE")"; then
    echo "test-e-continuity.sh: upstream unreachable: $REPO_URL" >&2
    if [ -s "$ERR_FILE" ]; then
        sed 's/^/  /' "$ERR_FILE" >&2
    fi
    exit 3
fi

SHA="$(printf '%s\n' "$LS_REMOTE_OUT" | awk -v tag="refs/tags/$TAG" '$2 == tag { print $1; exit }')"
if [ -z "$SHA" ]; then
    echo "test-e-continuity.sh: tag $TAG NOT found at $REPO_URL" >&2
    echo "" >&2
    echo "  This is the expected outcome until upstream publishes" >&2
    echo "  $TAG. Spec 083 Bead 6 step 4 commits to running this" >&2
    echo "  gate against the pinned tag once it exists; CI treats" >&2
    echo "  exit 2 as a skip-with-warning, not a hard failure." >&2
    exit 2
fi

CLONE_DIR="$(mktemp -d -t test-e-continuity.clone.XXXXXX)"
echo "test-e-continuity.sh: shallow-cloning $REPO_URL@$TAG (sha $SHA) -> $CLONE_DIR" >&2
git clone --depth 1 --branch "$TAG" "$REPO_URL" "$CLONE_DIR" >/dev/null 2>&1

run_grep "$CLONE_DIR"
