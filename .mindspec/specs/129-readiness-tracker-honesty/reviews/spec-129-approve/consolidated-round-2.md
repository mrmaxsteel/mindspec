# Spec 129 — spec gate, round 2: consolidation

- Reviewed commit: `cb272d50`. The result was 2 APPROVE (F1, O2) and 7 REQUEST_CHANGES (F2, F3, O1, O3, G1, G2, G3), with no hard block and 36 asks.
- I read every verdict's findings and asks, including those from the two APPROVE slots, whose asks count.
- Probes ran only in scratch, under `/private/tmp/claude-503/-Users-Max-replit-mindspec/d05dec1f-1671-4cbd-8202-e55ee47cdf39/scratchpad/p129s/reviser/`:
  - the throwaway bd database at `bdprobe/`;
  - the clone at `clone/`, with three probe tests: `internal/phase/zz_reviser_r2_probe_test.go`, `internal/lifecycle/zz_o3r2_probe_test.go` (O3's probe plus mine), and `internal/lifecycle/zz_reviser_probe_test.go`.

## Groups and what was done

### 1 — The `Merge bead/X` target gap (O3, G1, G3, O1, F3, F2, O2)

**Verified.** `gitutil.MergeInto` writes `-m "Merge <source>"` (`internal/gitutil/gitops.go`). The subject names no target.

**Reproduced O3's false PASS** in the clone with `TestO3R2_SiblingUndirectedSubject_PanelSetConfirmsStale`:
- With the round-1 fallback, the result was `lm=&{e510c29…}` and `err=<nil>`.
- The spec tip's `x.txt` is `x1`, so round 2's fix never landed.

**Emulated the new rule** with `TestRV2_FallbackPanelOnly_NoEvidence`: the same fixture, with the panel datum withheld on the fallback path. The result is `*LandedMergeNoEvidence`.

**Fixed (R4.2, the ADR-0041 amendment item (i), AC-11):**
- **Admitted forms.** The two `into` forms are target-pinned, and only with this spec branch as the target: `Merge branch 'bead/X' into <spec-branch>` and `Merge bead/X into <spec-branch>`. The `Merge bead/X` form is stated as UNDIRECTED.
- **Confirmation rule on the fallback path.** Only (b) a surviving tip or (d) a landed binding confirms a fallback candidate. A panel-only match → `*LandedMergeNoEvidence`, and `reattest` is the exit.
- **Residual stated.** A short-form merge into another branch that later reaches the tip is identified when (b) or (d) corroborates it.
- **Fallback ambiguity rule.** One second-parent group exactly corroborated by (b) or (d) resolves the ambiguity, in both FindLandedMerge and reattest. This answers G3's (2).
- **AC-11 (iii) reconciled.** X lands by its own merge, and the long-form X→Y merge is excluded with no ambiguity. (iii-neg) pins that a path only through Y gives NoCandidate. Also added: (vii) O3's false PASS, (viii) the short form X→Y with a surviving tip, and (ix) the short-form ambiguity.
- **`reattest` on a fallback candidate** renders a target-unknown caveat and an unreached-reviewed-head warning.
- **O1r2.2:** `Merge <source> into <target>` is written only by `gitutil.MergeBranch`, for spec→main finalize (`mindspec_executor.go`), never for a bead.
- **#251 boundary** named (F1r2.1).

### 2 — Override fingerprint (G1, G3; F2, F3)

**Verified.** `evaluateMF3` returns on the first failing dependency (`readiness.go`), so a digest of `Detail` covers only that one.

**Fixed (R6.2/R6.4):**
- `readiness.Signal.Failures []string` holds canonical, raw (pre-render) records for every failure. All four `evaluateMF*` functions aggregate instead of stopping at the first failure; MF-3 now evaluates every closed member dependency. This API change is priced in R6.2 and In Scope.
- One `readiness.FailureFingerprint` (sha256 over the sorted, deduplicated records) is used by both the writer and the reader.
- `indeterminate` records are never coverable.
- **ACs:** A then A+B is uncovered; the MF-4 second blocker is uncovered; two runs give byte-identical fingerprints (F2r2.7); the writer and reader agree over a Detail containing a control byte (F3r2.6).

### 3 — #255's own bead (O1; O3)

**Decision: fix it, with no core change.** R1.1 now selects every bd-ready `<E>.<N>` bead whose lineage, through the UNCHANGED `phase.FindEpicForBead`, resolves to E. A missing parent edge does not matter, and the selection uses no `--parent` and no title substring: one global `bd ready -n 0`, plus a `bd show` only for plan-form rows whose parent is not E.

**Evidence** (real scratch bd 1.1.0 plus the real `phase.FindEpicForBead` from the clone):
- Setup: `bd create "[129-probe] reporter shape" --parent pr-1rs` → `pr-1rs.4`, then `bd dep remove pr-1rs.4 pr-1rs`. `bd dep tree pr-1rs.4` then shows no dependencies, which is the reporter's "zero dependencies".
- Command: `RV2_BD_DIR=<bdprobe> go test ./internal/phase -run TestRV2_`.
  - Result: `pr-1rs.4 -> epic="pr-1rs" spec="129-probe" err=<nil>` (title fallback).
  - The neutral-title sibling gives `pr-1rs.5 -> err=no epic lineage` (class N1, repairable).
- `beadIndex` matches `.4`.
- AC-1 (bead G) claims the reporter shape with no override. AC-2 keeps N1.

**Honest limit, stated in OQ-9 and Non-Goals:** readiness also evaluates `<other>.<N>` beads by suffix. `next` names those as N3, and R1.8 refuses to claim them. 129b removes the title fallback and tightens `beadIndex`.

### 4 — Unscoped dispatch spec (F3, F2, O3)

**Verified.** When the target spec is empty, `DispatchBead` cuts the bead branch from `HEAD`.

**Fixed (R1.8):**
- Every claim must be a plan bead of its lineage spec T.
- A scoped spec or a resolved spec that differs from T refuses.
- **The dispatch spec is T**, so a claim is never cut from `HEAD` (AC-6 (c)).
- The N3 sentence is now true on every claim path (O3r2.3).

### 5 — Near-miss exit policy (F2, F3)

**Fixed (R1.2/R1.4/OQ-7).**
- *Blocking:* N1, N2, and an N3 whose parent is E and whose type is lifecycle (`phase.IsLifecycleIssueType`). These make the exit non-zero.
- *Informational:* title-only N3, metadata-only N3, and non-lifecycle children. These are printed and do not affect the exit code.
- AC-4 (v): a review-phase spec with a follow-up `bug` and a title hit exits 0. AC-4 (vi): a blocking near-miss exits non-zero.

### 6 — Binding veto versus audited repair (G2)

**Fixed (R4.3, the ADR-0041 amendment (ii)):**
- A contradictory binding vetoes automatic identification.
- Only `reattest` may replace it: with the scan-derived identity, after its checks pass, and with the prior pair audited. A refusal writes nothing.
- The two named reattest tests are preserved, and AC-12 (viii) was added.

### 7 — Smaller items

Each item is in the coverage table. In summary:
- the ADR-0045 Decision 5 transition clause, and the ADR-0044 back-pointer updated to it;
- the ADR-0040 back-pointer reworded for L3;
- AC-17 → AC-18 in the touchpoint;
- the R4.3 CLOSE-path reach, with a safety argument and AC-12 (vii);
- the waist tolerates an explicit `-n 0` (the `phase.Cache` callers; core stays untouched);
- the installed `.claude` copy described as a refresh (verified: last commit 2026-07-07, 0 `readiness_override` matches), the comment classification, and the `historical_skills/` fence;
- the 129b count is five;
- the null dependency `parent` is recorded (verified in round 1's probe output: `pr-1rs.1 … None`);
- the release-then-next legacy recovery (verified: `cmd/mindspec/release.go` removes the worktree and sets the bead open, with no branch delete);
- the unlinked-positional sentence;
- the nothing-failing scoped-flag rule;
- arms keyed on the tally, with AC-4 (iii-a)/(iii-b).

## Refutation

**BRIEF item (every slot's "Errors in this BRIEF": "TODO(skill) placeholders remain").** REFUTED.
- Command: `grep -n -i -E 'todo|<!--' BRIEF.md` shows only the two machine header markers.
- The slots resumed with round-1 memory.
- This is a BRIEF item, not a spec ask, so it is recorded here as refuted, outside the 36.

## Coverage table (36 asks)

| Ask | Group | Disposition |
|---|---|---|
| F1r2.1 | 1 | FIXED — #251 boundary in R4.2, Provenance; AC-11 (iii-neg) |
| F1r2.2 | 7 | FIXED — 129b = five, consistent everywhere |
| F1r2.3 | 7 | FIXED — ADR-0045 Decision 5 transition clause; ADR-0044 back-pointer |
| F2r2.1 | 5 | FIXED — blocking vs informational; AC-4 (v)/(vi) |
| F2r2.2 | 7 | FIXED — nothing failing: normal claim, no marker, no evidence; AC-15 leg |
| F2r2.3 | 7 | FIXED — keyed on the tally; AC-4 (iii-a)/(iii-b) |
| F2r2.4 | 1 | FIXED — AC-11 (iii) topology stated; (iii-neg) |
| F2r2.5 | 1 | FIXED — undirected-form guarantee and residual in R4.2 and ADR-0041 (i) |
| F2r2.6 | 4 | FIXED — R1.8 (c); AC-6 (c) |
| F2r2.7 | 2 | FIXED — determinism leg in AC-16 |
| F3r2.1 | 4 | FIXED — the lineage spec is the dispatch spec, never HEAD |
| F3r2.2 | 1 | FIXED — source-only for the undirected form; AC-11 (vii)/(viii); amendment and doc wording |
| F3r2.3 | 5 | FIXED — title/metadata near-misses are informational; AC-4 (v) |
| F3r2.4 | 7 | FIXED — "bd-ready but not linked" sentence; AC-3 |
| F3r2.5 | 7 | FIXED — release-then-next recovery; branch reuse; AC-17 |
| F3r2.6 | 2 | FIXED — raw records via one shared function; control-byte test |
| F3r2.7 | 7 | FIXED — null dependency `parent` recorded in Background C and R4.1 (iii) |
| O1r2.1 | 3 | FIXED — #255's bead is now claimable (R1.1); the limit stated honestly |
| O1r2.2 | 1 | FIXED — "target-pinned" dropped for the MergeInto form; MergeBranch form answered |
| O1r2.3 | 7 | FIXED — waist tolerates an explicit `-n 0`; AC-7 waist tests |
| O1r2.4 | 1 | FIXED — AC-11 (iii) |
| O1r2.5 | 7 | FIXED — refresh, not removal; comments classified; historical_skills fenced |
| O2r2.1 | 7 | FIXED — transition clause (grandfathered until step 7) |
| O2r2.2 | 6/7 | FIXED — R4.3 CLOSE reach and safety; AC-12 (vii) |
| O2r2.3 | 1 | FIXED — amendment (i) wording |
| O2r2.4 | 7 | FIXED — ADR-0040 back-pointer: L3 gains Decision 2 |
| O2r2.5 | 7 | FIXED — AC-18 |
| O2r2.6 | 1 | FIXED — AC-11 (iii) + (iii-neg) |
| O3r2.1 | 1 | FIXED — no panel-only confirmation on the fallback path; AC-11 (vii) (reproduced, then emulated) |
| O3r2.2 | 1 | FIXED — corrected; reattest caveat rendering |
| O3r2.3 | 4 | FIXED — one claim predicate in GateReadiness (R1.8 (a)) |
| G1r2.1 | 1 | FIXED — long form into bead/Y refused (iii-neg); short form bounded (viii) |
| G1r2.2 | 2 | FIXED — complete failure set; A-then-A+B AC |
| G2r2.1 | 6 | FIXED — veto vs audited repair; AC-12 (viii); tests preserved |
| G3r2.1 | 1 | FIXED — bounded reachability policy; AC-11 (viii)/(ix) in MF-3 and reattest |
| G3r2.2 | 2 | FIXED — fingerprint; MF-3 and MF-4 persistence ACs |
| (BRIEF) TODO(skill) | — | REFUTED — grep shows only the machine markers |

**Totals:** 36 asks: FIXED 36, REFUTED 0, DEFERRED 0. Separately, 1 BRIEF item is refuted.

## Errors in the round-2 dispatch brief

- (3) says "the reported bead is OUTSIDE `--parent`". On bd 1.1.0 a detached `<E>.<N>` bead is still INSIDE `bd ready --parent E`, because bd matches the ID prefix (verified). The reporter's exclusion fits an edge-only `--parent`, or a title naming `[NNN]` rather than the full spec ID. The fix is the same either way, because R1.1 no longer uses `--parent`.
- (1) says "require the surviving-tip leg or the landed binding" without addressing ambiguity on the fallback path. G3's second shape needed the added ambiguity rule.
- (7) "F3: bd show dependency `parent` key is null": confirmed. My own round-1 probe printed it, and I had not recorded it.
