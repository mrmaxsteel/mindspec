# spec-110-bead1 — Round 1 (bead panel, 8 reviewers, four families)

**Worktree (read here)**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.1
**Branch**: bead/mindspec-fbel.1
**Commit under review**: 17a2ed28406990b05b82a20b3684fe0a06123246 — "feat(panel): leaf-safe Create registration writer + panel schema doc" (sole commit on top of plan-approve b5711d21)
**Panel**: 8 reviewers — O1–O3 Opus, S1–S3 Sonnet 5, F1 Fable, G1 GPT-5.5 (codex). Pass = **>=7 APPROVE, no REJECT**.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; builds/tests must leave `git status` clean in the bead worktree.

## What the work does

Bead 1 of spec 110 (plan: `.mindspec/specs/110-panel-verbs-parser-parity/plan.md` § Bead 1 — judge the diff against it; the approved spec.md beside it is the contract). Adds `internal/panel/create.go`: a leaf-safe `Create(dir, CreateInput)` that writes `panel.json` (round + reviewed_head_sha in one struct, one write) and splices a machine-managed delimited header region into BRIEF.md atomically — computing everything in memory first so corrupt marker states (open-without-close, duplicated pairs) error without touching either file; legacy no-marker BRIEFs get the header prepended with the body kept byte-identical (incl. CRLF); verdict files `*-round-N.json` are never touched. Plus the R4 schema doc in `.mindspec/domains/workflow/interfaces.md` (panel.json filename, `<slot>-round-<N>.json` N>=1, consolidated name, verdict PAYLOAD contract: verdict enum + top-level `hard_block`) pinned by `TestPanelSchemaDoc_MatchesConstants`, which extracts the doc's own backtick-quoted examples against `panel.FileName`/`verdictFileRE`/`ConsolidatedName`.

## Files in scope (661 insertions, 3 files)

- `internal/panel/create.go` (new, 211 lines)
- `internal/panel/create_test.go` (new, 377 lines)
- `.mindspec/domains/workflow/interfaces.md` (+73)

## Slot lenses

| Slot | Family | Lens |
|:-----|:-------|:-----|
| O1 | Opus | Author-of-record — does the diff deliver plan §Bead 1's Steps 1–5 exactly (incl. its Acceptance Criteria)? Anything skipped, added, or reinterpreted? |
| O2 | Opus | Codebase-pin — do the named tests exist and pass as claimed? Run the full Verification checklist yourself in the bead worktree. |
| O3 | Opus | Contract stability — `Create`/`CreateInput` as the surface Bead 4's CLI will call: signature sane, plain-values-only honored, error semantics usable by a cobra RunE? |
| S1 | Sonnet | Empirical prober — write scratch programs under /tmp that call `panel.Create` against fixture dirs: re-panel co-bump, pre-seeded verdicts untouched, corrupt-marker rejection actually leaves both files unmodified (check mtimes/bytes). |
| S2 | Sonnet | Schema/type correctness — `Panel` struct marshaling (`bead_id` null when nil, no field ever omitted that must not be), JSON round-trip fidelity, error paths, marker-parse edge code. |
| S3 | Sonnet | Next-bead integration — will Bead 4 (`panel create` CLI) and Bead 5 (skill stub-filling) consume this cleanly? Is the BRIEF stub's "Your job" verdict-contract block consistent with what ms-panel-run/B5 will need? |
| F1 | Fable | Adversarial — attack the atomicity/byte-preservation/leaf-safety claims: find an input where panel.json and BRIEF diverge, where body bytes are altered, where a verdict file could be touched, or where the schema-doc test would stay green with a wrong doc. |
| G1 | GPT-5.5 | Second empirical prober + robustness — marker corruption variants (nested, whitespace-mangled, marker text inside code fences in the body), CRLF/CR-only, huge BRIEFs, and payload-schema doc consistency with `internal/panel/tally.go`'s actual parsing. |

## Your job

Evaluate the commit cold through your lens. Verdict: APPROVE / REQUEST_CHANGES / REJECT.
Output JSON to `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.mindspec/specs/110-panel-verbs-parser-parity/reviews/spec-110-bead1/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence`, `rationale` (<=200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
