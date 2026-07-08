# 0uur-headless-grill — round 1 consolidated changes required

**Tally: 3 APPROVE (R1/R2/R3, Opus) / 3 REQUEST_CHANGES (R4/R5/R6, Sonnet) / 0 REJECT — below 5/6 → FIX ROUND.**
Reviewed SHA: 24af2fdb. The plumbing (snapshots byte-exact, HC-6, mirrors, inventory) is unanimously CLEAN — do not touch the refresh mechanism except as required by the canonical-text changes below. The defect is the guard's SEMANTICS.

## The convergent core defect (R4 empirical, R5, R6 — HIGH)

The guard's deferral marker is an unchecked `- [ ]` in Open Questions; `checkOpenQuestions` (internal/validate/spec.go:226) hard-ERRORs on it and `ApproveSpec` aborts — R4 unit-probed the exact marker text (severity=ERROR). And because the guard SKIPS the grill entirely, the spec template's own pre-existing `- [ ]` placeholder is never cleared either (under PR #155's workaround the grill still ran and its contract cleared it). Net: a fully headless spec-create→approve flow — the bead's target scenario, and what `ScenarioSpecToIdle` asserts SUCCEEDS — now hard-fails at approve, undocumented and untested. This contradicts the bead's goal ("auto-defer with a recorded default, rather than blocking").

## Group 1 — Replace the binary guard with a three-mode disposition (must fix; both skills' canonical text + all mirrors)

1. **Interactive** (a human can answer): unchanged one-at-a-time grill.
2. **Instructed non-interactive** (an explicit instruction to proceed without asking exists — e.g. harness prompts, batch evaluation, an orchestrator/autopilot run that says to proceed): **SELF-ANSWER mode** — run the grill's full analysis; for each question, adopt the best repo-grounded default, apply the resulting spec fix, and record it as a RESOLVED (checked) line in Open Questions (`- [x] grill (self-answered, headless): <question> → <default taken>`); also fulfill the grill's existing contract of resolving or explicitly deferring every pre-existing Open Question (incl. the template placeholder). Approve then passes. This restores the PR #155 behavior as the product path and literally satisfies the bead's "recorded default". It also makes the bench-eval routing explicit (the eval's "do not ask questions; non-interactive batch evaluation" instruction lands squarely in this mode → findings, never a deferral).
3. **Bare headless** (no human AND no explicit instruction): defer with the existing UNCHECKED marker — the fail-safe backstop (R1's praised property, kept deliberately: approve blocks until a human or a documented resolution).

## Group 2 — Documented resolution path for the backstop marker (R6 — must fix)

Add a short clause to `ms-spec-approve`'s canonical SKILL.md (and mirrors): if the spec contains the `grill deferred: headless session` marker, either run `/ms-spec-grill` interactively and resolve it, or — in an orchestrated run with a passed spec review panel — resolve it by checking the box with a citation of the panel. Keep it to a few lines.

## Group 3 — Trigger-wording tightening (R5 — must fix)

Remove "invoked by an orchestrator" as an illustrative trigger (in this repo "orchestrator" also names interactive top-level sessions). Anchor the mode test on: is a human available to answer one-at-a-time questions? If not, is there an explicit instruction to proceed non-interactively? Keep the two guard texts' criteria and the marker string byte-consistent across create/grill and all mirrors.

## Group 4 — Tests (R4/R5 — must fix)

(a) A validate-level pin: `checkOpenQuestions` ERRORs on the exact unchecked deferral-marker line (backstop stays blocking by design). (b) A text-consistency pin: the marker string and the self-answer contract sentence appear identically in the canonical create text (`lifecycleSkillFiles`) and the grill SKILL.md (guards against drift the way R5 checked by hand). (c) Existing refresh tests updated only as needed for the new canonical bodies; `.pre0uur.md` snapshots must remain byte-identical to the d1b4bebf bodies (they pin the pre-guard→guarded refresh) — do NOT regenerate them.

## Group 5 — Consistency sweep (R6 medium — check, minimal edits)

Check PR #155's harness prompt clause and internal/harness/HISTORY.md notes for contradiction with self-answer mode (they should now agree — same behavior, now canonical). Adjust the harness prompt clause ONLY if it conflicts; do not rewrite history records.

## Deferred (NOT this round)

- Full `bench/grill` eval run (R4 recommendation): R4's live single-shot probe already produced a full finding set under the current wording, and Group 1 makes eval routing explicit/safer. Run the full eval post-merge if paranoia warrants; note in the PR body.
- Any binary/Go changes to validate/approve behavior — the backstop SHOULD block; that's design, not defect.
