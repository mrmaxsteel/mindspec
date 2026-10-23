---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 133-document-final-gates: Document Final Gates

*Citation scope: code citations were checked against `main` at `37c74d67` (v0.18.0) on 2026-10-23. The named symbol is authoritative.*

## Goal

After this spec, the binary enforces a review panel at every lifecycle gate, and `complete` tells an orchestrator its refusals before the bead panel runs.

- `spec approve` and `plan approve` refuse without a passing document panel.
- `impl approve` refuses without a passing final-review panel.
- `complete` treats a bead with no panel as a finding, not a pass. A config switch selects `warn` (the default) or `enforce`.
- A finding that names a missing artifact blocks until the artifact exists or the finding is discharged.
- A spec-panel finding can move to the plan. `plan approve` checks that the plan resolves it.
- `complete --check` runs the doc and ADR gates with no mutation, and the bead cycle runs it before the bead panel. The doc gate accepts a recorded "no doc impact" and a doc deferral with an owner bead. A file that no domain owns gives a warning, not a refusal.
- The approve skills say "run the gate verb". They do not tell the agent that the gate is guidance only.

## Background

**Source items.** G1, G2, G3, G5 and G8 (gate backlog); the G4 check; the skill change; GitHub issue #212 asks 1–3; beads `mindspec-0pij`, `mindspec-yesp` (with its notes) and `mindspec-k96x`.

**Base.** `main` at `37c74d67` holds spec 132 (PR #270). Its rules for `complete` (landing tree, `--merge-only`, the path order of spec 132 R3.0, the closing path), `impl approve` (early guards, re-run convergence, `FinalizeEpic`), `spec approve` and `plan approve` (`FindEpicBySpecID` before the frontmatter write) and `doctor` are the current behavior. This spec adds gates to that code. It does not change those rules except where a requirement says so.

**Defects, each checked on `37c74d67`:**

- **No document gate (G1, #212, 0pij).** `ApproveSpec` (`internal/approve/spec.go`) validates, writes the approval frontmatter and the `## Approval` section, creates the epic, scaffolds `plan.md`, commits, and then commits again ("chore: sync beads state after spec approval") and checks that the tree is clean. A failed epic creation is a warning, and the verb continues. `ApprovePlan` (`internal/approve/plan.go`) writes the plan frontmatter, creates beads and commits. Neither reads a panel. `panel create --gate spec_approve` records the gate name only. The skills `ms-spec-approve` and `ms-plan-approve` say: "This step is guidance, not a mechanized preflight".
- **No final-review gate (G2).** `ApproveImpl` (`internal/approve/impl.go`) runs `implPanelCore3Preflight` and `implPanelDeferralPreflight` on the panels that exist. No step requires a `final_review` panel.
- **Fail-open without a panel (G3).** `panelGate` (`internal/complete/panel_advisory.go`) returns no decision when no registration names the bead, and the bead completes. ADR-0037 §6 states this rule for solo and harness flows.
- **No artifact field (G5).** The v2 verdict item is `{text, basis, command?}` (`internal/panel/verdict_v2.go`). The validator refuses unknown fields. No finding can name an artifact path.
- **No deferral to the plan (G8).** `panel defer` refuses a `spec_approve` panel: "this panel has no owning epic for a deferral" (`cmd/mindspec/panel_defer.go`). A deferral writes a `deferral` record that needs a `bead`, and a `disposition` row `confirmed-deferred` with the bead as evidence. No record kind or `plan.md` section carries a spec finding into the plan.
- **Late doc and ADR refusals (yesp).** `complete` has no check-only mode (`cmd/mindspec/complete.go` flags). Plain `complete` returns on the first doc-sync failure before the ADR gate runs (`runMaterialization`, `internal/complete/materialize.go`). In one lane, `complete` refused 6 of 8 beads after the bead panel passed, on `internal-docs` (`internal/validate/docsync.go`) and `adr-divergence-unowned`. Each refusal cost a doc fix, a commit and a panel rebind. `--allow-doc-skew` records a reason on the bead and names no owner.
- **Unowned files refuse (k96x).** `validateDivergence` (`internal/validate/divergence.go`) adds an Error `adr-divergence-unowned` for a file that no `OWNERSHIP.yaml` claims. On `37c74d67`, `CHANGELOG.md` is such a file. The source lanes also hit `.mindspec/config.yaml` and `internal/rules/rules.go`; on `37c74d67` the `core` and `workflow` manifests claim them. Every case ended in `--override-adr`.

**Ordering constraint (checked before R1).** A document gate is passable only if committing a round's own records does not make the panel stale. This holds on `37c74d67`:

- `Subject` (`internal/panel/subject.go`) sets the subject root of `spec_approve` and `plan_approve` to the spec directory. It excludes `<specs>/*/reviews/**`, `.mindspec/reviews/**`, `review/**` and `.beads/issues.jsonl`.
- `PanelGateDecision` (`internal/panel/gate.go`) leg (6) blocks only when the reviewed SHA differs from the head and the subject digest differs too.
- `.gitignore` ignores `.mindspec/specs/*/reviews/**` except each panel's `dispositions.jsonl`. The tracked ledger is inside the excluded path. So a ledger commit moves the head but keeps the subject digest.
- Gaps that R1 and R2 close: (a) the approve verbs write inside the subject and commit, so a re-run after a fault sees the verb's own writes as a stale subject or as dirt; (b) `ResolveGateFacts` reads only `bead/<id>` as the head; (c) the dirty-tree leg (7) reads the bead worktree, but `ApproveSpec` and `ApprovePlan` commit all dirt in the spec worktree (`CommitAll`), and `FinalizeEpic` commits it as "chore: commit remaining spec artifacts".

**Already delivered (not re-specified):**

- **G4 check** — `d15787fd` (PR #264). `complete` (`checkPanelCore3`, `internal/complete/panel_core3.go`) and `impl approve` (`implPanelCore3Preflight`) call `panel.CheckCore3`. It ends in `CheckCompletenessAt`, the floor that `panel disposition check` runs.
- **#212 ask 1, review half** — `6cdb3b72` (PR #223) added the document-panel step to the approve skills. `77efdd85` (PR #271) ships the document lenses `falsifiability`, `scope-honesty`, `contradiction` and `feasibility`, and the acceptance questions, for `spec_approve` and `plan_approve` as config data (`internal/config/lenses.go`). R1 adds the gating half. The two other lenses that #212 asks for (completeness against the sources, adversarial sufficiency) are out of scope.
- **Durable ledger** — `d15787fd` and `77efdd85`: `dispositions.jsonl` is tracked, and it holds the coverage manifest and the `verdict` records.
- **Landed merge** — `complete` writes `mindspec_landed_merge_sha` on each bead that it lands (`closeMergedBead`, `internal/complete/closing.go`, PR #270). R8.3 reads it.

## Impacted Domains

- **workflow**:
  - `internal/approve/**`: document gate (R1), final-review gate and doc-deferral check (R2, R9), resolution check (R6).
  - `internal/panel/**`: gate head, subject and dirt for documents (R1), presence policy (R3), `artifact_path` (R5), plan deferral record (R6), gate message text (R1.9).
  - `internal/complete/**`: presence policy (R3), `--check` and the report of every content gate (R7), doc declarations (R8, R9).
  - `internal/executor/**`: finalize order for the final-review gate (R2).
  - `internal/validate/**`: doc-sync declarations (R8, R9), unowned warning (R10).
  - `internal/instruct/**`, `internal/doctor/**`: cues and policy checks (R4, R11).
  - `cmd/mindspec/**`: new flags on `complete` and `panel defer`; target check on `panel create`.
  - `internal/setup/**`, `.claude/skills/**`, `.agents/skills/**`, `plugins/mindspec/skills/**`: skill text and historical snapshots (R11).
- **core**: `internal/config/**` (`enforcement.panel_presence`).

Documentation surfaces: ADR amendments (R12); `.mindspec/domains/workflow/interfaces.md`; `project-docs/user/guides/review-panels.md`; `CHANGELOG.md`.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md) — AMENDED: §6 "fail-open without a panel" becomes the presence policy (R3, R4). The gate covers documents and the final review (R1, R2). The §7 hatches keep their behavior and audit. The skip-variable audit also goes on the epic for the document and final-review gates (R1.7).
- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED: the three approve verbs run the panel gate before their first write. A re-run does not refuse on the verb's own write set (R1, R2).
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md) — APPLIED: the document-panel precondition moves from skill text to a binary gate (R1, R11).
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md) — AMENDED: a `deferred_to_plan` record kind (R6).
- [ADR-0031](../../adr/ADR-0031-doc-sync-gate.md) — AMENDED: no-doc-impact declaration and doc-deferral owner. `complete --allow-doc-skew` needs an owner (R8, R9).
- [ADR-0032](../../adr/ADR-0032-adr-semantic-gates.md) — AMENDED: a file that no domain owns warns (R10).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with one `recovery:` line.

## Requirements

**Terms.**
- *Document gate*: the `spec_approve` or `plan_approve` panel gate.
- *Gate decision*: the result of `PanelGateDecision` for one registration.
- *Live registration*: a registration for the gate and spec that is not abandoned.
- *Presence policy*: the value of `enforcement.panel_presence` (R4).
- *Own write set*: the content that a verb writes itself (R1.6, R2.5).

### R1 — Document panel gate (G1, #212 ask 1, mindspec-0pij)

1. `spec approve <id>` runs the `spec_approve` gate. `plan approve <id>` runs the `plan_approve` gate. Each gate runs before the first write of the verb: frontmatter, `## Approval` section, epic, `plan.md` scaffold, bead creation, the supersede close of old beads, and commit. Each run of the verb runs the gate, also when the document is already approved.
2. The gate passes only when the spec has at least one live registration for that gate, and the gate decision of each live registration is Allow. It uses the same decision legs as `complete`: malformed registration, integrity, round mismatch, stale subject, incomplete, REJECT or hard block, and unresolved finding. It also runs the coverage check that `complete` and `impl approve` run, on each live registration. The presence policy does not change a failing leg: under `warn` and under `enforce`, a failing leg refuses.
3. The head of a document gate is the tip of the registration's recorded target. That target must be `spec/<id>`. `panel create` refuses another target for a `spec_approve`, `plan_approve` or `final_review` panel. The gate refuses a registration with another target and names the target. So `panel verify` and the gate judge the same commit. The subject is the spec directory, with the `Subject` exclusions and without the own write set (R1.6).
4. The gate refuses when the spec worktree has an uncommitted change inside the subject, outside the own write set. The message lists each path.
5. With no registration for the gate, the presence policy decides (R4). When every registration is abandoned, the abandonment hatch of R1.7 applies.
6. The own write set of `spec approve` is the `status`, `approved_at` and `approved_by` frontmatter fields of `spec.md`, the `## Approval` section of `spec.md` when it holds only the heading, blank lines and the four bullet lines `- **Status**:`, `- **Approved By**:`, `- **Approval Date**:` and `- **Notes**:`, each once, with any value, and a `plan.md` that is byte-equal to the scaffold the verb writes. The draft section that `spec create` writes and the section that `spec approve` writes both match. A matching section and an absent section give the same digest. Any other line in that section is subject content. The own write set of `plan approve` is the `status`, `approved_at`, `approved_by` and `bead_ids` frontmatter fields of `plan.md`. A frontmatter field of an own write set is left out with any value, present or absent. The subject digest of each document gate leaves out the own write sets of both verbs. One function computes this digest for `panel create`, `panel rebind`, `panel verify` and the gate. It compares frontmatter as parsed keys and values, so a rewrite that changes only key order or quoting does not change the digest. So a re-run after a fault at any point after the gate, before or after the commit, does not refuse on the verb's own writes. Any other change to `spec.md` or `plan.md`, committed or not, still makes the gate refuse. Approval fields written by hand do not skip the gate. A document or final-review round that a binary from before this change created can hold a digest of another form. When the head moves, the gate reports a stale subject for that round. The recovery is a new round at `spec/<id>`.
7. The hatches of `complete` (ADR-0037 §7) apply with the same behavior:
   - The skip variable passes the gate with a Warn. The verb records `panel_gate_skipped: true` and the time on the epic.
   - `enforcement.panel_gate: false` passes the gate and writes no record.
   - An abandoned registration passes with a Warn that names the reason. The verb records `panel_abandoned: true`, the reason and the time on the epic.
   - The verb writes these records, the `panel_absent` record (R4.2) and the deferral-id list (R6.4) after it finds or creates the epic. `plan approve` and `impl approve` refuse when the epic is absent, so only `spec approve` meets this case. When `spec approve` cannot create the epic and one of these records is due, it prints the record in a warning, exits non-zero after its commits, and prints `recovery: run the same command again in the same environment`. The re-run finds or creates the epic and writes the record (R1.6).
   - The refusal text does not name the skip variable.
8. A refusal names the gate, the panel and the failing leg. It ends with one `recovery:` line. With no registration, the line is `recovery: mindspec panel create <slug> --spec <id> --gate <gate> --target spec/<id>`, with the spec id and gate filled in.
9. A document-gate or final-review-gate message does not contain the bead merge fence or the bead fix-skill text that bead-gate messages carry.

### R2 — Final-review gate (G2)

1. `impl approve <id>` runs the `final_review` gate before its first write: the `--supersede-adr` placeholder ADR, the phase-reconcile write, the first epic mutation, and each commit or merge of `FinalizeEpic`. The rules of R1.2, R1.3, R1.5, R1.7, R1.8 and R1.9 apply. The gate applies to every spec, whatever the number of plan beads. The subject is the whole tree, with the `Subject` exclusions and without the own write set (R2.5).
2. (Removed.)
3. (Removed.)
4. Before its first write, the gate refuses when the spec worktree has an uncommitted change inside the subject, outside the own write set (R2.5). The message lists each path. A change in the spec worktree only in an excluded path (ledger, `.beads/issues.jsonl`) does not make this check refuse. The bead-worktree check of R2.6 is a separate check. The ledger-persistence preflight of `impl approve` (`implLedgerPreflight`) stays: on the flat layout, it refuses an uncommitted ledger file under each policy and hatch. A commit of only the ledger then satisfies it, and does not make a `final_review` panel stale.
5. The own write set of `impl approve` is the placeholder ADR that `--supersede-adr` creates, byte-equal to the placeholder text. The "commit remaining spec artifacts" commit of `FinalizeEpic` can then hold only own writes and excluded paths.
6. The final-review gate judges the subject on `spec/<id>` only. It does not read bead branches or bead worktrees. Instead, before its first write, `impl approve` refuses when a plan bead is not merged into `spec/<id>`, or has a remaining bead worktree with an uncommitted change. So the bead auto-merge of `FinalizeEpic` and its "chore: commit remaining bead artifacts" commit bring no content into `spec/<id>` after the gate.
   - *Unmerged bead (delivered).* `impl approve` refuses today, before its first write: an open plan bead in the plan-bead status gate of `ApproveImpl`, and a closed plan bead whose branch is not an ancestor of `spec/<id>` in `runOrphanObligationGate` and `runWorktreeEnumerationLeg` (`internal/approve/impl.go`). This spec keeps these refusals. AC11 locks them.
   - *Merged bead with a dirty worktree (new).* Today `checkMergedFinalizeWorktree` (`internal/executor/mindspec_executor.go`) refuses this case only inside `FinalizeEpic` (stage `auto_merge`), after the "commit remaining spec artifacts" commit and the epic writes. It counts every path, and its recovery does not name the bead. The new check runs before the first write of `impl approve`. It reads each plan bead worktree that the worktree list of `FinalizeEpic` names. It counts each change that `git status` reports, tracked or untracked, in every path, as `checkMergedFinalizeWorktree` and `checkFinalizeCleanupWorktrees` do. This check is separate from the final-review subject: the subject keeps the `Subject` exclusions, and this check has none. The message names the bead and lists each path. `checkMergedFinalizeWorktree` keeps its rule.
   - *Recovery.* When each listed path is the beads export (`.beads/issues.jsonl`) or a panel ledger (`reviews/<panel>/dispositions.jsonl` of a spec), the one recovery line is `recovery: mindspec complete <bead>`. On a merged bead, `complete` does not count these paths as user changes (`checkMergedBeadUserChanges`, `checkBeadCleanupWorktree`), and it removes the bead worktree. For any other path, the recovery lines are `recovery: commit or discard the listed paths in <bead worktree>`, then `recovery: mindspec complete <bead>`, then `recovery: after a commit, follow the follow-up bead steps that mindspec complete prints, then, if impl approve asks for a final-review panel, run mindspec panel create <slug> --spec <id> --gate final_review --target spec/<id>`. On a merged bead, `complete` refuses while such a path is uncommitted (`userDirtRefusal`). After a discard, `complete` removes the bead worktree. A commit moves the bead tip, so `complete` refuses with the follow-up bead steps (`movedTipFailure`): the change lands through a new bead with its own panel, and the spec then needs a final-review round.
   - A merged bead whose branch remains and whose worktree is clean passes. A branch that is an ancestor of `spec/<id>` adds nothing at merge.
   - The hatches of R1.7 and the presence policy do not change the check.
7. The gate is a pre-close gate under the re-run rule of `impl approve` (spec 132 R2.11): while the spec branch exists, a re-run runs every pre-close gate again. A re-run after a fault at any point after the gate, the epic close included, judges the current spec tip. It does not refuse on the own write set or on the excluded export.
8. A re-run with no spec branch or no spec worktree does not deadlock and does not pass new content:
   - When the spec branch is gone, the final-review gate does not run. The "already landed" checks of `impl approve` (spec 132 R2.12) decide, and their refusals stay.
   - When the spec branch exists, the spec worktree is gone, the epic is closed and the phase is done, the gate does not read a registration. It passes when the final-review subject at the spec tip equals the subject at `mindspec_finalize_tip`. Otherwise it refuses, names the gate, and prints `recovery: re-create the spec worktree at spec/<id>, then run a final-review panel`.
   - The dirt check of R2.4 runs only while the spec worktree exists.

### R3 — Bead panel presence (G3)

1. The presence policy (R4) replaces the "no registration → Allow" result of the bead panel gate. It applies on each `complete` path (spec 132 R3.0) that runs the panel gate today:
   - Open bead (the merge-only path, the closing path and the plain merge path): a refusal comes before the close. The bead stays open.
   - Closed bead with a naming merge (path 4): a refusal leaves the bead closed and the merge in place, and runs no later step.
   - Closed bead with no naming merge (path 5): a refusal leaves the bead closed and unmerged.
   - Each closed-bead refusal prints `recovery: run a panel for the bead at its tip`, then `recovery: mindspec complete <bead>`.
   - The path-4 cleanup guard (a layout that is not flat, an absent bead worktree and no registration found) stays a refusal under `warn` and under `enforce`. The `warn` policy does not change it to a warning.
   - The branch-less reconcile path and the busy-worktree refusal do not run the panel gate and stay unchanged.
2. (Removed.)
3. The refutation-obligation branch for a bead with no panel stays unchanged.

### R4 — Presence policy and migration

1. Config key `enforcement.panel_presence` is `warn` or `enforce`. The default is `warn`. No release changes this default. Config load refuses any other value and names the key.
2. Under `warn`, a gate with no registration prints one `warning:` line with the gate name and the R1.8 recovery command. The verb continues and exits as it does today, except when `spec approve` cannot create the epic (R1.7). The verb records `panel_absent` with the gate and time on the epic (R1, R2) or on the bead (R3). `warn` applies only to a missing registration (R1.2).
3. Under `enforce`, a gate with no registration refuses before the first write.
4. This change does not check approvals that exist. A spec in Plan or Implement mode keeps its spec and plan approvals, and no verb runs the `spec_approve` or `plan_approve` gate for them again. A new run of `spec approve` or `plan approve` is a new approval and runs its gate (R1.1). This includes `plan approve` on an edited plan whose old beads are all open: the gate runs before the supersede close and before the new beads. A re-run after a fault is not a new approval of changed content: R1.6 applies.
5. `doctor` prints one Info line when the policy is `warn`. The line says that gates without a panel only warn and names the key. `doctor` prints one Warn line for each in-flight spec in Plan or Implement mode whose epic has a `panel_absent` record.
6. This repository's `.mindspec/config.yaml` sets `enforcement.panel_presence: enforce`.

### R5 — Artifact hard block (G5)

1. A v2 verdict item can have an optional `artifact_path`. The value is a repository-relative slash path. It is not absolute, has no `..` segment and no control byte. The validator refuses other values with the JSON pointer of the field. The JSON Schema that `panel schema` prints includes the field.
2. The gate blocks while an item of a latest submission in the latest round has an `artifact_path` that is absent from the tree of that round. The tree of the round is the `reviewed_head_sha` of its `round` ledger record. The message names the slot, the item and the path.
3. A refutation, a `panel defer` deferral or a `deferred_to_plan` deferral (R6.3) of the item discharges that block, as for any other finding. A new round at a tree that contains the path also discharges it. A `panel rebind` does not discharge it: the rebind changes the reviewed commit of `panel.json`, not the tree of the round. The hatches of R1.7 pass the gate with their audit, as for any other block.
4. A verdict with no `artifact_path` gives the same result as today.

### R6 — Spec findings deferred to the plan (G8)

1. `panel defer <slug> --slot S --round N [--finding I] --to-plan --by <actor>` works on a `spec_approve` panel only. Other gates refuse `--to-plan`. `--to-plan` and `--bead` are mutually exclusive. With `--finding` omitted, it covers each finding of the slot, as `panel defer` does today.
2. Each run appends, in one ledger write:
   - one `deferred_to_plan` record for each covered finding, with a stable deferral id, the slot, round, finding id, finding hash, finding text, actor and time, and no `bead` field;
   - one `disposition` row `confirmed-deferred` for each covered finding, with the evidence `plan:<deferral id>`, so the coverage check counts the slot.
   The ledger validator accepts the new kind. `panel.json` gets a `plan_deferrals` list with the same entries. Its `deferrals` list does not change.
   The integrity leg refuses a `plan_deferrals` entry that no `deferred_to_plan` ledger record backs, as it does for `deferrals`. A new round that `panel create` makes carries `plan_deferrals` as it carries `deferrals`. The gate discharges a finding only from a ledger-backed entry.
3. A `deferred_to_plan` record discharges its finding for the `spec_approve` gate, with the same finding-hash rule as a refutation. The bead-deferral checks of `impl approve` (`implPanelDeferralPreflight`) do not read `plan_deferrals`.
4. When `spec approve` passes, it records the id of each `deferred_to_plan` record of the spec on the epic. `plan approve` reads that list and every `deferred_to_plan` record in the tracked ledger of every `spec_approve` registration of the spec: live, abandoned or replaced. So a deleted or edited ledger does not remove an obligation that `spec approve` accepted. `plan approve` refuses unless the `## Resolutions` section of `plan.md` names each deferral id. An id that occurs only outside that section does not count. The refusal lists each missing id. This check runs before the first write, under each presence policy.
5. `panel defer` without `--to-plan` on a `spec_approve` panel keeps today's refusal.

### R7 — `complete --check` (mindspec-yesp)

1. `mindspec complete <bead> --check` runs the content gates on the tree that `complete` would land, with the range and landing tree that plain `complete` uses (spec 132 R1, R3.13). The content gates are doc-sync, the ADR gates (ownership included) and the work-destruction preflight.
2. `--check` works on an open bead with a branch and no pending merge-only run (the closing path and the plain merge path). On another path it refuses, names the bead state and writes nothing.
3. It does not run the panel gate or the coverage check.
4. It changes nothing: no commit, no index change, no bead status or metadata change, no branch or worktree change.
5. It reports every failing content gate, not only the first, with each recovery line. It exits 0 when all pass and 1 when one fails.
6. Plain `complete` also runs every content gate before it refuses, and its refusal names every failing content gate. So for the same bead and tree, the failing set of `--check` equals the refusal set of plain `complete`.
7. `--check` refuses `--merge-only`, `--resolve-merge`, `--override-adr`, `--supersede-adr` and `--allow-net-deletion`. It accepts `--no-doc-impact` (R8) and `--allow-doc-skew` with `--doc-owner` (R9). It validates them as plain `complete` does, reports the domains they cover, and records nothing.

### R8 — No-doc-impact declaration (mindspec-yesp)

1. `complete <bead> --no-doc-impact "<domain>: <reason>"` can repeat. Each value satisfies the doc-sync gate for that domain on this bead only. The reason must not be empty. A domain that the bead's change does not touch refuses.
2. `complete` records domain, reason, actor and time on the bead.
3. The branch-wide doc-sync gate of `impl approve` accepts a domain when the branch changes its docs, or when each change to its sources on the branch came in through the landed merge of a bead that recorded no doc impact for it. A source change on `spec/<id>` that no bead merge carries gets no exemption.
4. Each printed recovery that repeats the flags of the failed run (the completion command, the merged-range failure and the doc-sync hint of `complete`) also repeats each `--no-doc-impact` value.

### R9 — Doc-deferral owner (mindspec-yesp notes)

1. `complete --allow-doc-skew "<reason>"` needs `--doc-owner <bead>`. The owner is an open bead in the same epic, and not the bead being completed. Other values refuse.
2. `complete` records the deferred domains, owner, reason, actor and time on both beads.
3. `complete` of the owner bead refuses unless its landed change updates the docs of each deferred domain. `--allow-doc-skew` with a new `--doc-owner` on the owner moves the deferral to the new owner.
4. `impl approve` lists each open deferral and refuses while one is open. A deferral is open until its owner bead lands with the doc change.
5. `impl approve --allow-doc-skew "<reason>"` takes no owner. It overrides R9.4 and the branch-wide doc-sync gate, and records each open deferral id with the reason on the epic.
6. Each printed recovery that repeats `--allow-doc-skew` also repeats `--doc-owner <bead>`.
7. A skew record with no owner, from before this change, stays under today's branch-wide doc-sync check.

### R10 — Unowned file warning (mindspec-k96x)

1. A changed file that no `OWNERSHIP.yaml` claims, in any domain, gives a Warn `adr-divergence-unowned`. `complete` and `impl approve` continue. The warning names the file and the domains root.
2. A file that a domain outside the spec's Impacted Domains claims stays an Error.
3. `adr-divergence-attribute` and `adr-divergence-uncovered` stay Errors.

### R11 — Skills and guidance (skill change, #212 asks 2–3, mindspec-yesp)

1. `ms-spec-approve` and `ms-plan-approve` state: run the panel with `--target spec/<id>`, then run the approve verb; the verb refuses without a passing panel. The sentence "This step is guidance, not a mechanized preflight" and the text that it qualifies are removed from the active skill text: the canonical bodies in `internal/setup` and the copies in `.claude/skills`, `.agents/skills` and `plugins/mindspec/skills`. The historical snapshots in `internal/setup/historical_skills` do not change.
2. For each skill that this spec changes, the current text is added as a historical snapshot. So the install refresh replaces an unchanged old copy, and it keeps a copy that the user changed.
3. `ms-impl-approve` and `ms-spec-final-review` state that `impl approve` refuses without a passing final review, for every spec, whatever the number of beads.
4. `ms-bead-cycle` runs `mindspec complete <bead> --check` after the implementation commit and before it convenes the bead panel. On a failure, it sends the failing gates to the implementer or fixer. The bead panel then reviews the fixed diff.
5. `instruct` in Spec mode and Plan mode renders one line: approval needs a passing document panel, with the `panel create` command for the gate. A second line names the change kinds of R11.6 that call for reviewers from more than one family.
6. The approve skills state when to use reviewers from more than one family: for a spec that changes a gate, a schema, a trust boundary or a merge path. `config resolve --gate <gate>` shows the families.
7. `ms-panel-run` states that a `spec_approve`, `plan_approve` or `final_review` panel uses `--target spec/<id>`, and that an older live registration with another target blocks the gate until a new round at `spec/<id>` replaces it or it is abandoned. A round that an older binary created can also need a new round at `spec/<id>`.
8. Text that this spec adds to a CLI message, a recovery line, a skill or `instruct` contains no ADR number, spec number, requirement id, bead id or issue number.

### R12 — ADR text

1. ADR-0037, ADR-0041, ADR-0043, ADR-0031 and ADR-0032 each get an amendment section that states the rules of R1–R10 for that ADR.

## Scope

### In Scope
- R1–R12 in the packages and documents in Impacted Domains.
- This repository's `enforcement.panel_presence: enforce` (R4.6).

### Out of Scope
- The rules of spec 132 for `complete`, `impl approve` and `doctor`, except the added gates that R2, R3 and R7 state.
- The round limit and other panel-execution items that spec 131 left open.
- The completeness-against-sources and adversarial-sufficiency lenses that #212 ask 1 names.

## Non-Goals

- Rebuilding a panel from the tracked ledger in a checkout where it did not run. A gate reads the registration in the checkout where the verb runs. R6.4 reads only the tracked ledger.
- Changing the gate authority of the approve verbs. A human still approves. The panel is a precondition.
- Proof that the actor of an override is a human. The hatch model of ADR-0037 §7 stays.
- A required second model family.

## Acceptance Criteria

Each refusal AC requires that the message names the gate or the check, so that a refusal for another reason fails the AC. An AC that names no presence policy holds under `warn` and again under `enforce`. Unless an AC says otherwise, each fixture passes every other check and gate that the verb runs, and has a passing panel for each gate other than the one under test. So the AC observes the rule it names, and no earlier refusal hides it.

- [ ] AC1 (R1.1, R1.2, R1.8): Under `enforce`, in a fixture spec with no `spec_approve` panel, `spec approve <id>` exits non-zero and names the `spec_approve` gate. The last line is `recovery: mindspec panel create <slug> --spec <id> --gate spec_approve --target spec/<id>` with the id filled in. `git rev-parse HEAD`, `spec.md`, the absence of `plan.md` and the absence of an epic are unchanged.
- [ ] AC2 (R1.2, R1.9): Under `warn` and again under `enforce`: for each of incomplete, REJECT, unresolved finding, a finding with no disposition row (the coverage check) and malformed `panel.json` on a `spec_approve` panel, `spec approve` exits non-zero, names that leg, and does not contain the bead merge fence. With a passing panel, it exits 0.
- [ ] AC3 (R1.3, R1.6): After a passing `spec_approve` panel, a commit that changes only `reviews/<slug>/dispositions.jsonl` leaves `spec approve` at exit 0. A commit that changes the body of `spec.md` makes it exit non-zero with a stale-subject message. `panel create --target <sha>` exits non-zero for each of `--gate spec_approve`, `--gate plan_approve` and `--gate final_review`. After `spec approve` rewrites the frontmatter with a changed key order, `panel verify` and the gate both still pass. A ledger-backed registration with a SHA target, made by a binary from before this change, makes `spec approve` exit non-zero and name the target. A `panel.json` whose target is edited by hand makes it exit non-zero and name the integrity leg.
- [ ] AC4 (R1.4): With a passing `spec_approve` panel, an uncommitted edit to the body of `spec.md` makes `spec approve` exit non-zero and name `spec.md`. No commit is made.
- [ ] AC5 (R1.6, R1.7): Under `enforce`, with a passing panel: a fault injected after the frontmatter write and before the commit, then a re-run, gives exit 0 and one approval commit. A fault injected after the approval commit (the "sync beads state" commit fails), then a re-run, gives exit 0. A `spec_approve` panel created at a spec whose `## Approval` section is the unedited draft that `spec create` writes passes `panel verify` and the gate after `spec approve` writes the approved section and commits. From that draft, a fault after the approval commit, then a re-run, gives exit 0. A panel created at a spec with no `## Approval` section gives the same results. A committed line under `## Approval` beyond the four labelled lines makes the gate refuse with a stale-subject message. With the skip variable set and the epic creation failing, `spec approve` exits non-zero with the re-run recovery, and a re-run writes `panel_gate_skipped` on the epic. The same holds for `panel_absent` under `warn` with no panel, and for the deferral-id list of R6.4. With no passing panel, `spec.md` with approval fields committed by hand makes `spec approve` exit non-zero and name the gate.
- [ ] AC6 (R1.1, R1.2, R1.6): For `plan approve` with a `plan_approve` panel: with no panel under `enforce`, it exits non-zero, names the gate and prints the recovery line for that gate, and `plan.md` and the bead set are unchanged. Each AC2 leg case gives the AC2 result. A fault after the plan frontmatter write, then a re-run, and a fault after the plan approval commit, then a re-run, each pass the gate. A committed body edit of `plan.md` makes the gate refuse with a stale-subject message. Under `enforce`, plan approval fields committed by hand with no `plan_approve` panel make it refuse and name the gate. Under `warn` and under `enforce`, plan approval fields committed by hand with a failing `plan_approve` panel make it refuse and name the failing leg. The `## Approval` section and epic-creation cases of AC5 apply to `spec approve` only: `plan approve` writes no `## Approval` section and creates no epic. A refusal creates no bead.
- [ ] AC7 (R1.7): Under `enforce`, with no panel: with the skip variable set, `spec approve` exits 0 and the epic has `panel_gate_skipped`. With `enforcement.panel_gate: false`, it exits 0 and the epic has no skip record. With only an abandoned panel, it exits 0, prints a Warn with the reason, and the epic has `panel_abandoned`. No message contains the skip variable name.
- [ ] AC8 (R2.1): Under `enforce`, with 2 plan beads and no `final_review` panel, `impl approve` exits non-zero and names the `final_review` gate. The epic stays open, the phase is unchanged, and no placeholder ADR exists after a run with `--supersede-adr`.
- [ ] AC9 (R2.1, R4.2, R4.3): With 1 plan bead that landed through a passing bead panel and no `final_review` panel: under `warn`, `impl approve` exits 0, prints one `warning:` line that names `final_review`, and records `panel_absent` on the epic. Under `enforce`, it exits non-zero before any write and names `final_review`.
- [ ] AC10 (R2.4, R2.5): Under `enforce`, with a passing `final_review` panel and an uncommitted change to a source file, `impl approve` exits non-zero, names the path, closes no epic and makes no commit. In a flat-layout fixture with only an uncommitted ledger change, the final-review dirt check does not name the ledger. The ledger-persistence preflight refuses, names the ledger file, closes no epic and makes no commit. After a commit of only the ledger, `impl approve` exits 0. With only an uncommitted `.beads/issues.jsonl` change, which the ledger-persistence preflight does not read, the final-review gate passes and no refusal names `.beads/issues.jsonl`.
- [ ] AC11 (R2.6): With a passing `final_review` panel:
  - (a) A closed plan bead whose branch has a commit that `spec/<id>` does not contain: `impl approve` exits non-zero and names the bead. It makes no commit in the bead worktree or on `spec/<id>`, merges nothing, and the epic stays open. The same holds for an open plan bead.
  - (b) A merged plan bead whose remaining worktree has an uncommitted source change: `impl approve` exits non-zero before any write, names the bead and the path, and prints the commit-or-discard recovery line before `recovery: mindspec complete <bead>`. It makes no commit in the bead worktree or on `spec/<id>`, merges nothing, and the epic stays open. The same holds with the skip variable set. The same holds for an untracked source file. After the change is discarded, `mindspec complete <bead>` exits 0 and removes the bead worktree, and `impl approve` exits 0. After the change is committed in the bead worktree instead, `mindspec complete <bead>` exits non-zero and prints the follow-up bead steps, and `impl approve` still refuses.
  - (c) A merged plan bead whose remaining worktree has an uncommitted change only in `.beads/issues.jsonl`: `impl approve` exits non-zero before any write, names the bead and the path, and prints `recovery: mindspec complete <bead>`, no `panel create` line and no commit-or-discard line. The epic status and phase are unchanged. It makes no commit on `spec/<id>` or in the bead worktree. After `mindspec complete <bead>` exits 0 and removes the bead worktree, `impl approve` exits 0. The same holds for a change only in a panel ledger (`reviews/<panel>/dispositions.jsonl` of a spec).
  - (d) A merged plan bead whose branch remains, with a tip that is an ancestor of `spec/<id>`, and whose worktree is clean: `impl approve` exits 0.
- [ ] AC12 (R2.7, R2.8): With a passing `final_review` panel, a fault injected after the "commit remaining spec artifacts" commit, then a re-run, gives exit 0. A fault injected after the epic close, then a re-run, gives exit 0. With `--supersede-adr`, a fault after the placeholder write, then a re-run, gives exit 0. Under `enforce`, after a real landing and spec-branch deletion, a re-run prints the "already landed" result and exits 0. Under `enforce`, in a fixture with a closed epic, phase done, the spec worktree removed and the spec branch present, a re-run exits 0 at the finalize tip and refuses with the gate name after a new source commit on `spec/<id>`.
- [ ] AC13 (R3.1, R4.3): Under `enforce`, plain `complete <bead>` on an open bead with no panel exits non-zero, names the gate, and the bead stays `in_progress`. On a closed bead with a naming merge and no panel, it exits non-zero with the two recovery lines, the bead stays closed, and the spec tip is unchanged. On a closed bead with no naming merge and no panel, it exits non-zero with the two recovery lines, the bead stays closed, and no merge is made. Under `warn`, the path-4 cleanup guard fixture still exits non-zero.
- AC14: (Removed.)
- [ ] AC15 (R4.1, R4.2): Under `warn`, each missing-registration case of AC1, AC8 and AC13 exits 0, prints one `warning:` line with the gate name and records `panel_absent`. The path-4 cleanup guard fixture of AC13 is not one of these cases: under `warn` it exits non-zero, prints no such warning and records no `panel_absent` (R3.1). A config value `panel_presence: maybe` makes every command refuse at load and name the key.
- [ ] AC16 (R4.4): Under `enforce`, a fixture spec in Plan mode with an approved `spec.md` and no `spec_approve` panel, plus a passing `plan_approve` panel, gives `plan approve` exit 0. An approved plan with all beads open, then edited, gives `plan approve` exit non-zero with no `plan_approve` panel: no bead is closed and no bead is created. With a passing panel at the edited plan, it exits 0 and replaces the beads.
- [ ] AC17 (R4.5): `mindspec doctor` prints the Info line under `warn`, and one Warn line for a fixture spec in Plan mode whose epic has `panel_absent`.
- [ ] AC18 (R4.6): `.mindspec/config.yaml` at the spec branch tip has `enforcement.panel_presence: enforce`. A test reads it with the config loader.
- [ ] AC19 (R5.1): `panel submit` refuses an item with `artifact_path` `../x`, `/x` or a control byte, and names the JSON pointer. `panel schema` output contains `artifact_path`.
- [ ] AC20 (R5.2, R5.3): A passing panel plus one item with an `artifact_path` absent from the round's tree makes `complete` exit non-zero and name the path. A refutation of the item makes `complete` pass the panel gate. So does a `panel defer` of the item. A new round at a tree with the path, with all findings discharged, passes. On a `spec_approve` panel, the same block makes `spec approve` exit non-zero and name the path, and a `panel defer --to-plan` of the item makes it pass the gate.
- [ ] AC21 (R6.1, R6.2, R6.3, R6.5): `panel defer --to-plan` on a `spec_approve` panel appends one `deferred_to_plan` record and one `confirmed-deferred` disposition row for each covered finding. `panel disposition check` passes, and `spec approve` then passes. With `--finding` omitted, it covers each finding of the slot. On a `bead` panel, `--to-plan` exits non-zero. `--to-plan` with `--bead` exits non-zero. `impl approve` does not refuse on a `plan_deferrals` entry. A `plan_deferrals` entry added to `panel.json` by hand makes `spec approve` exit non-zero and name the integrity leg. A new round carries the backed entries. `panel defer` without `--to-plan` on a `spec_approve` panel exits non-zero with today's message.
- [ ] AC22 (R6.4): Under `warn` and under `enforce`, with a passing `plan_approve` panel, `plan approve` exits non-zero and names each deferral id that `## Resolutions` does not name. An id that occurs only outside `## Resolutions` is refused. After the spec is approved with a to-plan deferral and its `spec_approve` panel is abandoned, the unresolved id is still refused. After the ledger file is deleted in a commit, an id that `spec approve` recorded on the epic is still refused. With every id named, it passes.
- [ ] AC23 (R7.1–R7.5): On a fixture bead with a doc-sync failure and an ADR failure, `complete <bead> --check` exits 1 and names both. `git status --porcelain`, `git rev-parse HEAD`, the bead status and the bead metadata are byte-equal before and after. On a closed bead, `--check` exits non-zero and writes nothing.
- [ ] AC24 (R7.6): For five fixture trees (pass, doc-sync fail, ADR fail, work-destruction fail, and all three), the failing set of `--check` equals the refusal set of plain `complete`. For "all three", each names doc-sync, the ADR gate and the work-destruction preflight.
- [ ] AC25 (R7.7): `--check` with each of `--merge-only`, `--resolve-merge`, `--override-adr`, `--supersede-adr` and `--allow-net-deletion` exits non-zero. `--check --allow-doc-skew "<reason>" --doc-owner <open bead in the epic>` exits 0 on a doc-sync-only failure and writes no record. With a closed owner, it exits non-zero.
- [ ] AC26 (R8.1–R8.4): `complete --no-doc-impact "workflow: test-only change"` passes the doc-sync gate for `workflow` and records the declaration. With two failing domains and one declared, `complete` exits non-zero and names the other domain. An empty reason and an untouched domain each exit non-zero. `impl approve` accepts `workflow` when the one bead that changed its sources declared it. With two beads that change `workflow` sources and one declaration, `impl approve` exits non-zero. With every bead declared and a `workflow` source commit made directly on `spec/<id>` with no doc change, `impl approve` exits non-zero and names the doc-sync gate. A failed run with `--no-doc-impact` prints a recovery with the same value.
- [ ] AC27 (R9.1–R9.4, R9.6, R9.7): `--allow-doc-skew` with no `--doc-owner`, with a closed owner, with the bead itself as owner, or with a bead in another epic exits non-zero. With a valid owner, `impl approve` refuses and lists the deferral. `complete <owner>` with no doc change in the deferred domain exits non-zero. After the owner lands with the doc change, `impl approve` passes. A failed run with `--allow-doc-skew` prints a recovery that repeats `--doc-owner`. After a valid skew, both beads carry the record. On the owner, `--allow-doc-skew` with a new `--doc-owner` moves the deferral, and `impl approve` then lists the new owner. A skew record with no owner, from before this change, gets today's branch-wide doc-sync result.
- [ ] AC28 (R9.5): With an open deferral, `impl approve --allow-doc-skew "<reason>"` exits 0 and records the deferral id and the reason on the epic.
- [ ] AC29 (R10.1–R10.3): A bead that changes `CHANGELOG.md` with no owning manifest completes with exit 0 and one `adr-divergence-unowned` warning. A file that an undeclared domain owns still refuses. An attribute failure and an uncovered failure each still refuse.
- [ ] AC30 (R11.1, R11.2, R11.8): `git grep -n 'guidance, not a mechanized' -- internal/setup/claude.go .claude/skills .agents/skills plugins` exits 1. An install over each pre-change copy of a changed skill writes the new text. An install over a copy that the user changed keeps it. The surface-reference test passes on every string that this spec adds.
- [ ] AC31 (R11.3, R11.4, R11.6, R11.7): The `ms-impl-approve` and `ms-spec-final-review` text states the final-review refusal for every spec. In the `ms-bead-cycle` text, the `mindspec complete <bead> --check` step comes before the bead-panel step. The approve skills name the four change kinds that call for more than one family. The `ms-panel-run` text gives `--target spec/<id>` for the three gates.
- [ ] AC32 (R11.5): `mindspec instruct` in Spec mode and in Plan mode prints the panel line with the gate name for that mode, and the line with the change kinds.
- [ ] AC33 (R12): Each of the five ADRs has an amendment section for this spec. A test or a `git grep` finds the presence policy in ADR-0037, the own write set in ADR-0041, `deferred_to_plan` in ADR-0043, `--doc-owner` in ADR-0031, and the unowned Warn in ADR-0032.
- [ ] AC34: `make test` (or the repository's CI test command) passes on the spec branch.

## Validation Proofs

- `go test ./internal/approve/... -run 'DocumentGate|FinalReviewGate|Resolutions|ReApprove'`: AC1–AC12, AC16, AC22, AC28 pass.
- `go test ./internal/complete/... -run 'Presence|CompleteCheck|NoDocImpact|DocOwner'`: AC13, AC23–AC27 pass.
- `go test ./internal/panel/... -run 'ArtifactPath|DeferToPlan'` and `go test ./cmd/mindspec/... -run 'PanelDeferToPlan|PanelCreateTarget'`: AC3, AC19–AC21 pass.
- `go test ./internal/config/... -run 'PanelPresence'`: AC15, AC18 pass.
- `go test ./internal/validate/... -run Unowned`: AC29 passes.
- `go test ./internal/setup/... -run 'Refresh|SurfaceRef|BeadCycleCheck'`: AC30, AC31 pass.
- `git grep -n 'guidance, not a mechanized' -- internal/setup/claude.go .claude/skills .agents/skills plugins`: exit 1.
- `mindspec doctor` in the AC17 fixture: the Info line and the Warn line are printed.
- `mindspec instruct` in a Spec-mode fixture: the panel line is printed.
- `make test`: exit 0.

## Open Questions

- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval.
- [x] OQ1: Default of `enforcement.panel_presence` for other repositories. Decided: `warn` in every release. A repository opts in with `enforcement.panel_presence: enforce` in its `.mindspec/config.yaml`.
- [x] OQ2: Can a refutation discharge an artifact block (R5.3)? Decided: yes. A refutation or a deferral discharges the block.
- [x] OQ3: Does `artifact_path` stay in verdict schema 2, or start schema 3? Decided: an optional field in schema 2. A verdict without it stays valid. A released binary refuses a verdict with it, so panels in this repository use the field only after the spec merges.
- [x] OQ4: Is there a list of bead types that need no panel? Decided: no exempt list. The hatches cover a bead that needs no panel.
- [x] OQ5: Can a small spec use its bead panel as the final review? Decided: no small-spec exception. The final-review gate follows the presence policy.
- [x] OQ6: Does a file that an undeclared domain owns stay an Error (R10.2)? Decided: yes. It is scope drift with a named owner, and the message gives the fix.
- [x] OQ7: Does an abandoned registration pass a gate under `enforce` (R1.7)? Decided: yes, with a Warn and the `panel_abandoned` record, as ADR-0037 §7 states. Abandonment is an audited hatch, and R6.4 keeps the plan obligations of an abandoned spec panel.
- [ ] OQ8: Must a document or final-review panel target `spec/<id>` (R1.3)? Panels in this lane now use a commit SHA. Recommended: yes. A SHA target never moves, so the gate cannot see a later commit. A live registration with a SHA target blocks the gate. The operator runs a new round of it at `spec/<id>`, or abandons it with a reason. A round that a binary from before this change created needs the same recovery, also with a `spec/<id>` target (R1.6).

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
