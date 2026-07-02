# spec-111-approve — Round 1 Panel (spec-approve gate)

**Worktree**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner | **Commit**: 2448b8abbcc7fe1a9ad1bf90e3627160c8b47fa5 (drafted; grill closed with ZERO findings — first of the series)
**Target**: the SPEC (.mindspec/docs/specs/111-workflow-panel-runner/spec.md). Series capstone: the first orchestration runner adapter behind 110's contracts, selected by 109's runner: key.

## What the spec does

A Claude Code dynamic workflow (.claude/workflows/ms-panel.js + embedded plugins/mindspec/workflows/ copy, distributed via a new WorkflowFiles() embed accessor mirroring SkillFiles() + claude-target install) that runs the 6-reviewer panel deterministically: registers via mindspec panel create; fans out claude slots as workflow agents and codex slots as WRAPPER agents (agent execs codex CLI, parses stdout, writes the verdict itself — codex never writes files, eliminating the sandbox failure class); quota substitution = config-driven branch (109's substitution.claude_sub_on_quota; slot kept, reviewer_id claude-sub); verdict validity enforced by prompt + panel verify's per-slot parse-status (NOT a platform schema — deliberately portable); re-rounds via the artifact contract (create --round N+1), not session-scoped workflow resume; finishes with panel verify + tally output as the workflow result; NEVER runs complete/consolidation (decision authority unchanged). Skills slim to lens-composition + one Workflow invocation on runner=claude-code-workflow (skills path retained as default). New .claude/workflows/** OWNERSHIP claim lands same-diff-or-earlier (108 pattern). Prereqs: 109+110 merged + rebase-forward before plan approve. Sole impacted domain: workflow.

## Your job (round 1 — superseded)

Is this SPEC ready for approval? Verdict → JSON to review/spec-111-approve/<slot>-round-1.json (relative to worktree root; keys: reviewer_id, verdict, confidence, rationale <=200w, concrete_changes_required, findings; optional hard_block).

---

# ROUND 2 ADDENDUM

**Commit under review**: bd898dfadfd1559939317d500c95927100af23d3 — `docs(spec-111): apply round-1 panel changes (REJECT remediation)` (spec.md only, +28/−14)
**Prior round verdict**: 5 APPROVE, 1 REJECT (R3 claude — gate-integrity attack). No hard_blocks. R4–R6 ran as claude-subs (codex quota-walled); round 2 restores real codex in those slots, same slot ids.

## Round-1 concrete_changes_required (consolidated)

1. **[R3-a] Codex transcription audit trail.** The wrapper agent transcribing codex stdout into the verdict file is a new unauditable fidelity surface (skills path has codex write its own file). Require codex raw stdout persisted as a tracked audit artifact alongside the verdict, or explicitly place transcription fidelity outside the trust model.
2. **[R3-b] Separate parse-failure recovery from quota substitution (HIGHEST severity).** "Re-prompts once or substitutes (R4) on a parse failure" conflated a rendered-but-malformed verdict with a no-verdict quota wall. A re-prompt must preserve the first verdict's semantics (re-serialize, never re-decide); persistent failure must fail CLOSED to a MISSING slot (gate Blocks), NEVER a claude-sub — else a rendered codex REJECT in bad JSON can be laundered into a fresh APPROVE. Substitution reserved exclusively for the no-verdict quota case. Falsified clauses must forbid the laundering path.
3. **[R3-c] `sha?` arg semantics.** State that the workflow's `sha?` arg is advisory-only and never sets `panel.json.reviewed_head_sha` (panel create self-resolves from `--target`), or drop it.
4. **[R1] Research-doc citation.** Background quoted §3.2/§3.3 of `project-docs/research/loop-engineering-adaptation.md`, a file absent from the repo and all git history. Commit it or decouple the spec from it.
5. **[R5-1] Parse-failure branch unfalsified.** No AC or Manual proof exercised the malformed-verdict (non-quota) recovery. Add a Manual-proof bullet driving it end-to-end; clarify slot identity on re-prompt.
6. **[R5-2] AC4 grep comment-defeat.** `! grep -q 'mindspec complete'` is defeated by a descriptive comment; positive greps satisfiable by comments. Anchor to call-shaped patterns or require the literal string appear nowhere.
7. **[R5-3] AC8 pins only 2 of 4 tally judgment sections.** Add greps for 'After a halt' and 'Escape hatch'.
8. **[R5-4] "Verbatim" vs "contains".** R5/ADR-0035 claim verify/tally output is carried verbatim, but the Manual proof only checked containment. Align prose and proof.

## What the remediation did (assess against the asks)

- **R3b (new requirement + AC + Scope):** every codex wrapper persists raw stdout to `<spec-dir>/reviews/<slug>/<slot>-round-<N>.codex.log` (wrapper writes it, never codex); framed as extending ADR-0037 §8 "plain reviewable files" to the transcription step. New AC greps `.codex.log` in the workflow file. → asks 1.
- **R3 anti-laundering ladder (rewritten R3 + R4):** three numbered steps — (1) parse failure on a *rendered* verdict → re-prompt the SAME reviewer exactly once, feeding back its own output, re-serialize without re-reviewing, same slot AND family reviewer_id; (2) still unparseable → fail CLOSED to MISSING (no file) → incomplete → gate Blocks, never substituted; (3) substitution reserved exclusively for a quota wall with *no* rendered verdict, and only when `claude_sub_on_quota == true`. R3's and R4's Falsified clauses now both name the laundering path ("no path exists by which a rendered REJECT/REQUEST_CHANGES can be replaced by a different reviewer's verdict or a re-reviewed APPROVE"). Open Questions rewritten to match. → asks 2, 5 (identity half).
- **R1 `sha?` advisory-only:** stated in R1 with a falsified clause ("if `sha?` rather than `panel create` sets the recorded `reviewed_head_sha`"). → ask 3.
- **Background citation decoupled:** the working paper is named as an untracked internal document, quotes/section numbers dropped, the two rationales restated in the spec's own words. → ask 4.
- **Manual proof expanded to four named behaviors:** verdict+audit files; parse-failure re-prompt (same reviewer_id, same family, never claude-sub; still-bad → NO verdict file → verify incomplete → Block, NOT substituted); quota substitution both flag values; result verbatim. → asks 5, 8.
- **AC4 → ALLOWED_CLI allowlist (R5 + AC):** the workflow declares an explicit `ALLOWED_CLI` set of exactly four commands (`mindspec panel create`, `codex exec`, `mindspec panel verify`, `mindspec panel tally`); `mindspec complete` must appear NOWHERE in the file, not even as a comment (the R5-offered "literal string nowhere" option, strengthened structurally). AC rewritten with `grep -qF` per command + `grep -q 'ALLOWED_CLI'` + `! grep -qF 'mindspec complete'`. → ask 6.
- **AC8 pins all four tally sections** ('After a halt', 'Escape hatch' greps added). → ask 7.
- **R5 "verbatim" made binding:** result carries verify/tally CLI stdout "verbatim (unmodified CLI stdout, not re-rendered or paraphrased)", with a falsified clause on altering it; Manual proof checks verbatim. → ask 8.

## Fix-author deviations (assess explicitly)

A. **New requirement numbered `3b`** rather than renumbering 4–9 — keeps cross-references (R4/R5/R6…) stable across rounds at the cost of non-sequential numbering.
B. **Ask 6 resolved via the stricter option**: instead of call-shaped grep anchoring, the spec bans the literal `mindspec complete` string from the file entirely AND adds the `ALLOWED_CLI` allowlist (a structural constraint the ask didn't demand). Positive greps remain satisfiable by the allowlist declaration itself — by design, since the allowlist IS the structural artifact.
C. **Ask 4 resolved by decoupling, not committing**: the research doc stays untracked; the spec now carries its rationale self-contained.

## Your job (round 2)

Each reviewer: mark EACH of your own round-1 asks ADDRESSED / PARTIAL / MISSED / NEW_ISSUE, assess the three deviations, and re-attack your lens against the remediated spec at bd898dfa. R3 in particular: try to break the anti-laundering ladder again. Verdict → JSON to review/spec-111-approve/<slot>-round-2.json (same slot filenames as round 1: r1|r2|r3|codex-r4|codex-r5|codex-r6). Keys: reviewer_id, verdict, confidence, rationale <=200w, concrete_changes_required (empty if APPROVE), findings (per your round-1 items); optional hard_block.
