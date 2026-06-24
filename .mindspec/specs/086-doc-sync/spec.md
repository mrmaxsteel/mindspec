---
approved_at: "2026-05-20T21:05:51Z"
approved_by: user
status: Approved
---
# Spec 086-doc-sync: Doc-sync: promote validate warnings to errors with per-domain OWNERSHIP.yaml + --allow-doc-skew override

## Goal

`mindspec complete <bead>` and `mindspec approve impl <spec>` exit
non-zero when the diff under enforcement contains source-file changes
in a domain that lacks corresponding doc updates in the same diff.
Failure is an error, not a warning. Overrides are explicit, flag-driven,
and recorded: `--allow-doc-skew "<reason>"` writes `reason + by + at`
into bead metadata (on `complete`) or spec-epic metadata (on
`approve impl`). Per-domain ownership is declared by
`.mindspec/domains/<domain>/OWNERSHIP.yaml`; when absent, the
validator falls back to `internal/<domain>/**`. The validator's error
message names the manifest file that decided ownership so the operator
can fix the right file. This spec is F2 of the converged transformation
plan; F4 (spec 085, `executor.Executor` boundary) landed on `main` so
the `getChangedFiles` rewire that doc-sync depends on is already in
place — this spec layers the gate on top of it.

## Background

The converged transformation plan sequences `F4 → F2 → F1 → F3 → F5`.
F4 (spec 085) landed: `internal/validate/docsync.go` already routes
through `executor.Executor.ChangedFiles` via its `getChangedFiles`
helper at line 49, and `ValidateDocs(root, diffRef string, exec
executor.Executor)` already takes an Executor. Three `AddWarning`
call sites remain in `docsync.go` (lines 37, 127, 154) and they fire
without blocking either lifecycle gate, so today a bead can complete
and an impl can be approved while domain docs drift silently. F2
closes that gap: the warnings become errors, an `OWNERSHIP.yaml`
manifest lets domains declare their source-path coverage rather than
relying on the `internal/<domain>/**` heuristic, and the
`--allow-doc-skew "<reason>"` flag gives the solo developer an explicit,
audited escape hatch. Per the plan, F1 (spec 087, ADR gating) depends
on the `OWNERSHIP.yaml` machinery introduced here, so F2 must merge
before 087 can begin.

The transformation plan section that governs this spec is at
`/Users/Max/replit/mindspec-transformation-plan.md` lines ~42–86 (the
F2 block) and is converged after three rounds of adversarial debate.
This spec implements that block verbatim; it does not redesign.

`Executor.MergeBase`, `Executor.ChangedFiles`, and `Executor.FileAtRef`
were added to the `executor.Executor` interface in spec 085 Bead 1
(now on `main`; see `/Users/Max/replit/mindspec/internal/executor/executor.go`
lines 64–76). No new Executor surface is required by this spec — it
consumes the boundary that 085 established.

## Impacted Domains

- **`internal/validate/`** — `docsync.go` is the centerpiece. The
  three `AddWarning` call sites at lines 37 (`"doc-sync"`), 127
  (`"internal-docs"`), and 154 (`"cmd-docs"`) are promoted to
  `AddError` for the first two; the third (`cmd-docs`) stays a
  warning per the plan's operator-docs lane policy. A new
  `validateSpecArtifactSync` lane is added: spec.md changes must be
  accompanied by `plan.md` or `.mindspec/specs/<id>/` updates.
  Ownership resolution becomes manifest-driven: a new
  `loadOwnership(root, domain string) (*Ownership, error)` returns a
  single `*Ownership` value (`Paths []string`, `Exclude []string`,
  `ManifestPath string`). It reads
  `.mindspec/domains/<domain>/OWNERSHIP.yaml` if present, else
  returns `&Ownership{Paths: []string{"internal/<domain>/**"},
  ManifestPath: ""}` as the fallback. The internal-docs error message
  names the manifest file (or `"<fallback: internal/<domain>/**>"`
  when none) so the operator knows exactly which `OWNERSHIP.yaml`
  to edit. A new `validate.CheckADRDivergence(root, diffRef string,
  exec executor.Executor) *validate.Result` placeholder is added,
  returning an empty `*Result` in this spec; spec 087 (F1) fills the
  body.
- **`internal/complete/`** — `complete.go` line 118 is the clean-tree
  check (`exec.IsTreeClean(checkPath)`); `closeBeadFn` is invoked at
  line 126. The doc-sync validator runs in between: after clean-tree
  succeeds, before `closeBeadFn`. The diff base is the bead branch's
  fork from the spec branch (`base = exec.MergeBase(specBranch,
  "HEAD")`, `head = "HEAD"`). `Run`'s signature gains a trailing
  `CompleteOpts` parameter (`CompleteOpts{AllowDocSkew string}`); a
  back-compat shim accepts callers that pass the zero value. CLI
  wires `--allow-doc-skew "<reason>"` to `opts.AllowDocSkew`. When
  `opts.AllowDocSkew != ""`, `complete.Run` skips the gate and writes
  `mindspec_doc_skew_reason` + `mindspec_doc_skew_at` +
  `mindspec_doc_skew_by` into the bead metadata via
  `bead.MergeMetadata(beadID, ...)`.
- **`internal/approve/`** — `impl.go` today runs, in source order:
  1. line 65: `implRunBDCombinedFn("close", epicID)` (epic-close)
  2. lines 71–76: `bead.MergeMetadata(epicID, mindspec_phase: done)`
     (phase-metadata write)
  3. lines 89–98: bead-status loop (enforcement)
  4. line 111: `exec.FinalizeEpic(epicID, specID, specBranch)` (the
     real merge/push to `main` — the lifecycle mutation that actually
     ships the work)

  All four are mutating or terminal operations. The plan's Round-3
  fix calls for enforcement BEFORE every one of them — including
  `FinalizeEpic`, since once that returns success the spec branch is
  merged. This spec performs the reorder so that the bead-status
  loop, the doc-sync validator, and the ADR-divergence placeholder
  ALL run before lines 65, 71–76, AND 111. `ApproveImpl`'s
  signature: `ImplOpts` (currently empty per R3) gains
  `AllowDocSkew string`. `--allow-doc-skew` writes
  `mindspec_impl_skew_reason` + audit fields into the spec epic's
  metadata (NOT the bead's — there is no per-bead context at
  `approve impl` time, per Codex's correction).
- **`internal/doctor/`** — `docs.go` line 13 declares the four
  `domainFiles` (`overview.md`, `architecture.md`, `interfaces.md`,
  `runbook.md`) and `checkDomains` (line 50) iterates each domain
  directory checking those files. A new check is added in the same
  loop: if `OWNERSHIP.yaml` is missing from a domain directory, emit
  a `Warn` with message `"missing OWNERSHIP.yaml; doc-sync falls back
  to internal/<domain>/**"`. This is a warning, not Missing, so
  `mindspec doctor` does not start failing on existing repos.
- **`internal/contextpack/`** — `domaindoc.go` already models a
  domain as a directory with the four standard files. No structural
  change; this spec confirms the model and uses the same domain
  directory as the OWNERSHIP.yaml location.
- **NEW: `.mindspec/domains/<domain>/OWNERSHIP.yaml`** —
  per-domain manifest co-located with the domain docs. Schema:
  ```yaml
  paths:
    - internal/contextpack/**
    - internal/tokenize/**
  exclude:
    - internal/contextpack/test_fixtures/**
  ```
  Resolution order: explicit `OWNERSHIP.yaml` → fallback
  `internal/<domain>/**`. No central `domain_map.yml`. F2 sub-task,
  not a separate feature: as part of landing this spec, an
  `OWNERSHIP.yaml` is created for each of the four currently-existing
  domain directories (`context-system`, `core`, `execution`,
  `workflow`) mapping each to its actual `internal/*` paths, because
  none of those four trivially match a literal `internal/<name>/`
  directory and the fallback heuristic would otherwise be dead on
  day one.

## ADR Touchpoints

- [ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md)
  (**new**): Records the warning-to-error promotion, the per-domain
  `OWNERSHIP.yaml` schema (`paths` + `exclude`, glob match, schema-
  level rejection of `viz/`, `agentmind/`, `bench/` first-segment
  entries), the `internal/<domain>/**` fallback rule, the operator-
  docs lane as a warning-only third bucket with the additive accept
  set (`CLAUDE.md`, `CONVENTIONS.md`, `project-docs/user/**`,
  `.mindspec/core/USAGE.md`), and the `--allow-doc-skew`
  override semantics with split storage (bead metadata on `complete`,
  spec-epic metadata on `approve impl`). Documents the `approve impl`
  enforcement-first reorder — explicitly including `FinalizeEpic` at
  impl.go:111 in the set of mutations that gates must precede.
  States that the validator error message MUST name the manifest
  file that decided ownership.
- [ADR-0030-executor-boundary.md](../../adr/ADR-0030-executor-boundary.md):
  Prerequisite. F2 depends on F4's `Executor.ChangedFiles`,
  `Executor.MergeBase`, and `Executor.FileAtRef` surfaces, which
  ADR-0030 records as the git/process I/O boundary. ADR-0031 cites
  ADR-0030 as the boundary it builds on; doc-sync's diff reads go
  through `exec.ChangedFiles(base, head)` and `exec.MergeBase(a, b)`,
  never through `os/exec` or `internal/gitutil`.
- ADR-0014 through ADR-0024: scanned for doc-sync touchpoints during
  ADR-0031 drafting. ADR-0023 (lifecycle state derived from beads,
  no more `.mindspec/focus`) is the most likely touchpoint because
  `complete` and `approve impl` are both lifecycle commands; ADR-0031
  notes that the doc-sync gate runs strictly inside those commands
  and does not introduce any new lifecycle-state artifact. Other ADRs
  in that range that turn out to touch doc-sync at drafting time are
  cited in ADR-0031's "Related ADRs" section.

## Requirements

### Hard Constraints (from converged plan)

1. **F2 must merge AFTER F4 (spec 085).** F4 has landed on `main`
   (commit `5f72c2e` per `git log`; `ValidateDocs` signature and the
   `getChangedFiles` helper at `docsync.go:49` already route through
   `Executor`; `MergeBase`, `ChangedFiles`, `FileAtRef` are on the
   `Executor` interface). This spec is unblocked.
2. **F1 (spec 087) depends on F2 having landed** because F1 reuses
   the `OWNERSHIP.yaml` machinery introduced here for ADR-divergence
   domain mapping, and fills the body of the
   `validate.CheckADRDivergence` placeholder this spec lands. This
   spec must merge before 087 starts.
3. **Solo-developer UX preserved.** The `--allow-doc-skew "<reason>"`
   override exists on both `complete` and `approve impl`; the
   override is explicit, flag-driven, and recorded. No env-var
   escape hatch.
4. **~794 existing tests preserved.** No test is skipped, excluded,
   or marked `t.Skip` relative to `main`. New tests are additive.
5. **`viz/agentmind/bench` excluded.** Doc-sync does not enforce on
   diffs under those trees; the `OWNERSHIP.yaml` fallback never
   resolves into them. The OWNERSHIP.yaml schema validator rejects
   any `paths:` or `exclude:` entry whose first segment is `viz`,
   `agentmind`, or `bench` — these names form a hard-coded
   rejected-set in the loader, so a misauthored manifest fails at
   load time rather than silently enforcing on excluded trees.
6. **Every commit `go build ./... && go test -short ./...` green.**
   No commit is allowed to break either gate.

### Spec-specific

7. **Warnings → errors.** `docsync.go:37` (`"doc-sync"`) and
   `docsync.go:127` (`"internal-docs"`) become `AddError`.
   `docsync.go:154` (`"cmd-docs"`) STAYS `AddWarning` per the plan's
   operator-docs lane policy.
8. **`validateSpecArtifactSync` lane.** New function in `docsync.go`:
   if any changed file matches `.mindspec/specs/<id>/spec.md`,
   require a sibling change to `plan.md` or any other file under
   `.mindspec/specs/<id>/`. Emits an `AddError` on failure.
   ADR-only diffs under `.mindspec/specs/<id>/` count as
   satisfying the lane.
9. **Per-domain `OWNERSHIP.yaml` is honored.** New helper
   `loadOwnership(root, domain string) (*Ownership, error)` returns
   a single struct value:
   ```go
   type Ownership struct {
       Paths        []string
       Exclude      []string
       ManifestPath string // "" signals fallback
   }
   ```
   It reads `.mindspec/domains/<domain>/OWNERSHIP.yaml` when
   present. When the file is absent, the helper returns
   `&Ownership{Paths: []string{"internal/<domain>/**"}, Exclude: nil,
   ManifestPath: ""}`. The validator's error message names
   `ManifestPath` when non-empty, else
   `"<fallback: internal/<domain>/**>"`.
10. **Operator-docs lane (warning, additive accept set).** `cmd/`
    changes are satisfied if the same diff touches ANY of:
    repo-root `CLAUDE.md`, repo-root `CONVENTIONS.md`, any file
    under `project-docs/user/`, or `.mindspec/core/USAGE.md`.
    The existing `CLAUDE.md` / `CONVENTIONS.md` paths remain valid
    — this lane EXTENDS the accept set, it does not replace it.
    `checkCmdChanges` is updated to include the two new paths in
    its `hasRelevantDoc` check; it remains an `AddWarning`.
11. **Override storage (split).**
    - On `complete`: `mindspec_doc_skew_reason` +
      `mindspec_doc_skew_at` (UTC RFC3339) + `mindspec_doc_skew_by`
      (best-effort git `user.email` or `"unknown"`) written into the
      BEAD's metadata via `bead.MergeMetadata(beadID, ...)`.
    - On `approve impl`: `mindspec_impl_skew_reason` + matching
      audit fields written into the SPEC EPIC's metadata via
      `bead.MergeMetadata(epicID, ...)`.
12. **Override empty-reason rejection.** `--allow-doc-skew ""`
    (empty string) is rejected at flag-parse / CLI-binding time with
    the error `"--allow-doc-skew requires a non-empty reason"`. The
    internal `complete.Run` and `ApproveImpl` callers treat an empty
    `opts.AllowDocSkew` string as "no override requested"; a
    non-empty string activates the override and is recorded
    verbatim.
13. **Call site (complete).** `internal/complete/complete.go` —
    after the clean-tree check at line 118, before `closeBeadFn` at
    line 126, call `validate.ValidateDocs(root, diffRef, exec)` with
    `base = exec.MergeBase(specBranch, "HEAD")` and `head = "HEAD"`
    (the bead's diff against its fork point on the spec branch). On
    error: return without closing the bead unless
    `opts.AllowDocSkew != ""`, in which case skip the call and write
    the override metadata. Signature change:
    `Run(root, beadID, specIDHint, commitMsg string, exec
    executor.Executor, opts CompleteOpts) (*Result, error)` where
    `CompleteOpts` is `struct { AllowDocSkew string }`. A back-
    compat shim that takes zero opts is acceptable if it simplifies
    the rewire.
14. **Call site (approve impl) — REORDER covering FinalizeEpic.**
    `internal/approve/impl.go` today runs four mutating / terminal
    operations after the in-function preflight:
    1. line 65: `implRunBDCombinedFn("close", epicID)` (EPIC CLOSE)
    2. lines 71–76: `bead.MergeMetadata(epicID, mindspec_phase: done)`
       (PHASE-METADATA WRITE)
    3. lines 89–98: bead-status loop (ENFORCEMENT — currently
       happens AFTER 1 and 2 but is non-mutating; conceptually
       belongs at the front)
    4. line 111: `exec.FinalizeEpic(epicID, specID, specBranch)`
       (THE REAL MERGE/PUSH — once this returns success the spec
       branch is merged into `main`)

    Reorder to enforcement-first; ALL gates run before ALL mutating
    or terminal operations:
    1. bead-status loop (today's lines 89–98)
    2. doc-sync validator:
       `validate.ValidateDocs(root, exec.MergeBase("main",
       specBranch), exec)` — NEW
    3. ADR-divergence placeholder:
       `validate.CheckADRDivergence(root, diffRef, exec)` —
       returns an empty `*Result` in this spec; spec 087 fills the
       body. The AST call-order test anchors on this named symbol
       so 087's wire-up does not move the call site.
    4. THEN `implRunBDCombinedFn("close", epicID)` (epic close)
    5. THEN `bead.MergeMetadata(epicID, mindspec_phase: done)`
       (phase-metadata write)
    6. THEN `exec.FinalizeEpic(epicID, specID, specBranch)`
       (merge/push to `main`)

    A test asserts the new order by reading the file with `go/ast`
    and checking the relative position of all six call sites,
    including `exec.FinalizeEpic`. When `opts.AllowDocSkew != ""`,
    steps 2 and 3 are skipped and the override metadata is written
    to the epic via `bead.MergeMetadata(epicID, ...)` before
    proceeding to step 4.

    Signature change: `ImplOpts` gains an `AllowDocSkew string`
    field. CLI wires `--allow-doc-skew` to `opts.AllowDocSkew`.
15. **`mindspec doctor` warns** when any domain directory under
    `.mindspec/domains/` lacks `OWNERSHIP.yaml`. Warning only;
    `OK` is reported when the file is present.
16. **OWNERSHIP.yaml created for every existing domain.** As part of
    landing this spec, an `OWNERSHIP.yaml` is committed alongside
    each currently-existing domain directory under
    `.mindspec/domains/`. The four known-existing domains and
    their proposed `paths:` mappings are:
    - `context-system/OWNERSHIP.yaml` →
      `internal/contextpack/**`, `internal/tokenize/**` (if present)
    - `core/OWNERSHIP.yaml` →
      `internal/state/**`, `internal/config/**`,
      `internal/workspace/**`, `internal/spec/**`,
      `internal/phase/**`, `internal/recording/**`
    - `execution/OWNERSHIP.yaml` →
      `internal/executor/**`, `internal/bead/**`,
      `internal/gitutil/**`, `internal/safeio/**`
    - `workflow/OWNERSHIP.yaml` →
      `internal/complete/**`, `internal/approve/**`,
      `internal/next/**`, `internal/resolve/**`,
      `internal/instruct/**`, `internal/validate/**`,
      `internal/doctor/**`

    The exact path lists are finalized during plan / impl based on
    a directory audit; the requirement is that every existing
    domain dir has a manifest at merge time so the fallback
    heuristic is exercised only by domains added later without a
    manifest.
17. **Multi-path conflict policy.** When a changed source file
    matches the `paths:` of more than one domain's `OWNERSHIP.yaml`,
    the first match wins. Ordering: domains are iterated in
    lexicographic order of the domain directory name (deterministic
    across runs and platforms). The validator does NOT report
    multi-attribution; it picks one owner and emits at most one
    `internal-docs` error per file.

## Scope

### In Scope

- Promotion of `docsync.go` warnings at lines 37 (`"doc-sync"`) and
  127 (`"internal-docs"`) to `AddError`.
- New `validateSpecArtifactSync` lane in `docsync.go` for spec.md
  changes.
- Per-domain `OWNERSHIP.yaml` schema (`paths` + `exclude`, with
  schema-level rejection of `viz/`, `agentmind/`, `bench/` first-
  segment entries), reader (`loadOwnership` returning `*Ownership`),
  and fallback to `internal/<domain>/**`.
- `--allow-doc-skew "<reason>"` flag on `mindspec complete` and on
  `mindspec approve impl`; signature plumbing through `complete.Run`
  (`CompleteOpts.AllowDocSkew`) and `ApproveImpl`
  (`ImplOpts.AllowDocSkew`); split-storage override metadata as
  specified in Requirement 11; empty-reason rejection at flag-parse
  time per Requirement 12.
- Operator-docs lane (warning-only, additive accept set per
  Requirement 10): `cmd/` changes satisfied by `CLAUDE.md`,
  `CONVENTIONS.md`, `project-docs/user/**`, or
  `.mindspec/core/USAGE.md`.
- Reorder of `approve/impl.go` so bead-status, doc-sync, and the
  ADR-divergence placeholder (`validate.CheckADRDivergence`) all
  run BEFORE epic-close (line 65), phase-metadata-write (lines
  71–76), AND `exec.FinalizeEpic` (line 111).
- Named `validate.CheckADRDivergence(root, diffRef string, exec
  executor.Executor) *validate.Result` placeholder returning an
  empty `*Result`; spec 087 fills the body.
- `mindspec doctor` warning when a domain directory lacks
  `OWNERSHIP.yaml`.
- Authoring `OWNERSHIP.yaml` for every existing domain directory
  under `.mindspec/domains/` (`context-system`, `core`,
  `execution`, `workflow`).
- ADR-0031 drafted and accepted as part of this spec.

### Out of Scope

- **Routing `bd` reads through `Executor`** — per F4's scope
  decision (recorded in ADR-0030), `bd` access stays behind
  `internal/bead`. F2 does not revisit that.
- **Central `domain_map.yml`** — per the plan, ownership is
  per-domain and co-located; no central map.
- **Per-bead override scope on `approve impl`** — the override on
  `approve impl` writes to the spec epic's metadata, not to any
  bead's, because there is no per-bead context at `approve impl`
  time (Codex's correction).
- **F1 ADR semantic gating body** — spec 087 will fill the
  `validate.CheckADRDivergence` body and add the `--override-adr` /
  `--supersede-adr` flags. F2 only lands the named call site so
  087 can drop in.
- **F3 context-pack budgeter** — spec 088.
- **F5 / F6** — out of scope here.
- **Promotion of `cmd-docs` warning to error** — the operator-docs
  lane is intentionally a warning per the converged plan.
- **Multi-spec branch self-merge** (R2:C3 deferred) — out of
  scope.

## Acceptance Criteria

- [ ] `TestCompleteBlocksOnDocSkew` passes: a bead whose diff
  touches `internal/contextpack/foo.go` only (no doc updates) causes
  `complete.Run` to return an error whose message contains
  `"doc-sync"` and names either
  `.mindspec/domains/context-system/OWNERSHIP.yaml` or the
  `<fallback: internal/context-system/**>` marker (whichever the
  fixture exercises). The bead is NOT closed; the worktree is NOT
  removed.
- [ ] `TestCompleteAllowsOverride` passes: the same bead invoked
  with `opts.AllowDocSkew = "wip — docs coming in followup"`
  returns success; the bead's metadata contains
  `mindspec_doc_skew_reason` set to that string,
  `mindspec_doc_skew_at` set to a parseable RFC3339 timestamp, and
  `mindspec_doc_skew_by` set to a non-empty string.
- [ ] `TestApproveImplBlocksOnSpecDocSkew` passes: a spec branch
  diff (vs `main`) that modifies
  `.mindspec/specs/086-doc-sync/spec.md` without touching
  `plan.md` or any other file under that spec directory causes
  `ApproveImpl` to return an error containing `"spec.md"`. The epic
  is NOT closed; `mindspec_phase: done` is NOT written;
  `exec.FinalizeEpic` is NOT invoked.
- [ ] `TestApproveImplOverrideRecordsToEpic` passes: same scenario
  with `opts.AllowDocSkew = "<reason>"` returns success and the
  EPIC's metadata (not any bead's) contains
  `mindspec_impl_skew_reason` plus audit fields.
- [ ] `TestOwnershipManifestHonored` passes: a domain directory
  containing `OWNERSHIP.yaml` with `paths: [internal/foo/**]` causes
  changes under `internal/foo/` to be attributed to that domain.
  A separate sub-test confirms the fallback: a domain directory
  WITHOUT `OWNERSHIP.yaml` still has `internal/<domain>/**`
  attributed to it, and the validator error names
  `"<fallback: internal/<domain>/**>"`.
- [ ] `TestOwnershipRejectsExcludedTrees` passes: an `OWNERSHIP.yaml`
  whose `paths:` or `exclude:` lists an entry beginning with
  `viz/`, `agentmind/`, or `bench/` fails at load time with an
  error naming the offending entry.
- [ ] `TestOwnershipMultiMatchFirstWins` passes: two domains'
  manifests both include `internal/foo/**`; the validator picks
  the lexicographically earlier domain and emits exactly one
  `internal-docs` error per offending file.
- [ ] `TestDoctorWarnsOnMissingOwnership` passes: `mindspec doctor`
  against a fixture repo whose domain directory has no
  `OWNERSHIP.yaml` emits a `Warn`-level `Check` whose message
  contains `"OWNERSHIP.yaml"`. With the file present, the check is
  `OK`.
- [ ] `TestApproveImplCallOrder` passes: the test parses
  `internal/approve/impl.go` with `go/ast`, locates the call sites
  for bead-status verification, the doc-sync validator
  (`validate.ValidateDocs`), the ADR-divergence placeholder
  (`validate.CheckADRDivergence`), epic close
  (`implRunBDCombinedFn("close", ...)`), phase-metadata write
  (`bead.MergeMetadata(epicID, "mindspec_phase": "done")`), and
  the finalize call (`exec.FinalizeEpic`), and asserts the
  source-order positions are:
  `bead-status < doc-sync < adr-divergence < epic-close < metadata-write < finalize-epic`.
  Critically, all three gates (bead-status, doc-sync,
  adr-divergence) must precede `exec.FinalizeEpic`.
- [ ] `TestOperatorDocsAdditiveAcceptSet` passes: a diff touching
  `cmd/foo.go` plus ANY ONE of `CLAUDE.md`, `CONVENTIONS.md`, a
  file under `project-docs/user/`, or `.mindspec/core/USAGE.md`
  produces no `cmd-docs` warning. A diff touching `cmd/foo.go` with
  none of those produces the warning.
- [ ] `cmd/complete.go` and `cmd/approve.go` (or the relevant
  subcommand files) expose `--allow-doc-skew "<reason>"`. Running
  the flag with an empty reason returns an error
  (`"--allow-doc-skew requires a non-empty reason"`) — see
  Requirement 12 for the contract.
- [ ] An `OWNERSHIP.yaml` exists for every currently-existing
  domain directory under `.mindspec/domains/` in this repo
  (at minimum: `context-system`, `core`, `execution`, `workflow`)
  at the merge commit; the fallback path is exercised only by
  hypothetical future domains added without a manifest.
- [ ] `go build ./... && go test -short ./...` is green on every
  commit in the spec branch's history (verified by per-commit CI
  or by `git rebase -x`).
- [ ] All existing tests still pass; no tests are skipped, excluded,
  or otherwise weakened relative to `main` (verified by diffing
  `go test -v ./...` output against a `main` baseline at merge
  time).

## Validation Proofs

- `go test ./internal/validate -run TestCompleteBlocksOnDocSkew -v` —
  expected: PASS, with the failure-mode error message printed in the
  test log so the manifest-file naming is visible.
- `go test ./internal/complete -run TestCompleteAllowsOverride -v` —
  expected: PASS, with the bead's metadata dump in the log showing
  the three `mindspec_doc_skew_*` keys.
- `go test ./internal/approve -run TestApproveImplBlocksOnSpecDocSkew -v` —
  expected: PASS, with the spec-epic's metadata BEFORE the test
  dumped to confirm `mindspec_phase: done` was NOT written and
  `exec.FinalizeEpic` was NOT called.
- `go test ./internal/approve -run TestApproveImplCallOrder -v` —
  expected: PASS, with the AST-derived positions printed so a
  reviewer can see all six call sites in source order, with
  `exec.FinalizeEpic` strictly LAST.
- `go test ./internal/validate -run TestOwnershipRejectsExcludedTrees -v` —
  expected: PASS.
- `go test ./internal/validate -run TestOwnershipMultiMatchFirstWins -v` —
  expected: PASS.
- `go test ./internal/validate -run TestOperatorDocsAdditiveAcceptSet -v` —
  expected: PASS.
- `go test ./internal/doctor -run TestDoctorWarnsOnMissingOwnership -v` —
  expected: PASS.
- `go build ./... && go test -short ./...` — expected: exit 0 on
  every commit in this spec's branch.
- Manual: run `mindspec complete <bead>` against a bead that has
  touched `internal/contextpack/` without doc changes — expected:
  exit non-zero, error message names the manifest file. Re-run with
  `--allow-doc-skew "test override"` — expected: exit 0, bead
  closes, metadata records the reason.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-20
- **Notes**: Approved via mindspec approve spec