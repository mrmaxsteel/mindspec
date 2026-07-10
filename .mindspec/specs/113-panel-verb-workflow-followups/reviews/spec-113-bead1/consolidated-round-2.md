# spec-113-bead1 (R1, LOAD-BEARING) — PASS (8/8 round-2, all findings resolved)

Round 1: 7 APPROVE / 1 REQUEST_CHANGES (F1). CODE was correct end-to-end (R8/F1/S2 empirical); all findings were TEST-strengthening. Adjudicated per standing rule (fix or evidenced-refute), NOT out-voted.
Round 2 (SHA 15dab51b, panel.go byte-identical — test-file-only amend): F1 0.9 ADDRESSED, O1 0.97, O2, O3, S1, S2 0.93, S3, R8 0.95 — ALL APPROVE.

## Findings resolved
1. [F1 M5 — LOAD-BEARING, mutation had survived] TestPanelVerify_NonBeadMissingTargetRef didn't pin the IsRefNotFound classification. FIXED: added `strings.Contains(out,"no longer exists")` (leg-5-only phrase). F1 re-ran the M5 mutation (IsRefNotFound→false) round 2 → test now RED. Genuinely pinned.
2. [O2+F1+R8] TestSanitizeNonBeadDecision missing leg(3) abandoned + leg(4) round-mismatch rows. FIXED: both added, built from real PanelGateDecision; O2/R8 confirmed genuine (route the real gate legs, non-vacuous RawMergeFence("") strip assertions).
3. [S2] Leg 8 incomplete non-bead lacked E2E CLI test. FIXED: new TestPanelVerify_NonBeadIncompleteBlocks drives the real runPanelVerbCmd path (1/2 verdicts → no PASS / tally non-zero). S2 confirmed ADDRESSED.
4. [R8 cosmetic] double-space/empty-Target artifact. EVIDENCED-REFUTED-ACCEPTED: empty --target rejected by panel create guard (R8 empirically confirmed `panel create --target ""` → error), so CLI-unreachable; outside ADR-0037 anti-footgun trust boundary. No code change.

Code correct (zero internal diff, tallyExitAction 2-arg, bead panels byte-identical, pinned tests unmodified). The fix made the tests as strong as the code.
