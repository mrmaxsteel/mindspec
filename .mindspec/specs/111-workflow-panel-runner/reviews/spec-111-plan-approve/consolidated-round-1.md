# spec-111-plan-approve — consolidated round-1 changes

Tally: 4 APPROVE (F2 0.88, O2, O3 0.90, G3 0.86) / 5 REQUEST_CHANGES (F1 0.85, F3 0.82, O1 0.70, G1 0.91, G2 0.84) / 0 REJECT. Threshold 8/9 not met → fix round. The DAG, coverage, carry-forward folding, process conformance, and downstream contract all APPROVED — every ask targets Bead 2 (the runner artifact): the self-contradictory snippet, enforcement-vs-declaration, deterministic codex handling, e2e provability, and one false platform claim. All plan-text amendments.

13 raw asks deduped to 10 items.

## The self-contradiction + fence strength (Bead 2)

1. **(G1.1 + F1.1 — two families independently) — must.** The plan's own ALLOWED_CLI snippet (plan.md ~lines 364–370) contains the literal string `mindspec complete` inside its comment, so implementing the snippet verbatim FAILS the plan's own whole-file absence greps (~lines 481/484). Rewrite the snippet/comment so it satisfies the checks (e.g. "the merge-terminal verb" — never the literal string).
2. **(F1.2) Positive enumeration replaces blocklist greps — must.** The anti-indirection greps (`mindspec ${`, backtick template, `"mindspec " +`) miss single-quoted concat and every non-`complete` lifecycle verb. Replace with the stronger invariant: extract EVERY occurrence of `mindspec` in the file and assert each is one of the exact-four allowlisted argv forms — a positive enumeration test, not a blocklist.
3. **(G2.1) Enforcement, not declaration — must.** `TestMsPanelWorkflow_AllowedCLIExactSet` proves the constant exists; nothing ties agent steps to it. Specify a single command-builder/wrapper through which every shell-capable step constructs its command from fixed argv prefixes (no free-form prompted shell strings), rejecting any verb outside the set; add a structural test that all command construction routes through it (composes with item 2's positive enumeration).
4. **(F1.3) Pin the codex sandbox — must.** Allowlisted `codex exec` runs default workspace-write — shell+write access no grep sees. Specify the read-only sandbox flag/config for reviewer codex invocations (write access ONLY to the verdict path per the established panel convention), and make the positive-enumeration test cover the `codex` argv form too.

## Determinism + hardening (Bead 2)

5. **(G2.2) Input/path hardening — must.** Validate slug/spec/target/bead_id/round/slot against the same contracts as 110's CLI (single clean path element etc.); derive verdict/log write paths from `panel create`'s reported layout, never from raw args; reject traversal and shell-metacharacter-bearing values at workflow entry.
6. **(G2.3 + G1 overlap) Deterministic codex stdout parsing — must.** Accept exactly ONE unambiguous verdict JSON object; multiple objects or mixed narrative → fail CLOSED to the reserialize-then-MISSING path; the value-fidelity comparison decodes both sides canonically and compares ALL gate-relevant fields (verdict, hard_block, concrete_changes_required), not string equality.

## Provability (Bead 2 Manual e2e)

7. **(G1.2) Branch-built binary — must.** The workflow's bare `mindspec panel …` calls must resolve to the branch build: specify `go build -o /tmp/ms111/mindspec ./cmd/mindspec` and invoking the e2e with `PATH=/tmp/ms111:$PATH` (or equivalent explicit resolution). As written the e2e can green against the installed binary.
8. **(G1.3 + F3.2 — convergent) Deterministic failure-branch induction — must.** A live codex can't be forced to emit malformed JSON or hit a quota wall on cue, so R3/R4's behavioral proofs are unexercisable prose. Specify a codex PATH-shim test double (a `codex` script earlier in PATH that emits scripted malformed/wall outputs per scenario) + the exact `/ms-panel` invocation args for each scenario (slug/spec/target/round/mix).

## Small but load-bearing

9. **(F3.1) Word-boundary the handoff grep — must.** Bead 3's `grep -q '/ms-panel'` passes TODAY via the pre-existing `/ms-panel-tally` / `/ms-panel-create` substrings. Anchor it (e.g. `grep -Eq '/ms-panel([^-a-z]|$)'` or match the exact invocation form) so it can fail if the handoff line is missing.
10. **(O1.1 + O1.2 + O1.3) Correct the false platform claim and reconcile — must.** The cited workflow docs DO document a `schema` option (validated structured output) on agent steps — the plan's three claims that "the docs describe none" (~141–142, ~406, ~645–646) are false. Fix: (a) correct the claims; (b) state the REAL reason `mindspec panel verify` remains required — `schema` governs the agent's RETURN VALUE, not the on-disk `<slot>-round-<N>.json` files 110's artifact contract requires — and direct each reviewer step to use `schema` for its return AND write the same shape to the verdict file; (c) name the concrete fan-out primitive from the docs the plan cites (`pipeline()` over slots, or an explicit sequence of `agent()` calls) instead of an unnamed/undocumented parallel primitive. Verify wording against the docs the plan cites (code.claude.com/docs/en/workflows) — do not re-assert from memory.

## Constraints for the fix author

- Plan-text only; ONE commit `docs(spec-111): apply round-1 plan-panel changes`. Do not renumber beads or change the DAG/citations (approved by F2/O2/O3/G3).
- Keep every bead ≤7 steps — fold into existing steps/Verification items.
- The four spec-approval carry-forwards (already folded) must remain intact — O3 verified them; don't regress them while editing.
- `~/.local/bin/mindspec validate plan 111-workflow-panel-runner` must pass (advisory WARN acceptable).
- Update Provenance rows whose verification anchors change.
