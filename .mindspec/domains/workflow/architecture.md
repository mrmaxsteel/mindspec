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
three-phase contract — **preflight** (resolve every immutable gate fact —
lineage/hint agreement, epic membership, ancestry/reconcile evidence,
orphan-sibling state, the panel decision, plan and obligation facts — and
evaluate every derivable refusal before any lifecycle-affecting mutation:
tracker close, bead→spec merge, branch deletion, epic close, or a `main`
commit) → **commit** (the mutation sequence proper) → **reconcile** (a
bounded, idempotent forward path back to completion or a clean named
refusal on any interruption — never a rollback). The idempotent ADR-0034
migration is the one exempt pre-preflight mutation in all three verbs,
since it is itself read-only-or-idempotent. `complete` additionally has an
**artifact-materialization subphase** (ADR-0041 §1): the optional user
`CommitAll` and the pathspec-scoped artifact-sync commit — local,
bead-branch-only, never-`main` — after which the doc-sync and
ADR-divergence gates validate the resulting committed bead tip and may
refuse; that refusal is forward-reconcilable (the bead-branch commits are
retained; a re-run after repair converges), not byte-identical. The
byte-identical-refusal claim holds only for the enumerated preflight
refusals.

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

Both materialization legs refuse rather than commit onto a main checkout
when no bead worktree is resolved (AC-3/AC-4; the user `--commit-msg`
`CommitAll` targets the matched bead worktree ONLY — no root fallback —
and a worktree-enumeration failure propagates instead of being swallowed),
and the artifact-sync follow-up commit is pathspec-scoped — never an
`add -A` equivalent. A bead's own advisory scope check
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

## Greenfield first-run integrity (spec 123)

Spec 123 (beads `mindspec-ud0w.1` through `.4`) makes the first hour of
MindSpec in a fresh consuming repo correct, convergent, and self-checking:
`init`, `setup claude|codex|copilot`, `domain add`, `adr create`, and
`panel create` now compose in any order without dead-ending, and
`mindspec doctor` detects every partial state with a named recovery
(ADR-0035). **ADR-0040 was AMENDED** with the consumer-identity clause:
content mindspec generates INTO a consuming repo carries only
framework-generic guidance or values sourced from the consumer's L2
declared config — never mindspec-the-framework's own repo facts.

### `init` scaffolds a first-run-complete workspace (#207, #208)

The `bootstrap.Run` manifest additionally scaffolds
`.mindspec/context-map.md` from `domain.ContextMapSkeleton()` — a title,
a `## Bounded Contexts` section, and a `---` separator, so
`appendContextMap`'s insertion scan finds its intended insertion point
immediately. The item follows the manifest's additive discipline (never
overwrites an existing file). After the manifest loop, `init` calls
`gitutil.EnsureGitignoreEntries(root, gitutil.RuntimeIgnoreEntries...)`
— an entry-granular, byte-idempotent append — so the two runtime files
(`.mindspec/session.json`, `.mindspec/focus`, ADR-0015) are gitignored
even when the repo already HAD a `.gitignore` (the pre-123 manifest item
was create-only and silently `Skipped` on existing files). The manifest
is now built per-run from `config.Load(root)` (`manifest(cfg)`) because
the starter `AGENTS.md` content is config-sourced (see consumer identity
below); a config load failure fails init loudly rather than scaffolding
from a half-read config.

### `domain add` converges from every partial state (#207)

`internal/domain/scaffold.go` replaced the dir-exists refusal with a
convergence check: when `context-map.md` is absent, `appendContextMap`
creates the skeleton first; when the domain dir exists but files or the
context-map entry are missing, `domain add <name>` backfills each missing
standard file (four templates + `OWNERSHIP.yaml`, written only if
absent — existing files are never overwritten) and the missing
`### <Title>` entry; only a fully-scaffolded AND mapped domain is
refused "already exists". Any failure after dir creation leaves a state
a bare re-run repairs — no terminal partial state.

The "is this domain mapped" predicate is ONE shared helper,
`domain.HasEntry` (`internal/domain/contextmap.go`), consumed by both
the emission side (scaffold's backfill) and the detection side
(doctor's unmapped-domain check, via the `docsMappedCheck` seam var) —
`TestDocsMappedCheckIsSharedHelper` pins the identity so the two sides
cannot silently diverge (AC-4). `HasEntry` is SECTION-AWARE: a
`### <Title>` heading counts only inside the `## Bounded Contexts`
section, before its `---` terminator — the exact place the writer emits
it.

### New doctor checks (#207, #208, #210, #211)

- **missing-context-map** (docs lane, `internal/doctor/docs.go`):
  `context-map.md` absent at the layout-resolved path → `Missing`, with
  a `--fix` that scaffolds the same `ContextMapSkeleton()` bytes init
  writes. An existing-but-unreadable file is an `Error` with NO fixer
  (the scaffold would no-op yet report Fixed — kept honest).
- **unmapped-domain** (docs lane): a `domains/` directory with no
  corresponding entry heading (per `domain.HasEntry`) → `Warn` naming
  the domain with recovery `mindspec domain add <name>` — deliberately
  no `--fix`, since the backfill is `domain add`'s own action. Runs only
  once the context map exists (one root cause, one finding).
- **runtime file not gitignored** (git lane, `internal/doctor/git.go`):
  a runtime file that is untracked AND misses `git check-ignore` →
  `Warn` ("one `git add .mindspec/` from being committed") with a
  `--fix` appending the entry via `EnsureGitignoreEntries`. The
  pre-existing tracked → `Error` + untrack `--fix` is unchanged and
  takes precedence. The protected file set is sourced from
  `gitutil.RuntimeIgnoreEntries` — the single canonical list bootstrap,
  setup, and doctor all share.
- **missing-models** / **missing-commands** (config lane,
  `internal/doctor/config.go`): mirror `checkSourceGlobs`'s ADR-0036
  stack. Each fires when the key has no non-blank entry
  (`HasDeclaredModels`/`HasDeclaredCommands` — an all-blank map is NOT
  declared), discloses the key's status honestly (`models:` is
  declared-but-INERT; `commands:` IS consumed by init/setup's managed
  AGENTS.md rendering), hints the populate command, and carries a
  `--fix` that scaffolds a literal commented schema block
  (`modelsBlock`/`commandsBlock`) with the three-state byte-preserving
  `scaffoldConfigBlock` contract (file absent / key absent / key
  present — operator bytes never rewritten).

These are first-run nudges, not CI breakers: missing-context-map is
`Missing` (structural, like the sibling dir checks); the rest are `Warn`.
A greenfield fixture DESIGNEDLY shows the missing-models and
missing-commands Warns (ZFC: the framework cannot guess them).

### ADR slugged-filename / canonical-ID convention (#206)

`adr create` now derives a kebab slug from the title (`deriveSlug`:
lowercase, non-alphanumeric runs collapse to single hyphens,
trimmed, length-capped) and writes `ADR-NNNN-<slug>.md`; `--slug`
overrides the derivation (validated lowercase-kebab; an explicit empty
`--slug` opts out); a title deriving an empty slug falls back to the
bare form. Every computed stem must pass `idvalidate.ADRID`. `ParseADR`
derives `ADR.ID` as the canonical `ADR-<digits>` prefix of the stem —
`list`/`show` report `ADR-0001` for `ADR-0001-integrate-at-contracts.md`,
never the long stem. ID→file READ resolution goes through the shared
`workspace.ResolveADRFile` (see the core domain docs): canonical-number
driven, bare-or-slugged tolerant, and COLLISION-ERRORING when both a
bare and a slugged file carry one number (with a `recovery:`-prefixed
prose diagnostic — not ADR-0035's copy-pastable command form) —
replacing the silent exact-`<id>.md` short-circuit in `show`
and the exact-join miss in `--supersedes`/`Supersede`/`CopyDomains`.
Existing bare files keep their IDs and behavior; no rename migration
(canonical `ADR-NNNN` remains the reference currency everywhere, so
ADR-citation gates are untouched — ADR-0032 protected).

### Declared-config parity + consumer identity (#210, #211)

`models:` reaches guidance parity with `source_globs:`: schema block
(doctor `--fix`), ZFC populate prompt (`mindspec models populate` —
prints, writes nothing), doctor nudge, and the `mindspec config` inert
annotation retained until an enforcement spec removes it. The new
`commands:` key (core domain: `config.Commands`) is the consumer's
declared build/test guidance with the same stack (`mindspec commands
populate`) — but NOT inert. Rendering the managed AGENTS.md
"Build & Test" section from config goes through the ONE renderer
`cfg.RenderBuildTestSection`, and only TWO verbs render it as ordinary
operation: `init` (`internal/bootstrap`, the starter AGENTS.md /
append block) and `setup codex` (`ensureAgentsMD`, which owns
AGENTS.md's managed block outright and refreshes it from config on
every run — so a codex setup refresh re-renders the operator's
declaration and it survives every wholesale block replacement).
`setup claude` and `setup copilot` do NOT render or refresh the
Build & Test section on an ordinary run — on AGENTS.md they are
heal-only (below; the heal reaches the same renderer, but only when a
pre-123 leak is positively detected).

Managed/scaffolded consumer content no longer carries framework facts:
the starter `AGENTS.md` title is the neutral `# AGENTS.md` (was
"# AGENTS.md — MindSpec Project"), and NO managed block hardcodes
`make build`/`make test` — with `commands:` unset the Build & Test
section is OMITTED entirely (never a placeholder that reads as
runnable). ALL THREE setup verbs (`codex`, `claude`, `copilot`) heal a
pre-123 framework leak in an EXISTING AGENTS.md (final review G3
closed the former claude/copilot block gap); both heals are
provenance-gated (FX-3: they fire only when the file also carries a
well-formed MindSpec managed BEGIN/END pair — an operator's own file
is never touched) and skipped in `--check` mode:

- **Title heal** (`healLegacyAgentsMDTitle`, run by all three verbs):
  rewrites only the byte-exact pre-123 leaked first line
  ("# AGENTS.md — MindSpec Project") to the neutral `# AGENTS.md`.
- **Block heal**: `setup codex` heals a leaked managed block as a
  side effect of `ensureAgentsMD`'s unconditional config-sourced
  refresh. `setup claude`/`setup copilot` instead run the narrow
  `healLegacyAgentsMDBlock`: it rewrites the managed block from
  config ONLY when the existing content positively carries a pre-123
  leak — the exact legacy hardcoded Build & Test comment literals
  (`legacyAgentsMDBlockLeakSnippets`) or the legacy title line. A
  clean, already-config-sourced AGENTS.md is left byte-untouched and
  config is not even loaded for it.

Both the codex render and the claude/copilot block heal FAIL LOUD on a
bad `.mindspec/config.yaml` (FX-1): the load error propagates and the
existing block is left byte-untouched, never silently regenerated from
`DefaultConfig` (which would erase the consumer's declared build
guidance). All three setup verbs also ensure the runtime gitignore
entries (R4b), since `setup` is the onboarding verb for repos that
never ran the greenfield-only `init`.

### Ad-hoc panel path (#209)

`mindspec panel create <slug> --gate adhoc --target <ref>` succeeds
WITHOUT `--spec`: `panelDirFor` gained an adhoc branch writing the
panel dir + `panel.json` to `.mindspec/reviews/<slug>/` on a flat
layout (repo-root `review/<slug>/` on non-flat) — exactly the location
the shipped `ms-panel-run` skill documents, whose ad-hoc note now
states the real invocation (skill↔binary contract, grep-pinned by
AC-17). Supplying `--spec` together with `--gate adhoc` is refused with
a recovery line naming both valid forms (keyed on
`Flags().Changed("spec")`, so a blank explicit `--spec` cannot slip
the guard); every non-adhoc gate still requires `--spec`,
byte-identical to before. `panel tally`/`verify` operate on the ad-hoc
dir, restoring the deterministic tally for design/proposal reviews.
Ad-hoc panels stay stored artifacts OUTSIDE the lifecycle gate:
`mindspec complete`'s panel scanning never consults
`.mindspec/reviews/` (ADR-0037 scope, pinned by the AC-16 isolation
test in `internal/complete/panel_adhoc_isolation_test.go`).
## Domain/ADR gate truthfulness (spec 122, ADR-0032 third amendment)

Spec 122 (beads `mindspec-gvb5.1` through `.4`; GH #147/#178/#145/#197 +
bead `mindspec-6ou2`) makes the domain/ADR gate lanes in
`internal/validate` reject bad domain labels at authoring time, resolve
BOTH sides of every coverage comparison, and tell the truth in their
remedy hints. It adds NO new gate lane, flag, config key, or override
(spec 122 R7 — the ceremony non-inflation guard in
`cmd/mindspec/ceremony_guard_test.go` pins this); the existing escapes
(`--override-adr`, `--supersede-adr`, `--allow-doc-skew`) are untouched.
ADR-0032 carries the codifying record as its THIRD `## Amendment`
section, including the evidenced supersession of bead `mindspec-6ou2`'s
6/6 panel decision (2026-06-26).

### Forward-only Rule-2 authoring reject (R1)

`normalizeImpactedDomains`'s Rule 2 keeps a bare `## Impacted Domains`
token that names no domain dir verbatim with no error — which let a
label that can never own a file (so every downstream coverage decision
is vacuously false) survive into an approved spec.
`bareUnresolvedImpactedDomains` + `impactedDomainsForwardOnlyErrors`
(`internal/validate/ownership_resolve.go`) now identify those Rule-2
entries and the two AUTHORING consumers —
`checkImpactedDomainsResolutionParity` (`spec.go`, so `validate spec` /
`spec approve` see it) and `ValidatePlan` (`plan.go`) — promote them to
hard `impacted-domains-resolve` errors, but ONLY when:

- the SPEC's own frontmatter status (`SpecStatusAt(specDir)` — never the
  plan's `isApproved`) is an explicit case-folded `Draft`. `Approved`,
  any other explicit non-Draft value, and status-less legacy specs (no
  frontmatter / no `status:` key) are GRANDFATHERED — the existing
  corpus never newly reddens; and
- the ownership model is IN USE: at least one enumerated domain dir
  whose `OWNERSHIP.yaml` actually LOADS (`ManifestPath != ""` through
  the shared per-run `ownershipCache`). A manifest-less workspace, or a
  scaffolded-but-empty domains tree, keeps Rule 2's verbatim-keep
  exactly (ADR-0036's manifest-less doctrine).

The error text names the offending entry (termsafe-escaped, per element
— the fl91 lesson), the sorted available domain-dir names, and both
working remedies (rename to a real domain-dir name, or claim a path
under the LAYOUT-AWARE domains root — see the hint-root helper below).

### ADR-side symmetric name-resolution (R2, supersedes 6ou2)

Before spec 122, a cited ADR's `Domain(s)` line was compared to the
spec's RESOLVED Impacted-Domains set by literal string equality, so an
ADR writing a directory path (`src/orders/`) never intersected the
spec-resolved name `orders` — the spurious `adr-cite-irrelevant` /
`adr-coverage-missing` pair (6ou2 items 3/4, #147's coverage tail).
`domainResolvingStore` (`internal/validate/adr_domain_resolve.go`) is an
`adr.Store` decorator that resolves every returned ADR's `Domains`
through the SAME deterministic explicit-manifest mechanism the spec side
already uses (glob-match against per-domain OWNERSHIP `paths:` minus
`exclude:`), layered at the two GATE-LANE store constructions only —
`ValidatePlan` and `ValidateDivergence`, each wrapping the spec-108
`newMemoStore`, each fed the lane's own exec/root/ownerRef so both
comparison sides read the same tree. The cmd-side `adrReadStore`
(`cmd/mindspec/adr.go`) is deliberately NOT wrapped: `adr show`/`adr
list` keep rendering the author's literal `Domain(s)` line.

Resolution is literal-first, two ordered phases: **Phase 1** glob-matches
the trimmed literal token against every domain's `paths:`; exactly one
clean owner is authoritative and returns immediately. **Phase 2** (zero
literal owners only) runs the exclude-gated synthetic-child probe
`<trimmed>/x` so a slashless directory label (`src/orders`) still
matches a `src/orders/**` glob — a domain that excludes the DECLARED
label is never resurrected by the probe. Three safety doctrines hold
throughout: (1) **no-new-error** — zero/ambiguous resolution leaves the
entry exactly as authored and compares literally as before; ADR
`Domain(s)` lines are historical documents this gate must not force
churn on (mindspec's own ADR-0032/-0031 lines carry non-short-tag
tokens); (2) **indeterminate-on-load-error** — if ANY enumerated
domain's manifest fails to load, cardinality is unknowable and the entry
stays literal, never promoted; (3) tuple/prose tokens (no `/`, e.g.
`api (lola, tools)`) are never parsed or guessed — which is what answers
the ZFC objection in 6ou2's superseded panel decision (resolution is
restricted to deterministic path-shaped entries against EXPLICIT
manifests, the identical spec-100 mechanism).

### Truthful gate hints (R3/R4)

- **Uncited-covering-ADR remedy (`plan.go`, #145 friction 1).** When
  `checkADRCoverage` finds domain `d` notCovered but the same in-hand
  (already domain-resolving) store contains UNCITED Accepted ADRs whose
  resolved `Domain(s)` cover `d`, the `adr-coverage-missing` error now
  names those ADR IDs and the true governing fix — add them to the
  plan's `adr_citations` frontmatter — FIRST, ahead of the spec-100
  remedies (amend a cited ADR's `Domain(s)`; `mindspec adr create` last).
  The trigger is the EXISTENCE of an uncited covering ADR
  (`uncitedCoveringADRs`), not an empty citation list; a `store.List`
  failure degrades to the pre-existing remedies rather than blocking on
  a secondary read.
- **Layout-aware, ref-consistent hint roots (`hint_root.go`).**
  `domainsRootLabel(root)` renders the domains-enumeration root that
  ACTUALLY resolves in the workspace (flat `.mindspec/domains` →
  canonical `.mindspec/docs/domains` → legacy `docs/domains`, mirroring
  `workspace.DomainsDir` exactly) instead of a hard-coded pre-flatten
  literal; `domainsRootLabelAtRef(exec, root, ownerRef)` is its
  ref-consistent sibling for the bead-time lanes, resolving the label
  from the SAME git tree the ownership enumeration read (via the
  existing `domainsTreeRoots` + `TreeDirsAtRef` seam), falling back to
  the ambient label on a git read failure or an unpopulated ref. Wired
  into `docsync.go`'s two `internal-docs` templates, `divergence.go`'s
  unowned remedy, and R1's error text; a hard-coded-literal sweep guard
  keeps new hint sites from regressing.
- **Owned-vs-unowned split (`divergence.go`, #178's message half).**
  When a diffed file is not claimed by the spec's DECLARED candidate
  domains, `ValidateDivergence` re-attributes it against the FULL domain
  enumeration (the `checkInternalPackages` pattern — message truthfulness
  only; the candidate-set pass/fail boundary and blast-radius guard are
  unchanged) and splits the finding: **owned-but-undeclared** (scope
  drift) names the real owning domain and the true remedy — add that
  domain to the spec's `## Impacted Domains`; **genuinely unowned**
  keeps the claim-it remedy, now with the layout-aware root. Ownership
  is INDETERMINATE — a distinct `adr-divergence-attribute` error naming
  the load failure and its remedy, never a false "not claimed by any
  OWNERSHIP.yaml" — when the enumeration cannot be read or any domain's
  manifest fails to load during attribution (a broken manifest may hide
  a real owner; the same load-error-swallowing anti-pattern fixed on the
  ADR side). All three are `SevError`, overridable via `--override-adr`
  exactly as before.

### Regression evidence + non-inflation pins (R5/R7, Bead 4 — test-only)

Bead `mindspec-gvb5.4` adds no behavior: the issue→test evidence map for
#147/#145/6ou2 (citing the pre-existing pins, adding the genuinely-new
#147 end-to-end divergence fixture that is red without R2 — the
strict-inequality witness catches a reverted resolver), the
`internal/approve` plan-scaffold `adr_citations` pin, and
`cmd/mindspec/ceremony_guard_test.go`'s R7 guard that the CLI grew no
new flag/key (pflag-metadata-based, so an underscore-spelled flag cannot
slip past it). The contextpack backtick-strip BEHAVIOR (6ou2 item 1)
already works — `internal/contextpack/spec.go` strips `**`/backtick
markdown noise from domain tokens before normalization (spec 087 Bead 1
fixup) — but its regression PIN is DEFERRED to a follow-up bead per the
plan's PF-3 decision, to avoid pulling the spec-excluded `context-system`
domain into this spec's scope. It is NOT a Bead 4 deliverable.

## Landed-merge attestation integrity (spec 125, ADR-0041 §2(ii) amendment)

Spec 125 (beads `mindspec-xhd5.1` through `.4`) makes the landed-merge
attestation substrate ADR-0041 §2(ii) relies on actually hold in the
real topology. Write side (execution domain — see its architecture doc):
`complete`/`FinalizeEpic` persist the binding via bead-tip
`gitutil.ExactSecondParentMerges` ground truth regardless of the merge's
subject format, and a genuine locate miss on a bead that DID land is
LOUD and cleanup-suppressing, never silently swallowed. Read side
(this domain): `FindLandedMerge` is rebuilt on a two-source
ownership/landed-ness model, the revert-vs-evolved discrimination gains
the `RevertShape` sub-classification, and the new `mindspec reattest`
verb is the explicit audited recovery. ADR-0041 carries the codifying
record as its `## Amendment (Spec 125): Re-attested landed-bindings
under §2(ii)` section, finalized by Bead 4 with the first citing code
(the 122/123 amendment-lifecycle precedent).

### `FindLandedMerge`: two-source ownership identity (R5)

Two orthogonal facts from two different sources, neither alone
sufficient:

- **OWNERSHIP** ("which bead") is the merge SUBJECT's bead-branch name —
  `parseMergeSubjectBeadBranch` recognizes BOTH subject forms
  (`MergeInto`'s `Merge bead/<id>` and git's default conflict-recovery
  `Merge branch 'bead/<id>' into …`), matched by FULL branch-name
  equality, never prefix/substring (AC-2f). The parser is deliberately
  conservative (G2-2): a `bead/…` token in an unrecognized subject shape
  still NOMINATES that bead (so a different bead's token rejects on the
  equality check) and is never collapsed into the names-no-bead state.
- **LANDED-NESS** ("did it land") is git TOPOLOGY: a two-parent
  first-parent merge on the spec branch whose second parent EQUALS the
  bead's landed tip. The pre-125 ancestor-TOLERANT confirmation legs
  (panel `reviewed_head_sha`, surviving branch tip, binding second
  parent, each of which could "confirm" via `IsAncestor`) are REMOVED:
  every corroboration is now exact-equality only, and a non-equal datum
  is a fail-closed contradiction (AC-2b/AC-2c/AC-2d), never a softened
  confirm. With no exact-and-owned match the function REFUSES rather
  than picking the newest ancestor-consistent merge.

The bd-metadata binding (and the panel SHA) is a git-corroborated CACHE
over an already-subject-owned candidate — never an ownership authority
of its own. The binding leg is PAIR-CONSISTENT (final-review FIX-2a,
mirroring the write-side two-key check): every present binding key must
agree with the SAME real merge — a present-but-contradictory merge SHA
or second parent fails closed, never ignored because the other key
matches. Same-second-parent re-merges (one bead's repeated landings)
are resolved newest-names-the-merge, but the R5(d) content check is
anchored on the OLDEST such merge M₁ (AC-2e): a later re-merge's own
first parent can itself be the post-revert state, so a newest-anchored
three-way would mis-attest reverted content. The single-merge case
reduces exactly to R3. Owned candidates with DIFFERENT second parents
are genuine ambiguity and FAIL CLOSED (final-review FIX-2b) — the same
refusal the reattest surface applies — never silently resolved to the
newest.

**Anonymous-subject merges FAIL CLOSED (G-1 — supersedes the spec's R1
residual wording).** spec.md's R1 residual originally said a merge with
a wholly-custom subject naming NO bead remains read-identifiable via a
complete-time binding. Bead 3 shipped the SAFER behavior and the docs
describe what shipped: there is deliberately NO anonymous-subject
binding-SHA entry point in `FindLandedMerge`. Git-corroborating a
binding proves the merge is REAL with that exact second parent — it does
NOT prove the merge is THIS bead's — so admitting an anonymous merge on
the binding alone would make the agent-writable binding an independent
ownership authority: a forged binding (a metadata-forge, easier than a
commit-forge and thus below the documented git-history threat boundary)
on a never-landed bead pointing at any real anonymous merge would be
positively identified. mindspec's own merges ALWAYS name the bead (both
subject forms carry the branch name), so an anonymous subject arises
only from a hand-crafted operator merge — and it is NOT auto-recoverable
via `mindspec reattest` either: reattest's ownership nominator is the
same subject parse, so it refuses an anonymous-subject merge too (it
cannot nominate a merge whose subject names no bead). The honest
recovery is to re-merge the work under a bead-naming subject, or to
accept the safe fail-closed (MF-3 refuses that dependency); the audited
ADR-0035 `mindspec-q9ea` human attested-restore is the last-resort exit.
The spec's forward-only residual text is superseded by this fail-closed
shape.

### Revert-vs-evolved discrimination (R3, the 8nhe.2 fix)

`FindLandedMerge`'s R5(d) content check still discriminates
`ContentSubsumedOutcome(base=M₁^1, ours=tip, theirs=M₁)`: Landed →
identified; Conflict → identified (landed-then-evolved, the spec-121
F2-2r arm, byte-identically UNTOUCHED by 125 — including its documented
pre-existing Conflict-hides-revert false-positive residual, a named
follow-up). What 125 changes is ONLY the `SubsumptionCleanDivergence`
arm, which used to refuse wholesale and thereby false-rejected
evolved-but-present content (bead `mindspec-8nhe.2`): it is now
SUB-CLASSIFIED by `gitutil.RevertShape` (`landedRevertShapeFn`, pinned
to the real primitive by an anti-drift test) — the reverse un-apply,
rename-safe (`merge.renames=false`) no-op test. Only the revert SHAPE —
the tip carries NONE of M's introduced content (a true `git revert M`,
or the content-indistinguishable clean-full-removal residual, a
deliberate false-negative floor) — refuses; a clean divergence that
RETAINS part of M's content (partial supersession by later honest work)
identifies. A RevertShape error is an UNDETERMINED result and
propagates — never mapped to either classification.

### `mindspec reattest <bead-id>`: explicit, git-corroborated, audited recovery (R4)

`cmd/mindspec/reattest.go` + `internal/lifecycle.ReattestLandedMerge`
are the operator-invoked recovery for an already-merged bead whose
binding is missing or stale (the pre-125 fleet state: 755/757 beads).
The engine DERIVES the binding from an independent git scan — a
two-parent first-parent merge whose subject names this bead (ownership)
and whose exact second-parent topology proves it landed — and writes
ONLY scan-derived SHAs. Non-circular by construction: there is no
parameter, flag, or argument through which a caller can assert a
merge/second-parent pair (AC-8), no bypass flag, no `--all` fleet mode
(mass mutation stays a scripted sequence of explicit per-bead
invocations), and `doctor` never writes through it. Fail-closed on
every ambiguity: no owned exact merge → refuse to the audited ADR-0035
`mindspec-q9ea` human attested-restore exit BY NAME; owned candidates
with different second parents → ambiguity refusal; a contradicting
surviving tip or panel SHA → refusal; a REVERTED classification (the
same R3 discrimination, M₁-anchored) → refusal — re-attesting reverted
content would forge landed evidence. Every write carries the
`mindspec_landed_reattest_*` audit keys in the same metadata call as
the binding: actor (`user@host via argv0`), timestamp, operation,
corroborating datum, prior before-values, and the scanned branch —
inspectable via `bd show <id> --json`; detectable-by-inspection, not
cryptographically tamper-proof, exactly as the amendment claims.

`--spec-branch` is SCOPING input only (WHERE to scan), consulted ONLY
when the bead's epic linkage is cleanly, determinately ABSENT (no epic
link recorded; linkage wins whenever derivable, and a
supplied-but-ignored flag says so loudly). A lineage LOOKUP ERROR is
indeterminate ownership and fails CLOSED even with the flag
(final-review FIX-1) — the flag never substitutes for a lookup that
might have derived a different branch. It is never a corroboration
substitute, and the branch actually scanned is recorded in the audit
either way. The verb registered its `reattest` token in
core-owned `internal/redact.CommandTokens` (the standing new-top-level-
command rule — see the core domain interfaces doc).
