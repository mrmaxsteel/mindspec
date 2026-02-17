---
adr_citations:
    - id: ADR-0001
      sections:
        - context-map
        - domain-docs
    - id: ADR-0005
      sections:
        - state-file
approved_at: "2026-02-14T07:39:51Z"
approved_by: user
last_updated: 2026-02-14T00:00:00Z
spec_id: 015-project-bootstrap
status: Approved
version: "0.1"
work_chunks:
    - depends_on: []
      id: 1
      scope: internal/bootstrap/, cmd/mindspec/init.go, cmd/mindspec/root.go
      title: Bootstrap logic and embedded templates
      verify:
        - make build succeeds
        - mindspec init --help prints usage
    - depends_on:
        - 1
      id: 2
      scope: internal/bootstrap/*_test.go, cmd/mindspec/init_test.go
      title: Doctor integration and tests
      verify:
        - make test passes
        - mindspec init in temp dir → mindspec doctor reports zero errors
        - mindspec init run twice → all items skipped
        - mindspec init --dry-run prints plan without writing
---

# Plan: Spec 015 — mindspec init — Project Bootstrap

**Spec**: [spec.md](spec.md)

---

## ADR Fitness

| ADR | Verdict | Notes |
|-----|---------|-------|
| ADR-0001 (DDD Enablement) | **Conform** | Init creates the exact structure ADR-0001 mandates: context-map, domain dirs with 4 required files, ADR dir. No divergence. |
| ADR-0005 (Explicit State Tracking) | **Conform** | Init writes `.mindspec/state.json` with `mode: "idle"`. Follows the committed-to-git convention. No divergence. |

---

## Bead 015-A: Bootstrap logic and embedded templates

**Scope**: Core `mindspec init` command — directory creation, starter file generation, additive-only safety, dry-run flag, Beads advisory.

**Steps**:

1. Create `internal/bootstrap/bootstrap.go` with `Run(root string, dryRun bool) (*Result, error)`:
   - Define the full manifest of dirs and files to create (matching doctor expectations)
   - Walk manifest: skip existing items, create missing ones
   - Return `Result` with created/skipped lists
2. Create `internal/bootstrap/templates/` with embedded starter content (`embed.FS`):
   - `glossary.md` — minimal table with ~10 core MindSpec terms
   - `claude.md` — 3-line bootstrap pointing to `mindspec instruct`
   - `context-map.md` — placeholder with 3 bounded contexts declared
   - `state.json` — `{"mode":"idle","lastUpdated":"..."}`
   - `policies.yml` — baseline policy set
   - Domain templates: reuse from `docs/templates/domain/` (overview, architecture, interfaces, runbook) with `{{.DomainName}}` placeholder replacement
   - Spec/plan/ADR templates: copy from `docs/templates/`
3. Create `internal/bootstrap/result.go` — `Result` struct, `FormatSummary()` for human-readable output
4. Create `cmd/mindspec/init.go` — cobra command wiring:
   - `--dry-run` flag
   - Resolve workspace root (or use current dir if no root found — this IS the init)
   - Call `bootstrap.Run()`, print summary
   - Check for `bd` in PATH, print advisory if absent
5. Register `initCmd` in `cmd/mindspec/root.go`

**Verification**:
- [ ] `make build` succeeds
- [ ] `mindspec init --help` prints usage with `--dry-run` flag documented

**Depends on**: nothing

---

## Bead 015-B: Doctor integration and tests

**Scope**: Tests proving the end-to-end contract: init produces a structure that doctor validates clean. Dry-run and idempotency tests.

**Steps**:

1. Write `internal/bootstrap/bootstrap_test.go`:
   - Test `Run()` in empty temp dir → all expected dirs/files created
   - Test `Run()` in already-bootstrapped dir → all items skipped, no file content changes
   - Test `Run()` with `dryRun=true` → nothing written to disk
   - Test Beads advisory logic (mock PATH lookup)
2. Write integration test: `mindspec init` in temp dir, then `mindspec doctor` → zero errors
3. Write `cmd/mindspec/init_test.go` — cobra command flag parsing, error cases
4. Update docs: add `mindspec init` to `docs/core/USAGE.md` quick-start section
5. Verify all acceptance criteria from spec pass

**Verification**:
- [ ] `make test` passes with all new tests green
- [ ] `mindspec init` in temp dir followed by `mindspec doctor` reports zero errors
- [ ] `mindspec init` run twice in same dir → second run reports all skipped
- [ ] `mindspec init --dry-run` prints what would be created without writing

**Depends on**: 015-A

---

## Dependency Graph

```
015-A (bootstrap logic + templates)
  └── 015-B (doctor integration + tests)
```
