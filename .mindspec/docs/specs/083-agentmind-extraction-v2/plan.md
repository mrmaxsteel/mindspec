---
adr_citations:
    - id: ADR-0011
    - id: ADR-0026
approved_at: "2026-05-20T00:03:04Z"
approved_by: user
bead_ids:
    - mindspec-6oxg.1
    - mindspec-6oxg.2
    - mindspec-6oxg.3
    - mindspec-6oxg.4
    - mindspec-6oxg.5
    - mindspec-6oxg.6
    - mindspec-6oxg.7
    - mindspec-6oxg.8
spec_id: 083-agentmind-extraction-v2
status: Approved
version: "1"
---
# Plan: 083-agentmind-extraction-v2

## ADR Fitness

- **ADR-0011** (One-way `mindspec → agentmind` dependency via OTLP/HTTP:4318): this
  plan is the physical realization of that decision. Every consumer rewire (Beads
  3a and 3b) and every deletion (Bead 5) tightens compliance with the one-way
  rule. Hard Constraints #1, #2, #3, and #7 from the spec are direct
  re-statements of ADR-0011's invariants and are enforced at multiple beads
  (import-boundary, no-circular-discovery, NDJSON contract).
- **ADR-0026** ("AgentMind extracted to standalone repo"): captures what
  shipped, the deferred decisions (UI-port discovery, version-skew,
  `setup` ownership, first-party installer), and the rollback procedure
  (`git revert <merge-sha>` + drop the `require` line). Drafted alongside
  the Phase 5 deletion (Bead 5) so the ADR text mirrors the as-shipped
  state; finalized in Bead 7 once Beads 5 and 6 have merged. The ADR is
  cited from plan-approval time; its `Status` flips from a placeholder
  acceptance to fully-locked content only after the deletion lands.

No accepted ADR is superseded or contradicted by this plan.

## Testing Strategy

This spec's failure mode is symbolic extraction — code that compiles and unit-tests
green but leaves the real work in mindspec. Static checks were proven blind to this
pattern in the v1 panel. The strategy layers static + runtime + CI gates:

1. **Unit tests** (per bead, run on every commit): the typed-sentinel detection
   in consumers (`errors.Is(err, client.ErrBinaryNotFound)`), the `sync.Once`
   warn-line emission, the per-class degradation policy at each call site, and
   the cobra re-exec wrappers in `cmd/mindspec/viz.go`. Locations:
   `internal/recording/`, `internal/bench/`, `cmd/mindspec/`.
2. **Byte-for-byte NDJSON parity test** (Bead 2, Phase-2 variant of spec
   Test D): a frozen-clock test harness emits a saved fixture under the
   canonical `wire/event.go` encoding (lex-sorted keys, fixed float precision,
   UTC RFC3339Nano timestamps); the alias-state mindspec build must produce a
   byte-identical NDJSON stream. Comparison via `diff`.
3. **Per-class degradation integration tests** (Bead 3a, spec Test C): for each
   of the three command classes, exercise the absent-binary path and assert
   the contract: telemetry-as-output and interactive exit non-zero with a clear
   error; batch exits 0 with exactly one centralized warn line.
4. **Read-side consumer rewire to NDJSON-over-stdout** (Bead 3b): replaces
   in-process OTLP parsing in `internal/bench/runner.go`,
   `internal/bench/session.go`, and `internal/recording/collector.go` with
   `client.ReadEvents` consuming the agentmind subprocess's stdout pipe. This
   is the load-bearing read-side wire that makes Bead 5's deletion of the
   OTLP parser per-commit-green by construction.
5. **Live-capture CI gate + interactive cobra wrappers** (Bead 4, spec
   Test D Phase-4+ variant + spec Test C interactive class): once the
   consumer swap is in, mindspec CI runs an end-to-end capture — start the
   agentmind binary, POST a synthetic OTLP payload, normalize the resulting
   NDJSON, and diff against a reference fixture using the normalization tool
   published in `agentmind/wire` (spec lines 251-255). The same bead adds
   the cobra re-exec wrappers and `--help` golden tests.
6. **Static boundary checks in CI from Phase 4 onward** (Bead 5 makes them
   green; the checks themselves wire in earlier): spec Tests E (no-circular-
   discovery grep against the agentmind tree) and F (`go list -deps
   ./cmd/mindspec | grep mrmaxsteel/agentmind` only matches `client` and
   `wire`). These are cheap and run on every commit once introduced.
7. **Per-commit baseline** (Hard Constraint #6 — no atomic cutover):
   `go build ./cmd/mindspec && go test -short ./...` passes on every commit
   produced by every bead. CI aborts the migration on failure.

Mapping spec Tests A–G to beads:

- **Test A** (standalone-binary exists): upstream in the agentmind repo, but
  Bead 1 verifies it from the mindspec side as a precondition gate. v0.0.1
  is scaffold-only per spec Phase 0; live-capture verification (Test D
  Phase-4+) is gated on a later tag, not v0.0.1.
- **Test B** (agentmind has no mindspec dep): upstream; Bead 1 records the
  agentmind v0.0.1 SHA after confirming it.
- **Test C** (per-class degradation): Bead 3a (telemetry-as-output + batch,
  including `mindspec agentmind setup`) + Bead 4 (interactive cobra
  wrappers).
- **Test D** (live capture): Phase-2 byte-for-byte variant in Bead 2;
  Phase-4+ normalization-and-diff variant wired into CI by Bead 4.
- **Test E** (no-circular-discovery): green after Bead 5's deletions;
  the grep is run in CI from Bead 4 onward. Post-deletion continuity is
  preserved by re-running the grep against a CI-only shallow clone of the
  agentmind sibling repo at the pinned tag (see Bead 6).
- **Test F** (import-boundary): green after Bead 5's deletions; checked
  in CI from Bead 4 onward.
- **Test G** (Phase 0 prerequisite gate, agentmind v0.0.1 tag exists):
  Bead 1.

## Bead 1: Phase 0 prerequisite gate — verify agentmind v0.0.1 and record SHA

Gates every later bead. No mindspec code changes; this is a verification +
spec amendment. **Scope note:** v0.0.1 is a scaffold-only tag per spec Phase
0 (lines 312-313); this bead does NOT gate live-capture on a runnable binary
from v0.0.1. Live-capture verification (Test D Phase-4+) is gated on the
later v0.3.0 tag in Bead 4.

**Steps**
1. Run `git ls-remote --tags https://github.com/mrmaxsteel/agentmind | grep 'refs/tags/v0.0.1$'`
   (spec Test G). Capture the SHA.
2. Clone the agentmind repo at the v0.0.1 tag to a scratch directory and run
   `go list -m -json all | jq -r '.Path' | grep '^github.com/mrmaxsteel/mindspec'`
   to confirm no mindspec dep (spec Test B). Expect zero matches.
3. Probe the v0.0.1 tag for binary scaffolding. If `cmd/agentmind/main.go`
   exists at the tag, build it and run `./bin/agentmind --version | grep
   '^agentmind'` (spec Test A). If `cmd/agentmind/` is absent at v0.0.1
   (scaffold-only Phase 0), record this finding in the commit message and
   defer spec Test A to the first tag that publishes a binary (v0.2.0 or
   v0.3.0 per spec Phase 3/4); the bead remains green on the scaffold-only
   outcome.
4. Edit `spec.md`: replace the `agentmind v0.0.1 SHA: <TBD — record before Phase 1>`
   placeholder with the actual SHA captured in step 1. Single commit.

**Verification**
- [x] `git ls-remote` output for the v0.0.1 tag is captured in the commit
      message (or, when upstream returns empty output, the empty-result
      evidence is recorded). At Bead 1 implementation time the upstream
      repo `github.com/mrmaxsteel/agentmind` returns an empty tag list
      with exit 0 — recorded in the Bead 1 fixup commit body.
- [ ] Test B and Test G pass against the v0.0.1 tag. **Deferred** until
      upstream publishes `v0.0.1`; the deferral is closed automatically
      when `scripts/verify-agentmind-tag.sh v0.0.1` exits 0 in CI.
- [ ] Test A either passes (if v0.0.1 produces a binary) or is documented
      as deferred to the binary-publishing tag (if v0.0.1 is scaffold-only).
      **Deferred** to the first agentmind tag that publishes
      `cmd/agentmind/main.go` (target: `v0.2.0` or `v0.3.0` per spec
      Phase 3/4); deferral is auditable in the Bead 1 fixup commit body.
- [ ] `spec.md` no longer contains the `<TBD>` placeholder. **Deferred**:
      the placeholder is closed by re-running
      `scripts/verify-agentmind-tag.sh v0.0.1 --record` once upstream
      publishes the tag. The record-on-republish mechanism owns AC closure.

**Acceptance Criteria**

The original four ACs are restated below, each annotated with the
deferral status that the panel review accepted. The bead is considered
"done as far as Phase 0 mindspec-side can be" — the script, Makefile
target, `internal/specgate` smoke tests, and spec amendment are
shipped; the remaining bullets are blocked on the upstream `agentmind`
repository scaffolding and tagging `v0.0.1`, which is outside this
codebase. Closure of the deferred ACs is mechanically gated by the
`--record` workflow and the strict-mode CI switch
`MINDSPEC_REQUIRE_GATE_PASS=1`.

- [ ] Spec Test G (agentmind v0.0.1 tag exists) passes and the SHA is
      recorded in `spec.md`. **Deferred** until upstream publishes
      `v0.0.1`. Closure mechanism: rerun
      `scripts/verify-agentmind-tag.sh v0.0.1 --record`; the script
      replaces the `<TBD>` placeholder with the captured SHA, and
      `TestVerifyAgentmindTagAgainstUpstream` (with
      `MINDSPEC_REQUIRE_GATE_PASS=1`) then enforces SHA-equality
      between the gate's report and the recorded value on every CI run.
- [ ] Spec Test B (no mindspec dep in agentmind) passes against the v0.0.1
      tag. **Deferred** until upstream publishes `v0.0.1`. Closure
      mechanism: a follow-up bead (or re-opened Bead 1 fixup) clones the
      tag, runs `go list -m -json all | jq … | grep mindspec`, and
      records the empty result in the spec's AC matrix.
- [ ] Spec Test A (standalone-binary exists) either passes against v0.0.1
      or is explicitly deferred (with the deferral target tag recorded) per
      the scaffold-only Phase 0 contract. **Deferred** to the first
      agentmind tag that publishes `cmd/agentmind/main.go`; target tag
      is `v0.2.0` or `v0.3.0` per spec Phase 3/4. The scaffold-only
      Phase 0 outcome is accepted here as the bead's resolution of
      this AC.
- [ ] Spec AC "agentmind v0.0.1 SHA recorded before Phase 1" is satisfied;
      no `<TBD>` placeholder remains. **Deferred**: identical mechanism
      as the first bullet — the `--record` invocation that closes
      Test G also removes the `<TBD>` placeholder. Phase 1 work cannot
      merge until this AC closes.

**Depends on**
None (but blocks Beads 2–7).

## Bead 2: Phase 2 — types alias re-export + `replace` directive + sibling-checkout helper + Phase-2 NDJSON parity

The first mindspec edit. Switches `bench.CollectedEvent` / `otlpValue` /
`otlpKeyValue` to type aliases of `wire.CollectedEvent` etc., adds the local
`replace` directive used through Phases 2–5, and proves byte-for-byte
NDJSON parity against the frozen-clock fixture **produced via the canonical
`wire/event.go` encoder**, not via pre-edit ad-hoc dumps.

**Steps**
1. **Precondition: agentmind v0.1.0 tag exists.** Run
   `git ls-remote --tags https://github.com/mrmaxsteel/agentmind | grep 'refs/tags/v0.1.0$'`
   (Test-G-style gate). Capture the SHA. Then confirm the Phase 1
   method-set precondition: run
   `grep -rn 'func (.*\\*\\?\\(CollectedEvent\\|otlpValue\\|otlpKeyValue\\))' internal/bench/`
   and record the finding. **Two branches:**
   - **Branch A (zero methods found, OR methods already moved to
     `agentmind/wire` in Phase 1):** the alias strategy proceeds.
   - **Branch B (methods found and not movable to `agentmind/wire`):** fall
     back to the spec-mandated full type duplication path (spec lines
     327-333). This bead then replaces the local types with full duplicates
     in mindspec (matching `wire.CollectedEvent` field-for-field) and
     schedules deletion of the bench-side copies for Bead 5. The
     alias-vs-duplicate decision MUST be recorded in the bead's commit
     message.
2. Edit `go.mod`: add `require github.com/mrmaxsteel/agentmind v0.1.0`
   (the Phase 1 tag, confirmed in step 1) and `replace
   github.com/mrmaxsteel/agentmind => ../agentmind`.
3. Edit `internal/bench/collector.go`: under Branch A, replace the local type
   declarations for `CollectedEvent`, `otlpValue`, `otlpKeyValue` with
   `type X = wire.X` aliases imported from
   `github.com/mrmaxsteel/agentmind/wire`. Under Branch B, leave the local
   types as full duplicates and document the deferred deletion. The OTLP
   parsing code stays in place for now (deleted in Bead 5).
4. Add `scripts/checkout-agentmind.sh`: clones agentmind at the tag pinned in
   `go.mod` to a sibling directory (`../agentmind`) so CI can resolve the
   `replace` directive. Hook into the CI workflow at the existing
   pre-test step.
5. **Generate the parity fixture via the canonical encoder, in a separate
   prior commit.** Before the alias swap (or, under Branch B, before the
   duplicate substitution), use the canonical `wire/event.go` encoder (from
   the sibling agentmind checkout at v0.1.0) to emit a representative event
   set under a frozen clock. The encoder MUST enforce lex-sorted JSON keys,
   `strconv.FormatFloat(f, 'f', -1, 64)` for floats, and UTC `RFC3339Nano`
   timestamps (spec lines 247-250). Commit the resulting fixture at
   `internal/bench/testdata/parity.ndjson` in a **separate prior commit**
   with provenance recorded in the commit message (encoder version, fixture
   generator command). Ad-hoc dumps from the pre-edit OTLP-parsing code are
   NOT acceptable unless covered by step 6's canonical-equivalence
   assertion.
6. (Optional, only if pre-edit dumps are reused) **Canonical-equivalence
   assertion.** If reusing a pre-edit fixture, add an assertion in this bead
   that proves byte-for-byte equality between the pre-edit dump and a fresh
   canonical-encoder emission for the same input set. Failure of this
   assertion forces regeneration via step 5.
7. Add the Phase-2 NDJSON parity test as
   `internal/bench/collector_parity_test.go`: uses a frozen clock, encodes a
   representative event set, diffs the output against the fixture committed
   in step 5.

**Verification**
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes on this commit
      (Hard Constraint #6).
- [ ] `go test ./internal/bench/... -run TestCollectorParity` passes — covers
      the Phase-2 variant of spec Test D.
- [ ] `scripts/checkout-agentmind.sh` runs clean in a fresh checkout.
- [ ] Hard Constraint #8 (wire-protocol contract): the only agentmind import
      from mindspec is `github.com/mrmaxsteel/agentmind/wire`.
- [ ] Commit history shows the fixture provenance commit (step 5) preceding
      the alias-swap commit (step 3).
- [ ] Branch A vs Branch B decision is recorded in the commit message.

**Acceptance Criteria**
- [ ] Spec AC "NDJSON byte-for-byte parity (Phase 2, alias state) — fixture
      produced via canonical `wire/event.go` encoder" passes (Test D
      Phase-2 variant).
- [ ] Spec AC "go build + go test -short pass on every commit" holds for
      both the fixture-provenance commit and the alias-swap commit (Hard
      Constraint #6).
- [ ] Spec Hard Constraint #8 (wire-protocol contract): mindspec's only
      agentmind import is `github.com/mrmaxsteel/agentmind/wire` after this
      bead.
- [ ] Test-G-style precondition gate for agentmind v0.1.0 tag (spec
      Provenance row) is satisfied.

**Depends on**
Bead 1.

## Bead 3a: Phase 4a — consumer swap to `client.AutoStart` + typed sentinel + per-class degradation + centralized warn line

Switches every `internal/agentmind.AutoStart` consumer to
`github.com/mrmaxsteel/agentmind/client.AutoStart` and implements the
three-class degradation contract (Hard Constraint #4). The typed-sentinel
detection uses `errors.Is(err, client.ErrBinaryNotFound)` so future
regressions to substring-matching on error messages are caught at
compile/test time rather than relying on a CI grep.

**Steps**
1. **Precondition: agentmind v0.3.0 tag exists.** Run
   `git ls-remote --tags https://github.com/mrmaxsteel/agentmind | grep 'refs/tags/v0.3.0$'`
   (Test-G-style gate). Capture the SHA. Then bump `go.mod` require to
   `agentmind v0.3.0` (the Phase 4 tag exposing `client.AutoStart` and
   `client.ErrBinaryNotFound`).
2. Edit `internal/recording/collector.go`: replace the
   `internal/agentmind.AutoStart` call with `client.AutoStart`. On error,
   detect the absent-binary condition via
   `errors.Is(err, client.ErrBinaryNotFound)` — if true, propagate as a clear
   error (telemetry-as-output class: `mindspec record start` MUST exit
   non-zero per spec AC). No warn-line emitted at this call site; the
   centralized `sync.Once` warn inside `agentmind/client` fires before the
   error returns.
3. Edit `internal/bench/runner.go`: same swap. On
   `errors.Is(err, client.ErrBinaryNotFound)`, swallow and return nil after
   the centralized warn fires (batch class: `mindspec bench run` MAY exit 0).
4. Edit `cmd/mindspec/setup.go` (or wherever `mindspec agentmind setup` is
   wired): same swap. On `errors.Is(err, client.ErrBinaryNotFound)`, swallow
   and return nil after the centralized warn fires (batch class: `mindspec
   agentmind setup` MAY exit 0 per spec AC lines 240-243 and Test C at spec
   line 282). **If `setup` does not currently call `AutoStart` at all**,
   document this explicitly in the commit message and mark the spec AC for
   `setup` as satisfied-by-design (no binary needed; no warn line required).
   Either way, the bead MUST name the file rewired (or document the
   no-rewire conclusion) and produce a setup-specific integration test in
   step 6.
5. Add a unit test asserting that the `sync.Once` warn-line emission from
   `agentmind/client` produces exactly one line across multiple `AutoStart`
   calls in the same process (invoke `AutoStart` three times with the
   binary absent; assert stderr contains exactly one occurrence of
   `WARN: agentmind binary not found; telemetry export will drop
   silently`).
6. Add per-class integration tests covering spec Test C:
   - **Telemetry-as-output:** `mindspec record start --spec test` with binary
     absent exits non-zero with a clear error.
   - **Batch — bench:** `mindspec bench run <fixture>` with binary absent
     exits 0 with exactly one centralized warn line.
   - **Batch — setup:** `mindspec agentmind setup` with binary absent exits
     0 with exactly one centralized warn line (or, if `setup` does not call
     `AutoStart`, exits 0 with no warn line and the test asserts the
     satisfied-by-design contract).
   - **Interactive cases** live in Bead 4 since they're cobra-level wrappers.
7. **Typed-sentinel prohibition assertion.** Add a unit-test assertion that
   walks the swapped files (`internal/recording/collector.go`,
   `internal/bench/runner.go`, `cmd/mindspec/setup.go`) and fails if any of
   them invokes `strings.Contains(err.Error(), …)` or similar substring
   matching on binary-not-found detection. The mechanism is
   `errors.Is(err, client.ErrBinaryNotFound)` — assert the typed-error path
   is present (positive assertion) and substring-matching is absent
   (negative assertion).

**Verification**
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.
- [ ] `go test ./internal/recording/... ./internal/bench/... ./cmd/mindspec/...`
      covers the new per-class behavior.
- [ ] Spec Test C passes for the telemetry-as-output and batch classes
      (the interactive class is covered in Bead 4). Both batch sub-classes
      (`bench run` and `agentmind setup`) are exercised.
- [ ] No `strings.Contains(err.Error(), …)` form of binary-not-found
      detection survives in `internal/recording/collector.go`,
      `internal/bench/runner.go`, or `cmd/mindspec/setup.go`.
- [ ] Typed-sentinel positive assertion passes
      (`errors.Is(err, client.ErrBinaryNotFound)` is the detection
      mechanism at every swapped call site).

**Acceptance Criteria**
- [ ] Spec AC "`mindspec record start` exits non-zero with binary absent
      (telemetry-as-output)" passes (Test C telemetry-as-output class).
- [ ] Spec AC "`mindspec bench run` exits 0 with exactly one centralized
      warn line (batch)" passes (Test C batch class + `sync.Once` unit
      test).
- [ ] Spec AC "`mindspec agentmind setup` exits 0 with exactly one
      centralized warn line — OR documented as satisfied-by-design if
      `setup` does not call `AutoStart`" is satisfied (Test C batch
      class).
- [ ] Spec Hard Constraint #4 (typed sentinel; no substring-matching on
      error messages) is enforced by the prohibition assertion at every
      swapped call site.
- [ ] Test-G-style precondition gate for agentmind v0.3.0 tag is
      satisfied.

**Depends on**
Bead 2.

## Bead 3b: Phase 4a (read-side rewire) — consume NDJSON via `client.ReadEvents` on subprocess stdout pipe

The load-bearing read-side wire. Rewires `internal/bench/runner.go`,
`internal/bench/session.go`, and `internal/recording/collector.go` from
in-process OTLP parsing to consumption via
`client.ReadEvents(io.Reader) <-chan wire.CollectedEvent` on the agentmind
subprocess's stdout pipe (Hard Constraint #3: outbound channel is
stdout-pipe NDJSON, NOT file-tail). This is the precondition that makes
Bead 5's deletion of the OTLP-parsing code per-commit-green by construction.

**Steps**
1. Edit `internal/recording/collector.go`: after `client.AutoStart` returns
   the running subprocess handle, obtain the subprocess's stdout pipe (via
   `exec.Cmd.StdoutPipe()` or the equivalent helper exported from
   `agentmind/client` at v0.3.0). Pass the resulting `io.Reader` to
   `client.ReadEvents(reader)` and consume the `<-chan wire.CollectedEvent`
   stream. Remove the in-process OTLP parser invocation at this call site.
2. Edit `internal/bench/runner.go`: same rewire. The `io.Reader` argument
   to `client.ReadEvents` MUST be the subprocess stdout pipe, **NOT** a
   file handle obtained from `os.Open(outputPath)` against the agentmind
   `--output` file path. File-tailing is a Hard-Constraint-#3 violation
   and is explicitly prohibited here.
3. Edit `internal/bench/session.go`: same rewire. Replace the in-process
   OTLP parsing path with `client.ReadEvents` on the subprocess stdout pipe.
4. **File-tailing-escape-hatch verification.** Add a unit-test or grep-style
   assertion confirming that none of the three rewired files contain
   `os.Open(.*outputPath)`, `os.Open(.*\\.ndjson)`, or
   `tail.*--output` constructs that would tail the agentmind `--output`
   file. The only acceptable `io.Reader` source feeding `client.ReadEvents`
   is the subprocess stdout pipe from `exec.Cmd.StdoutPipe()` (or the
   `agentmind/client` helper that wraps it).
5. **OTLP-parser-residue verification.** Add a grep-style assertion that no
   code in `internal/bench/**` or `internal/recording/**` parses OTLP after
   this bead completes. Concrete check:
   `grep -rEn 'http\\.HandleFunc.*"/v1/(logs|metrics|traces)"|parseOTLP|otlpKeyValue' internal/bench/ internal/recording/`
   returns no match except the file `internal/bench/collector.go` whose
   OTLP-parsing code is the one deleted by Bead 5 step 3 (and which is
   no longer in any consumer's read path).
6. Add or update integration tests asserting that `runner.go`, `session.go`,
   and `recording/collector.go` consume events via the
   `client.ReadEvents(subprocess.StdoutPipe())` path. The tests MUST
   exercise event flow from a synthetic agentmind subprocess (or a test
   double that writes NDJSON to its stdout) to the mindspec consumer, and
   MUST fail if any consumer is wired to file-tail the `--output` path.

**Verification**
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.
- [ ] All three files (`internal/bench/runner.go`,
      `internal/bench/session.go`, `internal/recording/collector.go`)
      consume `client.ReadEvents` on subprocess stdout. The
      file-tailing-escape-hatch assertion (step 4) passes.
- [ ] OTLP-parser-residue assertion (step 5) passes: no
      `internal/bench/**` or `internal/recording/**` code parses OTLP
      outside the soon-to-be-deleted code in
      `internal/bench/collector.go`.
- [ ] Hard Constraint #3 honored: outbound channel from agentmind to
      mindspec is the stdout-pipe NDJSON, not file-tail.

**Acceptance Criteria**
- [ ] Spec AC "Read-side consumer rewire: `runner.go`, `session.go`,
      `recording/collector.go` consume `client.ReadEvents` on subprocess
      stdout (Hard Constraint #3)" passes via the file-tailing-escape-hatch
      + OTLP-parser-residue assertions.
- [ ] Spec Hard Constraint #3 (outbound channel is stdout-pipe NDJSON, not
      file-tail) is enforced at all three rewired files.
- [ ] Spec AC "go build + go test -short pass on every commit" (Hard
      Constraint #6) holds for the rewire commit.
- [ ] Bead 5's deletion of the OTLP-parsing code becomes
      green-by-construction (no read-path consumer survives that needs the
      parser).

**Depends on**
Bead 3a.

## Bead 4: Phase 4b + 4c — cobra re-exec wrappers, `--help` golden, and live-capture CI gate

Merges the original Phase 4b (cobra re-exec wrappers for `mindspec viz`,
`agentmind serve`, `agentmind replay`) and Phase 4c (Test D live-capture
CI gate + Tests E/F static checks) into a single bead. Both halves
strictly depend on Beads 3a and 3b and ship together at the Phase 4
boundary, so they're combined to keep the bead count down without
weakening verification — the cobra wrappers close Test C interactive
class, and the CI gate activates Tests D Phase-4+, E, and F.

**Steps**
1. **Cobra re-exec.** Edit `cmd/mindspec/viz.go`: replace the
   `agentmindServeCmd` and `agentmindReplayCmd` `RunE` bodies with a
   single call to `client.RunStandalone(os.Args[1:])` (or the equivalent
   helper exported from `agentmind/client` at v0.3.0). On
   `errors.Is(err, client.ErrBinaryNotFound)`, exit non-zero per the
   interactive class. Also wire the top-level `vizCmd` (spec Hard
   Constraint #5: `mindspec viz` is the alias for `mindspec agentmind`):
   either declare `vizCmd` as a cobra alias of `agentmindServeCmd`, OR
   rewrite `vizCmd`'s `RunE` to call `client.AutoStart` +
   `client.RunStandalone(os.Args[1:])` with the same `ErrBinaryNotFound`
   handling. Record the choice in the commit message.
2. **`--help` golden + interactive Test C tests.** Capture
   `mindspec agentmind serve --help`, `mindspec agentmind replay --help`,
   and `mindspec viz --help` output from a commit SHA on `main` that is
   **strictly before** Bead 2's alias swap (so the golden cannot
   accidentally be captured from the alias-applied build). Record the
   source commit SHA in the commit message. Store the golden output at
   `cmd/mindspec/testdata/help_golden/{serve,replay,viz}.txt`. Add a unit
   test asserting byte equality between current `--help` output and the
   golden (covers spec AC: same usage text). Add an integration test
   exercising Test C interactive: `mindspec viz`, `mindspec agentmind
   serve`, `mindspec agentmind replay` each exit non-zero with the
   binary absent.
3. **Live-capture precondition + CI job.** Run a Test-G-style
   precondition gate confirming that the agentmind tag in `go.mod`
   (v0.3.0 or later) publishes the normalization tool/library under
   `github.com/mrmaxsteel/agentmind/wire` (spec lines 251-255); capture
   the agentmind SHA in the commit message. Add a CI job that runs
   `scripts/checkout-agentmind.sh`, builds the agentmind binary from the
   sibling checkout (`go build -o ../agentmind/bin/agentmind
   ./cmd/agentmind`), runs `./agentmind/bin/agentmind serve --otlp-port
   4318 --ui-port 0 --output /tmp/em.ndjson &`, POSTs a synthetic OTLP
   log payload via `curl`, runs the `agentmind/wire` normalization tool
   against `/tmp/em.ndjson` (sort events by event time, redact
   PID/host/wall-clock to canonical placeholder per spec lines 251-255),
   and diffs the normalized output against a reference fixture committed
   at `internal/bench/testdata/live_capture_reference.ndjson` (itself
   produced via the canonical encoder, analogous to Bead 2's parity
   fixture). The diff MUST be clean.
4. **Static Tests E and F CI steps.** Add CI steps for the cheap greps:
   - **Test E:** `grep -rEn 'exec\\.Command.*"mindspec"|LookPath.*"mindspec"|StartProcess.*"mindspec"' ../agentmind/client/ ../agentmind/cmd/ ../agentmind/internal/`
     returns no match. The `-E` flag is mandatory; without it the `|`
     alternation is interpreted literally and the gate silently
     false-negatives. Alternative: three separate invocations (one per
     pattern). Either form is acceptable; the `\\|` literal-alternation
     form is NOT.
   - **Test F:** `go list -deps ./cmd/mindspec | grep
     mrmaxsteel/agentmind | sort -u | grep -vE
     '^github.com/mrmaxsteel/agentmind/(client|wire)$'` returns no
     match.
5. **Defensive gating.** Gate the live-capture CI job on Bead 1's Test G
   result so the job is skipped (not failed) if the agentmind v0.0.1 tag
   isn't reachable — defensive against upstream unavailability. Manual
   smoke: `./bin/mindspec agentmind serve --help` prints the same text
   as the pre-extraction reference output.

**Verification**
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.
- [ ] `--help` golden test passes (spec AC: same usage text) for `serve`,
      `replay`, and `viz`. The golden's source commit SHA is recorded in
      the commit message and is verifiably pre-Bead-2.
- [ ] Spec Test C interactive-class assertions pass for `viz`,
      `agentmind serve`, and `agentmind replay`.
- [ ] CI green on a fresh PR — live-capture job runs end-to-end and the
      normalization-and-diff comparison against the reference fixture is
      clean.
- [ ] Tests E and F report zero matches (with `-E` flag in use for Test E).
- [ ] Phase-4+ variant of spec Test D is now enforced per-commit via the
      normalization-and-diff path, not via substring matching.
- [ ] Normalization-tool precondition gate (step 3) passes.

**Acceptance Criteria**
- [ ] Spec AC "`mindspec viz` / `agentmind serve` / `agentmind replay`
      exit non-zero with binary absent (interactive)" passes (Test C
      interactive class).
- [ ] Spec AC "`mindspec agentmind serve --help` / `replay --help` /
      `viz --help` print unchanged usage text" passes via the `--help`
      golden test (golden captured at a pre-Bead-2 commit SHA).
- [ ] Spec AC "NDJSON semantic equivalence (Phase 4+, subprocess state)
      via spec-mandated normalization tool" passes (Test D Phase-4+
      variant).
- [ ] Spec Tests E (no-circular-discovery, pre-deletion) and F
      (import-boundary) report zero matches in CI.
- [ ] Spec AC "go build + go test -short pass on every commit" (Hard
      Constraint #6) holds for the cobra-rewire commit and the
      CI-wiring commit.

**Depends on**
Beads 3a, 3b.

## Bead 5: Phase 5 — deletion of `internal/agentmind/`, `internal/viz/`, and OTLP-parsing code; binary-size delta; draft ADR-0026

The deletion bead. After this, Tests E and F go from "checked in CI" to
"trivially green by absence" inside mindspec. mindspec binary size shrinks
visibly. Test E continuity post-deletion is preserved via the CI shallow
clone described in Bead 6.

**Steps**
1. Record the pre-deletion `mindspec` binary size:
   `go build -o /tmp/mindspec-before ./cmd/mindspec && stat -f %z /tmp/mindspec-before`.
   Capture for the commit message.
2. `rm -rf internal/agentmind/`. The package has no remaining importers after
   Bead 3a, so deletion is mechanical.
3. Delete the OTLP-parsing code from `internal/bench/collector.go`. The type
   aliases stay until no caller uses them; once `runner.go`, `session.go`,
   and `recording/collector.go` read NDJSON directly via
   `client.ReadEvents` on the agentmind subprocess stdout (guaranteed by
   Bead 3b's OTLP-parser-residue assertion), the alias file itself can be
   reduced to a `// Deprecated:` shim pointing to `wire`, or deleted
   entirely if no caller remains. Under Bead 2's Branch B (full type
   duplication), the duplicates are deleted in this step as well.
4. `rm -rf internal/viz/`. Only `cmd/mindspec/viz.go`'s cobra shell-out
   survives. Update import sites: any remaining `mindspec/internal/viz` or
   `mindspec/internal/agentmind` references in test files or fixtures get
   removed or repointed to `agentmind/wire`.
5. Record the post-deletion binary size:
   `go build -o /tmp/mindspec-after ./cmd/mindspec && stat -f %z /tmp/mindspec-after`.
   Include both sizes and the delta in the commit message (spec AC:
   "mindspec binary size shrinks; the before/after sizes are recorded").
6. **Draft ADR-0026 alongside this deletion** so the ADR text mirrors the
   as-shipped state. ADR-0026 already exists at plan-approval time (cited
   in this plan's frontmatter); this step locks its content to the
   as-shipped surface (Context citing ADR-0011, Decision describing the
   import surface + IPC channels + per-class degradation contract,
   Deferred decisions: UI-port discovery / version-skew / `setup`
   ownership / first-party installer, Rollback procedure: `git revert
   <merge-sha>` + drop the `require` line). Bead 7 performs the final
   cross-reference pass once Bead 6 has merged.

**Verification**
- [ ] `find internal/agentmind` returns no results (spec AC).
- [ ] `grep -r 'http.HandleFunc.*"/v1/logs"' .` returns no results in
      mindspec (spec AC).
- [ ] `find internal/viz` returns no results.
- [ ] Spec Tests E and F pass by absence within mindspec (Bead 6 handles
      Test E continuity against the agentmind tree).
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.
- [ ] Commit message records `before=<bytes>`, `after=<bytes>`,
      `delta=<bytes>`.

**Acceptance Criteria**
- [ ] Spec AC "`find internal/agentmind` returns no results" is satisfied.
- [ ] Spec AC "`grep -r 'http.HandleFunc.*\"/v1/logs\"' .` returns no
      results" is satisfied (OTLP-parsing code deleted from
      `internal/bench/collector.go`).
- [ ] Spec AC "mindspec binary size shrinks; before/after recorded" is
      satisfied via the commit-message delta.
- [ ] Spec Tests E (no-circular-discovery within mindspec) and F
      (import-boundary) pass by absence post-deletion.
- [ ] ADR-0026 content is locked to the as-shipped state in the same
      commit (spec AC "ADR-0026 committed and referenced from spec's ADR
      Touchpoints" — finalization step lives in Bead 7).

**Depends on**
Beads 3a, 3b, 4.

## Bead 6: Phase 6 — drop `replace`, pin `agentmind v1.0.0`, document manual install, preserve Test E continuity

The release bead. After agentmind cuts its v1.0.0 tag and the soak gate
below is satisfied, mindspec drops the local `replace` directive and pins
the released tag. Adds the documented manual install path with checksum
verification (no install subcommand — deferred per spec Non-Goals). Also
preserves Test E continuity post-Bead-5 by re-running the grep against a
CI-only shallow clone of the agentmind repo at the pinned tag.

**Steps**
1. Bump `go.mod`: `require github.com/mrmaxsteel/agentmind v1.0.0`. Delete
   the `replace github.com/mrmaxsteel/agentmind => ../agentmind` line.
   Run `go mod tidy`.
2. Delete `scripts/checkout-agentmind.sh` (no longer needed once the tag is
   pinned) or leave it as a contributor-mode helper with a comment that it
   is only for local development against an in-progress agentmind branch —
   judgment call captured in the commit message.
3. Update the mindspec CI live-capture job (Bead 4) to download the released
   `agentmind` binary from the GitHub release artifacts instead of building
   from the sibling checkout. Verify the SHA256SUMS checksum during download.
4. **Preserve Test E continuity** (spec Test E, no-circular-discovery).
   Once Bead 5 deletes `internal/agentmind/`, the in-mindspec grep that
   Test E performs against the agentmind tree no longer has a target inside
   mindspec. Add a CI step that performs a shallow `git clone --depth 1`
   of the agentmind sibling repo at the pinned v1.0.0 tag and re-runs the
   Test E grep against that checkout:
   `grep -rEn 'exec\\.Command.*"mindspec"|LookPath.*"mindspec"|StartProcess.*"mindspec"' <agentmind-clone>/client/ <agentmind-clone>/cmd/ <agentmind-clone>/internal/`
   returns no match. The CI step is documented as the cross-repo gate; the
   agentmind side's own CI is the primary owner of Test E enforcement and
   this step is the mindspec-side mirror.
5. **Soak gate (replaces spec line 373's 7-day soak).** Convert the
   spec-mentioned 7-day soak into a verifiable, date-stamped CI check:
   maintain a nightly-CI artifact `agentmind-phase3-soak-history.txt`
   listing each date the Phase 3 integration test ran green; before
   merging this bead, assert seven consecutive green entries ending at a
   date no earlier than `today - 1 day`. The CI gate fails if the
   green-streak is broken or the latest entry is stale. Link to the
   nightly run history in the commit message. **Deferral path:** if the
   nightly soak artifact is not yet wired (depends on agentmind-side
   CI), record the deferral explicitly in the commit message and gate
   merging on a documented alternative (e.g. one-shot manual integration
   test re-run on mindspec's PR branch before merge). Either form MUST
   be verifiable; "7 days" alone is not.
6. Update `README.md` with an "Installing agentmind" section. Provide the
   exact `curl` + `sha256sum` (or `shasum -a 256`) invocation per platform
   (darwin-arm64, darwin-amd64, linux-amd64, windows-amd64). Instruct users
   to place the verified binary at `<mindspec-root>/bin/agentmind` or any
   PATH directory.
7. Add a Phase 6 release-notes entry (`CHANGELOG.md` or
   `.mindspec/docs/releases/` per repo convention) summarizing the
   extraction and pointing at the README install section.

**Verification**
- [ ] `go.mod` shows `require github.com/mrmaxsteel/agentmind v1.0.0` and no
      `replace` directive (spec AC).
- [ ] `go build ./cmd/mindspec && go test -short ./...` passes.
- [ ] CI live-capture job pulls the released binary, verifies its checksum,
      and runs Test D successfully (normalization-and-diff per Bead 4).
- [ ] Test E continuity verified: the CI shallow-clone step against
      the pinned agentmind v1.0.0 tag returns zero matches.
- [ ] Soak gate satisfied: seven consecutive green nightly-CI entries
      ending at a recent date, OR the documented deferral path is
      satisfied and recorded in the commit message.
- [ ] README has the install section with the exact curl + checksum
      invocation per platform.

**Acceptance Criteria**
- [ ] Spec AC "Release branch has `require agentmind v1.0.0` and no
      `replace` directive" is satisfied.
- [ ] Spec AC "Test E (no-circular-discovery) — post-deletion continuity"
      is satisfied via the CI shallow-clone step against pinned v1.0.0.
- [ ] Spec AC "go build + go test -short pass on every commit" (Hard
      Constraint #6) holds for the release commit.
- [ ] Spec AC "Soak gate satisfied" is verifiable (seven nightly greens
      OR documented deferral with a verifiable alternative).
- [ ] Spec AC "Manual install path documented in README with checksum
      verification" is satisfied (no install subcommand per Non-Goals).

**Depends on**
Bead 5.

## Bead 7: Finalize ADR-0026 and cross-references

ADR-0026 is *drafted* alongside the Bead 5 deletion so its text mirrors what
shipped; this bead is the **finalization** — once Beads 5 and 6 are merged,
the ADR's "as-shipped" content is locked, cross-references are added, and
the spec's ADR Touchpoints section is updated to link to the finalized
ADR.

**Steps**
1. Finalize `.mindspec/docs/adr/ADR-0026-agentmind-extracted-to-standalone-repo.md`
   with Status=Accepted (already set at plan-approval time; this step
   re-verifies the content matches the as-shipped state). Required
   sections: **Context** (cite ADR-0011), **Decision** (AgentMind lives
   at `github.com/mrmaxsteel/agentmind`; mindspec imports only `client`
   and `wire`; IPC is OTLP/HTTP inbound + NDJSON-over-stdout outbound),
   **Deferred decisions** (UI-port discovery — port 8420 stays
   hardcoded; version-skew handling — no `client.Probe()` checks for
   v1.0.0; `mindspec agentmind setup` ownership — stays in mindspec for
   now; first-party `mindspec install agentmind` subcommand — deferred
   to follow-up spec), **Rollback procedure** (`git revert <merge-sha>`
   of the mindspec PR + drop the `require github.com/mrmaxsteel/agentmind`
   line from `go.mod`), and **Cross-reference ADR-0011** in the body.
2. Edit `spec.md`'s ADR Touchpoints section to replace the
   "ADR-0026 (new, to be authored as part of this work)" placeholder with
   a live link to the finalized ADR file.
3. Run `mindspec doctor` (or equivalent ADR-existence validator) and confirm
   it reports ADR-0026 as present.

**Verification**
- [ ] `bd show ADR-0026` (or filename existence) succeeds.
- [ ] Plan frontmatter `adr_citations` includes `ADR-0026` (already
      present at plan-approval time; no edit required).
- [ ] `spec.md` ADR Touchpoints links to the finalized ADR.
- [ ] `mindspec doctor` reports no missing-ADR errors for this spec.

**Acceptance Criteria**
- [ ] Spec AC "ADR-0026 committed and referenced from spec's ADR
      Touchpoints" is fully satisfied (drafted in Bead 5, finalized
      here).
- [ ] ADR-0026 file exists at
      `.mindspec/docs/adr/ADR-0026-agentmind-extracted-to-standalone-repo.md`
      with Status=Accepted and cross-references ADR-0011.
- [ ] `spec.md`'s ADR Touchpoints section links to the finalized ADR
      (no `<TBD>` or "to be authored" placeholders remain).
- [ ] `mindspec doctor` reports no missing-ADR errors for spec 083.

**Depends on**
Beads 5, 6.

## Provenance

| Acceptance Criterion (from spec) | Verified By |
|---|---|
| `find internal/agentmind` returns no results | Bead 5 verification |
| `grep -r 'http.HandleFunc.*"/v1/logs"' .` returns no results | Bead 5 verification |
| `go list -deps ./cmd/mindspec` shows only `client` + `wire` from agentmind (Test F) | Bead 4 CI step + Bead 5 (green by absence) |
| `go build ./cmd/mindspec` and `go test -short ./...` pass every commit | Every bead (Hard Constraint #6) |
| `mindspec record start` exits non-zero with binary absent (telemetry-as-output) | Bead 3a integration test |
| `mindspec viz` / `agentmind serve` / `agentmind replay` exit non-zero with binary absent (interactive) | Bead 4 integration test |
| `mindspec bench run` exits 0 with exactly one centralized warn line (batch) | Bead 3a integration test + Bead 3a `sync.Once` unit test |
| `mindspec agentmind setup` exits 0 with exactly one centralized warn line (batch) — OR documented as satisfied-by-design if `setup` does not call `AutoStart` | Bead 3a step 4 + Bead 3a integration test (setup-specific) |
| NDJSON byte-for-byte parity (Phase 2, alias state) — fixture produced via canonical `wire/event.go` encoder | Bead 2 step 5 (canonical-encoder fixture) + Bead 2 parity test |
| NDJSON semantic equivalence (Phase 4+, subprocess state) via spec-mandated normalization tool | Bead 4 CI live-capture job (normalize-and-diff against reference fixture) |
| `mindspec agentmind serve --help` / `replay --help` / `viz --help` print unchanged usage text | Bead 4 `--help` golden test (golden captured at pre-Bead-2 commit SHA) |
| mindspec binary size shrinks; before/after recorded | Bead 5 commit message |
| ADR-0026 committed and referenced from spec's ADR Touchpoints | Bead 7 (drafted in Bead 5, finalized here) |
| Release branch has `require agentmind v1.0.0` and no `replace` directive | Bead 6 verification |
| Read-side consumer rewire: `runner.go`, `session.go`, `recording/collector.go` consume `client.ReadEvents` on subprocess stdout (Hard Constraint #3) | Bead 3b (file-tailing-escape-hatch + OTLP-parser-residue assertions) |
| Test A (standalone-binary check) | Bead 1 (deferred to binary-publishing tag if v0.0.1 is scaffold-only) |
| Test B (no-mindspec-dep check) | Bead 1 |
| Test C (per-class degradation) — telemetry-as-output + batch (`bench run`, `agentmind setup`) | Bead 3a |
| Test C (per-class degradation) — interactive (`viz`, `agentmind serve`, `agentmind replay`) | Bead 4 |
| Test D (live capture) — Phase 2 byte-for-byte | Bead 2 |
| Test D (live capture) — Phase 4+ normalize-and-diff | Bead 4 |
| Test E (no-circular-discovery) — pre-deletion | Bead 4 CI step against sibling agentmind checkout |
| Test E (no-circular-discovery) — post-deletion continuity | Bead 6 CI shallow-clone step against pinned agentmind v1.0.0 |
| Test F (import-boundary) | Bead 4 CI step + Bead 5 (green by absence within mindspec) |
| Test G (Phase 0 prerequisite gate, agentmind v0.0.1 tag) | Bead 1 |
| Test-G-style precondition gate, agentmind v0.1.0 tag | Bead 2 step 1 |
| Test-G-style precondition gate, agentmind v0.3.0 tag | Bead 3a step 1 |
| Test-G-style precondition gate, agentmind/wire normalization tool published | Bead 4 step 3 |
