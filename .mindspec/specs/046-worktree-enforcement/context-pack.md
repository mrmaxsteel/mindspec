# Context Pack

- **Spec**: 046-worktree-enforcement
- **Mode**: plan
- **Commit**: 52afae9baefb32401e9d18de2211831f8bbeb513
- **Generated**: 2026-02-26T08:37:10Z

---

## Goal

Enforce zero-on-main invariant: all mindspec-managed changes happen on branches in worktrees, never on main. Enforcement is deterministic (git hooks, CLI guards, agent hooks) — not prompt-based. An agent running in an IDE workspace rooted at main must have its CLI calls and file writes redirected to the correct worktree context.

## Impacted Domains

- **workflow**
- **git**
- **agent-integration**

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
