---
adr_citations:
    - id: ADR-0030
    - id: ADR-0031
approved_at: "2026-05-20T21:29:13Z"
approved_by: user
bead_ids:
    - mindspec-chi4.1
    - mindspec-chi4.2
    - mindspec-chi4.3
    - mindspec-chi4.4
spec_id: 086-doc-sync
status: Approved
version: "1"
---
# Plan: 086-doc-sync

## ADR Fitness

- **ADR-0031** (new — "Doc-Sync as an Enforcement Gate with Per-Domain
  OWNERSHIP.yaml"): the stub at
  `.mindspec/adr/ADR-0031-doc-sync-gate.md` already carries
  `Status: Accepted` (line 4) and records three sub-decisions: (1)
  warnings at `internal/validate/docsync.go:37/127/154` are promoted
  to errors (cmd-docs at :154 stays a warning per the operator-docs
  lane policy — see Requirement 7 of spec.md, and Bead 2 step 4
  below); (2) per-domain `OWNERSHIP.yaml` co-located at
  `.mindspec/domains/<domain>/OWNERSHIP.yaml` with
  `{paths, exclude}` schema and `internal/<domain>/**` fallback,
  first-match-wins on multi-attribution with lexicographic domain
  tie-break; (3) `--allow-doc-skew "<reason>"` recorded with
  `reason+by+at`, split storage between bead metadata (on `complete`)
  and spec-epic metadata (on `approve impl`). The narrative "Status"
  paragraph (line 15) still reads "Stub created during spec
  086-doc-sync drafting. Finalized in spec 086 Bead N…" — Bead 3
  step 8 replaces that paragraph with "Finalized in spec 086 Bead 3
  alongside the enforcement-first reorder in
  `internal/approve/impl.go` and the `--allow-doc-skew` plumbing
  through `complete.Run` and `ApproveImpl`."
- **ADR-0030** (just landed; "Executor as the Git/Process I/O
  Boundary"): prerequisite. F2's diff input flows through
  `executor.Executor.ChangedFiles` and `executor.Executor.MergeBase`
  per ADR-0030's boundary doctrine. The
  `ValidateDocs(root, diffRef string, exec executor.Executor) *Result`
  signature at `internal/validate/docsync.go:12` already takes an
  `Executor` (spec 085 Bead 2 landed). The
  `internal/lint/boundary_test.go` `TestEnforcementHasNoGitLeaks`
  invariant from spec 085 Bead 4 continues to hold: this plan adds
  NO new `os/exec` or `internal/gitutil` imports to any of
  `internal/{validate, approve, complete, state, phase}`. All
  git-shaped reads go through `exec.ChangedFiles` and
  `exec.MergeBase`. No contradiction with ADR-0030.
- **ADR-0023** (lifecycle state derived from beads): touchpoint
  scanned at spec-draft time and confirmed compatible. The doc-sync
  gate runs strictly INSIDE `complete.Run` and `ApproveImpl` and
  introduces no new lifecycle-state artifact; override metadata is
  written via `bead.MergeMetadata` to the bead (on `complete`) or
  spec epic (on `approve impl`), not to any `.mindspec/focus` or
  `lifecycle.yaml` file. ADR-0023's "lifecycle state is derived
  from beads" principle is preserved verbatim — override metadata
  IS bead / epic metadata.
- **ADR number reservation.** At plan-draft time the highest
  existing ADR is `ADR-0031-doc-sync-gate.md`, so no renumber is
  needed. If a sibling spec lands claiming `0031` first between
  plan-draft and impl, Bead 3 step 8 renumbers to the next free
  integer (`git mv` the file, update this plan's `adr_citations`
  frontmatter, the spec.md Background / ADR Touchpoints / Acceptance
  Criteria sections, and any test that cites the ADR number) as a
  1-bead followup before merge.

## Testing Strategy

This spec's failure mode is **silent doc drift**: a bead lands or
an impl is approved while domain docs go stale, and the only
feedback is an advisory warning nobody reads. The defense is
mechanical exit-code enforcement — both `complete.Run` and
`ApproveImpl` MUST return a non-nil error on doc-sync violations
unless the operator passed `--allow-doc-skew "<reason>"`, in which
case the reason is recorded in bead/epic metadata with a UTC
RFC3339 timestamp and a best-effort actor identity.

**Bead ordering note.** Bead 1 (OWNERSHIP.yaml loader + per-domain
manifests + multi-match policy + excluded-trees rejection) lands
FIRST because Bead 2's `checkInternalPackages` rewrite consumes
the `Ownership` type and `loadOwnership` helper. Bead 2 (promote
AddWarning→AddError + add `validateSpecArtifactSync` lane + add
`CheckADRDivergence` stub) depends on Bead 1 for the
`checkInternalPackages` ownership rewrite so HC-6 (`go build ./...
&& go test -short ./...` green on every commit) stays satisfied.
Bead 3 (wire enforcement into `complete.Run` + `ApproveImpl` with
override; reorder `approve/impl.go`) depends on Beads 1 AND 2 (it
calls `validate.ValidateDocs` which now returns errors per Bead 2
and attributes per Bead 1, and calls
`validate.CheckADRDivergence` introduced by Bead 2). Bead 4
(`mindspec doctor` OWNERSHIP.yaml warning + operator-docs lane
additive accept set) depends on Bead 1 only (uses the
`OWNERSHIP.yaml` on-disk filename convention) and is independent
of Bead 3 — may land in parallel with Bead 3.

Per the converged plan's HC-6 the per-commit gate is CI; locally
each bead's verification block ends with the exact command pair
`go build ./... && go test -short ./...` passing. HC-4 (~794
existing tests preserved, no skips relative to `main`) is
enforced per-bead: Bead 3 step 10 records a `go test -v ./...`
test-name + status diff vs `main` in its final commit message.

**New test additions across the four beads:**

- **Bead 1** (`internal/validate/ownership_test.go` — new file):
  - `TestOwnershipMultiMatchFirstWins` — fixture with two domain
    dirs whose `OWNERSHIP.yaml` both list `internal/foo/**`;
    assert the lexicographically earlier domain wins and exactly
    one `internal-docs` error is emitted per offending file.
  - `TestOwnershipRejectsExcludedTrees` — fixture
    `OWNERSHIP.yaml` files listing `viz/x/**`, `agentmind/y/**`,
    or `bench/z/**` in `paths:` or `exclude:`; assert
    `loadOwnership` returns an error naming the offending entry.
  - `TestOwnershipFallback` — fixture domain dir WITHOUT
    `OWNERSHIP.yaml`; assert `loadOwnership` returns
    `&Ownership{Paths: []string{"internal/<domain>/**"},
    ManifestPath: ""}`.
- **Bead 2** (`internal/validate/docsync_test.go` — extend
  existing):
  - `TestValidateDocsErrorsOnInternalDocSkew` — fixture diff
    touches `internal/contextpack/foo.go`; assert
    `ValidateDocs` returns a `*Result` for which
    `r.HasFailures()` is true AND `r.Issues` contains an entry
    where `issue.Name == "internal-docs"` and
    `issue.Severity == validate.SevError` and `issue.Message`
    names either
    `.mindspec/domains/context-system/OWNERSHIP.yaml` or
    the `"<fallback: internal/context-system/**>"` marker.
    Also asserts a positive companion case: same diff plus a
    sibling change under `.mindspec/domains/context-system/`
    produces a `*Result` for which `r.HasFailures()` is false
    AND no Issue named `"internal-docs"` exists in `r.Issues`.
    Positive companion is included so HC-4 "no skips" cannot be
    satisfied by an over-eager error path.
  - `TestValidateSpecArtifactSync` — fixture `allChanged` list
    containing `.mindspec/specs/086-doc-sync/spec.md` only;
    assert `validateSpecArtifactSync` appends an Issue with
    `Name == "spec-artifact-sync"` and
    `Severity == validate.SevError`. Negative cases (no Issue
    appended, `r.HasFailures()` remains false): (a) the same
    diff plus a `.mindspec/specs/086-doc-sync/plan.md`
    edit; (b) the same diff plus a new ADR file under
    `.mindspec/adr/ADR-0099-future.md` (ADR additions
    count as siblings per revision 8 of the panel CONSENSUS).
  - `TestCheckADRDivergenceReturnsEmpty` — assert the
    placeholder returns a non-nil `*Result` with
    `len(r.Issues) == 0` AND `r.HasFailures() == false`
    AND `r.SubCommand == "adr-divergence"` (named-symbol
    anchor for spec 087 to fill in).
  - `TestGlobMatchBasics` (lives in
    `internal/validate/ownership_test.go` per Bead 1 step 6 —
    listed here for completeness): exercises `globMatch` with
    leading `**/foo`, trailing `foo/**`, mid-path
    `foo/**/bar`, single-char `?` wildcard, escaped `*`, a
    multi-segment path that should match `**`, and a clear
    no-match case.
- **Bead 3** (`internal/complete/complete_test.go` and
  `internal/approve/impl_test.go` — extend existing):
  - `TestCompleteBlocksOnDocSkew` (in `complete_test.go`) —
    `complete.Run` with a `MockExecutor` returning a
    `ChangedFiles` result containing `internal/contextpack/foo.go`
    (no doc files) returns a non-nil error whose `.Error()`
    contains `"doc-sync"`; `closeBeadFn` is NOT invoked (asserted
    via swap-and-restore of the package-level
    `closeBeadFn = bead.Close` variable at `complete.go:21`).
  - `TestCompleteAllowsOverride` (in `complete_test.go`) — same
    fixture with `opts.AllowDocSkew = "wip — docs coming in
    followup"` returns success; the bead's metadata (asserted
    via a stub `mergeMetadataFn` recorder introduced in Bead 3
    step 4) contains `mindspec_doc_skew_reason`,
    `mindspec_doc_skew_at` (parseable RFC3339), and
    `mindspec_doc_skew_by` (non-empty). Crucially, the test
    also asserts the WRITE ORDER (panel CONSENSUS revision 4):
    the recorder records the order of recorded calls and the
    test asserts `mergeMetadataFn` was called STRICTLY AFTER
    `closeBeadFn` returned nil — a failing `closeBeadFn` stub
    case asserts NO `mergeMetadataFn` invocation occurred.
  - `TestApproveImplBlocksOnSpecDocSkew` (in `impl_test.go`) —
    `ApproveImpl` with a `MockExecutor.ChangedFiles` result
    containing `.mindspec/specs/086-doc-sync/spec.md` only
    (no plan.md sibling) returns a non-nil error containing
    `"spec.md"`; `implRunBDCombinedFn` is NOT invoked;
    `bead.MergeMetadata` for `mindspec_phase: done` is NOT
    invoked; `exec.FinalizeEpic` is NOT invoked (asserted via
    the mock recorder).
  - `TestApproveImplOverrideRecordsToEpic` (in `impl_test.go`)
    — same scenario with `opts.AllowDocSkew = "<reason>"`
    returns success; the spec EPIC's metadata (not any bead's)
    contains `mindspec_impl_skew_reason` + audit fields.
    Also asserts the WRITE ORDER (panel CONSENSUS revision 4):
    the recorder confirms `bead.MergeMetadata` for
    `mindspec_impl_skew_reason` was called STRICTLY AFTER
    `exec.FinalizeEpic` returned nil. A failing-`FinalizeEpic`
    sub-case asserts NO override-metadata write occurred (the
    Finalize failure is the audit trail).
  - `TestApproveImplCallOrder` (in `impl_test.go`) — parses
    `internal/approve/impl.go` with `go/ast`, locates the SEVEN
    call sites enumerated in Bead 3 step 9, and asserts their
    source positions satisfy:
    `bead-status < doc-sync < adr-divergence < epic-close < phase-metadata-write < pre-flight-commit-count < finalize-epic`.
    Additionally asserts the override-metadata write
    (`bead.MergeMetadata` with key literal
    `"mindspec_impl_skew_reason"`) appears AFTER
    `exec.FinalizeEpic` in source order — anchors the
    write-order rule per panel CONSENSUS revision 4.
- **Bead 4** (`internal/doctor/docs_test.go` and
  `internal/validate/docsync_test.go` — extend existing):
  - `TestDoctorWarnsOnMissingOwnership` (in
    `internal/doctor/docs_test.go`) — fixture domain dir without
    `OWNERSHIP.yaml`; assert `mindspec doctor` emits a
    `Warn`-level `Check` whose message contains
    `"OWNERSHIP.yaml"`. Negative case: fixture domain dir WITH
    `OWNERSHIP.yaml`; assert the check is `OK`.
  - `TestOperatorDocsAdditiveAcceptSet` (in
    `internal/validate/docsync_test.go`) — table test exercising
    `checkCmdChanges` with diffs touching `cmd/foo.go` plus
    each of the four accepting paths (`CLAUDE.md`,
    `CONVENTIONS.md`, `project-docs/user/foo.md`,
    `.mindspec/core/USAGE.md`); assert no `cmd-docs`
    warning in any of the four cases. Negative case:
    `cmd/foo.go` only; assert the warning fires.

**Existing-test preservation.** Bead 2's promotion of `AddWarning`
to `AddError` at `docsync.go:37` and `:127` changes the
`Issue.Severity` field on emitted issues. The real `Result` type
at `internal/validate/validate.go:35-40` is
`Result{SubCommand, TargetID, Issues []Issue}` with
`Issue{Name, Severity, Message}` — a SINGLE `Issues` slice (NO
split `Errors`/`Warnings` fields), with severity carried on each
Issue as `SevError` or `SevWarning`. The package already exposes
`(*Result).HasFailures() bool` at `validate.go:42-50` which
iterates `r.Issues` and returns true on any `SevError`-severity
issue — gate predicates throughout this plan use `HasFailures()`
rather than length-of-slice checks. Audit at plan-draft time of
`internal/validate/docsync_test.go`: the existing test functions
exercise the LOWER-LEVEL helpers, not `ValidateDocs` itself.
`TestCheckInternalPackages_WithoutDomainDocs` (lines 98-113)
ALREADY iterates `r.Issues` and matches on `issue.Name ==
"internal-docs"` (it does NOT assert on a `r.Warnings` field —
no such field exists in the current type). The
AddWarning→AddError promotion changes `issue.Severity` from
`SevWarning` to `SevError` on the same Issue, NOT slice
membership; the existing assertion continues to pass verbatim.
Bead 2 step 7 ADDS a severity assertion (`if issue.Severity !=
validate.SevError`) in the NEW tests rather than rewriting any
existing test. All other existing test functions are untouched.

**Override metadata write order.** ALL metadata writes (including
the `--allow-doc-skew` override reason) happen AFTER the terminal
mutation succeeds. In `complete.Run`,
`mergeMetadataFn(beadID, buildSkewMetadata(...))` is called only
after `closeBeadFn` returns nil. In `ApproveImpl`,
`bead.MergeMetadata(epicID, buildImplSkewMetadata(...))` for the
override reason is called only after `exec.FinalizeEpic` returns
nil. If the terminal mutation fails, no override metadata is
written — the failure itself is the audit trail, preventing a
durable "skew-overridden but never completed" lie. This ordering
rule is restated in Bead 3 steps 1 and 5.

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `TestCompleteBlocksOnDocSkew` passes — bead with `internal/contextpack/foo.go` only causes `complete.Run` error containing `"doc-sync"` and naming the manifest or fallback; bead NOT closed; worktree NOT removed | Bead 3 (steps 1-4 wire enforcement + override-empty guard; Bead 1 provides the manifest-naming attribution) |
| `TestCompleteAllowsOverride` passes — same bead with `opts.AllowDocSkew = "<reason>"` returns success; bead metadata contains `mindspec_doc_skew_reason` + `mindspec_doc_skew_at` (RFC3339) + `mindspec_doc_skew_by` (non-empty) | Bead 3 (step 1 override storage on `complete`) |
| `TestApproveImplBlocksOnSpecDocSkew` passes — spec branch diff with `spec.md` change but no `plan.md`/sibling causes `ApproveImpl` error containing `"spec.md"`; epic NOT closed; `mindspec_phase: done` NOT written; `exec.FinalizeEpic` NOT invoked | Bead 3 (step 5 reorder; Bead 2 provides `validateSpecArtifactSync`) |
| `TestApproveImplOverrideRecordsToEpic` passes — same with `opts.AllowDocSkew = "<reason>"` returns success; EPIC's metadata contains `mindspec_impl_skew_reason` + audit fields | Bead 3 (steps 5-6 override storage on `approve impl`) |
| `TestOwnershipManifestHonored` passes — domain dir with `OWNERSHIP.yaml` attributes correctly; without manifest falls back to `internal/<domain>/**` and error names `<fallback: internal/<domain>/**>` | Bead 1 (steps 2-4); Bead 2 (step 3 wires attribution into the error message) |
| `TestOwnershipRejectsExcludedTrees` passes — `OWNERSHIP.yaml` with `viz/`/`agentmind/`/`bench/` first-segment entries fails at load with error naming the offending entry | Bead 1 (step 3 load-time schema validation) |
| `TestOwnershipMultiMatchFirstWins` passes — two domains both match `internal/foo/**`; lexicographically earlier wins; exactly one `internal-docs` error per file | Bead 1 (step 4 multi-match policy) |
| `TestDoctorWarnsOnMissingOwnership` passes — domain dir without `OWNERSHIP.yaml` emits `Warn`-level `Check` containing `"OWNERSHIP.yaml"`; with file present, check is `OK` | Bead 4 (steps 1-2 doctor wire-up) |
| `TestApproveImplCallOrder` passes — AST asserts `bead-status < doc-sync < adr-divergence < epic-close < metadata-write < finalize-epic`; all three gates precede `exec.FinalizeEpic` | Bead 3 (steps 5, 9 reorder + AST test) |
| `TestOperatorDocsAdditiveAcceptSet` passes — `cmd/foo.go` + any of `CLAUDE.md`/`CONVENTIONS.md`/`project-docs/user/**`/`.mindspec/core/USAGE.md` produces no `cmd-docs` warning | Bead 4 (steps 3-4 `checkCmdChanges` extension) |
| `cmd/complete.go` and `cmd/approve.go` expose `--allow-doc-skew "<reason>"`; empty reason returns flag-parse error | Bead 3 (steps 2, 6 CLI flag wiring + empty-reason guard) |
| OWNERSHIP.yaml exists for every currently-existing domain dir (context-system, core, execution, workflow) | Bead 1 (step 5 manifest authoring) |
| `go build ./... && go test -short ./...` green on every commit | Every bead's verification block ends with this command pair |
| All existing tests still pass; no skips/exclusions vs `main` | Bead 3 (step 10 HC-4 audit); HC-6 enforced per-bead |

## Bead 1: OWNERSHIP.yaml loader, Ownership struct, day-one manifests, multi-match policy, excluded-trees rejection

Lands the manifest-driven attribution machinery that Beads 2 and 4
build on. Adds the `Ownership` struct, the
`loadOwnership(root, domain string) (*Ownership, error)` helper,
the multi-match first-wins iteration policy with lexicographic
domain tie-break, and the schema-level rejection of `viz/`,
`agentmind/`, `bench/` first-segment entries. Authors
`OWNERSHIP.yaml` for each of the four currently-existing domain
directories so the fallback heuristic is exercised only by
hypothetical future domains without a manifest.

**Steps**

1. Create the new file `internal/validate/ownership.go`. The type
   and helpers live in `internal/validate` because all consumers
   are in that package (Bead 2 `docsync.go` and Bead 4 `doctor`
   already imports `internal/validate` via the shared on-disk
   filename only — `doctor` does NOT import the Go type, see
   Bead 4 step 1). Package doc for the new file: "ownership.go
   provides per-domain OWNERSHIP.yaml resolution backing doc-sync
   attribution (ADR-0031). The schema rejects `viz/`,
   `agentmind/`, `bench/` first-segment entries at load time per
   Hard Constraint 5 of spec 086."

2. Declare the `Ownership` struct:

   ```go
   // Ownership describes which source-tree paths a domain owns
   // for doc-sync attribution. ManifestPath is the absolute path
   // to the OWNERSHIP.yaml that produced this value; an empty
   // ManifestPath signals the fallback "internal/<domain>/**"
   // heuristic.
   type Ownership struct {
       Paths        []string // glob patterns (e.g. "internal/foo/**")
       Exclude      []string // glob patterns subtracted from Paths
       ManifestPath string   // absolute path; "" signals fallback
   }
   ```

3. Implement `loadOwnership(root, domain string) (*Ownership, error)`:
   - Compute `manifestPath := filepath.Join(root, ".mindspec",
     "docs", "domains", domain, "OWNERSHIP.yaml")`.
   - If the file does not exist (`os.IsNotExist`), return
     `&Ownership{Paths: []string{"internal/"+domain+"/**"},
     Exclude: nil, ManifestPath: ""}, nil`. No error — fallback
     is the expected path for domains without a manifest.
   - If the file exists, `os.ReadFile` it and `yaml.Unmarshal`
     into a struct
     `{Paths []string yaml:"paths"; Exclude []string yaml:"exclude"}`.
     Use `gopkg.in/yaml.v3` (already a project dependency per
     `internal/approve/impl.go:18`).
   - Schema validation — REJECT at load time if ANY entry in
     `Paths` or `Exclude` has first path segment `"viz"`,
     `"agentmind"`, or `"bench"`. The rejected-set is a
     hard-coded
     `var excludedFirstSegments = map[string]struct{}{"viz": {},
     "agentmind": {}, "bench": {}}`. First segment is computed
     via `strings.SplitN(entry, "/", 2)[0]`. Error message:
     `fmt.Errorf("OWNERSHIP.yaml entry %q has excluded first
     segment %q (viz/agentmind/bench trees are out of doc-sync
     scope)", entry, segment)`.
   - On success return
     `&Ownership{Paths: parsed.Paths, Exclude: parsed.Exclude,
     ManifestPath: manifestPath}, nil`.

4. Implement the multi-match first-wins resolver. Add a
   package-level helper:

   ```go
   // attributeDomain returns the owning domain name for a
   // changed source-file path. It iterates domains in
   // lexicographic order of the domain directory name
   // (deterministic across runs and platforms) and returns the
   // FIRST domain whose Ownership matches via Paths minus
   // Exclude. Returns ("", nil) when no domain claims the file.
   func attributeDomain(root, sourcePath string, domains []string) (string, *Ownership, error)
   ```

   - `domains` is the sorted slice of domain directory names
     discovered by reading `.mindspec/domains/` (caller
     responsibility — `docsync.go` Bead 2 step 3 reads the dir
     and sorts via `sort.Strings`).
   - For each `d` in `domains`: call `loadOwnership(root, d)`,
     match `sourcePath` against `Paths` (implement a small
     helper `globMatch(pattern, path string) bool` that handles
     `**` as a multi-segment wildcard and otherwise delegates
     per-segment to `path/filepath.Match`). If any `Paths`
     matches AND no `Exclude` matches, return
     `(d, ownership, nil)`. First match wins; do NOT continue
     iterating.
   - If no domain matches: return `("", nil, nil)`.

5. Author `OWNERSHIP.yaml` for each currently-existing domain
   directory under `.mindspec/domains/`. The four domain
   dirs are `context-system`, `core`, `execution`, `workflow`.
   Plan-time audit (panel CONSENSUS revision 7): every
   `internal/<pkg>/` path proposed below was verified to exist
   in the current main checkout EXCEPT `internal/tokenize/`,
   which does NOT exist today and is therefore excluded from
   the day-one manifest. The remaining 17 paths
   (`contextpack`, `state`, `config`, `workspace`, `spec`,
   `phase`, `recording`, `executor`, `bead`, `gitutil`,
   `safeio`, `complete`, `approve`, `next`, `resolve`,
   `instruct`, `validate`, `doctor` — note `doctor` is in the
   list, making 18 total verified packages with the
   tokenize-exclusion bringing the manifests to 17 entries)
   are confirmed to exist as `internal/<pkg>/` directories at
   plan-draft time.

   - `.mindspec/domains/context-system/OWNERSHIP.yaml`:
     ```yaml
     paths:
       - internal/contextpack/**
     ```
     (Day-one manifest is contextpack-only. `internal/tokenize/`
     does NOT exist in main today; a future spec that adds a
     tokenizer package may add the corresponding `paths:` line
     in the same PR.)

   - `.mindspec/domains/core/OWNERSHIP.yaml`:
     ```yaml
     paths:
       - internal/state/**
       - internal/config/**
       - internal/workspace/**
       - internal/spec/**
       - internal/phase/**
       - internal/recording/**
     ```

   - `.mindspec/domains/execution/OWNERSHIP.yaml`:
     ```yaml
     paths:
       - internal/executor/**
       - internal/bead/**
       - internal/gitutil/**
       - internal/safeio/**
     ```

   - `.mindspec/domains/workflow/OWNERSHIP.yaml`:
     ```yaml
     paths:
       - internal/complete/**
       - internal/approve/**
       - internal/next/**
       - internal/resolve/**
       - internal/instruct/**
       - internal/validate/**
       - internal/doctor/**
     ```

   Each manifest is committed in the same commit as
   `internal/validate/ownership.go` so the loader and its inputs
   land together.

6. Create `internal/validate/ownership_test.go` with the three
   tests enumerated in the Testing Strategy section
   (`TestOwnershipMultiMatchFirstWins`,
   `TestOwnershipRejectsExcludedTrees`,
   `TestOwnershipFallback`) PLUS a `TestGlobMatchBasics` table
   test for the hand-rolled `globMatch` helper introduced in
   step 4 (panel CONSENSUS revision 10). Use `t.TempDir()` to
   build fixture domain directory trees; tests do NOT depend on
   the live repo state. Multi-match test uses domain names
   `"alpha"` and `"beta"` (alpha lexicographically earlier;
   the first-wins assertion checks that alpha is selected).

   `TestGlobMatchBasics` table cases (all required):
   - leading `**/` — `globMatch("**/foo.go", "internal/x/y/foo.go")` is true; `globMatch("**/foo.go", "internal/foo.go")` is true; `globMatch("**/foo.go", "foo.go")` is true.
   - trailing `/**` — `globMatch("internal/foo/**", "internal/foo/bar/baz.go")` is true; `globMatch("internal/foo/**", "internal/foo")` is true (the directory itself matches); `globMatch("internal/foo/**", "internal/bar/baz.go")` is false.
   - mid-path `**` — `globMatch("internal/**/foo.go", "internal/x/y/foo.go")` is true; `globMatch("internal/**/foo.go", "internal/foo.go")` is true.
   - single-char `?` wildcard — `globMatch("foo?.go", "foo1.go")` is true; `globMatch("foo?.go", "foo12.go")` is false.
   - escaped `*` — `globMatch(`foo\*.go`, "foo*.go")` is true; `globMatch(`foo\*.go`, "foobar.go")` is false. (If `globMatch` chooses not to support escaped wildcards because no day-one manifest needs them, document the choice and pin the test to assert the unsupported-escape behavior produces a clear non-match.)
   - clear no-match — `globMatch("internal/foo/**", "cmd/bar.go")` is false.

7. Run `go build ./... && go test -short ./internal/validate/...`.
   Green.

**Verification**
- [ ] `test -f internal/validate/ownership.go` returns success.
- [ ] `grep -nE 'func loadOwnership\(root, domain string\) \(\*Ownership, error\)' internal/validate/ownership.go` returns one match.
- [ ] `grep -nE 'type Ownership struct' internal/validate/ownership.go` returns one match.
- [ ] `grep -nE 'var excludedFirstSegments' internal/validate/ownership.go` returns one match listing `viz`, `agentmind`, `bench`.
- [ ] `test -f .mindspec/domains/context-system/OWNERSHIP.yaml && test -f .mindspec/domains/core/OWNERSHIP.yaml && test -f .mindspec/domains/execution/OWNERSHIP.yaml && test -f .mindspec/domains/workflow/OWNERSHIP.yaml` returns success.
- [ ] `go test -run 'TestOwnership' -v ./internal/validate/...` passes (the three new tests).
- [ ] `go test -run 'TestGlobMatchBasics' -v ./internal/validate/...` passes (all six required cases per panel CONSENSUS revision 10).
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec AC "TestOwnershipMultiMatchFirstWins passes" is satisfied.
- [ ] Spec AC "TestOwnershipRejectsExcludedTrees passes" is satisfied.
- [ ] Spec AC "OWNERSHIP.yaml exists for every currently-existing domain directory" is satisfied for the four day-one domains.
- [ ] Spec Requirement 9 (per-domain OWNERSHIP.yaml honored; `loadOwnership` returns `*Ownership`; fallback to `internal/<domain>/**`) is satisfied.
- [ ] Spec Requirement 17 (multi-path conflict policy: first match wins, lexicographic tie-break) is satisfied.
- [ ] HC-5 (`viz/agentmind/bench` excluded — loader rejects at load time) is satisfied.
- [ ] HC-6 (every commit builds + tests green) holds.

**Depends on**
None.

## Bead 2: Promote AddWarning to AddError at docsync.go:37/127; add validateSpecArtifactSync; add CheckADRDivergence stub

Removes the silent-drift escape route. Promotes
`internal/validate/docsync.go:37` (`"doc-sync"`) and
`internal/validate/docsync.go:127` (`"internal-docs"`) from
`r.AddWarning` to `r.AddError`. Adds the new
`validateSpecArtifactSync` lane for spec.md sibling-update
enforcement. Adds the named `validate.CheckADRDivergence(root,
diffRef string, exec executor.Executor) *Result` placeholder that
spec 087 will fill. `docsync.go:154` (`"cmd-docs"`) STAYS
`AddWarning` per the converged plan's operator-docs lane policy.

**Steps**

1. Re-verify the three AddWarning call-site line numbers in
   `internal/validate/docsync.go` (plan-draft confirms 37, 127,
   154 against the current `main`). If the file has been touched
   between plan-draft and impl, locate the same three logical
   sites by symbol: line 37 is inside `ValidateDocs` after the
   classify-changes block (`"source files changed but no
   documentation files updated"`); line 127 is the tail of
   `checkInternalPackages` (`"internal packages changed (...)
   but no domain docs files updated"`); line 154 is the tail of
   `checkCmdChanges` (`"cmd/ files changed but neither CLAUDE.md
   nor CONVENTIONS.md updated"`).

2. Promote line 37: change `r.AddWarning("doc-sync", ...)` to
   `r.AddError("doc-sync", ...)`. The message text is unchanged.

3. Rewrite `checkInternalPackages` (currently
   `docsync.go:97-129`) to use the Bead-1 ownership machinery
   and promote line 127's warning to an error:
   - At function entry, read the domain directory list from
     `.mindspec/domains/` (use `os.ReadDir` filtered to
     entries where `IsDir()`); sort with `sort.Strings`.
   - For each changed source file in the `source` parameter,
     call `attributeDomain(root, sourcePath, domains)`. The
     signature of `checkInternalPackages` changes to
     `func checkInternalPackages(r *Result, root string, source,
     docs []string)` — caller `ValidateDocs` at `docsync.go:41`
     is updated in the same commit to pass `root`.
   - Build a `map[string]*Ownership` keyed by attributed domain.
   - For each domain with attributed source files: check whether
     the `docs` slice contains ANY file under
     `.mindspec/domains/<domain>/` OR
     `docs/domains/<domain>/`. If NOT, emit
     `r.AddError("internal-docs", fmt.Sprintf("internal sources
     in domain %q changed (%s) but no doc updates under
     %s/; ownership decided by %s", domain, joinSourceFiles,
     filepath.Join(".mindspec", "docs", "domains", domain),
     manifestNameOrFallback))` where `manifestNameOrFallback`
     is `o.ManifestPath` when non-empty, else
     `fmt.Sprintf("<fallback: internal/%s/**>", domain)`. The
     error message MUST name the manifest file (or fallback
     marker) so the operator knows which `OWNERSHIP.yaml` to
     edit.
   - The "multi-match first-wins" property of `attributeDomain`
     ensures exactly one error per file (Bead 1 step 4
     first-wins iteration; the loop here records at most one
     domain per source file).

4. Line 154's `r.AddWarning("cmd-docs", ...)` is LEFT AS-IS.
   Operator-docs is intentionally a warning per ADR-0031
   sub-decision 1 and spec Requirement 7. Bead 4 step 3 extends
   the accept set (additive); it does NOT promote this to an
   error.

5. Add `validateSpecArtifactSync` as a new function in
   `docsync.go` (placed after `checkCmdChanges`):

   ```go
   // validateSpecArtifactSync requires that any change to a
   // spec.md file under .mindspec/specs/<id>/ is
   // accompanied by at least one sibling change. Siblings
   // include: any non-spec.md file under the same
   // .mindspec/specs/<id>/ directory (plan.md, lifecycle
   // artifacts, etc.) OR any added/modified file under
   // .mindspec/adr/**.md (ADR additions count as siblings
   // per panel CONSENSUS revision 8 — spec edits routinely add
   // or cite ADRs as the load-bearing artifact). Emits an
   // AddError on failure. Operates on the union of source and
   // doc file lists (spec.md is classified as a doc, sibling
   // changes may be either).
   func validateSpecArtifactSync(r *Result, allChanged []string) {
       const specPrefix = ".mindspec/specs/"
       const adrPrefix = ".mindspec/adr/"
       // Detect ADR-anywhere siblings once — applies to any
       // spec.md change in the diff.
       hasADRSibling := false
       for _, f := range allChanged {
           if strings.HasPrefix(f, adrPrefix) && strings.HasSuffix(f, ".md") {
               hasADRSibling = true
               break
           }
       }
       bySpec := map[string][]string{}
       for _, f := range allChanged {
           if !strings.HasPrefix(f, specPrefix) {
               continue
           }
           rest := strings.TrimPrefix(f, specPrefix)
           parts := strings.SplitN(rest, "/", 2)
           if len(parts) < 2 {
               continue
           }
           bySpec[parts[0]] = append(bySpec[parts[0]], parts[1])
       }
       for specID, files := range bySpec {
           hasSpec := false
           hasInSpecSibling := false
           for _, p := range files {
               if p == "spec.md" {
                   hasSpec = true
                   continue
               }
               hasInSpecSibling = true
           }
           if hasSpec && !hasInSpecSibling && !hasADRSibling {
               r.AddError("spec-artifact-sync", fmt.Sprintf(
                   "spec.md for %s changed without sibling updates under %s%s/ or new/modified ADRs under %s (expected plan.md, ADR additions, or other artifacts)",
                   specID, specPrefix, specID, adrPrefix))
           }
       }
   }
   ```

   **Call site (panel CONSENSUS revision 3 — early-return fix).**
   `ValidateDocs` at `internal/validate/docsync.go:12-45`
   returns early at line 26 when `len(changed) == 0` and at
   line 32 when `len(sourceChanges) == 0`. A spec.md-only diff
   has ZERO `sourceChanges` (spec.md is a doc file per
   `isDocFile` at `docsync.go:82-87`), so the function would
   return at line 32 and never reach a tail-placed
   artifact-sync call. To make the lane reachable, call
   `validateSpecArtifactSync(r, changed)` BEFORE the
   `if len(sourceChanges) == 0 { return r }` guard at line 31
   (after the `len(changed) == 0` early-return at line 26 — the
   lane has no work to do when nothing changed). The full
   `changed` slice is the input so the spec.md membership check
   works regardless of source-vs-doc classification. The
   existing early-return at line 26 is preserved verbatim;
   only the line-32 guard is bypassed for this lane.

6. Add the `CheckADRDivergence` placeholder in a new file
   `internal/validate/adr_divergence.go` (separate file so spec
   087 can fill the body without thrashing `docsync.go`):

   ```go
   package validate

   import (
       "github.com/mrmaxsteel/mindspec/internal/executor"
   )

   // CheckADRDivergence is a named-symbol placeholder filled by
   // spec 087 (F1 ADR gating). This spec lands the call site in
   // approve/impl.go so the AST call-order test (Bead 3,
   // TestApproveImplCallOrder) anchors on the symbol and 087's
   // body wire-up does not move the call site. Returns an empty
   // *Result in this spec.
   func CheckADRDivergence(root, diffRef string, exec executor.Executor) *Result {
       return &Result{SubCommand: "adr-divergence"}
   }
   ```

7. Extend `internal/validate/docsync_test.go` with the three new
   tests enumerated in the Testing Strategy section
   (`TestValidateDocsErrorsOnInternalDocSkew`,
   `TestValidateSpecArtifactSync`,
   `TestCheckADRDivergenceReturnsEmpty`). The existing
   `TestCheckInternalPackages_WithoutDomainDocs` (current lines
   98-113) iterates `r.Issues` and matches on
   `issue.Name == "internal-docs"` — that assertion continues to
   pass after the AddWarning→AddError promotion (the Issue is
   still emitted with the same Name; only its Severity flips
   from SevWarning to SevError). NO rewrite of the existing test
   is required. The NEW `TestValidateDocsErrorsOnInternalDocSkew`
   additionally asserts `issue.Severity == validate.SevError` to
   pin the new error-severity contract. The new tests use
   `(*Result).HasFailures()` for top-level assertions where
   appropriate.

8. Run `go build ./... && go test -short ./...`. Green.

**Verification**
- [ ] `grep -nE 'r\.AddError\("doc-sync"' internal/validate/docsync.go` returns one match.
- [ ] `grep -nE 'r\.AddError\("internal-docs"' internal/validate/docsync.go` returns one match.
- [ ] `grep -nE 'r\.AddWarning\("cmd-docs"' internal/validate/docsync.go` returns one match (intentionally retained).
- [ ] `grep -nE 'func validateSpecArtifactSync\(' internal/validate/docsync.go` returns one match.
- [ ] `test -f internal/validate/adr_divergence.go` returns success.
- [ ] `grep -nE 'func CheckADRDivergence\(root, diffRef string, exec executor\.Executor\) \*Result' internal/validate/adr_divergence.go` returns one match.
- [ ] `go test -run 'TestValidateDocsErrorsOnInternalDocSkew|TestValidateSpecArtifactSync|TestCheckADRDivergenceReturnsEmpty' -v ./internal/validate/...` passes.
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec Requirement 7 (AddWarning at lines 37 and 127 become AddError; line 154 stays AddWarning) is satisfied.
- [ ] Spec Requirement 8 (`validateSpecArtifactSync` lane) is satisfied.
- [ ] Named-symbol anchor `validate.CheckADRDivergence` exists so spec 087 wire-up does not move the call site.
- [ ] HC-6 (every commit builds + tests green) holds.

**Depends on**
Bead 1 (needs `Ownership` struct, `loadOwnership`, and
`attributeDomain`).

## Bead 3: Wire enforcement into complete.Run + ApproveImpl with --allow-doc-skew override; reorder approve/impl.go

Lands the lifecycle integration. Extends `complete.Run` with
`CompleteOpts.AllowDocSkew`; extends `ApproveImpl`'s existing
`ImplOpts` with `AllowDocSkew`; wires `--allow-doc-skew "<reason>"`
CLI flags on `mindspec complete` and `mindspec approve impl`;
reorders `internal/approve/impl.go` so bead-status, doc-sync, and
ADR-divergence all run BEFORE epic-close (line 65),
phase-metadata-write (lines 71-76), AND `exec.FinalizeEpic`
(line 111). Records override metadata to the bead (on `complete`)
or spec epic (on `approve impl`) via `bead.MergeMetadata`. Rejects
empty override reason at flag-parse time.

**Steps**

1. Extend `internal/complete/complete.go`:
   - Add a new exported `CompleteOpts` struct (placed after
     `Result` at line 40):

     ```go
     // CompleteOpts holds options for bead completion.
     type CompleteOpts struct {
         AllowDocSkew string // non-empty activates override; empty means "no override"
     }
     ```

   - Change `Run`'s signature from
     `Run(root, beadID, specIDHint, commitMsg string, exec
     executor.Executor) (*Result, error)` to
     `Run(root, beadID, specIDHint, commitMsg string, exec
     executor.Executor, opts CompleteOpts) (*Result, error)`.
     The trailing-parameter form (not variadic) is chosen for
     explicitness — callers always know whether they're
     requesting the override.

   - After the clean-tree check (currently `complete.go:118`)
     and BEFORE the `closeBeadFn` call (currently
     `complete.go:126`), insert the doc-sync gate.

     **Cwd context for the diff.** `complete.Run` constructs the
     Executor via `executor.NewMindspecExecutor(wtPath)` where
     `wtPath` (computed at `complete.go:88-95`) is the bead
     worktree. Inside the worktree, `HEAD` resolves to the bead
     branch tip and `specBranch` is the named ref of the spec
     branch (e.g. `spec/086-doc-sync`). The diff range
     `MergeBase(specBranch, HEAD)..HEAD` therefore captures
     exactly the bead's commits. When `wtPath == ""` (the
     bead-without-worktree fallback at `complete.go:88-100`),
     the Executor falls back to `root` and the same range
     captures whatever lives on `HEAD` of the root checkout —
     the operator-visible failure mode is identical, and the
     gate still runs.

     **Inline gate** (uses the real `Result` shape — single
     `Issues` slice with per-Issue `Severity`, and the existing
     `(*Result).HasFailures()` helper at
     `internal/validate/validate.go:42-50`):

     ```go
     // Spec 086 (F2): doc-sync enforcement gate.
     base, mbErr := exec.MergeBase(specBranch, "HEAD")
     if mbErr != nil {
         return nil, fmt.Errorf("computing merge-base for doc-sync: %w", mbErr)
     }
     docResult := validate.ValidateDocs(root, base, exec)
     if docResult.HasFailures() {
         if opts.AllowDocSkew == "" {
             return nil, fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason in bead metadata)", joinResultErrorMessages(docResult))
         }
         // Override path: do NOT write override metadata here.
         // Metadata is written AFTER closeBeadFn returns nil
         // (see step 4 — write-order rule). Falling through
         // intentionally; the override decision is captured in
         // opts.AllowDocSkew and consumed by the post-close
         // metadata block.
     }
     ```

     The override metadata write (`mergeMetadataFn(beadID,
     buildSkewMetadata(...))`) is inserted at a NEW position
     AFTER `closeBeadFn(beadID, commitMsg)` returns nil
     (currently `complete.go:126` — append the metadata write
     immediately after, before the return that emits the Result).
     If `closeBeadFn` fails, no metadata is written; the failure
     is the audit trail. This enforces the panel CONSENSUS
     revision 4 write-order rule.

   - Add the import
     `"github.com/mrmaxsteel/mindspec/internal/validate"` and
     `"time"`. Identity is read via a new helper in
     `internal/bead` (allowed package) — see step 7.

   - Implement the two helpers inside `complete.go` (private
     package-level functions):

     ```go
     // buildSkewMetadata builds a metadata map with reason +
     // RFC3339-UTC timestamp + best-effort actor identity, keyed
     // by the caller-provided field names.
     func buildSkewMetadata(reason, reasonKey, atKey, byKey string) map[string]interface{} {
         return map[string]interface{}{
             reasonKey: reason,
             atKey:     time.Now().UTC().Format(time.RFC3339),
             byKey:     bead.GitUserEmail(),
         }
     }

     // joinResultErrorMessages flattens the SevError-severity
     // Issues from a *validate.Result into a single string
     // suitable for fmt.Errorf wrapping. Iterates r.Issues
     // (the real Result type at validate.go:35-40 has no split
     // Errors/Warnings fields — severity is per-Issue).
     func joinResultErrorMessages(r *validate.Result) string {
         msgs := make([]string, 0, len(r.Issues))
         for _, i := range r.Issues {
             if i.Severity != validate.SevError {
                 continue
             }
             msgs = append(msgs, fmt.Sprintf("[%s] %s", i.Name, i.Message))
         }
         return strings.Join(msgs, "; ")
     }
     ```

     The shape used here (`r.Issues`, `Issue.Name`,
     `Issue.Severity`, `Issue.Message`, the `SevError` constant)
     is verified against `internal/validate/validate.go:9-60`
     at plan-draft time. No impl-time reconciliation is
     expected.

2. Wire the `--allow-doc-skew` CLI flag on `mindspec complete`.
   Locate the cobra command definition (grep
   `RunE.*complete\.Run` at impl time — likely
   `cmd/complete.go` or `cmd/mindspec/complete.go`). Add:

   ```go
   var allowDocSkew string
   cmd.Flags().StringVar(&allowDocSkew, "allow-doc-skew", "",
       "override doc-sync failure with a recorded reason (writes reason+by+at to bead metadata)")
   ```

   In the `RunE`, BEFORE calling `complete.Run(...)`:
   - If the user explicitly passed `--allow-doc-skew ""`
     (detect via `cmd.Flags().Changed("allow-doc-skew")` +
     empty value), return
     `fmt.Errorf("--allow-doc-skew requires a non-empty reason")`.
   - Pass `complete.CompleteOpts{AllowDocSkew: allowDocSkew}`
     as the new trailing argument.

3. Update every other caller of `complete.Run` to pass the new
   `CompleteOpts` argument. Plan-draft audit via
   `grep -RnE 'complete\.Run\(' cmd/ internal/`: the production
   caller is the cobra `RunE` above; test callers live in
   `internal/complete/complete_test.go` and possibly
   `internal/harness/lifecycle_scenario_test.go`. Each gets an
   explicit `complete.CompleteOpts{}` zero-value at the call
   site.

4. Introduce `mergeMetadataFn = bead.MergeMetadata` as a new
   package-level function variable in `complete.go` (alongside
   the existing `closeBeadFn` etc. at lines 21-29). This makes
   `TestCompleteAllowsOverride` able to swap in a recorder
   without touching `internal/bead`. Add
   `TestCompleteBlocksOnDocSkew` and `TestCompleteAllowsOverride`
   to `internal/complete/complete_test.go`. Mocks needed are
   already in scope (`MockExecutor` per spec 085 Bead 1; the
   `closeBeadFn` swap pattern at `complete.go:21-29` shows how
   to record/no-op the close call).

5. Reorder `internal/approve/impl.go` per Requirement 14.
   Current source order at plan-draft (verified above):
   1. Line 65: `implRunBDCombinedFn("close", epicID)` (EPIC CLOSE)
   2. Lines 71-76: `bead.MergeMetadata(epicID, mindspec_phase:
      done)` (PHASE-METADATA WRITE)
   3. Lines 89-98: bead-status loop (ENFORCEMENT — non-mutating)
   4. Line 111: `exec.FinalizeEpic(epicID, specID, specBranch)`
      (THE REAL MERGE/PUSH)

   New source order (all gates BEFORE all mutating/terminal
   ops; all metadata writes — including the override reason —
   AFTER the terminal `exec.FinalizeEpic` returns nil):
   1. Bead-status loop (today's lines 89-98).
   2. Doc-sync gate: `docResult := validate.ValidateDocs(root,
      base, exec)` where `base` is `exec.MergeBase("main",
      specBranch)`. On `docResult.HasFailures()` AND
      `opts.AllowDocSkew == ""`: return error. (Uses the real
      `(*Result).HasFailures()` helper at
      `internal/validate/validate.go:42-50`; the helper iterates
      `r.Issues` returning true on any `SevError`-severity
      Issue.) The override branch falls through; the actual
      metadata write happens AFTER FinalizeEpic in step 9.
   3. ADR-divergence placeholder: `adrResult :=
      validate.CheckADRDivergence(root, base, exec)`. On
      `adrResult.HasFailures()`: return error
      (UNCONDITIONALLY — the `--allow-doc-skew` override does
      NOT apply to ADR divergence per panel CONSENSUS
      revision 6; the spec / ADR-0031 only specify the override
      for doc-sync. In this spec the placeholder always returns
      a `*Result` with `len(r.Issues) == 0`, so this branch is
      never taken — the named call site is what the AST test
      anchors on, and the unconditional-failure semantics are
      what spec 087 will inherit when it fills the body. If a
      future `--override-adr` flag is needed, spec 087 adds it
      explicitly).
   4. THEN `implRunBDCombinedFn("close", epicID)` (today's
      line 65 — the EPIC CLOSE step).
   5. THEN `bead.MergeMetadata(epicID, mindspec_phase: done)`
      (today's lines 71-76 — the PHASE-METADATA WRITE).
   6. Pre-flight `exec.CommitCount` check (today's lines
      101-107) — unchanged, stays between metadata-write and
      FinalizeEpic.
   7. THEN `exec.FinalizeEpic(epicID, specID, specBranch)`
      (today's line 111 — the TERMINAL MUTATION).
   8. Check FinalizeEpic's error return — on error, return
      immediately WITHOUT writing override metadata (the
      failure is the audit trail; revision 4 write-order rule).
   9. Override-record (only if `opts.AllowDocSkew != ""` AND
      FinalizeEpic returned nil): call
      `bead.MergeMetadata(epicID,
      buildImplSkewMetadata(opts.AllowDocSkew))` where the
      helper writes `mindspec_impl_skew_reason`,
      `mindspec_impl_skew_at`, `mindspec_impl_skew_by`. (The
      `buildImplSkewMetadata` helper lives in `impl.go` as a
      private function mirroring `complete.go`'s
      `buildSkewMetadata` but with the impl-prefixed keys
      baked in; copying rather than sharing avoids cross-package
      coupling.)

   Reorder is in-file with cut-paste; no helper extraction. The
   diff is mechanical and reviewable in one sitting.

6. Extend `ImplOpts` (currently `impl.go:27` —
   `type ImplOpts struct{}`) with `AllowDocSkew string`. Wire
   the `--allow-doc-skew` flag on `mindspec approve impl`
   mirroring step 2 (grep `cmd/approve.go` or
   `cmd/mindspec/approve.go` at impl time for the cobra
   binding). Empty-reason rejection is identical to step 2's
   contract. The `ApproveImpl` signature variadic `...ImplOpts`
   is retained for back-compat; callers passing no opts get the
   zero value (no override).

7. Add `bead.GitUserEmail()` to `internal/bead/`. New file
   `internal/bead/identity.go`:

   ```go
   package bead

   import (
       "os/exec"
       "strings"
   )

   // GitUserEmail returns a best-effort git user.email or
   // "unknown" if git is unavailable or unconfigured. Lives in
   // internal/bead because that package is allowed to import
   // os/exec under the ADR-0030 boundary doctrine (enforcement
   // packages route shellouts through here).
   func GitUserEmail() string {
       out, err := exec.Command("git", "config", "--get", "user.email").Output()
       if err != nil {
           return "unknown"
       }
       email := strings.TrimSpace(string(out))
       if email == "" {
           return "unknown"
       }
       return email
   }
   ```

   ADR-0030's boundary doctrine permits `os/exec` in
   `internal/bead`; this helper is functionally a workspace-
   identity read but lives in `internal/bead` because that's
   the closest existing package with `os/exec` permission AND
   it is consumed exclusively from enforcement-package callers
   (`complete.Run`, `ApproveImpl`) that MUST NOT import
   `os/exec`.

   **Boundary reasoning (panel CONSENSUS revision 11).** The
   `TestEnforcementHasNoGitLeaks` invariant in
   `internal/lint/boundary_test.go:111-117` enumerates the
   enforcement set as `internal/{validate, approve, complete,
   state, phase}`. `internal/bead` is OUTSIDE that set and is
   explicitly permitted `os/exec` imports — placing
   `GitUserEmail()` here respects the boundary contract.
   Enforcement-package callers (`complete.Run`, `ApproveImpl`)
   consume `bead.GitUserEmail()` rather than calling `os/exec`
   directly. If a second identity-shaped helper is needed in a
   future spec, the team may promote both to a new
   `Executor.GitUserEmail()` method (the alternative R1
   surfaced in round 1); this spec lands with the
   `internal/bead` placement for minimal scope.

8. Finalize ADR-0031. Replace the narrative "Status" paragraph
   (currently
   `.mindspec/adr/ADR-0031-doc-sync-gate.md:14-16`,
   beginning "Stub created during spec 086-doc-sync drafting…")
   with: "Finalized in spec 086 Bead 3 alongside the
   enforcement-first reorder in `internal/approve/impl.go` and
   the `--allow-doc-skew` plumbing through `complete.Run` and
   `ApproveImpl`. The boundary doctrine is the operator's
   contract: doc drift requires explicit, recorded override."
   Confirm the frontmatter `Status: Accepted` (line 4) is
   unchanged via grep.

   **Verify Decision sub-decision 1 text matches spec Req 7.**
   The ADR §Decision sub-decision 1 was updated as part of the
   panel CONSENSUS revision 2 (October round-1 review) to read
   that lines 37 and 127 become `AddError` while line 154 (the
   operator-docs lane) deliberately REMAINS `AddWarning`. Bead 3
   step 8 verifies that text reads as expected in BOTH copies
   (the worktree copy at
   `.worktrees/worktree-spec-086-doc-sync/.mindspec/adr/ADR-0031-doc-sync-gate.md`
   AND the main-repo copy at
   `.mindspec/adr/ADR-0031-doc-sync-gate.md`) and that
   they are byte-identical via `diff`. If the two copies have
   drifted, the worktree copy is authoritative and the
   main-repo copy is brought into sync in this same commit.

9. Add `TestApproveImplBlocksOnSpecDocSkew`,
   `TestApproveImplOverrideRecordsToEpic`, and
   `TestApproveImplCallOrder` to `internal/approve/impl_test.go`.
   The AST test uses
   `parser.ParseFile(fset, "impl.go", nil, parser.ParseComments)`
   and walks `ast.CallExpr` nodes to locate SEVEN anchored
   call expressions (`exec.MergeBase` may appear multiple
   times — the test pins to the specific call inside
   `ApproveImpl` by checking the enclosing
   `*ast.FuncDecl.Name`). Anchor symbols and required source
   order:
   1. `readBeadStatus` (the bead-status loop has no single
      anchor call expr; use this loop-body call as the proxy)
   2. `validate.ValidateDocs` (doc-sync gate)
   3. `validate.CheckADRDivergence` (ADR-divergence placeholder)
   4. `implRunBDCombinedFn` with first arg literal `"close"`
      (epic close)
   5. `bead.MergeMetadata` with map key literal
      `"mindspec_phase"` (phase-metadata write — distinguished
      from the override-metadata write which uses
      `"mindspec_impl_skew_reason"`)
   6. `exec.CommitCount` (pre-flight check, pinned between
      metadata-write and FinalizeEpic per the panel CONSENSUS
      revision 9 — adding this seventh anchor catches a future
      regression that moves any call into the
      gate-vs-mutation gap)
   7. `exec.FinalizeEpic` (finalize)

   The test asserts the strict source-position ordering
   1 < 2 < 3 < 4 < 5 < 6 < 7. The override-metadata write
   (`bead.MergeMetadata` with key literal
   `"mindspec_impl_skew_reason"`) is asserted to appear AFTER
   `exec.FinalizeEpic` (position 7) — this anchors the panel
   CONSENSUS revision 4 write-order rule in the AST as well.

10. **HC-4 audit.** Per spec Hard Constraint 4 (~794 existing
    tests preserved, no skips relative to `main`). Procedure
    mirrors spec 085 Bead 4 step 9:

    ```sh
    tmp=$(mktemp -d)
    git worktree add "$tmp/main" main
    ( cd "$tmp/main" && go test -short -v ./... 2>&1 ) \
      | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP):' \
      | sort -u > /tmp/main-tests.txt
    git worktree remove --force "$tmp/main"
    go test -short -v ./... 2>&1 \
      | grep -E '^=== RUN|^--- (PASS|FAIL|SKIP):' \
      | sort -u > /tmp/f2-tests.txt
    diff /tmp/main-tests.txt /tmp/f2-tests.txt > /tmp/test-diff.txt
    rmdir "$tmp"
    ```

    The F2 list MUST contain every `=== RUN` line in main, add
    no `--- SKIP:` lines absent from main, and add exactly the
    new top-level test names enumerated in the Testing Strategy
    section plus their sub-tests. Paste `/tmp/test-diff.txt`
    inline in this bead's final commit message.

11. Run `go build ./... && go test -short ./...`. Green. Run
    the Validation Proofs commands from spec.md lines 483-511
    verbatim — each `go test -run <name> -v` produces PASS.

**Verification**
- [ ] `grep -nE 'type CompleteOpts struct' internal/complete/complete.go` returns one match with `AllowDocSkew string` field.
- [ ] `grep -nE 'opts CompleteOpts' internal/complete/complete.go` returns at least one match (the Run signature).
- [ ] `grep -nE 'validate\.ValidateDocs\(' internal/complete/complete.go` returns one match (inside Run, BEFORE the closeBeadFn call).
- [ ] `grep -nE 'AllowDocSkew string' internal/approve/impl.go` returns one match (inside ImplOpts).
- [ ] `grep -nE 'validate\.ValidateDocs\(' internal/approve/impl.go` returns one match.
- [ ] `grep -nE 'validate\.CheckADRDivergence\(' internal/approve/impl.go` returns one match.
- [ ] `grep -nE 'mindspec_doc_skew_reason' internal/complete/complete.go` returns one match.
- [ ] `grep -nE 'mindspec_impl_skew_reason' internal/approve/impl.go` returns one match.
- [ ] `grep -RnE '"--allow-doc-skew"|"allow-doc-skew"' cmd/` returns at least two matches (complete + approve).
- [ ] `test -f internal/bead/identity.go && grep -nE 'func GitUserEmail\(\) string' internal/bead/identity.go` returns success.
- [ ] `go test -run 'TestCompleteBlocksOnDocSkew|TestCompleteAllowsOverride' -v ./internal/complete/...` passes.
- [ ] `go test -run 'TestApproveImplBlocksOnSpecDocSkew|TestApproveImplOverrideRecordsToEpic|TestApproveImplCallOrder' -v ./internal/approve/...` passes.
- [ ] HC-4 audit (step 10): F2 test-name + status list contains every `=== RUN` line in main; adds no new `--- SKIP:` lines; adds exactly the enumerated new test names; full diff pasted in commit message.
- [ ] ADR-0031 narrative "Status" paragraph updated to "Finalized in spec 086 Bead 3…"; frontmatter Status: Accepted unchanged.
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec AC "TestCompleteBlocksOnDocSkew passes" is satisfied (step 4).
- [ ] Spec AC "TestCompleteAllowsOverride passes" is satisfied (step 4).
- [ ] Spec AC "TestApproveImplBlocksOnSpecDocSkew passes" is satisfied (step 9).
- [ ] Spec AC "TestApproveImplOverrideRecordsToEpic passes" is satisfied (step 9).
- [ ] Spec AC "TestApproveImplCallOrder passes — all three gates precede `exec.FinalizeEpic`" is satisfied (step 9).
- [ ] Spec AC "cmd exposes --allow-doc-skew; empty reason errors" is satisfied (steps 2, 6).
- [ ] Spec Requirement 11 (split storage: bead metadata on `complete`, spec-epic metadata on `approve impl`) is satisfied (steps 1, 5).
- [ ] Spec Requirement 12 (empty-reason rejection at flag-parse time) is satisfied (steps 2, 6).
- [ ] Spec Requirement 13 (call site on `complete`) is satisfied (step 1).
- [ ] Spec Requirement 14 (call site on `approve impl` — reorder covering FinalizeEpic) is satisfied (step 5).
- [ ] HC-1 (solo-developer UX preserved) is satisfied — override is explicit, flag-driven, recorded; no env-var escape hatch.
- [ ] HC-4 (existing tests preserved) is satisfied (step 10 audit).
- [ ] HC-6 (every commit builds + tests green) holds.

**Depends on**
Beads 1 AND 2 (consumes `validate.ValidateDocs` which now
returns errors per Bead 2, with attribution naming the manifest
per Bead 1; consumes `validate.CheckADRDivergence` per Bead 2).

## Bead 4: mindspec doctor OWNERSHIP.yaml warning + operator-docs lane additive accept set

The smaller-surface bead. Adds a `mindspec doctor` warning when
a domain directory lacks `OWNERSHIP.yaml`, and extends
`checkCmdChanges` in `internal/validate/docsync.go` to accept any
of `CLAUDE.md`, `CONVENTIONS.md`, `project-docs/user/**`, or
`.mindspec/core/USAGE.md` as satisfying the operator-docs
lane (additive — the existing accept set is preserved).

**Steps**

1. Extend `internal/doctor/docs.go`'s `checkDomains` (currently
   lines 50-83). After the inner `for _, f := range domainFiles`
   loop (currently lines 69-81), add a sibling check for
   `OWNERSHIP.yaml`:

   ```go
   ownerPath := filepath.Join(domainDir, "OWNERSHIP.yaml")
   ownerName := filepath.ToSlash(filepath.Join(docsRel, "domains", domain, "OWNERSHIP.yaml"))
   if fileExists(ownerPath) {
       r.Checks = append(r.Checks, Check{Name: ownerName, Status: OK})
   } else {
       r.Checks = append(r.Checks, Check{
           Name:    ownerName,
           Status:  Warn,
           Message: fmt.Sprintf("missing OWNERSHIP.yaml; doc-sync falls back to internal/%s/**", domain),
       })
   }
   ```

   This is a `Warn` (not `Missing`) per spec Requirement 15:
   existing repos must not start failing `mindspec doctor` on
   day one.

2. Add `TestDoctorWarnsOnMissingOwnership` to
   `internal/doctor/docs_test.go` (create the file if absent).
   Use `t.TempDir()` to build a fixture root with
   `.mindspec/domains/foo/overview.md` (so `checkDomains`
   iterates into `foo/`) and assert two cases:
   - Without `OWNERSHIP.yaml`: a `Check` with name
     `"<docsRel>/domains/foo/OWNERSHIP.yaml"`, status `Warn`,
     message containing `"OWNERSHIP.yaml"`.
   - With `OWNERSHIP.yaml` (write a minimal valid file):
     a `Check` with the same name, status `OK`.

3. Extend `internal/validate/docsync.go`'s `checkCmdChanges`
   (currently lines 132-156) to additively extend the accept
   set. Rewrite the `hasRelevantDoc` block:

   ```go
   hasRelevantDoc := false
   for _, f := range docs {
       // Existing accept set (preserved):
       if f == "CLAUDE.md" || strings.Contains(f, "CONVENTIONS.md") {
           hasRelevantDoc = true
           break
       }
       // Spec 086 (F2) additive operator-docs accept set:
       if strings.HasPrefix(f, "project-docs/user/") ||
           f == ".mindspec/core/USAGE.md" {
           hasRelevantDoc = true
           break
       }
   }
   ```

   The function remains an `r.AddWarning` (line 154) per spec
   Requirement 7. The accept-set extension is the only change
   to this function in this spec.

4. Add `TestOperatorDocsAdditiveAcceptSet` to
   `internal/validate/docsync_test.go`. Table-test with five
   cases:
   - `{cmd: []string{"cmd/foo.go"}, docs: []string{"CLAUDE.md"},
      wantWarning: false}` (existing — preserved)
   - `{cmd: []string{"cmd/foo.go"}, docs:
      []string{"path/to/CONVENTIONS.md"}, wantWarning: false}`
      (existing — preserved)
   - `{cmd: []string{"cmd/foo.go"}, docs:
      []string{"project-docs/user/cli.md"}, wantWarning: false}`
      (NEW additive case)
   - `{cmd: []string{"cmd/foo.go"}, docs:
      []string{".mindspec/core/USAGE.md"}, wantWarning:
      false}` (NEW additive case)
   - `{cmd: []string{"cmd/foo.go"}, docs: []string{},
      wantWarning: true}` (negative: no accept-set match fires
      the warning)

5. Run `go build ./... && go test -short ./...`. Green.

**Verification**
- [ ] `grep -nE 'OWNERSHIP\.yaml' internal/doctor/docs.go` returns at least one match (the new check).
- [ ] `grep -nE '\project-docs/user/' internal/validate/docsync.go` returns one match (inside checkCmdChanges).
- [ ] `grep -nE '\.mindspec/core/USAGE\.md' internal/validate/docsync.go` returns one match (inside checkCmdChanges).
- [ ] `go test -run 'TestDoctorWarnsOnMissingOwnership' -v ./internal/doctor/...` passes (both sub-cases).
- [ ] `go test -run 'TestOperatorDocsAdditiveAcceptSet' -v ./internal/validate/...` passes (all five table rows).
- [ ] `go build ./... && go test -short ./...` is green.

**Acceptance Criteria**
- [ ] Spec AC "TestDoctorWarnsOnMissingOwnership passes" is satisfied.
- [ ] Spec AC "TestOperatorDocsAdditiveAcceptSet passes" is satisfied.
- [ ] Spec Requirement 10 (operator-docs lane warning-only, additive accept set) is satisfied.
- [ ] Spec Requirement 15 (mindspec doctor warns on missing OWNERSHIP.yaml) is satisfied.
- [ ] HC-6 (every commit builds + tests green) holds.

**Depends on**
Bead 1 (uses the `OWNERSHIP.yaml` file convention on disk; does
not need the `Ownership` Go type directly). Independent of Beads
2 and 3 — may land in parallel with Bead 3.
