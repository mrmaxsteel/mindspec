# Workflow Domain — Runbook

## Common Operations

### Onboard a greenfield repo (spec 123)

In an empty directory, the first-run verbs now compose in any order:

```bash
git init .
mindspec init                      # scaffolds .mindspec/, context-map.md skeleton,
                                   # neutral AGENTS.md, and gitignores the runtime files
mindspec domain add alpha          # scaffolds the domain AND its context-map entry
mindspec adr create "First decision" --domain alpha   # writes ADR-0001-first-decision.md
mindspec setup codex               # refreshes the managed AGENTS.md block; also
                                   # ensures the runtime gitignore entries
mindspec doctor                    # green on the governed lanes; four advisory Warns
                                   # + a Beads [MISSING] line remain (see below)
```

- On this untouched greenfield state doctor reports FOUR Warns plus one
  Missing — none of them structural failures on the lanes spec 123
  governs (context-map, gitignore, scaffolding):
  - `missing-models` and `missing-commands` are the two Warns spec 123
    DESIGNED as ZFC nudges (and what its AC-1/AC-19 assertions scope
    to): run `mindspec models populate` (declare the per-phase
    protocol) and `mindspec commands populate` (declare this repo's
    real build/test — the framework never guesses; while unset, the
    managed AGENTS.md "Build & Test" section is omitted entirely).
  - Two further pre-existing advisory Warns also appear because the
    fresh scaffold hasn't been populated yet: `dead-manifest` (the
    scaffolded `OWNERSHIP.yaml` paths glob is empty — run
    `mindspec ownership populate alpha`) and `missing-source-globs`
    (doc-sync is classifying source with the built-in default — run
    `mindspec source populate`).
  - `Beads: [MISSING]` (".beads/ directory not found — run
    `beads init`"). Warns never fail doctor, so this Missing line is
    the only reason the untouched sequence exits non-zero.
- Recovering a pre-123 partial state (`domains/<name>/` present but no
  context-map entry, or missing standard files): re-run
  `mindspec domain add <name>` — it backfills whatever is missing and
  never overwrites existing files. `mindspec doctor` names each
  unmapped domain with exactly that recovery line.
- A runtime file reported "not gitignored" by doctor: `mindspec doctor
  --fix` appends the entry; a TRACKED runtime file is the worse state
  and gets the existing untrack `--fix`.

### Start a New Spec

Use `/spec-init` or create manually:
```
.mindspec/specs/<NNN-slug>/
  spec.md
  context-pack.md (placeholder)
```

### Approve a Spec

1. Verify all acceptance criteria are defined and measurable
2. Verify impacted domains and ADR touchpoints are declared
3. Verify all open questions are resolved
4. Use `/spec-approve` or update the spec's Approval section to `Status: APPROVED`

### Create Implementation Plan

1. Review accepted ADRs for impacted domains
2. Review domain docs (overview, architecture, interfaces)
3. Check Context Map for neighbor contracts
4. Decompose spec into bounded implementation beads
5. Use `/plan-approve` when ready

### Execute an Implementation Bead

1. Claim via `mindspec next` — the mechanical readiness floor (MF-1..MF-4)
   is evaluated BEFORE the claim; a NOT-READY refusal mutates nothing
   (spec 124). `mindspec bead ready-check <bead-id>` prints the same
   per-signal report standalone at any time (pure read).
2. Create worktree: `worktree-<bead-id>`
3. Load context pack for the bead
4. Dispatch via `/ms-bead-impl` — its ingress re-runs `ready-check` on
   every dispatch path, and the staged prompt's Phase 0 has the subagent
   judge the semantic signals (SR-1..SR-5) before any edit
5. Implement within the bead's scope
6. Capture proof (test outputs, command results)
7. Update documentation
8. Close bead with evidence

### Triage a NOT-READY bead (spec 124 R5/R8)

A subagent return whose first line is `NOT READY: <bead-id>` is a
pre-damage refusal, not an implementation failure: no panel round is
consumed, it does not count toward `max_consecutive_impl_failures`, it
never routes to `/ms-bead-fix`, and the worktree is intact (zero
commits). Choose exactly one disposition:

1. **ACCEPT (default)** — halt the bead, surface the ordinal-numbered
   report, revise the plan/spec section the reasons quote, re-dispatch
   via `/ms-bead-impl`.
2. **Clarify (once per bead, ever)** — author the reason-keyed record
   (each entry: ordinal, verbatim reason, concrete answer, authoritative
   source span) and write it:
   ```bash
   mindspec bead clarify <bead-id> --file <record.json>
   ```
   then re-dispatch; the ingress pairs each original reason with its
   clarification for the fresh Phase-0 judgment. The cap is categorical
   and restart-proof — a second `bead clarify` on the same bead is
   refused by the verb; the next NOT READY must ACCEPT.

A clarification can only DISAMBIGUATE existing spec/plan/landed-code
authority — when no span supports the answer, ACCEPT is correct, not
clarify. For a known-acceptable MECHANICAL fail, the lever is
`mindspec next --allow-not-ready` (recorded override marker), never
`bead clarify` — the two are not interchangeable.

### Handle ADR Divergence

1. Stop work immediately
2. Identify the ADR and nature of divergence
3. Present options to user: continue-as-is vs propose new ADR
4. If user approves divergence: create superseding ADR
5. Resume only after new ADR is accepted

## Troubleshooting

### Mode Confusion

Check current mode with `/spec-status`. If unclear:
- No approved spec? You're in Spec Mode.
- Approved spec but no approved plan? You're in Plan Mode.
- Both approved + active bead? You're in Implementation Mode.

### Missing or stale landed-binding on a merged bead (spec 125)

Symptom: `FindLandedMerge`-backed surfaces (`complete`'s
merged-unclosed reconcile, `doctor`, `next`) refuse to identify a bead
that you know merged — typically a pre-125 bead (the fleet state spec
125 fixed forward: the old exact-subject scan missed every
conflict-recovery merge and silently swallowed the miss) or a
hand-crafted operator merge.

Recovery: `mindspec reattest <bead-id>`. It derives the binding from an
independent exact-second-parent git scan of the spec branch (resolved
from the bead's epic linkage; pass `--spec-branch spec/<id>` only when
the linkage is underivable) and writes the scan-derived SHAs plus a
`mindspec_landed_reattest_*` audit record — inspect with
`bd show <bead-id> --json`. It is fail-closed: on no owned exact merge
it refuses and names the audited ADR-0035 `mindspec-q9ea` human
attested-restore as the only exit (inspect
`git log --first-parent --merges <spec-branch>`, VERIFY the candidate's
diff, then `git branch bead/<id> <verified-second-parent-sha>` — never
blind); on ambiguity, a contradicting datum, or reverted content it
refuses and writes nothing. A merge whose subject names NO bead is not
auto-identifiable anywhere (fail-closed by design — the binding alone is
never an ownership authority), and `reattest` CANNOT recover it either:
it too requires a subject-nominated merge. The honest recovery there is
to re-merge the work under a bead-naming subject (mindspec's own merges
always name the bead), or accept the safe fail-closed; the audited
q9ea attested-restore above is the last-resort exit.

Also: if `mindspec complete` refuses with "has landed … but the
landed-merge commit could not be located by identity", that is the spec
125 loud-miss guard — cleanup was suppressed and the bead branch
survives; re-run the lifecycle command after addressing the named
cause rather than deleting the branch.

## Maintenance Notes

- **2026-07-02 (spec 107 wave 1):** The hidden `spec init` alias
  (`cmd/mindspec/spec_init.go`) was de-duplicated to reuse `specCreateCmd.RunE`
  instead of carrying a byte-identical copy of the create flow, so future
  `spec create` changes propagate to the alias automatically. Behavior of
  `mindspec spec init` is unchanged; the alias still registers its own `--title`
  flag.
- **2026-07-02 (spec 108 wave 2, Bead 4):** `mindspec doctor`'s dead-manifest
  check (`internal/doctor/ownership.go`) now walks the workspace tree **once per
  ownership check** instead of once per domain. A single enumeration collects the
  live file list (still skipping `.git/`, `.worktrees/`, and `.beads/`, V2-6), and
  every domain's `paths:` globs are tested against that cached list. The walk is
  routed through the package-level `walkWorkspaceFn` seam so a test can count its
  invocations. Doctor output is unchanged: the same dead-manifest Warn/pass result
  per domain, just fewer directory walks on the `doctor` hot path.
- **2026-07-09 (spec 110 Bead 5):** The panel operator procedure is now
  mechanized behind `mindspec panel create|verify|tally` (spec 110 Bead 4) — a
  thin CLI layer over `internal/panel`'s single-home writer + `PanelGateDecision`.
  `/ms-panel-run` Step 0 registers (or re-panels) with one `panel create` call
  instead of a hand-typed `panel.json` schema and a skill-re-authored
  verdict-JSON template; `/ms-panel-tally` renders its decision and the
  aggregated `concrete_changes_required` with one `panel tally` call instead of
  a hand-tabulated decision matrix. The judgment sections both skills retain —
  `/ms-panel-run`'s **Launch the panel**, **Codex failure detection**,
  **Working directory matters**, **Slot lens defaults**, and
  **Anti-patterns**; `/ms-panel-tally`'s **Artifact gates** (the HARD-vs-soft `hard_block`
  judgment), **Consolidate** (semantic dedup + criticality ranking), and
  halt-recovery/escape-hatch procedure — are unchanged: the verbs mechanize the
  decision function and the artifact registration, not the human judgment
  layered on top of them.
- **2026-07-09 (spec 111 Bead 3):** The panel operator procedure now selects
  its runner via the spec 109 `runner:` config key (`mindspec config show`),
  read by `/ms-panel-run`'s new **Runner dispatch** section:
  `claude-code-workflow` composes the slot lenses (§ Slot lens defaults, the
  retained judgment step) and invokes the `/ms-panel` workflow (spec 111 Bead 2)
  **once** with the resolved `{slug, spec, target, bead_id?, round, lenses[],
  mix, claude_sub_on_quota}` (the latter resolved from config
  `panel.substitution.claude_sub_on_quota`, spec 109, the same way `mix` is
  resolved from `panel:` — the workflow cannot read config itself and
  fail-closes an omitted flag to `false`), letting the workflow's own
  registration + fan-out + verify/tally-return mechanics stand in for the
  manual **Launch the panel**, **Codex failure detection**, and **Working
  directory matters** sections (those sections are labelled
  `claude-code-skills` path only and superseded, not deleted, for the
  workflow path); `claude-code-skills` retains the hand-driven launch path
  unchanged as the default runner; `external` is a documented out-of-scope stub
  (human/skills-path per ADR-0040 degraded modes). `/ms-panel-tally` gained a
  matching note: on the workflow path the per-slot table and decision arrive
  pre-rendered in the workflow result, so its own job narrows to the Artifact
  gates Allow-screen, consolidation, and the merge terminal. Judgment sections
  in both skills — Slot lens defaults, Consolidate, Artifact gates, After a
  halt — recovery, and Escape hatch — are retained unchanged on both paths.
- **2026-07-18 (spec 119 Bead 5):** `mindspec complete` now emits an
  ADVISORY, non-fatal `WARN bead-scope: ...` (exit code unchanged) when a
  bead's changed files touch a domain OTHER than the domain(s) attributed
  to the bead's own declared `file_paths` scope (Bead 4's
  `work_chunks[].key_file_paths` metadata), while that file's domain is
  still one of the spec's Impacted Domains
  (`internal/complete/bead_scope.go`). It is a pure signal for a
  human/panel to judge — legitimate seams routinely cross a domain
  boundary atomically — never a gate. A bead with no declared `file_paths`
  baseline (a plan without structured `work_chunks`, or a bead created
  outside `plan approve`) is silently skipped — there is nothing to
  compare against. `mindspec plan approve` separately runs a
  double-assignment plan-lint (`internal/approve/plan_lint.go`,
  advisory-only): a single file referenced in TWO OR MORE beads'
  `**Steps**` lists is named in a warning alongside both bead headings —
  the unambiguous case a spec-118 plan panel had to catch by hand. Both
  checks route any path/ID-bearing text through `internal/termsafe`
  (spec 116). Separately, `internal/instruct`'s `setupRunTestProject` test
  fixture is now HERMETIC (a real `git init` + `os.Chdir` into the sandbox
  + `GIT_CEILING_DIRECTORIES`, restored via `t.Cleanup`) so
  `TestRun_IdleNoBeads` no longer reads live lifecycle state when `go
  test` happens to run from inside an active bead worktree
  (mindspec-z4ps).
- **2026-07-24 (spec 125):** The landed-merge attestation substrate is
  rebuilt on exact-second-parent git topology (see this domain's
  architecture doc, § Landed-merge attestation integrity): `complete`
  persists the binding regardless of the merge's subject format, a
  genuine locate miss is loud and cleanup-suppressing, the bead→spec
  conflict-recovery message now supplies `-m "Merge <beadBranch>"`,
  `FindLandedMerge` corroborates by exact equality only (ancestor
  tolerance removed; anonymous-subject merges fail closed), the
  CleanDivergence arm is sub-classified by `gitutil.RevertShape` so
  evolved-but-present content identifies while true reverts refuse, and
  the new `mindspec reattest <bead-id>` verb is the explicit audited
  recovery (troubleshooting entry above).
