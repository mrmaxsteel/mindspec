# spec-115-approve round-4 consolidated findings (revision brief for round 5)

Round-4 spec-approval panel @ `4cbf437b`: **8 APPROVE / 1 REQUEST_CHANGES**, no REJECT.
- APPROVE: G1 codex (0.99), G3 codex (0.99), F1 fable (0.95), F2 fable (0.93), F3 fable (0.95), O1 opus (0.95), O2 opus (0.95), O3 opus (0.95).
- REQUEST_CHANGES: **G2 codex (0.98)** — the branch-existence probe's exit-1 discriminator is not robust.

**8/9 clears the ≥8 threshold by count, but a CONFIRMED finding is NEVER out-voted** ([[findings-never-outvoted]]). The orchestrator EMPIRICALLY REPRODUCED G2's finding (evidence below), so it must be fixed, not out-voted. This is a bounded proof/robustness fix — no design change; the REFUSE core + all other ACs are validated across 4 rounds.

## THE FINDING [G2 0.98] — orchestrator-confirmed
Round-4 spec grounds `BranchExistsE`'s "genuinely absent" outcome on `git rev-parse --verify --quiet` exit **1** (via the `RevParseRef`/`ErrRefNotFound` precedent). G2: exit 1 does NOT uniquely prove absence.

**Orchestrator empirical reproduction** (throwaway repo, existing loose branch `bead/loose`, `.git/refs/heads` made unreadable via `chmod 000`):
| probe | genuinely absent | existing but store UNREADABLE |
|:------|:-----------------|:------------------------------|
| `rev-parse --verify --quiet` | exit 1 | **exit 1** (masks infra as absent — the bug) |
| `git branch --list 'bead/*'` | (n/a) | exit 128 (loud, but resolves HEAD incidentally) |
| `for-each-ref refs/heads/bead/` | exit 0 empty | **exit 0 empty** (silently drops — worse) |
| `git show-ref --verify` (NO `--quiet`) | **exit 1** | **exit 128** (loud — distinguishes!) |
| `git show-ref --verify --quiet` | exit 1 | exit 1 (masks — `--quiet` collapses it) |

So under the round-4 mapping (exit-1 → `(false,nil)`), an inaccessible loose-ref store → `BranchExistsE` returns `(false,nil)` → Scan sees no orphan → gate passes on a structural ref-store failure = residual fail-open (the original o4fd leg).

**The robust primitive (evidence-grounded): `git show-ref --verify` WITHOUT `--quiet`.** It is the ONLY probe tested that gives a clean three-way by exit code alone: **0 = present, 1 = genuinely absent, any other (128) = infra error**. Every `--quiet` variant and `rev-parse` masks the inaccessible case as exit 1; `for-each-ref` silently returns empty (no error at all).

**Exploitability is near-nil (defense-in-depth already present).** `ApproveImpl` calls `exec.MergeBase("main", specBranch)` at `impl.go:249` — which resolves `specBranch` through the SAME ref store — BEFORE the orphan-scan gate (which sits after the phase gate, before `:327`). An unreadable `refs/heads` makes that `MergeBase` fail (exit 128) and ApproveImpl aborts at `:249`, fail-closed, before the scan is ever reached; `exec.FinalizeEpic` at `:372` would also fail in that state. The exit-1 ambiguity is only reachable via a transient fault windowed to open AFTER `:249`, cover ONLY the scan's branch probe, and close again before `:372` — all within one synchronous single-process pass. Not attacker-controllable, physically implausible. But the fix below closes even that tail definitively.

## THE FIX (round 5)
1. **Change `BranchExistsE`'s specified contract to a fail-closed-on-ambiguity, exit-code-robust primitive.** Specify it via `git show-ref --verify refs/heads/<name>` (NO `--quiet`): **exit 0 → `(true, nil)`; exit 1 → `(false, nil)` (genuinely absent — no false-refusal); any other exit / spawn failure → `(false, non-nil error)` → the gate REFUSES.** Replace the `rev-parse --verify --quiet` / exit-1-means-absent grounding in R1's branch-probe leg + OQ2 + the Scope `internal/gitutil` bullet with this show-ref-based three-way, and state the GENERAL PRINCIPLE: *any ref-probe outcome that is not an unambiguous present (0) or unambiguous absent (1) fails closed* — so no `--quiet`-masked or silently-empty enumeration is used on the gate path. Cite the orchestrator evidence (show-ref no-quiet gives 0/1/128; the `--quiet` and `for-each-ref` variants mask/empty).
2. **Add the defense-in-depth note** (strengthens, not load-bearing): document that `ApproveImpl`'s `exec.MergeBase` at `impl.go:249` already fails closed on an unreadable ref store before the orphan scan — so the show-ref hardening closes the only (physically-implausible) transient-race tail, not a routinely-reachable hole.
3. **Extend the test surface** (G2's ask): extend `TestBranchExistsE_ThreeWay` (or an exact-named companion, e.g. `TestBranchExistsE_UnreadableRefStore`) with an EXISTING loose branch whose loose-ref store is unreadable → assert `BranchExistsE` returns a non-nil ERROR (not `(false,nil)`); and pin the `ApproveImpl` gate REFUSAL for that outcome (fold into AC1(b)'s infra-error leg — the branch-probe infra sub-case now includes the unreadable-store variant, not just a generic seam error). Keep the discriminators RED at the new SHA.
4. Update R1's Falsified-if: "a branch-probe INFRA error (incl. an unreadable/corrupt ref store returning a non-1 exit) that yields `(false,nil)` instead of an error → falsified."

## Disposition
Fix per the show-ref-no-quiet three-way + the unreadable-store test + the defense-in-depth note. No design change; grounded entirely in existing gitutil primitives + orchestrator-reproduced evidence. Re-panel round 5 (9-slot, ≥8). This closes the branch-probe leg definitively by exit-code construction (0/1/else), not by chasing a deeper `--quiet` edge — the anti-spiral move.
