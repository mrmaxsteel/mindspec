---
adr_citations:
    - id: ADR-0011
    - id: ADR-0026
    - id: ADR-0027
    - id: ADR-0028
approved_at: "2026-05-20T07:50:46Z"
approved_by: user
bead_ids:
    - mindspec-buh3.1
    - mindspec-buh3.2
    - mindspec-buh3.3
    - mindspec-buh3.4
spec_id: 084-mindspec-otel-only
status: Approved
version: "1"
---
# Plan: 084-mindspec-otel-only

## ADR Fitness

- **ADR-0011** (One-way `mindspec → agentmind` dependency via OTLP/HTTP:4318):
  **superseded in part** by this spec. The one-way dependency survives as a
  *concept* (telemetry flows from workloads to whichever OTLP/HTTP endpoint
  the user points at), but the literal Go-module dependency is removed.
  Bead 4 adds a prose postscript to ADR-0011 reflecting the no-Go-dep
  posture. The wire shape (OTLP/HTTP on 4318) is unchanged; mindspec just
  never participates in that wire as a producer of the receiver.
- **ADR-0026** (AgentMind extracted to standalone repo): carried forward
  unchanged. This spec is the *second half* of the move ADR-0026
  envisioned — spec 083 moved the code; spec 084 removes the residual
  coupling.
- **ADR-0027** (new — "MindSpec is OTEL-config only"): authored in Bead 4
  alongside the README/CHANGELOG pass. Records that mindspec emits
  telemetry through whatever OTEL endpoint the user supplies, never
  spawns a receiver, and treats agentmind as one of many possible
  downstream collectors (not the privileged one). Documents the rollback
  procedure (`git revert` of the merge commit).
- **ADR-0028** (new — "Bench removed from mindspec"): drafted in Bead 3
  alongside the bench deletion so the ADR text mirrors the as-shipped
  state. Records the deletion, the rescue procedure (`git checkout
  pre-spec-084-bench-delete -- internal/bench/`), and the explicit
  "extraction is not mindspec's problem" stance.

No accepted ADR is contradicted by this plan; ADR-0011's literal
import surface is the only thing being narrowed, and the prose
postscript makes the narrowing explicit.

## Testing Strategy

This spec's failure mode is **symbolic extraction** — code renamed but
not removed, leaving the agentmind surface in mindspec under a new
name. Spec 083's panel proved that one-shot grep checks at merge time
are blind to this pattern; this plan layers static + AST + runtime +
binary-size + e2e gates so every bead lands with an enforceable
boundary, and the **permanent specgate test in Bead 4 is the lifetime
invariant** preventing re-introduction.

**Bead ordering note**: per spec Migration Commits 1-2 (lines 723-727),
new otel surfaces land FIRST so users can diff old `agentmind setup`
vs new `otel setup` output for two commits before bench/agentmind
deletion. Bead 1 ships the otel skeleton, Bead 2 rewires `record.go`,
Bead 3 performs the deletion + deprecation stubs, Bead 4 drops the Go
module dep and lands the permanent specgate. This preserves the
spec's deliberate equivalence-verification window.

Tests A–K from spec.md map to beads as follows:

- **Test A** (no-agentmind-import; `go list -deps`): green-by-absence
  after Bead 4 lands the dep-drop; the permanent specgate test
  introduced in Bead 4 is the runtime enforcement.
- **Test B** (`go.mod` / `go.sum` cleanliness): green after Bead 4's
  `go mod tidy`.
- **Test C** (`mindspec --help` golden-file diff): Bead 3 ships the
  `cmd/mindspec/testdata/help-golden.txt` golden and the diff
  assertion. Cleared by Bead 3 (the rename + deletion) and re-asserted
  in Bead 4 (after deprecation messages and ADR/README updates).
- **Test D** (deprecation-message contract, AST-checked): Bead 3
  introduces the deprecation stubs and
  `cmd/mindspec/deprecated_commands_test.go`. Per-command stderr-line
  equality is asserted verbatim from spec lines 411-417's migration
  table — no single-template collapse.
- **Test E** (mindspec opens no TCP listener; portable Go runtime on
  Linux/Darwin): Bead 2 ships the test in
  `internal/recording/no_listener_test.go` alongside the `record start`
  rewrite — that is the bead where any latent listener would have been
  introduced. Runs on both `ubuntu-22.04` and `macos-14` CI runners.
- **Test F** (`record start` exit-code propagation + clean stderr): Bead 2.
- **Test G** (process-tree audit via `pgrep -P`): Bead 2.
- **Test H** (no-spawn AST check + net-call AST check + the
  two-allow-listed-files string-literal AST gate): part of the
  permanent specgate test in Bead 4; runs in every `go test -short`
  from Bead 4 forward. Scope explicitly includes `net.Dial`,
  `http.Get`, `http.Post`, `http.Client.Do`, and `url.Parse → Dial`
  call-site literals under `cmd/mindspec/otel.go` and `internal/otel/`
  (closes the "doctor-by-another-name" hole per spec lines 278-285),
  in addition to `exec.Command` / `exec.LookPath` / `os.StartProcess`
  first-argument literals everywhere under `cmd/` and `internal/`.
- **Test I** (binary-size shrinkage ≥30%): Bead 4 records the
  post-delete size, computes the delta against the pinned baseline
  (`10,734,354 bytes` on **`macos-14` / `darwin-arm64`** — same CI
  runner and architecture as the spec's pinned baseline), and fails
  the merge if shrinkage is <30%. The CI step that runs Test I MUST
  be pinned to `runs-on: macos-14` so the measurement environment
  matches the baseline.
- **Test J** (end-to-end point-at-a-real-collector check): Bead 4
  wires `.github/workflows/spec-084-test-j.yml` with the
  `otel/opentelemetry-collector-contrib` image pinned by sha256
  digest (no `:latest`, no floating tags). The chosen digest is
  recorded in the workflow YAML and rotated via Renovate's digest
  pinning (or manually on a documented quarterly cadence). Mandatory
  per spec.
- **Test K** (bench-rescue procedure / `pre-spec-084-bench-delete` tag
  survives squash-merge): the annotated tag is pushed to `origin` in
  Bead 3 *before* the deletion commit lands. A pre-merge proxy
  assertion runs at the end of Bead 3. The authoritative post-merge
  assertion is wired as a `.github/workflows/spec-084-test-k.yml`
  workflow that triggers on push to `main` and re-runs
  `git show pre-spec-084-bench-delete:internal/bench/runner.go` from
  a clean checkout of `main` after the squash-merge lands (per spec
  lines 697-706, which require post-merge verification). The
  annotated-tag immutability property (tags pushed to `origin` are
  not rewritten by squash-merge of the PR branch) is documented in
  Bead 3's commit message as the load-bearing invariant.

Hard Constraint #8 ("every commit builds and tests green") is enforced
per-bead: each bead's verification block ends with `go build ./cmd/mindspec
&& go test -short ./...` passing on every commit it produces.

## Bead 1: Add `internal/otel/` + `cmd/mindspec/otel.go` skeleton (otel surfaces land first)

The otel-first commit per spec Migration Commits 1-2 (lines 723-727).
Lands the new `mindspec otel setup` and `mindspec otel status` cobra
surface alongside the `internal/otel/` rendering package, while the
existing `mindspec agentmind setup` command remains in place untouched.
This preserves the spec's deliberate two-commit equivalence-verification
window: users can diff `mindspec agentmind setup --endpoint X` output
against `mindspec otel setup --endpoint X` output on identical inputs
before either is removed.

**Steps**

1. Add `internal/otel/config.go` (~150 LOC): pure-function rendering
   of an OTEL endpoint + protocol + headers into (a) Claude Code
   `.claude/settings.local.json` and (b) Codex `~/.codex/config.toml`.
   Implementation handles the spec's `--codex` contract verbatim —
   `[otel.exporter]` table replaced, sibling top-level keys preserved
   byte-for-byte, parent dir created with mode `0700` if absent,
   exit 1 on malformed pre-existing TOML, sha256-idempotent on
   re-run with identical inputs. No `net.Dial`, no `http.Client`,
   no `url.Parse`-followed-by-`Dial`. Pure file I/O only.

   **TOML merge contract for `--codex`**: implementation uses
   `github.com/BurntSushi/toml` for parsing the existing config to
   an in-memory map, replaces only the `[otel.exporter]` table (whole
   table replacement, no key-level merge inside it), then re-emits
   via `BurntSushi/toml`'s encoder configured with
   `Indent=""` and stable key ordering (alphabetical within each
   table) so re-runs are byte-deterministic. Sibling top-level
   tables and keys are read into the map and re-emitted unchanged.
   If `BurntSushi/toml`'s round-trip is non-deterministic in
   practice (it does not preserve comments), the implementer MUST
   fall back to the documented regex-based key-replacement strategy:
   anchored regex `(?ms)^\[otel\.exporter\][^\[]*` matches the
   stanza, replacement string is the freshly-rendered
   `[otel.exporter]` block; siblings remain byte-for-byte. The
   choice between round-trip and regex replacement MUST be recorded
   in `internal/otel/config.go`'s package doc comment. sha256
   idempotency is the binding AC (spec line 569) regardless of
   strategy.

2. Add `internal/otel/config_test.go` (~200 LOC): pure-function
   tests for the rendering paths. Specifically:
   - the `--codex` merge-semantics test (sibling tables preserved
     byte-for-byte);
   - the sha256-idempotency assertion (per spec AC line 569: re-run
     with identical inputs produces sha256-identical output);
   - the secret-redaction test (header values matching
     `(?i)bearer|token|key|secret|password` redacted to `***` in
     status output but written verbatim to the target file);
   - the exit-code matrix tests (0 / 1 / 2 only; no other exit
     codes).
3. Add `cmd/mindspec/otel.go` (~120 LOC): the cobra commands
   `mindspec otel setup` and `mindspec otel status`. Setup accepts
   `--endpoint`, `--protocol`, `--headers`, `--codex`,
   `--codex-config`. Status is read-only — reads
   `.claude/settings.local.json` and `~/.codex/config.toml` (or the
   path from `--codex-config`), prints a stable human-readable
   report to stdout. **No network I/O** anywhere in this file or in
   `internal/otel/` — Bead 4's Test H enforces this AST-statically;
   reviewers MUST manually verify net-call absence on every commit
   to these files until Bead 4 lands.
4. Add `cmd/mindspec/testdata/otel-status-golden.txt` and an
   `otel_status_test.go` golden-file diff for the `mindspec otel
   status` report shape (per spec lines 286-295).
5. Register `otelCmd` in `cmd/mindspec/root.go`. The existing
   `mindspec agentmind setup`, `mindspec viz`, `mindspec bench`,
   etc. surfaces remain in place untouched in this bead — they are
   removed in Bead 3.
6. Run `go build ./cmd/mindspec && go test -short ./...`. All green.
7. **Equivalence-window check (informal):** run both `mindspec
   agentmind setup --endpoint http://example:4318` and `mindspec
   otel setup --endpoint http://example:4318` against a temp HOME,
   diff the resulting `.claude/settings.local.json` files, record
   the diff (expected: identical or differing only in trailing
   whitespace / key ordering — and if it differs in any
   semantically significant way, halt and reconcile before Bead 3).

**Verification**
- [ ] `internal/otel/config.go` and `internal/otel/config_test.go`
      exist; all subtests (merge-semantics, idempotency, secret-
      redaction, exit-code matrix) pass.
- [ ] `cmd/mindspec/otel.go` exists; `mindspec otel --help` lists
      exactly `setup` and `status`.
- [ ] `mindspec otel setup --endpoint http://collector.example:4318`
      in a fresh repo writes `.claude/settings.local.json`
      containing the endpoint exactly once; re-running with
      identical inputs produces a sha256-identical file (idempotency).
- [ ] `mindspec otel setup --endpoint … --codex` writes the
      `otel.exporter` stanza to `~/.codex/config.toml` and is
      sha256-idempotent on re-run; sibling top-level keys preserved
      byte-for-byte.
- [ ] `mindspec otel status` produces stable output matching the
      golden file.
- [ ] Existing `mindspec agentmind setup` and other legacy commands
      still build and execute (no removals in this bead).
- [ ] Equivalence-window diff (step 7) is empty or semantically
      identical; result is recorded in the commit message.
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec otel setup --endpoint
      http://collector.example:4318` … sha256-identical on re-run"
      is satisfied.
- [ ] Spec AC "`mindspec otel setup --endpoint … --codex` … Codex
      `~/.codex/config.toml` contains matching `otel.exporter`
      stanza; sha256-idempotent on re-run" is satisfied.
- [ ] Spec Hard Constraint #5 (exit-code matrix for `mindspec otel
      setup` and `mindspec otel status`) is enforced by the
      `internal/otel/config_test.go` exit-code tests.
- [ ] Spec Migration Commit 1-2 ordering invariant ("new surfaces
      land first") is satisfied — bench/agentmind surfaces remain
      live through this bead.
- [ ] Hard Constraint #8 (every commit builds + tests green) holds
      for this bead's commits.

**Depends on**
None.

## Bead 2: Rewire `cmd/mindspec/record.go` to use the `internal/otel/` surface

Rewrites `cmd/mindspec/record.go` to ~80 lines of pure
config-emit + workload-launch, using the `internal/otel/` package
shipped in Bead 1 (no env-var direct-reads — load via the renderer
so the config path is identical to the final shape). At this bead,
`internal/recording/collector.go` and the `mindspec agentmind serve` /
`mindspec viz` subtree still exist; their deletion is Bead 3. This
keeps the diff-equivalence window open for one more commit while
the runtime path is migrated.

**Steps**

1. Rewrite `cmd/mindspec/record.go` to ~80 lines: call into
   `internal/otel/config.go` to load the OTEL endpoint/protocol/
   headers (same loader that powers `mindspec otel setup` and
   `mindspec otel status`), set the workload's environment, exec
   the workload via `exec.Cmd`, write the recording-directory
   manifest + phase markers, and exit with whatever code the
   workload returned. No NDJSON reader, no port wrangling, no
   `client.AutoStart`.
2. Drop the `--spec` flag's pre-spec coupling to bench/agentmind (if
   any latent flags exist beyond the one documented in spec lines
   421-434, follow the spec's amendment-bead rule and document the
   amendment in the bead's commit message). The bead's verification
   block enumerates `recordStartCmd`'s flag set explicitly so any
   undeclared hidden flag fails the check (closes R6:C6 / spec lines
   432-441).
3. Add `internal/recording/no_listener_test.go` (Test E): portable
   Go test that spawns `mindspec record start --spec test -- bash
   -c 'sleep 2'` with `OTEL_EXPORTER_OTLP_ENDPOINT` set, enumerates
   mindspec's open TCP sockets via `/proc/<pid>/net/tcp` on Linux
   and `lsof -p <pid> -iTCP -sTCP:LISTEN -n -P -F pn` on Darwin
   (sampled every 100ms until workload exit), and asserts zero
   listening sockets. Test uses `t.Errorf` (not `t.Skip`) when GOOS
   is linux/darwin so accidental skips fail CI. Skip is acceptable
   only on other GOOS values. CI matrix pins `ubuntu-22.04` and
   `macos-14`.
4. Add `cmd/mindspec/record_test.go` covering Test F (exit-code
   propagation): start a fresh tmp repo, set
   `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535`, run
   `mindspec record start --spec test -- bash -c 'echo workload;
   exit 42'`, assert exit code 42, stdout contains `workload`,
   stderr is empty, and the recording manifest exists. Same test
   file covers Test G (process-tree audit): snapshot
   `pgrep -P <mindspec-pid>` at 200ms, assert the only child is the
   workload `bash` process — no `agentmind`, no `otelcol`.
5. The legacy `internal/recording/collector.go` and `cmd/mindspec/
   viz.go` files still exist at this bead's HEAD; they are removed
   in Bead 3. `record.go` MUST NOT import them after this bead's
   rewrite — verified by `go list -deps ./cmd/mindspec | grep -v
   'recording/collector\|cmd/mindspec/viz'` (or equivalent).
6. Run `go build ./cmd/mindspec && go test -short ./...`. The
   `agentmind` Go module dep is still in `go.mod` (Bead 4 removes
   it) and `internal/recording/collector.go` still imports
   `agentmind/client`, so the build resolves; but the new
   `record.go` does not.

**Verification**
- [ ] `cmd/mindspec/record.go` is ≤120 lines (target ~80 per spec
      line 104; ≤120 is the lint ceiling).
- [ ] `cmd/mindspec/record.go` imports `internal/otel` (not
      `agentmind/client` or `agentmind/wire`).
- [ ] `recordStartCmd` flag enumeration recorded in commit message
      matches spec lines 421-434 (only `--spec <id>`); any
      additional flag triggers an amendment bead per spec lines
      432-441.
- [ ] Test E passes on `ubuntu-22.04` and `macos-14`; neither
      run produces a `t.Skip` when GOOS is linux/darwin.
- [ ] Tests F and G pass.
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "with `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535`
      … `mindspec record start --spec test -- echo hi` exits 0,
      prints `hi`, writes manifest + recording skeleton, stderr does
      not mention OTEL/agentmind/telemetry" is satisfied (Test F).
- [ ] Spec Hard Constraint #4 ("mindspec opens no TCP listener of
      its own") is satisfied — proved by Test E on both Linux and
      Darwin runners.
- [ ] Spec Test G (process-tree audit; only child is the workload)
      passes.
- [ ] Hard Constraint #8 (every commit builds + tests green) holds
      for this bead.

**Depends on**
Bead 1.

## Bead 3: Delete `internal/bench/`, `mindspec agentmind` cobra subtree, and `client.*` callers; ship deprecation stubs

The deletion bead. Combines the bench-subtree deletion (with
rescue-tag setup), the `mindspec agentmind` cobra subtree deletion,
the `internal/recording/collector.go` deletion, the
`internal/agentmind/` directory deletion (belt-and-suspenders), and
the per-command deprecation stubs in
`cmd/mindspec/deprecated_commands.go`. Also drafts ADR-0028 to mirror
the as-shipped state. After this bead, mindspec has zero
`client.AutoStart` / `client.ReadEvents` / `client.RunStandalone`
call-sites; only the `go.mod` dep line and the test-allowed
`agentmind` literal in `deprecated_commands.go` remain (Bead 4
finishes the cleanup).

**Steps**

1. Capture the pre-delete `go build ./cmd/mindspec` binary size on
   `macos-14 / darwin-arm64` for the size-delta baseline
   cross-check; sanity-check that the measured value equals the
   spec's pinned baseline `10,734,354 bytes` (spec line 342). **If
   the measured value differs by more than ±0.5%, halt and
   reconcile with the spec before proceeding** — drift here means
   the size gate's denominator is wrong. (Test I evaluation lives
   in Bead 4; the pre-delete sample is captured here for traceability.)
2. From the integration branch, `git tag -a pre-spec-084-bench-delete
   -m "Pre-deletion snapshot of internal/bench/ for spec 084 (HC #11
   option b)" HEAD` against the parent commit (the post-Bead-2 HEAD),
   then `git push origin pre-spec-084-bench-delete`. The tag MUST
   exist on `origin` BEFORE any deletion commit is pushed; the
   annotated-tag-pushed-to-origin property is what makes the rescue
   handle survive squash-merge (Test K). The commit message records
   "annotated tags pushed to origin are immutable across squash-merge
   of feature branches" as the load-bearing invariant.
3. `git rm -r internal/bench/` and `git rm cmd/mindspec/bench*.go`.
   Delete `cmd/mindspec/viz.go` and remove its `init()`-time cobra
   registration from `cmd/mindspec/root.go` (kills `mindspec viz`,
   `mindspec agentmind serve`, `mindspec agentmind replay`, and the
   old `mindspec agentmind setup` in one move — the new
   `mindspec otel setup` from Bead 1 is unaffected). Delete
   `internal/recording/collector.go` and its companion test
   `internal/recording/collector_test.go`. Delete `internal/agentmind/`
   if any remnant survived spec 083.
4. Add `BENCH-MOVED.md` at the repo root (~30 lines). Content: a
   paragraph pointing to the `pre-spec-084-bench-delete` annotated
   tag and the exact rescue command (`git checkout
   pre-spec-084-bench-delete -- internal/bench/`); a one-paragraph
   "extraction is not mindspec's problem" stance; a link to ADR-0028.
5. Draft `.mindspec/docs/adr/ADR-0028-bench-removed-from-mindspec.md`
   in the same commit. Status: Accepted. Sections: Context (cite
   spec 084), Decision (bench is gone from mindspec; rescue lives at
   the annotated tag), Rescue procedure, Cross-reference to ADR-0027.
6. Add `cmd/mindspec/deprecated_commands.go` (~70 LOC): hidden
   cobra commands for the removed top-level commands. Each command
   emits **exactly one stderr line** matching the spec's per-command
   migration table (spec lines 411-417) **verbatim** — no
   single-template collapse. The exact stderr lines (these are the
   strings Test D asserts against):

   - `mindspec bench run` →
     `"bench moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0028 for rationale)"`
   - `mindspec agentmind serve` →
     `"agentmind serve moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind serve' (see ADR-0027)"`
   - `mindspec agentmind replay` →
     `"agentmind replay moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind replay' (see ADR-0027)"`
   - `mindspec viz` →
     `"viz moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind viz' (see ADR-0027)"`
   - `mindspec agentmind setup` →
     `"agentmind setup renamed: use 'mindspec otel setup' (see ADR-0027 for rationale)"`

   Each emits its exact stderr line and exits with code 2. This is
   the **only** file under `cmd/` or `internal/` (outside the
   specgate test) permitted to contain the literal substring
   `agentmind` per Hard Constraint #2. If spec lines 411-417 diverge
   from this enumeration at implementation time, the spec text
   wins; update this step accordingly.
7. Add `cmd/mindspec/deprecated_commands_test.go` (Test D): exec
   each of the five commands above against the freshly built
   binary, assert exit code 2 and **stderr line equality** against
   the exact strings above (not pattern matching, not template
   matching).
8. Add `cmd/mindspec/testdata/help-golden.txt` and an
   `help_golden_test.go` (Test C): capture `mindspec --help` output,
   assert it contains none of `agentmind`, `bench`, `serve`,
   `replay`, `viz` as visible (non-hidden) subcommands; capture
   `mindspec otel --help`, assert it lists exactly two subcommands
   (`setup`, `status`). The golden is asserted-against on every PR.
9. Run the spec's Hard Constraint #2 grep — with exclusions matching
   files that EXIST at this bead's HEAD:
   `grep -rn 'agentmind\|wire\.CollectedEvent\|client\.AutoStart\|client\.ReadEvents\|client\.RunStandalone'
   cmd/ internal/ --exclude='deprecated_commands*.go'`. After this
   bead, the grep MUST return zero matches. The
   `verify_no_agentmind*.go` exclusion is intentionally NOT added
   here (the file does not exist until Bead 4); Bead 4's verification
   re-runs the grep with the additional exclusion once that file is
   in place.
10. Register the hidden deprecated-commands group in
    `cmd/mindspec/root.go`. Run `go build ./cmd/mindspec && go test
    -short ./...`. The `agentmind` Go module dep is still in
    `go.mod` (Bead 4 removes it), so the build resolves; but no
    mindspec code imports `agentmind/client` or `agentmind/wire`
    after this bead.
11. **Test K pre-merge proxy assertion (run last):** in a fresh
    worktree from the integration branch's tip, run `git show
    pre-spec-084-bench-delete:internal/bench/runner.go | head -n 20`
    and assert non-empty output. Record the assertion result in the
    bead's commit message. The authoritative post-merge Test K
    workflow is added in Bead 4 (`.github/workflows/spec-084-test-k.yml`).

**Verification**
- [ ] `find internal/bench -type f` returns no results.
- [ ] `find cmd/mindspec -name 'bench*.go'` returns no results.
- [ ] `find cmd/mindspec -name 'viz*.go'` returns no results.
- [ ] `find internal/recording -name 'collector*'` returns no results.
- [ ] `find internal/agentmind -type f` returns no results.
- [ ] `BENCH-MOVED.md` exists at repo root and cites the
      `pre-spec-084-bench-delete` annotated tag.
- [ ] `git ls-remote --tags origin | grep pre-spec-084-bench-delete`
      returns a match (the tag is reachable from `origin`, not a
      worktree-local ref).
- [ ] ADR-0028 file exists with Status=Accepted.
- [ ] Hard Constraint #2 grep (step 9) returns zero matches.
- [ ] `mindspec --help` golden test (Test C) passes — none of
      `agentmind`/`bench`/`serve`/`replay`/`viz` appear as visible
      subcommands.
- [ ] `mindspec otel --help` lists exactly `setup` and `status`.
- [ ] Deprecation-message contract test (Test D) passes with
      verbatim stderr-line equality for all five removed commands.
- [ ] Test K pre-merge proxy assertion (step 11) passes.
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "`find internal/bench -type f` returns no results" is
      satisfied.
- [ ] Spec AC "`find cmd/mindspec -name 'viz*.go' -o -name
      'bench*.go'` returns no results" is satisfied.
- [ ] Spec AC "`find internal/recording -name 'collector*'` returns
      no results" is satisfied.
- [ ] Spec AC "`find internal/agentmind -type f` returns no
      results" is satisfied.
- [ ] Spec AC "`mindspec --help` contains none of: `agentmind`,
      `bench`, `serve`, `replay`, `viz`. `mindspec otel --help`
      lists exactly two subcommands" is satisfied (Test C).
- [ ] Spec AC "`mindspec viz`, `mindspec agentmind serve`,
      `mindspec agentmind replay`, `mindspec agentmind setup`,
      `mindspec bench run` each exit code 2 with one stderr line
      matching the migration table" is satisfied (Test D) — per
      spec lines 411-417, the stderr lines are verbatim per-command,
      not template-generated.
- [ ] Spec Hard Constraint #7 (one-shot deprecation messages, exit
      2, one stderr line per migration table entry) is enforced by
      Test D.
- [ ] Spec AC "ADR-0028 committed and cross-referenced from this
      spec" — ADR-0028 half is drafted here; the spec.md
      cross-reference is added in Bead 4 alongside the ADR-0027
      cross-reference.
- [ ] Hard Constraint #8 (every commit builds and tests green) holds
      for this bead.

**Depends on**
Bead 2.

## Bead 4: Drop the `agentmind` Go module dep, land the permanent specgate test, verify size shrinkage, ship ADRs and README

The cleanup bead. After Beads 1–3 there are no remaining `client.*`
callers in mindspec, so `go mod tidy` removes
`github.com/mrmaxsteel/agentmind` from `go.mod` and `go.sum`. The
permanent specgate test
(`internal/specgate/verify_no_agentmind_dep_test.go`) lands in this
bead **for the first time** so its initial state is its permanent
enforced state (no build tags, no skips, runs unconditionally in
`go test -short` from this commit forward) — per spec Migration
Commit 6 (lines 762-770), which explicitly rejects the silenced-by-
build-tag pattern. Binary-size delta is recorded against the pinned
baseline on the spec-pinned runner (`macos-14 / darwin-arm64`).
ADR-0027, the ADR-0011 postscript, README rewrite, CHANGELOG,
Test J CI workflow, and Test K post-merge workflow all land here.

**Steps**

1. Run `go mod tidy` to remove the `require github.com/mrmaxsteel/
   agentmind` line from `go.mod` and the corresponding entries from
   `go.sum`. Do not hand-edit `go.sum`. Verify
   `go list -m all | grep mrmaxsteel` returns only
   `github.com/mrmaxsteel/mindspec`. Final build assertion in this
   bead runs as
   `GOFLAGS=-mod=readonly go build ./cmd/mindspec && go test -short
   ./...` (spec AC line 557 requires the `-mod=readonly` form
   verbatim).
2. Add `internal/specgate/verify_no_agentmind_dep_test.go` (~150 LOC).
   Three test functions:
   - `TestNoAgentmindInDepGraph`: runs `go list -deps ./...` (via the
     `go/build` package, not by shelling out) and asserts no
     `mrmaxsteel/agentmind` package appears.
   - `TestNoAgentmindExecLiteral` (Test H — process-spawn half):
     AST-walks every `*.go` file under `cmd/` and `internal/`,
     enumerates `exec.Command`, `exec.LookPath`, and
     `os.StartProcess` call-site first-argument literals, and
     asserts none equal `"agentmind"`.
   - `TestNoOtelNetCalls` (Test H — net-call half, per spec lines
     278-285): AST-walks every `*.go` file under
     `cmd/mindspec/otel.go` and `internal/otel/`, enumerates all
     calls to `net.Dial`, `net.DialTimeout`, `net.Listen`,
     `http.Get`, `http.Post`, `http.Head`, `http.PostForm`,
     `(*http.Client).Do`, `(*http.Client).Get`, `(*http.Client).Post`,
     and any call sequence of `url.Parse` followed by `net.Dial` /
     `Dial` in the same function. Asserts the count is zero. Closes
     the "doctor-by-another-name" hole: a status command that
     secretly reaches out to the configured endpoint to "verify"
     connectivity is the failure mode this gate catches.
   - The same file additionally AST-walks the two allow-listed
     files (`internal/specgate/verify_no_agentmind_dep_test.go`
     itself and `cmd/mindspec/deprecated_commands.go`) and verifies
     that the only occurrences of the substring `agentmind` in
     those files are inside string literals (not imports, not
     `exec.Command` first arguments) — closes the spec Hard
     Constraint #2 self-consistency gate.

   The test file is built with no build tags and no `t.Skip` paths;
   it runs in every `go test -short ./...` invocation.
3. Measure the post-delete `go build ./cmd/mindspec` binary size on
   `macos-14 / darwin-arm64` (the spec-pinned runner+arch — must
   match the baseline environment). Compute
   `shrinkage = 1 - (post_bytes / 10734354)`. **If `shrinkage <
   0.30`, fail the bead and investigate** (the gate is the spec's
   anti-symbolic-extraction defense). Record `before=10734354`,
   `after=<N>`, `delta=<N>`, `shrinkage=<X.YY>`,
   `runner=macos-14`, `arch=darwin-arm64` in the final commit
   message. The CI step that runs Test I is pinned to
   `runs-on: macos-14` so the measurement environment matches the
   baseline.
4. Author `.mindspec/docs/adr/ADR-0027-mindspec-otel-config-only.md`
   with Status=Accepted. Sections: Context (cite ADR-0011, ADR-0026,
   spec 084), Decision (mindspec emits OTEL config and execs
   workloads; never spawns a receiver; treats agentmind as one
   collector of many), Consequences (per-class behavior,
   permanent specgate gate, the rename to `mindspec otel setup`),
   Rollback procedure (`git revert <merge-sha>`).
5. Append a prose postscript to `.mindspec/docs/adr/ADR-0011.md`
   reflecting the no-Go-dep posture: the one-way dependency survives
   conceptually, but the literal Go module dep is removed; the
   OTLP/HTTP wire shape on port 4318 is unchanged; mindspec just
   never participates in that wire as a producer.
6. Edit `.mindspec/docs/specs/084-mindspec-otel-only/spec.md` ADR
   Touchpoints links to point at the now-finalized ADR-0027 and
   ADR-0028 files (Bead 3 drafted ADR-0028; this step does the
   spec-side cross-reference).
7. Rewrite `README.md`'s Telemetry / Observability section: "point
   mindspec at any OTLP/HTTP endpoint via `mindspec otel setup
   --endpoint …` or `OTEL_EXPORTER_OTLP_ENDPOINT=…`. Anything that
   speaks OTLP works — agentmind, Honeycomb, Tempo, Jaeger,
   opentelemetry-collector-contrib." Embed the per-command migration
   table verbatim (per spec lines 407-417).
8. Add a CHANGELOG entry naming every removed/renamed command and
   linking to the agentmind install instructions.
9. Add `.github/workflows/spec-084-test-j.yml` (Test J — mandatory
   end-to-end gate). Pulls
   `otel/opentelemetry-collector-contrib@sha256:<digest>` pinned by
   digest (no `:latest`, no floating tag — the workflow YAML
   records the exact digest used; Renovate's digest-pin mode rotates
   it, OR a quarterly manual rotation per the documented schedule
   in the workflow's leading comment). Starts the collector locally
   with `loggingexporter`, runs `mindspec otel setup --endpoint
   http://127.0.0.1:4318`, runs `mindspec record start --spec test
   -- internal/otel/testdata/emit-one-otlp-log.sh`, asserts the
   collector's stdout contains the emitted log line. Workflow
   triggers on PRs touching `cmd/mindspec/`, `internal/otel/`,
   `internal/recording/`, `go.mod`, **and `.github/workflows/spec-084-test-j.yml`
   itself** (so edits to the workflow re-trigger the workflow on
   the PR that edits it). Also runs nightly on `main`.
10. Add `.github/workflows/spec-084-test-k.yml` (Test K — post-merge
    enforcement). Triggers on `push` to `main`. In a clean checkout
    of `main` after the squash-merge lands, runs
    `git fetch --tags && git show
    pre-spec-084-bench-delete:internal/bench/runner.go | head -n 20`
    and asserts non-empty output. This is the authoritative
    post-merge Test K assertion per spec lines 697-706; Bead 3's
    pre-merge assertion is the proxy.
11. Add the fixture script `internal/otel/testdata/emit-one-otlp-log.sh`:
    a curl invocation that POSTs one OTLP/HTTP log payload to
    `$OTEL_EXPORTER_OTLP_ENDPOINT/v1/logs`. Plain shell, no Go
    binary needed.
12. Re-run the spec's Hard Constraint #2 grep with the now-complete
    exclusion set:
    `grep -rn 'agentmind\|wire\.CollectedEvent\|client\.AutoStart\|client\.ReadEvents\|client\.RunStandalone'
    cmd/ internal/ --exclude='*_specgate*.go'
    --exclude='*verify_no_agentmind*.go'
    --exclude='deprecated_commands*.go'`. Zero matches.
13. Run `GOFLAGS=-mod=readonly go build ./cmd/mindspec && go test
    -short ./...` and `golangci-lint run` (if wired). All green.

**Verification**
- [ ] `go list -m all | grep mrmaxsteel` returns only
      `github.com/mrmaxsteel/mindspec`.
- [ ] `grep -c "mrmaxsteel/agentmind" go.mod go.sum` returns `0`
      for both files.
- [ ] `go list -deps ./... | grep -c mrmaxsteel/agentmind` outputs
      `0`.
- [ ] `go mod tidy` is a no-op (idempotent).
- [ ] `internal/specgate/verify_no_agentmind_dep_test.go` exists,
      has no build tags, no `t.Skip` paths, and passes in `go test
      -short ./...`.
- [ ] Test H process-spawn half (`exec.Command` / `exec.LookPath` /
      `os.StartProcess` first-argument literals) passes.
- [ ] Test H net-call half (`net.Dial` / `http.Get` / `http.Post` /
      `http.Client.Do` / `url.Parse → Dial` scan of
      `cmd/mindspec/otel.go` and `internal/otel/`) passes — zero
      net-call sites.
- [ ] Allow-listed-file string-literal AST gate passes.
- [ ] Binary-size shrinkage ≥30% against the pinned baseline of
      `10,734,354 bytes`; measurement on `macos-14 / darwin-arm64`.
      The actual byte counts, `shrinkage` value, runner, and arch
      are recorded in the commit message.
- [ ] ADR-0027 file exists with Status=Accepted; ADR-0011 carries a
      prose postscript; spec.md cross-references both ADR-0027 and
      ADR-0028.
- [ ] README's Telemetry section is rewritten; per-command migration
      table is embedded verbatim.
- [ ] CHANGELOG entry names every removed/renamed command.
- [ ] `.github/workflows/spec-084-test-j.yml` exists, pins the
      collector image by sha256 digest, includes itself in its own
      trigger paths, and Test J passes on a fresh PR.
- [ ] `.github/workflows/spec-084-test-k.yml` exists and runs the
      post-merge tag-rescue assertion on push to `main`.
- [ ] HC #2 grep (step 12) with the now-existing exclusion files
      returns zero matches.
- [ ] `GOFLAGS=-mod=readonly go build ./cmd/mindspec && go test
      -short ./...` passes.

**Acceptance Criteria**
- [ ] Spec AC "`go list -m all | grep mrmaxsteel` lists only
      `github.com/mrmaxsteel/mindspec` itself" is satisfied.
- [ ] Spec AC "`go list -deps ./... | grep -c mrmaxsteel/agentmind`
      outputs `0`" is satisfied (Test A).
- [ ] Spec AC "`grep -rn "github.com/mrmaxsteel/agentmind" .`
      returns no matches in the mindspec tree (excluding
      allow-listed paths)" is satisfied (Test B).
- [ ] Spec AC "`internal/specgate/verify_no_agentmind_dep_test.go`
      exists, runs in `go test -short ./...`, and passes" is
      satisfied; the test's first appearance is its permanent
      enforced state (Test H, both halves: process-spawn + net-call).
- [ ] Spec AC "binary-size shrinkage ≥30%" is satisfied (Test I);
      delta recorded in the final commit message with explicit
      runner+arch; merge is blocked otherwise.
- [ ] Spec AC "ADR-0027 and ADR-0028 are committed and
      cross-referenced from this spec" is fully satisfied (ADR-0028
      drafted in Bead 3; ADR-0027 authored here; spec.md
      cross-reference added here).
- [ ] Spec AC "`go build ./cmd/mindspec` succeeds with
      `GOFLAGS=-mod=readonly` after `go mod tidy`, and `go test
      -short ./...` passes" is satisfied verbatim.
- [ ] Spec Test J (end-to-end point-at-a-real-collector check) is
      mandatory and passes; CI workflow is wired per the spec's
      cadence rule with a sha256-digest-pinned collector image.
- [ ] Spec Test K (bench-rescue procedure via
      `pre-spec-084-bench-delete` annotated tag survives
      squash-merge) is enforced post-merge via
      `.github/workflows/spec-084-test-k.yml`.
- [ ] Hard Constraint #8 (every commit builds + tests green) holds.

**Depends on**
Bead 3.

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `grep -rn "github.com/mrmaxsteel/agentmind" .` returns no matches (excluding allow-listed paths) | Bead 4 (Test B + dep-drop) |
| Hard Constraint #2 grep returns zero matches | Bead 3 step 9 (closes call-site closure); Bead 4 step 12 (permanent gate with full exclusion set) |
| `find internal/bench -type f` returns no results | Bead 3 verification |
| `find internal/agentmind -type f` returns no results | Bead 3 verification |
| `find internal/recording -name 'collector*'` returns no results | Bead 3 verification |
| `find cmd/mindspec -name 'viz*.go' -o -name 'bench*.go'` returns no results | Bead 3 verification |
| `mindspec --help` contains none of `agentmind`/`bench`/`serve`/`replay`/`viz`; `mindspec otel --help` lists exactly `setup` and `status` | Bead 1 (otel surface added) + Bead 3 (Test C golden after legacy removal) |
| `go build ./cmd/mindspec` succeeds with `GOFLAGS=-mod=readonly` after `go mod tidy`, and `go test -short ./...` passes | Bead 4 step 13 (final state); Hard Constraint #8 enforced per-bead |
| `go list -m all | grep mrmaxsteel` lists only `github.com/mrmaxsteel/mindspec` | Bead 4 step 1 |
| `go list -deps ./... | grep -c mrmaxsteel/agentmind` outputs `0` | Bead 4 (Test A; permanent specgate enforcement) |
| With `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:65535`, `mindspec record start --spec test -- echo hi` exits 0, prints `hi`, writes manifest, stderr does not mention OTEL/agentmind/telemetry | Bead 2 (Test F) |
| `mindspec otel setup --endpoint http://collector.example:4318` writes `.claude/settings.local.json` with sha256-identical re-run | Bead 1 |
| `mindspec otel setup --endpoint … --codex` writes Codex `~/.codex/config.toml` `otel.exporter` stanza, sha256-idempotent | Bead 1 (with BurntSushi/toml round-trip OR regex-replacement strategy documented in package doc) |
| `mindspec viz`, `mindspec agentmind serve`, `mindspec agentmind replay`, `mindspec agentmind setup`, `mindspec bench run` each exit code 2 with one stderr line matching the migration table verbatim | Bead 3 (Test D with per-command stderr-line equality, no template collapse) |
| Binary-size shrinkage ≥30% against the pinned `10,734,354 bytes` baseline (measured on `macos-14 / darwin-arm64`) | Bead 4 step 3 (Test I; CI step pinned to `runs-on: macos-14`) |
| `internal/specgate/verify_no_agentmind_dep_test.go` exists, runs in `go test -short`, passes | Bead 4 (Tests A and H — both process-spawn and net-call halves) |
| ADR-0027 and ADR-0028 committed and cross-referenced from this spec | Bead 3 (ADR-0028 draft) + Bead 4 (ADR-0027 + spec cross-ref) |
| Spec Test E — mindspec opens no TCP listener (Linux `/proc` + Darwin `lsof`) on `ubuntu-22.04` + `macos-14` | Bead 2 |
| Spec Test G — process-tree audit | Bead 2 |
| Spec Test J — end-to-end point-at-a-real-collector check (MANDATORY; sha256-digest-pinned collector image) | Bead 4 (`.github/workflows/spec-084-test-j.yml`) |
| Spec Test K — bench-rescue procedure via `pre-spec-084-bench-delete` annotated tag survives squash-merge | Bead 3 (tag pushed to origin before delete; pre-merge proxy assertion) + Bead 4 (`.github/workflows/spec-084-test-k.yml` post-merge enforcement) |
| Spec Migration Commits 1-2 ordering invariant ("new surfaces land first; two-commit diff-equivalence window") | Bead 1 (otel surface added) + Bead 2 (record.go rewired) — legacy `agentmind setup` remains live until Bead 3 |
| Hard Constraint #8 — every commit builds and tests green | Every bead verification block |
