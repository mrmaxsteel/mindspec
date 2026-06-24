---
status: Approved
spec_id: 006-validate
version: "1.0"
last_updated: 2026-02-12
approved_at: 2026-02-12
approved_by: user
bead_ids: [mindspec-pvs, mindspec-gqm, mindspec-f0z, mindspec-atc]
adr_citations:
  - id: ADR-0003
    sections: ["CLI Contract — mindspec validate"]
  - id: ADR-0005
    sections: ["State File Schema"]
---

# Plan: Spec 006 — Workflow Validation (`mindspec validate`)

**Spec**: [spec.md](spec.md)

---

## Design Notes

### Reusing the Check/Report Pattern

Doctor established `Check{Name, Status, Message}` + `Report{Checks}` + `HasFailures()`. Validate needs the same pattern but with richer output (JSON support, sub-command scoping). Rather than importing from doctor (tight coupling), validate defines its own `Issue` type with severity levels matching doctor's semantics. This keeps the packages independent while following the same idiom.

### Spec Markdown Parsing

Spec validation parses markdown by section headings (`## Goal`, `## Requirements`, etc.). For each required section, check:
1. Section exists (heading present)
2. Section has content (not empty or placeholder text)
3. Section-specific rules (e.g., criteria count >= 3, open questions resolved)

Vague criteria detection uses a simple blocklist: "works correctly", "is fast", "properly handles", "is good", "functions as expected".

### Plan YAML Frontmatter Parsing

Plan validation splits on `---` fences to extract YAML frontmatter, then parses with `yaml.v3`. Required fields: `status`, `spec_id`, `version`. On approval: `approved_at`, `approved_by`, `bead_ids`, `adr_citations`.

### Plan Bead Section Parsing

Bead sections are identified by `## Bead ` prefix. Within each bead section, check for:
- `**Steps**:` followed by numbered list items (count 3-7)
- `**Verification**:` followed by checkbox items (`- [ ]` or `- [x]`)
- `**Depends on**:` present

### Doc-Sync Heuristic

Convention-based mapping for v1:
- `internal/<pkg>/` changes → expect `docs/domains/` changes
- `cmd/mindspec/` changes → expect CLAUDE.md or `docs/core/CONVENTIONS.md` changes
- New packages (added directories) → flag for doc review

Uses `git diff --name-only <ref>` to get changed files. Default ref is `HEAD~1`; override with `--diff=<ref>`.

Reports missing doc-sync as **warnings** (not errors), since some code changes genuinely don't need doc updates.

### Bead ID Verification

`validate plan` checks bead IDs in frontmatter by shelling out to `bd show <id> --json`. If the command fails, report as a warning (Beads might be unavailable).

---

## Bead 006-A: Core validation framework + spec validator

**Scope**: `internal/validate/` package — Issue/Result types, spec validation logic, JSON output support

**Steps**:
1. Create `internal/validate/validate.go`: `Issue` struct (Name, Severity, Message), `Result` struct (Issues, SubCommand, SpecID), `HasFailures()`, `ToJSON()`, `FormatText()` rendering methods
2. Create `internal/validate/spec.go`: `ValidateSpec(root, specID string) *Result` — parses spec.md, checks all 9 quality criteria from spec-approve (goal defined, domains declared, ADR touchpoints, requirements >= 2, scope bounded, criteria count >= 3, criteria quality, not vague, open questions resolved)
3. Create `internal/validate/vague.go`: `IsVagueCriterion(text string) bool` — blocklist check for vague phrases
4. Write tests: spec validation for passing spec (005-next), failing spec (missing sections), vague criteria detection, JSON output formatting

**Verification**:
- [ ] `ValidateSpec()` returns no errors for a well-formed spec (e.g., 005-next)
- [ ] `ValidateSpec()` reports missing/empty sections correctly
- [ ] `ValidateSpec()` flags unresolved open questions
- [ ] `ValidateSpec()` detects vague acceptance criteria
- [ ] `Result.ToJSON()` produces valid JSON
- [ ] `Result.HasFailures()` correctly distinguishes errors from warnings
- [ ] `make test` passes with validate package tests

**Depends on**: nothing

---

## Bead 006-B: Plan validator + bead ID verification

**Scope**: `internal/validate/plan.go` — plan frontmatter and bead section validation

**Steps**:
1. Create `internal/validate/plan.go`: `ValidatePlan(root, specID string) *Result` — parses plan.md YAML frontmatter (required fields), finds bead sections, checks each for steps (3-7), verification items, and depends-on declaration
2. Create `internal/validate/beads.go`: `CheckBeadExists(id string) (bool, error)` — shells out to `bd show <id> --json`, returns true if bead found
3. Add bead ID verification to `ValidatePlan()`: for each ID in `bead_ids` frontmatter, call `CheckBeadExists()`; report as warning if Beads unavailable, error if ID doesn't exist
4. Add ADR citation check: verify `adr_citations` is non-empty in frontmatter
5. Write tests: plan validation for passing plan (005-next), missing frontmatter fields, beads without steps/verification, JSON output

**Verification**:
- [ ] `ValidatePlan()` checks YAML frontmatter required fields
- [ ] `ValidatePlan()` verifies at least one bead section with steps and verification
- [ ] `ValidatePlan()` checks ADR citations present
- [ ] `ValidatePlan()` verifies bead IDs exist in Beads (or warns if Beads unavailable)
- [ ] `make test` passes with plan validation tests

**Depends on**: 006-A

---

## Bead 006-C: Doc-sync validator

**Scope**: `internal/validate/docsync.go` — convention-based doc-sync checking via git diff

**Steps**:
1. Create `internal/validate/docsync.go`: `ValidateDocs(root, diffRef string) *Result` — runs `git diff --name-only <ref>`, buckets changed files by source area, checks for corresponding doc changes
2. Implement mapping heuristic: `internal/` → `docs/domains/`, `cmd/mindspec/` → `CLAUDE.md` or `docs/core/CONVENTIONS.md`
3. Report missing doc-sync as warnings (not errors)
4. Handle edge cases: no changed files (report OK), only doc changes (report OK), git diff failure (report error)
5. Write tests: mock git diff output parsing, mapping heuristic, warning generation

**Verification**:
- [ ] `ValidateDocs()` detects source changes without corresponding doc changes
- [ ] `ValidateDocs()` reports as warnings (not errors)
- [ ] `ValidateDocs()` handles no-change and doc-only-change cases
- [ ] `make test` passes with doc-sync validation tests

**Depends on**: 006-A

---

## Bead 006-D: CLI command wiring + stub replacement

**Scope**: `cmd/mindspec/validate.go` — replace stub, wire three sub-commands with `--format` flag

**Steps**:
1. Create `cmd/mindspec/validate.go`: parent `validateCmd` with three sub-commands (`validateSpecCmd`, `validatePlanCmd`, `validateDocsCmd`)
2. Each sub-command: find root, call corresponding validator, format output (text or JSON), exit non-zero on failure
3. Add `--format=json` persistent flag on parent command (inherited by sub-commands)
4. Add `--diff` flag on `validateDocsCmd` (default `HEAD~1`)
5. Remove `validateCmd` from `cmd/mindspec/stubs.go` (file should now be empty — delete it)
6. Write integration-style tests: verify sub-command routing, JSON output, exit codes

**Verification**:
- [ ] `./bin/mindspec validate spec 005-next` passes with no errors
- [ ] `./bin/mindspec validate plan 005-next` passes with no errors
- [ ] `./bin/mindspec validate docs` runs and reports findings
- [ ] `--format=json` produces valid JSON on all sub-commands
- [ ] Non-zero exit code on validation failure
- [ ] `stubs.go` deleted (no stubs remain)
- [ ] Existing commands unaffected
- [ ] `make test` passes with all tests

**Depends on**: 006-A, 006-B, 006-C

---

## Dependency Graph

```
006-A (core framework + spec validator)
  ├── 006-B (plan validator + bead ID check)
  ├── 006-C (doc-sync validator)
  └── 006-D (CLI wiring + stub replacement)
        ├── depends on 006-B
        └── depends on 006-C
```
