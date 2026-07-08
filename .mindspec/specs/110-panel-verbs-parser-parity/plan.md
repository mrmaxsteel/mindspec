---
adr_citations:
    - ADR-0040
    - ADR-0037
    - ADR-0036
    - ADR-0035
    - ADR-0034
    - ADR-0032
    - ADR-0039
spec_id: 110-panel-verbs-parser-parity
status: Draft
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/panel/create.go
        - internal/panel/create_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/validate/spec.go
        - internal/validate/spec_test.go
        - .mindspec/domains/workflow/architecture.md
    - depends_on: []
      id: 3
      key_file_paths:
        - internal/instruct/panelstate.go
        - internal/instruct/panelstate_test.go
        - .mindspec/domains/workflow/overview.md
    - depends_on:
        - 1
      id: 4
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on:
        - 4
      id: 5
      key_file_paths:
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .mindspec/domains/workflow/runbook.md
---
# Plan: 110-panel-verbs-parser-parity

## ADR Fitness

The sole impacted domain is **workflow** (spec § Impacted Domains: every
source edit — `internal/panel`, `cmd/**`, `internal/validate`,
`internal/instruct`, `plugins/mindspec/**`, `.claude/skills/**` — lands under
the `workflow` OWNERSHIP globs; `internal/config`, `internal/contextpack`,
`internal/executor`, and `internal/adr` are consumed **read-only** and are not
impacted). All six touchpoint ADRs plus the governing ratchet (ADR-0040) were
evaluated; each genuinely constrains a bead and is cited. One frequently-
adjacent ADR (ADR-0030) was evaluated and **deliberately not cited**. **No bead
diverges from any accepted ADR** — the honest boundaries this spec draws (R7)
are exactly the accepted designs, not departures from them.

- **ADR-0040 — Orchestration Layering Ratchet** (Domain(s): core, workflow;
  intersects workflow). The license and the load-bearing frame for the whole
  spec: a rule that lived in skill prose (`ms-panel-run` step 0's co-bump
  invariant, `ms-panel-tally`'s decision matrix) and proved load-bearing
  ratchets **down** into an in-binary verb behind a **stable artifact + CLI
  contract**. Its portability principle drives two hard rules this plan
  enforces: (a) the `panel.json`/verdict-file schemas are the agent-neutral
  surface a non-Claude-Code runner targets, documented as such (R4, Bead 1);
  (b) the verbs are adapters over the unchanged decision — they add **no** new
  ceremony and **no** second decision copy (Beads 3/4). ADR-0040 landed on
  `main` via spec 109 before this branch was rebased forward, so it is
  citable here (unlike 109's own plan). This plan **adheres**.
- **ADR-0037 — Panel Gate as Enforced Contract** (Domain(s): workflow,
  execution; intersects workflow). The spine, binding Beads 1/3/4. §3's single
  home (`internal/panel.PanelGateDecision` / `panel.Panel.ApproveThreshold`,
  as extended by 109's threshold amendment) is **not weakened**: `panel verify`
  and `panel tally` reuse `panel.ResolveGateFacts` + `panel.PanelGateDecision`
  and add **no** second interpreter, and the instruct refactor (Bead 3)
  *removes* the one pre-existing second copy rather than adding one. `panel
  create` writes the `panel.json` the gate reads, stamping the optional
  `approve_threshold` field 109's amendment introduced. §§6/8 (fail-open/
  closed, the plain-reviewable-files trust boundary — no signing) are
  untouched: `panel verify`/`tally` are read surfaces, not a new enforcement
  point (Non-Goals). This plan **adheres**.
- **ADR-0036 — Ownership Discovery** (Domain(s): workflow, validation,
  doc-sync, ownership; intersects workflow). Governs Bead 2's R5 domain-
  resolution parity: an Impacted-Domains entry resolves to its owning-domain
  NAME through the **shared** `normalizeImpactedDomains` (spec 100 R1) at the
  identical severity the plan-approve gate uses — path-like zero/multi-owner is
  an error, a bare name without a manifest is kept verbatim. Also governs the
  gate-forward doc-sync constraint every bead honors (a `workflow` source edit
  carries a `workflow` domain-doc region). Sound as-is.
- **ADR-0035 — Agent Error Contract** (Domain(s): workflow, execution, core;
  intersects workflow). Constrains `panel tally`'s non-zero Block (Bead 4) and
  the new spec-approve validation errors (Bead 2). The `tally` Block is a
  `guard.NewFailure` whose body keeps `PanelGateDecision`'s raw-`git merge`
  fence and whose **final** line is a genuine `recovery:` command — mirroring
  `internal/complete.panelGate` verbatim, so it passes
  `guard.HasFinalRecoveryLine`. The spec-approve errors are `validate.Result`
  Issues (not `guard.NewFailure`, which the validate layer does not use): each
  carries an **inline actionable recovery hint** naming the offending entry, in
  the exact style the existing `impacted-domains-resolve` / `adr-cite-missing`
  messages already use — the R5 messages are the resolver's own strings
  verbatim (no new wording). Sound as-is.
- **ADR-0034 — Ceremony Collapse** (Domain(s): workflow; intersects workflow).
  The verbs operate inside the collapsed single-bead lifecycle and must **not**
  add a gate: `panel create` replaces the hand-typed `panel.json` with one
  command (not an extra step), and `panel tally`'s pass hands off to the same
  `mindspec complete` merge terminal (Beads 4/5). Constrains the skill trim
  (Bead 5): the surviving `ms-panel-tally` steps still route a pass through
  `mindspec complete`. Sound as-is.
- **ADR-0032 — Semantic ADR Coverage Gates** (Domain(s): validation, adr,
  lifecycle, workflow; intersects workflow). Governs Bead 2's R6 touchpoint
  parity **and** the honest boundary it draws: the check verifies **existence
  only** against the same `adr.Store` the plan-time citation gate
  (`checkADRCitations`) reads; Accepted-status, domain-intersection
  (`adr-cite-irrelevant`), and coverage (`adr-coverage-missing`) stay at
  plan-approve. It does not resurrect touchpoints-as-citation-source (spec 097
  R2). Sound as-is.
- **ADR-0039 — Flat `.mindspec/` Layout v2** (Domain(s): core, workflow,
  execution, context-system; intersects workflow). `panel create` (Bead 4)
  writes the panel directory under the layout-aware location `panel.Scan`
  already reads — co-located `<spec-dir>/reviews/<slug>` on a flat tree
  (this repo, post-#168), repo-root `review/<slug>` otherwise — resolved via
  `workspace.SpecDir` + `workspace.DetectLayout`, reusing
  `internal/complete.panelGateRoots`' existing layout logic. It introduces no
  third convention. Sound as-is.

**Evaluated, deliberately NOT cited: ADR-0030 — Executor Boundary** (Domain(s):
**execution**). `panel create`'s target rev-parse routes through
`newExecutor(root).RevParseRef` (the git-I/O boundary this ADR pins), so
ADR-0030 is real *context*. But its Domain is `execution`, which does **not**
intersect the sole impacted domain `workflow`; citing it in `adr_citations`
would fire `adr-cite-irrelevant` at plan-approve. Consuming an executor method
is not an `internal/executor` *source* change, so `execution` is not impacted
(the spec's own Impacted-Domains note pins this). This mirrors the spec keeping
ADR-0030 in prose, and 112's plan omitting context-only ADR-0023.

**Coverage check.** `workflow` (the only impacted domain) is covered by every
citation — ADR-0040/0037/0036/0035/0034/0039 all name `workflow` in their
`Domain(s)` line and all are Accepted, so `checkADRCoverage` finds a cited
Accepted covering ADR and `checkADRCitations` finds no irrelevant citation.

**Divergence report: none.** No bead is better served by a design that departs
from an accepted ADR. The two designs a reviewer might probe are both refused
by the spec itself: (a) letting `internal/panel` import `internal/config` to
self-resolve the reviewer mix — rejected, it breaks the ADR-0037 leaf invariant
`PanelGateDecision` depends on (R7b); the caller passes plain values. (b)
Re-running plan-level ADR coverage at spec-approve — rejected, it breaks the
ADR-0032 boundary (R7c); spec-approve checks only existence + resolvability.

## Testing Strategy

**Approach.** Pure, table-driven unit tests added to each touched package,
plus the spec's one manual e2e (Bead 4 Validation Proof) exercised against a
built binary in a temp git repo. The heavy logic is fs-only
(`internal/panel.Create` — write `panel.json` + splice the BRIEF header,
preserving the body and prior-round verdict files) and pure
(`PanelGateDecision` rendering, the validate parity checks, the instruct
delegation); the CLI verbs are **thin adapters** over
`panel.Create`/`panel.ResolveGateFacts`/`panel.PanelGateDecision`, so each verb
is exercised through a **pure renderer** that takes fabricated
`panel.GateFacts` (no git needed) plus a thin git-seam-injected wiring proven
once by the manual e2e.

**Per-test proof discipline.** Every new test is verified with an anchored
PASS-line grep so a reviewer sees the specific test pass, not a bare package
green: `go test ./PKG -v -run 'TestName$' | grep -q -- '--- PASS: TestName'`.
The `$` anchor on `-run` stops a prefix sweeping in a sibling. The nine named
test functions use exactly the spec's Acceptance-Criteria names. **Grep note:**
this machine's `grep` is ugrep; the plain fixed-string `grep -q` /
`grep -q --` forms below are ugrep-safe as written, and any name-anchored
file-membership check uses `/usr/bin/grep -qxF` explicitly.

**Decision single-home (the anti-drift pin).** The spec's central claim — one
decision function, no second copy — is falsified by a *dedicated* contract
test, not merely by the per-verb tests: `TestPanelVerbs_DecisionIsPanelGateDecision`
(Bead 4) feeds a table of `panel.GateFacts` rows (Allow / Block / Warn) and
asserts both `panel verify` and `panel tally` render the **identical**
`panel.PanelGateDecision(facts).Action`, so relocating any decision branch into
a CLI adapter breaks it; `TestPanelStateVerdict_DelegatesToPanelGateDecision`
(Bead 3) does the same for `instruct --panel-state` and additionally asserts the
old `PanelStateEntry.verdict()` matrix is gone. Together they pin R7a.

**Leaf invariant.** `internal/panel` must stay import-clean of
`internal/config` (and of git) through the whole spec. Bead 1 (the only bead
adding an `internal/panel` symbol) asserts
`! go list -deps ./internal/panel | grep -q internal/config` exits `0`, and
Bead 4 (the last bead to land, the caller that resolves config + the target
SHA and passes them as plain values) re-asserts it. `panel.Create` takes
plain `int`/`string` arguments; it never sees a `*config.Config`.

**Parity is behavior-identical, merely earlier (Bead 2).** The R5/R6 tests
pin that spec-approve rejects **exactly** what plan-approve/divergence already
reject and nothing more: `TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove`
proves path-like/ambiguous → `impacted-domains-resolve` error (the same code
`plan.go`/`divergence.go` emit) while a bare-name-no-manifest entry that
plan-approve tolerates today still **passes**;
`TestValidateSpec_ADRTouchpointExtractionBoundary` proves an anchored link to a
missing ADR fails, a bare-prose `ADR-####` mention does not, and neither emits
any `adr-coverage-*`/`adr-cite-irrelevant` diagnostic.

**Regression.** Full `go test ./...` runs once at **plan time** and again
**pre-`/ms-impl-approve`** — not per bead; per-bead gates run the touched
packages only. Plan-time result (2026-07-08, this worktree): `go build ./...`
green; the four packages this spec touches (`internal/panel`,
`internal/validate`, `internal/instruct`, `cmd/mindspec`) plus
`internal/approve` and `internal/complete` are the enforcement surface and are
green. The pre-existing `internal/instruct` `TestRun_IdleNoBeads`
environment-isolation failure (tracked as `z4ps` — the idle "No Active Work"
template loses to ready beads leaking into `bd` discovery on a dev machine with
open work) is unrelated to this spec and excluded from its gates; Bead 3's own
new test (`TestPanelStateVerdict_DelegatesToPanelGateDecision`) is hermetic
(it builds `panel.GateFacts`/`panel.Result` fixtures and touches no `bd`).
Git-touching tests run with `GIT_TERMINAL_PROMPT=0`. No new external
dependency, no network access. Low cross-bead scope overlap is **by design**:
each bead owns a distinct package, so the advisory decomposition
scope-redundancy heuristic (which scans step/verify path mentions) may emit a
non-gating low-overlap note; it is the expected shape of a cleanly separated
five-package change, not a decomposition defect.

**Dependency shape (decomposition / the DAG).** Five beads (within the 3–5
optimal band's edge, ≤ the 6 advisory cap), with the shallowest DAG the
compile-time facts allow. Three are **roots** that run in parallel:
- **Bead 1** (`internal/panel` writer + schema doc) — root.
- **Bead 2** (`internal/validate` spec-approve parity) — root; touches no
  package Bead 1/3 touch.
- **Bead 3** (`internal/instruct` verdict-delegation refactor) — root: it
  depends only on `panel.ResolveGateFacts` + `panel.PanelGateDecision`, which
  **already exist on `main`** (spec 099); it introduces no new dependency on
  any 110 bead, so ordering it after another bead would be a false edge.

Two are consumers, each edge a **real compile-time dependency**, not an
ordering wish:
- **Bead 4** (`cmd/mindspec` panel verb tree) `depends_on: [1]` — `panel create`
  calls the new `panel.Create`; the edge is a compile dependency.
- **Bead 5** (skill de-dup) `depends_on: [4]` — the skills must **reference**
  the `mindspec panel create|verify|tally` verbs Bead 4 lands, and the R8 grep
  gate asserts those references exist, so Bead 5 cannot pass before Bead 4
  merges.

Longest chain is 1 → 4 → 5 (depth 3, not exceeding the advisory MaxChainDepth
of 3); bead count 5 (≤ 6); three of five beads are roots (parallelism 0.60,
well above the 0.25 floor). **Parallel-safety of doc-sync:** the three root
beads edit **disjoint** `workflow` domain-doc files — Bead 1 → `interfaces.md`,
Bead 2 → `architecture.md`, Bead 3 → `overview.md` — so no two roots touch the
same file. Bead 4 also edits `interfaces.md`, but it is cut *after* Bead 1
merges (real edge), so it appends on top of Bead 1's content with no conflict;
Bead 5 edits `runbook.md`, untouched by any other bead.

**Requirement → bead map.** R1 → Bead 1 (leaf writer + BRIEF-header co-bump
mechanism) + Bead 4 (the `panel create` CLI that stamps the 109 resolvers and
the target SHA); R2 → Bead 3 (the `PanelStateEntry.verdict()` ratchet onto the
single home) + Bead 4 (`panel verify`); R3 → Bead 4 (`panel tally`); R4 →
Bead 1 (schema doc + `TestPanelSchemaDoc_MatchesConstants`); R5 → Bead 2; R6 →
Bead 2; R7 (a: single-home / b: config-free leaf / c: no plan-work moved) →
pinned across Beads 1/2/3/4 by the contract, leaf, and boundary tests; R8 →
Bead 5. Every spec requirement is delivered; the Provenance table maps every
spec acceptance criterion.

## Bead 1: internal/panel — leaf-safe `Create` registration writer (panel.json + BRIEF machine-header co-bump) + verdict-file/slot schema doc

Delivers R1 (the writer half) and R4, and pins R7b (config-free leaf). The
source edit is the **workflow** domain (`internal/panel`); doc-sync +
R4's portability-contract documentation both land in
`.mindspec/domains/workflow/interfaces.md`.

**Steps**
1. Add `internal/panel/create.go` with a leaf-safe writer,
   `func Create(dir string, p Panel) error` (or `Create(dir string, in
   CreateInput) error` if a caller-facing value struct reads more cleanly —
   the wire shape written to disk stays the existing `Panel`). It takes
   **plain values only** (`BeadID *string`, `Spec`, `Target`, `Round`,
   `ExpectedReviewers int`, `ApproveThresholdExpr string`, `ReviewedHeadSHA`)
   and imports **no** `internal/config` and **no** git — the caller (Bead 4)
   resolves those and passes them in (spec 109's "config reaches the leaf only
   as plain values"). It `os.MkdirAll`s `dir`, marshals the `Panel` to
   `panel.json` via `encoding/json` (`MarshalIndent`), and writes it with
   `os.WriteFile` — one write, `reviewed_head_sha` and `round` present in the
   same struct **by construction** (no code path can emit one without the
   other). `bead_id` marshals to JSON `null` when the pointer is nil.
2. In the **same** `Create` call, rewrite the BRIEF machine-managed header
   **atomically with** `panel.json`: define an owned, delimited region
   (`<!-- mindspec:panel-header -->` … `<!-- /mindspec:panel-header -->`)
   holding **only** the machine-derived fields (slug = `filepath.Base(dir)`,
   round, branch = `Target`, resolved commit SHA = `ReviewedHeadSHA`). When
   `BRIEF.md` does not exist (first `create`), write the header region
   followed by a **stub body** (the skill fills it — the section headings from
   `ms-panel-run` step 3, left empty). When `BRIEF.md` exists (a re-panel),
   read it, replace **only** the delimited region in place, and write the file
   back **byte-for-byte preserving everything after the closing marker** (the
   skill-authored summary / files-in-scope / prior-round asks / lens). If the
   markers are absent from an existing BRIEF (legacy), prepend a fresh region
   and keep the whole existing body below it. `Create` **never** reads, writes,
   or deletes any `*-round-<N>.json` verdict file — a re-panel leaves prior
   rounds untouched by construction (it only ever touches `panel.json` and
   `BRIEF.md`).
3. Add `internal/panel/create_test.go`:
   `TestCreate_WritesRegistrationAtomically` — call `Create` into a `t.TempDir`
   with fixed values; assert the on-disk `panel.json` round-trips
   (`json.Unmarshal`) to a `Panel` whose `ExpectedReviewers`,
   `ApproveThresholdExpr`, `ReviewedHeadSHA`, `Round`, `BeadID`, `Spec`,
   `Target` **equal exactly what was passed** and that `panel.json` contains a
   non-empty `reviewed_head_sha` key (the field is never omitted); assert the
   BRIEF header region carries the same round + SHA; then pre-seed a
   skill-authored body, call `Create` again with `Round: 2` and a new SHA, and
   assert (a) `panel.json.round == 2` with the new SHA, (b) the BRIEF header
   shows round 2 + the new SHA, (c) the skill-authored body below the closing
   marker is **byte-identical** to before, and (d) a pre-seeded
   `R1-round-1.json` verdict file is still present and unchanged.
4. Add the R4 verdict-file/slot **schema documentation** to
   `.mindspec/domains/workflow/interfaces.md` (a bead-unique region): the
   agent-neutral panel artifact contract another runner targets — the
   `panel.json` registration filename (`panel.FileName`), the
   `<slot>-round-<N>.json` verdict-file shape with `N ≥ 1` (the
   `panel.verdictFileRE` pattern, stated with a conforming example like
   `R1-round-1.json` and the note that `-round-0` is nonconforming), and the
   `consolidated-round-<N>.md` name (`panel.ConsolidatedName`). Frame it, per
   ADR-0040's portability principle, as the artifact + CLI contract a runner
   adapts behind — degraded modes are the runner's concern, not the schema's.
5. Add `TestPanelSchemaDoc_MatchesConstants` to `internal/panel/create_test.go`:
   read `../../.mindspec/domains/workflow/interfaces.md` (the package test cwd
   is `internal/panel`, so `../../` is the repo root) and assert the documented
   registration filename **contains** `panel.FileName`, the documented
   verdict-file example **matches** `verdictFileRE` (and a `-round-0` example
   does not), and the documented consolidated name **contains**
   `ConsolidatedName(1)`. The test fails if the doc later names a pattern the
   constants reject (wrong round base, wrong extension, wrong consolidated
   prefix) — so the doc cannot drift from the code.

**Verification**
- [ ] `go test ./internal/panel -v -run 'TestCreate_WritesRegistrationAtomically$' | grep -q -- '--- PASS: TestCreate_WritesRegistrationAtomically'`
- [ ] `go test ./internal/panel -v -run 'TestPanelSchemaDoc_MatchesConstants$' | grep -q -- '--- PASS: TestPanelSchemaDoc_MatchesConstants'`
- [ ] `go test ./internal/panel` exits `0` (whole package green, existing gate/tally tests included)
- [ ] `! go list -deps ./internal/panel | grep -q internal/config` exits `0` (the new writer keeps `internal/panel` a config-free leaf — spec AC10 first assertion, R7b)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync: the workflow source edit carries the workflow domain-doc region + the R4 schema)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] `Create` records `expected_reviewers`/`approve_threshold`/
  `reviewed_head_sha`/`round`/ids exactly as passed, in one `panel.json`
  write; `reviewed_head_sha` is never omitted (spec AC1, R1)
- [ ] A re-panel co-bumps `round` + a re-resolved SHA in **both** `panel.json`
  and the BRIEF machine-header in the same operation, leaves prior-round
  verdict files and the skill-authored BRIEF body untouched (spec AC2 writer
  half, R1)
- [ ] The workflow schema doc names a verdict-file pattern, round base, and
  consolidated-file name **consistent with** `panel.FileName`/`verdictFileRE`/
  `ConsolidatedName`, enforced by `TestPanelSchemaDoc_MatchesConstants` (spec
  AC7, R4)
- [ ] `internal/panel` imports no `internal/config` (spec AC10, R7b)

**Depends on**
None

## Bead 2: internal/validate — spec-approve parser parity folded into ValidateSpec (Impacted-Domains resolution + ADR-Touchpoint existence)

Delivers R5, R6, and R7c (no plan-work moved). The source edit is the
**workflow** domain (`internal/validate/spec.go`); doc-sync lands in
`.mindspec/domains/workflow/architecture.md`. `internal/approve` needs **no**
source change: `ApproveSpec` already runs `validate.ValidateSpec` and hard-fails
on `vr.HasFailures()` (spec.go:47-50), so both `mindspec validate spec` and
`mindspec spec approve` inherit these checks the moment they are `ValidateSpec`
errors — the single-home decision the spec's Open Questions pinned.

**Steps**
1. In `ValidateSpec` (`internal/validate/spec.go`), after the existing required-
   section checks, add the **R5 Impacted-Domains parity** check as the
   **identical call** `plan.go:142-146` makes: extract via the existing
   `loadImpactedDomains(specDir)` helper (which is `contextpack.ParseSpec` —
   the same parser the downstream gates consume, and the only extractor used),
   then `normalized, normErrs := normalizeImpactedDomains(nil, root, "", impacted)`
   (working-tree read: `exec` nil, `ownerRef` ""), and
   `r.AddError("impacted-domains-resolve", e)` for each `e` in `normErrs`. This
   surfaces **only** the resolver's own errors under the **same** code
   `plan.go`/`divergence.go` emit (path-like zero/multi-owner), adds **no**
   stricter rule, and — because `normalizeImpactedDomains` keeps a
   bare-name-no-manifest entry verbatim (Rule 2, no error) — rejects **nothing**
   plan-approve tolerates today. A `contextpack.ParseSpec` read failure is
   handled the same way plan.go does (surface it, do not panic); it is
   unreachable in practice here because `ValidateSpec` already confirmed
   `spec.md` is readable.
2. Add the **R6 ADR-Touchpoint parity** check to `ValidateSpec`: take the
   `## ADR Touchpoints` section body from the existing `parseSections(content)`
   map, and extract **only anchored markdown-link references** — every
   `[ADR-\d{4}](<path>)` occurrence in that section — via a small regexp
   (`\[(ADR-\d{4})\]\([^)]+\)`). A **bare-prose `ADR-####` mention** (no
   `[...](...)` anchor) is **not** matched, so 110's own prose mentions of
   `ADR-0040` (not yet a file at authoring time) and `ADR-0030` inside the
   ADR-0037 bullet are correctly outside the check. Resolve each extracted ID
   against the **same** store the citation gate uses —
   `newMemoStore(adrStoreForSpecFn(root, specDir))` (identical to
   `plan.go:156` / `divergence.go:137`) — and on a `store.Get(id)` error emit
   `r.AddError("adr-touchpoint-missing", …)` with an inline recovery hint
   naming the missing/typo'd reference (a spec-approve-specific code modeled on
   `adr-cite-missing`'s **existence-only** shape; deliberately distinct from the
   plan-time citation code because this checks the spec's *touchpoints* prose,
   not the plan's frontmatter citations). The check verifies **existence only**
   — it emits **no** `adr-coverage-*` and **no** `adr-cite-irrelevant`
   diagnostic (those stay at plan-approve, R7c) — and does not re-derive plan
   citations from the touchpoints (spec 097 R2 boundary).
3. Add `TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove` to
   `internal/validate/spec_test.go` (a temp spec tree with domain
   `OWNERSHIP.yaml` fixtures): (a) a spec whose `## Impacted Domains` has a
   **path-like zero-owner** bullet fails `ValidateSpec` with the
   `impacted-domains-resolve` code and a message naming the entry; (b) a spec
   with a **bare-name-no-manifest** bullet (a plain domain name with no on-disk
   manifest) **passes** `ValidateSpec` (kept verbatim, no error) — the
   parity-with-plan-approve pin; (c) a bullet naming a real domain dir passes.
   Assert by scanning `r.Issues` for the code, not by `FormatText` substring.
4. Add `TestValidateSpec_ADRTouchpointExtractionBoundary` to
   `internal/validate/spec_test.go`: (a) a spec whose `## ADR Touchpoints` has
   an **anchored** bullet `- [ADR-9999](../../adr/ADR-9999-x.md): …` (no such
   ADR in the fixture store) fails `ValidateSpec` with `adr-touchpoint-missing`
   and a recovery hint; (b) a spec with a **bare-prose** `per ADR-9999` mention
   inside the section (no anchor) does **not** fail; (c) a **110-shaped** spec —
   anchored links to ADRs present in the fixture store **plus** bare-prose
   mentions of an absent ADR — passes; and assert **none** of the three adds
   any Issue whose code begins `adr-coverage` or equals `adr-cite-irrelevant`
   (coverage/relevance stay plan-approve).
5. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/architecture.md` documenting that `spec approve`
   (via `ValidateSpec`) now runs the two parser-parity checks — the
   Impacted-Domains resolution through the shared `normalizeImpactedDomains`
   (same code + severity as plan-approve) and the ADR-Touchpoint
   **existence-only** resolution of **anchored links** against the citation
   gate's store — and the explicit boundary that Accepted-status,
   domain-intersection, and coverage stay at plan-approve.

**Verification**
- [ ] `go test ./internal/validate -v -run 'TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove$' | grep -q -- '--- PASS: TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove'`
- [ ] `go test ./internal/validate -v -run 'TestValidateSpec_ADRTouchpointExtractionBoundary$' | grep -q -- '--- PASS: TestValidateSpec_ADRTouchpointExtractionBoundary'`
- [ ] `go test ./internal/validate` exits `0` (whole package green — the existing plan-approve `impacted-domains-resolve`/`adr-cite-missing` tests still pass, proving no shared-helper regression)
- [ ] `go test ./internal/approve` exits `0` (spec-approve inherits the new `ValidateSpec` failures with no `internal/approve` source change — existing approve tests stay green)
- [ ] `~/.local/bin/mindspec validate spec 110-panel-verbs-parser-parity` exits `0` (110's own spec — anchored touchpoints to existing ADRs, bare-prose ADR-0040/0030, single `workflow` Impacted-Domain — passes the checks it introduces; the self-consistency the spec's R5/R6 falsifications require)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] Path-like zero/ambiguous Impacted-Domains entries fail `ValidateSpec`
  under `impacted-domains-resolve`; a bare-name-no-manifest entry that
  plan-approve tolerates today still passes; a real-domain-dir entry passes
  (spec AC8, R5)
- [ ] An anchored `## ADR Touchpoints` link to a nonexistent ADR fails
  `ValidateSpec` with a recovery hint; a bare-prose `ADR-####` mention does
  not; a 110-shaped spec passes; no `adr-coverage-*`/`adr-cite-irrelevant`
  diagnostic is emitted at spec-approve (spec AC9, R6, R7c)
- [ ] `spec approve` inherits both checks via `ValidateSpec` with no
  `internal/approve` logic change; the resolver's own error strings are reused
  verbatim for R5 (no stricter rule) (R5, R7c)

**Depends on**
None

## Bead 3: internal/instruct — ratchet PanelStateEntry.verdict() onto panel.PanelGateDecision (delete the duplicate matrix)

Delivers the R2 instruct half — the "ratchet the pre-existing duplicate onto
the single home" work. The source edit is the **workflow** domain
(`internal/instruct`); doc-sync lands in
`.mindspec/domains/workflow/overview.md`. This bead is a **root**: it depends
only on `panel.ResolveGateFacts` + `panel.PanelGateDecision`, which already
exist on `main` (spec 099), not on any 110 bead.

**Steps**
1. Refactor `PanelStateEntry.verdict()` in `internal/instruct/panelstate.go` to
   **stop reproducing** the decision matrix and instead build `panel.GateFacts`
   and return the mapped outcome of `panel.PanelGateDecision(facts)`. Map
   `panel.Allow → GatePass`, `panel.Warn → GateWarn`, `panel.Block → GateBlock`,
   and use the `Decision.Message` as the one-line reason. Delete the abandoned/
   round-mismatch/staleness/incomplete/REJECT/threshold branches — the second
   copy — entirely; the enum (`GatePass`/`GateBlock`/`GateWarn`) and `gateLabel`
   stay (still used by `renderPanelState`).
2. Source the facts through `panel.ResolveGateFacts` for the **bead-panel**
   staleness path, honoring R2's "over `panel.ResolveGateFacts`": in
   `gatherPanelState`, for a bead panel wire a `panel.GateIO` whose `RevParse`
   adapts the existing branch-SHA resolver (`liveBranchSHA` /
   `BranchSHAResolver`), whose `IsRefNotFound` reflects that resolver's
   "branch gone" flag, and whose `Worktree` returns `""` — so
   `WorktreeAbsent` is true and the dirty-tree leg is **skipped**, preserving
   instruct's read-only snapshot behavior (it has never done dirty detection).
   For a **non-bead** panel (final-review/PR; `BeadID` null), build `GateFacts`
   **without** a `bead/<id>` rev-parse (leave `HeadSHA` empty) so
   `PanelGateDecision`'s staleness leg (`p.ReviewedHeadSHA != "" && f.HeadSHA
   != ""`) stays inert — byte-identical to today's `if p.IsBead()` guard. The
   `PanelStateEntry` carries the fields the decision needs (its tally/
   registration + resolved SHA / branch-missing flag); adjust its construction
   in `gatherPanelState` accordingly (all consumers are inside
   `internal/instruct`).
3. Add `TestPanelStateVerdict_DelegatesToPanelGateDecision` to
   `internal/instruct/panelstate_test.go`: over a table of fabricated facts
   spanning the decision surface — fresh/stale bead panel, incomplete,
   REJECT/hard_block, abandoned, sub-/at-threshold, and a non-bead panel —
   assert the mapped `verdict()` outcome equals `mapAction(panel.PanelGateDecision(facts).Action)`
   for the identical facts, so any surviving independent branch diverges and
   fails; additionally assert (structurally) that `verdict()` no longer carries
   its own threshold/staleness logic (it delegates). Keep the existing
   `renderPanelState`/`gatherPanelState` tests green (the rendered block shape
   is unchanged; only the verdict source moved).
4. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/overview.md` noting that `instruct --panel-state`
   now renders the **single** `panel.PanelGateDecision` (over
   `panel.ResolveGateFacts`) rather than a second read-only matrix — the
   ADR-0040 ratchet removing a two-places drift.

**Verification**
- [ ] `go test ./internal/instruct -v -run 'TestPanelStateVerdict_DelegatesToPanelGateDecision$' | grep -q -- '--- PASS: TestPanelStateVerdict_DelegatesToPanelGateDecision'`
- [ ] `go test ./internal/instruct -run 'PanelState|PanelRounds|renderPanel'` exits `0` (the panel-state render/gather tests stay green — the block shape is unchanged)
- [ ] `go build ./...` exits `0` (the delegation compiles; `panel.PanelGateDecision`/`ResolveGateFacts` are the existing 099 symbols)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/overview.md'` (doc-sync)

**Acceptance Criteria**
- [ ] `instruct --panel-state`'s per-panel verdict is produced by
  `panel.PanelGateDecision` (over `panel.ResolveGateFacts`), not an independent
  matrix; the old `PanelStateEntry.verdict()` reproduction is gone (spec AC6,
  R2)
- [ ] Non-bead panels keep today's behavior (staleness leg inert, no `bead/`
  rev-parse); bead panels route staleness through `ResolveGateFacts`; the
  rendered block shape is unchanged (R2)

**Depends on**
None

## Bead 4: cmd/mindspec — the `panel create | verify | tally` verb tree (thin adapters over the single home)

Delivers R1 (the CLI half), R2 (`panel verify`), R3 (`panel tally`), and pins
R7a (single-home decision) via the contract test. Both touched files are the
**workflow** domain (`cmd/**`); doc-sync lands in
`.mindspec/domains/workflow/interfaces.md`. **Depends on Bead 1** — `panel
create` calls the new `panel.Create` (a compile-time edge). Cut after Bead 1
merges, so its `interfaces.md` region appends on top of Bead 1's schema doc.

**Steps**
1. Add `cmd/mindspec/panel.go` with a `panel` parent cobra command and the
   `create` subcommand: `panel create <slug> --spec <id> --target <ref>
   [--bead <id>] [--round N]`. It `findRoot()`s, `config.Load(root)`s, and
   resolves `expected_reviewers` via `cfg.PanelExpectedReviewers()` and the raw
   threshold via `cfg.PanelApproveThresholdExpr()` (the 109 resolvers,
   read-only); resolves the panel **directory** layout-aware — reuse the same
   `workspace.DetectLayout` + `workspace.SpecDir` logic
   `internal/complete.panelGateRoots` uses (flat → `<spec-dir>/reviews/<slug>`,
   otherwise repo-root `review/<slug>`); resolves `reviewed_head_sha` from the
   live `--target` ref via a package-level seam
   `var revParseForPanelFn = func(root, ref string) (string, error) { return
   newExecutor(root).RevParseRef(root, ref) }` (swappable in tests, mirroring
   `internal/complete`'s `gateRevParseFn`); sets `round` (default `1`) and
   `bead_id` (nil when `--bead` absent); then calls `panel.Create(dir, Panel{…})`
   — the single write that co-bumps `panel.json` + the BRIEF header. Document
   in the command help that a `--bead <id>` panel expects `--target bead/<id>`
   and that a divergent `--target` can only **fail safe** (a stale-SHA
   false-BLOCK at gate time, never a false-PASS).
2. Add the `verify` subcommand: `panel verify <slug>` — `findRoot()`,
   `panel.Scan(configShowReviewRoots(root)...)` and match the registration
   whose `Slug() == slug` (a slug-not-found is a clear error with a recovery
   hint, **not** a panic and **not** a silent pass). Resolve gate facts exactly
   as `internal/complete.panelGate` does — `beadID` from the matched
   `panel.json` (empty for a non-bead panel), `scanRoot :=
   panel.PanelDirScanRoot(reg.Dir)`, `facts := panel.ResolveGateFacts(reg,
   beadID, scanRoot, panel.GateIO{RevParse: …, Status: …, IsRefNotFound: …,
   Worktree: …})` wired through the executor seams — then render via a **pure**
   `renderPanelVerify(res *panel.Result, facts panel.GateFacts) (line string,
   action panel.GateAction)` that computes `panel.PanelGateDecision(facts)` and
   prints: verdicts-present-vs-`expected_reviewers`, per-slot parse status
   (naming malformed files), `reviewed_head_sha` vs live tip (staleness), and a
   PASS/BLOCK preview line derived from `d.Action`. `verify` **writes nothing**
   and exits `0` (a read-only report is not a gate).
3. Add the `tally` subcommand: `panel tally <slug>` — resolve the registration
   + facts as in step 2, and render via a **pure**
   `renderPanelTally(res *panel.Result, facts panel.GateFacts, changes
   []slotChanges) (body string, d panel.Decision)` that prints (a) the per-slot
   verdict table (slot, verdict, hard_block from `res.Verdicts`), (b) the
   aggregate (APPROVE/REQUEST_CHANGES/REJECT counts + the resolved threshold
   from `res.Panel.ApproveThreshold()`), (c) the decision from
   `panel.PanelGateDecision(facts)`, and (d) the **aggregated
   `concrete_changes_required`** attributed to slot. The `concrete_changes_required`
   are read presentation-only by iterating `res.Verdicts` of the latest round
   and, for the REQUEST_CHANGES/REJECT ones, re-decoding
   `filepath.Join(reg.Dir, v.File)` for its `concrete_changes_required` array
   (the panel `verdictJSON` strips that field, so `tally` reads it itself); this
   read **never** feeds the decision. Exit `0` on `d.Action == panel.Allow`;
   on a Block return `guard.NewFailure(d.Message, fmt.Sprintf("re-run the panel
   (mindspec panel create %s --round <N+1> …), then mindspec complete <bead>",
   slug))` — the body keeps `PanelGateDecision`'s raw-`git merge` fence and the
   final line is a `recovery:` command, so it passes
   `guard.HasFinalRecoveryLine` and exits non-zero.
4. Add the tests to `cmd/mindspec/panel_test.go`:
   `TestPanelCreate_StampsResolversAndCoBumpsRoundSHA` — over a temp root with a
   config fixture and `revParseForPanelFn` stubbed to a fixed SHA, run `panel
   create demo --spec <id> --target bead/<id> --bead <id>`; assert `panel.json`
   carries `expected_reviewers`/`approve_threshold` from the config resolvers
   and the stubbed `reviewed_head_sha`; run a second `--round 2` with a new
   stub SHA and assert both `panel.json` and the BRIEF header show round 2 + the
   new SHA in one operation while a pre-seeded `R1-round-1.json` and the
   skill-authored BRIEF body are untouched.
   `TestPanelVerify_MatchesGateAndWritesNothing` — over fabricated facts,
   `renderPanelVerify`'s action equals `panel.PanelGateDecision(facts).Action`,
   and running the real `verify` over a temp panel dir mutates no file
   (compare a dir snapshot / `git status` before-after).
   `TestPanelTally_ExitCodeTracksDecision` — a passing panel yields exit `0`; a
   blocking (sub-threshold) panel yields a non-zero error whose message passes
   `guard.HasFinalRecoveryLine`; the printed decision equals
   `panel.PanelGateDecision`'s.
   `TestPanelVerbs_DecisionIsPanelGateDecision` — the R7a contract pin: over a
   table of `panel.GateFacts` rows (Allow/Block/Warn), **both**
   `renderPanelVerify` and `renderPanelTally` render the **identical** Action
   `panel.PanelGateDecision(facts)` returns, so relocating any decision branch
   into a CLI adapter breaks the test.
5. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/interfaces.md` documenting the `mindspec panel
   create | verify | tally` CLI surface — the flags, the round+SHA co-bump
   invariant `create` owns, `verify`'s read-only/exit-0 contract, and `tally`'s
   exit-code-tracks-decision + ADR-0035 recovery-line-on-Block behavior — as
   the agent-neutral CLI half of the ADR-0040 contract (the artifact half is
   Bead 1's schema region in the same file).

**Verification**
- [ ] `go test ./cmd/mindspec -v -run 'TestPanelCreate_StampsResolversAndCoBumpsRoundSHA$' | grep -q -- '--- PASS: TestPanelCreate_StampsResolversAndCoBumpsRoundSHA'`
- [ ] `go test ./cmd/mindspec -v -run 'TestPanelVerify_MatchesGateAndWritesNothing$' | grep -q -- '--- PASS: TestPanelVerify_MatchesGateAndWritesNothing'`
- [ ] `go test ./cmd/mindspec -v -run 'TestPanelTally_ExitCodeTracksDecision$' | grep -q -- '--- PASS: TestPanelTally_ExitCodeTracksDecision'`
- [ ] `go test ./cmd/mindspec -v -run 'TestPanelVerbs_DecisionIsPanelGateDecision$' | grep -q -- '--- PASS: TestPanelVerbs_DecisionIsPanelGateDecision'`
- [ ] `go test ./cmd/mindspec` exits `0` (whole package green, existing help/config tests included)
- [ ] `! go list -deps ./internal/panel | grep -q internal/config` exits `0` (leaf invariant re-asserted on the last bead to land — the caller resolves config + SHA and passes plain values; spec AC10, R7b)
- [ ] Manual e2e (spec Validation Proof): `go build -o /tmp/ms110 ./cmd/mindspec`; in a spec worktree, `/tmp/ms110 panel create demo --spec <id> --target bead/<id> --bead <id>` writes `<spec-dir>/reviews/demo/{panel.json,BRIEF.md}` with a captured `reviewed_head_sha`; `/tmp/ms110 panel verify demo` prints the completeness/staleness report, exit `0`, `git status` clean; `/tmp/ms110 panel tally demo` on a sub-threshold panel exits non-zero with a re-panel recovery line (`… | grep -q '^recovery: '`)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] `panel create` stamps `expected_reviewers`/`approve_threshold` from the
  109 config resolvers and `reviewed_head_sha` from the target ref; a second
  `create --round 2` co-bumps round + SHA in both `panel.json` and the BRIEF
  header, leaving prior verdict files and the skill-authored body untouched
  (spec AC2, R1)
- [ ] `panel verify`'s PASS/BLOCK equals `panel.PanelGateDecision`'s Action and
  the command writes no file (spec AC3, R2)
- [ ] `panel tally` exits `0` on Allow / non-zero on Block with a final
  recovery line, and its printed decision equals `panel.PanelGateDecision`'s
  (spec AC4, R3)
- [ ] Both verbs render the identical `panel.PanelGateDecision(facts)` Action
  over a `GateFacts` table — no second decision copy (spec AC5, R7a)
- [ ] `internal/panel` remains config-free with the caller passing plain values
  (spec AC10, R7b)

**Depends on**
Bead 1

## Bead 5: skills — remove the mechanized prose from ms-panel-run + ms-panel-tally, reference the verbs, keep the judgment sections

Delivers R8. The edits are the **workflow** domain (`plugins/mindspec/**` and
`.claude/skills/**` — the two `.claude` files are **byte-identical mirrors** of
the `plugins` files, so each must be edited identically); doc-sync lands in
`.mindspec/domains/workflow/runbook.md`. **Depends on Bead 4** — the skills must
reference verbs that exist, and the R8 grep gate asserts those references.

**Steps**
1. In `ms-panel-run/SKILL.md` **§ Step 0** (both the `plugins/…` file and its
   `.claude/skills/…` mirror), **replace** the `mkdir` + the hand-typed
   `panel.json` schema block + the "capture `reviewed_head_sha` NOW at fan-out"
   / "on every re-panel bump `round` AND `reviewed_head_sha` in the SAME write"
   invariant prose with a single `mindspec panel create <slug> --spec <id>
   --target <ref> [--bead <id>] [--round N]` invocation, and shrink the step-3
   BRIEF composition to **filling the stub** `create` wrote (the skill authors
   the summary / files-in-scope / prior-round asks / lens **below** the
   machine-managed header). The `"reviewed_head_sha"` hand-typed schema key
   must no longer appear in step 0.
2. **Keep** `ms-panel-run`'s **Launch the panel**, **Codex failure detection**,
   **Working directory matters**, **Slot lens defaults**, and **Anti-patterns**
   sections (runner-specific launch orchestration + lens judgment = L4). Update
   the one Anti-pattern that told the operator not to "skip the `panel.json`
   write" to point at the `mindspec panel create` verb instead.
3. In `ms-panel-tally/SKILL.md` (both files), **replace** Steps 1–3 (Load /
   Tabulate / the `| Condition | Action |` **decision-matrix table** with the
   N−1 threshold and the REJECT/incomplete/hard_block rows) with a single
   `mindspec panel tally <slug>` invocation, and **re-point** the halt-recovery
   stale-verdict step at `mindspec panel create --round <N+1>` (the co-bumping
   verb) rather than "/ms-panel-run step 0". The `| Condition | Action |` table
   must no longer appear.
4. **Keep** `ms-panel-tally`'s **Step 4 (Consolidate** — semantic dedup +
   criticality ranking, which authors `consolidated-round-<N>.md`),
   **§ Artifact gates** (the HARD-vs-soft `hard_block` **judgment** that SETS
   the flag `tally` reads), **§ After a halt — recovery**, **§ Escape hatch**,
   and **§ Abandon procedure** (judgment / human-audited procedure). The verb
   renders the mechanical union of `concrete_changes_required`; the semantic
   dedup + ranking + authoring the consolidated file stay skill judgment.
5. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/runbook.md` noting the panel operator procedure
   now uses `mindspec panel create|verify|tally` (mechanized registration +
   decision) with the judgment sections (Artifact gates, Consolidate, Slot lens
   defaults) retained in the skills.

**Verification**
- [ ] `S=plugins/mindspec/skills; ! grep -q '| Condition | Action |' "$S/ms-panel-tally/SKILL.md" && grep -q 'mindspec panel tally' "$S/ms-panel-tally/SKILL.md" && grep -q '## Artifact gates' "$S/ms-panel-tally/SKILL.md" && ! grep -q '"reviewed_head_sha"' "$S/ms-panel-run/SKILL.md" && grep -q 'mindspec panel create' "$S/ms-panel-run/SKILL.md" && grep -q 'Slot lens defaults' "$S/ms-panel-run/SKILL.md"` exits `0` (spec AC11, R8 — plugins copy)
- [ ] `S=.claude/skills; ! grep -q '| Condition | Action |' "$S/ms-panel-tally/SKILL.md" && grep -q 'mindspec panel tally' "$S/ms-panel-tally/SKILL.md" && ! grep -q '"reviewed_head_sha"' "$S/ms-panel-run/SKILL.md" && grep -q 'mindspec panel create' "$S/ms-panel-run/SKILL.md"` exits `0` (the `.claude` mirror is edited identically — no drift between the two copies)
- [ ] `diff -q plugins/mindspec/skills/ms-panel-run/SKILL.md .claude/skills/ms-panel-run/SKILL.md && diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` exits `0` (the mirrors stay byte-identical, as they are today)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/runbook.md'` (doc-sync)
- [ ] `go build ./...` exits `0` (no code touched — the tree still builds)

**Acceptance Criteria**
- [ ] The `| Condition | Action |` decision-matrix table is gone from
  `ms-panel-tally`, the hand-typed `panel.json` schema (`"reviewed_head_sha"`)
  is gone from `ms-panel-run` step 0, both skills reference the new verbs, and
  the named judgment sections (Artifact gates, Consolidate, Slot lens defaults)
  survive — in **both** the `plugins` and `.claude` copies (spec AC11, R8)

**Depends on**
Bead 4

## Risks / Sequencing

**In-flight hazard: spec 112 (per-gate panel config) lands first.** Spec 112 is
plan-approved with three unclaimed beads (`mindspec-lma4.1‖lma4.2→lma4.3`) that
merge to `main` **before** 110's beads are implemented. 112's touchpoints and
110's mechanical-rebase story:
- **`internal/config`** — 112 pointerizes `Reviewer.Count` (`int → *int` +
  exported `CountValue()`), exports `PanelGateKeys`, adds `PanelGateAdvisoryDefault`
  and the gate-scoped resolvers. **110 reads none of these**: `panel create`
  consumes only `PanelExpectedReviewers()` / `PanelApproveThresholdExpr()`
  (109's global resolvers, which 112 keeps), so 110 pins against the **109**
  config surface and a post-112 rebase touches no 110 line here.
- **`internal/panel`** — 112 adds a recorded, **decision-inert** `gate` field
  to `panel.Panel`. 110's `panel.Create` writes the `Panel` struct; after a
  112 rebase, `Create` may optionally also stamp `gate`, but its absence
  "costs nothing" (112's own contract), so 110's writer compiles and behaves
  identically whether or not the field exists. **No overlap in edited
  functions**: 112 edits `panel.go` (the struct + amendment note); 110 adds a
  new file `internal/panel/create.go`.
- **`internal/complete` + `cmd/mindspec`** — 112 makes the complete-gate
  advisory gate-aware and adds `config show --gate [--json]`. **110 touches
  neither** `internal/complete` nor `cmd/mindspec/config.go`; its `cmd`
  surface is a new file `cmd/mindspec/panel.go`. No function is edited by both
  specs.

**Bead boundaries keep the rebase mechanical.** Because 110's `internal/panel`
work is a **new file** (`create.go`), its `cmd` work is a **new file**
(`panel.go`), its `internal/validate` work appends checks to `ValidateSpec`
(untouched by 112), and its `internal/instruct` work is orthogonal to 112, a
post-112 rebase of `spec/110-…` is a clean fast-forward on every 110-owned
path. The one shared *symbol* is `panel.Panel` (112 adds a field, 110 writes
the struct) — additive on 112's side, so 110's `Create` continues to marshal a
superset-tolerant JSON with no edit required. If 110 lands **before** 112 (order
not guaranteed), the reverse also holds: 112's field-add is additive over
110's writer.

**Panel-substitution posture (unrelated to code):** per the standing model-
tiering protocol, the 9-reviewer spec/plan panels add 3× Fable; when Codex is
quota-walled, substitute Sonnet/Claude personas rather than block. This is an
orchestration note, not a plan constraint.

## Provenance

Spec ACs are numbered in the order they appear in the spec's Acceptance
Criteria checklist (spec.md lines 93–104). Every spec AC traces to a bead;
every requirement R1–R8 is delivered.

| Acceptance Criterion (spec) | Verified By |
|---------------------------|-------------|
| AC1 — `TestCreate_WritesRegistrationAtomically`: leaf writer records the fields in one `panel.json` write | Bead 1 verification (PASS-line grep) |
| AC2 — `TestPanelCreate_StampsResolversAndCoBumpsRoundSHA`: create stamps 109 resolvers + target SHA, `--round 2` co-bumps panel.json + BRIEF header, prior verdicts + skill body untouched | Bead 4 verification (PASS-line grep); writer mechanism proven in Bead 1 (`TestCreate_WritesRegistrationAtomically`) |
| AC3 — `TestPanelVerify_MatchesGateAndWritesNothing`: verify's PASS/BLOCK = `PanelGateDecision` Action, mutates nothing | Bead 4 verification (PASS-line grep) |
| AC4 — `TestPanelTally_ExitCodeTracksDecision`: exit 0 on Allow / non-zero + final recovery line on Block, printed decision = `PanelGateDecision` | Bead 4 verification (PASS-line grep) |
| AC5 — `TestPanelVerbs_DecisionIsPanelGateDecision`: both verbs render the identical `PanelGateDecision(facts)` Action over a facts table (R7a) | Bead 4 verification (PASS-line grep) |
| AC6 — `TestPanelStateVerdict_DelegatesToPanelGateDecision`: `instruct --panel-state` verdict = `PanelGateDecision`, old matrix gone (R2) | Bead 3 verification (PASS-line grep) |
| AC7 — `TestPanelSchemaDoc_MatchesConstants`: workflow schema doc consistent with `FileName`/`verdictFileRE`/`ConsolidatedName` (R4) | Bead 1 verification (PASS-line grep) |
| AC8 — `TestValidateSpec_ImpactedDomainSeverityMatchesPlanApprove`: path-like/ambiguous error, bare-name tolerated, real-dir passes (R5) | Bead 2 verification (PASS-line grep) |
| AC9 — `TestValidateSpec_ADRTouchpointExtractionBoundary`: anchored-missing fails, bare-prose ignored, 110-shaped passes, no coverage diagnostic (R6) | Bead 2 verification (PASS-line grep) |
| AC10 — `! go list -deps ./internal/panel | grep internal/config`: config-free leaf (R7b) | Bead 1 verification (asserted) + Bead 4 verification (re-asserted on the last bead to land) |
| AC11 — skills grep: decision-matrix table + hand-typed `panel.json` gone, verbs referenced, judgment sections survive (R8) | Bead 5 verification (plugins + `.claude` mirror greps + byte-identical `diff`) |
| AC12 — tree builds, touched packages fully green | every bead's `go build ./...` + per-package `go test`; full `go test ./...` regression at plan time and pre-`/ms-impl-approve` (Testing Strategy) |
| Validation Proof — `panel create` writes panel.json+BRIEF with captured SHA; `verify` report exit 0 git-clean; `tally` sub-threshold exits non-zero with re-panel recovery line | Bead 4 verification (manual e2e) |
| Validation Proof — mechanized prose gone from both skills | Bead 5 verification (skills greps) |
| Validation Proof — whole tree builds | every bead's `go build ./...` |
| R7c — `spec approve` emits no `adr-coverage-missing`; plan-level coverage stays plan-approve | Bead 2 verification (`TestValidateSpec_ADRTouchpointExtractionBoundary` no-coverage-diagnostic assertion) |
