# Lifecycle Modes: The Per-Verb Agreement Table

This guide checks every lifecycle verb's
success state against three independently-computed surfaces — `mindspec
state show`'s printed mode, the instruct tail's selected guidance
template, and the phase derivation itself — and pins the
result here. The phase-derivation rules keep derivation **authority** (the rules that
produce a phase from epic/bead state); this guide carries the **per-verb
view**: for each verb's success on its canonical state, what mode do all
three surfaces report, and where — by explicit design, not omission — do
they differ.

Every row below is backed by a fixture in
`internal/phase/agreement_table_test.go` (`TestAgreementTable_Rows`) that
constructs the row's epic+child state and asserts the surfaces agree (or,
for the rows carrying a documented carve-out, asserts the carved contract
instead). The enforcement is structural, not name-only: this table's
**Row / State show / Instruct tail / Derivation** cells are parsed
verbatim and compared, cell by cell, against the Go source of truth
(`agreementTableRows` in that file) by
`TestAgreementTable_DocRowsCrossPinFixtures`; each row's fixture then
reads its expected values from that SAME record and checks them against
real production functions (`phase.DerivePhaseWithCache`,
`resolve.ResolveModeWithCache`, `instruct.Render`, and — for the seven
ordinary three-surface rows plus the forced-backward row —
`phase.DerivePhaseFromChildren` run directly on the fixture's children,
independent of whatever that epic's stored phase says; the terminal row
and the repair-phase v2 row also carry seeded stored phases but have no
such leg by design — bare `DerivePhaseFromChildren` cannot express
`done` for an all-closed set). Editing this
table's cells alone, or editing this table AND `agreementTableRows`
together to the same wrong value, both fail: each of the seven ordinary
rows' and the terminal row's canonical fixture inputs are hardcoded to
the real verb's own write semantics in the Go source (never copied from
the row record) — the forced-backward row instead seeds its stored
phase from the record but is held independent by two pre-use setup
guards (`DerivePhaseFromChildren` against independently-fixed children
must equal the record's value, and `IsBackwardMove` must prove the
premise genuinely backward) — so
changing the record does not change what the fixture actually feeds
production code, and the `DerivePhaseFromChildren` leg checks the record
against a pure function of the (also independently-fixed) children with
no stored-phase trust involved at all. A row name added to the Go list
with no matching fixture function fails too
(`TestAgreementTable_Rows`'s own lookup).

## Reading the table

- **State show** is `mindspec state show`'s printed `mode` field
  (`resolve.ResolveModeWithCache`).
- **Instruct tail** is the guidance template the post-verb instruct tail
  selects (`ctx.Mode` feeding `internal/instruct`'s `<mode>.md` template
  lookup).
- **Derivation** is the status-aware phase derivation
  (`phase.DerivePhaseWithStatus`/`DerivePhaseWithStatusWithCache` in
  `internal/phase/derive.go` — function names, since line citations here
  have already gone stale once) run
  directly against the fixture's epic. Bare child-count derivation
  (`DerivePhaseFromChildren`) cannot express the terminal rows on its
  own — it returns `review` for an all-closed child set, not `done` —
  which is why every row's authoritative derivation surface is the
  status-aware one.

## The agreement table

<!-- agreement-table:rows -->

| Row | State show | Instruct tail | Derivation | Notes |
|:----|:-----------|:--------------|:-----------|:------|
| `spec create → spec` | `spec` | `spec` | `spec` | Pre-epic draft: spec directory + `spec/<id>` branch exist, no epic. Three DISTINCT mechanisms, no double-counting: state show via `resolve.ResolveModeWithCache`'s per-spec draft check; the tail via `ResolveContextFromDirWithCache`'s path-convention `WorktreeSpec` branch; derivation via `resolve.DiscoverDraftSpecsWithCache`, the enumerate-all draft discovery (bd-health-probed, git-branch-verified, on-disk-tree-verified) — the status-aware analog for a no-epic state that `DerivePhaseWithStatus` itself cannot express. **Disclosed scope:** the tail assertion here is the `WorktreeSpec` context-path proxy only, because `internal/phase`'s test package cannot import `cmd/mindspec`'s `emitInstruct` — not because `emitInstruct` is draft-unaware. The verb's own main-checkout `emitInstruct` invocation consults the same draft-discovery surface before finalizing idle; `cmd/mindspec/verb_output_test.go`'s `TestEmitInstruct_MainCheckoutSingleDraft_RendersSpecModeNotIdle` drives that real invocation end-to-end and confirms it renders spec mode, never idle. |
| `spec approve → plan` | `plan` | `plan` | `plan` | Epic just created, no children yet ("Epic exists, no children → plan"). |
| `plan approve → plan` | `plan` | `plan` | `plan` | `plan approve` writes `mindspec_phase: "plan"` (not `implement`); all lifecycle children exist and are open, none claimed. TRUE three-surface agreement — the earlier stored-`implement`/derived-`plan` window is dissolved, not merely suppressed. |
| `next (claim) → implement` | `implement` | `implement` | `implement` | The first lifecycle claim advances stored phase to `implement` (a new writer, `cmd/mindspec/next.go`'s `advancePhaseOnClaim`); one child `in_progress`. |
| `complete → implement (mid-spec)` | `implement` | `implement` | `implement` | Some lifecycle children closed, some open, none `in_progress` — "between beads" (`derive.go`'s amended row). Fixtured with a `blocked` sibling present alongside the closed+open pair — blocked counts as neither closed nor in_progress, so the "remaining beads blocked" trigger is fixture-backed, not just code-verified. |
| `complete → plan (empty lifecycle subset)` | `plan` | `plan` | `plan` | Completing a non-lifecycle follow-up (e.g. a `bug`) on an epic with **zero** lifecycle children. The empty-lifecycle-subset fallback — never `review` (the vacuous-review guard). |
| `complete → plan (all-open, non-lifecycle child completed)` | `plan` | `plan` | `plan` | Completing a non-lifecycle follow-up while every lifecycle task remains open and none was ever claimed. Stored is already `plan`, so `complete`'s write here is plan-over-plan — never a backward move, needing no sanction. |
| `complete → review (last bead)` | `review` | `review` | `review` | All lifecycle children closed. |
| `release → implement (sibling closed)` | `implement` | *(none — see below)* | `implement` | **Tail-less** (`release` emits no instruct tail — no template-selection surface exists for this row). Release's step-6.5 sync re-derives from fresh children via the sanctioned redesign leg (`phase.SanctionedDerivedSync`) and writes the child-derived value directly: a sibling bead is already closed, none `in_progress`. |
| `release → plan (no sibling closed)` | `plan` | *(none — see below)* | `plan` | Tail-less, same mechanism as above; no sibling closed (all lifecycle children open). TRUE two-surface agreement — the sanctioned sync writes exactly what it derives, no annotated stored/derived split. |
| `repair phase → plan (v2-reconciliation, already consistent)` | `plan` | *(none — see below)* | `plan` | Tail-less (repair phase emits no instruct tail either). A legacy (marker-absent, stored `implement`, all-open) epic runs through `phase.EnsurePhaseSemanticsV2`'s one-shot rewrite first (`repair.go`'s own first step), landing stored `plan` + the v2 marker; the children (still all-open) independently derive `plan` too, so repair's own re-derivation finds `freshStored == freshDerived` — the "already consistent" success line (`Phase for <spec> already consistent: plan`). |
| `repair phase → implement (forced backward)` | `implement` | *(none — see below)* | `implement` | Tail-less. Stored `review`, one lifecycle bead reopened to `open` (an operator or out-of-band edit) — a genuine backward disagreement (`phase.IsBackwardMove("review", "implement")` is true). `mindspec repair phase <spec-id> --force-backward "<reason>"` writes the derived value plus a recorded `mindspec_phase_force_reason`, unconditionally, once forced — the ONE way an operator can move phase backward at all (see the writer taxonomy below). |
| `impl approve / impl adopt → done / idle (terminal)` | `idle` | `idle` | `done` | **Two facts at two layers**, not one row's single mode (the terminal-rows carve-out). The closed epic with its done marker derives `done` through the status-aware surface directly. Separately, `state show`/the tail report `idle` — a **global, zero-open-epic discovery** outcome (the epic is closed and therefore excluded from every active-epic query), never a per-epic derivation. |

<!-- /agreement-table:rows -->

## Carve-outs, spelled out

Spec-create is NOT a carve-out: the status-aware derivation surface
applies to every row, and this table does not
narrow that on its own authority. Its Derivation cell is asserted against
`resolve.DiscoverDraftSpecsWithCache` — the status-aware,
enumerate-all draft-discovery mechanism — invoked independently of the
tail's context-path call (separate function, separate Cache, no shared
result). The two carve-outs that ARE authorized:

- **Release rows and repair phase rows**: both verbs mutate lifecycle
  state but emit no instruct tail — there is no third surface to
  check, ever, for either verb. All four of these rows assert two-surface
  agreement only, and their Instruct-tail cell is the literal sentinel
  `*(none — see below)*`. Release's and repair's fixtures both drive the
  real production writer (`phase.SanctionedDerivedSync`,
  `phase.EnsurePhaseSemanticsV2` respectively) rather than a
  hand-authored post-state alone.
- **Terminal row**: `idle` is a fact about the *whole tracker* (no open
  epics anywhere), not about this one epic. Once an epic reaches `done`,
  it stops appearing in active-epic queries entirely, so "the mode `state
  show` reports" and "the mode this epic derives to" are answering two
  different questions that happen to both be true at once.

## Legacy shapes (behind the backward-move guard)

Two shapes fall outside the table above because they are pre-v2 or
guard-refused states, not verb success states — named here so an operator
recognizes them rather than mistaking either for a table violation:

- **Marker-absent epic with an explicitly-blocked lifecycle child.**
  A legacy (pre-`mindspec_phase_semantics: "v2"`) epic whose derivation
  would otherwise regress no longer self-heals once one of its lifecycle
  children is explicitly `blocked` (strict by design). The chain out is: the stored-vs-derived mismatch warning fires
  first, and its OWN recovery line is the inspection action `bd show
  <epic-id>` — it deliberately never names the backward write itself
  (a warning must never recommend the move it is warning
  against). Inspecting confirms the epic is genuinely stuck; from there,
  `mindspec repair phase <spec-id>` (unforced) refuses the backward move
  and its OWN refusal output is what names the escape:
  `mindspec repair phase <spec-id> --force-backward "<reason>"`. So the
  real chain is **warning → inspect (`bd show`) → repair's unforced
  refusal → `--force-backward`**, not a single step. This is not a table
  row because it is not a verb's *success* state — it is the guard's own
  refusal surface (the table's `repair phase → implement (forced
  backward)` row above picks up the story at the far end of this chain,
  once forced).
- **`plan approve`'s step 4b preserving a present-but-invalid stored
  phase.** If an epic already carries a `mindspec_phase` value that fails
  validation, `plan approve` deliberately preserves it rather than
  overwriting silently (a conservative choice — an invalid value is
  surfaced for inspection, not papered over with a fresh write). This is
  a distinct shape from every table row above, none of which start from
  an invalid stored value.

## Writer taxonomy: exactly one sanctioned backward writer

Every `mindspec_phase` writer in the codebase is either forward-by-intent
(it only ever advances stored phase, per the verb's own lifecycle
progression), evidence-fresh forward-by-intent (it re-derives fresh
evidence under lock and writes only when that fresh evidence itself calls
for forward motion — never a stale or assumed value), or the one
sanctioned backward writer.

- `cmd/mindspec/next.go`'s `advancePhaseOnClaim` (the claim-time writer
  this table's `next` row exercises) is **evidence-fresh
  forward-by-intent**: under lock, it re-derives from a fresh children
  read and writes `implement` only while that fresh derivation itself
  says `implement`. On a `review`-stored epic with a newly claimed
  lifecycle bead, it writes `implement` over `review` — which the
  shipped lifecycle-order comparator (`phase.IsBackwardMove`, `spec <
  plan < implement < review < done`) does classify as backward-in-order.
  This is deliberately **not** labeled "forward": the write moves the
  epic to an earlier position in the order, driven by fresh evidence that
  new lifecycle work genuinely exists, and it is unforced, with no
  recorded reason — the same shape `SanctionedDerivedSync` has. It is
  still not counted as a second sanctioned backward writer, because the
  taxonomy's "sanctioned backward writer" label is reserved for the ONE
  surface built and fenced explicitly for that purpose (below); this
  writer's own justification is that it is intent-scoped (it only ever
  writes `implement`, driven by one specific fresh fact) rather than a
  general re-derive-and-write-anything surface.
- `phase.SanctionedDerivedSync` — release's step-6.5 sync (the two
  release rows above) — is the **only** sanctioned backward writer with
  general scope: it re-derives fresh child evidence inside its own
  write-lock critical section and writes whatever it derives
  unconditionally, backward move permitted, with no force flag and no
  recorded reason, because its one caller (`release.go`) is re-deriving
  from evidence it just read, not asking a human to override a guard
  against possibly-stale evidence. `TestSanctionedDerivedSync_
  NoCallersOutsideAllowlist` fences this to exactly one caller
  (`release.go`) — a LEXICAL fence, proving there is exactly one direct
  call site, not that no other writer anywhere can land a
  semantically-equivalent backward-converging value.
- **Disclosed residual, not silently excepted:** `internal/complete
  /complete.go`'s `completeSyncPhase` (the step-6.5 writer behind this
  table's `complete` rows) is forward-by-intent in every state this table
  fixtures, but its own doc comment (`completeSyncPhase` in
  `internal/complete/complete.go` — cited by function name, not by line
  range) records a
  narrower reachable residue that stays open: in an
  out-of-band state not fixtured here — stored `implement` after a
  previously-surfaced release-sync failure, or a crash between a claim
  and `next`'s stored-phase advance, with every lifecycle child still
  open — its unconditional write can land `plan` over `implement`, a
  value older in lifecycle order, without routing through
  `SanctionedDerivedSync`'s fenced surface. This is accepted as-is (its
  precondition is always a state some OTHER warning already surfaced)
  and left for future work rather than fixed here. So: the "exactly
  one sanctioned backward writer" claim above is precise about
  `SanctionedDerivedSync`'s fenced, general-purpose surface — it is not a
  claim that no code path anywhere can ever produce an equivalent effect.
- `EnsurePhaseSemanticsV2`'s one-shot legacy rewrite
  (`implement → plan` under the durable v2 marker, exercised by this
  table's `repair phase → plan (v2-reconciliation, already consistent)`
  row) sits **outside** this taxonomy entirely: it is a semantics
  *translation* of an unchanged underlying fact from pre-v2 to v2
  representation, performed once per epic and never again — not a
  lifecycle backward move.

Given all of the above, `mindspec repair phase` still refuses an unforced
backward write regardless of any of these writers (the
`--force-backward` row above is the one place an operator moves phase
backward on purpose), and it is the guard of last resort if a future
writer's forward/backward classification is ever wrong.

See also [merge-safety.md](merge-safety.md) for the release/merge-safety
verbs this table's `release` rows summarize the state-honesty half of,
including its "Gate-refused `mindspec complete` converges on a plain
retry" section for the `complete` rows' gate-before-mutate reorder
and its `mindspec impl adopt` section for the
frozen-contract/externally-merged-predecessor guidance this file's
terminal row's `idle`/`done` split is the destination state of.
