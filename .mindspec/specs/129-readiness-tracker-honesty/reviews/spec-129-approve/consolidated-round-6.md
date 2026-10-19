# Spec 129 — spec gate, round 6: consolidation (polish)

- **Result.** Reviewed commit `8b3a5535`: 9/9 APPROVE, tally PASS.
- **Remaining asks.** 10 minor asks from approving slots, each fixed or refuted.
- **This commit.** `spec.md` only.

## Decisions

- **`reviewed_head_sha` value classes** (O1r6.1, F2r6.1, F3r6.3), applied identically to R4.3 and F2:
  - (a) empty → skipped as "no review recorded";
  - (b) non-SHA placeholder → skipped and reported;
  - (c) well-formed object ID → evaluated;
  - (d) option-like or revision-expression text → MALFORMED, fail closed.

  Only (c) ever reaches git. The real `design-review-no-code` bead registration is in `.mindspec/reviews/6ou2-fix-zfc/panel.json`. AC-11 gains (x-e) and (x-p), and (x-m) is extended.
- **Unresolvable SHA** (F3r6.2). A well-formed SHA with no local commit (`git cat-file -e <sha>^{commit}` fails) is outside the tip, so it gives `UnlandedReview` and never an F5 error. Leg (x-g).
- **Per-checkout visibility** (F3r6.1). It is stated. The output names the absolute roots scanned and recommends the spec worktree. Leg (x-w).
- **Amend after panel** (F3r6.4). This is AC-11 (vii-a). The exit is the dependent's scoped override, and deleting the registration is deliberately not offered.
- **Plan blob probe** (O1r6.2, F2r6.2). Three steps: `rev-parse --verify refs/heads/spec/<S>`, then `ls-tree spec/<S> -- <path>`, then the blob read. A typed `ErrPathNotAtRef` separates absence from failure. If the branch exists but the blob is absent, the checkout file is used. The AC-1 fifth fixture covers this.
- **Repo convention** (F1r6.1). The counts are dropped. The convention is scoped to BEAD registrations, with a re-derivation recipe. My run at `8b3a5535` found 72 tracked panel.json, 41 bead registrations, and no `-rN` bead directory; the counts are not written into the spec.
- **Residual wording** (O2r6.1). Now "same forge MECHANISM as Spec-125 item (c); NEW, bounded REACH", in R4.2 and in the touchpoint, which AC-18 pins.

## Coverage table (10 asks)

| Ask | Disposition |
|---|---|
| F1r6.1 | FIXED — counts dropped; bead-only scope; re-derivation recipe |
| F2r6.1 | FIXED — empty is skipped, consistently with R4.3; (x-e) |
| F2r6.2 | FIXED — branch exists, blob absent → checkout; AC-1 fifth fixture |
| F3r6.1 | FIXED — per-checkout visibility; roots printed; spec-worktree advice; (x-w) |
| F3r6.2 | FIXED — unresolvable → outside-tip; (x-g) |
| F3r6.3 | FIXED — empty is absent (same as F2r6.1) |
| F3r6.4 | FIXED — (vii-a); deletion not offered |
| O1r6.1 | FIXED — skip-and-report placeholders; fail closed only on option-like or revision-expression text; (x-p) |
| O1r6.2 | FIXED — typed probe (ref check plus ls-tree) |
| O2r6.1 | FIXED — mechanism vs reach wording |

**Totals:** 10 FIXED, 0 REFUTED, 0 DEFERRED.

## Errors in this brief

None found. The 6ou2 placeholder, `FileAtRef`'s single error wrap and the untracked `reviews/` were all verified.
