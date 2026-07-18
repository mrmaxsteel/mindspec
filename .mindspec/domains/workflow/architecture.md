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
- **Recorded `approve_threshold` extension + leaf-safe reviewer-count
  advisory (Spec 109, ADR-0037 §3 amendment / ADR-0040).**
  `internal/panel.Panel` gains one new optional field,
  `ApproveThresholdExpr` (`json:"approve_threshold,omitempty"`).
  `(Panel).ApproveThreshold()` stays the SOLE interpreter (no second
  interpreter anywhere): absent/empty and `"n-1"` (case-insensitive) both
  resolve to `ExpectedReviewers − 1`; an integer string in
  `[1, ExpectedReviewers]` overrides the default for that panel only;
  an out-of-range integer (`0`, negative, `> N`) or any other
  unparseable value falls back to `ExpectedReviewers − 1` — a recorded
  `0` can never yield a free-pass threshold of `0`. That record-side
  fallback composes with, and does not replace, the pre-existing
  gate-side guard `threshold > 0` in `internal/panel/gate.go`'s
  `PanelGateDecision` (10) — the two defenses are deliberately
  redundant. `internal/panel` remains a dependency-clean leaf: it
  imports no `internal/config`, and `internal/config`'s
  `PanelApproveThresholdExpr()` resolver returns the raw, unresolved
  expression precisely so resolution stays single-homed here. The new
  pure, config-free helper `panel.ReviewerCountNote(recorded,
  configDefault int) string` gives the two caller-side surfaces (`mindspec
  config show`, the complete-gate advisory) an advisory line when a
  panel's recorded `expected_reviewers` differs from the config
  default; it returns `""` on a match and is never consulted by
  `PanelGateDecision`, so no `Allow`/`Block` outcome changes.
- **Spec-approve parser parity (Spec 110 Bead 2, R5/R6).** `spec approve`
  (via `internal/approve.ApproveSpec`, which hard-fails on
  `validate.ValidateSpec`'s `vr.HasFailures()`) now inherits two
  plan-approve parser-parity checks folded directly into `ValidateSpec` —
  no `internal/approve` source change was needed since the gate was
  already `ValidateSpec`-shaped. **R5**: the `## Impacted Domains` entries
  are resolved through the identical
  `normalizeImpactedDomains(nil, root, "", impacted)` call
  `internal/validate/plan.go` makes, emitting the same
  `impacted-domains-resolve` `SevError` for a path-like zero/multi-owner
  entry; a bare-name-no-manifest entry that plan-approve tolerates today
  still passes — no stricter rule than plan-approve. **R6**: anchored
  `## ADR Touchpoints` markdown links (both the bare-ID form
  `[ADR-0031](...)` and the filename-form `[ADR-0031-doc-sync-gate.md](...)`)
  are resolved for EXISTENCE ONLY against the same
  `newMemoStore(adrStoreForSpecFn(root, specDir))` the plan-time citation
  gate reads, emitting `adr-touchpoint-missing` on a dangling reference; a
  bare-prose `ADR-####` mention with no `[...](...)` anchor is never
  matched. Deliberately out of scope at spec-approve (R7c, ADR-0032):
  Accepted-status, domain-intersection (`adr-cite-irrelevant`), and
  coverage (`adr-coverage-*`) evaluation all stay plan-approve-only.
- **Recorded `gate` field, decision-inert (Spec 112, ADR-0037 §1
  amendment).** `internal/panel.Panel` gains one new optional field,
  `Gate` (`json:"gate,omitempty"`) — the gate mix
  (`spec_approve`/`plan_approve`/`bead`/`final_review`/`adhoc`) the
  panel was created from, by convention but parse-lenient like
  `AbandonReason`: an unexpected or absent value never sets
  `Registration.Err`. It is DECISION-INERT — `PanelGateDecision` and
  `ApproveThreshold()` never read it, so its presence, absence, or
  value changes no `Allow`/`Block` outcome and no threshold. Name
  (`gate`), type (string), `omitempty`, and parse-lenience are a
  stable contract (spec 112 R9); no follow-up may change any of the
  four silently. `internal/panel` remains a dependency-clean leaf:
  this field is the bead's only touch point — no new import, no new
  function, and the config-free-leaf invariant (no `internal/config`
  import) is unchanged.
- **Gate-aware reviewer-count advisory selection rule (Spec 112 R7).**
  Both `panel.ReviewerCountNote` callers — `internal/complete`'s
  complete-gate advisory and `mindspec config show`'s panel scan — resolve
  the comparison default through one shared, config-homed rule,
  `(*config.Config).PanelGateAdvisoryDefault(recordedGate string, isBead
  bool) (int, bool)`, so the two surfaces cannot drift from each other.
  The selection: (1) `gates:` unconfigured (`len(Gates) == 0`) → the flat
  global default, exactly as Spec 109 shipped it; (2) a recorded gate that
  is one of the five `panel.gates` keys → that gate's own resolved
  default; (3) a gate-less bead panel (`Panel.IsBead()`, `Gate == ""`) →
  the `bead` gate's default; (4) anything else — a gate-less non-bead
  panel, or a recorded value outside the five-key enum — returns `(0,
  false)`, and the caller must SKIP the note rather than guess: a
  correctly-configured 9-reviewer `spec_approve` panel or 12-reviewer
  `final_review` panel must never be flagged against an unrelated
  default, and a spurious advisory trains operators to ignore real ones.
  `PanelGateAdvisoryDefault` never calls a gate-scoped resolver with a
  value outside the enum, so an unrecognized recorded `gate` can never
  surface a resolver error through the advisory path. `internal/complete`'s
  call site additionally guards on `panelReg != nil` before invoking the
  rule (`panelReg.Panel` would otherwise be dereferenced on `panelGate`'s
  nil fail-open return — the common panel-less `mindspec complete`); the
  gate's own `Allow`/`Block` decision is fully computed before this call
  runs and is never affected by it.
- **`config show` gates/substitutes/known-model rendering + `--gate`
  resolved view (Spec 112 R8/R9).** `renderConfig` renders `panel.gates`
  in `config.PanelGateKeys` enum declaration order (never map-iteration
  order) and `panel.substitution.substitutes` in sorted-key order — both
  keys always present in the output, even empty (`gates: {}` /
  `substitutes: {}`) — alongside a warning-style, exit-code-inert note for
  any model id absent from the curated `config.KnownModels()` advisory
  list. `mindspec config show --gate <name> [--json]` exposes the R3
  gate-scoped resolvers as a single per-gate view through two pure
  functions sharing one `buildGateResolvedDoc` builder, so the text and
  JSON renders and the resolvers themselves cannot disagree. The JSON
  document's five members (`gate`, `slots`, `expected_reviewers`,
  `approve_threshold`, `substitution`) are a stable, additive-only
  contract (R9) — the surface the Spec 110 `panel.json` writer and the
  Spec 111 orchestration runner consume — carrying the same
  never-silently-changed guarantee as the recorded `gate` field above.
  This bead adds no writer- or runner-side behavior; both remain out of
  scope here.
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

## Ownership claims + carve-out cleanup — spec 108 wave 2 (2026-07-02)

Bead `mindspec-wpjv.1` brought two previously-unowned repo paths under the
workflow domain by adding them to `.mindspec/domains/workflow/OWNERSHIP.yaml`'s
`paths:` list, so `validate.attributeDomain` now resolves both to `"workflow"`
and neither trips `adr-divergence-unowned` when edited:

- `internal/trace/**` — the NDJSON tracer behind the `mindspec trace`
  subcommand. The owner-facing CLI (`cmd/**`) and two of the package's three
  importers (`cmd/mindspec`, `internal/instruct`) are workflow; the third
  (`internal/bead`) is an event-emitting consumer. Workflow already owned the
  trace command, so it now owns the package behind it.
- `.golangci.yml` — repo lint config, a sibling of the other workflow-owned
  repo-tooling paths (`scripts/bd-jsonl-merge-driver.sh`, `cmd/**`,
  `plugins/mindspec/**`, `.claude/skills/**`).

With those claims in place, this bead also deleted the dead
`trace.Event.MarshalJSON` (an aliased no-op marshaler byte-identical to Go's
default struct marshaling — proven unchanged by `TestEventNDJSONGolden`) and
removed the three stale `unparam` carve-outs in `.golangci.yml`
(`internal/brownfield/plan.go`, `internal/contextpack/builder.go` `isNeighbor`,
`internal/next/beads.go` `findRoot` — all matching nothing on the tree after
the wave-1 `findRoot` deletion), keeping the live `internal/validate/state.go`
`validateReviewMode` carve-out. `golangci-lint run ./...` stays clean.

## Ownership claim — `.claude/workflows/**` (spec 111, 2026-07-09)

Bead `mindspec-9cyu.1` adds `.claude/workflows/**` to
`.mindspec/domains/workflow/OWNERSHIP.yaml`'s `paths:` list, adjacent to the
existing `.claude/skills/**` claim, so the two `.claude/**` sub-tree claims
sit together. Claude Code dynamic workflows installed under
`.claude/workflows/` (the dogfood copy Claude Code reads) are governable
source — `isDocFile` and `isProcessArtifact` both return false for them — so
without this claim, editing one would trip `adr-divergence-unowned`.

The claim lands here, in the same bead and same diff as the doc-sync you're
reading, and **before** bead `mindspec-9cyu.2` adds the tracked
`.claude/workflows/ms-panel.js` workflow artifact itself — the ADR-0036
same-diff-or-earlier invariant (the spec-108 wave-2 precedent above).
`plugins/mindspec/workflows/**` (the plugin source of truth `ms-panel.js` is
embedded from) needs no separate claim: it already resolves to workflow
through the existing `plugins/mindspec/**` glob. With the claim in place,
`validate.attributeDomain` resolves `.claude/workflows/ms-panel.js` to
`"workflow"`.

## Lifecycle control-plane integrity: gate-before-mutate + forward-reconcile (spec 119)

Spec 119 (beads `mindspec-lc12.1` through `.6`) closes a recurring defect
shape across all three mutating lifecycle verbs — `mindspec complete`,
`mindspec plan approve`, `mindspec impl approve` — where a refusal
derivable from already-available facts was discovered *mid-sequence*,
after one or more mutations had already landed. **ADR-0041
(gate-before-mutate)** is the codifying record: every verb now follows a
three-phase contract — **preflight** (resolve every immutable gate fact
and evaluate every derivable refusal before the first mutation) →
**commit** (the mutation sequence proper) → **reconcile** (a bounded,
idempotent forward path back to completion or a clean named refusal on
any interruption — never a rollback). The idempotent ADR-0034 migration
is the one exempt pre-preflight mutation in all three verbs, since it is
itself read-only-or-idempotent.

### `mindspec complete`: lineage-authoritative preflight + forward reconcile

`internal/complete.Run` resolves the bead's owning spec from its **parent
epic** (lineage), not from cwd — cwd-derived resolution is now a fallback
used only when lineage genuinely cannot answer. A `--spec` hint is checked
AGAINST the lineage spec and refused pre-mutation on a mismatch, naming
both spec IDs (AC-1/AC-2). The bd_close lifecycle-bypass guard
(`findOrphanedClosedBeadsFn`, bead `mindspec-4gsz`) and the panel gate
(ADR-0037) both still run inside this same preflight window, before the
tracker auto-commit.

The **merged-unclosed / branch-less forward-reconcile** path
(`internal/lifecycle.MergedUnclosed`/`FindLandedMerge`) is the reconcile
half: when a bead has no matching worktree AND its canonical `bead/<id>`
ref genuinely no longer exists, `Run` no longer calls `exec.MergeBase`
against the absent ref (the exit-128 bug this replaces) — it positively
identifies the bead's already-landed bead→spec merge commit via
second-parent identity and anchors every per-bead gate (doc-sync,
ADR-divergence, the bead-scope advisory) at that commit's own `M^1..M`
diff, recording durable `mindspec_reconcile_landed_merge_sha` evidence
instead of performing a git merge. This is the recovery path a killed
`complete` invocation converges through after its bead→spec merge landed
for real but the invocation died before completing (see
`internal/complete/fault_injection_realgit_test.go`'s `c5` case).

Tracker-only commits (the `--commit-msg` auto-commit and the artifact-sync
follow-up commit) are now pathspec-scoped — never an `add -A` equivalent —
and refuse rather than commit onto a main checkout when no bead worktree
is resolved (AC-3/AC-4). A bead's own advisory scope check
(`internal/complete/bead_scope.go`) warns, non-fatally, when a bead's diff
touches files outside its plan-declared `file_paths` baseline — pure
advisory, never a gate.

### `mindspec plan approve`: preflight-resolved facts, no interleaved mutation

`internal/approve.resolvePlanApprovePreflight` (called before ANY mutation
in `ApprovePlan`) reads plan.md, parses bead sections and structured
`work_chunks`, validates their alignment, resolves the target epic
FAIL-CLOSED (`resolveTargetEpic` distinguishes a bd query failure from a
genuinely absent epic — two distinct refusals, two distinct recovery
lines), and resolves + safety-checks the epic's existing child set
(`queryExistingChildren`/`checkExistingBeadsSafety`, the spec-074
re-approval safeguard) — every refusal these facts can produce fires here,
before the first mutation (the supersede-close of an all-open child set).
`createBeadsFromParsed` then consumes the SAME preflight-resolved facts
for bead creation + dependency wiring, so no re-read/re-validation/re-query
can discover a fresh refusal after mutation has begun. A best-effort `bd
dep add` failure is now named in `result.Warnings` (both bead IDs) instead
of a silent `continue` (AC-20); a missing `work_chunks` block warns loudly
instead of silently wiring zero edges (AC-19).

### `mindspec impl approve`: epic-scoped finalize + the pre-terminal orphan/obligation gate

The FinalizeEpic lifecycle allow-set (`phase.LifecycleChildIDsForEpic` ∩
the plan-declared bead IDs) is resolved as a preflight FACT, immediately
after the last read-only gate (ADR-divergence) and BEFORE the
supersede-ADR placeholder's disk write — a classification failure refuses
pre-mutation rather than falling through to the executor's own fail-closed
abort. `runOrphanObligationGate` (spec 115, extended here) is the
pre-terminal refusal gate: it fails closed on every cleanly-signaled infra
error across its three legs (the orphan scan, the worktree-enumeration
merge-prevention leg, and the durable-obligation backstop) and performs NO
epic close, NO phase write, NO merge, NO push on a refusal.

### `mindspec doctor` + `mindspec next`: shared lifecycle-divergence predicates

`internal/lifecycle.FindOrphanedClosedBeads`/`ScanOrphanedClosedBeads`/
`StaleOpenBeads`/`FindLandedMerge`/`MergedUnclosed` are the single-sourced
predicates `mindspec complete`, `mindspec impl approve`, `mindspec next`,
and `mindspec doctor` all consume identically — the anti-drift guarantee
(AC-12): a bare `bd close` that bypassed `mindspec complete`, or a bead
whose Dolt status disagrees with its landed-merge git evidence, reads the
same way from every surface. `internal/doctor`'s `stale_open.go` and
`finalize_orphans.go` wire these predicates into `mindspec doctor`'s CI
mode (`--ci`, `SkipLocalEnv`) — now gated in `.github/workflows/ci.yml` —
so a lifecycle divergence fails CI the same way a stale-schema drift
already did.

### Fault-injection regression suite (spec 119 Bead 6, AC-26)

`internal/complete/fault_injection_test.go` +
`fault_injection_realgit_test.go`, `internal/approve/impl_fault_test.go` +
`plan_fault_test.go`, and `internal/executor/finalize_fault_test.go`
classify every significant post-preflight mutation point in all three
verbs as KILL-TESTED (a real mutation lands via a real-git decorator
executor, a terminating tracker seam, or `FinalizeEpic`'s new
`finalizeStepHookFn` stage hook, then a terminal error is forced) or
DOCUMENTED-FORWARD-SAFE (the error is swallowed by design; a kill test
would be fictitious). Every kill test re-invokes the same verb and asserts
convergence to completion or a clean, named, recoverable refusal — never a
fabricated "kill" that doesn't actually terminate anything. See ADR-0041
§3 for the standing classification rule this suite implements.

Every test in this suite is hermetic: real temp git repos (never this
repo's own working tree), in-memory tracker fakes for `bd`, and — where a
production seam has no test double (e.g. `internal/validate`'s
`bead.BeadExists` → real `bd show`, which has no mock seam) — a
scoped-PATH technique (a scratch `bin/` containing only a `git` symlink)
rather than a dependency on this dev machine's real Dolt store.
