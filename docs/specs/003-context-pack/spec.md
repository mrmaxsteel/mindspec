# Spec 003: Context Pack Generation

## Goal

Generate reproducible, DDD-informed context packs that bundle relevant documentation, policies, and metadata for agent sessions.

## Background

A context pack provides an agent with all the information needed to work on a spec without manual doc hunting. Per ADR-0001, context packs use the project's DDD artifacts (domains, Context Map, ADRs) as a routing layer for deterministic assembly.

## Impacted Domains

- context-system: this is the core context delivery mechanism
- workflow: context packs are mode-specific (Spec/Plan/Implement)

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): Defines DDD-informed assembly rules — impacted domains, 1-hop neighbor expansion via Context Map, provenance recording
- [ADR-0002](../../adr/ADR-0002.md): Context packs read from the documentation system, not from Beads content

## Requirements

1. **Context Pack Generation**: Given a spec ID, produce a complete context pack
2. **DDD-Informed Assembly**: Use impacted domains to route content selection; expand 1-hop via Context Map for neighbor contracts
3. **Provenance Tracking**: Record sources (file + anchor) for all included content
4. **Mode-Specific Content**: Different content for Spec vs Plan vs Implement modes
5. **Commit Tuple**: Include repo commit SHA for reproducibility
6. **Output Format**: Generate `context-pack.md` in the spec directory

## Scope

### In Scope
- `src/mindspec/context.py` (context pack generation)
- Integration with glossary matching (from Spec 002)
- DDD routing: read impacted domains from spec, include domain docs + accepted ADRs
- Context Map 1-hop expansion: include neighbor `interfaces.md`
- CLI command: `mindspec context pack <spec-id>`
- Commit SHA extraction via git
- Provenance section in output

### Out of Scope
- Memory recall integration (see future Memory spec)
- Token budget enforcement (future enhancement)
- Session dedupe cache (future enhancement)
- Multi-repo workspace support

## Non-Goals

- Replacing deterministic context with vector/semantic search
- Full token budget optimization in v1

## Acceptance Criteria

- [ ] `mindspec context pack 001` generates `docs/specs/001-skeleton/context-pack.md`
- [ ] Context pack includes: spec summary, domain docs for impacted domains, accepted ADRs for those domains, relevant policies
- [ ] If Context Map exists, neighbor `interfaces.md` is included for 1-hop neighbors
- [ ] Provenance section lists all sources with file paths, anchors, and reason for inclusion
- [ ] Commit tuple shows current repo SHA
- [ ] Command fails gracefully if spec ID doesn't exist
- [ ] Output varies by mode flag (e.g., `--mode plan` includes different content than `--mode implement`)

## Validation Proofs

- `python -m mindspec context pack 001`: Should generate context-pack.md
- Generated context-pack.md includes identifiable sections from domain docs and ARCHITECTURE.md
- Provenance section lists source files and reason for inclusion

## Open Questions

- (none)

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: Depends on 002-glossary for keyword matching. DDD assembly rules from ADR-0001.
