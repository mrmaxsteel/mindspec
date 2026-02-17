---
adr_citations:
    - id: ADR-0001
      sections:
        - DDD Enablement
        - Domain Docs
        - Context Map
    - id: ADR-0004
      sections:
        - Go
approved_at: "2026-02-13T17:15:41Z"
approved_by: user
bead_ids: [mindspec-7q5, mindspec-yux, mindspec-95s, mindspec-6dt]
last_updated: "2026-02-13"
spec_id: 011-domain-scaffold
status: Approved
version: 1
---

# Plan: 011-domain-scaffold

## Overview

Add `mindspec domain add|list|show` subcommands to manage DDD bounded contexts as first-class CLI primitives. The domain package is new (`internal/domain/`) and the CLI wiring follows established cobra patterns.

## ADR Fitness

### ADR-0001 (Project DDD Enablement + DDD-Informed Context Packs)

**Verdict: Sound — adhere.**

This spec directly implements ADR-0001's prescribed domain doc structure (`overview.md`, `architecture.md`, `interfaces.md`, `runbook.md`) and context map governance. The `domain add` scaffolding creates exactly the artifacts ADR-0001 mandates. `domain show` surfaces the same metadata that context packs consume, reinforcing the DDD routing model. No divergence.

### ADR-0004 (Go as CLI Language)

**Verdict: Sound — adhere.**

Standard Go implementation using cobra, matching all existing command patterns. No divergence.

## Design Notes

**Reuse strategy**: The `contextpack` package already parses the context map's `## Relationships` section (`ParseContextMap()`, `ResolveNeighbors()`) and reads domain docs (`ReadDomainDocs()`). The domain package will:
- Import `contextpack.ParseContextMap()` for relationship data
- Import `contextpack.ParseSpec()` for spec scanning (finding specs that impact a domain)
- Add **new** parsing for the `## Bounded Contexts` section (extracting `### Name` + `**Owns**:` lines) — this data isn't parsed today
- Use `workspace.DomainDir()` and `workspace.ContextMapPath()` for path resolution

**Context map update strategy for `domain add`**: Read the full file, find the `---` separator between Bounded Contexts and Relationships, insert the new entry before it. If no separator found, append at end of Bounded Contexts section.

**Template approach**: Embed Go string constants for the 4 domain doc templates (no external template files). Each template has the `# <Domain> Domain — <DocType>` title pattern and section headings with placeholder content.

---

## Bead 1: `domain add` — scaffold + context map update

**Scope**: `internal/domain/scaffold.go`, `internal/domain/scaffold_test.go`

**Steps**

1. Define `nameRe = regexp.MustCompile("^[a-z][a-z0-9-]*$")` for name validation.
2. Implement `titleCase(name string) string` to convert `my-domain` → `My-Domain` (capitalize each hyphen-separated segment, preserve hyphens).
3. Implement `Add(root, name string) error`: validate name against regex (return error if invalid), check `workspace.DomainDir(root, name)` exists (return error if so), create directory via `os.MkdirAll`, write 4 template files (overview.md, architecture.md, interfaces.md, runbook.md) with title-cased domain name in headings and section stubs, call `appendContextMap()`.
4. Implement `appendContextMap(root, name string) error`: read `docs/context-map.md`, find `---` line after `## Bounded Contexts`, insert new `### <Title-Cased-Name>` entry with `**Owns**:` placeholder and domain docs link before separator, write file back.
5. Write tests: valid scaffold creates 4 files with correct titles, idempotency guard errors on existing dir, invalid name produces clear error, context map gets new entry appended.

**Verification**
- [ ] `domain.Add(root, "payments")` creates `docs/domains/payments/` with 4 template files
- [ ] Running `Add` again returns "already exists" error
- [ ] `Add(root, "123bad")` returns name validation error
- [ ] Context map contains new `### Payments` section after add
- [ ] `go test ./internal/domain/ -run TestAdd` passes

**Depends on**: None

---

## Bead 2: `domain list` — list all domains with metadata

**Scope**: `internal/domain/list.go`, `internal/domain/list_test.go`

**Steps**

1. Define `BoundedContext` struct (`Name`, `Owns string`) and `DomainEntry` struct (`Name`, `Owns`, `Relationships []string`).
2. Implement `ParseBoundedContexts(path string) ([]BoundedContext, error)`: parse `## Bounded Contexts` section from context-map.md, extract `### Name` headings and `**Owns**:` text.
3. Implement `List(root string) ([]DomainEntry, error)`: read `docs/domains/` directory entries, parse bounded contexts from context map, parse relationships via `contextpack.ParseContextMap()`, merge domain dirs with context map data (ownership + upstream/downstream labels), sort alphabetically.
4. Implement `FormatTable(entries []DomainEntry) string`: format as aligned table with Domain, Owns, Relationships columns.
5. Write tests: list with multiple domains, empty domain dir returns empty slice, missing context map is handled gracefully (returns entries with empty owns/relationships).

**Verification**
- [ ] `List()` returns entries for all directories under `docs/domains/`
- [ ] Each entry has Owns extracted from context map bounded contexts section
- [ ] Relationships column shows upstream/downstream labels
- [ ] Entries are sorted alphabetically by name
- [ ] Empty domain dir returns empty slice
- [ ] `go test ./internal/domain/ -run TestList` passes

**Depends on**: None

---

## Bead 3: `domain show` — detailed domain view + JSON

**Scope**: `internal/domain/show.go`, `internal/domain/show_test.go`

**Steps**

1. Define `DomainInfo` struct (`Name`, `Owns`, `Boundaries`, `KeyFiles string`, `Relationships []RelInfo`, `Specs []string`) and `RelInfo` struct (`Domain`, `Direction string`).
2. Implement `extractSection(content, heading string) string`: extract content under a markdown heading until next heading of same or higher level, trim whitespace.
3. Implement `Show(root, name string) (*DomainInfo, error)`: verify domain dir exists (return error if not), read `overview.md` and extract "What This Domain Owns", "Boundaries", "Key Files" sections via `extractSection()`, parse context map relationships involving this domain, scan `docs/specs/*/spec.md` using `contextpack.ParseSpec()` to find specs listing this domain in impacted domains, assemble `DomainInfo`.
4. Implement `FormatSummary(info *DomainInfo) string`: plain text output with labeled sections (Owns, Boundaries, Relationships, Key Files, Specs).
5. Implement `FormatJSON(info *DomainInfo) (string, error)`: JSON output via `json.MarshalIndent`.
6. Write tests: show existing domain returns populated info, show nonexistent domain returns error, JSON output is valid, section extraction works for various heading levels.

**Verification**
- [ ] `Show(root, "core")` returns ownership, boundaries, relationships, key files, impacting specs
- [ ] `Show(root, "nonexistent")` returns clear error
- [ ] `FormatJSON()` produces valid JSON with all fields
- [ ] `extractSection()` correctly isolates content between headings
- [ ] `go test ./internal/domain/ -run TestShow` passes

**Depends on**: None

---

## Bead 4: CLI wiring — `cmd/mindspec/domain.go`

**Scope**: `cmd/mindspec/domain.go`, `cmd/mindspec/root.go`

**Steps**

1. Create `domainCmd` parent command (`Use: "domain"`, `Short: "Manage DDD bounded contexts"`).
2. Create `domainAddCmd` (`Use: "add <name>"`, `Args: cobra.ExactArgs(1)`): call `domain.Add(root, name)`, print scaffolded path and ADR recommendation message.
3. Create `domainListCmd` (`Use: "list"`): call `domain.List(root)`, print `FormatTable()` result or "No domains found." if empty.
4. Create `domainShowCmd` (`Use: "show <name>"`, `Args: cobra.ExactArgs(1)`, `--json` flag): call `domain.Show(root, name)`, print `FormatSummary()` or `FormatJSON()` based on flag.
5. Register subcommands in `init()` and add `rootCmd.AddCommand(domainCmd)` in `root.go`.
6. Build and verify: `make build && ./bin/mindspec domain --help`.

**Verification**
- [ ] `mindspec domain add test-domain` scaffolds domain and prints success
- [ ] `mindspec domain list` shows table with all domains
- [ ] `mindspec domain show core` displays domain summary
- [ ] `mindspec domain show core --json` outputs valid JSON
- [ ] `make build` succeeds
- [ ] `make test` passes

**Depends on**: Bead 1, Bead 2, Bead 3
