---
status: Draft
spec_id: 107-cleanup-deadcode-dry-wave1
version: "1"
adr_citations:
  - id: ADR-0030
    sections: ["Executor as the git/process I/O boundary"]
  - id: ADR-0033
    sections: ["Deterministic context-pack budgeting invariant"]
  - id: ADR-0034
    sections: ["Ceremony Collapse — single-bead lifecycle"]
  - id: ADR-0035
    sections: ["Agent Error Contract — recovery lines & exit codes"]
  - id: ADR-0036
    sections: ["Ownership Discovery — attribution & unowned-path rules"]
  - id: ADR-0037
    sections: ["Panel Gate as Enforced Contract"]
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/hook/helpers.go
      - internal/gitutil/gitops.go
      - plugins/mindspec/embed.go
      - internal/layout/mover.go
      - internal/doctor/doctor.go
      - internal/validate/frontmatter.go
      - internal/validate/plan.go
      - internal/validate/specid.go
      - internal/contextpack/adr.go
      - internal/contextpack/builder.go
      - internal/next/beads.go
      - internal/recording/codex_bootstrap.go
      - internal/harness/agent.go
      - internal/harness/asserts.go
      - internal/panel/gate.go
      - internal/instruct/instruct.go
      - cmd/mindspec/hook.go
      - cmd/mindspec/state.go
      - .mindspec/domains/workflow/architecture.md
      - .mindspec/domains/execution/architecture.md
      - .mindspec/domains/core/architecture.md
      - .mindspec/domains/context-system/architecture.md
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/setup/claude.go
      - internal/setup/codex.go
      - internal/setup/copilot.go
      - internal/setup/symlink_refusal_test.go
      - .mindspec/domains/workflow/architecture.md
  - id: 3
    depends_on: []
    key_file_paths:
      - internal/complete/complete.go
      - internal/complete/complete_test.go
      - internal/phase/cache.go
      - internal/phase/derive.go
      - .mindspec/domains/workflow/architecture.md
      - .mindspec/domains/core/architecture.md
  - id: 4
    depends_on: []
    key_file_paths:
      - AGENTS.md
      - cmd/mindspec/spec.go
      - cmd/mindspec/spec_init.go
      - .mindspec/domains/workflow/architecture.md
---
# Plan: 107-cleanup-deadcode-dry-wave1

## ADR Fitness

All six cited ADRs are Accepted and each declares at least one of this spec's four
impacted domains (workflow, core, execution, context-system). This wave adheres to
every one of them — it introduces no new abstraction that any ADR governs and
changes no externally observable behavior an ADR fixes, except the two the spec
already scopes (the codex symlink refusal, a fix, and the reduced `bd` subprocess
count, an optimization). No divergence is proposed.

- **ADR-0030 — Executor as the git/process I/O boundary** (execution). The
  `complete` perf pair (R5/R6) reduces how many `bd` subprocesses
  `internal/complete` and `internal/phase` spawn, but every call stays behind the
  existing `phase` seam — the change routes the post-close children read through
  the exported `phase.FetchChildren` wrapper rather than `complete`'s private copy,
  and reuses one spec→epic lookup instead of four throwaway `phase.NewCache()`
  constructions. No new direct `exec.Command` is introduced. **Fit: adhered** —
  the refactor tightens the boundary (one owner for the children query) rather than
  crossing it.

- **ADR-0033 — Pluggable Tokenizer & Deterministic Context-Pack Budgeting**
  (context-system). The `contextpack.NewADRStore` and `contextpack.readFileContent`
  deletions (R1) are confined to dead helpers with zero live callers; neither sits
  on the deterministic budgeting path (`BuildBead`) or the tokenizer interface the
  ADR fixes. **Fit: adhered** — pure subtraction outside the invariant.

- **ADR-0034 — Ceremony Collapse (single-bead lifecycle)** (workflow).
  `mindspec complete` is the single-bead lifecycle command the perf pair (R5/R6)
  optimizes. The returned child set and every gate outcome must stay byte-identical;
  only the subprocess count drops. **Fit: adhered** — the optimization preserves the
  collapsed-ceremony contract, proven by the existing `internal/complete` suite
  staying green plus new subprocess-count assertions.

- **ADR-0035 — Agent Error Contract (recovery lines & exit codes)** (workflow,
  execution, core). The `complete` + `phase` refactor runs on the enforcement path,
  so every gate-failure branch it touches must still emit the recovery-line error
  contract and its exit code unchanged. The change is subprocess-count-only and
  touches no error-formatting branch. **Fit: adhered** — error/exit behavior is
  invariant; regression-guarded by the existing tests.

- **ADR-0036 — Ownership Discovery (attribution & unowned-path rules)** (workflow).
  This ADR decides which touched files map to which domain and which unowned paths
  are safe. The only path this wave touches outside every OWNERSHIP.yaml glob is
  root `AGENTS.md` (R7), which `isDocFile`'s `rootOperatorDocs` allowlist classifies
  as documentation, so divergence never attributes it as source. The two other
  report-flagged unowned paths (`internal/trace`, `.golangci.yml`) are deliberately
  deferred to wave 2 precisely because they would trip `adr-divergence-unowned`.
  This ADR's attribution rules are also what make the per-bead doc-sync obligation
  concrete (see Testing Strategy): the complete-time gate requires a changed doc
  under `.mindspec/domains/<domain>/` for every domain a bead's source touches.
  **Fit: adhered** — the sweep introduces no domain-unclaimed source change, and
  each bead pairs its source edits with the required domain-doc note.

- **ADR-0037 — Panel Gate as Enforced Contract** (workflow, execution). The
  restored `## Bead-loop guardrails (mindspec)` section (R7) is the human-readable
  projection of the panel-gate-before-`mindspec complete` rule this ADR enforces;
  restoring it repairs the dangling `CLAUDE.md` cross-reference without changing the
  gate itself. **Fit: adhered** — documentation-integrity repair of an existing
  enforced contract, zero behavioral change.

## Testing Strategy

The wave is dominated by deletions and two small, behavior-preserving refactors, so
the test approach is "prove nothing changed" plus three targeted new tests. Shared
infrastructure is the existing per-package Go test suites and the `phase`/`complete`
list-JSON seams (`phase.SetListJSONForTest`); no new harness is introduced.

**Two independent completion bars per bead.** A bead is done only when BOTH hold, and
each Verification block below encodes both as commands whose exit status is the
pass/fail:

1. **Green suite** — `go test ./...` exits 0 (all existing tests, plus this bead's new
   tests, pass).
2. **A diff that clears the complete-time doc-sync gate** — `mindspec complete` runs
   `checkInternalPackages` (`internal/validate/docsync.go`), which raises a blocking
   `internal-docs` error unless the bead's diff changes at least one doc under
   `.mindspec/domains/<domain>/` for EVERY domain whose non-test `.go` source the
   bead touches. Editing `AGENTS.md` does NOT satisfy the workflow domain (it is a
   root operator doc, not a domain doc). A green `go test ./...` alone therefore does
   NOT imply `mindspec complete` will pass — each bead must also append a dated
   cleanup note to the `architecture.md` of every domain it edits source in.

Each bead's touched domains were derived by walking its file list against the four
`.mindspec/domains/*/OWNERSHIP.yaml` globs:

- **Bead 1** touches source in all four domains — workflow
  (`internal/{hook,next,doctor,validate,panel,layout,instruct}`, `cmd/mindspec/**`,
  `plugins/mindspec/**`), execution (`internal/{gitutil,harness}`), core
  (`internal/recording`), context-system (`internal/contextpack`) — so it must touch
  all four domain docs. Adds no tests (pure subtraction); correctness proved by a
  clean `deadcode -test`, `go build`, and `go test ./...`.
- **Bead 2** touches source only under `internal/setup/**` (workflow). It routes
  through the EXISTING `internal/safeio` helpers WITHOUT modifying them, so execution
  is not touched and only the workflow domain doc is required. Adds the per-agent
  full-equality managed-doc content test (claude/codex/copilot) and
  `TestRunCodex_RefusesSymlinkedAGENTSmd`; existing setup suite stays green.
- **Bead 3** touches `internal/complete/**` (workflow) and `internal/phase/**` (core)
  → workflow + core domain docs. Re-points `stubChildrenByStatus` at the
  `internal/phase` seam and adds `TestRun_CompletePerfPairSubprocessBudget`
  (`internal/complete`) and `TestFetchChildren` (`internal/phase`).
- **Bead 4** touches `cmd/mindspec/**` (workflow) → workflow domain doc; `AGENTS.md`
  is edited too but does not count toward the gate. Verified by grep on `AGENTS.md`
  and `CLAUDE.md` plus `go test ./cmd/...`.

`_test.go` files are not source (`isSourceFile` excludes them), so a bead's added or
edited tests create no additional domain-doc obligation on their own.

## Bead 1: Dead-code sweep

**Steps**
1. Delete the confirmed-dead workflow-domain symbols: the `internal/hook/helpers.go`
   cluster (`hasPathPrefix`, `stripEnvPrefixes`, `parseEnvPrefixes`, `isEnvVarName`,
   `getCwd` — keep the live `dirExists`); `next.findRoot` (`internal/next/beads.go:53`);
   `doctor.Run` (`internal/doctor/doctor.go:75`); the `panel` const `skipHumanHint`
   (`internal/panel/gate.go:69`); `layout.Mover.WithPlan`/`WithRules`/`WithRootDocs`
   (`internal/layout/mover.go:158-167`); and `plugins/mindspec/embed.go` `SkillNames`
   + `sortStrings`.
2. Delete the dead `internal/validate` shims — `SpecStatusFromBytes` +
   `SpecIsApproved` (`frontmatter.go:46,52`), `IsDomainCoveredCtx` (`plan.go:647`),
   and `BeadID` (`specid.go:24`) — then fix every dangling comment reference to a
   deleted symbol: `internal/instruct/instruct.go:102` (mentions
   `validate.SpecIsApproved`) and the `plan.go:628` comment mentioning
   `IsDomainCoveredCtx`, so no comment names a removed function (panel R3 nit).
3. Delete the execution/core/context-system dead symbols: `gitutil.MainWorktreePath`
   + `IsMainWorktree` (`internal/gitutil/gitops.go:213,230`); `harness.filterEnv`
   (`agent.go:124`) and `harness.assertCommandUsedFlag`/`assertCleanWorktree`
   (`asserts.go:104,294`); `recording.DefaultCodexConfigPath`
   (`internal/recording/codex_bootstrap.go:22`); and `contextpack.NewADRStore`
   (`adr.go:11`) + `readFileContent` (`builder.go:36`).
4. Delete the `cmd/mindspec` dead code: the no-op `SetUsageTemplate` call
   (`hook.go:191`, a `strings.Replace` of a string with itself) and the
   `--mode`/`--spec`/`--bead` flags registered on the deprecated no-op `state set`
   (`state.go:142-144`). Do NOT touch `setup.hasManagedBlock` (Bead 2),
   `phase.FindActiveBeadForEpic` (Bead 3), `internal/trace`, or `.golangci.yml`
   (both deferred to wave 2 — deleting there would trip `adr-divergence-unowned`).
5. Append a dated wave-1 cleanup note recording the removed symbols to the
   `architecture.md` of every domain this bead edits source in — all four:
   `.mindspec/domains/{workflow,execution,core,context-system}/architecture.md`
   (required to clear the complete-time doc-sync gate; AGENTS.md does not count).
6. Run the deadcode analyzer, build, and full test suite to confirm the sweep is
   clean and no live caller broke.

**Verification**
- [ ] `go build ./...` exits 0
- [ ] `go test ./...` exits 0
- [ ] Dead code clean except the allowed seams: `! (go run golang.org/x/tools/cmd/deadcode@latest -test ./cmd/... ./internal/... ./plugins/... | grep -vE 'internal/harness/|internal/trace/event\.go|hasManagedBlock|FindActiveBeadForEpic' | grep -q .)` exits 0
- [ ] No dangling reference to a symbol deleted in this bead: `! grep -rnE 'SpecIsApproved|SpecStatusFromBytes|IsDomainCoveredCtx' internal cmd plugins --include='*.go'` exits 0
- [ ] Doc-sync — this bead's single commit touches a doc under each of its four domains: `for d in workflow execution core context-system; do git show --name-only --format= HEAD | grep -qE "^\.mindspec/domains/$d/" || exit 1; done` exits 0

**Acceptance Criteria**
- [ ] The `deadcode -test` sweep is clean for every symbol this bead removes (spec AC1; the remaining `setup.hasManagedBlock` and `phase.FindActiveBeadForEpic` clear once Beads 2 and 3 merge, and the full-clean AC1 is confirmed at final review)
- [ ] `go test ./...` passes (spec AC9)

**Depends on**: None

## Bead 2: Setup managed-block unification (safeio)

**Steps**
1. Extract one `ensureManagedDoc`-style helper (carrying root, relative path, full
   file content, append block, and the managed-block check) whose every write/append
   routes through the EXISTING `safeio.WriteFileNoSymlink` /
   `safeio.OpenAppendNoSymlink` (do NOT modify `internal/safeio` — it is only
   called, keeping this bead's source strictly under `internal/setup/**`), folding
   the managed-block-presence logic in so `setup.hasManagedBlock` (`claude.go:555`)
   is no longer needed.
2. Re-point `ensureClaudeMD` (`claude.go`), `ensureAgentsMD` (`codex.go`), and
   `ensureCopilotInstructions` (`copilot.go`) to call the shared helper; remove the
   plain `os.WriteFile`/`os.OpenFile` calls in `codex.go:68,79,96` so the managed
   `AGENTS.md` document is written only through `safeio` (closing the symlink gap).
3. Delete `setup.hasManagedBlock`, now dead after step 1.
4. Fold `chainBeadsSetup` (`claude.go:536`) and `chainBeadsSetupCodex`
   (`codex.go:109`) — which differ only by the agent identifier string — into one
   parameterized helper, preserving each agent's current chained-setup output.
5. Add `TestRunCodex_RefusesSymlinkedAGENTSmd` to
   `internal/setup/symlink_refusal_test.go` (mirroring the claude/copilot refusal
   tests) plus a per-agent test asserting the produced managed-doc content for a
   non-symlinked target equals the expected block-constant-derived string in full
   (equality, not `strings.Contains`) for claude, codex, and copilot.
6. Append a dated wave-1 note (setup managed-block unified through `safeio`;
   `hasManagedBlock` removed) to `.mindspec/domains/workflow/architecture.md` — the
   only domain this bead edits source in — to clear the complete-time doc-sync gate.
7. Run the setup suite and the full suite.

**Verification**
- [ ] `go test ./internal/setup/...` exits 0 (existing suite plus the new refusal and per-agent equality tests)
- [ ] Refusal tests pass for all three agents: `go test ./internal/setup/... -run RefusesSymlinked -count=1` exits 0
- [ ] No plain file write remains in codex production code (managed `AGENTS.md` goes through `safeio` only): `! grep -nE 'os\.(WriteFile|OpenFile)' internal/setup/codex.go` exits 0
- [ ] `go test ./...` exits 0
- [ ] Doc-sync — this bead's single commit touches the workflow domain doc: `git show --name-only --format= HEAD | grep -qE '^\.mindspec/domains/workflow/'` exits 0

**Acceptance Criteria**
- [ ] `internal/setup/codex.go` contains no `os.WriteFile`/`os.OpenFile` call for the managed `AGENTS.md` document; all three agents write the managed block via `safeio` through the shared helper (spec AC2)
- [ ] A per-agent test asserts the produced managed-doc content equals the expected block-constant-derived string in full for claude, codex, and copilot (spec AC3)
- [ ] `mindspec setup codex` into a directory with a symlinked `AGENTS.md` refuses to write; `TestRunCodex_RefusesSymlinkedAGENTSmd` passes (spec AC4)
- [ ] `go test ./...` passes (spec AC9)

**Depends on**: None

## Bead 3: complete/phase perf pair

**Steps**
1. Add an exported wrapper `phase.FetchChildren(epicID string) ([]ChildInfo, error)`
   in `internal/phase` that delegates to the existing package-private
   `phase.fetchChildren` (`cache.go:213-230`, the single comma-joined `--status`
   uncached query); leave the memoized `Cache.GetChildren` path untouched.
2. Replace `complete.queryAllChildren`'s per-status `bd list --parent --status=<s>`
   loop (`complete.go:884-905`) with a direct call to `phase.FetchChildren` at the
   post-close children site, so the read stays fresh (uncached) and issues exactly
   one `bd list --parent` subprocess.
3. Resolve the immutable spec→epic mapping once near the top of `complete.Run` and
   reuse it at `complete.go:223,228,716,781` (each currently builds a throwaway
   `phase.NewCache()` + `bd list --type=epic`); keep the post-close children query
   re-issued after `complete` mutates bd mid-run. Delete `phase.FindActiveBeadForEpic`
   (`derive.go:713`), superseded by `FindActiveBeadForEpicWithCache`.
4. Re-point `complete`'s test stubs: `stubChildrenByStatus` (`complete_test.go:535`,
   today installs a `complete`-package `listJSONFn`) installs `phase.SetListJSONForTest`
   instead, since the children query now runs through `internal/phase`.
5. Add `TestRun_CompletePerfPairSubprocessBudget` (`internal/complete`) asserting, on
   `phase`'s `listJSONFn` seam, exactly one `bd list --parent`, at most one
   `bd list --type=epic`, and that the post-close children read reflects bd state
   mutated mid-run; add `TestFetchChildren` (`internal/phase`) covering the wrapper.
6. Append a dated wave-1 note (`complete` children/epic fan-out collapsed;
   `phase.FetchChildren` added, `FindActiveBeadForEpic` removed) to BOTH
   `.mindspec/domains/workflow/architecture.md` and
   `.mindspec/domains/core/architecture.md` — the two domains this bead edits source
   in — to clear the complete-time doc-sync gate; then run the suites.

**Verification**
- [ ] Subprocess budget + freshness proved: `go test ./internal/complete/... -run TestRun_CompletePerfPairSubprocessBudget -count=1` exits 0 (asserts one `bd list --parent`, at most one `bd list --type=epic`, and mid-run-fresh post-close read on the `phase` seam)
- [ ] Phase wrapper covered: `go test ./internal/phase/... -run TestFetchChildren -count=1` exits 0
- [ ] Full complete + phase suites green with the re-pointed stubs: `go test ./internal/complete/... ./internal/phase/...` exits 0
- [ ] Dead symbol removed: `! grep -rn 'FindActiveBeadForEpic(' internal cmd --include='*.go'` exits 0
- [ ] `go test ./...` exits 0
- [ ] Doc-sync — this bead's single commit touches both the workflow and core domain docs: `for d in workflow core; do git show --name-only --format= HEAD | grep -qE "^\.mindspec/domains/$d/" || exit 1; done` exits 0

**Acceptance Criteria**
- [ ] `mindspec complete` issues exactly one `bd list --parent` subprocess for the children query (was ~5), asserted on `phase`'s seam; the `stubChildrenByStatus` stub is re-pointed at `phase.SetListJSONForTest` (spec AC5)
- [ ] `complete.Run` constructs the immutable spec→epic mapping once (at most one `bd list --type=epic`), while the post-close children query still reflects bd state mutated mid-run (spec AC6)
- [ ] `go test ./...` passes (spec AC9)

**Depends on**: None

## Bead 4: Guardrails restoration + spec-init alias dedup

**Steps**
1. Add a `## Bead-loop guardrails (mindspec)` section to `AGENTS.md` carrying the
   five canonical fences, each with the grep-able substring the Verification checks
   for: (1) only the cycle runs `mindspec complete`, and only after the `panel gate`
   passes; (2) never a raw `git merge bead/<id>`; (3) exactly one `git push` at
   end-of-spec; (4) subagents make `exactly one commit`; (5) `tests must PASS` before
   completion.
2. Confirm the `CLAUDE.md:43` cross-reference ("See AGENTS.md § Bead-loop guardrails
   (mindspec)") now resolves to the real, non-empty section.
3. Change `specInitCmd` (`cmd/mindspec/spec_init.go`) to reuse `specCreateCmd`'s
   `RunE` (`cmd/mindspec/spec.go:29-73`) instead of carrying the byte-identical
   42-line copy, removing the inline `spec.Run(...)` body (and any now-unused
   imports) so future spec-create changes propagate to the hidden `spec init` alias
   automatically; `mindspec spec init` behavior stays unchanged.
4. Append a dated wave-1 note (`spec init` alias de-duplicated to reuse
   `specCreateCmd.RunE`) to `.mindspec/domains/workflow/architecture.md` — the only
   domain this bead edits source in (`cmd/**`) — to clear the complete-time doc-sync
   gate (`AGENTS.md` is a root operator doc and does not count).
5. Run `go build ./...` and the `cmd` test suite.

**Verification**
- [ ] Section present with all five fences: `grep -qF '## Bead-loop guardrails (mindspec)' AGENTS.md && for s in 'mindspec complete' 'panel gate' 'git merge bead/' 'git push' 'exactly one commit' 'tests must PASS'; do grep -Fqi "$s" AGENTS.md || exit 1; done` exits 0
- [ ] CLAUDE.md cross-reference resolves to the section: `grep -qF 'AGENTS.md § Bead-loop guardrails (mindspec)' CLAUDE.md && grep -qF '## Bead-loop guardrails (mindspec)' AGENTS.md` exits 0
- [ ] Alias reuses create's RunE and the duplicated body is gone: `grep -q 'specCreateCmd.RunE' cmd/mindspec/spec_init.go && ! grep -q 'spec.Run(' cmd/mindspec/spec_init.go` exits 0
- [ ] Build + cmd suite green (spec-init behavior unchanged): `go build ./... && go test ./cmd/...` exits 0
- [ ] Doc-sync — this bead's single commit touches the workflow domain doc: `git show --name-only --format= HEAD | grep -qE '^\.mindspec/domains/workflow/'` exits 0

**Acceptance Criteria**
- [ ] `AGENTS.md` contains a non-empty `## Bead-loop guardrails (mindspec)` section carrying all five canonical fences, and the `CLAUDE.md` cross-reference resolves to it (spec AC7)
- [ ] `cmd/mindspec/spec_init.go` no longer duplicates the 42-line `RunE` body; `specInitCmd` reuses `specCreateCmd.RunE`; `mindspec spec init` behavior is unchanged (spec AC8)
- [ ] `go test ./...` passes (spec AC9)

**Depends on**: None

## Provenance

**Spec source.** Spec `107-cleanup-deadcode-dry-wave1`, Approved 2026-07-02 by
`panel:spec-107-approve round-2` (6/6 APPROVE; R1-R3 claude, R4-R6 codex). The spec
scopes wave 1 of the 2026-07-02 repo review, whose panel-local tracked copy is
`review/spec-107-approve/source-report.md` — §1 (dead code), §2 DRY #1/#5, §3 Perf
#2/#3, and §4 cleanup order 1–3, plus the two carried fixes R7 (guardrails section,
a documentation-integrity fix beyond the report) and R8 (the spec-init slice of DRY
#10 pulled forward).

**Plan-panel round 1.** 4 APPROVE / 2 REQUEST_CHANGES. This revision applies the
consolidated fix list: (R3) each bead now updates the domain `architecture.md` for
every domain whose source it touches, so the complete-time doc-sync gate
(`checkInternalPackages`) is satisfiable per bead — the per-bead domain sets were
re-derived by walking each file list against the four
`.mindspec/domains/*/OWNERSHIP.yaml` globs (Bead 1 → all four; Bead 2 → workflow;
Bead 3 → workflow+core; Bead 4 → workflow); and (R5) every Verification item is now
a shell command whose exit status is the pass/fail, including the Bead 3
subprocess-count assertion (`TestRun_CompletePerfPairSubprocessBudget`), the Bead 4
guardrail-fence / cross-reference / dedup checks, and inverted grep-negative
predicates (`! grep …`) for the no-match cases.

**Bead cut.** Four independent beads, ownership-aligned per panel R6 so every
deletion rides the bead that owns the file (no cross-bead source overlap → no
dependency edges; all four `work_chunks` declare `depends_on: []`). Panel R3's nit
(fix dangling comment references when deleting `validate.SpecIsApproved` /
`IsDomainCoveredCtx`) is folded into Bead 1 step 2. Panel R5's stub re-point and R6's
spec→epic hoist are Bead 3.

**Acceptance-criteria → bead map (output provenance).**

| Spec Acceptance Criterion | Verified By |
|---|---|
| AC1 — `deadcode -test` zero findings (modulo harness seams + deferred `internal/trace`) | Bead 1 (bulk sweep) + Bead 2 (`setup.hasManagedBlock`) + Bead 3 (`phase.FindActiveBeadForEpic`); full-clean confirmed at final review |
| AC2 — `codex.go` writes managed `AGENTS.md` only via `safeio` (all three agents through the shared helper) | Bead 2 verification (negative grep + setup suite) |
| AC3 — per-agent full-equality managed-doc content test (claude/codex/copilot) | Bead 2 verification (`go test ./internal/setup/...`) |
| AC4 — `mindspec setup codex` refuses a symlinked `AGENTS.md`; `TestRunCodex_RefusesSymlinkedAGENTSmd` passes | Bead 2 verification (`-run RefusesSymlinked`) |
| AC5 — `complete` issues exactly one `bd list --parent`; `stubChildrenByStatus` re-pointed at `phase.SetListJSONForTest` | Bead 3 verification (`TestRun_CompletePerfPairSubprocessBudget`) |
| AC6 — `complete.Run` resolves spec→epic once (≤1 `bd list --type=epic`); post-close children query stays fresh | Bead 3 verification (`TestRun_CompletePerfPairSubprocessBudget`) |
| AC7 — `AGENTS.md` has a non-empty `## Bead-loop guardrails (mindspec)` section (five fences); `CLAUDE.md` xref resolves | Bead 4 verification (grep fences + xref) |
| AC8 — `spec_init.go` no longer duplicates the 42-line `RunE`; `specInitCmd` reuses `specCreateCmd.RunE`; behavior unchanged | Bead 4 verification (grep reuse / body-gone + `go test ./cmd/...`) |
| AC9 — `go test ./...` passes | Every bead (each ends on a green full suite) |

Requirement R4 (fold `chainBeadsSetup`/`chainBeadsSetupCodex`) has no standalone spec
AC and is verified inside Bead 2 (the setup suite stays green with each agent's
chained-setup output preserved). Beyond the spec ACs, every bead also carries a
**doc-sync** Verification checkbox asserting its single commit touches the required
domain `architecture.md`(s) — the second completion bar the `mindspec complete` gate
enforces (see Testing Strategy).
