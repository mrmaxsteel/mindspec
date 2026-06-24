# Spec 012-adr-lifecycle: ADR Lifecycle Tooling

## Goal

Give developers CLI commands to create, list, and supersede Architecture Decision Records, strengthen validation of ADR citations in plans, and make ADR fitness evaluation an explicit step in every planning session — so ADR governance is enforced by tooling and workflow rather than convention alone.

## Background

MindSpec treats ADRs as governed primitives with a defined lifecycle (Proposed → Accepted → Superseded) and explicit superseding links. Eight ADRs exist today, all created manually using the template at `docs/templates/adr.md`. The context pack builder already parses ADR metadata (status, domains) and filters to include only Accepted ADRs for relevant domains.

Current pain points:
1. **Manual ID assignment**: must scan `docs/adr/` and pick the next number by hand
2. **Manual template filling**: copy template, fill date/status/domains, remember the section structure
3. **Superseding is error-prone**: must update both old ADR (add `Superseded-by`) and new ADR (add `Supersedes`), easy to forget one side
4. **No quick overview**: listing ADRs by status or domain requires manual inspection
5. **Citation validation is shallow**: `mindspec validate plan` warns if `adr_citations` is empty but doesn't verify cited ADRs exist or are Accepted
6. **ADR fitness evaluation is passive**: the plan mode template says "Propose new ADRs if divergence detected" but doesn't instruct the agent to actively evaluate whether existing ADRs are still the right choice. Agents tend toward blind compliance — citing ADRs without questioning whether a better design would diverge from them

## Impacted Domains

- **core**: New CLI subcommands (`adr create`, `adr list`, `adr show`) register under the root command
- **workflow**: Validation of ADR citations in plans becomes stricter (existence + status checks)
- **context-system**: Existing `ScanADRs()`/`FilterADRs()` parsing logic may be shared or refactored into a common ADR package

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): ADRs are first-class DDD artifacts used in context pack assembly
- [ADR-0004](../../adr/ADR-0004.md): Go as implementation language

## Requirements

### `mindspec adr create <title>`

1. Auto-generates the next ADR number by scanning `docs/adr/ADR-*.md` filenames, finding the highest number, and incrementing by 1. Pads to 4 digits (e.g., `ADR-0009`).

2. Creates `docs/adr/ADR-NNNN.md` using the existing template at `docs/templates/adr.md`, pre-filling:
   - `NNNN` → computed ID
   - `<Title>` → the provided title
   - `<YYYY-MM-DD>` → today's date
   - Status → `Proposed`

3. Accepts optional flags:
   - `--domain <name>[,<name>...]` — pre-fills the Domain(s) field
   - `--supersedes <ADR-NNNN>` — pre-fills the Supersedes field and triggers the superseding workflow (see Req 6)

4. Prints the created file path and a message suggesting the user fill in the Context and Decision sections.

5. **Title validation**: title must be non-empty. If empty, exit with error.

### Superseding Workflow

6. When `--supersedes <ADR-NNNN>` is provided:
   - Verifies the referenced ADR file exists; errors if not
   - Sets `Supersedes: ADR-NNNN` in the new ADR
   - Updates the old ADR's `Superseded-by` field from `n/a` to the new ADR's ID
   - Copies the old ADR's Domain(s) to the new ADR (unless `--domain` explicitly overrides)
   - Prints a message noting both files were modified

### `mindspec adr list`

7. Scans `docs/adr/ADR-*.md` and prints a table with columns: **ID**, **Status**, **Domain(s)**, **Title** (from the `# ADR-NNNN: <Title>` heading). Sorted by ID.

8. Accepts optional filter flags:
   - `--status <status>` — filter to a single status (e.g., `--status accepted`)
   - `--domain <name>` — filter to ADRs affecting a given domain

9. If no ADRs exist, prints "No ADRs found."

### `mindspec adr show <id>`

10. Prints a concise summary of a single ADR: ID, title, status, date, domain(s), supersedes/superseded-by links, and the Decision section content. Useful for quick agent reference without reading the full document.

11. `--json` flag emits JSON for programmatic consumption.

12. If the ADR does not exist, exits with error and non-zero status.

### Enhanced Validation

13. `mindspec validate plan` gains additional checks for `adr_citations`:
    - **Existence check**: each cited ADR ID must correspond to a file in `docs/adr/`. Missing → error.
    - **Status check**: each cited ADR must have status `Accepted`. Citing a `Proposed` or `Superseded` ADR → warning with message suggesting the correct action (accept it or cite the superseding ADR).

### ADR Fitness Evaluation in Plan Mode

14. **Plan mode instruct template update** (`internal/instruct/templates/plan.md`): Add an explicit "ADR Fitness Evaluation" step to the planning workflow. The updated template must instruct the agent to:
    - After reviewing accepted ADRs for impacted domains, actively evaluate whether each relevant ADR still represents the best architectural choice for the work being planned
    - Prefer adherence to existing ADRs when they are sound
    - When a better design would diverge from an accepted ADR, propose the divergence with justification rather than silently conforming to a suboptimal architecture
    - Treat ADR divergence as a human gate: stop planning, present the divergence and the proposed alternative, and wait for approval before proceeding

    The framing must avoid two failure modes:
    - **Blind compliance**: planning around an ADR that should be superseded, producing a worse design
    - **Casual divergence**: ignoring ADRs without explicit justification and human approval

15. **Plan.md ADR fitness section**: plans must include a `## ADR Fitness` section (after the frontmatter, before bead sections) where the agent documents its evaluation. Content is either:
    - "All cited ADRs remain appropriate for this work" (with brief rationale), or
    - A divergence proposal: which ADR, why it should be superseded, and the proposed alternative approach

16. **Plan validation for ADR fitness**: `mindspec validate plan` checks that a `## ADR Fitness` section exists. Missing → warning ("plan should include ADR fitness evaluation").

## Scope

### In Scope

- `cmd/mindspec/adr.go` — cobra command wiring for `adr create`, `adr list`, `adr show`
- `internal/adr/create.go` — ID generation, template filling, file creation
- `internal/adr/supersede.go` — superseding workflow (update both ADRs)
- `internal/adr/list.go` — scan, parse, filter, format
- `internal/adr/show.go` — single ADR summary + JSON output
- `internal/adr/parse.go` — shared ADR metadata parsing (may refactor from `internal/contextpack/adr.go`)
- `internal/validate/plan.go` — enhanced ADR citation checks + ADR fitness section check
- `internal/instruct/templates/plan.md` — ADR fitness evaluation step added to plan mode guidance
- Unit tests for all new functions

### Out of Scope

- ADR acceptance workflow (changing status from Proposed → Accepted is a manual edit or future `/adr-approve` command)
- Automated divergence detection at implementation time (the plan mode evaluation is agent-driven, not static analysis)
- Modifying the ADR template format itself
- Domain-level ADR subdirectories (`docs/domains/<domain>/adr/`)

## Non-Goals

- Replacing human judgment on architectural decisions with automation
- Enforcing that every plan cites at least one ADR (the existing warning is sufficient)
- Interactive ADR editing (this is a creation + query tool)

## Acceptance Criteria

- [ ] `mindspec adr create "Use Redis for caching"` creates `docs/adr/ADR-0009.md` with correct ID, date, title, and Proposed status
- [ ] `mindspec adr create "Replace DDD approach" --supersedes ADR-0001 --domain core,workflow` creates the new ADR and updates ADR-0001's Superseded-by field
- [ ] `mindspec adr create --supersedes ADR-9999 "..."` fails with "ADR-9999 not found"
- [ ] `mindspec adr list` shows all 8+ ADRs in a table with ID, Status, Domain(s), Title
- [ ] `mindspec adr list --status accepted` filters to only Accepted ADRs
- [ ] `mindspec adr list --domain workflow` filters to ADRs affecting the workflow domain
- [ ] `mindspec adr show ADR-0001` prints a concise summary including the Decision section
- [ ] `mindspec adr show ADR-0001 --json` outputs valid JSON
- [ ] `mindspec validate plan` on a plan citing a nonexistent ADR reports an error
- [ ] `mindspec validate plan` on a plan citing a Superseded ADR reports a warning
- [ ] Plan mode instruct template includes an explicit ADR fitness evaluation step that instructs active evaluation, not passive compliance
- [ ] Plan mode instruct template frames divergence as permissible-with-justification, gated on human approval
- [ ] `mindspec validate plan` on a plan missing `## ADR Fitness` section reports a warning
- [ ] A plan with `## ADR Fitness` section passes validation without the warning
- [ ] All new code has unit tests; `make test` passes
- [ ] `make build` succeeds

## Validation Proofs

- `./bin/mindspec adr create "Test Decision" --domain core && cat docs/adr/ADR-0009.md`: Should show populated template with correct ID, date, title
- `./bin/mindspec adr list`: Should show table of all ADRs
- `./bin/mindspec adr list --status accepted`: Should show only Accepted ADRs
- `./bin/mindspec adr show ADR-0001`: Should print concise summary
- `./bin/mindspec adr show ADR-0001 --json | jq .status`: Should output "Accepted"
- `./bin/mindspec instruct`: Plan mode template should include "ADR Fitness Evaluation" guidance (verify by entering plan mode)
- `make test`: All tests pass

## Open Questions

(none)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-02-13
- **Notes**: Approved via mindspec approve spec