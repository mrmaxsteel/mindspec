---
adr_citations:
    - id: ADR-0002
      sections:
        - Beads as passive tracking substrate
    - id: ADR-0003
      sections:
        - Centralized instruction emission
approved_at: "2026-02-13T12:41:03Z"
approved_by: user
last_updated: 2026-02-13T00:00:00Z
spec_id: 008c-prime-compose
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: internal/instruct/prime.go, internal/instruct/instruct.go, internal/instruct/instruct_test.go
      title: Capture bd prime and compose into Render()
      verify:
        - '`CapturePrime()` shells out to `bd prime` and returns output'
        - '`CapturePrime()` returns empty string + no error when `bd` is unavailable'
        - '`Context.BeadsContext` is populated by `BuildContext()`'
        - '`Render()` appends Beads context section after mode guidance, before warnings'
        - '`RenderJSON()` includes `beads_context` field in JSON output'
        - '`make test` passes'
    - depends_on:
        - 1
      id: 2
      scope: CLAUDE.md, docs/core/CONVENTIONS.md
      title: Doc-sync + hook documentation
      verify:
        - CLAUDE.md documents that `mindspec instruct` includes Beads context
        - CONVENTIONS.md notes that `bd prime` hook is subsumed by `mindspec instruct`
---

# Plan: Spec 008c — Compose `bd prime` into `mindspec instruct`

**Spec**: [spec.md](spec.md)

---

## Bead 008c-1: Capture `bd prime` and compose into `Render()`

**Scope**: `internal/instruct/prime.go` (new), `internal/instruct/instruct.go`, `internal/instruct/instruct_test.go`

**Steps**:
1. Create `internal/instruct/prime.go` with `CapturePrime() string` — runs `bd prime` via `exec.Command`, captures stdout, returns output. On any error (not found, exit error, timeout), returns empty string (graceful degradation). Uses a package-level `var execPrime` for test overrides.
2. Add `BeadsContext string` field to `Context` struct
3. In `BuildContext()`, call `CapturePrime()` and assign to `ctx.BeadsContext`. If empty, append a warning: `[beads] bd prime unavailable — Beads workflow context not included`
4. In `Render()`, after rendering the mode template and before appending warnings, append a `---\n\n` separator followed by `ctx.BeadsContext` (if non-empty)
5. In `RenderJSON()`, add `BeadsContext string` field to `JSONOutput` and populate it
6. Write tests: `TestCapturePrime_Available` (mock exec returns content), `TestCapturePrime_Unavailable` (mock exec fails), `TestRender_IncludesBeadsContext` (verify Beads section in output), `TestRenderJSON_IncludesBeadsContext` (verify JSON field)

**Verification**:
- [ ] `CapturePrime()` returns `bd prime` output
- [ ] `CapturePrime()` returns empty on failure (no error propagated)
- [ ] `Render()` includes Beads context in output
- [ ] `RenderJSON()` includes `beads_context` field
- [ ] Warning emitted when `bd prime` unavailable

**Depends on**: nothing

---

## Bead 008c-2: Doc-sync + hook documentation

**Scope**: `CLAUDE.md`, `docs/core/CONVENTIONS.md`

**Steps**:
1. Update `CLAUDE.md` Build & Run section: note that `mindspec instruct` now includes Beads workflow context
2. Update `docs/core/CONVENTIONS.md` instruct-tail section: note that `mindspec instruct` composes `bd prime` output, making a separate `bd prime` SessionStart hook unnecessary
3. Add a note about the recommended hook configuration (single `mindspec instruct` hook)

**Verification**:
- [ ] CLAUDE.md documents unified context
- [ ] CONVENTIONS.md documents hook consolidation

**Depends on**: 008c-1

---

## Dependency Graph

```
008c-1 (capture + compose)
  └── 008c-2 (doc-sync)
```

Linear: implementation first, then documentation.
