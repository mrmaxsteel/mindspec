---
approved_at: "2026-06-13T14:35:01Z"
approved_by: user
status: Approved
---
# Spec 097-code-review-cleanup: Code-review cleanup (residual genuinely-open m557 findings)

## Goal

Remediate the seven genuinely-open residual findings from the 2026-05-14
multi-agent code review (epic `mindspec-m557`) that survive in the current
tree. The work closes one defense-in-depth security gap in the git argument
builder, retires four heuristic prose-scraping paths in favour of structured
plan frontmatter (Zero Framework Cognition), consolidates a stale two-package
split, and tightens one identifier validator plus unifies validator error
wording. Because two of the touched packages are currently unowned, the spec
also lands a dedicated ownership-claim bead FIRST so the later beads do not
hard-block at the `adr-divergence-unowned` gate. The outcome: an agent or
operator can trust that controlled-input git operations refuse hostile ref
operands, that plan-derived structured data comes from declared frontmatter
rather than guessed-from-prose regexes, and that the ID validators are
consistent and correctly strict — with no regression to any currently-valid
workflow.

## Background

The m557 review produced roughly 23 findings. A re-audit on 2026-06-12
confirmed that ~16 were already remediated and closed by specs 091–096; the
seven addressed here were each re-confirmed still-open against the current tree,
and every cited line below was re-opened and verified for this spec
(verify-don't-trust). The findings cluster into three sub-themes:

- **A — git argument safety (security, SEC-5 / bead `obxo`).** `internal/gitutil`
  passes ref/branch operands into `git` argv positional slots with no `--`
  separator and no rejection of `-`-prefixed values; a ref literally named
  `--upload-pack=…` or `-x` would be parsed as a git option. All current callers
  pass controlled refs (`main`, `spec/<id>`, `bead/<id>`), so this is
  defense-in-depth, not a live exploit — but the package is the I/O boundary
  (ADR-0030) and should reject hostile operands at its own edge.

- **B — stop extracting structured data from prose (ZFC, findings ZFC-5/6/7,
  beads `vbij` / `o2yk` / `4axk`).** Three code paths regex-scrape markdown PROSE
  for data that should be declared in structured plan frontmatter: ADR IDs for
  the bead `--design` field, bead-to-bead dependencies, and key file paths.
  Parallel structured fields already exist (`PlanFrontmatter.ADRCitations`) or
  are stubbed-but-dead (`work_chunks[*].depends_on` in the user template and in
  `internal/approve/plan_test.go` fixtures, parsed by nothing today). This
  violates the Zero Framework Cognition stance recorded in ADR-0036 (heuristic
  classification is forbidden; semantic data is declared, not guessed).

- **C — refactor & validation polish (findings ARCH-5 / idvalidate, beads
  `0yz3` / `bzb9` / `yqra`).** `internal/spec` and `internal/speclist` remain two
  packages after the duplicate parser that motivated the split was already
  removed by ARCH-6; the `domainNamePattern` still accepts trailing-hyphen
  names; and the four `idvalidate` validators use three inconsistent
  error-message conventions with one under-documented format assumption.

**Ownership prerequisite (the binding gate).** Two of the packages touched here
— `internal/idvalidate/**` (R6, R7) and `internal/speclist/**` (the source side
of the R5 merge) — are claimed by NO `OWNERSHIP.yaml`. The hard block this
creates is the **`adr-divergence-unowned` ERROR** at
`internal/validate/divergence.go:196` (`r.AddError`), NOT the advisory
`unclaimed-source` Warn at `internal/validate/docsync.go:280` (`r.AddWarning`).
`ValidateDivergence` reads `git diff --name-only base..head` (which includes
DELETED paths) and attributes each path by its literal OLD path string; a path
that no impacted-domain manifest claims attributes to domain `""` and raises the
ERROR. This fires (a) at each consuming bead's `mindspec complete`
(base = merge-base(spec, beadHead)) and (b) again at `impl-approve`, whose
whole-branch diff always re-contains R5's `internal/speclist/*.go` deletions.
Therefore an ownership-claim bead (R0 below) must land on the spec branch BEFORE
the consuming beads. This is the same unowned-package wall that bit spec 096.

## Impacted Domains

- execution: `internal/gitutil/**` (Requirement 1, the git argument-safety
  guard) and `internal/harness/**` (Requirement 4's deferred secondary site) are
  claimed by the execution OWNERSHIP manifest. Per-requirement detail:
  * Requirement 1 adds a package-boundary ref/branch operand guard plus `--`
    separators on single-ref subcommands in `internal/gitutil/gitops.go`.
  * Requirement 4 (deferred secondary site) `internal/harness/analyzer.go` is
    NOT changed by this spec; it is split to a named follow-up bead.
- workflow: `internal/approve/**`, `internal/validate/**`, and
  `internal/instruct/**` (Requirements 2, 3, 4) are claimed by the workflow
  OWNERSHIP manifest. Per-requirement detail:
  * Requirement 2 makes `internal/approve/plan.go` consume the structured
    `ADRCitations` frontmatter instead of regex-scraping the `## ADR
    Touchpoints` prose for the bead `--design` field.
  * Requirement 3 declares bead dependencies in structured plan frontmatter and
    consumes that in both `internal/approve/plan.go` and
    `internal/validate/plan.go`, retiring the duplicated `bead\s+(\d+)` regex.
  * Requirements 3 and 4 update the active plan template
    `internal/instruct/templates/plan.md` (workflow-owned) so future plans emit
    the structured fields.
- context-system: `internal/contextpack/**` (Requirement 4, primary site) is
  claimed by the context-system OWNERSHIP manifest. Per-requirement detail:
  * Requirement 4 (primary site) sources `## Key File Paths` from a structured
    `key_file_paths` plan-frontmatter field rather than
    `ExtractFilePathsFromText`'s hard-coded `{internal/,cmd/,pkg/}` prefix scan
    in `internal/contextpack/builder.go`.
- core: `internal/spec/**` (Requirement 5) is already claimed by the core
  OWNERSHIP manifest; Requirement 0 additionally adds `internal/idvalidate/**`
  (Requirements 6, 7) and `internal/speclist/**` (Requirement 5 source side) to
  the same core manifest. Per-requirement detail:
  * Requirement 0 edits `.mindspec/domains/core/OWNERSHIP.yaml` to claim
    `internal/idvalidate/**` and `internal/speclist/**`.
  * Requirement 5 merges `internal/speclist` into the core-owned `internal/spec`
    package (a pure refactor, no behavior change).

> **Ownership resolution (settled — see Requirement 0):** `internal/idvalidate/**`
> (Requirements 6, 7) and `internal/speclist/**` (Requirement 5, the source side
> of the merge) are NOT claimed by any `OWNERSHIP.yaml` today. Requirement 0
> claims BOTH under the core domain in a dedicated FIRST bead. `internal/spec/**`
> is already core-owned, so the merge end-state stays covered; the
> `internal/speclist/**` glob is retained through `impl-approve` (a dangling glob
> over a now-empty dir is harmless, and the whole-branch diff re-contains the
> deletions, which must keep attributing to core). The active plan template
> `internal/instruct/**` is already workflow-owned and the user template
> `project-docs/user/templates/plan.md` is a `.mindspec/docs/**` process
> artifact skipped by divergence, so the template edits in R3/R4 raise no
> ownership gap. All claimed globs map to core/workflow, covered by Accepted
> ADR-0035 / ADR-0036 — no new ADR or new domain is needed.

## ADR Touchpoints

- [ADR-0030](../../adr/ADR-0030-executor-boundary.md) (Accepted; Domains:
  execution, …): Establishes `internal/gitutil` / the executor as the Git/Process
  I/O boundary for enforcement packages. Requirement 1's operand guard belongs
  at exactly this boundary — the package validates its own argv before shelling
  out. Covers the **execution** impacted domain.
- [ADR-0035](../../adr/ADR-0035-agent-error-contract.md) (Accepted; Domains:
  workflow, execution, core): The agent error contract — guard failures must
  state what to RUN (a recovery line), not just what is wrong. Requirement 1's
  rejection error for a `-`-prefixed ref MUST carry an ADR-0035-shaped recovery
  line. Covers the **core** impacted domain (and reinforces execution/workflow);
  the `internal/idvalidate/**` and `internal/speclist/**` claims added by
  Requirement 0 attribute to **core** under this Accepted ADR.
- [ADR-0036](../../adr/ADR-0036-ownership-discovery.md) (Accepted; Domains:
  workflow, validation, doc-sync, ownership): Records the **Zero Framework
  Cognition** stance — heuristic classification is forbidden; semantic decisions
  are declared by an agent/operator, never guessed by framework code.
  Requirements 2, 3, and 4 are direct applications of this principle (retire
  prose-scraping regexes in favour of declared structured frontmatter). Covers
  the **workflow** impacted domain.
- [ADR-0033](../../adr/ADR-0033-tokenizer-interface.md) (Accepted; Domains:
  context-system): Governs deterministic context-pack construction in
  `internal/contextpack`. Requirement 4's primary site lives in this package and
  this domain. Covers the **context-system** impacted domain.

> **ZFC-theme ADR coverage (settled — non-gating):** there is no ADR that
> specifically codifies "declared structured frontmatter is the source of truth
> over prose extraction"; ADR-0036 codifies the underlying ZFC *principle* but
> lists only `workflow, validation, doc-sync, ownership` in its `Domains`. This
> is NOT a gate concern: `adr-divergence` checks DOMAIN coverage, not THEME
> coverage (`coverageOf`, divergence.go:208). Requirement 4's changed files
> attribute to **context-system** (`internal/contextpack` → ADR-0033, Accepted)
> and the deferred harness site would attribute to **execution**
> (`internal/harness` → ADR-0030/0035, Accepted). Every impacted domain therefore
> already maps to a cited Accepted ADR. Widening ADR-0036's `Domains` (or adding
> a new ZFC ADR) is optional governance hygiene and is intentionally NOT in scope
> — do not spend a bead on it.

## Requirements

0. **Ownership prerequisite — claim the two unowned packages FIRST (gate
   precondition for R5/R6/R7).**
   *Threat:* `internal/idvalidate/**` (R6, R7) and `internal/speclist/**` (R5
   source side) are claimed by no `OWNERSHIP.yaml`. Touching or deleting them
   raises the `adr-divergence-unowned` ERROR (divergence.go:196) at the
   consuming bead's `mindspec complete` AND at `impl-approve` (whose whole-branch
   diff re-contains R5's deletions of the OLD `internal/speclist/*.go` paths —
   `internal/spec/**` does NOT glob-match `internal/speclist/...`).
   *What must be true after:* a single doc-only bead edits
   `.mindspec/domains/core/OWNERSHIP.yaml` to add BOTH
   `internal/idvalidate/**` and `internal/speclist/**`, and that bead merges to
   the spec branch BEFORE R5/R6/R7. Both globs map to core, covered by Accepted
   ADR-0035, so once claimed the deletions and edits attribute to core and pass
   `coverageOf`. The `internal/speclist/**` glob is KEPT through `impl-approve`
   (a dangling glob over the emptied dir is harmless; the impl-approve diff
   always re-contains the deletions and must keep attributing to core). The
   OWNERSHIP.yaml edit is itself a process artifact skipped by both divergence
   (`isProcessArtifact`) and doc-sync, so the ownership bead passes its own gates.

1. **Git argument-safety guard at the gitutil boundary (SEC-5 / `obxo`,
   behavior-affecting at the hostile-input edge only).**
   *Threat:* `internal/gitutil/gitops.go` passes refs/branches in positional
   argv slots with no `--` separator and no rejection of `-`-prefixed operands.
   Verified current sites: `MergeBranch` checkout `:52` (`"checkout", target`)
   and merge `:58` (`"merge", "--no-ff", source, …`); `MergeInto` merge `:71`;
   `DiffStat` `:202` (`"diff", "--stat", base+".."+head`); `CheckoutNewBranch`
   `:555` (`gitArgs(workdir, "checkout", "-b", branch)`); `CreateBranch` `:40`
   (`"branch", name, from`); and the shared builder `gitArgs` `:274` performs no
   validation. A ref named `-x` or `--upload-pack=…` would be parsed as a git
   option.
   *What must be true after:* (a) a package-boundary guard REJECTS any
   ref/branch/refspec operand beginning with `-`, returning a clear error that
   carries an ADR-0035-shaped recovery line; (b) a `--` separator is inserted
   before the single ref operand on the single-ref subcommands (checkout,
   merge, branch / `checkout -b`); (c) **subtlety — a revision-range operand
   like `base+".."+head` (DiffStat, CommitCount) MUST NOT receive a `--`**,
   because `--` would reinterpret it as a pathspec; range sites rely on the
   leading-`-` guard alone. (d) All currently-controlled refs
   (`main`, `spec/<id>`, `bead/<id>`) continue to work unchanged. This is
   defense-in-depth; the only behavior change is that a `-`-prefixed operand now
   errors instead of being passed to git. R1 is independent of all other
   requirements (disjoint files).

2. **Consume structured `ADRCitations` for the bead `--design` field (ZFC-7 /
   `4axk`, behavior-affecting; forward-only, non-gating).**
   *Motivation:* `internal/approve/plan.go` regex-scrapes the `## ADR
   Touchpoints` PROSE for ADR IDs. Verified: `adrIDRe = regexp.MustCompile("ADR-(\\d{4})")`
   at `:596`, `parseADRIDs` at `:599`, called from `buildDesignField` at `:572`
   via `contextpack.ExtractSection(specContent, "ADR Touchpoints")`. This runs
   parallel to the already-parsed structured `PlanFrontmatter.ADRCitations`
   (`internal/validate/plan.go:29`), which is unused for this purpose.
   *Source-document shift (state explicitly):* the prose source is the SPEC's
   `## ADR Touchpoints` (`specContent`); the structured replacement
   `PlanFrontmatter.ADRCitations` lives in the PLAN. R2 therefore shifts the
   `--design` ADR source from spec-prose to plan-frontmatter. This is
   **forward-only and non-gating**: `approve` runs once per plan, so the 14
   currently-approved plans that cite ADRs only in prose with no `adr_citations`
   are not re-approved and carry no live regression.
   *What must be true after:* the bead `--design` field's ADR list is built from
   the structured `ADRCitations` frontmatter (the validated source of truth),
   not from a prose regex. The prose-scraping path (`adrIDRe` / `parseADRIDs`) is
   retired or no longer drives the design field. *Intended behavior change:* ADR
   IDs that appear only in prose but not in declared `adr_citations` are no
   longer harvested into `--design`; this is the intended drop of prose-only IDs
   — the frontmatter is the contract that the plan-validation gate already
   enforces. *Optional safeguard:* a validation assertion that a plan's
   `adr_citations` is a SUPERSET of the IDs in its spec's `## ADR Touchpoints`
   may be added so no relied-upon ADR is silently dropped; if not added, the
   intended drop is documented here.

3. **Declare and consume bead dependencies in structured plan frontmatter
   (ZFC-6 / `o2yk`, behavior-affecting; ATOMIC cutover required).**
   *Motivation:* the bead-dependency regex `(?i)bead\s+(\d+)` is DUPLICATED in
   `internal/approve/plan.go:373` (`depRe`, wires prose deps into bd
   dependencies via `bd dep add` at `:394` — the behavior-critical consumer) and
   `internal/validate/plan.go:819` (`beadDepRe`, used by the decomposition check
   `checkDecompositionQuality` at `:900`, which only `AddWarning`s — ADVISORY,
   not a hard cycle ERROR). There is NO structured field for a parser to read:
   `PlanFrontmatter` (`internal/validate/plan.go:21-30`) has no `depends_on` /
   `work_chunks` field, nothing parses the `work_chunks[*].depends_on` present in
   the user template / `internal/approve/plan_test.go:28,101` fixtures, and BOTH
   plan templates teach the prose `**Depends on** … Bead N` form — so 87/89 real
   plans express deps only in prose. Removing the regex without an atomic
   structured replacement would silently wire ZERO dependencies for every future
   plan (dead-on-arrival).
   *What must be true after (all atomic, one cutover — no window where both prose
   and structured are read, to avoid double-counting):*
   - A structured field is added to `PlanFrontmatter` plus a parser that
     populates it: a `work_chunks []WorkChunk` slice where each chunk carries an
     integer `id` and `depends_on []int` (reusing the shape already present —
     dead — in the user template and approve fixtures).
   - **Index → bead-ID mapping (specify exactly):** work-chunk `id: N` (1-based,
     in declaration order) corresponds to the Nth bead created from the plan,
     i.e. the Nth entry of `bead_ids`. A chunk's `depends_on: [M]` therefore
     wires `bd dep add` so `bead_ids[N-1]` depends on `bead_ids[M-1]`. This
     replaces the prose `Bead N` digit reference and resolves the user template's
     `## Bead <NNN>-A` letter-suffixed headings (which do NOT match the
     `bead\s+(\d+)` digit regex) by keying off the integer work-chunk `id`, not
     the heading text.
   - BOTH consumers switch atomically to the structured field: approve-side
     `bd dep add` wiring (`internal/approve/plan.go:373/:394`) and the
     validate-side decomposition check (`internal/validate/plan.go:819/:900`).
     The duplicated prose `bead\s+(\d+)` regex is removed from both.
   - BOTH plan templates are updated to emit/teach the structured `work_chunks`
     form with integer ids and matching `## Bead <N>` headings:
     `internal/instruct/templates/plan.md` (active, currently prose-only) and
     `project-docs/user/templates/plan.md` (currently has `work_chunks` but
     letter-suffixed headings) — reconciled to one consistent shape.
   *Migration:* existing APPROVED plans are not re-approved (`approve` runs once),
   so they carry no live risk; the template update ensures FUTURE plans declare
   structured deps. *Behavior change:* dependencies expressed only in prose
   "Depends on" text and not in the structured field are no longer wired.

4. **Source key file paths from a structured `key_file_paths` frontmatter field
   (ZFC-5 / `vbij`, lowest priority; harness site DEFERRED).**
   *Motivation:* `internal/contextpack/builder.go:35-37` `ExtractFilePathsFromText`
   hard-codes the prefixes `{"internal/", "cmd/", "pkg/"}` to guess file paths
   from prose for the `## Key File Paths` context surface. (A second heuristic,
   `internal/harness/analyzer.go:694-716` `extractMentionedPaths`, does the same
   with hard-coded extensions; it is OUT of scope here — see deferral below.)
   *What must be true after:* a concrete plan-frontmatter field
   `key_file_paths []string` is added to `PlanFrontmatter` plus its parser,
   emitted by BOTH plan templates, and consumed by
   `internal/contextpack/builder.go` so the `## Key File Paths` enrichment is
   sourced from the declared field rather than `ExtractFilePathsFromText`'s
   prefix scan. When a plan declares no paths the surface is empty — acceptable,
   because this is non-gating context enrichment.
   *Deferral (explicit, named):* the harness site
   (`internal/harness/analyzer.go` `extractMentionedPaths`) is NOT remediated by
   this spec. It is split to a named follow-up bead `mindspec-097-harness-paths`
   (to be filed; stays OPEN after this spec ships). Rationale: it widens the
   blast radius beyond context-system and is non-gating. R4 here is scoped to the
   contextpack site ONLY, with an objectively testable contract (no "handled or
   deferred" language).

5. **Merge `internal/speclist` into `internal/spec` (ARCH-5 / `0yz3`, pure
   refactor, no behavior change).**
   *Motivation:* the two packages remain separate. The duplicate frontmatter
   parser that originally justified the split was already removed by ARCH-6, so
   the split now carries no benefit. Verified: `internal/spec/` =
   {`create.go`, `create_test.go`}; `internal/speclist/` =
   {`speclist.go`, `speclist_test.go`}; one real caller imports it
   (`cmd/mindspec/spec_list.go`).
   *What must be true after:* the two are one package with no behavior change —
   identical public results, all callers updated, all tests green. Merge
   DIRECTION matters for ownership: merge `speclist` INTO `internal/spec` (which
   the core OWNERSHIP manifest already claims). The DELETION of the old
   `internal/speclist/*.go` paths is what triggers `adr-divergence-unowned`
   unless `internal/speclist/**` is claimed first — which Requirement 0
   guarantees. R5 depends on R0.

6. **Tighten `domainNamePattern` to reject trailing/double hyphens
   (idvalidate / `bzb9`, behavior-affecting). Migration-safe — confirmed.**
   *Threat / defect:* `internal/idvalidate/ids.go:32`
   `domainNamePattern = ^[a-z][a-z0-9-]*$` accepts `cli-`, `a--b`, and other
   malformed kebab names that should not be valid domain directory names.
   *What must be true after:* the pattern rejects trailing hyphens and
   consecutive hyphens while still accepting valid kebab names (e.g. `security`,
   `cli-handlers`, `context-system`); a POSIX backslash test pin is added (per
   the precedent already used for ADRID/BeadID glob-metacharacter rejection).
   *Behavior change:* names that validate today (e.g. `cli-`) are newly rejected
   — intended tightening. *Migration: confirmed safe* — the existing domain dirs
   are exactly `{context-system, core, execution, workflow}`, none of which has a
   trailing or doubled hyphen, so the stricter pattern rejects none of them.
   R6 ships in ONE bead with R7 (both edit `internal/idvalidate/ids.go`); R6+R7
   depend on R0.

7. **Unify `idvalidate` error wording and document the BeadID format assumption
   (idvalidate / `yqra`, pure wording/docs).**
   *Motivation:* `internal/idvalidate/ids.go:36-118` — the four validators
   (`SpecID`, `ADRID`, `BeadID`, `DomainName`) use three inconsistent
   error-message conventions; `DomainName` emits the raw-regex form
   (`must match [a-z][a-z0-9-]*`) rather than a human description; and the
   `BeadID` doc comment (`:78-83`) omits the leading-segment / flat-ID format
   assumption baked into `beadIDPattern` (`:29`).
   *What must be true after:* the four validators share one consistent
   error-wording convention (human-readable "must match <format>" phrasing, no
   raw regex leaked to users), and the `BeadID` doc comment documents the
   `<project-slug>-<4+alnum>` flat-ID / leading-segment assumption. No behavior
   change beyond message text; existing accept/reject decisions are unchanged
   (this is purely the wording/docs counterpart to Requirement 6's behavior
   change). R7 is combined with R6 in one bead — R7 re-words the very
   `DomainName` error string that R6's regex change rewrites; splitting them is a
   guaranteed same-line conflict.

## Decomposition Constraints

These constraints bind plan-time bead creation so the lifecycle gates do not
hard-block. The spec is ONE coherent unit of seven findings; do NOT split the
spec. It decomposes into effectively eight beads (the ownership bead plus the
seven findings, with R6+R7 folded into one).

- **(a) B0 ownership bead is FIRST.** B0 (Requirement 0) edits
  `.mindspec/domains/core/OWNERSHIP.yaml` to add `internal/idvalidate/**`
  and `internal/speclist/**`, and merges to the spec branch before any consuming
  bead. Every later bead then branches from a spec tip that already carries both
  claims (robust against branch-point races), and the `impl-approve` whole-branch
  diff sees them.
- **(b) ONLY B0 edits `core/OWNERSHIP.yaml`.** The R5 bead and the R6+R7 bead
  must NOT also edit the manifest, so they cannot conflict on it.
- **(c) Dependency edges:** B0 → {R5, R6+R7}. Do NOT make a single global
  "everyone depends on B0": only R5/R6/R7 touch the newly-claimed packages.
  Gating R1/R2/R3/R4 on B0 would needlessly serialize the spec.
- **(d) R1 (gitutil) is independent** (disjoint files) and may land any time.
- **(e) R2 → R3 → R4 MUST be SERIALIZED** (any order with no concurrency).
  Shared-file hubs: `internal/approve/plan.go` is edited by R2 and R3;
  `internal/validate/plan.go` `PlanFrontmatter` + parser is extended by R3
  (`work_chunks`) and R4 (`key_file_paths`). Running them concurrently risks
  struct/parser merge conflicts. They remain parallel with B0/R1/R5/(R6+R7)
  (disjoint files).
- **(f) R6 + R7 are ONE bead.** Both edit `internal/idvalidate/ids.go`, and R7
  re-words the very `DomainName` error that R6's regex change rewrites; splitting
  them is a same-line conflict.

Parallel lanes: `{R1}`, `{B0 → R5}`, `{B0 → (R6+R7)}`, `{R2 → R3 → R4}`.
Critical path ≈ 3.

## Scope

### In Scope
- `.mindspec/domains/core/OWNERSHIP.yaml` — Requirement 0 (claim
  `internal/idvalidate/**` and `internal/speclist/**` under core; ONLY this bead
  edits the manifest).
- `internal/gitutil/gitops.go` — Requirement 1 (operand guard + `--`
  separators; range-operand exclusion).
- `internal/approve/plan.go` — Requirements 2 (ADRCitations consumption) and 3
  (structured bead dependencies, approve-side `bd dep add` wiring).
- `internal/validate/plan.go` — Requirement 3 (`PlanFrontmatter` `work_chunks`
  field + parser + decomposition-check consumption) and Requirement 4
  (`key_file_paths` field + parser).
- `internal/contextpack/builder.go` — Requirement 4 (primary key-file-paths
  site, consumes `key_file_paths`).
- `internal/instruct/templates/plan.md` — Requirements 3, 4 (active template:
  emit structured `work_chunks` + `key_file_paths`).
- `project-docs/user/templates/plan.md` — Requirements 3, 4 (user template:
  reconcile `work_chunks` headings + add `key_file_paths`).
- `internal/spec/` and `internal/speclist/` — Requirement 5 (package merge) and
  `cmd/mindspec/spec_list.go` (the one real caller, updated).
- `internal/idvalidate/ids.go` — Requirements 6 and 7 (one combined bead).
- Test fixtures/tests for all of the above (notably the dead
  `work_chunks[*].depends_on` fixtures in `internal/approve/plan_test.go`).

### Out of Scope
- The ~16 m557 findings already remediated and closed by specs 091–096.
- The harness path-extraction site (`internal/harness/analyzer.go`
  `extractMentionedPaths`) — deferred to follow-up bead
  `mindspec-097-harness-paths` (R4).
- Changing git operations for callers beyond ref-operand safety (no new git
  features, no executor-routing changes beyond the in-package guard).
- Promoting any new validation warning to a blocking error.
- Re-litigating the plan-frontmatter schema beyond the minimal fields needed
  for Requirements 2–4 (`adr_citations` already exists; add only `work_chunks`
  and `key_file_paths`).

## Non-Goals

- Auditing or hardening git argument paths outside `internal/gitutil`.
- A repo-wide ZFC sweep of every remaining heuristic; only the three cited
  prose-scraping sites are in play (and the harness one is deferred).
- Introducing a new ownership domain; Requirement 0 places the unowned packages
  under the existing `core` domain — not a new-domain task.
- Widening ADR-0036's `Domains` or recording a new ZFC ADR (non-gating; see the
  ZFC-theme blockquote under ADR Touchpoints).

## Acceptance Criteria

- [ ] R0: `.mindspec/domains/core/OWNERSHIP.yaml` claims both
  `internal/idvalidate/**` and `internal/speclist/**`; this bead lands first and
  is the ONLY bead that edits the manifest; `mindspec validate` /
  `adr-divergence` attributes idvalidate and speclist files (including R5's
  deletions) to core with Accepted ADR-0035 coverage; the speclist glob is
  retained through impl-approve.
- [ ] R1: a `-`-prefixed ref/branch/refspec passed to any `internal/gitutil`
  operation returns an error with an ADR-0035-shaped recovery line; checkout,
  merge, and branch single-ref subcommands include a `--` before the ref
  operand; `DiffStat`/`CommitCount` range operands do NOT get a `--`; all
  controlled refs (`main`, `spec/<id>`, `bead/<id>`) still succeed. Covered by
  unit tests including hostile-operand and range-operand cases.
- [ ] R2: the bead `--design` ADR list is built from `PlanFrontmatter.ADRCitations`;
  the `adrIDRe`/`parseADRIDs` prose path no longer drives the design field;
  test proves a prose-only ADR ID is not harvested while a declared one is.
- [ ] R3: a structured `work_chunks` field (integer `id` + `depends_on []int`)
  is parsed by `PlanFrontmatter` and consumed by BOTH `internal/approve`
  (`bd dep add` wiring, mapping chunk `id N` → `bead_ids[N-1]`) and
  `internal/validate` (decomposition check); the duplicated `bead\s+(\d+)` regex
  is removed from both; the former dead `depends_on` fixtures now exercise the
  real path; BOTH plan templates emit the structured `work_chunks` form. A
  freshly-templated plan declares structured deps AND the approve-side wiring is
  exercised end-to-end (RED-on-revert: reverting the parser/wiring breaks the
  test).
- [ ] R4: `## Key File Paths` for the contextpack site is sourced from the
  structured `key_file_paths` frontmatter field (added to `PlanFrontmatter`,
  consumed by `internal/contextpack/builder.go`, emitted by both plan
  templates); `ExtractFilePathsFromText`'s hard-coded prefix scan no longer feeds
  it. The harness site is OUT of scope (follow-up bead
  `mindspec-097-harness-paths`). A test proves a declared `key_file_paths` value
  reaches the surface and that a prose-only path is not scraped.
- [ ] R5: `internal/speclist` is merged into `internal/spec`; the one real
  caller (`cmd/mindspec/spec_list.go`) compiles; the package builds and the full
  test suite passes with no behavior change.
- [ ] R6: `domainNamePattern` rejects trailing and consecutive hyphens, still
  accepts valid kebab names (`security`, `cli-handlers`, `context-system`), and
  has a POSIX backslash test pin.
- [ ] R7: the four idvalidate validators share one error-wording convention (no
  raw regex in user-facing messages), and the `BeadID` doc comment documents the
  flat-ID format assumption.
- [ ] `go build ./...`, `go test ./...`, and `golangci-lint run` (or the
  project's lint gate) pass.
- [ ] `mindspec validate spec 097-code-review-cleanup` passes (adr-coverage:
  every impacted domain mapped to a cited Accepted ADR).

## Validation Proofs

- `go test ./internal/gitutil/...`: hostile-operand and range-operand tests pass;
  controlled refs still succeed.
- `go test ./internal/approve/... ./internal/validate/...`: structured
  ADRCitations, structured `work_chunks` deps, and `key_file_paths` paths
  exercised; prose-regex paths gone.
- `go test ./internal/contextpack/...`: key file paths sourced from
  `key_file_paths` frontmatter.
- `go test ./internal/spec/...`: merged package green (no `internal/speclist`).
- `go test ./internal/idvalidate/...`: trailing/double-hyphen rejection +
  unified error wording + backslash pin assertions pass.
- `go build ./... && go vet ./...`: whole tree compiles after the package merge.
- Ownership: with R0's claims in place, `adr-divergence` raises no
  `adr-divergence-unowned` for idvalidate/speclist at any consuming bead's
  `complete` or at `impl-approve`.
- `mindspec validate spec 097-code-review-cleanup`: spec gate (incl.
  adr-coverage) passes.

## Open Questions

None — all questions raised in spec review are resolved and baked into the spec
body (Requirement 0 for ownership, Requirements 3 and 4 for the new
frontmatter field names/shapes, the ZFC-theme blockquote for the ADR-coverage
question, and Requirement 6 for the migration-safety check). The decisions:

- [x] **Ownership of `internal/idvalidate` and `internal/speclist`** — RESOLVED:
  Requirement 0 (a dedicated FIRST bead) claims both under core; the binding gate
  is `adr-divergence-unowned` (ERROR), not the `unclaimed-source` Warn.
- [x] **ZFC-theme ADR coverage for context-system/execution (R4)** — RESOLVED:
  non-gating (domain coverage, not theme); all impacted domains already map to
  Accepted ADRs. Not widening ADR-0036.
- [x] **R4 scope / split** — RESOLVED: R4 scoped to the contextpack site with the
  concrete `key_file_paths` field; the harness site is deferred to follow-up bead
  `mindspec-097-harness-paths`.
- [x] **New plan-frontmatter fields (R3, R4)** — RESOLVED: `work_chunks` (integer
  `id` + `depends_on []int`, chunk `id N` → `bead_ids[N-1]`) and
  `key_file_paths []string`; both plan templates updated to emit them.
- [x] **R6 migration risk** — RESOLVED: confirmed safe; existing domain dirs
  `{context-system, core, execution, workflow}` have no trailing/double hyphen.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-13
- **Notes**: Approved via mindspec approve spec