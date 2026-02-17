# Spec 002: Glossary-Based Context Injection

## Goal

Enable deterministic retrieval of documentation sections based on keyword matching against the project glossary.

## Background

MindSpec uses a glossary (`GLOSSARY.md`) to map keywords/concepts to specific documentation anchors. This spec implements the parsing and matching logic that powers context injection — the foundation of the Context Pack system.

## Impacted Domains

- context-system: glossary is a core input for Context Pack assembly

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): Glossary/registry is listed as a required primitive for deterministic context injection
- [ADR-0002](../../adr/ADR-0002.md): Glossary targets point to the documentation system (not Beads)

## Requirements

1. **Glossary Parsing**: Load and parse `GLOSSARY.md` into a keyword-to-target mapping
2. **Keyword Matching**: Given input text (prompt, spec, etc.), identify matching glossary terms
3. **Section Extraction**: Retrieve the targeted documentation sections
4. **CLI Command**: Expose functionality via `mindspec glossary` subcommands

## Scope

### In Scope
- `internal/glossary/` (parsing and matching logic)
- `internal/docs/` (section extraction from markdown)
- CLI integration via cobra (`cmd/mindspec/`) for glossary commands

### Out of Scope
- Full context pack generation (see Spec 003)
- Vector/semantic search
- Memory recall integration
- Domain-based routing (that's Context Pack's responsibility)

## Non-Goals

- Replacing glossary with embeddings or vector search
- Automatic glossary generation

## Acceptance Criteria

- [ ] `mindspec glossary list` displays all glossary terms and targets
- [ ] `mindspec glossary match "<text>"` returns matching terms from input text
- [ ] `mindspec glossary show <term>` displays the linked documentation section
- [ ] Glossary parsing handles table format with `| Term | Target |` structure
- [ ] Invalid anchors are reported with actionable error messages

## Validation Proofs

- `mindspec glossary list`: Should list all terms from GLOSSARY.md
- `mindspec glossary match "spec mode approval"`: Should match "Spec Mode" and related terms
- `mindspec glossary show "Context Pack"`: Should display the linked doc section

## Open Questions

- (none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-11
- **Notes**: Approved via /spec-approve workflow. Updated scope to Go paths per Spec 001.
