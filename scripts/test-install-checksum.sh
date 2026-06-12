#!/bin/sh
# Regression guard for the v0.8.0 install-blocker.
#
# v0.8.0 started shipping syft SBOMs, so checksums.txt gained a second line for
# every archive: the archive itself AND its <archive>.spdx.json SBOM. install.sh
# extracted the checksum with an unanchored `grep "$ARCHIVE_NAME"`, which matched
# BOTH lines, joined the two hashes with a newline, and made the compare against
# the single real archive hash fail — aborting every `curl | sh` install with
# "Checksum verification failed!" even though the binary was valid.
#
# The fix made extraction an exact field-2 match (extract_checksum in install.sh,
# `awk '$2==f'`). This test sources that REAL function from install.sh and asserts
# it returns exactly one hash — the archive's, not the SBOM's, not the two joined.
#
# Deterministic, offline, POSIX sh. Exits non-zero on any mismatch.

set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
INSTALL_SH="$REPO_ROOT/install.sh"

# Source install.sh for its functions only — MINDSPEC_INSTALL_NO_MAIN stops it
# from running main() (which would try to detect the OS and hit the network).
MINDSPEC_INSTALL_NO_MAIN=1
export MINDSPEC_INSTALL_NO_MAIN
# shellcheck source=../install.sh disable=SC1091
. "$INSTALL_SH"

if ! command -v extract_checksum >/dev/null 2>&1; then
    echo "FAIL: install.sh does not expose an extract_checksum function to source" >&2
    exit 1
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT
FIXTURE="$WORK_DIR/checksums.txt"

# Known hashes (arbitrary but distinct) so we can prove which line was picked.
ARCHIVE_AMD64_HASH="89c1ca7500000000000000000000000000000000000000000000000000000001"
SBOM_AMD64_HASH="ffffffff00000000000000000000000000000000000000000000000000000002"
ARCHIVE_ARM64_HASH="abcabcab00000000000000000000000000000000000000000000000000000003"
SBOM_ARM64_HASH="dededede00000000000000000000000000000000000000000000000000000004"

ARCHIVE_AMD64="mindspec_0.8.0_linux_amd64.tar.gz"
ARCHIVE_ARM64="mindspec_0.8.0_darwin_arm64.tar.gz"

# Mirror the real v0.8.0 goreleaser+syft layout: each archive is followed by its
# .spdx.json SBOM, hashes separated from the filename by two spaces. Interleave
# arches and a checksums.txt.spdx self-line for realism.
{
    printf '%s  %s\n'           "$ARCHIVE_AMD64_HASH" "$ARCHIVE_AMD64"
    printf '%s  %s.spdx.json\n' "$SBOM_AMD64_HASH"    "$ARCHIVE_AMD64"
    printf '%s  %s\n'           "$ARCHIVE_ARM64_HASH" "$ARCHIVE_ARM64"
    printf '%s  %s.spdx.json\n' "$SBOM_ARM64_HASH"    "$ARCHIVE_ARM64"
} > "$FIXTURE"

fail() { echo "FAIL: $1" >&2; exit 1; }

# --- Guard 0: prove the fixture actually reproduces the collision -------------
# The OLD, buggy extraction was: grep "$ARCHIVE_NAME" | awk '{print $1}'.
# Confirm it really returns TWO hashes here, so this fixture genuinely exercises
# the bug class. (Documented contrast — we do NOT use this form in install.sh.)
old_buggy_lines=$(grep -c "$ARCHIVE_AMD64" "$FIXTURE")
[ "$old_buggy_lines" = "2" ] || \
    fail "fixture does not reproduce the SBOM collision (old grep matched $old_buggy_lines lines, expected 2)"

# --- Guard 1: amd64 returns exactly one hash, the archive's -------------------
got=$(extract_checksum "$ARCHIVE_AMD64" "$FIXTURE")
lines=$(printf '%s\n' "$got" | grep -c .)
[ "$lines" = "1" ] || fail "amd64 extraction returned $lines hashes (expected 1): [$got]"
[ "$got" = "$ARCHIVE_AMD64_HASH" ] || \
    fail "amd64 extraction returned wrong hash: got [$got], expected archive hash [$ARCHIVE_AMD64_HASH]"
[ "$got" != "$SBOM_AMD64_HASH" ] || fail "amd64 extraction returned the SBOM hash, not the archive's"

# --- Guard 2: a second arch behaves identically -------------------------------
got=$(extract_checksum "$ARCHIVE_ARM64" "$FIXTURE")
lines=$(printf '%s\n' "$got" | grep -c .)
[ "$lines" = "1" ] || fail "arm64 extraction returned $lines hashes (expected 1): [$got]"
[ "$got" = "$ARCHIVE_ARM64_HASH" ] || \
    fail "arm64 extraction returned wrong hash: got [$got], expected [$ARCHIVE_ARM64_HASH]"

# --- Guard 3: the SBOM filename itself still resolves to its own single hash ---
got=$(extract_checksum "${ARCHIVE_AMD64}.spdx.json" "$FIXTURE")
[ "$got" = "$SBOM_AMD64_HASH" ] || fail "explicit SBOM lookup returned wrong hash: [$got]"

# --- Guard 4: unknown arch yields empty (skip-verification path) ---------------
got=$(extract_checksum "mindspec_0.8.0_plan9_riscv64.tar.gz" "$FIXTURE")
[ -z "$got" ] || fail "unknown arch should yield empty output, got: [$got]"

echo "PASS: extract_checksum returns exactly the archive hash and ignores SBOM lines"
