---
adr_citations: []
approved_at: "2026-02-13T14:53:55Z"
approved_by: user
bead_ids: []
last_updated: "2026-02-13"
spec_id: 010-spec-init-cmd
status: Approved
version: 1
work_chunks:
    - depends_on: []
      id: 1
      scope: internal/specinit/specinit.go, cmd/mindspec/spec_init.go, internal/specinit/specinit_test.go, .claude/commands/spec-init.md
      title: Implement mindspec spec-init command
      verify:
        - '`mindspec spec-init 999-test` creates `docs/specs/999-test/spec.md` with ID and title filled in'
        - '`mindspec spec-init 999-test` (second run) fails with ''already exists'' error'
        - '`mindspec state show` shows mode=spec, activeSpec=999-test'
        - '`mindspec spec-init 999-custom --title ''Custom Title''` uses the provided title'
        - CLI output includes instruct-tail (spec-mode guidance)
        - '`.claude/commands/spec-init.md` is under 15 lines'
        - '`make test` passes'
---

# Plan: `mindspec spec-init` CLI Command

**Spec**: [spec.md](spec.md)

## Bead 1: Implement `mindspec spec-init` command

**Scope**: `internal/specinit/specinit.go`, `cmd/mindspec/spec_init.go`, `internal/specinit/specinit_test.go`, `.claude/commands/spec-init.md`

**Steps**

1. Create `internal/specinit/specinit.go` with `Run(root, specID, title string) error` — validate no existing dir, derive title from slug if empty, create dir, read `docs/templates/spec.md`, replace `<ID>` and `<Title>`, write spec, set state via `state.Write()`
2. Create `cmd/mindspec/spec_init.go` — cobra command `spec-init [spec-id]` with `--title` flag, calls `specinit.Run()`, prints summary, calls `emitInstruct(root)`
3. Create `internal/specinit/specinit_test.go` — tests for happy path, title override, clobber protection, state verification
4. Rewrite `.claude/commands/spec-init.md` to thin shim (~10 lines): ask user for ID, run CLI, handle errors

**Verification**

- [ ] `mindspec spec-init 999-test` creates `docs/specs/999-test/spec.md` with ID and title filled in
- [ ] `mindspec spec-init 999-test` (second run) fails with "already exists" error
- [ ] `mindspec state show` shows mode=spec, activeSpec=999-test
- [ ] `mindspec spec-init 999-custom --title "Custom Title"` uses the provided title
- [ ] CLI output includes instruct-tail (spec-mode guidance)
- [ ] `.claude/commands/spec-init.md` is under 15 lines
- [ ] `make test` passes

**Depends on**: None
