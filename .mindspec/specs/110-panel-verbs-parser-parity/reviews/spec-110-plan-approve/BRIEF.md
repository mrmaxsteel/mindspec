# spec-110-plan-approve — Round 1 (plan review, 9 reviewers, three families)

**Under review**: `.mindspec/specs/110-panel-verbs-parser-parity/plan.md` @ **e86082d629adfc9f853b7dbe0870b3ed8c8b42fe** (773 lines) in worktree `/Users/Max/replit/mindspec/.worktrees/worktree-spec-110-panel-verbs-parser-parity`. Read the APPROVED `spec.md` beside it — the plan is judged against that contract.
**Panel**: 9 reviewers, three families — F1–F3 Fable, O1–O3 Opus, G1–G3 GPT-5.5 (codex). Pass = **≥8 APPROVE, no REJECT**.
**READ-ONLY RULE**: you write your verdict JSON and NOTHING else; scratch work under /tmp; pin all reads to the SHA above (do not edit the worktree; builds/tests you run must leave `git status` clean — use `go build -o /tmp/...` etc.).

## What the plan does

Spec 110 ratchets the panel lifecycle from skill prose into three in-binary verbs (`mindspec panel create | verify | tally`, per ADR-0040) and closes the spec-approve parser asymmetry (Impacted-Domains resolution + ADR-Touchpoint existence run at spec-approve with plan-approve's exact severity). The plan cuts **5 beads**: B1 `internal/panel` leaf-safe `Create` writer (panel.json + BRIEF machine-header co-bump) + R4 schema doc; B2 `internal/validate` spec-approve parity checks; B3 `internal/instruct` ratchet of the duplicated `PanelStateEntry.verdict()` matrix onto `panel.PanelGateDecision`; B4 `cmd/mindspec` verb tree (thin adapters, contract test pinning single-home R7a); B5 skill de-dup (mechanized prose out, judgment sections kept, `.claude`/`plugins` mirrors byte-identical). DAG: B1‖B2‖B3 roots → B4(←B1) → B5(←B4), depth 3.

## Context you need

- **Spec 109 is on `main`** (this branch is rebased onto it): config resolvers `PanelExpectedReviewers()`/`PanelApproveThresholdExpr()`, ADR-0040, ADR-0037 threshold amendment. The plan pins B4 against these.
- **Spec 112 (per-gate panel config) is plan-approved and in flight** — its 3 unclaimed beads will touch `internal/config`, `internal/panel` (adds a decision-inert `gate` field), `internal/complete` + `cmd/mindspec/config.go` before 110's beads run. The plan's §Risks/Sequencing claims zero function-level overlap (110's panel/cmd work is new files). Assess that claim.
- **Known advisory**: `mindspec validate plan 110-…` emits WARN `decomposition-scope-redundancy R=0.11 < 0.15` (low cross-bead scope overlap). The plan's Testing Strategy pre-empts it as the expected shape of a cleanly-separated five-package change. Judge whether that's right or a real decomposition smell.
- **Orchestrator pre-verified grounding** (you may re-verify, don't re-litigate blindly): `approve/spec.go:47-50` ValidateSpec+HasFailures; `plan.go:142-146` normalizeImpactedDomains; panel symbols (`FileName`, `Scan`, `PanelGateDecision`, `ResolveGateFacts`, `PanelDirScanRoot`, `verdictFileRE` = `^(.+)-round-([1-9][0-9]*)\.json$`, `ConsolidatedName`); the 109 config resolvers; `PanelStateEntry.verdict()` exists with NO consumers outside `internal/instruct`; `guard.NewFailure`/`HasFinalRecoveryLine`; `cmd/mindspec.configShowReviewRoots`; skill mirrors byte-identical.

## Slot lenses

| Slot | Lens |
|:-----|:-----|
| F1 | Adversarial — attack the plan's central claims: single-home R7a (is the contract test actually unfakeable?), config-free leaf R7b, "parity, not stricter" R5/R6 (could B2 reject something plan-approve tolerates, or miss something it rejects?). Find the strongest realistic failure. |
| F2 | DAG / merge-signal — false or missing dep edges, the three parallel roots' doc-sync collision claims (disjoint domain-doc files?), bead-boundary consumer ownership, the §Risks 112-overlap claim. |
| F3 | Falsification / test-coverage — for EACH Verification command: can it actually FAIL if the step it proves was skipped or done wrong? Hunt false-greens (the 112-r1 class: a check that passes with the work missing). |
| O1 | Implementability — will the steps work as written: B3's GateIO wiring (Worktree "" ⇒ dirty-leg skipped — check `ResolveGateFacts` semantics), B1's BRIEF delimited-region rewrite, B4's cobra wiring + seams, test fixture feasibility. |
| O2 | ADR / process conformance — adr_citations right (incl. the deliberate ADR-0030 omission — would citing it really fire `adr-cite-irrelevant`? would omitting anything fire `adr-coverage-missing`?), step counts 3–7, plan-approve gates pass. |
| O3 | Spec coverage — every spec requirement R1–R8 and AC1–AC12 delivered by exactly the beads claimed in the Requirement→bead map and Provenance table; no scope creep beyond the spec; Non-Goals respected. |
| G1 | Test-runnability — is every Verification command runnable as written on macOS (grep/ugrep forms, `go test -run` anchors, the manual e2e), and does each prove what it claims? |
| G2 | Security / robustness — `panel create` path handling (slug → filepath join), BRIEF legacy/missing-marker edge cases, guard/error message injection surface (the 109-final-G2 class), config-value rendering. |
| G3 | Downstream contract — spec 111 (workflow-panel-runner) builds on these verbs; is B1's R4 schema doc + B4's CLI contract a stable, agent-neutral surface? Does B5's skill trim leave the skills usable mid-transition? |

## Your job

Evaluate the plan cold through your lens. Verdict: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `.mindspec/specs/110-panel-verbs-parser-parity/reviews/spec-110-plan-approve/<your-slot>-round-1.json` (in the worktree above) with keys: `reviewer_id` ("<slot> <family>"), `verdict`, `confidence` (0–1), `rationale` (≤160 words), `concrete_changes_required` (array; empty if APPROVE), `findings` (array). A finding may set `"hard_block": true` only for a missing declared artifact.
