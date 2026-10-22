---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 133-document-final-gates: Document Final Gates

*Citation scope: code citations were checked against `main` at `77efdd85` and PR #270 (`origin/spec/132-readiness-followon`) on 2026-10-22. The named symbol is authoritative.*

## Goal

After this spec, the binary enforces a review panel at every lifecycle gate, and `complete` tells an orchestrator its refusals before the bead panel runs.

- `spec approve` and `plan approve` refuse without a passing document panel.
- `impl approve` refuses without a passing final-review panel. A small spec can use its bead panel as the final review.
- `complete` treats a bead with no panel as a finding, not a pass. A config switch gives a warn-first migration.
- A finding that names a missing artifact blocks until the artifact exists.
- A spec-panel finding can move to the plan. `plan approve` checks that the plan resolves it.
- `complete --check` runs the doc and ADR gates with no mutation. The doc gate accepts a recorded "no doc impact" and a doc deferral with an owner bead. A file that no domain owns gives a warning, not a refusal.
- The approve skills say "run the gate verb". They do not tell the agent that the gate is guidance only.

## Background

**Source items.** G1, G2, G3, G5 and G8 (gate backlog); the G4 check; the skill change; GitHub issue #212 asks 1–3; beads `mindspec-0pij`, `mindspec-yesp` (with its notes) and `mindspec-k96x`.

**Defects, each checked on `77efdd85`:**

- **No document gate (G1, #212, 0pij).** `ApproveSpec` (`internal/approve/spec.go`) validates, writes the approval frontmatter, scaffolds `plan.md`, creates the epic and commits. `ApprovePlan` (`internal/approve/plan.go`) does the same for the plan. Neither reads a panel. `panel create --gate spec_approve` records the gate name only. The skills `ms-spec-approve` and `ms-plan-approve` say: "This step is guidance, not a mechanized preflight".
- **No final-review gate (G2).** `ApproveImpl` (`internal/approve/impl.go`) runs `implPanelCore3Preflight` and `implPanelDeferralPreflight` on the panels that exist. No step requires a `final_review` panel.
- **Fail-open without a panel (G3).** `PanelGateDecision` (`internal/panel/gate.go`), leg (1): "no registered panel for the bead" returns `Allow`. ADR-0037 §6 states this rule for solo and harness flows.
- **No artifact field (G5).** The v2 verdict item is `{text, basis, command?}` (`internal/panel/verdict_v2.go`). The validator refuses unknown fields. No finding can name an artifact path.
- **No deferral to the plan (G8).** `panel defer` refuses a `spec_approve` panel: "this panel has no owning epic for a deferral" (`cmd/mindspec/panel_defer.go`). No record kind or `plan.md` section carries a spec finding into the plan.
- **Late doc and ADR refusals (yesp).** `complete` has no check-only mode (`cmd/mindspec/complete.go` flags). In one lane, `complete` refused 6 of 8 beads after the bead panel passed, on `internal-docs` (`internal/validate/docsync.go`) and `adr-divergence-unowned`. Each refusal cost a doc fix, a commit and a panel rebind. `--allow-doc-skew` records a reason on the bead and names no owner.
- **Unowned files refuse (k96x).** `validateDivergence` (`internal/validate/divergence.go`) adds an Error `adr-divergence-unowned` for a file that no `OWNERSHIP.yaml` claims, for example `.mindspec/config.yaml`, `CHANGELOG.md` and `internal/rules/rules.go`. Every case ended in `--override-adr`.

**Ordering constraint (checked before R1).** A document gate is passable only if committing a round's own records does not make the panel stale. This holds on `77efdd85`:

- `Subject` (`internal/panel/subject.go`) sets the subject root of `spec_approve` and `plan_approve` to the spec directory. It excludes `<specs>/*/reviews/**`, `.mindspec/reviews/**`, `review/**` and `.beads/issues.jsonl`.
- `PanelGateDecision` leg (6) blocks only when the reviewed SHA differs from the head and the subject digest differs too.
- `.gitignore` ignores `.mindspec/specs/*/reviews/**` except each panel's `dispositions.jsonl`. Verdict files and `panel.json` are untracked. The tracked ledger is inside the excluded path.
- So a "ledger" commit moves the head but keeps the subject digest. The panel stays fresh.
- The approve verbs write `spec.md` or `plan.md` inside the subject, then commit (`ApproveSpec`, `ApprovePlan`). A gate that runs before these writes reads the reviewed subject. The first run cannot fail on its own commit.
- Three gaps remain, and R1 closes them. (a) A re-run after a failure past the gate sees the verb's own writes as a stale subject. (b) `ResolveGateFacts` (`internal/panel/gate.go`) reads only `bead/<id>` as the head. (c) The dirty-tree leg (7) reads the bead worktree, but `ApproveSpec` commits all dirt in the spec worktree.

**Already delivered (not re-specified):**

- **G4 check** — `d15787fd` (PR #264). `complete` (`checkPanelCore3`, `internal/complete/panel_core3.go`) and `impl approve` (`implPanelCore3Preflight`) call `panel.CheckCore3`. It ends in `CheckCompletenessAt`, the floor that `panel disposition check` runs.
- **#212 ask 1, review half** — `6cdb3b72` (PR #223) added the document-panel step to the approve skills. `77efdd85` (PR #271) ships the document lenses and acceptance questions for `spec_approve` and `plan_approve` as config data (`internal/config/lenses.go`). R1 adds the gating half.
- **Durable ledger** — `d15787fd` and `77efdd85`: `dispositions.jsonl` is tracked, and it holds the coverage manifest and the `verdict` records.

**PR #270 compatibility.** PR #270 changes `complete` (landing-tree ownership, `--merge-only`), `impl approve` (early guards, re-run convergence) and `doctor`. This spec builds on it and does not change those rules. R2 is a pre-close gate under its re-run rule. R7 and R10 read ownership at its landing tree.

## Impacted Domains

- **workflow**:
  - `internal/approve/**`: document gate (R1), final-review gate and doc-deferral check (R2, R9), resolution check (R6).
  - `internal/panel/**`: gate head and dirt for documents (R1), presence policy (R3), `artifact_path` (R5), plan deferral record (R6).
  - `internal/complete/**`: presence policy (R3), `--check` (R7), doc declarations (R8, R9).
  - `internal/validate/**`: doc-sync declarations (R8, R9), unowned warning (R10).
  - `internal/instruct/**`, `internal/doctor/**`: cues and policy checks (R4, R11).
  - `cmd/mindspec/**`: new flags on `complete` and `panel defer`.
  - `internal/setup/**`, `.claude/skills/**`, `.agents/skills/**`, `plugins/mindspec/skills/**`: skill text (R11).
- **core**: `internal/config/**` (`enforcement.panel_presence`, `panel.exempt_bead_types`, `panel.gates.final_review.skip_when_beads_le`).

Documentation surfaces: ADR amendments (R12); `.mindspec/domains/workflow/interfaces.md`; `project-docs/user/guides/review-panels.md`; `CHANGELOG.md`.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED: §6 "fail-open without a panel" becomes the presence policy (R3, R4). The gate covers documents and the final review (R1, R2). §7 hatches apply unchanged.
- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED: the three approve verbs run the panel gate before their first write (R1, R2).
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — APPLIED: the document-panel precondition moves from skill text to a binary gate (R1, R11).
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED: a plan-deferral record (R6).
- [ADR-0031](../../adr/ADR-0031-doc-sync-gate.md) — AMENDED: no-doc-impact declaration and doc-deferral owner (R8, R9).
- [ADR-0032](../../adr/ADR-0032-adr-semantic-gates.md) — AMENDED: a file that no domain owns warns (R10).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.

## Requirements

**Terms.**
- *Document gate*: the `spec_approve` or `plan_approve` panel gate.
- *Gate decision*: the result of `PanelGateDecision` for one registration.
- *Live registration*: a registration for the gate and spec that is not abandoned.
- *Presence policy*: the value of `enforcement.panel_presence` (R4).

### R1 — Document panel gate (G1, #212 ask 1, mindspec-0pij)

1. `spec approve <id>` runs the `spec_approve` gate. `plan approve <id>` runs the `plan_approve` gate. Each gate runs before the first write of the verb: frontmatter, `plan.md` scaffold, epic, bead creation and commit.
2. The gate passes only when the spec has at least one live registration for that gate, and the gate decision of each live registration is Allow. It uses the same decision legs as `complete`: malformed registration, integrity, round mismatch, stale subject, incomplete, REJECT or hard block, and unresolved finding.
3. For a document gate, the head is the spec branch tip. The subject is the spec directory, with the `Subject` exclusions.
4. The gate refuses when the spec worktree has an uncommitted change inside the subject. The message lists each path.
5. With no live registration, the presence policy decides (R4). An abandoned registration counts as no registration.
6. A re-run of the verb, after a failure that came after a passing gate, does not refuse because of the verb's own writes.
7. The hatches of `complete` apply: the skip variable and `enforcement.panel_gate: false`. Each use records reason class, actor and time on the epic. The refusal text does not name the skip variable.
8. A refusal names the gate, the panel and the failing leg. It ends with one `recovery:` line. With no registration, the line is `mindspec panel create <slug> --spec <id> --gate <gate> --target <ref>`.

### R2 — Final-review gate (G2)

1. `impl approve <id>` runs the `final_review` gate before the phase-reconcile write and before the first epic mutation. The rules of R1.2, R1.5, R1.7 and R1.8 apply. The head is the spec branch tip. The subject is the whole tree, with the `Subject` exclusions.
2. Config key `panel.gates.final_review.skip_when_beads_le` is a non-negative integer. The default is 1. When the plan has that number of beads or fewer, the gate needs no `final_review` panel. Instead, each plan bead needs a passing bead panel recorded at its landing. `impl approve` prints one line that says the bead panel stands as the final review.
3. A bead that landed with no panel (R3 warn or exempt) does not count for R2.2. The spec then needs a `final_review` panel.
4. The gate is a pre-close gate under PR #270's re-run rule. The export commit changes only `.beads/issues.jsonl`, which the subject excludes.

### R3 — Bead panel presence (G3)

1. When `complete <bead>` finds no live registration for the bead, the presence policy decides (R4). The policy replaces leg (1) "no panel → Allow" of `PanelGateDecision`.
2. Config key `panel.exempt_bead_types` is a list of bead issue types. The default is empty. A bead whose type is on the list passes with one Info line and no record.
3. The refutation-obligation branch for a bead with no panel stays unchanged.

### R4 — Presence policy and migration

1. Config key `enforcement.panel_presence` is `warn` or `enforce`. The default is `warn`. Config load refuses any other value and names the key.
2. Under `warn`, a gate with no live registration prints one `warning:` line with the gate name and the R1.8 recovery command. The verb continues and exits as it does today. The verb records `panel_absent` with the gate and time on the epic (R1, R2) or on the bead (R3).
3. Under `enforce`, a gate with no live registration refuses before the first write.
4. A spec that passed a gate before this change ships is not checked again at that gate. `spec approve` and `plan approve` run one time for each spec.
5. `doctor` prints one Info line when the policy is `warn`. The line says that gates without a panel only warn and names the key. `doctor` prints one Warn line for each in-flight spec in Plan or Implement mode whose epic has a `panel_absent` record.
6. This repository's `.mindspec/config.yaml` sets `enforcement.panel_presence: enforce`.

### R5 — Artifact hard block (G5)

1. A v2 verdict item can have an optional `artifact_path`. The value is a repository-relative slash path. It is not absolute, has no `..` segment and no control byte. The validator refuses other values with the JSON pointer of the field. The JSON Schema that `panel schema` prints includes the field.
2. The gate blocks while an item of a latest submission in the latest round has an `artifact_path` that is absent from the tree at `reviewed_head_sha`. The message names the slot, the item and the path.
3. A refutation or a deferral does not discharge that block. A new round at a tree that contains the path discharges it.
4. A verdict with no `artifact_path` gives the same result as today.

### R6 — Spec findings deferred to the plan (G8)

1. `panel defer <slug> --slot S --round N --finding I --to-plan --by <actor>` works on a `spec_approve` panel only. It appends one `deferred_to_plan` record with a stable deferral id, the slot, round, finding hash, actor and time. Other gates refuse `--to-plan`.
2. A `deferred_to_plan` record discharges its finding for the `spec_approve` gate, with the same finding-hash rule as a refutation.
3. `plan approve` refuses unless `plan.md` has a `## Resolutions` section with one entry that names each deferral id of the spec's live `spec_approve` registrations. The refusal lists each missing id. This check runs before the first write and does not depend on the presence policy.
4. `panel defer` without `--to-plan` on a `spec_approve` panel keeps today's refusal.

### R7 — `complete --check` (mindspec-yesp)

1. `mindspec complete <bead> --check` runs the doc-sync gate, the ADR gates and the work-destruction preflight on the tree that `complete` would land. It uses the PR #270 landing tree for ownership.
2. It does not run the panel gate or the coverage check.
3. It changes nothing: no commit, no index change, no bead status or metadata change, no branch or worktree change.
4. It reports every failing gate, not only the first, with each recovery line. It exits 0 when all pass and 1 when one fails.
5. For the same tree, the set of gates that `--check` reports as failing equals the set that plain `complete` refuses on.
6. `--check` refuses `--merge-only`, `--resolve-merge` and the override flags.

### R8 — No-doc-impact declaration (mindspec-yesp)

1. `complete <bead> --no-doc-impact "<domain>: <reason>"` can repeat. Each value satisfies the doc-sync gate for that domain on this bead. The reason must not be empty. A domain that the bead's change does not touch refuses.
2. `complete` records domain, reason, actor and time on the bead.
3. The branch-wide doc-sync gate of `impl approve` accepts a domain when the branch changes its docs, or when every bead that changed its sources recorded no doc impact for it.
4. `--check` accepts `--no-doc-impact`, because it is a declaration and not an override.

### R9 — Doc-deferral owner (mindspec-yesp notes)

1. `--allow-doc-skew "<reason>"` needs `--doc-owner <bead>`. The owner is an open bead in the same epic, and not the bead being completed. Other values refuse.
2. `complete` records the deferred domains, owner, reason, actor and time on both beads.
3. `complete` of the owner bead refuses unless its landed change updates the docs of each deferred domain.
4. `impl approve` lists each open deferral and refuses while one is open. A deferral is open until its owner bead lands with the doc change.
5. A skew record with no owner, from before this change, stays under today's branch-wide doc-sync check.

### R10 — Unowned file warning (mindspec-k96x)

1. A changed file that no `OWNERSHIP.yaml` claims, in any domain, gives a Warn `adr-divergence-unowned`. `complete` and `impl approve` continue. The warning names the file and the domains root.
2. A file that a domain outside the spec's Impacted Domains claims stays an Error.
3. `adr-divergence-attribute` and `adr-divergence-uncovered` stay Errors.

### R11 — Skills and guidance (skill change, #212 asks 2–3)

1. `ms-spec-approve` and `ms-plan-approve` state: run the panel, then run the approve verb; the verb refuses without a passing panel. The sentence "This step is guidance, not a mechanized preflight" and the text that it qualifies are removed from every installed and source copy. The existing install refresh replaces old copies.
2. `ms-impl-approve` and `ms-spec-final-review` state that `impl approve` refuses without a passing final review, and state the small-spec rule of R2.2.
3. `instruct` in Spec mode and Plan mode renders one line: approval needs a passing document panel, with the `panel create` command for the gate.
4. The approve skills state when to use reviewers from more than one family: for a spec that changes a gate, a schema, a trust boundary or a merge path. `config resolve --gate <gate>` shows the families.
5. Text that this spec adds to a CLI message, a recovery line, a skill or `instruct` contains no ADR number, spec number, requirement id, bead id or issue number.

### R12 — ADR text

1. ADR-0037, ADR-0041, ADR-0043, ADR-0031 and ADR-0032 get amendment sections that state the rules of R1–R10.

## Scope

### In Scope
- R1–R12 in the packages and documents in Impacted Domains.
- This repository's `enforcement.panel_presence: enforce` (R4.6).

### Out of Scope
- PR #270's changes to `complete`, `impl approve` and `doctor`. This spec builds on them.
- The round limit and other panel-execution items that spec 131 left open.

## Non-Goals

- Rebuilding a panel from the tracked ledger in a checkout where it did not run. A gate reads the registration in the checkout where the verb runs.
- Changing the gate authority of the approve verbs. A human still approves. The panel is a precondition.
- Proof that the actor of an override is a human. The hatch model of ADR-0037 §7 stays.
- A required second model family.

## Acceptance Criteria

- [ ] AC1 (R1.1, R1.2): In a fixture spec with no `spec_approve` panel and `panel_presence: enforce`, `mindspec spec approve <id>` exits non-zero. `git rev-parse HEAD`, `spec.md` and the absence of `plan.md` and of an epic are unchanged.
- [ ] AC2 (R1.2): For each of incomplete, REJECT, unresolved finding and malformed `panel.json` on a `spec_approve` panel, `spec approve` exits non-zero and the message names that leg. With a passing panel, it exits 0.
- [ ] AC3 (R1.3, ordering): After a passing `spec_approve` panel, a commit that changes only `reviews/<slug>/dispositions.jsonl` leaves `spec approve` at exit 0. A commit that changes `spec.md` makes it exit non-zero with a stale-subject message.
- [ ] AC4 (R1.4): An uncommitted edit to `spec.md` makes `spec approve` exit non-zero and name `spec.md`. No commit is made.
- [ ] AC5 (R1.6): A fault injected after the approval commit of `spec approve`, then a re-run, gives exit 0 and no stale-subject refusal.
- [ ] AC6 (R1.1, R1.2): The AC1 and AC2 cases pass for `plan approve` with a `plan_approve` panel. A refusal creates no bead.
- [ ] AC7 (R1.7): With the skip variable set, `spec approve` with no panel exits 0, and the epic has the skip record. No refusal message contains the variable name.
- [ ] AC8 (R2.1): With 2 plan beads and no `final_review` panel under `enforce`, `impl approve` exits non-zero. The epic stays open and the phase is unchanged.
- [ ] AC9 (R2.2, R2.3): With 1 plan bead that landed with a passing bead panel, `impl approve` passes the gate with no `final_review` panel and prints the stand-in line. With 1 plan bead that landed with no panel, it refuses.
- [ ] AC10 (R3.1, R4.3): Under `enforce`, `complete <bead>` with no panel exits non-zero before the close. The bead stays `in_progress`.
- [ ] AC11 (R3.2): Under `enforce`, a bead whose type is in `panel.exempt_bead_types` completes with exit 0 and one Info line.
- [ ] AC12 (R4.1, R4.2): Under `warn`, AC1, AC8 and AC10 exit 0. Each prints one `warning:` line and records `panel_absent`. A config value `panel_presence: maybe` makes every command refuse at load and name the key.
- [ ] AC13 (R4.5): `mindspec doctor` prints the Info line under `warn`, and one Warn line for a fixture spec in Plan mode whose epic has `panel_absent`.
- [ ] AC14 (R5.1): `panel submit` refuses an item with `artifact_path` `../x`, `/x` or a control byte, and names the JSON pointer. `panel schema` output contains `artifact_path`.
- [ ] AC15 (R5.2, R5.3): A passing panel plus one item with an `artifact_path` absent at `reviewed_head_sha` makes `complete` exit non-zero and name the path. A refutation of that item does not change the result. A new round at a tree with the path, with all findings discharged, passes.
- [ ] AC16 (R6.1, R6.2): `panel defer --to-plan` on a `spec_approve` panel appends one `deferred_to_plan` record, and `spec approve` then passes. On a `bead` panel, `--to-plan` exits non-zero.
- [ ] AC17 (R6.3): `plan approve` exits non-zero and names each deferral id that `## Resolutions` does not name. With every id named, it passes.
- [ ] AC18 (R7.1–R7.4): On a fixture bead with a doc-sync failure and an ADR failure, `complete <bead> --check` exits 1 and names both. `git status --porcelain`, `git rev-parse HEAD`, the bead status and the bead metadata are byte-equal before and after.
- [ ] AC19 (R7.5): For four fixture trees (pass, doc-sync fail, ADR fail, both), the failing-gate set of `--check` equals the refusal set of plain `complete`.
- [ ] AC20 (R8.1–R8.3): `complete --no-doc-impact "workflow: test-only change"` passes the doc-sync gate for `workflow` and records the declaration. An empty reason and an untouched domain each exit non-zero. `impl approve` accepts that domain at the branch-wide check.
- [ ] AC21 (R9.1–R9.4): `--allow-doc-skew` with no `--doc-owner`, or with a closed owner, exits non-zero. With a valid owner, `impl approve` refuses and lists the deferral until the owner lands with a doc change in the domain. Then it passes.
- [ ] AC22 (R10.1, R10.2): A bead that changes `CHANGELOG.md` with no owning manifest completes with exit 0 and one `adr-divergence-unowned` warning. A file that an undeclared domain owns still refuses.
- [ ] AC23 (R11.1, R11.5): A search of the installed and source skill copies for "guidance, not a mechanized" finds no match. The surface-reference test passes on every string that this spec adds.
- [ ] AC24 (R11.3): `mindspec instruct` in Spec mode and in Plan mode prints the panel line with the gate name for that mode.
- [ ] AC25: `make test` (or the repository's CI test command) passes on the spec branch.

## Validation Proofs

- `go test ./internal/approve/... -run 'DocumentGate|FinalReviewGate'`: AC1–AC9 pass.
- `go test ./internal/panel/... -run 'Presence|ArtifactPath|DeferToPlan'`: AC10–AC17 pass.
- `go test ./internal/complete/... -run 'CompleteCheck|NoDocImpact|DocOwner'`: AC18–AC21 pass.
- `go test ./internal/validate/... -run Unowned`: AC22 passes.
- `rg -n 'guidance, not a mechanized' internal/setup .claude/skills .agents/skills plugins`: exit 1, no match.
- `mindspec doctor` in the AC13 fixture: the Info line and the Warn line are printed.
- `mindspec instruct` in a Spec-mode fixture: the panel line is printed.
- `make test`: exit 0.

## Open Questions

- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval.
- [ ] OQ1: Default of `enforcement.panel_presence` for other repositories. Recommended: `warn` in the release that ships this spec, and `enforce` one minor release later, with a changelog note in both releases.
- [ ] OQ2: Can a refutation discharge an artifact block (R5.3)? Recommended: no. An absent artifact is a fact at a tree, and only a new tree can change it.
- [ ] OQ3: Does `artifact_path` stay in verdict schema 2, or start schema 3? Recommended: an optional field in schema 2. A verdict without it stays valid. A released binary refuses a verdict with it, so panels in this repository use the field only after the spec merges.
- [ ] OQ4: Does `panel.exempt_bead_types` ship empty, or with `chore`? Recommended: empty. An operator adds types on purpose.
- [ ] OQ5: Does the R2.2 small-spec default stay at 1? Recommended: yes, as the operator ruled.
- [ ] OQ6: Does a file that an undeclared domain owns stay an Error (R10.2)? Recommended: yes. It is scope drift with a named owner, and the message gives the fix.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
