---
anchor_currency: all file:line anchors re-verified at transcription 2026-06-11 against this spec's fork tree (main @ 1ded99c — ALL Spec-092 beads merged, including Bead 7 a5d913d and Bead 9 657e4c8; see §Background and §Sequencing risks for transcription-time deltas)
approved_at: "2026-06-11T19:40:28Z"
approved_by: user
drafted_at: "2026-06-11"
drafted_by: spec-drafting research agent
gate_panel: round-1 findings applied (g1 minors; g2 M1-M4 + m1-m5; g3 G3-1..G3-8); round-2 targeted confirm applied (NF-1..NF-4) — see §Appendix Gate panel revision logs
roadmap_step: mindspec-jkhd.3
source_design: /Users/Max/replit/orchestrator-staging-2026-06-10/skills-thin-down-design.md (CONFIRM_READY after 2 panel rounds; transcribed from the gate-cleared spec draft in the same staging dir)
status: Approved
transcribed_at: "2026-06-11"
transcribed_by: spec-creation subagent
---
# Spec 093-skills-thin-down: 16 skills → 11, orchestration-only + panel gate as enforced contract

## Goal

Thin the MindSpec skill surface from 16 skills to 11 (4 lifecycle + 7
plugin), making every surviving skill orchestration-only, under three
invariants:

1. **Zero operational-knowledge loss** — every deleted or shrunk piece of
   operational prose lands in CLI point-of-use output (routed through the
   Spec-092 `guard.FormatFailure` recovery convention), an enforced hook,
   or exactly one canonical doc section. Sequencing is enforced by the
   bead dependency graph: no prose deletion merges before its replacement
   exists (HC-2).
2. **The plugin's central convention becomes an enforced contract for
   REGISTERED panels** — a PreToolUse hook blocks `mindspec complete`
   unless the bead's review panel shows the required APPROVE majority
   (`expected_reviewers − 1`; 5-of-6 for the default panel) against the
   **current** branch HEAD, with no uncommitted edits waiting to be
   auto-folded into the merge. For a panel registered via `panel.json`
   (written by /ms-panel-run step 0), no skill, compaction event, or
   prose-under-pressure shortcut can *silently* bypass it: the skip
   hatch is human-controlled (env var set outside the agent's reach) or
   a persistent config toggle, and panel abandonment — which IS
   agent-performable, being a plain repo-file edit — is always audited
   on bead metadata (Req 13e), not merely warned. Honest limitation:
   panels that are never registered via panel.json (external
   orchestration styles keeping verdicts outside `review/` — including
   the hand-rolled style that built Spec 092 itself) are UNENFORCED by
   the gate by design; not running /ms-panel-run step 0 opts out of the
   contract. The required complete-side advisory tally (Req 13d) is the
   only signal for those flows. See Non-Goals.
3. **Compaction recovery is automatic** — `mindspec instruct
   --panel-state` (and its auto-include at SessionStart in implement mode)
   reconstructs in-progress bead / open-panel / stale-worktree state from
   the filesystem, using the same tally code the hook enforces with — one
   source of truth, previewed.

Addresses Max's three concerns from the 2026-06-10 skills audit:
(a) overlap with existing skills, (b) not fully thought through with full
mindspec context, (c) too many. Closes out most of mindspec-ch8h.

## Background

The mindspec plugin ships 16 skills (5 lifecycle skills inlined in
`internal/setup/claude.go::lifecycleSkillFiles()`, claude.go:533-605, plus
11 plugin skills under `plugins/mindspec/skills/*/SKILL.md`, 961 lines
total, embedded via `plugins/mindspec/embed.go:19`). The 2026-06-10 audit
(`bd show mindspec-jkhd.3`, `plugins/mindspec/FINDINGS.md` items 1, 2, 8,
9, 10) found:

- **Duplicated operational prose**: the worktree/claim fallback recipe is
  verbatim-duplicated (ms-bead-next:36-41 ≡ ms-bead-impl:28-33); the
  MINDSPEC_ALLOW_MAIN legitimacy context is duplicated
  (ms-bead-fix:70-81 + ms-spec-final-review:96-106); subagent guardrails
  are triplicated (ms-bead-prep:67-72, ms-bead-impl:41+57-61,
  ms-bead-fix:45-50); the artifact-gate HARD-block logic exists in two
  **diverging** copies (ms-panel-tally:33-40 vs
  ms-spec-final-review:59-74).
- **Pure passthrough skills**: `/ms-spec-status` wraps `mindspec state
  show` + `mindspec instruct` (claude.go:593-602), both self-describing
  and the latter emitted by the SessionStart hook anyway.
- **Documentation drift in surviving prose (C2-1)**: ms-bead-fix:72 claims
  the implement-mode commit gate blocks `bead/<id>` branches — verified
  FALSE against `internal/hook/dispatch.go` (the gate blocks protected
  branches in any mode, dispatch.go:89-102, and `spec/`-prefixed branches
  in implement mode only, dispatch.go:110-118; bead branches always pass,
  which is precisely what lets impl/fix subagents commit).
- **The ≥5/6 panel convention is prose-only**: the lola-f4a8 incident
  ($417) showed a round-2 APPROVE earned by *describing* a fix instead of
  *making* it, and a post-approval "tiny touch-up" commit completed
  against a stale 5/6. Nothing mechanical prevents either.
- **Compaction loses panel state** (FINDINGS item 8): post-compaction
  sessions cannot cheaply reconstruct which panels are open, what round
  they are on, or which verdicts are in.
- **The lifecycle skills taught the deprecated gate order** (spec-092
  Bead-8 panel R2 minor): `lifecycleSkillFiles()` emitted `mindspec
  approve spec <id>` / `approve plan` / `approve impl` — the exact
  verb-noun form Spec 092 Req 11 scrubbed from both instruct channels
  (mindspec-v7ez). **Transcription-time update: the template rewrite has
  LANDED on main pre-fork** (092 Bead 9, commit 657e4c8 — claude.go:556,
  :569, :583 now teach `spec approve` / `plan approve` / `impl approve`,
  pinned by `TestLifecycleSkills_CanonicalApproveOrder`,
  claude_test.go:715). Still open, and explicitly deferred to THIS spec
  by that test's own boundary NOTE: `RunClaude`'s skill install is
  create-or-skip (claude.go:118-135) — the content fix never reaches
  repos that already have the files, so a refresh mechanism is required
  (Req 19); and the landed negative test covers `lifecycleSkillFiles()`
  only, not the plugin-embedded `skillFiles()` surface.

### What Spec 092 already delivered (honest baseline)

The panel-cleared design predates the 092 beads that landed 2026-06-10/11.
Re-verified at transcription against this spec's fork tree (main @
1ded99c, all 092 beads merged), these design items are **already done**
and become verify-only or constraints here:

1. **Lazy `stateFn` hook dispatch** — `hook.Run(name string, inp *Input,
   stateFn func() *HookState, enforce bool)` with documented
   short-circuit-before-state discipline is landed
   (`internal/hook/dispatch.go:29`, doc comment :24-28; `hook.Names`
   stays in hook.go:52-53; `runPreCommit` short-circuits at
   dispatch.go:70-74 before resolving state). The design's "rebase note"
   dissolves: the pre-complete hook builds directly on this pattern.
2. **`guard.FormatFailure` + recovery-line convention + convention test**
   (092 Bead 1) — `internal/guard/recovery.go:46` (`FormatFailure`),
   `:71` (`NewFailure`), `:78` (`HasFinalRecoveryLine`), `:94`
   (`IsBannedRecoveryCommand`), enforced by
   `internal/guard/recovery_convention_test.go`. The design's hand-rolled
   CLI error texts (§2.1-2.3 of the design) MUST now route through this
   helper and end with `recovery: <command>` lines (HC-5 here) — a
   constraint the design predates.
3. **`gatesForMode` + `templates/review.md` canonical noun-verb rewrite**
   (092 Bead 8, Req 11) — `internal/instruct/instruct.go:219,222,232` now
   emit `mindspec spec/plan/impl approve`; all instruct templates use the
   canonical order. The *instruct* half of v7ez is done.
4. **Location-agnostic completion guidance** (092 Bead 4, Req 5) —
   `cmd/mindspec/next.go:294-300` (`completionGuidance`) now says: run
   `mindspec complete` from the repo root; it resolves the bead worktree
   itself and removes it on success. The design's folded merge prose must
   ALIGN with this (the cycle's merge terminal must not re-introduce
   cd-then-complete) — verify-only constraint folded into Req 16.
5. **`complete.FormatResult` cd hints + STOP-HERE guidance** (092 Beads
   4/5) — all three mode branches now print `Run: cd <spec-worktree>`
   when the worktree was removed (`internal/complete/complete.go:467+`).
   The design's claim that FormatResult lacks the cd hint is obsolete.
   Still true: FormatResult does NOT print the merge commit SHA, so the
   cycle's one-line `git log --oneline -3` merge verify stays (Req 16).
6. **`complete` is safe to run from anywhere** (092 Req 3c) —
   `os.Chdir(root)` inside `complete.Run` after the terminal merge
   (complete.go:351). Recovery recipes emitted by Reqs 2-4 may assume
   root-runnable `complete`.
7. **`workspace.ContextLine(dir, checkedPath)`** exists
   (`internal/workspace/workspace.go:320`) — available to new error
   messages; no need to re-specify it.
8. **092 Bead 7 (a5d913d) is merged into the fork tree** — the
   cross-spec sequencing risk the draft flagged as MANDATORY has been
   absorbed by forking from current main: `internal/next/guard.go` (+
   `guard_test.go`) exists with `next.DirtyTreeFailure` via
   `guard.NewFailure` and the `artifactPaths` classification, and is the
   constructor precedent/home for the Req 3-4 failures. The Bead-1
   anchors below are post-a5d913d actuals, verified at transcription.
9. **Lifecycle skill canonical noun-verb rewrite landed** (092 Bead 9,
   657e4c8) — see the Background bullet above. Req 19 here narrows to
   the provenance-gated content refresh + extending the negative
   assertion to the full `skillFiles()` surface.
10. **Anchor drift** (no behavior relevance, recorded so the plan uses
   current lines — all values verified at transcription against the fork
   tree): commit-gate blocks at dispatch.go:89-102 (protected, hatch
   printed at :100) and :110-118 (spec-branch implement, conditional cd
   line :113-115, hatch at :116); ADR-divergence hint at
   complete.go:303-304; `checkUnmergedBeads` at next.go:365-381;
   `Names` sorted-order test at hook_test.go:145-150;
   `isMindspecHookEntry` at claude.go:387-400; SessionStart 12s timeout
   at cmd/mindspec/hook.go:130; `FormatResult` at complete.go:467+;
   `--override-adr` metadata auto-fill ~:425; `os.Chdir(root)` :351;
   `hook.ReadState` starts hook.go:201; `githooks.CleanStaleGitHooks`
   install.go:106; complete auto-commit (`exec.CommitAll`)
   complete.go:176-186; artifact-aware dirty check complete.go:190-245
   (checkDirtyTreeFn call :200, artifact-dirt follow-up commit
   :229-243); `exec.CompleteBead` call complete.go:335.

The plugin skill files themselves are untouched between the draft's
verification and this fork (zero commits under `plugins/` on the 092
branch; all 11 SKILL.md line counts re-verified at transcription — 961
total, matching the design's disposition table exactly), so the design's
per-skill line anchors remain valid.

## Impacted Domains

- workflow
- execution

## Affected packages (per domain)

- **`internal/panel` (NEW, workflow)** — `panel.json` convention
  (`bead_id`/`spec`/`target`/`round`/`expected_reviewers`/
  `reviewed_head_sha`/`abandoned`), `Scan(roots ...string)`,
  `Tally(dir)` with filename-derived rounds, verdict-JSON parse incl.
  optional `hard_block`. Shared by the pre-complete hook and
  `instruct --panel-state` (Req 6).
- **`internal/hook`** (workflow) — new `pre-complete` dispatch case in
  `Run` (internal/hook/dispatch.go:29-36) on the lazy-stateFn pattern;
  `Names` insert
  between "pre-commit" and "session-start" (hook.go:52-53,
  hook_test.go:145-150 enforces sorted order); anchored command matcher
  (Reqs 9-13).
- **`internal/hook/dispatch.go`** (workflow) — spec-branch block message
  (:110-118) gains legitimacy context with C2-1-correct coverage (Req 1).
- **`internal/complete/complete.go`** (workflow) — ADR-divergence hint
  (:303-304) becomes the repair ladder
  (Req 2); `panel_gate_skipped` + `panel_abandoned` metadata writes and
  the REQUIRED advisory tally warning inside `Run` (Reqs 13b/13d/13e).
- **`cmd/mindspec/next.go` + `internal/next/beads.go`** (execution) —
  claim-failure recipe (next.go:218 → `ClaimBead`, beads.go:159-166) and
  worktree-setup-failure recipe (next.go:231) (Reqs 3, 4). 092 Bead 7's
  `internal/next/guard.go` is in the fork tree and is the constructor
  precedent/home for these failures (anchors are post-a5d913d actuals).
- **`internal/setup/claude.go`** (execution) — hook-entry ownership
  redesign (`hookEntryExists` :286-299, `hookEntryStale` :302-335 —
  same matcher-keyed identity, in scope for the same redesign,
  `replaceHookEntry` :338-352, `removeStalePreToolUse` :355-385,
  `isMindspecHookEntry` :387-400, `ensureSettings` :185-265,
  `wantedHooks` :268-284) (Reqs 7, 8);
  PreToolUse entry ships from `wantedHooks()` (Req 9);
  `lifecycleSkillFiles()` (:533-605) — ms-spec-status deletion (:593-602)
  (the canonical noun-verb rewrite at :556/:569/:583 LANDED pre-fork,
  Req 19 verify-only); skill install refresh
  + stale-skill cleanup in `RunClaude` (:118-135) (Reqs 18, 19);
  `claudeMDManagedBlock` tables (:612-657, ms-spec-status row :629).
- **`internal/setup/codex.go`** (execution) — `agentsMDManagedBlock`
  (:138-139) gains the bead-loop guardrails section (Req 17).
- **`internal/config/config.go`** (workflow) — `Enforcement.PanelGate`
  bool, default true (struct at config.go:43-48, default at :56) (Req 13).
- **`internal/instruct`** (workflow) — `--panel-state` flag
  (cmd/mindspec/instruct.go:34-37 `init`), options on `instruct.Run`
  (run.go:40), `Context.PanelState` field (instruct.go:33-48),
  `templates/implement.md` auto-include (Reqs 14, 15).
- **`plugins/mindspec/`** (execution) — skills consolidation (Req 16),
  README rationale + inventory refresh, FINDINGS adjudications (Req 20);
  `embed.go` package comment.
- **`internal/harness`** (workflow) — sandboxes install the full hook set
  via `setupClaudeForSandbox` → `setup.RunClaude` (sandbox.go:75,
  :445-448), so the no-panel fail-open invariant and the zero-bd
  non-match invariant are load-bearing for the entire LLM-test suite
  (HC-4, HC-3); one new harness scenario (Req 12 AC).

## ADR Touchpoints

- **ADR-0023** (beads as single state authority) — `panel.json` and
  verdict JSONs are **review artifacts**, not workflow state: the bead's
  lifecycle state stays derived from bd statuses; the panel gate reads
  artifacts to decide whether a state-changing command may proceed, and
  writes nothing to them. The only bead-metadata writes this spec adds
  (`panel_gate_skipped`, Req 13) use `bead.MergeMetadata` merge semantics
  per the Spec-092 Req 19 precedent (raw `bd update --metadata` is banned
  from all emitted output, 092 HC-5 carried forward in HC-5 here).
- **ADR-0025** (artifact handling) — behavior untouched; cited because
  Req 3's emitted recipes reference `mindspec complete`'s artifact-aware
  behavior, and because the Req 11 dirty-tree check honors it: the
  ADR-0025 artifact paths (`.beads/issues.jsonl`) are filtered out of
  the Block predicate, so the gate never blocks on the artifact dirt
  092 made never-blocking at complete time.
- **ADR-0030** (executor boundary) — the pre-complete hook performs at
  most two git subprocesses on the matched-command path (one
  `git rev-parse` for staleness, one `git status --porcelain` for the
  dirty-tree check, Req 11), mirroring the existing hook-side precedent
  (`runPreCommit` calls `gitutil.CurrentBranch` directly,
  dispatch.go:63); boundary lint must stay green.
- **NEW ADR (proposed): panel gate as enforced contract** — number
  claimed at implementation time per the standard next-free-number
  procedure (not pinned here; the ADR lane has concurrent in-flight
  claims). Records the
  panel.json convention, the `expected_reviewers − 1` threshold (5-of-6
  default) + reviewed_head_sha staleness rule, the dirty-tree rule, the
  env-only/audited escape-hatch asymmetry vs MINDSPEC_ALLOW_MAIN, the
  fail-open-when-no-panel rule, the registered-panels-only scope
  limitation (Goal invariant 2 / Non-Goals), and the trust boundary:
  every gate input (panel.json, verdict JSONs, reviewed_head_sha, round,
  expected_reviewers, abandoned — and equally the
  `cfg.Enforcement.PanelGate` config toggle, Req 13c, a git-visible
  repo file like abandonment) is an agent-writable repo artifact —
  the gate is an anti-footgun device, not an anti-adversary one; a
  future panel must not "fix" perceived forgeability at this layer (the
  env channel is the only agent-proof input, HC-7). Panel to confirm ADR
  vs doc section (Design Question 3).

## Requirements

### Hard Constraints

- **HC-1** Existing test suite preserved: `go build ./... && go test
  -short ./...` green on every commit; no test skipped relative to the
  branch base. Boundary lint (`internal/lint/boundary_test.go`) green
  (ADR-0030).
- **HC-2** Zero operational-knowledge loss: every sentence deleted from a
  skill has a live replacement (CLI output, hook message, AGENTS.md
  section, or canonical skill section) merged BEFORE the deletion merges.
  Enforced by bead dependencies (Bead 6 depends on Beads 1 and 4), not
  convention.
- **HC-3** Hook cost: ZERO bd subprocesses, zero config/git/fs work on a
  non-matching Bash command — the pre-complete hook's non-match path is
  pure stdin+string work. Tested invariant using the `internal/phase`
  `SetListJSONForTest`/`SetRunBDForTest` stubs
  (internal/phase/derive_recovery_test.go:48-52 shows the pattern) with a
  stub that FAILS the test if invoked. The SessionStart path (12s budget,
  cmd/mindspec/hook.go:130) gains no cost when no panel dir exists.
  Accepted floor: this spec ships the repo's first PreToolUse Bash entry
  (wantedHooks currently carries SessionStart only), so EVERY Bash call
  in every consumer repo and every harness sandbox now spawns one
  `mindspec hook pre-complete` process — the per-Bash-call spawn
  overhead is the accepted budget floor (the zero-cost invariant is
  in-process work only); a harness-suite wall-clock sanity check is
  recommended at plan time.
- **HC-4** Fail-open without a panel, fail-closed with one: absence of any
  `panel.json` referencing the bead → silent Pass (solo/non-panel/harness
  flows structurally unaffected — `internal/harness` completes beads
  constantly with no panels); a present panel with malformed/missing
  verdicts blocks. Legacy panel dirs (BRIEF.md, no panel.json) pass.
- **HC-5** Every guard/error message added or changed by this spec routes
  through `guard.FormatFailure`/`guard.NewFailure` and ends with
  `recovery: <command>` line(s) (Spec-092 Req 12/21 convention; the
  convention test must cover the new constructors). No emitted message
  contains a raw `bd update --metadata` command. Exception: PreToolUse
  Block messages follow the hook `Emit` protocol (stderr + exit 2,
  hook.go) rather than the error-return convention, but still end with an
  actionable next step.
- **HC-6** Setup never harms user configuration: `mindspec setup claude`
  never overwrites, deletes, or reorders a non-mindspec-owned hook entry
  in `.claude/settings.json`, and never deletes or overwrites a
  user-modified skill file (provenance-gated refresh only, Reqs 18-19).
- **HC-7** The panel-gate Block message never prints a paste-able skip
  incantation: `MINDSPEC_SKIP_PANEL` appears in human-facing docs
  (ms-panel-tally § Escape hatch) only — never in the hook's block output.
  (Contrast deliberately preserved: the git pre-commit hook MAY keep
  printing `MINDSPEC_ALLOW_MAIN=1 git commit ...` because git spawns it as
  a child of the prefixed command, so the prefix legitimately reaches it;
  a PreToolUse hook inherits Claude Code's env, so `os.Getenv` is
  agent-proof and env-only. Do not harmonize this asymmetry in either
  direction.)

### Numbered requirements

#### CLI point-of-use errors (replacing skill prose)

1. **Commit-gate legitimacy context + C2-1 coverage truth.** The
   spec-branch-during-implement block message
   (`internal/hook/dispatch.go:110-118`) is enriched to carry the
   when-is-it-legitimate context currently duplicated in
   ms-bead-fix:70-81 and ms-spec-final-review:96-106:

   ```
   mindspec: commits on spec branch '<branch>' are blocked during implement mode.
     Implementation code belongs on bead branches.
     Run: mindspec next   (to claim a bead and create a bead worktree)
     Or switch to your bead worktree: cd <active-worktree>
     Legitimate direct spec-branch commits (final-review fix-ups: PR-body precision,
     stray-file reverts, CI-unblocking test fixes) may use the escape hatch:
       MINDSPEC_ALLOW_MAIN=1 git commit ...
     Do NOT use the escape hatch to land feature code outside a bead branch.
   ```

   The existing conditional "Or switch to your bead worktree: cd <wt>"
   line (dispatch.go:113-115, printed only when state carries an active
   worktree) is PRESERVED, conditionality included — the rewrite must
   not drop this live affordance (G1-3).
   The protected-branch message (:89-102) keeps its bare hint (no
   legitimate routine use on main). All migrated prose (hook message,
   surviving ms-bead-fix text, AGENTS.md guardrails) states the ACTUAL
   gate coverage per C2-1: protected branches any-mode + `spec/` branches
   implement-mode only; `bead/` branches always pass. Extending the gate
   to bead branches is an explicit Non-Goal (it would block the
   autopilot's own impl subagents). Tests:
   `internal/hook/dispatch_test.go` message assertions.
2. **ADR-divergence repair ladder in the `complete` error.** The hint at
   `internal/complete/complete.go:303-304` (currently bypass-first:
   `--override-adr`/`--supersede-adr`) is replaced by the
   ms-bead-merge:31-38 triage ladder, repair-first:
   (1) file belongs to this bead's domain → add to
   `.mindspec/domains/<name>/OWNERSHIP.yaml` and re-run;
   (2) accidental stray edit picked up by auto-stage → revert and re-run;
   (3) only then bypass via `--override-adr "<reason>"` (recorded on bead
   metadata) or `--supersede-adr ADR-NNNN`. The findings
   (`joinResultErrorMessages(adrResult)`) carry file names, so the ladder
   is actionable without any skill. Message formatted per HC-5 (ladder in
   the body; the final `recovery:` lines carry the re-run and bypass
   commands).
3. **Claim-failure recovery recipe.** On `next.ClaimBead` failure
   (`cmd/mindspec/next.go:218` → `internal/next/beads.go:159-166`), the
   error (at the caller, which has root/specFlag context for
   interpolation) appends the recovery recipe — `--claim` kept VERBATIM
   from the battle-tested skill prose (ms-bead-next:36-41 ≡
   ms-bead-impl:28-33; `--claim` carries the atomic claim/assignee
   semantics that prevent two agents on one bead):

   ```
   claiming bead failed (may already be claimed): <bd output>
   If this is a bd event-recording failure (e.g. Dolt Error 1105 on large
   descriptions), claim manually:
     bd update <bead-id> --claim --status in_progress
     git -C <spec-worktree> worktree add .worktrees/worktree-<bead-id> -b bead/<bead-id> <spec-branch>
   recovery: mindspec next --spec <slug>   (re-run to auto-recover the worktree)
   ```

   A claim-less 1105 auto-fallback is explicitly DEFERRED (jkhd.5
   validator batch) unless the bead's test plan first reproduces a 1105
   and proves the claim event is the trigger.
4. **Worktree-setup-failure recipe.** `cmd/mindspec/next.go:231`
   currently prints `Warning: worktree setup failed: <err>` and continues,
   leaving the agent claimed-but-homeless. It now emits the concrete
   `git worktree add` recipe interpolated via
   `workspace.BeadWorktreeName`/`BeadBranch`
   (internal/workspace/worktree.go:32,40) + `workspace.SpecWorktreePath`
   (worktree.go:62), and references the existing in-progress auto-recovery
   (`re-run mindspec next --spec <slug>`, next.go:170-188 path).
5. **Recovery-convention compliance for Reqs 1-4.** All new/changed
   messages in Reqs 2-4 route through `internal/guard`
   (`FormatFailure`/`NewFailure`) and are covered by the Spec-092
   convention test (extend `conventionFixtures`,
   internal/guard/recovery_convention_test.go:51, as needed). Req 1 is a
   hook Block message (HC-5 exception applies) but its text is asserted in
   `dispatch_test.go`.

#### Panel plumbing (foundation)

6. **`internal/panel` package + `panel.json` convention.** New package
   providing one source of truth for panel state:
   - `panel.json` schema (written by /ms-panel-run step 0, Req 16):
     `{"bead_id": string|null, "spec": string, "target": string,
     "round": int, "expected_reviewers": int (6 for the default panel),
     "reviewed_head_sha": string, "abandoned": bool (optional),
     "abandon_reason": string (required when abandoned, with who/why —
     Req 13e)}`. `bead_id` null for non-bead targets
     (final-review/PR panels); `reviewed_head_sha` = `git rev-parse` of
     the target ref at fan-out. On every re-panel, `round` and
     `reviewed_head_sha` are bumped IN THE SAME WRITE — the two fields
     move together by construction.
   - `Scan(roots ...string)` — fs-only glob of `review/*/panel.json`
     under each root, deduped.
   - `Tally(dir)` — parses verdict JSONs (incl. optional
     `"hard_block": true`), derives the latest round as max(N) over
     `*-round-<N>.json` FILENAMES (never trusting `panel.json.round`,
     which can lag), reports verdict count vs `expected_reviewers`,
     APPROVE tally, round-mismatch (panel.json.round ≠ filename max), and
     staleness inputs. Malformed verdict JSON = missing (named in
     results). Unit tests for all shapes.

#### Settings-merge identity redesign (must land before the gate ships)

7. **Hook-entry ownership by command content, never matcher (Landmine A,
   S3-1 blocker).** `hookEntryExists` (claude.go:286-299),
   `hookEntryStale` (:302-335), and
   `replaceHookEntry` (:338-352) currently identify entries by matcher
   string alone; for PreToolUse matcher `"Bash"`, user lint/guard hooks
   routinely collide, and `ensureSettings` (:209-221) would silently
   overwrite them. New rule: an entry is mindspec-owned iff one of its
   hook commands prefix-matches `mindspec hook ` OR contains
   `mindspec instruct` — the ownership test RETAINS the `mindspec
   instruct` arm of the current `isMindspecHookEntry` (claude.go:387-400),
   because the spec-072 retired guard hooks it cleans include the instruct
   form (N1); instruct-form entries are never in the wanted set, so
   wanted-set-derived staleness removes them correctly. `ensureSettings`
   locates/updates only mindspec-owned entries and APPENDS mindspec's
   entry alongside user entries sharing the matcher — it never replaces
   or deletes a non-mindspec entry.
8. **Wanted-set-derived staleness (Landmine B, S1-1).**
   `removeStalePreToolUse` (claude.go:355-385) currently strips ANY
   PreToolUse entry whose command contains `mindspec hook` — and
   `ensureSettings` merges wantedHooks (:209-221) THEN strips (:225) in
   the same pass, so a naive PreToolUse gate entry would be added and
   immediately removed: the gate would never install on any repo with an
   existing settings.json (only the fresh-install path, :246-262,
   escapes — until the next setup run). Fix: staleness = mindspec-owned
   AND not in the current wanted set, with the keep-list DERIVED from
   `wantedHooks()` (no hardcoded whitelist, so the next new hook does not
   re-create this bug). Three regression tests in
   `internal/setup/claude_test.go`: (i) merge path — pre-existing
   settings.json without mindspec entries → setup → pre-complete entry
   present; (ii) user-entry survival — pre-existing user PreToolUse Bash
   entry → setup → both entries present, user's command byte-identical;
   (iii) re-run idempotence — second setup run leaves exactly one
   mindspec pre-complete entry.

#### PreToolUse panel gate (the enforced contract)

9. **`pre-complete` hook: lazy dispatch + anchored command match + cost
   budget.** New `pre-complete` case in `hook.Run`
   (internal/hook/dispatch.go:29-36), shipped
   via `wantedHooks()` (claude.go:268-284) as
   `{"matcher": "Bash", "hooks": [{"type": "command", "command":
   "mindspec hook pre-complete", "statusMessage": "Checking panel
   verdicts..."}]}`; `Names` insert between "pre-commit" and
   "session-start" (sorted-order test, hook_test.go:145-150). Exact
   short-circuit order (each step only if the previous didn't exit):
   (1) `hook.ParseInput` → `Input.Command` — pure stdin;
   (2) anchored command match — non-match → Pass with NO stateFn, config,
   git, or fs work (HC-3);
   (3) `MINDSPEC_SKIP_PANEL` env check → audited Pass (Req 13);
   (4) `workspace.FindLocalRoot` + `config.Load`;
   `cfg.Enforcement.PanelGate == false` → Pass (mirrors the
   PreCommitHook toggle, dispatch.go:58);
   (5) bead-id + scan-root resolution (Req 10) — first possible stateFn
   consultation;
   (6) `internal/panel.Scan` (fs-only);
   (7) staleness check — one `git rev-parse` (Req 11);
   (8) dirty-tree check — one `git status --porcelain` in the resolved
   bead worktree (Req 11, runs only when a panel.json matched);
   (9) tally + decision (Req 12).
   Matched-path git budget: at most TWO git subprocesses total (the
   rev-parse + the porcelain status); the matched path is rare and
   HC-3's zero-cost non-match invariant is untouched.
   **Match rule (S3-6)**: `mindspec complete` (optionally
   `<path>/mindspec`) matches only at command position — string start or
   immediately after an unquoted shell separator (`;`, `&&`, `||`, `|`,
   newline, `$(`, backtick), optionally preceded by env assignments
   and/or a `cd <path> &&`/pushd prefix. Quoted-string mentions MUST NOT
   match (commit messages quoting the phrase, `grep 'mindspec complete'
   SKILL.md`, echoed `--panel-state` output that itself says "run
   `mindspec complete <id>`" — exactly what orchestrators type while a
   panel is open). Implementation tokenizes on unquoted separators
   (skipping `'…'`/`"…"` content). Table-driven tests covering every
   mention-in-string case plus legit forms (`mindspec complete X`,
   `cd wt && mindspec complete X`, `FOO=1 mindspec complete X`,
   `a && mindspec complete X`). Documented residual heuristic: remaining
   false NEGATIVES fail open (the REQUIRED complete-side advisory,
   Req 13d, is the backstop); false POSITIVES are the pinned bug class.
10. **Scan-root resolution from the COMMAND's target, never the hook
    process cwd (S3-3).** Claude Code spawns PreToolUse hooks with the
    session's cwd; an orchestrator at the main root running
    `cd .worktrees/worktree-spec-X && mindspec complete <bead>` would,
    under cwd-based scanning, miss the worktree's panel.json and silently
    Pass — a false PASS in exactly the autopilot path the gate exists
    for. Rule: bead-id = first non-flag token after `complete` (absent →
    Pass; explicit-id completes only in v1). Scan roots (union, deduped):
    (a) the matched command's `cd <path>` prefix resolved against session
    cwd, normalized via `workspace.FindLocalRoot`; (b) the spec worktree
    derived from the COMMAND's bead-id — bead-id → owning epic/spec
    (bd/phase lookup) → `workspace.SpecWorktreePath` — NOT the
    active-bead resolution `hook.ReadState` performs (hook.go:201+,
    which resolves the ACTIVE phase context from session cwd and would
    pick the wrong spec worktree under multi-active-spec when the
    completed bead belongs to a different spec); this bd/phase lookup is
    the stateFn consultation point. If the bead-id→spec lookup fails,
    coverage rests on roots (a)/(c) — state this fallback in the hook's
    doc comment;
    (c) `workspace.FindLocalRoot(session cwd)`. Required test:
    panel.json in the spec worktree, hook cwd = main root, incomplete
    panel → Block; plus the cd-prefix variant.
11. **Round derivation + reviewed-HEAD staleness (S3-4/S2-3).**
    - No `review/` dir under any scan root, or no panel.json referencing
      this bead-id → silent Pass (HC-4). Legacy BRIEF-only dirs pass.
    - Latest round = filename-derived max(N). `panel.json.round` ≠
      filename max → Block: "panel.json round (<r>) out of date vs
      verdict files (round <N>) — re-run /ms-panel-run step 0". Never
      tally a round below the filename max.
    - `reviewed_head_sha` vs current `git rev-parse` of the derived bead
      branch (`bead/<bead-id>` in the scan root). Mismatch → **Block**
      (not Warn — a Warn here
      is the same prose-under-pressure failure the hook exists to close):
      "panel round <N> reviewed <sha7>, branch now at <sha7'> — commits
      landed after review; bump round and re-panel (/ms-panel-run step
      0)." This mechanizes the stale-verdict rule (the lola-f4a8 bypass
      class). (bead_id-null panels — final-review/PR targets — are
      outside v1 enforcement entirely per the explicit-id-only match
      rule, Design Question 4; their staleness is surfaced via
      `--panel-state` only. The non-bead-target enforcement branch is
      deliberately NOT specified here — dead text in v1, plumbing for a
      possible future spec-merge gate.)
    - **Missing-ref semantics (the rerun-after-merge case)**: if
      `git rev-parse bead/<bead-id>` FAILS because the branch no longer
      exists → **Pass with Warn** naming the missing ref ("panel for
      <bead-id> references branch bead/<id>, which no longer exists —
      assuming the merge already landed; deferring to mindspec
      complete's own handling"). Rationale: `exec.CompleteBead` merges,
      removes the worktree, and DELETES the bead branch (invoked at
      complete.go:335; merge/cleanup inside the executor,
      mindspec_executor.go:172+), and re-running `mindspec complete <id>`
      after a partial failure is the documented recovery path (092
      Req 14 messaging; the folded merge terminal's partial-failure
      rule, Req 16). A deleted bead branch + closed bead means the merge
      already happened (or the artifacts are stale) — the gate passes
      through to `complete.Run`'s own idempotent handling rather than
      false-blocking the recovery rerun (false positives are the pinned
      bug class, Req 9).
    - **Dirty-tree check (closes the CommitAll bypass)**: when a
      panel.json matched the bead, run one `git status --porcelain` in
      the resolved bead worktree and classify the output by path:
      entries under the ADR-0025 artifact paths (`.beads/issues.jsonl`
      — the same `artifactPaths` classification
      `internal/next/guard.go` pins post-a5d913d, mirrored by
      `complete.Run`'s own artifact-aware check at complete.go
      :190-245) are filtered out before deciding — 092 deliberately
      made artifact dirt in the bead worktree never-blocking at
      complete time (it is bd-export-normalized and committed as a
      follow-up sync commit; the user-dirt error text promises this).
      Any USER-AUTHORED entry remaining → **Block**: "uncommitted
      changes in <worktree> — `mindspec complete` would auto-commit
      them past review (CommitAll); commit and re-panel, or revert."
      This is pure path filtering on the existing porcelain output —
      do NOT add a bd-export normalization call; the matched-path
      budget stays at two git subprocesses (Req 9 / ADR-0030).
      **Worktree resolution + absence**: resolve the bead worktree per
      `complete.Run`'s precedent (worktree-list match on
      `workspace.BeadWorktreeName`/`BeadBranch`); if no worktree exists
      — reachable when a prior `exec.CompleteBead` failed between
      worktree removal and branch deletion (branch present, SHA
      matches, worktree gone) — SKIP the dirty check and pass through
      to the tally (Req 12), mirroring the missing-ref Pass-through
      semantics above: a porcelain failure from a missing worktree must
      not false-block the documented partial-failure recovery rerun.
      Rationale: `complete.Run` runs `exec.CommitAll` on the bead
      worktree AFTER the hook fires (complete.go:176-186), so a
      post-approval "tiny touch-up" left uncommitted would pass the
      reviewed_head_sha check (HEAD unchanged at hook time) and then be
      merged unreviewed — the cheapest variant of the lola-f4a8 class.
      Design choice: Block, not a residual-risk note — in a panel-gated
      flow the bead worktree is clean of user edits by construction at
      complete time (impl/fix subagents end with exactly one commit;
      the panel reviewed committed HEAD) — clean-by-construction holds
      for user edits, NOT artifact churn, which is designed-for and
      therefore filtered — so user dirt here is precisely the
      unreviewed-edit case; legitimate no-panel flows never reach this
      check (it runs only on the panel.json-matched path, HC-4), so it
      cannot over-block them.
12. **Decision matrix (deterministic subset of /ms-panel-tally's).**
    Threshold rule (Design Question 5 resolved per its draft position):
    the hook reads `expected_reviewers` (= N) from panel.json and the
    approval threshold is **N − 1** (one dissent tolerated) — 5-of-6 for
    the default panel, which all examples and Block texts use; no second
    hardcode of the literal 6 (the README's own ceil(5N/6) scaling note
    stays as the human guidance for choosing N).
    - `"abandoned": true` in panel.json → Pass + Warn naming the
      abandoned panel AND its recorded `abandon_reason` (legitimate
      exit, distinct from the escape hatch; audited on bead metadata
      per Req 13e).
    - verdicts < expected_reviewers → Block: "panel <slug> round <N>
      incomplete: <k>/<N₀> verdicts present (missing: <slots>) — finish
      /ms-panel-run or tally first".
    - any REJECT or `"hard_block": true` → Block with tally + "HARD
      block / REJECT recorded — halt path, see /ms-panel-tally".
    - APPROVE ≥ N − 1 AND head SHA matches AND tree clean (Req 11) →
      Pass.
    - else Block: "panel <slug> round <N>: <A>/<N₀> APPROVE — threshold
      is <N₀−1>/<N₀>. Run /ms-bead-fix with consolidated-round-<N>.md,
      then re-panel."
    - Malformed verdict JSON = missing (block names the file).
    Block protocol: existing `hook.Emit` (stderr + exit 2;
    `permissionDecision: deny` for Copilot) — no changes there. Every
    Block message ends with the raw-merge fence (G3-1): "Do NOT bypass
    with raw `git merge bead/<id>` — it skips bd closure, worktree
    cleanup, and this gate; only `mindspec complete` merges bead
    branches." (Once the gate blocks `mindspec complete`, a raw git
    merge on the spec branch is the obvious workaround: the matcher
    fires only on `mindspec complete`, and mindspec installs no
    pre-merge-commit git hook — git runs no commit hook for automatic
    merge commits.)
13. **Escape hatches: env-only + audited; config toggle; complete-side
    audit write.**
    (a) The hook checks `os.Getenv("MINDSPEC_SKIP_PANEL") == "1"` — the
    hook process inherits Claude Code's environment, which the agent
    cannot alter; the command string is NEVER consulted for the hatch
    (the env-prefix on a Bash line is the agent-writable channel — a
    blocked LLM's highest-probability next action is pasting a suggested
    prefix). Every use emits a Warn/systemMessage ("panel gate skipped
    via MINDSPEC_SKIP_PANEL for <bead-id>"). Block messages end with:
    "Skipping the panel requires a human: a user must set the skip
    variable in their own environment before launching the session — see
    /ms-panel-tally § Escape hatch" (HC-7).
    (b) **Non-optional audit write**: `complete.Run` records
    `panel_gate_skipped: true` + timestamp on bead metadata via
    `bead.MergeMetadata` when the env var was set for a panel-gated bead
    — mirroring `--override-adr`'s recorded-on-metadata discipline
    (complete.go ~:425 area). Without it the audit is only a transient
    Warn.
    (c) `cfg.Enforcement.PanelGate` bool, default true
    (config.go:43-48/:56 precedent) — the persistent disable path.
    (d) **REQUIRED (promoted from optional per gate finding M1; Design
    Question 2 resolved)**: a warning-only tally check inside
    `complete.Run` — when a registered panel references the bead, print
    the tally (and would-PASS/BLOCK line via the same `panel.Tally`)
    before proceeding. This is the ONLY signal for every flow that
    never routes through Claude Code hooks: codex sessions, raw-shell
    agents, and externally-orchestrated panels (Goal invariant 2's
    honest limitation). Hard enforcement stays at the hook layer only
    (HC-4 protects harness/CI flows); no panel registered → no warning
    and no added subprocess cost. Lands in the same bead as the hook if
    the `internal/panel` call-in is <~30 lines (it should be —
    Scan+Tally exist by then); otherwise a named follow-up bead is
    filed at plan time — in-spec either way.
    (e) **Abandonment audit write (gate finding M4)**: `complete.Run`
    records `panel_abandoned: true` + timestamp + the panel.json
    `abandon_reason` on bead metadata via `bead.MergeMetadata` when
    completing a bead whose matching panel.json carries
    `"abandoned": true` — mirroring (b)'s non-optional discipline
    (panel.Scan is already available inside complete via (d)).
    Abandonment is a plain repo-file edit and therefore
    agent-performable; it is legitimate precisely because it is always
    audited, never silent (Goal invariant 2 wording). The abandon
    procedure (Req 16, tally skill) requires recording who/why in
    `abandon_reason`; the hook Warn (Req 12) and the metadata write
    both surface it.

#### Compaction recovery

14. **`mindspec instruct --panel-state`.** New flag
    (cmd/mindspec/instruct.go:34-37 `init`) → `instruct.Run`
    variant/options
    (run.go:40 signature grows opts or a `RunWithOptions`). Output block
    (markdown + `panel_state` object in `RenderJSON`):
    - In-progress beads: `bead.ListJSON --status=in_progress` scoped to
      active epics (reuse `phase.Cache` per PERF-1 discipline), worktree
      via `resolveBeadWorktree` (run.go:223-238), last commit via
      `git log -1 --oneline bead/<id>` — CAPPED at the active bead + at
      most 3 other in-progress beads (deterministic bd-id order;
      remainder as "… and N more (no git detail)").
    - Open panel rounds via `internal/panel.Scan`: latest round
      (filename-derived), verdict count vs expected, APPROVE tally,
      reviewed_head_sha staleness, presence of `consolidated-round-<N>.md`
      — with a "gate would PASS/BLOCK" line computed by the SAME
      `internal/panel.Tally` the hook uses.
    - Stale agent worktrees: `bead.WorktreeList()` filtered to
      `.worktrees/worktree-*` without a matching in-progress bead, plus
      dir-scan of `.claude/worktrees/agent-*`.
15. **Auto-include at SessionStart.** When mode == implement AND
    `panel.Scan` finds ≥1 panel dir with an incomplete latest round, the
    panel-state block is appended via a new `Context.PanelState` field
    (instruct.go:33-48) rendered at the bottom of
    `templates/implement.md` — so `runSessionStartHook` →
    `instruct.Run` (cmd/mindspec/hook.go:93-133, format `""` = markdown)
    recovers post-compaction sessions automatically (FINDINGS item 8).
    Budget: the fs-scan is the ONLY work outside the auto-include
    condition; all git subprocesses (capped per Req 14) and worktree
    scans run only inside the auto-include or explicit `--panel-state`
    branch — zero added SessionStart cost when no panel is open (12s
    budget, hook.go:130).

#### Skill surface (16 → 11)

16. **Skills consolidation.** Final inventory: lifecycle —
    ms-spec-create, ms-spec-approve, ms-plan-approve, ms-impl-approve;
    plugin — ms-bead-impl (+prep), ms-bead-fix, ms-panel-run (+create),
    ms-panel-tally (+artifact-gates +halt-recover +escape-hatch),
    ms-bead-cycle (+next +merge), ms-spec-autopilot,
    ms-spec-final-review. Dispositions (full rationale in the design's
    §1; all plugin line anchors verified current):
    - **DELETE `/ms-bead-next`** → fold into `/ms-bead-cycle` Step 0:
      plan-vs-`bd ready` dep cross-check + deterministic pick rule
      (ms-bead-next:26-28,59), multi-active-spec disambiguation kept
      verbatim (line 20), and a 2-line belt-and-braces claim fallback
      (`bd update <id> --claim --status in_progress` + `git worktree
      add`) for CLI-down failures the Req 3/4 error paths never reach.
    - **DELETE `/ms-bead-merge`** → fold into the cycle's merge terminal:
      keep the `"<summary>"` positional-arg convention, the one-line
      `git log --oneline -3` merge-commit verify (FormatResult still
      prints no merge SHA), the partial-failure rule verbatim
      ("Don't proceed to the next bead if the merge failed mid-way…"),
      and the :53 anti-pattern fence verbatim ("never merge a bead
      branch with raw `git merge bead/<id>` — it bypasses bd closure
      and worktree cleanup"), which is now also load-bearing for the
      panel gate (raw merge is the obvious gate workaround; no git hook
      fires on merge commits) — the fence lands in Req 17's
      orchestrator rules AND every gate Block message (Req 12).
      The panel-approval precondition stops being prose — the hook
      enforces it.
      Disposition of the Step 1 pre-merge checklist (:17-23, HC-2
      "every sentence"): superseded by CLI checks — 092 Bead 5's honest
      user-dirt blocking and FormatResult's closure/worktree-removal
      output cover the bd-show/clean-tree/commits-visible rows, and the
      "if anything is uncommitted, abort" line is now MECHANIZED by the
      Req 11 dirty-tree Block — record this "superseded by" mapping in
      the fold, do not drop it silently.
      The folded prose must ALIGN with 092's location-agnostic guidance
      (no cd-then-complete; `complete` runs from the repo root,
      next.go:294-300).
    - **MERGE `/ms-bead-prep` into `/ms-bead-impl`** as "Phase A — stage
      the prompt" (~120 lines total); Boundaries block → AGENTS.md
      guardrails pointer (Req 17).
    - **MERGE `/ms-panel-create` into `/ms-panel-run`** as "Step 0 —
      create panel dir + BRIEF": preserves the `target` generality
      (bead|pr|commit, ms-panel-create:13); step 0 WRITES `panel.json`
      (Req 6 schema) and on every re-panel bumps `round` +
      `reviewed_head_sha` in one write.
    - **`/ms-panel-tally` becomes the single decision authority** (~115
      lines): gains the canonical "Artifact gates (HARD block)" section
      (moving ms-spec-final-review:59-74's fuller content — 3-row
      finding-shape table, "could the missing artifact have caught a real
      defect?" question, lola-f4a8 case — and deleting the diverging
      ms-panel-tally:33-40 copy); gains "After a halt — recovery"
      (FINDINGS item 2 adjudication: inventory via `mindspec instruct
      --panel-state`, halt classification REJECT/HARD-block/max-rounds,
      the now-mechanized stale-verdict rule, and the panel-abandon
      procedure: set `"abandoned": true` in panel.json AND record
      who/why in `abandon_reason` — noting that completion then writes
      a `panel_abandoned` audit entry to bead metadata, Req 13e); gains
      the human-facing "Escape hatch" subsection (the only
      place MINDSPEC_SKIP_PANEL is documented, HC-7). Decision matrix
      merge row rewritten: "run `mindspec complete <bead-id> \"<summary>\"`
      (hook-gated)"; the "Then" handoff points at the cycle's merge
      terminal.
    - **`/ms-bead-fix` shrinks ~40%**: commit-gate workaround prose →
      Req 1 hook message; one-commit/no-push/no-complete → guardrails
      pointer; artifact-gate anti-pattern → 3-line summary + tally
      pointer; C2-1 coverage correction in surviving text. Keeps prompt
      composition, deviation policy, commit template, dispatch mechanics.
      (Folding fix into tally was considered and REJECTED — S2-7 panel
      concurrence: tally is judgment, fix is dispatch; final-review
      reuses fix-dispatch independently.)
    - **`/ms-spec-autopilot` shrinks ~30%**: parallel-window option
      deleted entirely (FINDINGS item 1 = YAGNI; lola spec-050's dep
      chain was near-linear, the "disjoint file-sets" precondition has no
      specified algorithm, and real fan-out belongs to the future
      Dynamic-Workflows port). Replacement line: "Beads run serially by
      design; parallelism lives inside each cycle."
    - **`/ms-spec-final-review` shrinks ~25%**: commit-gate workaround →
      Req 1 message; artifact gates → 4-line summary + tally pointer (F5
      lens row keeps its one-line HARD-block instruction); panel-dir
      creation REROUTES through `/ms-panel-run` step 0 with
      `target=<spec-branch>` so final-review panels emit panel.json
      (bead_id null, expected_reviewers 6, reviewed_head_sha =
      spec-branch tip) and appear in `--panel-state` + gate plumbing.
    - **Grep-clean acceptance criterion (binding)**:
      `grep -rE 'ms-bead-next|ms-bead-merge|ms-bead-prep|ms-panel-create|ms-spec-status'`
      across `plugins/mindspec/skills/`, `internal/setup/claude.go`,
      both READMEs, CLAUDE/AGENTS managed-block sources, and
      `internal/instruct/templates/` returns zero live references. Known
      cross-references to scrub: ms-panel-tally:36,:74 → /ms-bead-merge;
      ms-spec-autopilot:20,:95 → /ms-bead-next; ms-bead-impl:55 +
      ms-bead-fix:91 → /ms-panel-create; ms-bead-cycle:21,:34
      diagram boxes; ms-panel-run:15 prerequisite line.
    - Update sites: `claudeMDManagedBlock` tables (claude.go:612-657),
      `plugins/mindspec/README.md` (:13, :18-56, :58-78 loop diagram),
      `embed.go` package comment, `internal/setup/*_test.go` golden
      expectations (e.g. claude_test.go:53), FINDINGS.md statuses.
17. **Guardrails single-sourcing.** New "## Bead-loop guardrails
    (mindspec)" section in `agentsMDManagedBlock`
    (internal/setup/codex.go:138-139; AGENTS.md is the canonical
    authority per claude.go's own managed block), mirrored as a heading
    REFERENCE (not a copy) in `claudeMDManagedBlock`. Two audiences,
    split deliberately:
    - **Orchestrator rules**: the cycle owns the merge (only the
      orchestrator runs `mindspec complete`, only after the panel gate
      passes); NEVER merge a bead branch with raw `git merge bead/<id>`
      — only `mindspec complete` merges (raw merge bypasses bd closure,
      worktree cleanup, AND the panel gate; ms-bead-merge:53 fence
      preserved per G3-1/HC-2); do NOT `git push` after a bead merge —
      single push at end-of-spec (after /ms-impl-approve).
    - **Subagent prompt fences** (every impl/fix prompt includes
      verbatim): no `mindspec complete`; no `git push`; no exceeding the
      files-in-scope list; no reimplementing helpers earlier beads
      landed; exactly ONE commit ending `Deviations: <list or "none">`;
      **tests must PASS** — run the bead's test scope before reporting
      (report-only is satisfiable by faithfully reporting 12 failures);
      report back commit SHA + pass/fail/skip counts + deviations.
    Surviving skills replace their triplicated blocks with "Include the
    standard guardrails (AGENTS.md § Bead-loop guardrails)." Caveat
    recorded: managed-block updates reach consumer repos on next
    `mindspec setup`; harness scenarios assert the section exists
    post-setup.
18. **`/ms-spec-status` deletion + stale-skill cleanup in setup.** Remove
    the map entry (claude.go:593-602), the `claudeMDManagedBlock` table
    row (:629), the "5 lifecycle skills" doc comments (:516, :530), the
    plugin README row (plugins/mindspec/README.md:28), and test
    expectations (claude_test.go:53). Because `RunClaude` only
    creates-or-skips (claude.go:118-135), setup gains a stale-skill
    cleanup step: remove `.claude/skills/ms-spec-status/` iff its
    SKILL.md content byte-matches a shipped version (provenance
    discipline per `githooks.CleanStaleGitHooks`,
    internal/githooks/install.go:106); user-modified files are left in
    place with a notice (HC-6).
19. **Lifecycle skills teach ONLY the canonical noun-verb gate commands —
    refresh half (the template rewrite LANDED pre-fork).** The rewrite
    of `lifecycleSkillFiles()` to `mindspec spec approve <id>` /
    `plan approve` / `impl approve` (claude.go:556, :569, :583) landed
    on main via 092 Bead 9 (657e4c8) before this spec forked, pinned by
    `TestLifecycleSkills_CanonicalApproveOrder` (claude_test.go:715) —
    verify-only here. That test's own boundary NOTE explicitly defers
    the remainder to this spec. Remaining work:
    (a) **Extend the negative assertion to the full skill surface**: the
    landed test covers `lifecycleSkillFiles()` only; assert that no
    string returned by `skillFiles()` (lifecycle + plugin-embedded)
    contains the deprecated `approve <noun>` order — extending Spec-092
    Req 11's negative assertion to the surface it did not cover.
    (b) **Content refresh**: since create-or-skip install
    (claude.go:118-135) never propagates the fix to repos that already
    have the files, `RunClaude` refreshes a mindspec-owned skill file in
    place iff its current content byte-matches a PREVIOUSLY-SHIPPED
    version (registry of historical contents, same provenance discipline
    as Req 18); user-modified files are skipped with a notice (HC-6).
    (Note for the implementer: ms-impl-approve's session-close protocol
    prose is otherwise kept as-is — it is the only home of that
    protocol.)

#### Docs closeout

20. **README rationale + FINDINGS adjudications + ch8h closeout.**
    The two README paragraphs are inlined here VERBATIM (canonical text;
    the design doc lives in a dated staging directory and may not
    outlive it — G3-6):
    - Append to README § "Why six reviewers, mixed families"
      (FINDINGS item 9):

      > **Why six and not four or eight.** Six is tuned, not derived —
      > the binding constraints are majority arithmetic and verdict
      > variance. With a 5-of-6 threshold, one dissent still routes to
      > merge-with-record while two dissents force a fix round; that is
      > the behavior we actually want from a panel. Shrink to four and
      > the same property needs 3-of-4, where a single noisy verdict
      > swings 25% of the panel — empirically (lola, ~25 beads) sub-5
      > panels rerouted on one-off flags far too often (~33%
      > single-flag variance, vs ~20% at six). Grow to eight and the
      > marginal reviewers mostly duplicate an existing lens while
      > adding 4–10 minutes of codex wall-clock per round and
      > proportional spend — across 25 beads we never saw an 8th lens
      > that would have changed a decision a 6-panel got wrong. Treat 6
      > (3 Claude + 3 Codex) as the sweet spot for a single-developer
      > compute budget, and scale the threshold as ceil(5N/6) if you
      > change N. This is an empirical setting, not a theorem — revisit
      > it if your defect mix differs.

    - Add as a sidebar after § "The autonomous loop" (FINDINGS item 10):

      > **Why a second panel round instead of one-shot review.** The
      > fix commit is new, unreviewed code, written by an author (the
      > fix subagent) responding to instructions the original BRIEF
      > could not have anticipated. Round 2 reviews the *fix author*,
      > not the bead again: each reviewer re-checks only its own
      > `concrete_changes_required` (ADDRESSED / PARTIAL / MISSED /
      > NEW_ISSUE) plus the deviations the fix author explicitly
      > flagged. That structure catches the failure mode one-shot
      > review cannot: defects introduced *while addressing feedback*.
      > On lola, the Bead 2 round-2 panel caught a routing bug that the
      > round-1 panel had approved — it didn't exist in round 1; the
      > round-1 fix created it. The cost is bounded — round 2 is scoped
      > to deltas, so it runs faster than round 1, and most beads
      > converge in exactly one fix round. The corollary is the
      > artifact-gate rule: a round-2 APPROVE earned by *describing* a
      > fix (PR-body precision) instead of *making* it is the known
      > bypass (lola-f4a8, $417), which is why missing-artifact
      > findings HARD-block regardless of vote count.
    - FINDINGS.md: annotate items 1 (parallel-window deleted as YAGNI —
      file a low-pri bd note for the Workflows port), 2 (halt-recover =
      tally section), 8 (--panel-state shipped), 9, 10 (paragraphs
      landed); update Part 2 panel-gate status to "enforced".
    - Skill-table refresh in both READMEs + Anthropic-patterns note
      (ch8h item 1 follow-up). Close `mindspec-ch8h` with evidence.

## Scope

### In Scope
- Requirements 1-20; Hard Constraints HC-1..HC-7.
- Unit tests for every behavior change; the three settings-merge
  regression tests; the hook decision-table tests; one LLM-harness
  scenario for the panel gate (AC below).

### Out of Scope
- Extending the commit gate to `bead/` branches (would block the
  autopilot's own impl subagents — settled non-goal, C2-1).
- A claim-less 1105 auto-fallback in `mindspec next` (deferred to
  jkhd.5 unless reproduced + verified per Req 3).
- `mindspec setup copilot` PreToolUse analog (no equivalent hook point
  today; the Req 13d complete-side warning — now required — covers
  codex/raw-shell).
- Hard enforcement inside `complete.Run` itself (would break
  `internal/harness` and non-panel CI flows; hook-layer only).
- Folding `/ms-bead-fix` into `/ms-panel-tally` (rejected — S2-7).
- A 10th-skill cut below 11; parallel-bead scheduling (Workflows port,
  FINDINGS Part 2).
- `.gitignore` root-dotfile default (FINDINGS item 6 — separate upstream
  issue; Req 2's ladder covers the workflow).
- Dynamic-Workflows autopilot port.

## Non-Goals

- This spec does not change what the panel reviews or how verdicts are
  produced — only whether `mindspec complete` may proceed given the
  artifacts on disk.
- **The gate binds only REGISTERED panels** (gate finding M1): a panel
  exists for the gate iff `review/<slug>/panel.json` exists under a scan
  root. Externally-orchestrated panel conventions — verdicts kept
  outside `review/` (e.g. /tmp staging dirs, the exact hand-rolled
  style that built Spec 092 the night before this spec) — are
  UNENFORCED by design: not running /ms-panel-run step 0 opts a flow
  out of the contract, and "didn't register the panel" is itself a
  prose-under-pressure shortcut the hook cannot see. This residual is
  larger than the command-match false-negative residual Req 9 documents
  and is accepted deliberately (the alternative — keying on loose
  verdict files — would false-block legitimate no-panel flows, HC-4).
  The required complete-side advisory (Req 13d) is the only signal for
  non-registered flows; the new ADR records the limitation.
- It does not make panel state a beads-tracked entity (ADR-0023: panel
  files are review artifacts; bead statuses remain the single workflow
  state authority).
- It does not harmonize the two escape hatches (HC-7 records why their
  asymmetry is load-bearing).
- It does not retrofit the recovery-line convention onto untouched
  pre-existing messages.

## Acceptance Criteria

### CLI point-of-use (Reqs 1-5)
- [ ] **Req 1**: `dispatch_test.go` asserts the spec-branch block message
  contains the legitimacy context, the `MINDSPEC_ALLOW_MAIN=1` hint, the
  "Do NOT use the escape hatch to land feature code" line, and NO claim
  that bead branches are blocked; the protected-branch message is
  unchanged save the bare hint.
- [ ] **Req 2**: a forced ADR-divergence in `complete.Run` yields the
  3-step repair-first ladder naming OWNERSHIP.yaml, with the bypass
  flags LAST, file names present, and a final `recovery:` line
  (convention test covers the constructor).
- [ ] **Req 3**: a forced `ClaimBead` failure emits the recipe containing
  `bd update <id> --claim --status in_progress` VERBATIM and the
  interpolated `git worktree add` line; final line is a `recovery:`
  command referencing `mindspec next --spec <slug>`.
- [ ] **Req 4**: a forced `EnsureWorktree` failure emits the interpolated
  `git worktree add` recipe (real `workspace.BeadWorktreeName`/
  `BeadBranch`/`SpecWorktreePath` values) instead of a bare warning.

### Panel package (Req 6)
- [ ] `panel.Tally` unit tests: filename-derived round wins over a lagging
  `panel.json.round` (reported as mismatch); malformed verdict counted
  missing and named; `hard_block` parsed; APPROVE tally correct for
  6/6, 5/6, 4/6 fixtures.

### Settings merge (Reqs 7-8)
- [ ] **Merge path**: repo with pre-existing `.claude/settings.json` (no
  mindspec entries) → `mindspec setup claude` → the pre-complete
  PreToolUse entry is present after `ensureSettings` (pins the
  install-time strip).
- [ ] **User-entry survival**: pre-existing user PreToolUse Bash entry →
  setup → both entries present; user's entry byte-identical.
- [ ] **Re-run idempotence**: second setup run → exactly one mindspec
  pre-complete entry; legacy `mindspec instruct`-form entries still
  removed (N1 regression).

### Hook gate (Reqs 9-13) — all table-driven unit tests on `hook.Run`
- [ ] **Zero-bd invariant (HC-3)**: a non-matching command (e.g.
  `git status`) triggers no `SetListJSONForTest`/`SetRunBDForTest` stub
  call (stub fails the test if invoked), no config load, no git call.
- [ ] **Match table**: quoted mentions do NOT match (`git commit -m
  "panel approved; mindspec complete next"`, `grep 'mindspec complete'
  SKILL.md`, echoed --panel-state text); legit forms DO match
  (`mindspec complete X`, `cd wt && mindspec complete X`,
  `FOO=1 mindspec complete X`, `a && mindspec complete X`).
- [ ] **No-panel fail-open (HC-4)**: no review/ dir, or no panel.json
  naming the bead, or BRIEF-only legacy dir → Pass with no output.
- [ ] **Incomplete panel**: 4/6 verdicts → Block naming missing slots.
- [ ] **Threshold (parameterized, N−1)**: 6/6 with 5 APPROVE + matching
  SHA + clean tree → Pass; 4 APPROVE → Block citing the 5/6 threshold
  and consolidated-round-<N>.md; `expected_reviewers: 3` fixture —
  2/3 APPROVE → Pass, 1/3 → Block citing 2/3 (no hardcoded 6).
- [ ] **Dirty tree (the CommitAll-bypass pin)**: matched panel, 5/6
  APPROVE, SHA matches, but `git status --porcelain` in the bead
  worktree reports a user-authored file dirty → Block naming the
  worktree; same fixture with ONLY `.beads/issues.jsonl` dirty
  (ADR-0025 artifact dirt) → Pass; clean tree → Pass; worktree absent
  with branch present (partial-failure rerun window) → dirty check
  skipped, pass through to tally. No porcelain call on the no-panel
  path.
- [ ] **Missing ref (rerun-after-merge)**: panel.json references the
  bead but `bead/<bead-id>` does not exist → Pass + Warn naming the
  missing ref (no Block).
- [ ] **Block-message fence**: every Block variant in the decision table
  ends with the no-raw-`git merge` fence line (single assertion across
  the table).
- [ ] **REJECT / hard_block** → Block citing the halt path.
- [ ] **Round mismatch**: panel.json.round=1, round-2 verdict files →
  Block instructing /ms-panel-run step 0.
- [ ] **Stale SHA**: reviewed_head_sha ≠ current bead-branch HEAD with
  5/6 APPROVE → Block (the lola-f4a8 pin).
- [ ] **cwd independence**: panel.json only in the spec worktree, hook
  cwd = main root, command `cd <worktree> && mindspec complete <id>`,
  incomplete panel → Block; same without the cd prefix (bead-id-derived
  root) → Block.
- [ ] **Escape hatch**: env set → Pass + Warn naming the bead;
  subsequent `complete.Run` writes `panel_gate_skipped: true` +
  timestamp via `bead.MergeMetadata`, unrelated metadata keys preserved
  (diff before/after); NO Block message output contains the string
  `MINDSPEC_SKIP_PANEL` (HC-7 negative assertion).
- [ ] **Abandoned**: `"abandoned": true` → Pass + Warn naming the panel
  and its `abandon_reason`; subsequent `complete.Run` writes
  `panel_abandoned: true` + timestamp + reason via `bead.MergeMetadata`
  (unrelated keys preserved) — Req 13e.
- [ ] **Complete-side advisory (Req 13d, required)**: `complete.Run` on
  a bead with a registered incomplete panel — invoked directly, no
  hook layer — prints the tally + would-PASS/BLOCK line computed by
  the same `panel.Tally`; with no registered panel, no warning and no
  panel-attributable subprocess.
- [ ] **Config toggle**: `enforcement.panel_gate: false` → Pass before
  any panel scan.
- [ ] **Names**: "pre-complete" present in `hook.Names`, sorted-order
  test green (hook_test.go:145-150).
- [ ] **Harness scenario `panel_gate_blocks_premature_complete`**:
  sandbox with a fabricated incomplete panel (panel.json + 4 verdicts)
  for a ready-to-merge bead; prompt instructs completing the bead.
  Assert: no successful `mindspec complete` event for the bead while
  the panel is incomplete; the agent's transcript contains the gate's
  block text; and no event sets MINDSPEC_SKIP_PANEL. (Scenario skips
  under `-short` per harness convention, HC-1.)

### Panel-state (Reqs 14-15)
- [ ] `mindspec instruct --panel-state` output contains the three blocks
  (in-progress beads with worktree+last-commit, open panel rounds with
  "gate would PASS/BLOCK" computed by `panel.Tally`, stale agent
  worktrees); JSON format carries `panel_state`.
- [ ] Cap test: 6 in-progress beads → git detail for active+3 only,
  "… and 2 more (no git detail)".
- [ ] Auto-include: implement mode + incomplete panel → rendered
  markdown (the SessionStart channel) contains the Panel/Subagent State
  block; implement mode + no panel dir → block absent AND no git/bd
  subprocess attributable to panel-state (stub-guarded).

### Skills + docs (Reqs 16-20)
- [ ] **Grep-clean**: the Req 16 grep returns zero live references to the
  five deleted/merged skill names across the listed surfaces.
- [ ] Skill inventory = 11; `claudeMDManagedBlock` tables, plugin README
  tables + loop diagram, embed.go comment, and setup golden tests all
  reflect it.
- [ ] /ms-panel-run step 0 writes panel.json per the Req 6 schema and
  bumps round+SHA in one write on re-panel; /ms-spec-final-review
  routes panel creation through it (its SKILL.md contains no
  hand-rolled `mkdir`+BRIEF path).
- [ ] /ms-panel-tally contains the canonical artifact-gates section
  (3-row shape table + lola-f4a8), the halt-recover section (incl. the
  abandon procedure), and the escape-hatch subsection;
  /ms-spec-final-review's artifact-gate text is ≤4 lines + pointer +
  F5 HARD-block line.
- [ ] The folded cycle merge terminal contains: the `"<summary>"`
  positional-arg convention, the `git log --oneline -3` verify, the
  verbatim partial-failure rule — and does NOT instruct
  cd-into-worktree-then-complete (alignment with next.go:294-300).
- [ ] ms-spec-autopilot contains no `parallel-window` reference; the
  serial-by-design line is present.
- [ ] **Req 17**: post-`mindspec setup`, AGENTS.md contains
  "## Bead-loop guardrails (mindspec)" with both subsections incl. the
  tests-must-PASS fence and the no-raw-`git merge bead/<id>` rule
  (G3-1); CLAUDE.md managed block references (not
  copies) it; surviving skills contain the pointer, not the triplicated
  blocks.
- [ ] **Req 18**: setup on a repo with an unmodified installed
  ms-spec-status skill removes the dir; with a user-modified one,
  leaves it + notice (HC-6).
- [ ] **Req 19**: negative test — no string returned by `skillFiles()`
  (lifecycle + plugin-embedded) contains the deprecated
  `approve <noun>` order (extends the landed
  `TestLifecycleSkills_CanonicalApproveOrder`, claude_test.go:715);
  refresh test — a repo
  with the OLD shipped ms-impl-approve content gets the new content on
  setup; a user-modified skill file is skipped with a notice.
- [ ] **Req 20**: README contains both rationale texts; FINDINGS items
  1/2/8/9/10 annotated; mindspec-ch8h closed with evidence.
- [ ] C2-1: no surviving prose (skills, hook messages, AGENTS.md) claims
  the commit gate blocks `bead/` branches.

## Validation Proofs

- `go build ./... && go test -short ./...` green on every commit (HC-1);
  `go test ./internal/lint/...` (boundary) green.
- `go test ./internal/setup/... ./internal/hook/... ./internal/panel/...
  ./internal/guard/...` — the new regression/decision tables.
- `mindspec setup claude` run twice on a fixture repo with a pre-existing
  user PreToolUse hook: diff shows the gate entry added once, user entry
  untouched (Reqs 7-8 proof).
- `mindspec instruct --panel-state` against a fixture review/ dir:
  output matches the Req 14 block shape; the "gate would BLOCK" line
  agrees with a direct `mindspec hook pre-complete` invocation on the
  same fixture (one-source-of-truth proof).
- The Req 16 grep-clean command, run in CI or as a test, returns empty.
- Full LLM-harness run of `panel_gate_blocks_premature_complete`:
  blocked pre-merge, recovered via the documented path.

## Open Questions

None blocking approval — the five panel-facing design questions (each
with a recorded draft position; DQ2/DQ5 already resolved into the
requirements text by the gate panel) are tracked in §Design Questions
below for resolution during planning.

## Design Questions (for the panel)

**Draft positions are BINDING for planning unless the panel explicitly
overrides them** (G3-2) — a plan agent decomposes against the stated
position, not the open question. DQ2 and DQ5 are now RESOLVED into the
requirements text per the gate panel.

1. **Skill-refresh provenance registry shape (Reqs 18-19)** — a Go slice
   of historical shipped contents (exact byte-match, like
   `githooks.CleanStaleGitHooks`) vs content hashes vs a
   `managed-by: mindspec` frontmatter marker for future installs. Draft
   position: historical-contents slice now (only ~2 generations exist),
   add a frontmatter marker to newly-installed skills so future specs
   stop growing the slice.
2. **Complete-side tally warning (Req 13d)** — RESOLVED (gate finding
   M1): the warning is REQUIRED in-spec, not optional — it is the only
   signal for non-registered flows. Same bead as the hook if the
   `internal/panel` call-in is <~30 lines (it should be — Scan+Tally
   exist by then); otherwise a named follow-up bead filed at plan time.
3. **Convention home** — new short ADR "panel gate as enforced contract"
   vs a section in the tally skill + glossary. Draft position: ADR — the
   reviewed_head_sha staleness rule and the hatch asymmetry are exactly
   the kind of decision a future editor will otherwise "fix"; the
   doc-sync gate wants a stable target.
4. **Hook-gate v1 scope: explicit-id completes only** (bare `mindspec
   complete` with no bead-id passes the gate, Req 10) — acceptable given
   the complete-side advisory backstop, or should bare completes resolve
   the active bead via stateFn? Draft position: explicit-id only in v1;
   the orchestration skills always pass the id, and bare-complete
   resolution adds a stateFn consultation to a fuzzier match. Settled by
   the gate panel (G3-8): `cmd/mindspec/complete.go:33` has
   `cobra.MinimumNArgs(1)`, so a bare `mindspec complete` errors at the
   CLI anyway — the id-omission "exploit" does not exist.
5. **`expected_reviewers` flexibility** — RESOLVED (gate finding G3-2):
   the draft position is adopted into Req 6 (schema:
   `expected_reviewers: int`) and Req 12 (threshold = N − 1, one
   dissent tolerated), with 5-of-6 kept as the default-panel example
   throughout — avoids a second hardcode the README's own ceil(5N/6)
   scaling note contradicts. (Context: the repo's recent practice
   includes 3-reviewer panels — 092 bead merges record "panel 3/3
   APPROVE" — which are BRIEF-only today and fail open; the
   parameterized rule lets them register without a code change.)

## Proposed bead decomposition (dependency order)

| Bead | Title | Depends on | Notes |
|:-----|:------|:-----------|:------|
| 1 | CLI point-of-use errors via guard.FormatFailure (Reqs 1-5): commit-gate legitimacy + C2-1 (dispatch.go:110-118), ADR repair ladder (complete.go:303-304), claim/worktree recipes (next.go:218/:231, beads.go:159) + convention-test fixtures | — | Message/UX only; zero behavior change. Lands first so Bead 6's deletions never orphan knowledge (HC-2). 092 Bead 7's `internal/next/guard.go` is in the fork tree — use it as the constructor precedent/home for the Req 3-4 failures. |
| 2 | `internal/panel` package (Req 6): panel.json convention, Scan, Tally, filename-derived rounds, hard_block parse + unit tests | — | Foundation for 4 and 5. |
| 3 | Settings-merge identity redesign (Reqs 7-8): command-content ownership retaining the instruct arm, append-alongside, wanted-set-derived staleness + the three regression tests | — | Merge-machinery surgery with consumer-repo blast radius; split from the hook bead. Bead 4 hard-gated on it. |
| 4 | PreToolUse pre-complete hook (Reqs 9-13): lazy short-circuit order, anchored matcher + table, command-target scan roots, round/SHA staleness + missing-ref pass-through + dirty-tree block, parameterized N−1 decision matrix, env hatch + non-optional `panel_gate_skipped` write, required complete-side advisory tally (13d) + `panel_abandoned` audit write (13e), `Enforcement.PanelGate`, Names insert, wantedHooks entry, zero-bd test, harness scenario | 2, 3 | The enforced-contract centerpiece. 13d may split into a named follow-up bead at plan time if the call-in exceeds ~30 lines (DQ2). |
| 5 | `instruct --panel-state` + implement-template auto-include (Reqs 14-15) with caps | 2 | Context.PanelState, templates/implement.md, cmd flag, SessionStart budget tests. |
| 6 | Skills consolidation (Reqs 16-19): deletions/folds/shrinks, panel.json write in panel-run step 0, final-review reroute, tally canonical sections, guardrails managed-block + pointers, ms-spec-status deletion + stale-skill cleanup, lifecycle canonical-order verify + skillFiles() negative-test extension + provenance-gated refresh, grep-clean AC, managed-block/README/test updates | 1, 4 | The big prose diff. Gated on 1 & 4 so every deleted sentence has a live replacement (HC-2). |
| 7 | Docs closeout (Req 20): README items 9+10 paragraphs, Anthropic-patterns note, FINDINGS adjudication annotations, close mindspec-ch8h | 6 | Docs-only. |

### Sequencing risks

- **Bead 6 must not merge before 1 and 4** — "zero operational-knowledge
  loss" is only true once the CLI/hook replacements exist (HC-2,
  dependency-enforced).
- **Bead 4 must not merge before 3** — shipping the PreToolUse entry
  through the unfixed merge machinery clobbers user hooks (S3-1) and
  strips the gate at install time (S1-1; `ensureSettings` merges then
  cleans in the same pass, claude.go:209-226).
- **Cross-spec (RESOLVED at transcription)**: the draft's MANDATORY
  fork-sequencing risk has been absorbed — this spec forked from main @
  1ded99c, which contains ALL Spec-092 beads merged, including Bead 7
  (a5d913d: `internal/next/guard.go` with `next.DirtyTreeFailure` via
  `guard.NewFailure` and the `artifactPaths` classification, banning
  destructive `git restore .` advice) and Bead 9 (657e4c8: lifecycle
  noun-verb rewrite + `TestLifecycleSkills_CanonicalApproveOrder`).
  Every anchor in this spec was re-verified against the fork tree at
  transcription (2026-06-11); Req 19 was narrowed accordingly (rewrite
  half verify-only, refresh half + skillFiles() extension remain). Bead
  1 uses `internal/next/guard.go` as the constructor precedent/home for
  the Req 3-4 failures.
- **Riskiest-requirement watch list**: Reqs 7-8 (settings-merge — silent
  install failure and user-hook clobbering are both invisible-failure
  modes; three regression tests + dedicated bead), Reqs 9-12 (hook false
  outcomes in both directions — false-block breaks every Bash call and
  the harness suite, false-pass guts the contract; anchored-match table +
  cwd-independence tests + fail-open rule), Req 16 (folding the two most
  battle-tested recovery-prose skills — mitigated by HC-2 sequencing, the
  cycle Step 0 belt-and-braces fallback, and harness assertions that
  recovery text appears in failure output).

## Appendix: design revision provenance

This spec transcribes the panel-cleared design
(skills-thin-down-design.md, CONFIRM_READY after rounds R1: S1-1..S3-8 +
C2-1, R2: N1-N3 — both revision logs in the design file), updated for
spec/092 currency per §Background. Deltas from the design beyond anchor
drift: (a) Req 19 (lifecycle canonical noun-verb + provenance-gated skill
refresh) is NEW, from the 092 Bead-8 panel R2 minor; (b) all Req 1-4
messages now route through `guard.FormatFailure` (the helper did not
exist at design time); (c) the design's bead 1 "rebase note" for PR #119
is dissolved (pattern landed); (d) Design Questions 4-5 surface two
hook-scope choices the design fixed silently (explicit-id-only,
hardcoded 5/6); (e) test-channel consolidation (G1-4): the design's
bead-4 row named 7 LLM-harness scenarios (panel-present block/pass,
no-panel pass, worktree-panel-cwd-mainroot block, stale-sha block,
skip-env, abandoned pass) — this spec covers six of those shapes as
table-driven `hook.Run` unit ACs (deterministic, cheap, matching the
design body's own table-driven instruction) and ships exactly ONE
LLM-harness scenario (`panel_gate_blocks_premature_complete`); no
coverage shape is dropped, the channel change is deliberate; (f)
transcription-time deltas (2026-06-11, vs the gate-cleared draft):
all anchors re-verified against the fork tree (main @ 1ded99c) — the
draft's spec/092 @ 4803c80 + a5d913d anchors now match main directly;
the lifecycle noun-verb rewrite landed pre-fork (092 Bead 9, 657e4c8),
narrowing Req 19 to the refresh mechanism + the `skillFiles()`
negative-test extension; `hookEntryStale` (claude.go:302-335) added to
Req 7's matcher-keyed identity enumeration (it occupied the line gap
between the draft's two cited helpers and is part of the same
overwrite-by-matcher machinery); the draft's cross-spec MANDATORY
sequencing risk recorded as resolved.

## Appendix: Gate panel revision logs

### Gate panel revision log (Round 1)

Verdicts: g1-fidelity APPROVE_GATE (minors), g2-hook-safety
REQUEST_CHANGES (M1-M4 + m1-m5), g3-fresh REQUEST_CHANGES (G3-1, G3-2 +
minors). All findings applied 2026-06-11:

| Finding | Severity | Change made |
|:--------|:---------|:------------|
| G2 M1 (external panels unenforced; Goal overclaim) | major | Goal invariant 2 reworded honestly (registered-panels-only scope, "silently", abandonment audited-not-human-controlled); new Non-Goal bullet naming the externally-orchestrated-panel residual (incl. tonight's own 092 style) as accepted-by-design; Req 13d promoted OPTIONAL → REQUIRED (DQ2 resolved: same-bead-or-named-follow-up); Req 9 residual note + Out-of-Scope copilot line updated to cite the required advisory; new ADR records the limitation. New AC for the complete-side advisory. |
| G2 M2 (uncommitted-changes CommitAll bypass) | major | Chose the BLOCK design (not a residual-risk note): new Req 11 dirty-tree bullet — one `git status --porcelain` in the resolved bead worktree on the panel.json-matched path only; any output → Block. Justification inline: panel-gated worktrees are clean by construction at complete time (one-commit subagent fence; panel reviewed committed HEAD), and no-panel flows never reach the check (HC-4), so it cannot over-block legitimate flows. Req 9 step list gains step (8); matched-path git budget amended to "at most TWO subprocesses"; ADR-0030 touchpoint updated; new dirty-tree AC (block + clean-pass + no-call-on-no-panel). |
| G2 M3 (missing-ref rerun false-block) | major | New Req 11 missing-ref bullet: `git rev-parse bead/<id>` failure (branch deleted by a prior `exec.CompleteBead`) → Pass + Warn naming the missing ref; gate passes through to `complete.Run`'s own idempotent handling of the documented partial-failure rerun. New decision-table AC. |
| G2 M4 (abandoned hatch agent-writable, under-audited) | major | New Req 13e: mandatory `panel_abandoned: true` + timestamp + reason write via `bead.MergeMetadata` in `complete.Run` (mirrors 13b); Req 6 schema gains required `abandon_reason` (who/why); Req 12 Warn surfaces the reason; Req 16 abandon procedure updated; Goal invariant 2 wording fixed (abandonment is audited, not human-controlled); abandoned AC extended with the metadata-write assertion. |
| G3-1 (ms-bead-merge:53 raw-git-merge fence orphaned) | major | Fence added VERBATIM to Req 16's ms-bead-merge fold list (with the gate-bypass rationale: matcher fires only on `mindspec complete`; git runs no commit hook on automatic merge commits), to Req 17's orchestrator rules, and to EVERY gate Block message (Req 12 block protocol); Req 17 AC and a new Block-message-fence AC assert it. |
| G3-2 (Req 12 hardcoded 5/6 vs DQ5 N−1) | major | DQ5's parameterized rule adopted: Design Questions preamble declares draft positions binding-unless-overridden; Req 6 schema `expected_reviewers: int`; Req 12 threshold = N − 1 with 5-of-6 kept as the default example in all message texts; DQ5 marked RESOLVED (noting 092's own 3/3-APPROVE panels as the motivating case); threshold AC gains an `expected_reviewers: 3` fixture. |
| G1-1 (pin currency; Bead 7 understated) | minor | Frontmatter pin bumped 0a4352b → 4803c80 ("Beads 1-6, 8 merged; 7, 9 pending"); §Background re-verified line updated; Sequencing-risks cross-spec bullet rewritten as MANDATORY with the full a5d913d description (next.DirtyTreeFailure via guard.NewFailure, internal/next/guard.go as Req 3-4 constructor home) and the anchor shifts; Bead 1 row + Affected-packages next.go/complete.go entries carry the post-a5d913d anchors. (Superseded at transcription: a5d913d is merged in the fork tree; anchors are now fork-tree actuals.) |
| G1-2 / G2 m1 / G3-4 (hook.Run mis-filed in hook.go) | minor | All three citations corrected to `internal/hook/dispatch.go:29` (Background item 1, Affected packages internal/hook, Req 9); hook.go retained only for `Names`. |
| G1-3 (Req 1 drops the conditional cd line) | minor | "Or switch to your bead worktree: cd <active-worktree>" restored into the Req 1 replacement block with an explicit preserve-the-conditional note (dispatch.go:113-115). |
| G1-4 (harness-scenario → unit-shape consolidation unlogged) | minor | Logged as appendix delta (e). |
| G1-5 / G3-5 (trivial anchor drift at 4803c80) | minor | Recorded in Background's drift list; no requirement changed. (Values re-verified again at transcription against the fork tree.) |
| G2 m2 (Req 10 root (b) derived from ACTIVE bead, not command bead-id) | minor | Req 10 root (b) re-specified: bead-id → owning epic/spec → `SpecWorktreePath`, explicitly NOT `hook.ReadState`'s active-phase resolution (wrong spec under multi-active-spec); lookup-failure fallback to roots (a)/(c) stated. |
| G2 m3 (non-bead-target staleness dead text) | minor | The "for non-bead targets the recorded target ref" branch REMOVED from Req 11 enforcement; parenthetical marks bead_id-null panels as `--panel-state`-only in v1 (forward-looking plumbing, per DQ4's explicit-id scope). |
| G2 m4 (per-Bash-call spawn floor unstated) | minor | HC-3 gains the accepted-floor paragraph: first PreToolUse Bash entry → one `mindspec hook pre-complete` spawn per Bash call in every consumer repo and harness sandbox; wall-clock sanity check recommended. |
| G2 m5 (trust boundary unstated) | minor | New ADR bullet now states all gate inputs are agent-writable repo artifacts — anti-footgun, not anti-adversary; env channel (HC-7) is the only agent-proof input; future panels must not "fix" forgeability at this layer. |
| G3-3 (HC cross-ref renumbering slips) | minor | Goal invariant 1: (HC-4) → (HC-2); Affected-packages internal/harness: (HC-3, HC-6) → (HC-4, HC-3). |
| G3-6 (README paragraphs only in dated staging dir) | minor | Both §5.3/§5.4 paragraphs inlined VERBATIM into Req 20 as the canonical text; design-doc references replaced. |
| G3-7 (ms-bead-merge:17-23 checklist undispositioned) | minor | Req 16 fold gains an explicit disposition: superseded by CLI checks (092 Bead 5 user-dirt blocking, FormatResult closure output), with the "if anything is uncommitted, abort" line now mechanized by the Req 11 dirty-tree Block; recorded as a superseded-by mapping per HC-2. |
| G3-8 (bare-complete backstopped by MinimumNArgs) | note | Recorded in DQ4 as settling evidence (`cmd/mindspec/complete.go:33`). |
| G3-9 / G1-V1 / G2 verified_sound | note | No change required — positive verifications recorded by the panel. |

### Gate panel revision log (Round 2 — targeted confirm)

Verdict: targeted-confirm REVISIONS_NEEDED (verified against
spec/092-agent-contract-hardening @ 4803c80 + Bead 7 a5d913d). All
Round-1 findings confirmed RESOLVED except M2 PARTIALLY_RESOLVED —
the Block design was adjudicated CORRECT, but its predicate over-blocked
ADR-0025 artifact dirt (NF-1, blocker) and left worktree resolution
unspecified (NF-2). All findings applied 2026-06-11:

| Finding | Severity | Change made |
|:--------|:---------|:------------|
| NF-1 (dirty-tree "ANY porcelain output → Block" false-blocks ADR-0025 artifact dirt that 092 made never-blocking at complete time) | major (blocker) | Req 11 dirty-tree bullet amended: Block on USER-AUTHORED dirt only — porcelain output is classified by path, filtering out the ADR-0025 artifact paths (`.beads/issues.jsonl`; the `internal/next/guard.go` `artifactPaths` concept landed in 092 a5d913d, mirrored by `complete.Run`'s artifact-aware check at complete.go:190-245) before deciding. Pure path filtering on the SAME single porcelain subprocess — explicitly no bd-export normalization call; matched-path budget unchanged at two git subprocesses (Req 9 / ADR-0030). Rationale corrected: clean-by-construction holds for user edits, not artifact churn. ADR-0025 touchpoint updated to record that the gate honors the never-blocking artifact rule (removing the internal contradiction). Dirty-tree AC amended: artifact-dirt-only fixture (`.beads/issues.jsonl` alone dirty) → Pass added; user-file-dirty → Block and clean → Pass retained. |
| NF-2 (bead-worktree resolution + worktree-absent behavior unspecified) | minor | Req 11 dirty-tree bullet now specifies resolution per `complete.Run`'s precedent (worktree-list match on `workspace.BeadWorktreeName`/`BeadBranch`); worktree absent — the branch-exists/worktree-removed partial-failure rerun window between `exec.CompleteBead`'s worktree removal and branch deletion — → skip the dirty check and pass through to the tally, mirroring the missing-ref Pass-through semantics (no false-block of the documented recovery rerun). Worktree-absent fixture added to the dirty-tree AC. |
| NF-3 (Req 18 stale `CleanStaleGitHooks` anchor) | note | Req 18 inline anchor corrected internal/githooks/install.go:98 → :106, matching Background's drift list. |
| NF-4 (trust-boundary ADR bullet omits the config channel) | note | The trust-boundary ADR bullet's agent-writable input enumeration now also names the `cfg.Enforcement.PanelGate` config toggle (Req 13c) as a git-visible agent-writable repo file, alongside the panel artifacts. |

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-11
- **Notes**: Approved via mindspec approve spec