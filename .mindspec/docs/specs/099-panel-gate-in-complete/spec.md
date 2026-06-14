---
approved_at: "2026-06-14T19:11:57Z"
approved_by: user
status: Approved
---
# Spec 099-panel-gate-in-complete: Enforce the panel gate inside mindspec complete (declared bead ID); retire the shell-parsing matcher (wpf0)

## Goal

Relocate the ADR-0037 panel-gate enforcement from the shell-command-parsing
PreToolUse hook to INSIDE `mindspec complete <id>` itself, where the bead ID
arrives as a DECLARED argument. The in-binary check becomes the AUTHORITATIVE
gate; because it reads the bead ID from the command's own parsed argument
(never from a free-form shell string), EVERY invocation form — wrapped,
quoted, aliased, novel — is covered with no heuristic matcher, no
wrapper-stripping, and no fail-open residual. The PreToolUse hook is retained
as a defense-in-depth backstop for the Claude-Code-routed common case, and
the heuristic matcher is left in place (now non-authoritative); its eventual
retirement is a deferred follow-up, not this spec's scope.

## Background

ADR-0037 turned "the panel must approve before merge" into a machine gate: a
PreToolUse hook on `mindspec complete` (`internal/hook/precomplete.go`) reads
panel state from the filesystem (panel.json + the verdict JSONs + the live
`reviewed_head_sha`) and blocks a premature merge. That hook must first DETECT
that a free-form Bash command invokes `mindspec complete` and then EXTRACT the
bead ID from the command string — both done heuristically by
`matchMindspecComplete` / `strippedCompleteFields` / `completeBeadID`, which
tokenize the raw command, strip env-assignments + `cd`/`pushd` + the catchable
`env`/`timeout`/`command`/`xargs` wrappers, and match a terminal
`mindspec complete`.

Two structural problems motivate wpf0:

- **ZFC violation (ADR-0036).** Detecting intent by parsing a free-form shell
  string is exactly the framework-cognition pattern ADR-0036 rejects: there is
  no declared signal at the hook boundary, so the matcher GUESSES whether a
  command is a complete. ADR-0037 §6's gate semantics are sound; the heuristic
  that feeds them is not.
- **Structural fail-open residual (ADR-0037 §6 acceptance + 7eup).** A
  non-executing tokenizer cannot reach a command wrapped inside a quoted
  string — `sh -c '… complete'`, `eval '… complete'` — so those forms are an
  explicitly-ACCEPTED residual. Worse, any unhandled wrapper (`nice`, `nohup`,
  `sudo`, or a novel form) silently slips the gate. Spec 098 R4 (mindspec-7eup)
  extended the matcher to catch more wrappers and fixed `completeBeadID` to
  actually extract the id — but that ADDS heuristic surface rather than
  removing it. Each new wrapper is another arms-race patch against a
  fundamentally fail-open design.

The ZFC-clean fix is to enforce the gate INSIDE `mindspec complete` at the
start of its own execution, where the bead ID is a declared `cobra` argument.
`complete.Run` already KNOWS exactly which bead it is completing and already
runs an advisory tally (`panelAdvisory`, step 2.25) over the same
`internal/panel` `Scan`/`Tally`/`ForBead` FACT READER the hook uses — but that
advisory reuses only the vote-only `panel.VoteDecision` and does NOT apply the
hook's full ADR-0037 decision matrix or gather its staleness/dirty facts. The
change is to promote that site into an AUTHORITATIVE block that honors the full
ADR-0037 §§2-6 contract — threshold (N−1), staleness (`reviewed_head_sha` vs
the live ref), dirty-tree, round-mismatch, abandonment, and the
fail-open-without-a-panel / fail-closed-with-one asymmetry — by running the
SAME extracted `panelGateDecision` + fact-gathering the hook runs (Req 3), and
BLOCKS via the ADR-0035 recovery-line protocol when unmet. No shell parsing, no
wrapper-stripping, no quoted-form blind spot.

**Dogfooding implication (load-bearing).** After this lands and the binary is
rebuilt, the orchestrator's OWN `mindspec complete` calls hit the in-binary
gate. ADR-0037 §6's fail-open-without-a-panel rule is preserved verbatim: a
complete for a bead with NO `panel.json` referencing it passes silently. The
test harness, solo flows, and any bead completed without a registered panel
remain structurally unaffected — this spec does NOT block panel-less completes.
The gate fails CLOSED only once a panel is registered (the same commitment
semantics the hook already enforces).

**Hook reconciliation + matcher decision (this spec's posture).** The hook is
KEPT as a defense-in-depth backstop (belt-and-suspenders) for the
Claude-Code-routed case, but the in-binary check is AUTHORITATIVE — it covers
every invocation path, including the codex/raw-shell/aliased flows the hook
never sees. Removing the hook in the same spec would briefly regress
Claude-Code coverage if a rebuild lagged, and the hook's env-only escape hatch
(`MINDSPEC_SKIP_PANEL`, ADR-0037 §7) has spawn-topology properties the
in-binary path must re-derive carefully; doing both at once couples two risky
changes. The heuristic matcher (`strippedCompleteFields` /
`segmentInvokesComplete` / `completeBeadID`) is therefore LEFT INTACT while the
hook stays — shrinking it now would break the backstop it still feeds. Its
retirement (and the hook's, once the in-binary gate has soaked) is recorded as
a deferred follow-up bead so the heuristic surface is removed only when nothing
depends on it.

## Impacted Domains

- workflow: owns `internal/complete/**`, `internal/hook/**`, `internal/panel/**`, and `internal/guard/**` (per `.mindspec/docs/domains/workflow/OWNERSHIP.yaml`). Carries every requirement in this spec — the in-binary gate is added to `internal/complete`, invokes the ADR-0037 decision + fact-gathering EXTRACTED into the `internal/panel` leaf package (Req 3) shared with the hook, and reports blocks through `internal/guard`'s recovery-line formatter.
  * Req 1 (in-binary panel-gate enforcement) touches `internal/complete/complete.go` (promote the step-2.25 advisory into an authoritative pre-merge gate) and `internal/complete/panel_advisory.go` (the shared `internal/panel` scan/tally seams already in this package).
  * Req 1 reads panel state via `internal/panel` (`Scan`/`Tally`/`ForBead`, `Panel.ApproveThreshold`) — the SAME FACT READER the hook calls. The ADR-0037 DECISION MATRIX is NOT in `internal/panel` today: it lives in `internal/hook` (`panelGateDecision` + the `gateFacts` struct + the staleness/dirty fact-gathering, `precomplete.go`). Req 3 extracts that shared decision so the in-binary verdict can never disagree with the hook's — see Req 3 for the explicit extraction.
  * Req 1 reports a block via `internal/guard` (`guard.NewFailure` / the `recovery:` convention) so the in-binary block obeys the ADR-0035 contract identically to the hook's stderr+exit-2 Emit.
  * Req 3 (extract the shared decision) RELOCATES `panelGateDecision` + the `gateFacts` struct + the fact-gathering (`resolvePanelFacts`-equivalent) from `internal/hook/precomplete.go` into the `internal/panel` leaf package, and REWIRES `internal/hook` + `internal/complete` to both invoke that one source — no second matrix copy.
  * Req 4 (hook backstop + matcher-retirement deferral) leaves the heuristic matcher (`strippedCompleteFields` / `segmentInvokesComplete` / `completeBeadID`) in `internal/hook/precomplete.go` behaviorally unchanged and records the deferred-follow-up posture — no removal in this spec.
- execution: owns `internal/executor/**` (per `.mindspec/docs/domains/execution/OWNERSHIP.yaml`). Carries the ADR-0030 boundary fix from Req 3's extraction — the in-binary gate (an enforcement-package caller in `internal/complete`) routes its git facts through the executor rather than shelling out directly.
  * `internal/executor/**` gains thin git-I/O boundary methods (`RevParseRef`/`Status`/`IsRefNotFound`) so the in-binary gate (an enforcement-package caller in `internal/complete`) routes git facts through the executor per ADR-0030 — owned by the execution domain.

Note: every source path this spec edits is OWNED by a DECLARED impacted domain on current `main`, so no ownership-claim bead is required and the `adr-divergence-unowned` gate will not hard-block them: `internal/complete/**`, `internal/hook/**`, `internal/panel/**`, `internal/guard/**` are owned by the **workflow** domain, and `internal/executor/**` (the new `RevParseRef`/`Status`/`IsRefNotFound` git-I/O boundary methods) is owned by the **execution** domain.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): the panel-gate enforced contract this spec relocates. Status **Accepted**; Domain(s) **workflow, execution** — intersects BOTH impacted domains, **workflow** and **execution** (so it covers the executor's new git-I/O boundary methods). The in-binary check MUST honor the SAME contract the hook does: registration (§1, panel.json identity), round derivation from filenames (§2), the N−1 threshold (§3, single home in `panel.ApproveThreshold`), staleness via `reviewed_head_sha` vs the live ref (§4, block-not-warn), the dirty-tree rule (§5), the fail-open-without-a-panel / fail-closed-with-one asymmetry (§6), and the escape hatches + trust boundary (§§7-8). This spec moves the ENFORCEMENT POINT from the hook to the command without changing the contract; an amendment note records that the in-binary path is now authoritative and the hook is a backstop.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): the recovery-line + exit-code contract. Status **Accepted**; Domain(s) **workflow, execution, core** — intersects BOTH impacted domains, **workflow** and **execution**. The in-binary block MUST route through `internal/guard` (`guard.NewFailure`) so it ends with a `recovery:` line and is a PRE-terminal gate failure that exits non-zero having mutated NOTHING (HC-4 exit-code contract) — placed BEFORE the `bd close` / merge in `complete.Run`. It carries the same raw-`git merge` fence the hook block carries.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md): Zero Framework Cognition. Status **Accepted**; Domain(s) **workflow, validation, doc-sync, ownership** — intersects the impacted **workflow** domain. This spec is the direct application of ZFC to the gate's INPUT path: it eliminates the heuristic that guesses "is this a complete and which bead?" from a free-form string, replacing it with the bead ID DECLARED on `complete`'s own argument — no framework cognition, no inferred signal.

## Requirements

1. **In-binary panel-gate enforcement (authoritative).** Enforce the full
   ADR-0037 contract using the declared bead ID, at the PRE-MUTATION point — at
   the existing step-2.25 advisory site (`complete.go` ~:254, where
   `panelAdvisory` runs today) and CRITICALLY **before step 2.5
   `exec.CommitAll` (`complete.go` ~:273)** and before `bd close` / the
   bead→spec merge. This ordering is load-bearing: `CommitAll` advances the
   `bead/<id>` branch tip PAST `reviewed_head_sha` and clears the user-authored
   dirt, so a gate that ran AFTER `CommitAll` would FALSE-FIRE both the §4
   staleness clause (live ref no longer equals the reviewed SHA) and the §5
   dirty-tree clause (the dirt is gone). The gate's staleness rev-parse target
   is the `bead/<id>` branch ref (`workspace.BeadBranch`), evaluated BEFORE
   `CommitAll`, identical to the hook's `gateFacts.headSHA` source. The check:
   - read the registered panel(s) for the bead via the existing
     `internal/panel` reader (`Scan`/`ForBead`/`Tally`), the same one the hook
     and the step-2.25 advisory already use;
   - apply the §3 threshold (N−1, via `panel.ApproveThreshold`), the §4
     staleness check (`reviewed_head_sha` vs the live bead-branch ref — BLOCK,
     never warn), the §2 round-derivation/round-mismatch rule, the §5
     dirty-tree rule (user-authored dirt only, ADR-0025 artifact paths
     filtered), and the §6 fail-open-without-a-panel / fail-closed-with-one
     asymmetry;
   - honor the §7 escape hatches — the env-only `MINDSPEC_SKIP_PANEL` skip
     (read via `os.Getenv`, NEVER named in a block message per HC-7), audited
     abandonment, and the `enforcement.panel_gate` config toggle;
   - on an unmet gate, BLOCK via `internal/guard` (`guard.NewFailure`) with the
     ADR-0035 recovery line and the raw-`git merge` fence, exiting non-zero
     having mutated nothing.
   Because the bead ID is a declared argument, the gate covers every
   invocation form with no shell parsing.

2. **Fail-open-without-a-panel preserved (dogfooding safety).** A
   `mindspec complete <id>` for a bead with NO `panel.json` referencing it MUST
   pass silently (ADR-0037 §6). The in-binary gate adds ZERO new block for
   panel-less completes — the harness, solo, and ordinary no-panel flows are
   structurally unaffected after the rebuild. A test pins that a panel-less
   complete ACTUALLY COMPLETES the bead — closed + merged + exit 0 — not merely
   that it is "not blocked" (a no-op that exits 0 without merging would falsely
   pass a weaker assertion).

3. **Extract the shared decision + fact-gathering (no decision drift).**
   Today only the FACT READER (`internal/panel`: `Scan`/`ForBead`/`Tally`/
   `ApproveThreshold`/`VoteDecision`) is shared between the hook and complete's
   existing advisory. The ADR-0037 DECISION MATRIX is NOT shared: the ordered
   §§2-6 short-circuit (`panelGateDecision`: hatch → no-panel → malformed →
   abandoned → round-mismatch → missing-ref → stale-SHA → dirty → incomplete →
   reject → threshold), its `gateFacts` struct, and its I/O fact-gathering
   (`resolvePanelFacts`: `git rev-parse bead/<id>`, porcelain dirty check,
   scan-root union) all live HOOK-LOCAL in `internal/hook/precomplete.go`
   (`panelGateDecision`/`gateFacts` ~:51-241, fact-gathering ~:683-882).
   `complete`'s existing `panelAdvisory` reuses ONLY the vote-only
   `panel.VoteDecision` — it does NOT call `panelGateDecision` and does NOT
   gather the staleness/dirty/round/missing-ref facts. So the decision and its
   facts must be EXTRACTED, not merely "called", before the in-binary path can
   share them. This is EXPLICIT SCOPED WORK, not a deferred plan-time choice.

   The requirement: the PreToolUse hook's gate path AND the in-binary gate path
   MUST invoke the IDENTICAL `panelGateDecision` function over an IDENTICAL
   `gateFacts` value produced by the IDENTICAL fact-gathering — one source, not
   two copies. This single-source IS the structural guarantee against
   decision-drift (the whole point of this requirement). Two mechanisms are
   viable; this spec picks **(a)**:
   - **(a) PREFERRED — move into `internal/panel`.** Relocate `panelGateDecision`
     + `gateFacts` + the fact-gathering (`resolvePanelFacts`-equivalent: the
     bead-ref rev-parse, porcelain dirty check, ADR-0025 artifact filter,
     scan-root union) into `internal/panel`, the dependency-clean LEAF package
     (it imports NO other internal package). Both `internal/hook` and
     `internal/complete` then import the one decision + the one fact-gatherer.
     This keeps the matrix in the package that already owns the fact reader and
     adds no new import edge.
   - **(b) ALTERNATIVE — export from `internal/hook`.** Export
     `panelGateDecision` + `resolvePanelFacts` from `internal/hook` and have
     `internal/complete` call them. The `internal/complete → internal/hook`
     import edge ALREADY EXISTS (`panel_advisory.go` imports `hook` for
     `hook.SkipPanelEnv`) and `internal/hook` does NOT import
     `internal/complete`, so there is NO import cycle. Rejected only because it
     leaves the decision matrix in the consumer-heavy `hook` package rather than
     the leaf; either is mechanically sound.

   The chosen mechanism (a) keeps the ADR-0037 decision matrix and its facts in
   the leaf package both call sites depend on, so neither call site can hold a
   private copy of the short-circuit ORDER (e.g. abandoned-before-staleness,
   missing-ref pass-through) — eliminating drift by construction.

   **Out of convergence (acknowledged):** `instruct --panel-state`'s
   `PanelStateEntry.verdict` (`internal/instruct/panelstate.go`) is a THIRD,
   ADVISORY-ONLY decision copy with its own ordering and NO dirty-tree/gitErr
   branch. This spec does NOT converge it: it INFORMS, it does not ENFORCE, so
   drift there mis-informs but cannot mis-gate. Converging it is intentionally
   OUT OF SCOPE here (a candidate for the same follow-up that lands this
   extraction).

4. **Keep the hook as a backstop; defer matcher retirement.** The PreToolUse
   hook (`internal/hook/precomplete.go`) and its heuristic matcher
   (`strippedCompleteFields` / `segmentInvokesComplete` / `completeBeadID`)
   are LEFT IN PLACE and behaviorally UNCHANGED by this spec — the hook remains
   a defense-in-depth backstop for the Claude-Code-routed case. The
   in-binary gate is authoritative; the hook is redundant-but-harmless. A
   follow-up bead is filed (and noted in this spec's Open Questions resolution
   / the bead tracker) to retire the hook + shrink the heuristic matcher once
   the in-binary gate has soaked, so the heuristic surface is removed only when
   nothing depends on it.

5. **Block parity with the hook.** The in-binary block messages MUST be
   actionable to the SAME standard as the hook's: name the panel/round, the
   present/missing verdicts or the stale SHA pair, end with the ADR-0035
   `recovery:` line, and carry the raw-`git merge bead/<id>` fence — so an
   agent blocked at the binary recovers identically to one blocked at the hook.

## Scope

### In Scope
- `internal/complete/complete.go` — add the authoritative pre-mutation panel
  gate (promoting the step-2.25 advisory) before `bd close` / merge.
- `internal/complete/panel_advisory.go` — the panel scan/tally seams reused by
  the gate; the call site that invokes the EXTRACTED `panelGateDecision` +
  fact-gathering (Req 3) instead of the vote-only `VoteDecision`.
- `internal/panel/**` — read-only reuse of the existing `Scan`/`Tally`/
  `ForBead`/`ApproveThreshold` reader, PLUS the new home of the extracted
  ADR-0037 decision: `panelGateDecision` + the `gateFacts` struct + the
  fact-gathering (`resolvePanelFacts`-equivalent — bead-ref rev-parse,
  porcelain dirty check, ADR-0025 artifact filter, scan-root union) relocated
  here from `internal/hook` (Req 3 mechanism (a)). No panel.json schema change.
- `internal/guard/**` — the block-message formatter the in-binary gate reports
  through (`guard.NewFailure`, recovery line).
- `internal/hook/precomplete.go` — REWIRED to call the extracted
  `panelGateDecision` + fact-gathering from `internal/panel` (so the hook and
  the in-binary gate share one source), plus a documentation/comment amendment
  recording that the in-binary gate is now authoritative and the hook is a
  backstop. The matcher heuristics are unchanged.

### Out of Scope
- Removing the PreToolUse hook or shrinking/retiring the heuristic matcher
  (`strippedCompleteFields` / `segmentInvokesComplete` / `completeBeadID`) —
  deferred to a follow-up bead.
- Any change to the panel.json schema, the verdict-file convention, the round
  derivation, the threshold value, or the escape-hatch semantics — this spec
  relocates the ENFORCEMENT POINT, not the contract.
- Non-bead (PR/final-review) panel enforcement — out of v1 hook scope per
  ADR-0037 §1 and unchanged here.

## Non-Goals

- This spec does not strengthen the trust boundary (ADR-0037 §8): the in-binary
  inputs are the same agent-writable repo artifacts, and the goal stays
  anti-footgun, not anti-adversary. No signing, hashing, or out-of-repo store.
- This spec does not change WHAT blocks (the decision matrix) — only WHERE the
  decision is enforced (the command, not a shell-string heuristic).

## Acceptance Criteria

- [ ] `mindspec complete <id>` for a bead with a registered panel whose tally is BELOW threshold (or stale, or has missing/malformed verdicts, or a REJECT/hard_block) BLOCKS before any `bd close` / merge, exits non-zero, mutates nothing, and prints a `recovery:` line plus the raw-`git merge` fence — for EVERY invocation form (bare, wrapped, quoted, aliased), with no shell parsing involved.
- [ ] The in-binary gate runs at the step-2.25 advisory site BEFORE step-2.5 `exec.CommitAll` (and before `bd close` / merge): a test confirms a stale-SHA / dirty-tree block fires on the pre-`CommitAll` `bead/<id>` tip and is NOT spuriously caused (or cleared) by `CommitAll` having already advanced the tip past `reviewed_head_sha`.
- [ ] `mindspec complete <id>` for a bead with NO `panel.json` referencing it ACTUALLY COMPLETES — bead closed + merged + exit 0 — not merely "not blocked"; the fail-open-without-a-panel rule is preserved; a unit/harness test pins the actual completion.
- [ ] A registered SUB-THRESHOLD `panel.json` for bead X does NOT block `mindspec complete <Y>` for a DIFFERENT bead Y (different-bead isolation via `panel.ForBead`, which matches only `panel.BeadID == beadID`); a test pins it.
- [ ] `mindspec complete <id>` for a bead whose registered panel meets the N−1 threshold AND is fresh (`reviewed_head_sha` matches the live `bead/<id>` ref) AND has a clean tree proceeds to close+merge as before; a freshness test (panel reviewed-HEAD == live bead ref → pass) proves the in-binary HEAD source is the same `bead/<id>` ref the hook rev-parses.
- [ ] The env-only `MINDSPEC_SKIP_PANEL=1` skip, audited abandonment, and the `enforcement.panel_gate: false` toggle all behave identically to the hook from inside `complete`; the block message never names the skip variable (HC-7).
- [ ] The in-binary gate and the hook invoke the IDENTICAL extracted `panelGateDecision` function over the IDENTICAL `gateFacts` produced by the IDENTICAL fact-gathering (one source, per Req 3) — NOT just the same `internal/panel` reader; a test pins that both call sites reach the same decision so they cannot diverge.
- [ ] The PreToolUse hook and its heuristic matcher remain present and behaviorally unchanged (the hook now calls the extracted decision but its matcher + block behavior are identical); a follow-up bead to retire them is recorded.
- [ ] `mindspec validate spec 099-panel-gate-in-complete` passes from the worktree.

## Validation Proofs

- `mindspec validate spec 099-panel-gate-in-complete`: passes (Impacted Domains owned, ADR touchpoints Accepted + intersecting, no unresolved Open Questions).
- `go test ./internal/complete/... ./internal/hook/... ./internal/panel/...`: green, including new in-binary-gate cases (sub-threshold blocks, stale blocks, dirty-tree blocks, panel-less pass-through, different-bead isolation, freshness pass, skip/abandon/toggle hatches).
- A panel-less `mindspec complete` in the test harness ACTUALLY COMPLETES a bead — closed + merged + exit 0 (dogfooding-safety regression; not a silent no-op).
- A pre-`CommitAll` ordering test: a stale-SHA / dirty-tree block is evaluated against the `bead/<id>` tip as it stands BEFORE step-2.5 `exec.CommitAll`, proving the gate site precedes the auto-commit that would otherwise false-fire/false-clear those clauses.
- A different-bead-isolation test: a sub-threshold `panel.json` registered for bead X does not block `complete` of bead Y.
- A freshness test: when the panel's `reviewed_head_sha` equals the live `bead/<id>` ref, the in-binary gate PASSES — confirming the in-binary HEAD source matches the hook's `gateFacts.headSHA`.
- A shared-decision test: hook and in-binary call sites resolve to the same `panelGateDecision` over the same `gateFacts` (single source, no second matrix copy).

## Open Questions

_All resolved during drafting; recorded here for the plan author._

- Hook retirement vs. backstop: RESOLVED — keep the hook as a defense-in-depth
  backstop and make the in-binary check authoritative; retiring the hook +
  shrinking the heuristic matcher is a DEFERRED follow-up bead (Req 4), so the
  heuristic surface is removed only once nothing depends on it.
- Decision-logic sharing: RESOLVED — only the FACT READER (`internal/panel`)
  is shared today; the ADR-0037 DECISION matrix (`panelGateDecision` + the
  `gateFacts` struct + the staleness/dirty fact-gathering) is currently
  HOOK-LOCAL in `internal/hook`. Req 3 makes the extraction EXPLICIT SCOPED
  WORK (not a deferred choice): it relocates `panelGateDecision` + `gateFacts`
  + the fact-gathering into the `internal/panel` leaf package (mechanism (a)),
  so the hook AND the in-binary gate import and invoke the IDENTICAL decision +
  facts — the structural single-source against drift. The alternative
  (export from `internal/hook`; the `complete → hook` edge already exists, no
  cycle) is recorded and rejected in favor of the leaf-package home.
- `instruct --panel-state` convergence: RESOLVED (out of scope) — the
  `PanelStateEntry.verdict` is a THIRD, advisory-only decision copy that
  INFORMS but does not ENFORCE; this spec does not converge it (drift there
  mis-informs, never mis-gates). Converging it is deferred to the same
  follow-up that lands the Req 3 extraction.
- Dogfooding safety: RESOLVED — the fail-open-without-a-panel rule (ADR-0037
  §6) is preserved verbatim, so panel-less completes (harness, solo) are NOT
  blocked (Req 2 + its pinned test).

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-14
- **Notes**: Approved via mindspec approve spec