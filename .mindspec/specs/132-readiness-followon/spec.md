---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 132-readiness-followon: Readiness Followon

*Citation scope: code citations were verified against `main` at `d15787fd` on 2026-10-20 in the spec-132 worktree. Line numbers are approximate; the named symbol is authoritative.*

## Goal

This spec closes the lifecycle and attestation items that spec 129 deferred. After this spec:

- `complete` reads domain ownership from the tree that will land.
- `impl approve` refuses in preflight for the failures that it can predict. For every other failure, it prints the real state and a recovery, and a re-run finishes the landing.
- An operator can merge a bead, verify the merge, and then close the bead. Git state, not a marker, decides the close.
- The unused `merge_strategy` key is gone. `impl approve` decides one time whether to push or merge locally.
- A reviewed tree that differs from the landed tree only in tool-written files corroborates the landing.
- Reattest names the case where a bead's commits arrived without a merge that names the bead. It claims that the work is present only when a content check proves it.
- Bead titles no longer decide lineage. Lettered spec IDs resolve to their own spec.

Lifecycle output and diagnostics items (R8–R12 of the first draft) move to a follow-on spec. See "Deferred to follow-on spec".

## Background

Provenance: spec 129 deferred G (#234), H (#244), #250, I (`merge_strategy`) and phase-lineage unification (129 spec.md, "Deferred to the follow-on spec"). Spec 130 Non-Goals moved #233's reviewed-tree check, #251 and e07y item 1 here.

Defects, each verified on `d15787fd`:

- **#234 (ownership ref).** `runCommittedRangeGate` (`internal/complete/complete.go:1653`) passes `ownerRef = beadHead` to `ValidateDocsRange` and `CheckADRDivergence`. `materialize.go:429,435` passes the bead tree. Six readers use that ref: Impacted-Domains normalization and domain enumeration (`internal/validate/divergence.go:197,208`), path attribution (`:234`), the ADR domain-resolving store (`adr_domain_resolve.go:72`), `checkInternalPackages` and `checkUnclaimedSource` (`docsync.go:493,228`). A claim made on the spec branch after the bead forked is invisible, so `adr-divergence-unowned` blocks owned paths.
- **#244 (half-done impl approve).** `ApproveImpl` (`internal/approve/impl.go`) closes the epic (MUTATION 1/3, :618) and writes `mindspec_phase=done` (2/3) before `FinalizeEpic` (3/3, :676). `FinalizeEpic` runs the auto-merge, push, direct-merge and cleanup stages (`internal/executor/mindspec_executor.go:584-1033`; fault seams `finalizeStepHook` at `auto_merge`, `push`, `orphan_finalize`, `pre_cleanup`, `post_cleanup`). A failure returns `finalizing epic: %w` with no state line. The early work-destruction preflight (`impl.go:418`) runs for the direct path only. A re-run after spec-branch deletion gets the branch-missing refusal (`impl.go:381`).
- **#250 (no verification window).** `complete` closes the bead before the merge (ORDERING DECISION, `complete.go:1235`) so that the merge commit's export records the close (ADR-0025). Then it merges and removes the branch and worktree. An operator cannot inspect the merge first. A hand merge followed by `complete` works, but `doctor` reports the gap as a stale-open Error (`internal/doctor/lifecycle_integrity.go:43`).
- **`merge_strategy` is dead.** `config.MergeStrategy` (`internal/config/config.go:20`, default `auto`) has no landing-policy reader; `config show` and config tests read the field. `FinalizeEpic` decides from `gitutil.HasRemote()` (`internal/executor/mindspec_executor.go:822`), and `ApproveImpl` makes a separate call through `implHasRemoteFn` (`internal/approve/impl.go:136`, `:418`). Each call runs `git remote` again, and an error reads as no remote (`internal/gitutil/gitops.go:409-415`). So the two calls can disagree. The harness sandbox writes `merge_strategy: direct` (`internal/harness/sandbox.go:63`, `lifecycle_scenario_test.go:205`). This repo sets `pr`.
- **#233 (reviewed tree).** Spec 130 writes `mindspec_landed_reviewed_sha`. Nothing reads it. In `FindLandedMerge` (`internal/lifecycle/landed.go:479`), a reviewed SHA confirms only when it equals the merge's second parent (`reviewedHeadSHAForBead`, `:731`). `complete` lands commits after the panel SHA, so this datum never confirms on the normal path. A branch-less bead then gets "panel set non-corroborating" (`:655`).
- **#251 (ancestor without naming merge).** Reattest nominates ownership only from a merge subject. When bead P's commits reached the spec tip inside a later bead's branch, reattest returns `no-owned-exact-merge` (`internal/lifecycle/reattest.go:191`). `doctor` reports the downstream bead, not P (`internal/lifecycle/scan.go:173`).
- **Internal refs.** The `no-owned-exact-merge` text, its CLI render (`cmd/mindspec/reattest.go:286,289`), the reattest `Short` text (`:49`) and two `materialize.go` messages (`:603,636`) print ADR numbers or a bead ID.
- **Lineage (wtjr, fdfg.6).** `FindEpicForBeadWithCache` (`internal/phase/derive.go:939`) accepts any dependency whose issue type is epic, including a `blocks` edge, and the first one wins. Then it falls back to a `[NNN]` bead-title parse with `Sscanf %d`. `parseSpecID` (`internal/approve/spec.go:423`) uses `Atoi`, so spec `129b` gets `spec_num 0`, and `FindEpicForBeadWithCache` skips that epic. `beadIndexRe` (`internal/validate/readiness/readiness.go`) accepts any trailing `.N`. `next` runs a second `bd show` per foreign-lineage plan row because `internal/phase` returns no provenance. Six open beads in this repo have a `[NNN ...]` title and no epic dependency.

Already fixed: 3gvd (`.cache/` is in `.gitignore:87`; nothing under `.cache/` is tracked).

## Impacted Domains

- **workflow**:
  - `internal/complete/**`: landing-tree ownership (R1), `--merge-only` and the closing call (R3).
  - `internal/approve/**`: early guards, state line, re-run (R2); one has-remote decision (R4); `spec_id` metadata (R7).
  - `internal/lifecycle/**`: reviewed-tree datum (R5), ancestor states (R6).
  - `internal/validate/**`: landing-tree ownership (R1), `beadIndex` (R7).
  - `internal/doctor/**`: stale-open severity (R3), `merge_strategy` Info (R4), ancestor finding (R6), title-only lineage (R7).
  - `internal/next/**`: lineage provenance (R7).
  - `cmd/mindspec/**`: `complete --merge-only` (R3), reattest text (R6), `spec create` refusal (R7).
- **execution**: `internal/executor/**` (has-remote parameter, finalize tip record, stage resume: R2, R4); `internal/harness/**` (sandbox config: R4).
- **core**: `internal/phase/**` (lineage and spec-ID resolution: R7); `internal/config/**` (key deletion: R4); `cmd/mindspec/config.go` (`config show` row: R4); `.mindspec/config.yaml` (this repo's key: R4).

Documentation surfaces: ADR-0041 and ADR-0036 amendments (R13); `.mindspec/domains/workflow/{architecture,runbook}.md`; `project-docs/user/guides/merge-safety.md`; `.mindspec/core/GIT-WORKFLOW.md` and `.mindspec/domains/core/interfaces.md` (R4.2); ADR-0006 and ADR-0019 dated notes (R4.2); spec 129 status notes (R13).

Sequencing with the panel-execution lane (spec 131): both lanes touch `internal/config/**` and `internal/executor/**`. The R4 bead lands after spec 131's config changes, or rebases on them. R4 does not change panel config loading or its source reporting. Panel commands and panel skills stay in spec 131. Each bead validates against the landed panel lane before `complete`.

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED (R13.1): early guard copies, the state line and stage resume for `impl approve` (R2). `--merge-only` is a commit phase with no close, and the closing call publishes the close (R3). The Spec-129 amendment (ii) changes: the reviewed-tree rule is a confirming datum in `FindLandedMerge` (R5). It never vetoes.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — AMENDED (R13.1): `complete` reads ownership at the landing tree (R1).
- [ADR-0025](../../adr/ADR-0025-jsonl-as-build-artifact.md) — APPLIED: the spec branch's export shows the close before evidence is deleted (R3.8).
- [ADR-0023](../../adr/ADR-0023.md) — APPLIED: lineage is a bd parent-child dependency (R7).
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: the has-remote probe stays behind the gitutil seam; ApproveImpl calls it one time and passes the value to the executor; the push probe and stage git work stay in the executor (R2, R4).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with a `recovery:` line (R2, R3, R4.4a, R6, R7).
- [ADR-0019](../../adr/ADR-0019.md), [ADR-0006](../../adr/ADR-0006.md) — NOTED: the `merge_strategy` key they list is removed (R4).

Terms:

- *Spec tip*: the commit at `spec/<id>` when the verb runs.
- *Landing tree*: `git merge-tree --write-tree --merge-base=<B>` of the spec tip and the bead commit (or the materialized bead tree T), through the net-effect primitive (`internal/gitutil/neteffect.go:147`). B is `git merge-base <spec tip> <bead HEAD>` for both legs. merge-tree writes an output tree also when paths conflict. This is the case before any merge. During a merge in progress, the landing tree is the resolved index (R1.5). After a naming merge M is committed, the gates use base M^1, head M, and M's tree (R3.13).
- *Naming merge*: a merge on the spec branch's first-parent chain whose subject names `bead/<id>`.
- *Valid merge M for bead B*: M is reachable from the spec tip; M is a naming merge for B; M's second parent equals the current `bead/<B>` tip; and M's change is present at the spec tip (R3.5).
- *Tool-artifact paths*: the set that `panel.ToolArtifactPaths` returns.

## Requirements

Every new refusal prints full commit SHAs and escapes subjects, IDs and inner errors with `termsafe`.

### R1 — Ownership at the landing tree (#234)

1. Both `complete` ownership legs (committed range and materialize) pass the landing tree as `ownerRef`.
2. All six readers in Background use that one ref. No reader takes a union of two refs.
3. A changed path is owned when an `OWNERSHIP.yaml` glob at the landing tree matches it for a domain in the spec's resolved Impacted Domains.
4. When paths conflict, `MergeInto` runs and keeps the conflict as today. When `OWNERSHIP.yaml` is not a conflicted path, the gate first reads it from the merge-tree output tree. When `OWNERSHIP.yaml` is a conflicted path, plain `complete` on an open bead (R3.0 path 7) refuses before the close with `recovery: mindspec complete <bead> --merge-only`. That run keeps the bead open and the conflict in place. `--merge-only`, and a closed bead on R3.0 path 5, do not refuse here: `MergeInto` keeps the conflict, and the gate runs at re-entry under R1.5.
5. On `--resolve-merge` re-entry (a merge is in progress in the spec worktree), the landing tree is `git write-tree` of the spec worktree index. When the index still has unmerged entries, today's unresolved-conflict refusal stays. When the operator committed the merge by hand, the landing tree is M's tree (R3.13).

### R2 — `impl approve` that can finish (#244)

1. `ApproveImpl` runs an early copy of the merge-layout guard and of the work-destruction preflight for the R4 has-remote decision. The copy runs before the phase-reconcile write, the `--supersede-adr` placeholder write and MUTATION 1/3.
2. The executor keeps its own guards in front of every `MergeInto` and `MergeBranch` call. They run on the tip after the auto-merge. Both copies must pass.
3. When the R4 has-remote decision says push, `ApproveImpl` runs `git push --dry-run --no-verify` to the same remote and refspec as `PushBranch`, with `GIT_TERMINAL_PROMPT=0`, before MUTATION 1/3.
4. Only a failed probe refuses. A missing credential helper alone never refuses.
5. The refusal adds a credential-helper hint only when the probe fails with an authentication error on an HTTP(S) remote.
6. A pre-push hook failure on the real push is a post-close failure under R2.7.
7. A failure after MUTATION 1/3 adds one state line before the inner error. `ApproveImpl` reads the epic status after the failure and prints it ("epic closed" or "epic still open"), plus the finalize stage that failed.
8. The inner error's recovery lines stay unchanged and come last. `ApproveImpl` adds `recovery: mindspec impl approve <spec>` only when the inner error has none.
9. `FinalizeEpic` writes the final publication tip to epic metadata `mindspec_finalize_tip`. It writes the key after the export commit and all bead auto-merges, immediately before the spec-to-main merge or the spec-branch push. Every run overwrites it. The pre-finalize tip record stays separate.
10. A re-run converges from each durable stage, on both paths:

| Failed stage | Re-run outcome |
|---|---|
| close or phase write | Closes idempotently, then runs `FinalizeEpic`. |
| `auto_merge` | Merges the remaining beads. Merged beads are ancestors and are skipped. |
| `push` (push path) | Pushes again. A branch that is already pushed is a no-op. |
| `orphan_finalize` (protected main) | Reuses the existing `chore/finalize-<id>` branch, and pushes it with `--force-with-lease` on the remote SHA that `git ls-remote` reads just before the push. The tool opens no PR, so a PR that already exists for that head branch stays the one PR. This holds before and after the carrier push, and after local spec-branch deletion. |
| direct merge to main | Skips the merge when the spec tip is an ancestor of main. |
| `pre_cleanup` / cleanup | Finishes the remaining removals. |
| after spec-branch deletion | Prints "already landed" and exits 0 only under R2.12. Otherwise today's branch-missing refusal and adopt recovery stay. |

11. While the spec branch exists, a re-run runs every pre-close gate again on the current spec tip. No gate is skipped because the epic is closed.
12. The "already landed" re-run needs all of these: the epic is closed and the phase is done; `mindspec_finalize_tip` is an ancestor of the target; and every lifecycle child bead's landed binding is an ancestor of the target. Lifecycle children are the beads that get a landed binding; other children do not count. The target is main (direct). On the push path it is the remote spec branch, read with `git ls-remote`. When the host deleted that branch after the PR merged, the target is the remote main branch. The re-run fetches the target into a temporary ref before the ancestor checks. A fetch failure or a missing object refuses with today's branch-missing recovery. A squash or rebase merge on the host is not detected and gets the same refusal. The re-run does not push a spec branch again that the host deleted. Cleanup then runs the destruction guard again and deletes only bead branches whose tips are ancestors of the target.

### R3 — Merge, then verify, then close (#250)

0. `complete` selects its path in this order:
   1. `bead/<id>` is absent: today's branch-less reconcile path, unchanged. With `--merge-only`, it refuses because there is nothing to merge, with `recovery: mindspec complete <bead>`.
   2. `--merge-only` is passed, or `mindspec_merge_only=pending` is recorded: the merge-only path (R3.1–R3.3, R3.10). It never closes the bead. When a plain `complete` takes this path only because of the pending key, it says that a merge-only run is pending and that the bead stays open, and prints `recovery: mindspec complete <bead>` to close the bead after recording. To abandon a merge-only run before its merge, the recovery is `bd update <bead> --unset-metadata mindspec_merge_only`.
   3. The bead is closed, the first-parent chain has a naming merge for the bead, and a merge is in progress in the spec worktree: it refuses with the R3.8 busy-worktree text and the recovery for that merge. It never merges. When the merge in progress is the bead's own branch (`MERGE_HEAD` equals the bead tip), the recovery is `git -C <spec worktree> merge --abort`, then the R3.6 lines.
   4. The bead is closed, the first-parent chain has a naming merge for the bead, and no merge is in progress: it first runs the panel gate at the bead tip (M's second parent), the R3.13 gates on M, and the R3.4 tip and user-changes checks. A moved tip refuses with the R3.4 text and the R3.6 recovery. User changes refuse with today's dirty-worktree recovery. An absent bead worktree passes the user-changes check. A closed bead skips the presence check (R3.5), as R3.12 does. Path 4 runs the full panel gate, unchanged, on every run, a resume included. No metadata key stands for the gate. A real resume converges for these reasons:
      - Freshness compares the reviewed SHA and subject digest with the bead tip (`internal/panel/gate.go:285-292`). A bead that passed before passes again at the same tip. A panel with no subject digest goes stale at M's second parent when the reviewed SHA differs. It needs a new panel round at the tip, and the recovery is the R3.6 step-2 re-panel line. Freshness is not loosened.
      - In the flat layout the gate reads registrations only from `<spec-dir>/reviews` (`panelGateRoots`, `internal/complete/panel_advisory.go:813-816`). Step 6 does not remove them.
      - An absent bead worktree skips the user-changes check.

      Cleanup guard for an interrupted step 6: in a layout that is not flat, the scan roots include the bead worktree (`panel_advisory.go:815-822`). A missing root gives no match and no error (`internal/panel/panel.go:279`), and no registration means the gate allows (`panel_advisory.go:188-190`). So after a partial step 6, a panel that lived only in the bead worktree would fail open. In a layout that is not flat, when the panel gate is enforced, the bead worktree is absent, and the scan finds no registration for the bead, path 4 refuses. It prints `recovery: run a new panel round for the bead at its tip`, then `recovery: mindspec complete <bead>`. With `enforcement.panel_gate: false` or the skip environment variable, path 4 behaves as today (`panel_advisory.go:183-212`). This guard is not a trust check. A worktree re-created empty on purpose is out of scope.

      Then, in every layout, it resumes steps 2–6 of R3.8. Each step is a no-op when its done signal shows that it is done. No step is skipped.
   5. The bead is closed and has no naming merge, with or without a merge in progress: today's merge path and its `--resolve-merge` re-entry, unchanged.
   6. The bead is open, the branch exists, and the first-parent chain has a naming merge for the bead: the closing path (R3.4–R3.9).
   7. Otherwise: today's merge path.

   On paths 2, 4 and 6, a `mindspec_merge_only_sha` marker that names another bead's merge, or a commit that is not a naming merge for the bead, refuses with `recovery: bd update <bead> --unset-metadata mindspec_merge_only_sha`.
1. `mindspec complete <bead> --merge-only` runs every gate that `complete` runs, materializes, and merges the bead into the spec branch. It prints the merge SHA and both parents. Before the merge, it writes `mindspec_merge_only=pending` to bead metadata. On a merge conflict, the recovery is `mindspec complete <bead> --merge-only --resolve-merge`. A plain `--resolve-merge` re-entry with the pending mode recorded also stays merge-only. When the operator committed the merge by hand, the merge-only path finds the naming merge, runs the panel gate at the bead tip and the R3.13 gates on M, and then finishes recording. A gate failure refuses and keeps the pending key. It does not close the bead.
2. After the merge, it replaces the pending key with `mindspec_merge_only_sha=<merge>`. Path selection (R3.0) reads the pending key. The marker check (R3.0) reads the SHA key. Closing step 5 clears both keys. The markers are hints. Git state decides.
3. It does not close the bead, commit the panel ledger, write the landed binding, clear the stage marker, or remove the branch or worktree.
4. A bead has at most one naming merge. A `--merge-only` or any other step that would make a second naming merge refuses (R3.10 makes no merge). The closing path refuses before any close or cleanup when:
   - the bead tip moved past the merge's second parent (the text names the new commits);
   - the bead worktree has user changes (tool-artifact paths are ignored);
   - the naming merge is not valid;
   - M's change is not present at the spec tip (R3.5).
5. The presence check covers the paths that M changes against its first parent, minus tool-artifact paths. A path is reverted when its content at the spec tip equals its content at M's first parent, or when M added it and it is absent at the spec tip. A later naming merge for another bead that changed the path exempts it. Other later content is not a revert. A reverted path makes the revert full or partial.
6. A fix found after verification goes to a follow-up bead. The refusal says this, in this order:
   1. When the bead tip moved: it lists the new commits and prints `recovery: git branch <bead>-followup <tip>`, then `recovery: git -C <bead worktree> reset --keep <second parent>`.
   2. When the latest panel round's reviewed SHA is not the bead tip, the text says to run a new panel round for the bead at its tip first. Then `recovery: mindspec complete <bead>`. Run it until it exits 0.
   3. `recovery: bd create "<title>" --parent <epic> --deps <bead>`. N is the index of the new bead.
   4. The text says to edit plan.md on the spec branch: append a section as the Nth `## Bead` heading with a concrete acceptance criterion, add a `work_chunks` entry with id N and its `key_file_paths`, and commit plan.md.
   5. `recovery: mindspec next <new bead>`.
   6. When the bead tip moved: `recovery: git -C <new bead worktree> cherry-pick <second parent>..<bead>-followup`, then `recovery: git branch -D <bead>-followup`.

   The text says that a panel round recorded at the fix tip does not carry over. The new bead gets its own panel.
7. Do not revert a `--merge-only` merge by hand. Send a rejected change to a follow-up bead. When M is reverted anyway, the refusal lists each commit that changed a reverted path after M, with its SHA, its escaped subject and the M paths it changes. For each, newest first, it prints `recovery: git -C <spec worktree> revert <commit>`, with `-m 1` for a merge commit. The text warns that a revert undoes the whole commit. As a path-scoped choice, it prints `recovery: git -C <spec worktree> checkout <M> -- <paths>` followed by a commit. It then prints the R3.6 lines from step 2 on. The restore commit is not a naming merge, so the bead keeps one naming merge.
8. Before step 1, the closing path (R3.0 path 6) checks that the spec worktree has no merge in progress and no staged changes outside the export and ledger paths. Otherwise it refuses with the recovery for that other merge. The closing path makes no commit on the bead branch. It re-runs the panel gate at the bead tip and the doc-sync, ADR and ownership gates on M (R3.13), makes no merge, and does these steps in this order:

| Step | Effect | Done signal |
|---|---|---|
| 1 | `bd close` the bead. | The bead is closed. |
| 2 | Write the landed binding and the reviewed-SHA keys for the valid merge. | The binding names M. |
| 3 | Write the doc-skew and ADR override metadata. | Each needed key is present. |
| 4 | Export the tracker and commit the export and the panel ledger on the spec branch, in the spec worktree, with a path-scoped commit. This commit publishes the close. | At the spec tip, the export shows the bead closed and the ledger paths have no diff against the worktree. |
| 5 | Clear the stage marker and both merge-only keys. | The marker and the keys are absent. |
| 6 | Remove the bead branch and worktree. | Both are absent. |

9. A failure at steps 1–4 refuses with recovery `mindspec complete <bead>`. When the run used `--allow-doc-skew` or `--override-adr`, the recovery repeats the same flag and reason, because the gates read the flags only (`internal/complete/complete.go:1662`, `:1703`). A re-run converges through R3.0 path 6 or path 4. Step 6 does not start until step 4 is done.
10. A repeated `--merge-only` when a valid merge for the current tip exists makes no new merge. It writes the marker for that merge. This is also the recovery when the marker write fails after the merge.
11. `doctor` reports a stale-open bead at Info when a valid merge exists for it, with recovery `mindspec complete <bead>`. Without a valid merge, the finding stays at Error. The marker does not change the severity. Info is a new doctor status (`internal/doctor/doctor.go:8-14` has OK, Missing, Error, Warn and Fixed). It does not count in `HasFailures` and does not change the exit code. R4.6 uses the same status. The first bead that needs it adds it. `doctor --fix` takes no action on an Info finding. The stale-open Info carries no FixFunc. The operator runs `mindspec complete <bead>` after verification. Info renders as `[INFO]`.
12. `doctor` reports, at Error, an open bead with a naming merge whose change is reverted fully or partly (R3.5). It prints the R3.7 recovery. For that bead, this is the one finding: the stale-open finding is not also reported, and `doctor --fix` takes no action. A closed bead gets no R3.12 finding.
13. When a naming merge M exists for the bead (R3.0 paths 2, 4 and 6), the doc-sync, ADR and ownership gates use base M^1, head M, and M's tree as the landing tree and `ownerRef`. This is what the branch-less reconcile path does today (`internal/complete/complete.go:983-986`). The gates never measure an empty range. A refusal on M says that a later commit does not change M, and prints the exits that converge:
   - For an ownership or doc-sync refusal only, when M is the spec tip and no landed binding is written yet: stage only the files that the refusal names (`OWNERSHIP.yaml`, or the named doc paths) in the spec worktree, then `recovery: git -C <spec worktree> commit --amend --no-edit`, `recovery: bd update <bead> --unset-metadata mindspec_merge_only_sha`, and `recovery: mindspec complete <bead>`. The amend keeps M's parents and subject, so it is still the one naming merge: naming-merge matching uses the subject and the second parent (`internal/lifecycle/landed.go`, candidate scan). Any other staged path is out of scope for this exit. The binary does not check this limit; the refusal text states it as operator guidance. The re-run gates the amended M again.
   - For an ownership refusal on a domain that owns the path: add the domain to the spec's `## Impacted Domains` in the spec worktree and commit it. The gate reads `spec.md` from disk (`internal/validate/divergence.go:140`).
   - The audited overrides that exist today: `--override-adr "<reason>"` for an ADR or ownership refusal, and `--allow-doc-skew "<reason>"` for a doc-sync refusal.

### R4 — remove `merge_strategy`

1. Delete `config.MergeStrategy`, its `auto` default and the `config show` row (`cmd/mindspec/config.go:139`). Update the field assertions in internal/config/config_test.go (lines 21, 100, 135, 305, 319, 329 and 430) and internal/config/feedback_remote_test.go (line 236). Use worktree_root with two valid relative values in TestLoadCaches_DiskMutationIgnored, so the test still checks caching and ResetCache. Use a retained project-config field in the feedback-remote scope test. Remove merge_strategy from TestCeremonyNonInflation_ConfigKeys in cmd/mindspec/ceremony_guard_test.go. Remove its mention from the defaulting comment in internal/config/config.go. Keep FinalizeResult.MergeStrategy and its readers and tests.
2. Remove the key from ordinary config writers and fixtures: internal/harness/sandbox.go, internal/harness/lifecycle_scenario_test.go, cmd/mindspec/verb_output_test.go, internal/config/config_test.go, internal/config/feedback_remote_test.go, internal/doctor/config_test.go and internal/doctor/models_commands_test.go. Keep the key only in fixtures that test legacy-key loading or detection under R4.5 and R4.6. Remove `merge_strategy: pr` from this repo's `.mindspec/config.yaml`. Update the config example in .mindspec/core/GIT-WORKFLOW.md and the Config description in .mindspec/domains/core/interfaces.md. Add dated removal notes to the merge_strategy guidance in ADR-0006 and ADR-0019. Historical specs, plans, review records and tracker exports remain historical evidence.
3. `impl approve` keeps today's rule: push when a git remote exists, else merge locally.
4. `ApproveImpl` decides has-remote one time, in preflight, and passes the result to `FinalizeEpic` as a parameter. `FinalizeEpic` does not call `gitutil.HasRemote()` again. The early guards (R2.1), the push probe (R2.3) and `FinalizeEpic` use this one value. The executor-internal `result.MergeStrategy` field follows the passed value. Update the two-probe comments on `HasRemote` (`internal/lifecycle/gitquery.go:22-34`) and on `implHasRemoteFn` and the preflight gate (`internal/approve/impl.go:124-135`, `:397-416`) to describe the one decision.
4a. The preflight decision uses a probe that returns an error. Today an error reads as no remote (`internal/gitutil/gitops.go:411-417`). Add an error-returning probe in gitutil (for example `RemoteConfigured() (bool, error)`) behind the lifecycle wrapper and `implHasRemoteFn`. `gitutil.HasRemote()` and its other callers do not change. A probe error refuses before the --supersede-adr placeholder write, the phase-reconcile write and MUTATION 1/3 (the EnsureMigrated exemption stays first): `cannot tell whether a git remote is configured: <err>`, then `recovery: git remote -v`.
5. A config that still has `merge_strategy` loads. The loader uses `yaml.Unmarshal` from `gopkg.in/yaml.v3` without `KnownFields` (`internal/config/config.go:551`), so it ignores unknown keys.
6. `doctor` reports one Info when `.mindspec/config.yaml` has a top-level `merge_strategy` key: `merge_strategy is no longer used; remove it from .mindspec/config.yaml`. The check is a line regex, the same as the existing key checks (`internal/doctor/config.go:54`).
7. `scenario_finalize_pr` keeps its push leg.

### R5 — Reviewed-tree datum (#233)

1. The reviewed set R comes from the panel registrations (`reviewedHeadSHAForBead`). The `mindspec_landed_reviewed_sha` key counts only when it is in that set. Otherwise the provenance calls it unverified, and it does not confirm.
2. In `FindLandedMerge`, the panel datum confirms a candidate with second parent D when D is in R (today's rule), or when some r in R is an ancestor of D and `git diff --no-renames --name-only -z r D` lists no path or only tool-artifact paths. The tool-artifact set is `panel.ToolArtifactPaths` for the spec directory of the bead's own epic.
3. The datum never vetoes and never overrides a tip or binding contradiction. The revert check still runs after it.
4. The datum is not evaluated on the reachable-fallback path.
5. The provenance string names the rule that matched: `exact` or `reviewed-tree`.

Safety: subject text still nominates ownership, and topology still proves landed-ness. The new rule accepts only a D that adds tool-written files to a reviewed tree. It confirms nothing that an exact match would not confirm for reviewed content. A panel ledger change such as `dispositions.jsonl` in D is safe: it records review results and changes no code, and the panel gate reads it separately. A tool-artifact path of another spec does not match.

### R6 — Ancestor without a naming merge (#251)

1. Reattest evaluates R6 only after the primary scan and the reachable fallback both find no candidate.
2. When `bead/<P>` is absent, the outcome stays `no-owned-exact-merge`. A git read error refuses as indeterminate and makes no claim.
3. P's tip must be an ancestor of the spec tip. Otherwise the outcome stays `no-owned-exact-merge`, with no history claim. The base B is the newest commit on the spec branch's first-parent chain that is an ancestor of the `bead/<P>` tip. P's range is B..P-tip. When B cannot be found, the state is unknown, with no override recommendation. When the range is empty, the outcome stays `no-owned-exact-merge`.
4. When P's tip equals the second parent of another bead's naming merge, the text says "not P's own work", with no override recommendation. Limit: a branch restored at a commit strictly inside another bead's range is not told apart from P's own work. The person-must-verify sentence (R6.6) covers this case.
5. Otherwise reattest checks the range at the spec tip with base-taking helpers: the content check (`ContentSubsumedOutcome` with base B, head P-tip, target spec tip) and a new revert check. The new check takes a base, a head and a target, and reports whether the target reverts any change in base..head.
6. Content present and not reverted: state `ancestor-without-naming-merge`. The text says that P's changes are present on the spec branch, that no merge names P, that a re-merge cannot help, and that a person must verify the work before using the override.
7. Content reverted or indeterminate: the outcome stays `no-owned-exact-merge`. One more line says that P's commits are in the spec history, and that the tool cannot show that their changes are present. It makes no claim that the work is present.
8. The R6.6 recovery has one line for each open bead in the same epic that depends on P: `mindspec next <dependent> --allow-not-ready=MF-3 --evidence <file>`. With zero dependents, the text says that no bead waits on P, and the recovery is `mindspec doctor`.
9. `doctor` reports P itself, at Warn, when P has no landed binding and R6.6 applies. It prints the R6.8 recovery lines.
10. Reattest gets no operator nomination.
11. Regression guards: error-injection tests cover the three existing `fallbackSafety` reads (forward content, reverse content, branch tip). Each read error refuses. A separate test covers a read error on the new R6 path.

### R7 — One lineage (wtjr, fdfg.6)

1. A bead's lineage is its one dependency with `dependency_type` parent-child on an epic. A `blocks` edge to an epic is not lineage.
2. More than one parent-child epic refuses with recovery `bd dep remove <bead> <epic>`.
3. The bead-title fallback in `FindEpicForBeadWithCache` goes.
4. `internal/phase` returns the lineage provenance with the epic. `next` uses it and runs no second `bd show` per plan row.
5. `spec approve` writes the full spec ID to epic metadata `spec_id`.
6. Epic-to-spec resolution has three sources: `spec_id`, then `spec_num` plus `spec_title`, then the epic's own `[SPEC <id>]` title parsed as a full spec ID with any letter. A lettered ID resolves to its own spec, never to the base number. Rules:
   - A `spec_id` that passes `idvalidate.SpecID` decides. The other sources are not compared.
   - A `spec_id` that fails `idvalidate.SpecID` refuses for that epic. Resolution does not fall through.
   - Without `spec_id`, `spec_num` 0 or an empty `spec_title` counts as absent. When the legacy pair and the title ID disagree, resolution refuses and prints `recovery: bd update <epic> --set-metadata spec_id=<id>` with the exact title ID. It never picks the plain ID. After the repair, `spec_id` decides.
   - When no source gives an ID, resolution refuses with the same recovery. The text says to use the spec directory name as the ID.
   - When more than one epic resolves to one spec ID, a verb that resolves that spec refuses and names each epic. `doctor` reports it and continues. Commands that list all epics skip a bad epic with a warning, as today (`internal/phase/derive.go:558-569`). The live tracker has closed duplicate epics for test specs 001–006 and two `[PLAN 008b-human-gates]` epics. These block no verb that does not resolve their spec. Historical epics with no ID source (seven `[PLAN …]` epics and two others) stay skipped in listings with no new warning, as today.
7. `beadIndex` accepts a bead ID only when it equals `<epicID>.<N>`.
8. `doctor` lists, at Warn, every open bead that resolves only through the removed bead-title rule. Its recovery is `bd update <bead> --parent <epic>`. Doctor uses the title parse only for this diagnosis.
9. `spec create` refuses a new lettered spec ID such as `129b-x`. Existing lettered directories stay valid. No directory is renamed.

### R13 — Text amendments (this spec's half)

1. ADR-0041 and ADR-0036 get a "Spec 132 amendment" section with the R1, R2, R3 and R5 changes.
2. Spec 129 gets dated status notes: the "Deferred to the follow-on spec" list names spec 132 for G, H, #250, I and lineage; AC-11 says that advisories never change the exit code, the binding or the audited tuple; AC-1 quotes the emitted diagnostic; the plan verification command sets `-timeout=20m`.
3. `merge-safety.md` and the workflow runbook name the attested-restore exit, the R6 states, `--merge-only`, the one-naming-merge rule, "do not revert a `--merge-only` merge by hand", and the restore-then-close recovery.

## Deferred to follow-on spec

These items move to one follow-on spec. Each carries the round-1 corrections below. That spec builds on PR #266 (`fix/resolve-merge-hook-next-bead-line`), which must reach main first.

- **R8 parallel-safe advice.** Templates, `complete` and `plan approve` say "start the next bead in a fresh context or a fresh agent with `mindspec next`". The freshness-gate refusal names the parallel-dispatch path and drops "Do NOT attempt workarounds". The session file records each warned reason with `SessionStartedAt`. A new reason warns again. The scan excludes `internal/setup/historical_skills/` by name. `internal/harness` STOP-after-complete scenarios are in scope. Resolved 2026-10-21 (operator ruling): warn once per session per reason, together with the refusal-text change.
- **R9 spec-named tail.** `spec create`, `spec approve`, `plan approve` and `next` render the tail for their spec through the `instruct --spec` resolution (`cmd/mindspec/spec.go:79,226`, `plan_cmd.go:81`, `next.go:420,425`).
- **R10 readiness diagnostics** (qdoh 1–5, fdfg.4, 4hdt, e07y.3). Lineage and scope checks before file I/O. Hide the bare-override sentinel. One line per signal, with each MF-3 recovery as its own escaped `recovery:` line. `ApplyOverride` addresses lines by signal ID. Recovery lines are commands with no prose and no parentheses; the AC names each refusal case. `bead ready-check` on a bead that is not in_progress and has a legacy marker prints `recovery: mindspec release <bead>`. The loud-empty refusal prints `recovery: mindspec next --spec <id>`.
- **R11 CommitPaths** (0a84, 5x3j). Stage into a temporary `GIT_INDEX_FILE` copy and commit from it, so any failure before the commit leaves the real index unchanged. Cover the nil-paths mode. Refuse an index with unmerged entries. Escape git stderr. The `ls-files` exit-code leg and the bootstrap dry-run leg are regression guards. The spec 129 R8.2 and plan step 6 notes move with R11. R11 gets its own bead.
- **R12** (m0ww.1, dvw3). A ready-list decode failure in `advanceState` takes the indeterminate path. `next` runs `checkUnmergedBeads` for the spec that the gate learns.

## Scope

### In Scope
- R1–R7 and R13 in the packages and documents in Impacted Domains.
- The removal of `merge_strategy` from this repo's `.mindspec/config.yaml` (R4.2).

### Out of Scope
- The deferred items above.
- PR #266: `--resolve-merge` and the pre-commit hook, the empty "Next bead ready:" line, and the worktree redirect after `complete`. R3 edits `complete.go` near that change and rebases on it if it lands first.

## Non-Goals

- Operator nomination of landed ownership (#251 option 3).
- A verification hook between merge and close (#250 option 3).
- Automatic lineage migration from titles. Writing lineage from a title would keep titles as an authority.
- Test-only hygiene: e07y items 2–9, fdfg items 1, 2, 3 and 5, m0ww items 2 and 3, fl6d, 79os, 1x70. One chore bead holds them.
- Renaming lettered spec directories.

## Acceptance Criteria

- [ ] **AC-1 (R1.1–R1.3)**: A claim added at the spec tip after the bead forks makes `complete` pass in both legs. A claim revoked at the spec tip makes `complete` refuse with `adr-divergence-unowned`. Tests: `TestCommittedRangeGate_SpecTipClaimOwns`, `TestMaterialize_SpecTipClaimOwns`, `TestCommittedRangeGate_RevokedAtSpecTipRefuses`, `TestCheckUnclaimedSource_LandingTree`.
- [ ] **AC-2 (R1.4, R1.5, R3.13)**: A conflict on a source file only passes the gate, `MergeInto` keeps the conflict, and `--resolve-merge` then completes with ownership read from the resolved index. A conflict on `OWNERSHIP.yaml` between two beads that add claims to one domain makes plain `complete` refuse before the close, with the bead open. The printed `--merge-only` recovery, then the printed conflict template (`git add`, `git commit --no-edit`), then re-entry, then plain `complete` pass. A resolution that revokes a needed claim refuses with the R3.13 exits, and the amend exit then passes. Tests: `TestCommittedRangeGate_SourceConflictPassesThenResolves`, `TestComplete_OwnershipConflictRefusesBeforeClose`, `TestComplete_OwnershipConflictMergeOnlyLoop`, `TestComplete_OwnershipResolutionRevokesClaimRefuses`.
- [ ] **AC-3 (R2.1)**: A layout-guard failure and a destruction failure each exit non-zero with the epic open and the phase unchanged, including from a stale-phase fixture. Tests: `TestApproveImpl_EarlyLayoutGuardRefuses`, `TestApproveImpl_EarlyDestructionRefuses`, `TestApproveImpl_EarlyGuardBeforePhaseReconcile`.
- [ ] **AC-4 (R2.2)**: An auto-merge in `FinalizeEpic` makes the spec-to-main merge destructive after the early check passed. The executor guard still refuses. Test: `TestFinalizeEpic_ExecutorGuardAfterAutoMerge`.
- [ ] **AC-5 (R2.3–R2.5)**: Through a push-probe seam, an HTTPS auth failure refuses before the close and names the credential helper. A non-fast-forward failure refuses and does not name it. A file remote with `GIT_CONFIG_GLOBAL=/dev/null` and no helper passes. The probe argv has `--dry-run --no-verify` and the `PushBranch` remote and refspec. Tests: `TestApproveImpl_PushProbeAuthFailureNamesHelper`, `TestApproveImpl_PushProbeNonFastForwardNoHelperHint`, `TestApproveImpl_PushProbeFileRemoteNoHelperPasses`, `TestPushProbe_ArgvMatchesPushBranch`.
- [ ] **AC-6 (R2.7, R2.8)**: An injected spec-to-main conflict after the close prints "epic closed", the stage, and the `--resolve-merge` recovery last, with no added `impl approve` line. An injected close failure prints "epic still open". An inner error with no recovery gets `recovery: mindspec impl approve <spec>`. Tests: `TestApproveImpl_PostCloseConflictKeepsInnerRecovery`, `TestApproveImpl_StateLineReadsEpicStatus`, `TestApproveImpl_PostCloseBareErrorGetsRerun`.
- [ ] **AC-7 (R2.9, R2.10)**: For each `finalizeStepHook` stage, `orphan_finalize` included, on both paths, a failed run then a re-run lands the spec with one close and exit 0. Each remote ref is pushed at most once per converged state, and the carrier re-run pushes the same head branch. A fixture with a finalize-time auto-merge shows that `mindspec_finalize_tip` equals the merged or pushed tip. Tests: `TestApproveImpl_RerunConvergesFromEachStage` (table over stages and paths; runs both calls), `TestFinalizeEpic_FinalizeTipIsPublishedTip`.
- [ ] **AC-8 (R2.11, R2.12)**: A source commit that fails doc-sync, added to the spec branch between the calls, makes the re-run refuse. After a real landing and spec-branch deletion, the re-run prints "already landed". After the host deletes the remote spec branch, the re-run reads remote main and prints "already landed". A failed fetch of the target refuses with the branch-missing recovery. A forged `mindspec_finalize_tip` (main HEAD or the fork point) with a deleted spec branch and an unlanded bead branch refuses, and the bead branch survives. Tests: `TestApproveImpl_RerunReRunsGates`, `TestApproveImpl_AfterDeletionAlreadyLanded`, `TestApproveImpl_RemoteSpecBranchDeletedUsesMain`, `TestApproveImpl_TargetFetchFailureRefuses`, `TestApproveImpl_ForgedFinalizeTipRefuses`.
- [ ] **AC-9 (R3.0–R3.3, R3.13)**: `--merge-only` leaves the bead open, the branch and worktree present, no binding, no ledger commit, and the marker written. A hand naming merge with no marker takes the closing path. A marker that names another bead's merge refuses with the unset recovery, also when a valid merge for the bead exists. A branch-less bead takes the reconcile path unchanged. A closed bead whose merge finished outside `complete` resumes steps 2–6 and gets the ledger commit, the override metadata and the export. A `--merge-only` that conflicts, resumed with `--merge-only --resolve-merge` or with plain `--resolve-merge`, leaves the bead open. A `--merge-only` conflict committed by hand, then `--merge-only --resolve-merge`, records the SHA key and leaves the bead open. A plain `complete` conflict, then `--resolve-merge`, merges and finishes. A failure after the close and before the merge, then a re-run, merges and writes the binding, export and ledger, cleans up, and makes no second close. A closed bead with a naming merge refuses on a moved tip, on user changes, and when another merge is in progress. A hand naming merge that adds an unowned source file refuses on the closing path with the R3.13 exits, and the Impacted Domains exit then passes. A closed bead with a hand naming merge and a registered panel that does not pass at the bead tip refuses. With no registered panel, path 4 allows as today. A forged `mindspec_landed_reviewed_sha` equal to the tip does not skip a blocking panel: non-zero exit, no binding, no export commit, and the branch survives. In a layout that is not flat (canonical or legacy fixture), an absent bead worktree with no registration found refuses; with `enforcement.panel_gate: false` it does not refuse. The fail-open leg uses a flat-layout fixture. A closed bead whose own merge is in progress gets the `merge --abort` recovery. `--merge-only` on a branch-less bead refuses. A plain `complete` with only the pending key says why and leaves the bead open. Tests: `TestComplete_ClosedBeadMovedTipRefuses`, `TestComplete_ClosedBeadUserDirtRefuses`, `TestComplete_ClosedMergedForeignMergeInProgressRefuses`, `TestComplete_ClosingPathGatesMergeRange`, `TestComplete_ClosedHandMergeStalePanelRefuses`, `TestComplete_ClosedHandMergeNoPanelFailsOpen`, `TestComplete_ClosedHandMergeForgedReviewedKeyRefuses`, `TestComplete_ClosedNonFlatAbsentWorktreeRefuses`, `TestComplete_ClosedNonFlatAbsentWorktreeGateOffAllows`, `TestComplete_ClosedOwnMergeInProgressAborts`, `TestComplete_MergeOnlyBranchlessRefuses`, `TestComplete_PendingKeyPlainCompleteStaysOpen`, `TestComplete_MergeOnlyKeepsBeadOpen`, `TestComplete_MergeOnlyHandCommitStaysOpen`, `TestComplete_PlainConflictResolveFinishes`, `TestComplete_ClosedUnmergedRerunMerges`, `TestComplete_HandMergeTakesClosingPath`, `TestComplete_ForeignMarkerRefuses`, `TestComplete_BranchlessReconcileUnchanged`, `TestComplete_ClosedBeadResumesAllSteps`, `TestComplete_MergeOnlyConflictStaysMergeOnly`.
- [ ] **AC-10 (R3.4–R3.7)**: Each refusal leaves the bead open and the branch and worktree present: a bead commit after `--merge-only` (named in the text), user changes in the worktree (tool-artifact changes alone pass), a full revert, a partial revert, and a second `--merge-only`. A subject with control characters prints escaped. A later naming merge for another bead that changes one of M's paths exempts only that path. A later plain commit that edits an M path to new content is not a revert. The restore refusal prints `-m 1` for a merge commit and the path-scoped checkout. Follow-up loop: the plan has no section and no chunk for the new bead; `--merge-only`, a fix commit, and a second `--merge-only` refuses; a panel round is recorded at the fix tip; only the printed steps run, including the printed new panel round; `mindspec next` dispatches the follow-up bead with no MF-1 override; the follow-up bead completes. Restore loop: a full revert and a partial revert each run the printed recovery. After each loop, `doctor` is clean for the bead, the dependents pass MF-3, and `impl approve` exits 0. Tests: `TestComplete_BeadAdvancedAfterMergeOnlyRefuses`, `TestComplete_UserDirtAfterMergeOnlyRefuses`, `TestComplete_RevertedMergeOnlyRefuses`, `TestComplete_PartlyRevertedMergeOnlyRefuses`, `TestComplete_PresenceCheckLaterMergeExemptsPath`, `TestComplete_PresenceCheckLaterEditNotRevert`, `TestComplete_SecondMergeOnlyRefuses`, `TestComplete_MergeOnlyRefusalEscapes`, `TestComplete_MergeOnlyFollowUpLoop`, `TestComplete_MergeOnlyRestoreLoop`.
- [ ] **AC-11 (R3.8, R3.9)**: The closing path makes no new merge and no bead-branch commit. The spec branch's last commit has an export that shows the bead closed, and it lands before the branch removal. A spec worktree with another merge in progress refuses before the close. An injected export-commit failure keeps the branch and worktree, and a re-run converges. An injected step-5 failure and a partial step 6 (worktree absent, branch present) each finish on a re-run. A failed run with `--override-adr "<reason>"` prints a recovery with the same flag and reason, and that recovery finishes. Tests: `TestComplete_AfterMergeOnlyPublishesClose`, `TestComplete_ClosingPathSpecWorktreeBusyRefuses`, `TestComplete_AfterMergeOnlyExportFailureConverges`, `TestComplete_ClosedBeadFinishesCleanup`, `TestComplete_OverrideRunRecoveryRepeatsFlag`.
- [ ] **AC-12 (R3.10)**: After an injected marker-write failure, a repeated `--merge-only` writes the marker and makes no new merge. Test: `TestComplete_RepeatedMergeOnlyNoNewMerge`.
- [ ] **AC-13 (R3.11, R3.12)**: `doctor` gives Info for a stale-open bead with a valid merge. An open bead with a fully or partly reverted naming merge gets one finding, the R3.12 finding with the restore recovery, and `doctor --fix` runs nothing for it. A closed bead whose merge is reverted later gets no R3.12 finding. `doctor --fix` on a stale-open bead with a valid merge runs nothing and the bead stays open. An Info check prints `[INFO]`. Tests: `TestDoctor_StaleOpenSeverityFromGitState`, `TestDoctor_RevertedMergeOnlyOneFinding`, `TestDoctor_FixSkipsRevertedMergeOnly`, `TestDoctor_ClosedRevertedNoFinding`, `TestDoctor_FixSkipsStaleOpenInfo`, `TestDoctorStatusTag_Info`.
- [ ] **AC-14 (R4.1, R4.2, R4.5, R4.6)**: A config with `merge_strategy: pr` loads with no error. `config show` has no `merge_strategy` row. The harness sandbox config has no `merge_strategy` key. `doctor` gives the R4.6 Info for a config with the key, and nothing for one without it. The Info does not make doctor exit non-zero. No Go source, test fixture or this repo's `.mindspec/config.yaml` writes `merge_strategy`, except the legacy-key tests (R4.2), the doctor check and its test. The exemptions are named test functions or named fixture constants, not whole files. A `# merge_strategy:` line and an indented `  merge_strategy:` key give no Info. Tests: `TestConfigLoad_LegacyMergeStrategyIgnored`, `TestConfigShow_NoMergeStrategyRow`, `TestHarnessSandbox_NoMergeStrategy`, `TestDoctor_LegacyMergeStrategyInfo`, `TestNoMergeStrategyWriters`, `TestDoctor_LegacyMergeStrategyCommentedOrNested`.
- [ ] **AC-15 (R4.3, R4.4, R4.4a)**: `FinalizeEpic` behavior is unchanged with and without a remote: with a remote it pushes, and with none it merges locally. `FinalizeEpic` uses the passed has-remote value and makes no `git remote` call of its own. A real-git repo with a remote configured, given has-remote=false, merges locally and makes no push. A repo with no remote, given has-remote=true, takes the push leg and does not merge into main. `TestApproveImpl_OneHasRemoteDecision` asserts that the probe runs exactly one time and that the mock's `FinalizeEpic` receives that value. An injected probe error, from a stale-phase fixture run with --supersede-adr, refuses with the epic open, the stored phase unchanged and no placeholder ADR file. Tests: `TestFinalizeEpic_WithRemotePushes`, `TestFinalizeEpic_NoRemoteMergesLocally`, `TestFinalizeEpic_PassedFalseWithRemoteMergesLocally`, `TestFinalizeEpic_PassedTrueNoRemoteTakesPushLeg`, `TestApproveImpl_OneHasRemoteDecision`, `TestApproveImpl_HasRemoteProbeErrorRefuses`.
- [ ] **AC-16 (R5.1–R5.5)**: A branch-less bead whose D adds only tool-artifact paths to a reviewed r is identified on the `complete` reconcile path, with provenance `reviewed-tree`. This fails on the old code. An empty wrapper commit on r also confirms. A D that also changes a source file, renames a source file to a tool-artifact path, or changes another spec's tool-artifact path is not confirmed. A metadata key that is not in R does not confirm. The fallback path ignores the datum. Tests: `TestFindLandedMerge_ReviewedTreeConfirms`, `TestFindLandedMerge_ReviewedTreeEmptyWrapperConfirms`, `TestFindLandedMerge_ReviewedTreeSourceDiffNoConfirm`, `TestFindLandedMerge_ReviewedTreeRenameNoConfirm`, `TestFindLandedMerge_ReviewedTreeOtherSpecDirNoConfirm`, `TestFindLandedMerge_UnregisteredKeyNoConfirm`, `TestReattestFallback_IgnoresPanelDatum`.
- [ ] **AC-17 (R6.1–R6.6)**: P's commits arrive inside a later bead's merge with content present, so reattest returns `ancestor-without-naming-merge` with the person-must-verify sentence. Test: `TestReattest_AncestorWithoutNamingMerge`.
- [ ] **AC-18 (R6.2–R6.5, R6.7)**: A zero-commit `bead/<P>`, a branch restored at a first-parent commit, a deleted branch, and a tip that is not an ancestor of the spec tip each give `no-owned-exact-merge`. Paired real-git fixtures: a P tip strictly before another bead's second parent reaches the content check and R6.6; a P branch restored exactly at another bead's tip, and a zero-commit branch cut from it, give "not P's own work". A revert of P's earlier commit, and fully reverted content, each give `no-owned-exact-merge` with the history line and no "present" claim. A missing base gives unknown. An injected git read error on the new path refuses as indeterminate. Tests: `TestReattest_AncestorZeroCommitsNoClaim`, `TestReattest_AncestorFirstParentRestoreNoClaim`, `TestReattest_AncestorDeletedBranch`, `TestReattest_AncestorNotInSpecTip`, `TestReattest_AncestorInsideOtherRangeChecksContent`, `TestReattest_AncestorOtherBeadTipNotOwn`, `TestReattest_AncestorEarlierCommitReverted`, `TestReattest_AncestorRevertedHistoryOnly`, `TestReattest_AncestorNoBaseUnknown`, `TestReattest_AncestorReadErrorIndeterminate`.
- [ ] **AC-19 (R6.8, R6.9)**: Two dependents give two recovery lines. Zero dependents give `recovery: mindspec doctor`. `doctor` names P, not its dependent. Tests: `TestReattest_AncestorRecoveryPerDependent`, `TestReattest_AncestorZeroDependents`, `TestDoctor_AncestorNamesPrerequisite`.
- [ ] **AC-20 (R6.11, regression guards)**: An injected error on each of the three existing `fallbackSafety` reads refuses. Tests: `TestFallbackSafety_ForwardContentErrorRefuses`, `TestFallbackSafety_ReverseContentErrorRefuses`, `TestFallbackSafety_BranchTipErrorRefuses`.
- [ ] **AC-21 (internal refs)**: The reattest refusals, the reattest `Short` text, the two `materialize.go` messages, the R4.4a refusal and the R4.6 Info text match none of `ADR-[0-9]`, `q9ea` or `[Ss]pec [0-9]{3}`. The no-owned refusal contains "attested restore" and a plain sentence that a person must verify the merge first. Recovery lines are commands with no parentheses. A fixture bead `mindspec-ab12` and its `bead/mindspec-ab12` branch render in their operand positions. Tests: `TestReattestText_NoHistoricalCitations`, `TestReattestText_OperandsRender`, `TestMaterializeText_NoHistoricalCitations`.
- [ ] **AC-22 (R7.1–R7.3)**: A `blocks` edge to a foreign epic is not lineage. Two parent-child epics refuse. A `[129]` title with no parent-child edge has no lineage. Tests: `TestFindEpicForBead_BlocksDepIgnored`, `TestFindEpicForBead_TwoParentsRefuse`, `TestFindEpicForBead_NoTitleFallback`.
- [ ] **AC-23 (R7.4)**: A foreign-lineage plan row costs one `bd show`. Test: `TestBuildReadySet_OneShowPerPlanRow`.
- [ ] **AC-24 (R7.5, R7.6)**: With epics for `129-x` and `129b-y` both present, each bead resolves to its own spec, and `next`, `complete` and `plan approve` act on `129b-y`. A `[SPEC 008b-human-gates]` epic with legacy `spec_num=8`, `spec_title=human-gates` and no `spec_id` refuses with the `--set-metadata spec_id=008b-human-gates` recovery. A second call, after the printed repair, resolves through `next`, `complete` and `plan approve`. A title with no ID and no other source refuses with the same recovery. `spec_num` 0 counts as absent. An invalid `spec_id` refuses. Two epics on one spec, one closed, refuse in `next` for that spec and name both, while `doctor` reports and continues and a listing of all epics warns and skips. Tests: `TestResolveSpec_PlainAndLetteredBothPresent`, `TestLifecycle_LetteredSpecResolves`, `TestResolveSpec_LegacyPairRepairResolves`, `TestResolveSpec_TitleWithoutIDRefuses`, `TestResolveSpec_SpecNumZeroAbsent`, `TestResolveSpec_InvalidSpecIDRefuses`, `TestResolveSpec_TwoEpicsScopedRefusal`.
- [ ] **AC-25 (R7.7–R7.9)**: `beadIndex` rejects `mindspec-x.3` for epic `mindspec-y`. `doctor` lists a title-only open bead with the `bd update --parent` recovery. `spec create 129b-x` refuses, and `idvalidate` accepts `008b-human-gates`. Tests: `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID`.
- [ ] **AC-26 (R13)**: Each ADR and spec-129 note exists, and `mindspec validate spec 129-readiness-tracker-honesty` passes. Test: `TestDocs_Spec132Amendments`.

## Validation Proofs

- `go build ./...`: exit 0.
- `go test ./... -short -timeout 20m`: PASS.
- `golangci-lint run`: no new findings.
- `mindspec validate spec 132-readiness-followon`: PASS.
- Opt-in, separate: the live-agent harness suite for the finalize scenarios.

## Open Questions

- [x] **`merge_strategy`: honour or delete?** Resolved 2026-10-21 (operator ruling): (b) delete the key and its `config show` row, and keep the has-remote rule. R4 is the deletion.
- [x] **#250 shape.** Every option keeps one naming merge per bead (R3.4): a fix found after verification goes to a follow-up bead, and the bead closes over its first merge. Do not revert a `--merge-only` merge by hand; when it happens, the restore-then-close recovery applies. (a) `--merge-only` plus the git-state severity rule, as R3 says. (b) No new flag. Document "merge by hand, then `complete`" as supported. `doctor` gives Info when a valid merge exists (R3.11 rule), with its own AC: a hand naming merge whose second parent is the bead tip gives Info, and a reverted one gives Error. (c) Only `--merge-only`, with Info gated by the marker. Recommended: (a). It gives a verified tool path, and R3.11 also covers hand merges. Option (c) trusts an agent-writable marker. Resolved 2026-10-21 (operator ruling): (a).
- [x] **Confirm the split.** R8–R12 move to one follow-on spec, as "Deferred to follow-on spec" says. Recommended: confirm. Spec 132 is then about 9 beads: R1, R2 (two beads), R3 (two beads), R4, R5, R6, R7. Resolved 2026-10-21 (operator ruling): confirmed.
- [x] **Freshness warning (follow-on spec).** (a) Warn once per session per reason. (b) A dispatch flag on `next` that an orchestrator sets. Recommended: (a), together with the refusal-text change. With (a) alone, the gate still refuses fresh agents. Resolved 2026-10-21 (operator ruling): (a), together with the refusal-text change. The follow-on spec implements it.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
