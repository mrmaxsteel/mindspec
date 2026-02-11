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
| `src/mindspec/docs.py` | Glossary parsing, section extraction, health checks |
| `src/mindspec/glossary.py` | Keyword matching (planned, Spec 002) |
| `src/mindspec/context.py` | Context pack generation (planned, Spec 003) |
| `GLOSSARY.md` | Concept-to-doc-section mapping |

## Current State

Basic glossary parsing exists in `docs.py`. Full keyword matching (Spec 002) and context pack generation (Spec 003) are planned.
