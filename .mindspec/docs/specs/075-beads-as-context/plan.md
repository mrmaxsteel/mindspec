---
adr_citations:
    - id: ADR-0023
      sections:
        - Decision
approved_at: "2026-03-08T16:37:35Z"
approved_by: user
bead_ids:
    - mindspec-9d0q.1
    - mindspec-9d0q.2
    - mindspec-9d0q.3
spec_id: 075-beads-as-context
status: Approved
version: "1"
---
# Plan: 075-beads-as-context

## ADR Fitness

**ADR-0023 (Beads as Single State Authority)**: Sound. This spec extends beads-as-authority to architectural decisions. The `Store` interface is the abstraction that lets enforcement code access decisions without caring whether they live in files or beads.

**ADR-0024 (ADR Storage Abstraction)**: Sound — this spec is the direct implementation of ADR-0024's decision. Interface-first, file-based default, consumers migrated to the interface. (ADR-0024 lives on this branch; citation omitted from frontmatter to avoid cross-worktree validation failure.)

## Testing Strategy

Unit tests in `internal/adr/store_test.go` covering:
- `FileStore` implementing the `Store` interface (List, Get, Search)
- Filter behavior (by status, domain)
- Edge cases (missing files, empty directory)

Existing consumer tests (`internal/validate/plan_test.go`, `internal/contextpack/...`, `internal/approve/plan_test.go`) must continue to pass with the refactored code — this validates behavioral parity.

No new integration tests needed — the refactoring preserves existing behavior behind a new interface.

## Bead 1: Store Interface and FileStore Implementation

**Steps**
1. Define `Store` interface in `internal/adr/store.go` with methods: `List(opts ListOpts) ([]ADR, error)`, `Get(id string) (*ADR, error)`, `Search(query string) ([]ADR, error)`, `Create(title string, opts CreateOpts) (string, error)`, `Supersede(oldID, newID string) error`
2. Implement `FileStore` struct in `internal/adr/filestore.go` wrapping existing functions: `ScanADRs`, `Show`, `List`, `Create`, `Supersede` — delegating to the existing implementations with the `root` path stored on the struct
3. Add `NewFileStore(root string) *FileStore` constructor
4. Write unit tests in `internal/adr/store_test.go` verifying `FileStore` satisfies `Store` via interface assignment and testing List/Get/Search behavior

**Verification**
- [ ] `go test ./internal/adr/...` passes with new store tests
- [ ] `FileStore` satisfies `Store` interface (compile-time check via `var _ Store = (*FileStore)(nil)`)

**Depends on**
None

## Bead 2: Migrate Consumers to Store Interface

**Steps**
1. Refactor `internal/validate/plan.go` `checkADRCitations()` to accept a `Store` parameter instead of `root string`, replacing `adr.ParseADR(path)` with `store.Get(id)`
2. Refactor `internal/approve/plan.go` `buildDesignField()` to accept a `Store` parameter instead of calling `adr.ScanADRs(root)` directly
3. Refactor `internal/contextpack/adr.go` to use the `Store` interface instead of re-exporting `ScanADRs`/`FilterADRs` — replace type alias and function vars with a store-based approach
4. Update `cmd/mindspec/adr.go` CLI subcommands (`adr list`, `adr show`, `adr create`) to construct a `FileStore` and delegate to it
5. Update all call sites that pass `root` to validation/approval to also pass or construct the store
6. Verify no consumer imports `internal/adr` for direct file access (only through Store)

**Verification**
- [ ] `go test ./internal/validate/...` passes (existing ADR citation tests)
- [ ] `go test ./internal/approve/...` passes (existing plan approval tests)
- [ ] `go test ./internal/contextpack/...` passes
- [ ] `go test ./...` passes (full suite, no regressions)

**Depends on**
Bead 1

## Bead 3: Mock Store and Swappability Verification

**Steps**
1. Create a `MemoryStore` test helper in `internal/adr/store_test.go` that implements `Store` with in-memory ADR data (no filesystem)
2. Write a test that uses `MemoryStore` to drive validation logic (`checkADRCitations` with a mock store), proving consumers are decoupled from the filesystem
3. Verify `MemoryStore` can be used as a drop-in replacement for `FileStore` in any consumer

**Verification**
- [ ] `go test ./internal/adr/ -run TestMemoryStore` passes
- [ ] `go test ./internal/validate/ -run TestCheckADRCitations_MockStore` passes (if consumer accepts Store)

**Depends on**
Bead 2

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| `Store` interface exists with List, Get, Search methods | Bead 1 (compile-time check) |
| `Decision` type captures ID, title, status, domains, content, supersede links | Bead 1 (existing `ADR` struct already has these) |
| File-based implementation passes all existing ADR tests | Bead 1 (`go test ./internal/adr/...`) |
| `mindspec adr list` works unchanged | Bead 2 (CLI delegates to FileStore) |
| `mindspec adr show <id>` works unchanged | Bead 2 (CLI delegates to FileStore) |
| `mindspec adr create` works unchanged | Bead 2 (CLI delegates to FileStore) |
| ADR markdown files remain source of truth | Bead 1 (FileStore reads from `.mindspec/docs/adr/`) |
| contextpack uses interface, no direct os.ReadFile | Bead 2 verification |
| validate uses interface for ADR citation checks | Bead 2 verification |
| approve uses interface for ADR lookups | Bead 2 verification |
| All existing tests pass with refactored code | Bead 2 (`go test ./...`) |
| Mock store can be swapped without changing consumer code | Bead 3 (MemoryStore test) |
