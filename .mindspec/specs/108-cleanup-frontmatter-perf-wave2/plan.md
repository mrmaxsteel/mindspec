---
status: Draft
spec_id: 108-cleanup-frontmatter-perf-wave2
version: "1"
adr_citations:
  - id: ADR-0036
    sections: [Ownership Discovery, attributeDomain, LoadOwnershipAtRef]
  - id: ADR-0032
    sections: [Semantic ADR Coverage Gates]
  - id: ADR-0033
    sections: [Deterministic Context Pack Budgeting]
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - .mindspec/domains/workflow/OWNERSHIP.yaml
      - internal/trace/event.go
      - .golangci.yml
      - .mindspec/domains/workflow/architecture.md
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/approve/plan.go
      - internal/approve/spec.go
      - internal/approve/impl.go
      - internal/contextpack/budgeter.go
      - internal/validate/plan.go
      - internal/validate/state.go
      - .mindspec/domains/workflow/overview.md
      - .mindspec/domains/context-system/architecture.md
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/validate/divergence.go
      - internal/validate/docsync.go
      - internal/validate/ownership_resolve.go
      - internal/validate/ownership.go
      - .mindspec/domains/workflow/interfaces.md
  - id: 4
    depends_on: []
    key_file_paths:
      - internal/doctor/ownership.go
      - .mindspec/domains/workflow/runbook.md
---
# Plan: 108-cleanup-frontmatter-perf-wave2

Cleanup wave 2. Four independent beads, one per spec work area: (1) claim the
two previously-unowned paths and land the deferred trace/golangci deletions;
(2) collapse the re-implemented YAML-frontmatter scanners onto the canonical
`internal/frontmatter` package and unify the approval-status source of truth;
(3) hoist per-`(file × domain)` `OWNERSHIP.yaml` loading and memoize per-run
ADR parsing in `internal/validate`; (4) make `doctor`'s dead-manifest check walk
the repo tree once. Every externally observable behavior is byte-identical
except the two deliberate changes (frontmatter fence strictness → canonical
`Parse` semantics, and approval status → YAML frontmatter). Spec 107 is already
in this branch's base (merge `b02e236f`), satisfying the R11 prerequisite:
`grep -c findRoot internal/next/beads.go` is `0` in the current tree, so the
`findRoot` carve-out removal (R3) is inert.

## ADR Fitness

Three Accepted ADRs cover the impacted domains (workflow ← ADR-0036/0032;
context-system ← ADR-0033). Each is a good fit and none is superseded by this
wave — the work removes duplication and redundant I/O strictly within their
existing contracts (Non-Goals).

- **ADR-0036 — Ownership Discovery (Accepted; workflow, validation, doc-sync,
  ownership).** Fit. Work area 1 (Bead 1) adds two `paths:` claims exactly as
  this ADR's ZFC attribution model prescribes; work areas 3 and 4 (Beads 3, 4)
  cache the per-`(file × domain)` `LoadOwnership`/`LoadOwnershipAtRef` loads and
  the doctor glob-resolution walk *without* changing any attribution result —
  the same owning domain, the same `Ownership`, the same emitted findings, with
  fewer reads and git subprocesses. No divergence: the ADR fixes the mechanism,
  not the number of times a manifest is parsed. Keep.
- **ADR-0032 — Semantic ADR Coverage Gates (Accepted; validation, adr,
  lifecycle, workflow).** Fit. Work area 3 Perf #4 (Bead 3) memoizes ADR parsing
  inside the coverage/citation gates this ADR defines. Every coverage,
  relevance (`adr-cite-irrelevant`), and supersede-chain decision, and every
  emitted error/warning, is byte-identical — this is a pure I/O reduction inside
  the gate contract, proven by the shared golden-diagnostics test. Keep.
- **ADR-0033 — Pluggable Tokenizer + Deterministic Context Pack Budgeting
  (Accepted; context-system).** Fit. Work area 2's `internal/contextpack`
  migration (Bead 2) changes only how `budgeter.go` locates frontmatter fences;
  the deterministic `BuildBead` budgeting path and the tokenizer interface this
  ADR fixes are untouched. Correctness is anchored by the existing
  `TestContextPackDeterministic` and `TestContextPackFlatVsCanonicalByteIdentical`
  golden tests, which must keep passing unchanged. Keep.

No ADR divergence is proposed. `internal/frontmatter` is a low-level shared
package claimed by no `OWNERSHIP.yaml`; this wave consumes it read-only and does
NOT modify it, so it never enters a bead diff (Scope / Out of Scope).

## Testing Strategy

Unit tests per touched package (`internal/{trace,approve,validate,contextpack,doctor}`),
with three test shapes matching the three kinds of change:

1. **Behavior-preservation (golden / byte-identical):** the approval-fields and
   `bead_ids` approve writes (`TestApprovalWriteByteIdentical`,
   `TestBeadIDsWriteByteIdentical`), the trace NDJSON bytes
   (`TestEventNDJSONGolden`), the contextpack determinism goldens (existing
   `TestContextPackDeterministic` + `TestContextPackFlatVsCanonicalByteIdentical`,
   which must keep passing unchanged), and the validate diagnostics golden
   (`TestValidateDiagnosticsByteIdenticalAfterCaching`).
2. **Seam-counting (perf proofs):** injectable/countable seams prove I/O is
   reduced without asserting timing — per-domain manifest-load counters
   (`TestDivergenceOwnershipLoadedPerDomainNotPerFile`,
   `TestCheckInternalPackagesOwnershipLoadedPerDomainNotPerFile`,
   `TestNormalizeImpactedDomainsOwnershipLoadedPerDomain`), a counting `adr.Store`
   (`TestADRParsedOncePerValidationRun`), and a `walkWorkspaceFn` invocation
   counter (`TestOwnershipCheckWalksTreeOnce`).
3. **Intended-behavior-change proofs:** the frontmatter fence tightening
   (`TestPlanFrontmatterFenceStrictnessTightened`), the approval source-of-truth
   correction (`TestSpecApprovalStatusFromFrontmatter`), and the new ownership
   attribution (`TestWorkflowOwnsTraceAndGolangci`).

Shared infra: in-repo fixture directories for the validate and doctor golden
tests; no new test frameworks. Each bead's exit gate is: its named targeted
suites green **and** the harness-excluded full suite green
(`go test $(go list ./... | grep -v 'internal/harness')`) — the pre-existing
`internal/instruct` `TestRun_IdleNoBeads` isolation flake (bead `z4ps`) is not
introduced by this work and is out of scope — **and** a diff that touches a doc
for every domain whose non-doc source the bead changed, so `mindspec complete`
divergence stays green. `golangci-lint run ./...` (which lints non-test files
only, per `run.tests: false`) must be clean after Bead 1.

## Bead 1: Claim trace/golangci ownership, delete dead marshaler + stale carve-outs

**Steps**
1. Add `internal/trace/**` and `.golangci.yml` to the `paths:` list of
   `.mindspec/domains/workflow/OWNERSHIP.yaml`, so the claims land in the SAME
   diff as this bead's edits to those paths (R1 / R10a same-diff invariant).
2. Delete `trace.Event.MarshalJSON` (the `json.Marshal(Alias(e))` no-op) from
   `internal/trace/event.go` and drop the now-unused `encoding/json` import so
   the package still builds; default struct marshaling now produces the NDJSON.
3. Verify `grep -c findRoot internal/next/beads.go` prints `0` (R11 precondition),
   then remove the three stale `unparam` carve-outs from `.golangci.yml`
   (`internal/brownfield/plan.go` `buildMigrationPlan`,
   `internal/contextpack/builder.go` `isNeighbor`, `internal/next/beads.go`
   `findRoot`), keeping the live `internal/validate/state.go` `validateReviewMode`
   carve-out.
4. Add `internal/trace/event_test.go` `TestEventNDJSONGolden` asserting
   `json.Marshal` of a representative populated `Event` and a zero-value `Event`
   yields the exact current NDJSON bytes (golden), proving removal is invisible.
5. Add `internal/validate` `TestWorkflowOwnsTraceAndGolangci` asserting
   `attributeDomain` resolves both `internal/trace/event.go` and `.golangci.yml`
   to `"workflow"` against the updated manifest set.
6. Append a bead-unique paragraph to `.mindspec/domains/workflow/architecture.md`
   recording that `internal/trace/**` and `.golangci.yml` are now workflow-owned
   (gate-forwardness; this also covers the workflow-owned `OWNERSHIP.yaml`
   self-edit, which requires a same-diff workflow doc touch).
7. Run `golangci-lint run ./...` and the trace + validate suites; confirm clean.

**Verification**
- [ ] `grep -Eq '^[[:space:]]*-[[:space:]]+internal/trace/\*\*' .mindspec/domains/workflow/OWNERSHIP.yaml` (trace claim present; exit 0)
- [ ] `grep -Eq '^[[:space:]]*-[[:space:]]+\.golangci\.yml' .mindspec/domains/workflow/OWNERSHIP.yaml` (golangci claim present; exit 0)
- [ ] `go test ./internal/validate/ -v -run 'TestWorkflowOwnsTraceAndGolangci$' | grep -q -- '--- PASS: TestWorkflowOwnsTraceAndGolangci'`
- [ ] `! grep -q 'func (e Event) MarshalJSON' internal/trace/event.go` (dead marshaler gone; exit 0)
- [ ] `go build ./internal/trace/` succeeds (unused `encoding/json` import removed)
- [ ] `go test ./internal/trace/ -v -run 'TestEventNDJSONGolden$' | grep -q -- '--- PASS: TestEventNDJSONGolden'`
- [ ] `for p in 'brownfield/plan' 'isNeighbor' 'findRoot'; do grep -q "$p" .golangci.yml && exit 1; done; exit 0` (three stale carve-outs removed)
- [ ] `grep -q 'validateReviewMode' .golangci.yml` (live carve-out retained; exit 0)
- [ ] `golangci-lint run ./...` exits 0
- [ ] `go test $(go list ./... | grep -v 'internal/harness')` exits 0
- [ ] `git show --name-only --format= HEAD | grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync touch present)

**Acceptance Criteria**
- [ ] Both `internal/trace/**` and `.golangci.yml` claims are present in the workflow `OWNERSHIP.yaml` (spec AC 1).
- [ ] `attributeDomain` returns `"workflow"` for `internal/trace/event.go` and `.golangci.yml` (spec AC 2).
- [ ] `trace.Event.MarshalJSON` is deleted and NDJSON bytes are unchanged for a populated and a zero-value event (spec AC 3, 4).
- [ ] All three stale `unparam` carve-outs are removed, the `validateReviewMode` carve-out is retained, and `golangci-lint run ./...` is clean (spec AC 5, 6).

**Depends on**
None

## Bead 2: Consolidate frontmatter scanners + unify approval source of truth

**Steps**
1. Add one `internal/approve` helper built on `frontmatter.Parse` that locates
   the frontmatter block, `yaml.Unmarshal`s it into `map[string]interface{}`,
   applies a caller-supplied mutation, `yaml.Marshal`s it, and splices the
   re-marshaled block ahead of the exact `data[bodyOffset:]` body bytes;
   refactor `updatePlanApproval` and `writeBeadIDsToFrontmatter` in
   `internal/approve/plan.go` to call it, preserving byte-identical output for
   well-formed inputs. `internal/frontmatter` is NOT modified.
2. Migrate the reader fence-scans onto `frontmatter.Parse`/`frontmatter.Field`:
   `approve/spec.go` `splitFrontmatter`, `approve/impl.go` `readPlanBeadIDs`,
   `contextpack/budgeter.go` `parseCitedADRs` and the `extractPlanBeadSection`
   strip, and `validate/plan.go` `parsePlanFrontmatter` — dropping the redundant
   manual `#`-comment filtering (YAML already ignores comments) and resolving
   the fence-strictness difference to the canonical `TrimRight(line, "\r\n")`
   semantics (a space-padded fence now reads as no-frontmatter everywhere).
3. Delete `validate/state.go` `readSpecApprovalStatus` and route its two callers
   (`state.go:70`, `state.go:111`) through `validate.SpecStatusAt(specDir)`
   compared case-insensitively (`strings.EqualFold(..., "Approved")`); both
   callers already hold `specDir` in scope.
4. Update the existing `validate/state_test.go` `TestReadSpecApprovalStatus`
   (:200) to exercise the frontmatter-based path, and add
   `TestSpecApprovalStatusFromFrontmatter` proving the YAML `status:` field
   decides when it disagrees with the `## Approval` prose (the ZFC fix).
5. Add `internal/approve` `TestApprovalWriteByteIdentical` and
   `TestBeadIDsWriteByteIdentical` (both writes byte-identical through the shared
   helper), plus `internal/validate`
   `TestPlanFrontmatterFenceStrictnessTightened` (space-padded fence → no
   frontmatter via the migrated path).
6. Append bead-unique paragraphs to `.mindspec/domains/workflow/overview.md` (the
   approve+validate frontmatter/approval unification) AND
   `.mindspec/domains/context-system/architecture.md` (the contextpack
   `budgeter.go` frontmatter migration) — one doc per touched domain (R10b/R12).
7. Run the approve/validate/contextpack suites; confirm the existing contextpack
   determinism goldens still pass unchanged.

**Verification**
- [ ] `go test ./internal/approve/ -v -run 'TestApprovalWriteByteIdentical$' | grep -q -- '--- PASS: TestApprovalWriteByteIdentical'`
- [ ] `go test ./internal/approve/ -v -run 'TestBeadIDsWriteByteIdentical$' | grep -q -- '--- PASS: TestBeadIDsWriteByteIdentical'`
- [ ] `go test ./internal/validate/ -v -run 'TestPlanFrontmatterFenceStrictnessTightened$' | grep -q -- '--- PASS: TestPlanFrontmatterFenceStrictnessTightened'`
- [ ] `! grep -q 'func readSpecApprovalStatus' internal/validate/state.go` (prose-scan deleted; exit 0)
- [ ] `go test ./internal/validate/ -v -run 'TestSpecApprovalStatusFromFrontmatter$' | grep -q -- '--- PASS: TestSpecApprovalStatusFromFrontmatter'`
- [ ] `go test ./internal/contextpack/ -v -run 'TestContextPackDeterministic$' | grep -q -- '--- PASS: TestContextPackDeterministic'`
- [ ] `go test ./internal/contextpack/ -v -run 'TestContextPackFlatVsCanonicalByteIdentical$' | grep -q -- '--- PASS: TestContextPackFlatVsCanonicalByteIdentical'`
- [ ] `git show --name-only --format= HEAD | grep -q '^internal/frontmatter/' && exit 1 || exit 0` (`internal/frontmatter` unmodified in this bead's diff)
- [ ] `go test $(go list ./... | grep -v 'internal/harness')` exits 0
- [ ] `git show --name-only --format= HEAD | grep -qxF '.mindspec/domains/workflow/overview.md'` (workflow doc-sync touch)
- [ ] `git show --name-only --format= HEAD | grep -qxF '.mindspec/domains/context-system/architecture.md'` (context-system doc-sync touch)

**Acceptance Criteria**
- [ ] Both approve writes produce byte-identical files for well-formed inputs through the shared `frontmatter.Parse`-based helper (spec AC 7).
- [ ] A space-padded `---` fence is treated as no-frontmatter through the migrated readers, matching `frontmatter.Parse` (spec AC 8).
- [ ] `readSpecApprovalStatus` is deleted and the YAML `status:` field decides when it disagrees with the `## Approval` prose (spec AC 9).
- [ ] The existing `BuildBead` contextpack goldens still pass unchanged after the `budgeter.go` migration (spec AC 12).

**Depends on**
None

## Bead 3: Hoist OWNERSHIP loading + memoize ADR parsing in validate

**Steps**
1. Introduce a countable per-domain manifest-load seam in `internal/validate` (a
   package-level load counter or an injectable loader func wrapping
   `loadOwnershipForRef`) so a test can assert load counts.
2. Hoist per-domain `Ownership` loading in `ValidateDivergence`
   (`divergence.go`): load each candidate domain's manifest once into an
   in-memory map keyed by domain and attribute changed files against that map
   (mirroring `checkUnclaimedSource`'s `states` pattern at `docsync.go:220-230`),
   replacing the per-`(file × domain)` `attributeDomain` re-load — including the
   up-to-three `git show` subprocesses per domain in `LoadOwnershipAtRef`.
3. Apply the same per-domain-once hoist to `checkInternalPackages`
   (`docsync.go:459`) and `normalizeImpactedDomains` (`ownership_resolve.go:72`).
4. Add a per-run ADR-parse memoization in `internal/validate` (pre-parse cited
   IDs into a map, or a memoizing `adr.Store` decorator wrapping the store
   returned by `adrStoreForSpec`) so `coverageOf`, `hasAcceptedCitation`, and
   `checkADRCitations` read each distinct cited ADR at most once per run.
   `internal/adr` is NOT modified.
5. Add the three seam-counting tests (one per R7 call site) proving manifest-load
   count over a multi-file diff is a function of domain count only, plus
   `TestADRParsedOncePerValidationRun` using a counting `adr.Store`.
6. Add `TestValidateDiagnosticsByteIdenticalAfterCaching`: a fixture-repo golden
   asserting the full ordered `(code, message)` diagnostics set from
   `ValidateDivergence` and `checkADRCoverage` equals a committed golden that
   captures current behavior (the shared identical-outcomes proof for R7 + R8).
7. Append a bead-unique paragraph to `.mindspec/domains/workflow/interfaces.md`
   documenting the per-run ownership-map and ADR-parse caching seams.

**Verification**
- [ ] `go test ./internal/validate/ -v -run 'TestDivergenceOwnershipLoadedPerDomainNotPerFile$' | grep -q -- '--- PASS: TestDivergenceOwnershipLoadedPerDomainNotPerFile'`
- [ ] `go test ./internal/validate/ -v -run 'TestCheckInternalPackagesOwnershipLoadedPerDomainNotPerFile$' | grep -q -- '--- PASS: TestCheckInternalPackagesOwnershipLoadedPerDomainNotPerFile'`
- [ ] `go test ./internal/validate/ -v -run 'TestNormalizeImpactedDomainsOwnershipLoadedPerDomain$' | grep -q -- '--- PASS: TestNormalizeImpactedDomainsOwnershipLoadedPerDomain'`
- [ ] `go test ./internal/validate/ -v -run 'TestADRParsedOncePerValidationRun$' | grep -q -- '--- PASS: TestADRParsedOncePerValidationRun'`
- [ ] `go test ./internal/validate/ -v -run 'TestValidateDiagnosticsByteIdenticalAfterCaching$' | grep -q -- '--- PASS: TestValidateDiagnosticsByteIdenticalAfterCaching'`
- [ ] `git show --name-only --format= HEAD | grep -q '^internal/adr/' && exit 1 || exit 0` (`internal/adr` unmodified in this bead's diff)
- [ ] `go test $(go list ./... | grep -v 'internal/harness')` exits 0
- [ ] `git show --name-only --format= HEAD | grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync touch)

**Acceptance Criteria**
- [ ] All three R7 call sites load manifests per-domain-not-per-file, each proven by its own seam-counting test (spec AC 10).
- [ ] Each distinct cited ADR is read from disk at most once per validation run (spec AC 11).
- [ ] The full ordered `(code, message)` diagnostics set from `ValidateDivergence` and `checkADRCoverage` is byte-identical to the committed golden (spec AC 13, shared R7+R8 proof).

**Depends on**
None

## Bead 4: Doctor walks the workspace tree once per ownership check

**Steps**
1. Add a package-level seam `walkWorkspaceFn = filepath.WalkDir` in
   `internal/doctor/ownership.go` and route the single workspace enumeration
   through it (honoring the existing `walkExclusions` for `.git/`, `.worktrees/`,
   `.beads/`).
2. Refactor the dead-manifest check so `checkOwnershipManifests` enumerates the
   live workspace file list ONCE per call and tests each domain's `paths:` globs
   against that cached list, replacing the per-domain `manifestResolvesAny` full
   `filepath.WalkDir(root, …)` walk.
3. Add `internal/doctor` `TestOwnershipCheckWalksTreeOnce` asserting BOTH that the
   tree is walked exactly once per ownership check regardless of domain count
   (counted via the `walkWorkspaceFn` seam) AND that the full per-domain doctor
   `Report` set (check code + status) is unchanged across populated, empty-stub,
   and dead-manifest fixture domains.
4. Append a bead-unique paragraph to `.mindspec/domains/workflow/runbook.md`
   documenting the single-walk dead-manifest behavior.
5. Run the doctor suite and the harness-excluded full suite; confirm green.

**Verification**
- [ ] `go test ./internal/doctor/ -v -run 'TestOwnershipCheckWalksTreeOnce$' | grep -q -- '--- PASS: TestOwnershipCheckWalksTreeOnce'`
- [ ] `grep -q 'walkWorkspaceFn' internal/doctor/ownership.go` (the countable seam exists; exit 0)
- [ ] `go test ./internal/doctor/` exits 0
- [ ] `go test $(go list ./... | grep -v 'internal/harness')` exits 0
- [ ] `git show --name-only --format= HEAD | grep -qxF '.mindspec/domains/workflow/runbook.md'` (doc-sync touch)

**Acceptance Criteria**
- [ ] The workspace tree is walked exactly once per ownership check regardless of domain count, counted via the `walkWorkspaceFn` seam (spec AC 14, walk-once half).
- [ ] The full per-domain doctor `Report` set (check code + status) is unchanged across populated, empty-stub, and dead-manifest fixture domains (spec AC 14, identical-report half).

**Depends on**
None

## Provenance

| Spec Acceptance Criterion | Bead | Verified By |
|---------------------------|------|-------------|
| AC 1 — both ownership claims present in `OWNERSHIP.yaml` | 1 | Bead 1: both `grep -Eq` claim checks |
| AC 2 — `attributeDomain` → `"workflow"` for trace + golangci | 1 | Bead 1: `TestWorkflowOwnsTraceAndGolangci` |
| AC 3 — `Event.MarshalJSON` deleted | 1 | Bead 1: `! grep 'func (e Event) MarshalJSON'` + `go build` |
| AC 4 — NDJSON bytes unchanged after removal | 1 | Bead 1: `TestEventNDJSONGolden` |
| AC 5 — three stale carve-outs removed, live one retained | 1 | Bead 1: carve-out `for`-loop grep + `validateReviewMode` grep |
| AC 6 — `golangci-lint run ./...` clean | 1 | Bead 1: `golangci-lint run ./...` exit 0 |
| AC 7 — both approve writes byte-identical | 2 | Bead 2: `TestApprovalWriteByteIdentical`, `TestBeadIDsWriteByteIdentical` |
| AC 8 — fence strictness tightened via migrated path | 2 | Bead 2: `TestPlanFrontmatterFenceStrictnessTightened` |
| AC 9 — approval prose-scan deleted; frontmatter decides | 2 | Bead 2: `! grep 'func readSpecApprovalStatus'` + `TestSpecApprovalStatusFromFrontmatter` |
| AC 10 — three R7 sites load per-domain-not-per-file | 3 | Bead 3: three seam-counting tests |
| AC 11 — each cited ADR parsed at most once per run | 3 | Bead 3: `TestADRParsedOncePerValidationRun` |
| AC 12 — contextpack `BuildBead` goldens still pass | 2 | Bead 2: `TestContextPackDeterministic`, `TestContextPackFlatVsCanonicalByteIdentical` |
| AC 13 — validate diagnostics byte-identical (R7+R8) | 3 | Bead 3: `TestValidateDiagnosticsByteIdenticalAfterCaching` |
| AC 14 — doctor walks tree once; report unchanged | 4 | Bead 4: `TestOwnershipCheckWalksTreeOnce` |
| AC 15 — `go test ./...` green | 1–4 | Every bead: harness-excluded full suite |
| AC 16 — `validate spec 108` only `lifecycle-binding` warning | 1–4 | Final `mindspec validate spec` before impl-approve |
