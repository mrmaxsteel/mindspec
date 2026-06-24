---
adr_citations:
    - ADR-0036
    - ADR-0024
approved_at: "2026-06-16T19:43:53Z"
approved_by: user
bead_ids:
    - mindspec-nekh.1
    - mindspec-nekh.2
    - mindspec-nekh.3
spec_id: 100-ownership-gate-resolution
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
    - depends_on: []
      id: 2
    - depends_on: []
      id: 3
---
# Plan: 100-ownership-gate-resolution

## ADR Fitness

This plan applies three Accepted ADRs. Two (ADR-0036, ADR-0024) are cited in
`adr_citations` because they declare the impacted `workflow` domain. The third
(ADR-0032) is AMENDED by this spec but is NOT a frontmatter citation — its
current `Domain(s)` do not yet intersect `workflow`, so citing it would (rightly)
trip `adr-cite-irrelevant`; the amendment that adds `workflow` to its `Domain(s)`
is authored in Bead 1.

- **ADR-0036 (Ownership Discovery, Accepted; Domain(s): workflow, validation,
  doc-sync, ownership)** — the canonical decision that ownership is resolved from
  an explicit per-domain `OWNERSHIP.yaml` `paths:` glob with NO
  framework-synthesized fallback. Bead `mindspec-4ft2`'s normalization helper
  applies it directly: it resolves an Impacted-Domains entry to its owner by
  glob-matching the entry against every domain's EXPLICIT `paths:` manifest. It
  consumes declared data and explicit globs; it never synthesizes a fallback
  owner, and a zero/multi-owner entry is a hard ERROR (resolution is total and
  unambiguous). Path-PREFIX inference is explicitly NOT introduced (it would be
  the synthesized-fallback ZFC violation ADR-0036 removed).
- **ADR-0032 (ADR Semantic Gates, Accepted; Domain(s): validation, adr,
  lifecycle)** — defines the divergence/coverage/citation gate mechanism and
  (sub-decision 1) that the domain identifier is the `OWNERSHIP.yaml` dir-name
  with path-like identifiers REJECTED as ambiguous. **This plan AMENDS ADR-0032**
  (the way spec 099 amended ADR-0037): bead `mindspec-4ft2` ACCEPTS a path-like
  Impacted-Domains entry and RESOLVES it to its owning-domain dir-name when
  exactly one domain claims it, rather than rejecting it outright — erroring only
  on zero / more-than-one owners. The amendment note AND the addition of
  `workflow` to ADR-0032's `Domain(s)` line are authored as part of
  `mindspec-4ft2` (one bead owns the doc edit). ADR-0032 is amended-by, not a
  workflow-covering citation, so it is deliberately left out of `adr_citations`
  (see frontmatter note). R2 (coverage-missing hint) and R4 (`adr_citations` key)
  operate inside this gate mechanism without changing it.
- **ADR-0024 (ADR storage abstraction, Accepted; Domain(s): adr, context-system,
  workflow)** — the `adr.Store` / `FileStore` boundary that `adr show` /
  `adr list` read through. Bead `mindspec-3cfr`'s worktree-aware root resolution
  makes the show/list read path agree with the worktree-overlay store the
  validator already uses.

The two CITED ADRs both declare the single impacted domain **workflow** (ADR-0036
and ADR-0024 each list `workflow`), so `checkADRCoverage` finds `workflow`
covered and `checkADRCitations` finds no irrelevant citation. ADR-0032 (amended,
not cited) applies to the gate mechanism the workflow source implements; its
amendment is recorded in Bead 1.

## Testing Strategy

Test-first (RED → GREEN) per bead. Every test is hermetic and CI-runnable —
temp-repo fixtures with `MockExecutor`, no live git, no LLM-harness scenario. The
existing fixture machinery is reused (do NOT reinvent it):

- `internal/validate/divergence_test.go`: `writeSpecAndPlan(t, root, specDir,
  specID, impactedDomains, citationIDs)`, `writeADR(t, root, id, status,
  domains)`, and `executor.MockExecutor{ChangedFilesResult: [...]}`. Note
  `writeSpecAndPlan` writes the spec's `## Impacted Domains` bullets VERBATIM, so
  passing a file-path string (e.g. `internal/genevieve/review.py`) as an
  "impacted domain" is exactly the file-path-Impacted-Domains fixture R1 needs.
- `internal/validate/ownership_test.go`: `writeManifest(t, root, domain, body)`
  for `OWNERSHIP.yaml` (the overlapping-glob ambiguity fixture at L28-29 writes
  `alpha` and `beta` both claiming `internal/foo/**` — reuse that shape for the
  multi-owner ERROR case).
- `internal/validate/plan_test.go`: `writeTestSpec(t, root, impactedDomains)`,
  `writeTestADRWithDomains(t, root, id, status, domains, supersededBy)`,
  `makePlanWithCitations(t, root, citations, hasADRFitness)` — drive
  `ValidatePlan` / `checkADRCoverage` / `checkADRCitations`.
- `cmd/mindspec/adr_test.go`: `setupWorktreePair(t)` returns
  `(mainRoot, worktreeRoot)` with a hand-crafted git-worktree linkage (main
  `.git/worktrees/wt-adr` gitdir+commondir, worktree `.git` FILE), plus the
  `Chdir`-with-`t.Cleanup` helper — the exact pattern for exercising
  `FindLocalRoot`-vs-`FindRoot` root resolution for `adr show` / `adr list`.
- `internal/adr/parse_test.go`: `TestParseADR` table style (content string →
  `ParseADR` → assert `Domains`) for the non-list `**Domain(s)**:` regression.

Gate commands per bead (NEVER `go test ./internal/harness/...`):
`go build ./...` + filtered `go test -run <Name> -timeout 120s ./internal/...`
(or `./cmd/...` for R3 CLI tests).

## Decomposition (heuristic justification)

Three beads, NOT one-per-requirement. Justification against the mindspec plan
heuristics:

- **File-overlap / merge-signal (R>0.5) drives the R1+R2 merge.** R1 and R2 both
  modify the SAME function `checkADRCoverage` in `internal/validate/plan.go`
  (R1 feeds it the normalized domain set via `loadImpactedDomains` at ~L124; R2
  edits its `adr-coverage-missing` message at ~L529). Two agents editing one
  function would collide on every merge. They are fused into ONE bead
  (`mindspec-4ft2`). R1 is the substantive piece; R2 is a one-line message edit in
  the function R1 already touches, so the marginal cost of folding it in is near
  zero (trivial-work heuristic) and the merge cost of splitting is high.
- **Independence justifies the other two beads staying separate.** R3
  (`cmd/mindspec/adr.go` + `internal/adr/parse_test.go`) and R4
  (`internal/approve/spec.go` + the `adr-citations` WARN at `plan.go` ~L141)
  touch disjoint files from each other and from the R1 `checkADRCoverage`
  function. R4's `plan.go` edit (~L141, `checkFrontmatterFields`/empty-citations
  branch) is a DIFFERENT function from R1's `checkADRCoverage` (~L516) and
  `checkADRCitations` (~L465), so the two `plan.go` touchpoints do not collide at
  the hunk level — but to be safe the merge order is R1 first (it is the larger
  `plan.go` change), then R4 rebases its small ~L141 edit on top. This is captured
  as a sequencing preference, not a hard `depends_on` edge (see below).
- **Target band + serial-chain.** 3 beads sits inside the 3–5 target. The
  dependency graph is fully parallel (longest serial chain = 1), well under the
  ≤3 limit. No bead consumes another's runtime output: R3 and R4 do not read R1's
  normalization helper, and R1 does not read R3/R4. Per spec 097 R3, only a
  genuine output-consumption relationship earns a `depends_on` edge; a mere
  merge-order preference does not, so all `depends_on` arrays are empty.

**work_chunks depends_on graph** (structured frontmatter — the wired form;
`id` is the 1-based positional index of the `## Bead N` section per the spec 097
R3 schema, `depends_on` is a `[]int`):

```
id 1 = ## Bead 1 (mindspec-4ft2, R1+R2)  depends_on: []
id 2 = ## Bead 2 (mindspec-3cfr, R3)     depends_on: []
id 3 = ## Bead 3 (mindspec-gpoq, R4)     depends_on: []
```

All three are independent / parallelizable. (Bead `mindspec-3d84` from the spec —
the R2 coverage-hint — is absorbed into `mindspec-4ft2`; it is not a separate
work_chunk because it edits the same function as R1.)

---

## Bead 1 (mindspec-4ft2) — R1 normalize-at-source + R2 adr-coverage-missing hint

**Satisfies spec ACs:** all five R1 ACs (L177–198), both R2 ACs (L208–213), and
the spec-level R1 + R2 ACs (L307–315). Authors the ADR-0032 amendment note.

**Steps**

1. Write the RED helper + gate tests first (see the test list below the steps),
   confirm they fail.
2. Add the shared normalization helper (changed-files item 1).
3. Wire it into `divergence.go` ~L142 (changed-files item 2).
4. Wire it into `plan.go` `loadImpactedDomains`/`ValidatePlan` feeding both
   plan-time gates (changed-files item 3a).
5. Rewrite the R2 `adr-coverage-missing` message in `checkADRCoverage`
   (changed-files item 3b).
6. Author the ADR-0032 amendment note (changed-files item 4).
7. Run the gate command until GREEN.

**Changed files**

- `internal/validate/ownership_resolve.go` *(NEW)* — the shared normalization
  helper. **Placement decision: `internal/validate`, not `internal/ownership`.**
  Justification: the helper must reuse `resolveDomains` (ownership.go L211),
  `listDomainDirs` (docsync.go L351), `LoadOwnership`/`loadOwnershipForRef`
  (ownership.go), `matchesAny`/`GlobMatch` (ownership.go L269/L294), and feed
  `divergence.go` + `plan.go` — ALL of which already live in `internal/validate`
  and are mostly unexported. Putting the helper in `internal/validate` reuses
  them with ZERO new exports and adds NO new cross-package edge.
  `internal/ownership` (populate/source/stub) does not own these primitives, so
  placing it there would force exporting them and create a `validate→ownership`
  dependency for no benefit. Both packages are `workflow`-owned
  (`internal/validate/**` and `internal/ownership/**` are both in the workflow
  `OWNERSHIP.yaml` `paths:`), so the choice is layering-driven, not
  ownership-driven. Critically, ParseSpec/`contextpack` is NOT made to depend on
  `validate` — normalization lives in the gate layer, the parser stays dumb.
  - New func, e.g. `normalizeImpactedDomains(exec executor.Executor, root,
    ownerRef string, entries []string) (normalized []string, errs []string)`:
    for each raw entry — (1) if `.mindspec/domains/<entry>/OWNERSHIP.yaml`
    exists (domain-dir check, via `resolveDomains`/`listDomainDirs` membership),
    KEEP it verbatim; (2) else glob-match the entry against every domain's
    `paths:` (reuse `loadOwnershipForRef` + `matchesAny`) and collect owners;
    (3) exactly-one owner → REPLACE with that owner's dir-NAME; (4) zero owners →
    ERROR naming the entry, stating no `OWNERSHIP.yaml` claims it; (5) >1 owner →
    ambiguity ERROR naming the entry and the conflicting owners. Returns the
    resolved domain-NAME set + any errors. Mirror the `ownerRef`/`exec` plumbing
    of `attributeDomain` so it reads the same tree (working-tree on disk for the
    plan path; ref for the divergence path).
- `internal/validate/divergence.go` (~L142) — replace `candidateDomains :=
  append([]string(nil), meta.Domains...)` with a call to
  `normalizeImpactedDomains(exec, root, ownerRef, meta.Domains)`; surface any
  normalization errors via `r.AddError(...)` and return early on error.
  PRESERVE the empty-`meta.Domains` fallback (enumerate all domain dirs) and the
  per-file `attributeDomain` loop + blast-radius guard UNCHANGED — the candidate
  set is still the resolved DECLARED domains, so a changed file owned by an
  undeclared domain still fails `adr-divergence-uncovered`.
- `internal/validate/plan.go`:
  - `loadImpactedDomains` (~L730) → route its result through the normalization
    helper, OR call the helper at the `ValidatePlan` call site (~L124) before
    feeding `checkADRCoverage`/`checkADRCitations`. (Plan path uses `ownerRef ==
    ""` → on-disk working-tree read; needs `root` + an executor — confirm the
    `ValidatePlan(root, specID)` signature can build one, else use the on-disk
    `LoadOwnership`/`resolveDomains` branch which needs no executor.) Surface
    normalization errors as a new diagnostic (e.g. `impacted-domains-resolve`).
  - `checkADRCoverage` (~L529) **(R2)** — rewrite the `adr-coverage-missing`
    message so that when ≥1 Accepted ADR is cited it presents BOTH remedies
    (add/declare the domain on an existing cited Accepted ADR, AND
    `mindspec adr create`); when NO ADR is cited it keeps the
    `mindspec adr create --domain <X>` remedy. The function already has
    `citations` in scope to branch on `len(citations)`.
- `.mindspec/adr/ADR-0032-adr-semantic-gates.md` — ADR-0032 amendment note
  (sub-decision 1 softened): path-like Impacted-Domains entries are NORMALIZED to
  their owning-domain dir-name when exactly one domain's `OWNERSHIP.yaml` claims
  them, erroring only on zero / more-than-one owners — the canonical dir-name is
  still the compared identifier, reached by resolution rather than rejection.
  Also add `workflow` to the `**Domain(s)**:` line (this spec's workflow source
  amends the gate ADR-0032 governs), so a future plan may cite it for workflow.
  (Doc/process artifact — `isProcessArtifact`→`isDocFile` skips it at the
  divergence gate, so it needs no `OWNERSHIP.yaml` claim.)

**RED tests first (then GREEN)**

In `internal/validate/ownership_resolve_test.go` (NEW) — helper unit tests
(`TestNormalizeImpactedDomains*`, reusing `writeManifest`):
- `*KeepsDomainDirNameVerbatim` — entry that IS a domain dir name is returned
  unchanged, no glob-match attempted.
- `*GlobResolvesPathEntry` — `writeManifest(t, root, "workflow",
  "paths:\n  - internal/validate/**\n")`; entry `internal/validate/plan.go`
  resolves to `["workflow"]`.
- `*ZeroOwnerErrors` — an entry no manifest claims → error slice names the entry.
- `*AmbiguousOwnerErrors` — reuse the L28-29 overlapping-glob shape
  (`alpha` + `beta` both claim `internal/foo/**`); entry `internal/foo/x.go` →
  ambiguity error naming both `alpha` and `beta`.

In `internal/validate/divergence_test.go` — gate tests
(`TestValidateDivergence*`, reusing `writeSpecAndPlan`/`writeADR`/`writeManifest`/
`MockExecutor`):
- `*FilePathImpactedDomainResolves` (R1 AC1, L177) —
  `writeSpecAndPlan(..., impactedDomains=["internal/genevieve/review.py"], ...)`,
  `writeManifest(t, root, "genevieve", "paths:\n  - internal/genevieve/**\n")`,
  `writeADR("ADR-0099","Accepted",["genevieve"])`, changed file
  `internal/genevieve/review.py` → ZERO `adr-divergence-unowned`,
  `coveredAccepted` silent pass.
- `*NamedDomainUnchanged` (R1 AC3, L188) — same fixture but
  `impactedDomains=["genevieve"]` → still passes, no new false positives.
- `*GenuinelyUnownedStillFails` (R1 AC4, L194) — a changed file no manifest
  `paths:` matches → still `adr-divergence-unowned` (mirror existing
  `TestUnownedFileRejected`).
- **Existing `TestCompleteRejectsUndeclaredDomainTouch` (L97) and
  `TestUnownedFileRejected` (L177) MUST still pass** — blast-radius guard intact
  (R1 AC3, L188).

In `internal/validate/plan_test.go` — plan-gate consistency + R2 message
(`TestPlanCoverage*` / `TestCheckADRCitations*`, reusing
`writeTestSpec`/`writeTestADRWithDomains`/`makePlanWithCitations`):
- `TestPlanCoverageFilePathImpactedResolves` (R1 AC2, L183) — file-path impacted
  domain + manifest + cited Accepted ADR → NO spurious `adr-coverage-missing`.
- `TestCheckADRCitationsFilePathImpactedResolves` (R1 AC2, L183) — same fixture →
  NO spurious `adr-cite-irrelevant`.
- `TestNormalizeZeroAndAmbiguousAtPlanGate` (R1 AC5, L196) — zero-owner and
  multi-owner Impacted-Domains entries surface the clear resolution ERROR.
- `TestPlanCoverageHintMentionsExistingADR` (R2 AC1, L208) — uncovered domain +
  a cited Accepted ADR → `adr-coverage-missing` text mentions
  adding/declaring the domain on an existing cited ADR.
- `TestPlanCoverageHintCreateWhenNoCitation` (R2 AC2, L212) — no ADR cited →
  message still surfaces the `adr create` remedy.

**Verification**

- [ ] `go build ./...` succeeds.
- [ ] `go test -run 'TestNormalizeImpactedDomains|TestValidateDivergence|TestCompleteRejectsUndeclaredDomainTouch|TestUnownedFileRejected|TestPlanCoverage|TestCheckADRCitations' -timeout 120s ./internal/validate/...` passes (new R1/R2 tests GREEN; existing blast-radius tests still GREEN).

**Acceptance Criteria**

- [ ] R1 AC1: bead-time divergence resolves a file-path Impacted-Domains entry to
  its owner and reports zero `adr-divergence-unowned` (spec L177).
- [ ] R1 AC2: the same file-path spec passes `checkADRCoverage` (no
  `adr-coverage-missing`) and `checkADRCitations` (no `adr-cite-irrelevant`) (L183).
- [ ] R1 AC3: named-domain path unchanged and `TestCompleteRejectsUndeclaredDomainTouch`
  still fails (blast-radius guard intact) (L188).
- [ ] R1 AC4: a genuinely-unowned changed file still reports `adr-divergence-unowned`
  (L194).
- [ ] R1 AC5: zero-owner and ambiguous-owner entries produce the clear ERROR (L196).
- [ ] R2 AC1: `adr-coverage-missing` mentions the add-Domain-to-existing-ADR remedy
  when an Accepted ADR is cited (L208).
- [ ] R2 AC2: the message still surfaces `adr create` when no ADR is cited (L212).
- [ ] ADR-0032 amendment note authored (spec R1 design decision).

**Depends on**

None.

---

## Bead 2 (mindspec-3cfr) — R3 adr show/list worktree-aware + parse regression

**Satisfies spec ACs:** both R3 ACs (L228–233) and the spec-level R3 AC (L316).

**Steps**

1. Write the RED `TestAdrShowWorktree*` and `TestParseADR_NonListDomainLine`
   tests first; confirm the show/list test fails on the current `FindRoot` path.
2. Make `adrListCmd.RunE` worktree-aware (changed-files item 1a).
3. Make `adrShowCmd.RunE` worktree-aware (changed-files item 1b).
4. Run the gate command until GREEN (parser is unchanged — its test is a lock).

**Changed files**

- `cmd/mindspec/adr.go`:
  - `adrListCmd.RunE` (~L97) — change `workspace.FindRoot(cwd)` to the
    worktree-aware resolution so a worktree-local ADR is visible, consistent with
    `adr create` (`FindLocalRoot`, L32) and the validator's `adrStoreForSpec`
    overlay. Prefer the overlay store (worktree branch ADRs unioned over main),
    matching `adrStoreForSpec`'s read semantics, rather than only swapping
    `FindRoot`→`FindLocalRoot` (so a worktree ADR is found AND main-only ADRs stay
    visible).
  - `adrShowCmd.RunE` (~L138) — same root-resolution change for `store.Get`.
- `internal/adr/parse_test.go` — TEST ONLY regression; NO change to `parse.go`.

**RED tests first (then GREEN)**

In `cmd/mindspec/adr_test.go` (`TestAdrShowWorktree*`, reusing
`setupWorktreePair`/`Chdir` helpers): create an ADR ONLY in the worktree-local
`.mindspec/adr/`, `Chdir` into the worktree, run the show (and list)
root-resolution path → the ADR is found and rendered with its `Domain(s)`,
where the pre-fix `FindRoot` path (resolving back to main) would have missed it.

In `internal/adr/parse_test.go` (`TestParseADR_NonListDomainLine`, table style):
ADR content with `**Domain(s)**: foo, bar` (the NON-list form, no leading `- `) →
`ParseADR` yields `Domains == ["foo","bar"]`. Locks current behaviour
(`parse.go` L74 `strings.Contains(trimmed, "**Domain(s)**:")` already accepts
both forms); parser unchanged.

**Verification**

- [ ] `go build ./...` succeeds.
- [ ] `go test -run 'TestAdrShowWorktree|TestParseADR' -timeout 120s ./cmd/... ./internal/adr/...` passes.

**Acceptance Criteria**

- [ ] R3 AC1: an ADR present only in the worktree-local `.mindspec/adr/` is
  found and rendered with its `Domain(s)` by `adr show`/`adr list` (L228).
- [ ] R3 AC2: a non-list `**Domain(s)**: foo, bar` line parses to
  `Domains == ["foo","bar"]` (parser unchanged, behaviour locked) (L231).

**Depends on**

None.

---

## Bead 3 (mindspec-gpoq) — R4 scaffold adr_citations + WARN names the key

**Satisfies spec ACs:** both R4 ACs (L242–246) and the spec-level R4 AC (L317).

**Steps**

1. Write the RED `TestScaffoldPlanEmitsADRCitations` and
   `TestValidatePlanCitationsWarnNamesKey` tests first; confirm they fail.
2. Add the `adr_citations` key to `scaffoldPlan`'s frontmatter (changed-files item 1).
3. Make the `adr-citations` WARN/ERROR text name the `adr_citations` key
   (changed-files item 2).
4. Run the gate command until GREEN.

**Changed files**

- `internal/approve/spec.go` — `scaffoldPlan` (~L228): add an `adr_citations`
  key to the generated YAML frontmatter (between `spec_id`/`version` and the
  closing `---`), empty-but-named (e.g. `adr_citations: []` or a commented
  `# adr_citations:\n#   - ADR-XXXX`) so the author sees the exact key the gate
  reads. Keep `status`/`spec_id`/`version` intact.
- `internal/validate/plan.go` — the `adr-citations` empty-citations diagnostic
  (~L141 WARN, and the ~L143 ERROR for completeness) message text MUST contain
  the literal string `adr_citations` so the fix is unambiguous. **No collision
  with bead 4ft2:** this edits the empty-citations branch inside `ValidatePlan`
  (~L137–144), a different function from `checkADRCoverage` (~L516) /
  `checkADRCitations` (~L465). Merge R1 first, then rebase this small edit.

**RED tests first (then GREEN)**

In `internal/approve/spec_test.go` (`TestScaffoldPlanEmitsADRCitations`):
`scaffoldPlan("100-x")` output contains the literal `adr_citations` key within
the YAML frontmatter region (between the opening and closing `---`).

In `internal/validate/plan_test.go` (`TestValidatePlanCitationsWarnNamesKey`,
reusing `makePlanWithCitations` with empty citations + an `## ADR Fitness`
section to hit the WARN branch): the `adr-citations` message emitted by
`ValidatePlan` contains the string `adr_citations`.

**Verification**

- [ ] `go build ./...` succeeds.
- [ ] `go test -run 'TestScaffoldPlan|TestValidatePlanCitations' -timeout 120s ./internal/approve/... ./internal/validate/...` passes.

**Acceptance Criteria**

- [ ] R4 AC1: `scaffoldPlan(specID)` output contains the literal `adr_citations`
  key in the YAML frontmatter region (L243).
- [ ] R4 AC2: the `adr-citations` WARN/ERROR message emitted by `ValidatePlan`
  contains the string `adr_citations` (L245).

**Depends on**

None.

---

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| R1 AC1 file-path divergence resolves (L177) | mindspec-4ft2 · `TestValidateDivergenceFilePathImpactedDomainResolves` |
| R1 AC2 plan-time gates consume normalized set (L183) | mindspec-4ft2 · `TestPlanCoverageFilePathImpactedResolves`, `TestCheckADRCitationsFilePathImpactedResolves` |
| R1 AC3 named-domain unchanged + blast-radius guard (L188) | mindspec-4ft2 · `TestValidateDivergenceNamedDomainUnchanged`, existing `TestCompleteRejectsUndeclaredDomainTouch` |
| R1 AC4 genuinely-unowned still fails (L194) | mindspec-4ft2 · `TestValidateDivergenceGenuinelyUnownedStillFails`, existing `TestUnownedFileRejected` |
| R1 AC5 zero/ambiguous-owner ERROR (L196) | mindspec-4ft2 · `TestNormalizeImpactedDomains{ZeroOwnerErrors,AmbiguousOwnerErrors}`, `TestNormalizeZeroAndAmbiguousAtPlanGate` |
| R2 AC1 hint mentions existing-ADR remedy (L208) | mindspec-4ft2 · `TestPlanCoverageHintMentionsExistingADR` |
| R2 AC2 hint keeps create remedy when no citation (L212) | mindspec-4ft2 · `TestPlanCoverageHintCreateWhenNoCitation` |
| R3 AC1 worktree-local ADR visible to show/list (L228) | mindspec-3cfr · `TestAdrShowWorktree*` |
| R3 AC2 non-list Domain(s) parse locked (L231) | mindspec-3cfr · `TestParseADR_NonListDomainLine` |
| R4 AC1 scaffold emits adr_citations (L243) | mindspec-gpoq · `TestScaffoldPlanEmitsADRCitations` |
| R4 AC2 WARN/ERROR names adr_citations (L245) | mindspec-gpoq · `TestValidatePlanCitationsWarnNamesKey` |
| Build + tests green (L320) | all beads · `go build ./...` + filtered `go test -run <Name> -timeout 120s ./internal/... ./cmd/...` |
