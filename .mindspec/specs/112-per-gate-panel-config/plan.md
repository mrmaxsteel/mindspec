---
adr_citations:
    - ADR-0040
    - ADR-0037
    - ADR-0035
    - ADR-0034
    - ADR-0039
approved_at: "2026-07-08T10:15:08Z"
approved_by: user
bead_ids:
    - mindspec-lma4.1
    - mindspec-lma4.2
    - mindspec-lma4.3
spec_id: 112-per-gate-panel-config
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/config/config.go
        - internal/config/config_test.go
        - cmd/mindspec/config.go
        - .mindspec/domains/core/interfaces.md
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/panel/panel.go
        - internal/panel/panel_test.go
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
        - .mindspec/domains/workflow/architecture.md
    - depends_on:
        - 1
        - 2
      id: 3
      key_file_paths:
        - cmd/mindspec/config.go
        - cmd/mindspec/config_test.go
        - internal/complete/complete.go
        - internal/complete/panel_advisory.go
        - internal/complete/panel_advisory_test.go
        - .mindspec/domains/workflow/interfaces.md
        - .mindspec/domains/workflow/architecture.md
---
# Plan: 112-per-gate-panel-config

## ADR Fitness

The impacted domains are **core** (`internal/config`) and **workflow**
(`internal/panel`, `internal/complete`, `cmd/mindspec`). All six ADRs the spec's
Touchpoints section names were evaluated; five genuinely constrain the
implementation and are cited, one is architectural context and is deliberately
omitted from `adr_citations`. Each remains the best design for this work —
**no divergence from any accepted ADR was found or is planned**:

- **ADR-0040 — Orchestration Layering Ratchet** (Domain(s): core, workflow;
  covers both impacted domains). The license for this spec and the load-bearing
  constraint on Bead 1: the per-gate mixes, lenses, and substitution policy
  ratchet from L4 (skill prose + operator memory) down to L2 declared config —
  "move it to L2 the moment it is a checkable fact about a run". Its
  portability principle drives two hard rules this plan enforces: model ids are
  **open-vocabulary** strings (structural validation only, never
  name-membership — Bead 1's known-model list is advisory-only by construction
  and is never consulted by `Load`), and 112 declares-and-surfaces without
  consuming (no dispatch, no spawning — that is spec 111's layer). Its §3
  anti-pattern (`internal/harness.Agent` — surface ahead of need) is why
  substitution stays global and one-step. This plan **adheres**; ADR-0040
  remains the correct frame, and unlike 109's plan it is citable here because
  109 landed it on `main` before this branch was rebased.
- **ADR-0037 — Panel Gate as Enforced Contract** (Domain(s): workflow,
  execution; covers workflow). The constraint this spec must not weaken,
  binding Beads 2–3: §3's threshold single home
  (`internal/panel.Panel.ApproveThreshold`, as extended by 109's amendment)
  and "identical decision over identical facts". This plan **adheres**: the
  config-side gate-scoped threshold resolver returns the **raw** expression
  (no second interpreter, Bead 1); the one `internal/panel` change is a
  recorded, **decision-inert** `gate` field that `PanelGateDecision` and
  `ApproveThreshold()` never read (Bead 2); the `threshold > 0` third-defense
  guard at `internal/panel/gate.go:257` survives verbatim; §§3/6/8 are
  untouched. Bead 2 appends an honest second **amendment note** recording the
  field as metadata — an addition-of-recorded-metadata note, not a supersede
  and not a rule change, so no `--supersedes` flow triggers.
- **ADR-0035 — Agent Error Contract** (Domain(s): workflow, execution, core;
  covers both impacted domains). Constrains Bead 1's new `Load` refusals (all
  eight R4 classes) and Bead 3's `config show --gate` unknown-gate error: every
  failure path is guard-style, carries a recovery line, and never panics. The
  unknown-gate recovery lines (both `Load`'s and the CLI's) enumerate the five
  valid keys. Sound as-is.
- **ADR-0034 — Ceremony Collapse** (Domain(s): workflow; covers workflow).
  Fixes the lifecycle-gate vocabulary that the `gates:` enum keys deliberately
  **relate to but do not copy** (the spec's resolved gate-naming Open
  Question: `bead`/`final_review` name review events, `loop.gate_authority`'s
  `bead_merge`/`impl_approve` name approval acts). It constrains Bead 1's
  unknown-gate-key recovery line, which must disambiguate the two
  vocabularies. Sound as-is.
- **ADR-0039 — Flat `.mindspec/` Layout v2** (Domain(s): core, workflow,
  execution, context-system; covers both impacted domains). `config.yaml`, the
  spec-scoped `reviews/` panel dirs, and the ad-hoc `.mindspec/reviews/`
  routing (the practice that motivates the `adhoc` gate key) all live under
  this layout; Bead 3's `config show` scanning keeps using the existing
  layout-aware roots unchanged. Sound as-is.
- **ADR-0023 — Beads as Single State Authority** (Domain(s): workflow, git,
  state) — **evaluated, deliberately NOT cited** (mirroring 109's plan, which
  kept `adr_citations` load-bearing by omitting context-only ADRs). It frames
  this spec — forward-only lifecycle is why 112 extends rather than re-opens
  approved 109, and config stays *policy* while the recorded `panel.json`
  stays the per-round *fact record* — but it imposes no implementation
  constraint on any bead beyond what ADR-0037 already pins (the recorded
  panel as sole gate authority). Sound as-is; no divergence.

**Coverage check.** core is covered by ADR-0040, ADR-0035, and ADR-0039;
workflow by all five cited. Every citation's `Domain(s)` line intersects the
impacted set {core, workflow}, so none is architecturally irrelevant.

**Divergence report: none.** No bead in this plan is better served by a design
that departs from any accepted ADR; in particular the two designs a reviewer
might probe — a config-side threshold integer (rejected: breaks ADR-0037 §3's
single home; the resolvers return raw expressions) and enum-validated model
names (rejected: breaks ADR-0040's portability principle and the spec's
Fable-day precedent) — are both refused by the spec itself.

## Testing Strategy

**Approach.** Pure unit tests, table-driven, added to each touched package's
existing `_test.go` file (`internal/config/config_test.go`,
`internal/panel/panel_test.go`, `internal/complete/panel_advisory_test.go`,
`cmd/mindspec/config_test.go`). No integration or e2e layer is needed: the
work is parse / default / validate / resolve / render logic with no new I/O
surface. Bead 3 keeps `renderConfig` pure over `*config.Config` and adds two
more testable pure functions (`renderGateResolved`, `gateResolvedJSON`) so
`config show --gate [--json]` is exercised without spawning a process; the
`--json` document is produced by `encoding/json` over a typed struct (spec R8
forbids string-concatenation JSON), which also pins sorted-map-key output for
`substitutes` by construction.

**Per-test proof discipline.** Every new test is verified with an anchored
PASS-line grep so a reviewer sees the specific test pass, not a bare package
green: `go test ./PKG -v -run 'TestName$' | grep -q -- '--- PASS: TestName'`.
The `$` anchor on `-run` prevents a prefix from sweeping in a sibling test.
The eight named test functions use exactly the spec's AC names. **Grep note:**
this machine's `grep` is ugrep; any verification needing GNU semantics (`\b`,
word boundaries) is written against `/usr/bin/grep` explicitly — plain
fixed-string `grep -q` commands are ugrep-safe as written.

**Leaf invariant.** `internal/panel` must stay import-clean of
`internal/config` through the whole spec: verified in Bead 2 (the only bead
that touches the package) and re-asserted in Bead 3 (the last bead to land)
with `go list -deps ./internal/panel | /usr/bin/grep -q internal/config`
returning **non-zero**. Gate-appropriate config defaults reach
`panel.ReviewerCountNote` only as plain `int`s resolved by the callers.

**Backward-compatibility pins.** Bead 1's `TestLoad_GatesAbsentByteIdentical109`
is the R2 identity test: zero-config defaults equal 109's exactly, the 109 AC7
populated fixture loads unchanged, `gates: {}` ≡ absent everywhere (every
"gates is configured" predicate is `len(gates) > 0`, never key-presence), and
the one deliberate monotone relaxation (count-less reviewer entry → `count: 1`,
which 109 refuses) is asserted as *loading*, not as changing any 109-accepted
input's meaning.

**Regression.** The full suite `go test ./...` runs once at **plan time** and
again **pre-`/ms-impl-approve`** — not per bead; per-bead gates run the
touched packages only. Plan-time result (2026-07-08, this worktree): every
package green **except** the pre-existing `internal/instruct`
`TestRun_IdleNoBeads` environment-isolation failure tracked as `z4ps` (the
test expects the idle "No Active Work" template but the surrounding
environment's ready beads leak into `bd` discovery; it fails even in
isolation on a dev machine with ready work). It is unrelated to this spec —
no bead here touches `internal/instruct` — and is excluded from this spec's
gates; the four touched packages (`internal/config`, `internal/panel`,
`internal/complete`, `cmd/mindspec`) are all green at plan time. This spec
introduces no new shared-state test. Git-touching tests run with
`GIT_TERMINAL_PROMPT=0`. No new external dependency, no network access.

**Dependency shape (decomposition).** Three beads — under the 3–5 target, with
the shallowest DAG the spec's compile-time facts allow. Bead 1
(`internal/config` plus the one-line `cmd/mindspec` count-render deref its
pointerization forces, R1–R5) and Bead 2 (`internal/panel` + the ADR-0037
amendment, R6) are package- and file-disjoint with no produced-then-consumed
state between them: they run in **parallel**. Bead 3 (`internal/complete` +
`cmd/mindspec`, R7–R9) depends on **both**, and both edges are real
compile-time dependencies, not ordering wishes: its advisory callers and
render/JSON functions call Bead 1's new resolvers
(`PanelGateExpectedReviewers`/`PanelGateReviewerSlots`/
`PanelGateApproveThresholdExpr`/`PanelGateAdvisoryDefault`) and read Bead 2's
new `panel.Panel.Gate` field. Longest chain is 2 (< the 3 advisory cap); bead
count 3 (≤ 6); two of three beads are roots (parallelism 0.67, well above the
0.25 floor). Applied heuristics: the ADR-0037 amendment note is doc-only and
tiny, so it **folds into Bead 2** (trivial-work test) — it also documents
exactly the field that bead adds, so the commit is self-carrying; R1–R5 stay
**one** bead because they are a single schema+validation+resolution unit — the
R4 cross-field inheritance check *executes* the R3 per-field resolution chain,
so splitting schema from resolvers would add a produced-then-consumed edge,
deepen the chain to 3, and buy no parallelism; R7 (two call sites) is too
small to stand alone and shares both parents with R8/R9, so the three
surfacing requirements land together as the one consumer bead, exactly as
109's Bead 4 grouped the same two caller packages.

**Requirement → bead map.** R1–R5 → Bead 1 (R5's supersession *reporting*
half — the `substitution.in_force` member — surfaces in Bead 3; Bead 1 lands
its schema + validation half); R6 → Bead 2; R7–R9 → Bead 3.
Every spec requirement is delivered; the Provenance table below maps every
spec acceptance criterion.

## Bead 1: internal/config — gates map, generalized reviewers, substitutes, per-gate refusals, gate-scoped resolvers + deterministic slot expansion, known-model advisory list; + the one-line cmd/mindspec count-render deref

Delivers R1–R5. The primary source edits are the **core** domain
(`internal/config/config.go` + its test file); pointerizing `Reviewer.Count`
additionally forces one mechanical **workflow**-domain edit,
`cmd/mindspec/config.go` — the sole out-of-package `Reviewer.Count` reader
(step 1's second half) — so the branch stays green between beads. Doc-sync is
`.mindspec/domains/core/interfaces.md` (core) plus a one-line note in
`.mindspec/domains/workflow/interfaces.md` (the per-domain doc-sync gate
blocks a `cmd/**` source edit — workflow-owned per
`.mindspec/domains/workflow/OWNERSHIP.yaml` — with no workflow doc in the
same range; Bead 2 runs in parallel but touches only `architecture.md`, so
the two root beads stay file-disjoint).

**Steps**
1. Generalize the `Reviewer` entry (R1): add `Model string \`yaml:"model"\``
   and `Lens string \`yaml:"lens"\`` (both open-vocabulary; no membership
   validation anywhere in `Load`), and change `Count int` to
   `Count *int \`yaml:"count"\`` so an **absent** count (nil → defaults to 1,
   the R2 monotone relaxation) is distinguishable from an **explicit**
   `count: 0`/negative (refused, R4). Add two value accessors: **exported**
   `(Reviewer).CountValue() int` (nil → 1; exported because `cmd/mindspec`'s
   renderer is an out-of-package consumer — the deref below — and Go forbids
   a `Count()` method beside the `Count` field) and unexported
   `(Reviewer).model() string`
   (`Model` if non-empty, else `Family` — "model wins for slot expansion");
   every consumer (validation, resolvers, expansion, and the `cmd/mindspec`
   renderer) resolves through these two accessors so the legacy-`family` and
   both-keys cases are decided in exactly one place. Update `DefaultConfig()` (pointer counts via a tiny
   `intp(n int) *int` helper) and the existing global
   `PanelExpectedReviewers()` / `validateOrchestration` reviewer arithmetic to
   the accessors — semantics for every 109-accepted input unchanged (3+3 → 6).
   Same step, the pointerization's out-of-package half — keep `cmd/mindspec`
   green: update the sole external `Reviewer.Count` reader,
   `cmd/mindspec/config.go:149`
   (`fmt.Fprintf(&b, "      count: %d\n", r.Count)` in `renderConfig`'s
   panel-reviewers loop), to render `r.CountValue()`. This edit is
   load-bearing, not cosmetic: `%d` on the new `*int` passes `go build`
   AND `go vet` but prints the pointer address, breaking the existing 109
   renderer test `TestConfigShow_EmitsPanelModelsLoop` — without it
   the branch is red between Bead 1 and Bead 3 while Bead 1's
   package-scoped gate stays false-green. Scope fence: config.go:149 is the
   verified ONLY external `Reviewer.Count` consumer
   (`cmd/mindspec/report.go`'s `r.Count` is an unrelated journal-report
   struct — do not touch it; `internal/config`'s own uses are this step's).
   Because `cmd/**` is workflow-owned, this edit rides with the one-line
   workflow doc note in step 7 to satisfy the per-domain doc-sync gate.
2. Add the new schema (R1, R5) and its inert companions: `GatePanel` struct
   (`Reviewers []Reviewer \`yaml:"reviewers"\``,
   `ApproveThreshold string \`yaml:"approve_threshold"\``);
   `Panel.Gates map[string]GatePanel \`yaml:"gates"\``;
   `Panel.Note string \`yaml:"note"\`` (inert advisory free-text: parsed,
   echoed by `config show` in Bead 3, never read by any validation or
   resolver); `Substitution.Substitutes map[string]string \`yaml:"substitutes"\``.
   Declare the gate-key enum **once** as an ordered **exported**
   package-level slice,
   `PanelGateKeys = []string{"spec_approve", "plan_approve", "bead",
   "final_review", "adhoc"}` — exported because Bead 3's consumers cannot
   reference an unexported name (`cmd/mindspec` is package `main`,
   `internal/complete` is another package); it is the single source for
   validation, recovery lines, and Bead 3's enum-order rendering. `DefaultConfig()` leaves `Gates`,
   `Substitutes`, and `Note` empty (R2: the standing protocol is the
   documented example, never the default), and `loadUncached`'s backfill adds
   **no** backfill for them (absent → empty → global behavior). Pin the R2
   configured-predicate rule: every "gates is configured" branch in this
   package (and, via the `PanelGateAdvisoryDefault` helper below, in the
   Bead 3 callers) keys off `len(cfg.Panel.Gates) > 0`, never key-presence —
   `gates: {}` is everywhere equivalent to an absent `gates:`. Also home the
   curated known-model advisory list here (R8's input, versioned with the
   schema): an exported `KnownModels() []string` (or equivalent exported
   slice) seeded with exactly `claude-fable-5`, `claude-opus-4-8`,
   `claude-sonnet-5`, `gpt-5.5`, `claude`, `codex` — non-exhaustive by
   design, consumed only by Bead 3's `config show` annotation, referenced
   **nowhere** in `Load`/`validateOrchestration` (R1; ADR-0040).
3. Add the three gate-scoped resolvers with deterministic slot expansion
   (R3), each returning an error for a gate name outside `PanelGateKeys`
   (fail loud on caller typos; recovery line enumerates the five keys):
   `(*Config).PanelGateExpectedReviewers(gate string) (int, error)`,
   `(*Config).PanelGateApproveThresholdExpr(gate string) (string, error)` —
   the **raw** expression, never a resolved integer (resolution stays
   single-homed in `internal/panel.Panel.ApproveThreshold`, ADR-0037 §3) —
   and `(*Config).PanelGateReviewerSlots(gate string) ([]ReviewerSlot, error)`
   with `ReviewerSlot{Slot, Model, Lens string}`. The resolution chain is
   walked **per field** (reviewers and threshold independently): the gate's
   own configured value (`len(Reviewers) > 0` / non-blank `ApproveThreshold`)
   → for `adhoc` only, `bead`'s **resolved** value → the global list /
   expression → the built-in defaults. `PanelGateExpectedReviewers` returns
   the expanded slot count (sum of `count()` over the resolved list) and must
   equal `len(PanelGateReviewerSlots(gate))` by construction. Expansion is
   deterministic: entries in declaration order, each `count()` expanded, slot
   ids `"R1"…"Rn"`; an entry's explicit `Lens` applies to all its expanded
   slots and those slots do **not** advance the cursor; lens-less slots take
   `defaultLenses[cursor%6]` from the interleaved ordering
   `author-of-record, empirical-prober, codebase-pin, adversarial,
   contract-stability, integration`, with **one global cursor per expansion
   starting at index 0** that advances only over lens-less slots — the worked
   example is normative: a 9-reviewer all-lens-less panel (3 entries × count 3)
   expands to R1 author-of-record, R2 empirical-prober, R3 codebase-pin,
   R4 adversarial, R5 contract-stability, R6 integration, R7 author-of-record,
   R8 empirical-prober, R9 codebase-pin.
4. Add the R7 caller-side selection helper (single home, so Bead 3's two call
   sites cannot drift): `(*Config).PanelGateAdvisoryDefault(recordedGate
   string, isBead bool) (int, bool)` — when `len(Gates) == 0`: return
   `(PanelExpectedReviewers(), true)` (every panel compares against the
   global default exactly as 109 ships it); when gates are configured:
   a `recordedGate` in `PanelGateKeys` → that gate's
   `PanelGateExpectedReviewers` and `true`; empty `recordedGate` with
   `isBead` → the `bead` gate's value and `true`; anything else (non-bead
   with no recorded gate, or any recorded value outside the enum) →
   `(0, false)` — skip the note, and **never** call an R3 resolver with the
   unknown value (R7: no resolver error can surface through the advisory).
5. Extend `validateOrchestration` with the R4 refusals, each a guard-style
   error with an ADR-0035 recovery line, never a panic:
   (a) a `gates` key outside `PanelGateKeys` — recovery line enumerates all
   five valid keys AND disambiguates from `loop.gate_authority`'s different
   vocabulary (`bead_merge`/`impl_approve`); (b) a configured gate entry
   setting neither `reviewers` nor `approve_threshold` (both per-field "set"
   predicates false — a silent no-op is a likely indentation mistake); (c) a
   reviewer entry (global or per-gate) with `model() == ""` (neither `model`
   nor `family`); (d) an explicit non-positive count (`Count != nil &&
   *Count < 1`); (e) a configured per-gate reviewers list whose expanded sum
   is `< 2` (the per-gate no-always-pass floor); (f)+(g) the per-gate
   threshold bound as one resolved check: for **every configured gate**,
   resolve its threshold expression and reviewer sum through the R3 per-field
   chain (own → `bead` donor for `adhoc` → global) and refuse when the
   resolved expression is neither `"n-1"` nor an integer in
   `[1, that gate's resolved sum]` — this single formulation covers both an
   own out-of-range integer and the cross-field inheritance failures (a
   global `"5"` inherited by a smaller gate; a `bead` integer inherited
   through the adhoc→bead chain by a smaller reviewers-only `adhoc`) at every
   chain link, because the check always runs against the *inheriting* gate's
   resolved sum; (h) a `substitutes` entry with an empty key or value, or
   key == value (one-step resolution; a mutual pair `A→B, B→A` stays legal).
   **No model/lens name-membership check exists on any `Load` path** (R1;
   ADR-0040), and no size maxima beyond these refusals (deliberate spec
   deferral to `naq0`/`pev1`).
6. Add the tests to `internal/config/config_test.go`:
   `TestLoad_GatesAbsentByteIdentical109` (zero-config defaults equal 109's
   exactly — 3+3 family reviewers, `"n-1"`, `claude_sub_on_quota=true`, empty
   `gates`/`substitutes`; the 109 AC7 populated fixture loads with every field
   unchanged; gates-absent → every gate-scoped resolver returns the
   global-derived values and the 109 resolvers are unchanged; `gates: {}` ≡
   absent everywhere; a count-less reviewer entry loads as `count: 1`);
   `TestLoad_PerGateProtocolRoundTrips` (the spec Goal's full standing-protocol
   YAML — four gates, mixed compact/exploded entries, explicit lenses,
   `substitutes`, `note` — embedded as a fixture, round-trips through `Load`
   with every field equal to what was written; two loads differing only in
   `panel.note` yield identical validation results and identical outputs from
   every resolver); `TestLoad_RefusesPerGateKnobs` (every R4 refusal (a)–(h)
   errors with a recovery line — including the two named inheritance shapes:
   global integer inherited by a differently-sized gate, and a `bead` integer
   inherited through the adhoc→bead chain by a smaller reviewers-only `adhoc`
   — the unknown-gate-key recovery line names all five valid keys, and an
   unknown model id alone does NOT error);
   `TestPanelGateResolvers_FallbackAndAdhoc` (configured gate → own values;
   threshold-only and reviewers-only entries inherit the missing half per
   field, including through `adhoc`'s chain in both directions; unconfigured
   gate → global; unconfigured `adhoc` → `bead`'s resolved mix; unknown gate
   name → error; threshold resolvers return raw expressions only);
   `TestPanelGateSlots_DeterministicExpansion` (slots `R1…Rn` in declaration
   order with counts expanded; explicit lenses preserved and not
   cursor-consuming; cursor starts at index 0 — first lens-less slot asserted
   `author-of-record`; the 9-reviewer worked example expands to exactly the
   pinned assignment; a gate with ≥ 6 lens-less reviewers covers all six
   default lenses and every lens-less `count ≥ 2` entry spans a structural and
   a sharp lens; two loads of identical config yield identical slot lists;
   fixture ordering is normative for falsifiability: at least one
   explicitly-lensed entry must PRECEDE lens-less entries at a position
   where slot-index mod 6 diverges from the cursor value, so a wrong
   `lens[slot-index % 6]` implementation fails instead of passing every
   listed assertion);
   plus `TestPanelGateAdvisoryDefault_SelectionRule` (the step-4 helper:
   known-gate / bead-fallback / skip / gates-absent-global rows — the config
   half of R7, ahead of Bead 3's caller-side AC7 test).
7. Doc-sync (core + the step-1 workflow ride-along): document in
   `.mindspec/domains/core/interfaces.md` the
    `panel.gates`/`note`/`substitutes` schema, the generalized reviewer entry
    (open vocabulary, count default 1, legacy `family`), the R4 refusal
    surface, the three gate-scoped resolvers + `PanelGateAdvisoryDefault`,
    the deterministic slot-expansion contract (interleaved default-lens
    ordering, cursor-start-0, the normative worked example), the known-model
    advisory list, and the operator's standing-protocol YAML from the spec's
    Goal **reproduced as the documented example** (R2). Also (workflow —
    required by the step-1 `cmd/**` edit): add a one-line note to
    `.mindspec/domains/workflow/interfaces.md` recording that `config show`
    renders reviewer counts through the exported `CountValue()` accessor
    (an absent `count` renders as its default, `1`).

**Verification**
- [ ] `go test ./internal/config -v -run 'TestLoad_GatesAbsentByteIdentical109$' | grep -q -- '--- PASS: TestLoad_GatesAbsentByteIdentical109'`
- [ ] `go test ./internal/config -v -run 'TestLoad_PerGateProtocolRoundTrips$' | grep -q -- '--- PASS: TestLoad_PerGateProtocolRoundTrips'`
- [ ] `go test ./internal/config -v -run 'TestLoad_RefusesPerGateKnobs$' | grep -q -- '--- PASS: TestLoad_RefusesPerGateKnobs'`
- [ ] `go test ./internal/config -v -run 'TestPanelGateResolvers_FallbackAndAdhoc$' | grep -q -- '--- PASS: TestPanelGateResolvers_FallbackAndAdhoc'`
- [ ] `go test ./internal/config -v -run 'TestPanelGateSlots_DeterministicExpansion$' | grep -q -- '--- PASS: TestPanelGateSlots_DeterministicExpansion'` (per step 6, the fixture places an explicitly-lensed entry BEFORE lens-less ones where slot-index mod 6 ≠ the cursor value — a wrong `lens[slot-index % 6]` impl must not pass)
- [ ] `go test ./internal/config -v -run 'TestPanelGateAdvisoryDefault_SelectionRule$' | grep -q -- '--- PASS: TestPanelGateAdvisoryDefault_SelectionRule'`
- [ ] `go test ./internal/config` exits `0` (whole package green, existing 109 tests included — the R2 identity claim over 109's own test surface)
- [ ] `go test ./cmd/mindspec` exits `0` (the cross-package pointerization gate: `TestConfigShow_EmitsPanelModelsLoop` still passes — `%d` on the new `*int` passes `go build` AND `go vet`, so only this test run can falsify a skipped step-1 cmd deref)
- [ ] `/usr/bin/grep -qF 'fable-window 2026-07, codex-enabled' .mindspec/domains/core/interfaces.md && /usr/bin/grep -q 'claude-fable-5' .mindspec/domains/core/interfaces.md` exits `0` (the `note` line is unique to the Goal's standing-protocol YAML — the known-model seed list alone cannot satisfy it — and the seed ids are documented)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/core/interfaces.md' && git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync: the core edits carry the core domain-doc; the workflow-owned `cmd/mindspec` edit carries the one-line workflow note)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] Zero-config and 109-era-fixture loads are identical to 109's in every field; `gates: {}` ≡ absent; count-less entry → `count: 1`; `DefaultConfig` has empty `gates`/`substitutes` (spec AC1, R2)
- [ ] The Goal's standing-protocol YAML round-trips unchanged; `note` is validation- and resolver-inert (spec AC2, R1)
- [ ] Every R4 refusal errors with a recovery line (unknown gate key naming all five valid keys; empty gate entry; model-less entry; explicit `count <= 0`; per-gate sum `< 2`; per-gate/inherited threshold out of the inheriting gate's resolved range at every adhoc→bead→global link; self-mapping/empty-sided `substitutes`), and an unknown model id alone never errors (spec AC3, R4)
- [ ] Gate-scoped resolvers walk the per-field chain (own → `bead` for `adhoc` → global → defaults), error on unknown gate names, and return raw threshold expressions only (spec AC4, R3)
- [ ] Slot expansion is deterministic with the cursor starting at index 0 and the 9-reviewer worked example exactly as pinned; explicit lenses never consume the cursor (spec AC5, R3)
- [ ] `cmd/mindspec` renders reviewer counts through the exported `CountValue()` accessor and its package tests pass inside this bead's own gate — no cross-bead red window, no `%d`-on-pointer false-green (round-1 panel must-fix 1)
- [ ] `substitutes` loads as a well-formed one-step map; `claude_sub_on_quota` keeps its 109 meaning while `substitutes` is empty (R5 — supersession *reporting* is Bead 3's surface)

**Depends on**
None

## Bead 2: internal/panel — recorded decision-inert `gate` field + ADR-0037 second amendment note

Delivers R6. The `internal/panel` edit is the **workflow** domain; doc-sync is
`.mindspec/domains/workflow/architecture.md`. The ADR amendment is doc-only
(`.mindspec/adr/**` carries itself) and folds in here because it records
exactly the field this bead adds.

**Steps**
1. Add one optional field to `panel.Panel`:
   `Gate string \`json:"gate,omitempty"\`` — recording which gate mix the
   panel was created from. Doc comment pins the R6/R9 contract: values drawn
   from the five-key enum (`spec_approve`/`plan_approve`/`bead`/
   `final_review`/`adhoc`) **by convention but parse-lenient** like
   `AbandonReason` — an unexpected value is surfaced by consumers, never a
   parse error (flagging it malformed would set `Registration.Err` and tip
   the gate toward fail-open, the wrong direction); **decision-inert** —
   `PanelGateDecision` and `ApproveThreshold()` do not read it; stamped by
   the spec-110 writer (until then `ms-panel-run` step 0 may hand-write it),
   and absence costs nothing; name (`gate`), type (string), `omitempty`, and
   parse-lenience are a **stable contract** no follow-up may change silently
   (R9). This is the **only** `internal/panel` edit — no other function,
   type, or import changes; the package still imports no `internal/config`.
2. Add `TestPanel_GateFieldDecisionInert` to `internal/panel/panel_test.go`:
   over fixed facts, `PanelGateDecision` and `ApproveThreshold()` return
   identical results with the `gate` field absent, present-and-known
   (`"bead"`), and present-and-unexpected (e.g. `"weird"`); a `panel.json`
   fixture carrying an unexpected `gate` value loads with
   `Registration.Err == nil`; a gate-less legacy `panel.json` parses into a
   struct identical (field-for-field) to today's, taking byte-identical code
   paths.
3. Append the second **amendment note** to
   `.mindspec/adr/ADR-0037-panel-gate-enforced-contract.md`, dated with the
   bead-commit date and labeled `spec 112`, homed under **§1 ("Registration:
   `panel.json` is the panel's identity" — the schema block)**, where the
   schema-addition precedents live (`abandon_reason` and the 099/102/106
   notes sit with the thing they record) — NOT under §3: the 2026-07-07
   spec-109 note sits in §3 only because it extended the threshold rule,
   which the decision-inert `gate` field does not. The note's content:
   `panel.json` gains one optional recorded field, `gate` —
   **decision-inert metadata** in exactly the sense `abandon_reason` is
   (recorded intent for advisory consumers, parse-lenient, never an input to
   `PanelGateDecision` or `ApproveThreshold()`); §§3/6/8 are **untouched**
   (not an extension of any rule — the threshold single home, fail-open/
   closed, and the trust boundary are unchanged by recorded metadata).
4. Doc-sync (workflow): note in
   `.mindspec/domains/workflow/architecture.md` the `panel.json` schema
   addition (the `gate` field's convention/lenience/inertness) and reaffirm
   the config-free-leaf invariant.

**Verification**
- [ ] `go test ./internal/panel -v -run 'TestPanel_GateFieldDecisionInert$' | grep -q -- '--- PASS: TestPanel_GateFieldDecisionInert'`
- [ ] `go test ./internal/panel` exits `0` (whole package green, existing tests included)
- [ ] `go list -deps ./internal/panel | /usr/bin/grep -q internal/config` returns **non-zero** (leaf invariant: `internal/panel` imports no `internal/config` — spec AC11 first half)
- [ ] `/usr/bin/grep -q 'threshold > 0' internal/panel/gate.go` exits `0` (the third-defense gate guard survives — spec AC11 second half)
- [ ] `/usr/bin/grep -q 'spec 112' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md && /usr/bin/grep -q 'decision-inert' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` exits `0` and `/usr/bin/grep -c -i 'amendment' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` reports a count higher than on the parent commit (the new §1 note is present and 109's §3 note untouched — spec Validation Proof)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] The `gate` field's presence, absence, or value changes no `Allow`/`Block` and no threshold; an unexpected value never sets `Registration.Err`; a gate-less legacy `panel.json` parses and decides identically to before (spec AC6, R6)
- [ ] `internal/panel` remains import-clean of `internal/config` and the `threshold > 0` guard is intact (spec AC11)
- [ ] The ADR-0037 second amendment note recording `gate` as decision-inert metadata is present under §1's schema block, with §§3/6/8 untouched (R6; spec Validation Proof)

**Depends on**
None

## Bead 3: internal/complete + cmd/mindspec — gate-aware advisory, config show gates/substitutes/known-model/escaping, `--gate [--json]`, and the R9 stable contract

Delivers R7–R9. Both touched packages (`internal/complete`, `cmd/mindspec`)
are the **workflow** domain; doc-sync is
`.mindspec/domains/workflow/interfaces.md` (CLI surface + the R9 contract) and
`.mindspec/domains/workflow/architecture.md` (the gate-aware advisory rule).
Depends on Bead 1 (calls the new resolvers — compile-time) and Bead 2 (reads
`panel.Panel.Gate` — compile-time). **Scope fence (R9):** no `panel.json`
writer behavior (spec 110), no runner/dispatch or substitution consumption
(spec 111), no change to `PanelGateDecision`/`ApproveThreshold()` semantics.

**Steps**
1. Gate-aware advisory at the complete-gate call site (R7): in
   `internal/complete/complete.go` (the `reviewerCountAdvisory(panelReg,
   gateCfg.PanelExpectedReviewers(), advisoryOut)` site, currently
   `complete.go:384`), replace the flat global default with the Bead 1
   selection helper, **guarded on the registration**: only when
   `panelReg != nil` — `panelGate` returns a nil registration on its
   fail-open paths (empty bead ID, no registered panel: the common
   panel-less `mindspec complete`), and the helper's arguments deref it, so
   an unguarded `panelReg.Panel.Gate` read panics where 109 relied on
   `reviewerCountAdvisory`'s own nil-check — compute `def, ok :=
   gateCfg.PanelGateAdvisoryDefault(panelReg.Panel.Gate,
   panelReg.Panel.IsBead())` and call
   `reviewerCountAdvisory(panelReg, def, advisoryOut)` only when `ok`
   (the R7 skip carve-out prints nothing); a nil registration skips the
   advisory silently, exactly as 109 does. `panel.ReviewerCountNote` and
   `reviewerCountAdvisory`'s own signature stay unchanged; the gate's
   `Allow`/`Block` (already computed by `panelGate` before this point) is
   untouched. Surfaces may still display a raw recorded `gate` value; only
   the count comparison is suppressed.
2. Gate-aware advisory in `config show` (R7): in `cmd/mindspec/config.go`'s
   `reviewerCountNotesFor`, replace the single hoisted
   `cfg.PanelExpectedReviewers()` with the same per-registration helper call
   (`cfg.PanelGateAdvisoryDefault(reg.Panel.Gate, reg.Panel.IsBead())`),
   skipping registrations where `ok` is false. Both call sites now share the
   one selection rule homed in `internal/config` — no drift possible.
3. Extend `renderConfig` (R8), keeping it pure over `*config.Config`: echo a
   set `panel.note` verbatim modulo escaping; render the `gates` map — only
   configured gates, in `PanelGateKeys` **enum declaration order** (never map
   iteration order) — each with its configured entries (`model`/`family`/
   `lens`/`count` as configured), its resolved reviewer sum
   (`PanelGateExpectedReviewers`), and its **raw** threshold expression
   (`PanelGateApproveThresholdExpr`); render `substitutes` in **sorted-key
   order** with a line stating the slot-id-preservation convention (a
   substituted reviewer writes `reviewer_id "<slot> <substitute-model>-sub"`,
   keeping the slot id); keep 109's "declared, not yet enforced" annotations
   on the still-inert `models:`/`loop:`/`runner:` blocks. **Every**
   config-controlled string this spec adds to the text path — `note`,
   reviewer `model`, reviewer `lens`, `substitutes` keys and values, and any
   warning line embedding one — passes through `escapeConfigValue`.
4. Known-model advisory (R8): after rendering, annotate any model id
   appearing in the global reviewers, any gate's reviewers, or either side of
   `substitutes` that is absent from `config.KnownModels()` with a
   warning-style note ("not in the known-model list — fine if intentional"),
   the embedded id escaped. Advisory-only by construction: it never affects
   the exit code and is never consulted by validation — a made-up id warns
   while `Load` and `config show` still exit `0`; the four seeded protocol
   ids and the legacy `claude`/`codex` family strings never warn.
5. Add `--gate <name>` and `--json` flags to `configShowCmd` (R8/R9), backed
   by two **pure, testable** functions in `cmd/mindspec/config.go`:
   `renderGateResolved(cfg *config.Config, gate string) (string, error)`
   (text: expanded slots `slot`/`model`/`lens`, expected reviewer count, raw
   threshold expression, effective substitution policy — all
   config-controlled strings escaped) and
   `gateResolvedJSON(cfg *config.Config, gate string) ([]byte, error)`
   (a typed struct marshaled with `encoding/json` — never string
   concatenation — whose members are **exactly** the R9 contract: `gate`
   (string, the requested gate key), `slots` (array of `{slot, model, lens}`
   in R3 expansion order), `expected_reviewers` (int), `approve_threshold`
   (raw expression string), `substitution` (object: `substitutes` map —
   `encoding/json` emits map keys sorted, pinning the order — the legacy
   `claude_sub_on_quota` bool, and `in_force`, either `"substitutes"` or
   `"claude_sub_on_quota"` per R5's supersession rule: non-empty map ⇒ the
   map is the policy and the bool is inert)). Both functions delegate to the
   R3 resolvers so `--gate` output cannot disagree with them. Errors: a
   `--gate` value outside the five keys exits non-zero with a recovery line
   enumerating them; `--json` without `--gate` exits non-zero with a recovery
   line (the resolved view is per-gate). The command stays read-only: no file
   writes on any path.
6. Add the tests: `TestPanelAdvisory_GateAwareCompare` in
   `internal/complete/panel_advisory_test.go` (using the existing
   `panelScanFn`/`panelTallyFn`/advisory-writer seams): a recorded bead panel
   matching the configured `bead` default (6) but differing from another
   gate's default yields NO note; a genuine mismatch against its own gate's
   default yields the note; with `gates:` configured, a non-bead panel with
   no recorded gate yields no note even when its count differs from every
   default; a panel recording an unknown `gate` value yields no note and
   surfaces no resolver error; a **panel-less complete** (nil registration
   through `panelGate`'s fail-open path) emits no advisory and does not
   panic, with `gates:` configured AND absent — the step-1 nil-guard case;
   with `gates:` absent every panel compares
   against the global default exactly as 109; the gate decision is unchanged
   in all cases. `TestConfigShow_GatesSubstitutesAndModelAdvisory` in
   `cmd/mindspec/config_test.go`: gates rendered in enum order with resolved
   sums + raw exprs, `substitutes` sorted with the slot-id-preservation
   convention, `note` echoed, a made-up model id warns while exit stays `0`,
   and none of the six seeded ids/family strings warns (negative control);
   two renders of one config are byte-identical (deterministic-output pin).
   `TestConfigShow_ReviewerCountNoteGateAware` in
   `cmd/mindspec/config_test.go` — the R7 **cmd-side falsification pin**:
   the existing `TestConfigShow_ReviewerCountNoteWhenPanelDiffers` runs
   gates-ABSENT, where R7 mandates 109-identical behavior, so it passes
   whether or not step 2 is wired. Over a temp root with `gates:` configured
   and registered panels on disk (the full `config show` path, exactly as
   the existing test drives it): a bead panel whose recorded count matches
   the configured `bead` gate's default but differs from the global default
   emits NO note (this case FAILS against unwired 109 code, which compares
   globally); a genuine mismatch against the panel's own recorded gate's
   default emits the note; a gate-less non-bead registration emits nothing.
   `TestConfigShowGate_ResolvedJSON`: `--gate bead --json` emits exactly the
   five R9 members matching the R3 resolvers over the same config; the
   `substitution.in_force` member flips per R5 (non-empty map vs empty);
   unknown `--gate` exits non-zero with the five-key recovery line; nothing
   is written. `TestConfigShow_HostileStringsEscaped`: with `note`, a
   reviewer `model`, a reviewer `lens`, and a `substitutes` key and value
   each carrying ESC/BEL and embedded newlines, `config show` and
   `config show --gate <name>` text output contains no raw control byte and
   no forged extra line (each hostile value appears only in its single-line
   quoted-escape form, including inside the known-model warning), and
   `--gate <name> --json` round-trips every hostile value byte-exactly as a
   JSON string under a real `encoding/json` decode.
7. Doc-sync (workflow): in `.mindspec/domains/workflow/interfaces.md`,
   document the extended `config show` surface and the
   `config show --gate <name> [--json]` view, including the **R9 stable
   contract**: the five `--json` members with names and types as pinned in
   step 5, additive-only evolution (renaming/retyping/removing a documented
   member is a breaking change no follow-up may make silently), and the
   recorded `gate` field's matching guarantee (name/type/`omitempty`/
   parse-lenience fixed). In `.mindspec/domains/workflow/architecture.md`,
   document the gate-aware advisory selection rule (known gate → its default;
   gate-less bead panel → `bead`; otherwise skip iff gates are configured;
   gates absent → global, exactly as 109).

**Verification**
- [ ] `go test ./internal/complete -v -run 'TestPanelAdvisory_GateAwareCompare$' | grep -q -- '--- PASS: TestPanelAdvisory_GateAwareCompare'`
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShow_GatesSubstitutesAndModelAdvisory$' | grep -q -- '--- PASS: TestConfigShow_GatesSubstitutesAndModelAdvisory'`
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShowGate_ResolvedJSON$' | grep -q -- '--- PASS: TestConfigShowGate_ResolvedJSON'`
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShow_HostileStringsEscaped$' | grep -q -- '--- PASS: TestConfigShow_HostileStringsEscaped'`
- [ ] `go test ./cmd/mindspec -v -run 'TestConfigShow_ReviewerCountNoteGateAware$' | grep -q -- '--- PASS: TestConfigShow_ReviewerCountNoteGateAware'` (the gates-configured panel-scan case — the only cmd-side proof that fails if step 2's wiring is skipped)
- [ ] `go test ./cmd/mindspec ./internal/complete` exits `0` (both packages green, existing tests included — proving the complete-gate `Allow`/`Block` is unchanged)
- [ ] `go build ./... && go run ./cmd/mindspec config show >/dev/null && [ -z "$(git status --porcelain)" ]` exits `0` (read-only, even when panels are scanned — spec Validation Proof)
- [ ] Protocol-YAML proof (spec Validation Proofs): `go build -o /tmp/ms112 ./cmd/mindspec && T=$(mktemp -d) && mkdir -p "$T/.mindspec"` then write the spec Goal's standing-protocol YAML (the `TestLoad_PerGateProtocolRoundTrips` fixture) to `$T/.mindspec/config.yaml`; `(cd "$T" && /tmp/ms112 config show)` prints all four configured gates with resolved sums 9/9/6/12 and raw threshold expressions plus the substitutes map, exit `0`; `[ "$(cd "$T" && /tmp/ms112 config show --gate final_review --json | jq '.slots | length')" = "12" ]` exits `0` and `[ "$(cd "$T" && /tmp/ms112 config show --gate final_review --json | jq -r '.approve_threshold')" = "11" ]` exits `0` (jq structural checks, not a `grep -c '"slot"'` count or a spaced-substring grep — the plan pins `encoding/json`, whose compact output would FALSE-FAIL both grep forms on a correct impl); `[ "$(cd "$T" && /tmp/ms112 config show --gate adhoc --json | jq -c .slots)" = "$(cd "$T" && /tmp/ms112 config show --gate bead --json | jq -c .slots)" ]` exits `0` (unconfigured `adhoc` ≡ `bead`'s slots)
- [ ] `/usr/bin/grep -q 'expected_reviewers' .mindspec/domains/workflow/interfaces.md` exits `0` (the R9 `--gate --json` schema is documented in the workflow doc-sync)
- [ ] `go list -deps ./internal/panel | /usr/bin/grep -q internal/config` returns **non-zero** (spec AC11 re-asserted on the last bead to land)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md' && git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] Both `ReviewerCountNote` callers compare against the gate-appropriate default via the single shared selection rule; the skip carve-outs (non-bead no-gate, unknown recorded gate — both only while gates are configured) hold; a panel-less complete (nil registration) stays advisory-silent and panic-free; gates-absent behavior is 109's verbatim; no `Allow`/`Block` changes anywhere (spec AC7, R7)
- [ ] `config show` renders gates in enum order and `substitutes` in sorted-key order with the slot-id convention, echoes `note`, and warns on unknown model ids without affecting the exit code — seeded ids and family strings never warn (spec AC8, R8)
- [ ] `config show --gate <name> --json` emits exactly the five documented members, agreeing with the R3 resolvers, with `substitution.in_force` flipping per R5; unknown `--gate` exits non-zero with the five-key recovery line; the command writes nothing (spec AC9, R8/R9)
- [ ] Hostile config-controlled strings never reach text output raw and round-trip byte-exactly through the real-encoder `--json` path (spec AC10, R8)
- [ ] The `--gate --json` schema and the recorded `gate` field are documented in `.mindspec/domains/workflow/interfaces.md` as a forward-compatible, additive-only contract; no writer- or runner-side behavior is added (R9)

**Depends on**
Bead 1, Bead 2

## Provenance

Spec ACs are numbered in the order they appear in the spec's Acceptance
Criteria checklist. Every spec AC traces to a bead; every requirement R1–R9 is
delivered (R1–R5 → Bead 1, R6 → Bead 2, R7–R9 → Bead 3).

| Acceptance Criterion (spec) | Verified By |
|---------------------------|-------------|
| AC1 — gates-absent identity with 109, `gates: {}` ≡ absent, count-less → 1 (`TestLoad_GatesAbsentByteIdentical109`) | Bead 1 verification (PASS-line grep) |
| AC2 — standing-protocol YAML round-trip + `note` inertness (`TestLoad_PerGateProtocolRoundTrips`) | Bead 1 verification (PASS-line grep) |
| AC3 — every R4 refusal incl. both inheritance shapes; unknown model id never errors (`TestLoad_RefusesPerGateKnobs`) | Bead 1 verification (PASS-line grep) |
| AC4 — per-field fallback chain, adhoc→bead, unknown-gate error, raw-expr-only thresholds (`TestPanelGateResolvers_FallbackAndAdhoc`) | Bead 1 verification (PASS-line grep) |
| AC5 — deterministic slot expansion, cursor-start-0, normative 9-slot worked example (`TestPanelGateSlots_DeterministicExpansion`) | Bead 1 verification (PASS-line grep) |
| AC6 — recorded `gate` field decision-inert, parse-lenient, legacy-identical (`TestPanel_GateFieldDecisionInert`) | Bead 2 verification (PASS-line grep) |
| AC7 — gate-aware advisory compare + skip carve-outs + gates-absent 109 identity (`TestPanelAdvisory_GateAwareCompare`) | Bead 3 verification (PASS-line greps — incl. the gates-configured cmd-side `TestConfigShow_ReviewerCountNoteGateAware`; helper rule also pinned by Bead 1's `TestPanelGateAdvisoryDefault_SelectionRule`) |
| AC8 — gates/substitutes/note rendering, enum + sorted order, known-model warning never flips exit code (`TestConfigShow_GatesSubstitutesAndModelAdvisory`) | Bead 3 verification (PASS-line grep) |
| AC9 — `--gate --json` members = R9 contract, `in_force` per R5, unknown-gate recovery, read-only (`TestConfigShowGate_ResolvedJSON`) | Bead 3 verification (PASS-line grep) |
| AC10 — hostile strings escaped in both text paths, byte-exact JSON round-trip (`TestConfigShow_HostileStringsEscaped`) | Bead 3 verification (PASS-line grep) |
| AC11 — `internal/panel` imports no `internal/config`; `threshold > 0` guard survives | Bead 2 verification (`go list -deps` non-zero + gate.go grep); leaf invariant re-asserted in Bead 3 verification |
| AC12 — tree builds, touched packages fully green | every bead's `go build ./...` + per-package `go test`; full `go test ./...` regression at plan time and pre-`/ms-impl-approve` (Testing Strategy) |
| Validation Proof — `config show` over the Goal YAML: four gates, sums 9/9/6/12, substitutes, exit 0, read-only | Bead 3 verification (protocol-YAML proof + porcelain-clean check) |
| Validation Proof — `--gate final_review --json` 12 slots / `"11"`; unconfigured `adhoc` slots ≡ `bead` | Bead 3 verification (protocol-YAML proof) |
| Validation Proof — ADR-0037 second amendment present alongside 109's, `decision-inert` anchored | Bead 2 verification (amendment greps) |
| R2 — standing protocol ships as the documented example, not `DefaultConfig` | Bead 1 step 7 + the `fable-window 2026-07, codex-enabled` note-anchor grep (unique to the Goal YAML — a known-model seed list alone cannot satisfy it); `DefaultConfig` emptiness pinned in AC1 |
| R9 — `--gate --json` schema + recorded `gate` field documented as additive-only stable contract | Bead 3 verification (`expected_reviewers` doc grep) + Bead 3 step 7 |
