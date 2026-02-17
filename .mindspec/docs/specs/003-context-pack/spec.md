# Spec 003: Context Pack Generation

## Goal

Generate reproducible, DDD-informed context packs that bundle relevant documentation, policies, and metadata for agent sessions.

## Background

A context pack provides an agent with all the information needed to work on a spec without manual doc hunting. Per ADR-0001, context packs use the project's DDD artifacts (domains, Context Map, ADRs) as a routing layer for deterministic assembly.

## Impacted Domains

- context-system: this is the core context delivery mechanism
- workflow: context packs are mode-specific (Spec/Plan/Implement)

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): Defines DDD-informed assembly rules — impacted domains, 1-hop neighbor expansion via Context Map, provenance recording (Accepted)
- [ADR-0002](../../adr/ADR-0002.md): Context packs read from the documentation system, not from Beads content

## Requirements

1. **Context Pack Generation**: Given a spec ID, produce a complete context pack
2. **DDD-Informed Assembly**: Use impacted domains to route content selection; expand 1-hop via Context Map for neighbor contracts
3. **Provenance Tracking**: Record sources (file + anchor) for all included content
4. **Mode-Specific Content**: Different content tiers per mode (see Mode Content Rules below)
5. **Commit Tuple**: Include repo commit SHA for reproducibility
6. **Output Format**: Generate `context-pack.md` in the spec directory

## Scope

### In Scope
- `internal/context/` package (context pack generation logic)
- `cmd/mindspec/context.go` (Cobra subcommand wiring)
- Integration with `internal/glossary/` matching (from Spec 002)
- DDD routing: read impacted domains from spec, include domain docs + accepted ADRs
- Context Map 1-hop expansion: include neighbor `interfaces.md`
- CLI command: `mindspec context pack <spec-id>` (follows existing Cobra patterns from Specs 001/002)
- Commit SHA extraction via git
- Provenance section in output

### Out of Scope
- Memory recall integration (see future Memory spec)
- Token budget enforcement (future enhancement)
- Session dedupe cache (future enhancement)
- Multi-repo workspace support

## Mode Content Rules

Each mode includes progressively more detail:

| Content | Spec Mode | Plan Mode | Implement Mode |
|:--------|:---------:|:---------:|:--------------:|
| Spec summary | Yes | Yes | Yes |
| Domain `overview.md` (impacted) | Yes | Yes | Yes |
| Accepted ADRs (impacted domains) | Yes | Yes | Yes |
| Relevant `architecture/policies.yml` | Yes | Yes | Yes |
| Domain `architecture.md` (impacted) | — | Yes | Yes |
| Neighbor `interfaces.md` (1-hop) | — | Yes | Yes |
| Domain `interfaces.md` (impacted) | — | — | Yes |
| Domain `runbook.md` (impacted) | — | — | Yes |
| Active bead context (scope, verification steps) | — | — | Yes |

The `--mode` flag selects which tier to apply. Default is `spec`.

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

- `./bin/mindspec context pack 001`: Should generate context-pack.md
- `./bin/mindspec context pack 001 --mode plan`: Should include architecture docs and neighbor contracts
- Generated context-pack.md includes identifiable sections from domain docs
- Provenance section lists source files and reason for inclusion

## Open Questions

- Should the default `--mode` be inferred from the spec's current lifecycle state, or always require an explicit flag?

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-11
- **Notes**: Approved via /spec-approve workflow. Depends on 002-glossary for keyword matching. DDD assembly rules from ADR-0001 (Accepted).
