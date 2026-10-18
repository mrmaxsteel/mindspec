# Bead 2 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.2
- Branch: bead/mindspec-ta30.2
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.2
- Spec branch: spec/128-lifecycle-verb-state-honesty

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R3, the
  AC-5 text, settled fact 1 (the plan-approve row pins PLAN) and settled
  fact 3 (`-short` discipline).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §"OQ-3 —
  plan-approve row: option 1" (the resolution this bead implements, with
  its sanction boundary and taxonomy note) and §Bead 2 in full (its seven
  step texts are the scope authority; the load-bearing parts are quoted
  below).
- **Bead 1's MERGED code in THIS worktree** — `internal/phase/phaselock.go`,
  `internal/phase/order.go`, `internal/phase/sanctioned_sync.go`,
  `internal/phase/migrate.go`, `internal/phase/writer_inventory_test.go`.
  The merged code is the API authority; plan.md's shorthand for these
  surfaces predates bead 1's landing and has drifted (bead-1 deviation A,
  panel-approved).

## Bead 1's LANDED API (verified against the merged code in this worktree —
use these, do not use plan.md's older shorthand)
- `phase.WithPhaseWriteLock(root, epicID string, mergeMetadata
  MergeMetadataFunc, fn func(stored string, present bool) (updates
  map[string]interface{}, err error)) error` (`phaselock.go:239`). The
  `mergeMetadata` parameter takes each call site's OWN seam var. The
  freshness guarantee covers ONLY `stored` — anything else `fn` decides on
  (children, markers, metadata) must be read INSIDE `fn` with a fresh
  per-call `phase.NewCache()`. The lock is NON-REENTRANT: nested
  acquisition self-blocks until `ErrPhaseLockTimeout` (`phaselock.go:116`)
  — never call a locked helper from inside another `fn`.
- `phase.SanctionedDerivedSync(root, epicID string) error`
  (`sanctioned_sync.go:52`) — this bead wires `cmd/mindspec/release.go` to
  it AND tightens the `sanctioned_sync_test.go` call-site fence from
  zero-production-callers to EXACTLY-ONE (`cmd/mindspec/release.go`).
- `phase.EnsureMigrated(root, specID)` / `EnsureMigratedWithCache(root, c,
  specID)` (`migrate.go:44/:78`) — root-first now; children reads happen
  inside the lock with a fresh per-call Cache, and read errors PROPAGATE
  with no write. **Named hazard — do not repeat it**: the bead-1 panel
  hard-blocked TWICE on writers that discarded the error from the
  derivation they were about to write; `EnsurePhaseSemanticsV2` must carry
  the same discipline from day one (fresh in-`fn` Cache, propagate every
  read error, write nothing on failure).
- `phase.DescribeLockFailure(err, recoveryCommand string) string`
  (`phaselock.go:136`) — the lock-timeout rendering helper; every call
  site passes its own copy-pastable recovery command (the established
  per-path convention — imitate the bead-1 call sites).
- Bead 1's guard fixtures already construct epics with the
  `mindspec_phase_semantics: "v2"` marker PRESENT (forward-compat from the
  bead-1 prompt). Your `EnsurePhaseSemanticsV2` must leave those fixtures
  green unmodified — the marker makes it a no-op there by design.

## Files in scope (from plan §Bead 2; steps 1–4 quoted verbatim)
1. "`internal/approve/plan.go:455`: write `\"plan\"` (was `\"implement\"`),
   under the bead-1 lock." (The wrap already exists from bead 1 — this is
   the VALUE change only.)
2. "`cmd/mindspec/next.go`: after a successful `ClaimBead` of a LIFECYCLE
   (task-typed or untyped — the ADR-0023 discriminator) bead, advance
   stored phase to `implement` under the lock (forward write; no-op when
   already `implement`). A non-lifecycle child claim advances nothing. This
   is a NEW writer: it registers in the bead-1 inventory by construction."
   (The AC-5(vi) writer-inventory test must stay green with the new writer
   INSIDE the lock — if it goes red, you wrapped it wrong; never loosen the
   inventory test.)
3. "`cmd/mindspec/release.go`: the sanctioned redesign leg — `syncPhase`
   (`release.go:152`) becomes the ONE production caller of
   `phase.SanctionedDerivedSync` … the operator guard in `repair.go` is
   unaffected. This step also TIGHTENS bead 1's enumeration fence from
   zero-callers-with-allowlist to exactly-one."
4. "`internal/complete/complete.go:1366`: step-6.5 keeps writing derived
   `NextMode` under the lock (wrapped in bead 1; value semantics unchanged
   — in the trigger-2 state this is plan-over-plan, not backward)."
   (Expect NO edit needed here beyond verifying; say so in Deviations if
   an edit was needed.)
5. Fixtures for the two #248 routes (plan step 5): plan-approve-shaped
   (`plan approve` → state probes) and release-shaped (claim → release →
   state probes) assert stored == derived at every probe point and that NO
   stored-vs-derived warning or `repair phase` recovery line is emitted
   (`derive.go:190-193` never fires on these states).
6. AC-5(viii) fixture (plan step 6): on the released all-open state,
   release's internal sync succeeded unforced (stored back to `plan`)
   while `mindspec repair phase` on a constructed backward case still
   refuses without `--force-backward`.
7. `internal/phase/semantics_v2.go` + `semantics_v2_test.go` (plan step 7,
   the whole step is binding — key excerpts): `EnsurePhaseSemanticsV2` —
   one-shot, idempotent, lock-protected, mirroring the landed
   `EnsureMigrated` pattern (root-first signature per the landed
   convention, even though the plan's shorthand omits root). Durable
   discriminator: epic metadata marker `mindspec_phase_semantics: "v2"`.
   "Marker absent AND the legacy signature holds (stored `implement`, ≥1
   lifecycle child, all open, none closed, none in_progress …) → rewrite
   stored to `plan` + write the marker, one critical section. Marker
   absent, any other state → write the marker only, phase untouched.
   Marker present → no-op." Invocation sites: `impl.go:277`,
   `plan.go:300`, `complete.go:522` (all now root-first EnsureMigrated
   call sites — add the v2 call beside each), PLUS `next`'s claim path,
   PLUS `repair phase` and `release` (the READ-path self-heal). Exported
   lock-free predicate `phase.LegacyPhaseSignature(stored string,
   children []ChildInfo) bool` consumed by the read-side consistency
   branch (`derive.go:179-193`) and `state show`: marker absent +
   signature → emit the LEGACY NOTICE in place of the mismatch warning —
   "legacy phase semantics detected (pre-v2 stored implement); recovery:
   mindspec repair phase <spec-id>" — no read path ever takes the write
   lock.
8. `derive.go:179-193` — two changes to the warning emission: (a) the
   legacy-notice substitution above; (b) AC-5(iv)'s direction-conditional
   recovery on the RETAINED warning (forward disagreement keeps
   `recovery: mindspec repair phase <spec>`; backward disagreement emits
   the `bd show <epic-id>` inspection line + one-line explanation — never
   the backward write).

## Spec ACs this bead claims (quoted verbatim from spec.md AC-5)
- "(i) the emitted stderr does not contain `recovery: mindspec repair
  phase` for the backward disagreement (or the warning is absent, per the
  chosen mechanism);" — asserted "on BOTH R3.1 routes, plan-approve-shaped
  and release-shaped" (AC-5 lead-in); under OQ-3 option 1 the
  warning-ABSENT arm is the one that fires (stored == derived throughout).
- "(iv) any retained form of the phase-mismatch warning ends with a safe
  final recovery line — an inspection action, never the backward write
  (ADR-0035 — round-1 G6.3);" — per the plan's recorded disposition this
  is DIRECTION-CONDITIONAL (forward keeps repair-phase; backward emits the
  `bd show` inspection line); the legacy notice is a REPLACEMENT notice,
  outside this AC's target.
- "(viii) sanction-boundary leg, applicable only if the R1.3 release-sync
  redesign leg is selected (round-3 G5): on the same released all-open
  state, release's internal guard-routed backward sync succeeds unforced
  while `mindspec repair phase` on that state still refuses without
  force+reason — the test distinguishes the sanctioned internal caller
  from the operator path." — the redesign leg IS selected (plan OQ-3
  resolution), so this leg is live.
- Binding from AC-5's close: "No test executes an emitted string."

## Fixture-state pinning (plan round-3 G2 — construct states exactly so)
- Guard/warning fixtures (the AC-5(iv)/(viii) backward cases) are built
  MARKER-PRESENT, their backward states produced by post-marker means
  (seam writes or the sanctioned sync) — so EnsurePhaseSemanticsV2 no-ops
  and the condition under test survives to the guard.
- The marker-ABSENT stored-implement/all-open state is EXCLUSIVELY the
  legacy fixture (notice + self-heal).
- Six v2 fixtures per plan step 7: upgrade-before-first-claim;
  upgrade-then-first-claim; non-lifecycle claim (no advance, marker
  written); release (returns to plan, marker present, no re-reconcile);
  already-progressed epics (some-closed / in_progress / review / done —
  phase untouched, marker written); the READ-path fixture (`state show`
  emits the legacy notice, never the obsolete warning → `repair phase`
  reconciles → `state show` clean).

## Pinned decisions — do NOT relitigate
- OQ-3 option 1 is decided: plan approve writes `plan`; next's first
  LIFECYCLE claim advances stored to `implement`; release syncs
  child-derived via the sanctioned API. Settled fact 1 (plan-approve row
  = PLAN) is binding.
- The sanctioned caller is release ONLY; the operator `repair phase` keeps
  its `--force-backward` guard untouched.
- The v2 reconciliation is a semantics TRANSLATION outside the sanction
  taxonomy (plan taxonomy note) — do not route it through
  SanctionedDerivedSync; it is its own locked one-shot with the marker.
- Non-reentrancy: EnsurePhaseSemanticsV2 must not call other locked
  helpers from inside its own `fn` (self-block → timeout).

## Shared helpers to REUSE (do not reimplement)
- Everything under "Bead 1's LANDED API" above.
- Existing seams: `planMergeMetadataFn` (internal/approve/plan.go),
  `completeMergeMetadataFn` (internal/complete/complete.go),
  `repairMergeMetadataFn` (cmd/mindspec/repair.go),
  `SetMergeMetadataForTest` (internal/phase/migrate.go), release's
  `deps.syncPhase` closure (cmd/mindspec/release.go:151-153), and
  `bd`-CLI seam vars per package.
- `phase.DerivePhaseFromChildren`, `phase.FetchChildren`,
  `phase.NewCache` — the derivation surface; do not fork it.

## Tests to add
- The two #248-route fixtures (plan step 5) — AC-5(i) both routes,
  zero-warning stderr assertions.
- AC-5(viii) sanctioned-vs-operator distinction test (plan step 6).
- `internal/phase/semantics_v2_test.go` — the six v2 fixtures.
- `derive.go` warning tests — AC-5(iv) direction-conditional recovery
  (forward/backward legs) + legacy-notice substitution; keep
  `derive_recovery_test.go`'s format alignment with
  `guard.HasFinalRecoveryLine`.
- Tighten `sanctioned_sync_test.go` zero→exactly-one.
- Bead 1's fixtures (repair_test, phaselock_test, writer_inventory_test)
  must remain green UNMODIFIED except where a test's own text pins the
  old plan-approve `implement` value — those update to `plan` with a
  comment citing OQ-3 option 1 (list each in Deviations).

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above,
no reimplementing bead-1 helpers, exactly ONE commit whose message ends
with `Deviations: <list or "none">`, and tests must PASS —
`go build ./... && go test ./... -short` green in the bead worktree
(`-short` is mandatory — spec settled fact 3). Absolute paths everywhere;
scratch ONLY under
/private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/.
Work exclusively inside the bead worktree listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.2): OQ-3 option-1 phase semantics + v2 reconciliation

- plan approve writes plan; next advances stored on lifecycle claim
- release wired to SanctionedDerivedSync; fence tightened to exactly-one
- EnsurePhaseSemanticsV2 + LegacyPhaseSignature + legacy notice
- derive.go direction-conditional warning recovery
- #248 both-route fixtures + six v2 fixtures + AC-5(viii)

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.2`.
- Test counts: passed / failed / skipped for the bead's test scope
  (`internal/phase`, `internal/approve`, `cmd/mindspec`, plus the full
  `go test ./... -short` result line).
- Flagged deviations from this prompt (one line each + reason) —
  explicitly including any bead-1 test that needed its pinned value
  updated, and whether complete.go:1366 needed any edit.
