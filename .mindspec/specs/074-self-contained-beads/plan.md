---
adr_citations:
    - id: ADR-0023
      sections:
        - Decision
    - id: ADR-0012
      sections:
        - Decision
approved_at: "2026-03-08T08:16:09Z"
approved_by: user
bead_ids:
    - mindspec-p68b.1
    - mindspec-p68b.2
    - mindspec-p68b.3
last_updated: "2026-03-08"
spec_id: 074-self-contained-beads
status: Approved
version: 1
---
# Plan: 074-self-contained-beads — Self-Contained Beads

## ADR Fitness

**ADR-0023 (Beads as Single State Authority)**: Sound and directly extended by this spec. ADR-0023 established beads as the state authority (lifecycle phase, active spec). This plan extends that to context authority. No divergence — this is the natural next step.

**ADR-0012 (Compose with External CLIs)**: Sound. We continue to call `bd create` and `bd show` via `exec.Command()` at call sites. The new render helper reads JSON from `bd show` output — it doesn't wrap the CLI, it processes its output. No divergence.

## Testing Strategy

- **Unit tests**: Test `createImplementationBeads()` with a stubbed `bd` runner to verify all flags are passed correctly. Test the new `RenderBeadContext()` helper with canned JSON.
- **Integration test**: `TestApproveSpec` end-to-end flow verifying beads have populated fields after plan approval (uses stubbed bd).
- **LLM harness**: Existing harness tests must pass with bead-sourced context (no new LLM tests needed — the existing tests exercise `mindspec next` and `mindspec instruct` which will use the new code path).
- **Manual validation**: `bd show <bead-id> --json | jq '.description'` returns non-empty content after plan approval.

## Bead 1: Populate Bead Fields at Plan Approval

Modify `createImplementationBeads()` in `internal/approve/plan.go` to pass `--description`, `--acceptance-criteria`, `--design`, and `--metadata` when creating each bead.

**Steps**
1. In `createImplementationBeads()`, read spec.md and extract `## Requirements` and `## Acceptance Criteria` sections using `contextpack.ExtractSection()`
2. Scan ADRs referenced in the spec's `## ADR Touchpoints` section, extract their `## Decision` sections, concatenate as snapshot text
3. For each bead section from `validate.ParseBeadSections()`, extract the full `## Bead N:` markdown block as the description
4. Extract file paths from each work chunk using `contextpack.ExtractFilePathsFromText()`
5. Build metadata JSON with `spec_id` and `file_paths`
6. Pass `--description`, `--acceptance-criteria`, `--design`, `--metadata` flags to `bd create`
7. Add unit test verifying all flags are passed correctly for each bead

**Verification**
- [ ] `go test ./internal/approve/ -run TestCreateImplementationBeads` passes with populated fields
- [ ] `go test ./internal/approve/ -v` — all existing tests still pass

**Depends on**
None

## Bead 2: Replace Primer Consumers with bd show Rendering

Replace all call sites that use `BuildBeadPrimer()`/`RenderBeadPrimer()` with a new `RenderBeadContext()` helper that reads from `bd show --json` output. Then delete the primer functions.

**Steps**
1. Create `internal/contextpack/beadctx.go` with `RenderBeadContext(beadID string) (string, error)` — calls `bd show <beadID> --json`, parses the response, renders markdown with: title, description (work chunk), acceptance criteria, design (requirements + ADR snapshots), file paths from metadata
2. Replace `BuildBeadPrimer`/`RenderBeadPrimer` call in `cmd/mindspec/next.go:242-257` with `RenderBeadContext()`
3. Replace `BuildBeadPrimer`/`RenderBeadPrimer` call in `cmd/mindspec/context.go:16-43` with `RenderBeadContext()`
4. Replace `BuildBeadPrimer`/`RenderBeadPrimer` call in `internal/instruct/instruct.go:90-96` with `RenderBeadContext()`
5. Delete `internal/contextpack/primer.go` (BuildBeadPrimer, RenderBeadPrimer, extractBeadSection, and helpers)
6. Delete or update `internal/contextpack/primer_test.go`
7. Add unit test for `RenderBeadContext()` with canned `bd show` JSON

**Verification**
- [ ] `grep -r "BuildBeadPrimer\|RenderBeadPrimer" internal/` — no matches
- [ ] `go test ./internal/contextpack/ -v` passes
- [ ] `go test ./cmd/mindspec/ -v` passes
- [ ] `make build` succeeds

**Depends on**
Bead 1

## Bead 3: Plan Re-Approval Safeguards

Add close-and-recreate logic when `createImplementationBeads()` is called for a spec that already has implementation beads.

**Steps**
1. At the start of `createImplementationBeads()`, query existing children of the epic via `bead.ListJSON("--parent", epicID)`
2. If children exist and any are `in_progress` or `closed`, return an error: `"cannot re-approve plan: bead <id> is <status> — close or complete active work first"`
3. If children exist and all are `open`, close them with reason `"superseded by plan v<N>"` (derive version from plan frontmatter)
4. Proceed with normal bead creation
5. Add unit test for re-approval: existing open beads are closed, in-progress beads cause error

**Verification**
- [ ] `go test ./internal/approve/ -run TestReApproval` passes
- [ ] `go test ./internal/approve/ -v` — all tests pass

**Depends on**
Bead 1

## Provenance

| Spec Acceptance Criterion | Satisfied By |
|:---|:---|
| `bd show` returns populated `description` with plan work chunk | Bead 1 step 3, 6 |
| `bd show` returns populated `acceptance_criteria` from spec | Bead 1 step 1, 6 |
| `bd show` returns populated `design` with requirements + ADR decisions | Bead 1 step 1, 2, 6 |
| `bd show` returns `metadata` with `spec_id` and `file_paths` | Bead 1 step 4, 5, 6 |
| `BuildBeadPrimer()` and `RenderBeadPrimer()` deleted | Bead 2 step 5, 6 |
| `mindspec instruct` renders from `bd show` | Bead 2 step 4 |
| `mindspec next` renders from `bd show` | Bead 2 step 2 |
| Existing LLM harness tests pass | Bead 2 verification |
| `mindspec approve plan` creates populated beads | Bead 1 step 7 |
| Plan re-approval closes existing beads / errors on active work | Bead 3 steps 2-4 |
