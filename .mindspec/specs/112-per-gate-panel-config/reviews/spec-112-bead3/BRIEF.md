# spec-112-bead3 — Round 1 Review Panel (8 reviewers)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-112-per-gate-panel-config/.worktrees/worktree-mindspec-lma4.3`
**Branch**: `bead/mindspec-lma4.3`
**Commit under review**: `b27e793b2715b5f9e22c3a5e978351da0983b0a8` — `feat(112): gate-aware advisory + config show gates/substitutes/--gate contract [mindspec-lma4.3]`
**Panel**: 8 slots — R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex. **Pass = ≥7 APPROVE, no REJECT.**
**This is the FINAL bead of spec 112** — a clean pass here leads to the 12-slot final review then `impl approve`.

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; scratch under /tmp; pin all reads to SHA `b27e793b`; leave `git status` clean. Edit nothing except your own verdict file.

## What the work does (bead lma4.3 — delivers spec 112 R7–R9)

Spec 112 makes reviewer panels per-GATE (spec/plan/bead/final each get their own model+lens mix), building on 109's config schema. Beads 1–2 (already merged) added the resolvers and the recorded `panel.Panel.Gate` field. **This bead is the read/advisory surface**: it makes the reviewer-count advisory gate-aware and exposes the resolved per-gate config to operators. It writes NO panel.json, drives NO runner/substitution behavior, and changes NO decision semantics (scope fence R9).

Steps (verify each):
1. **R7 — gate-aware advisory at the complete-gate call site** (`internal/complete/complete.go` ~:379-390): the flat `gateCfg.PanelExpectedReviewers()` compare is replaced with `gateCfg.PanelGateAdvisoryDefault(panelReg.Panel.Gate, panelReg.Panel.IsBead())`, **guarded on `panelReg != nil`** (panelGate returns a nil registration on fail-open paths — empty bead ID, no registered panel; an unguarded `panelReg.Panel.Gate` deref would panic where 109 relied on `reviewerCountAdvisory`'s own nil-check). The advisory runs only when `panelReg != nil` AND `ok`. A nil registration stays advisory-silent, panic-free. The gate's Allow/Block (computed earlier by panelGate) is untouched.
2. **R7 — same gate-aware selection in `config show`** (`cmd/mindspec/config.go` `reviewerCountNotesFor`): per scanned registration, same `PanelGateAdvisoryDefault(reg.Panel.Gate, reg.Panel.IsBead())`, skipping when `ok` is false. Both call sites share the one selection rule (no drift).
3. **R8 — `renderConfig` extension** (pure over `*config.Config`): echo a set `panel.note` (escaped); render `panel.gates` — only configured gates, in `config.PanelGateKeys` **enum declaration order** (NEVER map iteration order) — each with its configured entries (model/family/lens/count), its resolved reviewer sum (`PanelGateExpectedReviewers`), and its **raw** threshold expression (`PanelGateApproveThresholdExpr`); render `panel.substitution.substitutes` in **sorted-key order** with the slot-id-preservation convention line. `gates:`/`substitutes:` keys are **never omitted** (render `{}` when empty — R8 falsification clause). 109's "declared, not yet enforced" annotations on inert models:/loop:/runner: stay.
4. **R8 — known-model advisory**: scan global reviewers + every gate's reviewers + both sides of `substitutes`; warn (escaped, **exit-code-inert**) on any id absent from `config.KnownModels()`. Seeded ids/family strings never warn.
5. **R8/R9 — `config show --gate <name> [--json]`**: one `buildGateResolvedDoc` (calling only the R3 resolvers) feeds both a text renderer and a `gateResolvedJSON` using **real `encoding/json`** (never string concat). The JSON doc has **exactly 5 members**: `gate`, `slots[{slot,model,lens}]`, `expected_reviewers`, `approve_threshold`, `substitution{substitutes,claude_sub_on_quota,in_force}` — `in_force` flips per R5. Unknown `--gate` exits non-zero with the ADR-0035 five-key recovery line; `--json` without `--gate` is refused with its own recovery line. The command writes nothing to disk.

## The CRITICAL escaping/injection requirement (R7 Fable + R8 codex — probe hardest)
This is the spec-109-final-review G2 finding class. **EVERY** config-controlled string this bead adds to the text path — `note`, reviewer model, reviewer lens, substitutes keys AND values, and any warning line embedding one — MUST pass through `escapeConfigValue`. The `--json` path must round-trip **byte-exactly** through the real encoder (no hand-rolled escaping). A hostile config (control bytes, ANSI, newlines forging fake recovery lines) must never reach a raw terminal write and must survive a JSON round-trip intact. Verify NO config-controlled string reaches stdout unescaped on any path (text or the warning lines).

## The headline acceptance proof (R4 Sonnet + R8 codex — reproduce empirically)
Build the branch binary and drive it against the standing-protocol YAML (spec 112's Goal). Expected, verify by hand (use `jq`, not grep):
- `config show` prints all 4 gates with resolved sums **9 / 9 / 6 / 12** (spec+plan 9, bead 6... note the protocol: spec/plan = 3F+3O+3G = 9; bead = 3O+3S+1F+1G = 8; final = four families = 12 — **confirm the actual configured sums against whatever mix the test/proof YAML encodes**; the point is the resolved sum equals the configured entry counts, in enum order).
- `config show --gate final_review --json | jq '.slots|length'` equals the configured final mix; `jq -r .approve_threshold` equals the configured raw expression string.
- `config show --gate <adhoc> --json`'s slots equal `bead`'s slots if the config maps adhoc→bead (per the resolver rule).
- Unknown `--gate` → non-zero exit + 5-key recovery line. `--json` with no `--gate` → refused.
(Use the exact numbers the proof YAML encodes; the implementer's report cited 9/9/6/12 and final=12/threshold "11" for the standing protocol — reproduce against whatever config you set.)

## One deviation the author flagged (assess it — R5 especially)
`rootCmd`/`configShowCmd` is a package-level cobra singleton; cobra does NOT reset a flag between `Execute()` calls when a later invocation omits it, so `--gate`/`--json` state leaked across tests. Fixed with a `resetConfigShowGateFlags(t)` helper (`Set` + `t.Cleanup`) applied at the start of every `config show`-executing test. Judge: is this a sound test-hygiene fix, or does it mask a real state-capture bug in the command itself? (The author read flags via `cmd.Flags().GetString/GetBool`, not package vars — confirm no shipped-code state capture.)

## Scope fence (R9 — confirm honored)
NO `internal/config` edits (all resolvers pre-existed from Bead 1); NO panel.json writer behavior (spec 110); NO runner/dispatch/substitution consumption (spec 111); NO change to `PanelGateDecision`/`ApproveThreshold()` semantics; the `internal/panel` leaf invariant intact (`go list -deps ./internal/panel | grep internal/config` → non-zero exit).

## Files in scope (final state at b27e793b)
- `cmd/mindspec/config.go` (+326) — renderConfig extension, known-model advisory, `--gate [--json]`
- `cmd/mindspec/config_test.go` (+521) — 4 new tests + the flag-reset helper
- `internal/complete/complete.go` (+20/−…) — gate-aware advisory call site
- `internal/complete/panel_advisory_test.go` (+143) — `TestPanelAdvisory_GateAwareCompare` (9 subtests)
- `.mindspec/domains/workflow/interfaces.md` (+62) — CLI surface + R9 additive-only contract
- `.mindspec/domains/workflow/architecture.md` (+42) — gate-aware advisory rule

## Known pre-existing failures (NOT this bead)
`go test ./...` shows 2 unrelated failures: `internal/harness` (timeout/hang) and `internal/instruct` `TestRun_IdleNoBeads` (z4ps). Both reproduce on the parent baseline `a07bb399`; neither is caused by this diff. The bead's own packages (`cmd/mindspec`, `internal/complete`, `internal/config`) are green.

## Your job
Evaluate cold against spec 112 R7–R9. Confirm: both advisory call sites are gate-aware + nil-guarded; renderConfig enum-order/sorted-key/never-omit rules hold; known-model advisory is exit-inert and correct; the `--gate --json` doc has exactly 5 members via real encoding/json with the ADR-0035 recovery on unknown gate; **all** config-controlled strings are escaped on every text path and round-trip byte-exact through JSON; the headline gate-sum/slots/threshold proof reproduces; the scope fence holds; the cobra flag-reset is sound not masking a bug.

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", e.g. "R1 opus" / "R7 fable" / "R8 gpt-5.5"), `verdict`, `confidence` (0–1), `rationale` (≤180 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
