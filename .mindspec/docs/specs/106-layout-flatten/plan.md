---
adr_citations:
    - ADR-0037
    - ADR-0018
    - ADR-0023
    - ADR-0025
approved_at: "2026-06-24T00:03:09Z"
approved_by: user
bead_ids:
    - mindspec-3d3i.1
    - mindspec-3d3i.2
    - mindspec-3d3i.3
    - mindspec-3d3i.4
    - mindspec-3d3i.5
    - mindspec-3d3i.6
spec_id: 106-layout-flatten
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/workspace/workspace.go
        - internal/workspace/worktree.go
        - internal/bootstrap/bootstrap.go
        - internal/workspace/workspace_test.go
      title: Phase 1 — per-artifact three-tier resolvers + DetectLayout + shared layout-signature helper + flat worktree tier + born-flat bootstrap
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/validate/docsync.go
        - internal/validate/divergence.go
        - internal/validate/ownership.go
        - internal/contextpack/budgeter.go
        - internal/spec/list.go
        - internal/domain/list.go
        - internal/domain/show.go
        - internal/doctor/docs.go
      title: Phase 1 — diff-string layout classifier + permanently multi-prefix gate matchers (ADDITIVE reviews) + tier-aware enumerating consumers
    - depends_on: []
      id: 3
      key_file_paths:
        - internal/layout/mover.go
        - internal/layout/runstate.go
        - internal/layout/rewrite.go
        - internal/executor/executor.go
        - internal/gitutil/gitops.go
        - internal/doctor/links.go
        - cmd/mindspec/migrate.go
        - .mindspec/docs/domains/workflow/OWNERSHIP.yaml
      title: Phase 2 — net-new internal/layout mover + executor/gitutil mover primitives + link-rewriter + run-state + lineage + doctor link-check + migrate layout CLI
    - depends_on:
        - 1
      id: 4
      key_file_paths:
        - internal/panel/panel.go
        - internal/panel/gate.go
        - internal/complete/complete.go
        - internal/executor/mindspec_executor.go
        - internal/doctor/migration.go
      title: Phase 3 — layout-aware (transitional) reviews panel scan + complete gate + DIRECTIONAL merge-time layout-fingerprint hard-fail at the REAL executor seams + doctor layout detection
    - depends_on:
        - 2
        - 3
        - 4
      id: 5
      key_file_paths:
        - .mindspec/docs/specs
        - .mindspec/docs/glossary.md
        - .mindspec/policies.yml
        - review
        - .mindspec/docs/domains/workflow/OWNERSHIP.yaml
      title: Phase 3 — execute the IRREVERSIBLE flatten via the mover — flatten specs/adr/domains/core + context-map, evict dogfood → project-docs, migrate root review → reviews, drop vestigial files
    - depends_on:
        - 5
      id: 6
      key_file_paths:
        - internal/setup/skills.go
        - internal/setup/claude.go
        - internal/setup/codex.go
        - internal/instruct/templates/spec.md
        - cmd/mindspec/migrate.go
        - internal/setup/historical_skills
        - .mindspec/core/DOCS-LAYOUT.md
      title: Phase 3 — post-move cleanup — flat-path skills/snapshots/setup-text + governance (DOCS-LAYOUT, ADR-0039, ADR-0037 amend) + migrate rubric + harness/testdata fixture migration
---
# Plan: 106-layout-flatten

> Six beads on the spec's mandated 3-phase spine, with the **irreversible
> filesystem move isolated into its own reviewable bead** (R6 blocker 4).
> **Phase 1** (Beads 1+2) is a behavior-preserving, ZERO-layout-change refactor
> that lands fully green against the EXISTING canonical + legacy forms while
> already recognizing the flat prefix in reads (Bead 1) and diff-string matchers
> (Bead 2). **Phase 2** (Bead 3) is the net-new, golden-tested `internal/layout`
> mover — INCLUDING the net-new executor/gitutil git-mv / hard-reset / clean /
> ref-discovery primitives it requires (R4 blocker 2) and the net-new doctor
> link-check lane (R2 major 9) — that never touches the live tree until invoked.
> **Phase 3** is split into three beads: Bead 4 wires only the CODE that must
> RECOGNIZE the flat tree (a LAYOUT-AWARE / transitional panel scan + `complete` gate
> — both root `review/` and co-located `<spec-dir>/reviews/` while the tree is still
> canonical, co-located only once flat — the DIRECTIONAL merge-time layout-fingerprint
> hard-fail at the REAL executor merge seams, and the doctor layout detector) — no file
> moves, no static-text rewrites; Bead 5
> EXECUTES the one-way flatten through the Phase-2 mover and migrates the existing
> root `review/**` tree; Bead 6 is the post-move cleanup (flat-path skills/snapshots,
> governance, migrate rubric, harness/testdata fixtures) that can only run once the
> files have physically moved. The DAG is a 2-wide fan-out (roots: Bead 1 and Bead 3)
> converging on the move bead, then a single intrinsic move→cleanup edge. Longest
> serial chain = 4 (1 → 2 → 5 → 6 and 1 → 4 → 5 → 6); the depth-3 heuristic is
> deliberately overridden — see Decomposition rationale.

## ADR Fitness

The spec declares FOUR impacted domains — **workflow, core, execution,
context-system** — and the four cited ADRs are all **Accepted** and jointly cover
every one of them, so `adr-coverage` passes at bead-complete and no cite is
irrelevant:

- **ADR-0037 (Panel Gate as Enforced Contract)** — Accepted; Domain(s)
  **workflow, execution** (both impacted). This spec RELOCATES the
  `review/<slug>/panel.json` registration convention to the spec-scoped tracked
  `<spec-dir>/reviews/<panel-slug>/panel.json` and makes the `mindspec complete`
  gate scan LAYOUT-AWARE (Bead 4 wires the transitional scan — both root `review/` and
  co-located `<spec-dir>/reviews/` while the tree is canonical, co-located only once
  flat; Bead 5 migrates the existing root `review/**` artifacts and removes the root
  tree) — it AMENDS, does not change, the contract: the
  registration, round derivation, N−1 threshold, staleness, dirty-tree, and
  fail-open/closed semantics are all untouched. The ADR-0037 amendment note and the
  new layout-v2 ADR record the new `panel.json` LOCATION only, and they land in the
  post-move governance bead (Bead 6) so the amendment describes the SHIPPED tree.
- **ADR-0018 (Lean Bootstrap — Remove Glossary and Policies Subsystems)** —
  Accepted; Domain(s) **core, context-system** (both impacted). The spec drops the
  residual `glossary.md`, `policies.yml`, and the stale
  `internal/instruct/templates/spec.md:35` glossary line. The two on-disk file drops
  ride with the move bead (Bead 5, as the low-stakes exercise of the mover); the
  template-line drop also lands in Bead 5. This is the ADR-0018-CONSISTENT cleanup
  of files the ADR's subsystem-removal left on disk — a vestigial drop, not a new
  decision; it does not violate the ADR, it completes it.
- **ADR-0023 (Beads as Single State Authority — forward-only lifecycle)** —
  Accepted; Domain(s) **workflow, git, state** (workflow impacted). The flatten is
  forward-only: the lifecycle cannot rewind, so the cross-layout merge guard (Bead 4)
  HARD-FAILS a nested-onto-flat merge (the REGRESSION direction — a canonical/legacy
  source onto a flat target; the flat-onto-canonical MIGRATION direction is deliberately
  allowed so the flatten can land, run-state exempt) with a rebase recovery line —
  installed at the REAL local merge seams (`MindspecExecutor.CompleteBead`'s
  `gitutil.MergeInto` and `FinalizeEpic`'s `gitutil.MergeInto`/direct
  `gitutil.MergeBranch`; the remote path pushes for a PR and relies on the Bead-3
  precondition + PR review) rather than trusting git's heuristic rename detection — and
  the mover REFUSES to auto-rollback after
  publish (Bead 3). This APPLIES ADR-0023's forward-only authority to the layout cut;
  it does not supersede it.
- **ADR-0025 (`.beads/issues.jsonl` Is a Build Artifact)** — Accepted; Domain(s)
  **workflow, execution, bootstrap** (workflow + execution impacted). The diff-string
  classifier (Bead 2) continues to treat process artifacts the same way, and
  co-locating reviews under `<spec-dir>/reviews/` keeps the tracked-but-non-blocking
  dirty-tree semantics consistent. Applied, not changed.

ADR-coverage check: workflow → {0037, 0023, 0025}; core → {0018}; execution →
{0037, 0025}; context-system → {0018}. Every impacted domain is covered by ≥1
cited Accepted ADR → `adr-coverage` passes; every cite intersects ≥1 impacted
domain → no `adr-cite-irrelevant`.

**ADR-0022 (Worktree-Aware Spec Resolution) is deliberately NOT cited.** It is
**Proposed**, and relying on a Proposed ADR as sole coverage for any domain would
only earn the advisory `adr-coverage-proposed` warning; the four Accepted ADRs
above already cover all four domains, so ADR-0022 is unnecessary as a cite. It
remains architecturally RELEVANT — this spec EXTENDS its worktree → canonical →
legacy resolution order into a per-artifact, three-tier flat → canonical → legacy
resolver (Bead 1) — but that extension is formalized by the NEW layout-v2 ADR
below, not by leaning on a Proposed cite.

**A NEW ADR-0039 (layout-v2) is warranted and is created in the post-move
governance bead (Bead 6)** via `mindspec adr create` (the plan SPECIFIES it; it is
NOT created at plan-authoring time, mirroring the spec-105 convention of never
citing an unwritten id as a touchpoint). It formalizes (a) the flat
`.mindspec/{specs,adr,domains,core}` + top-level `project-docs/` layout, (b) the
per-artifact three-tier first-exists-wins resolver and the whole-tree `DetectLayout`
write-default, (c) the PERMANENT multi-prefix git-ref matcher posture (decoupled
from the filesystem read-tier deprecation lifecycle), and (d) the ADR-0037
reviews-location amendment. **No existing Accepted ADR is violated or superseded.**
Specifically, the flatten EXTENDS ADR-0022's resolution order (an additional
first-precedence tier) rather than superseding it, so an EXTENSION — recorded in
ADR-0039 — suffices; ADR-0022 needs no `Superseded-by`, and when it is later
flipped to Accepted it and ADR-0039 are consistent. The reviews relocation is an
ADR-0037 AMENDMENT (location only), not a supersede. ADR-0039 lands **Proposed and
uncited**, so it earns no `adr-divergence-proposed` block.

## Testing Strategy

Per-bead tests are **package-scoped** and run by the bead subagent; the harness
scenarios are the orchestrator's job. **NEVER run `go test ./internal/harness/...`
inside a bead subagent** — harness scenarios are exercised only by the orchestrator
in the main loop after a bead's package tests are green.

### Per-bead gating tests (the "tests PASS" contract each bead must meet)

- **Bead 1 (core resolvers + DetectLayout + signature helper + born-flat bootstrap):**
  `go test ./internal/workspace/... ./internal/bootstrap/...`. The resolver matrix
  exercises every accessor (`SpecDir`, `ADRDir`, `DomainDir`, `ContextMapPath`,
  `RecordingDir`) on THREE fixtures — canonical (`.mindspec/docs/…`), legacy
  (`root/docs/…`), and flat (`.mindspec/{specs,adr,domains,core,context-map.md}`)
  — asserting byte-identical resolution on canonical/legacy with NO flat tree
  (AC1) and first-exists-wins flat resolution (AC2). `DetectLayout` is asserted for
  all five states incl. the `mixed` hard error and the recorded-recovery exception
  (AC3). The pure layout-signature classifier (the shared helper of minor 12 — see
  below) is unit-tested for each tree shape. Both worktree shapes (nested + flat)
  plus the `TreeRootForSpecDir` flat-shape ew79 regression are pinned (AC12).
  Bootstrap writes the flat manifest and `DetectLayout` classifies the bootstrapped
  tree `flat` (AC4).
- **Bead 2 (diff-string classifier + multi-prefix matchers + consumers):**
  `go test ./internal/validate/... ./internal/contextpack/... ./internal/spec/...
  ./internal/domain/... ./internal/doctor/...`. (`internal/spec` + `internal/domain`
  are added per R2 major 8: `spec list` lives in `internal/spec/list.go` and
  `domain list|show` in `internal/domain/list.go`/`show.go`, both joining
  `workspace.DocsDir`, so AC5's enumerating-consumer half is exercised where the
  code actually lives.) A three-prefix equivalence table asserts a legacy-,
  canonical-, and flat-prefix diff classify identically across `isDocFile`/
  `isSourceFile`, the spec-artifact literals, the cmd-docs accept-set,
  `listDomainDirs`/`TreeDirsAtRef`, `LoadOwnership`, AND — critically — the
  ref-anchored `LoadOwnershipAtRef`/`domainManifestRelPath` pair and
  `isProcessArtifact` (AC11). The `isProcessArtifact` test asserts that BOTH a root
  `review/<slug>/...` path AND a `<spec-dir>/reviews/<slug>/...` path classify
  non-source (the ADDITIVE matcher — see Bead 2 step 3). `project-docs/**` is
  classified non-source docs (AC11/AC21). The context-pack builder assembles
  byte-identical sections on a flat vs an otherwise-identical canonical fixture
  (AC6); the enumerating consumers (spec list, domain list/show, doctor scans)
  return the same inventory on flat vs canonical (AC5).
- **Bead 3 (net-new mover + executor/gitutil primitives + link-check, golden-tested):**
  `go test ./internal/layout/... ./internal/gitutil/... ./internal/executor/...
  ./internal/doctor/... ./cmd/mindspec/...`. (`internal/gitutil` + `internal/executor`
  are added per R4 blocker 2: the mover's git-mv / hard-reset / clean / branch+ref
  discovery primitives are NET-NEW and have no home today — they are added to
  `internal/gitutil` (the git-process I/O boundary) and surfaced on the `Executor`
  interface so `internal/layout` calls them through the ADR-0030 boundary, not by
  shelling out. `internal/doctor` is added per R2 major 9: the link-existence lane is
  net-new and lands in `internal/doctor/links.go`.) A golden-file test over a CAPTURED
  COPY of the real `.mindspec/docs` tree asserts: the deterministic post-migration
  tree; that each move's FIRST commit is a 100%-similarity rename (`git log --follow`
  survives); idempotent re-run is a no-op; hard-reset-to-pre-run-ref rollback on an
  injected mid-run failure (nothing published); crash-resume at EVERY run-state
  boundary (AC7/AC8). The lineage manifest under `.mindspec/lineage/` parses under
  the doctor migration-metadata schema (AC9). The doctor link-existence lane scans
  EVERY link in the moved tree + affected root docs and FAILS on both an injected
  dangling rewritten link AND an un-rewritten breaking link the finite set missed
  (AC10 build half). Precondition tests: an unmerged pre-flatten branch BLOCKS; a
  locked worktree + simulated external-fork ref do NOT (AC16). The `migrate` test
  asserts the `internal/layout` package is OWNED (no `adr-divergence-unowned`) (AC21).
- **Bead 4 (panel/merge/doctor wiring — code only, no moves, no skills):**
  `go test ./internal/panel/... ./internal/complete/... ./internal/executor/...
  ./internal/doctor/...`. The LAYOUT-AWARE `Scan` (mode chosen via the Bead-1
  `DetectLayout`/signature helper) finds, on a canonical fixture, BOTH a co-located
  `<spec-dir>/reviews/<slug>/panel.json` and a repo-root `review/<slug>/panel.json`, and
  a sub-threshold panel in EITHER blocks `complete`; on a flat fixture only the
  co-located panel drives the gate (AC13); doctor reports the detected layout, emits
  `would-migrate-layout`, and ERRORs on a dual-layout duplicate id (AC14); the
  DIRECTIONAL cross-layout merge guard hard-fails a canonical/legacy-source-onto-flat-target
  merge (regression) with the rebase recovery line and mutates nothing, while a
  flat-source-onto-canonical-target merge (migration) is allowed and the block is exempt
  under a recorded run-state — exercised at `internal/executor` where the guard is
  installed, in front of `gitutil.MergeInto` (`CompleteBead`/`FinalizeEpic`) and the
  direct no-remote `gitutil.MergeBranch` (`FinalizeEpic`); the remote path pushes for a
  PR and is covered by the Bead-3 precondition + PR review (AC15). No `internal/setup`
  here — the skills/snapshot rewrites moved to Bead 6 (R6 blocker 6).
- **Bead 5 (execute the irreversible move + review migration + vestigial drops):**
  the moves themselves are exercised by the doctor link-check lane (AC10, green on
  the migrated tree), a resolver test that `ADRDir`/`DomainDir`/`ContextMapPath`
  resolve to flat `.mindspec/…` and NOT into `project-docs/` or a root `docs/` alias
  (AC18), the load-bearing `grep -rn 'GLOSSARY.md\|policies.yml' internal cmd | grep
  -v _test` (NO `*.go` filter, so it reaches the template `.md` line) is empty (AC19),
  a domain-manifest self-glob check (AC20), and assertions that the root `review/`
  tree is GONE and its 42 artifacts now live under `<spec-dir>/reviews/**`. This bead
  does NOT claim a full `go test ./...` (see AC22 reconciliation below) — the harness
  scenario fixtures still encode the canonical shape and are migrated in Bead 6.
- **Bead 6 (post-move cleanup — skills + governance + rubric + fixtures):**
  `go test ./internal/setup/... ./cmd/mindspec/...`, PLUS the harness/testdata fixture
  migration that lets the orchestrator re-run `go test ./...` green AFTER the move.
  A breadth-covering grep asserts NO path-bearing LIVE skill/setup-text surface retains
  a pre-flatten `.mindspec/docs/{specs,adr,domains,core}` path or a repo-root
  `review/<slug>/` panel path (AC17) — the embedded `internal/setup/historical_skills/*.md`
  provenance snapshots are EXPLICITLY EXCLUDED from this grep and recorded as
  intentionally frozen (byte-exact prior-shipped captures for the HC-6 refresh/cleanup
  byte-match; their pre-flatten paths are load-bearing). A `DOCS-LAYOUT.md` content grep asserts the flat layout is documented
  (AC23), and `go test ./cmd/mindspec/...` asserts the migrate rubric targets
  `project-docs/` + the flat children never `.mindspec/docs/user/` (AC24).

### The flat-layout test matrix (AC22) and the Phase-1 behavior-preservation gate

A flat-layout matrix is built incrementally and exercises the resolvers (Bead 1),
the classifier predicate + context-pack builder (Bead 2), the mover (Bead 3), the
panel scan + doctor detector + merge guard (Bead 4) on flat AND canonical/legacy
fixtures. The **Phase-1 behavior-preservation gate is the orchestrator's
responsibility at the Phase-1 tip (after Beads 1+2 merge):** run `go test ./...`
green AND `git diff --name-status main...<phase1-tip>` shows NO renames/moves under
`.mindspec/docs/` (using `--name-status` against the merge base, since committed
renames would not surface in `git status`).

**AC22 reconciled honestly (R6 blocker 7).** In Phase 1 NOTHING has moved: the
canonical lifecycle tree is physically present and the canonical resolver tier
still resolves it byte-for-byte (the flat tier is read-precedence-FIRST but the
flat dirs do not exist, so it never fires). The harness scenario files and testdata
fixtures still reference the still-present canonical `.mindspec/docs/…` shape, so
`go test ./...` is green for the reason that the fixtures match the on-disk tree — a
factual, not aspirational, claim. The FLAT path is exercised only by the new matrix
fixtures the beads add, never by mutating the live tree. The move (Bead 5) is the
point at which the live tree changes shape; at that moment the harness fixtures that
encode the physical `.mindspec/docs/…` shape no longer match, so they are migrated
to the flat shape in Bead 6 (the post-move cleanup), and the orchestrator re-runs
the full suite green only AFTER Bead 6. There is NO standing "full-suite green"
claim attached to the move bead that a later step falsifies: Phase-1 green is pinned
to the Phase-1 tip (no move yet), and post-move green is pinned to the Bead-6 tip
(fixtures migrated). Fixtures are migrated ONLY where they must exercise the flat
path post-move — fixtures that are layout-agnostic are left untouched.

## Decomposition rationale (6 beads)

Six beads map onto the spec's three-phase spine while ISOLATING the one irreversible
filesystem move into its own reviewable bead. The count is at the heuristic ceiling
(≤6). The longest serial chain is **4** (1 → 2 → 5 → 6 and 1 → 4 → 5 → 6), one above
the depth-3 heuristic — a deliberate, justified override (below).

| Bead | Phase | Reqs | Domain(s) | Primary surface |
|:-----|:------|:-----|:----------|:----------------|
| 1 | 1 | 1, 2, 7, 15 | core (+ workflow: bootstrap) | `internal/workspace`, `internal/bootstrap` |
| 2 | 1 | 3, 6, 8 (classifier), 14 (classification) | workflow + context-system | `internal/validate`, `internal/contextpack`, `internal/spec`, `internal/domain`, `internal/doctor` (scans) |
| 3 | 2 | 4, 5, 11, 14 (ownership) | workflow + execution | net-new `internal/layout`, `internal/executor`, `internal/gitutil`, `internal/doctor` (link-check), `cmd/mindspec/migrate.go` |
| 4 | 3 | 8 (wiring), 9, 10 | workflow + execution | `internal/panel`, `internal/complete`, `internal/executor` (merge guard), `internal/doctor` (detector) |
| 5 | 3 | 13, 8 (migration), 14 (self-globs) | workflow (+ core/context-system drops) | the MOVES + root `review/**` migration + vestigial drops |
| 6 | 3 | 12, 16, 17, 15 (fixtures) | workflow (+ governance) | skills/snapshots, governance (DOCS-LAYOUT/ADR-0039/ADR-0037), migrate rubric, fixtures |

Heuristic justification:

- **The irreversible move is isolated into its own bead (Bead 5), overriding the
  depth-3 heuristic.** R6's reviewability hard_block — "the one-way filesystem cut
  must be independently reviewable" — outranks the depth-3 chain heuristic here. Bead
  5 does ONLY the irreversible work (the `specs/adr/domains/core` + `context-map`
  flatten, the dogfood eviction to `project-docs/`, the root `review/**` → spec-dir
  migration, and the vestigial drops) so a panel can scrutinize the one-way cut in
  isolation, without a reviewer also having to reason about static-text rewrites,
  governance prose, or fixture churn in the same diff.
- **The move → cleanup edge (5 → 6) is intrinsic, not artificial fragmentation.**
  You CANNOT rewrite skills/snapshots/setup-text to flat paths, document the flat
  layout in `DOCS-LAYOUT.md`/ADR-0039, or migrate the harness fixtures to the flat
  shape until the files have physically moved — doing it earlier (R6 blocker 6) would
  leave a canonical repo whose live skill instructions point at not-yet-existing
  `.mindspec/{domains,adr}` paths. So Bead 6 must depend on Bead 5 by construction;
  the depth-4 chain is the honest minimum, not padding.
- **Tool-heavy multi-file work grouped into ONE bead.** Bead 3 is the entire net-new
  mover: the transactional git-mv driver, the NET-NEW executor/gitutil git-mv /
  hard-reset / clean / branch+ref-discovery primitives it stands on, the
  checkpoint/resume run-state machine, rollback, the lineage writer, the
  finite-pattern link-rewriter, the net-new doctor 404 link-check lane, the
  `migrate layout` CLI, and the branch/PR discovery precondition. Splitting it would
  fracture the run-state machine from the rollback it serves and the link-check that
  gates it.
- **Phase-1 is split by DIFFERENT review lenses, not by dependency.** Bead 1 is
  filesystem-read resolvers (core / `internal/workspace`); Bead 2 is git-DIFF-STRING
  classifiers + enumerating consumers (workflow + context-system). The spec stresses
  these are categorically different ("the accessors cannot absorb the matchers — they
  match diff path strings, not filesystem reads"), so they get different reviewers.
- **Phase-3 wiring (Bead 4) is separated from the EXECUTION (Bead 5) and the CLEANUP
  (Bead 6).** Bead 4 lands ONLY the code that must recognize the flat tree (panel
  scan, merge fingerprint hard-fail, doctor detector) BEFORE any file moves and with
  NO static-text rewrites; Bead 5 then pulls the trigger through the Bead-3 mover and
  migrates the root reviews; Bead 6 re-points the live skills/snapshots/fixtures and
  lands governance. This keeps the irreversible moves in their own reviewable,
  mover-driven bead and keeps the not-yet-flat repo's live skill instructions
  correct throughout Beads 1–4.
- **The CRITICAL ref-anchored ownership matcher is pinned into Bead 2.** Per the
  spec's hard warning, the `complete` ADR-divergence gate loads ownership through
  `LoadOwnershipAtRef`/`domainManifestRelPath` (NOT `LoadOwnership` at :79); updating
  only :79 would leave every domain manifest reading "missing" on a flat tree and
  hard-block EVERY 106 bead on `adr-divergence-unowned`. Bead 2 lands the full
  multi-prefix posture, including this pair, in Phase 1 — before any move.
- **Born-flat bootstrap rides with Bead 1.** The greenfield write-default
  (`bootstrap.go:222-223` → flat dirs) is the write-side counterpart of
  `DetectLayout` and is tested together with it (AC4), so it lands in Bead 1 even
  though `internal/bootstrap` is workflow-owned (covered by the cited ADRs).

**Dependency edges (honest, produced-then-consumed only; shared source files are
NOT edges):**

```
Bead 1 (core resolvers + DetectLayout + signature helper + bootstrap)  depends_on: []
Bead 2 (classifier + matchers + consumers)                            depends_on: [1]      # budgeter + enumerating consumers CONSUME Bead 1's per-artifact accessors (Req 3)
Bead 3 (net-new mover + executor/gitutil primitives + link-check)     depends_on: []       # operates on physical paths in a captured fixture; carries its own thin ref-shape probe; no resolver dependency
Bead 4 (panel/merge/doctor wiring — code only)                        depends_on: [1]      # spec-scoped RecordingDir sibling + DetectLayout/LayoutSignature for the merge fingerprint + doctor detector
Bead 5 (execute the irreversible move + review migration)             depends_on: [2,3,4]  # runs the Bead-3 mover; needs Bead-2 matchers (so moves + migrated reviews don't hard-block) + Bead-4 wiring (so panel/doctor/merge-guard work on the post-move flat tree)
Bead 6 (post-move cleanup — skills/governance/rubric/fixtures)        depends_on: [5]      # cannot rewrite flat-path skills/snapshots/fixtures or document the flat layout until the files physically move
```

- **Roots:** Bead 1 (Phase-1 resolvers) and Bead 3 (net-new mover) are independent —
  the mover is golden-tested on a captured copy of the tree, carries its own thin
  pre/post-flatten ref-shape probe for the Req 11 precondition, and never reads the
  live resolvers, so it can be built fully in parallel with Phase 1.
- **Fan-out:** Beads 2 and 4 both fan out from Bead 1 and are independent of each
  other (Bead 2 = `internal/validate`/`internal/contextpack`/`internal/spec`/
  `internal/domain`; Bead 4 = `internal/panel`/`internal/complete`/`internal/executor`/
  `internal/doctor`-detector).
- **Convergence + tail:** Bead 5 consumes 2+3+4; Bead 6 consumes 5. **Longest chain =
  4** (1 → 2 → 5 → 6 and 1 → 4 → 5 → 6). Parallelism ratio = 2 roots / 6 = 0.33
  (> the 0.25 floor). No cyclic edges — a DAG.
- **Shared-file note (not a dependency):** Beads 3 and 4 both touch
  `internal/executor` (Bead 3 = the net-new mover primitives `GitMv`/`ResetHard`/
  `Clean`/ref-discovery; Bead 4 = the merge-fingerprint hard-fail in
  `CompleteBead`/`FinalizeEpic`) and `internal/doctor` (Bead 3 = the link-check lane
  `links.go`; Bead 4 = the layout-detector + dual-id lane in `migration.go`). Beads 3,
  5, and 6 all touch `cmd/mindspec/migrate.go` (Bead 3 adds the `migrate layout`
  subcommand; Bead 6 reconciles the prompt rubric; Bead 5 touches only the moved trees,
  not migrate.go). Because the consumers depend on the producers they rebase cleanly;
  the overlaps are different functions and cycle cleanly in either order. A
  non-blocking `decomposition-scope-redundancy` WARN is acceptable.

## Bead 1: Phase 1 — per-artifact three-tier resolvers, `DetectLayout`, shared layout-signature helper, flat worktree tier, born-flat bootstrap (Reqs 1, 2, 7, 15)

**Scope:** The HEART of the refactor, entirely in `internal/workspace` plus the
`internal/bootstrap` greenfield manifest. ZERO layout change on disk: the flat
directories do not exist yet, so a canonical/legacy tree resolves exactly as today.

**Changed files:** `internal/workspace/workspace.go`, `internal/workspace/worktree.go`,
`internal/bootstrap/bootstrap.go`, plus `*_test.go` for the resolver matrix.

**Steps**
1. Dissolve the single `DocsDir` join-point (workspace.go:116) into PER-ARTIFACT
   three-tier resolvers ordered flat → canonical → legacy, first-exists-wins:
   `SpecDir` (gain the flat `.mindspec/specs/<id>` tier alongside its existing
   worktree/canonical/legacy tiers), `ADRDir` (`.mindspec/adr/`), `DomainDir`
   (`.mindspec/domains/<d>`), `ContextMapPath` (`.mindspec/context-map.md`),
   `RecordingDir`, and a new `CoreDir` (`.mindspec/core/`). "Flat FIRST" is
   READ-PRECEDENCE, not delivery order — with no flat tree present the resolver
   returns the canonical/legacy path byte-for-byte as today.
2. Add `DetectLayout(root) → {flat | canonical | legacy | greenfield | mixed}` as a
   WHOLE-TREE probe. Distinguish `canonical` (a `.mindspec/docs/…` lifecycle tree)
   from `legacy` (a root `docs/…` tree) as DISTINCT write targets. `mixed` (ANY flat
   lifecycle tree coexisting with ANY canonical/legacy tree, across spec ids OR
   artifact classes) is a HARD ERROR, with the SOLE exception of a recorded
   in-progress `.mindspec/migrations/<run-id>/` recovery. The write-default keys off
   this whole-tree classification ("no canonical/legacy lifecycle tree exists
   ANYWHERE under root" → born flat), never a per-ID probe.
3. **Extract the shared layout-signature helper (minor 12).** Factor the pure
   tree-shape classification — given the set of present `.mindspec` lifecycle child
   names (or the presence of `.mindspec/docs/` vs the flat `.mindspec/{specs,adr,
   domains,core}` children), return the layout kind — into a small PURE function in
   `internal/workspace` (the sibling of `DetectLayout`, which consumes it for the
   filesystem probe). It takes pre-listed directory entries, so it does NO git or
   filesystem I/O itself and has no cycle risk. The Bead-4 merge guard (in
   `internal/executor`, which already imports `internal/workspace`) consumes THIS
   helper to fingerprint source/target trees from `executor.TreeDirsAtRef` output —
   one source of truth for the signature. (The Bead-3 mover keeps a thin local
   pre/post-flatten ref-shape probe of its own to preserve its captured-fixture
   golden-test independence — the small, intentional residual duplication is recorded
   here; a follow-up may converge the mover onto the same helper once the layout cut
   has landed and Bead 3's independence is no longer load-bearing.)
4. Make `SpecDir`'s worktree tier additionally probe the flat worktree shape
   (`.worktrees/worktree-spec-<id>/.mindspec/specs/<id>`), and make
   `TreeRootForSpecDir` (workspace.go:206-222) recognize the flat
   `.mindspec/specs/<id>` shape — its current `filepath.Base(docs) != "docs"`
   assertion (:214) returns `""` for a flat dir and would regress the mindspec-ew79
   cross-worktree ADR-visibility fix. Make the worktree-add recipes in
   `internal/workspace/worktree.go` layout-aware.
5. Replace the bootstrap greenfield manifest (`bootstrap.go:222-223`,
   `.mindspec/docs/{domains,specs}`) with the flat `.mindspec/{specs,domains}`
   (and any other born-flat dirs), so `mindspec init`/`bootstrap` creates a flat
   tree that `DetectLayout` classifies `flat`.
6. Add the resolver matrix test (canonical / legacy / flat fixtures), the
   `DetectLayout` five-state test (incl. `mixed`-hard-error and the
   new-id-in-legacy → `legacy` non-split case and the recorded-recovery exception),
   the signature-helper unit test, both worktree shapes, the `TreeRootForSpecDir`
   flat ew79 regression, and the bootstrap born-flat assertion. The fixtures carry no
   flat tree for the non-breaking cases (AC1) and a full flat tree for the flat cases
   (AC2).

**Verification**
- [ ] `go test ./internal/workspace/... ./internal/bootstrap/...` PASS (NEVER `./internal/harness/...`)
- [ ] Resolver matrix: canonical + legacy fixtures with NO flat tree resolve byte-identically to pre-spec behavior (AC1); the flat fixture resolves flat first-exists-wins (AC2)
- [ ] `DetectLayout` returns each of the five states; `mixed` is a hard error except under a recorded `.mindspec/migrations/<run-id>/` recovery; new-id-in-legacy classifies `legacy` (AC3)
- [ ] The pure layout-signature helper classifies each tree shape and is the single source of truth the Bead-4 merge guard reuses (minor 12)
- [ ] Both worktree shapes resolve and `TreeRootForSpecDir` returns the root for the flat shape (mindspec-ew79 regression green) (AC12)
- [ ] `go test ./internal/bootstrap/...` asserts the greenfield manifest writes flat dirs and `DetectLayout` classifies them `flat` (AC4)

**Acceptance Criteria**
- [ ] Every accessor resolves byte-identically on canonical/legacy with no flat tree (AC1) and flat-first on a flat tree (AC2); `DetectLayout` is whole-tree five-state with `mixed` as a hard error (AC3); greenfield bootstrap is born flat (AC4); both worktree shapes + the ew79 flat-shape `TreeRootForSpecDir` are pinned (AC12).

**Depends on**
None

## Bead 2: Phase 1 — diff-string layout classifier, permanently multi-prefix gate matchers (ADDITIVE reviews), tier-aware enumerating consumers (Reqs 3, 6, 14)

**Scope:** The git-DIFF-STRING classifiers and every root-enumerating consumer.
Lands fully green against the EXISTING two forms while ALREADY recognizing the flat
prefix (the panel R3 constraint). ZERO layout change on disk.

**Changed files:** `internal/validate/docsync.go`, `internal/validate/divergence.go`,
`internal/validate/ownership.go`, `internal/contextpack/budgeter.go`,
`internal/spec/list.go`, `internal/domain/list.go`, `internal/domain/show.go`,
`internal/doctor/docs.go`, with `*_test.go`.

**Steps**
1. Introduce a relative-path layout-classifier predicate and make EVERY git-ref
   matcher recognize all three prefixes (flat + canonical + legacy):
   `isDocFile`/`isSourceFile` (docsync.go:330/338), the
   `.mindspec/docs/specs/<id>/` literals in `validateSpecArtifactSync`/`specMDID`
   (docsync.go:561-622), the domain-doc prefixes in `checkInternalPackages`
   (docsync.go:485-488), the cmd-docs accept-set (docsync.go:534-538),
   `listDomainDirs`/`TreeDirsAtRef(".mindspec/docs/domains")` (docsync.go:352,
   ownership.go:198), `LoadOwnership`'s manifest path (ownership.go:79), and
   `isProcessArtifact` (divergence.go:277-281).
2. **CRITICAL:** make the ref-anchored ownership pair `LoadOwnershipAtRef`
   (ownership.go:150) + `domainManifestRelPath` (ownership.go:123-125) multi-prefix
   too. The `complete` ADR-divergence gate loads ownership through THIS pair, not
   `LoadOwnership` at :79; missing it leaves every flat-tree domain manifest reading
   "missing" and hard-blocks EVERY 106 bead on `adr-divergence-unowned`. Pin a
   flat-tree `LoadOwnershipAtRef` test that returns the domain's claims.
3. **Reviews exclusion is ADDITIVE, never a destructive REPLACE (R5 blocker 3).**
   `isProcessArtifact` (divergence.go:277-281) today excludes
   `strings.HasPrefix(path, "review/")`. KEEP that root `review/**` exclusion as a
   PERMANENT historical-ref compatibility matcher — analogous to the permanently
   multi-prefix doc/ownership matchers, and decoupled from the read-tier deprecation
   lifecycle (historical refs, old branches, and external forks never stop emitting
   the root `review/` path). ADD, alongside it, a NEW `/reviews/`-segment exclusion
   that classifies `<spec-dir>/reviews/<slug>/...` non-source (note: the literal
   `review/` does NOT substring-match `reviews/`, so the two are independent
   matchers). BOTH must classify non-source so that, during the transition (root
   `review/**` still live until Bead 5 migrates it, co-located `<spec-dir>/reviews/**`
   appearing after), neither reads as source/unowned and trips a gate. Mirror the
   same additive posture wherever doc-sync references `review/`.
4. Classify `project-docs/**` as NON-SOURCE docs in the predicate (Req 14) so
   dogfood eviction trips neither `adr-divergence-unowned` nor the doc-sync source
   lane; make the cmd-docs accept-set accept the post-flatten `core/USAGE.md` and the
   evicted `project-docs/user/**`.
5. Make `internal/contextpack/budgeter.go` consume the Bead-1 per-artifact accessors
   instead of the hardcoded `.mindspec/docs/specs/<id>` (budgeter.go:170) and
   `.mindspec/docs/domains/<d>/<kind>` (budgeter.go:218) joins, so a context pack
   assembles byte-identically — same sections, same bytes — on a flat vs canonical
   vs legacy project (no silent spec/domain drop).
6. Make EVERY other root-enumerating `DocsDir` consumer tier-aware via the Bead-1
   accessors: `mindspec spec list` (`internal/spec/list.go:22`, joins
   `workspace.DocsDir`+"specs"), `mindspec domain list`/`show`
   (`internal/domain/list.go:102`, `internal/domain/show.go:32,74`), context-map
   enrichment, the `core/` doc paths, and the doctor orphan/docs scans
   (`internal/doctor/docs.go` — `checkDocs`/`checkDomains`/`checkStaleFocusLifecycle`/
   `docsRootRel`, all via `workspace.DocsDir`).
7. Add the three-prefix equivalence tests, the flat-tree ref-anchored-ownership
   test, the ADDITIVE reviews test (root `review/<slug>/...` AND
   `<spec-dir>/reviews/<slug>/...` both non-source), the `project-docs/**`
   classification test, the context-pack flat-vs-canonical byte-identity test, and
   command-level flat-fixture tests for the enumerating consumers (spec list, domain
   list/show, doctor scans).

**Verification**
- [ ] `go test ./internal/validate/... ./internal/contextpack/... ./internal/spec/... ./internal/domain/... ./internal/doctor/...` PASS (NEVER `./internal/harness/...`)
- [ ] A legacy-, canonical-, and flat-prefix diff classify identically across doc-sync / ownership / divergence; `project-docs/foo.md` is neither source nor `adr-divergence-unowned` (AC11)
- [ ] `isProcessArtifact` classifies BOTH a root `review/<slug>/...` path (permanent historical matcher) AND a `<spec-dir>/reviews/<slug>/...` path (new segment matcher) as non-source (AC11)
- [ ] A flat-tree `LoadOwnershipAtRef` returns the domain's claims (not "missing") so beads do not hard-block `adr-divergence-unowned` (AC11)
- [ ] The context-pack builder yields byte-identical sections on flat vs an otherwise-identical canonical fixture (AC6); spec-list/domain-list/doctor scans return the same inventory on a flat fixture (AC5)
- [ ] `project-docs/**` is classified non-source docs by the predicate (AC21 classification half)

**Acceptance Criteria**
- [ ] Doc-sync, ownership (incl. the ref-anchored `LoadOwnershipAtRef`/`domainManifestRelPath` pair), and divergence classify flat/canonical/legacy diff paths identically, keep the permanent root `review/**` exclusion AND add the `<spec-dir>/reviews/` exclusion, and treat `project-docs/**` as non-source docs (AC11); enumerating consumers resolve on flat fixtures (AC5); context packs assemble byte-identically flat vs canonical (AC6).

**Depends on**
Bead 1 (the budgeter and the enumerating consumers CONSUME Bead 1's per-artifact
accessors; the diff-string matcher portion is independent but ships in the same
bead).

## Bead 3: Phase 2 — net-new `internal/layout` mover + executor/gitutil mover primitives + link-rewriter + run-state machine + lineage + doctor link-check + `migrate layout` CLI (Reqs 4, 5, 11, 14)

**Scope:** A net-new, golden-tested, transactional mover AND the net-new git
primitives it stands on. Tool-heavy → ONE bead. It NEVER mutates the live tree until
invoked; all tests run on a CAPTURED COPY of the real `.mindspec/docs` tree. Adds
`internal/layout/**` to the workflow `OWNERSHIP.yaml` IN THIS BEAD.

**Changed files:** new `internal/layout/**` (mover, run-state, link-rewriter,
lineage); `internal/gitutil/gitops.go` (the net-new git primitives — see step 1);
`internal/executor/executor.go` + `internal/executor/mindspec_executor.go` +
`internal/executor/mock.go` (surface those primitives on the `Executor` interface);
new `internal/doctor/links.go` (the net-new link-existence lane);
`cmd/mindspec/migrate.go` (the `migrate layout` subcommand + precondition);
`.mindspec/docs/domains/workflow/OWNERSHIP.yaml` (claim the package); with golden
fixtures + `*_test.go`.

**Steps**
1. **Add the NET-NEW git primitives the mover requires (R4 blocker 2).** `internal/
   gitutil` today has merge/diff/worktree/commit helpers but NO git-mv, hard-reset,
   clean, or remote-tracking/PR ref-discovery, and the `Executor` interface has none
   either — confirmed in code. Add to `internal/gitutil/gitops.go` (the ADR-0030 git
   I/O boundary, applying the existing `RejectOptionLike` SEC-5 guard per operand):
   a history-preserving `GitMv(workdir, src, dst)` (`git mv`), `ResetHard(workdir,
   ref)` (`git reset --hard <ref>` for rollback), `CleanForce(workdir)` (`git clean
   -fd`), and the discovery helpers `LocalBranchRefs(workdir)` /
   `RemoteTrackingRefs(workdir)` (`git for-each-ref refs/heads`/`refs/remotes`) for
   the Req 11 precondition. Surface these on the `Executor` interface
   (`internal/executor/executor.go`) + `MindspecExecutor` impl
   (`internal/executor/mindspec_executor.go`) + `MockExecutor`
   (`internal/executor/mock.go`) so `internal/layout` calls them THROUGH the executor
   boundary, never by shelling out. Hosted-PR metadata (when a remote + token exist)
   is read via the existing remote helpers; offline it degrades (step 4).
2. Build the history-preserving git-mv DRIVER in `internal/layout`, routed through
   the executor primitives from step 1. It is idempotent (re-run on an already-flat
   tree is a no-op) and lands each artifact-group move as TWO commits: a pure
   100%-similarity `git mv` first, then the link-rewrite second — so
   `git log --follow` and 3-way-merge rename detection stay reliable.
3. Define the RUN-STATE MACHINE with a durable checkpoint at every mutation boundary
   (pre-run base ref; then per group: before `git mv`, after `git mv`/before
   pure-move commit, after pure-move commit, after link-rewrite, after rewrite
   commit; then lineage/state finalize). Implement RESUME: discard/re-stage
   working-tree-only partials, SKIP already-landed commits idempotently, re-run the
   deterministic link-rewrite. Implement ROLLBACK: `ResetHard` to the pre-run ref +
   `CleanForce` while nothing is published (no compensating reverts); AFTER publish,
   REFUSE auto-rollback and emit the forward-only recovery line (ADR-0023). Write a
   lineage manifest under `.mindspec/lineage/` and a `.mindspec/migrations/<run-id>/`
   state record consistent with the doctor migration-metadata schema
   (`internal/doctor/migration.go`).
4. Build the FINITE-PATTERN markdown link-rewriter — NOT a general parser. Rewrite
   only the closed breaking subset (one-level `../adr/`, absolute `docs/core/` and
   `.mindspec/docs/core/`/context-map refs, review-co-location depth changes, dogfood
   eviction depth changes, repo-root README/AGENTS refs INTO the moved trees);
   PRESERVE symmetric `../../adr/ADR-NNNN.md` spec→ADR links unchanged. Add the
   GATING net-new doctor link-existence lane (`internal/doctor/links.go`) that scans
   EVERY link in the post-migration moved tree AND affected root docs and FAILS the
   migration (and the run-state machine) on ANY 404.
5. Add the `migrate layout` CLI with the precondition: a deterministic branch/PR
   discovery scan over (1) local refs (`LocalBranchRefs`), (2) remote-tracking refs
   (`RemoteTrackingRefs`), (3) hosted PR metadata when a remote+token exist; compute
   a pre/post-flatten fingerprint from the ref's merge-base tree layout SIGNATURE
   (its own thin probe — see the minor-12 note in Bead 1); a ref BLOCKS iff unmerged
   AND pre-flatten. Offline → degrade to local + remote-tracking refs and WARN (do
   not silently pass). Require a clean idle working tree before mutating. TOLERATE
   locked agent worktrees and external forks (fingerprint-guarded at merge, Req 10 /
   Bead 4 — not counted as blockers).
6. Add `- internal/layout/**` to the workflow `OWNERSHIP.yaml` so the package is
   owned and does not trip `adr-divergence-unowned` at complete.
7. Add the golden-file test (deterministic post-tree, 100%-similarity first commit,
   idempotent re-run, hard-reset rollback, crash-resume at every boundary, lineage
   parses under the doctor schema), the new git-primitive tests against a real temp
   git repo, the link-check tests (zero 404s on the migrated tree; FAIL on an
   injected dangling rewritten link AND an un-rewritten breaking link), and the
   precondition tests (pre-flatten branch BLOCKS; locked worktree + external-fork ref
   do NOT).

**Verification**
- [ ] `go test ./internal/layout/... ./internal/gitutil/... ./internal/executor/... ./internal/doctor/... ./cmd/mindspec/...` PASS (NEVER `./internal/harness/...`)
- [ ] The net-new `GitMv`/`ResetHard`/`CleanForce`/ref-discovery primitives exist on `internal/gitutil` + the `Executor` interface and pass against a real temp git repo (R4 blocker 2)
- [ ] Golden-file: deterministic post-tree, two-commit per move (100%-similarity first), idempotent re-run, hard-reset-to-pre-run-ref rollback (AC7); crash-resume at every run-state boundary + pre-publish `--abort` (AC8); lineage manifest under `.mindspec/lineage/` parses under the doctor schema (AC9)
- [ ] The net-new doctor link-check lane (`internal/doctor/links.go`) scans EVERY link in the moved tree + affected root docs; zero 404s; FAILS on an injected dangling rewritten link AND an un-rewritten breaking link (AC10 build half)
- [ ] Precondition BLOCKS on an unmerged pre-flatten branch and on a dirty tree; offline degrades + WARNs; does NOT block on a locked worktree or external-fork ref (AC16)
- [ ] `internal/layout/**` is claimed in the workflow `OWNERSHIP.yaml` (no `adr-divergence-unowned`) (AC21 ownership half)

**Acceptance Criteria**
- [ ] A deterministic, idempotent, two-commit `migrate layout` with defined run-state checkpoints, crash-resume, and hard-reset rollback (AC7/AC8); a lineage manifest under the doctor schema (AC9); a 404-gating link-check over every link (AC10 build); a discovery precondition that blocks pre-flatten branches/PRs and a dirty tree but tolerates locked worktrees/forks (AC16); `internal/layout/**` owned (AC21).

**Depends on**
None (net-new package, golden-tested on a captured copy of the tree; it operates on
physical paths and carries its own thin ref-shape probe, so it does not consume the
live resolvers or the Bead-1 signature helper).

## Bead 4: Phase 3 — layout-aware (transitional) reviews panel scan + complete gate, DIRECTIONAL merge-time layout-fingerprint hard-fail at the REAL executor seams, doctor layout detection (Reqs 8, 9, 10)

**Scope:** ONLY the code that must RECOGNIZE the flat tree — landed BEFORE any file
moves and with NO static-text rewrites (the skills/snapshots moved to Bead 6 per R6
blocker 6). No `.mindspec/docs/` file moves in this bead.

**Changed files:** `internal/panel/panel.go`, `internal/panel/gate.go`,
`internal/complete/complete.go`, `internal/executor/mindspec_executor.go`,
`internal/doctor/migration.go`, plus `*_test.go`.

**Steps**
1. **Make `internal/panel.Scan`/`PanelDirScanRoot` and the `mindspec complete`
   panel-gate scan LAYOUT-AWARE / TRANSITIONAL, not unconditionally spec-scoped (R5
   blocker).** `panel.Scan` (panel.go:118-142) globs `review/*/panel.json` under each
   root today; the spec-scoped target is the `<spec-dir>/reviews/*/panel.json` glob (a
   sibling of `RecordingDir`, workspace.go:244-250). A HARD switch to spec-scoped-only
   would break THIS spec's own remaining bead cycle: root `review/**` is not migrated
   until Bead 5 and the panel skills still write to repo-root `review/` until Bead 6,
   so the canonical intermediate after Beads 1–4 must keep honoring the still-live root
   `review/` convention. Therefore the scan mode is chosen from the tree shape via the
   Bead-1 `DetectLayout`/layout-signature helper:
   - **On a canonical/legacy (pre-move) tree:** scan BOTH the repo-root
     `review/<slug>/panel.json` glob AND the spec-scoped
     `<spec-dir>/reviews/<slug>/panel.json` glob (the union — `Scan` already dedupes by
     resolved path, so the spec-scoped roots are simply added alongside the existing
     repo-root/worktree roots; the gate sees panels from EITHER location). A
     sub-threshold panel under EITHER location blocks `complete`.
   - **On a flat (post-move) tree:** enforce co-located reviews ONLY — scan the
     spec-scoped `<spec-dir>/reviews/*/panel.json` glob and IGNORE repo-root `review/`
     (which Bead 5 has migrated away and removed). A panel.json under the OLD repo-root
     `review/` no longer drives the gate once the tree is flat.
   This keeps the gate live throughout the transition: the root `review/` path keeps
   driving the gate while the tree is still canonical (Beads 1–4 + the run up to Bead
   5), and only stops once the tree is flat and root `review/` has been migrated/removed.
   `PanelDirScanRoot` (gate.go:418-420) recognizes both a repo-root `review/<slug>`
   and a `<spec-dir>/reviews/<slug>` panel dir as its scan-root owner. (The divergence
   `/reviews/` ADDITIVE exclusion itself landed in Bead 2, and the permanent root
   `review/**` exclusion is KEPT there.)
2. **Add the merge-time layout-fingerprint HARD-FAIL at the REAL merge seams, and
   make it DIRECTIONAL (R4 blocker 1 + R1).** There is NO `internal/complete
   MergeBranch`. The real bead→spec merge is `gitutil.MergeInto` called from
   `MindspecExecutor.CompleteBead` (mindspec_executor.go:291) and the auto-merge loop
   in `FinalizeEpic` (:370); the real spec→main merge is `gitutil.MergeBranch` from
   `FinalizeEpic` (:432). Install the guard in `internal/executor/mindspec_executor.go`
   directly in front of EACH of those three call sites: compute the layout signature of
   source and target (via the shared Bead-1 `internal/workspace` helper, fed by
   `executor.TreeDirsAtRef` reads of the `.mindspec` tree at each ref —
   `internal/executor` already imports `internal/workspace`, so no import cycle).
   **The guard is DIRECTIONAL — it blocks ONLY the REGRESSION direction.** It HARD-FAILS
   a **canonical/legacy-layout source merging onto a flat target** (the merge that would
   resurrect pre-flatten `.mindspec/docs/…` paths on top of an already-flattened tree)
   with a "rebase onto post-flatten main" recovery line, mutating nothing (ADR-0023
   forward-only). It EXPLICITLY does NOT block the MIGRATION direction — a
   **flat source merging onto a canonical/legacy target** — because that direction IS
   the flatten landing: Bead 5's own move-merge has a flat source (`bead/…5` is flat
   after running the mover) merging into the still-canonical spec branch, and the
   eventual flat-spec→canonical-main merge is likewise flat-onto-canonical; hard-failing
   either would deadlock this spec. A same-layout merge (canonical→canonical,
   flat→flat) is always allowed. The directional rule is precise: `block ⟺ source is
   canonical/legacy AND target is flat`. The regression block is EXEMPT inside a
   recorded in-progress migration run-state (a live `.mindspec/migrations/<run-id>/`,
   the same exception `DetectLayout` honors in Bead 1), where a transient cross-layout
   merge is part of the recovery path. Both surfaces — bead→spec AND spec→main — are
   guarded at their real seams.
   **Honest coverage scope (R4).** `FinalizeEpic`'s `gitutil.MergeBranch` site is the
   DIRECT / no-remote spec→main path ONLY: when a remote exists, `FinalizeEpic` sets
   `MergeStrategy = "pr"` and PUSHES the branch for a PR (mindspec_executor.go:401-405)
   instead of doing a local merge, so the in-binary guard never runs on the remote path.
   The in-binary guard therefore covers the LOCAL-merge seams — `CompleteBead`'s and
   `FinalizeEpic`'s `gitutil.MergeInto` (bead→spec) and `FinalizeEpic`'s direct
   `gitutil.MergeBranch` (spec→main, no-remote) — and cross-layout protection on the
   remote-PR path relies on the branches-not-worktrees pre-flatten precondition (Bead 3)
   plus PR review, NOT on this in-binary guard. This is stated so the guard's coverage
   is honest, not overclaimed.
3. Give `mindspec doctor` a real layout detector (reusing `DetectLayout`), a
   `would-migrate-layout` Warn (analogous to `would-migrate: spec=…`,
   migration.go:101-106), and an ERROR when the SAME spec id exists under two
   layouts. Make `checkDryRunMigration` (migration.go:39) tier-aware. (The orphan/docs
   scans in `docs.go` were made tier-aware in Bead 2.)
4. Add tests: on a canonical fixture the layout-aware scan finds a co-located
   `<spec-dir>/reviews/<slug>/panel.json` AND a repo-root `review/<slug>/panel.json`,
   and a sub-threshold panel in EITHER location blocks `complete` (the transition
   union); on a flat fixture only the co-located panel drives the gate and a root
   `review/` panel is ignored. Cross-layout merge tests cover BOTH directions: a
   canonical/legacy source → flat target HARD-FAILS with the recovery line and mutates
   nothing (regression blocked), while a flat source → canonical target is ALLOWED
   (migration permitted), and the regression block is suppressed under a recorded
   `.mindspec/migrations/<run-id>/` run-state. Doctor reports the layout, emits
   `would-migrate-layout`, and ERRORs on a dual-layout duplicate id.

**Verification**
- [ ] `go test ./internal/panel/... ./internal/complete/... ./internal/executor/... ./internal/doctor/...` PASS (NEVER `./internal/harness/...`)
- [ ] Layout-aware scan: on a canonical/legacy tree the gate scans BOTH repo-root `review/<slug>/panel.json` AND `<spec-dir>/reviews/<slug>/panel.json`, and a sub-threshold panel in EITHER blocks `complete`; on a flat tree only `<spec-dir>/reviews/<slug>/panel.json` drives the gate and a root `review/` panel is ignored (AC13)
- [ ] Doctor reports the detected layout, emits `would-migrate-layout`, and ERRORs on a dual-layout duplicate id (AC14)
- [ ] DIRECTIONAL merge guard: a canonical/legacy source → flat target hard-fails with a rebase recovery line and mutates nothing (regression blocked), while a flat source → canonical target is allowed (migration permitted) and the block is exempt under a recorded run-state — exercised at `internal/executor` in front of `gitutil.MergeInto` (`CompleteBead`/`FinalizeEpic`, bead→spec) and the DIRECT (no-remote) `gitutil.MergeBranch` (`FinalizeEpic`, spec→main); the remote path pushes for a PR (mindspec_executor.go:401-405) and is covered by the Bead-3 precondition + PR review, not this guard (AC15)

**Acceptance Criteria**
- [ ] Layout-aware/transitional panel gate — both root `review/` and co-located `<spec-dir>/reviews/` drive the gate on a canonical tree, co-located only on a flat tree (AC13); doctor layout detection + dual-layout-duplicate-ID ERROR (AC14); DIRECTIONAL cross-layout merge hard-fail (regression blocked, migration allowed, run-state exempt) with recovery line at the real local bead→spec and spec→main executor merge seams — the remote-PR path is covered by the Bead-3 precondition + PR review, stated honestly (AC15).

**Depends on**
Bead 1 (the spec-scoped reviews glob is a sibling of the `RecordingDir` accessor; the
merge fingerprint reuses the Bead-1 `internal/workspace` layout-signature helper; the
doctor detector reuses `DetectLayout`). It does NOT depend on Bead 3: the merge guard
reads the signature through the Bead-1 workspace helper, not the mover, so there is no
executor→`internal/layout` import (which would cycle, since the mover imports the
executor).

## Bead 5: Phase 3 — execute the IRREVERSIBLE flatten via the mover + root `review/**` migration + vestigial drops (Reqs 13, 8 migration, 14 self-globs)

**Scope:** Pull the trigger. This bead does ONLY the one-way, irreversible filesystem
work so a panel can review the cut in isolation (R6 blocker 4): EXECUTE the flatten
through the Bead-3 mover, evict dogfood, MIGRATE the existing root `review/**` tree,
and drop the vestigial files. NO static-text/skills/governance/fixture work here
(that is Bead 6).

**Changed files:** the `.mindspec/docs/{specs,adr,domains,core}` trees +
`context-map.md` (MOVED to flat `.mindspec/…`), the dogfood
`user/installation/research` (EVICTED to top-level `project-docs/`), the root
`review/**` tree (MIGRATED to `<spec-dir>/reviews/**` and the root tree REMOVED),
`.mindspec/docs/glossary.md` + `.mindspec/policies.yml` (DROPPED),
`internal/instruct/templates/spec.md:35` (glossary line dropped), and each domain's
`OWNERSHIP.yaml` self-glob.

**Steps**
1. Run the Bead-3 mover to flatten `specs/`, `adr/`, `domains/`, `core/` to top-level
   `.mindspec/` children and move `context-map.md` to `.mindspec/context-map.md` —
   KEEPING the names `adr` and `core`. Do the dogfood eviction + vestigial drop FIRST
   as the low-stakes exercise of the mover: evict `user/`, `installation/`,
   `research/` to top-level `project-docs/` (explicitly NOT `docs/`, which would alias
   `LegacyDocsDir`), drop `glossary.md`, `.mindspec/policies.yml`, and the
   `internal/instruct/templates/spec.md:35` glossary line.
2. **MIGRATE the existing root `review/**` tree (R6 blocker 5 — currently missing).**
   The repo carries 42 tracked artifacts under repo-root `review/` (e.g.
   `review/099-final-panel/`, `…/prep`, loose `*.md`). Run the mover's
   review-co-location step to move each `review/<slug>/...` to its owning spec's
   `<spec-dir>/reviews/<slug>/...` (keyed by the leading spec id in the slug), let the
   finite-pattern link-rewriter fix the depth-changed links, and REMOVE the root
   `review/` tree entirely so the homeless-review friction (adwu) is resolved.
   Artifacts that cannot be attributed to a spec id are co-located under the nearest
   owning spec (or recorded explicitly) so none are orphaned. The doctor link-check
   (Bead 3 lane) gates the result on zero 404s.
3. Update each domain's `OWNERSHIP.yaml` that self-claims
   `.mindspec/docs/domains/<d>/OWNERSHIP.yaml` to the flat
   `.mindspec/domains/<d>/OWNERSHIP.yaml`. Do NOT add any `bench/**`, `viz/**`, or
   `agentmind/**` glob (excluded first-segments; a hard `LoadOwnership` schema error).
4. Verify the post-move tree: the doctor link-check is green (zero 404s); the resolver
   assertions point at flat `.mindspec/…` and NOT into `project-docs/` or a root
   `docs/` alias; the glossary/policies greps are empty; the domain self-globs are
   flat; the root `review/` tree is gone and its artifacts now live under
   `<spec-dir>/reviews/**`.

**Verification**
- [ ] `test -d project-docs` AND a resolver test asserts `ADRDir`/`DomainDir`/`ContextMapPath` resolve to flat `.mindspec/…` and NOT into `project-docs/` or a root `docs/` alias (AC18)
- [ ] `grep -rn 'GLOSSARY.md\|policies.yml' internal cmd | grep -v _test` is empty (NO `*.go` filter, so it reaches the template `.md` line) (AC19)
- [ ] No domain `OWNERSHIP.yaml` self-claims the pre-flatten `.mindspec/docs/domains/` path (AC20)
- [ ] The root `review/` tree no longer exists (`! test -d review`) and its 42 tracked artifacts now resolve under `<spec-dir>/reviews/**` (AC13 realized on the real tree)
- [ ] **Transition asserted post-move:** the tree is now flat, so the Bead-4 layout-aware scan enforces co-located reviews ONLY — `<spec-dir>/reviews/<slug>/panel.json` is found and a sub-threshold co-located panel blocks `complete`, while a (leftover) repo-root `review/<slug>/panel.json` no longer drives the gate now that the tree is flat and root `review/` has been removed (the mirror of the Bead-4 pre-move case, where a sub-threshold root `review/` panel still blocked `complete`)
- [ ] `mindspec doctor` link-check is green on the migrated tree (AC10 realized post-move)
- [ ] (No full `go test ./...` claim here — harness fixtures still encode the canonical shape and are migrated in Bead 6; see AC22 reconciliation in Testing Strategy)

**Acceptance Criteria**
- [ ] The flatten + dogfood eviction to `project-docs/` (no root `docs/` alias) + vestigial drops land with no dangling refs (AC18/AC19); the root `review/**` tree is migrated to `<spec-dir>/reviews/**` and removed (AC13 realized); domain self-globs are flat (AC20); the doctor link-check is green on the migrated tree (AC10 post-move).

**Depends on**
Beads 2, 3, 4 (runs the Bead-3 mover; needs the Bead-2 multi-prefix matchers — incl.
the ADDITIVE `<spec-dir>/reviews/` classifier and the ref-anchored ownership pair —
so the move + the migrated reviews do not hard-block `adr-divergence-unowned`, and the
Bead-4 spec-scoped panel scan + doctor wiring + merge guard so the gate, doctor, and
merges work on the post-move flat tree and find the migrated reviews).

## Bead 6: Phase 3 — post-move cleanup: flat-path skills/snapshots/setup-text + governance + migrate rubric + harness/testdata fixture migration (Reqs 12, 16, 17, 15 fixtures)

**Scope:** The mechanical, NON-irreversible cleanup that can only run once the files
have physically moved (R6 blocker 6): re-point every path-bearing skill/snapshot to
the flat tree, land governance, reconcile the migrate prompt rubric, and migrate the
harness/testdata fixtures that the moved tree breaks.

**Changed files:** `internal/setup/skills.go`, `internal/setup/claude.go`
(incl. the `lifecycleSkillFiles` literals), `internal/setup/codex.go`,
`internal/setup/historical_skills/*.md` (NEW pre-106 snapshots ADDED so existing
installs refresh; the EXISTING snapshots are FROZEN byte-exact — see step 1), the live
`.agents/skills/**` + `.claude/skills/**` + `plugins/mindspec/skills/**` (ms-spec-grill,
ms-spec-create, ms-panel-run/tally, ms-bead-impl/cycle/fix, ms-spec-final-review),
`.mindspec/core/DOCS-LAYOUT.md` (amended; itself already flattened by Bead 5), a new
`ADR-0039` (via `mindspec adr create`), the ADR-0037 amendment note,
`cmd/mindspec/migrate.go` (prompt rubric), and the harness scenario files + testdata
fixtures the moves break, with `*_test.go`.

**Steps**
1. Update EVERY path-bearing LIVE skill/setup-text surface to the post-flatten paths
   NOW that the tree is flat: `ms-spec-grill`'s SKILL.md domain-set read
   (`.mindspec/docs/domains/`) AND next-ADR-number scan (`.mindspec/docs/adr/`) — its
   central correctness claim; the `lifecycleSkillFiles` literals (in
   `internal/setup/claude.go`); `setup/claude.go` + `setup/codex.go` generated text;
   the panel skills (ms-panel-run/tally) repo-root `review/` panel path; `ms-spec-create`
   (named explicitly per minor 11); ms-bead-impl/cycle/fix; ms-spec-final-review; plugin
   README/FINDINGS + bootstrap text.
   **The embedded provenance snapshots live in `internal/setup/historical_skills/*.md`
   (embedded via `//go:embed historical_skills/*.md` in `internal/setup/skills.go:18`),
   NOT as literals in `skills.go` (R5 minor — corrected).** These snapshots are
   byte-EXACT captures of PREVIOUSLY-shipped SKILL.md content; their sole purpose is the
   HC-6 provenance byte-match (`previouslyShippedSkills`/`matchesShipped`) that lets
   `installSkills` tell a MindSpec-shipped file from a user-authored one so it can
   refresh/remove it. They are therefore **INTENTIONALLY FROZEN: they MUST retain their
   pre-flatten `.mindspec/docs/{specs,adr,domains,core}` and repo-root `review/<slug>/`
   paths verbatim** — rewriting them to flat paths would break the byte-match against
   every existing install that carries the pre-106 content (it would no longer be
   recognized as shipped, so it would never be refreshed). Recorded reason, one line:
   *frozen — byte-exact prior-shipped captures for the HC-6 refresh/cleanup byte-match;
   flat paths would defeat detection of pre-106 installs.* To let existing pre-106
   installs refresh to the flat skills, Bead 6 ADDS a NEW `historical_skills/<skill>.md`
   snapshot of each rewritten skill's pre-106 canonical bytes (the spec-105
   convention) — those new snapshots ALSO legitimately carry pre-flatten paths, which
   reinforces the exclusion. Consequently the frozen `historical_skills/*.md` surface is
   EXPLICITLY EXCLUDED from the AC17 clean-grep (its pre-flatten paths are load-bearing,
   not stale); it is the exact "`historical_skills` snapshot intentionally frozen at a
   pre-flatten ref" surface AC17 names as requiring a recorded exclusion reason.
2. Governance: amend `DOCS-LAYOUT.md` to describe the flat layout, the per-artifact
   three-tier read order, and reviews co-location; create the layout-v2 ADR
   **ADR-0039** via `mindspec adr create` (formalizing the flat layout, the
   per-artifact resolver, the permanent multi-prefix matcher posture, and the
   ADR-0037 reviews-location amendment); add the ADR-0037 amendment note for the new
   `panel.json` location.
3. Reconcile the `cmd/mindspec/migrate.go` prompt rubric (migrate.go:264-298,
   286-287): route user/dogfood docs to top-level `project-docs/` and lifecycle/
   authored artifacts to the flat `.mindspec/{specs,adr,domains,core}` children,
   never to `.mindspec/docs/user/`/`…/agent/`.
4. Migrate the broken fixtures (R6 blocker 7 — only NOW, once the live tree is flat):
   the harness scenario files (`scenario_spec_lifecycle.go`, `scenario_bead_lifecycle.go`,
   `scenario_worktree.go`, `scenario_contract_hardening.go`, `scenario_safety.go`) and
   any testdata fixtures that encode the physical `.mindspec/docs/…` shape, so the
   orchestrator can re-run `go test ./...` green AFTER the move. Fixtures that are
   layout-agnostic are left untouched.
5. Verify: `go test ./internal/setup/...` is green; the AC17 breadth grep over the FULL
   named surface returns empty of pre-flatten `.mindspec/docs/{specs,adr,domains,core}`
   / repo-root `review/<slug>/` paths; `DOCS-LAYOUT.md` documents the flat layout; and
   `go test ./cmd/mindspec/...` asserts the migrate rubric targets `project-docs/` +
   the flat children, never `.mindspec/docs/user/`.

**Verification**
- [ ] `go test ./internal/setup/... ./cmd/mindspec/...` PASS (NEVER `./internal/harness/...`; the harness scenarios are re-run by the orchestrator after fixture migration)
- [ ] `go test ./internal/setup/...` green AND `grep -rn '.mindspec/docs/\(specs\|adr\|domains\|core\)\|review/' .agents/skills .claude/skills plugins/mindspec/skills internal/setup/skills.go internal/setup/claude.go internal/setup/codex.go internal/bootstrap` returns empty (the co-located `reviews/` path is permitted; `review/` does not substring-match `reviews/`). `internal/setup/historical_skills/*.md` is DELIBERATELY OUT of this grep scope — recorded reason: frozen byte-exact prior-shipped captures for the HC-6 refresh/cleanup byte-match, whose pre-flatten paths are load-bearing (AC17)
- [ ] `DOCS-LAYOUT.md` documents the flat `.mindspec/{specs,adr,domains,core}` + `project-docs/` layout and no longer presents `.mindspec/docs/{specs,adr,domains}` as live (AC23)
- [ ] `go test ./cmd/mindspec/...` asserts the migrate rubric targets `project-docs/` + the flat children, never `.mindspec/docs/user/` (AC24)
- [ ] The harness/testdata fixtures that encoded the canonical shape are migrated to the flat shape so the orchestrator's post-move `go test ./...` is green (AC22 fixture half)

**Acceptance Criteria**
- [ ] ALL path-bearing skills/snapshots read the layout-correct path or are explicitly classified out (AC17); `DOCS-LAYOUT.md` is amended and ADR-0039 + the ADR-0037 amendment are authored (AC23); the migrate prompt rubric is reconciled to the flat target (AC24); the harness/testdata fixtures are migrated so the full suite is green post-move (AC22 fixture half).

**Depends on**
Bead 5 (the flat-path skill/snapshot/setup-text rewrites, the `DOCS-LAYOUT.md`/ADR-0039
governance describing the SHIPPED flat tree, and the harness/testdata fixture
migration all require the files to have PHYSICALLY MOVED first; doing them earlier
would leave a canonical repo whose live skill instructions point at not-yet-existing
flat paths — R6 blocker 6).

## Provenance

Every spec acceptance criterion (AC1–AC25) maps to ≥1 bead; every bead satisfies ≥1
criterion.

| Acceptance Criterion (spec) | Bead(s) | Verified By |
|:----------------------------|:--------|:------------|
| **AC1** — non-breaking canonical+legacy reads | Bead 1 | resolver matrix on canonical/legacy fixtures with no flat tree |
| **AC2** — flat resolution first-exists-wins | Bead 1 | resolver matrix on a flat fixture |
| **AC3** — `DetectLayout` whole-tree five-state, `mixed` hard error | Bead 1 | detector table test (incl. new-id-in-legacy, recorded-recovery exception) |
| **AC4** — greenfield bootstrap born flat | Bead 1 | `go test ./internal/bootstrap/...` + `DetectLayout` classifies `flat` |
| **AC5** — root-enumerating consumers on flat fixtures | Bead 2 | command-level flat-fixture tests in `internal/spec` (spec list), `internal/domain` (list/show), `internal/doctor` (scans) |
| **AC6** — context packs byte-identical flat vs canonical | Bead 2 | `go test ./internal/contextpack/...` byte-identity test |
| **AC7** — deterministic two-commit mover + hard-reset rollback | Bead 3 | golden-file test (100%-similarity first commit, idempotent, `ResetHard` rollback) |
| **AC8** — crash recovery at every run-state boundary | Bead 3 | table test injecting a crash at each checkpoint + pre-publish `--abort` |
| **AC9** — lineage manifest under the doctor schema | Bead 3 | golden-file test parses `.mindspec/lineage/` manifest under the schema |
| **AC10** — link gate scans every link, zero 404s | Bead 3 (build) + Bead 5 (post-move) | doctor link-check (`internal/doctor/links.go`); FAILS on dangling-rewritten AND un-rewritten breaking link; green on the migrated tree |
| **AC11** — gate matchers three-prefix incl. ref-anchored ownership pair + ADDITIVE reviews | Bead 2 | `go test ./internal/validate/...` three-prefix equivalence + flat `LoadOwnershipAtRef` + root `review/**` AND `<spec-dir>/reviews/**` both non-source + `project-docs/**` classification |
| **AC12** — worktree shapes resolve; mindspec-ew79 preserved | Bead 1 | `go test ./internal/workspace/...` both shapes + flat `TreeRootForSpecDir` regression |
| **AC13** — layout-aware/transitional panel gate + co-located tracked reviews | Bead 4 (wiring) + Bead 5 (real migration) | `go test ./internal/panel/... ./internal/complete/...`: on a canonical tree the gate scans BOTH repo-root `review/` and co-located `<spec-dir>/reviews/` (union) and a sub-threshold panel in EITHER blocks; on a flat tree co-located ONLY; Bead 5 migrates the real root `review/**` to `<spec-dir>/reviews/**` and asserts the post-move transition (root `review/` stops driving the gate once flat) |
| **AC14** — doctor layout detection + dual-layout-duplicate-ID ERROR | Bead 4 | `go test ./internal/doctor/...` (all three) |
| **AC15** — DIRECTIONAL cross-layout merge hard-fail (regression blocked, migration allowed) | Bead 4 | `go test ./internal/executor/...`: a canonical/legacy source → flat target hard-fails with the recovery line (regression), a flat source → canonical target is allowed (migration), run-state exempt — in front of `gitutil.MergeInto` (`CompleteBead`/`FinalizeEpic`, bead→spec) and the DIRECT no-remote `gitutil.MergeBranch` (`FinalizeEpic`, spec→main); the remote path pushes for a PR (mindspec_executor.go:401-405) and relies on the Bead-3 precondition + PR review, not the in-binary guard |
| **AC16** — migrate precondition blocks pre-flatten branch/PR, tolerates locked worktrees/forks | Bead 3 | precondition tests over `LocalBranchRefs`/`RemoteTrackingRefs` (block half + tolerate half) |
| **AC17** — skills/snapshots layout-correct across all path-bearing surfaces | Bead 6 | `go test ./internal/setup/...` + the breadth grep returns empty of pre-flatten paths (post-move) |
| **AC18** — dogfood eviction to `project-docs/` (resolver assertion) | Bead 5 (move) + Bead 1 (resolver) | `test -d project-docs` + resolver test (not into `project-docs/` or root `docs/`) |
| **AC19** — `glossary.md` + `policies.yml` removed, no dangling refs | Bead 5 | `grep -rn 'GLOSSARY.md\|policies.yml' internal cmd \| grep -v _test` empty |
| **AC20** — domain OWNERSHIP self-globs flat | Bead 5 | self-glob assertion (test or doctor check) |
| **AC21** — mover package owned; `project-docs/**` classified docs | Bead 3 (ownership) + Bead 2 (classification) | complete-gate test (`internal/layout/**` owned) + `go test ./internal/validate/...` (doc-classified) |
| **AC22** — Phase-1 behavior-preservation gate; flat matrix incl. context-pack; honest fixture timing | Beads 1+2 (Phase-1 gate) + Beads 1–4 (matrix) + Bead 6 (post-move fixtures) | orchestrator at Phase-1 tip: `go test ./...` green (fixtures match the still-present canonical tree) + `git diff --name-status main...<phase1-tip>` shows no `.mindspec/docs/` renames; Bead 6 migrates fixtures so the orchestrator's post-move `go test ./...` is green |
| **AC23** — `DOCS-LAYOUT.md` amended to flat layout | Bead 6 | doc-content grep asserts flat layout documented |
| **AC24** — `migrate.go` prompt rubric reconciled | Bead 6 | `go test ./cmd/mindspec/...` rubric targets `project-docs/` + flat children |
| **AC25** — `mindspec validate spec 106-layout-flatten` passes | All beads (spec-level) | spec frontmatter + ADR touchpoints kept valid; `mindspec validate spec` reports 0 errors |
