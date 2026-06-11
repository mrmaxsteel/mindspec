---
approved_at: "2026-06-10T21:22:26Z"
approved_by: user
status: Approved
---
# Spec 091-ownership-discovery: Ownership discovery: empty-stub scaffolding, agent populate prompts, ZFC fallback removal

## Goal

Close the three operator-discovery gaps left by spec 086 (F2 doc-sync gate):

1. New mindspec projects start with no `OWNERSHIP.yaml` in any domain
   directory and no example to copy from.
2. Existing mindspec projects must hand-write one YAML file per domain
   after reading `mindspec doctor` warnings — there is no
   `--fix` affordance.
3. When a domain name does not match its Go package name (the spec 086
   motivating example: `context-system` ↔ `internal/contextpack`), the
   silent fallback `paths: [internal/<domain>/**]` resolves to zero
   matching files and doc-sync becomes a no-op for that domain. No
   warning fires today.

Spec 091 ships eight Goal-level changes (listed below) that
decompose into the numbered Requirements in the Requirements
section. The mapping is recorded in the Goal-to-Requirement
Index immediately after this list. The whole package closes
the gap under
**Zero Framework Cognition** discipline (the framework
mechanically writes schema-valid stubs; the agent does all the
semantic cognition of choosing paths):

1. `mindspec doctor --fix` scaffolds an EMPTY-STUB
   `OWNERSHIP.yaml` (`paths: []` with a populate-this comment).
   The framework deliberately does NOT propose a heuristic
   default like `internal/<domain>/**` — that would be
   framework-level classification and a ZFC violation.
2. `mindspec domain add <name>` scaffolds the same empty stub
   alongside the four standard docs, and prints the populate
   prompt.
3. **A new `mindspec ownership populate [<domain>]` subcommand**
   emits a templated agent prompt instructing the resident
   coding agent to inspect the repo and propose `paths:`
   entries. The framework prints the prompt; the agent does the
   cognitive work.
4. `mindspec migrate` is extended with two new phases that
   invoke the populate flows: one for ownership manifests (per
   identified domain) and one for repo-wide source globs.
5. The doc-sync validator emits an `unclaimed-source` Warn when
   a diff touches source files no domain claims (continuous
   accuracy loop — diff-time), and that Warn is actually
   VISIBLE in the flows operators run: `mindspec complete` and
   `mindspec approve impl` print doc-sync warnings
   (Requirement 22) — today they silently drop them.
6. `mindspec doctor` emits a `dead-manifest` Warn for any
   EXISTING manifest whose `paths` glob resolves to zero files
   in the workspace (continuous accuracy loop — static-time;
   newly-scaffolded empty stubs Warn on first doctor run until
   populated). A domain directory with NO manifest at all does
   NOT fire `dead-manifest` — that state is covered solely by
   the rewritten missing-OWNERSHIP Warn (Requirement 21): one
   state, one Warn, with paired remedies (missing →
   `doctor --fix`; dead → `ownership populate`).
7. **The silent `internal/<domain>/**` fallback is removed**
   from `internal/validate/ownership.go`. When a manifest is
   absent the loader returns `Ownership{Paths: []}` — the
   domain claims nothing until populated. This closes the
   ZFC violation inherited from spec 086. Breaking change to
   doc-sync semantics; HC-6 documents the migration path.
8. **The `cmd/**` + `internal/**` source-classification
   heuristic becomes an operator-overridable DISCLOSED
   default.** A new operator-declared `source_globs:` field in
   `.mindspec/config.yaml`, populated via a new
   `mindspec source populate` subcommand, FULLY OVERRIDES the
   built-in classifier when non-empty (override, never union).
   While `source_globs` is empty or absent — the universal
   default — the existing in-code classifier (`.go` files
   under `cmd/` and `internal/`, excluding `_test.go`) remains
   active as a disclosed fallback, so the gate's behavior is
   byte-identical to today (HC-7). The heuristic is no longer
   silent: the `missing-source-globs` Warn (Requirement 18),
   the scaffolded config comment (Requirement 11), and the
   complete/approve migration-status line (Requirement 22)
   all name it. Full deletion of the in-code classifier is
   deferred to a future spec.

### Goal-to-Requirement Index

| Goal | Implemented by Requirements |
|---|---|
| Goal 1 (empty-stub scaffold via doctor --fix) | 8, 15 |
| Goal 2 (empty-stub scaffold via domain add) | 9 |
| Goal 3 (`mindspec ownership populate` subcommand) | 10 |
| Goal 4 (migrate phases for both populates) | 14 |
| Goal 5 (`unclaimed-source` Warn diff-time, made visible) | 16, 22 |
| Goal 6 (`dead-manifest` Warn doctor-time) | 17 |
| Goal 7 (remove silent fallback) | 13 |
| Goal 8 (`source_globs` overrides the disclosed default) | 11, 12, 18 |
| Cross-cutting | 19 (CLI surface), 20 (hygiene Warns), 21 (missing-manifest Warn rewrite), 22 (complete/approve warning printing + migration nudge) |

The ZFC stance is the load-bearing design choice. The previous
draft of this spec shipped a heuristic default
`paths: [internal/<domain>/**]` and was wrong on its own
terms — the spec acknowledged that the literal default would
silently fail for the `context-system` ↔ `internal/contextpack`
mismatch case (the motivating example). Yegge's ZFC framing
makes the fix explicit: the framework should not be in the
classification business at all. This revision deletes the
OWNERSHIP loader's heuristic fallback, demotes the
source-classification heuristic to a disclosed,
operator-overridable default (the agent or operator can fully
replace it via `source_globs:`), and routes every NEW semantic
decision through the agent. Two legacy classification branches
survive — the built-in source classifier (fallback when
`source_globs` is empty) and the zero-domains attribution
branch (Requirement 13) — both as DISCLOSED defaults with
explicit Warn/marker text, not silent heuristics. Earlier
drafts claimed the heuristic default was deleted "everywhere";
that claim is corrected here.

## Background

Spec 086 (PR #114) shipped the doc-sync gate: source changes in a
domain must be matched by changes to the domain's docs in the same
bead/spec diff, with an explicit `--allow-doc-skew` override. The
gate consults `.mindspec/docs/domains/<domain>/OWNERSHIP.yaml` to
resolve which source paths belong to which domain. When the manifest
is absent, `internal/validate/ownership.go:48-53` falls back to
`paths: [internal/<domain>/**]`. Spec 086 Requirement 15 ("existing
repos must not start failing on day one") made this fallback
silent-by-design: `mindspec doctor` emits a `Warn` (not `Missing`,
not `Error`) per-domain when the manifest is absent
(`internal/doctor/docs.go:83-99`).

Spec 086 deliberately stopped at warning. Spec 091 is the
operator-ergonomics follow-up: it converts those warnings into a
one-command auto-fix, makes new projects start with a manifest in
place, and adds the missing diagnostic for the silent failure mode
where the fallback matches nothing.

This spec is F2's discoverability follow-up. It adds no new
gate and no new override. The OWNERSHIP.yaml manifest schema
(`paths` + `exclude`, with the `viz`/`agentmind`/`bench`
excluded-first-segment guard from spec 086) is unchanged. The
`.mindspec/config.yaml` schema gains exactly one new optional
field (`source_globs:`, documented default `[]`) per
Requirement 11. The CLI surface gains exactly two new top-level
subcommands (`mindspec ownership populate`, `mindspec source
populate`) per Requirement 19; no other new CLI surface. The
silent `internal/<domain>/**` fallback from spec 086 is removed
(Requirement 13) — that is the load-bearing semantic change,
documented in HC-6.

## Impacted Domains

- workflow: the doctor surface, domain scaffolding, doc-sync
  validation, and the complete/approve warnings pipe — the
  workflow OWNERSHIP manifest claims `internal/doctor/**`,
  `internal/validate/**`, `internal/complete/**`, and
  `internal/approve/**`, which are this spec's primary code
  surfaces. Per-surface detail in the next section.
- execution: the CLI command surface (`cmd/mindspec`, per the
  spec-092 precedent for command-layer changes) gains the two
  new populate subcommands and the two new migrate phases
  (Requirements 10, 12, 14, 19).
- core: `.mindspec/config.yaml` parsing (`internal/config`,
  claimed by the core OWNERSHIP manifest) gains the single new
  optional `source_globs:` field (Requirement 11).

## Affected surfaces (per domain)

- **workflow** (the doctor surface): `internal/doctor/docs.go`
  gains the ability to mark the per-domain OWNERSHIP check as
  `Fixable` and to write an empty-stub manifest (`paths: []` +
  populate-this comment) under `--fix`.
  `cmd/mindspec/doctor.go` is unchanged — the existing `--fix`
  plumbing dispatches to the new fixer.
- **workflow** (domain scaffolding):
  `internal/domain/scaffold.go` (`Add()`, reached via
  `mindspec domain add <name>`) is extended to write the
  empty-stub `OWNERSHIP.yaml` alongside the four standard
  files (`overview.md`, `architecture.md`, `interfaces.md`,
  `runbook.md`) and to print the populate prompt
  (Requirement 9). `cmd/mindspec/init.go` is intentionally NOT
  modified — init scaffolds no domain directories (audit
  recorded in Requirement 9).
- **workflow** (complete/approve output): `mindspec complete`
  and `mindspec approve impl` currently consume the doc-sync
  `*Result` only via `HasFailures()` and error messages
  (`internal/complete/complete.go:201-204`,
  `internal/approve/impl.go:157-158`); warning-severity issues
  are silently dropped. Requirement 22 makes both flows print
  doc-sync warnings and adds the recurring migration-status
  line for unset `source_globs`.
- **workflow** (doc-sync validation): `internal/validate/docsync.go` and
  `internal/validate/ownership.go` cooperate to emit a new Warn
  (`unclaimed-source`) whenever a diff modifies source files
  (files matching the operator-declared `source_globs:` from
  `.mindspec/config.yaml`, Requirement 11) that do not match
  any domain's resolved `paths`, **regardless of each domain's
  derived `Source()` state** (`"manifest"`, `"empty-stub"`, or
  `"missing"`, per Requirement 13). This is the diff-time half
  of the continuous accuracy loop: a still-empty scaffolded
  stub fires the Warn the first time a diff touches the files
  its domain should claim. Purely-docs diffs do not trigger
  the Warn, and the Warn is disabled while `source_globs` is
  empty (Requirement 16) — in that state the built-in
  classifier keeps driving the existing blocking lanes exactly
  as today. The Warn does NOT block the gate.
  The silent per-domain fallback in
  `internal/validate/ownership.go` is removed — a missing
  manifest claims nothing (Requirement 13).
- **workflow** (doctor diagnostics): a new static-time check emits a `dead-manifest`
  Warn for any domain whose EXISTING `OWNERSHIP.yaml` `paths`
  glob resolves to zero files in the live workspace (including
  the freshly-scaffolded empty stubs, until populated). Runs on
  every `mindspec doctor` invocation; complements the
  diff-time `unclaimed-source` Warn by catching zero-match
  manifests at-rest before any diff surfaces them. Domains
  with no manifest file are covered by the rewritten
  missing-OWNERSHIP Warn (Requirement 21), not by
  `dead-manifest`. The paths
  themselves are never auto-proposed — the agent or operator
  populates the manifest (rationale per Requirement 17).
- **ADR record** (`.mindspec/docs/adr/`, not a code domain): A
  new ADR-0035 records the Zero
  Framework Cognition stance, the empty-stub scaffold default
  (`paths: []` + populate-this comment), the removal of the
  silent per-domain fallback, the demotion of the hard-coded
  source-classification heuristic to a disclosed default that
  the operator-declared `source_globs:` config field fully
  overrides, the continuous-accuracy Warns,
  and the decision to keep every new Warn advisory (not
  promote to error in this spec). See ADR Touchpoints for the
  full contents list and the number-reservation procedure.
  ADR-0031 gains a superseded-in-part note (see ADR
  Touchpoints).

## ADR Touchpoints

- [ADR-0035-ownership-discovery.md](../../adr/ADR-0035-ownership-discovery.md)
  (**new**): Records (a) the **Zero Framework Cognition**
  stance (citing Yegge 2024) and the empty-stub scaffold
  default — `paths: []` with a populate-this comment that
  points operators at `mindspec ownership populate <domain>`;
  (b) the agent-prompt design for `mindspec ownership
  populate` and `mindspec source populate`, including the
  prompts' templated text; (c) the removal of the silent
  `internal/<domain>/**` fallback in
  `internal/validate/ownership.go:48-53` (closes the ZFC
  violation inherited from spec 086) and the demotion of
  the `cmd/**` + `internal/**` source heuristic to a
  DISCLOSED in-code default that the operator-declared
  `source_globs:` config field fully overrides when
  non-empty (full deletion of the in-code classifier is an
  explicit deferral, recorded under (h)); (d) the migration
  path on first doctor run after
  the spec lands (HC-6); (e) the continuous accuracy loop
  (diff-time `unclaimed-source` Warn — printed at
  complete/approve per Requirement 22 — + static-time
  `dead-manifest` Warn + static-time
  `missing-source-globs` Warn); (f) the hygiene-rule Warns
  (`duplicate-entry`, `redundant-subpath`,
  `domain-overlap`); (g) the no-auto-mutation policy
  (framework writes only the empty stub on creation; never
  overwrites; even `--fix --force` is read-only against
  existing manifests); (h) the deliberate deferrals
  (canonical ordering, trailing-slash style nits, case-
  sensitivity validation, full deletion of the in-code
  source classifier, resolved-file-set domain-overlap — all
  out of v1 scope); (i) the **accepted gap**: a populated
  manifest whose glob is wrong-but-RESOLVING (it matches
  real files, just not the domain's files) is caught by no
  check in this spec — `dead-manifest` needs zero matches
  and `domain-overlap` compares literal strings only.
  Verification of populate output is on the operator/agent;
  extending `domain-overlap` to resolved-file-set
  intersection is the named follow-up candidate.

  **ADR number reservation.** At spec-draft time the highest
  existing ADR is `ADR-0034-ceremony-collapse.md`, so
  `ADR-0035` is free. **Creation of the ADR file is
  deferred to the first implementation bead**: the bead
  whose responsibility it is to author the ADR re-checks
  `.mindspec/docs/adr/` at bead-claim time, takes the
  next free integer (`0035` if still free, otherwise the
  next), and updates all cross-references in this spec to
  match in the same commit. This eliminates the
  PR-open-time renumbering race: the ADR is created at
  implementation time, after sibling-spec contention is
  resolved.
- [ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md):
  Prerequisite, **amended with a superseded-in-part note**.
  ADR-0031 records the silent `internal/<domain>/**` fallback
  as live behavior (ADR-0031 lines 54, 81); Requirement 13
  removes exactly that fallback, so this spec adds a short
  superseded-in-part note to ADR-0031 pointing at ADR-0035
  for the replacement semantics. The manifest schema and the
  warning-to-error promotion that ADR-0031 records are
  unchanged; only the fallback semantics are superseded. The
  amendment is in the in-scope file list.

No other ADR is amended.

## Requirements

**Hard constraints (from the converged transformation plan):**

1. **HC-1 Solo-developer UX preserved.** No new flags on existing
   commands beyond the auto-fix path on `doctor --fix` (which is
   itself an existing flag). No new daemons, no new mandatory
   manual steps.
2. **HC-2 Standalone CLI.** No new long-lived processes, and no
   new persisted state (the Requirement 22 migration-status
   nudge is deliberately stateless for this reason).
3. **HC-3 Existing test suite preserved, modulo the enumerated
   updates that encode the removed fallback.** All existing
   tests pass on the 091 branch EXCEPT the following, which
   assert the very behavior Requirement 13 removes and are
   updated (not deleted) in the same bead that changes the
   behavior:
   - `internal/validate/ownership_test.go:76`
     (`TestOwnershipFallback`) — currently asserts the loader
     returns `[internal/<domain>/**]` for a missing manifest;
     updated to assert `Paths: []` / `Source() == "missing"`
     (this update IS the regression gate of the fallback
     removal AC).
   - `internal/validate/docsync_test.go:188`
     (`TestValidateDocsErrorsOnInternalDocSkew_Fallback`) —
     asserts a blocking error and the literal
     `<fallback: internal/<domain>/**>` marker for a
     manifest-less domain; updated to the post-Requirement-13
     semantics (a manifest-less domain claims nothing).
   The classifier tests (`docsync_test.go:10-108`,
   `TestClassifyChanges` / `TestIsSourceFile`) remain valid
   UNCHANGED: under Requirement 16's override semantics the
   built-in classifier survives as the empty-`source_globs`
   fallback path. All other existing tests pass unchanged;
   new tests are additive.
4. **HC-4 viz / agentmind / bench excluded.** Those subsystems were
   removed per specs 083/084 and remain in the excluded-first-segments
   guard inherited from spec 086.
5. **HC-5 Each commit `go build ./... && go test -short ./...` green.**
6. **HC-6 Behavioral change to spec 086 semantics — accepted
   and documented.** Two changes are intentional in this spec:
   (a) the silent fallback in
   `internal/validate/ownership.go:48-53` is removed (a
   missing manifest now returns `paths: []`, not
   `paths: [internal/<domain>/**]`); and (b) the source-
   classification heuristic in `internal/validate/docsync.go`
   becomes an operator-overridable DISCLOSED default: a
   non-empty operator-declared `source_globs:` in
   `.mindspec/config.yaml` fully replaces it; while
   `source_globs` is empty or absent the built-in classifier
   stays active and the gate behaves exactly as today. (a) is
   a ZFC correction that previous deferrals had marked as
   out-of-scope; (b) is the disclosed-default compromise —
   full deletion of the in-code classifier is deferred to a
   future spec. The migration path is:
   - On the next `mindspec doctor` run after this spec
     lands, repos with domain directories that previously
     relied on the silent fallback (i.e. domains with NO
     manifest file) fire the rewritten missing-OWNERSHIP
     Warn (Requirement 21), which names
     `mindspec doctor --fix` as the remedy. They do NOT fire
     `dead-manifest` — that Warn is reserved for existing
     manifest files (Requirement 17). Until each such
     domain's manifest is populated, the domain claims
     nothing: diffs that would have FAILED under the old
     fallback now pass. This is the accepted breaking
     change; it is surfaced DIRECTLY by the Requirement 21
     Warn at doctor time, and only INDIRECTLY (a two-step
     chain) in the complete/approve flows: the Requirement
     22(b) migration-status line names `mindspec source
     populate`; once `source_globs` is populated, the
     `unclaimed-source` Warn's domain-state report
     (Requirement 16) names any `Source() == "missing"`
     domain and the `doctor --fix` remedy. The 22(b) line
     itself does not mention missing manifests.
     After `doctor --fix` scaffolds the empty stubs,
     `dead-manifest` fires until the stubs are populated.
   - Repos that lack `source_globs:` in
     `.mindspec/config.yaml` (including repos with no
     config.yaml at all) lose NOTHING at upgrade time:
     source classification is byte-identical to today
     (HC-7). On the next `mindspec doctor` run they fire the
     new `missing-source-globs` Warn (Requirement 18), which
     discloses the active built-in default; every
     `mindspec complete` / `mindspec approve impl` run
     prints the recurring migration-status line
     (Requirement 22).
   - Operator runs `mindspec doctor --fix` (scaffolds
     empty stubs where missing, scaffolds the `source_globs`
     config block where absent), then runs
     `mindspec ownership populate` per domain and
     `mindspec source populate` once, with the resident
     agent populating both.
   The mindspec repo itself ships with populated manifests
   already (four hand-authored), so its own doctor run does
   not break on this transition. Other repos that previously
   depended on the silent fallback will see Warns on first
   doctor run after upgrading — by design, since the silent
   fallback was the violation this spec is meant to close.
   Existing hand-authored manifests are never overwritten.

   **Coverage responsibility shifts with `source_globs`.** A
   repo that populates `source_globs` takes responsibility
   for classification coverage: the declared globs fully
   replace the built-in default, so a too-narrow list narrows
   what the gate sees as source. The populate prompt
   (Requirement 12) says this explicitly.

   **Accepted gap — wrong-but-resolving globs.** A populated
   manifest whose `paths` glob is wrong but still resolves to
   real files is caught by no check in this spec:
   `dead-manifest` (Requirement 17) requires zero matches,
   and `domain-overlap` (Requirement 20) compares literal
   path strings only. The misclaim surfaces only indirectly,
   as `unclaimed-source` Warns on the files the domain
   SHOULD have claimed. Verification of populate output is
   therefore on the operator/agent; extending
   `domain-overlap` to resolved-file-set intersection is
   recorded in ADR-0035 item (i) as the follow-up candidate.

   **Accepted gap — partial-dead manifests.** A sibling
   gap: `dead-manifest` (Requirement 17) evaluates the
   manifest's `paths` glob SET as a whole, so a manifest
   with one dead glob among live ones (e.g.
   `paths: [internal/real/**, internal/deleted/**]`) fires
   nothing — the set still resolves to files. Per-entry
   evaluation is a plan-phase candidate alongside the
   resolved-file-set follow-up; until then this state, like
   wrong-but-resolving, surfaces only indirectly via
   `unclaimed-source`.

   **Agent-failure recovery.** If the agent that reads a
   `mindspec ownership populate` or `mindspec source populate`
   prompt declines or fails to populate the manifest, the
   operator's recovery paths are: (a) re-invoke the populate
   command and route the prompt to a different agent;
   (b) edit the manifest by hand using the schema documented
   in spec 086 / ADR-0031; (c) accept the empty stub and
   tolerate `dead-manifest` + `unclaimed-source` Warns until
   populated. None of these paths require mindspec changes;
   they are documented here so the operator knows the
   failure modes are recoverable.

   **No deprecation window.** Per the panel's D1 resolution
   (D1 asked whether the fallback removal needs a deprecation
   period; the panel resolved NO): mindspec is primarily a
   single-operator tool at this
   stage, and the cost of a deprecation period exceeds the
   cost of the operator running `mindspec doctor --fix`
   once on upgrade. If the user-base grows enough that this
   stops being true, a future spec can re-introduce a
   deprecation window for that specific transition.

7. **HC-7 Doc-sync's pass/fail gate is unchanged for repos
   that have not opted in.** Two halves:
   - **Source classification:** while `source_globs` is empty
     or absent — the universal state at upgrade time, since
     nothing scaffolds it before `doctor --fix` runs — the
     classifier code path is the SAME `isSourceFile` function
     as today (`internal/validate/docsync.go:108-112`), so
     classification is byte-identical, the blocking lanes
     fire exactly as before, and the gate is never inert.
     Only an operator who populates `source_globs` changes
     classification — by explicit action, with full-override
     semantics (Requirement 16).
   - **Pass/fail rule:** the new Warns (`unclaimed-source`,
     `dead-manifest`, `missing-source-globs`, plus the three
     hygiene Warns) are advisory only — none promotes to
     error in this spec. The Requirement 13 fallback removal
     changes the gate's INPUTS (domains with no manifest now
     claim nothing — the accepted breaking change documented
     in HC-6), but the pass/fail rule against those inputs is
     unchanged.

   *Note on the old HC-6 numbering:* prior drafts had a
   single "no breaking change" HC-6; this spec replaces it
   with HC-6 (acknowledge the breaking change + document
   migration) and HC-7 (preserve the doc-sync gate's pass/
   fail rule). HC-1..HC-5 are unchanged.

**Spec-specific requirements:**

8. **`mindspec doctor --fix` writes an empty stub `OWNERSHIP.yaml`**
   for every domain directory under
   `.mindspec/docs/domains/<domain>/` that lacks one. The stub
   content is:

   ```yaml
   # Auto-generated by <command> on <RFC3339 timestamp>.
   # mindspec deliberately does NOT guess which paths belong to
   # the "<domain>" domain — that is a semantic decision a coding
   # agent or human operator makes by inspecting the repo. See
   # spec 086 / ADR-0031 for the schema and ADR-0035 for the
   # Zero-Framework-Cognition rationale.
   #
   # To populate this manifest:
   #   • Run `mindspec ownership populate <domain>` to emit an
   #     agent prompt that proposes paths based on the actual
   #     repo layout; or
   #   • Edit the `paths:` list below by hand to list the source
   #     globs that this domain owns (e.g. `internal/ledger/**`
   #     for a domain named "payments").
   #
   # Until populated, `mindspec doctor` will Warn `dead-manifest`
   # and — once `source_globs:` is populated in
   # .mindspec/config.yaml (run `mindspec source populate`) —
   # the doc-sync gate will Warn `unclaimed-source` on any
   # diff that touches source files this domain should claim.
   paths: []
   ```

   The body (`paths: []`) is identical across all scaffolding
   sources. The leading comment's `<command>` token is the
   only thing that varies; both variants come from a
   single comment-generation helper with one substitution
   parameter. Substitution table:

   | Scaffolding source | `<command>` substituted as |
   |---|---|
   | `mindspec doctor --fix` writes a stub | `mindspec doctor --fix` |
   | `mindspec domain add <name>` writes a stub for the new domain | `mindspec domain add <name>` |

   The `mindspec migrate` flow needs no third variant: migrate
   instructs the agent to invoke `mindspec domain add` per
   identified domain, so its stubs are written (and labeled)
   by the `domain add` row above. `mindspec ownership
   populate` is a prompt emitter that never writes files
   (Requirement 19), so it never generates this comment.

   The plan-phase bead implementing scaffolding produces ONE
   templating helper (e.g. `func renderStub(generatedBy string) []byte`)
   and two call sites that pass the appropriate string.
   No copy-paste of the comment body across files.

   **Design rationale (ZFC).** The framework deliberately
   declines to guess paths. Heuristic defaults like
   `internal/<domain>/**` are framework-level classification
   that should live in the agent prompt under Zero Framework
   Cognition (Yegge, "Zero Framework Cognition", 2024 — agents
   classify, frameworks orchestrate). An empty stub is
   schema-valid, makes the populate-this gap visible (via
   `dead-manifest` and `unclaimed-source` Warns), and forces
   the cognitive work — "which paths actually belong to this
   domain in this repo" — to happen in the agent or the
   operator, not in mindspec. The previous draft of this spec
   shipped a heuristic default for the same reason and was
   wrong; this revision corrects it.

   The fixer is idempotent: if `OWNERSHIP.yaml` exists for a
   domain (regardless of content, including a hand-populated
   non-empty `paths:`), it is not touched. If `--fix --force`
   is passed, the file is STILL not overwritten — by design.
   `OWNERSHIP.yaml` content is a semantic decision (which
   paths a domain owns) that belongs to the operator or
   agent; overwriting it would violate the ZFC principle and
   silently erase the cognitive work that justifies the
   manifest's existence. To replace a manifest, delete it
   manually first, then re-run `--fix` to scaffold a fresh
   stub. This is the only `--fix --force` carve-out in
   mindspec; it is intentional and documented here so a
   future maintainer does not "fix" the inconsistency.

9. **`mindspec domain add <name>` scaffolds an empty-stub
   `OWNERSHIP.yaml`** alongside the four standard docs, and
   prints the populate prompt to the operator (see
   Requirement 10 for the prompt shape). The hook is
   `internal/domain/scaffold.go:Add()` — currently writes
   `overview.md`, `architecture.md`, `interfaces.md`,
   `runbook.md` (templates at lines 138-206) and appends a
   context-map entry; spec 091 extends `Add()` to also write
   `OWNERSHIP.yaml` with the Requirement 8 stub after the
   four standard files. No new flag, no opt-out — every
   `mindspec domain add` invocation produces the stub plus
   prints the populate prompt. The resident coding agent
   reads the prompt and populates the manifest in-session
   (this is the ZFC-compliant path: framework writes the
   stub mechanically, agent does the cognitive work of
   choosing paths).

   `mindspec init` is intentionally NOT modified. Audit at
   spec-draft time confirmed:
   (a) `internal/bootstrap/bootstrap.go:215` creates the empty
   parent `.mindspec/docs/domains/` directory but no domain
   subdirectories;
   (b) `internal/bootstrap/bootstrap_test.go:166` —
   `TestRun_NoDomainScaffolding` — explicitly enforces this
   empty-on-init invariant ("no default domains are scaffolded");
   (c) `cmd/mindspec/init.go` Long description says *"Use
   `mindspec migrate` to onboard an existing brownfield
   repository"*, making the greenfield/brownfield split explicit.
   The reasoning: a brand-new project has no business presuming
   what domains it will need; defaulting to `auth` / `api` /
   `worker` would be presumptuous. Domain directories are
   created exclusively via `domain add`, so the single
   `Add()` hook is sufficient to cover both flows:
   - **Greenfield**: user runs `mindspec domain add <name>` as
     each domain crystallises → manifest scaffolded.
   - **Brownfield**: `mindspec migrate` emits a multi-phase
     prompt whose Phase 2 "Domain Identification"
     (`cmd/mindspec/migrate.go:186-199`) carries the
     instruction to call `mindspec domain add` per identified
     domain → manifest scaffolded transitively.
   - **Hand-created directories** (any path that bypasses
     `domain add`): caught by `mindspec doctor --fix`
     (Requirement 8).

10. **A new `mindspec ownership populate <domain>` subcommand
   emits an agent prompt** instructing the resident coding
   agent to inspect the repo and propose `paths:` entries
   for the named domain's `OWNERSHIP.yaml`. The framework's
   role here is purely orchestration: it prints a templated
   prompt; the framework itself does not propose paths.
   The prompt shape:

   ```
   Populate .mindspec/docs/domains/<domain>/OWNERSHIP.yaml
   for the "<domain>" domain.

   Read `.mindspec/docs/domains/<domain>/overview.md` and
   `architecture.md` to understand what this domain owns.
   Then inspect THIS repo's actual layout — `ls`, `find`,
   `go list ./...`, or whatever discovery commands fit your
   tools — and identify the source globs that implement the
   behaviour described in those docs.

   The framework deliberately provides no pattern hints. The
   domain name is a semantic label; the source paths are an
   empirical question about this specific repo. Do not assume
   the domain name matches any directory name (e.g. a domain
   named "payments" may correspond to `internal/ledger/`,
   or to something else entirely — only the repo can tell you).

   Manifest schema: a `paths:` list of globs, plus an
   optional `exclude:` list of globs subtracted from
   `paths` (see spec 086 / ADR-0031). Entries whose first
   path segment is `viz`, `agentmind`, or `bench` are a
   HARD ERROR — those subsystems are out of doc-sync scope;
   never claim paths under them.

   When done, edit the manifest's `paths:` list. Verify each
   path resolves to at least one file (`mindspec doctor`
   will Warn `dead-manifest` if it does not). Run
   `mindspec doctor` to confirm no `dead-manifest` /
   `redundant-subpath` / `duplicate-entry` / `domain-overlap`
   Warns remain. (`unclaimed-source` is a diff-time Warn
   surfaced by `mindspec complete` / `mindspec approve impl`,
   not by doctor.)
   ```

   `mindspec ownership populate` (no domain arg) emits one
   prompt per domain whose manifest is missing or empty
   (`paths: []`), so the agent can populate all of them in
   one pass. With an EXPLICIT `<domain>` argument the prompt
   is emitted regardless of the manifest's populated state —
   re-emitting for an already-populated manifest is fine
   (the agent edits the existing `paths:` list, e.g. to
   widen it). This explicit-arg behavior is what the
   Requirement 16 all-populated hint ("`mindspec ownership
   populate <domain>` works for populated domains when named
   explicitly") relies on.

11. **A new `source_globs:` field is added to
    `.mindspec/config.yaml`** declaring which path globs in
    the repo count as "source" for doc-sync purposes, with
    FULL-OVERRIDE semantics (Requirement 16): a non-empty
    list completely replaces the built-in classifier; an
    empty or absent list leaves the built-in classifier
    active as the disclosed default. This
    exact block — comment plus `source_globs: []` — is the
    LITERAL scaffolded default; the framework never scaffolds
    any glob values:

    ```yaml
    # source_globs: which path globs count as "source" for
    # the doc-sync gate. OVERRIDE semantics: a non-empty
    # list FULLY REPLACES mindspec's built-in default
    # (never a union with it). While this list is empty,
    # mindspec falls back to its built-in classifier:
    # files under cmd/ or internal/ ending in .go,
    # excluding _test.go. The framework does NOT guess
    # repo-specific globs — operator/agent declares them
    # (run `mindspec source populate` for an agent prompt).
    # While empty, the unclaimed-source Warn is disabled
    # (doctor fires missing-source-globs as a nudge).
    source_globs: []
    ```

    Example of an ALREADY-POPULATED config — what the field
    looks like after the agent or operator fills it in (these
    values are illustrative output of agent cognition, never
    framework-scaffolded content):

    ```yaml
    source_globs:
      - cmd/**
      - internal/**
    ```

    **Scaffolding ownership.** `mindspec init` does NOT
    create `.mindspec/config.yaml`
    (`internal/bootstrap/bootstrap.go:211-227` — the
    bootstrap manifest has no config.yaml entry) and is NOT
    modified by this spec; `internal/bootstrap/` is out of
    scope. The SOLE scaffolder of this block is
    `mindspec doctor --fix`, with defined behavior for all
    three config states:
    - **config.yaml absent** (the common brownfield state):
      `--fix` creates `.mindspec/config.yaml` containing
      exactly the comment + `source_globs: []` block above.
    - **config.yaml present, no `source_globs:` field:**
      `--fix` APPENDS the block at the end of the file —
      append-only; the fixer never reorders, reformats, or
      rewrites operator-authored content.
    - **config.yaml present with `source_globs:`** (empty or
      populated): `--fix` leaves the file untouched.

    **Raw-YAML state detection.** The fixer's
    append-vs-leave decision (field absent vs present)
    MUST inspect the raw file bytes (key-presence check or
    `yaml.Node`): `config.Load` unmarshals into a typed
    struct, so an absent `source_globs:` key and
    `source_globs: []` both parse to an empty slice and are
    indistinguishable through the typed loader. The Warn
    side (Requirement 18) is unaffected — it deliberately
    collapses both states to `len(globs) == 0`, which
    `config.Load` delivers.

    **Config cache.** `config.Load` caches parsed config
    per-process per absolute root
    (`internal/config/config.go:76-110`). The fixer (and any
    fix-then-recheck flow in the same process) must call
    `config.ResetCache()` after writing; tests that mutate
    config.yaml on disk must do the same.

    Under Requirement 16's override semantics the framework
    still ships an in-code classifier as the empty-list
    fallback — disclosed in this comment block, in the
    `missing-source-globs` Warn (Requirement 18), and in the
    migration-status line (Requirement 22). What this field
    removes is the operator's inability to override it: the
    framework no longer has the last word on what counts as
    source.

12. **A new `mindspec source populate` subcommand emits an
    agent prompt** instructing the resident coding agent to
    inspect the repo and propose `source_globs:` entries.
    Same shape as `mindspec ownership populate`. The prompt
    text:

    ```
    Populate the `source_globs:` field in
    .mindspec/config.yaml.

    Inspect THIS repo's directory layout — `ls -R`, `find`,
    `go list ./...`, or whatever discovery commands fit your
    tools — and identify the path globs that match all
    hand-authored source code (any language), excluding
    documentation, generated artifacts, vendored
    dependencies, and test fixtures.

    The framework deliberately provides no pattern hints —
    that classification depends on THIS repo's layout and
    conventions, not a template. Reach your own
    determination by reading the tree.

    IMPORTANT: a non-empty `source_globs:` list FULLY
    REPLACES mindspec's built-in default classifier (it is
    never merged with it). Your list must therefore cover
    EVERYTHING the doc-sync gate should treat as source —
    a too-narrow list narrows the gate.

    The resulting `source_globs:` determines which file
    changes the doc-sync gate considers "source": files
    that match `source_globs` but do not match any domain's
    OWNERSHIP.yaml `paths` fire the `unclaimed-source` Warn
    (Requirement 16).

    When done, edit .mindspec/config.yaml's `source_globs:`
    list. Run `mindspec doctor` to confirm
    `missing-source-globs` no longer Warns.
    ```

    Unlike `ownership populate` (per-domain), `source
    populate` is repo-wide and takes no domain argument.

13. **The silent `internal/<domain>/**` fallback is removed
    from `internal/validate/ownership.go`** (the loader at
    lines 48-53 currently returns `Paths:
    ["internal/<domain>/**"]` when a manifest file is
    absent). Replaced with `Paths: []`. Closes the inherited
    ZFC violation from spec 086.

    **Ownership.Source — derived, not stored.** Callers
    (especially doc-sync's Warn-message construction in
    Requirement 16) need to distinguish three states. Per the
    panel's D2 resolution (D2 asked whether the three-state
    distinction should be a new stored struct field or
    derived; the panel resolved: derived), `Source` is a
    derived method on
    `Ownership`, computed from existing fields — not a new
    stored field that requires loader path-specific assignment.
    Concrete semantics:

    | Condition (post-load) | `ManifestPath` | `Paths` | `Source()` returns |
    |---|---|---|---|
    | `OWNERSHIP.yaml` absent on disk | `""` | `[]` | `"missing"` |
    | File exists, `paths: []` (empty stub) | `<abs path>` | `[]` | `"empty-stub"` |
    | File exists, `paths: [...]` non-empty | `<abs path>` | `[...]` | `"manifest"` |

    The derived form means there is exactly one decision
    point (the loader's existing path-set-empty check)
    rather than three; correctness is structural. Doc-sync
    uses `Source()` only for diagnostic Warn text; the
    gate's pass/fail rule does not branch on it.

    **Surviving attribution fallbacks — disclosed, not
    silent.** Removing the loader fallback does NOT delete
    every legacy classification branch in
    `internal/validate/docsync.go`; the survivors are
    retained as DISCLOSED defaults with explicit obligations:
    - **The zero-domains legacy branch**
      (`checkInternalPackages`, docsync.go:~163-204): when NO
      domain directories exist at all, changed
      `internal/<pkg>/` files are still attributed
      per-package and emit blocking `internal-docs` errors
      labeled `<fallback: internal/<pkg>/**>`. This branch
      SURVIVES — it is the only drift coverage for bare
      checkouts with no domain docs. Obligations: the error
      text keeps the explicit fallback marker (it is the
      disclosure); the code comment is updated to state this
      is the deliberate no-domains default, not a leftover;
      and a test pinning the zero-domains branch is ADDED —
      NO existing test reaches `len(domains)==0` (both
      `TestCheckInternalPackages_*` tests write ownership
      fixtures, so they exercise the manifest path), so the
      disclosure obligation includes creating that coverage.
    - **The per-domain `<fallback: internal/<domain>/**>`
      marker** (docsync.go:~250-260, emitted when an
      attribution carries an empty `ManifestPath`): after
      this requirement a missing manifest claims nothing, so
      an attribution with empty `ManifestPath` should no
      longer occur. The implementation bead MUST audit
      reachability: if the marker path is still reachable,
      its text is relabeled to disclose the actual source of
      the claim (a "disclosed-fallback" marker); if it is
      dead code, it is removed. Either way a test pins the
      chosen outcome. Tracked in Open Questions.
    - **Stale comments** that describe the removed fallback
      as live — `internal/validate/ownership.go:21-25`
      (struct doc: "an empty ManifestPath signals the
      fallback"), `internal/validate/docsync.go:114-118`
      (`listDomainDirs` comment), and
      `internal/doctor/docs.go:84-86` (Warn rationale
      comment) — are updated in the same bead.

14. **`mindspec migrate` instructs the agent to populate
    both OWNERSHIP.yaml and source_globs** via two new phases
    inserted into the existing migrate prompt.

    **Audited anchors (corrected).** Phase 2 "Domain
    Identification" (`cmd/mindspec/migrate.go:186-199`) is
    the phase that CARRIES the `mindspec domain add <slug>`
    instructions. Phase 4 "Domain Doc Population"
    (migrate.go:222-232) fills the already-scaffolded doc
    files and contains no `domain add` invocations. (An
    earlier draft mis-attributed the `domain add`
    instructions to Phase 4; corrected here against the
    code.)

    The two new phases, with the ONLY two binding ordering
    constraints:
    - **Source-Globs Population** (new): instructs the agent
      to populate `.mindspec/config.yaml`'s `source_globs:`
      using `mindspec source populate` (one invocation,
      repo-wide). Binding constraint: this phase appears
      AFTER the Domain Identification phase (it needs the
      repo-layout understanding Phases 1-2 build).
    - **Ownership-Manifest Population** (new): instructs the
      agent to run `mindspec ownership populate <domain>` per
      identified domain. Binding constraint: this phase
      appears AFTER the phase carrying the `mindspec domain
      add` instructions (Phase 2 per the audit above) — the
      manifests it populates are the empty stubs that
      `domain add` scaffolds, so populate cannot precede the
      adds.

    **Exact insertion sites and final phase numbers are
    deferred to the plan phase.** Spec-draft time confirmed
    the two anchors above but did NOT audit
    `buildMigratePrompt`'s full internal structure (whether
    the prompt permits mid-string insertion, what Phase 3
    contains, etc.). The first bead generated by `mindspec
    plan approve` for this spec MUST, as its opening step,
    read `buildMigratePrompt` in full and produce an audit
    naming the exact insertion sites (and final phase
    numbers) for both new phases, honouring the two binding
    ordering constraints above. This is explicitly in-scope
    for the implementation bead, not for this spec to
    predetermine; tracked in Open Questions.

15. **`internal/doctor/docs.go` marks the per-domain OWNERSHIP
    check as fixable** so `--fix` dispatches to a fixer that
    writes the Requirement 8 empty stub AND prints the
    populate prompt for each scaffolded domain. The existing
    fixable-check pattern is `Check.FixFunc` (used by
    `internal/doctor/beads.go` to create `.beads/config.yaml`
    when absent and by `internal/doctor/git.go` to gitignore
    tracked runtime files); spec 091 attaches a `FixFunc` to
    the OWNERSHIP check that writes the stub. No new dispatch
    mechanism is introduced. The printed prompt routes the
    cognitive work to the agent; `doctor --fix` itself never
    proposes paths.

16. **The doc-sync validator emits a new Warn
    `unclaimed-source`** whenever a diff modifies one or more
    files matching a NON-EMPTY `source_globs:` (from
    `.mindspec/config.yaml`, per Requirement 11) that do not
    match any domain's resolved `paths` glob set.

    **Override semantics (binding).** Source classification
    consults `source_globs` with FULL-OVERRIDE, never-union
    semantics:
    - When `source_globs` is NON-EMPTY, it is the ONLY
      classifier: a file matching the globs is source even
      if the built-in classifier would reject it (e.g. a
      `.ts` file, or a file outside `cmd/`/`internal/`); a
      file the built-in classifier would accept (e.g.
      `internal/foo/bar.go`) is NOT source unless a glob
      matches it. The built-in classifier is fully bypassed.
    - When `source_globs` is EMPTY or ABSENT, the built-in
      classifier (`isSourceFile`,
      `internal/validate/docsync.go:108-112` — `.go` files
      under `cmd/` or `internal/`, excluding `_test.go`)
      remains the active DISCLOSED fallback: it drives the
      existing blocking lanes byte-identically to today
      (HC-7), and the `unclaimed-source` Warn is disabled
      (the operator has not declared what "source" means;
      the nudges are doctor's `missing-source-globs` Warn,
      Requirement 18, and the complete/approve
      migration-status line, Requirement 22).

    **Populated globs + zero domain DIRECTORIES — a
    specified double-report, not a bug.** When `source_globs`
    is non-empty and NO domain directories exist at all,
    BOTH the surviving zero-domains legacy branch
    (Requirement 13 — blocking `internal-docs` errors with
    the `<fallback: internal/<pkg>/**>` marker, which
    governs pass/fail) AND the advisory `unclaimed-source`
    Warn (every glob-matched file is trivially unclaimed at
    zero domains) fire on the same files; the
    all-`"manifest"` hint variant below is vacuously
    triggered, and its hint set already includes
    `mindspec domain add` — the correct remedy — so the
    path degrades gracefully. The implementing bead must
    NOT "fix" this by suppressing either side.

    **Warn message — mechanical state report, no
    candidate-guessing.** The message lists (a) each
    unclaimed source file; (b) a mechanical report of every
    domain annotated with its derived `Ownership.Source()`
    state (`"manifest"` / `"empty-stub"` / `"missing"`, per
    Requirement 13) — the framework reports state and does
    NOT rank or guess which domain should claim the file
    (that is agent/operator cognition, per ZFC); and (c) the
    hint `run 'mindspec doctor --fix' to scaffold missing
    manifests, then 'mindspec ownership populate <domain>'
    to populate one`. When EVERY domain's `Source()` is
    `"manifest"` (no unpopulated candidates exist), the
    message says so explicitly and the hint changes to: the
    unclaimed file may belong to a domain whose populated
    manifest needs widening (`mindspec ownership populate
    <domain>` works for populated domains when named
    explicitly) or to a domain that does not exist yet
    (`mindspec domain add <name>`) — pointing at commands
    that do nothing is worse than no hint.
    The Warn is attached to the doc-sync `*Result` alongside
    any errors; it does NOT block the gate. Purely-docs diffs
    (no file matches `source_globs:`) do not trigger the Warn.

    **Drift surfaces per-invocation.** The Warn is
    re-evaluated on every `ValidateDocs` call site — every
    `mindspec complete`, every `mindspec approve impl`, and
    `mindspec validate docs`
    (`cmd/mindspec/validate.go:70`) — like every
    other Warn this validator produces, and is PRINTED in
    the complete/approve flows per Requirement 22. There is
    no additional caching or persistence layer; an
    OWNERSHIP.yaml whose `paths` glob stops matching the
    real layout (e.g. after a package rename) re-surfaces
    via this Warn on the first subsequent diff that touches
    the now-orphaned file. Pair this with the static
    per-doctor-run `dead-manifest` Warn (Requirement 17)
    for at-rest coverage.

17. **`mindspec doctor` emits a new Warn `dead-manifest`** per
    domain whose EXISTING `OWNERSHIP.yaml` has a `paths` glob
    set that resolves to zero
    files in the workspace (including the freshly-scaffolded
    `paths: []` stub — empty paths trivially resolves to zero
    files, so every newly-scaffolded manifest fires this Warn
    until populated). The check runs on every `doctor`
    invocation, has no dependency on a diff, and fires whether
    the manifest is an empty stub, was hand-authored against a
    now-deleted package, or has drifted out of sync with the
    live tree for any other reason — PROVIDED the manifest
    file exists. A domain directory with NO `OWNERSHIP.yaml`
    (`Source() == "missing"`) does NOT fire `dead-manifest`;
    that state is covered solely by the rewritten
    missing-OWNERSHIP Warn (Requirement 21). One state, one
    Warn, with paired remedies: missing →
    `mindspec doctor --fix` (scaffolds the stub); dead →
    `mindspec ownership populate <domain>` (fills it in).
    The Warn names the
    manifest path, the suspect glob pattern (or `(empty)` for
    a stub), and the hint `run 'mindspec ownership populate
    <domain>' to emit an agent prompt`. Static-time complement
    to Requirement 16: catches the unpopulated-stub case
    at-rest, before any diff surfaces it.

    **Workspace-walk exclusions (binding).** The
    "resolves to zero files" check walks the workspace tree
    using the shared `globMatch`; the walk MUST skip
    `.git/`, `.worktrees/`, and `.beads/` — a stray match
    inside those trees (e.g. `internal/foo/**` matching
    `.worktrees/<wt>/internal/foo/bar.go`) would mask a
    genuinely dead manifest. A test pins the exclusion set
    (a manifest whose glob matches ONLY a file under an
    excluded tree still fires `dead-manifest`).

    `--fix` for the missing-manifest state writes the empty
    stub via the
    Requirement 8 path AND prints the populate prompt; it
    does NOT propose paths itself (that would be ZFC violation
    — Yegge: "Heuristic Classification is forbidden"). The
    agent reading the printed prompt does the cognitive work.
    Note the scope of `dead-manifest` (existing files only,
    whole-set evaluation) means neither a wrong-but-RESOLVING
    glob nor a PARTIALLY-dead manifest (one dead glob among
    live ones) fires it — both accepted gaps are documented
    in HC-6 and ADR-0035 item (i).

18. **`mindspec doctor` emits a new Warn
    `missing-source-globs`** when ANY of the following holds:
    `.mindspec/config.yaml` does not exist (the common
    brownfield state — `mindspec init` does not create the
    file), the file exists but lacks the `source_globs:`
    field, or the field is present with an empty list. The
    Warn names the expected config file path
    (`.mindspec/config.yaml`, even when the file is absent),
    DISCLOSES the active built-in default — e.g. `doc-sync is
    classifying source with the built-in default: .go files
    under cmd/ and internal/, excluding _test.go` — and gives
    the hint `run 'mindspec source populate' to emit an agent
    prompt`. Fires on every doctor invocation; clears once
    `source_globs:` has at least one non-empty entry. The
    `unclaimed-source` Warn (Requirement 16) is disabled
    while this Warn fires (`unclaimed-source` cannot
    meaningfully fire without operator-declared
    `source_globs`; the built-in fallback meanwhile keeps the
    blocking lanes running exactly as today, so nothing is
    silently lost in this state).

19. **Two new top-level CLI subcommands are introduced**:
    `mindspec ownership populate [<domain>]` per Requirement 10
    and `mindspec source populate` per Requirement 12. Both
    are ZFC-compliant prompt emitters — the framework prints
    the prompt; the agent does the cognitive work. No other
    new top-level subcommands; all other new behavior lands
    behind existing surfaces (`doctor --fix`, `domain add`,
    `migrate`, and the complete/approve warning printing of
    Requirement 22).

20. **`mindspec doctor` emits hygiene Warns for malformed
    `OWNERSHIP.yaml` content**, surfacing problems the schema
    inherited from spec 086 / ADR-0031 does not catch. Three
    Warns, all static, all advisory (never block any gate),
    all hand-edit-only (no auto-fix in this spec):
    - **`duplicate-entry`**: the same literal path appears
      more than once in `paths` or more than once in `exclude`
      for a single domain.
      Example: `paths: [internal/foo/**, internal/foo/**]`.
    - **`redundant-subpath`**: a `paths` entry is a strict
      glob-subpath of another `paths` entry in the same
      domain (i.e. the narrower entry is fully implied by
      the wider one).
      Example: `paths: [internal/foo/**, internal/foo/bar/**]`
      — the second entry is implied by the first; either it is
      noise or the first entry is wrong. The Warn names both
      entries so the operator can decide which to remove. The
      check uses prefix matching on the literal path string
      after stripping trailing `**`; pathological cases
      (e.g. `internal/{foo,bar}/**` brace expansion if any
      future schema extension allows it) are out of scope.
    - **`domain-overlap`**: the same literal path appears in
      `paths` across two or more domains' manifests (e.g.
      domain A and domain B both claim `internal/shared/**`).
      The Warn names every domain that claims the path so the
      operator can decide which domain owns it (or whether
      the overlap is intentional and the path needs to be
      split). Literal-string comparison only: two different
      glob strings that resolve to overlapping file sets are
      NOT flagged (the wrong-but-resolving accepted gap,
      HC-6 / ADR-0035 item (i); resolved-file-set
      intersection is the named follow-up).

    These checks run on every `mindspec doctor` invocation,
    after the existing `dead-manifest` check from Requirement
    17. They complement (not replace) the schema validation
    in `internal/validate/ownership.go` (the excluded-first-
    segment guard from spec 086 stays as a hard error; the
    new hygiene checks are advisory Warns).

    **Out of scope for v1 of this spec** (deliberate
    deferrals; recorded in ADR-0035): canonical ordering of
    entries (alphabetical sort) is NOT enforced — manifests
    work in any order; auto-normalization (rewriting the
    manifest in canonical form) is NOT introduced;
    trailing-slash and absolute-path style nits are NOT
    flagged; case-sensitivity rules are documented in the
    scaffold comment but not validator-enforced (matches
    Go's underlying filesystem semantics — case-sensitive on
    Linux, case-insensitive on macOS — a portability hazard
    the operator owns).

21. **The doctor warning text from spec 086 is REPLACED, not
    suffixed** (the existing Warn at
    `internal/doctor/docs.go:95`). The current message —
    `missing OWNERSHIP.yaml; doc-sync falls back to
    internal/<domain>/**` — becomes FALSE the moment
    Requirement 13 removes the fallback; appending a hint to
    a false sentence is not acceptable. Full replacement
    message:

    ```
    missing OWNERSHIP.yaml; this domain claims no source
    paths until the manifest exists — run 'mindspec doctor
    --fix' to scaffold a default manifest, then 'mindspec
    ownership populate <domain>' to populate it
    ```

    so the operator's first encounter with a missing
    manifest names the fix command verbatim. This Warn is
    the SOLE coverage for the `Source() == "missing"` state
    (Requirement 17's `dead-manifest` covers existing
    manifests only). The stale rationale comment at
    `internal/doctor/docs.go:84-86` ("doc-sync falls back to
    internal/<domain>/** mapping") is updated in the same
    change (see also Requirement 13's stale-comment list).

22. **`mindspec complete` and `mindspec approve impl` print
    doc-sync warnings — and a recurring migration-status
    line while `source_globs` is unset.** Two parts:

    **(a) Warning printing (closes a verified gap).** Today
    both flows consume the `validate.ValidateDocs` `*Result`
    only via `HasFailures()` plus error-message joining
    (`internal/complete/complete.go:201-204`,
    `internal/approve/impl.go:157-158`); warning-severity
    issues are computed and silently dropped — no `.Issues`
    consumer for warnings exists in `internal/complete/` or
    `internal/approve/`. That makes Requirement 16's
    "re-evaluated on every complete and approve impl" dead
    on arrival: only `mindspec validate docs` would ever
    display the `unclaimed-source` Warn, and no journey step
    directs the operator there. This requirement: both flows
    MUST print every warning-severity issue from the
    doc-sync `*Result` (format: `WARN <name>: <message>`,
    one line per issue, to stderr) — INCLUDING when
    `HasFailures()` is false and the flow proceeds normally.
    Without this, the diff-time half of the continuous
    accuracy loop is invisible in the flows operators
    actually run.

    **(b) Migration-status nudge — recurring, stateless.**
    When `source_globs` is absent or empty, the doc-sync
    validator adds a warning-severity `missing-source-globs`
    issue to its `*Result` (mirroring the static doctor Warn
    of Requirement 18), so the nudge rides the same printing
    pipe as (a) for free. Effect: every `mindspec complete`
    and `mindspec approve impl` run prints one
    migration-status line, e.g.:

    ```
    WARN missing-source-globs: source_globs not set in
    .mindspec/config.yaml — doc-sync is classifying source
    with the built-in default (.go under cmd/ and internal/,
    excluding _test.go); run 'mindspec source populate' to
    declare your own
    ```

    The line RECURS on every run until `source_globs` is
    populated; there is deliberately NO one-time/seen-marker
    state (a persisted marker would be new state in a
    standalone CLI — HC-2 — and a one-time line is missable
    in scrollback). This line — not doctor — is the primary
    discovery surface for `mindspec source populate` in the
    flows operators actually run; doctor's
    `missing-source-globs` Warn (Requirement 18) is the
    static complement. Under the Requirement 16 override
    semantics this is a nudge, not an alarm: while the line
    keeps firing, the built-in default keeps the gate fully
    enforced, so nothing is broken in the unmigrated state.

    **Discovery-scope honesty note.** This line's condition
    is unset-`source_globs` ONLY; it names `mindspec source
    populate` and does NOT directly surface the Requirement
    13 missing-manifest regression. That regression reaches
    the complete/approve surface via a two-step chain: the
    nudge leads to `source_globs` being populated, after
    which the `unclaimed-source` Warn's domain-state report
    (Requirement 16) names any `Source() == "missing"`
    domain and the `doctor --fix` remedy. Doctor's
    Requirement 21 Warn remains the DIRECT surface for
    missing manifests.

## Scope

### In Scope

- `internal/doctor/docs.go` — mark the per-domain OWNERSHIP check
  as fixable; REPLACE the missing-manifest Warn message
  (Requirement 21) and update its stale rationale comment; add
  the fixer function that
  writes the EMPTY-STUB manifest AND prints the populate prompt;
  add the new `dead-manifest` check (Requirement 17, existing
  manifest files only) and the
  three hygiene Warns (Requirement 20).
- `internal/doctor/<fixer>.go` (or wherever the existing
  fixable-check fixers live) — add the OWNERSHIP fixer and the
  `source_globs` config-block fixer (Requirement 11's three
  config states), calling `config.ResetCache()` after writes.
- `cmd/mindspec/ownership.go` (new) and `internal/ownership/`
  (new package) — implement `mindspec ownership populate
  [<domain>]` per Requirement 10. The populate command emits
  a templated prompt; it does NOT call any LLM directly
  (the framework prints; the agent reads).
- `cmd/mindspec/source.go` (new) and `internal/ownership/`
  (same new package) — implement `mindspec source populate`
  per Requirement 12 (repo-wide source-globs prompt emitter).
- `internal/config/` (and the config-loading site for
  `.mindspec/config.yaml`) — add the `source_globs:` field
  per Requirement 11. Existing config consumers continue to
  work; the new field is optional with a documented empty
  default. Tests and fixer flows use `config.ResetCache()`
  around on-disk mutations (the `config.Load` per-process
  cache, config.go:76-110).
- `internal/validate/ownership.go` — remove the silent
  fallback lines 48-53 per Requirement 13. Return
  `Ownership{Paths: []}` when the manifest is absent; add the
  derived `Source()` method per Requirement 13's table (no
  new stored field on the `Ownership` struct); update the
  stale struct-doc comment (lines 21-25). Export (or move to
  a shared package) `loadOwnership` and `globMatch`
  (currently package-private at ownership.go:43 and :143) so
  `internal/doctor` can reuse them for Requirements 17 and
  20 — doctor must NOT reimplement manifest loading or glob
  matching (divergent semantics would make the Warns lie).
- `internal/validate/docsync.go` — implement the
  Requirement 16 override semantics: read `source_globs:`
  from `.mindspec/config.yaml`; when non-empty it FULLY
  REPLACES the `isSourceFile` classifier; when empty/absent,
  the existing `isSourceFile` path runs byte-identically to
  today (HC-7) and `unclaimed-source` is disabled. Add the
  diff-time `missing-source-globs` warning issue
  (Requirement 22(b)). Update the zero-domains-branch
  comment and audit the empty-`ManifestPath` marker per
  Requirement 13's disclosed-fallback obligations.
- `internal/complete/complete.go` and
  `internal/approve/impl.go` (plus their cmd-layer output
  rendering, wherever the printed output is produced) — print
  doc-sync warning-severity issues per Requirement 22, even
  when `HasFailures()` is false.
- `cmd/mindspec/migrate.go` — extend the emitted prompt to
  include both populate phases per Requirement 14.
- `internal/domain/scaffold.go` — `Add()` is extended to write
  `OWNERSHIP.yaml` after the four standard files. `cmd/mindspec/init.go`
  and `internal/bootstrap/` are intentionally NOT in scope
  (audit confirmed init does not scaffold domain directories
  and does not create config.yaml; `doctor --fix` is the sole
  scaffolder for both the manifest stubs and the
  `source_globs` block).
- `internal/validate/docsync.go` and
  `internal/validate/ownership.go` — emit the new
  `unclaimed-source` Warn per Requirement 16, using the
  derived `Ownership.Source()` (Requirement 13) for the
  Warn's mechanical domain-state report.
- `.mindspec/docs/adr/ADR-0035-ownership-discovery.md` (new).
- `.mindspec/docs/adr/ADR-0031-doc-sync-gate.md` — add the
  superseded-in-part note pointing at ADR-0035 (the fallback
  semantics recorded at ADR-0031 lines 54 and 81 are removed
  by Requirement 13).
- Tests for every behavior above (`doctor_test.go`, `init_test.go`
  or equivalent, `domain_test.go` if applicable, `docsync_test.go`,
  `ownership_test.go`, `complete`/`approve` output tests), plus
  the two enumerated existing-test updates from HC-3.

### Out of Scope

- **Promoting any new Warn to Error.** Every new Warn in spec
  091 is advisory; the doc-sync gate's pass/fail behavior is
  unchanged from spec 086 (modulo the HC-6 accepted input
  change).
- **Schema changes to `OWNERSHIP.yaml`.** Same `paths` +
  `exclude` + excluded-first-segments guard as spec 086.
- **Auto-mutation of existing manifests.** Framework never
  rewrites a manifest — even an empty stub it scaffolded
  earlier. The agent or operator populates by editing.
- **Calling out to an LLM directly from the framework.** The
  populate command prints a prompt for whichever agent is
  active in the session; it does not embed an LLM client,
  it does not require any specific model. ZFC-compliant
  prompt emission, not framework-internal AI.
- **Renumbering or restructuring the four standard domain docs.**
  Unchanged.
- **A `mindspec domain add --no-ownership` opt-out.** Every
  invocation scaffolds the empty stub; no per-invocation
  suppression. (The stub is cheap and is `dead-manifest`-warned
  until populated, so there is no harm in always producing it.)
- **Deleting the built-in source classifier.** Under the
  Requirement 16 override semantics, `isSourceFile` survives
  as the disclosed empty-`source_globs` fallback. Its full
  deletion (making `source_globs` mandatory) is deferred to a
  future spec, recorded in ADR-0035 item (h).
- **Resolved-file-set `domain-overlap`.** Requirement 20
  compares literal path strings only; glob-resolution
  intersection is the named follow-up (ADR-0035 item (i)).

## Non-Goals

- This spec adds exactly two new top-level CLI subcommands
  (`mindspec ownership populate`, `mindspec source populate`)
  per Requirement 19 — and no other new CLI surface. Any
  additional subcommands are out of scope.
- This spec introduces no new manifest schema and no
  data-migration step. The single optional `source_globs:`
  config field (Requirement 11) and the two new migrate-prompt
  phases (Requirement 14) are the only schema/prompt additions.
- This spec does not change the doc-sync gate's pass/fail
  behavior for any repo that has not opted in: with
  `source_globs` empty or absent (the universal default at
  upgrade time) source classification is byte-identical to
  today (HC-7), so every diff that passes today still passes.
  Two honest exceptions, both documented in HC-6: (a) some
  diffs that FAIL today — source touched in a manifest-less
  domain that the removed `internal/<domain>/**` fallback
  used to claim — now PASS until that domain's manifest is
  populated (the accepted Requirement 13 breaking change,
  surfaced by the Requirement 21 Warn at doctor time and the
  Requirement 22 migration-status line at complete/approve
  time); (b) once an operator populates `source_globs`, the
  gate's source set is whatever they declared — full
  override — and classification-coverage responsibility
  shifts to the repo.
- This spec does not auto-update existing `OWNERSHIP.yaml` files
  (no migration, no normalization, no canonicalization). Spec 091
  is strictly additive: it only writes manifests that did not
  exist before.

## Acceptance Criteria

- [ ] **`mindspec doctor --fix` scaffolds an empty-stub
  `OWNERSHIP.yaml`** for every domain directory under
  `.mindspec/docs/domains/<domain>/` that lacks one. After
  `--fix` runs, the manifest exists; its first non-comment
  line is `paths: []`; `--fix` prints the Requirement 10
  populate prompt for each scaffolded domain. Files are NEVER
  overwritten — a hand-authored manifest is left untouched
  even under `--fix --force`. A second `mindspec doctor` run
  STILL reports a Warn for each scaffolded domain (because
  the now-EXISTING stub's `paths: []` resolves to zero files
  — see `dead-manifest`, Requirement 17); the Warn only
  clears once the agent or operator populates the manifest.
- [ ] **`mindspec doctor --fix` scaffolds the `source_globs`
  config block** per Requirement 11's three states: config
  file absent → file created containing exactly the
  documented comment + `source_globs: []` block; file
  present without the field → block appended, all prior
  bytes of the file unchanged; file present with the field →
  file byte-identical before and after. Validation: three
  unit tests (one per state), each calling
  `config.ResetCache()` before re-reading.
- [ ] **`mindspec domain add <name>` scaffolds an empty-stub
  OWNERSHIP.yaml** alongside the four standard docs and prints
  the populate prompt. Validation: `mindspec domain add foo`
  in a temp repo produces
  `.mindspec/docs/domains/foo/OWNERSHIP.yaml` matching the
  Requirement 8 stub (paths: []); stdout contains the
  populate prompt text. `internal/domain/scaffold_test.go`
  adds a test asserting the file is present and contains
  the empty stub after `Add()`.
- [ ] **`mindspec ownership populate <domain>`** prints a
  templated agent prompt to stdout. The prompt names the
  manifest path, the domain name, the relevant doc files to
  read (`overview.md`, `architecture.md`), the manifest
  schema (`paths:` + optional `exclude:`, with the
  `viz`/`agentmind`/`bench` excluded-first-segments hard
  error named), and the
  instruction to inspect the repo before proposing paths.
  `mindspec ownership populate` with no domain arg prints
  one prompt per missing-or-empty manifest. Validation: unit
  test in `internal/ownership/populate_test.go` (new package)
  asserts the prompt contains the literal domain name and
  the manifest path, and that the prompt does NOT contain
  any literal `internal/<domain>/**` proposal (proving the
  framework does not pre-fill paths).
- [ ] **`mindspec migrate` includes BOTH populate phases,
  honouring the two binding ordering constraints
  (Requirement 14).** Validation:
  `cmd/mindspec/migrate_test.go` asserts the emitted prompt
  string (a) contains a `mindspec source populate` reference
  whose index is AFTER the Domain Identification phase
  heading, and (b) contains a `mindspec ownership populate`
  reference whose index is AFTER the `mindspec domain add`
  instruction text. No assertion is made about the relative
  order of the two new phases against each other or about
  final phase numbers — those are the first bead's
  `buildMigratePrompt` audit output.
- [ ] **`source_globs:` field is added to the config schema.**
  Validation: a unit test in `internal/config/` (or
  wherever config parsing lives) round-trips a config with
  `source_globs: [cmd/**, internal/**]` and asserts the
  parsed value matches. A second test asserts a config with
  no `source_globs:` field parses successfully with an
  empty default. A third asserts a missing config FILE
  yields the same empty default (the brownfield state,
  Requirement 18).
- [ ] **`mindspec source populate` prints the source-globs
  prompt.** Validation: unit test in
  `internal/ownership/source_populate_test.go` (or
  equivalent) asserts the emitted prompt contains the
  literal string `source_globs:`, the path
  `.mindspec/config.yaml`, the full-override warning
  ("FULLY REPLACES"), and does NOT contain any
  literal `cmd/**` or `internal/**` proposal as a suggested
  glob value (proving the framework does not pre-fill).
- [ ] **Doctor emits `missing-source-globs` Warn** in all
  three Requirement 18 states. Validation: tests in
  `internal/doctor/config_test.go` (or equivalent) — (a) no
  `.mindspec/config.yaml` at all → Warn fires and names the
  expected path; (b) config with `source_globs: []` → Warn
  fires; in both, the message DISCLOSES the built-in default
  (contains `built-in default`). Companion test: config with
  `source_globs: [cmd/**]` → no `missing-source-globs` Warn.
- [ ] **`unclaimed-source` Warn is disabled when
  `source_globs` is empty — and the built-in fallback still
  drives the gate.** Validation: test in
  `internal/validate/docsync_test.go` asserts that with
  an empty `source_globs:` and a diff touching
  `internal/foo/bar.go`, no `unclaimed-source` Warn fires,
  AND the blocking-lane outcome for that diff is identical
  to the pre-091 outcome (the fallback classifier ran).
- [ ] **The silent fallback in `internal/validate/ownership.go`
  is removed.** Validation: a test that calls the loader
  with a domain whose manifest does not exist asserts the
  returned `Ownership.Paths` is empty (not
  `[internal/<domain>/**]`) and the derived
  `Ownership.Source()` returns `"missing"` (Requirement 13;
  no stored field). The previously-existing
  `TestOwnershipFallback` (ownership_test.go:76) is updated
  to assert the
  new behaviour; this update is the regression gate
  proving the violation is closed — and it, not the
  doctor-surface shell proof, is the authoritative proof of
  the loader change.
- [ ] **Empty `source_globs` preserves today's classification
  byte-for-byte (HC-7 / override-semantics gate, empty
  side).** Validation: a test in
  `internal/validate/docsync_test.go` runs the classifier
  over a fixture diff containing a mix of `.go` files under
  `cmd/` and `internal/`, `_test.go` files, doc files, and
  non-Go files, with `source_globs` empty/absent, and
  asserts (a) the classified source set is exactly what
  `isSourceFile` selects, and (b) `*Result.HasFailures()`
  matches the pre-091 outcome for the identical fixture.
  Satisfiable by construction: the empty case executes the
  same `isSourceFile` code path. (Replaces the prior draft's
  unsatisfiable AC that demanded a POPULATED glob set
  reproduce `isSourceFile` exactly — include-only globs
  cannot express the `_test.go` exclusion.)
- [ ] **Populated `source_globs` fully overrides — never
  union (override-semantics gate, populated side).**
  Validation: with `source_globs: [pkg/**]`, (a) a diff
  touching `pkg/foo.js` classifies it as source (the
  built-in `.go`-only rule does NOT apply), and (b) a diff
  touching `internal/foo/bar.go` does NOT classify it as
  source (the built-in `cmd/`+`internal/` rule does NOT
  apply).
- [ ] **The doctor Warn message for missing OWNERSHIP.yaml**
  is the full Requirement 21 replacement: it does NOT
  contain the stale `falls back` claim and DOES contain the
  literal hint `run 'mindspec doctor --fix' to scaffold
  a default manifest`. Grep:
  `mindspec doctor 2>&1 | grep -q "run 'mindspec doctor --fix'"`
  and `! mindspec doctor 2>&1 | grep -q "falls back"`
  (NOT `grep -qv`, which succeeds whenever ANY line fails to
  match and is therefore vacuous)
  in a repo with at least one missing manifest.
- [ ] **The doc-sync validator emits an `unclaimed-source` Warn**
  whenever a diff modifies a file matching a non-empty
  `source_globs:` that does not match any domain's resolved
  `paths`, regardless of how each domain's Ownership was
  loaded (`Source()` is `"manifest"`, `"empty-stub"`, or
  `"missing"`). Validation: three unit tests in
  `internal/validate/docsync_test.go` —
  (a) `source_globs: [internal/**]`, diff touching
  `internal/contextpack/foo.go`, domain `context-system` has
  no manifest → loader returns `Paths: []` / `Source()
  == "missing"` (post-fallback-removal) → Warn fires, the
  domain-state report annotates `context-system` as
  `missing`.
  (b) `source_globs: [internal/**]`, same diff, domain
  `context-system` has a scaffolded empty-stub
  (`Source() == "empty-stub"`, `Paths: []`) → Warn fires,
  the state report annotates `context-system` as
  `empty-stub`.
  (c) `source_globs: [internal/**]`, same diff, domain
  `context-system` has a hand-populated manifest pointing
  somewhere else (`Source() == "manifest"`, `Paths:
  [internal/something-else/**]`, where
  `internal/something-else/` EXISTS and contains files) →
  Warn fires for the unclaimed file; the state report
  annotates `context-system` as `manifest` (it is not
  presented as a populate candidate). Honesty note
  (corrected from the prior draft): NOTHING in this spec
  flags the wrong-but-resolving manifest itself —
  `dead-manifest` (Requirement 17) catches a wrong manifest
  only when its glob resolves to ZERO files, and
  `domain-overlap` compares literal strings. The misclaim
  surfaces only as this `unclaimed-source` Warn on the
  orphaned file; the residual gap is accepted per HC-6 /
  ADR-0035 item (i).
  Tests (a) and (b) are the regression gates proving the
  fallback-removal semantics work: case (b) proves
  "initial accuracy is not forever accuracy" (empty stub
  doesn't silently pass).
- [ ] **`unclaimed-source` with zero unpopulated domains
  names the right remedies.** Validation: unit test where
  EVERY domain's `Source()` is `"manifest"` and an
  unclaimed source file is touched → the Warn message
  states no unpopulated domains exist and hints at widening
  an existing manifest (`mindspec ownership populate
  <domain>`) or creating a new domain (`mindspec domain
  add`); it does NOT hint `mindspec doctor --fix` (which
  would do nothing in that state).
- [ ] **The Warn does NOT block the gate.** A unit test asserts
  that with unclaimed source files and no other doc-sync error,
  `validate.ValidateDocs` returns a `*Result` whose
  `HasFailures()` is false.
- [ ] **The Warn does NOT fire for purely-docs diffs.** A unit
  test asserts that when the diff modifies only files under
  `.mindspec/docs/**` (no path matches `source_globs:`), no
  `unclaimed-source` Warn is emitted.
- [ ] **`mindspec doctor` emits a `dead-manifest` Warn** for
  every domain whose EXISTING `OWNERSHIP.yaml` `paths` glob
  resolves to
  zero files in the workspace. Validation: a test in
  `internal/doctor/docs_test.go` creates a domain `foo` with
  manifest `paths: [internal/foo/**]`, no `internal/foo/`
  directory present → `doctor` reports a `Warn` whose name is
  `docs/domains/foo/OWNERSHIP.yaml` and whose message contains
  `dead-manifest` and the literal suspect glob.
- [ ] **`dead-manifest` does NOT fire for a missing manifest.**
  Companion test: domain `ghost` with NO `OWNERSHIP.yaml`
  file → no `dead-manifest` Warn for `ghost`; the
  Requirement 21 missing-OWNERSHIP Warn fires instead (one
  state, one Warn).
- [ ] **The `dead-manifest` Warn clears once the manifest
  matches at least one file.** Second test in
  `docs_test.go`: same setup as the dead-manifest test, but
  with one file
  created at `internal/foo/bar.go` → no `dead-manifest` Warn
  for domain `foo`.
- [ ] **`doctor` Warns `duplicate-entry`** when a `paths`
  list contains the same literal entry twice. Validation:
  unit test in `internal/doctor/docs_test.go` writes a
  manifest with `paths: [internal/foo/**, internal/foo/**]`
  → Warn fires, message names the duplicated entry.
- [ ] **`doctor` Warns `redundant-subpath`** when a `paths`
  entry is strictly contained within another. Validation:
  unit test writes
  `paths: [internal/foo/**, internal/foo/bar/**]` → Warn
  fires, message names both entries and identifies
  `internal/foo/bar/**` as the redundant one.
- [ ] **`doctor` Warns `domain-overlap`** when the same
  literal path appears in two domains' `paths`. Validation:
  unit test writes manifests under domains `a/` and `b/`
  both containing `paths: [internal/shared/**]` → Warn
  fires, message names both domains and the overlapping
  path.
- [ ] **Hygiene Warns do NOT block any gate.** Unit test
  asserts that with all three hygiene Warns present and no
  other doctor errors, `doctor` exits zero.
- [ ] **`mindspec complete` and `mindspec approve impl` print
  doc-sync warnings (Requirement 22(a)).** Validation: unit
  tests at the layer where each flow renders output assert
  that a doc-sync `*Result` carrying one warning-severity
  issue and NO errors produces output containing `WARN` and
  the issue message, AND the flow proceeds (no failure).
  Companion: with zero warning issues, no `WARN` line is
  printed.
- [ ] **The migration-status line recurs statelessly
  (Requirement 22(b)).** Validation: with `source_globs`
  absent/empty, the doc-sync `*Result` carries the
  warning-severity `missing-source-globs` issue (so complete
  and approve print it) on EVERY invocation — a test runs
  the validation twice and asserts the issue is present both
  times and that no marker/state file was created. With
  populated `source_globs`, the issue is absent.
- [ ] **Existing OWNERSHIP.yaml files are never modified.** A
  unit test seeds a hand-authored manifest, runs the fixer, and
  asserts the file bytes are byte-identical before and after.
- [ ] **The ownership-discovery ADR exists** under
  `.mindspec/docs/adr/` (as `ADR-0035-ownership-discovery.md`
  if `0035` is still free at bead-claim time, else the next
  free integer per the ADR Touchpoints reservation
  procedure) and records the ZFC stance, the empty-stub
  scaffold content, the fallback removal, the
  disclosed-default override semantics, the accepted
  wrong-but-resolving-glob gap, and
  the no-overwrite policy. The authoring bead takes the
  number at bead-claim time and updates every
  cross-reference in this spec in the same commit that
  creates the file — no PR-open-time renumbering followup
  exists or is needed. **ADR-0031 carries the
  superseded-in-part note** pointing at ADR-0035.
- [ ] **`go build ./... && go test -short ./...` is green** on
  the 091 branch with no skipped or excluded tests vs. `main`,
  modulo the two existing-test updates enumerated in HC-3
  (`TestOwnershipFallback`,
  `TestValidateDocsErrorsOnInternalDocSkew_Fallback`), which
  are updated in-place, not skipped or deleted.

## Validation Proofs

- **`doctor --fix` scaffolds empty-stub manifests:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  # init scaffolds ZERO domains (bootstrap.go:215,
  # TestRun_NoDomainScaffolding) — create some first, or the
  # globs below match nothing and the proof tests nothing:
  mindspec domain add alpha
  mindspec domain add beta
  rm -f .mindspec/docs/domains/*/OWNERSHIP.yaml
  mindspec doctor 2>&1 | grep -q "missing OWNERSHIP.yaml"
  out=$(mindspec doctor --fix 2>&1)
  for d in .mindspec/docs/domains/*/; do
    test -s "${d}OWNERSHIP.yaml" \
      || { echo "missing: $d"; exit 1; }
    grep -qE '^paths:\s*\[\s*\]\s*$' "${d}OWNERSHIP.yaml" \
      || { echo "expected paths: [] stub in $d"; exit 1; }
    grep -qE '^\s*-\s+internal/' "${d}OWNERSHIP.yaml" \
      && { echo "stub must NOT contain pre-filled paths in $d"; exit 1; }
  done
  # --fix prints the populate prompt:
  echo "$out" | grep -q "mindspec ownership populate" \
    || { echo "--fix did not print populate prompt"; exit 1; }
  # missing-manifest Warn clears, but dead-manifest Warn now fires
  # (the stub EXISTS now, so dead-manifest owns this state — Req 17):
  out2=$(mindspec doctor 2>&1)
  echo "$out2" | grep -q "missing OWNERSHIP.yaml" \
    && { echo "missing-manifest still warns after --fix"; exit 1; } || :
  echo "$out2" | grep -q "dead-manifest" \
    || { echo "dead-manifest should warn on empty stub"; exit 1; }
  echo OK
  ```
- **Missing manifest fires the rewritten Requirement 21 Warn —
  and NOT `dead-manifest`:** (doctor-surface proof only; the
  AUTHORITATIVE proof of the Requirement 13 loader change is
  the `TestOwnershipFallback` unit-test update in the
  Acceptance Criteria — a doctor grep cannot observe loader
  behavior)
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  mkdir -p .mindspec/docs/domains/ghost
  # no OWNERSHIP.yaml — Source()=="missing": covered solely by
  # the Requirement 21 Warn; dead-manifest is reserved for
  # EXISTING manifest files (Requirement 17).
  out=$(mindspec doctor 2>&1)
  echo "$out" | grep -q "missing OWNERSHIP.yaml" \
    || { echo "missing-OWNERSHIP Warn did not fire"; exit 1; }
  echo "$out" | grep -q "run 'mindspec doctor --fix'" \
    || { echo "Warn lacks the --fix remedy"; exit 1; }
  echo "$out" | grep -q "falls back" \
    && { echo "stale fallback claim still in Warn text"; exit 1; } || :
  echo "$out" | grep -q 'dead-manifest' \
    && { echo "dead-manifest must NOT fire for a missing manifest"; exit 1; } || :
  echo OK
  ```
- **`missing-source-globs` Warn fires on a fresh repo:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  # init does NOT create .mindspec/config.yaml — the file is
  # absent, which is exactly the file-absent state Requirement
  # 18 must cover.
  out=$(mindspec doctor 2>&1)
  echo "$out" | grep -q 'missing-source-globs' \
    || { echo "expected missing-source-globs Warn"; exit 1; }
  # The Warn must disclose the active built-in default:
  echo "$out" | grep -qi 'built-in default' \
    || { echo "Warn must disclose the built-in classifier"; exit 1; }
  echo OK
  ```
- **`mindspec source populate` prints repo-wide prompt:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  out=$(mindspec source populate 2>&1)
  echo "$out" | grep -q 'source_globs' \
    || { echo "prompt missing source_globs reference"; exit 1; }
  echo "$out" | grep -q '.mindspec/config.yaml' \
    || { echo "prompt missing config path"; exit 1; }
  echo "$out" | grep -q 'FULLY REPLACES' \
    || { echo "prompt missing the full-override warning"; exit 1; }
  # Framework must NOT pre-propose globs:
  echo "$out" | grep -qE '^\s*-\s+cmd/\*\*' \
    && { echo "prompt must not pre-fill globs"; exit 1; } || :
  echo OK
  ```
- **`unclaimed-source` Warn is disabled when source_globs is empty
  (and the built-in fallback still drives the blocking lanes):**
  Covered by unit tests in `internal/validate/docsync_test.go`
  per Acceptance Criteria (including the byte-identical
  empty-globs classification gate); validation is the tests
  passing in CI.
- **`mindspec ownership populate <domain>` prints a prompt:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  mkdir -p .mindspec/docs/domains/auth
  printf 'paths: []\n' > .mindspec/docs/domains/auth/OWNERSHIP.yaml
  out=$(mindspec ownership populate auth 2>&1)
  echo "$out" | grep -q "OWNERSHIP.yaml" \
    || { echo "prompt missing manifest path"; exit 1; }
  echo "$out" | grep -q '"auth"' \
    || { echo "prompt missing domain name"; exit 1; }
  # Framework must NOT pre-propose paths:
  echo "$out" | grep -qE 'internal/auth/\*\*' \
    && { echo "prompt must not pre-fill paths"; exit 1; } || :
  echo OK
  ```
- **Hand-authored manifest is preserved:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  mkdir -p .mindspec/docs/domains/foo
  printf 'paths:\n  - cmd/foo-cli/**\n' > .mindspec/docs/domains/foo/OWNERSHIP.yaml
  # shasum -a 256 is portable (stock macOS has no sha256sum):
  before=$(shasum -a 256 .mindspec/docs/domains/foo/OWNERSHIP.yaml)
  mindspec doctor --fix
  after=$(shasum -a 256 .mindspec/docs/domains/foo/OWNERSHIP.yaml)
  [ "$before" = "$after" ] || { echo "hand-authored manifest mutated"; exit 1; }
  echo OK
  ```
- **Even `--fix --force` does not overwrite:** same as above but
  with `mindspec doctor --fix --force`; assert hash unchanged.
- **`mindspec domain add` scaffolds the empty stub + prints prompt:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  out=$(mindspec domain add foo 2>&1)
  test -s .mindspec/docs/domains/foo/OWNERSHIP.yaml \
    || { echo "domain add did not scaffold OWNERSHIP.yaml"; exit 1; }
  grep -qE '^paths:\s*\[\s*\]\s*$' .mindspec/docs/domains/foo/OWNERSHIP.yaml \
    || { echo "expected paths: [] stub"; exit 1; }
  grep -qE '^\s*-\s+internal/foo/\*\*' .mindspec/docs/domains/foo/OWNERSHIP.yaml \
    && { echo "stub must NOT contain pre-filled paths"; exit 1; } || :
  echo "$out" | grep -q "mindspec ownership populate" \
    || { echo "domain add must print populate prompt"; exit 1; }
  echo OK
  ```
- **Doctor's warning text names the fix command:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  # Create a domain first — init scaffolds zero domains, so a
  # bare init has no missing manifest to Warn about:
  mindspec domain add foo
  rm -f .mindspec/docs/domains/*/OWNERSHIP.yaml
  mindspec doctor 2>&1 \
    | grep -q "run 'mindspec doctor --fix' to scaffold a default manifest"
  ```
- **`mindspec complete` / `mindspec approve impl` print doc-sync
  warnings + the migration-status line (Requirement 22):**
  Primary validation is the unit tests in the Acceptance
  Criteria (output-layer tests for both flows — an end-to-end
  shell proof needs a full spec lifecycle). End-to-end grep
  sketch for the autopilot to run inside any 091-branch repo
  with `source_globs` unset, at the next natural `mindspec
  complete` of a real bead:
  ```bash
  mindspec complete <bead-args> 2>&1 | tee /tmp/complete.out
  grep -q 'WARN missing-source-globs' /tmp/complete.out \
    || { echo "complete did not print the migration-status line"; exit 1; }
  # run a second complete later in the same repo state — the
  # line must RECUR (stateless nudge):
  ```
- **The `unclaimed-source` Warn fires whether the manifest is
  absent OR is a scaffolded empty stub:** covered by the
  three unit tests (a)/(b)/(c) in
  `internal/validate/docsync_test.go` per Acceptance Criteria.
  Test (b) (empty stub whose domain's real files are touched)
  is the regression gate proving the spec's "continuous
  accuracy loop" claim — without it, "initial accuracy" is
  also "forever accuracy".
- **The `dead-manifest` Warn fires at static `doctor` time
  for any EXISTING manifest that resolves to zero files:**
  covered by
  the unit tests in `internal/doctor/docs_test.go` per
  Acceptance Criteria. End-to-end shell proof:
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  mkdir -p .mindspec/docs/domains/ghost
  printf 'paths:\n  - internal/ghost/**\n' \
    > .mindspec/docs/domains/ghost/OWNERSHIP.yaml
  mindspec doctor 2>&1 | grep -q 'dead-manifest' \
    || { echo "doctor did not Warn dead-manifest"; exit 1; }
  echo OK
  ```
- **The hygiene Warns fire on a hand-crafted bad manifest:**
  ```bash
  tmp=$(mktemp -d)
  cd "$tmp"
  mindspec init
  mkdir -p .mindspec/docs/domains/{a,b}
  # Duplicate + subpath redundancy in domain a:
  cat > .mindspec/docs/domains/a/OWNERSHIP.yaml <<'YAML'
paths:
  - internal/a/**
  - internal/a/**
  - internal/a/sub/**
  - internal/shared/**
YAML
  # Cross-domain overlap on internal/shared:
  cat > .mindspec/docs/domains/b/OWNERSHIP.yaml <<'YAML'
paths:
  - internal/b/**
  - internal/shared/**
YAML
  out=$(mindspec doctor 2>&1)
  echo "$out" | grep -q 'duplicate-entry'   || { echo missing duplicate; exit 1; }
  echo "$out" | grep -q 'redundant-subpath' || { echo missing subpath; exit 1; }
  echo "$out" | grep -q 'domain-overlap'    || { echo missing overlap;  exit 1; }
  echo OK
  ```
- **Existing test suite preserved (modulo the HC-3 enumerated
  updates):**
  ```bash
  go test -short ./...
  ```
- **ADR-0035 present, ADR-0031 amended:**
  ```bash
  test -f .mindspec/docs/adr/ADR-0035-ownership-discovery.md
  grep -qi 'superseded in part' .mindspec/docs/adr/ADR-0031-doc-sync-gate.md
  ```

## Open Questions

Resolved at spec-draft time by code audit (recorded under
Requirements 9 and 15):

- **`mindspec init` does NOT scaffold domain directories** — by
  deliberate design, not omission. `internal/bootstrap/bootstrap.go:215`
  creates the empty parent `.mindspec/docs/domains/` directory;
  `internal/bootstrap/bootstrap_test.go:166` (`TestRun_NoDomainScaffolding`)
  explicitly enforces the empty-on-init invariant. The Long
  description of `cmd/mindspec/init.go` directs brownfield projects
  to `mindspec migrate`. **Both flows lead to `mindspec domain
  add`**: greenfield users invoke it directly as domains emerge;
  brownfield migrate emits a phased prompt whose Phase 2
  (`cmd/mindspec/migrate.go:186-199`) instructs the agent to
  call `mindspec domain add` per identified domain. The single
  `Add()` hook in spec 091 therefore covers both flows; `doctor
  --fix` covers any remaining hand-created domain directories.
  (init likewise does not create `.mindspec/config.yaml`;
  `doctor --fix` is the sole `source_globs` scaffolder,
  Requirement 11.)
- `mindspec domain add <name>` exists at `cmd/mindspec/domain.go`,
  delegates to `internal/domain/scaffold.go:Add()`, which creates
  the domain directory + the four standard docs + a context-map
  entry. `OWNERSHIP.yaml` write is added there.
- Existing fixable-check fixers use the `Check.FixFunc` pattern,
  exercised by `internal/doctor/beads.go` and
  `internal/doctor/git.go`. The OWNERSHIP fixer attaches to the
  same shape.

**Still open — deferred to the first implementation bead** (the
prior draft's "no questions remain unresolved" claim was wrong;
these two are tracked here explicitly):

- **`buildMigratePrompt` insertion-site audit (Requirement 14).**
  Resolved-by: the first bead generated by `mindspec plan
  approve` reads `buildMigratePrompt` in full and names the
  exact insertion sites and final phase numbers for the two new
  phases, honouring the two binding ordering constraints.
- **Reachability of the per-domain empty-`ManifestPath`
  fallback marker (Requirement 13,
  `internal/validate/docsync.go:~250-260`).** Resolved-by: the
  bead implementing Requirement 13 audits whether the marker
  path is reachable post-fallback-removal, then either relabels
  it as a disclosed-fallback marker or deletes it as dead code,
  with a test pinning the outcome.

## Appendix: Revision History

- **2026-06-10 — Fresh panel round 1 + adjudicated decisions.**
  A fresh blind 3-reviewer panel (z1-design/ZFC,
  z2-implementability, z3-operator; consolidated in
  `panel-091-fresh/CONSOLIDATED.md`) returned 3×
  REQUEST_CHANGES with 2 distinct blockers and 21 deduplicated
  mechanical fixes. Three design questions were adjudicated by
  a separate 3-agent decision panel
  (`panel-091-decide/a{1,2,3}.json`):
  - **Q-A (3/3, overturning the consolidation's option-1
    recommendation):** `source_globs` FULLY OVERRIDES a
    DISCLOSED in-code fallback (`isSourceFile`) — never union;
    empty/absent globs = today's behavior byte-identically, so
    HC-7 genuinely holds and the silent-regression window of
    an inert gate never opens. Goal 8 reworded from "replaced"
    to "overridable disclosed default"; full classifier
    deletion deferred (ADR-0035 item (h)). The unsatisfiable
    exact-equivalence regression AC was replaced with the
    two-sided override-semantics ACs.
  - **Q-B (3/3):** `dead-manifest` fires ONLY for existing
    manifest files; `Source()=="missing"` is covered solely by
    the rewritten Requirement 21 Warn — one state, one Warn,
    paired remedies (missing → `doctor --fix`; dead →
    `ownership populate`). HC-6 bullet 1 and the ghost-domain
    proof rewritten accordingly.
  - **Q-C (2/3, with the recurring-stateless correction):**
    complete/approve print a RECURRING stateless
    migration-status line while `source_globs` is unset,
    riding on the new Requirement 22 warning-printing
    requirement (the panel's verified blocker:
    complete.go:201-204 drops doc-sync `Result` warnings).
    Non-Goals reworded honestly (the Requirement 13 pass→fail
    flip for fallback-claimed manifest-less domains is an
    accepted, surfaced exception).
  All 21 mechanical fixes (M1-M21) applied, with M4
  (zero-domains heuristic) and M8 (populate discoverability)
  adapted to the Q-A/Q-C outcomes: the surviving fallbacks are
  disclosure+test obligations rather than deletions, and the
  Requirement 22 nudge is the populate-discovery surface.
  Requirement 22 added (no renumbering of 8-21 required).

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-10
- **Notes**: Approved via mindspec approve spec