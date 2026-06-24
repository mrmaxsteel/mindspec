# Spec 011-domain-scaffold: Domain Scaffold + Context Map

## Goal

Make DDD bounded contexts a first-class CLI primitive — scaffolding new domains for structural consistency, and querying them for token-efficient agent context. Domains are the routing key for context pack assembly (ADR-0001); this spec gives them proper tooling.

## Background

ADR-0001 mandates a standard domain doc structure under `docs/domains/<domain>/` (overview, architecture, interfaces, runbook). Three domains (core, context-system, workflow) were created manually and are fully populated. The context map at `docs/context-map.md` declares bounded contexts, their ownership, and integration relationships.

Domains serve two roles in MindSpec:
1. **Structural**: consistent documentation boundaries per bounded context
2. **Operational**: the context pack builder uses impacted domains to route what gets included — domain docs, neighbor interfaces, and relevant ADRs. This is the core mechanism for keeping agent context concise and relevant.

Today there is no CLI surface for domains: adding one requires manual file creation, and there's no quick way to query "what does this domain own?" or "how do domains relate?" without reading multiple files.

## Impacted Domains

- **core**: New CLI subcommands (`domain add`, `domain list`, `domain show`) register under the root command; `workspace.DomainDir()` already exists
- **context-system**: `domain show` surfaces the same domain metadata that context packs consume, reinforcing the DDD routing model

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): Defines the required domain doc structure, context map governance, and DDD-informed assembly rules
- [ADR-0004](../../adr/ADR-0004.md): Go as implementation language

## Requirements

### `mindspec domain add <name>`

1. Scaffolds a new domain directory at `docs/domains/<name>/` containing four template files:
   - `overview.md` — section headings: "What This Domain Owns", "Boundaries", "Key Files", "Current State"
   - `architecture.md` — section headings: "Key Patterns", "Invariants", "Design Decisions"
   - `interfaces.md` — section headings: "Contracts", "Integration Points"
   - `runbook.md` — section headings: "Development Workflows", "Debugging", "Common Tasks"
   Each file has a `# <Domain> Domain — <DocType>` title and placeholder content under each heading.

2. Appends a new bounded context entry to `docs/context-map.md` under `## Bounded Contexts` with the domain name, a placeholder **Owns** line, and a link to the domain's overview doc.

3. Prints a message recommending an ADR for the new bounded context (e.g., "Consider creating an ADR for the new '<name>' domain").

4. **Idempotency guard**: refuses to overwrite an existing domain directory. If `docs/domains/<name>/` already exists, exits with an error and non-zero status.

5. **Name validation**: domain names must match `^[a-z][a-z0-9-]*$`. Invalid names produce a clear error.

### `mindspec domain list`

6. Reads `docs/domains/` and prints a table with columns: **Domain**, **Owns** (extracted from the context map entry or the first line of overview.md's "What This Domain Owns" section), and **Relationships** (upstream/downstream from context map). Sorted alphabetically by name.

7. If no domains exist, prints "No domains found."

### `mindspec domain show <name>`

8. Emits a concise, token-efficient summary of a single domain:
   - **Owns**: from context map or overview.md
   - **Boundaries**: from overview.md "Boundaries" section
   - **Relationships**: upstream/downstream domains with direction, parsed from context map
   - **Key Files**: from overview.md "Key Files" section
   - **Specs**: list of specs whose "Impacted Domains" includes this domain (scan spec frontmatter/headers)

9. If the domain does not exist, exits with an error and non-zero status.

10. Output is plain text by default. `--json` flag emits JSON for programmatic consumption.

## Scope

### In Scope

- `cmd/mindspec/domain.go` — cobra command wiring for `domain add`, `domain list`, `domain show`
- `internal/domain/scaffold.go` — scaffolding logic (create dir, write templates, update context map)
- `internal/domain/list.go` — list logic with context map parsing for ownership/relationships
- `internal/domain/show.go` — show logic with domain doc parsing and spec scanning
- Embedded Go templates for the four domain doc files
- Unit tests for all domain package functions

### Out of Scope

- Context map relationship management (adding/editing relationships between contexts)
- Domain removal or renaming
- Validation of domain doc completeness (covered by existing doctor checks)
- ADR creation itself (deferred to Spec 012)

## Non-Goals

- Modifying existing domain docs or context map entries beyond the initial `add` stub
- Interactive prompts for domain metadata (keep it simple: name only for `add`)
- Generating code-level bounded context boundaries (this is a documentation + query tool)

## Acceptance Criteria

- [ ] `mindspec domain add payments` creates `docs/domains/payments/` with 4 correctly-titled template files
- [ ] Running `domain add payments` again fails with "domain 'payments' already exists"
- [ ] `mindspec domain add 123bad` fails with a name validation error
- [ ] After `domain add`, `docs/context-map.md` contains a new `### Payments` section under Bounded Contexts
- [ ] `domain add` prints an ADR recommendation message to stdout
- [ ] `mindspec domain list` outputs a table with Domain, Owns, and Relationships columns for all domains
- [ ] `mindspec domain list` on a project with no domains prints "No domains found."
- [ ] `mindspec domain show core` outputs ownership, boundaries, relationships, key files, and impacting specs
- [ ] `mindspec domain show core --json` outputs valid JSON with the same fields
- [ ] `mindspec domain show nonexistent` fails with a clear error
- [ ] All new code has unit tests; `make test` passes
- [ ] `make build` succeeds

## Validation Proofs

- `./bin/mindspec domain add test-domain && ls docs/domains/test-domain/`: Should list overview.md, architecture.md, interfaces.md, runbook.md
- `./bin/mindspec domain add test-domain`: Should fail (already exists)
- `./bin/mindspec domain list`: Should show table with all domains including "test-domain"
- `./bin/mindspec domain show core`: Should display core domain summary with relationships
- `./bin/mindspec domain show core --json | jq .owns`: Should output ownership string
- `grep "Test-Domain" docs/context-map.md`: Should find the new bounded context entry
- `make test`: All tests pass

## Open Questions

(none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec