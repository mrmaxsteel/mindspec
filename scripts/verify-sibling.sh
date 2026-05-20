#!/usr/bin/env bash
# verify-sibling.sh — Panel bead-3a-v1 REV-6 sibling cross-check.
#
# Confirms that the agentmind sibling repository at the path resolved by
# the gitignored go.work file (or the go.mod replace directive) actually
# compiles and passes its own `go test -short ./...` gate. Without this
# check, a reviewer reading only the mindspec diff cannot independently
# verify that the typed-sentinel + sync.Once + findBinary contract is
# satisfied in the sibling — the panel flagged this as a blocking
# concern (R2:C5, R5:C5, R6:C5).
#
# This script is intentionally narrow:
#   1. Resolve the sibling path from go.work (preferred — written by
#      scripts/checkout-agentmind.sh) or fall back to ../agentmind.
#   2. Assert the sibling has a go.mod whose module path is
#      github.com/mrmaxsteel/agentmind.
#   3. Run `go build ./...` and `go test -short ./...` inside the
#      sibling. Stream output. Fail loudly on any non-zero exit.
#
# Exit codes:
#   0 — sibling present, build green, tests green.
#   2 — sibling not found at any expected path.
#   3 — sibling found but `go build ./...` failed.
#   4 — sibling found, build OK, but `go test -short ./...` failed.

set -euo pipefail

MINDSPEC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$MINDSPEC_ROOT"

# Resolve sibling path.
SIBLING=""

# 1. Read go.work if present — it pins the absolute sibling path.
if [[ -f "go.work" ]]; then
	# Extract the second `use` line — the sibling — by parsing for a
	# path that ends in /agentmind. The mindspec line ends in the
	# mindspec worktree path; the sibling line is distinguished by
	# its /agentmind suffix.
	while IFS= read -r line; do
		# trim leading whitespace
		trimmed="${line#"${line%%[![:space:]]*}"}"
		case "$trimmed" in
			"/"*"/agentmind"|"/"*"/agentmind"/*)
				# strip trailing comment / whitespace
				candidate="${trimmed%%[[:space:]]*}"
				if [[ -d "$candidate" ]]; then
					SIBLING="$candidate"
					break
				fi
				;;
		esac
	done < go.work
fi

# 2. Fall back to ../agentmind.
if [[ -z "$SIBLING" && -d "../agentmind" ]]; then
	SIBLING="$(cd ../agentmind && pwd)"
fi

if [[ -z "$SIBLING" ]]; then
	echo "verify-sibling: agentmind sibling not found." >&2
	echo "  Looked in go.work and ../agentmind." >&2
	echo "  Run 'make checkout-agentmind' to materialize it." >&2
	exit 2
fi

# 3. Validate it's the right module.
if [[ ! -f "$SIBLING/go.mod" ]]; then
	echo "verify-sibling: $SIBLING has no go.mod" >&2
	exit 2
fi
if ! grep -q "^module github.com/mrmaxsteel/agentmind\b" "$SIBLING/go.mod"; then
	echo "verify-sibling: $SIBLING/go.mod is not github.com/mrmaxsteel/agentmind" >&2
	exit 2
fi

echo "verify-sibling: agentmind sibling at $SIBLING"
echo "verify-sibling: running 'go build ./...' in sibling..."
if ! (cd "$SIBLING" && go build ./...); then
	echo "verify-sibling: sibling 'go build ./...' FAILED" >&2
	exit 3
fi
echo "verify-sibling: sibling build OK"

echo "verify-sibling: running 'go test -short ./...' in sibling..."
if ! (cd "$SIBLING" && go test -short ./...); then
	echo "verify-sibling: sibling 'go test -short ./...' FAILED" >&2
	exit 4
fi
echo "verify-sibling: sibling tests OK"

echo "verify-sibling: PASS"
