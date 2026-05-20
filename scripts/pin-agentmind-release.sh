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
#   scripts/pin-agentmind-release.sh v1.0.0 --self-test
#     # dry-run the edit against a temporary copy of the real go.mod,
#     # assert that exactly the replace directive is dropped and the
#     # require line is bumped to $VERSION, then exit. Does NOT touch
#     # the real go.mod. Intended for CI to exercise the editor against
#     # the live go.mod shape BEFORE the day Phase 6 fires for real.
#
# Exit codes:
#   0  — go.mod pinned at $VERSION and `go build` + `go test -short` pass
#        (or --dry-run / --self-test completed successfully).
#   2  — upstream reachable but the requested tag is absent OR upstream
#        proxy cannot resolve the module at the tag. Expected today:
#        v1.0.0 not yet published; the pin step is deferred until the
#        agentmind side ships v1.0.0. Re-run after the tag exists.
#   3  — upstream unreachable. Use --skip-upstream-check to proceed
#        anyway (e.g. behind a corporate mirror).
#   4  — invocation error (bad args, not run from repo root, missing
#        tools).
#   5  — go.mod edit succeeded but `go mod tidy` or the build/test
#        verification failed. The script leaves go.mod in the edited
#        state so the operator can inspect and decide. Use --no-verify
#        to skip if the build is known to be broken for unrelated
#        reasons. Recover with: git checkout go.mod
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
SELF_TEST=0

usage() {
    cat <<'EOF'
pin-agentmind-release.sh — Spec 083 Bead 6 release-pin helper.

Drops the local `replace` directive from go.mod and pins
`require github.com/mrmaxsteel/agentmind` to the supplied release tag.

Usage:
  scripts/pin-agentmind-release.sh [VERSION] [--dry-run] [--no-verify]
                                   [--skip-upstream-check] [--self-test]

  VERSION                Tag to pin (default: v1.0.0).
  --dry-run              Show the planned go.mod diff without writing.
  --no-verify            Skip `go build` + `go test -short` after the edit.
  --skip-upstream-check  Do not require the tag to be reachable upstream.
  --self-test            Dry-run the edit against a temp copy of the real
                         go.mod and assert the diff shape is correct.

Exit codes:
  0  go.mod pinned and verification passed (or dry-run / self-test completed).
  2  upstream reachable, tag absent (deferral: re-run after agentmind ships).
  3  upstream unreachable (use --skip-upstream-check to override).
  4  invocation error.
  5  go.mod edit succeeded but verification failed; edit left in place
     (recover with: git checkout go.mod).
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
        --self-test)
            SELF_TEST=1
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
TMP_GOMOD_DIR=""
ERR_FILE=""
cleanup() {
    [ -n "$TMP_GOMOD_DIR" ] && [ -d "$TMP_GOMOD_DIR" ] && rm -rf "$TMP_GOMOD_DIR"
    [ -n "$ERR_FILE" ]  && rm -f "$ERR_FILE"
    return 0
}
trap cleanup EXIT

# 1. Upstream-tag reachability gate.
#    Two layers: (a) git ls-remote confirms the tag exists in the upstream
#    repo; (b) `go mod download` confirms the module is fetchable via Go's
#    module proxy (GOPROXY), which is what the require-line resolution will
#    actually use after the pin. A clean git tag with no proxy entry would
#    fail at `go mod tidy` later — catch that here so the deferral mode is
#    proxy-aware, not just repo-aware.
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

    # GOPROXY verification: confirm the module is actually fetchable
    # through Go's proxy at the requested tag. This rules out "git tag
    # exists but proxy index has not yet picked it up" and also prevents
    # a local sibling/module cache from silently satisfying the later
    # verify chain when verification is supposed to exercise the
    # released artifact. Probe with a clean GOPROXY in a scratch
    # GOMODCACHE so on-disk caches cannot short-circuit the lookup.
    PROXY_GOMODCACHE="$(mktemp -d -t pin-agentmind-release.gomodcache.XXXXXX)"
    if ! GOPROXY="https://proxy.golang.org,direct" GOFLAGS=-mod=mod \
            GOMODCACHE="$PROXY_GOMODCACHE" \
            go mod download -x "github.com/mrmaxsteel/agentmind@$VERSION" \
            >"$ERR_FILE" 2>&1; then
        echo "pin-agentmind-release.sh: tag $VERSION exists in git but the Go module proxy could not resolve github.com/mrmaxsteel/agentmind@$VERSION" >&2
        if [ -s "$ERR_FILE" ]; then
            sed 's/^/  /' "$ERR_FILE" >&2
        fi
        echo "" >&2
        echo "  The proxy may not have indexed the release yet (typical" >&2
        echo "  lag is a few minutes). Re-run after the proxy catches up," >&2
        echo "  or override with --skip-upstream-check if proxying through" >&2
        echo "  a mirror." >&2
        rm -rf "$PROXY_GOMODCACHE"
        exit 2
    fi
    rm -rf "$PROXY_GOMODCACHE"
    echo "pin-agentmind-release.sh: module proxy resolves github.com/mrmaxsteel/agentmind@$VERSION" >&2
fi

# 2. Compute the new go.mod content using `go mod edit` against a
#    temp copy of go.mod. Using the native tool (rather than a
#    hand-rolled awk state machine) handles all the formatting edge
#    cases natively: `// indirect` trailing comments on require lines,
#    multi-line `replace ( ... )` blocks, preceding comment blocks,
#    standalone vs grouped require directives.
# `go mod edit -modfile` requires the file path to end in `.mod`, so we
# create a temp directory and place the working copy inside it.
TMP_GOMOD_DIR="$(mktemp -d -t pin-agentmind-release.gomod.XXXXXX)"
TMP_GOMOD="$TMP_GOMOD_DIR/go.mod"
cp "$GOMOD" "$TMP_GOMOD"

# Drop any local replace targeting agentmind (single-line or grouped form).
# `go mod edit -dropreplace` is a no-op when no matching replace exists,
# which keeps the edit idempotent.
if ! go mod edit -modfile="$TMP_GOMOD" \
        -dropreplace=github.com/mrmaxsteel/agentmind; then
    echo "pin-agentmind-release.sh: internal error — go mod edit -dropreplace failed" >&2
    exit 5
fi

# Pin the require line to $VERSION. `-require` adds-or-rewrites the
# entry, so this works whether the current go.mod has the zero
# pseudo-version, a real version, or no require at all.
if ! go mod edit -modfile="$TMP_GOMOD" \
        -require="github.com/mrmaxsteel/agentmind@$VERSION"; then
    echo "pin-agentmind-release.sh: internal error — go mod edit -require failed" >&2
    exit 5
fi

# Sanity: confirm the produced file no longer contains the replace
# directive and the require line carries the desired version.
if grep -qE '^[[:space:]]*replace[[:space:]]+github\.com/mrmaxsteel/agentmind[[:space:]]+=>' "$TMP_GOMOD"; then
    echo "pin-agentmind-release.sh: internal error — replace directive still present after edit" >&2
    exit 5
fi
if ! grep -qE "(^|[[:space:]])github\.com/mrmaxsteel/agentmind[[:space:]]+$VERSION([[:space:]]|$)" "$TMP_GOMOD"; then
    echo "pin-agentmind-release.sh: internal error — require line did not pick up $VERSION" >&2
    diff -u "$GOMOD" "$TMP_GOMOD" >&2 || true
    exit 5
fi

# Self-test mode: assert the diff is exactly "replace dropped, require
# bumped" and nothing else. Intended for CI to run against the live
# go.mod long before Phase 6 actually fires.
if [ "$SELF_TEST" -eq 1 ]; then
    echo "=== SELF-TEST: editor diff against real go.mod ==="
    DIFF_OUT="$(diff -u "$GOMOD" "$TMP_GOMOD" || true)"
    printf '%s\n' "$DIFF_OUT"

    # Required signals: at least one `-replace github.com/mrmaxsteel/agentmind`
    # line (replace dropped) and at least one `+...github.com/mrmaxsteel/agentmind $VERSION`
    # line (require bumped). If either is missing the editor did not do
    # what its contract claims.
    if ! printf '%s\n' "$DIFF_OUT" \
            | grep -qE '^-[[:space:]]*replace[[:space:]]+github\.com/mrmaxsteel/agentmind[[:space:]]+=>'; then
        echo "pin-agentmind-release.sh: SELF-TEST FAILED — diff does not drop the replace directive" >&2
        exit 5
    fi
    if ! printf '%s\n' "$DIFF_OUT" \
            | grep -qE "^\+.*github\.com/mrmaxsteel/agentmind[[:space:]]+$VERSION"; then
        echo "pin-agentmind-release.sh: SELF-TEST FAILED — diff does not bump require to $VERSION" >&2
        exit 5
    fi
    echo "pin-agentmind-release.sh: SELF-TEST PASSED — editor produces correct diff against real go.mod" >&2
    exit 0
fi

if [ "$DRY_RUN" -eq 1 ]; then
    echo "=== DRY RUN: planned go.mod diff ==="
    diff -u "$GOMOD" "$TMP_GOMOD" || true
    exit 0
fi

mv "$TMP_GOMOD" "$GOMOD"
# mv consumed the temp file; clear the variable so cleanup tears
# down only the now-empty directory.
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
