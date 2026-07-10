# spec-113-final — PASS (effective 12/12 APPROVE, all findings adjudicated)

Round 1: 9 APPROVE / 3 RC (G1,G2,G3). Round 2: 11 APPROVE / 1 RC (S2, non-bead GitErr leak). Round 3: 12 APPROVE / 0 unresolved.

## Findings & adjudication (findings never out-voted)
- **G2 r1 (non-bead target injection, RCE-class footgun + control-byte forgery)** — FIXED (82b28b2f): escapeConfigValue on target displays + shellQuoteTarget on the copyable recovery command. G2 r2 APPROVE (empirically: copied recovery no longer executes the `;`-suffix; control bytes escaped).
- **S2 r2 (non-bead GitErr leak — NUL-target forces GitErr leg, raw %v re-embeds hostile target)** — FIXED (45b34ee7): escapeConfigValue on facts.GitErr at verify branch + sanitizeNonBeadDecision leg-5b rebuilt from escaped gitErr. New TestPanelVerbs_NonBeadGitErrHostileTargetEscaped, RED/GREEN-verified. S2 r3 + G2 r3 APPROVE (empirically re-verified).
- **F3-1/F3-2 (advisory-text)** — FIXED (folded into 82b28b2f): recovery now includes --gate when stamped; omits empty --target.
- **G3-1 (committed .beads showed r6hk.3 in_progress)** — FIXED: regenerated from live Dolt (r6hk.3 closed). G3 r2 APPROVE.
- **G1 r1 + G3-2 (`go test ./...` red)** — EVIDENCED-REFUTED (environmental): internal/harness TestLLM_* need Claude Code auth the sandbox lacks + parallel-load timeouts; S1 proved isolated reruns pass; packages untouched by the 113 diff. Round-2/3 scoped to touched packages → G1 r2 + G3 r2/r3 APPROVE.
- **S2 r3 (bead-path leak: gate.go raw f.BeadID/f.GitErr for bead panels)** — REAL but PRE-EXISTING (commit d90d29d09, 2026-06-14) + OUT OF 113's FENCE (113 AC-global mandates ZERO diff to gate.go/complete/instruct; fixing requires editing the shared decision message → falsifies R1's fence + blast radius). FILED as **mindspec-fl91 (P1)**. S2 independently verified the blame + fence + falsifier and ACCEPTED the deferral → re-issued APPROVE.
- **F1/F3 cosmetic gate.go notes** — pre-existing, package-fenced this wave → fl91-adjacent follow-up material.

## 12/12 sign-off
R1 zero-internal-diff fence holds NET (gate.go/instruct/complete empty diff; only create.go +13 for R3; tallyExitAction 2-arg). R2 no bypass + monotone + mirror byte-identical. R3 decision-inert across all 5 enum values + single-enum + config-free leaf. R4 comment+test only. All 4 ACs + AC-global met. ADRs adhere (0037/0035/0040/0030). No private content (public repo). Clean merge to main. .beads consistent (all 4 beads closed). 113's own attack surface (non-bead panel verbs) fully hardened + RED/GREEN-verified.
