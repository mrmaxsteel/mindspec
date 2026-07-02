---
status: Draft
approved_at: ""
approved_by: ""
---
# Spec 107-cleanup-deadcode-dry-wave1: Cleanup wave 1: dead-code sweep, setup managed-block unification (safeio), complete perf pair, guardrails restoration

## Goal

Land the four zero-behavioral-risk cleanup items from the 2026-07-02 repo review (`review/repo-review-2026-07-02.md` §4, order 1–3) as one bounded wave: delete confirmed-dead code, unify the triplicated agent-setup managed-block installer through `safeio` (closing a real codex symlink-safety gap), collapse the `mindspec complete` children/epic subprocess fan-out, and restore the `## Bead-loop guardrails (mindspec)` section that CLAUDE.md and every `ms-*` orchestration skill already reference. Target outcome: fewer lines to maintain, one fewer security hole, a faster hot-path lifecycle command, and no dangling documentation reference — with byte-identical externally observable behavior everywhere except the codex symlink refusal (a fix) and the reduced `bd` subprocess count (an optimization).

## Background

The 2026-07-02 review swept 341 tracked Go files with `deadcode` (with and without `-test`), `golangci-lint`, and four subsystem review agents, verifying every finding against source. It ranked a cleanup order (§4). This spec is **wave 1** — the first three ordered items plus one documentation-integrity fix — chosen because each is low-risk and independently verifiable by a review panel against the report:

- **Dead code (§1)** is confirmed unreachable even from tests, so deletion is a pure subtraction (~500 lines). The stale `.golangci.yml` carve-outs reference code that no longer exists.
- **Setup managed-block unification (§2 DRY #1)** is the one DRY item that also fixes a live defect: `internal/setup/codex.go:68,79,96` writes the managed `AGENTS.md` block with plain `os.WriteFile`/`os.OpenFile`, missing the `safeio` symlink protection that `claude.go` and `copilot.go` both have — and that has dedicated refusal tests for the other two agents but not codex.
- **`complete` perf pair (§3 Perf #2 + #3, = §2 DRY #5)** is two small edits on the most-used lifecycle command: `queryAllChildren` fans out one `bd list --parent --status=<s>` per status where `phase.fetchChildren` already does it in one comma-joined call; and `complete.Run` rebuilds the immutable spec→epic mapping four times, each spawning a throwaway `phase.NewCache()` + `bd list --type=epic`.
- **Guardrails restoration** repairs a dangling reference: `CLAUDE.md:43` and the surviving `ms-*` skills say "See AGENTS.md § Bead-loop guardrails (mindspec)", but that section is absent from `AGENTS.md` (verified — the file's headers stop at `## Session Completion` / `## Architecture` with no guardrails section). The `specInit` command alias also carries a byte-identical 42-line copy of `specCreate`'s `RunE`.

Wave 2 (frontmatter consolidation, ownership/ADR validation caching, markdown-section and TOML-parser unification, harness and bd-show helper dedup) is deliberately deferred — those are larger, higher-blast-radius changes that deserve their own spec.

## Impacted Domains

- **workflow**: Owns the bulk of the edits — `internal/hook` (dead helper cluster), `internal/setup` (managed-block unification + `hasManagedBlock` deletion), `internal/complete` (perf pair), `internal/next` (`findRoot` deletion), `internal/validate` (dead shims), `internal/doctor` (`Run` deletion), `internal/panel` (`skipHumanHint` deletion), `internal/layout` (`Mover.With*` deletion), `cmd/**` (`hook.go` no-op, `state.go` dead flags, `spec.go`/`spec_init.go` alias dedup), and `plugins/mindspec` (`SkillNames`/`sortStrings` deletion).
- **core**: `internal/phase` (delete dead `FindActiveBeadForEpic`; export the shared children-query helper for `complete`) and `internal/recording` (`DefaultCodexConfigPath` deletion).
- **execution**: `internal/gitutil` (`MainWorktreePath`/`IsMainWorktree` cluster deletion), `internal/safeio` (the `WriteFileNoSymlink`/`OpenAppendNoSymlink` sink all three setup writers must route through), and `internal/harness` (`filterEnv`, `assertCommandUsedFlag`, `assertCleanWorktree` deletions).
- **context-system**: `internal/contextpack` (`NewADRStore`, `readFileContent` deletions).

Note: the only touched non-Go paths are root-level `.golangci.yml` and root-level `AGENTS.md`; because they are not Go source, the doc-sync classifier and divergence attribution never treat them as source and never require a domain claim. `internal/trace` — which is claimed by zero OWNERSHIP.yaml files — is therefore left untouched (its dead `Event.MarshalJSON` is deferred to wave 2; see Out of Scope), so this spec introduces no domain-unclaimed Go source change.

## ADR Touchpoints

- [ADR-0030](../../adr/ADR-0030-executor-boundary.md): Executor as the git/process I/O boundary (Domains: execution, validation, lifecycle, lint). Governs how `internal/complete` and the shared `internal/phase` helper issue `bd` subprocesses — the perf pair reduces the count without leaking new direct `exec` calls. Finalized alongside the lint boundary this spec's `.golangci.yml` carve-out cleanup and dead `gitutil` cluster removal sit against.
- [ADR-0034](../../adr/ADR-0034-ceremony-collapse.md): Ceremony Collapse — single-bead lifecycle (Domain: workflow). `mindspec complete` is the single-bead lifecycle command whose children fan-out (5→1) and 4× spec→epic re-resolution this spec optimizes; behavior must stay identical.
- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): Panel Gate as Enforced Contract (Domains: workflow, execution). The restored `## Bead-loop guardrails (mindspec)` section codifies the panel-gate-before-`mindspec complete` rule this ADR enforces — the guardrails text is the human-readable projection of that contract.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md): Ownership Discovery — zero framework cognition for OWNERSHIP.yaml and source_globs (Domains: workflow, validation, doc-sync, ownership). Governs how the touched files map to Impacted Domains and the `internal/<domain>/**` fallback; directly relevant because the deletions and setup/complete refactors change source claimed across the impacted domains, and this ADR's attribution rules are why `internal/trace` (claimed by no manifest) is descoped while root-level `.golangci.yml` and `AGENTS.md` (not Go source) are never attributed — keeping the doc-sync gate green (no new drift) after the sweep.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md): Agent Error Contract — recovery lines and exit codes (Domains: workflow, execution, core). The `internal/complete` + `internal/phase` perf refactor runs on the enforcement path, so every gate-failure branch it touches must still emit the recovery-line error contract (`internal/guard/recovery.go` format) unchanged; the refactor reduces subprocess count without altering error/exit-code behavior.
- [ADR-0033](../../adr/ADR-0033-tokenizer-interface.md): Pluggable Tokenizer Interface and Deterministic Context Pack Budgeting (Domain: context-system). The `contextpack.NewADRStore`/`readFileContent` deletions are confined to dead helpers and must not touch the deterministic budgeting path (`BuildBead`) or the tokenizer interface; the ADR fixes those as the invariant this cleanup preserves.

## Requirements

1. **Dead-code deletion.** Delete every confirmed-dead function/cluster enumerated in report §1: the `internal/hook/helpers.go` cluster (`hasPathPrefix`, `stripEnvPrefixes`, `parseEnvPrefixes`, `isEnvVarName`, `getCwd`; keep live `dirExists`); `gitutil.MainWorktreePath` + `IsMainWorktree`; `plugins/mindspec/embed.go` `SkillNames` + `sortStrings`; `layout.Mover.WithPlan`/`WithRules`/`WithRootDocs`; `cmd/mindspec/hook.go:191` no-op `SetUsageTemplate`; `doctor.Run`; `phase.FindActiveBeadForEpic`; `validate.SpecStatusFromBytes`/`SpecIsApproved`/`IsDomainCoveredCtx`/`BeadID` (dead re-export shim); `contextpack.NewADRStore`/`readFileContent`; `next.findRoot`; `recording.DefaultCodexConfigPath`; `setup.hasManagedBlock`; `harness.filterEnv`/`assertCommandUsedFlag`/`assertCleanWorktree`; `panel` const `skipHumanHint`; and `cmd/mindspec/state.go` `--mode`/`--spec`/`--bead` flags on the deprecated no-op `state set`. (The report's `trace.Event.MarshalJSON` finding is deliberately excluded — see Out of Scope.) After deletion, `deadcode -test` over `./cmd/... ./internal/... ./plugins/...` reports zero findings outside the intentional `internal/harness` LLM-eval test seams and the deferred `internal/trace` finding.
2. **Stale lint carve-out removal.** Remove the three stale `unparam` exclusions from `.golangci.yml` that reference removed/dead code: `internal/brownfield/plan.go` (package no longer exists), `internal/contextpack/builder.go` `isNeighbor` (function no longer exists), and `internal/next/beads.go` `findRoot` (deleted in R1). `golangci-lint run` passes with these three carve-outs gone and no new lint findings introduced.
3. **Setup managed-block helper extraction.** Extract one `ensureManagedDoc`-style helper (signature carrying root, relative path, full-file content, append block, and the managed-block check) that `ensureClaudeMD` (`claude.go`), `ensureAgentsMD` (`codex.go`), and `ensureCopilotInstructions` (`copilot.go`) all route through, with every write/append going through `safeio.WriteFileNoSymlink`/`safeio.OpenAppendNoSymlink`. After the change, `internal/setup/codex.go` contains no plain `os.WriteFile`/`os.OpenFile` call for the managed `AGENTS.md` document. Externally observable setup output (the produced file contents for a non-symlinked target) is byte-identical to today for all three agents.
4. **Codex symlink refusal + test.** `mindspec setup codex` targeting a directory whose `AGENTS.md` is a symlink refuses to write through it (matching claude and copilot behavior), verified by a new test `TestRunCodex_RefusesSymlinkedAGENTSmd` in `internal/setup/symlink_refusal_test.go` mirroring the existing `TestRunClaude_RefusesSymlinkedCLAUDEmd` and `TestRunCopilot_RefusesSymlinkedInstructions`.
5. **chainBeads fold.** Fold `chainBeadsSetup` (`claude.go`) and `chainBeadsSetupCodex` (`codex.go`) — which differ only by the agent identifier string — into one parameterized helper, preserving each agent's current chained-setup output.
6. **Single children query in `complete`.** Replace `complete.queryAllChildren`'s per-status `bd list --parent --status=<s>` loop (`complete.go:884-905`) with a single comma-joined-status query. Add a new exported wrapper `phase.FetchChildren(epicID string) ([]ChildInfo, error)` that delegates to the existing package-private `phase.fetchChildren` (`cache.go:213-230`, the single comma-joined `--status` uncached query); `internal/complete` calls `phase.FetchChildren` directly at the post-close children site so the read stays fresh, and the memoized `Cache.GetChildren` path is left untouched. This is the minimal public-surface widening and stops the copy from drifting again. After the change, one `mindspec complete` run issues exactly one `bd list --parent` subprocess for the children query (was one per status, ~5). The returned child set is unchanged.
7. **Resolve spec→epic once.** Resolve the immutable spec→epic mapping a single time near the top of `complete.Run` and reuse it, instead of constructing a throwaway `phase.NewCache()` + `bd list --type=epic` at each of `complete.go:223,228,716,781`. The **post-close children query must stay fresh** (re-issued after `complete` mutates bd mid-run) — only the immutable spec→epic lookup may be reused. After the change, `complete.Run` issues at most one `bd list --type=epic` subprocess for that lookup.
8. **Guardrails section restoration.** Add a `## Bead-loop guardrails (mindspec)` section to `AGENTS.md` carrying the canonical fences that `CLAUDE.md:43` and the `ms-*` skills point at: only the cycle runs `mindspec complete`, and only after the panel gate passes; never a raw `git merge bead/<id>`; exactly one `git push` at end-of-spec; subagents make exactly one commit; tests must PASS before completion. After the change, the cross-reference in `CLAUDE.md` resolves to a real, non-empty section.
9. **specInit alias dedup.** Make `specInitCmd` (`cmd/mindspec/spec_init.go:20-61`) reuse `specCreateCmd`'s `RunE` instead of carrying the byte-identical 42-line copy (`cmd/mindspec/spec.go:30-71`), so a change to spec-create behavior updates the hidden `spec init` alias automatically. `mindspec spec init` behavior is unchanged.
10. **No behavioral regressions.** No function removed by R1 is referenced by any live (non-test-seam) caller; `go test ./...` passes with all existing tests green plus the new codex symlink-refusal test.

## Scope

### In Scope
- Dead-code deletions listed in R1 across `internal/{hook,gitutil,layout,doctor,phase,validate,contextpack,next,recording,setup,harness,panel}`, `cmd/mindspec/{hook.go,state.go}`, and `plugins/mindspec/embed.go`. (`internal/trace` is deliberately excluded — see Out of Scope.)
- The three stale `.golangci.yml` `unparam` carve-outs (R2).
- `internal/setup` managed-block unification through `safeio` + codex symlink fix + refusal test + `chainBeads*` fold (R3–R5).
- `internal/complete` + `internal/phase` perf pair (R6–R7).
- `AGENTS.md` guardrails section (R8) and the `cmd/mindspec` spec-init alias dedup (R9).

### Out of Scope
- **Wave 2 items** (deferred to a separate spec): frontmatter fence-scanner consolidation (DRY #3), override-flag validation dedup (DRY #2), TOML-parser consolidation (DRY #4), markdown `## section` extraction helper (DRY #6), `bd show ... [0]` `ShowOne` helper (DRY #7), OTEL env-var key-map dedup (DRY #8), `git status --porcelain` tokenizer share (DRY #9), remaining `cmd/mindspec` wiring dedup beyond spec-init (DRY #10), harness internal refactors (DRY #11), and the low-impact batch (DRY #12–18).
- **Wave 2 perf items**: OWNERSHIP.yaml load hoisting (Perf #1), ADR re-parse memoization (Perf #4), doctor per-domain repo walk (Perf #5), migrate `check-ignore` batching (Perf #6), and the misc prealloc/analyzer items (Perf #7).
- Any change to the deprecated-but-intentional APIs the report explicitly flags as live (`contextpack.RenderBeadContext` no-budget path; the `internal/harness`, `internal/lint`, `internal/specgate` test-guard packages).
- **`internal/trace/event.go` `Event.MarshalJSON` removal** and any `internal/trace` OWNERSHIP.yaml claim — deferred to wave 2. `internal/trace` is claimed by zero manifests, so deleting a symbol there would trip the `adr-divergence-unowned` gate at `mindspec complete`; wave 2 will land the ownership claim first, then the deletion.

## Non-Goals

- This spec does not change any user-facing command output, flag surface, or file format, except (a) `mindspec setup codex` now refuses a symlinked `AGENTS.md` (a security fix) and (b) `mindspec complete` spawns fewer `bd` subprocesses (an internal optimization).
- It does not attempt broader architectural refactors, new abstractions beyond the two small shared helpers (`ensureManagedDoc`, `phase.FetchChildren`), or performance work outside the two named `complete` hot paths.

## Acceptance Criteria

- [ ] `go run golang.org/x/tools/cmd/deadcode@latest -test ./cmd/... ./internal/... ./plugins/...` reports zero findings outside the `internal/harness` LLM-eval test seams and the deferred `internal/trace/event.go` `Event.MarshalJSON` finding (wave 2).
- [ ] `golangci-lint run` passes with the three stale `unparam` carve-outs removed from `.golangci.yml` and no new findings introduced.
- [ ] `internal/setup/codex.go` contains no `os.WriteFile`/`os.OpenFile` call for the managed `AGENTS.md` document; all three agents (claude/codex/copilot) write the managed block via `safeio.WriteFileNoSymlink`/`OpenAppendNoSymlink` through the shared helper.
- [ ] `mindspec setup codex` into a directory with a symlinked `AGENTS.md` refuses to write; new test `TestRunCodex_RefusesSymlinkedAGENTSmd` passes.
- [ ] `mindspec complete` issues exactly one `bd list --parent` subprocess for the children query (was ~5), asserted via the `listJSONFn` call count in a test.
- [ ] `complete.Run` constructs the immutable spec→epic mapping once (at most one `bd list --type=epic` subprocess for that lookup), while the post-close children query still reflects bd state mutated mid-run.
- [ ] `AGENTS.md` contains a non-empty `## Bead-loop guardrails (mindspec)` section carrying all five canonical fences, and the `CLAUDE.md` cross-reference resolves to it.
- [ ] `cmd/mindspec/spec_init.go` no longer duplicates the 42-line `RunE` body; `specInitCmd` reuses `specCreateCmd.RunE`.
- [ ] `go test ./...` passes (all existing tests green plus the new codex refusal test).

## Validation Proofs

- `go run golang.org/x/tools/cmd/deadcode@latest -test ./cmd/... ./internal/... ./plugins/...`: zero findings outside `internal/harness`.
- `golangci-lint run`: clean, with the three carve-outs removed.
- `go test ./internal/setup/... -run RefusesSymlinked`: codex/claude/copilot refusal tests all pass.
- `go test ./internal/complete/... ./internal/phase/...`: children-query-count and spec→epic-once assertions pass; child sets unchanged.
- `go test ./...`: full suite green.
- `grep -c "Bead-loop guardrails (mindspec)" AGENTS.md`: ≥ 1.

## Open Questions

_All open questions are resolved; both resolutions are folded into the requirements and scope above._

- **RESOLVED — domain-unclaimed path attribution.** `internal/trace` is claimed by zero OWNERSHIP.yaml files, so deleting `Event.MarshalJSON` there would trip `adr-divergence-unowned` at `mindspec complete`. That deletion is therefore dropped from R1 and deferred to wave 2 (see Out of Scope), leaving this spec with no domain-unclaimed Go source change. The two remaining non-domain paths — root-level `.golangci.yml` and `AGENTS.md` — are not Go source, so the doc-sync classifier and divergence attribution never see them as source files and never require a domain claim.
- **RESOLVED — shared children helper shape.** Add a new exported wrapper `phase.FetchChildren(epicID string) ([]ChildInfo, error)` that delegates to the existing package-private `phase.fetchChildren` (single comma-joined `--status` query, uncached). `internal/complete` calls it directly at the post-close children site so the read stays fresh; the memoized `Cache.GetChildren` path is untouched. This is the minimal public-surface widening (R6).

## Approval

- **Status**: DRAFT
- **Approved By**: -
- **Approval Date**: -
- **Notes**: -
