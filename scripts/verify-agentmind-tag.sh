#!/usr/bin/env bash
# verify-agentmind-tag.sh — Spec 083 Bead 1 / Test G prerequisite gate.
#
# Verifies that the upstream agentmind repository has published a given tag
# (default: v0.0.1) and prints the tag's commit SHA on success.
#
# Spec reference:
#   .mindspec/docs/specs/083-agentmind-extraction-v2/spec.md
#   - "Test G — Phase 0 prerequisite gate (the agentmind v0.0.1 tag exists)"
#   - Acceptance criterion: "agentmind v0.0.1 SHA recorded before Phase 1"
#
# Exit codes:
#   0  — tag was found upstream; SHA printed to stdout.
#   2  — tag NOT found upstream (expected today: v0.0.1 not yet published).
#   3  — upstream repository unreachable (network or repo-doesn't-exist).
#   4  — invocation error (bad arguments / missing git).
#
# Usage:
#   scripts/verify-agentmind-tag.sh                 # default: v0.0.1
#   scripts/verify-agentmind-tag.sh v0.1.0          # check a different tag
#   scripts/verify-agentmind-tag.sh v0.0.1 --record # also patch spec.md placeholder
#
# Designed so that when v0.0.1 is eventually published upstream, re-running with
# --record will replace the spec's <TBD> placeholder with the real SHA.

set -euo pipefail

REPO_URL="${AGENTMIND_REPO_URL:-https://github.com/mrmaxsteel/agentmind}"

# Handle --help / -h as either the first or any positional argument.
for arg in "$@"; do
    case "$arg" in
        --help|-h)
            sed -n '2,30p' "$0"
            exit 0
            ;;
    esac
done

TAG="${1:-v0.0.1}"
shift || true

RECORD=0
for arg in "$@"; do
    case "$arg" in
        --record) RECORD=1 ;;
        *)
            echo "verify-agentmind-tag.sh: unknown argument: $arg" >&2
            exit 4
            ;;
    esac
done

if ! command -v git >/dev/null 2>&1; then
    echo "verify-agentmind-tag.sh: git not found on PATH" >&2
    exit 4
fi

# Resolve repo root so --record can find spec.md regardless of cwd.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SPEC_FILE="$REPO_ROOT/.mindspec/docs/specs/083-agentmind-extraction-v2/spec.md"

# Probe the upstream. Capture stdout+stderr separately so we can tell
# "repo unreachable" from "repo reachable but tag absent".
LS_REMOTE_OUT="$(git ls-remote --tags "$REPO_URL" 2>/tmp/verify-agentmind-tag.err)" && LS_REMOTE_RC=0 || LS_REMOTE_RC=$?

if [ "$LS_REMOTE_RC" -ne 0 ]; then
    echo "verify-agentmind-tag.sh: upstream unreachable: $REPO_URL" >&2
    echo "  (git ls-remote exited $LS_REMOTE_RC)" >&2
    if [ -s /tmp/verify-agentmind-tag.err ]; then
        sed 's/^/  /' /tmp/verify-agentmind-tag.err >&2
    fi
    echo "" >&2
    echo "  Expected during the parallel mindspec-side migration:" >&2
    echo "  the github.com/mrmaxsteel/agentmind repository has not yet been" >&2
    echo "  created. Phase 1 of spec 083 may not begin until this gate passes." >&2
    exit 3
fi

# Match exactly "refs/tags/<TAG>" (no peeled-tag suffix "^{}").
SHA="$(printf '%s\n' "$LS_REMOTE_OUT" | awk -v tag="refs/tags/$TAG" '$2 == tag { print $1; exit }')"

if [ -z "$SHA" ]; then
    echo "verify-agentmind-tag.sh: tag $TAG NOT found at $REPO_URL" >&2
    echo "" >&2
    echo "  Expected during the parallel mindspec-side migration:" >&2
    echo "  agentmind $TAG has not yet been published upstream. Phase 1 of" >&2
    echo "  spec 083 may not begin until the tag is published and this gate" >&2
    echo "  passes. Re-run with --record to update spec.md once the tag exists:" >&2
    echo "    scripts/verify-agentmind-tag.sh $TAG --record" >&2
    exit 2
fi

# Found.
echo "$SHA"

if [ "$RECORD" -eq 1 ]; then
    if [ ! -f "$SPEC_FILE" ]; then
        echo "verify-agentmind-tag.sh: --record specified but spec.md not found at $SPEC_FILE" >&2
        exit 4
    fi
    if [ "$TAG" != "v0.0.1" ]; then
        echo "verify-agentmind-tag.sh: --record currently only updates the v0.0.1 placeholder; got $TAG" >&2
        exit 4
    fi
    # Replace the spec's placeholder line in-place. The placeholder shape is:
    #   `agentmind v0.0.1 SHA: <TBD — record before Phase 1>`
    # We use a portable Python invocation to avoid sed -i portability issues
    # between GNU and BSD sed.
    python3 - "$SPEC_FILE" "$SHA" <<'PY'
import pathlib, sys, re
path = pathlib.Path(sys.argv[1])
sha = sys.argv[2]
src = path.read_text()
new = re.sub(
    r"agentmind v0\.0\.1 SHA: <TBD[^`>]*>",
    f"agentmind v0.0.1 SHA: {sha}",
    src,
)
if new == src:
    print(
        "verify-agentmind-tag.sh: --record found no <TBD> placeholder to "
        "replace (already recorded?)",
        file=sys.stderr,
    )
    sys.exit(4)
path.write_text(new)
print(f"verify-agentmind-tag.sh: recorded v0.0.1 SHA {sha} in {path}", file=sys.stderr)
PY
fi

exit 0
