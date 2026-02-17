# Context-System Domain — Architecture

## Key Patterns

### Glossary-Based Resolution

The glossary maps concepts to documentation anchors. Given input text, the system:
1. Tokenizes and matches against glossary terms
2. Resolves each term to a file path + anchor
3. Extracts the targeted section

This is deterministic — same input always produces same context.

### Context Pack Assembly (ADR-0001)

Context packs are assembled using DDD-informed rules:

1. **Start** from impacted domains declared in the spec bead
2. **Include** domain `overview.md`, `architecture.md`, `interfaces.md`, and accepted ADRs
3. **Expand 1-hop** via Context Map: neighbor `interfaces.md` + referenced contracts only
4. **Record provenance** back to the bead

### Properties

All context packs must be:
- **Mode-specific** — different content for Spec vs Plan vs Implement
- **Budgeted** — hard token budget
- **Deduped** — no repeated sections within a session
- **Provenance-preserving** — records exactly what was loaded and why

## Invariants

1. Context assembly is deterministic — same inputs produce same pack.
2. Context packs never include full neighbor internals — only `interfaces.md` for 1-hop neighbors.
3. Provenance must be recorded for every included section.
4. Glossary targets use relative paths from project root.
