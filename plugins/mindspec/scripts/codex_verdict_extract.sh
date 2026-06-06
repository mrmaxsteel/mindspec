#!/usr/bin/env bash
# codex_verdict_extract.sh — extract a verdict JSON object from a codex .out log
# when codex finished thinking but didn't write the file to disk.
#
# Usage: codex_verdict_extract.sh /tmp/codex_<panel>_r<slot>.out > <expected-json-path>
#
# Strategy: scan for the largest contiguous JSON object containing both
#   `"verdict"` and `"confidence"` keys. Codex's terminal-output verdict
#   typically appears in a fenced or unfenced JSON block at the end of its
#   reasoning stream.
#
# Returns: 0 if a verdict object was extracted, 1 if nothing matched.

set -eo pipefail

OUT="${1:?usage: $0 <codex-out-log>}"
[ -f "$OUT" ] || { echo "no such file: $OUT" >&2; exit 1; }

# Strip ANSI escapes, then find JSON objects containing the verdict marker.
# This is greedy on purpose — codex sometimes emits multiple draft JSONs;
# the LAST one with both keys is the final verdict.
sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g' "$OUT" \
  | awk '
    /^[[:space:]]*\{/ { depth=1; buf=$0"\n"; next }
    depth > 0 {
      buf = buf $0 "\n"
      for (i = 1; i <= length($0); i++) {
        c = substr($0, i, 1)
        if (c == "{") depth++
        else if (c == "}") depth--
      }
      if (depth == 0 && buf ~ /"verdict"/ && buf ~ /"confidence"/) {
        candidate = buf
        buf = ""
      }
    }
    END { if (candidate != "") print candidate }
  '

# Exit 1 if nothing was emitted (caller falls back to claude-sub).
test -n "$(sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g' "$OUT" | grep -c '"verdict"')" || exit 1
