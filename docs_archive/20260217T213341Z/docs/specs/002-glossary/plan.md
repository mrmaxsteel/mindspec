---
status: Approved
spec_id: 002-glossary
version: "1.0"
last_updated: 2026-02-11
approved_at: 2026-02-11
approved_by: user
bead_ids:
  - mindspec-97r   # 002-A: Glossary parsing package
  - mindspec-epu   # 002-B: Term matching
  - mindspec-mgh   # 002-C: Section extraction
  - mindspec-qax   # 002-D: CLI commands + wiring
  - mindspec-on2   # 002-E: Doc-sync + refactor doctor
adr_citations:
  - id: ADR-0001
    sections: ["Glossary as required primitive for deterministic context injection"]
  - id: ADR-0002
    sections: ["Glossary targets point to documentation system; doctor validates links"]
---

# Plan: Spec 002 — Glossary-Based Context Injection

**Spec**: [spec.md](spec.md)

## Context

MindSpec's context-system domain needs glossary parsing and matching to power deterministic context injection. The Go CLI (Spec 001) already has glossary regex patterns in `internal/doctor/docs.go` for health checks — but no reusable glossary data model or matching logic. This plan builds a new `internal/glossary/` package with parsing, matching, and section extraction, then wires it into the CLI as `mindspec glossary {list,match,show}`.

## Design Decisions

| Decision | Choice | Rationale |
|:---------|:-------|:----------|
| Package location | `internal/glossary/` | Context-system domain; separate from doctor (which is core domain) |
| Reuse regex patterns | Duplicate in glossary package | Doctor uses them for validation only; glossary needs them for full parsing. Keep each package self-contained. |
| Term matching | Case-insensitive substring, longest-match-first | Matches spec requirement; simple and deterministic |
| Section extraction | Read markdown, find heading matching anchor, return content until next same-or-higher-level heading | Standard markdown section extraction |
| Cobra wiring | `glossary` parent command with `list`, `match`, `show` subcommands | Matches spec acceptance criteria |

---

## Bead 002-A: Glossary parsing package

**Scope**: Create `internal/glossary/` with `Entry` type and `Parse(root string)` function that reads GLOSSARY.md and returns structured entries.

**Steps**:
1. Create `internal/glossary/glossary.go` with `Entry` struct and `Parse(root string) ([]Entry, error)`
2. Create `internal/glossary/glossary_test.go` with tests for well-formed glossary, missing file, anchor vs no-anchor, and FilePath/Anchor splitting

**Verification**:
- [ ] `go test ./internal/glossary/...` passes
- [ ] Parses all 18 entries from the real GLOSSARY.md
- [ ] Correctly splits `docs/core/ARCHITECTURE.md#beads` into FilePath and Anchor

**Depends on**: nothing

---

## Bead 002-B: Term matching

**Scope**: Implement `Match(entries []Entry, text string) []Entry` — case-insensitive matching of glossary terms against input text, longest-match-first.

**Steps**:
1. Add `Match()` to `internal/glossary/match.go` with longest-match-first, case-insensitive substring search
2. Write tests in `internal/glossary/match_test.go`

**Verification**:
- [ ] `Match(entries, "spec mode approval")` returns "Spec Mode" entry
- [ ] `Match(entries, "context pack and bead")` returns both "Context Pack" and "Bead"
- [ ] Case-insensitive: "CONTEXT PACK" matches "Context Pack" entry
- [ ] `go test ./internal/glossary/...` passes

**Depends on**: 002-A

---

## Bead 002-C: Section extraction

**Scope**: Implement `ExtractSection(root, filePath, anchor string) (string, error)` — reads a markdown file and extracts the section matching the given anchor.

**Steps**:
1. Add `ExtractSection()` to `internal/glossary/section.go` with heading-level-aware extraction
2. Write tests in `internal/glossary/section_test.go`

**Verification**:
- [ ] Extracts `## Beads` section from a file with multiple `##` sections
- [ ] Stops at next same-level heading
- [ ] Returns error with message when anchor not found
- [ ] `go test ./internal/glossary/...` passes

**Depends on**: 002-A

---

## Bead 002-D: CLI commands + wiring

**Scope**: Wire `mindspec glossary {list, match, show}` cobra commands.

**Steps**:
1. Create `cmd/mindspec/glossary.go` with `glossaryCmd`, `glossaryListCmd`, `glossaryMatchCmd`, `glossaryShowCmd`
2. Register `glossaryCmd` in `cmd/mindspec/root.go`
3. Handle errors with actionable messages

**Verification**:
- [ ] `mindspec glossary list` displays all 18 terms and targets
- [ ] `mindspec glossary match "spec mode approval"` returns matching terms
- [ ] `mindspec glossary show "Context Pack"` displays the linked doc section
- [ ] Invalid anchor reports actionable error
- [ ] `mindspec --help` shows `glossary` command

**Depends on**: 002-B, 002-C

---

## Bead 002-E: Doc-sync + refactor doctor

**Scope**: Update doctor to use the glossary package for term counting, update context-system domain docs to Go signatures, update runbook.

**Steps**:
1. Refactor `internal/doctor/docs.go` `checkGlossary()` to use `glossary.Parse()`
2. Update `docs/domains/context-system/interfaces.md` — Python → Go signatures
3. Update `docs/domains/context-system/runbook.md` — Python → Go CLI commands
4. Verify `mindspec doctor` still passes and `go test ./...` passes

**Verification**:
- [ ] Doctor still reports correct term count and broken links
- [ ] `mindspec doctor` exits 0
- [ ] Context-system domain docs reference Go interfaces
- [ ] Runbook uses Go CLI commands
- [ ] `go test ./...` passes

**Depends on**: 002-D

---

## Dependency Graph

```
002-A  (glossary parsing)
  ├── 002-B  (term matching)
  │     └── 002-D  (CLI commands)
  └── 002-C  (section extraction)
        └── 002-D  (CLI commands)
              └── 002-E  (doc-sync + doctor refactor)
```

---

## End-to-End Verification

```bash
make build
./bin/mindspec glossary list
./bin/mindspec glossary match "spec mode approval"
./bin/mindspec glossary show "Context Pack"
./bin/mindspec glossary show "Nonexistent"
./bin/mindspec doctor
go test ./...
```
