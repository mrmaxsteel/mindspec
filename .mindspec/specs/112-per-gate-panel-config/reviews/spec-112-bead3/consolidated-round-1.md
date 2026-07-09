# spec-112-bead3 — consolidated round-1 changes

Tally: 6 APPROVE (R1 0.93, R2 0.95, R3 0.95, R4 0.94, R6, R8 codex 0.94) / 2 REQUEST_CHANGES (R5 sonnet 0.85, R7 fable 0.75) / 0 REJECT. Threshold 7/8 not met → fix round. Both RCs converge on ONE defect. The escaping/injection class (this bead's load-bearing risk) is CONFIRMED CLEAN by R7 (strconv.Quote/%q throughout, no raw control bytes, JSON encoding/json byte-exact round-trip), R8, and R4 independently. Headline proof (gate sums 9/9/8/12 enum order, adhoc==bead, unknown-gate + --json-without-gate refusals, threshold raw "n-1"/"11") reproduced by R4/R5/R7/R8. Scope fence + leaf invariant hold. Nil-guard sufficient (Registration.Panel is a value field).

## The one defect (R5 + R7 convergent — must fix)

1. **`--gate --json` emits `"substitutes":null` instead of `{}` for the DEFAULT (unconfigured-substitutes) config.** `buildGateResolvedDoc` (cmd/mindspec/config.go) assigns `cfg.Panel.Substitution.Substitutes` — a nil map when unconfigured — straight into the JSON doc, so `config show --gate <g> --json` on the common no-substitutes config prints `"substitutes":null`. This breaks the jq consumers the R9 contract names (`.substitution.substitutes|keys` errors on null; the ms-panel-run step-0 pattern), is inconsistent with the sibling text path (`renderSubstitutes` special-cases empty → `substitutes: {}`) and with `slots`' deliberate never-null treatment, and would bake `null` into 110/111's additive-only clause. The existing empty-substitutes test asserts only `in_force`, so the shape is unexamined (typed-struct unmarshal treats nil == {}).
   - **Fix**: in `buildGateResolvedDoc`, backfill a non-nil map — normalize `Substitutes` to `map[string]string{}` when nil — before building the doc, so it marshals to `"substitutes":{}`.
   - **Test**: add a RAW-STRING assertion (not typed-struct unmarshal) that `config show --gate <g> --json` on a config with no substitutes contains `"substitutes":{}` and NOT `"substitutes":null`.

## Non-blocking (carry to follow-up mindspec-naq0, do NOT fix in this round)
- R7 F4a: the global `reviewers:` block renders family/count only — a model-only entry prints an empty `- family:` line (pre-existing display gap).
- R7 F4b: `reg.Slug()` written unescaped at config.go:539 (pre-existing, out of this bead's scope).
- R4: (subsumed by the fix above — it was the same null-vs-{} observation, non-blocking as R4 framed it).

## Constraints for the fix author
- ONE commit on `bead/mindspec-lma4.3`: `fix(112): normalize empty substitutes to {} in --gate --json [mindspec-lma4.3]`.
- Touch only `cmd/mindspec/config.go` + `cmd/mindspec/config_test.go`. All existing tests stay green.
- Do NOT write any scratch `.mindspec/config.yaml` into a real worktree — use ABSOLUTE `/tmp` paths only for manual checks (a round-1 reviewer's relative-path scratch write, combined with the harness cwd-reset, contaminated a sibling worktree).
- No push, no bd, no `mindspec complete`, no lifecycle command.
