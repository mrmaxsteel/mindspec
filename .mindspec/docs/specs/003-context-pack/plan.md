---
status: Approved
spec_id: 003-context-pack
version: "1.0"
last_updated: 2026-02-12
approved_at: 2026-02-12T00:00:00Z
approved_by: user
bead_ids:
  - mindspec-1ol   # 003-A: Spec Parser + Domain Doc Reader
  - mindspec-0ke   # 003-B: Context Map Parser + 1-Hop Resolution
  - mindspec-91s   # 003-C: ADR Scanner + Policy Reader
  - mindspec-kns   # 003-D: Assembler + Provenance + Writer
  - mindspec-9d8   # 003-E: CLI Wiring + Integration + Doc-sync
adr_citations:
  - id: ADR-0001
    sections: ["Decision Details C"]
  - id: ADR-0002
    sections: ["Decision Details A"]
---

# Plan: Spec 003 — Context Pack Generation

**Spec**: [spec.md](spec.md)

---

## Bead 003-A: Spec Parser + Domain Doc Reader

**Scope**: Parse `docs/specs/<id>/spec.md` to extract impacted domains and goal. Read domain doc files. Add workspace path helpers.

**Steps**:
1. Add 5 path helpers to `internal/workspace/workspace.go`
2. Create `spec.go`: read file, extract `## Goal` section text, extract `## Impacted Domains` bullets
3. Create `domaindoc.go`: read 4 files from `docs/domains/<domain>/`, missing files = empty string
4. Write tests with `t.TempDir()` fixtures

**Verification**:
- [ ] `go test ./internal/workspace/... ./internal/contextpack/...` passes

**Depends on**: nothing

---

## Bead 003-B: Context Map Parser + 1-Hop Resolution

**Scope**: Parse `docs/context-map.md` relationships. Given impacted domains, return 1-hop neighbor domain names.

**Steps**:
1. Parse `## Relationships` section, match `### X → Y (direction)` headings
2. Extract `**Contract**:` markdown link paths
3. `ResolveNeighbors()`: collect other side of relationships, deduplicate
4. Normalize domain names to lowercase

**Verification**:
- [ ] `go test ./internal/contextpack/...` passes
- [ ] Parsing project's context-map.md yields 4 relationships

**Depends on**: nothing

---

## Bead 003-C: ADR Scanner + Policy Reader

**Scope**: Scan `docs/adr/ADR-*.md` for accepted ADRs by domain. Parse `policies.yml` and filter by mode.

**Steps**:
1. `ScanADRs()`: glob ADR files, parse Status and Domain(s) lines
2. `FilterADRs()`: keep where Status=Accepted and domains overlap
3. Add `gopkg.in/yaml.v3` dependency
4. `ParsePolicies()`: unmarshal YAML policies list
5. `FilterPolicies()`: keep where mode matches or mode is empty

**Verification**:
- [ ] `go test ./internal/contextpack/...` passes

**Depends on**: nothing

---

## Bead 003-D: Assembler + Provenance + Writer

**Scope**: Combine all parsers. Apply mode content tiers. Generate provenance. Write `context-pack.md`.

**Steps**:
1. `Build()` orchestrates all parsers
2. Mode selection logic (progressive tiers)
3. Build provenance entries
4. `gitCommitSHA()` helper
5. `Render()` markdown output
6. `WriteToFile()` writer

**Verification**:
- [ ] `go test ./internal/contextpack/...` passes
- [ ] Build with mode=spec excludes architecture.md; mode=plan includes it

**Depends on**: 003-A, 003-B, 003-C

---

## Bead 003-E: CLI Wiring + Integration + Doc-sync

**Scope**: Wire Cobra commands. End-to-end validation. Update domain docs.

**Steps**:
1. Create `cmd/mindspec/context.go` with parent + child commands
2. Register in `root.go`
3. Update domain docs
4. End-to-end testing

**Verification**:
- [ ] `make build` succeeds
- [ ] `./bin/mindspec context pack 001-skeleton` generates context-pack.md
- [ ] `./bin/mindspec context pack nonexistent` fails gracefully
- [ ] `go test ./...` passes
- [ ] `./bin/mindspec doctor` passes

**Depends on**: 003-D

---

## Dependency Graph

```
003-A (Spec Parser + Domain Doc Reader)  ─┐
003-B (Context Map Parser + 1-Hop)       ─┼──▶ 003-D (Assembler) ──▶ 003-E (CLI + Integration)
003-C (ADR Scanner + Policy Reader)      ─┘
```
