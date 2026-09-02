# Lifecycle Modes: The Per-Verb Agreement Table

Spec 128 (lifecycle-verb-state-honesty) audited every lifecycle verb's
success state against three independently-computed surfaces — `mindspec
state show`'s printed mode, the instruct tail's selected guidance
template, and the phase derivation itself (ADR-0023 §3) — and pinned the
result here. ADR-0023 keeps derivation **authority** (the rules that
produce a phase from epic/bead state); this guide carries the **per-verb
view**: for each verb's success on its canonical state, what mode do all
three surfaces report, and where — by explicit design, not omission — do
they differ.

Every row below is backed by a fixture test in
`internal/phase/agreement_table_test.go` that constructs the row's
epic+child state and asserts the surfaces agree (or, for the two rows
carrying a documented carve-out, asserts the carved contract instead).
Editing this table without updating that file's fixtures — or vice versa
— fails `TestAgreementTable_DocRowsCrossPinFixtures`.

## Reading the table

- **State show** is `mindspec state show`'s printed `mode` field
  (`resolve.ResolveModeWithCache`).
- **Instruct tail** is the guidance template the post-verb instruct tail
  selects (`ctx.Mode` feeding `internal/instruct`'s `<mode>.md` template
  lookup).
- **Derivation** is the status-aware phase derivation
  (`phase.DerivePhaseWithStatus`-class, `derive.go:166`/`:174`) run
  directly against the fixture's epic. Bare child-count derivation
  (`DerivePhaseFromChildren`) cannot express the terminal rows on its
  own — it returns `review` for an all-closed child set, not `done` —
  which is why every row's authoritative derivation surface is the
  status-aware one.

## The agreement table

<!-- agreement-table:rows -->

| Row | State show | Instruct tail | Derivation | Notes |
|:----|:-----------|:--------------|:-----------|:------|
| `spec create → spec` | `spec` | `spec` | *(no epic — see below)* | Pre-epic draft: spec directory + `spec/<id>` branch exist, no epic. No epic ID exists yet to feed the status-aware derivation; both the tail and the "derivation" fact here are the same no-epic `ResolveContextFromDirWithCache` `WorktreeSpec` branch ADR-0023's OQ-2 amendment names as the shipped mechanism. |
| `spec approve → plan` | `plan` | `plan` | `plan` | Epic just created, no children yet ("Epic exists, no children → plan"). |
| `plan approve → plan` | `plan` | `plan` | `plan` | **Settled fact 1.** OQ-3 option 1: `plan approve` now writes `mindspec_phase: "plan"` (not `implement`); all lifecycle children exist and are open, none claimed. TRUE three-surface agreement — the pre-128 stored-`implement`/derived-`plan` window is dissolved, not merely suppressed. |
| `next (claim) → implement` | `implement` | `implement` | `implement` | The first lifecycle claim advances stored phase to `implement` (a new writer, `cmd/mindspec/next.go`'s `advancePhaseOnClaim`); one child `in_progress`. |
| `complete → implement (mid-spec)` | `implement` | `implement` | `implement` | Some lifecycle children closed, some open, none `in_progress` — "between beads" (`derive.go`'s amended row; a `blocked` sibling counts the same way, neither closed nor in_progress). |
| `complete → plan (empty lifecycle subset)` | `plan` | `plan` | `plan` | Completing a non-lifecycle follow-up (e.g. a `bug`) on an epic with **zero** lifecycle children. The empty-lifecycle-subset fallback — never `review` (the vacuous-review guard). |
| `complete → plan (all-open, non-lifecycle child completed)` | `plan` | `plan` | `plan` | Completing a non-lifecycle follow-up while every lifecycle task remains open and none was ever claimed. Stored is already `plan` under OQ-3 option 1, so `complete`'s write here is plan-over-plan — never a backward move, needing no sanction (discharges the inherited round-6 R3 / OQ-3-interlock advisory). |
| `complete → review (last bead)` | `review` | `review` | `review` | All lifecycle children closed. |
| `release → implement (sibling closed)` | `implement` | *(none — see below)* | `implement` | **Tail-less** (R2.1: `release` emits no instruct tail — no template-selection surface exists for this row). Release's step-6.5 sync re-derives from fresh children via the sanctioned redesign leg (`phase.SanctionedDerivedSync`) and writes the child-derived value directly: a sibling bead is already closed, none `in_progress`. |
| `release → plan (no sibling closed)` | `plan` | *(none — see below)* | `plan` | Tail-less, same mechanism as above; no sibling closed (all lifecycle children open). TRUE two-surface agreement — the sanctioned sync writes exactly what it derives, no annotated stored/derived split. |
| `impl approve / impl adopt → done / idle (terminal)` | `idle` | `idle` | `done` | **Two facts at two layers**, not one row's single mode (R1.3's terminal-rows carve-out). The closed epic with its done marker derives `done` through the status-aware surface directly. Separately, `state show`/the tail report `idle` — a **global, zero-open-epic discovery** outcome (the epic is closed and therefore excluded from every active-epic query), never a per-epic derivation. |

<!-- /agreement-table:rows -->

## Carve-outs, spelled out

- **Spec-create row**: no epic exists yet, so there is nothing for the
  status-aware derivation to run against. The table still names a
  "Derivation" value for symmetry, but it is the same code path as the
  tail's, not an independent third computation — see the row's note.
- **Release rows**: `release` mutates lifecycle state but emits no
  instruct tail (R2.1) — there is no third surface to check, ever, for
  this verb. Both release rows assert two-surface agreement only.
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
  children is explicitly `blocked` (G2 strictness, by design — spec 128
  bead 2). The only way out is the backward warning's named recovery:
  `mindspec repair phase <spec-id> --force-backward "<reason>"`. This is
  not a table row because it is not a verb's *success* state — it is the
  guard's own refusal surface.
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
progression), forward-by-evidence (it can move stored phase to whatever
fresh evidence says, which is usually forward but is never a value older
than what it just observed), or the one sanctioned backward writer.

- `cmd/mindspec/next.go`'s `advancePhaseOnClaim` (the claim-time writer
  this table's `next` row exercises) is **forward-by-evidence**, not a
  second backward writer: claiming a newly-added lifecycle bead on a
  `review`-stored epic writes `implement` over `review`, which is a
  forward move driven by fresh, evidence-confirmed reality (the epic
  genuinely has new lifecycle work), not a regression.
- `phase.SanctionedDerivedSync` — release's step-6.5 sync (the two
  release rows above) — is the **only** sanctioned backward writer: it
  re-derives fresh child evidence inside its own write-lock critical
  section and writes it unconditionally, backward move permitted, with
  no force flag and no recorded reason, because its one caller
  (`release.go`) is re-deriving from evidence it just read, not asking a
  human to override a guard against possibly-stale evidence.
- `EnsurePhaseSemanticsV2`'s one-shot legacy rewrite
  (`implement → plan` under the durable v2 marker) sits **outside** this
  taxonomy entirely: it is a semantics *translation* of an unchanged
  underlying fact from pre-v2 to v2 representation, performed once per
  epic and never again — not a lifecycle backward move.

This sentence stays true given all of the above: the operator-facing
`mindspec repair phase` still refuses an unforced backward write
regardless of any of these writers, and it is the guard of last resort if
a future writer's forward/backward classification is ever wrong.

See also [merge-safety.md](merge-safety.md) for the release/merge-safety
verbs this table's `release` rows summarize the state-honesty half of.
