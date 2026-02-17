---
status: Approved
spec_id: 001-skeleton
version: "1.0"
last_updated: 2026-02-11
approved_at: 2026-02-11T00:00:00Z
approved_by: user
bead_ids:
  - mindspec-u6o   # 001-A: Go scaffolding + CLI entry point
  - mindspec-8zk   # 001-B: Workspace detection
  - mindspec-7rj   # 001-C: Doctor docs structure checks
  - mindspec-49b   # 001-D: Doctor Beads hygiene checks
  - mindspec-2nf   # 001-E: Python retirement + doc-sync
adr_citations:
  - id: ADR-0002
    sections: ["Beads integration", "doctor owns project health"]
  - id: ADR-0003
    sections: ["CLI command surface: instruct, next, validate stubs"]
  - id: ADR-0004
    sections: ["Go as v1 language"]
---

# Plan: Spec 001 — Go CLI Skeleton + Doctor

**Spec**: [spec.md](spec.md)

---

## Design Decisions

| Decision | Choice | Rationale |
|:---------|:-------|:----------|
| Module path | `github.com/mindspec/mindspec` | Go convention; works locally, avoids rename if published later |
| CLI framework | cobra | ADR-0004 mentions it; handles subcommands, help, flags out of the box |
| Build system | Makefile | Universal availability; targets: `build`, `install`, `test`, `clean` |
| Glossary in doctor | Existence + term count + broken links | Structural checks, not "parsing" (Spec 002 scope) |
| Python retirement | Delete `src/mindspec/`, `pyproject.toml`, `MANIFEST.in` | Clean break per ADR-0004; Go binary fully replaces Python |

---

## Bead 001-A: Go project scaffolding + CLI entry point

**Scope**: Initialize Go module, create directory layout, wire cobra with root + all subcommands (doctor + 3 stubs), produce a building binary with Makefile.

**Steps**:
1. Create Go layout: `cmd/mindspec/main.go`, `cmd/mindspec/root.go`, `cmd/mindspec/doctor.go`, `cmd/mindspec/stubs.go`, `internal/workspace/`, `internal/doctor/`
2. `go mod init github.com/mindspec/mindspec` + add cobra dependency
3. Implement root cobra command with description, doctor subcommand (shell calling `internal/doctor`), and 3 stub commands printing "not yet implemented"
4. Create `Makefile` with `build` (outputs `./bin/mindspec`), `install`, `test`, `clean`
5. Update `.gitignore` with Go entries (`bin/`, `*.exe`)
6. Verify `go build` + `--help` + all 3 stubs work

**Verification**:
- [x] `go build ./cmd/mindspec` produces binary
- [x] `mindspec --help` shows doctor, instruct, next, validate
- [x] `mindspec instruct`, `next`, `validate` each print stub message

**Depends on**: nothing

---

## Bead 001-B: Workspace detection

**Scope**: Implement `internal/workspace` — walk up from cwd to find project root via `mindspec.md` or `.git`.

**Steps**:
1. Implement `FindRoot(startDir string) (string, error)` — walk up, check `mindspec.md` first then `.git` at each level
2. Add `DocsDir(root)`, `GlossaryPath(root)` helpers (path joins)
3. Write unit tests: finds via mindspec.md, finds via .git, walks up from nested dir, error when no marker

**Verification**:
- [x] `go test ./internal/workspace/...` passes
- [x] Finds root via mindspec.md; finds root via .git only
- [x] Walks up correctly; errors when no marker

**Depends on**: 001-A

---

## Bead 001-C: Doctor — docs structure checks

**Scope**: Implement doctor checks for documentation structure, GLOSSARY.md health, and domain structure.

**Steps**:
1. Define `doctor.Check` (Name, Status, Message) and `doctor.Report` (slice of checks, `HasFailures()` method)
2. Implement checks in `internal/doctor/docs.go`: `docs/core/`, `docs/domains/`, `docs/specs/`, `architecture/`, `GLOSSARY.md` (existence + term count via `| **` regex + broken link validation), `docs/context-map.md`
3. Domain subdirectory checks: for `core`, `context-system`, `workflow` check for `overview.md`, `architecture.md`, `interfaces.md`, `runbook.md` (warnings)
4. Wire into cobra doctor command: call `FindRoot()`, run checks, print `[OK]`/`[MISSING]`/`[ERROR]`, exit 1 on failures
5. Write unit tests with temp directory fixtures

**Verification**:
- [x] `mindspec doctor` reports `[OK]` for existing dirs in the MindSpec repo
- [x] Reports `[MISSING]` with actionable message for absent dirs
- [x] Glossary term count reported; broken links reported as `[ERROR]`
- [x] Exit code 0 when all pass, 1 when any fail
- [x] `go test ./internal/doctor/...` passes

**Depends on**: 001-B

---

## Bead 001-D: Doctor — Beads hygiene checks

**Scope**: Port Beads hygiene checks to Go: `.beads/` existence, durable state files, git-tracked runtime artifact detection.

**Steps**:
1. Implement in `internal/doctor/beads.go`:
   - `.beads/` existence
   - Durable files: `issues.jsonl`, `config.yaml`, `metadata.json`
   - `git ls-files .beads/` filtered against known runtime patterns and extensions
2. Git-tracked runtime artifacts → `[ERROR]` with actionable message
3. Handle missing `git` gracefully (skip with warning)
4. Integrate into `doctor.Run()` report
5. Write unit tests with mock git output

**Verification**:
- [x] Reports Beads directory status and durable files
- [x] Detects git-tracked runtime artifacts as `[ERROR]`
- [x] Exit 1 on tracked runtime artifacts; exit 0 when clean
- [x] `mindspec doctor` passes in the actual MindSpec repo (exit 0)
- [x] `go test ./internal/doctor/...` passes

**Depends on**: 001-C

---

## Bead 001-E: Python retirement + doc-sync

**Scope**: Remove Python prototype and update all documentation to reference Go CLI.

**Steps**:
1. Remove `src/mindspec/` directory, `pyproject.toml`, `MANIFEST.in`
2. Update `CLAUDE.md` "Build & Run" section → Go paths and build commands
3. Update `docs/domains/core/overview.md` → Go paths
4. Update `docs/domains/core/interfaces.md` → Go function signatures
5. Verify `mindspec doctor` still exits 0

**Verification**:
- [x] `src/mindspec/` removed
- [x] `pyproject.toml` and `MANIFEST.in` removed
- [x] Documentation references Go paths and build commands
- [x] `mindspec doctor` passes (exit 0)

**Depends on**: 001-C, 001-D

---

## Dependency Graph

```
001-A  (Go scaffolding + CLI + stubs)
  └── 001-B  (workspace detection)
        └── 001-C  (doctor: docs structure)
              └── 001-D  (doctor: Beads hygiene)
                    └── 001-E  (Python retirement + doc-sync)
```
