# Bead 10 implementation prompt — 128-lifecycle-verb-state-honesty

## Identity
- Bead id: mindspec-ta30.10
- Branch: bead/mindspec-ta30.10
- Worktree: /Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.10
- Spec branch: spec/128-lifecycle-verb-state-honesty (forked from the FULL
  spec tip — all nine code beads are beneath you; this is the last bead)

## Read first
- `.mindspec/specs/128-lifecycle-verb-state-honesty/spec.md` — §R7, §R4.3,
  AC-12, AC-13 (quoted below).
- `.mindspec/specs/128-lifecycle-verb-state-honesty/plan.md` — §Bead 10
  (quoted verbatim below).
- `project-docs/user/guides/merge-safety.md` and
  `project-docs/user/guides/lifecycle-modes.md` in THIS worktree — the
  surfaces step 2 updates; the latter is CELL-PINNED (see constraints).
- The delivered ADR amendments (ADR-0041, ADR-0037 note, ADR-0025,
  ADR-0023) — landed IN their beads; this sweep must not duplicate or
  contradict them.

## Files in scope (from plan §Bead 10, quoted verbatim)
1. "`internal/instruct/templates/idle.md` + `review.md`: the R7 guidance —
   new implementation work (including implementing a frozen contract)
   goes through a new spec; the externally-merged predecessor's exit is
   named by its real invocation `mindspec impl adopt <spec-id> --reason
   \"<why>\"`; template test pins the command name against the registered
   cobra command (AC-12's rename-is-red clause)."
2. "`project-docs/user/guides/merge-safety.md`: R4.3's operator-visible
   updates for the bead-6 reorder, scoped to the GATE-REFUSAL class (a
   gate-refused complete leaves the tip at the reviewed SHA; plain re-run
   + the original gate's override is the whole recovery; POST-gate
   failures — merge conflicts, close failures — keep the 127
   preserved-merge/forward-reconcile guidance unchanged, and the doc says
   so); `lifecycle-modes.md` cross-links."
3. "Doc-sync gate coverage over every touched source; final whole-tree
   `go test ./... -short` as the AC-13 method gate."

## THE DOC-SKEW DEBT LEDGER (enumerated live in this worktree,
2026-09-03, by running the gate's own validation over the whole branch:
`mindspec validate docs --diff ae45fd7a8697b8790894412f233c977ca5e0f11a`
— base = merge-base(main, spec tip ac07dc32))
Every one of the eight bead completes ran `--allow-doc-skew "docs owned
by bead 10's sweep"`. This bead PAYS that debt. The validation returns
THREE ERROR legs — internal-docs per domain, each listing changed sources
with zero doc updates under `.mindspec/domains/<domain>/`:
- **core**: internal/phase/{derive,migrate,order,phaselock,
  phaselock_unix,phaselock_windows,phaselock_windows_errors,
  sanctioned_sync,semantics_v2}.go, internal/state/state.go — the lock,
  comparator, sanctioned sync, v2 reconciliation, draft-mode derivation.
- **execution**: internal/bead/bdcli.go, internal/executor/{executor,
  mindspec_executor,mock}.go, internal/gitutil/{gitignore,gitops,
  materialize}.go, internal/harness/{sandbox,scenario_bead_lifecycle,
  scenario_contract_hardening}.go — the stderr passthrough, the seven
  seam methods, the snapshot/marker machinery.
- **workflow**: cmd/mindspec/{complete,impl,instruct_tail,next,release,
  repair,root,state}.go, internal/approve/{adopt,impl,plan,spec}.go,
  internal/bootstrap/bootstrap.go, internal/complete/{complete,
  materialize,panel_advisory}.go, internal/guard/registries.go,
  internal/hook/{dispatch,hook}.go, internal/instruct/{instruct,run}.go,
  internal/next/{beads,guard,ready_gate,select}.go,
  internal/panel/gate.go, internal/resolve/{resolve,target}.go,
  internal/validate/{adr_divergence,divergence,docsync}.go — the verb
  honesty, guard, selection, and gate-reorder surfaces.
The fix: update the domain docs (`architecture.md` / `interfaces.md` /
`overview.md` / `runbook.md` under each domain — follow each file's
existing structure and register) to cover what this spec CHANGED in each
domain — real content documenting the landed behavior, never a
token-touch to green the gate (the doc-sync gate checks for updates, but
the PANEL checks the updates are true).
**EXIT CRITERION (the honest gate — put it in your verification)**:
re-running `mindspec validate docs --diff
ae45fd7a8697b8790894412f233c977ca5e0f11a` from this worktree returns
ZERO ERROR legs — the branch can complete WITHOUT `--allow-doc-skew`.
(The fourth finding — WARN missing-source-globs — is PRE-EXISTING on
main, not this spec's debt: out of scope, leave it.)

## The OWNERSHIP debt (tally-carried from bead 1's --override-adr)
Bead 1's `.gitignore` edit (the `.mindspec/phase.lock` entry) tripped the
ADR-divergence unowned-path gate and completed under `--override-adr`.
This bead lands the OWNERSHIP.yaml claim for `.gitignore`: read bead 1's
recorded override reason (`bd show mindspec-ta30.1` metadata) to confirm
the intended owning domain; default disposition is the domain whose
machinery motivated the entry (core owns `internal/phase/**` and the lock
whose path the entry ignores). Note the chosen domain + rationale in
Deviations. (The plan's §Bead 10 text does not name this item — it is
tally-carried debt with the force of a step; treat it as step 4.)

## Spec ACs this bead claims (quoted verbatim)
- "**AC-12 (R7)**: Idle and review instruct output contain the
  new-spec-for-frozen-contract guidance and the literal invocation
  `mindspec impl adopt` with `--reason`; a template test pins the command
  name against the registered cobra command so a future rename is red."
- "**AC-13 (method)**: All new/changed tests under `internal/harness` and
  `internal/instruct` run under `-short` discipline (settled fact 3);
  `go test ./... -short` green."

## Landed-surface constraints (verified)
- `lifecycle-modes.md` is CELL-PINNED by bead 4's
  `agreement_table_test.go` (structural cell pins) — prose edits and
  cross-links must keep the pins green; changing a pinned cell requires
  changing the fixture in the same commit, and nothing in this bead's
  scope should need that (you are adding prose/links, not changing
  modes). If a pin goes red, stop and re-check rather than editing the
  fixture.
- Bead 5 reworded FormatResult's messages and delivered the
  indeterminate template; bead 7 delivered the ignored-dirt notice; your
  guide text describes THOSE landed strings — quote from the code, not
  from the plan's older shorthand.
- The ADR-0041 §1/§3 amendment, the ADR-0037 non-amendment posture, the
  ADR-0025 clarifying amendment, and ADR-0023's draft-exception + row
  strikes are ALL delivered in-bead. The sweep may cross-link them but
  must not restate their normative content (drift risk) or contradict
  them.
- The idle.md/review.md templates were touched by bead 5 (indeterminate
  work) — your R7 additions graft onto the CURRENT template text.

## Hazards
- internal/lint ratchets: doc/template edits can trip the template
  census and doc-shape lint (the ratchet universe walks these files) —
  run the lint suite early (`go test ./internal/lint/... -short`), not
  only at the end.
- Error-discarding discipline applies to any helper you touch; the
  template test pins the adopt command name against the REGISTERED cobra
  command (read it from the command tree, not a string constant twice).
- Hermeticity: template tests via the embedded/rendered surfaces;
  fixture roots in `t.TempDir()`.
- Both suites are part of the gate: full `go test ./... -short`, the
  bead-scope packages (`internal/instruct`, `internal/lint`,
  `cmd/mindspec`), AND the no-short `cmd/mindspec` suite with
  `-timeout 30m` — report all result lines.
- Interim statuses on long runs; scratch ONLY under
  /private/tmp/claude-503/-Users-Max-replit-mindspec/da570e49-a6d2-4faa-a36d-9cb5b5386c7a/scratchpad/
  and cleaned when done.

## Guardrails
AGENTS.md § Bead-loop guardrails applies in full: no `mindspec complete`,
no `git push`, no raw `git merge`, no scope creep beyond the files above
+ the domain docs the ledger names + the OWNERSHIP claim, exactly ONE
commit whose message ends with `Deviations: <list or "none">`, and tests
must PASS. Absolute paths. Work exclusively inside the bead worktree
listed under Identity.

## Expected commit shape
```
impl(mindspec-ta30.10): frozen-contract guidance + docs sweep + method gate

- idle.md/review.md: R7 new-spec-for-frozen-contract + impl adopt naming;
  cobra-pinned template test
- merge-safety.md: gate-refusal recovery (bead-6 reorder); lifecycle-modes
  cross-links
- domain docs: core/execution/workflow updated for the spec's changes
  (pays the eight --allow-doc-skew debts; validate docs = zero errors)
- OWNERSHIP: .gitignore claimed (bead-1 override debt)

Deviations: <list any prompt items you couldn't address literally, or "none">
```

## Report back with
- Commit SHA on `bead/mindspec-ta30.10`.
- The `mindspec validate docs --diff ae45fd7a…` output AFTER your sweep
  (must be zero ERROR legs) plus all suite result lines (full `-short`,
  bead-scope, no-short `cmd/mindspec` `-timeout 30m`).
- Which domain got the `.gitignore` OWNERSHIP claim and why.
- Flagged deviations (one line each + reason).
