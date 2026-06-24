# ADR-0030: Executor as the Git/Process I/O Boundary for Enforcement Packages

- **Date**: 2026-05-20
- **Status**: Accepted
- **Domain(s)**: execution, validation, lifecycle, lint
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) (references `internal/executor` as the commit-mediation seam)

---

## Status

Finalized in spec 085 Bead 4 alongside the AST boundary lint at
`internal/lint/boundary_test.go`. The two historical leak sites —
`internal/validate/docsync.go`'s `getChangedFiles` and
`internal/validate/beads.go`'s `CheckBeadExists` — were rewired in
Beads 2 and 3 respectively.

## Context

The enforcement packages (`internal/{validate,approve,complete,state,phase}`)
historically shell out to git and bd directly. Two known leaks remain on
`main`: `internal/validate/docsync.go` (function `getChangedFiles`, ~line 49 —
uses `exec.Command("git","diff", ...)`) and `internal/validate/beads.go:10`
(`exec.Command("bd","show", ...)`). These leaks defeat Executor-based
mocking in tests and make the enforcement code path inconsistent with the
rest of mindspec, which routes mutating git operations through
`internal/executor`.

F4 of the converged transformation plan extends the existing
`internal/executor.Executor` interface with three new methods
(`ChangedFiles`, `FileAtRef`, `MergeBase`), rewires `docsync.go` to use
them, refactors `beads.go` to use `internal/bead.RunBD`, and installs an
AST-based lint that prevents regression.

## Decision

Three sub-decisions:

1. **Boundary scope: git AND process I/O via Executor; bd via
   `internal/bead`.** Enforcement packages may not import `os/exec` or
   `internal/gitutil`. They route git through Executor and bd through
   `internal/bead` (which itself may shell out, but acts as the single
   point of mediation per the transformation plan's "bd stays wrapped"
   stance). Rejected alternatives: routing bd through Executor (rejected
   per plan — would expand Executor surface unnecessarily); allowing
   direct `exec.Command` for "trivial" cases (rejected — defeats the
   lint).

2. **AST-based boundary lint at `internal/lint/boundary_test.go`.**
   Primary gate: `os/exec` / `internal/gitutil` import bans on
   enforcement packages. Secondary gate: literal-walker that catches
   `exec.Command("git", …)` / `exec.Command("bd", …)` style call sites
   (with constant-folding to catch `const cmd = "git"` cases). Rejected:
   grep-based check (per plan: "AST-based, not grep").

3. **Opt-in escape hatch: leading comment
   `// boundary-allowlisted: <reason and reviewer-id>`.** The lint
   recognizes the marker and exempts the file; reviewer-gated, not
   default. Rejected: no escape hatch at all (too rigid for legitimate
   future needs).

## Consequences

- (+) Future enforcement code cannot regress to direct shellouts without
  explicit opt-in.
- (+) Executor mocking covers more of the lifecycle code.
- (+) The `docsync.go` and `beads.go` leaks are mechanically asserted
  removed.
- (−) Executor interface gains 3 methods (additive only; no breaking
  changes).
- (−) Any future enforcement-package contributor must understand the
  boundary.
- (−) The lint adds ~50ms to `go test -short ./...`.

## Procedure: Adding a new enforcement package

To extend the boundary gate to a new package (e.g., `internal/foo`):

1. **Update the lint test:** add the package path to the
   `enforcementPkgs` slice in `internal/lint/boundary_test.go`.
2. **Verify clean state:** confirm the package's non-test `*.go` files
   do not import `"os/exec"` or `"github.com/mrmaxsteel/mindspec/internal/gitutil"`.
   If any do, either rewire them through `internal/executor` / `internal/bead`
   (preferred) or add a `// boundary-allowlisted: <reason and reviewer-id>`
   marker to the file's leading doc comment (escape hatch — opt-in per
   ADR-0030).
3. **Declare the boundary:** add a package doc comment declaring the
   import bans and naming the boundary destinations (mirror the comment
   already on `internal/validate`).
4. **Re-run the lint:** `go test ./internal/lint/...` must pass; the
   new package is now under the gate.

## Rollback

Revert spec 085 PR's merge commit in a single git command
(`git revert -m 1 <merge-sha>`); the new Executor methods are additive
and removing them is a no-op for non-enforcement callers. Existing call
sites in `docsync.go` and `beads.go` would revert to their pre-spec
shellouts.

## Related

- [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) — establishes
  `internal/executor` as the commit-mediation seam this ADR extends.
