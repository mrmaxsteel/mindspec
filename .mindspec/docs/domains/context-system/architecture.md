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

### Provenance Model

MindSpec tracks provenance in two directions:

- **Input provenance** — context packs record exactly what was loaded and why. Each included section is tagged with its source (domain doc, ADR, glossary match) and the reason it was selected (impacted domain, 1-hop neighbor, keyword match). This answers: *"what informed this work?"*
- **Output provenance** — plans map spec acceptance criteria to bead verification steps via a `## Provenance` section. This answers: *"what spec requirements does this plan satisfy?"* Output provenance is validated during plan approval (Spec 039).

Together these close the loop: input provenance traces from context → plan, output provenance traces from plan → spec requirements.

### Properties

All context packs must be:
- **Mode-specific** — different content for Spec vs Plan vs Implement
- **Budgeted** — hard token budget
- **Deduped** — no repeated sections within a session
- **Provenance-preserving** — records exactly what was loaded and why (input provenance)

### Tier-Aware Artifact Resolution (Spec 106)

The bead context-pack budgeter (`internal/contextpack/budgeter.go`) resolves the
spec directory and per-domain docs through the Bead-1 tier-aware accessors
(`workspace.SpecsDir` / `workspace.DomainsDir`) instead of hardcoded
`.mindspec/docs/...` joins, and the ADR store is already tier-aware via
`workspace.ADRDir`. A pack therefore assembles byte-identical CONTENT sections
(Bead / Spec / Cited ADRs / Plan / Domain Docs) on a flat
(`.mindspec/{specs,adr,domains}`), canonical (`.mindspec/docs/...`), or legacy
(`docs/...`) project — only the Provenance block, which embeds the resolved
file paths, differs by layout. No spec, domain, or ADR is silently dropped
across the flatten.

## Invariants

1. Context assembly is deterministic — same inputs produce same pack.
2. Context packs never include full neighbor internals — only `interfaces.md` for 1-hop neighbors.
3. Input provenance must be recorded for every included section.
4. Output provenance (AC → bead verification mapping) should be present in every plan.
4. Glossary targets use relative paths from project root.
