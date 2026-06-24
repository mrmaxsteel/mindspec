---
approved_at: "2026-05-20T19:38:06Z"
approved_by: user
status: Approved
---
# Spec 085-executor-boundary: Executor boundary — route git/process I/O through executor.Executor with AST lint

## Goal

Packages `internal/{validate,approve,complete,state,phase}` perform no
direct `os/exec`, `internal/gitutil`, or raw `bd`/`git` shellouts; all
such I/O is routed through `executor.Executor` (for git/process) or
through `internal/bead` (for `bd`). An AST-based test in
`internal/lint/boundary_test.go` parses those packages and fails if any
banned identifier appears. The `executor.Executor` interface is the
declared **git/process I/O boundary** (recorded as such in its package
doc-comment), and is additively extended with three new query methods
(`ChangedFiles`, `FileAtRef`, `MergeBase`) so the enforcement packages
have a complete-enough surface to call. Two currently-known leaks are
rewired as the canonical proof that the lint gate works:
1. The git leak in `internal/validate/docsync.go` (function
   `getChangedFiles`, currently line ~49 — the call to
   `gitutil.DiffNameOnlyRef`) is rewired through the new
   `Executor.ChangedFiles` method.
2. The `bd` leak in `internal/validate/beads.go` (function
   `CheckBeadExists`, currently line ~10 — the call to
   `exec.Command("bd", "show", id, "--json")`) is rewired through
   `internal/bead` (e.g. `bead.RunBD`, the helper already in use per
   the `internal/validate/state.go:220` comment).

The transformation plan's earlier claim that "`bd` access is already
wrapped by `internal/bead` in enforcement code" was inaccurate at the
time it was written: `internal/bead` helpers do exist (per the
`state.go:220` reference), but `validate/beads.go` is the one
remaining direct-shellout holdout. Spec 085 closes that holdout as
part of F4 scope so the AST lint can land green on commit one.

## Background

F4 is the first feature in the converged transformation plan's
F4 → F2 → F1 → F3 → F5 chain because the boundary needs to be expanded
**before** F2 (doc-sync warning → gate) and F1 (ADR gates) can enforce
on top of it. The executor abstraction already exists — see
[`internal/executor/executor.go`](../../../internal/executor/executor.go)
— but a small number of enforcement-package call sites still reach
around it. The two known leaks at consensus time are:
1. `internal/validate/docsync.go` — the `getChangedFiles` helper
   (currently line ~49) calls `gitutil.DiffNameOnlyRef` directly.
2. `internal/validate/beads.go` — the `CheckBeadExists` helper
   (currently line ~10) imports `os/exec` and calls
   `exec.Command("bd", "show", id, "--json")` directly.

This spec EXPANDS the existing boundary to cover both leaks; it does
not redesign the abstraction. Per the converged plan, the scope
decision (Codex's view wins) is that `bd` reads are NOT routed through
`Executor` in this milestone: `bd` access stays behind the existing
`internal/bead` helpers (the `state.go:220` comment references
`bead.RunBD`). The single remaining `validate/beads.go` holdout is
rewired through `internal/bead` in this spec; routing future `bd`
surface through `Executor` itself is out of scope. The boundary-test
approach (Claude's view wins, Round 2) is AST-based parsing, not grep.

## Impacted Domains

- **`internal/executor/`**: `executor.go` gains three new methods on the
  `Executor` interface (`ChangedFiles`, `FileAtRef`, `MergeBase`).
  `MindspecExecutor` (in `mindspec_executor.go`) implements them
  against local git. `MockExecutor` (in `mock.go`) gains record-and-stub
  implementations so existing test fixtures continue to compile. The
  package doc-comment is updated to declare Executor as the
  **git/process I/O boundary** explicitly.
- **`internal/validate/`**: Two leaks are rewired:
  - `docsync.go`'s `getChangedFiles` helper (currently line ~49 — the
    call to `gitutil.DiffNameOnlyRef("", ref)`) is replaced by
    `exec.ChangedFiles(base, head)`. The `internal/gitutil` import is
    removed from the file.
  - `beads.go`'s `CheckBeadExists` (currently line ~10 — the call to
    `exec.Command("bd", "show", id, "--json")`) is replaced by a call
    through `internal/bead` (e.g. `bead.RunBD` or an equivalent
    helper). The `os/exec` import is removed from the file.
  Any other in-package `os/exec` / `internal/gitutil` use that the AST
  lint surfaces is rewired through `Executor` (git/process) or
  `internal/bead` (bd).
- **`internal/approve/`**: AST-walked by the lint test; any direct
  `os/exec`, `internal/gitutil`, or raw `git`/`bd` shellout the walker
  surfaces is rewired through Executor.
- **`internal/complete/`**: Same treatment as `internal/approve/`.
- **`internal/state/`**: Same treatment as `internal/approve/`.
- **`internal/phase/`**: Same treatment as `internal/approve/`.
- **`internal/lint/`** (new package): `boundary_test.go` is added,
  containing `TestEnforcementHasNoGitLeaks`, which AST-walks the five
  enforcement packages listed above and fails if any banned import or
  banned-literal call-site reappears. Seed data for the test includes
  BOTH of the now-removed leaks, identified by `file + symbol` (not
  by line number, which drifts):
  - `internal/validate/docsync.go` :: `getChangedFiles` must not
    import `internal/gitutil` and must not call
    `gitutil.DiffNameOnlyRef`.
  - `internal/validate/beads.go` :: `CheckBeadExists` must not
    import `os/exec` and must not call `exec.Command("bd", ...)`.
  Pinning by symbol means normal edits to lines above the call site
  do not invalidate the assertion.

## ADR Touchpoints

- [ADR-0030-executor-boundary.md](../../adr/ADR-0030-executor-boundary.md)
  (**new**): Records the boundary definition — the `Executor` interface
  is the **git/process I/O boundary** for enforcement packages, and
  `internal/bead` is the corresponding **bd boundary** (the one
  remaining direct `bd` shellout in `validate/beads.go` is rewired
  through it here). Documents the scope decision that the `bd` surface
  is NOT routed through `Executor` in this milestone (Codex's view
  wins per the converged plan). Records the AST-based (not grep) lint
  enforcement choice (Claude's view wins per Round 2). Records the
  five enforcement packages currently in scope
  (`internal/{validate,approve,complete,state,phase}`), the procedure
  for adding a sixth (extend the lint test's package list; re-run;
  rewire surfaced call sites; commit), and the
  `// boundary-allowlisted: <reason and reviewer>` opt-in escape
  hatch for legitimate non-git/non-bd subprocess use (see
  Requirement 11).
- No prior ADR speaks to the executor abstraction directly. A survey
  of `.mindspec/adr/` (ADR-0001 through ADR-0029) for
  `executor`/`gitutil`/`boundary` yields no decision document this
  spec contradicts or supersedes, so ADR-0030 is a clean addition.
- **ADR number reservation.** At consensus time the highest existing
  ADR is `ADR-0029-supply-chain-attestations.md`, so `ADR-0030` is
  free and is the intended number for this spec. The implementer MUST
  re-check `.mindspec/adr/` at PR-open time; if `ADR-0030` has
  been claimed by a sibling spec landing first, renaming the file and
  updating cross-references (Background, Impacted Domains, this
  section, Acceptance Criteria, and the boundary-test seed-data if it
  cites the ADR) is a **1-bead followup** under this spec, not a
  spec amendment.
- **Sequencing tie-breaker.** Per HC-5 the ordering is
  `085 → 086 → 087`. Spec 085's open beads block `mindspec next` from
  surfacing spec 086 work via the existing spec sequencing — no
  separate ADR rule is needed. Two ADRs with the
  `-executor-boundary.md` suffix MUST NOT coexist; if a renumber is
  required, the prior file is moved, not copied.

## Requirements

**Hard constraints (from the converged transformation plan):**

1. **HC-1 Solo-developer UX preserved.** No new flags, no new daemons,
   no new commands required for everyday `mindspec` use.
2. **HC-2 Standalone CLI.** No extra long-lived processes are
   introduced.
3. **HC-3 Existing test suite preserved.** All existing tests (the
   full `go test -short ./...` suite) MUST still pass on the F4
   branch with **no skipped or excluded tests vs. `main`**. New tests
   (`TestEnforcementHasNoGitLeaks`, mock-fixture updates) are
   additive. The literal pre-F4 count (~794 at draft time) is
   informational only and not an AC — counts drift with normal work.
4. **HC-4 viz / agentmind / bench excluded.** Those subsystems have
   been removed per specs 083/084; nothing in F4 touches them.
5. **HC-5 F4 must merge before F2/F1.** The ordering chain is
   **085 → 086 → 087** (F4 → F2 → F1). F2 (doc-sync gate) and F1
   (ADR gate) build on the expanded `Executor` surface delivered here
   and cannot land until F4 is on `main`.
6. **HC-6 Each commit `go build ./... && go test -short ./...` green.**
   Every commit on the F4 branch — including the interface-extension
   commit, the `MockExecutor` stub commit, the `docsync.go`
   `getChangedFiles` rewire commit, the `beads.go` `CheckBeadExists`
   rewire commit, and the boundary-test commit — MUST build and pass
   `-short` tests. The canonical per-commit proof is CI's per-commit
   gate; the illustrative shell snippet in **Validation Proofs**
   below uses `git worktree add` so the working tree is not mutated
   across iterations.
7. **HC-7 Boundary lint is AST-based, not grep.** The lint test parses
   each package via `go/parser` + `go/ast` and walks
   `ast.ImportSpec` / `ast.CallExpr` nodes; substring matching over
   raw source is explicitly rejected.

**Spec-specific requirements:**

8. **Extend `executor.Executor` with three new methods**, additively
   and with these exact signatures:
   - `ChangedFiles(base, head string) ([]string, error)` — replaces
     `gitutil.DiffNameOnlyRef` (and the direct
     `exec.Command("git","diff",...)` it wraps) in `docsync.go`'s
     `getChangedFiles`. Doc-comment MUST pin the empty-string-base
     semantics: passing `base == ""` means "working tree vs `head`",
     matching the current `gitutil.DiffNameOnlyRef("", ref)` call
     site, so `MindspecExecutor` and `MockExecutor` agree.
   - `FileAtRef(ref, path string) ([]byte, error)` — needed for F1 to
     read past versions of ADRs/specs.
   - `MergeBase(a, b string) (string, error)` — needed for F1/F2 diff
     scoping.
9. **Update `MockExecutor` and all external fixtures that compose
   it** to provide stub implementations of the three new methods, so
   existing tests across the tree continue to compile and pass under
   the extended interface. The complete enumerated inventory at spec
   time (verified by `grep -RnE 'MockExecutor' internal/`) is:
   - `internal/executor/mock.go` — the `MockExecutor` type itself.
     Gains the three new methods plus matching `ChangedFilesCalls`,
     `FileAtRefCalls`, `MergeBaseCalls` recording slices and
     optional `*Err` and `*Result` stub-return fields, consistent
     with the existing `CommitAllErr` / `IsTreeCleanErr` pattern
     visible in `internal/harness/lifecycle_scenario_test.go:560`
     and `:594`.
   - `internal/next/next_test.go` (constructs `&executor.MockExecutor{}`
     at lines 428, 457, 474).
   - `internal/complete/complete_test.go` (`newMockExec` helper at
     line 47).
   - `internal/spec/create_test.go` (`newMockExecutor` helper at
     line 26).
   - `internal/harness/lifecycle_scenario_test.go` (multiple
     `&executor.MockExecutor{}` literals).
   - `internal/approve/impl_test.go` (multiple `&executor.MockExecutor{}`
     literals).
   The implementer MUST re-run the same grep at PR-open time and
   reconcile any drift. Embedding-based fixtures (if any) auto-
   satisfy the extended interface; literal struct constructions do
   not need stub fields unless tests assert on the new methods.
10. **Replace the git leak in `internal/validate/docsync.go`'s
    `getChangedFiles` helper** (currently line ~49 — the call to
    `gitutil.DiffNameOnlyRef("", ref)`) with
    `exec.ChangedFiles(base, head)`. An `executor.Executor` is
    threaded through the call chain; no new global singleton is
    introduced. The complete caller audit at spec time:
    - `internal/validate/docsync.go:12` —
      `ValidateDocs(root, diffRef string) *Result` is the entry
      point.
    - `cmd/mindspec/validate.go:68` — the only non-test production
      caller of `ValidateDocs`.
    The implementer MAY EITHER (a) change `ValidateDocs`'s signature
    in the same commit and update both call sites, OR (b) add a
    separate Executor-aware entry point (e.g.
    `ValidateDocsWithExecutor(root, diffRef string, exec executor.Executor)`)
    that wraps the existing function, leaving the legacy signature
    in place for downstream consumers. The audit must be reproduced
    in the PR description; if any caller surfaces that is not in
    this list, the spec body is amended in a 1-bead followup before
    the rewire commit lands.

10a. **Replace the `bd` leak in `internal/validate/beads.go`'s
    `CheckBeadExists`** (currently line ~10 — the call to
    `exec.Command("bd", "show", id, "--json")`) with a call through
    `internal/bead` (the `bead.RunBD` helper or an equivalent
    package-level function; the `internal/validate/state.go:220`
    comment confirms the helper exists). The `os/exec` import is
    removed from `internal/validate/beads.go` in the same commit so
    the import-ban half of Requirement 11 stays green.
11. **Add `internal/lint/boundary_test.go`** containing
    `TestEnforcementHasNoGitLeaks`. The test:
    - Uses `go/parser.ParseDir` to load each of
      `internal/validate`, `internal/approve`, `internal/complete`,
      `internal/state`, `internal/phase`.
    - **Primary gate — import ban (load-bearing).** Walks each
      file's `ast.ImportSpec` nodes and FAILS if any import path
      matches `os/exec` or
      `github.com/mrmaxsteel/mindspec/internal/gitutil`. This is
      the gate that actually prevents bypass: a variable-bound or
      runtime-computed first argument can slip past a literal
      walker, but cannot import `os/exec` without tripping this
      check.
    - **Secondary gate — literal call-site ban (belt-and-braces).**
      Walks each file's `ast.CallExpr` nodes and FAILS if any
      selector call resolves to a raw `git`/`bd` shellout where
      the first argument is a string literal or a folded constant:
      `exec.Command("git", ...)`, `exec.Command("bd", ...)`,
      `exec.CommandContext(_, "git", ...)`,
      `exec.CommandContext(_, "bd", ...)`. Constant-bound forms
      (`const cmd = "git"; exec.Command(cmd, ...)`) are caught via
      a one-level constant-folding pass over the file's
      `*ast.GenDecl` constants. Variable-bound or computed forms
      are NOT caught by the literal walker — by design; the
      import ban is what closes that hole.
    - **Allowlist mechanism (opt-in, reviewer-gated).** If a future
      file in one of the five enforcement packages has a
      legitimate non-git/non-bd need to import `os/exec` (e.g. to
      shell out to an editor or `gofmt`), it MAY carry the exact
      leading file-level comment
      `// boundary-allowlisted: <reason and reviewer-id>` on the
      first non-blank line after the package clause. The AST
      walker recognises this exact marker (case-sensitive,
      whitespace-trimmed) and skips both the import-ban and
      literal-call-site checks for that file. The allowlist is
      opt-in and per-file; the default is no exemption.
    - **Seed-data for the two removed leaks (pinned by symbol,
      not by line).** A `sub-test` table entry asserts that the
      walker FAILS on a synthetic input mirroring the now-removed
      leaks, pinned by `file + symbol` so normal edits do not
      invalidate the assertion:
      - `internal/validate/docsync.go` :: `getChangedFiles` must
        not import `internal/gitutil` and must not call
        `gitutil.DiffNameOnlyRef`.
      - `internal/validate/beads.go` :: `CheckBeadExists` must not
        import `os/exec` and must not call `exec.Command("bd", ...)`.
      The seed entries are exercised by parsing a fixture file
      shipped alongside `boundary_test.go` (e.g.
      `internal/lint/testdata/seed_docsync_leak.go.txt` and
      `internal/lint/testdata/seed_beads_leak.go.txt`) and
      asserting `TestEnforcementHasNoGitLeaks`'s walker returns
      the expected failure — proof of the proof.
12. **Declare both boundaries in package doc-comments.** The package
    comment in `internal/executor/executor.go` is updated to state
    explicitly that `Executor` is the **git/process I/O boundary**
    for the enforcement packages and that `bd` access is *out of
    scope* for this boundary (it lives behind `internal/bead`). The
    package comment in `internal/bead/` is updated (or, if absent,
    added) to state that `internal/bead` is the **bd boundary** for
    enforcement packages — direct `exec.Command("bd", ...)` calls
    from any of the five lint-scoped packages are prohibited and
    must route through this package.

## Scope

### In Scope

- The three new `Executor` interface methods (`ChangedFiles`,
  `FileAtRef`, `MergeBase`) with the exact signatures in
  Requirement 8.
- `MindspecExecutor` implementations of the three new methods,
  shelling to local `git` (this is the *only* package permitted to
  invoke raw `git` per the boundary doctrine).
- `MockExecutor` stubs of the three new methods, with `CallsTo`
  recording so existing test fixtures keep working.
- Rewiring `internal/validate/docsync.go`'s `getChangedFiles`
  (currently line ~49) to call `exec.ChangedFiles(base, head)`
  instead of `gitutil.DiffNameOnlyRef(...)`, and dropping the
  `internal/gitutil` import from the file.
- Rewiring `internal/validate/beads.go`'s `CheckBeadExists`
  (currently line ~10) to call through `internal/bead`
  (`bead.RunBD` or equivalent) instead of
  `exec.Command("bd", "show", id, "--json")`, and dropping the
  `os/exec` import from the file.
- `internal/lint/boundary_test.go` — new package and new test, the
  AST-based gate described in Requirement 11.
- Updating the package doc-comment in
  `internal/executor/executor.go` to declare the
  git/process I/O boundary explicitly (Requirement 12).
- Threading an `executor.Executor` value through the
  `validate.ValidateDocs` / `validate.getChangedFiles` call chain so
  the rewire in Requirement 10 has something to call. The complete
  caller audit at spec time is in Requirement 10. Where this changes
  a call-site signature in the same five enforcement packages, that
  change is in scope; updating `cmd/mindspec/validate.go:68` to
  construct the live `MindspecExecutor` is also in scope (and
  `cmd/mindspec` is NOT one of the five lint-scoped packages, so the
  wiring there is unaffected by the boundary lint).

### Out of Scope

- **Routing `bd` reads through `Executor`.** Explicitly out per the
  converged plan: `bd` access stays behind `internal/bead`. This
  spec rewires the one remaining direct `bd` shellout
  (`validate/beads.go`'s `CheckBeadExists`) through `internal/bead`
  to satisfy the AST lint, but does NOT add `Executor.BD*` methods
  or route the `bd` surface through the `Executor` interface — that
  is a future spec's concern.
- **Other enforcement packages** beyond the five named
  (`internal/{validate,approve,complete,state,phase}`). If a sixth
  package later wants enforcement coverage, the procedure is in
  ADR-0030: extend the lint test's package list and rewire
  surfaced call sites in a follow-up spec.
- **Any change to call sites that aren't in the enforcement
  packages.** Tooling that legitimately needs to invoke `git` or
  `bd` directly (e.g., `cmd/mindspec`, `internal/bead`,
  `internal/executor/mindspec_executor.go` itself) is unaffected.
- **F2 doc-sync gating logic** (warning → error promotion). F4
  delivers the boundary surface F2 will build on; the gate semantics
  live in spec 086.
- **F1 ADR-gate logic.** F4 delivers `FileAtRef` and `MergeBase`
  because F1 needs them, but the gate itself lives in spec 087.

## Non-Goals

- This spec does not introduce a new linter binary, a `golangci-lint`
  plugin, or any non-`go test` enforcement vehicle. The boundary lint
  rides inside the existing `go test ./...` invocation.
- This spec does not change `executor.Executor`'s existing methods
  (`InitSpecWorkspace`, `HandoffEpic`, `DispatchBead`, `CompleteBead`,
  `FinalizeEpic`, `Cleanup`, `IsTreeClean`, `DiffStat`, `CommitCount`,
  `CommitAll`). The interface extension is strictly additive.
- This spec does not refactor the `internal/gitutil` package. Callers
  inside the five enforcement packages stop importing it; the package
  itself remains in place for use by `internal/executor/mindspec_executor.go`
  and any non-enforcement consumer.

## Acceptance Criteria

- [ ] `TestEnforcementHasNoGitLeaks` exists at
  `internal/lint/boundary_test.go` and passes. The test AST-walks
  `internal/{validate,approve,complete,state,phase}` and FAILS if any
  banned import (`os/exec`,
  `github.com/mrmaxsteel/mindspec/internal/gitutil`) or banned-literal
  call-site (`exec.Command("git", ...)`, `exec.Command("bd", ...)`,
  and their `CommandContext` equivalents — including one level of
  constant-folding) reappears. The import ban is the primary gate;
  the literal call-site walk is belt-and-braces. The
  `// boundary-allowlisted: <reason and reviewer-id>` opt-in escape
  hatch is recognised per Requirement 11.
- [ ] The pre-spec git leak in
  `internal/validate/docsync.go`'s `getChangedFiles` (the call to
  `gitutil.DiffNameOnlyRef`) is removed. Pinned by file + symbol so
  normal edits do not invalidate the assertion; the boundary test's
  seed-data fixture exercises the walker against a synthetic version
  of this leak.
- [ ] The pre-spec `bd` leak in
  `internal/validate/beads.go`'s `CheckBeadExists` (the call to
  `exec.Command("bd", "show", id, "--json")`) is replaced by a call
  through `internal/bead` (`bead.RunBD` or equivalent), and the
  `os/exec` import is removed from `internal/validate/beads.go`.
  Pinned by file + symbol; the boundary test's seed-data fixture
  exercises the walker against a synthetic version of this leak.
- [ ] `internal/executor/executor.go` declares the three new methods
  with the exact signatures `ChangedFiles(base, head string) ([]string, error)`,
  `FileAtRef(ref, path string) ([]byte, error)`, and
  `MergeBase(a, b string) (string, error)` — verifiable via `grep` and
  via the AST check inside the boundary test.
- [ ] `internal/executor/mock.go` implements the three new methods on
  `MockExecutor` with record-and-stub behaviour matching the existing
  pattern: `ChangedFilesCalls`, `FileAtRefCalls`, `MergeBaseCalls`
  recording slices plus optional `*Err` / `*Result` stub-return
  fields, consistent with `CommitAllErr` / `IsTreeCleanErr` already
  present. All enumerated external fixtures in Requirement 9 compile
  unchanged or with the minimal stub additions required.
- [ ] The full existing test suite passes on the F4 branch (`go test
  -short ./...` returns zero with no skipped or excluded tests vs
  `main`). The only test additions are
  `TestEnforcementHasNoGitLeaks`, its seed-fixture files under
  `internal/lint/testdata/`, and any `MockExecutor`-fixture helpers
  required for compilation.
- [ ] `go build ./... && go test -short ./...` is green on **every
  commit** of the F4 branch — interface extension, mock stubs,
  `docsync.go` `getChangedFiles` rewire, `beads.go` `CheckBeadExists`
  rewire, and boundary-test addition each commit independently
  green. The canonical proof is CI's per-commit gate; the
  illustrative shell snippet in Validation Proofs uses
  `git worktree add` so the local working tree is not mutated.
- [ ] Exactly one ADR file matching
  `.mindspec/adr/ADR-*-executor-boundary.md` exists, named
  `ADR-0030-executor-boundary.md` at consensus time (highest existing
  ADR was `ADR-0029-supply-chain-attestations.md`). If `0030` was
  claimed by a sibling spec landing first, the file is renamed to
  the next free integer and all cross-references in this spec are
  updated as a 1-bead followup before merge. The ADR records: the
  git/process-I/O boundary definition, the bd-stays-behind-
  `internal/bead` scope decision, the AST-based (not grep) lint
  choice, the current list of five enforcement packages, the
  procedure for adding a sixth, and the
  `// boundary-allowlisted: <reason and reviewer-id>` opt-in escape
  hatch.

## Validation Proofs

A reviewer can establish each acceptance criterion via concrete
commands run from the repo root.

- **Boundary test exists and passes:**
  ```
  test -f internal/lint/boundary_test.go \
    && go test -run TestEnforcementHasNoGitLeaks ./internal/lint/...
  ```
- **No banned imports remain in any enforcement package
  (sanity grep — the AST test is authoritative):**
  ```
  # Should produce NO output. Any line printed is a violation
  # that the AST test would also catch.
  grep -RnE '"os/exec"|"github\.com/mrmaxsteel/mindspec/internal/gitutil"' \
    internal/validate internal/approve internal/complete \
    internal/state internal/phase
  ```
- **No raw `git`/`bd` shellout literals remain in the enforcement
  packages:**
  ```
  # Should produce NO output.
  grep -RnE 'exec\.Command(Context)?\([^,]*,\s*"(git|bd)"' \
    internal/validate internal/approve internal/complete \
    internal/state internal/phase
  ```
- **The `docsync.go` `getChangedFiles` rewire landed:**
  ```
  grep -nE 'ChangedFiles\(' internal/validate/docsync.go
  # Should print the new exec.ChangedFiles call inside getChangedFiles.
  ! grep -nE 'gitutil\.DiffNameOnlyRef' internal/validate/docsync.go
  # gitutil call should be gone from the file.
  ! grep -nE '"github\.com/mrmaxsteel/mindspec/internal/gitutil"' internal/validate/docsync.go
  # gitutil import should be gone from the file.
  ```
- **The `beads.go` `CheckBeadExists` rewire landed:**
  ```
  grep -nE 'bead\.RunBD|bead\.[A-Z]' internal/validate/beads.go
  # Should print the new internal/bead call inside CheckBeadExists.
  ! grep -nE 'exec\.Command\(' internal/validate/beads.go
  # Direct exec.Command should be gone from the file.
  ! grep -nE '"os/exec"' internal/validate/beads.go
  # os/exec import should be gone from the file.
  ```
- **`Executor` interface has the three new methods with exact
  signatures:**
  ```
  grep -nE 'ChangedFiles\(base, head string\) \(\[\]string, error\)' \
    internal/executor/executor.go
  grep -nE 'FileAtRef\(ref, path string\) \(\[\]byte, error\)' \
    internal/executor/executor.go
  grep -nE 'MergeBase\(a, b string\) \(string, error\)' \
    internal/executor/executor.go
  ```
- **`MockExecutor` implements all three:**
  ```
  grep -nE 'func \(m \*MockExecutor\) ChangedFiles\(' \
    internal/executor/mock.go
  grep -nE 'func \(m \*MockExecutor\) FileAtRef\('   \
    internal/executor/mock.go
  grep -nE 'func \(m \*MockExecutor\) MergeBase\('   \
    internal/executor/mock.go
  ```
- **Test suite preserved (no skipped or removed tests vs. main):**
  ```
  # On main:
  go test -short ./... 2>&1 | tee /tmp/main-tests.log
  # On F4 branch:
  go test -short ./... 2>&1 | tee /tmp/f4-tests.log
  # Suite must exit 0 on both; any tests present on main but missing
  # from the F4 run are HC-3 violations. The literal test count drifts
  # with normal work and is informational only.
  ```
- **Per-commit green discipline (worktree-isolated; CI is canonical):**
  ```
  # Canonical: the per-commit CI gate enforces this. Illustrative
  # local proof using git worktree so the working tree is not mutated:
  tmp=$(mktemp -d)
  for sha in $(git rev-list main..HEAD | tac); do
    git worktree add -d "$tmp/wt" "$sha"
    ( cd "$tmp/wt" && go build ./... && go test -short ./... ) \
      || { echo "FAIL at $sha"; git worktree remove --force "$tmp/wt"; exit 1; }
    git worktree remove "$tmp/wt"
  done
  rmdir "$tmp"
  ```
- **ADR-0030 (or next free integer) present, and exactly one such ADR:**
  ```
  ls .mindspec/adr/ADR-*-executor-boundary.md
  # Output must list exactly one file. Two ADRs sharing the
  # -executor-boundary suffix is a violation (see ADR Touchpoints
  # tie-breaker: prior file is moved, not copied, on renumber).
  test "$(ls .mindspec/adr/ADR-*-executor-boundary.md | wc -l)" -eq 1
  ```

## Risks

- **None material** per the converged plan. The interface extension is
  additive — existing executor implementations get three new methods,
  and `MockExecutor` gets matching stubs. No behavioural change is
  introduced beyond the two rewires (`docsync.go`'s `getChangedFiles`
  and `beads.go`'s `CheckBeadExists`), each of which preserves the
  same observable semantics — `git diff --name-only` against a ref
  for the former, `bd show <id> --json` exit-status semantics for the
  latter.
- **Threading risk (minor).** The `validate.ValidateDocs` entry point
  currently takes `(root, diffRef string)`. Mitigation: per
  Requirement 10, the implementer either extends the signature in
  the same commit (updating both audited callers —
  `internal/validate/docsync.go:12` and `cmd/mindspec/validate.go:68`)
  or adds an Executor-aware constructor alongside. No new global
  singleton is introduced. `cmd/mindspec` is not in the five
  lint-scoped packages, so the live-MindspecExecutor wiring there is
  unaffected by the boundary lint.
- **Mock-fixture drift (minor).** Requirement 9 enumerates every
  external file that constructs a `MockExecutor` literal; mitigation
  is the enumerated audit plus HC-6's per-commit green discipline,
  which surfaces any missed fixture immediately.
- **`bd` rewire surface (minor).** `internal/bead` must expose a
  helper (`RunBD` per the `state.go:220` comment, or equivalent)
  that `CheckBeadExists` can call without changing its observable
  return contract (`bool, error`). If the helper does not yet exist
  in the form needed, adding it is a 1-commit prerequisite within
  this spec's branch, ordered before the `beads.go` rewire commit so
  HC-6 stays green.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-20
- **Notes**: Approved via mindspec approve spec