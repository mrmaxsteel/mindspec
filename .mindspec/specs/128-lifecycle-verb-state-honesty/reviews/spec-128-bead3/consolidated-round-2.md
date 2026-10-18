# spec-128-bead3 — consolidated concrete_changes_required, round 2

Tally: 5 APPROVE (R1, R2, R3, R5, R6-slot hard block CLEARED by own re-derivation) / 1 REQUEST_CHANGES (R6 ×1 NEW minor). Round-2 R4/R5/R6 slots ran as claude-subs (codex capacity outage persisted through spaced retries). One group.

## G1 (R6, NEW minor, verified repro + verified fix) — Fully-qualified ref in the tree probe — MUST FIX
resolve.go:261's tree-probe passes the SHORT branch name to FileExistsAtRef; git's ambiguous-short-name resolution prefers refs/tags, so a same-named tag (`git tag spec/132-branchy main`, fetchable into any clone) shadows the branch and the draft silently reverts to {"mode":"idle"} exit 0 — defeating the primitive's documented precondition. Fix: probe `refs/heads/<branch>:<rel>` (the exact ref BranchExistsIn confirmed). One test: branch-only draft + same-named shadow tag → still discovered.

## Carried (recorded for bead 5's prompt, again): run.go's two disclaimed windows + derive.go's fabricating fallbacks — bead 5 scope extension in writing.
