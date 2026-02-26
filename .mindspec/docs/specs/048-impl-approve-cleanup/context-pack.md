# Context Pack

- **Spec**: 048-impl-approve-cleanup
- **Mode**: plan
- **Commit**: 1bdf01cd2cd317558d2f3238ffaa1848acdddb6f
- **Generated**: 2026-02-26T17:52:06Z

---

## Goal

Fix three related gaps in the spec lifecycle endgame: (1) `spec-init` writes spec files to main before creating the worktree, violating zero-on-main, (2) `impl-approve` silently performs merge/cleanup without informing the user what happened, offering no interactive PR flow, and never waiting for CI, and (3) the `PreToolUse` enforcement hooks (spec 046) block agents completely — the Bash hook uses `pwd` which always returns the main CWD, and the Edit/Write hooks receive an empty `$CLAUDE_TOOL_ARG_FILE_PATH`, making it impossible for an agent to comply.

## Impacted Domains

- **workflow**
- **git**
- **agent-integration**

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
