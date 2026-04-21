---
adr_citations:
    - id: ADR-0006
    - id: ADR-0023
approved_at: "2026-04-21T22:17:38Z"
approved_by: user
bead_ids:
    - mindspec-hzdh.1
    - mindspec-hzdh.2
    - mindspec-hzdh.3
    - mindspec-hzdh.4
    - mindspec-hzdh.5
spec_id: 082-beads-artifact-handling
status: Approved
version: "1"
---
# Plan: 082-beads-artifact-handling

## ADR Fitness

- **ADR-0023** (Beads as single state authority) — reinforces the plan. Dolt remains the authoritative store; `.beads/issues.jsonl` is a deterministic projection whose purpose is durability-via-git and grep-ability. Every remediation in this plan is consistent with that stance: we never treat JSONL diffs as user authorship, and we never bypass Dolt.
- **ADR-0006** (Protected main + PR-based merging) — the dirty-tree guard change (Bead 1) must not open a back door to direct main writes. The guard still refuses on user-authored changes; it only auto-handles the JSONL artifact, and always by running `bd export` + continuing through a mindspec-owned worktree path.
- **New ADR drafted by Bead 1**: "`.beads/issues.jsonl` is a build artifact" — captures the cross-cutting stance so later specs can cite it by number. Slot under the ADR-0023 lineage.

No accepted ADRs are superseded or contradicted by this plan.

## Testing Strategy

Three layers, applied at the bead boundaries:

1. **Unit tests** (Go test, per package): each new helper — the refactored dirty-tree guard, `ensureBeadsConfig`, the executor's export hook, the new doctor checks — has tests covering happy path, edge cases (missing files, user-authored values, stale exports, parse errors), and idempotence.
2. **Integration tests** using temp-dir git repos + fake `bd` on `PATH` (same pattern as `internal/setup/claude_test.go:installFakeBD` after PR #83): verify end-to-end CLI flows — `mindspec init` + `bd init` → config correct; `mindspec next` with only `.beads/issues.jsonl` dirty → succeeds; `mindspec doctor` on drifted configs → reports and `--fix`es.
3. **LLM harness scenario** (`internal/harness/`): one new scenario `TestLLM_BeadsArtifactPasssthrough` that replays the exact Lola friction pattern — agent files an issue via `bd create`, then runs `mindspec next` and expects no stash/branch intervention. This is the canary that proves the fix works under realistic agent behaviour, not just unit-level assertions.

Existing harness scenarios (`TestLLM_SpecToIdle`, `TestLLM_MultiBeadDeps`, etc.) must continue to pass without modification — they exercise the approve/complete paths that Bead 2 changes.

Skipped on the user's own machine (harness tests are long-running and require `env -u CLAUDECODE`), run in CI.

## Bead 1: Artifact-aware dirty-tree guard + JSONL-as-artifact ADR

Applies ADR-0006 and ADR-0023; drafts a new ADR under the ADR-0023 lineage capturing the artifact stance referenced by later beads.

**Steps**
1. Draft `.mindspec/docs/adr/ADR-NNNN-jsonl-as-build-artifact.md` (next free ADR number). Content: Dolt is authoritative per ADR-0023; `.beads/issues.jsonl` is a deterministic projection co-managed by `bd export` and mindspec's commit points; tooling must not treat it as user-authored.
2. Refactor the dirty-tree guard in `internal/next/` (currently rejects any non-empty `git status --porcelain`). Introduce `classifyDirty(paths []string) (artifactDirt, userDirt []string)` that splits the dirty set. `.beads/issues.jsonl` is the sole artifact today; keep the classifier list-driven so future artifacts (e.g., `.beads/events.jsonl`) can be added by a one-liner.
3. Before deciding, run `bd export -o .beads/issues.jsonl` against the main repo's `.beads/` (resolve via `bead.FindBeadsDir()` or shell out and let bd resolve). This normalizes the diff against stale throttled exports. Re-read `git status --porcelain` after.
4. Decision table: if `userDirt` is empty, proceed with the claim; otherwise abort with a message that (a) lists the user-dirty paths, (b) explicitly notes `.beads/issues.jsonl` is auto-handled and doesn't need stashing.
5. Update `.mindspec/docs/domains/workflow/overview.md` (or equivalent) with a short "JSONL as artifact" section referencing the new ADR.

**Verification**
- [ ] `go test ./internal/next/...` passes new unit tests for `classifyDirty` and the guard's decision table.
- [ ] `go test ./internal/harness/... -run TestLLM_BeadsArtifactPassthrough` passes (new harness scenario; smoke test of the happy path with a real agent).
- [ ] `mindspec doctor` on this repo still reports a clean tree after the change.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec next` succeeds when the only dirty path is `.beads/issues.jsonl`" — covered by a unit test in the guard package and the new harness scenario.
- [ ] Spec AC "`mindspec next` still refuses on other dirty paths" — covered by a unit test asserting the guard aborts when both `.beads/issues.jsonl` and an arbitrary `foo.txt` are dirty.
- [ ] Spec AC "Domain doc explains JSONL-as-artifact policy" — the new section exists and references the new ADR by number.
- [ ] New ADR exists in `.mindspec/docs/adr/` with Status=Accepted, cross-references ADR-0023, and is cited in this plan's frontmatter (edit `plan.md` to add it to `adr_citations` after creation).

**Depends on**
None

## Bead 2: Executor `bd export` at commit points

Applies ADR-0006 (the JSONL must land in each PR-merged commit to preserve main-is-authoritative semantics) and ADR-0023 (the committed JSONL must match Dolt at commit time — no drift).

**Steps**
1. Identify every executor code path that ends in a `git add -A && git commit`. Today: `internal/executor/mindspec_executor.go:CommitAll` wrapping `gitutil.CommitAll`. Used by `approve spec`, `approve plan`, `approve impl`, and `complete`.
2. Introduce a pre-commit hook in the executor (not a git hook — a Go-level hook): before `CommitAll`'s `git add -A`, run `bd export -o <mainBeadsDir>/issues.jsonl` where `mainBeadsDir` comes from `bead.FindBeadsDir()` or equivalent (must resolve correctly when called from inside a bead worktree — `bd` itself handles the redirect, but we must not hard-code the current worktree's `.beads/`).
3. On `bd export` failure, return the error up through `CommitAll` — do not swallow. Caller's error handling already surfaces commit failures to the user.
4. Add unit tests using the existing `MockExecutor` pattern: assert that each `approve`/`complete` call path invokes the new export step before `git add`.
5. Add an integration test that runs a real `approve spec` against a temp-dir repo with Dolt issues, then asserts `git show --stat HEAD` includes `.beads/issues.jsonl` and `git cat-file blob HEAD:.beads/issues.jsonl` byte-matches a fresh `bd export` run.

**Verification**
- [ ] `go test ./internal/executor/...` passes new tests.
- [ ] `go test ./internal/approve/...` and `./internal/complete/...` pass (no regression).
- [ ] Manual smoke: run `mindspec approve spec 082-beads-artifact-handling` in a throwaway repo with a mutated Dolt state — the resulting commit contains the updated JSONL.

**Acceptance Criteria**
- [ ] Spec AC "after approve/complete, `git show --stat HEAD` includes `.beads/issues.jsonl` whenever Dolt changed" — covered by the integration test.
- [ ] Spec AC "committed JSONL is byte-identical to a fresh `bd export` at commit time" — covered by the integration test's `cat-file` comparison.
- [ ] No existing harness scenario regresses (`TestLLM_SpecToIdle`, `TestLLM_MultiBeadDeps`, etc.).

**Depends on**
None

## Bead 3: `ensureBeadsConfig` helper with structural YAML merge

Applies ADR-0023 (this is the bootstrap layer that ensures the projection/config stays in alignment with what mindspec's runtime reads at approve/complete time).

**Steps**
1. Create `internal/bead/config.go`. Export `EnsureBeadsConfig(root string, force bool) (*ConfigResult, error)`. `ConfigResult` records: `Added []string` (keys newly written), `AlreadyCorrect []string` (mindspec-required keys already set), `UserAuthored []ConfigDrift` (keys where a user-authored value disagrees with mindspec's required value — left alone unless `force=true`), `CreatedFile bool`.
2. Required keys: `types.custom: "gate"`, `status.custom: "resolved"`, `export.git-add: false`, `issue-prefix: <default = filepath.Base(root)>`. Constant-define the required set so Bead 5's doctor check can import it.
3. Use `yaml.v3` Node-based editing (not `Marshal`) so existing keys and comments are preserved byte-for-byte. Write through a temp file + rename for atomicity.
4. Handling `issue-prefix`: never overwrite an existing value — this is the user's project-naming choice. Only write it when the key is absent.
5. Handling `export.git-add`: when the file has `export.git-add: true` explicitly set, record it in `UserAuthored` and leave it (resolves Open Question 1 from the spec). `force=true` overrides.
6. Unit tests for: fresh file creation, merging into a partially-configured file, preserving comments, preserving non-mindspec keys, detecting user-authored drift, idempotence (second call is a no-op), `force=true` overrides.

**Verification**
- [ ] `go test ./internal/bead/... -run EnsureBeadsConfig` passes all new cases.
- [ ] Golden test: a known input file round-trips through `EnsureBeadsConfig` and produces the expected diff (structural preservation proof).

**Acceptance Criteria**
- [ ] `EnsureBeadsConfig` exists and compiles; exported symbols stable.
- [ ] Calling it twice on the same file produces byte-identical output (idempotence).
- [ ] User-authored keys not in the mindspec-required set are never touched.
- [ ] User-authored `export.git-add: true` is recorded in `UserAuthored`, not silently flipped (without `force`).

**Depends on**
None

## Bead 4: Wire `EnsureBeadsConfig` into init/setup + fix `mindspec spec create` redirect gap

Applies ADR-0023 (bootstrap path) and resolves Open Question 2 by verifying the redirect is the durable mechanism (or fixing the mechanism that creates it, if the verification fails).

**Context added from plan-time investigation**: during this spec's own Plan Mode transition, running `mindspec spec approve` failed with `bd list --type=epic failed: exit status 1` because the newly-created spec worktree had no `.beads/redirect` file — `bd` spawned its own empty dolt server. Inspection of `internal/executor/mindspec_executor.go:InitSpecWorkspace` shows it correctly calls `bead.WorktreeCreate` (which shells out to `bd worktree create`), but the redirect was not written. Either `bd worktree create` is not completing the redirect step when the branch already exists, or a later step in mindspec's flow overwrites the worktree's `.beads/`. This bead investigates and patches the root cause.

**Steps**
1. Reproduce the redirect-missing bug in an isolated test: run `mindspec spec create <id>` end-to-end in a temp repo with `bd` installed, then assert `<worktree>/.beads/redirect` exists and contains the correct relative path to the main repo's `.beads/`.
2. Diagnose: enable bd's `BD_DEBUG_ROUTING=1`, re-run, read the trace. Determine whether `bd worktree create` is failing silently, not being called with the right args, or being called correctly but then the redirect file is clobbered by a subsequent checkout/init.
3. Patch the root cause. Likely shapes: (a) ensure `bead.WorktreeCreate` error is not swallowed; (b) add a post-worktree-create assertion in `InitSpecWorkspace` that the redirect file exists, with remediation if not; (c) fix an ordering bug where the spec files are written to `.beads/` before `bd worktree create` runs. The integration test from Step 1 is the bead's acceptance gate.
4. Call `EnsureBeadsConfig(root, false)` from `cmd/mindspec/init.go:bootstrap.Run`. Print `ConfigResult` summary.
5. Call `EnsureBeadsConfig(root, false)` from `cmd/mindspec/setup.go`'s claude and copilot subcommands, after `chainBeadsSetup`/`chainBeadsSetupCodex`. Print summary.
6. Integration tests (temp-dir + fake `bd` on PATH): (a) `mindspec init` + real `bd init` leaves config mindspec-ready; (b) `mindspec setup claude` after a bare `bd init` patches the config; (c) second run is a no-op.

**Verification**
- [ ] `go test ./internal/bead/... -run Worktree` passes (including the new redirect-missing regression test).
- [ ] `go test ./internal/executor/... -run InitSpecWorkspace` passes (including the redirect assertion).
- [ ] `go test ./cmd/mindspec/... -run "Init|Setup"` passes.
- [ ] Manual smoke: `mindspec spec create NNN-smoke` in a scratch repo — the new worktree has a correct `.beads/redirect` file, and `bd list` from inside the worktree returns main's issues (not empty).

**Acceptance Criteria**
- [ ] Spec AC "fresh `mindspec init` + `bd init` produces a `.beads/config.yaml` with the mindspec-required keys" — covered by integration test (a).
- [ ] Spec AC "running `mindspec init` a second time is a no-op on the config file" — covered by integration test (c).
- [ ] `mindspec spec create <id>` produces a worktree with a correct `.beads/redirect` file — regression test covers it.
- [ ] Open Question 2 from spec is resolved: either the redirect works (verified by test), or this bead fixes the mechanism that should create it.

**Depends on**
Bead 3

## Bead 5: Doctor checks for beads/mindspec integration health

Applies ADR-0023 (doctor surfaces deviations from the single-state-authority + projection model).

**Steps**
1. Extend `internal/doctor/beads.go` with four checks, each returning a `doctor.Check` result:
   - `checkBeadsConfigDrift(root)`: calls a new `EnsureBeadsConfig(root, force=false)` in dry-run mode (or compares against the required-keys constant from Bead 3). Reports each missing or drifted key.
   - `checkStrayRootJSONL(root)`: runs `git ls-files --full-name -- issues.jsonl` from main. If non-empty, warns and suggests `git rm --cached issues.jsonl`.
   - `checkDurabilityRisk(root)`: reads `.beads/config.yaml` for `export.auto`; shells `bd dolt remote list` (or equivalent) to check for a configured remote. If `export.auto=false` AND no remote, warns with remediation.
   - `checkBdVersionFloor(root)`: runs `bd --version`; parses; warns if `< 1.0.2`.
2. Wire a new `--fix` mode on `mindspec doctor` that, for `checkBeadsConfigDrift`, calls `EnsureBeadsConfig(root, force=false)` and surfaces the `ConfigResult`. `--fix --force` passes `force=true`.
3. Unit tests for each check using temp dirs + fake `bd` on PATH.
4. Integration test: run `mindspec doctor` on a scratch repo with default `bd init` → reports config drift; run `mindspec doctor --fix` → patches; re-run → clean.

**Verification**
- [ ] `go test ./internal/doctor/...` passes new tests.
- [ ] Manual smoke: `mindspec doctor` on this repo (which already has the mindspec keys) reports clean beads section.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec doctor` on a bd-default repo reports each missing mindspec key" — covered by integration test.
- [ ] Spec AC "`mindspec doctor --fix` patches; re-running reports clean" — covered by integration test.
- [ ] Spec AC "`mindspec doctor` on a repo with tracked root `issues.jsonl` warns" — covered by unit test.
- [ ] `checkDurabilityRisk` fires exactly when `export.auto=false` AND no Dolt remote.
- [ ] `checkBdVersionFloor` fires on `bd --version` reporting < 1.0.2 and passes on >= 1.0.2.

**Depends on**
Bead 3

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `mindspec next` succeeds with only `.beads/issues.jsonl` dirty | Bead 1 (unit + harness) |
| `mindspec next` refuses on other dirty paths | Bead 1 (unit test) |
| Approve/complete commits include fresh `.beads/issues.jsonl` | Bead 2 (integration test) |
| Committed JSONL is byte-identical to `bd export` at commit time | Bead 2 (integration test) |
| Fresh `mindspec init` + `bd init` produces mindspec-ready config | Bead 4 (integration test a) |
| `mindspec init` is a no-op on second run | Bead 4 (integration test c) |
| `mindspec doctor` reports missing mindspec keys; `--fix` patches | Bead 5 (integration test) |
| `mindspec doctor` warns on tracked root-level `issues.jsonl` | Bead 5 (unit test) |
| LLM harness scenario exercises the friction pattern end-to-end | Bead 1 (`TestLLM_BeadsArtifactPassthrough`) |
| No regression on existing harness scenarios | Beads 1, 2 (CI guard) |
| Domain doc explains JSONL-as-artifact policy | Bead 1 (doc update + ADR reference) |
