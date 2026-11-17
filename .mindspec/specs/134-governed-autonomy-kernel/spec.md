---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 134-governed-autonomy-kernel: Governed Autonomy Kernel

*Code citations: `origin/main` at `371dc360` (v0.19.0), 2026-11-15; the named symbol is authoritative. Normative:
[contracts.md](contracts.md) (§1-§12 operator rulings; §13 readings where a ruling is silent).*

## Goal

Mindspec has one path: the binary decides the next step of a plan-approved spec, and an orchestrator executes it.
`plan approve` enrols every new spec; warnings become refusals. `next-action` prints the next steps; `--apply` makes
kernel beads (type `step`) just in time; `lease` and `submit` claim and close a step with a fenced token and a receipt.
The inert `loop:` block goes. People and agents that do not use mindspec share the bd store with no conflict.

## Background

At `371dc360` no `next-action`, `lease` or `submit` exists. The `Loop` comment (`internal/config/config.go`) calls every
`loop:` key INERT. `panel.one-fixer` and `lifecycle.complete-by-orchestrator` (`internal/rules/rules.yaml`) are
advisory; `panel_presence` defaults to `warn`. `classifyChild` (`internal/phase/derive.go`) counts every `task` or
untyped child as lifecycle work; `bd create` defaults to `task`. `EnsureBeadsConfig` (`internal/bead/config.go`)
replaces a differing `types.custom` under `--force`. Already delivered: `77daf455`, `bf41b40f`, panels, final gates.

## Impacted Domains

- **workflow**: `cmd/**` (new `next-action`, `lease`, `submit`; changed lifecycle and panel verbs),
  `internal/{panel,complete,approve,next,lifecycle,doctor,setup,rules,bead}/**`, `.claude/skills/**`,
  `plugins/mindspec/**`, `README.md`, `project-docs/user/guides/autonomy.md`, `.mindspec/domains/workflow/*.md`.
- **core**: `internal/{config,phase,redact}/**` (delete `loop:`; add `panel.max_rounds`, `panel.lease_ttl`),
  `.mindspec/domains/core/interfaces.md`. **execution**: `internal/executor/**` (finalize allow-set, read only).

## ADR Touchpoints

[ADR-0045](../../adr/ADR-0045-binary-owns-sequence-and-rules.md); [ADR-0023](../../adr/ADR-0023.md) (`step`);
[ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md); [ADR-0038](../../adr/ADR-0038-friction-reporter.md);
[ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md); [ADR-0035](../../adr/ADR-0035-agent-error-contract.md);
[ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md). HC-4 status clause: OQ8.

## Requirements

Every requirement holds contracts §2 and B1-B13 (§3). A bare `§<n>` cites contracts.md.

1. **Read-only reducer.** `next-action [<spec>] [--json]` writes nothing and prints `mindspec.next-action/v1` (§4).
   Failure: two skills pick the next bead by different rules (ADR-0045 Verification; #146, #187).
2. **Just-in-time steps.** `--apply` does one idempotent reduction under the phase lock; it resumes after a crash
   (B5) and refuses a cycle-making link (B7) before any write. Failure: a run stalls (`mindspec-qu9z`).
3. **Lease and submit.** `lease` (§5) refuses a held or blocked step, ancestor or upstream; `submit` (§6) checks
   evidence and writes the receipt before the close. Failure: bd accepts `--claim` on a blocked item; subagents run
   `bd close`; `panel.one-fixer` and `lifecycle.complete-by-orchestrator` are advisory.
4. **Existing verbs are kernel steps.** The §7 verbs are the commands `run` lists; each closes the step it names and
   keeps its refusals. Failure: today `next`, `complete` and the skills each follow their own rules.
5. **Bead gate as child.** Enrolment creates `<w>.review` per work item and `<epic>.final`; a passed gate is frozen
   (B12, §13.2). Failure: nothing deters a raw work-item close (`mindspec-xaoq`).
6. **Enrol at plan approve.** `plan approve` enrols every new spec and pins its policy (§8). Nothing clears it; an
   enrolled spec always enforces the panel gate. Failure: `panel_presence` defaults to `warn`; `loop:` is editable.
7. **Round limit across panels.** A gate counts every registration of its work item or spec (§10, §13.1). Failure: a
   new slug restarts at round 1 (`mindspec-ztax`).
8. **Local decisions and cascades.** A REJECT adds a `fix`/`halt` decision for that gate only (§13.8); `halt` stops
   the spec (B11, §13.4); B10 supersedes (§13.7). Failure: `Load` forces `on_reject: halt`.
9. **Unreceipted closes and audit.** §9 and §11; `--allow-not-ready` joins `escapeHatchFlags` and
   `EscapeHatchTokens`. Failure: closes not persisted (`mindspec-2u0u`); `--allow-not-ready` is in no registry.
10. **Config honesty.** `Load` warns once per removed `loop:` key and ignores it (alias: §10); `config show` labels
    each key truthfully. Failure: inert keys look live; `runner:` is mislabelled (`mindspec-aili`).
11. **Shared bd store.** A person or agent that does not use mindspec works in the same store with bd and git only,
    with no mindspec command or setup, and nothing conflicts (§12). Failure: a `bd create --parent <epic>` child
    counts as lifecycle work (`classifyChild`); bd has no default view filter; the finalizer once merged another
    spec's bead (`77daf455`); risk: a concurrent bd write loses or doubles a step.
12. **Skills, docs and setup.** `ms-bead-cycle` loops `next-action --apply`, `lease`, dispatch, `submit` and each
    `run` entry; skills and docs drop deleted keys and spec-wide halt rows (§13.6); `setup` adds `step` and keeps
    other `types.custom` entries. Failure: skills name default-3 rounds; `setup --force` replaces `types.custom`.

**Constraints.** Consumer-facing text (CLI output, refusals, recovery lines, `next-action` output, skills, setup
files) cites no ADR, spec, requirement, bead or issue number. No new use of the committed `.beads/issues.jsonl`.

## Migration

A spec past `plan approve` when this ships has no witness: it is pre-kernel and finishes on today's rules (warnings;
`panel_presence` applies). Only new plan approvals enrol; there is no version stamp (§8). Without `step`, `plan
approve` refuses and names the setup command.

## Scope

### In Scope
- The plan-approved tree of an enrolled spec; the shared store. After a check: close `mindspec-4xzi`, `mindspec-naq0`,
  and `mindspec-sxjc` if copies match; re-scope `mindspec-qu9z` ("an expired lease frees the step").
### Out of Scope
- Spec-draft and plan-draft steps; the merge as a bead (#250). Authority table, grants DSL, blocking caps on escape
  hatches, per-wake ceilings, `token_budget`, lane kit ledger.
- Rejected: a hand mode or opt-in inside mindspec; enrol on read; bd's lease; unfenced leases; ownership by type;
  version stamp; auto-reopen; round beads; per-directory round counts; `--ephemeral` steps.
- Later: export removal (`mindspec-lzx4`); document steps; parallel waves (`mindspec-m4ks`); a driver; skill merge.

## Acceptance Criteria

Scratch repo and bd database; spec `x` plan-approved by this binary; items 2 and 3 depend on item 1. "Snapshot" =
`bd list --all --json`, every dependency list, `git for-each-ref`, `git status --porcelain`. "Foreign" = by `bd` only.

- [ ] AC1 (R1): `next-action x --json` twice: byte-identical, item 1 `implement` with `"exists":false`; same snapshot.
- [ ] AC2 (R2, R5, R6): `plan approve` creates exactly `<w>.review` per item, `<epic>.final` and `mindspec_run`. The
  first `--apply` creates exactly `<1>.impl.a1`; a second, or a rerun after a kill at its first write, gives the same
  snapshot and `revision`. After item 1 closes, one `--apply` creates exactly `<2>.impl.a1` and `<3>.impl.a1`. A
  foreign blocks link item 1 -> item 2: `--apply` exits non-zero naming it; snapshot unchanged.
- [ ] AC3 (R3, R4): `next 1` prints a token (generation 1); a second `next 1` or `lease <1>.impl.a1` exits non-zero
  naming `lease --take`; after the TTL, `lease` prints generation 2. A foreign bug blocking item 1: `lease
  <1>.review.r1.R1` exits non-zero naming it. `panel submit` without `--lease` exits non-zero; the slot stays open.
- [ ] AC4 (R3, B5, B13): each exits non-zero, step open: `submit` with generation 1 after `--take`, an old subject,
  another slot, a changed blob, a verdict to a fix, an implement commit not past the claim base. A valid `submit`
  writes `mindspec_close` = path, commit, blob id; a rerun exits 0; another artifact exits non-zero, `bd show --json`
  unchanged. Killed between receipt and close, then rerun: exit 0, no flag.
- [ ] AC5 (R5, B12): `bd close <w>` exits non-zero (open child). After a pass, `panel create <w>` exits non-zero. A
  `.mindspec/config.yaml` commit: `complete` exits non-zero and creates `<w>.review.a2` (`mindspec_from` set); after
  its pass, `complete` exits 0. With `r2.fix` or a limit decision open, `panel refute` of the only blocking finding
  closes the gate `passed` and the fix or decision `superseded`.
- [ ] AC6 (R6): with `panel_presence: warn`, `panel.max_rounds: 99`, a step retyped `chore` and `mindspec_run` removed,
  `complete` with no passing panel and `panel create` round 6 exit non-zero; `next-action` flags `run-marker-missing`.
  Spec `y` approved by v0.19.0: `next-action y` exits non-zero, same snapshot; `complete` with no panel warns, exits 0.
- [ ] AC7 (R7): 3 rounds on slug A, `panel abandon` A, 2 on slug B: round 3 of B exits non-zero; `next-action` lists
  one decision (`continue:<n>`, `halt`); `continue:1` allows one more round. The same holds for the final gate.
- [ ] AC8 (R8): REJECT on item 2, `--apply`: a decision under item 2 (text: `halt` stops the spec), an action for item
  3. Before the answer, `lease` on the fix and `panel create 2` exit non-zero; after `--answer fix`, `lease` exits 0.
- [ ] AC9 (R8): a commit on `bead/2` in round 2, no fix open: one pass marks round-2 slots `superseded` and lists
  `panel create 2`; in an open fix, none. Answer `halt`: open steps are `halted`; `next-action x` exits 4; `lease`,
  `complete`, `next 2`, `panel create 2` exit non-zero, snapshot unchanged; a foreign child stays open.
- [ ] AC10 (R9): `bd close <slot>` with no verdict flags `unreceipted-close` and offers `<slot>.a2`; with a valid
  verdict and token, `submit` writes the receipt and no `.a2` exists. `bd close <w>` after a pass flags `raw-close`;
  `bd close --force <w>` flags `forced-close`; `complete` and `impl approve` exit non-zero for each.
- [ ] AC11 (R9): one each of `--allow-not-ready`, `--allow-doc-skew`, `--override-adr`, `--allow-net-deletion`, `lease
  --take`, `panel defer`, a decision answer, an unreceipted close, `MINDSPEC_SKIP_PANEL=1 complete`: `impl approve`
  and a rerun print 1 for each; both registries hold `allow-not-ready`.
- [ ] AC12 (R10): `loop.gate_authority`, `loop.halt.{on_reject: fix, panel_deadlock_rounds,
  max_consecutive_impl_failures}` load with exit 0, one warning each; `config show` labels them ignored and `runner:`
  read by a skill. `loop.halt.max_rounds_per_bead: 4` gives limit 4; with `panel.max_rounds: 6`, 6.
- [ ] AC13 (R11): foreign `bd create --parent <epic> note`, a bug blocking item 2, `bd update <1>.impl.a1 --claim`. A
  snapshot diff around each kernel verb changes only computed ids and links between them. `next-action` lists no
  action for the note and flags `raw-claim` and `foreign-child-open`; `lease <1>.impl.a1` exits 0. `bd ready` titles
  each kernel bead `[mindspec]`; `bd ready --exclude-label mindspec` shows the foreign beads and no kernel bead.
- [ ] AC14 (R11): a foreign `bd create --id <1>.review.r1.R1`: `panel create 1` exits non-zero printing `id-collision`;
  that bead is unchanged. A fake bd answers exit 13 to the first `--if-status` write: `submit` exits 0 with one
  receipt and one close; always 13: non-zero, snapshot unchanged.
- [ ] AC15 (R12): with K = `max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority`,
  `git grep -n -i -E "$K|Any bead REJECT|REJECT / HARD block|do not proceed to the next bead|on_reject|loop\.halt|
  max-rounds|One halted bead|keep going after a halt|default .?3" -- .claude/skills plugins/mindspec/skills
  plugins/mindspec/README.md README.md project-docs/user/guides` and `git grep -n -E "$K|on_reject|type Loop struct"
  -- .mindspec/domains` each exit 1 (`historical_skills` keeps old text). Both `ms-bead-cycle` copies name
  `next-action --apply`, `lease`, `submit`; `cmp` of each skill pair exits 0. With `types.custom: gate,spike2`, `plan
  approve` exits non-zero naming `mindspec setup`; after `setup --force`, it holds `gate,spike2,step`.
- [ ] AC16 (constraints): added non-test Go lines under `cmd/`, `internal/` and skill lines hold no ADR, spec,
  requirement, bead or issue number and do not touch `.beads/issues.jsonl`.
- [ ] AC17 (R1-R4, B8): only `next-action --apply`, `lease`, `submit` and each `run` entry reach `impl approve` exit 0
  and `"status":"done"`. After `submit <1>.impl.a1`, `run` lists `panel create 1`; after it, `--apply` lists slots
  `<1>.review.r1.R<s>`, `ForBead` counts 1 round, and a rerun of `panel create 1` exits non-zero.

## Validation Proofs
The scratch-repo tests for AC1-AC17 each pass and fail on a named mutation; `make test` exits 0.

## Open Questions

- [ ] grill deferred: headless session
- [ ] OQ1: Does `bd create --id` accept word segments? Recommended: probe first; fallback `mindspec_key` (§12).
- [ ] OQ2: lease refuses an "open ancestor or upstream", but every step has one. Recommended: held or blocked (§5).
- [ ] OQ5: B12 freezes a passed gate; `panel rebind` refuses a config change. Recommended: successor gates (§13.2).
- [ ] OQ4, OQ6: no final-gate fix id (`ForBead` skips finals); a halted spec is active. Recommended: §13.1, §13.4.
- [ ] OQ8: exits 3/4/5 conflict with ADR-0035 HC-4; `indeterminate` = 1 equals a refusal. Recommended: amend HC-4.
- [ ] OQ9, OQ11: B8 has no implement outcome; refusals omit today's gates and halt rows. Recommended: §13.3, §13.5-6.
- [ ] OQ10, OQ12: B8, B10 name no next round; only `panel create` writes one. Recommended: `run` entry (§13.7, §13.9).
- [ ] OQ13: B9 gives a refused round a fix, R8 a REJECT a decision. Recommended: both; the decision holds (§13.8).
- [ ] OQ14: bd has no default view filter. Recommended: mark kernel beads; `bd ready --exclude-label mindspec` (§12).
- [ ] OQ15: does `plan approve` enrol every new spec, with no hand mode or opt-in? Recommended: yes (§8).
- [ ] OQ16: do the existing verbs stay as the `run` commands that close their steps? Recommended: yes (§7).
- [ ] OQ17: does a spec past `plan approve` at release finish on today's rules ("no witness")? Recommended: yes (§8).
- [ ] OQ18: does an enrolled spec always enforce the panel gate? Recommended: yes; `panel_presence` is pre-kernel only.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
