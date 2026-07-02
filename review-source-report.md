# MindSpec repo review — dead code, DRY, refactoring, performance
**Date:** 2026-07-02 · **Scope:** 341 tracked Go files (~95.5k lines) in `cmd/`, `internal/`, `plugins/`. The untracked `beads/` nested checkout was excluded.
**Method:** `golang.org/x/tools/cmd/deadcode` (with and without `-test`), `golangci-lint` (`unused`, `dupl`, `gocritic`, `prealloc`, `staticcheck`), plus four parallel subsystem review agents. Every finding below was verified against source.

---

## 1. Dead code (confirmed — unreachable even from tests; safe to delete)

### Whole-file / cluster deletions
| Location | What | Notes |
|:--|:--|:--|
| `internal/hook/helpers.go` | `hasPathPrefix`, `stripEnvPrefixes`, `parseEnvPrefixes`, `isEnvVarName`, `getCwd` | 63 of the file's 68 lines are dead; only `dirExists` is live |
| `internal/gitutil/gitops.go:213,230` | `MainWorktreePath` + `IsMainWorktree` | dead cluster (only reference each other) |
| `plugins/mindspec/embed.go:50,64` | `SkillNames` + `sortStrings` | dead cluster |
| `internal/layout/mover.go:158-167` | `Mover.WithPlan`, `WithRules`, `WithRootDocs` | unused builder methods |

### Single-function deletions
| Location | Function | Notes |
|:--|:--|:--|
| `cmd/mindspec/hook.go:191` | no-op `SetUsageTemplate` | `strings.Replace` old == new — literally replaces a string with itself |
| `internal/doctor/doctor.go:75` | `Run` | thin wrapper; CLI uses `RunWithOptions` |
| `internal/phase/derive.go:713` | `FindActiveBeadForEpic` | superseded by `FindActiveBeadForEpicWithCache` |
| `internal/validate/frontmatter.go:46,52` | `SpecStatusFromBytes`, `SpecIsApproved` | only referenced in a comment |
| `internal/validate/plan.go:647` | `IsDomainCoveredCtx` | only referenced in comments |
| `internal/validate/specid.go:24` | `BeadID` | dead re-export shim; callers use `idvalidate.BeadID` |
| `internal/contextpack/adr.go:11` | `NewADRStore` | |
| `internal/contextpack/builder.go:36` | `readFileContent` | |
| `internal/next/beads.go:53` | `findRoot` | also has a stale `.golangci.yml` unparam carve-out |
| `internal/recording/codex_bootstrap.go:22` | `DefaultCodexConfigPath` | |
| `internal/setup/claude.go:555` | `hasManagedBlock` | |
| `internal/harness/agent.go:124` | `filterEnv` | live path uses `filterEnvPrefix` |
| `internal/harness/asserts.go:104,294` | `assertCommandUsedFlag`, `assertCleanWorktree` | unused even by the LLM harness tests |
| `internal/panel/gate.go:69` | const `skipHumanHint` | |
| `cmd/mindspec/state.go:142-144` | `--mode`/`--spec`/`--bead` flags | registered on the deprecated no-op `state set`, which ignores them |
| `internal/trace/event.go:52-56` | `Event.MarshalJSON` | aliased pass-through; byte-identical to default marshaling |

### Stale config referencing dead/removed code
- `.golangci.yml` unparam exclusions reference **`internal/brownfield/plan.go`** (package no longer exists), **`internal/contextpack/builder.go` `isNeighbor`** (function no longer exists), and **`internal/next/beads.go` `findRoot`** (dead, above). Three stale carve-outs to drop.

### Explicit non-findings (looks dead, isn't)
- `internal/harness` (5,618 non-test lines, zero external importers) is the **LLM eval harness**, driven via `Makefile:17` → `go test ./internal/harness/ -run TestLLM`. Not dead.
- `internal/lint`, `internal/specgate` — test-only repo-guard packages. Intentional.
- 256 functions unreachable from the binary but reachable from tests are almost all intentional test seams (`Set*ForTest`, `MockExecutor`).
- `contextpack.RenderBeadContext` deprecated-API usage (`cmd/mindspec/context.go:48`, `next.go:299,388`) is intentional — preserved for byte-identical no-budget output per its doc comment.

---

## 2. DRY violations

### High impact (drift already happened or security-relevant)
1. **Agent-setup managed-block installer triplicated — and already drifted on symlink safety.** `internal/setup/claude.go:476-530` (`ensureClaudeMD`), `codex.go:50-103` (`ensureAgentsMD`), `copilot.go:44-100` (`ensureCopilotInstructions`) are ~50-line copies of the same upsert flow. The copies have diverged: claude/copilot write via `safeio.WriteFileNoSymlink`/`OpenAppendNoSymlink`, but **codex uses plain `os.WriteFile`/`os.OpenFile` (`codex.go:68,79,96`) and lacks the symlink protection** the repo added deliberately (there are tests asserting symlink refusal for the other two). Fix: extract `ensureManagedDoc(root, relPath, fullContent, appendBlock, check, r)` on safeio. Also `chainBeadsSetup`/`chainBeadsSetupCodex` (`claude.go:536`, `codex.go:109`) differ only by the agent string.
2. **Override-flag validation duplicated across commands.** The `--allow-doc-skew`/`--override-adr`/`--supersede-adr` empty-reason + ADRID + mutual-exclusion validation block is byte-identical in `cmd/mindspec/complete.go:86-107` and `cmd/mindspec/impl.go:72-93`; flag registration is triplicated (`complete.go:145`, `impl.go:36`, `approve.go:56`). This is escape-hatch/audit validation — the exact place silent drift hurts. Fix: `parseOverrideFlags(cmd)` + `addOverrideFlags(cmd, scope)`.
3. **Frontmatter fence-scanning re-implemented against the canonical package's explicit prohibition.** `internal/frontmatter.Parse` is documented as the single implementation ("callers must NOT re-implement… they will silently drift"), yet: `internal/approve/plan.go:207-268` and `:686-745` are ~40-line near-duplicate read-mutate-rewrite scanners; `approve/spec.go:270`, `approve/impl.go:440-465`, `contextpack/budgeter.go:581-631`, and `validate/plan.go:266-303` each hand-roll the `---` fence scan. Drift is real: `frontmatter.Parse` requires `TrimRight(line,"\r\n")=="---"` while `parsePlanFrontmatter` accepts `TrimSpace`d fences. Separately, `validate/state.go:189-215` (`readSpecApprovalStatus`) still substring-scans the `## Approval` prose that `validate.SpecStatus` (YAML frontmatter) was created to replace — two sources of truth that can disagree on the same spec.md.
4. **Two hand-rolled TOML parsers for the same `~/.codex/config.toml`.** `internal/recording/codex_bootstrap.go:107-380` (~200 lines of line-based helpers) vs `internal/otel/config.go:286-405` + `otel/status.go:124-214` (regex-scoped strategy). Both upsert the same `[otel]` block in the same file; the otel doc comment claims it "reuses the existing helper patterns from codex_bootstrap.go" but reimplements them, so the two can classify the same TOML differently. Consolidate — or delete the legacy recording path if otel (ADR-0027) supersedes it.
5. **`complete.queryAllChildren` duplicates `phase.fetchChildren`** — see Perf #1; it's both a copy and a subprocess fan-out.

### Medium impact
6. **Markdown `## section` extraction implemented ~7 ways**: `adr/show.go:67` (`ExtractDecision`), `domain/show.go:100` (`extractSection`), `validate/spec.go:84` (`parseSections`), `validate/plan.go:830` (`hasSection`), `domain/list.go:41`, `contextpack/spec.go:19`, `contextpack/budgeter.go:552`, `instruct/instruct.go:256` — while the generic `contextpack.ExtractSection` (`builder.go:11`) already exists. Copies have drifted on `##`-vs-`#` terminators and `---` handling. One shared helper (e.g. `internal/mdsection`).
7. **`bd show <id> --json` → unmarshal slice → take `[0]` in 7 places**: `contextpack/beadctx.go:40`, `contextpack/budgeter.go:151`, `next/beads.go:178` (`FetchBeadByID`, the generic form), `bead/bdcli.go:249`, `phase/cache.go:233`, `phase/derive.go:653`, `approve/impl.go:421`. One `bead.ShowOne` helper.
8. **OTEL env-var key map built identically in 5 places**: `otel/config.go:224,424`, `otel/env.go:44,108` (`canonicalEnvOrder`), `recording/bootstrap.go:58`. Renaming one key means five edits.
9. **`git status --porcelain` parser hand-rolled ×3**: `next/guard.go:79`, `panel/gate.go:433`, `layout/mover.go:624` — identical XY/rename decode, different filters. Share the tokenizer, keep the filters local.
10. **`cmd/mindspec` command-wiring duplication**: `spec.go:30-71` ≡ `spec_init.go:20-61` (byte-identical 42-line RunE — point the hidden alias at `specCreateCmd.RunE`); the three `setup` subcommand RunEs (`setup.go:38,79,122`) differ only in the run function + label; `next.go:155-180` vs `runEmitOnly` `:356-386` duplicate the ready-work query + spec filter; `approveSpecRunE`/`approvePlanRunE` share a verbatim preflight-warning scaffold (`spec.go:105`, `plan_cmd.go:46`).
11. **Harness internals**: six assert helpers repeat the same event-scan skeleton (`asserts.go:13-136`); the two recorder shim scripts are ~90% identical (`recorder.go:15-119`, including a duplicated 15-line phase-cache block); three arg-extraction helpers (`flatArgs`/`eventArgsList`/`eventArgs`) have **divergent semantics** — an analyzer rule and an assert can disagree on the same event.

### Low impact (worth batching into a cleanup bead)
12. `idvalidate/ids.go` — four validators repeat the same guard prologue (SEC-1 chokepoint; table-drive it).
13. `phase/derive.go:568-581` `readStoredPhaseWithCache` inlines `extractPhaseFromMetadata` verbatim (`:204-218`).
14. `guard/guard.go:31-67` vs `hook/hook.go:203-231` — same worktree-resolution ladder; both carry private `dirExists`.
15. `bootstrap/mergedriver.go:202-222` and `doctor/beads.go:723,736` shell out `git config` directly — `gitutil` (the ADR-0030 git boundary) has no `ConfigGet/ConfigSet` to route through.
16. Scattered trivial helpers: `readFileOrEmpty`/`readFileContent` ×3, `firstPathSegment`/`firstSegment` ×2, `exists`/`dirExists`/`fileExists` in six packages; `config.Load`-or-default idiom inline ×3 in `cmd` when `next.go:430` already extracted it; `validate.ListDomainDirs`-style `ReadDir`+`IsDir`+sort in ~6 places across validate/doctor/domain/spec.
17. `validate/plan.go:348-472` `ParseBeadSections` — the four-flag reset block repeats ~6×; model the current subsection as one enum + a `flush` closure. Also `plan.go:848-854` two warning arms emit the identical message (collapse to `||`).
18. Test-only duplication flagged by `dupl` (setup idempotency tests ×3 agents, `symlink_refusal_test.go` pair, `workspace_test.go` pair) — fine to leave, or table-drive per agent.

---

## 3. Performance

1. **[High] OWNERSHIP.yaml re-loaded once per (changed-file × domain).** `validate/ownership.go:299-314` (`attributeDomain`) calls `loadOwnershipForRef` per domain — a fresh `ReadFile`+`yaml.Unmarshal`, or up to 3 `git show` subprocesses when reading at a ref — and is invoked once per changed file from `divergence.go:180-204` and `docsync.go:524`. One divergence gate = N×D parses/git-spawns for the same handful of manifests. `docsync.go:220-230` (`checkUnclaimedSource`) already shows the fix: load all manifests once, attribute in memory.
2. **[High] `mindspec complete` fans out ~5 `bd` subprocesses where 1 suffices.** `complete/complete.go:884-905` (`queryAllChildren`) loops `bead.AllStatuses` issuing one `bd list --parent --status=<s>` per status; `phase/cache.go:213-230` (`fetchChildren`) does the identical job in one comma-joined call and is memoized. The comment even says it "mirrors phase.queryChildren" — a helper that no longer exists in that form.
3. **[Medium] `complete.Run` re-resolves the immutable spec→epic mapping 4×** (`complete.go:223,228,716,781`), each constructing a throwaway `phase.NewCache()` and running a full `bd list --type=epic` subprocess. `instruct/run.go:75` already threads one cache through; resolve `epicID` once (keep the post-close children query fresh — `complete` mutates bd mid-run).
4. **[Medium] Plan validation re-parses cited ADRs O(domains × citations).** `validate/plan.go:534-561` → `coverageOf` (`:660-745`) calls `store.Get` per citation per domain; every `Get` is an `os.ReadFile` + full parse (`adr/show.go:25`, `adr/parse.go:39`), doubled by `OverlayStore`. Parse the cited set once into a map, or add a memoizing Store decorator.
5. **[Medium] `doctor` walks the entire repo tree once per domain.** `doctor/ownership.go:121-153` (`manifestResolvesAny`) does a full `filepath.WalkDir(root)` per domain, and the dead-manifest case (the one the check exists for) always walks to completion → O(domains × repo files). Walk once, cache the file list, test globs against it.
6. **[Medium-low] `migrate` spawns one `git check-ignore` per markdown file** (`cmd/mindspec/migrate.go:264,496` → `gitutil.CheckIgnore`). Batch via `git check-ignore --stdin`. Rare one-shot command, so low urgency.
7. **[Low] Misc**: prealloc sites (`gitutil/gitops.go:646`, `panel/gate.go:277`, `approve/spec.go:219`); harness analyzer makes ~8 separate full passes over the event stream (`analyzer.go:216-461`); `harness/asserts.go:377-406` spawns one `git diff-tree` per commit (test-only).

---

## 4. Suggested cleanup order

1. **Dead-code sweep** (Section 1 + stale `.golangci.yml` carve-outs) — pure deletions, zero behavioral risk, ~500 lines removed.
2. **Setup managed-block unification** — fixes the real codex symlink-safety gap while deduplicating (DRY #1).
3. **`complete` perf pair** — DRY #5 / Perf #2+#3 are one small change each on the most-used lifecycle command.
4. **Frontmatter consolidation** (DRY #3) — highest correctness payoff; the canonical package already exists and documents the rule.
5. **Ownership/ADR validation caching** (Perf #1, #4, #5) — one shape of fix (hoist loads, pass maps down) across three gates.
6. Batch the low-impact DRY items into a single cleanup bead.
