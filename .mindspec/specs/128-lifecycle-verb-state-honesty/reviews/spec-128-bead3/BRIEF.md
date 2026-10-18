<!-- mindspec:panel-header -->
## Panel Registration

- **Slug**: `spec-128-bead3`
- **Round**: 4
- **Branch**: `bead/mindspec-ta30.3`
- **Reviewed commit**: `f6847729edbe6312daa564a2e7f9b84f7a69ed10`

## Your job

Review the diff at the reviewed commit above and write your verdict as
a single JSON file named `<your-slot>-round-4.json` beside this BRIEF
(the panel artifact schema — see `.mindspec/domains/workflow/interfaces.md`
§ Panel Artifact Schema). Required and optional top-level fields:

- `verdict` (required): one of `APPROVE`, `REQUEST_CHANGES`, `REJECT`.
- `hard_block` (optional boolean, a top-level sibling of `verdict` — never
  a per-finding field): set `true` only for an evidence-bearing halt that
  no vote count may override.
- `reviewer_id`, `confidence`, `rationale`, `concrete_changes_required`,
  `findings`: reviewer-authored context, read presentation-only by
  `mindspec panel tally` — not consumed by the gate decision.
<!-- /mindspec:panel-header -->

## Summary

**Bead implementation review** (gate: bead). Under review: commit `7b6d83f1` on `bead/mindspec-ta30.3` — spec 128 Bead 3: draft-mode derivation + `state show` honesty (OQ-2, R1.1/R1.2/R1.4). Delivered: `ResolveModeWithCache` probes on-disk pre-epic drafts (SpecDir + spec/<id> branch) before the idle fallback and PROPAGATES discovery errors (direct AllEpics health check) instead of collapsing to idle; new `DraftSpec`/`DiscoverDraftSpecs(WithCache)`; state.go rebuilt on ONE `phase.Cache` per invocation (R3.5) with the no-active-spec path refusing on bd failure (ADR-0035, `mindspec doctor` recovery), rendering a single draft as `{"mode":"spec",...}`, listing multiple drafts via ErrAmbiguousTarget, and bare idle ONLY when genuinely zero drafts; the narrow ADR-0023 draft-discovery amendment (the :107/:108 strike is bead 4's, untouched); AC-1's three hermetic legs red-on-revert against the 2026-09-01 captures. Judge against spec AC-1 + R1.1/R1.2/R1.4 + the OQ-2 resolution and plan §Bead 3. Staged prompt: `reviews/prep/bead3_impl_prompt.md`.

**Implementer-flagged judgment call (assess explicitly — A)**: R1.2's "one authoritative derivation" is satisfied by sourcing BOTH Mode and ActiveBead from the same resolved specID/epicID via one Cache (ResolveModeWithCache + FindEpicBySpecIDWithCache + FindActiveBeadForEpicWithCache) rather than the literal ctx.Phase substitution a reading of the plan prescribes — the implementer's argument: ResolveContextFromDirWithCache derives ambiguity purely from CWD (WorktreeMain scans ALL active specs, ignores an explicit --spec), so the literal substitution would regress `state show --spec <one-of-several>`. Judge on the merits: does this satisfy R1.2's letter ("one derivation authoritative, the other removed or asserted-consistent") or does it leave TWO derivations (ResolveMode's and ResolveContext's) alive with divergence risk on paths this bead didn't touch (instruct tails still use ResolveContext — bead 5's territory)?

Plus one side effect to verify: two new entries in the workspace-composition call-site allowlist (draftSpecStatusWithCache's SpecDir/SpecBranch uses) — house-pattern conformance.

**Schema note (GH #254)**: header citation DANGLING; the header's field list is authoritative; verdict exactly APPROVE / REQUEST_CHANGES / REJECT.

## Files in Scope

`git show 7b6d83f1` in the bead worktree. Key: internal/resolve/resolve.go + draft_test.go, cmd/mindspec/state.go + state_show_honesty_test.go, .mindspec/adr/ADR-0023.md (amendment only), internal/lint/ratchet_composition_allowlist_test.go.

## Acceptance Criteria (verbatim from spec.md)

AC-1's three legs (spec-worktree draft → spec-not-idle; main-checkout discovery-failure → non-zero refusal, never {"mode":""} exit 0; main-checkout single draft → spec-not-bare-idle). Read in place.

## Prior-Round Asks

Rounds 1-3: 1A/5RC+HB → 5A/1RC (HB cleared) → 3A/3RC (the round-2 tag-shadow fix closed only the PROBE layer; three slots' end-to-end repros found the ENUMERATION layer — %(refname:short) ambiguity — still hides a branch-only draft). Round-3 fix landed as ONE commit `f6847729` — the commit under review: new `gitutil.LocalBranchNames` sibling helper on `%(refname)` full-refname enumeration with deterministic TrimPrefix (LocalBranchRefs left in place — Executor-surface consumers — only the draft-discovery caller switched, disclosed); the state-show-level ambient fixture (branch-only draft + same-named shadow tag → draft still rendered), red-on-revert verified; direct LocalBranchNames unit test; one argv-ratchet entry. Round-4 duty (final confirm): re-run the END-TO-END shadow-tag repro at f6847729 (it must now discover the draft with the tag present), run the two new tests, confirm the remainder unchanged, re-verdict.
## Lens

| Slot | Family | Lens |
|:-----|:-------|:-----|
| R1 | Claude | **Author-of-record** — plan §Bead 3 steps delivered and only them; the ADR-0023 amendment genuinely NARROW (no general spec-artifact-authority leak); deviation A judged on the merits; bead-2 surfaces undisturbed. |
| R2 | Claude | **Codebase-pin** — the 7+3 new tests real, hermetic (t.TempDir, seams, no PATH games), and red-on-revert against the three captures; the composition-allowlist entries house-pattern; suite green from the bead worktree. |
| R3 | Claude | **Contract stability** — deviation A's divergence risk: enumerate every OTHER consumer of ResolveContextFromDir's Phase (instruct tails, spec.go, complete tails — bead 5's targets) and judge whether this bead's split leaves a disagreement window bead 5 must close or an actual contradiction NOW; the draft-probe's interaction with bead 2's semantics (a draft has no epic — does any v2/legacy path misfire on drafts?). |
| R4 | Codex | **Empirical prober** — build/test in the bead worktree; reproduce AC-1's three legs via the new tests AND a scratch-fixture hand-probe (fake repo: draft dir + branch, no epic → state show output; kill the bd seam → refusal shape + exit code; two drafts → ambiguity listing); verify {"mode":""} exit 0 is DEAD on every path you can construct. |
| R5 | Codex | **Schema/correctness** — the refusal's ADR-0035 shape (final recovery line, doctor command); the draft-probe's branch check (BranchExistsIn — worktree-correct?); ErrAmbiguousTarget rendering; the ADR-0023 amendment text vs the plan's OQ-2 wording (narrow draft-discovery exception, nothing more). |
| R6 | Codex | **Adversary** — hunt: any path where a failed/partial read still fabricates a mode (the beads-1/2 hard-block class — e.g. draft-probe IO errors, branch-check errors, half-created drafts (dir without branch / branch without dir), specs dir unreadable); race between draft discovery and concurrent spec approve creating the epic; --spec flag vs multi-draft interplay. |

## Reviewer conduct (binding)

Build/test only in the bead worktree (/Users/Max/replit/mindspec/.worktrees/worktree-spec-128-lifecycle-verb-state-honesty/.worktrees/worktree-mindspec-ta30.3); leave byte-identical; fixture roots in /tmp; no commits/lifecycle verbs/tracked-file edits; verify-never-confirm; contradiction licensed; write your verdict JSON to the exact path in your prompt BEFORE terminating.
