# spec-115-bead1 — Round 1 consolidated tally

**Reviewed**: `bead/mindspec-fgmg.1` @ `75e82b52`. **Panel**: 8 slots (R1–R3 Opus, R4–R6 Sonnet, R7 Fable, R8 codex). **Threshold**: 8/8 UNANIMOUS. Findings never out-voted.

## Verdicts

| Slot | Family | Lens | Verdict | Conf |
|------|--------|------|---------|------|
| R1 | Opus | author-of-record | APPROVE | 0.95 |
| R2 | Opus | fail-open/fail-closed | APPROVE | high |
| R3 | Opus | RED-on-revert | APPROVE | high |
| R4 | Sonnet | empirical prober | APPROVE | high |
| R5 | Sonnet | seam/type | APPROVE | high |
| R6 | Sonnet | no-regression | APPROVE | high |
| R7 | Fable | scope/grounding | APPROVE | high |
| R8 | codex | adversarial/integration | **REQUEST_CHANGES** | 0.98 |

**Aggregate: 7 APPROVE / 1 REQUEST_CHANGES.** Does NOT meet 8/8 → fix + re-panel round 2.

## The finding (R8, confirmed by orchestrator code read — uncontested)

**Present-but-JSON-null `refutation_pending_entries` slips the fail-closed gate.** In `internal/complete/panel_advisory.go` `uncoveredPendingObligations` (`:518`):
```go
pending, decErr := decodePendingEntries(meta["refutation_pending_entries"])
```
`meta["refutation_pending_entries"]` yields nil for BOTH an absent key AND a present key whose JSON value is `null`. `decodePendingEntries(nil)` returns `(empty, nil)`, so a **present-but-null** obligation record is treated exactly like an **absent** key — a no-op that passes the gate. This contradicts the plan's R3 fail-closed contract ("a present-but-corrupt `refutation_pending_entries`/`panel_refuted_entries` value … REFUSES — never decodes-as-empty"). Realizable: bead metadata is JSON; `{"refutation_pending_entries": null}` unmarshals to a present key with nil value (R8 verified). The 9 existing `TestPendingObligationPredicate` corrupt-cases use present non-null malformed values (which DO error via `decodePendingEntries`); none exercises the present-null edge, which is why R1–R7's "fail-closed on corrupt" confirmations did not catch it.

## Fix (consolidated → one Sonnet fix round → re-panel round 2)

1. In `uncoveredPendingObligations`, distinguish **absent** from **present-null** via the comma-ok idiom for `refutation_pending_entries`: a present key with a nil value → return an error (fail-closed, "present but null / corrupt"); an ABSENT key → preserve the no-obligation no-op. Apply the same present-null → error treatment to `panel_refuted_entries` for contract consistency (present-null there is already fail-safe in direction — empty coverage → refuse — but the fail-closed contract says present-but-corrupt → error, so make it explicit and symmetric).
2. Add subtest(s) to `TestPendingObligationPredicate` proving: present-null `refutation_pending_entries` → error (RED-on-revert: fails if the comma-ok guard is reverted); absent-key → nil no-op still preserved; and (if handled) present-null `panel_refuted_entries` → error.
3. Re-run the full gate set (build, `go test -count=1` the 3 pkgs + the named tests, fences, `git diff main -- internal/gitutil/` empty, gofmt/vet/golangci-lint) and confirm `reconcilePendingRefutations`'s existing suite still passes (the shared core change must not alter reconcile's behavior on valid inputs). One fix commit; clean tree.

No other findings. Everything else (build/tests/vet/gofmt/lint/AC8/fences, all four exports + signatures, acyclicity, byte-identity, shared-core structure, RED-on-revert, AC12(a) deviation, scope, grounding) was independently confirmed sound by multiple slots.
