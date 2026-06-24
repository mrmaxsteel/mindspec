---
approved_at: "2026-06-16T19:19:02Z"
approved_by: user
status: Approved
---
# Spec 100-ownership-gate-resolution: Ownership/ADR gate resolution + plan-validate ergonomics

## Goal

Make the ownership and ADR gates resolve each `## Impacted Domains` entry to its
owning domain NAME at a single shared source — a normalization helper that the
bead-time divergence gate AND the two plan-time gates (`checkADRCoverage`,
`checkADRCitations`) all call — so a present, correct manifest stops being
rejected and stops forcing `--override-adr` on every bead, and the same
file-path-Impacted-Domains spec stops failing `plan validate` with spurious
`adr-coverage-missing` / `adr-cite-irrelevant`. Alongside the core fix, remove the surrounding
friction that makes these gates hard to satisfy honestly: a misleading
`adr-coverage-missing` fix hint, the main-vs-worktree split in `adr show` /
`adr list`, and the undocumented `adr_citations` plan-frontmatter key. The
target outcome: an author with a real OWNERSHIP manifest and a real cited ADR
passes the gates with no override, and the CLIs/scaffolds tell them the truth
about how to satisfy the gate.

## Background

The bead-complete ADR-divergence gate (`internal/validate/divergence.go`) is
meant to attribute each changed file to an owning domain and then check that the
plan cites an Accepted ADR covering that domain. Attribution is delegated to
`attributeDomain` (`internal/validate/ownership.go`), which already glob-matches
a file path against a domain's `OWNERSHIP.yaml` `paths:` via `GlobMatch`. The
manifests are correct: e.g. the `workflow` domain owns `internal/validate/**`,
`cmd/**`, `internal/adr/**`, and `context-system` owns `internal/contextpack/**`.

The defect (bead mindspec-4ft2, GH #147): the set of domains glob-matching is
consulted against is `candidateDomains := append([]string(nil), meta.Domains...)`
(`divergence.go` ~L142) — the spec's `## Impacted Domains` bullets, taken
literally. `contextpack.ParseSpec` (`internal/contextpack/spec.go` ~L62-85)
stores each bullet as a bare domain NAME with no path normalization, and
`attributeDomain` -> `LoadOwnership(root, domain)` (`ownership.go` ~L78-80)
turns that name into `.mindspec/domains/<name>/OWNERSHIP.yaml` — a directory
lookup. So when a spec's Impacted Domains are FILE PATHS (e.g.
`genevieve/review.py`), the gate looks for
`.mindspec/domains/genevieve/review.py/OWNERSHIP.yaml`, finds nothing, and
every file in the bead diff is reported `adr-divergence-unowned` even though a
real `genevieve` domain whose `paths: [genevieve/**/*.py]` clearly owns the
file. The only escape is `--override-adr` on every bead, which defeats the gate.

The SAME root cause bites a SECOND set of gates the bead-time divergence fix
alone does not reach (this is literally #145's own symptom — #147 and #145 share
one root cause at two places). The plan-time gates iterate the raw
Impacted-Domains strings BY NAME: `checkADRCoverage` (`plan.go` ~L516, loops
`for _, d := range impactedDomains`) demands each entry name a domain with a
cited Accepted ADR, and `checkADRCitations` (`plan.go` ~L465) intersects each
cited ADR's `Domains` with those same raw entries (`adr-cite-irrelevant` on
empty intersection). When the entries are file paths, NO ADR declares a
file-path "domain", so a file-path-Impacted-Domains spec STILL fails `plan
validate` with spurious `adr-coverage-missing` and `adr-cite-irrelevant` even
after the divergence gate is fixed. Both plan-time gates consume
`loadImpactedDomains(specDir)` (`plan.go` ~L124), which returns the raw
`meta.Domains` unchanged — so the fix must normalize at THAT source, not at one
gate. Note these checks fire only against a PLAN's `adr_citations` frontmatter,
so `mindspec validate spec` (no plan) does not run them; they bite at `plan
validate` / `plan approve` time.

Three smaller friction defects compound this (bead cluster GH #145):

* mindspec-3d84 (#145.1): `adr-coverage-missing` (`plan.go` ~L529) always emits
  `run: mindspec adr create --domain <X>`, even when the correct fix is adding a
  `Domain(s)` field to an EXISTING cited Accepted ADR rather than authoring a new
  one.
* mindspec-3cfr (#145.2/.3): `adr show` (`cmd/mindspec/adr.go` ~L138) and
  `adr list` (~L97) resolve the repo root via `workspace.FindRoot`, which walks a
  worktree back to the MAIN checkout; `adr create` (~L32) and the plan VALIDATOR
  (`adrStoreForSpec`, fixed in mindspec-ew79) are worktree-aware via
  `FindLocalRoot`. So an ADR edited in a spec worktree is invisible to
  `adr show`, which looks like the ADR's `Domain(s)` is empty. The reporter's
  "#145.3 empty Domain(s)" symptom was this main-vs-worktree split, not a parse
  bug: `internal/adr/parse.go` ~L74 already accepts BOTH `**Domain(s)**:` and
  `- **Domain(s)**:` via `strings.Contains`.
* mindspec-gpoq (#145.4): the plan scaffold (`internal/approve/spec.go`
  `scaffoldPlan` ~L228-263) emits frontmatter with `status`/`spec_id`/`version`
  but NO `adr_citations` key, and the `adr-citations` WARN (`plan.go` ~L141) never
  names the `adr_citations` frontmatter key. So an author has no scaffolded slot
  and no hint about the exact key the gate reads.

## Impacted Domains

- workflow: every changed source file lands here. Under normalize-at-source
  (R1) ParseSpec stays dumb — `internal/contextpack/spec.go` is NOT touched — so
  the only changed files are the gate logic, the CLIs, and the shared
  normalization helper, all owned by `workflow`: the new normalization helper
  and `divergence.go`, `plan.go`, `ownership.go` under `internal/validate/**`;
  `internal/adr/**` (`parse.go` regression test, R3); `internal/approve/**`
  (`scaffoldPlan`, R4); and `cmd/**` (`cmd/mindspec/adr.go` for `adr
  show`/`adr list`, R3). The `workflow` `OWNERSHIP.yaml` `paths:` claims all of
  these via `internal/validate/**`, `internal/ownership/**`, `internal/adr/**`,
  `internal/approve/**`, and `cmd/**` — re-verified against the globs (the helper
  in `internal/validate/` or `internal/ownership/` is covered either way). The
  ADR-0032 amendment is a doc under `.mindspec/adr/**`, which the divergence
  gate treats as a process artifact (`isProcessArtifact` -> `isDocFile`) and
  skips, so it needs no domain claim. `context-system` is intentionally NOT
  declared: the original R1 listed it only for the `internal/contextpack/spec.go`
  change that normalize-at-source removes; declaring an Impacted Domain with no
  changed file would be dishonest scope. ADR-0024 (cited for R3) still covers
  this spec because its `Domains` include `workflow`.

## ADR Touchpoints

- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md): Zero Framework Cognition
  ownership discovery — the canonical decision that ownership is resolved from an
  explicit per-domain `OWNERSHIP.yaml` `paths:` glob, with no framework-synthesized
  fallback. Status **Accepted**; Domain(s) **workflow, validation, doc-sync,
  ownership** — intersects the impacted **workflow** domain. R1's normalization
  helper applies this ADR: it resolves an Impacted-Domains entry to its owner by
  glob-matching the entry against every domain's EXPLICIT `paths:` manifest — it
  consumes declared data and explicit globs, it does not synthesize a fallback.
- [ADR-0032](../../adr/ADR-0032-adr-semantic-gates.md): ADR semantic gates — the
  decision that the divergence/coverage/citation gates attribute files to domains
  and check cited-ADR coverage, and (sub-decision 1) that the domain identifier is
  the `OWNERSHIP.yaml` directory name, with path-like identifiers REJECTED as
  ambiguous. Status **Accepted**; Domain(s) **validation, adr, lifecycle**.
  **This spec AMENDS ADR-0032** (the way spec 099 amended ADR-0037): R1
  normalize-at-source ACCEPTS a path-like Impacted-Domains entry and RESOLVES it
  to its owning-domain dir-name when exactly one domain's `OWNERSHIP.yaml` claims
  it, rather than rejecting it outright — erroring only when zero or more-than-one
  domains own it. The spec adds an amendment note to ADR-0032 recording this
  softening of sub-decision 1 (path-like entries are normalized, not rejected,
  when unambiguously owned). R2 (coverage-missing hint) and R4 (`adr_citations`
  key) operate inside the mechanism this ADR defines without changing it.
- [ADR-0024](../../adr/ADR-0024.md): ADR storage abstraction (interface-first,
  file-based default) — the `adr.Store` / `FileStore` boundary that `adr show` /
  `adr list` read through. Status **Accepted**; Domain(s) **adr, context-system,
  workflow** — intersects the impacted **workflow** domain. R3's worktree-aware
  root resolution makes the show/list read path agree with the worktree-overlay
  store the validator already uses.

## Requirements

1. **(bead mindspec-4ft2) Normalize Impacted-Domains entries to owning domain
   NAMES at one shared source, consumed by all three ADR gates.** A single
   shared normalization helper (placed in `internal/validate` — or
   `internal/ownership` — so BOTH `divergence.go` and `plan.go` can call it
   without breaking layering; ParseSpec/`contextpack` is NOT made to depend on
   `validate`) MUST resolve each raw `## Impacted Domains` entry to its owning
   domain NAME by the following rule:
   - an entry that already names a domain directory (a domain dir whose
     `.mindspec/domains/<entry>/OWNERSHIP.yaml` exists) is KEPT verbatim;
   - any other entry — a file path, or any string that is not a domain dir name —
     is glob-matched against every domain's `OWNERSHIP.yaml` `paths:` (reusing the
     existing domain enumeration — `resolveDomains` / `listDomainDirs` — and
     `GlobMatch`) and REPLACED with the owning domain's NAME;
   - an entry that resolves to ZERO domains is an ERROR naming the entry and
     stating that no `OWNERSHIP.yaml` claims it;
   - an entry that resolves to MORE THAN ONE domain is an ambiguity ERROR naming
     the entry and the conflicting owners.

   After normalization, the domain set the gates consume is a clean domain-NAME
   set. The bead-time divergence gate (`divergence.go` ~L142, where
   `candidateDomains` is built from `meta.Domains`) AND the two plan-time gates
   (`checkADRCoverage` and `checkADRCitations`, fed by `loadImpactedDomains` at
   `plan.go` ~L124) MUST consume the normalized set, so a file-path
   Impacted-Domains spec passes the divergence gate (no spurious
   `adr-divergence-unowned`) AND the plan gates (no spurious
   `adr-coverage-missing` / `adr-cite-irrelevant`) — the cross-gate consistency
   the original single-gate fix only claimed.

   The existing per-file attribution and blast-radius guard are PRESERVED, NOT
   loosened. The divergence gate still attributes each CHANGED FILE to a domain
   via `attributeDomain` against the normalized DECLARED set; a changed file owned
   by a domain NOT in that resolved declared set still fails (exactly as today —
   `TestCompleteRejectsUndeclaredDomainTouch` keeps its original intent). The
   NAMED-domain path is unchanged: a spec declaring `workflow`/`execution` by name
   resolves identically (each already names a domain dir, so normalization keeps
   it verbatim).

   **Acceptance Criteria**
   - [ ] A hermetic test builds a temp repo with a domain dir whose
     `OWNERSHIP.yaml` has `paths:` covering a changed file, a `spec.md` whose
     `## Impacted Domains` is a FILE PATH (not the domain dir name), and a plan
     citing an Accepted ADR declaring that domain; the BEAD-TIME divergence gate
     resolves the entry to the owning domain and reports ZERO
     `adr-divergence-unowned` findings (`coveredAccepted` -> silent pass).
   - [ ] A hermetic test asserts the SAME file-path-Impacted-Domains spec ALSO
     passes the PLAN-TIME gates: `checkADRCoverage` emits no spurious
     `adr-coverage-missing` and `checkADRCitations` emits no spurious
     `adr-cite-irrelevant`, because both consume the normalized domain NAME.
   - [ ] A hermetic regression test asserts the named-domain path (099-style) is
     unchanged: a spec declaring the owning domain BY NAME with the same manifest
     and citation still resolves and still passes — no new false positives — and
     `TestCompleteRejectsUndeclaredDomainTouch` (a changed file in a domain the
     spec never declared) still fails with its original `adr-divergence-uncovered`
     intent (the blast-radius guard is not silently dropped).
   - [ ] A hermetic test asserts a genuinely unowned changed file (no domain
     manifest `paths:` matches it) still reports `adr-divergence-unowned` — the
     gate is not globally disabled, only correctly scoped.
   - [ ] A hermetic test asserts an Impacted-Domains entry owned by NO domain (and
     a second case owned by MORE THAN ONE) produces the clear normalization ERROR
     naming the entry (zero-owner) / the conflicting owners (ambiguous).

2. **(bead mindspec-3d84) `adr-coverage-missing` hint covers the
   add-Domain-to-existing-ADR fix.** When an impacted domain is uncovered but the
   plan already cites one or more Accepted ADRs, the `adr-coverage-missing`
   diagnostic (`plan.go` ~L529) MUST present BOTH legitimate remedies — adding a
   `Domain(s)` entry to an existing cited Accepted ADR, and creating a new ADR —
   rather than implying `mindspec adr create` is the only option.

   **Acceptance Criteria**
   - [ ] A hermetic test on `checkADRCoverage` / `ValidatePlan` over a fixture
     with an impacted domain not declared by any cited ADR, where the plan DOES
     cite an Accepted ADR, asserts the `adr-coverage-missing` message text mentions
     adding/declaring the domain on an existing cited ADR (not only `adr create`).
   - [ ] A hermetic test asserts that when NO ADR is cited at all, the message still
     surfaces the `adr create` remedy (the create path is not lost).

3. **(bead mindspec-3cfr) `adr show` / `adr list` are worktree-aware; lock the
   non-list `Domain(s)` parse with a regression test.** `adr show`
   (`cmd/mindspec/adr.go` ~L138) and `adr list` (~L97) MUST resolve the read root
   so that an ADR present in the ACTIVE worktree (spec/bead worktree) is visible —
   consistent with `adr create` (`FindLocalRoot`) and the validator's
   `adrStoreForSpec` overlay — instead of always resolving back to the main
   checkout via `FindRoot`. This requirement MUST NOT modify the ADR parser; it
   adds a regression test locking the existing behaviour that `parse.go` accepts
   the non-list `**Domain(s)**:` form (as well as `- **Domain(s)**:`).

   **Acceptance Criteria**
   - [ ] A hermetic test exercising the `adr show` (and `adr list`) root-resolution
     helper asserts an ADR that exists only in the worktree-local
     `.mindspec/adr/` is found and rendered with its `Domain(s)`, where the
     pre-fix `FindRoot` path would have missed it.
   - [ ] A hermetic `ParseADR` regression test asserts an ADR whose Domain line is
     the non-list `**Domain(s)**: foo, bar` form parses to
     `Domains == ["foo","bar"]` (current behaviour is locked; the parser is
     unchanged).

4. **(bead mindspec-gpoq) Scaffold and document the `adr_citations` plan key.**
   The plan scaffold (`internal/approve/spec.go` `scaffoldPlan`) MUST emit an
   `adr_citations` entry in the generated frontmatter (commented or empty-but-named
   so the author sees the exact key the gate reads), and the plan `adr-citations`
   diagnostic (`plan.go` ~L141) MUST name the `adr_citations` frontmatter key in
   its message so the fix is unambiguous.

   **Acceptance Criteria**
   - [ ] A hermetic test asserts `scaffoldPlan(specID)` output contains the literal
     `adr_citations` key in the YAML frontmatter region.
   - [ ] A hermetic test asserts the `adr-citations` WARN/ERROR message text emitted
     by `ValidatePlan` (empty-citations branch) contains the string `adr_citations`.

## Scope

### In Scope
- The shared normalization helper (R1) — a NEW helper in `internal/validate`
  (or `internal/ownership`; both are owned by `workflow`) that enumerates domain
  dirs, keeps an entry that already names a domain dir, glob-matches every other
  entry against each domain's `OWNERSHIP.yaml` `paths:`, replaces it with the
  owning domain NAME, and errors on zero / more-than-one owners. Placed so
  `divergence.go` AND `plan.go` both call it without a `contextpack -> validate`
  dependency.
- `internal/validate/divergence.go` — `candidateDomains` is built from the
  NORMALIZED declared set; per-file attribution + blast-radius guard unchanged
  (R1).
- `internal/validate/plan.go` — `checkADRCoverage` + `checkADRCitations` consume
  the NORMALIZED domain set from `loadImpactedDomains` (R1);
  `adr-coverage-missing` hint text (R2); `adr-citations` message naming the
  `adr_citations` key (R4).
- `internal/validate/ownership.go` — `attributeDomain` / `GlobMatch` /
  `resolveDomains` reused by the helper; no behavioral change to attribution (R1).
- `internal/adr/parse.go` — TEST ONLY: regression test for the non-list
  `**Domain(s)**:` form (R3). No parser change.
- `cmd/mindspec/adr.go` — worktree-aware root resolution for `adr show` /
  `adr list` (R3).
- `internal/approve/spec.go` — `scaffoldPlan` emits the `adr_citations` key (R4).
- `.mindspec/adr/ADR-0032-adr-semantic-gates.md` — amendment note: path-like
  Impacted-Domains entries are normalized to their owning-domain dir-name when
  unambiguously owned, erroring only on zero / more-than-one owners (R1). This is
  a doc/process artifact (skipped by the divergence gate), not owned source.
- NOT changed: `internal/contextpack/spec.go` — ParseSpec stays dumb (stores the
  raw Impacted-Domains entries verbatim); normalization happens in the gates, not
  the parser.

### Out of Scope
- The GH #146 `mindspec next` ergonomics beads — a separate spec.
- The GH #76 branch-base / diff-base selection work — a separate spec.
- Any change to `internal/adr/parse.go` parser logic — R3 is test-only; the
  parser already accepts both `Domain(s)` forms.
- Any change to `internal/contextpack/spec.go` `ParseSpec` — it stays dumb and
  stores raw Impacted-Domains entries; the gates normalize.
- Changing the `OWNERSHIP.yaml` schema, the `GlobMatch` glob dialect, or the
  excluded-first-segment (viz/agentmind/bench) rules.
- The doc-sync attribution gate's own pass/fail semantics (this spec reuses its
  attribution helpers but does not alter doc-sync's rules).

## Non-Goals

- This spec normalizes Impacted-Domains entries to owning domain NAMES by
  glob-matching against EXPLICIT `OWNERSHIP.yaml` manifests, but does NOT
  introduce path-PREFIX inference (mapping an entry to a domain by longest
  path-prefix). See the R1 design decision below: prefix inference is rejected as
  framework cognition; manifest glob-matching is not.
- This spec does NOT relax or remove the divergence/coverage gates — a genuinely
  unowned file or a genuinely uncovered domain still fails.
- This spec does NOT change how `--override-adr` / `--supersede-adr` behave; it
  removes the NEED to reach for them in the false-unowned case, not the escape
  hatch itself.

## Acceptance Criteria

- [ ] R1: a shared helper normalizes Impacted-Domains entries to owning domain
  NAMES (glob-matching path-like entries against `OWNERSHIP.yaml`, erroring on
  zero / more-than-one owners); the divergence gate AND the plan-time
  coverage + citation gates consume the normalized set, so a file-path entry no
  longer yields a spurious `adr-divergence-unowned` / `adr-coverage-missing` /
  `adr-cite-irrelevant`, while named-domain specs, the blast-radius guard, and
  genuinely-unowned files behave exactly as before (hermetic tests).
- [ ] R2: `adr-coverage-missing` presents both the add-Domain-to-existing-ADR and
  the create-new-ADR remedies (hermetic message-text test).
- [ ] R3: `adr show` / `adr list` find a worktree-local ADR, and a regression test
  locks the non-list `Domain(s)` parse (hermetic tests; parser unchanged).
- [ ] R4: the plan scaffold emits `adr_citations` and the `adr-citations`
  diagnostic names that key (hermetic tests).
- [ ] `go build ./...` succeeds and the new hermetic tests pass under
  `go test -run <Name> -timeout 120s ./...` (no LLM-harness scenario required).

## Validation Proofs

- `mindspec validate spec 100-ownership-gate-resolution`: 0 errors (a
  `lifecycle-binding` WARN before `spec approve` is acceptable).
- `go build ./...`: succeeds after implementation.
- `go test -run 'TestValidateDivergence|TestNormalizeImpactedDomains' -timeout 120s ./internal/validate/...`:
  R1 tests pass (new tests named `TestValidateDivergence*` for the gate cases and
  `TestNormalizeImpactedDomains*` for the helper's keep / glob-resolve / zero-owner
  / ambiguous-owner cases; file-path Impacted Domains resolve at the divergence
  gate; named-domain unchanged; genuinely-unowned still fails). The existing
  `TestCompleteRejectsUndeclaredDomainTouch` and `TestUnownedFileRejected` must
  still pass (blast-radius guard intact).
- `go test -run 'TestPlanCoverage|TestCheckADRCitations|TestValidatePlan' -timeout 120s ./internal/validate/...`:
  R1 plan-gate consistency (new `TestPlanCoverage*` / `TestCheckADRCitations*` case
  asserting a file-path Impacted-Domains spec emits no spurious
  `adr-coverage-missing` / `adr-cite-irrelevant`) + R2/R4 message-text tests pass.
- `go test -run 'TestAdrShowWorktree|TestParseADR' -timeout 120s ./...`: R3
  worktree-aware show (new test named `TestAdrShowWorktree*`) + the existing
  `TestParseADR` non-list `Domain(s)` regression pass.

## Open Questions

None. (Design decision for R1 is recorded below; the chosen normalize-at-source
approach is the ZFC-clean reading of ADR-0036, fixes all three gates, preserves
the blast-radius guard, and is recorded as an amendment to ADR-0032.)

## R1 Design Decision (mindspec-4ft2)

**Decision: normalize-at-source.** Resolve each raw `## Impacted Domains` entry
to its owning domain NAME via a SINGLE shared helper that ALL THREE consuming
gates call (bead-time divergence, plan-time `checkADRCoverage`, plan-time
`checkADRCitations`). An entry that already names a domain dir is kept verbatim;
any other entry (a file path, or any non-dir-name string) is glob-matched against
every domain's `OWNERSHIP.yaml` `paths:` and replaced with the owning domain's
name. An entry owned by ZERO domains, or by MORE THAN ONE domain (ambiguous
overlap), is a clear ERROR naming the entry. After normalization, the domain set
the gates consume is a clean domain-NAME set. ParseSpec stays dumb (stores raw
entries); normalization lives in the gate layer (`internal/validate` /
`internal/ownership`), so there is no `contextpack -> validate` dependency.

**Why this beats the original single-gate fix (recorded per panel):** the
original R1 ("glob-match changed files across ALL domains at the bead-time
divergence gate, treat Impacted Domains as advisory") was:

1. INCOMPLETE — it only fixed the bead-time divergence gate (`divergence.go`
   ~L142). The plan-time gates `checkADRCoverage` (`plan.go` ~L516) and
   `checkADRCitations` (`plan.go` ~L465) still iterate the raw file-path strings
   BY NAME, so a file-path-Impacted-Domains spec STILL failed `plan validate` with
   spurious `adr-coverage-missing` / `adr-cite-irrelevant`. This is literally
   #145's own symptom — #147 and #145 share this one root cause at two gates.
2. SILENTLY DROPPED the blast-radius guard — attributing changed files against
   ALL domains (instead of the declared set) means a changed file in a domain the
   spec never declared resolves to an owner and passes, defeating
   `TestCompleteRejectsUndeclaredDomainTouch` (the Spec 087 guard that rejects
   editing an undeclared domain).
3. CONTRADICTED ADR-0032 sub-decision 1 (which mandates dir-name identifiers and
   REJECTS path-like as ambiguous) while over-claiming ADR-0036 support.

Normalize-at-source fixes all three: the shared helper feeds a clean domain-NAME
set to divergence + coverage + citations (the cross-gate consistency the original
only claimed); the existing per-file attribution + blast-radius guard stay
UNCHANGED (the candidate set is the resolved DECLARED domains, so a changed file
owned by an undeclared domain still fails — no silent loosening); and it conforms
to ADR-0032 by CONVERTING path-like entries to dir-names (and recording the
amendment) rather than leaving the gates to choke on them.

**Why this is ZFC-clean (ADR-0036):** glob-matching a DECLARED Impacted-Domains
entry against EXPLICIT per-domain `OWNERSHIP.yaml` manifests consumes declared
data and explicit globs — no guessing, no synthesized fallback. The
zero-owner / multi-owner ERROR keeps the resolution total and unambiguous; the
framework never invents an owner.

**Rejected alternatives:**

- (a) *Glob-across-ALL-changed-files at the bead-time gate only* (the original
  R1): INCOMPLETE (leaves the two plan-time gates broken), DROPS the blast-radius
  guard, and sits in tension with ADR-0032. Rejected — see above.
- (b) *Reject path-like entries at `spec approve` (strict ADR-0032 sub-decision
  1)*: considered, and the cleanest from a "one canonical identifier" view, but
  more hostile — it forces the author to rewrite a working spec (the
  genevieve-style file-path case) instead of resolving it. Normalize-at-source is
  more forgiving: it auto-resolves the unambiguous case and errors only on
  genuine 0/>1-owner ambiguity. Rejected in favor of normalize + amendment.
- (c) *Path-PREFIX normalization* (map an entry to a domain by longest
  path-prefix): re-introduces framework inference — guessing a domain from a
  prefix is exactly the synthesized-fallback ZFC violation ADR-0036 removed.
  Rejected. (Normalize-at-source matches against EXPLICIT `paths:` globs, not an
  inferred prefix.)

**ADR-0032 amendment plan:** ADR-0032 sub-decision 1 currently REJECTS path-like
identifiers as ambiguous. This spec adds an amendment note (mirroring spec 099's
amendment of ADR-0037) stating that path-like Impacted-Domains entries are
NORMALIZED to their owning-domain dir-name when exactly one domain's
`OWNERSHIP.yaml` claims them, and error only when zero or more-than-one domains
own them — so the canonical dir-name identifier is still what the gates compare,
reached by resolution rather than by rejecting the author's path.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-16
- **Notes**: Approved via mindspec approve spec