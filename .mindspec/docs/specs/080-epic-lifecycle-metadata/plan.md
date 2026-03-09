---
approved_at: "2026-03-09T00:46:34Z"
approved_by: user
bead_ids:
    - mindspec-gjda.1
    - mindspec-gjda.2
    - mindspec-gjda.3
last_updated: "2026-03-09"
spec_id: 080-epic-lifecycle-metadata
status: Approved
version: "1"
---
# Plan: 080-epic-lifecycle-metadata

## ADR Fitness

**ADR-0023** (Beads as Single State Authority): This ADR established child-status-based phase derivation as the primary mechanism (§3). Spec 080 modifies this: `mindspec_phase` metadata becomes the primary source, child-based derivation becomes a validation check. This is an evolution of ADR-0023 §3, not a contradiction — ADR-0023 already uses epic metadata for `spec_num`, `spec_title`, and `mindspec_done`. Adding `mindspec_phase` extends the same pattern. The derivation table in ADR-0023 §3 remains valid as the consistency check. No ADR supersession needed.

## Testing Strategy

- **Unit tests**: `internal/phase/derive_test.go` — test `DerivePhaseWithStatus()` with metadata-first logic and consistency warnings
- **Unit tests**: `internal/validate/plan_test.go` — test that missing per-bead AC is an error
- **Integration**: `make test` (full Go test suite)
- **LLM harness**: `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM -timeout 10m` — verify existing tests still pass
- **Manual validation**: `bd show <epic-id> --json` after each approval step to confirm `mindspec_phase` metadata

## Bead 1: Metadata-first phase derivation

**Steps**
1. In `internal/phase/derive.go`, modify `DerivePhaseWithStatus()` to check `metadata.mindspec_phase` first — if present and valid (one of spec/plan/implement/review/done), return it directly
2. After returning the stored phase, run `DerivePhaseFromChildren()` and compare. If they disagree, emit a warning to stderr but trust the stored phase
3. Update `hasDoneMarker()` to also check `mindspec_phase: done` as an alternative to `mindspec_done: true` (backward compat)
4. Update `DiscoverActiveSpecs()` to use the metadata-first phase when available (skip child query if metadata phase is present and not "done")
5. Add unit tests in `internal/phase/derive_test.go` covering: metadata present → returns it; metadata absent → falls back to child derivation; metadata disagrees with children → warning emitted + metadata wins
6. For epics without `mindspec_phase` (pre-080), the existing child-based fallback path is unchanged — no migration needed

**Verification**
- [ ] `go test ./internal/phase/...` passes with new metadata-first tests
- [ ] `make test` passes

**Acceptance Criteria**
- [ ] `DerivePhase()` reads `mindspec_phase` from metadata and returns it directly when present
- [ ] `DerivePhase()` emits a warning when stored phase disagrees with child-derived phase
- [ ] Epics without `mindspec_phase` metadata fall back to child-based derivation (backward compat)

**Depends on**
None

## Bead 2: Phase writes in approval commands + spec-init

**Steps**
1. In `internal/approve/spec.go`, after epic creation, write `mindspec_phase: plan` by merging into the epic's metadata (same merge pattern as impl.go lines 71-85)
2. In `internal/approve/plan.go`, after bead creation, write `mindspec_phase: implement` to the epic's metadata
3. In `internal/approve/impl.go`, replace `mindspec_done: true` with `mindspec_phase: done` (keep setting `mindspec_done: true` for backward compat during transition)
4. In `internal/specinit/specinit.go`, write `mindspec_phase: spec` — since epic doesn't exist yet at spec-init time (epic is created at spec approve), this is a no-op. Document this: spec phase is implicit (no epic = spec mode). Remove from spec if confirmed unnecessary.
5. Extract the metadata merge pattern (read existing → merge → write) into a shared helper in `internal/bead/bdcli.go` to avoid duplication across approve commands
6. Add tests verifying each approval command writes the correct phase

**Verification**
- [ ] `go test ./internal/approve/...` passes
- [ ] `go test ./internal/bead/...` passes
- [ ] `make test` passes

**Acceptance Criteria**
- [ ] `mindspec approve spec <id>` writes `mindspec_phase: plan` to the epic's metadata
- [ ] `mindspec approve plan <id>` writes `mindspec_phase: implement` to the epic's metadata
- [ ] `mindspec approve impl <id>` writes `mindspec_phase: done` to the epic's metadata

**Depends on**
None

## Bead 3: Remove auto-next + plan validation enforcement

**Steps**
1. In `cmd/mindspec/plan_cmd.go`, remove the auto-next block (lines 76-90) and the `--no-next` flag (line 33). After approval output, emit the stop guidance: "Plan approved. Implementation beads created.\n\nNext steps:\n  1. Run /clear to reset your context\n  2. Run `mindspec next` to claim your first bead"
2. Update the `planApproveCmd` Long description to remove references to auto-next and `--no-next`
3. Update `.claude/skills/ms-plan-approve/SKILL.md` step 4: change from "run `mindspec next`" to "Stop. Tell the user to run `/clear` then `mindspec next` to start fresh"
4. In `internal/validate/plan.go`, change the missing per-bead AC check (line 393) from `AddWarning` to `AddError` and remove the `!isApproved` guard — per-bead AC is a structural requirement
5. Verify plan validation catches plans without per-bead AC sections

**Verification**
- [ ] `go test ./internal/validate/...` passes — missing AC is now an error
- [ ] `make test` passes
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM -timeout 10m` — all LLM tests pass

**Acceptance Criteria**
- [ ] `mindspec approve plan <id>` does NOT auto-call `mindspec next` — outputs stop guidance instead
- [ ] `ms-plan-approve` skill tells agents to stop and output proceed instructions
- [ ] Plan validator errors (not warns) when a bead section is missing `**Acceptance Criteria**`
- [ ] All existing LLM harness tests pass

**Depends on**
None

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `approve spec` writes `mindspec_phase: plan` | Bead 2 verification (approve tests) |
| `approve plan` writes `mindspec_phase: implement` | Bead 2 verification (approve tests) |
| `approve plan` does NOT auto-next | Bead 3 verification (plan_cmd changes) |
| `approve impl` writes `mindspec_phase: done` | Bead 2 verification (approve tests) |
| `DerivePhase()` reads metadata first | Bead 1 verification (phase tests) |
| `DerivePhase()` warns on disagreement | Bead 1 verification (phase tests) |
| Backward compat for epics without metadata | Bead 1 verification (fallback tests) |
| Skill file updated | Bead 3 verification (SKILL.md change) |
| Plan validator errors on missing AC | Bead 3 verification (validate tests) |
| LLM harness tests pass | Bead 3 verification (harness test run) |
