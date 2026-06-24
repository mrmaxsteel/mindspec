# Context Pack

- **Spec**: 047-clean-context-enforcement
- **Mode**: plan
- **Commit**: c0b3636535ed2999d26915085c5ece0215ff9c0e
- **Generated**: 2026-02-26T10:03:58Z

---

## Goal

Ensure every agent starting a new implementation bead begins with a clean, focused context window — free of stale reasoning, resolved decisions, and file states from prior beads. This must work in both single-agent mode (one Claude Code session doing sequential beads) and multi-agent mode (team lead spawning fresh agents per bead).

## Impacted Domains

- **agent-lifecycle**
- **instruct**
- **state**

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
