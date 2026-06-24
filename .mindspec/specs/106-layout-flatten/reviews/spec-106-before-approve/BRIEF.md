# spec-106-before-approve — Round 2 Review Panel

**Target**: `spec/106-layout-flatten` (spec-before-approve; `bead_id` null)
**Spec under review (REVISED)**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-106-layout-flatten/.mindspec/docs/specs/106-layout-flatten/spec.md`
**Repo root**: `/Users/Max/replit/mindspec`

## Round 2 context

Round 1 returned **REQUEST_CHANGES** (2 APPROVE / 4 REQUEST_CHANGES). The design direction (flatten + 3-phase spine + co-located reviews) was endorsed by all six; the asks were mechanism-precision and coverage. The spec has since been REVISED to address every consolidated round-1 ask — it is now **17 requirements / 25 acceptance criteria** (was 14/16), validator-clean.

The full round-1 consolidated change list is here — read it: `/Users/Max/replit/mindspec/review/spec-106-before-approve/consolidated-round-1.md`

Key round-1 blockers the revision claims to fix:
1. **DetectLayout** now `{flat | canonical | legacy | greenfield | mixed}` with `mixed` = HARD ERROR (Req 2, AC3).
2. **Mover** now has a defined checkpoint/crash-recovery state machine + rollback semantics (hard-reset pre-publish, refuse-after-publish) (Req 4, AC7/AC8).
3. **Branch/PR discovery algorithm** for the migrate precondition now specified (Req 11, AC16).
4. **budgeter.go / DocsDir-root consumers** now governed (Req 3, AC5/AC6).
Plus: ref-anchored `LoadOwnershipAtRef`/`domainManifestRelPath` named in Req 6; link gate scans ALL markdown links (Req 5); AC8/AC19 grep fixed; `redact` claim struck; DOCS-LAYOUT.md amendment governed (Req 16); AC13/AC17 skill coverage broadened + ms-spec-grill flat-path assertion; occurrence stat corrected (~99/21).

## Your job (round 2)

Re-read the REVISED spec.md in full. For YOUR lens (same lens you held in round 1 — see your round-1 verdict JSON at `<your-slot>-round-1.json`), evaluate each concrete_changes_required item you raised in round 1 as **ADDRESSED / PARTIAL / MISSED**, and surface any **NEW** issue the revision introduced (e.g. a new req/AC that is wrong, a renumbering error, an over-correction). Verify claims against the real code where relevant; don't take the spec's word.

The bar for APPROVE: your round-1 asks are addressed (or any remaining gap is minor enough to defer to plan/impl), and the revision introduced no new blocker.

**Verdict**: APPROVE / REQUEST_CHANGES / REJECT.

Output JSON to `/Users/Max/replit/mindspec/review/spec-106-before-approve/<your-slot>-round-2.json` with keys:
`reviewer_id`, `verdict`, `confidence` (0-1), `rationale` (≤200 words), `concrete_changes_required` (array; empty if APPROVE), `findings` (array of {severity, area, issue, status?, hard_block?}) where `status` is ADDRESSED/PARTIAL/MISSED/NEW for each round-1 item. An artifact-gate/factual-error finding may set `"hard_block": true`.
