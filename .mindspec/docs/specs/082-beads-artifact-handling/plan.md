---
adr_citations:
    - id: ADR-0006
    - id: ADR-0023
    - id: ADR-0025
approved_at: "2026-04-21T22:31:03Z"
approved_by: user
bead_ids:
    - mindspec-hzdh.6
    - mindspec-hzdh.7
    - mindspec-hzdh.8
    - mindspec-hzdh.9
    - mindspec-hzdh.10
    - mindspec-hzdh.11
spec_id: 082-beads-artifact-handling
status: Approved
version: "2"
---
# Plan: 082-beads-artifact-handling

## ADR Fitness

- **ADR-0023** (Beads as single state authority) — reinforces the plan. Dolt remains the authoritative store; `.beads/issues.jsonl` is a deterministic projection whose purpose is durability-via-git and grep-ability. Every remediation in this plan is consistent with that stance: we never treat JSONL diffs as user authorship, and we never bypass Dolt.
- **ADR-0006** (Protected main + PR-based merging) — the dirty-tree guard change (Bead 1a) must not open a back door to direct main writes. The guard still refuses on user-authored changes; it only auto-handles the JSONL artifact, and always by running `bd export` + continuing through a mindspec-owned worktree path.
- **ADR-0025 (drafted by Bead 1a)**: "`.beads/issues.jsonl` is a build artifact" — captures the cross-cutting stance so later specs can cite it by number. Slot under the ADR-0023 lineage. Not in this plan's `adr_citations` frontmatter because the ADR doesn't exist at plan-approval time (the validator checks existence). Bead 1a adds the citation to this plan as part of its ADR-creation work (amending plan.md inside the bead's first commit is within scope since the plan lives under the spec).

No accepted ADRs are superseded or contradicted by this plan.

## Testing Strategy

Three layers, applied at the bead boundaries:

1. **Unit tests** (Go test, per package): each new helper — the refactored dirty-tree guard, `EnsureBeadsConfig`, the executor's export step, the new doctor checks — has tests covering happy path, edge cases (missing files, user-authored values, stale exports, parse errors), and idempotence.
2. **Integration tests** using temp-dir git repos + fake `bd` on `PATH` (same pattern as `internal/setup/claude_test.go:installFakeBD` after PR #83): verify end-to-end CLI flows — `mindspec init` + `bd init` → config correct; `mindspec next` with only `.beads/issues.jsonl` dirty → succeeds; `mindspec doctor` on drifted configs → reports and `--fix`es.
3. **LLM harness scenario** (`internal/harness/`): one new scenario `TestLLM_BeadsArtifactPassthrough` (Bead 1b) that replays the Lola friction pattern — agent files an issue via `bd create`, then runs `mindspec next` and expects no stash/branch intervention, followed by a bead's first commit carrying the filed issue's JSONL line. This is the canary that proves the fix works under realistic agent behavior. It asserts the combined effect of Bead 1a (guard) and Bead 2 (executor export), so it deliberately depends on both.

Existing harness scenarios (`TestLLM_SpecToIdle`, `TestLLM_MultiBeadDeps`, etc.) must continue to pass without modification.

Skipped on the user's own machine (harness tests are long-running and require `env -u CLAUDECODE`), run in CI.

### Shared conventions

- **`FindBeadsDir` resolution** (used by Beads 1a, 2): mindspec has no equivalent of bd's `beads.FindBeadsDir`. Rather than adding a Go-side reimplementation, beads that need the main-repo `.beads/` path will shell out to `bd` directly and let bd's own resolver handle the redirect. Concretely: `cmd := exec.Command("bd", "export", "-o", "-")` or similar, run with `cmd.Dir = <repoRoot>`. The subprocess boundary also guarantees the same redirect semantics every other bd invocation sees.

## Bead 1a: Artifact-aware dirty-tree guard + ADR-0025 + domain doc

Applies ADR-0006 and ADR-0023; drafts ADR-0025 (pre-allocated) capturing the artifact stance referenced by later beads.

**Steps**
1. Draft `.mindspec/docs/adr/ADR-0025-jsonl-as-build-artifact.md` with Status=Accepted. Content: Dolt is authoritative per ADR-0023; `.beads/issues.jsonl` is a deterministic projection co-managed by `bd export` and mindspec's commit points; tooling must not treat JSONL diffs as user authorship.
2. Refactor the dirty-tree guard in `internal/next/` (currently rejects any non-empty `git status --porcelain`). Introduce `classifyDirty(paths []string) (artifactDirt, userDirt []string)` that splits the dirty set. `.beads/issues.jsonl` is the sole artifact today; keep the classifier list-driven so future artifacts (e.g., `.beads/events.jsonl`) can be added by a one-liner.
3. Before deciding, shell out `bd export -o .beads/issues.jsonl` from `repoRoot` as CWD (see "Shared conventions" above). This normalizes the diff against stale throttled exports. Re-read `git status --porcelain` after.
4. Decision table: if `userDirt` is empty, proceed with the claim; otherwise abort with a message that (a) lists the user-dirty paths, (b) explicitly notes `.beads/issues.jsonl` is auto-handled and doesn't need stashing.
5. Update `.mindspec/docs/domains/workflow/overview.md` (or the closest matching workflow-domain doc if that exact path doesn't exist) with a short "JSONL as artifact" section referencing ADR-0025 by number.

**Verification**
- [ ] `go test ./internal/next/...` passes new unit tests for `classifyDirty` and the guard's decision table, including a case where `.beads/issues.jsonl` and `foo.txt` are both dirty (guard must abort) and a case where only `.beads/issues.jsonl` is dirty (guard must proceed).
- [ ] `bd show ADR-0025` on the drafted ADR succeeds (validates filename + frontmatter).
- [ ] `mindspec doctor` on this repo still reports a clean tree after the change.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec next` succeeds when the only dirty path is `.beads/issues.jsonl`" — covered by unit test in the guard package.
- [ ] Spec AC "`mindspec next` still refuses on other dirty paths" — covered by unit test with mixed dirt.
- [ ] Spec AC "Domain doc explains JSONL-as-artifact policy" — the new section exists and cites ADR-0025 by its pre-allocated number.
- [ ] ADR-0025 exists at `.mindspec/docs/adr/ADR-0025-jsonl-as-build-artifact.md` with Status=Accepted, cross-references ADR-0023 in its body, and is already present in this plan's `adr_citations` frontmatter (no plan edit required).

**Depends on**
None

## Bead 1b: LLM harness scenario — `TestLLM_BeadsArtifactPassthrough`

End-to-end canary that exercises the combined behavior of Beads 1a and 2 under a realistic agent. Separate from 1a because writing a new harness scenario is non-trivial (prompt tuning, Haiku quirks, shim setup per `internal/harness/HISTORY.md`) and pairs differently with 1a's code changes.

**Steps**
1. Add a new scenario to `internal/harness/scenario.go` matching the pattern of `TestLLM_SpecToIdle` / `TestLLM_MultiBeadDeps`. Sandbox setup: spec already approved with one simple bead ready; agent has bd available. Starting state: `.beads/issues.jsonl` dirty with one new issue (simulating `bd create` before session).
2. Wire `internal/harness/scenario_test.go` to register the new scenario and the `TestLLM_BeadsArtifactPassthrough` Go test function.
3. Prompt design: imperative, Haiku-compatible (per `internal/harness/HISTORY.md`). Task: "Run `mindspec next` to claim the ready bead, implement the change, then `mindspec complete`". Deliberately do NOT mention the dirty JSONL — the point is that the agent should not need to do anything special.
4. Analyzer assertions: (a) agent did not run `git stash` or create a `chore/` branch; (b) `mindspec next` succeeded without retry; (c) the final `mindspec complete` commit includes `.beads/issues.jsonl` and the diff contains the pre-seeded issue's id.
5. Update `internal/harness/HISTORY.md` with a new history-table row for the first run.

**Verification**
- [ ] `env -u CLAUDECODE go test ./internal/harness/ -v -run TestLLM_BeadsArtifactPassthrough -timeout 10m` passes end-to-end.
- [ ] Analyzer correctly detects a "happy path" run (no stash, no chore branch) and fails loudly if either appears.
- [ ] `internal/harness/HISTORY.md` has a new row with the metrics (events, turns, time, retries).

**Acceptance Criteria**
- [ ] Spec AC "LLM harness scenario exercises the friction pattern end-to-end" — the new scenario exists and passes.
- [ ] Scenario prompt is imperative/Haiku-compatible per the project's harness conventions.
- [ ] No regression on existing harness scenarios.

**Depends on**
Bead 1a, Bead 2

## Bead 2: Executor `bd export` at commit points

Applies ADR-0006 (the JSONL must land in each PR-merged commit to preserve main-is-authoritative semantics) and ADR-0023/ADR-0025 (the committed JSONL must match Dolt at commit time — no drift).

**Steps**
1. Identify every executor code path that ends in a `git add -A && git commit`. Today: `internal/executor/mindspec_executor.go:CommitAll` wrapping `gitutil.CommitAll`. Used by `approve spec`, `approve plan`, `approve impl`, and `complete`.
2. Inline a "pre-stage step" in `CommitAll` (not a git hook — to avoid the naming overload): before the `git add -A` call, shell out `bd export -o <repoRoot>/.beads/issues.jsonl` with `cmd.Dir = repoRoot`. Per "Shared conventions", we let bd resolve the redirect rather than reimplementing `FindBeadsDir` in Go.
3. On `bd export` failure, return the error up through `CommitAll` — do not swallow. Caller's error handling already surfaces commit failures to the user.
4. Note in a code comment: bd's own pre-commit hook (`.beads/hooks/pre-commit`) also runs `bd export` via `exportJSONLForCommit`. The second export produces byte-identical output, so there is no duplication bug, but a future maintainer should not "optimize away" one of the two — the executor-level export guarantees the commit contains the current state even if the git hook is skipped (e.g., `--no-verify`).
5. Unit tests using `MockExecutor`: assert that each `approve`/`complete` call path invokes the new `bd export` step before `git add -A`.
6. Integration test: run a real `approve spec` against a temp-dir repo with a mutated Dolt state. Assert `git show --stat HEAD` includes `.beads/issues.jsonl` and `git cat-file blob HEAD:.beads/issues.jsonl` byte-matches a fresh `bd export` run.

**Verification**
- [ ] `go test ./internal/executor/...` passes new tests.
- [ ] `go test ./internal/approve/...` and `./internal/complete/...` pass (no regression).
- [ ] Manual smoke: `mindspec approve spec <throwaway>` in a scratch repo with a mutated Dolt state — the resulting commit contains the updated JSONL.

**Acceptance Criteria**
- [ ] Spec AC "after approve/complete, `git show --stat HEAD` includes `.beads/issues.jsonl` whenever Dolt changed" — covered by the integration test.
- [ ] Spec AC "committed JSONL is byte-identical to a fresh `bd export` at commit time" — covered by the integration test's `cat-file` comparison.
- [ ] No existing harness scenario regresses.

**Depends on**
None

## Bead 3: `EnsureBeadsConfig` helper with structural YAML merge

Applies ADR-0023 (this is the bootstrap layer that ensures the projection/config stays in alignment with what mindspec's runtime reads at approve/complete time).

**Steps**
1. Create `internal/bead/config.go`. Export `EnsureBeadsConfig(root string, force bool) (*ConfigResult, error)`. `ConfigResult` fields: `Added []string`, `AlreadyCorrect []string`, `UserAuthored []ConfigDrift` (keys where user-authored value disagrees with mindspec's required value — left alone unless `force=true`), `CreatedFile bool`. Exported required-keys constant so Bead 5 can import it: `types.custom: "gate"`, `status.custom: "resolved"`, `export.git-add: false`, `issue-prefix: <default = filepath.Base(root)>`.
2. Implementation: use `yaml.v3` Node-based editing to preserve existing keys and comments byte-for-byte when the file exists; when it doesn't exist, create it via `yaml.Marshal` with a brief header comment (the fresh-file path has no structural-preservation concerns — document the exception inline). Write through temp file + `os.Rename` for atomicity.
3. Handling `issue-prefix`: never overwrite an existing value (user's project-naming choice). Only write when absent.
4. Handling `export.git-add: true` when set by the user: record in `UserAuthored`, leave untouched unless `force=true` (resolves Open Question 1 from the spec).
5. Unit tests for each core path: fresh file creation, merging into a partially-configured file, preserving non-mindspec keys, detecting user-authored drift, idempotence (second call byte-identical), `force=true` overrides.
6. **YAML edge-case golden tests** (per plan review): (a) top-of-file comment preserved, (b) comments interleaved between keys preserved, (c) block-style value preserved, (d) YAML anchor reference preserved.
7. Atomic-write test: simulate a write failure partway through `os.Rename` (or verify via inspection that `os.Rename` is atomic on the target filesystem) — ensures no partial-file corruption on interrupt.

**Verification**
- [ ] `go test ./internal/bead/... -run EnsureBeadsConfig` passes all cases including the four edge-case goldens.
- [ ] Round-trip test: a known input file through `EnsureBeadsConfig` with no changes required produces byte-identical output.

**Acceptance Criteria**
- [ ] `EnsureBeadsConfig` exists and compiles; exported symbols stable.
- [ ] Calling it twice produces byte-identical output (idempotence).
- [ ] User-authored keys outside the mindspec-required set are never touched.
- [ ] User-authored `export.git-add: true` is recorded in `UserAuthored`, not silently flipped (without `force`).
- [ ] All four YAML edge-case goldens pass.

**Depends on**
None

## Bead 4: Wire `EnsureBeadsConfig` into init/setup + fix `mindspec spec create` redirect gap

Applies ADR-0023 (bootstrap path) and implements the plan-time finding that `mindspec spec create` currently produces worktrees without `.beads/redirect`. The diagnosis and patch are scoped with a spike timebox; if diagnosis exceeds the timebox the wiring half ships alone and the redirect work splits into a follow-up spec.

**Context added from plan-time investigation**: during this spec's own Plan Mode transition, running `mindspec spec approve` failed with `bd list --type=epic failed: exit status 1` because the newly-created spec worktree had no `.beads/redirect` file — `bd` spawned its own empty Dolt server. Inspection of `internal/executor/mindspec_executor.go:InitSpecWorkspace` shows it correctly calls `bead.WorktreeCreate` (shells out to `bd worktree create`), but the redirect was not written. Either `bd worktree create` is not completing the redirect step when the branch already exists, or a later step in mindspec's flow overwrites the worktree's `.beads/`.

**Spike timebox for redirect diagnosis (Step 2 below)**: one agent session. If the root cause isn't conclusively identified in that window, file a follow-up bead for the patch and ship the wiring work alone — do not let diagnostic uncertainty block `EnsureBeadsConfig` from reaching users.

**Steps**
1. Reproduce the redirect-missing bug in an isolated integration test: run `mindspec spec create <id>` end-to-end in a temp repo with `bd` installed, then assert `<worktree>/.beads/redirect` exists and contains the correct relative path to the main repo's `.beads/`.
2. Diagnose (spike-timeboxed): enable `BD_DEBUG_ROUTING=1`, re-run, read the trace. Determine whether `bd worktree create` fails silently, is called with wrong args, or is called correctly but the redirect file is clobbered later. If timebox expires without conclusion, file follow-up bead and skip to Step 4.
3. Patch the root cause (only if Step 2 concluded). Likely shapes: (a) surface `bead.WorktreeCreate` errors that are currently swallowed, (b) post-worktree-create assertion in `InitSpecWorkspace` that the redirect file exists, with remediation if not, (c) fix an ordering bug where spec files are written to `.beads/` before `bd worktree create` runs.
4. Call `EnsureBeadsConfig(root, false)` from `cmd/mindspec/init.go:bootstrap.Run`. Print `ConfigResult` summary.
5. Call `EnsureBeadsConfig(root, false)` from `cmd/mindspec/setup.go`'s claude and copilot subcommands, after `chainBeadsSetup`/`chainBeadsSetupCodex`. Print summary.
6. Integration tests (temp-dir + fake `bd` on `PATH`): (a) `mindspec init` + real `bd init` leaves config mindspec-ready; (b) `mindspec setup claude` after a bare `bd init` patches the config; (c) second run is a no-op.

**Verification**
- [ ] `go test ./internal/bead/... -run Worktree` passes (including the new redirect-missing regression test, even if Step 3 is deferred — the test captures the bug either way).
- [ ] `go test ./internal/executor/... -run InitSpecWorkspace` passes.
- [ ] `go test ./cmd/mindspec/... -run "Init|Setup"` passes.
- [ ] Manual smoke: `mindspec spec create NNN-smoke` in a scratch repo — the new worktree has `.beads/redirect` and `bd list` from inside returns main's issues.

**Acceptance Criteria**
- [ ] Spec AC "fresh `mindspec init` + `bd init` produces `.beads/config.yaml` with the mindspec-required keys" — covered by integration test (a).
- [ ] Spec AC "running `mindspec init` a second time is a no-op on the config file" — covered by integration test (c).
- [ ] `mindspec spec create <id>` produces a worktree with a correct `.beads/redirect` file — regression test exists and passes (patch implemented here or deferred to follow-up bead per spike timebox).
- [ ] Open Question 2 from spec is resolved: redirect is the durability mechanism; this bead makes its creation reliable.

**Depends on**
Bead 3

## Bead 5: Doctor checks for beads/mindspec integration health

Applies ADR-0023/ADR-0025 (doctor surfaces deviations from the single-state-authority + projection model).

**Steps**
1. Extend `internal/doctor/beads.go` with four checks, each returning a `doctor.Check` result:
   - `checkBeadsConfigDrift(root)`: calls a new read-only variant of the required-keys scan from Bead 3 (share the required-keys constant, don't rewrite). Reports each missing or drifted key.
   - `checkStrayRootJSONL(root)`: runs `git ls-files --full-name -- issues.jsonl` at the repo root. If non-empty, warns and suggests `git rm --cached issues.jsonl` (cleanup across branches is out of scope; see spec).
   - `checkDurabilityRisk(root)`: reads `.beads/config.yaml` for `export.auto`. Detects "Dolt remote configured" by (in order, falling back): reading `sync.remote` from `.beads/config.yaml`, reading `.beads/dolt/.dolt/config.json` for any configured remote, or shelling out `bd` and scraping whatever remote-listing surface the installed version exposes. If `export.auto: false` AND no remote is detected, warns with remediation. If remote detection itself fails, skip the check with an info-level note rather than false-positive warn.
   - `checkBdVersionFloor(root)`: runs `bd --version` and parses with regex `\bv?([0-9]+\.[0-9]+\.[0-9]+)` (handles `bd version 1.0.2 (Homebrew)`-style output and `v1.0.2` alike). On parse failure, skip the check with "unknown bd version" info — do not false-warn. On successful parse < 1.0.2, warn.
2. Add a `--fix` mode (or extend existing) on `mindspec doctor` that, for `checkBeadsConfigDrift`, calls `EnsureBeadsConfig(root, force=false)` and surfaces the `ConfigResult`. `--fix --force` passes `force=true`. If mindspec doctor already has a different `--fix` surface, integrate; otherwise add the flag and wire it.
3. Unit tests for each check using temp dirs + fake `bd` on `PATH`. Include a test for Check 4's graceful-degradation-on-unknown-version-format path.
4. **Bead review finding**: add a test that asserts `--fix --force` replaces user-authored `export.git-add: true` while `--fix` alone preserves it (this complements Bead 3's unit coverage by asserting the doctor wiring honors the `force` flag).
5. Integration test: run `mindspec doctor` on a scratch repo with default `bd init` → reports config drift; run `mindspec doctor --fix` → patches; re-run → clean.

**Verification**
- [ ] `go test ./internal/doctor/...` passes new tests.
- [ ] Manual smoke: `mindspec doctor` on this repo (which already has the mindspec keys) reports clean beads section.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec doctor` on a bd-default repo reports each missing mindspec key" — covered by integration test.
- [ ] Spec AC "`mindspec doctor --fix` patches; re-running reports clean" — covered by integration test.
- [ ] Spec AC "`mindspec doctor` on a repo with tracked root `issues.jsonl` warns" — covered by unit test.
- [ ] `checkDurabilityRisk` fires exactly when `export.auto: false` AND no Dolt remote is detected, and gracefully skips when remote detection itself fails.
- [ ] `checkBdVersionFloor` fires on parseable versions < 1.0.2, skips on parse failure, passes on >= 1.0.2.
- [ ] `--fix --force` replaces user-authored `export.git-add: true`; `--fix` alone preserves it.

**Depends on**
Bead 3

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `mindspec next` succeeds with only `.beads/issues.jsonl` dirty | Bead 1a (unit) + Bead 1b (harness) |
| `mindspec next` refuses on other dirty paths | Bead 1a (unit test) |
| Approve/complete commits include fresh `.beads/issues.jsonl` | Bead 2 (integration test) |
| Committed JSONL is byte-identical to `bd export` at commit time | Bead 2 (integration test) |
| Fresh `mindspec init` + `bd init` produces mindspec-ready config | Bead 4 (integration test a) |
| `mindspec init` is a no-op on second run | Bead 4 (integration test c) |
| `mindspec doctor` reports missing mindspec keys; `--fix` patches | Bead 5 (integration test) |
| `mindspec doctor` warns on tracked root-level `issues.jsonl` | Bead 5 (unit test) |
| LLM harness scenario exercises the friction pattern end-to-end | Bead 1b (depends on Bead 1a + Bead 2) |
| No regression on existing harness scenarios | Beads 1a, 2 (CI guard) |
| Domain doc explains JSONL-as-artifact policy | Bead 1a (doc update + ADR-0025 reference) |
