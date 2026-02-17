---
status: Approved
spec_id: 000-beads-hygiene
version: "1.0"
last_updated: 2026-02-11
approved_at: 2026-02-11
approved_by: user
bead_ids:
  - mindspec-kj8      # spec bead
  - mindspec-kj8.1    # 000-A: init + gitignore
  - mindspec-kj8.2    # 000-B: packaging excludes
  - mindspec-kj8.3    # 000-C: doctor checks
adr_citations:
  - id: ADR-0002
    sections: ["A (responsibility boundaries)", "C (active workset discipline)"]
---

# Plan: Spec 000 — Repo + Beads Hygiene

**Spec**: [spec.md](spec.md)

---

## Bead 000-A: Initialize Beads + selective gitignore

**Scope**: Bootstrap `.beads/` in the repo and configure git to track durable state only.

**Steps**:
1. Run `beads init` in the repo root
2. Inventory files Beads creates (`.beads/*.db`, `issues.jsonl`, `bd.sock`, `*.db-wal`, `*.db-shm`, locks, tmp)
3. Update `.gitignore` with selective `.beads/` rules:
   - Ignore runtime: `bd.sock`, `*.lock`, `*.pid`, `tmp/`, `*.db`, `*.db-wal`, `*.db-shm`
   - Allow durable: `issues.jsonl`, config files
4. Stage durable files and verify `git status` shows no runtime artifacts

**Verification**:
- [ ] `.beads/` directory exists with durable state files
- [ ] `git status` shows no runtime artifacts as untracked
- [ ] Durable files (`issues.jsonl`) are stageable/trackable

**Depends on**: nothing

---

## Bead 000-B: Packaging excludes

**Scope**: Ensure `.beads/` is excluded from Python packaging contexts.

**Steps**:
1. Add `.beads/` exclude to `pyproject.toml` (`[tool.setuptools]` exclude or equivalent)
2. Add `MANIFEST.in` with `prune .beads` if needed for sdist
3. Build sdist and verify tarball contains no `.beads/` files
4. Verify wheel excludes `.beads/` (src layout should handle this; confirm)

**Verification**:
- [ ] `python -m build --sdist` tarball contains no `bd.sock`, lock, tmp, or db files
- [ ] Wheel does not contain `.beads/` artifacts

**Depends on**: 000-A

---

## Bead 000-C: Doctor Beads hygiene checks

**Scope**: Add Beads-specific checks to `mindspec doctor` command.

**Steps**:
1. Add check: `.beads/` directory exists
2. Add check: durable state file (`issues.jsonl`) is present
3. Add check: no Beads runtime artifacts tracked by git (run `git ls-files .beads/` filtered against known runtime patterns)
4. Report results in doctor output with actionable messages (e.g., "Run `beads init` to initialize" or "Runtime artifact tracked: .beads/bd.sock — add to .gitignore")
5. Exit code 1 if runtime artifacts are tracked by git
6. Doc-sync: update core domain docs to reflect new doctor capabilities

**Verification**:
- [ ] `mindspec doctor` reports Beads directory status (present/missing)
- [ ] `mindspec doctor` reports durable state status (issues.jsonl present/missing)
- [ ] `mindspec doctor` warns and exits non-zero if runtime artifacts are git-tracked
- [ ] Core domain docs updated with new doctor check descriptions

**Depends on**: 000-A

---

## Dependency Graph

```
000-A (init + gitignore)
  ├── 000-B (packaging excludes)
  └── 000-C (doctor checks)
```

000-B and 000-C can run in parallel after 000-A completes.
