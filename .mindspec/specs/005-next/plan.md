---
status: Approved
spec_id: 005-next
version: "1.0"
last_updated: 2026-02-12
approved_at: 2026-02-12
approved_by: user
bead_ids: [mindspec-5yd, mindspec-ja3]
adr_citations:
  - id: ADR-0003
    sections: ["CLI Contract — mindspec next"]
  - id: ADR-0002
    sections: ["Beads as passive substrate"]
  - id: ADR-0005
    sections: ["State File Schema", "Write Surface"]
---

# Plan: Spec 005 — Work Selection + Claiming (`mindspec next`)

**Spec**: [spec.md](spec.md)

---

## Design Notes

### Beads JSON Interface

Both `bd ready --json` and `bd show <id> --json` return structured JSON arrays:

```json
[{
  "id": "mindspec-25p",
  "title": "Test bead for parsing",
  "status": "open",
  "priority": 4,
  "issue_type": "task",
  "owner": "max@enubiq.com",
  "created_at": "2026-02-12T08:50:30Z",
  "updated_at": "2026-02-12T08:50:30Z"
}]
```

Key field: `issue_type` distinguishes `task` (implementation bead) from `feature` (spec bead).

### Mode Mapping

| Beads `issue_type` | MindSpec mode |
|:--------------------|:-------------|
| `feature` | `spec` or `plan` (check if spec is approved) |
| `task` | `implement` |
| `bug` | `implement` |

For `feature` type: if the spec is `APPROVED` and has a plan, mode is `plan`; if spec is `DRAFT`, mode is `spec`.

### Claiming

Claim via `bd update <id> --status=in_progress`. This is idempotent — claiming an already in_progress bead is a no-op.

### Spec ID Resolution

The bead title or description should reference the spec ID. Convention: bead titles include the spec slug (e.g., "005-next: Implement work selection"). Parse the spec ID from the title prefix before the first colon, or fall back to asking the user.

### Git Clean Tree Check

Run `git status --porcelain` — empty output means clean. If non-empty, refuse to claim and emit recovery guidance.

---

## Bead 005-A: Beads query + selection logic

**Scope**: `internal/next/` package — query ready work, parse JSON, present choices, select item

**Steps**:
1. Create `internal/next/beads.go`: `BeadInfo` struct matching the JSON schema, `QueryReady()` function that shells out to `bd ready --json` and parses the result
2. Create `internal/next/select.go`: `SelectWork(items []BeadInfo)` — if one item, return it (auto-claim); if multiple, print numbered list to stdout and return the first (CLI selection is deferred to the command layer)
3. Create `internal/next/git.go`: `CheckCleanTree()` — runs `git status --porcelain`, returns error if dirty with a message listing the dirty state
4. Create `internal/next/mode.go`: `ResolveMode(root string, bead BeadInfo)` — maps bead type + artifact state to MindSpec mode. Returns mode string and spec ID
5. Write tests: mock JSON parsing, mode resolution for each type, clean/dirty tree detection

**Verification**:
- [ ] `QueryReady()` correctly parses `bd ready --json` output into `[]BeadInfo`
- [ ] `SelectWork()` auto-returns single item, returns list for multiple
- [ ] `CheckCleanTree()` returns nil for clean tree, error with details for dirty
- [ ] `ResolveMode()` maps task→implement, feature→spec/plan based on artifact state
- [ ] `make test` passes with next package tests

**Depends on**: nothing

---

## Bead 005-B: CLI command + state/instruct integration

**Scope**: `cmd/mindspec/next.go` replacing the stub — wires query, selection, claiming, state write, and guidance emission

**Steps**:
1. Create `cmd/mindspec/next.go`: full command implementation:
   - Check clean tree (`next.CheckCleanTree()`)
   - Query ready work (`next.QueryReady()`)
   - Handle no-work case (print message, suggest next steps)
   - Select item (auto-claim single, list for multiple with `--pick` flag or default to first)
   - Claim via `bd update <id> --status=in_progress`
   - Resolve mode and spec ID (`next.ResolveMode()`)
   - Write state (`state.SetMode()`)
   - Emit guidance (`instruct.BuildContext()` + `instruct.Render()`)
2. Remove `nextCmd` from `cmd/mindspec/stubs.go`
3. Register the new command in `root.go` (already registered, just needs the stub replaced)
4. Write integration-style tests covering the full flow: no-work, single-item, dirty-tree paths

**Verification**:
- [ ] `./bin/mindspec next` with ready work claims it, updates state, emits guidance
- [ ] `./bin/mindspec next` with no ready work reports clearly and suggests next steps
- [ ] `./bin/mindspec next` with dirty tree warns and refuses to claim
- [ ] `./bin/mindspec next` when `bd` is unavailable fails gracefully
- [ ] `./bin/mindspec state show` confirms state updated after claim
- [ ] `make test` passes with all tests
- [ ] Existing commands unaffected

**Depends on**: 005-A

---

## Dependency Graph

```
005-A (beads query + selection logic)
  └── 005-B (CLI command + state/instruct integration)
```
