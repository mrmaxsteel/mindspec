---
adr_citations:
    - id: ADR-0030
approved_at: "2026-05-20T19:55:03Z"
approved_by: user
bead_ids:
    - mindspec-c9e8.1
    - mindspec-c9e8.2
    - mindspec-c9e8.3
    - mindspec-c9e8.4
spec_id: 085-executor-boundary
status: Approved
version: "1"
---
# Plan: 085-executor-boundary

## ADR Fitness

- **ADR-0030** (new — "Executor as the Git/Process I/O Boundary for
  Enforcement Packages"): authored at spec-draft time with
  `Status: Accepted` in the frontmatter (verified at plan-draft time:
  the file at `.mindspec/docs/adr/ADR-0030-executor-boundary.md`
  already has `Status: Accepted` on line 4). Records the three
  sub-decisions the spec turns on: (1) git/process I/O via
  `executor.Executor`, bd via `internal/bead` (the
  one-bd-shellout-in-`validate/beads.go` holdout is rewired through
  `internal/bead.BeadExists` here — a wrapper around `RunBD` added in
  Bead 3's prerequisite commit — NOT routed through `Executor` —
  Codex's view wins per the converged plan); (2) AST-based lint at
  `internal/lint/boundary_test.go`, not grep (Claude's view wins per
  Round 2); (3) opt-in
  `// boundary-allowlisted: <reason and reviewer-id>` escape hatch per
  Requirement 11. The ADR's narrative "Status" paragraph (line 15)
  still reads "Stub created during spec 085 drafting" — Bead 4
  step 8 updates that paragraph to read "Finalized in spec 085 Bead 4
  alongside the AST boundary lint" so the narrative matches the
  frontmatter Status. The Decision section already records the final
  list of five enforcement packages
  (`internal/{validate,approve,complete,state,phase}`); Bead 4
  step 8 also adds the procedure for adding a sixth if not already
  present.
- **ADR-0025** (JSONL as build artifact — referenced as Related by
  ADR-0030): carried forward unchanged. ADR-0025 established
  `internal/executor` as the commit-mediation seam; ADR-0030 extends
  that seam additively (three new query methods, zero behavioural
  changes to existing methods). No contradiction.
- The spec.md's "ADR Touchpoints" section explicitly surveys
  ADR-0001 through ADR-0029 and confirms no prior ADR speaks to the
  executor abstraction directly. This plan does not contradict any
  accepted ADR — the change is strictly additive (three new interface
  methods + a new lint test + two leak-rewires that preserve observable
  semantics).
- **ADR number reservation.** At plan-draft time the highest existing
  ADR is `ADR-0029-supply-chain-attestations.md`, so `ADR-0030` is
  free. Bead 4 re-checks `.mindspec/docs/adr/` at PR-open time and
  renumbers as a 1-bead followup under this spec if a sibling spec
  claimed `0030` first (per spec lines 126-134). The
  `-executor-boundary.md` suffix is unique by construction — two ADRs
  sharing it MUST NOT coexist.

## Testing Strategy

This spec's failure mode is **silent regression**: a future
enforcement-package edit re-introduces `os/exec` or `gitutil` and
defeats the boundary. The defense is the
**`TestEnforcementHasNoGitLeaks`** AST test in Bead 4 — it is the
lifetime invariant the rest of the work exists to enable. Beads 1-3
remove the two known leaks and expand the `Executor` surface so the
lint can land green on commit one; Bead 4 lands the lint itself plus
the two seed-data fixtures that prove the walker fails on synthetic
versions of the historical leaks.

**Bead ordering note.** Bead 1 (interface extension + MockExecutor
stubs) lands FIRST so Beads 2-3 have something to call. Bead 2
(`docsync.go` rewire) and Bead 3 (`beads.go` rewire) are independent
of each other and depend only on Bead 1 — they can land in either
order. Bead 4 (the lint + the doc-comment boundary declarations)
lands LAST because both leaks must be removed before the AST test can
pass against the live tree (the seed-data assertions run against
fixture files under `internal/lint/testdata/`, not the live source).

Per HC-6 the per-commit gate is CI; locally the worktree-isolated
loop from spec.md's Validation Proofs is the canonical illustration.
Each bead's verification block ends with
`go build ./... && go test -short ./...` passing on every commit.

The test additions across the four beads are:

- **Bead 1**: extends `internal/executor/executor_test.go` (if absent
  for the new methods, table tests are added in-package) plus the
  `MockExecutor` stub additions in `internal/executor/mock.go`. The
  external fixtures enumerated in spec Requirement 9 are touched
  ONLY if they break — pure literal `&executor.MockExecutor{}`
  constructions auto-compile against an extended interface; only
  fixtures that assert on the new methods need stub-field additions.
- **Bead 2**: existing `internal/validate/docsync_test.go` continues
  to pass. Verified at plan-draft time by enumerating the file's
  10 test functions: NONE of them call `ValidateDocs` directly —
  they all exercise lower-level helpers (`ClassifyChanges`,
  `ParseChangedFiles`, `CheckInternalPackages`, `CheckCmdChanges`)
  that take pre-classified inputs and do not touch the git seam.
  So the signature change to `ValidateDocs(root, diffRef string,
  exec executor.Executor)` requires NO updates to
  `docsync_test.go`. The sole non-test production caller
  (`cmd/mindspec/validate.go:68`, per spec Requirement 10) is
  updated to construct a live `MindspecExecutor` via
  `executor.NewMindspecExecutor(root)` (constructor signature
  verified at `internal/executor/mindspec_executor.go:53`).
- **Bead 3**: existing tests against `CheckBeadExists` (if any in
  `internal/validate/validate_test.go` or
  `internal/validate/state_test.go`) continue to pass; the call still
  returns `(bool, error)` with the same semantics. The rewire is
  internal-only — caller contract is preserved.
- **Bead 4**: NEW package `internal/lint/`, NEW test
  `TestEnforcementHasNoGitLeaks` at `internal/lint/boundary_test.go`,
  NEW fixture directory `internal/lint/testdata/` containing
  `seed_docsync_leak.go.txt` and `seed_beads_leak.go.txt`. The
  fixtures are `.go.txt` (not `.go`) so they are not compiled by
  `go build ./...` — they are parsed by `go/parser.ParseFile` from
  the test. The test runs in every `go test -short ./...` invocation
  with no build tags and no `t.Skip` paths so its first appearance
  is its permanent enforced state (mirroring spec 084's specgate
  pattern from `internal/specgate/`).

**Mutation-style proof.** The seed-data fixtures ARE the mutation
proof: `seed_docsync_leak.go.txt` is a synthetic file containing the
historical `getChangedFiles` body that imports `internal/gitutil` and
calls `gitutil.DiffNameOnlyRef`. The test asserts that the walker,
run against this fixture, returns the expected failure (i.e. flags
both the import and the call site). Same for the
`seed_beads_leak.go.txt` fixture (synthetic `CheckBeadExists` body
with `os/exec` import and `exec.Command("bd", ...)` call). If the
walker ever stops flagging these fixtures, the test fails — proof of
the proof per spec Requirement 11.

**Gate division of labour.** `internal/gitutil` function calls (e.g.
`gitutil.DiffNameOnlyRef`, `gitutil.CurrentBranch`, etc.) are caught
by the IMPORT-BAN gate alone: any file in the five enforcement
packages that imports `internal/gitutil` fails the test, regardless
of what symbols it then calls. The literal-walker secondary gate is
defense-in-depth for `os/exec.Command` / `exec.CommandContext`
first-argument literals only — it does NOT need to enumerate every
gitutil-exported symbol. This division means the literal walker
stays small and the lint's primary correctness claim rests on the
import-ban check.

Hard Constraint HC-6 ("every commit `go build ./... && go test -short
./...` green") is enforced per-bead: each bead's verification block
ends with that exact command pair passing on every commit it
produces. HC-3 ("no skipped or excluded tests vs. main") is enforced
by reading the test-name list from both branches and diffing — Bead 4
records the diff in its commit message.

## Bead 1: Extend `executor.Executor` with three new methods and the MockExecutor stubs

Lands the additive interface extension that Beads 2-3 will call into.
Three new methods on `executor.Executor` with the exact signatures
from spec Requirement 8. The production implementation in
`MindspecExecutor` (file `internal/executor/mindspec_executor.go`)
wraps the existing `gitutil` helpers / direct git invocations; the
production executor IS one of the allowed `os/exec` consumers per
the boundary doctrine, so its implementations may shell out to local
git directly. `MockExecutor` gains matching record-and-stub
implementations consistent with the existing
`CommitAllErr`/`IsTreeCleanErr`/`DiffStatResult`+`DiffStatErr` pattern
visible in `internal/executor/mock.go:21-26`.

**Steps**

1. Edit `internal/executor/executor.go` to add three new methods to
   the `Executor` interface with these exact signatures (Requirement
   8):

   ```
   ChangedFiles(base, head string) ([]string, error)
   FileAtRef(ref, path string) ([]byte, error)
   MergeBase(a, b string) (string, error)
   ```

   Doc-comments for each method MUST pin semantics:
   - `ChangedFiles`: doc-comment states "passing `base == ""` means
     working tree vs `head`, matching `gitutil.DiffNameOnlyRef("",
     ref)` semantics" (spec line 184).
   - `FileAtRef`: doc-comment states "returns the content of `path`
     at git `ref` (wraps `git show ref:path`); empty `ref` is
     undefined".
   - `MergeBase`: doc-comment states "returns the merge-base SHA of
     refs `a` and `b` (wraps `git merge-base a b`)".

2. In the SAME commit, update the **package** doc-comment at the top
   of `internal/executor/executor.go` to declare the boundary
   explicitly (Requirement 12): "`Executor` is the **git/process I/O
   boundary** for the enforcement packages
   (`internal/{validate,approve,complete,state,phase}`). `bd` access
   is OUT OF SCOPE for this boundary and lives behind
   `internal/bead`." Replace or augment the existing doc-comment
   (currently lines 1-7) — do not delete the existing "enforcement
   packages call Executor methods; they never perform git or
   workspace operations directly" line; just extend it with the
   explicit boundary-doctrine paragraph.

3. Add the production implementations in
   `internal/executor/mindspec_executor.go`:
   - `ChangedFiles(base, head string) ([]string, error)`: for the
     `base == ""` case, calls `gitutil.DiffNameOnlyRef("", head)`
     verbatim (preserves the exact semantics the
     `docsync.go:getChangedFiles` call site relies on). For the
     two-ref case, wraps `exec.Command("git", "diff",
     "--name-only", base+".."+head).Output()` and parses the
     newline-separated output with an INLINE `strings.Split` loop
     (trim trailing newline; split on `"\n"`; drop empty entries).
     The implementation MUST NOT import `internal/validate` to reuse
     `ParseChangedFiles` — `internal/executor` is forbidden from
     importing any enforcement package per the package doc at
     `internal/executor/executor.go:6`, and `validate.ParseChangedFiles`
     lives in `internal/validate`. Inline is the only acceptable
     form.
   - `FileAtRef(ref, path string) ([]byte, error)`: wraps
     `exec.Command("git", "show", ref+":"+path).Output()`.
   - `MergeBase(a, b string) (string, error)`: wraps
     `exec.Command("git", "merge-base", a, b).Output()` and
     trims trailing whitespace.

4. Extend `internal/executor/mock.go`'s `MockExecutor` struct with
   the stub-return fields per spec Requirement 9:

   ```
   ChangedFilesResult []string
   ChangedFilesErr    error
   FileAtRefResult    []byte
   FileAtRefErr       error
   MergeBaseResult    string
   MergeBaseErr       error
   ```

   Recording reuses the shared `m.Calls` slice via `m.record(...)`
   (the pattern at `mock.go:35-52`); callers query via the existing
   `CallsTo("ChangedFiles")` / `CallsTo("FileAtRef")` /
   `CallsTo("MergeBase")` helper. No new public getters are
   introduced — the recording pattern is uniform across the
   existing methods and the three new ones.

5. Add the three method implementations on `*MockExecutor` (after
   `CommitAll` at `mock.go:99`), each calling `m.record(...)` and
   returning the configured `*Result` + `*Err` field per the
   pattern.

6. Re-run `grep -RnE 'MockExecutor' internal/ cmd/` to verify the
   complete external-fixture inventory matches spec Requirement 9.
   At plan-draft time the inventory is:
   - `internal/executor/mock.go` (the type itself — covered by
     steps 4-5).
   - `internal/next/next_test.go` (lines 428, 457, 474).
   - `internal/complete/complete_test.go` (`newMockExec` at line
     47).
   - `internal/spec/create_test.go` (`newMockExecutor` at line 26).
   - `internal/harness/lifecycle_scenario_test.go` (lines 306, 321,
     362, 372, 388, 398, 436, 450, 513, 522, 551, 560, 585, 594,
     614).
   - `internal/approve/impl_test.go` (lines 52, 101, 121, 151, 187,
     228, 258, 372, 402, 433, 458, 484).
   - `internal/approve/plan_test.go` (line 1043 + the
     `recordingExecutor` wrapper at line 1083 that embeds
     `*executor.MockExecutor`).

   None of these fixtures assert on the three new methods today, so
   the pure literal `&executor.MockExecutor{}` constructions
   auto-compile against the extended interface (zero-value `*Result`
   fields + nil `*Err` fields are the correct defaults). The
   embedding-based `recordingExecutor` at `plan_test.go:1083`
   auto-satisfies the extended interface for the same reason. If
   any fixture turns out to break (e.g. an embedding-based wrapper
   that does NOT auto-satisfy), it is touched up minimally in the
   same commit and the audit drift is recorded in the commit
   message per spec Requirement 9.

7. Run `go build ./... && go test -short ./...`. Both green.

**Verification**
- [ ] `grep -nE 'ChangedFiles\(base, head string\) \(\[\]string, error\)' internal/executor/executor.go` returns one match.
- [ ] `grep -nE 'FileAtRef\(ref, path string\) \(\[\]byte, error\)' internal/executor/executor.go` returns one match.
- [ ] `grep -nE 'MergeBase\(a, b string\) \(string, error\)' internal/executor/executor.go` returns one match.
- [ ] `grep -nE 'func \(m \*MockExecutor\) ChangedFiles\(' internal/executor/mock.go` returns one match.
- [ ] `grep -nE 'func \(m \*MockExecutor\) FileAtRef\(' internal/executor/mock.go` returns one match.
- [ ] `grep -nE 'func \(m \*MockExecutor\) MergeBase\(' internal/executor/mock.go` returns one match.
- [ ] `internal/executor/executor.go` package doc-comment contains the literal phrase "git/process I/O boundary".
- [ ] `var _ Executor = (*MockExecutor)(nil)` at `mock.go:105` still type-checks (compile-time interface satisfaction is the canonical verifier).
- [ ] `go build ./... && go test -short ./...` is green.
- [ ] The complete `MockExecutor` external-fixture inventory in step 6 has been re-grepped at commit time; any drift is recorded in the commit message.

**Acceptance Criteria**
- [ ] Spec AC "`internal/executor/executor.go` declares the three new methods with the exact signatures" is satisfied (verifiable via grep + AST check inside the boundary test once Bead 4 lands).
- [ ] Spec AC "`internal/executor/mock.go` implements the three new methods on `MockExecutor` with record-and-stub behaviour" is satisfied.
- [ ] Spec Requirement 12 (Executor package doc-comment declares the git/process I/O boundary) is partly satisfied here (the `internal/executor` half); the `internal/bead` half lands in Bead 3.
- [ ] HC-6 (every commit builds + tests green) holds for this bead.

**Depends on**
None.

## Bead 2: Rewire `internal/validate/docsync.go`'s `getChangedFiles` through `Executor.ChangedFiles`

Removes the first of the two known leaks. The current implementation
at `internal/validate/docsync.go:49` calls
`gitutil.DiffNameOnlyRef("", ref)` directly, requiring the
`internal/gitutil` import on line 7. This bead replaces the call with
`exec.ChangedFiles("", ref)` and drops the `internal/gitutil` import,
threading an `executor.Executor` through the
`ValidateDocs` → `getChangedFiles` call chain.

**Steps**

1. Re-confirm the audit:
   - Function `getChangedFiles(ref string) ([]string, error)` at
     `internal/validate/docsync.go:47-54` is the sole call site for
     `gitutil.DiffNameOnlyRef` in the file.
   - Entry point `ValidateDocs(root, diffRef string) *Result` at
     `internal/validate/docsync.go:12-45` is the sole in-package
     caller.
   - Production caller: `cmd/mindspec/validate.go:68` (per spec
     Requirement 10 audit). Run `grep -RnE 'ValidateDocs\(' cmd/
     internal/` at commit time and reconcile any drift; if any new
     caller surfaces, the spec is amended in a 1-bead followup
     before the rewire commit lands.

2. Choose Requirement 10's option (a) — change `ValidateDocs`'s
   signature in the same commit. New signature:

   ```go
   func ValidateDocs(root, diffRef string, exec executor.Executor) *Result
   ```

   Rationale: option (b) (separate `ValidateDocsWithExecutor`
   wrapper) leaves the original function in place importing
   `gitutil`, which does not satisfy the import-ban gate of
   Requirement 11. Option (a) is the cleaner satisfaction of the
   spec; the caller surface is small (one production call site).

3. Update `getChangedFiles` to take the executor:

   ```go
   func getChangedFiles(exec executor.Executor, ref string) ([]string, error) {
       files, err := exec.ChangedFiles("", ref)
       if err != nil {
           return nil, fmt.Errorf("changed files for %s: %w", ref, err)
       }
       return files, nil
   }
   ```

   And the call inside `ValidateDocs` becomes
   `getChangedFiles(exec, diffRef)`.

4. Remove the `"github.com/mrmaxsteel/mindspec/internal/gitutil"`
   import from `internal/validate/docsync.go`. Add
   `"github.com/mrmaxsteel/mindspec/internal/executor"` import.

5. Update the sole production caller `cmd/mindspec/validate.go:68`
   to construct a live `MindspecExecutor` and pass it as the third
   argument. The constructor signature is pinned at plan-draft
   time by reading `internal/executor/mindspec_executor.go:53`:

   ```go
   func NewMindspecExecutor(root string) *MindspecExecutor
   ```

   The wiring at `cmd/mindspec/validate.go:68` becomes:

   ```go
   exec := executor.NewMindspecExecutor(root)
   result := validate.ValidateDocs(root, diffRef, exec)
   ```

   `cmd/mindspec` is NOT one of the five lint-scoped packages
   (spec line 342-343), so the wiring there is unaffected by the
   boundary lint. If the constructor signature has drifted between
   plan-draft and implementation time, the implementer reconciles
   in this same commit (HC-6 makes the drift surface immediately).

6. Enumerate `internal/validate/docsync_test.go` callers needing
   the new signature. Verified at plan-draft time by reading the
   file: the test functions present are `TestClassifyChanges`,
   `TestIsDocFile`, `TestIsSourceFile`, `TestParseChangedFiles`,
   `TestParseChangedFiles_Empty`,
   `TestCheckInternalPackages_WithDomainDocs`,
   `TestCheckInternalPackages_WithoutDomainDocs`,
   `TestCheckCmdChanges_WithRelevantDocs`,
   `TestCheckCmdChanges_WithoutRelevantDocs`,
   `TestCheckCmdChanges_NoCmdFiles`. **None of these call
   `ValidateDocs` directly** — they all exercise lower-level
   helpers (`ClassifyChanges`, `ParseChangedFiles`,
   `CheckInternalPackages`, `CheckCmdChanges`, etc.) that take
   already-classified inputs and do NOT touch the git seam. So
   the signature change to `ValidateDocs` requires NO updates to
   `docsync_test.go`. The only `ValidateDocs` caller in the tree
   is the production site at `cmd/mindspec/validate.go:68`
   handled in step 5. If a future test wants end-to-end
   `ValidateDocs` coverage with executor mocking, it constructs
   `&executor.MockExecutor{ChangedFilesResult: []string{...}}`
   and asserts on `m.CallsTo("ChangedFiles")` — but this bead
   adds no such test (the executor-call assertion is covered by
   the boundary lint's seed fixture in Bead 4).

7. Run `go build ./... && go test -short ./...`. Green.

**Verification**
- [ ] `! grep -nE 'gitutil\.DiffNameOnlyRef' internal/validate/docsync.go` returns nothing.
- [ ] `! grep -nE '"github\.com/mrmaxsteel/mindspec/internal/gitutil"' internal/validate/docsync.go` returns nothing.
- [ ] `grep -nE 'exec\.ChangedFiles\(' internal/validate/docsync.go` returns one match (inside `getChangedFiles`).
- [ ] `grep -nE 'executor\.Executor' internal/validate/docsync.go` returns at least one match (the parameter type on `ValidateDocs` and/or `getChangedFiles`).
- [ ] `grep -nE 'ValidateDocs\(' cmd/ internal/` audit at commit time matches the spec Requirement 10 audit (one production caller at `cmd/mindspec/validate.go:68`); any drift is reconciled in a 1-bead followup before merge.
- [ ] `internal/validate/docsync_test.go` passes against the new signature with `&executor.MockExecutor{}`.
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec AC "The pre-spec git leak in `internal/validate/docsync.go`'s `getChangedFiles` (the call to `gitutil.DiffNameOnlyRef`) is removed" is satisfied.
- [ ] Spec AC "Pinned by file + symbol so normal edits do not invalidate the assertion" is partly satisfied here (the live leak is gone); the seed-data fixture half lands in Bead 4.
- [ ] HC-6 (every commit builds + tests green) holds.

**Depends on**
Bead 1 (needs `Executor.ChangedFiles`).

## Bead 3: Refactor `internal/validate/beads.go`'s `CheckBeadExists` through `internal/bead.RunBD`; declare bd boundary in package doc

Removes the second of the two known leaks. The current implementation
at `internal/validate/beads.go:10` calls
`exec.Command("bd", "show", id, "--json").Run()` directly, requiring
the `os/exec` import on line 5. This bead replaces the call with
`bead.RunBD("show", id, "--json")` — mirroring the pattern already in
use at `internal/validate/state.go:225` (per the existing comment at
`state.go:220` confirmed in plan-draft research).
`internal/bead.RunBD` exists at `internal/bead/bdcli.go:60` with
signature `func RunBD(args ...string) ([]byte, error)`.

**Steps**

1. **MANDATORY PREREQUISITE COMMIT — add `BeadExists` to
   `internal/bead/bdcli.go`.** This is NOT conditional. Verified at
   plan-draft time:
   `internal/bead/bdcli.go:60` defines
   `func RunBD(args ...string) ([]byte, error)`, which delegates to
   `tracedOutput` at `internal/bead/bdcli.go:228` which returns
   `cmd.Output()` directly. Therefore non-zero `bd` exits propagate
   as naked `*exec.ExitError`. Preserving `CheckBeadExists`'s
   `(bool, error)` not-found-vs-bd-missing contract WITHOUT
   re-importing `os/exec` into `internal/validate/beads.go` REQUIRES
   a helper inside `internal/bead` (where `os/exec` is allowed).

   Add to `internal/bead/bdcli.go` (placed next to `RunBD` at line
   ~60 for locality; `bdcli.go` is the right file because it owns
   the `RunBD` primitive this helper wraps — keeping the wrapper
   adjacent avoids needing a new file):

   ```go
   // BeadExists reports whether bead id is present in Beads. Returns
   // (true, nil) if bd show <id> --json succeeds; (false, nil) if bd
   // ran but reported the bead as missing (non-zero exit captured as
   // *exec.ExitError); (false, err) only if bd itself is unavailable
   // or some other non-exit error occurred. The os/exec type-switch
   // is performed inside this package so enforcement-package callers
   // never import os/exec.
   func BeadExists(id string) (bool, error) {
       _, err := RunBD("show", id, "--json")
       if err == nil {
           return true, nil
       }
       var exitErr *exec.ExitError
       if errors.As(err, &exitErr) {
           return false, nil
       }
       return false, err
   }
   ```

   Add `"errors"` to the import block (the file already imports
   `"os/exec"`). Add a table test to
   `internal/bead/bdcli_test.go` (create the file if absent)
   covering the three branches via the existing `execCommand`
   test seam at `bdcli.go:17` — the seam lets tests substitute a
   fake `exec.Command` and return canned `*exec.ExitError` /
   success / non-exit errors.

   This commit lands FIRST in Bead 3 so HC-6 stays green (per spec
   Risks "`bd` rewire surface" paragraph, lines 568-573). Bead 3
   produces TWO commits: (1) the helper, (2) the
   `validate/beads.go` rewire.

2. With `BeadExists` available, the rewire is mechanical — no
   error-shape investigation remains. Proceed to step 3.

3. Rewrite `internal/validate/beads.go`:

   ```go
   package validate

   import (
       "fmt"

       "github.com/mrmaxsteel/mindspec/internal/bead"
   )

   // CheckBeadExists verifies a bead ID exists in Beads by routing
   // through internal/bead (the bd boundary; see ADR-0030).
   func CheckBeadExists(id string) (bool, error) {
       exists, err := bead.BeadExists(id)
       if err != nil {
           return false, fmt.Errorf("running bd show: %w", err)
       }
       return exists, nil
   }
   ```

   The `os/exec` import is removed from the file in the same
   commit so the import-ban half of Requirement 11 stays green.

4. Verify the `checkBeadIDs` helper at `beads.go:21-36` continues to
   compile — it calls `CheckBeadExists(id)` and the signature is
   unchanged. Its `r.AddWarning` / `r.AddError` paths are
   semantics-preserving.

5. Update the **`internal/bead`** package doc-comment (Requirement
   12, second half). The package currently has no top-level
   package comment in `bdcli.go`; add one (or create
   `internal/bead/doc.go`) stating: "Package `bead` is the **bd
   boundary** for enforcement packages
   (`internal/{validate,approve,complete,state,phase}`). Direct
   `exec.Command("bd", ...)` calls from any of those packages are
   prohibited and must route through this package." The marker
   phrase "bd boundary" MUST appear verbatim so it is grep-visible.

6. Run the existing test suite. Any test in
   `internal/validate/validate_test.go`,
   `internal/validate/state_test.go`, or
   `internal/bead/bdcli_test.go` that exercises the affected
   surface continues to pass (the `CheckBeadExists` `(bool, error)`
   contract is preserved; any new `BeadExists` helper added in
   step 2's prerequisite gets a minimal table test in
   `bdcli_test.go`).

7. Run `go build ./... && go test -short ./...`. Green on every
   commit produced by this bead (including step 2's prerequisite if
   it landed).

**Verification**
- [ ] `! grep -nE 'exec\.Command\(' internal/validate/beads.go` returns nothing.
- [ ] `! grep -nE '"os/exec"' internal/validate/beads.go` returns nothing.
- [ ] `grep -nE 'bead\.RunBD|bead\.BeadExists' internal/validate/beads.go` returns at least one match (inside `CheckBeadExists`).
- [ ] `grep -RnE 'bd boundary' internal/bead/` returns at least one match (the package doc-comment marker).
- [ ] If step 2's prerequisite commit landed, `grep -nE 'func BeadExists\(' internal/bead/bdcli.go` (or wherever the helper lives) returns one match.
- [ ] Existing tests against `CheckBeadExists` continue to pass; the `(bool, error)` contract is preserved.
- [ ] `go build ./... && go test -short ./...` is green on every commit produced by this bead.

**Acceptance Criteria**
- [ ] Spec AC "The pre-spec `bd` leak in `internal/validate/beads.go`'s `CheckBeadExists` (the call to `exec.Command("bd", "show", id, "--json")`) is replaced by a call through `internal/bead` (`bead.RunBD` or equivalent), and the `os/exec` import is removed from `internal/validate/beads.go`" is satisfied.
- [ ] Spec Requirement 12 second half (the `internal/bead` package doc-comment declares the **bd boundary**) is satisfied.
- [ ] HC-6 (every commit builds + tests green) holds for this bead.

**Depends on**
None (independent of Beads 1 and 2 — different file, different
shellout, no shared call chain). May land in parallel with Bead 2.

## Bead 4: Add the AST boundary lint at `internal/lint/boundary_test.go` and finalize ADR-0030

The lint bead. Creates the new `internal/lint/` package and ships
`TestEnforcementHasNoGitLeaks` per spec Requirement 11. The test is
the lifetime invariant the spec exists to install. It lands LAST
because the live tree under
`internal/{validate,approve,complete,state,phase}` must be clean of
banned imports and banned literals before the test can pass — that
cleanliness is delivered by Beads 2 and 3.

**Steps**

1. Create the new package directory `internal/lint/` with a single
   file `boundary_test.go`. No `package lint` non-test file is
   needed — the AST walker is internal to the test (the standard
   Go `_test.go` convention; the test file's `package lint` clause
   creates the package).

2. Implement `TestEnforcementHasNoGitLeaks` (`boundary_test.go`)
   per spec Requirement 11:

   **Primary gate — import ban (load-bearing).** Anchor enforcement
   package paths to the test source file's location via
   `runtime.Caller(0)` so the test is robust to any cwd convention
   drift. Inline:

   ```go
   _, thisFile, _, _ := runtime.Caller(0)
   thisDir := filepath.Dir(thisFile)
   repoRoot := filepath.Join(thisDir, "..", "..")
   enforcementPkgs := []string{
       filepath.Join(repoRoot, "internal", "validate"),
       filepath.Join(repoRoot, "internal", "approve"),
       filepath.Join(repoRoot, "internal", "complete"),
       filepath.Join(repoRoot, "internal", "state"),
       filepath.Join(repoRoot, "internal", "phase"),
   }
   ```

   Import `"path/filepath"` and `"runtime"` in the test file. For
   each, call
   `parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)`.
   For each `*ast.File`, check the allowlist-comment marker FIRST
   (step 4). If not allowlisted, walk `file.Imports` and FAIL if
   any `ImportSpec.Path.Value` (the quoted literal) matches:
   - `"os/exec"`
   - `"github.com/mrmaxsteel/mindspec/internal/gitutil"`

   **Secondary gate — literal call-site ban.** Walk `ast.CallExpr`
   nodes with `ast.Inspect`. For each call, check if the function
   is a `*ast.SelectorExpr` whose `X` is an identifier `exec` and
   `Sel` is `Command` or `CommandContext`. If so, extract the
   first-string-argument (or the second, for `CommandContext`
   whose first arg is the context). FAIL if the literal value (or
   constant-folded value via a one-level pass over the file's
   `*ast.GenDecl` `CONST` declarations — see step 3) is `"git"`
   or `"bd"`.

   The failure message includes file path, line number, function
   name (looked up via the nearest enclosing `*ast.FuncDecl`),
   and the offending import/literal so the diagnostic is
   actionable.

3. Implement the one-level constant-folding helper. The helper
   walks the file's top-level `*ast.GenDecl` `Tok == token.CONST`
   blocks once, builds a `map[string]string` of `name → literal
   value` for string-typed constants, and uses this map when the
   call-expr first argument is an `*ast.Ident` rather than a
   `*ast.BasicLit`. This catches `const cmd = "git";
   exec.Command(cmd, ...)`. Variable-bound or computed forms are
   NOT caught — by design; the import ban is what closes that
   hole (spec Requirement 11 secondary-gate paragraph, lines
   263-270).

4. Implement the allowlist-comment mechanism (spec Requirement 11
   "Allowlist mechanism" paragraph, lines 271-281). For each
   `*ast.File`, examine `file.Doc` — Go's standard file-doc
   convention attaches a comment group BEFORE the `package` clause
   to `ast.File.Doc`. The walker iterates `file.Doc.List` and
   strips each comment's leading `// ` (or `/* ... */` framing)
   plus surrounding whitespace; if any resulting line begins with
   the case-sensitive literal prefix `boundary-allowlisted:`, the
   file is skipped for BOTH the import-ban and
   literal-call-site checks. If `file.Doc` is nil, no exemption
   applies. The reviewer-id portion of the marker is NOT parsed
   (this is a marker, not a structured field; the value's
   existence + the reviewer signature on the commit are what gate
   the exemption). Pinning to `file.Doc` (rather than scanning
   `file.Comments` for floating leading comments) is unambiguous
   and matches how Go reviewers naturally place file-doc comments.

5. Add fixture files for the seed-data assertion. Create
   `internal/lint/testdata/seed_docsync_leak.go.txt`:

   ```go
   package validate

   import (
       "fmt"

       "github.com/mrmaxsteel/mindspec/internal/gitutil"
   )

   func getChangedFiles(ref string) ([]string, error) {
       files, err := gitutil.DiffNameOnlyRef("", ref)
       if err != nil {
           return nil, fmt.Errorf("git diff --name-only %s: %w", ref, err)
       }
       return files, nil
   }
   ```

   And `internal/lint/testdata/seed_beads_leak.go.txt`:

   ```go
   package validate

   import (
       "fmt"
       "os/exec"
   )

   func CheckBeadExists(id string) (bool, error) {
       err := exec.Command("bd", "show", id, "--json").Run()
       if err != nil {
           if _, ok := err.(*exec.ExitError); ok {
               return false, nil
           }
           return false, fmt.Errorf("running bd show: %w", err)
       }
       return true, nil
   }
   ```

   Files end in `.go.txt` (not `.go`) so they are not picked up
   by `go build ./...`. The test parses them via
   `parser.ParseFile(fset, path, src, parser.ParseComments)`.

6. Add a sub-test `TestEnforcementHasNoGitLeaks/seed_fixtures`
   that:
   - Parses each fixture via `parser.ParseFile`.
   - Runs the same walker logic against the parsed file.
   - Asserts the walker returns the **expected** failures:
     - `seed_docsync_leak.go.txt` MUST flag both the
       `internal/gitutil` import AND the
       `gitutil.DiffNameOnlyRef` call site (pinned by symbol:
       function name `getChangedFiles`).
     - `seed_beads_leak.go.txt` MUST flag both the `os/exec`
       import AND the `exec.Command("bd", ...)` call site (pinned
       by symbol: function name `CheckBeadExists`).
   - If either fixture stops producing failures, the test fails —
     proof of the proof (spec Requirement 11 seed-data paragraph,
     lines 282-297).

7. Add a third sub-test
   `TestEnforcementHasNoGitLeaks/allowlist_marker` that:
   - Constructs a synthetic in-memory `*ast.File` from source
     containing the allowlist marker comment AND an `os/exec`
     import.
   - Runs the walker.
   - Asserts the walker returns NO failures (proves the
     allowlist mechanism actually exempts the file).

8. Finalize `.mindspec/docs/adr/ADR-0030-executor-boundary.md`.
   Verified at plan-draft time: the frontmatter Status field
   already reads `Status: Accepted` (line 4 of the ADR file). The
   only remaining drift is the narrative "Status" section
   (line 15) which still reads "Stub created during spec 085
   drafting. Finalized in spec 085 Bead N alongside the AST
   boundary lint." This step:
   - Confirms the frontmatter `Status: Accepted` is unchanged
     (`grep -n '^- \*\*Status\*\*: Accepted'
     .mindspec/docs/adr/ADR-0030-executor-boundary.md` returns
     line 4).
   - Replaces the narrative "Status" paragraph (currently lines
     14-16, beginning "Stub created…") with: "Finalized in spec
     085 Bead 4 alongside the AST boundary lint at
     `internal/lint/boundary_test.go`. The boundary doctrine
     here is the lifetime invariant the spec was drafted to
     install."
   - Verifies the Decision section (lines 36-60) already records
     the five enforcement packages in scope, the AST-vs-grep
     choice, and the `// boundary-allowlisted:` opt-in escape
     hatch. If any of those three sub-decisions has drifted,
     reconcile in this same commit so the ADR matches the
     implemented marker pattern from step 4 verbatim.
   - Adds the procedure for adding a sixth enforcement package
     (extend `enforcementPkgs` in `boundary_test.go`, re-run
     `go test ./internal/lint/`, rewire any surfaced call sites
     through Executor or `internal/bead`, commit) under the
     Decision section if not already present.

9. **HC-3 audit.** `go test -list` is insufficient: it surfaces
   only top-level `Test*` functions (not `t.Run` sub-tests) and
   reports nothing about runtime `t.Skip` calls. Use
   `go test -short -v ./...` on both `main` and the F4 branch,
   parse the `=== RUN` and `--- (PASS|FAIL|SKIP):` lines, and diff
   the resulting test-name + status list. Use a `git worktree add`
   envelope so the F4 working tree is not mutated.

   Concrete procedure (run from the spec branch worktree):

   ```sh
   tmp=$(mktemp -d)
   # Snapshot main
   git worktree add "$tmp/main" main
   ( cd "$tmp/main" && go test -short -v ./... 2>&1 ) \
     | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP):' \
     | sort -u > /tmp/main-tests.txt
   git worktree remove --force "$tmp/main"
   # Snapshot F4 (current branch)
   go test -short -v ./... 2>&1 \
     | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP):' \
     | sort -u > /tmp/f4-tests.txt
   # Diff
   diff /tmp/main-tests.txt /tmp/f4-tests.txt > /tmp/test-diff.txt
   rmdir "$tmp"
   ```

   The F4 list MUST:
   - Contain every `=== RUN` line present in main (no removals).
   - Contain no `--- SKIP:` line absent from main (no new skips).
   - Add exactly the new top-level test name
     `TestEnforcementHasNoGitLeaks` plus its sub-test lines
     (`=== RUN TestEnforcementHasNoGitLeaks/seed_fixtures`,
     `=== RUN TestEnforcementHasNoGitLeaks/allowlist_marker`).

   Record `/tmp/test-diff.txt` in the final commit message
   (paste it inline, do NOT reference an out-of-tree path).

10. **ADR-renumber check** (spec lines 126-134). Run
    `ls .mindspec/docs/adr/ADR-*-executor-boundary.md`. If exactly
    one file (`ADR-0030-executor-boundary.md`), proceed. If a
    sibling spec landed and claimed `0030` first, rename the file
    to the next free integer (e.g.
    `ADR-0031-executor-boundary.md`), `git mv` (not copy) the
    prior file, and update all cross-references: this plan's
    frontmatter, the spec.md Background / Impacted Domains /
    Acceptance Criteria sections, the ADR-0030 file's own
    self-references, and any test that cites the ADR number.
    This is the 1-bead followup the spec authorizes; if no
    renumber is needed, this step is a no-op grep.

11. Run `go build ./... && go test -short ./...`. Green. Run the
    Validation Proofs grep block from spec.md lines 458-491
    verbatim — every command produces the expected zero-output or
    one-match result.

**Verification**
- [ ] `test -f internal/lint/boundary_test.go` returns success.
- [ ] `test -f internal/lint/testdata/seed_docsync_leak.go.txt && test -f internal/lint/testdata/seed_beads_leak.go.txt` returns success.
- [ ] `go test -run TestEnforcementHasNoGitLeaks ./internal/lint/...` passes (including the `seed_fixtures` and `allowlist_marker` sub-tests).
- [ ] Spec Validation Proofs "No banned imports remain" grep (lines 459-466) produces zero output.
- [ ] Spec Validation Proofs "No raw `git`/`bd` shellout literals" grep (lines 469-473) produces zero output.
- [ ] Spec Validation Proofs `Executor` interface signature greps (lines 494-502) each return one match.
- [ ] Spec Validation Proofs `MockExecutor` method greps (lines 503-511) each return one match.
- [ ] HC-3 audit (step 9): F4 `go test -short -v ./...` test-name + status list contains every `=== RUN` line present in main, adds no new `--- SKIP:` lines, and adds exactly `TestEnforcementHasNoGitLeaks` + its two named sub-tests; full diff pasted inline in the commit message.
- [ ] `ls .mindspec/docs/adr/ADR-*-executor-boundary.md | wc -l` returns exactly `1`.
- [ ] ADR-0030 (or renumbered equivalent) has Status: Accepted.
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec AC "`TestEnforcementHasNoGitLeaks` exists at `internal/lint/boundary_test.go` and passes" is fully satisfied.
- [ ] Spec AC "The test AST-walks `internal/{validate,approve,complete,state,phase}` and FAILS if any banned import or banned-literal call-site reappears" is satisfied (primary import gate + secondary literal-walker + one-level constant folding).
- [ ] Spec AC "the `// boundary-allowlisted: <reason and reviewer-id>` opt-in escape hatch is recognised per Requirement 11" is satisfied (proven by the `allowlist_marker` sub-test).
- [ ] Spec AC "seed-data fixture exercises the walker against a synthetic version of this leak" is satisfied for BOTH known leaks (docsync.go's `getChangedFiles` and beads.go's `CheckBeadExists`).
- [ ] Spec AC "exactly one ADR file matching `.mindspec/docs/adr/ADR-*-executor-boundary.md` exists" is satisfied.
- [ ] Spec AC "The full existing test suite passes on the F4 branch (`go test -short ./...` returns zero with no skipped or excluded tests vs main)" is satisfied (HC-3 audit step 9).
- [ ] HC-7 (boundary lint is AST-based, not grep) is satisfied — the test uses `go/parser` + `go/ast` exclusively.
- [ ] HC-6 (every commit builds + tests green) holds for this bead.

**Depends on**
Beads 2 AND 3 (both live-tree leaks must be removed before the
walker can pass against the live source; the seed fixtures cover
the walker-correctness half independently). Bead 1 is a transitive
dependency via Bead 2.

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `TestEnforcementHasNoGitLeaks` exists at `internal/lint/boundary_test.go` and passes | Bead 4 (step 2 + verification) |
| Test AST-walks the five enforcement packages and FAILS on banned imports / banned-literal call-sites (with one level of constant folding) | Bead 4 (steps 2-3) |
| `// boundary-allowlisted: <reason and reviewer-id>` opt-in escape hatch is recognised | Bead 4 (step 4 + `allowlist_marker` sub-test in step 7) |
| Pre-spec git leak in `internal/validate/docsync.go`'s `getChangedFiles` is removed; seed-data fixture exercises the walker | Bead 2 (live-tree removal) + Bead 4 (`seed_docsync_leak.go.txt` fixture, step 5; `seed_fixtures` sub-test, step 6) |
| Pre-spec `bd` leak in `internal/validate/beads.go`'s `CheckBeadExists` is replaced by a call through `internal/bead`; `os/exec` import removed; seed-data fixture exercises the walker | Bead 3 (live-tree removal + `bd boundary` package doc) + Bead 4 (`seed_beads_leak.go.txt` fixture; `seed_fixtures` sub-test) |
| `internal/executor/executor.go` declares the three new methods with exact signatures `ChangedFiles`, `FileAtRef`, `MergeBase` | Bead 1 (steps 1, 3 + verification greps) |
| `internal/executor/mock.go` implements the three new methods on `MockExecutor` with record-and-stub behaviour matching the `CommitAllErr` / `IsTreeCleanErr` pattern; all enumerated external fixtures compile | Bead 1 (steps 4-6) |
| Full existing test suite passes on the F4 branch with no skipped/excluded tests vs main | Bead 4 (step 9 HC-3 audit); HC-6 enforced per-bead |
| `go build ./... && go test -short ./...` green on every commit | Every bead verification block ends with this command pair |
| Exactly one ADR file matching `ADR-*-executor-boundary.md` exists, named `ADR-0030-executor-boundary.md` at consensus time; renumber is a 1-bead followup | Bead 4 (step 10 ADR-renumber check) |
| Spec Requirement 12: Executor package doc declares git/process I/O boundary; `internal/bead` package doc declares **bd boundary** | Bead 1 (Executor half, step 2) + Bead 3 (bead half, step 5) |
| HC-1 / HC-2 / HC-4 (solo-dev UX preserved; standalone CLI; viz/agentmind/bench untouched) | Out of scope of any bead's changes — no flags, daemons, or commands added; spec 084's deletions already excluded those subsystems |
| HC-5 (F4 must merge before F2/F1; ordering chain 085 → 086 → 087) | Out of scope of this plan — enforced by spec sequencing and the ADR-0030 boundary declaration; no bead-level work |
| HC-7 (lint is AST-based, not grep) | Bead 4 (the test uses `go/parser` + `go/ast` exclusively; the spec.md Validation Proofs grep block is a reviewer sanity-check, not the gate) |
