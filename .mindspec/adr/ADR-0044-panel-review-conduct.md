# ADR-0044: Panel Review Conduct — Portable Reviewer-Conduct Doctrine for Every Lifecycle Gate

- **Date**: 2026-07-24
- **Status**: Accepted
- **Domain(s)**: workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (gate MECHANICS — this ADR is reviewer CONDUCT, the skill-layer judgment the gate mechanism does not encode), [ADR-0040](ADR-0040-orchestration-layering-ratchet.md) (conduct is L4 skill-layer doctrine; the binary contributes mechanism only), [ADR-0043](ADR-0043-panel-disposition-telemetry-store.md) (the findings-never-out-voted doctrine this ADR makes reviewer-facing)

---

## Context

MindSpec's panel gate is mechanically strict: canonical verdict enum, the
any-unresolved-REQUEST_CHANGES Block, the audited refutation procedure
(ADR-0037 §7). But the gate only adjudicates verdicts once they exist — it
cannot encode HOW a reviewer arrives at a finding, how the orchestrator
verifies one, or where a reviewer's scratch files land. That judgment lived
only in this project's operator memory, and field evidence showed the cost
of not shipping it:

- **spec-116 Bead3a**: mutation/empirical reviewers probing the SHARED bead
  worktree contaminated each other — one slot deleted an escape to probe it
  while another grepped the same file — producing 3 false REQUEST_CHANGES.
- **spec-119 beads 4/5/6**: three false codex "suite is red in clean
  checkout" claims, all refuted by reproducing in a clean detached checkout,
  plus 3 wrong "hollow fixture" BLOCKINGs — while the shipped cycle skill
  carried only the dangerous half ("trust the empirical check").
- **spec-121 Bead-2**: a summarised BRIEF silently dropped an AC clause; the
  panel reviewed the summary, not the AC, and the gap sailed unanimous
  approval — while `ms-bead-impl` mandates verbatim quoting for impl
  prompts and `ms-panel-run` said the opposite for the BRIEF ("Don't paste
  the plan; summarise it.").
- **2026-07-08 run**: reviewer relative-path scratch writes plus harness
  cwd-resets corrupted SIBLING worktrees; the gate's dirty-tree Block caught
  the strays only after the mess existed.
- **spec-119 + PR #217**: local-green ≠ CI-green; no shipped skill told any
  reviewer to reproduce the project's declared CI invocation.

Spec 126 promotes this discipline into the shipped product. Per ADR-0040,
it ships as skill/ADR text, not binary enforcement. This ADR is the single
normative home; the shipped skills carry only role-local operational
fragments plus a citation back here. It lands Accepted, not Proposed: it
codifies already-practiced doctrine, and the divergence lane errors on
Proposed-only citations.

## Decision

The following conduct rules are the normative reviewer-conduct doctrine for
every mindspec panel gate. Skills cite this ADR; they do not restate it.

1. **Reviewer mutation-isolation.** Any reviewer that EXECUTES or MUTATES
   code — runs tests, deletes an escape to probe it, edits a fixture —
   does so in its OWN isolated detached checkout at the reviewed SHA
   (`git worktree add --detach` or `git archive`), never in the shared bead
   worktree; read-only inspection of the shared tree remains fine. The
   orchestrator holds the matching half: before acting on a CONTESTED
   empirical finding, re-verify it yourself in a fresh isolated checkout;
   the reproduction (or its failure) is the refutation evidence.
   (Provenance: spec-116 Bead3a.)

2. **Verify-never-confirm.** An empirical finding from ANY reviewer —
   especially a non-interactive CLI reviewer — is a VERIFY, never a
   CONFIRMED. Every blocking finding, test-execution and code-reasoning
   alike, is reproduced in isolation before fix-dispatch or refutation.
   Never out-vote by count in either direction; weight the reviewer that
   did the deeper code trace. When a reviewer family's safety classifier
   refuses a security-framed lens, re-dispatch the SAME slot
   correctness-framed. (Provenance: spec-119 false-red / hollow-fixture
   over-flags — and, symmetrically, CLI reviewers catching real bugs the
   interactive reviewers missed.)

3. **Findings-never-out-voted, reviewer-facing.** Every finding is fixed or
   evidence-refuted via the audited refutation procedure — never dropped
   because the APPROVE count cleared the threshold. The threshold is a
   floor, not a license; a "minor" finding is still adjudicated. This is
   the reviewer-facing statement of the ADR-0043 doctrine; the binary
   already enforces the any-unresolved-RC Block.

4. **Absolute scratch + clean worktree.** Every reviewer/fixer scratch file
   lives at an ABSOLUTE path (under `/tmp`, or the panel directory for
   verdicts), never a relative one — harness cwd-resets turn relative
   writes into sibling-worktree corruption. Verify the bead worktree is
   porcelain-clean before every `mindspec complete`; the mechanized
   dirty-tree Block is the backstop, not the plan.

5. **CI-parity kernel.** A final-review reviewer reproduces the PROJECT's
   declared CI invocation — `commands.ci`, falling back to `commands.test`
   — verbatim on the spec branch, recording an explicit advisory when
   neither is declared. Local-green ≠ CI-green. The rule is abstract; the
   value is per-project config; no mindspec-specific invocation ships in
   any skill.

**Portability principle.** This doctrine is PORTABLE — true for any project
on the mindspec lifecycle. Mindspec-self-development specifics (`go test
-short`, bd-off-PATH choreography, harness/instruct pre-existing-RED lore,
the argv-ratchet, specific model values) are explicitly NOT part of the
conduct and never ship in consumer skills. Incident IDs and provenance live
in this ADR only, never in shipped skill text — skills describe each
failure MODE portably.

## Consequences

- The five shipped skills — `ms-panel-run`, `ms-panel-tally`,
  `ms-bead-cycle`, `ms-bead-fix`, `ms-spec-final-review` — carry role-local
  operational fragments plus a citation to this ADR; no normative text is
  duplicated across them, and none carries an incident ID.
- Binary enforcement is unchanged: the gate, tally, and decision matrix are
  untouched — mechanism only, per ADR-0040. Mechanizing the spec/plan-gate
  panels (gate-before-mutate on the approve verbs) remains a separate
  future decision, tracked as `mindspec-0pij`.
- A fresh `mindspec setup <agent>` install carries the same panel
  discipline the origin project practices; the conduct no longer lives
  only in operator memory.
