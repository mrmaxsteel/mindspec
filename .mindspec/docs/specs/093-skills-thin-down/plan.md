---
adr_citations:
    - id: ADR-0023
    - id: ADR-0025
    - id: ADR-0030
    - id: ADR-0035
    - id: ADR-0037
approved_at: "2026-06-11T20:07:59Z"
approved_by: user
bead_ids:
    - mindspec-cter.1
    - mindspec-cter.2
    - mindspec-cter.3
    - mindspec-cter.4
    - mindspec-cter.5
    - mindspec-cter.6
    - mindspec-cter.7
spec_id: 093-skills-thin-down
status: Approved
version: "1"
---
# Plan: 093-skills-thin-down

This plan decomposes spec 093 (Reqs 1-20, HC-1..HC-7) into the seven
beads pinned by the spec's §Proposed bead decomposition, with the
spec's dependency edges adopted verbatim (Bead 4 hard-gated on Bead 3;
Bead 6 on Beads 1 and 4; Bead 7 last). Bead descriptions cite
requirement and AC numbers from the spec rather than inlining their
text (per the mindspec-lawq rule: bead payloads stay lean; full text
lives in `spec.md`). All file:line anchors below were re-verified
against this branch (fork of main @ 1ded99c) at plan-fill time.

## ADR Fitness

The spec's impacted domains are **workflow** (`internal/panel`,
`internal/hook`, `internal/complete`, `internal/config`,
`internal/instruct`, `internal/harness`) and **execution**
(`cmd/mindspec/next.go`, `internal/setup`, `plugins/mindspec`).
Frontmatter citations cover each:

- **ADR-0023** (beads as single state authority — Accepted, domains:
  workflow, git, state). Covers the workflow domain. The spec honours
  its stance exactly as its ADR Touchpoints record: `panel.json` and
  verdict JSONs are review ARTIFACTS, not workflow state — the gate
  reads them to decide whether a state-changing command may proceed and
  writes nothing to them; lifecycle state stays derived from bd
  statuses. The only bead-metadata writes added (`panel_gate_skipped`,
  `panel_abandoned`, Reqs 13b/13e) use `bead.MergeMetadata` merge
  semantics per the Spec-092 Req 19 precedent. Sound; adhere.
- **ADR-0025** (beads JSONL as build artifact — Accepted, domains:
  workflow, execution, bootstrap). Covers both impacted domains.
  Behavior untouched; load-bearing in two places: Req 3's emitted
  recipes reference `mindspec complete`'s artifact-aware behavior, and
  the Req 11 dirty-tree Block FILTERS the ADR-0025 artifact paths
  (`.beads/issues.jsonl`) out of its predicate so the gate never blocks
  on artifact dirt 092 made never-blocking at complete time (spec
  round-2 finding NF-1). Sound; adhere — Bead 4 encodes the filter.
- **ADR-0030** (executor boundary — Accepted, domains: execution,
  validation, lifecycle, lint). Covers the execution domain. The
  pre-complete hook performs at most TWO git subprocesses on the
  matched path (one `rev-parse` staleness check, one
  `status --porcelain` dirty check, Reqs 9/11), mirroring the existing
  hook-side precedent (`runPreCommit` calls `gitutil.CurrentBranch`
  directly, dispatch.go:63); zero git/fs/bd work on the non-match path
  (HC-3). Boundary lint (`internal/lint`) green per HC-1. Sound;
  adhere.
- **ADR-0035** (agent error contract — Accepted, domains: workflow,
  execution, core). On main and present in this branch's fork base
  (main @ 1ded99c); citable. Covers both impacted domains. This spec
  is a direct consumer: HC-5 routes every new/changed guard message
  through `guard.FormatFailure`/`NewFailure` with final
  `recovery: <command>` lines, extends the Spec-092 convention test to
  the new constructors (Req 5, Bead 1), and carries forward the
  raw-`bd update --metadata` ban. The HC-5 exception (PreToolUse Block
  messages follow the hook `Emit` stderr+exit-2 protocol, not the
  error-return convention) is the ADR's own exit-code contract, not a
  divergence. Sound; adhere.
- **Proposed new ADR — panel gate as enforced contract** (spec ADR
  Touchpoints + DQ3): Bead 2 authors it alongside the `panel.json`
  convention it records — schema, `expected_reviewers − 1` threshold,
  reviewed_head_sha staleness rule, dirty-tree rule, env-only/audited
  hatch asymmetry vs MINDSPEC_ALLOW_MAIN, fail-open-when-no-panel,
  registered-panels-only scope, and the trust boundary (every gate
  input incl. the `Enforcement.PanelGate` toggle is agent-writable;
  anti-footgun, not anti-adversary; the env channel is the only
  agent-proof input, HC-7). Domains: workflow, execution.
  **Number resolution**: NOT 0035 (taken: ADR-0035 is on main). As of
  plan-fill, this branch tops at ADR-0035 and the 091 branch
  (spec/091-ownership-discovery) tops at ADR-0034 — its planned
  `ADR-0036-ownership-discovery.md` is not yet created. The ADR lane
  has concurrent in-flight claims, so Bead 2 claims the next free
  integer by re-checking `.mindspec/docs/adr/` on the current branch,
  main, AND the 091 branch at bead-claim time (expected 0036 or 0037),
  then adds the landed ID to this frontmatter. Hand-create the file
  following the existing ADR format; do NOT use `mindspec adr create`
  (two live bugs: mindspec-8lzq worktree mis-write, mindspec-bn3u
  colliding IDs — spec 092's Bead 1 and the 091 plan both
  hit/recorded this).

No accepted ADR is unfit for this work; no superseding ADR is
proposed. ADR-0034 (ceremony collapse, domain: workflow) was evaluated
and not cited: this spec touches no migration/doctor surface and no
phase-derivation semantics; nothing here refines or contradicts it.

## Design Question Resolutions

Spec §Design Questions — draft positions are BINDING for planning
unless the plan-approve panel explicitly overrides (the spec's own
G3-2 preamble). DQ2/DQ5 arrived already resolved; DQ4 arrived settled:

1. **Skill-refresh provenance registry shape (DQ1, Reqs 18-19)**:
   adopt the draft position — a Go slice of historical shipped
   contents (exact byte-match, `githooks.CleanStaleGitHooks`
   discipline, install.go:106; only ~2 generations exist), PLUS a
   `managed-by: mindspec` frontmatter marker written into
   newly-installed skill files so future specs stop growing the slice.
   Encoded in Bead 6.
2. **Complete-side advisory tally (DQ2, Req 13d)**: RESOLVED in-spec
   (gate finding M1) — REQUIRED, not optional. Plan decision: lands in
   Bead 4 with the hook. The `internal/panel` call-in is expected
   <~30 lines (Scan+Tally exist from Bead 2); if Bead 4's implementer
   finds it materially larger, they file a named follow-up bead inside
   this spec at that point and record the split as a deviation —
   either way in-spec, per the spec's text.
3. **Convention home (DQ3)**: new short ADR, per the spec's draft
   position (the staleness rule and hatch asymmetry are exactly what a
   future editor would otherwise "fix"; the doc-sync gate wants a
   stable target). Authored in Bead 2 (see ADR Fitness, number
   procedure included).
4. **Hook-gate v1 scope (DQ4)**: explicit-id completes only — settled
   by the gate panel (G3-8: `cmd/mindspec/complete.go:33` has
   `cobra.MinimumNArgs(1)`, so the bare-complete "exploit" cannot
   exist). Encoded in Bead 4's match rule; bead_id-null panels
   (final-review/PR targets) surface via `--panel-state` only (Bead 5).
5. **`expected_reviewers` flexibility (DQ5)**: RESOLVED in-spec
   (G3-2) — schema carries `expected_reviewers: int`, threshold is
   N − 1, 5-of-6 stays the default-panel example in all message texts,
   no second hardcode of 6. Encoded in Beads 2 (schema/Tally) and 4
   (decision matrix), with the `expected_reviewers: 3` AC fixture.

## Testing Strategy

- **Unit tests are the primary gate**: every behavior change lands
  with unit tests asserting the exact spec AC for that requirement —
  string assertions on guard/hook messages (Beads 1, 4), parser/tally
  shape tests (Bead 2), settings-JSON before/after assertions
  (Bead 3), table-driven decision-matrix tests on `hook.Run` (Bead 4),
  rendered-output assertions for instruct (Bead 5), and content/grep
  assertions on skill files and managed blocks (Beads 6, 7).
  `go build ./... && go test -short ./...` green on every commit, no
  test skipped vs the branch base, boundary lint green (HC-1).
- **Convention test extension (ADR-0035 / HC-5)**: Bead 1 extends
  `conventionFixtures` (internal/guard/recovery_convention_test.go:51)
  to cover every new guard-failure constructor (Reqs 2-4); Req 1 is a
  hook Block message (HC-5 exception) asserted in
  `internal/hook/dispatch_test.go` instead.
- **Zero-bd / zero-cost invariant (HC-3)**: Bead 4 pins the non-match
  path with the `internal/phase` `SetListJSONForTest`/`SetRunBDForTest`
  stubs (pattern at internal/phase/derive_recovery_test.go:48-52)
  wired to FAIL the test if invoked, plus no-config/no-git assertions.
  The accepted per-Bash-call spawn floor (first PreToolUse Bash entry
  ever shipped) gets a harness-suite wall-clock sanity check, recorded
  as Bead 4 verification.
- **Settings-merge regression trio (Reqs 7-8)**: the three spec-named
  tests (merge path, user-entry survival, re-run idempotence + N1
  legacy-instruct removal) land in `internal/setup/claude_test.go` in
  Bead 3 — they are the authoritative gate for the Landmine A/B fixes
  and a hard precondition for Bead 4 shipping the PreToolUse entry.
- **One LLM-harness scenario** (test-channel consolidation per spec
  appendix delta (e)): `panel_gate_blocks_premature_complete`,
  authored in Bead 4, skips under `-short` (HC-1); the other six
  design-named shapes are covered as table-driven `hook.Run` unit ACs.
  Full run lands as a Bead 4 / spec close-out validation proof.
- **Grep-clean as executable proof**: the Req 16 grep over the five
  deleted/merged skill names runs as a test or CI step in Bead 6, not
  a manual check.
- **One-source-of-truth proof (Reqs 12/14)**: Bead 5 asserts the
  "gate would PASS/BLOCK" line in `--panel-state` agrees with a direct
  `mindspec hook pre-complete` invocation on the same fixture — both
  sides call the same `panel.Tally`.
- **No new test frameworks**: existing harness sandbox
  (`internal/harness/sandbox.go`, setup via `setup.RunClaude`
  sandbox.go:75/:445-448 — which is exactly why HC-3/HC-4 are
  load-bearing for the whole suite), existing bd test seams, existing
  setup-test golden patterns.

## Decomposition Notes

Seven beads exceeds the >6 advisory default
(`decomposition-bead-count`); the count is spec-pinned and the
overage is the safety structure, not granularity: the spec's
§Sequencing risks REQUIRES the settings-merge surgery (Bead 3) split
from the hook (Bead 4) — shipping the PreToolUse entry through the
unfixed merge machinery silently strips the gate at install time
(S1-1) and clobbers user hooks (S3-1) — and requires the skill
deletions (Bead 6) gated on their CLI/hook replacements (Beads 1, 4)
so HC-2's zero-knowledge-loss invariant is dependency-enforced, not
conventional. Folding any pair re-couples a hard gate.

Expected `decomposition-chain-depth` warning: longest chain
Bead 2/3 → 4 → 6 → 7, depth 4 vs threshold 3. Accepted: every edge in
that chain is a spec-mandated hard gate (4←3 install-safety; 6←4 and
6←1 HC-2 replacement-before-deletion; 7←6 docs describe the
post-consolidation surface and Req 20 closes ch8h with evidence of
it). Parallelism is 3/7 ≈ 0.43 zero-inbound (Beads 1, 2, 3 start
immediately), above the 0.25 floor — no parallelism warning expected.

File overlap is small and deliberate; a `decomposition-scope-redundancy`
warning, if it fires, is accepted on the same grounds:
`internal/setup/claude.go` is touched by Beads 3 (ownership/staleness
functions), 4 (`wantedHooks` entry), and 6 (`lifecycleSkillFiles`,
managed-block tables, RunClaude refresh) — disjoint regions, and the
dependency edges already serialize all three. `internal/complete/complete.go`
is touched by Beads 1 (Req 2 repair ladder) and 4 (Req 13b/d/e
writes + advisory) — disjoint regions; the orchestrator may run 1
before 4 anyway (both are on 4's ancestor path via 6 only, so if
merge friction appears, serialize 1 → 4 cheaply). Dependency wiring
at plan-approve time is best-effort; the orchestrator verifies all
7 edges post-approve.

Cross-spec sequencing: RESOLVED. The 093 branch forked from main @
1ded99c with ALL Spec-092 beads merged (per spec §Sequencing risks),
including 092 Bead 7 (a5d913d — `internal/next/guard.go` present as
the constructor home/precedent for the Req 3-4 failures) and 092
Bead 9 (657e4c8 — the lifecycle noun-verb rewrite, see Bead 6 Step 5).
No rebase is needed; all anchors in this plan are fork-tree actuals
(ClaimBead caller next.go:218, worktree warning :231, ADR hint
complete.go:303-304).

## Bead 1: CLI point-of-use errors via guard.FormatFailure

**Scope**
Reqs 1-5. Message/UX only; zero behavior change. Lands first so
Bead 6's deletions never orphan knowledge (HC-2). Files:
`internal/hook/dispatch.go` (spec-branch block message, :110-118) +
`dispatch_test.go`; `internal/complete/complete.go` (ADR-divergence
repair ladder, :303-304); `cmd/mindspec/next.go` (claim-failure :218,
worktree-setup-failure :231) + `internal/next/beads.go:159-166`
context; `internal/next/guard.go` (constructor home, 092 Bead 7
precedent); `internal/guard/recovery_convention_test.go` fixture
extensions.

**Steps**
1. Precedent check: 092 Bead 7 (a5d913d) is in the branch base (fork
   of main @ 1ded99c) — use its `internal/next/guard.go`
   (`DirtyTreeFailure` via `guard.NewFailure`) as the home/precedent
   for the new Req 3-4 failure constructors.
2. Req 1: enrich the spec-branch-during-implement Block message with
   the spec's exact legitimacy-context block, PRESERVING the
   conditional "Or switch to your bead worktree: cd <wt>" line
   (dispatch.go:113-115, G1-3) and stating the ACTUAL gate coverage
   per C2-1 (protected any-mode + `spec/` implement-only; `bead/`
   always passes). Protected-branch message keeps its bare hint.
3. Req 2: replace the bypass-first ADR-divergence hint with the
   3-step repair-first ladder (OWNERSHIP.yaml → revert stray edit →
   bypass flags LAST), formatted per HC-5 with `recovery:` final
   lines carrying re-run + bypass commands.
4. Req 3: claim-failure recovery recipe at the `ClaimBead` caller,
   `--claim` line VERBATIM from the spec; final `recovery:` line is
   `mindspec next --spec <slug>`. (1105 auto-fallback stays DEFERRED
   per the spec.)
5. Req 4: replace the bare worktree-setup warning with the
   interpolated `git worktree add` recipe via
   `workspace.BeadWorktreeName`/`BeadBranch`/`SpecWorktreePath`,
   referencing the in-progress auto-recovery path.
6. Req 5: route every new/changed message (Reqs 2-4) through
   `guard.FormatFailure`/`NewFailure`; extend `conventionFixtures`;
   assert the Req 1 hook message in `dispatch_test.go` (HC-5
   exception).

**Verification**
- [ ] `go build ./... && go test -short ./...` and
      `go test ./internal/lint/...` pass (HC-1)
- [ ] `go test ./internal/hook/... ./internal/complete/...
      ./internal/next/... ./internal/guard/...` passes, including the
      extended convention test
- [ ] No emitted message contains raw `bd update --metadata` (HC-5);
      no surviving message claims bead branches are commit-gated (C2-1)

**Acceptance Criteria**
- [ ] Spec AC "Req 1" (legitimacy context + ALLOW_MAIN hint +
      do-NOT-land-feature-code line + no bead-branch claim;
      protected-branch message unchanged save the bare hint)
- [ ] Spec AC "Req 2" (repair-first ladder, OWNERSHIP.yaml named,
      bypass flags last, file names present, final `recovery:` line)
- [ ] Spec AC "Req 3" (verbatim `--claim` recipe + interpolated
      worktree line + `recovery:` referencing `mindspec next --spec`)
- [ ] Spec AC "Req 4" (interpolated `git worktree add` recipe replaces
      the bare warning)

**Depends on**
None

## Bead 2: `internal/panel` package + panel.json convention + contract ADR

**Scope**
Req 6 + the DQ3 ADR. New package `internal/panel`: `panel.json`
schema (incl. `expected_reviewers`, `reviewed_head_sha`, `abandoned`/
`abandon_reason`), `Scan(roots ...string)` (fs-only glob of
`review/*/panel.json`, deduped), `Tally(dir)` (verdict-JSON parse
incl. optional `hard_block`, filename-derived latest round = max(N)
over `*-round-<N>.json`, round-mismatch report, APPROVE tally vs
`expected_reviewers`, staleness inputs, malformed-verdict-as-missing
with names). Foundation for Beads 4 and 5. Plus the new
panel-gate-as-enforced-contract ADR (number per claim-time
procedure — see ADR Fitness; never 0035, re-check 091's branch too).

**Steps**
1. Define the `panel.json` schema types per Req 6 (bead_id nullable;
   round + reviewed_head_sha documented as bumped in the same write
   by /ms-panel-run step 0 — the writer lands in Bead 6).
2. Implement `Scan` (fs-only, no git/bd) and `Tally` per Req 6,
   never trusting `panel.json.round` over filename-derived max.
3. Unit tests for all shapes: lagging-round mismatch, malformed
   verdict counted missing + named, `hard_block` parsed, APPROVE
   tallies for 6/6, 5/6, 4/6 fixtures, and an
   `expected_reviewers: 3` fixture (DQ5 parameterization
   groundwork for Bead 4's threshold AC).
4. Author the contract ADR (domains: workflow, execution) covering
   the spec's ADR Touchpoints bullet in full (threshold rule,
   staleness rule, dirty-tree rule, hatch asymmetry, fail-open rule,
   registered-panels-only scope, trust boundary incl. the config
   toggle, anti-footgun-not-anti-adversary stance); hand-create the
   file; update this plan's frontmatter with the landed ID.

**Verification**
- [ ] `go build ./... && go test -short ./...` and boundary lint pass
- [ ] `go test ./internal/panel/...` passes, all Req 6 AC shapes
- [ ] `Scan`/`Tally` make zero git and zero bd calls (fs-only —
      reviewed by inspection + no executor import)
- [ ] The new ADR file exists with a non-0035 number; its Domain(s)
      line intersects {workflow, execution}

**Acceptance Criteria**
- [ ] Spec AC "Panel package (Req 6)": filename-derived round wins
      over lagging panel.json.round (reported as mismatch); malformed
      verdict = missing + named; hard_block parsed; APPROVE tally
      correct for 6/6, 5/6, 4/6
- [ ] DQ3/spec ADR Touchpoints: contract ADR landed (registered-only
      scope + trust boundary recorded)

**Depends on**
None

## Bead 3: Settings-merge identity redesign

**Scope**
Reqs 7-8 (Landmines A + B; the spec's riskiest-requirement watch
list). Files: `internal/setup/claude.go` (`hookEntryExists` :286-299,
`hookEntryStale` :302-335, `replaceHookEntry` :338-352,
`removeStalePreToolUse` :355-385, `isMindspecHookEntry` :387-400,
`ensureSettings` :185-265, `wantedHooks` :268-284) +
`internal/setup/claude_test.go`. `hookEntryStale` is in scope on the
same grounds as its neighbours: it selects entries by matcher alone
and feeds `replaceHookEntry` — the same matcher-keyed overwrite
machinery Req 7 redesigns (spec Req 7's landmine enumeration).
Merge-machinery surgery with consumer-repo blast radius; Bead 4 is
hard-gated on this bead.

**Steps**
1. Req 7: ownership by command content, never matcher — an entry is
   mindspec-owned iff a hook command prefix-matches `mindspec hook `
   OR contains `mindspec instruct` (RETAIN the instruct arm per N1;
   instruct-form entries are never in the wanted set so wanted-set
   staleness removes them). Rework `hookEntryExists`/`hookEntryStale`/
   `replaceHookEntry` off matcher-keyed identity; `ensureSettings`
   updates only mindspec-owned entries and APPENDS alongside user
   entries sharing a matcher — never replaces or deletes a
   non-mindspec entry (HC-6).
2. Req 8: staleness = mindspec-owned AND not in the current wanted
   set, keep-list DERIVED from `wantedHooks()` (no hardcoded
   whitelist) — kills the merge-then-strip-in-same-pass bug
   (ensureSettings :209-225) before Bead 4 ships the PreToolUse entry.
3. The three regression tests in `claude_test.go`: (i) merge path —
   pre-existing settings.json without mindspec entries → setup →
   pre-complete-shaped entry survives `ensureSettings` (use a
   synthetic wanted PreToolUse entry until Bead 4 ships the real
   one, or assert on the wanted-set mechanics directly); (ii)
   user-entry survival — user PreToolUse Bash entry byte-identical
   after setup; (iii) re-run idempotence — exactly one mindspec
   entry per wanted hook after a second run, legacy
   `mindspec instruct`-form entries still removed (N1 regression).

**Verification**
- [ ] `go build ./... && go test -short ./...` and boundary lint pass
- [ ] `go test ./internal/setup/...` passes incl. the three new
      regression tests
- [ ] Fixture proof: `mindspec setup claude` run twice on a repo with
      a pre-existing user PreToolUse hook — diff shows mindspec
      entries added once, user entry untouched (spec Validation
      Proofs)

**Acceptance Criteria**
- [ ] Spec AC "Merge path" (pre-existing settings.json → entry present
      after ensureSettings; pins the install-time strip)
- [ ] Spec AC "User-entry survival" (both entries present; user's
      byte-identical)
- [ ] Spec AC "Re-run idempotence" (exactly one mindspec entry;
      N1 legacy-instruct removal still works)

**Depends on**
None

## Bead 4: PreToolUse pre-complete panel gate

**Scope**
Reqs 9-13 (the enforced-contract centerpiece; riskiest-requirement
watch list: false outcomes in both directions). Files:
`internal/hook/dispatch.go` (new `pre-complete` case on the lazy
stateFn pattern, :29-36) + `hook.go` (`Names` insert between
"pre-commit" and "session-start") + `hook_test.go` (sorted-order test
:145-150) + new decision-table tests; `internal/setup/claude.go`
(`wantedHooks` PreToolUse entry — lands on Bead 3's fixed machinery);
`internal/config/config.go` (`Enforcement.PanelGate`, default true,
:43-47/:55 precedent); `internal/complete/complete.go`
(`panel_gate_skipped` 13b + advisory tally 13d + `panel_abandoned`
13e, all via `bead.MergeMetadata`); one harness scenario in
`internal/harness`. Consumes Bead 2's `Scan`/`Tally`.

**Steps**
1. Req 9: implement the spec's short-circuit order (pure-stdin parse →
   anchored match → env hatch → root+config → bead-id/scan-root
   resolution (first bd-subprocess point — see step 2) → `panel.Scan`
   → abandoned short-circuit → rev-parse staleness → porcelain dirty
   check → tally+decision). PINNED ordering (gate finding T3-1): the
   Req 12 abandoned→Pass+Warn check runs IMMEDIATELY after the
   panel.json match — it is a pure JSON-field read — and BEFORE the
   rev-parse staleness check, so an abandoned panel whose bead branch
   gained commits after fan-out is never false-Blocked by the stale-SHA
   rule (Req 9 pins false POSITIVES as the bug class; also saves a git
   subprocess on the abandon path). Anchored match rule per S3-6:
   command-position only, tokenizing on unquoted separators; quoted
   mentions never match; false negatives fail open (13d backstop),
   false positives are the pinned bug class.
2. Req 10: scan roots from the COMMAND's target (cd-prefix root +
   bead-id→owning-spec worktree via the bd/phase lookup — explicitly
   NOT `hook.ReadState`/stateFn active-phase resolution, which resolves
   the ACTIVE phase and picks the wrong spec under multi-active-spec
   (gate finding T3-3) — + session-cwd root), union, deduped;
   lookup-failure fallback documented in the hook's doc comment.
3. Req 11: filename-derived round + round-mismatch Block;
   reviewed_head_sha mismatch → Block (lola-f4a8 pin); missing ref →
   Pass + Warn (rerun-after-merge); dirty-tree check filtering
   ADR-0025 artifact paths, Block on user dirt only, worktree-absent
   → skip to tally (NF-1/NF-2 semantics); two-git-subprocess budget.
4. Req 12: parameterized N−1 decision matrix (abandoned → Pass+Warn
   with reason, ordered per step 1; incomplete → Block stating the
   tally vs `expected_reviewers` and enumerating the PRESENT verdict
   files — missing slot NAMES are not derivable from the Req 6 schema,
   which carries only an `expected_reviewers` int (gate finding T3-2);
   REJECT/hard_block → Block halt path; threshold met + SHA + clean →
   Pass; else Block citing threshold + consolidated-round-<N>.md);
   every Block ends with the raw-`git merge` fence (G3-1); existing
   `hook.Emit` protocol unchanged.
5. Req 13: env-only hatch (`MINDSPEC_SKIP_PANEL`, never printed in
   Block output — HC-7), `Enforcement.PanelGate` toggle,
   `panel_gate_skipped` + `panel_abandoned` metadata writes and the
   REQUIRED complete-side advisory tally inside `complete.Run`
   (DQ2: same bead; named follow-up only if the call-in balloons).
6. `wantedHooks()` gains the PreToolUse entry (statusMessage
   "Checking panel verdicts..."); `Names` insert + sorted-order test.
7. Tests: the full hook-gate AC table (below) as table-driven
   `hook.Run` tests; zero-bd stub invariant (HC-3); author + run the
   `panel_gate_blocks_premature_complete` harness scenario (skips
   under `-short`); harness-suite wall-clock sanity check (HC-3
   accepted floor).

**Verification**
- [ ] `go build ./... && go test -short ./...` and boundary lint pass
- [ ] `go test ./internal/hook/... ./internal/setup/...
      ./internal/config/... ./internal/complete/...` passes, full
      decision table green
- [ ] Zero-bd invariant test wired to FAIL on any stub invocation for
      non-matching commands; no config/git/fs work on non-match (HC-3)
- [ ] Harness suite wall-clock before/after sanity check recorded as
      bd evidence (accepted spawn floor)
- [ ] Full LLM run of `panel_gate_blocks_premature_complete`: blocked
      pre-merge, recovered via the documented path (spec Validation
      Proofs; full-run evidence may land at spec close-out)

**Acceptance Criteria**
- [ ] Spec ACs "Hook gate (Reqs 9-13)", all of: zero-bd invariant;
      match table (quoted mentions never match; legit forms match);
      no-panel fail-open (HC-4) incl. BRIEF-only legacy dirs;
      incomplete panel Block states tally vs expected_reviewers and
      enumerates the PRESENT verdict files (T3-2); parameterized
      threshold (5/6 default + `expected_reviewers: 3` fixture, no
      hardcoded 6); dirty tree (user dirt Block / artifact-only Pass /
      clean Pass / worktree-absent skip / no porcelain on no-panel
      path); missing ref Pass+Warn; Block-message fence on every
      variant; REJECT/hard_block Block; round mismatch Block; stale
      SHA Block; cwd independence (cd-prefix + bead-id-derived root);
      escape hatch (Pass+Warn, `panel_gate_skipped` write, unrelated
      keys preserved, no `MINDSPEC_SKIP_PANEL` string in Block
      output); abandoned (Pass+Warn with reason, `panel_abandoned`
      write); complete-side advisory (13d, hookless invocation, no
      cost when unregistered); config toggle; Names sorted
- [ ] Abandoned-before-staleness ordering fixture (T3-1):
      `abandoned: true` + stale `reviewed_head_sha` → Pass + Warn
      (never Block)
- [ ] Spec AC "Harness scenario `panel_gate_blocks_premature_complete`"
      (registered + assertions per spec; skips under `-short`)

**Depends on**
Bead 2, Bead 3

## Bead 5: `instruct --panel-state` + implement-template auto-include

**Scope**
Reqs 14-15. Files: `cmd/mindspec/instruct.go` (flag, :34-37 `init`),
`internal/instruct/run.go` (options on `Run`, :40),
`internal/instruct/instruct.go` (`Context.PanelState`, :33-48),
`internal/instruct/templates/implement.md` (auto-include render),
tests. Consumes Bead 2's `Scan`/`Tally`. Parallel with Beads 3/4.

**Steps**
1. Req 14: `--panel-state` flag → options on `instruct.Run`; output
   block (markdown + `panel_state` object in `RenderJSON`) with the
   three sections: in-progress beads (via `bead.ListJSON` scoped to
   active epics, `phase.Cache` per PERF-1; worktree via
   `resolveBeadWorktree` run.go:223-238; last commit via
   `git log -1 --oneline`; CAPPED at active + 3, deterministic order,
   "… and N more (no git detail)"); open panel rounds via
   `panel.Scan`/`Tally` incl. the "gate would PASS/BLOCK" line
   computed by the SAME `panel.Tally` the hook uses; stale agent
   worktrees (`bead.WorktreeList()` filter + `.claude/worktrees/agent-*`
   dir-scan).
2. Req 15: auto-include at SessionStart — mode == implement AND
   `panel.Scan` finds ≥1 incomplete latest round → `Context.PanelState`
   rendered at the bottom of `templates/implement.md`; the fs-scan is
   the ONLY work outside the auto-include condition (12s SessionStart
   budget, cmd/mindspec/hook.go:130).
3. Tests: block-shape assertions (markdown + JSON), cap test (6
   in-progress → detail for active+3, "… and 2 more"), auto-include
   positive + negative (no panel dir → block absent AND no git/bd
   subprocess attributable to panel-state, stub-guarded).

**Verification**
- [ ] `go build ./... && go test -short ./...` and boundary lint pass
- [ ] `go test ./internal/instruct/...` passes incl. cap + budget tests
- [ ] One-source-of-truth proof: on a fixture review/ dir, the
      "gate would BLOCK" line agrees with a direct
      `mindspec hook pre-complete` invocation (spec Validation Proofs)

**Acceptance Criteria**
- [ ] Spec AC "--panel-state output contains the three blocks; JSON
      carries `panel_state`"
- [ ] Spec AC "Cap test" (active+3 detail, remainder summarized)
- [ ] Spec AC "Auto-include" (implement+incomplete-panel → block in
      SessionStart markdown; no panel → absent + zero added cost)

**Depends on**
Bead 2

## Bead 6: Skills consolidation (16 → 11)

**Scope**
Reqs 16-19 — the big prose diff, dependency-gated so every deleted
sentence has a live replacement (HC-2: Bead 1's CLI errors, Bead 4's
enforced gate). Files: `plugins/mindspec/skills/*` (delete
ms-bead-next, ms-bead-merge, ms-bead-prep→impl, ms-panel-create→run;
tally/fix/autopilot/final-review rewrites; cycle folds),
`internal/setup/claude.go` (`lifecycleSkillFiles` ms-spec-status
deletion :593-602; `RunClaude` create-or-skip → refresh + stale-skill
cleanup :118-135; `claudeMDManagedBlock` :612-657; canonical
noun-verb lines :556/:569/:583 — verify-only, see Step 5),
`internal/setup/codex.go` (`agentsMDManagedBlock` guardrails section,
:138-139), both READMEs, `embed.go` comment,
`internal/setup/*_test.go` goldens.

**Steps**
1. Req 16 dispositions exactly per the spec: ms-bead-next → cycle
   Step 0 (dep cross-check, pick rule, multi-spec disambiguation
   verbatim, 2-line belt-and-braces claim fallback); ms-bead-merge →
   cycle merge terminal (summary-arg convention, `git log --oneline -3`
   verify, partial-failure rule verbatim, the raw-merge fence quoted
   VERBATIM from `plugins/mindspec/skills/ms-bead-merge/SKILL.md:53` —
   the FILE is the verbatim source for the fold; the spec's Req 16
   rendition paraphrases it (gate finding T3-5) — Step-1 checklist
   recorded as superseded-by mapping, ALIGN with location-agnostic
   completion — no cd-then-complete); prep → impl "Phase A";
   panel-create → panel-run "Step 0" WRITING panel.json (Req 6 schema;
   round+SHA bumped in one write on re-panel); tally becomes single
   decision authority (canonical artifact-gates section, halt-recover
   incl. abandon procedure with `abandon_reason` + audit note,
   escape-hatch subsection — the ONLY MINDSPEC_SKIP_PANEL doc, HC-7;
   hook-gated merge row); fix −40%; autopilot −30% (parallel-window
   deleted, serial-by-design line); final-review −25% (panel creation
   rerouted through panel-run step 0 with target=<spec-branch>).
2. Req 16 grep-clean: scrub all listed cross-references; land the
   grep as a test/CI step over the listed surfaces.
3. Req 17: "## Bead-loop guardrails (mindspec)" in
   `agentsMDManagedBlock` (orchestrator rules + subagent prompt
   fences incl. tests-must-PASS and the raw-merge rule); CLAUDE.md
   managed block REFERENCES it; surviving skills carry the pointer,
   not the triplicated blocks.
4. Req 18: delete ms-spec-status (map entry, table row, doc comments,
   README row, test expectations); stale-skill cleanup in setup,
   provenance-gated byte-match (HC-6: user-modified left + notice).
5. Req 19 (narrowed per the transcribed spec): the lifecycle
   canonical noun-verb rewrite is VERIFY-ONLY — it landed pre-fork
   (092 Bead 9, 657e4c8; claude.go:556/:569/:583 already teach
   `mindspec spec approve` / `plan approve` / `impl approve`, pinned
   by `TestLifecycleSkills_CanonicalApproveOrder` at
   claude_test.go:715). Remaining deliverables: (a) EXTEND the
   negative canonical-order assertion from `lifecycleSkillFiles()` to
   the full plugin `skillFiles()` surface (claude.go:522) — no skill
   content anywhere contains the deprecated `approve <noun>` order;
   (b) the provenance-gated in-place refresh via the DQ1
   historical-contents slice + `managed-by: mindspec` marker on new
   installs, so existing installs receive the canonical content;
   ms-impl-approve session-close protocol prose kept (only home).
6. Update sites: managed-block tables, plugin README tables + loop
   diagram, embed.go comment, setup golden tests.

**Verification**
- [ ] `go build ./... && go test -short ./...` and boundary lint pass
- [ ] `go test ./internal/setup/...` passes (goldens updated,
      negative deprecated-order test over `skillFiles()`, refresh +
      cleanup tests)
- [ ] The Req 16 grep-clean command returns empty (test/CI)
- [ ] Post-`mindspec setup` fixture: AGENTS.md contains the guardrails
      section with both subsections; CLAUDE.md references it

**Acceptance Criteria**
- [ ] Spec AC "Grep-clean" (zero live references to the five names)
- [ ] Spec AC "Skill inventory = 11" (tables, diagram, embed comment,
      goldens all reflect it)
- [ ] Spec AC "/ms-panel-run step 0 writes panel.json per Req 6 schema,
      bumps round+SHA in one write; final-review routes through it"
- [ ] Spec AC "/ms-panel-tally canonical sections" (artifact-gates
      3-row table + lola-f4a8, halt-recover + abandon procedure,
      escape-hatch; final-review ≤4-line summary + F5 line)
- [ ] Spec AC "folded cycle merge terminal" (summary arg, log verify,
      verbatim partial-failure rule, no cd-then-complete)
- [ ] Spec AC "autopilot: no parallel-window; serial-by-design line"
- [ ] Spec AC "Req 17" (guardrails section + pointer pattern,
      tests-must-PASS + raw-merge fences)
- [ ] Spec AC "Req 18" (unmodified stale skill removed; user-modified
      left + notice)
- [ ] Spec AC "Req 19" (canonical order verified landed
      (claude_test.go:715); negative deprecated-order test extended to
      `skillFiles()`; old-shipped content refreshed; user-modified
      skipped + notice)
- [ ] Spec AC "C2-1" (no surviving prose claims bead branches are
      commit-gated)

**Depends on**
Bead 1, Bead 4

## Bead 7: Docs closeout

**Scope**
Req 20. Docs-only. Files: `plugins/mindspec/README.md` (+ repo README
skill table if listed), `plugins/mindspec/FINDINGS.md`, bd
(`mindspec-ch8h` closure + the low-pri Workflows-port note).

**Steps**
1. Land the two README paragraphs VERBATIM from the spec's Req 20
   (why-six-reviewers append; second-round sidebar) — the spec text is
   canonical, not the staging-dir design doc.
2. FINDINGS.md adjudications: items 1 (YAGNI + low-pri bd note for the
   Workflows port), 2 (halt-recover = tally section), 8
   (--panel-state shipped), 9, 10 (paragraphs landed); Part 2
   panel-gate status → "enforced".
3. Skill-table refresh in both READMEs + the Anthropic-patterns note
   (ch8h item 1 follow-up).
4. Close `mindspec-ch8h` with evidence.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes (docs-only diff;
      goldens already updated in Bead 6)
- [ ] `grep` finds both rationale headers/paragraph anchors in the
      README; FINDINGS items 1/2/8/9/10 carry adjudication annotations
- [ ] `bd show mindspec-ch8h` shows closed with evidence comment

**Acceptance Criteria**
- [ ] Spec AC "Req 20" (both rationale texts landed; FINDINGS items
      annotated; ch8h closed with evidence)

**Depends on**
Bead 6

## Provenance

Spec acceptance criterion → owning bead + verification:

| Spec AC | Bead | Verified by |
|---|---|---|
| Req 1 (commit-gate legitimacy + C2-1 truth) | Bead 1 | dispatch_test.go message assertions |
| Req 2 (ADR repair ladder) | Bead 1 | forced-divergence test + convention fixture |
| Req 3 (claim-failure recipe, verbatim --claim) | Bead 1 | forced ClaimBead-failure string test |
| Req 4 (worktree-setup recipe) | Bead 1 | forced EnsureWorktree-failure string test |
| Req 5 / HC-5 (convention compliance) | Bead 1 | extended conventionFixtures; dispatch_test for the hook exception |
| panel.Tally shapes (Req 6) | Bead 2 | round/malformed/hard_block/threshold fixture tests |
| Contract ADR (DQ3 / ADR Touchpoints) | Bead 2 | ADR file landed, non-0035 number, frontmatter updated |
| Merge path (Req 7-8) | Bead 3 | regression test (i) |
| User-entry survival (HC-6) | Bead 3 | regression test (ii), byte-identity |
| Re-run idempotence + N1 | Bead 3 | regression test (iii) |
| Zero-bd invariant (HC-3) | Bead 4 | fail-on-invoke stub test |
| Match table (S3-6) | Bead 4 | table-driven quoted/legit-form tests |
| No-panel fail-open (HC-4) | Bead 4 | no-review-dir / no-panel.json / BRIEF-only fixtures |
| Incomplete panel | Bead 4 | 4/6-verdicts Block test enumerating present verdict files (T3-2) |
| Threshold N−1 (DQ5) | Bead 4 | 5/6 Pass, 4/6 Block, expected_reviewers:3 fixture |
| Dirty tree (NF-1/NF-2) | Bead 4 | user-dirt Block / artifact-only Pass / clean Pass / worktree-absent skip / no-call-on-no-panel |
| Missing ref Pass+Warn | Bead 4 | deleted-branch fixture |
| Block-message fence (G3-1) | Bead 4 | single assertion across the decision table |
| REJECT / hard_block | Bead 4 | halt-path Block test |
| Round mismatch | Bead 4 | lagging panel.json.round fixture |
| Stale SHA (lola-f4a8 pin) | Bead 4 | reviewed_head_sha mismatch Block test |
| cwd independence (S3-3) | Bead 4 | worktree-panel/main-cwd fixtures, cd-prefix + bead-id root |
| Escape hatch + audit write (13a/13b, HC-7) | Bead 4 | env Pass+Warn, MergeMetadata diff, no-SKIP-string negative |
| Abandoned + audit write (13e) | Bead 4 | abandoned Pass+Warn with reason + MergeMetadata diff + abandoned-before-staleness fixture (abandoned:true + stale SHA → Pass+Warn, T3-1) |
| Complete-side advisory (13d, required) | Bead 4 | hookless complete.Run tally test + no-cost-unregistered |
| Config toggle (13c) | Bead 4 | enforcement.panel_gate:false Pass-before-scan test |
| Names sorted | Bead 4 | hook_test.go:145-150 sorted-order test |
| Harness scenario | Bead 4 | registered + assertions; full-run evidence at close-out |
| --panel-state three blocks + JSON (Req 14) | Bead 5 | block-shape + RenderJSON tests |
| Cap test | Bead 5 | 6-bead fixture, active+3 detail |
| Auto-include + zero-cost-when-no-panel (Req 15) | Bead 5 | implement-mode render + stub-guarded negative |
| Gate-would-PASS/BLOCK = hook agreement | Beads 4, 5 | shared-Tally fixture proof (spec Validation Proofs) |
| Grep-clean (Req 16) | Bead 6 | grep as test/CI, empty output |
| Inventory = 11 | Bead 6 | tables/diagram/goldens assertions |
| panel-run step 0 panel.json + final-review reroute | Bead 6 | SKILL.md content assertions (schema + one-write bump; no hand-rolled mkdir+BRIEF) |
| Tally canonical sections | Bead 6 | SKILL.md content assertions |
| Cycle merge terminal fold | Bead 6 | content assertions incl. no cd-then-complete |
| Autopilot serial-by-design | Bead 6 | no-parallel-window grep + line assertion |
| Req 17 guardrails | Bead 6 | post-setup AGENTS.md/CLAUDE.md fixture assertions |
| Req 18 stale-skill cleanup | Bead 6 | unmodified-removed / modified-kept+notice tests |
| Req 19 canonical-only + refresh | Bead 6 | skillFiles()-wide negative deprecated-order test + refresh/skip tests |
| C2-1 surviving-prose truth | Beads 1, 6 | message + skill-content negative assertions |
| Req 20 README/FINDINGS/ch8h | Bead 7 | grep anchors + FINDINGS annotations + bd closure evidence |
| build/test/boundary green (HC-1) | all beads | per-bead build/test/lint verification lines |
