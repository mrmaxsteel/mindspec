# Workflow Domain — Architecture

## Key Patterns

### Five-Mode Lifecycle

```
Explore -> [Spec Mode] -> approval -> [Plan Mode] -> approval -> [Implement Mode] -> [Review] -> Done
```

Each mode gates:
- **Allowed outputs** — what artifacts can be created/modified
- **Required context** — what must be reviewed before proceeding
- **Transition gates** — what conditions must hold to advance

### Beads as Substrate (ADR-0023)

Beads is both the single state store and the **contract between the planning and execution layers**. Each bead is a self-contained work packet that encapsulates requirements, context (impacted domains, ADR citations), dependencies, and acceptance criteria. A fresh agent picking up a bead doesn't need session history — the bead carries everything it needs.

This is what makes execution pluggable: any orchestrator that can read beads can dispatch work. The planning layer writes beads; the execution engine reads them.

All lifecycle state is derived from Beads — no filesystem state files (no `focus`, no `lifecycle.yaml`):

| Concern | Owner |
|:--------|:------|
| Execution tracking (issues, dependencies) | Beads |
| Workflow orchestration (modes, gates) | MindSpec (this domain) |
| Phase derivation (spec lifecycle stage) | MindSpec, from Beads statuses |
| Long-form specs, ADRs, domain docs | Documentation system |

Phase is derived from epic metadata and child bead statuses:

| Condition | Derived phase |
|:----------|:-------------|
| No epic for spec | spec |
| Epic exists, no children | plan |
| Any child in_progress | implement |
| All children closed | review |
| Epic closed with done marker | done |

### Workflow/Execution Boundary (Spec 077)

Workflow packages determine *what* should happen and delegate *how* to the Executor:

```
approve/spec.go   ──▶ exec.InitSpecWorkspace()
approve/plan.go   ──▶ exec.HandoffEpic(), exec.DispatchBead()
complete/         ──▶ exec.CompleteBead(), exec.CommitAll()
approve/impl.go   ──▶ exec.FinalizeEpic()
cleanup/          ──▶ exec.Cleanup()
```

**Import rule**: Workflow packages (`approve/`, `complete/`, `next/`, `cleanup/`, `spec/`) call `executor.Executor` methods. They MUST NOT import `internal/gitutil/` directly.

### Plan Quality Responsibility

The workflow layer ensures plans are well-decomposed before handoff to the execution engine. This is critical because AI agents perform significantly better on well-structured, bitesize tasks than on vague or monolithic ones (see [arXiv:2512.08296](https://arxiv.org/abs/2512.08296)).

Workflow enforces:
- **Bead decomposition** — each bead must be a focused, independently completable unit of work
- **Clear acceptance criteria** — every bead has verifiable completion conditions
- **Dependency ordering** — beads declare dependencies so the execution engine dispatches them in the right order
- **Validation gates** — `internal/validate/` checks structural requirements and ADR compliance before plan approval

The execution engine trusts that approved plans are well-decomposed and simply executes them — it does not assess plan quality.

### ADR Governance

- Plans must cite ADRs they rely on
- Divergence detected at any mode triggers the ADR divergence protocol
- New superseding ADRs require human approval before work resumes

## Invariants

1. No code changes without an approved spec AND approved plan.
2. Implementation scope cannot widen — discovered work becomes new beads.
3. ADR divergence always triggers a human gate.
4. Bead closure requires proof-of-done + doc-sync.
5. Beads is the single state store — no filesystem state files.
6. Workflow packages never import `internal/gitutil/` — all execution goes through `Executor`.

## Agent error contract (spec 092, ADR-0035)

- Guard failures route through `internal/guard.NewFailure` /
  `FormatFailure(msg, commands...)`: the final line of every guard
  failure is a copy-pastable `recovery: <command>` line.
  `IsBannedRecoveryCommand` rejects raw `bd update --metadata`
  suggestions, and the convention test
  (`internal/guard/recovery_convention_test.go`) fails when an
  exported failure constructor produces a failure without one.
- `mindspec impl approve` treats `mindspec_phase` as a trusted cache:
  when the stored phase fails the review/done gate but the
  child-derived phase passes, gating continues read-only on the
  derived phase; the forward reconcile is deferred until after the
  last pre-terminal gate (Reqs 1–2). `mindspec repair phase <spec-id>`
  is the manual reconcile.
- `internal/next.CheckDirtyTreeDetail` splits dirty-tree detection
  into artifact dirt vs user dirt; only user dirt blocks, and the
  failure names the offending paths and the active worktree.
- Operator-facing command forms are noun-verb (`mindspec spec
  approve`, `mindspec plan approve`, `mindspec impl approve`) across
  instruct templates and skipped-gate warnings.
- **Layout-aware panel gate (Spec 106).** `internal/panel.Scan` globs
  BOTH the historical repo-root `review/<slug>/panel.json` and the
  spec-scoped co-located `<spec-dir>/reviews/<slug>/panel.json`
  conventions under each root it is handed (internal/panel stays a
  dependency-clean leaf and does no layout I/O). `complete.Run` chooses
  WHICH roots to scan from the tree's docs layout
  (`workspace.DetectLayout`, via `panelGateRoots`): a canonical/legacy
  tree honors the UNION — repo-root + bead-worktree `review/` AND the
  co-located `<spec-dir>/reviews/` — so root `review/` panels keep
  driving the gate through the transition; a flat tree honors the
  co-located `reviews/` ONLY, and a leftover root `review/` panel no
  longer drives the gate. A sub-threshold panel in EITHER honored
  location blocks `complete`.
- **Doctor layout detection (Spec 106).** `mindspec doctor` reports the
  detected docs layout (reusing `workspace.DetectLayout`), emits a
  `would-migrate-layout` Warn when a canonical/legacy tree would flatten
  on the next `mindspec migrate layout`, and ERRORs (`dual-layout-spec:
  <id>`) when the SAME spec id exists under two layout tiers — the
  stale-duplicate read hazard a half-migrated tree creates. The
  dry-run-migration spec walk is tier-aware (`workspace.SpecsDir`), so a
  flat tree's specs are still enumerated.
- **Permanently multi-prefix gate matchers + tier-aware enumerators
  (Spec 106).** The doc-sync / ADR-divergence / ownership lanes match git-DIFF
  PATH STRINGS, so the per-artifact filesystem resolvers cannot absorb them.
  `internal/validate` carries a relative-path layout classifier whose matchers
  (`isDocFile`, `specMDID`, the domain-doc prefixes in `checkInternalPackages`,
  the cmd-docs accept-set, `isADRMarkdown`, `isProcessArtifact`,
  `listDomainDirs`/`listDomainDirsAtRef`, `LoadOwnership`, and — critically —
  the ref-anchored `LoadOwnershipAtRef` / `domainManifestRelPaths` pair the
  `complete` ADR-divergence gate loads ownership through) recognize ALL THREE
  layouts: flat `.mindspec/{specs,adr,domains,core}/`, canonical
  `.mindspec/docs/...`, and legacy `docs/...`. This posture is PERMANENT and
  decoupled from the filesystem read-tier lifecycle — historical refs, old
  branches, and external forks emit the canonical/legacy paths forever. The
  root `review/**` exclusion is KEPT as a permanent historical-ref matcher; an
  ADDITIVE `/reviews/` segment matcher classifies co-located
  `<spec-dir>/reviews/**` non-source; and `project-docs/**` (the dogfood
  eviction tree) classifies non-source docs so neither the doc-sync source lane
  nor `adr-divergence-unowned` trips. The enumerating consumers (`spec list`,
  `domain list`/`show`, the doctor docs/orphan scans via `docsRootRel`) swap
  hardcoded `.mindspec/docs/...` joins for the Bead-1 tier-aware accessors
  (`workspace.SpecsDir`/`DomainsDir`), so they enumerate identically on
  flat/canonical/legacy. `isProcessArtifact` additionally classifies the
  irreversible-flatten run-state (`isMoverRunState`: `.mindspec/lineage/**`,
  `.mindspec/migrations/<run>/**`) and the ADR-0018 vestigial config drops
  (`isVestigialConfigDrop`: `.mindspec/policies.yml`,
  `.mindspec/docs/glossary.md`) as workflow-owned process artifacts, so the
  flatten's own diff (and future real user cutovers) does not trip
  `adr-divergence-unowned`. `isDocFile` recognizes the repo-root operator docs
  (`rootOperatorDocs`: `CLAUDE.md`, `AGENTS.md`, `README.md`, `BENCH-MOVED.md`)
  by EXACT name — consistent with `internal/layout` `DefaultRootDocs` and
  `internal/doctor` `movedTreeRootDocs` — so the link-repair edits the flatten
  makes to `README.md`/`BENCH-MOVED.md` classify as docs and likewise do not
  trip `adr-divergence-unowned`.

## Dead-code sweep — spec 107 wave 1 (2026-07-02)

Bead `mindspec-oexu.1` removed the following confirmed-dead workflow-domain
symbols (zero live callers, `deadcode -test`-clean):

- `internal/hook/helpers.go`: `hasPathPrefix`, `stripEnvPrefixes`,
  `parseEnvPrefixes`, `isEnvVarName`, `getCwd` (kept the live `dirExists`).
- `internal/next/beads.go`: `findRoot` (unused workspace-root probe; the live
  `cmd/mindspec` `findRoot` is a separate function that stays).
- `internal/doctor/doctor.go`: the thin `Run` wrapper (all callers use
  `RunWithOptions`).
- `internal/validate`: `SpecStatusFromBytes` + `SpecIsApproved`
  (`frontmatter.go`), `IsDomainCoveredCtx` (`plan.go`), and the `BeadID`
  re-export (`specid.go`); the dangling comment references to the removed
  helpers were repaired in `internal/instruct/instruct.go` and
  `internal/validate/plan.go`.
- `internal/panel/gate.go`: the unused `skipHumanHint` const.
- `internal/layout/mover.go`: `Mover.WithPlan`/`WithRules`/`WithRootDocs`
  (never-called chaining setters).
- `plugins/mindspec/embed.go`: `SkillNames` + its `sortStrings` helper.
- `cmd/mindspec`: the no-op `SetUsageTemplate` line in `hook.go` (a
  `strings.Replace` of a string with itself) and the `--mode`/`--spec`/`--bead`
  flags registered on the deprecated no-op `state set` command.
