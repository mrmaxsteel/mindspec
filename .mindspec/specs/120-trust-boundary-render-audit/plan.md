---
adr_citations:
    - ADR-0042
    - ADR-0037
    - ADR-0035
approved_at: "2026-07-19T01:57:26Z"
approved_by: user
bead_ids:
    - mindspec-ws9y.1
    - mindspec-ws9y.2
    - mindspec-ws9y.3
    - mindspec-ws9y.4
    - mindspec-ws9y.5
spec_id: 120-trust-boundary-render-audit
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/idvalidate/ids.go
        - internal/idvalidate/ids_test.go
        - internal/idvalidate/testdata/live_spec_ids.txt
        - internal/idvalidate/testdata/live_bead_ids.txt
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/workspace/worktree.go
        - internal/workspace/workspace.go
        - internal/phase/derive.go
        - internal/phase/cache.go
        - internal/complete/complete.go
        - internal/next/mode.go
        - internal/next/beads.go
        - internal/next/guard.go
        - internal/approve/impl.go
        - internal/approve/plan.go
        - internal/approve/spec.go
        - internal/resolve/target.go
        - internal/lifecycle/finalize_orphans.go
        - internal/lifecycle/landed.go
        - internal/lifecycle/orphans.go
        - internal/lifecycle/stale_open.go
        - internal/lifecycle/scan.go
        - internal/layout/mover.go
        - internal/panel/gate.go
        - internal/instruct/panelstate.go
        - internal/instruct/run.go
        - internal/hook/hook.go
        - internal/recording/markers.go
        - internal/recording/recording.go
        - internal/bead/bdcli.go
        - internal/bead/hygiene.go
        - internal/validate/plan.go
        - internal/validate/state.go
        - internal/validate/beads.go
        - internal/validate/adr_divergence.go
        - internal/contextpack/budgeter.go
        - internal/contextpack/beadctx.go
        - internal/executor/mindspec_executor.go
        - internal/spec/list.go
        - internal/guard/guard.go
        - cmd/mindspec/repair.go
        - cmd/mindspec/impl.go
        - cmd/mindspec/next.go
        - cmd/mindspec/complete.go
        - cmd/mindspec/bead.go
        - cmd/mindspec/panel.go
        - cmd/mindspec/release.go
        - cmd/mindspec/state.go
    - depends_on: []
      id: 3
      key_file_paths:
        - internal/config/config.go
        - internal/workspace/containment.go
        - internal/workspace/containment_test.go
        - internal/guard/guard.go
        - internal/executor/mindspec_executor.go
        - internal/gitutil/gitops.go
        - internal/complete/complete.go
        - internal/next/guard.go
        - internal/hook/dispatch.go
        - internal/instruct/worktree.go
        - internal/instruct/run.go
        - internal/instruct/templates/implement.md
        - cmd/mindspec/impl.go
        - cmd/mindspec/complete.go
        - cmd/mindspec/cwdsafety.go
        - cmd/mindspec/next.go
        - cmd/mindspec/spec.go
        - cmd/mindspec/instruct_tail.go
    - depends_on:
        - 1
        - 2
        - 3
        - 5
      id: 4
      key_file_paths:
        - internal/lint/boundary_test.go
        - internal/lint/composition_ratchet_test.go
        - internal/lint/reverse_derivation_ratchet_test.go
        - internal/lint/exec_operand_ratchet_test.go
        - internal/lint/render_ratchet_test.go
        - internal/lint/template_classification_test.go
        - internal/lint/testdata/ratchet_fixtures.go
        - .mindspec/domains/workflow/OWNERSHIP.yaml
    - depends_on:
        - 1
      id: 5
      key_file_paths:
        - internal/idvalidate/idrender/idrender.go
        - internal/idvalidate/idrender/idrender_test.go
        - internal/complete/complete.go
        - internal/complete/panel_advisory.go
        - internal/next/guard.go
        - internal/next/select.go
        - internal/instruct/instruct.go
        - internal/hook/dispatch.go
        - internal/approve/impl.go
        - internal/approve/plan.go
        - internal/approve/spec.go
        - internal/contextpack/beadctx.go
        - internal/contextpack/budgeter.go
        - internal/bead/hygiene.go
        - internal/executor/mindspec_executor.go
        - internal/panel/tally.go
        - internal/config/config.go
        - internal/redact/redact.go
        - internal/redact/redact_test.go
        - internal/harness/idgate.go
        - internal/harness/sandbox.go
        - internal/harness/scenario_worktree.go
        - internal/harness/scenario_bead_lifecycle.go
        - internal/harness/scenario_contract_hardening.go
        - internal/harness/scenario_spec_lifecycle.go
        - internal/harness/scenario_safety.go
        - internal/harness/asserts.go
        - cmd/mindspec/next.go
        - cmd/mindspec/config.go
        - cmd/mindspec/release.go
        - cmd/mindspec/bead.go
        - internal/doctor/lifecycle_integrity.go
        - internal/doctor/orphaned_beads.go
        - internal/layout/mover.go
---
# Plan: 120-trust-boundary-render-audit

Five beads implement the trust-boundary spine per the spec's bead
sequencing block (round-5 ruling: NO sixth bead; sanctioned fallback
splits 2a/2b, 4a/4b, 5a/5b are noted inside their bead sections). Only
genuinely consumed outputs are edges — the plan-gate round-1 panel
corrected two edges the spec's sequencing sketch had wrong (4 gains its
REAL edge on 5; 3 drops a FALSE edge on 1):

- **Bead 1 is the grammar prerequisite for Beads 2 and 5** (R1 grammar
  correction): without it the waist would brick 489 of 774 live bd IDs
  and 2 of 120 spec dirs. Bead 2's waist and consumer gates call the
  corrected `idvalidate` patterns; Bead 5's `idrender` identity leg
  requires them (under the old patterns every dotted-child bead ID would
  forcibly quote, breaking byte-identity repo-wide).
- **Bead 3 has NO dependencies**: it validates the config `worktree_root`
  path (charset/containment), not IDs — it never calls the corrected
  grammar — and ADR-0042 exists at plan time, before any bead. It is
  root-parallel with Bead 1.
- **Bead 4 depends on Beads 1–3 AND 5**: its two-way allowlists must be
  TRUE at the bead boundary, never asserted-but-false — the (a)/(b)/(c)
  forward scans assert the waist-routed and gated dispositions Bead 2
  lands, the check-at-use call-site pins reference Bead 3's containment
  routing, and the (g)/(h) scans consume Bead 5's outputs (the
  `idrender`-routed `cmd/mindspec/next.go:200/:284` state lines and the
  R7 harness `runBD`/`runBDMust` caller-site gates). Bead 4 must never
  paper over a pending site with an asserted-but-false allowlist entry —
  the 4→5 edge makes that ordering structural.
- Waves: W1 = {1, 3}, W2 = {2, 5}, W3 = {4}. Longest chains: 1 → 2 → 4
  and 1 → 5 → 4, both depth 3. Parallelism: 1 ∥ 3 in W1; 2 ∥ 5 in W2
  after Bead 1; Bead 4 runs last, after 1–3 and 5.

**Shared-file adjacencies WITHOUT edges** (shared source files are not
dependencies; whichever bead merges second rebases trivially — flagged for
the implementers):

- Beads 2 ∥ 3 both touch `internal/executor/mindspec_executor.go`
  (Bead 2: the reverse-derivation gates at `:289/:453/:647`; Bead 3: the
  check-at-use insertions at `:112/:132/:216/:226/:227/:756/:759`) and
  `cmd/mindspec/impl.go`/`complete.go` (Bead 2: the R3 `args[0]`/beadID
  ingress gates at `impl.go:44`; Bead 3: the auto-cd check-at-use at
  `impl.go:66`/`complete.go:76`). The `impl.go:44` and `:66` hunks sit in
  the same function — coordinate the rebase carefully. Bead 3's predicate
  home is a NEW file (`internal/workspace/containment.go`), disjoint from
  Bead 2's `worktree.go`/`workspace.go` waist edits. The wave order (3 in
  W1, 2 in W2) means Bead 2 branches after Bead 3 merges, reducing the
  flagged same-function rebases for the 2-vs-3 pairs to zero.
- Beads 2 ∥ 5 share `complete.go` (Bead 2: ingress gates + D1 refusal
  routing; Bead 5: `FormatResult` free-text escapes), `approve/impl.go`
  (Bead 2: early gate + `readPlanBeadIDs`; Bead 5: `implAdvisorySlotLine`
  + open-child-hint escapes), `approve/plan.go` (Bead 2: the `:586/:616/
  :748/:820` argv gates; Bead 5: the `:378/:385/:391` display escapes),
  `contextpack/budgeter.go` (Bead 2: the `:152`/`:179` gates; Bead 5:
  `renderHeader` escapes), `bead/hygiene.go` (Bead 2: the `:155`
  status-update argv gate; Bead 5: `FormatReport` escapes),
  `cmd/mindspec/next.go` (Bead 2: ingress; Bead 5: `idrender` state lines
  + the `formatClaimLine` seam), and the waist-caller files
  `cmd/mindspec/bead.go`/`cmd/mindspec/release.go`/`approve/spec.go`
  (Bead 2: `(string, error)` waist-call routing; Bead 5:
  `FormatReport`-adjacent and refusal-text escapes — disjoint lines).
  `internal/harness` is NOT shared: ALL harness id-gating — the
  `sandbox.go:414`/`:430` IN-WRAPPER gates AND the NEW `idgate.go`
  helper + caller-site routing — lives in Bead 5b; Bead 2 owns
  production packages only. All disjoint functions.
- Beads 3 ∥ 5 share `internal/config/config.go` (Bead 3: `worktree_root`
  ingress at `:424-425`; Bead 5: R8 `gate_authority` quoting at `:510`)
  and the render functions that carry BOTH an executable `cd` line and
  free-text fields (`complete.FormatResult`, `next/guard.go
  DirtyTreeFailure`, the executor conflict failures): Bead 3 routes ONLY
  the `cd` line through the emitter; Bead 5 escapes ONLY the free-text
  fields — disjoint lines within the same function.

**Plan-level choices the spec delegates, resolved here:**

- **AC-24 empty-sentinel discipline (round-6 F1, "plan's pick")**:
  `idrender.Spec`/`idrender.Bead` treat the EMPTY string as identity —
  `""` in, `""` out, documented in the package contract as the sentinel
  passthrough. Rationale: `""` is the established no-value sentinel
  (spec-mode `ResolvedWork.SpecID`, and the post-gate product of
  `parseSpecID` on an invalid title), it carries zero bytes so it cannot
  smuggle hostile content, and a single-home rule beats scattering
  per-site empty guards. The clean spec-mode state line therefore renders
  `spec=` byte-identically to today, pinned by AC-24's subtest.
- **AC-23 `TestResolveModeHostileTitle` no-recording-write leg (round-6
  F2, "the plan names which leg asserts it")**: the asserting leg is the
  explicit cross-reference to **`TestRecordingWriteGates`** — the class-5
  write-gate in `internal/recording` is the covering gate, and
  `TestResolveModeHostileTitle` asserts `ResolvedWork.SpecID == ""` which
  the existing `cmd/mindspec/next.go:287-293` `SpecID != ""` guard turns
  into no write attempt. The property is doubly held; no separate
  cmd-level subtest is added.
- **R7 bd-wrapper gate placement ("plan's pick", revised at plan-gate
  round 1)**: a shared harness-local id-gating helper
  (`requireValidBeadID(t testing.TB, id string)` in a NEW
  `internal/harness/idgate.go`, Bead 5b) that every `runBD`/`runBDMust`
  caller-site id operand routes, fataling via `t.Fatalf` before any bd
  spawn — one point for AC-19's bd leg. The two IN-wrapper builds
  (`sandbox.go:414` `CreateBead --parent`, `:430` `ClaimBead`) gate with
  direct inline `idvalidate.BeadID` + fail-fast ALSO in Bead 5b, beside
  the helper and the R7 harness git sweep — ALL `internal/harness`
  id-gating lives in one bead, so AC-19's
  `TestHarnessBDWrapperRejectsMalformedIDs` exercises only Bead-5 work
  (no 5→2 coupling) and Bead 2 owns production packages only.
- **R5 `O_NOFOLLOW`-style openings ("plan-level")**: the AC-11
  grep-complete use-site inventory contains NO file-open use — every site
  is a worktree-create, `Chdir`, or `MkdirAll` — so no `O_NOFOLLOW` open
  lands in this spec. The doctrine ("a future USE that opens a file at a
  composed worktree path must open `O_NOFOLLOW`-style") is recorded in the
  containment predicate's doc comment, beside its ADR-0042 citation.
- **OQ3 latitude (R9 residual (b))**: the accepted-residual baseline
  stands — no new scrub pass for bare `user:pass`-without-`@` or internal
  hostnames. The decision is recorded in the R9 fixtures (Bead 5b) with
  the named over-scrub tradeoff pins; HC-7 DROP already bounds the leak.
  Re-measurement on the golden corpus is left to a future spec.
- **OQ1/OQ2/OQ4/OQ5 are already RESOLVED at spec approval** and are
  consumed as decided: predicate/emitter home `internal/workspace`
  (OQ1 — the new `containment.go`), `SpecIDFromMetadata` →
  `(string, error)` (OQ2), the nine waist helpers → `(string, error)`
  per the `SpecDir` SEC-1 precedent (OQ4-a; the newtype is the deferred
  post-120 terminal escalation, recorded in ADR-0042), `idrender` as an
  `internal/idvalidate` sub-package covered by core's existing
  `internal/idvalidate/**` glob with zero manifest change (OQ5).
- **ADR-0042 timing**: authored WITH this plan on the spec branch, before
  `mindspec plan approve` runs, `Status: Accepted` exactly — so
  `checkADRCitations`' `adr-cite-missing` and `ValidateDivergence`'s
  impl-approve-lane `adr-divergence-proposed` both pass (round-3 F3
  attribution). Unlike spec 119's ADR-0041 (authored in its Bead 6, hence
  uncitable at plan time), ADR-0042 EXISTS at plan-approve time and is
  therefore cited in this plan's `adr_citations`.

## ADR Fitness

- **ADR-0042 (NEW — Render + Derivation Provenance)** — authored at plan
  time alongside this plan (`.mindspec/adr/ADR-0042-render-derivation-
  provenance.md`, `Status: Accepted`, `Domain(s): workflow, core,
  execution, context-system` — matching this spec, so `checkADRCitations`'
  intersection gate passes). It is the PRIMARY contract this plan
  implements: grammar-correct `idvalidate` as prerequisite; fail-closed
  validation at the composition waist AND at all five authority-bearing
  consumer classes under the gate-all-ids rule (no bd-minted exemption —
  bd ids are agent-writable, proven by `bd create --force --id`);
  `termsafe.Escape` for free-text display; the forced-safe `idrender`
  rule; the `worktree_root` predicate with the TOCTOU-bounded check-at-use
  residual; the R6(g) wrapper-agnostic lint as the by-construction
  enforcer; the convergence stopping rule; and the OQ4-b newtype deferral.
  It fits — no existing ADR records any of this, and every bead cites or
  enforces a section of it (Bead 1 from the `ids.go` doc header, Bead 2
  from the waist helpers' doc comments, Bead 3 from the predicate's doc
  comment, Bead 4 as the mechanized enforcer, Bead 5 as the render-layer
  application).
- **ADR-0037 (Panel Gate as Enforced Contract)** — remains the best
  choice, consumed UNCHANGED. Its 116 amendment records the termsafe
  single-home doctrine this spec's R4 sweep extends to new consumers;
  nothing 0037 governs (panel decision ladder, thresholds, registration)
  changes. The `internal/panel` leaf gains a leaf-safe `idvalidate` import
  at the `ResolveGateFacts` seam (Bead 2) — a validation insertion, not a
  gate-semantics change. No amendment.
- **ADR-0035 (Agent Error Contract)** — remains the best choice, applied
  not amended. Every NEW refusal this plan introduces (waist/D1/D2
  derivation refusals, the R3 ingress refusals, the R5 config refusal, the
  AC-25 plan-frontmatter refusal) carries a genuine final `recovery:` line
  with a single convergent lever, the hostile value escaped-only —
  `guard.HasFinalRecoveryLine`-conformant. The spine STRENGTHENS 0035's
  recovery-executability guarantee: a printed recovery command now embeds
  either a validated-clean operand or a trusted shell-quoted root, never a
  hostile value.
- **ADR-0030 (Executor Boundary)** — untouched, prose-only context by
  choice (per the spec's ADR Touchpoints): the executor changes in Beads
  2/3/5 alter recovery-message rendering, reverse-derivation gating, and
  check-at-use routing only — not the enforcement-vs-executor import
  boundary 0030 governs. It is deliberately NOT in `adr_citations`
  (citing adds nothing; its Domain(s) intersection is carried by the
  three cited ADRs).

## Testing Strategy

- **Unit tests per package, named per the spec's ACs.** Every AC-1..AC-27
  proof is a NAMED new test (the spec's exact names) chained with an
  existence discriminator (`grep -q 'func Test<Name>'`) so each proof is
  RED at the spec-init SHA (zero `*.go` hits today). Hostile pattern
  throughout is 116's control triple `"x\x00\x1b[31m\nrecovery: forged"`
  asserted by the `assertClean` triple; the hostile-OPERAND pattern for
  derivation tests is the printable triple (`".worktrees && curl evil|sh
  #"`, `"../../outside"`, a space+`;`-bearing segment).
- **Byte-identity discipline in every named test**: a clean-fixture
  subtest asserts rendered/derived bytes IDENTICAL to today for
  well-formed input, and the clean set always includes the round-3
  shapes — dotted child `mindspec-9cyu.1`, multi-level `mindspec-69y.2.2`,
  legacy `mindspec-0ke`, letter-suffixed `008b-human-gates` — so a
  degenerate always-quote, always-refuse, or still-too-narrow grammar
  fails at every site.
- **The live-inventory fixture (AC-1)** is a committed testdata snapshot
  of ALL 120 current `.mindspec/specs/` dir names and ALL live bd IDs
  (~774 at plan time; regenerated at fixture-commit time — the counts are
  descriptive of the snapshot date, not pinned) — RED today (489+2
  failures against the old patterns), and re-generated at review time
  against live data (additions must pass or the fixture is extended — the
  ratchet against future legacy forms).
- **Hermetic fixtures.** Derivation/containment tests build real
  throwaway trees (scratch specs roots, real `os.Symlink` ancestors for
  AC-10/AC-11, scratch `.mindspec/config.yaml` for AC-13, temp git repos
  for the finalize-orphan/branch fixtures); bd is never spawned — every
  bd-touching assertion runs through the existing recording seams
  (`runBDFn`/`listJSONFn` in `internal/phase` per `derive.go:48`, the
  `internal/approve` `plan*`/`impl*` seam-vars, `beadShowFn` in
  `internal/contextpack`, the `internal/bead` `execCommand` stub at
  `bdcli.go:24`) so zero-spawn properties are assertable (AC-25/26/27).
- **The lint tests (R6/AC-14) are themselves the consumer-inventory
  proof**: eight two-way `go/ast` scans in `internal/lint` with negative
  fixtures under `internal/lint/testdata` proving each scan FLAGS (a new
  helper call, a new `"bead/"+x` and `".mindspec/specs/"+x` concat, a new
  `filepath.Join(dir, specID)`, an unvalidated workspace-prefix
  `TrimPrefix`, a specs-root `ReadDir` feeding a Join, an ungated
  id-operand argv — including one held in a non-id-named variable and one
  introduced through a NEW wrapper function (the round-10 wrapper-agnostic
  discriminator), an id-provenance allowlist justification (itself a
  failure, round 9), an ungated `Printf` of a `beadID`-named value, an
  unclassified template field) AND that deleting an allowlisted site fails
  on the allowlist side (two-way non-vacuity).
- **Mutation-style hostile-triple tests**: every gate is exercised with
  `--help` (option injection), `x;evil` (printable metacharacter), and the
  control triple, asserting zero spawn / zero write / forced quote per the
  gate's class; RED-on-revert is the standing discipline — deleting any
  enumerated gate turns its named test RED and (for argv/render gates)
  additionally trips the AC-14 (g)/(h) scans.
- **Pre-existing-RED caveat (z4ps)**: `internal/harness` and
  `internal/instruct` carry known pre-existing test-isolation flakiness
  (`TestRun_IdleNoBeads` cwd/state leakage, filed as `mindspec-z4ps`).
  Verification gates require the WHOLE suite (`go build ./... && go test
  ./...`, harness per existing convention) to pass EXCLUDING only those
  pre-existing failures, which must be byte-identically the same failures
  as at the spec-init SHA — no new red anywhere.
- **AC-global** runs per bead (build + full test + `golangci-lint run
  ./...` clean) and once more at spec end alongside `mindspec validate
  spec 120-trust-boundary-render-audit` (advisory WARN acceptable).

## Bead 1: idvalidate grammar correction + live-inventory fixture

R1 — the prerequisite for everything. Corrects the two validator patterns
to the framework's REAL ID grammar (a strict contract WIDENING), fixes the
false doc comments, and pins the whole live inventory with a committed
fixture so a future legacy form cannot silently regress the grammar.
Without this bead the waist would brick 489 of 774 live bd IDs (every
dotted epic-child and every short-suffix legacy ID) and the 2 letter-
suffixed spec dirs.

**Steps**

1. In `internal/idvalidate/ids.go`, correct `specIDPattern` (`:19`) to
   `^[0-9]{3,}[a-z]?-[a-z0-9]+(?:-[a-z0-9]+)*$` (one optional letter
   suffix after the number; everything else unchanged) and
   `beadIDPattern` (`:29`) to
   `^[a-z][a-z0-9]*(?:-[a-z0-9]+)+(?:\.[0-9]+)*$` (any-length alnum
   segments — drops the false `{4,}` floor; still ≥2 segments; zero or
   more dotted NUMERIC child suffixes at any depth). Keep every existing
   explicit rejection check in `SpecID`/`BeadID` verbatim: empty, `.`,
   `..`, path separators `/\`, glob metacharacters `*?[]{}`. Traversal
   stays structurally impossible (every `.` must be followed by digits).
2. Correct the doc comments at `ids.go:25-29` and `:81-94` — the
   "intentionally does NOT model any nested/hierarchical ID format" and
   `{4,}` forward-compat claims are false against bd's real minting.
   Cite ADR-0042 from the `ids.go` doc header (the AC-22 citation leg).
   The SEC-1 header hazard (`ids.go:1-9`) is unchanged; the package stays
   a stdlib-only leaf (regex + tests only — no new imports).
3. Commit the live-inventory fixture: `internal/idvalidate/testdata/
   live_spec_ids.txt` (all 120 current `.mindspec/specs/` dir names,
   incl. `008b-human-gates`, `008c-prime-compose`) and
   `live_bead_ids.txt` (all unique bd IDs from `.beads/issues.jsonl` —
   ~774 at plan time, regenerated at fixture-commit time — incl.
   `mindspec-9cyu.1`, `mindspec-69y.2.2`, `mindspec-0ke`,
   `mindspec-mol-015`). Add **`TestIDValidateAcceptsLiveInventory`**:
   every snapshot entry passes `SpecID`/`BeadID` — RED today (489+2
   failures against the old patterns).
4. Add **`TestIDValidateWideningPreservesRejections`**: the full hostile
   table (metacharacters `;|$&()#` + backtick + quotes, `/\`, `..`,
   `.`-leading/trailing, whitespace, control bytes, `*?[]{}`, uppercase,
   empty, bare `.`/`..`, `a..1`, `a.b` non-numeric child) still REJECTS;
   plus a sample of old-pattern-accepted IDs still passes (the widening
   direction proven: old-accept ⊂ new-accept).

**Verification**

- [ ] `go test ./internal/idvalidate/...` passes; `golangci-lint run
      ./internal/idvalidate/...` clean
- [ ] `TestIDValidateAcceptsLiveInventory` green with the FULL regenerated
      inventory (all bd IDs + all 120 spec dirs); existence discriminator
      proves it RED at the spec-init SHA
- [ ] `TestIDValidateWideningPreservesRejections` green: every hostile
      fixture rejected, widening-direction sample passes
- [ ] Inventory-freshness spot check: regenerate the snapshot from live
      data and diff against testdata (additions must also pass)
- [ ] `ids.go` doc header cites ADR-0042; doc-comment falsehoods gone
- [ ] `go build ./... && go test ./...` — no new red anywhere (z4ps
      caveat per Testing Strategy)

**Acceptance Criteria**

- [ ] AC-1 — corrected grammar accepts the full live inventory
      (fixture-pinned, RED today); every hostile rejection preserved;
      widening direction proven
- [ ] AC-22 (citation leg) — `ids.go` doc header cites ADR-0042

**Domain:** core (`internal/idvalidate/**`).

**Depends on**: None (foundational; the prerequisite for Beads 2 and 5,
and transitively 4; Bead 3 is grammar-independent and root-parallel).

## Bead 2: Composition waist, reverse/consumer gates, explicit-ingress + repair verb

R2 + R3 — the L bead (sanctioned 2a/2b split; the boundary is clean: 2a
compiles + greens standalone as the waist + all consumer gates; 2b is pure
ingress ergonomics + the repair verb; split trigger — if the session
cannot get 2a compiling + green with context to spare, commit 2a and
dispatch 2b as a follow-on at this documented boundary, no re-plan
needed). This bead owns PRODUCTION packages only — all `internal/harness`
id-gating is Bead 5b's. Moves the hard guarantee to the ten
`workspace` composition helpers, lands the D1/D2 policy points, gates
every reverse-derivation site and all five consumer classes under the
round-9 gate-all-ids rule (NO bd-minted exemption), and adds the explicit
CLI early-gates plus the `mindspec repair spec-title` convergent lever.

**Steps**

*2a — the waist and the consumer gates:*

1. **The waist**: the nine pure composition helpers in
   `internal/workspace/worktree.go:39-99` (`SpecBranch`, `BeadBranch`,
   `SpecWorktreeName`, `BeadWorktreeName`, `SpecWorktreePath`,
   `BeadWorktreePath`, `FinalizeBranch`, `FinalizeWorktreeName`,
   `FinalizeWorktreePath`) validate their ID argument via the corrected
   `idvalidate` and return `(string, error)` (OQ4-a — the `SpecDir`
   SEC-1 precedent, `workspace.go:463-468`; `SpecDir`'s existing contract
   unchanged). Route ALL 62 non-test call sites across `cmd/`+`internal/`
   (SpecBranch 10, BeadBranch 19, SpecWorktreeName 4, BeadWorktreeName
   10, SpecWorktreePath 13, BeadWorktreePath 3, FinalizeBranch 2,
   FinalizeWorktreePath 1): ambient callers map the error to their
   existing degrade channel (skip + one escaped warning), explicit-verb
   callers to a convergent refusal — the D1/D2 doctrine. The
   `lifecycle/orphans.go:133/:141` compositions inherit: the fail-closed
   gate consumer refuses via the existing `firstErr` discipline
   (`orphans.go:151-155`), the fail-open wrapper keeps skip semantics.
   Cite ADR-0042 from the waist helpers' doc comments (AC-22 leg).
2. **D1**: `phase.SpecIDFromMetadata` (`derive.go:106`) becomes
   `(string, error)` (OQ2). Caller routing exhaustively per the spec:
   `FindEpicForBeadWithCache` propagates a NEW malformed-metadata error,
   `errors.Is`-distinct from `phase.ErrNoEpicLineage` (`derive.go:27`),
   so `complete.Run`'s spec-119 preflight (`complete.go:349-381`) REFUSES
   before any mutation and never falls back to cwd resolution;
   `DiscoverActiveSpecsWithCache` and `lifecycle/scan.go:100` skip the
   malformed epic with one escaped warning naming the repair lever;
   `cache.go:189` and `approve/plan.go:223` treat invalid-derived as
   no-match; `specIDForEpicWithCache` (`:225`) falls back to its existing
   `<spec-id>` placeholder.
3. **D2**: `DetectWorktreeContext` (`workspace.go:655`) validates the
   trimmed suffixes at `:671/:675`; a failing segment leaves that match
   unrecognized (existing `WorktreeMain`/empty-ID semantics), so
   `guard.ActiveWorktree` is never composed from a hostile dir name.
4. **Panel leaf**: `internal/panel` imports `idvalidate` (leaf-safe —
   itself a stdlib-only leaf) and validates `beadID` at the
   `ResolveGateFacts` seam before the inline `"bead/"+beadID` at
   `gate.go:447`, thereby gating the `panelstate.go:261` on-disk
   `panel.json` BeadID feed and the `cmd/mindspec/panel.go:413` feed.
5. **Reverse-derivation gates** at the grep-complete round-4 inventory,
   per-site dispositions per the spec: `lifecycle/finalize_orphans.go:131`
   (skip + one escaped warning, never minted as a `FinalizeOrphan`),
   `lifecycle/landed.go:163` (corroboration proceeds root-only),
   `instruct/panelstate.go:555` (derived value discarded, never matched),
   `approve/impl.go:737` (enumeration entry skipped),
   `executor/mindspec_executor.go:453/:647` (entry skipped — never
   auto-merged/cleaned/embedded), `mindspec_executor.go:289`
   (`CompleteBead` REFUSES on a spec branch whose trimmed suffix fails
   `idvalidate.SpecID`), `layout/mover.go:394` `listSpecIDs`
   (validate-and-drop, so `routeReviewSlug`'s numeric-prefix fallback can
   only return a validated ID and the `:340` `Dst` composition is clean;
   unroutable slugs keep the existing `skipped` semantics).
6. **Round-5 consumer boundaries**: `layout/mover.go:367` `panelSpec`
   passes `idvalidate.SpecID` BEFORE `specExists` (`:383` is a bare
   `os.Stat` a `../..` value passes), invalid → `""`;
   `next/mode.go:64` `parseSpecID` validates its sliced result, invalid →
   `""` (existing no-spec semantics), with a NEW function-var epic-lookup
   seam in `internal/next` (per the established `findEpicForBeadFn`
   pattern — `mode.go:119` calls `phase.FindEpicBySpecID` directly
   today); `phase.FindEpicBySpecID`/`FindEpicBySpecIDWithCache`
   (`derive.go:582/:587` → `cache.go:171`) one-line boundary validation
   (invalid → the existing not-found error) PLUS the round-9 RETURN gate
   on `epic.ID` at `cache.go:189`; `recording.EmitBeadMarker`
   (`markers.go:87`) and `AddBeadToPhase` (`recording.go:128`) validate
   `specID` AND `beadID` before ANY write, invalid → skip + one escaped
   warning via the existing best-effort channel (`next.go:289-294`).
7. **Class-2 caller + consumer gates (rounds 6–7)**:
   `approve/impl.go:readPlanBeadIDs` (`:575-595`) idvalidate-gates every
   parsed `bead_ids` entry at the read and REFUSES convergently on a
   malformed one (lever: fix the plan frontmatter / re-run `mindspec plan
   approve`; hostile value escaped-only) — closing the `:290`→`:293`
   `readBeadStatus` option-injection and the `:696`
   `implCheckObligationsFn` leg; the `internal/bead` bd-argv CONSUMER
   boundary gates every id-taking call before any spawn — `BeadExists`
   (`bdcli.go:78-80`, malformed → `(false, nil)` not-found-by-
   construction), `GetMetadata` (`:314`), `MergeMetadata` (`:294`),
   `Close` (`:139-144`), `FixHygiene`'s status-update (`hygiene.go:155`)
   — covering `validate plan`'s `checkBeadIDs`→`CheckBeadExists` path and
   every future caller by construction; `validate/state.go:199`
   `checkBeadStatus` gains `idvalidate.BeadID` at its argv build
   (invalid → the existing could-not-verify warning, no spawn).
8. **Epic-side + gate-all-ids sweep (rounds 8–9)**: the three
   `bd list --parent <epicID>` builds gate `epicID` via
   `idvalidate.BeadID` before any spawn — `lifecycle/stale_open.go:32`,
   `lifecycle/orphans.go:31`, `phase/cache.go:261` (`fetchChildren`) —
   ambient skip doctrine, error through each site's existing channel,
   value escaped-only, ZERO bd argv. Then EVERY remaining id-position
   `bd`/`git` exec operand gates (NO bd-minted exemption — bd ids are
   agent-writable, `bd create --force --id` proven): `phase/cache.go:290`
   (`fetchEpic`), `phase/derive.go:720` (`FindEpicForBeadWithCache` show
   — doubly covered with R3), `next/beads.go:44/:123/:153/:165/:184`
   (`QueryReadyForEpic`/`ResolveActiveBead`/`ClaimBead`/`FetchBeadByID`/
   `FetchBeadAsOf`), `complete/complete.go:1166` (`advanceState`),
   `approve/plan.go:586` (bead-create `--parent`), `:616` (`dep add`),
   `:748` (`queryExistingChildren`), `:820`
   (`supersedeCloseExistingBeads`), `approve/impl.go:455`
   (`close <epicID>`), `contextpack/budgeter.go:152` (`BuildBead`), the
   `budgeter.go:179` Join (idvalidate-then-join on the agent-writable
   bd-metadata `spec_id`, per R6(c)), `contextpack/beadctx.go:12`
   `beadShowFn` (R3-gated CLI ingress), and the `internal/spec/list.go:34`
   and `next/guard.go:248` Join sites (waist-routed or
   validate-and-skip). (The harness IN-WRAPPER builds `sandbox.go:414`/
   `:430` gate in Bead 5b beside the `idgate.go` helper — not here.)
   Dispositions follow doctrine: ambient scans skip + one escaped
   warning; explicit verbs refuse convergently; a valid id (incl. dotted
   children) passes byte-identically — zero regression.

*2b — explicit-ingress early gates + the repair lever:*

9. **R3 specID**: `resolve.ResolveSpecPrefixWithCache` validates its
   RESULT (both the hyphen pass-through at `target.go:36-38` and the
   prefix-resolved value) — a hostile `--spec` refuses at the CLI surface
   with the "run `mindspec spec list`" lever before any composition;
   `impl approve` validates `args[0]` (`cmd/mindspec/impl.go:44`) before
   the `SpecWorktreePath` + `os.Chdir` at `:64-67`.
10. **R3 beadID**: `idvalidate.BeadID` at `complete.Run`'s `beadID`
    argument and each `cmd/mindspec` bead-taking verb (lever: `bd
    ready`); `internal/next`'s ready-set claim seam (explicit claim of a
    malformed ID refuses); `phase.FindActiveBeadForEpicWithCache`
    (ambient: not selected + escaped warning); epic IDs validated before
    any recovery-line embed (else the `<epic-id>` placeholder precedent,
    `derive.go:228`).
11. **The lever**: NEW `mindspec repair spec-title <epic-id> "<title>"`
    in `cmd/mindspec/repair.go` beside `repair phase`: validates its OWN
    `<epic-id>` argument via `idvalidate.BeadID` before any bd argv
    embed; hardcodes the `spec_title` key; merge-writes via the
    `repairMergeMetadataFn`/`bead.MergeMetadata` seam (`repair.go:22-25`
    — HC-5-safe, never raw replace semantics); refuses a replacement
    title whose slug fails the corrected `idvalidate.SpecID`; prints only
    escaped/validated values. All refusals: single lever, true source
    named, hostile value escaped-only, `guard.HasFinalRecoveryLine`-
    conformant (ZFC).
12. **Tests**: `TestCompositionHelpersRejectInvalidIDs` (workspace),
    `TestSpecIDFromMetadataRejectsInvalidSlug` (phase),
    `TestCompleteRunMalformedLineageRefusesConvergently` (complete —
    `findEpicForBeadFn` stubbed to RETURN the malformed-metadata error;
    lever applied via re-stub; convergence execution-proven),
    `TestDetectWorktreeContextRejectsMalformedNames` (workspace) +
    `TestGuardStateIgnoresMalformedWorktreeDirs` (guard),
    `TestCompleteRunRejectsInvalidBeadIDArg` +
    `TestNextClaimRejectsMalformedBeadID` + the epic-ID-embed subtest,
    `TestResolveSpecPrefixValidatesResult` +
    `TestImplApproveRejectsInvalidSpecIDArg`, `TestRepairSpecTitle`,
    `TestDiscoverActiveSpecsSkipsMalformedEpic`, the AC-23 suite
    (`TestFinalizeOrphansSkipsMalformedBranch`,
    `TestRouteReviewSlugIgnoresMalformedSpecDirs`,
    `TestCompleteBeadRejectsMalformedSpecBranch`, the enumeration-skip
    subtests, `TestResolveModeHostileTitle`,
    `TestPanelSpecRejectsTraversal` — positive fixture's slug prefix
    matching NO listed dir per round-6 F2 — and
    `TestRecordingWriteGates`), `TestReadPlanBeadIDsRejectsMalformed`
    (AC-25), `TestBeadIDArgvConsumerGate` +
    `TestValidatePlanMalformedBeadIDsZeroBD` +
    `TestBDListParentEpicIDGate` + `TestFetchChildrenEpicIDGate`
    (AC-26), and `TestHostileBDStoreIDNeverReachesArgv` with its
    companion legs in next/approve/complete/contextpack/bead (AC-27's
    production consumers; the harness fail-fast leg lands in Bead 5's
    `TestHarnessBDWrapperRejectsMalformedIDs`). Every test carries the
    clean-shape byte-identity subtest.

**Verification**

- [ ] `go test ./internal/workspace/... ./internal/phase/...
      ./internal/complete/... ./internal/next/... ./internal/approve/...
      ./internal/resolve/... ./internal/lifecycle/... ./internal/layout/...
      ./internal/panel/... ./internal/instruct/... ./internal/recording/...
      ./internal/bead/... ./internal/validate/... ./internal/contextpack/...
      ./internal/executor/... ./internal/spec/... ./internal/guard/...
      ./cmd/mindspec/...` passes; `golangci-lint run ./...` clean
- [ ] AC-2..AC-9 named tests green with hostile-triple refusal/skip and
      clean-shape byte-identity in each
- [ ] AC-23 suite green: every reverse site + round-5 consumer gated per
      its disposition; `TestPanelSpecRejectsTraversal`'s `../..`
      enforcement-was-missing leg proven
- [ ] AC-25/AC-26/AC-27: ZERO bd spawn on every malformed-id path
      (asserted at the named seams); valid ids spawn byte-identical argv;
      RED-on-revert spot-checked per site
- [ ] AC-4 and AC-8 convergence execution-proven (apply the lever →
      re-run passes)
- [ ] Waist helper doc comments cite ADR-0042
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-2 — nine waist helpers reject hostile IDs, byte-identical clean
      compositions
- [ ] AC-3 — `SpecIDFromMetadata` rejects hostile slugs
      (enforcement-was-missing proven)
- [ ] AC-4 — malformed-lineage refusal pre-mutation, `errors.Is`-distinct,
      convergent via `repair spec-title`
- [ ] AC-5 — `DetectWorktreeContext` + guard ignore malformed worktree
      dirs; dotted-child and finalize worktrees parse byte-identically
- [ ] AC-6 — beadID ingress gates (complete arg, next claim, ambient
      in_progress, epic-ID placeholder)
- [ ] AC-7 — specID ingress gates (`ResolveSpecPrefix` result,
      `impl approve args[0]` before chdir)
- [ ] AC-8 — `repair spec-title` merge-write seam, own-arg gate, slug
      refusal, escaped output
- [ ] AC-9 — ambient degrade: one hostile epic skipped with one escaped
      warning; instruct renders
- [ ] AC-23 — reverse-derivation gates + round-5 consumer boundaries, all
      dispositions
- [ ] AC-25 — `bead_ids` read-gate, zero bd invocation, convergent lever
- [ ] AC-26 — `internal/bead` consumer boundary + validate leg + the
      three `bd list --parent` epicID gates, zero spawn
- [ ] AC-27 — gate-all-ids: hostile store id (`--help`-minted) reaches
      ZERO bd/git argv through ANY production consumer; RED-on-revert per
      site (the harness fail-fast leg is Bead 5's)
- [ ] AC-22 (citation leg) — waist doc comments cite ADR-0042

**Domain:** core (`internal/idvalidate` consumption, `internal/workspace`,
`internal/phase`, `internal/recording`, `internal/spec`) + workflow
(`internal/complete`, `internal/next`, `internal/approve`,
`internal/resolve`, `internal/lifecycle`, `internal/layout`,
`internal/panel`, `internal/instruct`, `internal/hook`,
`internal/validate`, `internal/guard`, `cmd/mindspec`) + execution
(`internal/bead`, `internal/executor` reverse gates — NO
`internal/harness`; its id-gates are Bead 5's) + context-system
(`internal/contextpack` argv/Join gates) — per the spec's Impacted
Domains assignments for each touched glob.

**Depends on**: Bead 1 (the corrected grammar — applying the OLD patterns
at the waist would brick 489 live bead IDs and change D2 behavior for
every dotted-child and finalize worktree).

## Bead 3: worktree_root predicate, symlink containment, check-at-use, shell-safe emitter

R5 — the config path-component gate. `worktree_root` is agent-writable,
NOT ID-derived, and participates in every composed worktree path; it gets
ingress validation, symlink-aware containment with an honestly bounded
TOCTOU residual, check-at-use at the grep-complete use-site inventory, and
one exported shell-safe `cd` emitter that every executable-`cd` render
routes.

**Steps**

1. **Predicate home (OQ1)**: NEW `internal/workspace/containment.go` —
   the `worktree_root` ingress predicate, applied where the default is
   applied (`internal/config/config.go:424-425`): relative (no leading
   `/`), segments in the conservative charset (letters, digits, `.`, `-`,
   `_`; `/` separator), no `..` segment; `filepath.Rel`/lexical checks
   explicitly NAMED lexical-only in the doc comment. Cite ADR-0042 from
   the predicate's doc comment (AC-22 leg) and record the plan-level
   `O_NOFOLLOW` doctrine there (no file-open use exists in today's
   inventory — see the plan preamble).
2. **Symlink-aware containment** in the same home:
   `filepath.EvalSymlinks` on the resolved root AND on the deepest
   EXISTING ancestor of the composed path, which must resolve under the
   resolved root; any symlink component inside the agent-controlled
   suffix ancestry rejected; leaf validated lexically, existing-ancestor
   chain physically (the in-repo precedents: `validate/specid.go:
   SafePath`, `config.CanonicalPath`, the `journal.go:164`
   nearest-existing-ancestor pattern).
3. **Refusal/degrade routing**: lifecycle verbs REFUSE convergently on a
   failing `worktree_root` (lever: ``set worktree_root to .worktrees (the
   default) in .mindspec/config.yaml, then re-run``); never-block
   consumers that swallow config errors into `DefaultConfig()`
   (`guard.go:46-47`) degrade to the default + one escaped warning.
4. **Check-at-use** immediately before each USE of a composed worktree
   path, at the grep-complete inventory (AC-11 pins set-equality):
   worktree-create — `mindspec_executor.go:132` (`WorktreeOps.Create`,
   the primary spec-worktree create), `:227` (bead worktrees, inside
   `withWorkingDir(anchorRoot)`), `:759` (`gitutil.WorktreeAdd`,
   chore/finalize carrier) + its `:756` `MkdirAll`, and the
   `gitutil.WorktreeAdd`/`WorktreeAddDetach` wrappers themselves (the
   wrapper-level check covers any future caller by construction);
   composed-path chdir — `cmd/mindspec/impl.go:66`,
   `cmd/mindspec/complete.go:76`, and the executor
   `withWorkingDir(anchorRoot)` at `:226`; composed worktree-root mkdir —
   `mindspec_executor.go:112/:216`. Root-only chdirs (`impl.go:127`,
   `complete.go:78/:980`, `mindspec_executor.go:93`) are trusted-root,
   excluded. The check-at-use-to-operation window is the accepted
   residual (Non-Goals) — no atomicity claim.
5. **The single shell-safe `cd` emitter**, exported beside the predicate
   (representation per `cmd/mindspec/panel.go:shellQuoteTarget`:
   byte-identical when all bytes are unquoted-safe, else POSIX
   single-quoted). Route every executable-`cd` render: `guard.go:130`
   `CheckCWD`, `mindspec_executor.go:1195/:1221`,
   `complete.FormatResult`, `next/guard.go:DirtyTreeFailure`,
   `cmd/mindspec/next.go:196/:271`, `cmd/mindspec/spec.go:61`,
   `cmd/mindspec/instruct_tail.go:39` + the `run.go` redirect,
   `instruct/worktree.go:33`, `templates/implement.md:41`,
   `cwdsafety.go:emitCdBackNote`, `hook/dispatch.go`. Root-only sinks
   quote-emit and NEVER refuse (round-2 C8).
6. **Tests**: `TestWorktreeRootPredicate` (neg: metacharacter/absolute/
   `..`; the symlinked-ancestor fixture that lexical `Rel` alone would
   pass — the lexical-insufficiency discriminator; each printable-hostile
   value asserted to pass `termsafe.Escape` unchanged — the
   escaping-is-insufficient proof; pos: `.worktrees` and nested clean
   values byte-identical), `TestContainmentCheckAtUse` (composition-time
   pass → ancestor swapped for an outside symlink → check-at-use REFUSES,
   operation never attempted; the `WorktreeOps.Create` subtest pinning
   G3's site; the grep-complete set-equality companion over the pinned
   call-site list), `TestExecutableCdRendersShellSafe` (table-driven over
   the sink set; space-bearing root → quoted + `sh -c` round-trip; clean
   root → byte-identical; root-only sinks never refuse),
   `TestWorktreeRootRejectionRecoveryConverges` (refusal satisfies
   `HasFinalRecoveryLine`, no raw hostile bytes, lever applied → re-run
   passes; the guard `DefaultConfig`-degrade companion subtest).

**Verification**

- [ ] `go test ./internal/workspace/... ./internal/config/...
      ./internal/guard/... ./internal/executor/... ./internal/gitutil/...
      ./cmd/mindspec/...` passes; `golangci-lint run ./...` clean
- [ ] AC-10: symlinked-ancestor discriminator proves lexical-insufficiency;
      hostile values pass `termsafe.Escape` unchanged (escaping-is-
      insufficient proof); clean values byte-identical
- [ ] AC-11: check-at-use refuses the swapped-symlink race model at the
      use site; set-equality pin over the grep-complete inventory is
      green (a new create/chdir/mkdir site, or reverting any one
      check-at-use call, is RED)
- [ ] AC-12: every emitted `cd` round-trips a real shell with a
      space-bearing root; clean roots byte-identical; root-only sinks
      quote-emit, never refuse
- [ ] AC-13: refusal convergence execution-proven (rewrite to
      `.worktrees` → re-run passes); guard degrade path pinned
- [ ] Exactly ONE non-test predicate implementation and ONE non-test
      emitter implementation exist (single-home grep)
- [ ] Predicate doc comment cites ADR-0042
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-10 — predicate + symlink-aware containment with the
      lexical-insufficiency discriminator
- [ ] AC-11 — check-at-use at the grep-complete use-site inventory;
      TOCTOU residual accepted, not asserted un-winnable
- [ ] AC-12 — single shell-safe emitter across all executable-`cd` sinks;
      root-only sinks never refuse
- [ ] AC-13 — convergent config lever, execution-proven; never-block
      degrade pinned
- [ ] AC-22 (citation leg) — predicate doc comment cites ADR-0042

**Domain:** core (`internal/config`, `internal/workspace` predicate home)
+ workflow (`internal/guard`, `internal/hook`, `internal/instruct`
templates, `cmd/mindspec` sinks) + execution (`internal/executor`
check-at-use sites, `internal/gitutil` wrappers) — per the spec's
Impacted Domains assignments.

**Depends on**: None. This bead validates the config `worktree_root`
path (charset/containment) and shell-quotes `cd` renders — it never
calls the corrected ID grammar, and ADR-0042 exists at plan time (the
plan-gate round-1 panel removed the false edge on Bead 1). Root-parallel
with Bead 1; independent of Beads 2 and 5 (see the preamble adjacency
flags for the shared files).

## Bead 4: The consumer ratchet — eight lint scans, template classification, ownership claim

R6 — enforcement, not inventory (sanctioned 4a/4b split: 4a = the
consumer scans (a)/(b)/(c)/(g)/(h); 4b = the advisory pair (e)/(f) + the
template classification (d) — each side compiling + greening standalone).
Eight two-way `go/ast` scans in `internal/lint` (precedent
`boundary_test.go`), each against an audited allowlist, plus the
`internal/lint/**` ownership self-claim. The (g) scan is THE exhaustive,
by-construction enforcer of the round-9 gate-all-ids rule — wrapper-
agnostic, resolving bd/git invocations SEMANTICALLY at the exec seam /
wrapper-call graph, never by a fixed wrapper-name list; every prose site
list in the spec is an illustrative audited seed, not the enforcement
surface.

**Steps**

1. **(a) Composition-helper allowlist scan**
   (`TestWorkspaceCompositionCallSiteAllowlist`): every non-test call of
   the ten `workspace` composition helpers across `cmd/`+`internal/`,
   two-way; each allowlist entry names its covering gate (waist-internal;
   plus any early D-gate).
2. **(b) Inline-composition scan** (`TestInlineBranchCompositionForbidden`):
   any string concatenation with the `workspace.*Prefix` constants or the
   literals `"spec/"`/`"bead/"`/`"worktree-spec-"`/`"worktree-"`/
   `"chore/finalize-"`/`"worktree-finalize-"`/`".mindspec/specs/"` — and
   any `"reviews/"` path-segment concat with an ID-bearing identifier —
   outside `internal/workspace` fails unless allowlisted-with-
   justification. The `internal/panel` leaf entry records its in-package
   `idvalidate` gate; harness self-minted-literal sites record
   mindspec-authored provenance (the scan discovers the full set — e.g.
   `scenario_worktree.go:67/:69/:422/:430`,
   `scenario_contract_hardening.go:473-475` — and each entry gets its
   annotation at implementation time, round-4 F2).
3. **(c) Join scan** (`TestJoinWithIDForbidden`): any `filepath.Join`
   outside `internal/workspace` whose arguments include a
   `specID`/`beadID`-named identifier fails unless allowlisted-with-gate.
   `layout/mover.go:385`, `spec/list.go:34`, `next/guard.go:248`, and
   `contextpack/budgeter.go:179` each resolved (routed or gated — with
   `budgeter.go:179` specifically GATED per its agent-writable
   bd-metadata provenance, never allowlisted-with-a-false-gate).
4. **(e)/(f) Reverse defense-in-depth pair**
   (`TestTrimPrefixReverseDerivationGated`,
   `TestRootEnumerationReverseDerivationGated`): the workspace-prefix
   `TrimPrefix` scan and the specs-root/worktrees-root enumeration scan —
   both LABELED defense-in-depth, NO completeness assertion anywhere (the
   round-5 falsification); the nine Background-inventory sites appear
   gated, not allowlisted-ungated; deleting a site's `idvalidate` call is
   RED.
5. **(g) Whole-tree wrapper-agnostic exec-operand audit**
   (`TestArgvIDOperandGated`): every `bd`/`git` invocation across
   `cmd/`+`internal/` — `bead.RunBD`/`RunBDCombined`/`ListJSON`, the
   package seam-var defaults (`runBDFn`/`listJSONFn`/`planRunBDFn`/
   `planRunBDCombinedFn`/`implRunBDFn`/`implRunBDCombinedFn`/
   `specRunBDFn` and the `internal/lifecycle` seam closures), every
   `execCommand("bd"|"git", …)` build, every direct
   `exec.Command("bd"|"git", …)`, AND every wrapper form resolved
   SEMANTICALLY at its exec seam (the harness `Sandbox.runBD`/`runBDMust`
   spawn via `exec.LookPath("bd")`+`exec.Command(bdPath, …)` — invisible
   to name matchers — and any FUTURE wrapper). Call-site-keyed, two
   dispositions only: **(i) GATED** — every ID-position operand passes
   `idvalidate` at or before the call; **(ii) AUDITED-ALLOWLISTED —
   genuine NON-id operands ONLY** (framework-authored subcommands/flags,
   string literals, waist-composed branch operands, `rev-parse` SHAs,
   `--`-separated pathspecs, free-text operands, `RejectOptionLike`-gated
   refs, no-operand verbs). The allowlist SCHEMA admits only non-id
   justifications: an entry justified by id provenance ("bd-minted", "not
   agent-steerable") is itself a test failure (round 9). Git-side non-id
   sites land allowlisted with justifications (`gitutil/gitops.go`
   wrappers, `executor/mindspec_executor.go:868-1027`,
   `bootstrap/mergedriver.go:203/:216`, `doctor/beads.go:723/:736`, the
   harness git sites).
6. **(h) Raw-ID-render scan** (`TestRawIDRenderForbidden`): any
   `specID`/`beadID`-named identifier (or `ResolvedWork.SpecID`-shaped
   field) reaching a Printf-family/render position fails unless it routes
   `idrender.*` or is allowlisted-with-gate; the
   `cmd/mindspec/next.go:200/:284` state lines resolved as
   `idrender`-routed (Bead 5 lands the routing — hence this bead's
   dependency on Bead 5).
7. **(d) Template classification**
   (`TestInstructTemplateFieldClassification`): every
   `{{.Field}}`/`{{termsafe .Field}}` interpolation in
   `internal/instruct/templates/*.md` classified two-way {spine-gated ID,
   emitter-gated path, termsafe-routed free text, fenced payload};
   `.SpecGoal` pinned as fenced payload under 116's inherited persuasion
   Non-Goal.
8. **Negative fixtures** under `internal/lint/testdata` proving each scan
   flags: a new helper call; a new `"bead/"+x` concat; a new
   `".mindspec/specs/"+x+"/reviews/"` concat (G1's shape); a new
   `filepath.Join(dir, specID)`; a new unvalidated
   `strings.TrimPrefix(x, workspace.BeadBranchPrefix)`; a specs-root
   `os.ReadDir` whose names reach a Join; an ungated exec argv carrying a
   `specID`-named identifier; a new un-audited `exec.Command("bd", x)`
   whose id operand is held in a NON-id-named variable (the round-8
   call-site-keyed discriminator); a new id→bd-argv call through a NEW
   wrapper function that itself spawns bd via `exec.LookPath`+
   `exec.Command`, matching neither `bead.RunBD` nor `Sandbox.runBD` by
   name (the round-10 wrapper-agnostic discriminator); an allowlist entry
   with an id-provenance justification (the round-9 no-exemption
   discriminator); an ungated `fmt.Printf` of a `beadID`-named value; an
   unclassified template field. Plus delete-an-allowlisted-site fixtures
   proving the two-way direction.
9. **Ownership self-claim**: add `internal/lint/**` to
   `.mindspec/domains/workflow/OWNERSHIP.yaml` in the SAME commit as the
   ratchet tests (the manifest edit is itself workflow-owned), so the
   adr-divergence-unowned gate never sees an unowned touched file
   (AC-22).

**Verification**

- [ ] `go test ./internal/lint/...` passes; `golangci-lint run ./...`
      clean
- [ ] Every negative fixture provably flagged; every
      delete-an-allowlisted-site fixture provably fails (two-way
      non-vacuity)
- [ ] The (g) audit reports ZERO un-classified `bd`/`git` exec sites in
      `cmd/`+`internal/`; no allowlist entry carries an id-provenance
      justification; the round-10 new-wrapper fixture is flagged
- [ ] The (e)/(f) scans are labeled defense-in-depth in code comments and
      test names — no completeness claim anywhere
- [ ] `rg -n 'internal/lint' .mindspec/domains/workflow/OWNERSHIP.yaml`
      non-empty, same commit as the ratchet tests; the ADR-divergence
      gate passes with no unowned-file error
- [ ] The advisory seed greps from the spec's Validation Proofs run clean
      (every hit a gated site or an allowlist entry)
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-14 — all eight scans two-way green with every named fixture
      discriminator (extended literal set, call-site-keyed (g),
      no-id-exemption schema, wrapper-agnostic coverage, idrender-routed
      state lines, `.SpecGoal` fenced)
- [ ] AC-22 (ownership + gate leg) — `internal/lint/**` claimed by
      workflow in the same commit; ADR-divergence gate passes

**Domain:** workflow (`internal/lint/**` — claimed by this bead into
`.mindspec/domains/workflow/OWNERSHIP.yaml`; the manifest edit is itself
workflow-owned).

**Depends on**: Beads 1, 2, 3, 5 — the allowlists and gated dispositions
the scans assert must be TRUE at the bead boundary (Bead 2's waist
routing + consumer gates; Bead 3's containment/emitter routing; Bead 5's
`idrender` routing of the `next.go:200/:284` state lines and the R7
harness `runBD`/`runBDMust` caller-site gates the (g)/(h) scans
resolve), never asserted-but-false. This bead runs last (W3).

## Bead 5: Display sweep (termsafe + idrender) + R7/R8/R9 riders

R4 + the P3 riders (sanctioned 5a/5b split: 5a = the R4 sweep, both
halves incl. `idrender`, + AC-15..18 + AC-24; 5b = the R7/R8/R9 riders +
AC-19..21 — each side compiling + greening standalone). Free-text
agent-writable fields escape via `termsafe.Escape` per field; ID-typed
positions route the NEW forced-safe `idrender.Spec`/`idrender.Bead`
(valid → byte-identical; invalid → `strconv.Quote`, even when printable —
because `termsafe.Escape` is the identity on printable ASCII).

**Steps**

*5a — the R4 sweep + idrender:*

1. **`idrender`** as the `internal/idvalidate/idrender` sub-package
   (OQ5 — covered by core's existing `internal/idvalidate/**` glob, zero
   manifest change): `Spec(s)`/`Bead(s)` — `idvalidate` passes → return
   `s` byte-identical; `""` → identity (the plan-pick empty-sentinel
   passthrough, preamble); else → `strconv.Quote(s)`. Stdlib-only leaf.
2. **R4-freetext sweep** at the verified inventory, per-field never
   per-message, per-line for line-oriented bodies:
   `complete.FormatResult` free-text fields + userDirt porcelain
   (`:611-619` area); `next/guard.go:DirtyTreeFailure` userDirt;
   `cmd/mindspec/next.go:244` claiming line — extracted to the named
   testable seam `formatClaimLine(id, title)`; `next/select.go:
   FormatWorkList` (`:96`); `instruct/instruct.go` Warnings
   (`:147`/`:205`); `hook/dispatch.go:98/:126` free-text portions;
   `cmd/mindspec/config.go:reviewerCountNotesFor` (`reg.Slug()`);
   `approve/impl.go:implAdvisorySlotLine` + the open-child hint
   (`:265-276`); `approve/plan.go:378/:385/:391` +
   `approve/spec.go:119/:125/:129` + the per-line `IsTreeClean` porcelain
   bodies; `contextpack/beadctx.go:RenderBeadContext` single-line fields
   + `budgeter.go:renderHeader` (multi-line bodies stay fenced payload);
   `cmd/mindspec/release.go:205-213`; `bead/hygiene.go:FormatReport`
   (`:106/:115/:123`) + `FixHygiene` action strings (`:152`, printed at
   `cmd/mindspec/bead.go:135`); executor message bodies
   (`mindspec_executor.go:1186/:1212` conflicted joins per line, the
   `:826` porcelain body per line, git error text per line — the
   waist-validated `beadBranch`/`specBranch` operands stay RAW);
   defense-in-depth last: the production-dead `panelAdvisory`
   (`complete/panel_advisory.go`) + `panel/tally.go:VoteDecision`. The
   sweep remains by principle: re-grep `cmd/`+`internal/` for
   Printf-family renders of free-text agent-writable provenance and
   record additions in review evidence.
3. **R4-id routing**: `cmd/mindspec/next.go:200/:284` state lines (spec
   AND bead fields through `idrender`), the doctor/instruct
   orphan-report ID positions, mover plan/commit-text ID positions; plus
   the sweep-by-principle re-grep of Printf-family renders of
   `specID`/`beadID`-named values (ratchet-backed by Bead 4's (h) scan).
4. **5a tests**: `TestIDRenderForcedSafe` (every clean shape
   byte-identical; hostile triple + printable-malformed `120-x;evil`
   quoted; the in-test discriminator that `termsafe.Escape` ALONE passes
   `120-x;evil` unchanged; the state-line sink subtest; the empty-
   sentinel spec-mode byte-identity subtest),
   `TestCompleteFormatResult_HostileFieldsEscaped`,
   `TestNextDirtyTreeFailure_HostileFieldsEscaped`,
   `TestFormatWorkListHostileTitleEscaped`,
   `TestNextCmd_HostileBeadTitleEscaped` (via `formatClaimLine`),
   `TestCLISinksHostileFieldsEscaped`,
   `TestApproveHostileRendersEscaped`,
   `TestHygieneFormatReportHostileTitleEscaped`,
   `TestInstructHostileWarningsEscaped`,
   `TestHookDispatchHostileFieldsEscaped`,
   `TestRenderBeadContextHostileTitleEscaped`,
   `TestConflictFailureBodiesEscapedPerLine`,
   `TestLatentPanelSinksHostileFieldsEscaped` — each with the clean
   byte-identity subtest; plus the AC-18 negative pins (40-hex SHA,
   validated IDs incl. dotted child + `008b` raw through `idrender`,
   `spec/<id>`/`bead/<id>` operands, validated argv, template prose;
   116's AC5 six-test set and `panel tally` clean renders unmodified).

*5b — the riders:*

5. **R7 git**: option-injection guards at the dynamic-operand inventory
   (`sandbox.go:252/:264`, `scenario_worktree.go:388`, `asserts.go:458`,
   `scenario_safety.go:202`), fail-fast `t.Fatalf` via `RejectOptionLike`
   before any git spawn; literal/no-operand sites excluded; `mustRunGit`
   NOT blind-guarded; `--` separators where git grammar admits ambiguity.
6. **R7 bd-wrapper caller sites (round 10)**: NEW
   `internal/harness/idgate.go` shared helper
   (`requireValidBeadID(t, id)` — the plan pick) routed by every
   `runBD`/`runBDMust` id-operand caller site: `scenario_bead_lifecycle.
   go:242/:331/:335/:418/:602/:641`, `scenario_contract_hardening.go:
   125/:131/:265/:270/:349/:722/:859/:971`,
   `scenario_spec_lifecycle.go:373/:666`, `asserts.go:390/:408/:430` —
   fail-fast pre-spawn; non-id invocations (`export -o …`,
   `--title`/`--metadata` free text) excluded-with-justification and
   never false-fataled. PLUS the two IN-WRAPPER builds — `sandbox.go:414`
   (`CreateBead --parent`) and `:430` (`ClaimBead`) — gated with direct
   inline `idvalidate.BeadID` + fail-fast `t.Fatalf` before any spawn:
   ALL `internal/harness` id-gating (helper, caller sites, in-wrapper)
   lives in this bead (plan-gate round-1 move from Bead 2a — Bead 2 owns
   production packages only).
7. **R8**: `config.go:510` routes the `loop.gate_authority` key through
   `strconv.Quote` at BOTH interpolation points (the `:592` pattern); the
   one-line `expandSlots` fixture killing the dedup mutant (two
   same-named reviewers → distinct `R1`/`R2`, length 2).
8. **R9**: `internal/redact` fixtures — `ASIA`-prefixed AWS session
   keys, GCP markers (`"private_key_id"`, `AIza`), Azure `AccountKey=` —
   each scrub-or-drop, never raw with `ok==true`; the OQ3
   accepted-residual decision recorded (bare `user:pass`-without-`@`,
   internal hostnames — no new scrub pass, per the preamble); the IPv6
   over-scrub tradeoff fixtures pin today's `12:34:56`/`std::vector`
   behavior. Enum-first primary design byte-untouched.
9. **5b tests**: `TestHarnessGitExecRejectsOptionLikeRefs`,
   `TestHarnessBDWrapperRejectsMalformedIDs` (hostile ids fatal pre-spawn
   with ZERO bd processes through `ClaimBead`/`CreateBead --parent`/the
   shared helper path — all three now this bead's own work, and also
   AC-27's harness fail-fast leg; dotted-child ids byte-identical argv;
   non-id invocations never false-fatal; RED-on-revert also trips the
   AC-14 (g) scan), the AC-20 `gate_authority`/`expandSlots` fixtures,
   the AC-21 redact fixtures alongside the shipped Golden/Mutation
   suites.

**Verification**

- [ ] `go test ./internal/idvalidate/... ./internal/complete/...
      ./internal/next/... ./internal/instruct/... ./internal/hook/...
      ./internal/approve/... ./internal/contextpack/... ./internal/bead/...
      ./internal/executor/... ./internal/panel/... ./internal/config/...
      ./internal/redact/... ./internal/harness/... ./cmd/mindspec/...`
      passes; `golangci-lint run ./...` clean
- [ ] AC-15/16/17: hostile triple clean at every enumerated sink;
      per-line escaping preserves real newlines; branch operands stay RAW
- [ ] AC-24: identity on every clean shape; forced quote on
      printable-malformed with the Escape-is-identity discriminator;
      empty-sentinel `spec=` byte-identity
- [ ] AC-18: raw-presence pins green; 116's AC5 set and `panel tally`
      clean renders unmodified; review panel confirms no existing test
      expectations touched outside the new suites
- [ ] AC-19: git leg + bd-wrapper leg both fatal pre-spawn on hostile
      operands; no false-fatal on non-id invocations
- [ ] AC-20/AC-21: fixtures green; `go test ./internal/redact -run
      'Golden|Mutation'` stays green
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-15 — R4 terminal sinks escaped (complete/next/CLI/approve/bead)
- [ ] AC-16 — R4 transcript sinks escaped (instruct/hook/contextpack;
      fenced payloads byte-identical)
- [ ] AC-17 — executor message bodies per-line + latent panel sinks
- [ ] AC-18 — no over-escaping: validated values raw byte-identically;
      class fences pinned
- [ ] AC-19 — R7 git option-injection + bd-wrapper id gates, fail-fast
      pre-spawn
- [ ] AC-20 — R8 `gate_authority` quoting + `expandSlots` mutant killed
- [ ] AC-21 — R9 scrub class-completeness fixtures + accepted residuals
      pinned
- [ ] AC-24 — `idrender` forced-safe contract + state-line sink +
      empty-sentinel discipline

**Domain:** core (`internal/idvalidate/idrender`, `internal/config` R8,
`internal/redact` R9) + workflow (`internal/complete`, `internal/next`,
`internal/instruct`, `internal/hook`, `internal/approve`,
`internal/panel`, `cmd/mindspec`) + execution (`internal/executor`
message bodies, `internal/bead` renders, `internal/harness` R7) +
context-system (`internal/contextpack`) — per the spec's Impacted Domains
assignments.

**Depends on**: Bead 1 ONLY (`idrender`'s identity leg requires the
CORRECTED grammar — under the old patterns every dotted-child bead ID
would forcibly quote, breaking byte-identity repo-wide; the harness
in-wrapper/caller-site gates likewise call the corrected
`idvalidate.BeadID`). Independent of Beads 2 and 3 (runs beside Bead 2
in W2); Bead 4 depends on THIS bead (its (g)/(h) scans consume the
`idrender` routing and the harness gates). See the preamble adjacency
flags for the shared files with Beads 2/3.

## Provenance

Every spec AC maps to the bead and verification that satisfies it. AC-22
is a plan-time + multi-bead composite (the ADR file is authored WITH this
plan; the citation legs land in Beads 1–3; the ownership claim and gate
proof land in Bead 4).

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (grammar + live-inventory fixture) | Bead 1 Steps 1–4 | Bead 1 verification: `TestIDValidateAcceptsLiveInventory`, `TestIDValidateWideningPreservesRejections` |
| AC-2 (waist unit) | Bead 2 Step 1 | Bead 2 verification: `TestCompositionHelpersRejectInvalidIDs` |
| AC-3 (D1 unit) | Bead 2 Step 2 | Bead 2 verification: `TestSpecIDFromMetadataRejectsInvalidSlug` |
| AC-4 (malformed-lineage convergence) | Bead 2 Step 2 | Bead 2 verification: `TestCompleteRunMalformedLineageRefusesConvergently` (lever applied, re-run passes) |
| AC-5 (D2) | Bead 2 Step 3 | Bead 2 verification: `TestDetectWorktreeContextRejectsMalformedNames` + `TestGuardStateIgnoresMalformedWorktreeDirs` |
| AC-6 (beadID ingress) | Bead 2 Step 10 | Bead 2 verification: `TestCompleteRunRejectsInvalidBeadIDArg`, `TestNextClaimRejectsMalformedBeadID`, epic-ID-embed subtest |
| AC-7 (specID ingress) | Bead 2 Step 9 | Bead 2 verification: `TestResolveSpecPrefixValidatesResult`, `TestImplApproveRejectsInvalidSpecIDArg` |
| AC-8 (repair lever) | Bead 2 Step 11 | Bead 2 verification: `TestRepairSpecTitle` |
| AC-9 (ambient degrade) | Bead 2 Step 2 | Bead 2 verification: `TestDiscoverActiveSpecsSkipsMalformedEpic` |
| AC-10 (R5 predicate + containment) | Bead 3 Steps 1–2 | Bead 3 verification: `TestWorktreeRootPredicate` |
| AC-11 (check-at-use, TOCTOU bound) | Bead 3 Step 4 | Bead 3 verification: `TestContainmentCheckAtUse` + set-equality companion |
| AC-12 (shell-safe emit + root-only) | Bead 3 Step 5 | Bead 3 verification: `TestExecutableCdRendersShellSafe` |
| AC-13 (config-lever convergence) | Bead 3 Step 3 | Bead 3 verification: `TestWorktreeRootRejectionRecoveryConverges` |
| AC-14 (R6 ratchet, eight scans) | Bead 4 Steps 1–8 | Bead 4 verification: the eight named lint tests + negative/two-way fixtures |
| AC-15 (R4 terminal sinks) | Bead 5 Step 2 | Bead 5 verification: the seven AC-15 named tests |
| AC-16 (R4 transcript sinks) | Bead 5 Step 2 | Bead 5 verification: instruct/hook/contextpack named tests |
| AC-17 (executor bodies + latent) | Bead 5 Step 2 | Bead 5 verification: `TestConflictFailureBodiesEscapedPerLine`, `TestLatentPanelSinksHostileFieldsEscaped` |
| AC-18 (no over-escaping) | Bead 5 Steps 2–3 | Bead 5 verification: raw-presence pins; 116 AC5 set + `panel tally` unmodified; panel-channel diff check |
| AC-19 (R7 git + bd-wrapper) | Bead 5 Steps 5–6 | Bead 5 verification: `TestHarnessGitExecRejectsOptionLikeRefs`, `TestHarnessBDWrapperRejectsMalformedIDs` |
| AC-20 (R8) | Bead 5 Step 7 | Bead 5 verification: `gate_authority` quoting + `expandSlots` fixtures |
| AC-21 (R9) | Bead 5 Step 8 | Bead 5 verification: redact fixtures + Golden/Mutation suites green |
| AC-22 (ADR + ownership) | Plan-time (ADR-0042 authored Accepted with this plan) + Bead 1 Step 2 + Bead 2 Steps 1, 11 + Bead 3 Step 1 (doc-comment citations) + Bead 4 Step 9 (ownership claim) | Bead 4 verification: OWNERSHIP.yaml grep + ADR-divergence gate pass; Beads 1–3 citation checks |
| AC-23 (reverse gates + round-5 consumers) | Bead 2 Steps 5–6 | Bead 2 verification: the AC-23 suite (`TestFinalizeOrphansSkipsMalformedBranch`, `TestRouteReviewSlugIgnoresMalformedSpecDirs`, `TestCompleteBeadRejectsMalformedSpecBranch`, enumeration-skip subtests, `TestResolveModeHostileTitle`, `TestPanelSpecRejectsTraversal`, `TestRecordingWriteGates`) |
| AC-24 (idrender forced-safe) | Bead 5 Step 1 | Bead 5 verification: `TestIDRenderForcedSafe` incl. state-line sink + empty-sentinel subtests |
| AC-25 (bead_ids read-gate) | Bead 2 Step 7 | Bead 2 verification: `TestReadPlanBeadIDsRejectsMalformed` |
| AC-26 (bead consumer boundary + epicID gates) | Bead 2 Steps 7–8 | Bead 2 verification: `TestBeadIDArgvConsumerGate`, `TestValidatePlanMalformedBeadIDsZeroBD`, `TestBDListParentEpicIDGate`, `TestFetchChildrenEpicIDGate` |
| AC-27 (gate-all-ids, hostile store id) | Bead 2 Step 8 (production consumers) + Bead 5 Step 6 (harness leg) | Bead 2 verification: `TestHostileBDStoreIDNeverReachesArgv` + companion legs; Bead 5 verification: the harness `ClaimBead`/`CreateBead --parent` fail-fast legs of `TestHarnessBDWrapperRejectsMalformedIDs` |
| AC-global (build/test/lint/validate) | All beads | Every bead's final verification line; re-run at spec end with `mindspec validate spec 120-trust-boundary-render-audit` |
