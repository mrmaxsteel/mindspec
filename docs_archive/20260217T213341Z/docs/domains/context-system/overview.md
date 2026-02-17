# Context-System Domain — Overview

## What This Domain Owns

The **context-system** domain owns deterministic context delivery:

- **Glossary** — parsing `GLOSSARY.md`, keyword-to-doc-section mapping, term matching
- **Context Packs** — assembling mode-specific, budgeted, deduped, provenance-preserving bundles
- **DDD-informed assembly** — routing content selection based on impacted domains and Context Map
- **Provenance** — tracking what was included in a context pack and why

## Boundaries

Context-system does **not** own:
- CLI infrastructure or health checks (core)
- Mode enforcement, spec lifecycle, or Beads integration (workflow)
- The Context Map itself (that's a shared project artifact at `docs/context-map.md`)

Context-system **reads** the Context Map and domain docs; it does not govern their lifecycle.

## Key Files

| File | Purpose |
|:-----|:--------|
| `internal/glossary/glossary.go` | Glossary parsing and entry types (Spec 002) |
| `internal/glossary/match.go` | Keyword matching, longest-first (Spec 002) |
| `internal/glossary/section.go` | Section extraction from markdown (Spec 002) |
| `internal/contextpack/builder.go` | Context pack assembler, renderer, writer (Spec 003) |
| `internal/contextpack/spec.go` | Spec parser (goal, impacted domains) (Spec 003) |
| `internal/contextpack/domaindoc.go` | Domain doc file reader (Spec 003) |
| `internal/contextpack/contextmap.go` | Context Map parser + 1-hop resolution (Spec 003) |
| `internal/contextpack/adr.go` | ADR scanner + domain filter (Spec 003) |
| `internal/contextpack/policy.go` | Policies parser + mode filter (Spec 003) |
| `cmd/mindspec/context.go` | CLI `context pack` command (Spec 003) |
| `GLOSSARY.md` | Concept-to-doc-section mapping |

## Current State

Glossary parsing and keyword matching implemented in Go (Spec 002). Context pack generation implemented (Spec 003) with DDD-informed assembly, mode-specific content tiers, and provenance tracking.
