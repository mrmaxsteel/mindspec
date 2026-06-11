#!/bin/sh
# git merge driver for .beads/issues.jsonl — ADR-0025: the jsonl is a
# deterministic projection of the Dolt DB, so the semantically correct merge
# is to regenerate it from the DB (ADR-0023: Dolt is the single state
# authority). Replaces the orphaned `bd merge` driver removed in bd 1.0.x.
#
# Args (configured as %A %O %B):
#   $1 = %A current version  (the merge result must land here)
#   $2 = %O common ancestor
#   $3 = %B other branch's version
#
# Exit contract: 0 = merged (result in %A); 1 = conflict (git keeps the
# three stages and the merge proceeds for other paths). NEVER exit >= 129
# (git treats signal-range exit codes as fatal and aborts the whole merge).
#
# NOTE: keep the export invocation in lockstep with bd's own auto-export
# profile (bd export defaults exclude infrastructure beads and memories);
# a doctor assertion should guard this (incident panel 3/3, 2026-06-11).
set -eu

fail() {
    echo "bd-jsonl-merge-driver: $1" >&2
    echo "recovery: install bd >= 1.0.4 (driver regenerates .beads/issues.jsonl from the Dolt DB)" >&2
    exit 1
}

[ "$#" -eq 3 ] || fail "expected 3 args (%A %O %B), got $#"

command -v bd >/dev/null 2>&1 || fail "bd not found on PATH"

bd export -o "$1" || fail "bd export failed (DB unreachable?)"

[ -s "$1" ] || fail "regenerated export is empty — refusing silent data loss"

# Fail-closed superset gate: every issue id present in the ancestor (%O) or
# the other side (%B) must appear in the regenerated result, otherwise a
# stale/foreign DB could silently drop rows. Deliberate deletions surface
# here as a visible conflict with the missing ids listed on stderr.
ids_of() {
    # shellcheck disable=SC2086
    { grep -o '"id":"[^"]*"' "$1" 2>/dev/null || true; } | sort -u
}

tmpdir=$(mktemp -d) || fail "mktemp failed"
trap 'rm -rf "$tmpdir"' EXIT

ids_of "$1" >"$tmpdir/result"

for side in "$2" "$3"; do
    ids_of "$side" >"$tmpdir/side"
    missing=$(comm -23 "$tmpdir/side" "$tmpdir/result" || true)
    if [ -n "$missing" ]; then
        echo "bd-jsonl-merge-driver: ids present in merge parent $side but missing from regenerated export:" >&2
        printf '%s\n' "$missing" >&2
        fail "superset validation failed"
    fi
done

exit 0
