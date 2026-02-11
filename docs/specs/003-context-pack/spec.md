# Spec 003: Context Pack Generation

## Goal

Generate reproducible context packs that bundle relevant documentation, policies, and metadata for agent sessions.

## Background

A context pack provides an agent with all the information needed to work on a spec without manual doc hunting. It includes matched glossary sections, spec details, relevant policies, and a commit tuple for reproducibility.

## Requirements

1. **Context Pack Generation**: Given a spec ID, produce a complete context pack
2. **Provenance Tracking**: Record sources (file + anchor) for all included content
3. **Commit Tuple**: Include repo commit SHA for reproducibility
4. **Output Format**: Generate `context-pack.md` in the spec directory

## Scope

### In Scope
- `src/mindspec/context.py` (context pack generation)
- Integration with glossary matching (from Spec 002)
- CLI command: `mindspec context pack <spec-id>`
- Commit SHA extraction via git

### Out of Scope
- Memory recall integration (see future Memory spec)
- Token budget enforcement
- Multi-repo workspace support

## Acceptance Criteria

- [ ] `mindspec context pack 001` generates `docs/specs/001-skeleton/context-pack.md`
- [ ] Context pack includes: spec summary, matched glossary sections, relevant policies
- [ ] Provenance section lists all sources with file paths and anchors
- [ ] Commit tuple shows current repo SHA
- [ ] Command fails gracefully if spec ID doesn't exist

## Validation Proofs

- `python -m mindspec context pack 001`: Should generate context-pack.md
- Generated context-pack.md includes identifiable sections from ARCHITECTURE.md
- Provenance section lists source files used

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Depends on 002-glossary for keyword matching.
