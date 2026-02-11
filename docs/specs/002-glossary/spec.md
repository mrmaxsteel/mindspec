# Spec 002: Glossary-Based Context Injection

## Goal

Enable deterministic retrieval of documentation sections based on keyword matching against the project glossary.

## Background

Mindspec uses a glossary (`GLOSSARY.md`) to map keywords/concepts to specific documentation anchors. This spec implements the parsing and matching logic that powers context injection.

## Requirements

1. **Glossary Parsing**: Load and parse `GLOSSARY.md` into a keyword → target mapping
2. **Keyword Matching**: Given input text (prompt, spec, etc.), identify matching glossary terms
3. **Section Extraction**: Retrieve the targeted documentation sections
4. **CLI Command**: Expose functionality via `mindspec glossary` subcommands

## Scope

### In Scope
- `src/mindspec/glossary.py` (parsing and matching logic)
- `src/mindspec/docs.py` (section extraction from markdown)
- CLI integration for glossary commands

### Out of Scope
- Full context pack generation (see Spec 003)
- Vector/semantic search
- Memory recall integration

## Acceptance Criteria

- [ ] `mindspec glossary list` displays all glossary terms and targets
- [ ] `mindspec glossary match "<text>"` returns matching terms from input text
- [ ] `mindspec glossary show <term>` displays the linked documentation section
- [ ] Glossary parsing handles table format with `| Term | Target |` structure
- [ ] Invalid anchors are reported with actionable error messages

## Validation Proofs

- `python -m mindspec glossary list`: Should list all terms from GLOSSARY.md
- `python -m mindspec glossary match "spec mode approval"`: Should match "Spec Mode" and "Approval Gate"
- `python -m mindspec glossary show "Context Pack"`: Should display the linked doc section

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Extracted from original 001-skeleton scope.
