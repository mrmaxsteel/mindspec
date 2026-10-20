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
- `merge_strategy` controls how `impl approve` lands a spec.
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
- **`merge_strategy` is dead.** `config.MergeStrategy` (`internal/config/config.go:20`, default `auto`) is read only by `config show`. `FinalizeEpic` decides from `gitutil.HasRemote()` (`:822`), and `ApproveImpl` uses a separate `implHasRemoteFn`. The harness sandbox writes `merge_strategy: direct` (`internal/harness/sandbox.go:63`, `lifecycle_scenario_test.go:205`). This repo sets `pr`.
- **#233 (reviewed tree).** Spec 130 writes `mindspec_landed_reviewed_sha`. Nothing reads it. In `FindLandedMerge` (`internal/lifecycle/landed.go:479`), a reviewed SHA confirms only when it equals the merge's second parent (`reviewedHeadSHAForBead`, `:731`). `complete` lands commits after the panel SHA, so this datum never confirms on the normal path. A branch-less bead then gets "panel set non-corroborating" (`:655`).
- **#251 (ancestor without naming merge).** Reattest nominates ownership only from a merge subject. When bead P's commits reached the spec tip inside a later bead's branch, reattest returns `no-owned-exact-merge` (`internal/lifecycle/reattest.go:191`). `doctor` reports the downstream bead, not P (`internal/lifecycle/scan.go:173`).
- **Internal refs.** The `no-owned-exact-merge` text, its CLI render (`cmd/mindspec/reattest.go:286,289`), the reattest `Short` text (`:49`) and two `materialize.go` messages (`:603,636`) print ADR numbers or a bead ID.
- **Lineage (wtjr, fdfg.6).** `FindEpicForBeadWithCache` (`internal/phase/derive.go:939`) accepts any dependency whose issue type is epic, including a `blocks` edge, and the first one wins. Then it falls back to a `[NNN]` bead-title parse with `Sscanf %d`. `parseSpecID` (`internal/approve/spec.go:423`) uses `Atoi`, so spec `129b` gets `spec_num 0`, and `FindEpicForBeadWithCache` skips that epic. `beadIndexRe` (`internal/validate/readiness/readiness.go`) accepts any trailing `.N`. `next` runs a second `bd show` per foreign-lineage plan row because `internal/phase` returns no provenance. Six open beads in this repo have a `[NNN ...]` title and no epic dependency.

Already fixed: 3gvd (`.cache/` is in `.gitignore:87`; nothing under `.cache/` is tracked).

## Impacted Domains

- **workflow**:
  - `internal/complete/**`: landing-tree ownership (R1), `--merge-only` and the closing call (R3).
  - `internal/approve/**`: early guards, state line, re-run (R2); strategy preflight (R4); `spec_id` metadata (R7).
  - `internal/lifecycle/**`: strategy resolver (R4), reviewed-tree datum (R5), ancestor states (R6).
  - `internal/validate/**`: landing-tree ownership (R1), `beadIndex` (R7).
  - `internal/doctor/**`: stale-open severity (R3), strategy notice (R4), ancestor finding (R6), title-only lineage (R7).
  - `internal/next/**`: lineage provenance (R7).
  - `cmd/mindspec/**`: `complete --merge-only` (R3), reattest text (R6), `spec create` refusal (R7).
- **execution**: `internal/executor/**` (strategy parameter, finalize tip record, stage resume: R2, R4); `internal/harness/**` (fixtures: R4).
- **core**: `internal/phase/**` (lineage and spec-ID resolution: R7); `internal/config/**` (strategy values: R4).

Documentation surfaces: ADR-0041 and ADR-0036 amendments (R13); `.mindspec/domains/workflow/{architecture,runbook}.md`; `project-docs/user/guides/merge-safety.md`; spec 129 status notes (R13).

Sequencing with the panel-execution lane (spec 131): both lanes touch `internal/config/**` and `internal/executor/**`. The bead that adds R4 lands after spec 131's config changes, or rebases on them. R4 does not change panel config loading or its source reporting. Panel commands and panel skills stay in spec 131. Each bead validates against the landed panel lane before `complete`.

## ADR Touchpoints

- [ADR-0041](../../adr/ADR-0041-gate-before-mutate.md) — AMENDED (R13.1): early guard copies, the state line and stage resume for `impl approve` (R2). `--merge-only` is a commit phase with no close, and the closing call publishes the close (R3). The Spec-129 amendment (ii) changes: the reviewed-tree rule is a confirming datum in `FindLandedMerge` (R5). It never vetoes.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) — AMENDED (R13.1): `complete` reads ownership at the landing tree (R1).
- [ADR-0025](../../adr/ADR-0025-jsonl-as-build-artifact.md) — APPLIED: the spec branch's export shows the close before evidence is deleted (R3.6).
- [ADR-0023](../../adr/ADR-0023.md) — APPLIED: lineage is a bd parent-child dependency (R7).
- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) — APPLIED: strategy, probe and stage git work stays in the executor (R2, R4).
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) — APPLIED: every new refusal ends with a `recovery:` line (R2, R3, R4, R6, R7).

Terms:

- *Spec tip*: the commit at `spec/<id>` when the verb runs.
- *Landing tree*: `git merge-tree --write-tree --merge-base=<B>` of the spec tip and the bead commit (or the materialized bead tree T), through the net-effect primitive (`internal/gitutil/neteffect.go:147`). B is `git merge-base <spec tip> <bead HEAD>` for both legs. merge-tree writes an output tree also when paths conflict.
- *Naming merge*: a merge on the spec branch's first-parent chain whose subject names `bead/<id>`.
- *Valid merge M for bead B*: M is reachable from the spec tip; M is a naming merge for B; M's second parent equals the current `bead/<B>` tip; and the landed-evidence revert check (`FindLandedMerge` R5(d) rules) finds M not reverted at the spec tip; and M's change is fully present at the spec tip (no partial revert, R3.4).
- *Tool-artifact paths*: the set that `panel.ToolArtifactPaths` returns.

## Requirements

Every new refusal prints full commit SHAs and escapes subjects, IDs and inner errors with `termsafe`.

### R1 — Ownership at the landing tree (#234)

1. Both `complete` ownership legs (committed range and materialize) pass the landing tree as `ownerRef`.
2. All six readers in Background use that one ref. No reader takes a union of two refs.
3. A changed path is owned when an `OWNERSHIP.yaml` glob at the landing tree matches it for a domain in the spec's resolved Impacted Domains.
4. When `OWNERSHIP.yaml` itself is a conflicted path in the landing tree, the gate refuses before any mutation. It prints the same recovery as a bead-to-spec merge conflict.
5. When only other paths conflict, the gate reads `OWNERSHIP.yaml` from the merge-tree output tree. `MergeInto` then runs and keeps the conflict as today.
6. On `--resolve-merge` re-entry (a merge is in progress in the spec worktree), the landing tree is `git write-tree` of the spec worktree index. When the index still has unmerged entries, today's unresolved-conflict refusal stays.

### R2 — `impl approve` that can finish (#244)

1. `ApproveImpl` runs an early copy of the merge-layout guard and of the work-destruction preflight for the resolved strategy. The copy runs before the phase-reconcile write, the `--supersede-adr` placeholder write and MUTATION 1/3.
2. The executor keeps its own guards in front of every `MergeInto` and `MergeBranch` call. They run on the tip after the auto-merge. Both copies must pass.
3. When the resolved strategy pushes (`pr`, or `auto` with a remote), `ApproveImpl` runs `git push --dry-run --no-verify` to the same remote and refspec as `PushBranch`, with `GIT_TERMINAL_PROMPT=0`, before MUTATION 1/3.
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
| `orphan_finalize` (protected main) | Reuses the existing `chore/finalize-<id>` branch, pushes it with `--force-with-lease` on the recorded remote SHA, and never opens a second PR. This holds before and after the carrier push, and after local spec-branch deletion. |
| direct merge to main | Skips the merge when the spec tip is an ancestor of main. |
| `pre_cleanup` / cleanup | Finishes the remaining removals. |
| after spec-branch deletion | Prints "already landed" and exits 0 only under R2.12. Otherwise today's branch-missing refusal and adopt recovery stay. |

11. While the spec branch exists, a re-run runs every pre-close gate again on the current spec tip. No gate is skipped because the epic is closed.
12. The "already landed" re-run needs all of these: the epic is closed and the phase is done; `mindspec_finalize_tip` is an ancestor of the target; and every child bead's landed binding is an ancestor of the target. The target is main (direct) or the remote spec branch, read with `git ls-remote` (push). Cleanup then runs the destruction guard again and deletes only bead branches whose tips are ancestors of the target.

### R3 — Merge, then verify, then close (#250)

0. Git state selects the path. When the spec branch's first-parent chain has a naming merge for the bead, `complete` takes the closing path (R3.4–R3.7). Otherwise it takes today's merge path. A marker that names another bead's merge, or a commit that is not a naming merge for the bead, refuses, whatever else exists.
1. `mindspec complete <bead> --merge-only` runs every gate that `complete` runs, materializes, and merges the bead into the spec branch. It prints the merge SHA and both parents.
2. It writes `mindspec_merge_only_sha=<merge>` to bead metadata. The marker is a hint. Git state decides.
3. It does not close the bead, commit the panel ledger, write the landed binding, clear the stage marker, or remove the branch or worktree.
4. A bead has at most one naming merge. A `--merge-only` or any other step that would make a second naming merge refuses (R3.8 makes no merge). The closing path refuses before any close or cleanup when:
   - the bead tip moved past the merge's second parent (the text names the new commits);
   - the bead worktree has user changes (tool-artifact paths are ignored);
   - the naming merge is not valid;
   - M's change is reverted fully or partly on the spec branch: the content check gives a clean divergence on M's paths, and no later naming merge for another bead touches them.
5. A fix found after verification goes to a follow-up bead. The refusal says this and prints `recovery: bd create "<title>" --parent <epic> --deps <bead>`. When the bead tip moved, the text lists the new commits for the follow-up bead and adds `recovery: git -C <bead worktree> reset --keep <second parent>`, then `recovery: mindspec complete <bead>`. For a reverted merge, the text says that a person must decide what happens to the bead, and adds `recovery: mindspec doctor`.
6. Before step 1, the closing path checks that the spec worktree has no merge in progress and no staged changes outside the export and ledger paths. Otherwise it refuses with the recovery for that other merge. The closing path makes no commit on the bead branch. It re-runs every gate (panel, doc-sync, ADR) on the current tip, makes no merge, and does these steps in this order:

| Step | Effect |
|---|---|
| 1 | `bd close` the bead. |
| 2 | Write the landed binding and the reviewed-SHA keys for the valid merge. |
| 3 | Write the doc-skew and ADR override metadata. |
| 4 | Export the tracker and commit the export and the panel ledger on the spec branch, in the spec worktree, with a path-scoped commit. This commit publishes the close. |
| 5 | Clear the stage marker. |
| 6 | Remove the bead branch and worktree. |

7. A failure at steps 1–4 refuses with recovery `mindspec complete <bead>`. A re-run converges, because each step is idempotent. Step 6 does not start until step 4 succeeds. After step 4, a re-run on the closed bead finishes steps 5 and 6 and does not evaluate the valid-merge rule again.
8. A repeated `--merge-only` when a valid merge for the current tip exists makes no new merge. It writes the marker for that merge. This is also the recovery when the marker write fails after the merge.
9. `doctor` reports a stale-open bead at Info when a valid merge exists for it, with recovery `mindspec complete <bead>`. Without a valid merge, the finding stays at Error. The marker does not change the severity.

### R4 — `merge_strategy` decides the landing

1. `merge_strategy` accepts `auto`, `pr` and `direct`.
2. One resolver in `internal/lifecycle` returns the strategy and its source (`config` or `default`). `ApproveImpl` calls it one time in preflight, from the main checkout's config, and passes the strategy to `FinalizeEpic` as a parameter. `FinalizeEpic` does not read config again. The early guard gate (R2.1) uses the same value.
3. A config load failure or an unknown value refuses before any mutation. It never falls back to the default. Text: `cannot use merge_strategy: <reason>`. Recovery: `set merge_strategy to auto in .mindspec/config.yaml, then re-run`.
4. `auto` keeps today's rule: push when a remote exists, else merge locally.
5. `pr` requires a remote. With no remote, preflight refuses with `merge_strategy is pr but no git remote is configured` and `recovery: git remote add origin <url>`.
6. `direct` merges into main locally and does not push, even with a remote.
7. `impl approve` prints `merge strategy: <value> (from <source>)`.
8. When `direct` and a remote both exist, `impl approve` prints a notice that it will not push, and `doctor` reports a Warn.
9. The harness sandbox, the lifecycle scenario fixture and the verb-output fixture (`cmd/mindspec/verb_output_test.go:419`) change to `auto`. These fixtures have no remote, so behavior does not change. `scenario_finalize_pr` keeps its push leg.

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
3. The base B is the newest commit on the spec branch's first-parent chain that is an ancestor of the `bead/<P>` tip. P's range is B..P-tip. When B cannot be found, the state is unknown, with no override recommendation. When the range is empty, the outcome stays `no-owned-exact-merge`.
4. When P's tip equals, or every commit in the range is reachable from, the second parent of another bead's naming merge, the text says "not P's own work", with no override recommendation.
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
   - `spec_num` 0 or an empty `spec_title` counts as absent.
   - A `spec_id` that fails `idvalidate.SpecID` refuses for that epic. Resolution does not fall through.
   - When two present sources disagree, resolution refuses and prints `recovery: bd update <epic> --set-metadata spec_id=<id>` with the exact ID from the title. It never picks the plain ID.
   - When more than one epic, closed epics included, resolves to one spec ID, resolution refuses and names each epic.
7. `beadIndex` accepts a bead ID only when it equals `<epicID>.<N>`.
8. `doctor` lists, at Warn, every open bead that resolves only through the removed bead-title rule. Its recovery is `bd update <bead> --parent <epic>`. Doctor uses the title parse only for this diagnosis.
9. `spec create` refuses a new lettered spec ID such as `129b-x`. Existing lettered directories stay valid. No directory is renamed.

### R13 — Text amendments (this spec's half)

1. ADR-0041 and ADR-0036 get a "Spec 132 amendment" section with the R1, R2, R3 and R5 changes.
2. Spec 129 gets dated status notes: the "Deferred to the follow-on spec" list names spec 132 for G, H, #250, I and lineage; AC-11 says that advisories never change the exit code, the binding or the audited tuple; AC-1 quotes the emitted diagnostic; the plan verification command sets `-timeout=20m`.
3. `merge-safety.md` and the workflow runbook name the attested-restore exit, the R6 states, `--merge-only`, the one-naming-merge rule and the partial-revert refusal.

## Deferred to follow-on spec

These items move to one follow-on spec. Each carries the round-1 corrections below. That spec builds on PR #266 (`fix/resolve-merge-hook-next-bead-line`), which must reach main first.

- **R8 parallel-safe advice.** Templates, `complete` and `plan approve` say "start the next bead in a fresh context or a fresh agent with `mindspec next`". The freshness-gate refusal names the parallel-dispatch path and drops "Do NOT attempt workarounds". The session file records each warned reason with `SessionStartedAt`. A new reason warns again. The scan excludes `internal/setup/historical_skills/` by name. `internal/harness` STOP-after-complete scenarios are in scope.
- **R9 spec-named tail.** `spec create`, `spec approve`, `plan approve` and `next` render the tail for their spec through the `instruct --spec` resolution (`cmd/mindspec/spec.go:79,226`, `plan_cmd.go:81`, `next.go:420,425`).
- **R10 readiness diagnostics** (qdoh 1–5, fdfg.4, 4hdt, e07y.3). Lineage and scope checks before file I/O. Hide the bare-override sentinel. One line per signal, with each MF-3 recovery as its own escaped `recovery:` line. `ApplyOverride` addresses lines by signal ID. Recovery lines are commands with no prose and no parentheses; the AC names each refusal case. `bead ready-check` on a bead that is not in_progress and has a legacy marker prints `recovery: mindspec release <bead>`. The loud-empty refusal prints `recovery: mindspec next --spec <id>`.
- **R11 CommitPaths** (0a84, 5x3j). Stage into a temporary `GIT_INDEX_FILE` copy and commit from it, so any failure before the commit leaves the real index unchanged. Cover the nil-paths mode. Refuse an index with unmerged entries. Escape git stderr. The `ls-files` exit-code leg and the bootstrap dry-run leg are regression guards. The spec 129 R8.2 and plan step 6 notes move with R11. R11 gets its own bead.
- **R12** (m0ww.1, dvw3). A ready-list decode failure in `advanceState` takes the indeterminate path. `next` runs `checkUnmergedBeads` for the spec that the gate learns.

## Scope

### In Scope
- R1–R7 and R13 in the packages and documents in Impacted Domains.

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
- [ ] **AC-2 (R1.4–R1.6)**: Conflicting `OWNERSHIP.yaml` edits refuse with the merge-conflict recovery and change nothing. A conflict on a source file only passes the gate, `MergeInto` keeps the conflict, and `--resolve-merge` then completes with ownership read from the resolved index. Tests: `TestCommittedRangeGate_LandingTreeConflictRefuses`, `TestCommittedRangeGate_SourceConflictPassesThenResolves`.
- [ ] **AC-3 (R2.1)**: A layout-guard failure and a destruction failure each exit non-zero with the epic open and the phase unchanged, including from a stale-phase fixture. Tests: `TestApproveImpl_EarlyLayoutGuardRefuses`, `TestApproveImpl_EarlyDestructionRefuses`, `TestApproveImpl_EarlyGuardBeforePhaseReconcile`.
- [ ] **AC-4 (R2.2)**: An auto-merge in `FinalizeEpic` makes the spec-to-main merge destructive after the early check passed. The executor guard still refuses. Test: `TestFinalizeEpic_ExecutorGuardAfterAutoMerge`.
- [ ] **AC-5 (R2.3–R2.5)**: Through a push-probe seam, an HTTPS auth failure refuses before the close and names the credential helper. A non-fast-forward failure refuses and does not name it. A file remote with `GIT_CONFIG_GLOBAL=/dev/null` and no helper passes. The probe argv has `--dry-run --no-verify` and the `PushBranch` remote and refspec. Tests: `TestApproveImpl_PushProbeAuthFailureNamesHelper`, `TestApproveImpl_PushProbeNonFastForwardNoHelperHint`, `TestApproveImpl_PushProbeFileRemoteNoHelperPasses`, `TestPushProbe_ArgvMatchesPushBranch`.
- [ ] **AC-6 (R2.7, R2.8)**: An injected spec-to-main conflict after the close prints "epic closed", the stage, and the `--resolve-merge` recovery last, with no added `impl approve` line. An injected close failure prints "epic still open". An inner error with no recovery gets `recovery: mindspec impl approve <spec>`. Tests: `TestApproveImpl_PostCloseConflictKeepsInnerRecovery`, `TestApproveImpl_StateLineReadsEpicStatus`, `TestApproveImpl_PostCloseBareErrorGetsRerun`.
- [ ] **AC-7 (R2.9, R2.10)**: For each `finalizeStepHook` stage, `orphan_finalize` included, on both paths, a failed run then a re-run lands the spec with one close and exit 0. Each remote ref is pushed at most once per converged state, and no second PR opens. A fixture with a finalize-time auto-merge shows that `mindspec_finalize_tip` equals the merged or pushed tip. Tests: `TestApproveImpl_RerunConvergesFromEachStage` (table over stages and paths; runs both calls), `TestFinalizeEpic_FinalizeTipIsPublishedTip`.
- [ ] **AC-8 (R2.11, R2.12)**: A source commit that fails doc-sync, added to the spec branch between the calls, makes the re-run refuse. After a real landing and spec-branch deletion, the re-run prints "already landed". A forged `mindspec_finalize_tip` (main HEAD or the fork point) with a deleted spec branch and an unlanded bead branch refuses, and the bead branch survives. Tests: `TestApproveImpl_RerunReRunsGates`, `TestApproveImpl_AfterDeletionAlreadyLanded`, `TestApproveImpl_ForgedFinalizeTipRefuses`.
- [ ] **AC-9 (R3.0–R3.3)**: `--merge-only` leaves the bead open, the branch and worktree present, no binding, no ledger commit, and the marker written. A hand naming merge with no marker takes the closing path. A marker that names another bead's merge refuses, also when a valid merge for the bead exists. Tests: `TestComplete_MergeOnlyKeepsBeadOpen`, `TestComplete_HandMergeTakesClosingPath`, `TestComplete_ForeignMarkerRefuses`.
- [ ] **AC-10 (R3.4, R3.5)**: Each refusal leaves the bead open and the branch and worktree present: a bead commit after `--merge-only` (named in the text), user changes in the worktree (tool-artifact changes alone pass), a fully reverted merge, a partly reverted merge, and a second `--merge-only`. A subject with control characters prints escaped. Full loop: `--merge-only`, a fix commit, a second `--merge-only` refuses, the fix goes to a follow-up bead, the reset and plain `complete` close the bead over the first merge, and reattest, `doctor`, the dependents' MF-3 and `impl approve` all read the bead as landed. Tests: `TestComplete_BeadAdvancedAfterMergeOnlyRefuses`, `TestComplete_UserDirtAfterMergeOnlyRefuses`, `TestComplete_RevertedMergeOnlyRefuses`, `TestComplete_PartlyRevertedMergeOnlyRefuses`, `TestComplete_SecondMergeOnlyRefuses`, `TestComplete_MergeOnlyRefusalEscapes`, `TestComplete_MergeOnlyFollowUpLoop`.
- [ ] **AC-11 (R3.6, R3.7)**: The closing path makes no new merge and no bead-branch commit. The spec branch's last commit has an export that shows the bead closed, and it lands before the branch removal. A spec worktree with another merge in progress refuses before the close. An injected export-commit failure keeps the branch and worktree, and a re-run converges. An injected step-5 failure and a partial step 6 each finish on a re-run. Tests: `TestComplete_AfterMergeOnlyPublishesClose`, `TestComplete_ClosingPathSpecWorktreeBusyRefuses`, `TestComplete_AfterMergeOnlyExportFailureConverges`, `TestComplete_ClosedBeadFinishesCleanup`.
- [ ] **AC-12 (R3.8)**: After an injected marker-write failure, a repeated `--merge-only` writes the marker and makes no new merge. Test: `TestComplete_RepeatedMergeOnlyNoNewMerge`.
- [ ] **AC-13 (R3.9)**: `doctor` gives Info for a stale-open bead with a valid merge, and Error for a forged marker with no valid merge. Test: `TestDoctor_StaleOpenSeverityFromGitState`.
- [ ] **AC-14 (R4.2–R4.3)**: A broken `config.yaml` and `merge_strategy: bogus` each refuse before the close with the R4.3 text. `FinalizeEpic` with `direct` makes no push even when config on disk says `pr`. Tests: `TestApproveImpl_StrategyLoadFailureRefuses`, `TestApproveImpl_UnknownStrategyRefuses`, `TestFinalizeEpic_UsesPassedStrategy`.
- [ ] **AC-15 (R4.4–R4.9)**: `pr` with no remote refuses. `direct` with a remote merges locally, makes no push call, prints the notice, and `doctor` warns. `auto` keeps both paths. Output names the strategy and source. The harness and verb-output fixtures read `auto`. Tests: `TestApproveImpl_PRStrategyNoRemoteRefuses`, `TestFinalizeEpic_DirectWithRemoteDoesNotPush`, `TestFinalizeEpic_AutoUnchanged`, `TestDoctor_DirectWithRemoteWarns`, `TestHarnessSandbox_StrategyAuto`.
- [ ] **AC-16 (R5.1–R5.5)**: A branch-less bead whose D adds only tool-artifact paths to a reviewed r is identified on the `complete` reconcile path, with provenance `reviewed-tree`. This fails on the old code. An empty wrapper commit on r also confirms. A D that also changes a source file, renames a source file to a tool-artifact path, or changes another spec's tool-artifact path is not confirmed. A metadata key that is not in R does not confirm. The fallback path ignores the datum. Tests: `TestFindLandedMerge_ReviewedTreeConfirms`, `TestFindLandedMerge_ReviewedTreeEmptyWrapperConfirms`, `TestFindLandedMerge_ReviewedTreeSourceDiffNoConfirm`, `TestFindLandedMerge_ReviewedTreeRenameNoConfirm`, `TestFindLandedMerge_ReviewedTreeOtherSpecDirNoConfirm`, `TestFindLandedMerge_UnregisteredKeyNoConfirm`, `TestReattestFallback_IgnoresPanelDatum`.
- [ ] **AC-17 (R6.1–R6.6)**: P's commits arrive inside a later bead's merge with content present, so reattest returns `ancestor-without-naming-merge` with the person-must-verify sentence. Test: `TestReattest_AncestorWithoutNamingMerge`.
- [ ] **AC-18 (R6.2–R6.5, R6.7)**: A zero-commit `bead/<P>`, a branch restored at a first-parent commit, and a deleted branch each give `no-owned-exact-merge`. A branch restored at another bead's tip, and a zero-commit branch cut from another bead's tip, each give "not P's own work". A revert of P's earlier commit, and fully reverted content, each give `no-owned-exact-merge` with the history line and no "present" claim. A missing base gives unknown. An injected git read error on the new path refuses as indeterminate. Tests: `TestReattest_AncestorZeroCommitsNoClaim`, `TestReattest_AncestorFirstParentRestoreNoClaim`, `TestReattest_AncestorDeletedBranch`, `TestReattest_AncestorOtherBeadTipNotOwn`, `TestReattest_AncestorEarlierCommitReverted`, `TestReattest_AncestorRevertedHistoryOnly`, `TestReattest_AncestorNoBaseUnknown`, `TestReattest_AncestorReadErrorIndeterminate`.
- [ ] **AC-19 (R6.8, R6.9)**: Two dependents give two recovery lines. Zero dependents give `recovery: mindspec doctor`. `doctor` names P, not its dependent. Tests: `TestReattest_AncestorRecoveryPerDependent`, `TestReattest_AncestorZeroDependents`, `TestDoctor_AncestorNamesPrerequisite`.
- [ ] **AC-20 (R6.11, regression guards)**: An injected error on each of the three existing `fallbackSafety` reads refuses. Tests: `TestFallbackSafety_ForwardContentErrorRefuses`, `TestFallbackSafety_ReverseContentErrorRefuses`, `TestFallbackSafety_BranchTipErrorRefuses`.
- [ ] **AC-21 (internal refs)**: The reattest refusals, the reattest `Short` text, the two `materialize.go` messages and the R4.3 recovery match none of `ADR-[0-9]`, `q9ea` or `[Ss]pec [0-9]{3}`. The no-owned refusal contains "attested restore" and a plain sentence that a person must verify the merge first. Recovery lines are commands with no parentheses. A fixture bead `mindspec-ab12` and its `bead/mindspec-ab12` branch render in their operand positions. Tests: `TestReattestText_NoHistoricalCitations`, `TestReattestText_OperandsRender`, `TestMaterializeText_NoHistoricalCitations`.
- [ ] **AC-22 (R7.1–R7.3)**: A `blocks` edge to a foreign epic is not lineage. Two parent-child epics refuse. A `[129]` title with no parent-child edge has no lineage. Tests: `TestFindEpicForBead_BlocksDepIgnored`, `TestFindEpicForBead_TwoParentsRefuse`, `TestFindEpicForBead_NoTitleFallback`.
- [ ] **AC-23 (R7.4)**: A foreign-lineage plan row costs one `bd show`. Test: `TestBuildReadySet_OneShowPerPlanRow`.
- [ ] **AC-24 (R7.5, R7.6)**: With epics for `129-x` and `129b-y` both present, each bead resolves to its own spec, and `next`, `complete` and `plan approve` act on `129b-y`. A `[SPEC 008b-human-gates]` epic with legacy `spec_num=8`, `spec_title=human-gates` and no `spec_id` refuses with the `--set-metadata spec_id=008b-human-gates` recovery. `spec_num` 0 counts as absent. An invalid `spec_id` refuses. Two epics on one spec, one closed, refuse and name both. Tests: `TestResolveSpec_PlainAndLetteredBothPresent`, `TestLifecycle_LetteredSpecResolves`, `TestResolveSpec_LegacyPairDisagreesRefuses`, `TestResolveSpec_SpecNumZeroAbsent`, `TestResolveSpec_InvalidSpecIDRefuses`, `TestResolveSpec_TwoEpicsRefuse`.
- [ ] **AC-25 (R7.7–R7.9)**: `beadIndex` rejects `mindspec-x.3` for epic `mindspec-y`. `doctor` lists a title-only open bead with the `bd update --parent` recovery. `spec create 129b-x` refuses, and `idvalidate` accepts `008b-human-gates`. Tests: `TestBeadIndex_RequiresEpicPrefix`, `TestDoctor_TitleOnlyLineage`, `TestSpecCreate_RefusesLetteredID`.
- [ ] **AC-26 (R13)**: Each ADR and spec-129 note exists, and `mindspec validate spec 129-readiness-tracker-honesty` passes. Test: `TestDocs_Spec132Amendments`.

## Validation Proofs

- `go build ./...`: exit 0.
- `go test ./... -short -timeout 20m`: PASS.
- `golangci-lint run`: no new findings.
- `mindspec validate spec 132-readiness-followon`: PASS.
- Opt-in, separate: the live-agent harness suite for the finalize scenarios.

## Open Questions

- [ ] **`merge_strategy`: honour or delete?** (a) Honour it, as R4 says. (b) Delete the key and its `config show` row, and keep the `HasRemote` rule. Recommended: (a). Repos already set the key, and `direct` with a remote is a real need. The cost is the harness fixture change and the R4.8 notice.
- [ ] **#250 shape.** Every option keeps one naming merge per bead (R3.4): a fix found after verification goes to a follow-up bead, and the bead closes over its first merge. (a) `--merge-only` plus the git-state severity rule, as R3 says. (b) No new flag. Document "merge by hand, then `complete`" as supported. `doctor` gives Info when a valid merge exists (R3.9 rule), with its own AC: a hand naming merge whose second parent is the bead tip gives Info, and a reverted one gives Error. (c) Only `--merge-only`, with Info gated by the marker. Recommended: (a). It gives a verified tool path, and R3.9 also covers hand merges. Option (c) trusts an agent-writable marker.
- [ ] **Confirm the split.** R8–R12 move to one follow-on spec, as "Deferred to follow-on spec" says. Recommended: confirm. Spec 132 is then about 9 beads: R1, R2 (two beads), R3 (two beads), R4, R5, R6, R7.
- [ ] **Freshness warning (follow-on spec).** (a) Warn once per session per reason. (b) A dispatch flag on `next` that an orchestrator sets. Recommended: (a), together with the refusal-text change. With (a) alone, the gate still refuses fresh agents.

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
