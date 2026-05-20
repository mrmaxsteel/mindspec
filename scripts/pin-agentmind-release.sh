#!/usr/bin/env bash
# pin-agentmind-release.sh — Spec 083 Bead 6 release-pin helper.
#
# Performs the go.mod edit that Phase 6 commits to: drops the local
# `replace github.com/mrmaxsteel/agentmind => ../agentmind` directive
# and pins `require github.com/mrmaxsteel/agentmind` to the supplied
# release tag (default: v1.0.0). Then runs `go mod tidy` and verifies
# `go build ./cmd/mindspec && go test -short ./...` pass.
#
# This script is the "actual v1.0.0 pinning" mechanism whose execution
# is deferred until upstream `github.com/mrmaxsteel/agentmind` publishes
# the tagged release. Until then the script is dormant; invoking it
# will exit non-zero with a clear deferral message if the tag is not
# reachable upstream.
#
# Idempotent: re-running with the same version leaves go.mod unchanged
# and re-verifies the build.
#
# Usage:
#   scripts/pin-agentmind-release.sh                 # default: v1.0.0
#   scripts/pin-agentmind-release.sh v1.0.0
#   scripts/pin-agentmind-release.sh v1.0.0 --dry-run
#   scripts/pin-agentmind-release.sh v1.0.0 --no-verify
#     # skip the build + test verification (advanced; not recommended)
#   scripts/pin-agentmind-release.sh v1.0.0 --skip-upstream-check
#     # do not require the tag to be reachable upstream (advanced; for
#     # offline / mirror scenarios only)
#
# Exit codes:
#   0  — go.mod pinned at $VERSION and `go build` + `go test -short` pass
#        (or --dry-run printed the planned diff).
#   2  — upstream reachable but the requested tag is absent. Expected
#        today: v1.0.0 not yet published; the pin step is deferred until
#        the agentmind side ships v1.0.0. Re-run after the tag exists.
#   3  — upstream unreachable. Use --skip-upstream-check to proceed
#        anyway (e.g. behind a corporate mirror).
#   4  — invocation error (bad args, not run from repo root, missing
#        tools).
#   5  — go.mod edit succeeded but `go mod tidy` or the build/test
#        verification failed. The script leaves go.mod in the edited
#        state so the operator can inspect and decide. Use --no-verify
#        to skip if the build is known to be broken for unrelated
#        reasons.
#
# Spec reference:
#   .mindspec/docs/specs/083-agentmind-extraction-v2/spec.md
#   - Phase 6 (lines 396-406): "mindspec drops the local `replace`
#     directive and pins `agentmind v1.0.0`."
#   .mindspec/docs/specs/083-agentmind-extraction-v2/plan.md
#   - Bead 6 step 1.

set -euo pipefail

export GIT_TERMINAL_PROMPT=0

REPO_URL="${AGENTMIND_REPO_URL:-https://github.com/mrmaxsteel/agentmind}"

VERSION=""
DRY_RUN=0
NO_VERIFY=0
SKIP_UPSTREAM=0

usage() {
    cat <<'EOF'
pin-agentmind-release.sh — Spec 083 Bead 6 release-pin helper.

Drops the local `replace` directive from go.mod and pins
`require github.com/mrmaxsteel/agentmind` to the supplied release tag.

Usage:
  scripts/pin-agentmind-release.sh [VERSION] [--dry-run] [--no-verify]
                                   [--skip-upstream-check]

  VERSION                Tag to pin (default: v1.0.0).
  --dry-run              Show the planned go.mod diff without writing.
  --no-verify            Skip `go build` + `go test -short` after the edit.
  --skip-upstream-check  Do not require the tag to be reachable upstream.

Exit codes:
  0  go.mod pinned and verification passed (or dry-run completed).
  2  upstream reachable, tag absent (deferral: re-run after agentmind ships).
  3  upstream unreachable (use --skip-upstream-check to override).
  4  invocation error.
  5  go.mod edit succeeded but verification failed; edit left in place.
EOF
}

for arg in "$@"; do
    case "$arg" in
        --help|-h)
            usage
            exit 0
            ;;
        --dry-run)
            DRY_RUN=1
            ;;
        --no-verify)
            NO_VERIFY=1
            ;;
        --skip-upstream-check)
            SKIP_UPSTREAM=1
            ;;
        --*)
            echo "pin-agentmind-release.sh: unknown flag '$arg'" >&2
            usage >&2
            exit 4
            ;;
        *)
            if [ -z "$VERSION" ]; then
                VERSION="$arg"
            else
                echo "pin-agentmind-release.sh: unexpected positional argument '$arg'" >&2
                usage >&2
                exit 4
            fi
            ;;
    esac
done

VERSION="${VERSION:-v1.0.0}"

# Validate the version string. Module SemVer requires `vMAJOR.MINOR.PATCH`,
# optionally with `-prerelease` and `+build` suffixes.
if ! printf '%s' "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
    echo "pin-agentmind-release.sh: VERSION '$VERSION' is not a valid Go module SemVer tag" >&2
    echo "  expected: vMAJOR.MINOR.PATCH (e.g. v1.0.0)" >&2
    exit 4
fi

if ! command -v git >/dev/null 2>&1; then
    echo "pin-agentmind-release.sh: git not found on PATH" >&2
    exit 4
fi
if ! command -v go >/dev/null 2>&1; then
    echo "pin-agentmind-release.sh: go not found on PATH" >&2
    exit 4
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GOMOD="$REPO_ROOT/go.mod"

if [ ! -f "$GOMOD" ]; then
    echo "pin-agentmind-release.sh: go.mod not found at $GOMOD" >&2
    exit 4
fi

# Single cleanup hook used throughout. Variables are cleared as files
# get consumed so the trap stays a no-op for the consumed paths.
TMP_GOMOD=""
ERR_FILE=""
cleanup() {
    [ -n "$TMP_GOMOD" ] && rm -f "$TMP_GOMOD"
    [ -n "$ERR_FILE" ]  && rm -f "$ERR_FILE"
    return 0
}
trap cleanup EXIT

# 1. Upstream-tag reachability gate.
if [ "$SKIP_UPSTREAM" -eq 0 ]; then
    ERR_FILE="$(mktemp -t pin-agentmind-release.err.XXXXXX)"

    if ! LS_REMOTE_OUT="$(git ls-remote --tags "$REPO_URL" 2>"$ERR_FILE")"; then
        echo "pin-agentmind-release.sh: upstream unreachable: $REPO_URL" >&2
        if [ -s "$ERR_FILE" ]; then
            sed 's/^/  /' "$ERR_FILE" >&2
        fi
        echo "" >&2
        echo "  Re-run with --skip-upstream-check if you have a verified" >&2
        echo "  local mirror of the agentmind release." >&2
        exit 3
    fi

    SHA="$(printf '%s\n' "$LS_REMOTE_OUT" | awk -v tag="refs/tags/$VERSION" '$2 == tag { print $1; exit }')"
    if [ -z "$SHA" ]; then
        echo "pin-agentmind-release.sh: tag $VERSION NOT found at $REPO_URL" >&2
        echo "" >&2
        echo "  This is the expected outcome until upstream publishes" >&2
        echo "  $VERSION. Spec 083 Bead 6 step 1 commits to pinning the" >&2
        echo "  tag once it exists; re-run this script after the agentmind" >&2
        echo "  release lands." >&2
        exit 2
    fi
    echo "pin-agentmind-release.sh: upstream tag $VERSION resolves to $SHA" >&2
fi

# 2. Compute the new go.mod content. We rewrite the require line and
#    drop any `replace github.com/mrmaxsteel/agentmind => ...` directive
#    (and its leading comment block, if it's the spec-prescribed one).
TMP_GOMOD="$(mktemp -t pin-agentmind-release.gomod.XXXXXX)"

# Use awk to do the edit deterministically. State machine:
#   - When we see `require github.com/mrmaxsteel/agentmind <ver>`,
#     replace `<ver>` with $VERSION.
#   - When we see a comment block immediately preceding a
#     `replace github.com/mrmaxsteel/agentmind => ...` line, drop the
#     whole block plus the replace line plus one trailing blank line.
#   - Otherwise, pass through.
awk -v ver="$VERSION" '
    BEGIN { buffer = ""; in_replace_block = 0 }

    # Match the require line in either form:
    #   require github.com/mrmaxsteel/agentmind v1.2.3
    #   <TAB>github.com/mrmaxsteel/agentmind v1.2.3   (inside require block)
    # In both cases rewrite the trailing version to $ver.
    /^[[:space:]]*require[[:space:]]+github\.com\/mrmaxsteel\/agentmind[[:space:]]+v/ ||
    /^[[:space:]]+github\.com\/mrmaxsteel\/agentmind[[:space:]]+v/ {
        sub(/[[:space:]]+v[0-9][^[:space:]]*[[:space:]]*$/, " " ver)
        print
        next
    }

    # Match the replace directive itself. If we have buffered comment
    # lines, drop them; otherwise just skip the line. Also consume the
    # immediately-following blank line if present.
    /^replace[[:space:]]+github\.com\/mrmaxsteel\/agentmind[[:space:]]+=>/ {
        buffer = ""
        in_replace_block = 1
        next
    }

    # When we just dropped the replace, also drop the immediately-
    # following blank line so the file stays tidy.
    in_replace_block == 1 {
        if ($0 == "") {
            in_replace_block = 0
            next
        }
        # Non-blank: flush whatever was buffered (likely empty) and
        # process this line normally.
        in_replace_block = 0
    }

    # Buffer comment lines. They will either be flushed (when the next
    # line is not the replace directive) or dropped (when it is).
    /^\/\// {
        if (buffer == "") {
            buffer = $0
        } else {
            buffer = buffer "\n" $0
        }
        next
    }

    # Any non-comment, non-replace line: flush the buffer and pass
    # the current line through.
    {
        if (buffer != "") {
            print buffer
            buffer = ""
        }
        print
    }

    END {
        if (buffer != "") {
            print buffer
        }
    }
' "$GOMOD" > "$TMP_GOMOD"

# Sanity: confirm the produced file no longer contains the replace
# directive and the require line carries the desired version.
if grep -qE '^replace[[:space:]]+github\.com/mrmaxsteel/agentmind[[:space:]]+=>' "$TMP_GOMOD"; then
    echo "pin-agentmind-release.sh: internal error — replace directive still present after edit" >&2
    exit 5
fi
if ! grep -qE "^([[:space:]]*require[[:space:]]+)?[[:space:]]*github\.com/mrmaxsteel/agentmind[[:space:]]+$VERSION([[:space:]]|$)" "$TMP_GOMOD"; then
    echo "pin-agentmind-release.sh: internal error — require line did not pick up $VERSION" >&2
    diff -u "$GOMOD" "$TMP_GOMOD" >&2 || true
    exit 5
fi

if [ "$DRY_RUN" -eq 1 ]; then
    echo "=== DRY RUN: planned go.mod diff ==="
    diff -u "$GOMOD" "$TMP_GOMOD" || true
    exit 0
fi

mv "$TMP_GOMOD" "$GOMOD"
# mv consumed the temp file; clear the variable so cleanup is a no-op.
TMP_GOMOD=""
echo "pin-agentmind-release.sh: go.mod pinned to agentmind $VERSION" >&2

# 3. Tidy + verify.
if [ "$NO_VERIFY" -eq 1 ]; then
    echo "pin-agentmind-release.sh: --no-verify set; skipping go mod tidy + build + test" >&2
    exit 0
fi

cd "$REPO_ROOT"

if ! go mod tidy; then
    echo "pin-agentmind-release.sh: go mod tidy failed; go.mod left edited for inspection" >&2
    exit 5
fi

if ! go build ./cmd/mindspec; then
    echo "pin-agentmind-release.sh: go build failed; go.mod left edited for inspection" >&2
    exit 5
fi

if ! go test -short ./...; then
    echo "pin-agentmind-release.sh: go test -short failed; go.mod left edited for inspection" >&2
    exit 5
fi

echo "pin-agentmind-release.sh: done — agentmind pinned at $VERSION, build + test green" >&2
exit 0
