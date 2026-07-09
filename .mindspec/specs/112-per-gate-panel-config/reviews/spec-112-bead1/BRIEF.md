# spec-112-bead1 — Round 1 (bead panel, 8 reviewers, four families)

**Worktree (read here)**: /Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.worktrees/worktree-mindspec-lma4.1
**Branch**: bead/mindspec-lma4.1
**Commit under review**: 68fe576e34c25dfe4677a1e03d31cf9744cbfa64 — "feat(config): per-gate panel.gates schema, generalized reviewers, gate-scoped resolvers" (sole commit on top of plan-approve c338b2ba; 5 files, +1291/−32)
**Panel**: 8 reviewers — O1–O3 Opus, S1–S3 Sonnet 5, F1 Fable, G1 GPT-5.5 (codex). Pass = **>=7 APPROVE, no REJECT**.
**READ-ONLY RULE**: verdict JSON only; scratch under /tmp; pin reads to the SHA; builds/tests leave `git status` clean.

## What the work does

Bead 1 of spec 112 (plan: `.mindspec/specs/112-per-gate-panel-config/plan.md` § Bead 1 — judge the diff against it; `spec.md` beside it is the contract, R1–R5 as mapped). In `internal/config`: `Reviewer` generalized (`Model`, `Lens`, pointerized `Count *int` + exported `CountValue()`); `GatePanel`/`Panel.Gates` per-gate map + `Panel.Note`; `Substitution.Substitutes` one-step map; exported ordered `PanelGateKeys`; curated `KnownModels()`; three gate-scoped resolvers (`PanelGateExpectedReviewers`/`PanelGateApproveThresholdExpr`/`PanelGateReviewerSlots`) with per-field inheritance chain (own → `bead` for `adhoc` → global → defaults) and the **deterministic interleaved global-cursor lens expansion** (default lens order over 6 lenses; one cursor advances ONLY over lens-less slots; explicit lenses never consume it; cursor starts at 0); `PanelGateAdvisoryDefault`; the full R4(a)–(h) refusal surface with recovery lines; NO model/lens name-membership validation (unknown model id never errors). In `cmd/mindspec/config.go`: reviewer-count rendering migrated to `CountValue()` (the `%d`-on-`*int` false-green fix, gated by `go test ./cmd/mindspec`). Doc-sync in core+workflow interfaces.md (incl. the standing-protocol YAML with the `note:` line).

**Known accepted deviation**: the implementer ran per-bead gates (touched packages only) per the plan's Testing Strategy instead of repo-wide `go test ./...` (which spawns real LLM subprocesses in internal/harness); packages internal/adr→internal/guard were green before the run was cut. Do not re-litigate unless your lens finds an actual cross-package break.

## Files in scope

- `internal/config/config.go` (+439/−~30), `internal/config/config_test.go` (+716)
- `cmd/mindspec/config.go` (+6/−2)
- `.mindspec/domains/core/interfaces.md` (+154), `.mindspec/domains/workflow/interfaces.md` (+8)

## Slot lenses

| Slot | Family | Lens |
|:-----|:-------|:-----|
| O1 | Opus | Author-of-record — diff delivers plan §Bead 1 Steps 1–7 + Acceptance Criteria exactly; nothing skipped/added/reinterpreted. |
| O2 | Opus | Codebase-pin — run the full Bead-1 Verification checklist yourself in the bead worktree; confirm each item. |
| O3 | Opus | Contract stability — 109 byte-compat identity (zero-config + 109-era fixtures load identical), and the surfaces Beads 2/3 + specs 110/111 consume (resolver signatures, PanelGateKeys order, CountValue, slots shape vs 112 R9's `--gate --json` members). |
| S1 | Sonnet | Empirical prober — scratch programs/configs under /tmp: inheritance chains (adhoc→bead→global→default per FIELD), threshold-range refusals at every link, substitutes edge cases, round-trip fidelity of the standing-protocol YAML. |
| S2 | Sonnet | Schema/type correctness — `Count *int` semantics (omitempty? nil→default-1 everywhere? any place still formatting the pointer?), YAML unmarshal edges, deterministic map ordering in slot expansion + rendering, error wrapping. |
| S3 | Sonnet | Next-bead integration — Bead 2 (panel.json `gate` field) and Bead 3 (gate-aware advisory + `config show --gate --json`) per the plan: are the symbols/shapes they consume present and ergonomic? Any missing accessor that forces Bead 3 to reach into internals? |
| F1 | Fable | Adversarial — attack the R3 interleaved global-cursor expansion (construct mixes where a wrong implementation — per-entry rotation, slot-index mod, cursor-consuming explicit lenses — would produce the same output as the correct one and check the tests distinguish them; hunt non-determinism); attack R4 refusal completeness (find a malformed config that loads) and the R2 identity claim (find a 109 config whose behavior changed). |
| G1 | GPT-5.5 | Second empirical prober + robustness — hostile YAML: enormous/negative counts, control bytes and ANSI in model ids/lens names/note (do they reach `config show` output or error text unescaped? cross-check 109's escapeConfigValue pattern), duplicate gate keys, threshold expression edge parsing. |

## Your job

Verdict: APPROVE / REQUEST_CHANGES / REJECT → `<your-slot>-round-1.json` in this dir (`/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.mindspec/specs/112-per-gate-panel-config/reviews/spec-112-bead1/`). Keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence`, `rationale` (<=200 words), `concrete_changes_required` (empty if APPROVE), `findings`.
