---
adr_citations:
    - id: ADR-0003
      sections:
        - instruct
    - id: ADR-0012
      sections:
        - explore
    - id: ADR-0015
      sections:
        - state
approved_at: "2026-02-20T11:32:32Z"
approved_by: user
bead_ids: []
last_updated: 2026-02-20T00:00:00Z
spec_id: 041-explore-mode
status: Approved
version: 1
---

# Plan: 041 — Explore Mode

## ADR Fitness

### ADR-0003: Centralized Agent Instruction Emission
**Verdict: Conform.** Explore Mode adds a new `explore.md` template to the instruct system. The pattern is identical to existing modes — `BuildContext()` constructs context, `Render()` selects `explore.md` by mode name. No deviation needed.

### ADR-0012: Compose with External CLIs
**Verdict: Conform.** The `dismiss --adr` path calls `mindspec adr create` (or its internal equivalent `adr.Create()`). The `promote` path calls `specinit.Run()`. No new wrapper layers — direct calls at the call site.

### ADR-0015: Per-Spec Molecule-Derived Lifecycle State
**Verdict: Conform with note.** Explore Mode is pre-spec — no molecule exists yet, so mode cannot be derived from a molecule. State is tracked via `state.json` with `mode: explore`. This is consistent with ADR-0015's demotion of `state.json` to a "convenience cursor" — explore is literally a cursor-only mode since there's no spec to bind a molecule to. Once `promote` fires, normal molecule-driven state takes over.

## Testing Strategy

- **Unit tests** for each new package function (`internal/explore/`, `internal/state/`)
- **Unit tests** for the instruct template rendering with explore context
- **Integration test** via CLI: `mindspec explore "test" && mindspec explore dismiss` round-trip
- All tests runnable via `go test ./...` and `make test`

## Provenance

| Acceptance Criterion | Bead / Verification |
|---|---|
| `mindspec explore "desc"` sets state to explore and emits guidance | Bead A: test `Run()` sets state; test CLI emits instruct output |
| `mindspec instruct` in Explore Mode emits correct guidance | Bead A: test `Render()` with explore context |
| `mindspec explore dismiss` transitions to idle; `--adr` scaffolds ADR | Bead B: test `Dismiss()` state transition; test `--adr` calls `adr.Create()` |
| `mindspec explore promote <id>` runs spec-init, transitions to Spec Mode | Bead B: test `Promote()` calls `specinit.Run()` and state becomes spec |
| State validation rejects invalid transitions | Bead A: test `SetMode` rejects `explore → implement` |
| Existing `spec-init` unaffected | Bead B: existing spec-init tests continue to pass |

## Bead A: State + Instruct foundation

Add `explore` as a valid mode and create the instruct template.

**Steps**:
1. Add `ModeExplore = "explore"` to `internal/state/state.go`, update `ValidModes`
2. Update `SetMode()` validation: `explore` requires no `--spec` (it's pre-spec)
3. Create `internal/instruct/templates/explore.md` — guidance template covering: problem clarification, prior art check (`mindspec adr list`, `mindspec glossary list`, scan existing specs), feasibility assessment, alternatives, recommendation, exit paths (`dismiss`/`promote`)
4. Update `BuildContext()` in `internal/instruct/instruct.go` to handle explore mode (no spec goal to load, no plan to check)
5. Add `explore` case to `gatesForMode()` — gates: dismiss and promote
6. Write tests: state validation accepts explore, instruct renders explore template

**Verification**:
- [ ] `go test ./internal/state/` passes with explore mode validation tests
- [ ] `go test ./internal/instruct/` passes with explore template rendering test

**Depends on**: nothing

## Bead B: CLI commands (explore / dismiss / promote)

Wire up the `mindspec explore` command tree and `internal/explore/` package.

**Steps**:
1. Create `internal/explore/explore.go` with `Enter()`, `Dismiss()`, `Promote()` — each validates current state and transitions
2. Create `cmd/mindspec/explore.go` with `explore "description"`, `explore dismiss [--adr --title --domain]`, `explore promote <spec-id> [--title]`
3. `Dismiss()` with `--adr` calls `adr.Create()` directly; `Promote()` delegates to `specinit.Run()`
4. Register `exploreCmd` in `root.go`
5. Write `internal/explore/explore_test.go`: Enter/Dismiss/Promote state transitions, error on wrong mode
6. Run full suite: `make test`

**Verification**:
- [ ] `go test ./internal/explore/` passes with Enter/Dismiss/Promote tests
- [ ] `make test` passes with no regressions

**Depends on**: Bead A
