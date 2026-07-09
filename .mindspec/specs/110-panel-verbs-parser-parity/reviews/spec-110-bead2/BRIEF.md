# spec-110-bead2 — Round 1 (bead panel, 8 reviewers, four families)

**Worktree (read here)**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.worktrees/worktree-mindspec-fbel.2
**Branch**: bead/mindspec-fbel.2
**Commit under review**: 62cd6df3 — "feat(validate): fold spec-approve parser parity into ValidateSpec (R5 Impacted-Domains resolution + R6 ADR-Touchpoint existence)" (sole impl commit; 3 files, +344)
**Panel**: 8 reviewers — O1–O3 Opus, S1–S3 Sonnet 5, F1 Fable, G1 GPT-5.5 (codex). Pass = **>=7 APPROVE, no REJECT**.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; leave `git status` clean.

## What the work does

Bead 2 of spec 110 (plan §Bead 2 in `.mindspec/specs/110-panel-verbs-parser-parity/plan.md`; spec.md beside it): folds two parser-parity checks into `ValidateSpec` so `mindspec spec approve` (which hard-fails on `vr.HasFailures()`, approve/spec.go:47-50, unchanged) inherits them. R5: Impacted-Domains resolution via the IDENTICAL `normalizeImpactedDomains(nil, root, "", impacted)` call plan-approve makes — same `impacted-domains-resolve` code, same SevError severity, nothing plan-approve tolerates newly rejected. R6: ADR-Touchpoint existence for ANCHORED links only, via the WIDENED filename-form regex `\[(ADR-\d{4})[^\]]*\]\([^)]+\)` (panel round-1 fix — covers the `[ADR-0031-doc-sync-gate.md](…)` convention of specs 085–094), resolving against the same store as plan.go:156, emitting `adr-touchpoint-missing` with a recovery hint, existence-only (NO adr-coverage-*/adr-cite-irrelevant at spec-approve — R7c). Tests pin SEVERITY (SevError/HasFailures), not just codes; the self-check runs a BRANCH-BUILT binary.

## Files in scope

- `internal/validate/spec.go` (+84), `internal/validate/spec_test.go` (+239)
- `.mindspec/domains/workflow/architecture.md` (+21)

## Slot lenses

| Slot | Family | Lens |
|:-----|:-------|:-----|
| O1 | Opus | Author-of-record — diff delivers plan §Bead 2 Steps 1–5 + ACs exactly. |
| O2 | Opus | Codebase-pin — run the full Bead-2 Verification checklist yourself (incl. the branch-built self-check `go build -o /tmp/… && … validate spec 110-…`, `go test ./internal/approve`). |
| O3 | Opus | Contract stability — ApproveSpec inherits with zero internal/approve change; the new issue codes/severities as a stable surface; no change to plan-approve's own checks (shared-helper regression). |
| S1 | Sonnet | Empirical prober — fixture specs under /tmp: path-like zero/multi-owner entries fail with SevError; bare-name-no-manifest passes; anchored-missing fails (BOTH bare `[ADR-9999]` and filename-form `[ADR-9999-foo.md]` anchors); bare-prose mentions pass; a 110-shaped spec passes. |
| S2 | Sonnet | Severity/typing — the SevError pins are real (would an AddWarning impl fail these tests?), issue-code hygiene, regex correctness (probe `[^\]]*` edge cases: nested brackets, empty tail, multiline). |
| S3 | Sonnet | Integration/self-consistency — the repo's OWN specs still validate (run the branch binary against several merged specs incl. 085–094 and 110 itself); plan-approve behavior byte-identical (its tests untouched+green); doc-sync region correct. |
| F1 | Fable | Adversarial — attack "parity, not stricter": construct a spec plan-approve tolerates today that the new ValidateSpec rejects (or a spec it should reject but passes — e.g. regex blind spots: reference-style links, angle-bracket URLs, `[ADR-12345]` five digits); attack the existence-only boundary (any coverage/relevance diagnostic leakage?). |
| G1 | GPT-5.5 | Second empirical prober — build the branch binary to /tmp; run `validate spec` against EVERY spec dir in `.mindspec/specs/` and diff pass/fail against the installed 109 binary (the parity regression oracle); hostile touchpoint fixtures (control bytes in link text, enormous sections). |

## Your job

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<your-slot>-round-1.json` in this dir (`/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity/.mindspec/specs/110-panel-verbs-parser-parity/reviews/spec-110-bead2/`). Keys: `reviewer_id`, `verdict`, `confidence`, `rationale` (<=200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
