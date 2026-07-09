# spec-113-bead4 (mindspec-r6hk.4, R4) — PASS (8/8 effective, all findings resolved)

Round 1: 8/8 APPROVE — O1 0.95, O2 0.92, O3 0.97, S1 0.97, S2 0.95, S3 0.97, R8 0.97, F1 0.92.
  O2 + F1 independently raised ONE real finding (not out-voted — fixed per standing rule):
  subtest (b) of TestLoad_EmptyStringModel asserted only `panel.reviewers[0]` + generic `recovery:`,
  which the `count must be >= 1` branch (config.go:~564) also satisfies → could pass on the wrong branch.
Fix (amend, c92b3d26): subtest (b) now asserts the distinctive `sets neither "model" nor "family"` phrase
  (config.go:561 byte-for-byte), uniquely pinning the neither-set branch. Kept prior asserts too.
Round 2: O2 0.97 ADDRESSED, F1 APPROVE ADDRESSED (F1 mutation-probe: injecting a generic
  `panel.reviewers[0]...recovery:` error now reds subtest (b) specifically — loophole gone).

Bead = R4: doc comment (config.go) superseding 112 R4's "empty-string model" phrase + TestLoad_EmptyStringModel
pinning resolve-to-family (accept {model:"",family:codex}→codex) and neither-set refusal ({model:""} alone).
No behavioral change: Reviewer.Model stays string; no UnmarshalYAML; existing 112 tests unmodified.
Note: two round-2 reviewers initially wrote verdicts to a stray <bead-wt>/reviews/ path (ambiguous "..." in prompt);
relocated to the spec-dir panel + removed the stray before complete (contamination-avoidance).
