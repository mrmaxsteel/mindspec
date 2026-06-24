---
status: Approved
spec_id: "012-adr-lifecycle"
version: "1.0"
last_updated: "2026-02-13"
approved_at: "2026-02-13"
approved_by: user
bead_ids: [mindspec-hub, mindspec-4d8, mindspec-8rk, mindspec-lkm]
adr_citations:
  - id: ADR-0001
    sections: ["DDD Enablement"]
  - id: ADR-0004
    sections: ["Go"]
---

# Plan: Spec 012 — ADR Lifecycle Tooling

## ADR Fitness

ADR-0001 (DDD Enablement) and ADR-0004 (Go) remain appropriate. This spec directly extends ADR-0001's principle that ADRs are first-class governed primitives — adding CLI enforcement. No divergence.

## Bead 1: Shared ADR parse package

**Scope**: `internal/adr/parse.go`, `internal/adr/parse_test.go`, `internal/contextpack/adr.go`, `internal/contextpack/builder.go`

**Steps**

1. Create `internal/adr/parse.go` with extended `ADR` struct (add `Title`, `Date`, `Supersedes`, `SupersededBy` to existing fields). Export `ParseADR(path) (ADR, error)` — same logic as current `contextpack.parseADR()` plus: extract title from `# ADR-NNNN: <Title>` heading (split on `: `), extract `Supersedes`/`Superseded-by` metadata lines via `extractValue()`.
2. Move `ScanADRs(root)`, `FilterADRs(adrs, domains)`, `extractValue()` into `internal/adr/parse.go`. Sort results by ID in `ScanADRs`.
3. Add `NextID(root) (string, error)` — scan existing ADRs, find max numeric suffix, increment, pad to 4 digits. Return `"0001"` if none exist.
4. Refactor `internal/contextpack/adr.go` — remove all parsing logic, import `internal/adr` directly. Replace `contextpack.ADR` with `adr.ADR` in `builder.go` (lines 106, 115). Delete `contextpack.ADR` struct, `parseADR`, `extractValue`, `ScanADRs`, `FilterADRs`. Keep `adr.go` as thin re-exports or remove entirely if `builder.go` can import `adr` directly.
5. Update `internal/contextpack/adr_test.go` to use `adr.ScanADRs` / `adr.FilterADRs` imports.
6. Write `internal/adr/parse_test.go` — test `ParseADR` (all fields), `ScanADRs` (sorted, all fields), `NextID` (increment + empty dir), `FilterADRs`.
7. `make test` — verify no regressions.

**Verification**
- [ ] ADR struct has Title, Date, Supersedes, SupersededBy
- [ ] `ParseADR` extracts all metadata fields correctly
- [ ] `NextID` returns correct next ID
- [ ] `contextpack/builder.go` compiles using `adr.ScanADRs`/`adr.FilterADRs`
- [ ] `make test` passes

**Depends on**: None

---

## Bead 2: ADR create + supersede commands

**Scope**: `internal/adr/create.go`, `internal/adr/create_test.go`, `internal/adr/supersede.go`, `internal/adr/supersede_test.go`, `cmd/mindspec/adr.go`, `cmd/mindspec/root.go`

**Steps**

1. Create `internal/adr/create.go` with `Create(root, title string, opts CreateOpts) (string, error)`. `CreateOpts`: `Domains []string`, `Supersedes string`. Validate non-empty title. Call `NextID(root)`. Read `docs/templates/adr.md`. Replace placeholders: `NNNN`→ID, `<Title>`→title, `<YYYY-MM-DD>`→today, `<comma-separated list>`→joined domains. Write to `docs/adr/ADR-NNNN.md`. Return path.
2. Create `internal/adr/supersede.go` with `Supersede(root, oldID, newID string) error` — read old ADR file, find `**Superseded-by**:` line, replace its value with new ID, write back. Add `CopyDomains(root, oldID string) ([]string, error)` — parse old ADR, return its Domains.
3. In `Create()`, if `opts.Supersedes != ""`: verify old ADR file exists, if `opts.Domains` is empty call `CopyDomains()` to inherit, after writing new file call `Supersede()`.
4. Create `cmd/mindspec/adr.go` with cobra parent `adrCmd` (Use: "adr") and `adrCreateCmd` (Use: `create`, Args: `cobra.ExactArgs(1)`). Flags: `--domain` string, `--supersedes` string. RunE: findRoot, parse flags, split domain on comma, call `adr.Create()`, print result + ADR recommendation message.
5. Register `rootCmd.AddCommand(adrCmd)` in `cmd/mindspec/root.go`.
6. Write tests: `create_test.go` (happy path, empty title error, domain fill, NextID integration), `supersede_test.go` (both-file update, domain copy, missing old ADR error).
7. `make build && make test`.

**Verification**
- [ ] `adr create "Title"` creates correctly populated file
- [ ] Empty title → error
- [ ] `--domain core,workflow` pre-fills Domain(s)
- [ ] `--supersedes ADR-0001` updates old ADR + copies domains
- [ ] `--supersedes ADR-9999` → "not found" error
- [ ] `make test` passes

**Depends on**: Bead 1

---

## Bead 3: ADR list + show commands

**Scope**: `internal/adr/list.go`, `internal/adr/list_test.go`, `internal/adr/show.go`, `internal/adr/show_test.go`, `cmd/mindspec/adr.go`

**Steps**

1. Create `internal/adr/list.go` with `List(root string, opts ListOpts) ([]ADR, error)`. `ListOpts`: `Status string`, `Domain string`. Call `ScanADRs`, filter by status (case-insensitive) and domain. Add `FormatTable(adrs []ADR) string` — columnar output: ID, Status, Domain(s), Title.
2. Create `internal/adr/show.go` with `Show(root, id string) (*ADR, error)` — construct path `docs/adr/<id>.md`, call `ParseADR`, error if not found. Add `ExtractDecision(content string) string` — scan for `## Decision` heading, capture until next `##`. Add `FormatSummary(a *ADR) string` and `FormatJSON(a *ADR) (string, error)` (JSON struct: id, title, status, date, domains, supersedes, superseded_by, decision).
3. Wire `adrListCmd` in `cmd/mindspec/adr.go`: flags `--status`, `--domain`. Print "No ADRs found." if empty, otherwise table + count.
4. Wire `adrShowCmd`: Args `cobra.ExactArgs(1)`, flag `--json`. Print summary or JSON.
5. Write tests: `list_test.go` (unfiltered, status filter, domain filter, empty), `show_test.go` (summary format, JSON validity, Decision extraction, nonexistent ADR error).
6. `make build && make test`.

**Verification**
- [ ] `adr list` shows table sorted by ID
- [ ] `--status accepted` and `--domain workflow` filter correctly
- [ ] Empty dir → "No ADRs found."
- [ ] `adr show ADR-0001` prints summary with Decision section
- [ ] `--json` produces valid JSON
- [ ] Nonexistent ADR → error
- [ ] `make test` passes

**Depends on**: Bead 1

---

## Bead 4: Plan validation + instruct template

**Scope**: `internal/validate/plan.go`, `internal/validate/plan_test.go`, `internal/instruct/templates/plan.md`

**Steps**

1. Add `checkADRCitations(r *Result, root string, citations []ADRCitation)` in `plan.go`. For each citation: check `docs/adr/<id>.md` exists (error if not), parse status — Superseded → warning (suggest superseding ADR), Proposed → warning (suggest accepting first).
2. Add `checkADRFitnessSection(r *Result, content string)` — call `parseSections(content)`, check for key `"ADR Fitness"`. Missing → warning `"adr-fitness-missing"`.
3. Wire both checks into `ValidatePlan()` after `checkBeadIDs` (line 59). Add `import "github.com/mindspec/mindspec/internal/adr"`.
4. Update `internal/instruct/templates/plan.md`:
   - Replace the "Required Review" section to include ADR Fitness Evaluation as an explicit step instructing the agent to actively evaluate ADR fitness, prefer adherence when sound, propose divergence with justification when better, and treat divergence as a human gate
   - Add `ADR Fitness evaluation (## ADR Fitness section in plan.md)` to Required Output
   - Strengthen the ADR divergence human gate with concrete instructions (use `mindspec adr create --supersedes`)
5. Write tests in `plan_test.go`: cite nonexistent ADR → error, cite Superseded ADR → warning, cite Proposed ADR → warning, missing `## ADR Fitness` → warning, present `## ADR Fitness` → no warning.
6. `make build && make test`.

**Verification**
- [ ] Nonexistent ADR citation → error `adr-cite-missing`
- [ ] Superseded ADR citation → warning `adr-cite-superseded`
- [ ] Proposed ADR citation → warning `adr-cite-proposed`
- [ ] Missing `## ADR Fitness` → warning `adr-fitness-missing`
- [ ] Present `## ADR Fitness` → no warning
- [ ] Plan mode template instructs active evaluation, not passive compliance
- [ ] `make test` passes

**Depends on**: Bead 1

---

## Dependency Graph

```
Bead 1 (shared ADR parse)
  ├── Bead 2 (create + supersede)
  ├── Bead 3 (list + show)
  └── Bead 4 (validation + instruct template)
```
