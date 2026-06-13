---
adr_citations:
    - id: ADR-0030
      sections:
        - executor / gitutil as the Git-process I/O boundary (Bead 2 operand guard at the package edge)
    - id: ADR-0035
      sections:
        - recovery-line agent error contract (Bead 2 hostile-operand rejection; Beads 1/6/7 core-domain coverage)
    - id: ADR-0036
      sections:
        - Zero Framework Cognition — declared structured frontmatter over prose scraping (Beads 3, 4, 5)
    - id: ADR-0033
      sections:
        - deterministic context-pack construction (Bead 5 key_file_paths source)
approved_at: "2026-06-13T14:57:50Z"
approved_by: user
bead_ids:
    - mindspec-r04i.1
    - mindspec-r04i.2
    - mindspec-r04i.3
    - mindspec-r04i.4
    - mindspec-r04i.5
    - mindspec-r04i.6
    - mindspec-r04i.7
spec_id: 097-code-review-cleanup
status: Approved
version: "1"
---
# Plan: 097-code-review-cleanup

> Seven beads remediating the seven genuinely-open residual m557 findings. Bead 1 is the ownership
> prerequisite (claims `internal/idvalidate/**` + `internal/speclist/**` under core) and lands FIRST
> so the package-merge and idvalidate beads do not hard-block at `adr-divergence-unowned`. Bead 2
> (gitutil) is an independent security fix. Beads 3–5 retire three prose-scraping heuristics in
> favour of declared structured plan frontmatter (Zero Framework Cognition, ADR-0036) and are
> SERIALIZED (3→4→5) because they share `internal/approve/plan.go` and the `PlanFrontmatter`
> struct/parser. Bead 6 merges the stale `internal/speclist` split into `internal/spec`. Bead 7
> tightens `domainNamePattern` and unifies idvalidate error wording (R6+R7 folded into one bead —
> same-file, same-line). Impacted domains execution / workflow / context-system / core all map to a
> cited Accepted ADR (ADR-0030, ADR-0033, ADR-0035, ADR-0036); NO new ADR is required.

## ADR Fitness

- **ADR-0030** (Executor as the Git/Process I/O boundary; Status: **Accepted**; Domain(s):
  execution, validation, lifecycle, lint): establishes `internal/gitutil` / the executor as the
  Git-process I/O boundary for enforcement packages. Bead 2's ref/branch operand guard belongs at
  exactly this boundary — the package validates its own argv before shelling out. Domain(s) include
  **execution** (an impacted domain) → no `adr-cite-irrelevant`. **No new ADR** (applies the
  existing boundary decision).
- **ADR-0035** (agent error contract — recovery lines; Status: **Accepted**; Domain(s): workflow,
  execution, core): Bead 2's rejection error for a `-`-prefixed operand carries an ADR-0035-shaped
  recovery line. The same Accepted ADR provides **core**-domain coverage for the
  `internal/idvalidate/**` and `internal/speclist/**` claims added by Bead 1 and consumed by Beads
  6/7. Domain(s) include **workflow/execution/core** (all impacted) → no `adr-cite-irrelevant`.
- **ADR-0036** (Ownership Discovery — Zero Framework Cognition; Status: **Accepted**; Domain(s):
  workflow, validation, doc-sync, ownership): records the ZFC stance — heuristic classification is
  forbidden; semantic data is declared, not guessed. Beads 3, 4, and 5 are direct applications
  (retire the ADR-ID, bead-dependency, and key-file-path prose-scraping regexes in favour of
  declared structured frontmatter). Domain(s) include **workflow** (an impacted domain) → no
  `adr-cite-irrelevant`. **No new ADR** (applies the existing ZFC principle).
- **ADR-0033** (deterministic context-pack construction; Status: **Accepted**; Domain(s):
  context-system): governs `internal/contextpack`. Bead 5 genuinely edits this package — it DELETES
  the prose prefix-scan `ExtractFilePathsFromText` (`internal/contextpack/builder.go:36`), retiring
  the heuristic feed for the `## Key File Paths` surface (rendered from per-bead `metadata.file_paths`
  in `internal/contextpack/beadctx.go:83-94`). The declared replacement is now sourced upstream at
  approve time, but `internal/contextpack` remains a genuinely touched package and domain. Domain(s) =
  **context-system** (an impacted domain) → no `adr-cite-irrelevant`.

No new ADR is required by any bead. All four cited ADRs are Accepted, and every impacted domain
(execution, workflow, context-system, core) is covered by at least one cited Accepted ADR
(execution → ADR-0030/0035; workflow → ADR-0035/0036; context-system → ADR-0033; core → ADR-0035).
Per the spec's settled ZFC-theme blockquote, `adr-divergence` checks DOMAIN coverage (not THEME
coverage), so no ZFC-specific ADR and no widening of ADR-0036's Domains is needed.

## Testing Strategy

Every behavior-affecting change is proven **RED-on-revert** — the test FAILS if the fix is reverted
to the cited pre-fix code; the pure refactor (Bead 6) and pure wording change (Bead 7's wording
half) are proven by GREEN behavior parity (identical accept/reject decisions, all callers compile):

- Bead 1 (ownership): the OWNERSHIP.yaml edit is a process artifact; its proof is downstream —
  with the claims present, `adr-divergence` attributes idvalidate/speclist files (including Bead
  6's deletions) to core with Accepted ADR-0035 coverage at every consuming bead's `complete` and
  at `impl-approve`.
- Bead 2 (gitutil): hostile-operand unit tests (a `-x` / `--upload-pack=…` ref errors with an
  ADR-0035 recovery line, RED if the guard is removed); a `--` separator precedes the single ref on
  checkout/merge/branch; range operands (`base..head`) do NOT get a `--` (RED if a `--` is wrongly
  inserted — it would be reinterpreted as a pathspec); controlled refs (`main`, `spec/<id>`,
  `bead/<id>`) still succeed.
- Bead 3 (ADRCitations): a plan whose spec cites an ADR ID ONLY in prose but NOT in
  `adr_citations` no longer harvests that ID into `--design`; a declared `adr_citations` ID IS
  harvested. RED if reverted to the `adrIDRe`/`parseADRIDs` prose scrape.
- Bead 4 (work_chunks): a freshly-templated plan declares structured `work_chunks` deps and the
  approve-side `bd dep add` wiring maps chunk `id N` → `bead_ids[N-1]` end-to-end; a prose-only
  "Depends on Bead N" line wires ZERO deps. RED if the parser/wiring is reverted to `bead\s+(\d+)`.
- Bead 5 (key_file_paths): END-TO-END through the rendered surface — a work-chunk that declares
  `key_file_paths` flows approve-side into that bead's `metadata.file_paths` and reaches the
  `## Key File Paths` surface (asserted at `internal/contextpack/beadctx_test.go:64`); a prose-only
  `internal/...` path in the chunk body is NOT scraped. RED if reverted to
  `ExtractFilePathsFromText`'s prefix scan at the approve producer.
- Bead 6 (merge): `go build ./...` + the full suite green with `internal/speclist` deleted and the
  one caller (`cmd/mindspec/spec_list.go`) updated — identical public results (no behavior change).
- Bead 7 (idvalidate): `domainNamePattern` rejects trailing/double hyphens, still accepts valid
  kebab names, a POSIX backslash pin asserts metacharacter rejection, and the four validators share
  one human-readable wording convention with no raw regex leaked.

Each bead runs **golangci-lint locally (CI Lint-job parity)** — American spelling only (`behavior`,
not `behaviour`) and no new gosec findings — and gates on `go build ./...` + `go test ./...` green.

## Bead 1: Claim `internal/idvalidate/**` and `internal/speclist/**` under core (R0)

The ownership prerequisite. Edit `.mindspec/docs/domains/core/OWNERSHIP.yaml` to add BOTH
`internal/idvalidate/**` and `internal/speclist/**` to the core `paths:` list, so Beads 6 and 7
(which touch / delete those packages) do not hard-block at the `adr-divergence-unowned` ERROR
(`internal/validate/divergence.go:196`) at their `mindspec complete` AND at `impl-approve` (whose
whole-branch diff always re-contains Bead 6's deletions of the OLD `internal/speclist/*.go` paths).
Doc-only — NO code change. This bead lands FIRST and is the ONLY bead that edits the manifest. (S.)

**Steps**
1. Edit `.mindspec/docs/domains/core/OWNERSHIP.yaml`: append `internal/idvalidate/**` and
   `internal/speclist/**` to the core domain's `paths:` glob list, keeping the existing
   `internal/spec/**` entry. Preserve the file's existing key ordering and comment style.
2. Confirm BOTH globs map to the core domain, which is covered by Accepted ADR-0035 — so once
   claimed, idvalidate edits (Bead 7), speclist deletions (Bead 6), and the impl-approve
   whole-branch re-diff all attribute to core and pass `coverageOf`. Note the `internal/speclist/**`
   glob is intentionally KEPT through impl-approve (a dangling glob over the emptied dir is
   harmless; the whole-branch diff re-contains the deletions and must keep attributing to core).
3. Verify the OWNERSHIP.yaml edit is itself a process artifact skipped by divergence
   (`isProcessArtifact`) and doc-sync, so this bead passes its own gates with no `--override-adr`.

**Verification**
- [ ] `.mindspec/docs/domains/core/OWNERSHIP.yaml` lists both `internal/idvalidate/**` and
      `internal/speclist/**` under core; the existing `internal/spec/**` claim is retained
- [ ] `mindspec validate` is clean for this doc-only bead (the manifest edit is a process artifact)
- [ ] This is the ONLY bead that edits `core/OWNERSHIP.yaml`; it lands before Beads 6 and 7

**Acceptance Criteria**
- [ ] `.mindspec/docs/domains/core/OWNERSHIP.yaml` claims both `internal/idvalidate/**` and
      `internal/speclist/**`; this bead lands first and is the ONLY bead that edits the manifest.
- [ ] `adr-divergence` attributes idvalidate and speclist files (including Bead 6's deletions) to
      core with Accepted ADR-0035 coverage at every consuming bead's `complete` and at
      `impl-approve`; the speclist glob is retained through impl-approve.

**Depends on**
None

## Bead 2: Git argument-safety guard at the gitutil boundary (R1)

Add a package-boundary guard in `internal/gitutil/gitops.go` that REJECTS any ref/branch/refspec
operand beginning with `-` (returning an error with an ADR-0035-shaped recovery line). The spec AC
says "any `internal/gitutil` operation", so the guard must cover EVERY ref-bearing entry point —
not just the checkout/merge/branch trio. A re-read of `gitops.go` end-to-end yields the full set,
classified by how each operand must be protected:

- **Single-ref subcommands (reject-guard + a `--` separator** to defend against a future non-`-` but
  ref/pathspec-ambiguous value): `CreateBranch` (`:40`, `branch name from`), `MergeBranch`
  checkout (`:52`) and merge (`:58`), `MergeInto` (`:71`), `DeleteBranch` (`:116`, `branch -D name`),
  `CheckoutNewBranch` (`:555`, `checkout -b branch`), `LogOneline` (`:333`, `log -1 --oneline ref` —
  trailing `--`), and `DiffNameOnlyRef` (`:436`, `diff --name-only ref` — trailing `--`).
- **Ref operands where `--` is N/A** (the subcommand takes no pathspec, so a separator is wrong/
  meaningless) — **reject-guard ALONE**: `BranchExists` (`:34`, `rev-parse --verify refs/heads/`+name;
  the `refs/heads/` prefix already blocks a leading `-` reaching git, but guard `name` for
  consistency), `PushBranch` (`:162`, `push -u origin branch`), `IsAncestor` (`:227`,
  `merge-base --is-ancestor ancestor descendant`), `RevParseRef` (`:305`,
  `rev-parse --verify --quiet ref^{commit}`), `WorktreeAddDetach` (`:513`, `worktree add --detach
  wtPath commit`), and `WorktreeAdd` (`:523`, `worktree add wtPath branch`).
- **Revision-range operands (`base..head`)** — **reject-guard ALONE, MUST NOT get a `--`** (a `--`
  would reinterpret the range as a pathspec): `DiffStat` (`:202`, `diff --stat base..head`),
  `CommitCount` (`:212`, `rev-list --count base..head`), and `DiffNameOnly` (`:425`,
  `diff --name-only base..head`).
- **Pathspec site (already uses `--`)**: `DiffPathspec` (`:448`, `diff base head -- pathspecs…`) —
  apply the reject-guard to the `base`/`head` ref operands; the pathspecs already sit safely after
  the existing `--`.

The shared builder `gitArgs` (`:274`) is used by the `gitArgs`-routed sites but CANNOT itself
distinguish a ref from an option-flag or a pathspec, so the reject-guard is applied per-operand at
each ref-bearing call site (not blanket-applied inside `gitArgs`). Defense-in-depth at the I/O
boundary (ADR-0030). Independent of all other beads (disjoint files). (S.)

**Steps**
1. Add a boundary validator (e.g. `rejectOptionLike(operand)`) that returns an ADR-0035-shaped
   error ("operand %q looks like a git option … run: <recovery>") for any ref/branch/refspec value
   beginning with `-`. Apply it to EVERY ref-bearing operand across all the entry points enumerated
   above (single-ref, ref-only, range, and the `base`/`head` of `DiffPathspec`) — not just the
   checkout/merge/branch trio.
2. Insert a `--` separator on the single-ref subcommands only (`CreateBranch`, `MergeBranch`
   checkout/merge, `MergeInto`, `DeleteBranch`, `CheckoutNewBranch`, and a TRAILING `--` on the
   single-ref `LogOneline` / `DiffNameOnlyRef`) so a future non-`-` but ambiguous ref cannot be
   reparsed as an option or a pathspec.
3. Leave the revision-range operands in `DiffStat` (`:202`), `CommitCount` (`:212`) and
   `DiffNameOnly` (`:425`) WITHOUT a `--` (the leading-`-` guard alone protects them; a `--` would
   turn `base..head` into a pathspec), and leave the ref-only sites (`BranchExists`, `PushBranch`,
   `IsAncestor`, `RevParseRef`, `WorktreeAddDetach`, `WorktreeAdd`) reject-guard-only.
4. Add unit tests in `internal/gitutil/` (precedent: the `execCommand` seam + `assertArgs`, and
   `TestDiffPathspec_InsertsSeparator` at `gitops_test.go:508`): a hostile `-x` / `--upload-pack=…`
   operand errors with a recovery line at EACH class of site (a single-ref site, a ref-only site,
   AND a range site — not only checkout/merge/branch); single-ref subcommands include the `--`;
   range operands do not; controlled refs (`main`, `spec/<id>`, `bead/<id>`) still succeed.

**Verification**
- [ ] `go build ./... && go test ./internal/gitutil/...` green
- [ ] A `-`-prefixed ref/branch/refspec errors with an ADR-0035 recovery line at EVERY ref-bearing
      entry point — single-ref (`CreateBranch`/`MergeBranch`/`MergeInto`/`DeleteBranch`/
      `CheckoutNewBranch`/`LogOneline`/`DiffNameOnlyRef`), ref-only (`BranchExists`/`PushBranch`/
      `IsAncestor`/`RevParseRef`/`WorktreeAddDetach`/`WorktreeAdd`), range (`DiffStat`/`CommitCount`/
      `DiffNameOnly`), and `DiffPathspec`'s base/head (RED if any guard removed)
- [ ] single-ref subcommands include a `--`; the three `base..head` range operands do NOT get a `--`
      (RED if wrongly inserted); `DiffPathspec`'s existing pathspec `--` is preserved
- [ ] Controlled refs (`main`, `spec/<id>`, `bead/<id>`) still succeed; golangci-lint clean

**Acceptance Criteria**
- [ ] A `-`-prefixed ref/branch/refspec passed to ANY `internal/gitutil` operation (every
      ref-bearing entry point enumerated above, not just checkout/merge/branch) returns an error
      with an ADR-0035-shaped recovery line; the single-ref subcommands include a `--` before the
      ref operand.
- [ ] The `base..head` range operands (`DiffStat`/`CommitCount`/`DiffNameOnly`) do NOT get a `--`;
      all controlled refs (`main`, `spec/<id>`, `bead/<id>`) still succeed. Covered by unit tests
      including hostile-operand cases at each site class and range-operand cases.

**Depends on**
None

## Bead 3: Consume structured `ADRCitations` for the bead `--design` field (R2)

Build the bead `--design` ADR list in `internal/approve/plan.go` from the structured
`PlanFrontmatter.ADRCitations` (`internal/validate/plan.go:29`, the validated source of truth)
instead of regex-scraping the SPEC's `## ADR Touchpoints` PROSE. Retire the prose-scraping path:
`adrIDRe = regexp.MustCompile("ADR-(\\d{4})")` (`:596`) and `parseADRIDs` (`:599`), called from
`buildDesignField` (`:572`). Forward-only and non-gating (`approve` runs once per plan). (S.)

**Steps**
1. In `internal/approve/plan.go` `buildDesignField` (`:563`), replace the
   `contextpack.ExtractSection(specContent, "ADR Touchpoints")` + `parseADRIDs` flow with the ADR
   IDs already parsed into `PlanFrontmatter.ADRCitations` (each `ADRCitation.ID`). This requires a
   SIGNATURE CHANGE: the current `buildDesignField(specDir, specContent, requirements)` reads spec
   prose, so it must instead take the parsed citations (or the plan content) as a parameter, and the
   caller `createImplementationBeads` (`:305`) must parse the plan frontmatter to supply them. The
   function stays directly unit-testable (precedent: `internal/approve/plan_spec092_test.go:118`
   calls `buildDesignField` directly).
2. Remove the now-dead `adrIDRe` (`:596`) and `parseADRIDs` (`:599`) so the prose regex no longer
   drives the design field. (Intended drop: ADR IDs that appear ONLY in spec prose but not in
   declared `adr_citations` are no longer harvested — the frontmatter is the contract the
   plan-validation gate already enforces.)
3. Remove or migrate the EXISTING tests that call `parseADRIDs` directly
   (`internal/approve/plan_test.go:726`, `:735`, `:741`) — deleting `parseADRIDs` while those stay
   leaves `internal/approve` non-compiling. Re-point their intent (dedup, "None applicable") onto
   the new `ADRCitations`-driven path where still meaningful.
4. Add a test proving a prose-only ADR ID (present in `## ADR Touchpoints` but absent from
   `adr_citations`) is NOT harvested into `--design`, while a declared `adr_citations` ID IS.

**Verification**
- [ ] `go build ./... && go test ./internal/approve/...` green
- [ ] The `--design` ADR list is built from `PlanFrontmatter.ADRCitations`; `adrIDRe`/`parseADRIDs`
      no longer drive the design field
- [ ] A prose-only ADR ID is not harvested while a declared one is (RED on revert to the prose scrape)
- [ ] golangci-lint clean (American spelling; no new gosec)

**Acceptance Criteria**
- [ ] The bead `--design` ADR list is built from `PlanFrontmatter.ADRCitations`; the
      `adrIDRe`/`parseADRIDs` prose path no longer drives the design field.
- [ ] A test proves a prose-only ADR ID is not harvested while a declared one is.

**Depends on**
None

## Bead 4: Declare and consume bead dependencies in structured plan frontmatter (R3)

Add a structured `work_chunks []WorkChunk` slice (each chunk: integer `id`, `depends_on []int`) to
`PlanFrontmatter` + its parser (`internal/validate/plan.go`), and switch BOTH consumers ATOMICALLY
to it: the approve-side `bd dep add` wiring (`internal/approve/plan.go:373/:394`) and the
validate-side decomposition check (`internal/validate/plan.go:819/:900`). Remove BOTH duplicated
prose `bead\s+(\d+)` regexes (`depRe` and `beadDepRe`). Index→bead-ID mapping: chunk `id: N`
(1-based, declaration order) → `bead_ids[N-1]`; `depends_on: [M]` wires `bead_ids[N-1]` to depend on
`bead_ids[M-1]`. Update BOTH plan templates. Depends on Bead 3 (shares `internal/approve/plan.go`;
serialized per spec constraint (e)). Bead 5 EXTENDS the `WorkChunk` type this bead introduces (adds
`key_file_paths`), so keep the type defined here. (L — new type + `PlanFrontmatter` field + parser +
exported frontmatter parse in approve + two consumer rewrites + alignment guard + two template
rewrites + two regex removals + tests. Do NOT split: spec constraint (e)'s atomic cutover forbids
any window where one consumer reads structured and the other reads prose.)

**Steps**
1. Add a `WorkChunk` type (`id int`, `depends_on []int`) and a `work_chunks []WorkChunk` field to
   `PlanFrontmatter` (`internal/validate/plan.go:21-30`) plus parser support, following the nested
   `ADRCitation` slice precedent (`plan.go:29-53`). NOTE the existing dead stub is RICHER than
   `{id, depends_on}`: the user template (`.mindspec/docs/user/templates/plan.md:15-27`) and the
   approve fixtures (`internal/approve/plan_test.go:23-28`, `:96-101`) carry `title`, `scope`, and
   `verify` keys too. Because `parsePlanFrontmatter` uses NON-strict `yaml.Unmarshal`
   (`internal/validate/plan.go`), the `WorkChunk` struct parsing only `id`+`depends_on` HARMLESSLY
   ignores `title`/`scope`/`verify`, so the templates keep those human-readable keys while the
   parser reads only the two it needs. (Bead 5 later adds `key_file_paths` to this same struct.)
2. Add a frontmatter-parse step to approve: `buildBeadsFromPlan`/`createImplementationBeads`
   currently parses ONLY `validate.ParseBeadSections` (`internal/approve/plan.go:287`) and never
   reads plan frontmatter in the dep-wiring path. To read `work_chunks`, either EXPORT the currently
   unexported `parsePlanFrontmatter` from `internal/validate` (e.g. `ParsePlanFrontmatter`) and call
   it, or reuse the generic-map `yaml.Unmarshal` approach already used in `writeBeadIDsToFrontmatter`
   (`internal/approve/plan.go:707-708`). This is real added work, not a one-line consumer swap.
3. Add an ALIGNMENT GUARD before any positional `bead_ids[N-1]` wiring: validate that the
   `work_chunks` ids are contiguous `1..K` and that `K` equals the `## Bead N` section count
   (`len(sections)`). Misaligned ids (gaps, dups, or count mismatch) must ERROR, not mis-wire or
   panic. Bounds-check every `depends_on` target mirroring the existing `depIdx >= 0 && depIdx <
   len(sections)` guard at `internal/validate/plan.go:905`. The positional mapping is otherwise sound
   because `bead_ids` is appended in `## Bead` section declaration order
   (`internal/approve/plan.go:362`), so `bead_ids[N-1]` is deterministically the Nth section.
4. Switch the approve-side consumer (`internal/approve/plan.go:373/:394`) to wire `bd dep add` from
   `work_chunks[*].depends_on`, mapping chunk `id N` → `bead_ids[N-1]`; remove `depRe`.
5. Switch the validate-side decomposition check (`internal/validate/plan.go:819/:900`,
   `checkDecompositionQuality`) to read the structured `work_chunks` field; remove `beadDepRe`. (No
   window where both prose and structured are read — atomic cutover, no double-counting.)
6. Update BOTH plan templates to emit/teach the structured `work_chunks` form with integer ids and
   matching `## Bead <N>` headings: `internal/instruct/templates/plan.md` (active, currently
   prose-only) and `.mindspec/docs/user/templates/plan.md` (reconcile its letter-suffixed headings
   to integer-keyed chunks). Document the chunk-id → `bead_ids[N-1]` mapping in both.
7. Add tests: a freshly-templated plan declares structured deps AND the approve-side wiring maps
   `id N` → `bead_ids[N-1]` end-to-end (precedent harness: `TestCreateImplementationBeads_CreatesAndWiresDeps`
   with the `planRunBDFn` mock, `plan_test.go:119`); a prose-only "Depends on Bead N" line wires ZERO
   deps; a misaligned `work_chunks` id set ERRORs via the alignment guard; the former dead
   `depends_on` fixtures now exercise the real path.

**Verification**
- [ ] `go build ./... && go test ./internal/approve/... ./internal/validate/...` green
- [ ] `work_chunks` (integer `id` + `depends_on []int`) is parsed by `PlanFrontmatter` and consumed
      by BOTH approve (`bd dep add`) and validate (decomposition check); both `bead\s+(\d+)` regexes
      removed
- [ ] BOTH plan templates emit the structured `work_chunks` form; the dead fixtures now exercise it
- [ ] The alignment guard rejects non-contiguous / count-mismatched `work_chunks` ids with an ERROR
      (no mis-wire, no panic); every `depends_on` target is bounds-checked
- [ ] RED-on-revert: reverting the parser/wiring breaks the dependency-wiring test; golangci-lint clean

**Acceptance Criteria**
- [ ] A structured `work_chunks` field (integer `id` + `depends_on []int`) is parsed by
      `PlanFrontmatter` and consumed by BOTH `internal/approve` (`bd dep add` wiring, mapping chunk
      `id N` → `bead_ids[N-1]`) and `internal/validate` (decomposition check); the duplicated
      `bead\s+(\d+)` regex is removed from both; the former dead `depends_on` fixtures now exercise
      the real path; BOTH plan templates emit the structured form.
- [ ] A freshly-templated plan declares structured deps AND the approve-side wiring is exercised
      end-to-end (RED-on-revert: reverting the parser/wiring breaks the test).

**Depends on**
Bead 3 (shares the `internal/approve/plan.go` design/dependency-wiring seam; the R2→R3→R4 lane is
serialized per the spec's Decomposition Constraint (e) to avoid struct/parser merge conflicts)

## Bead 5: Source key file paths from a structured `key_file_paths` frontmatter field (R4)

Bead 4 introduced the per-bead `WorkChunk` shape; this bead EXTENDS it with a `key_file_paths
[]string` field (co-located with `work_chunks`). This resolves a granularity mismatch: the
`## Key File Paths` surface is rendered PER BEAD from `metadata.file_paths`, so the declared source
must also be per-bead — a single plan-level list would give every bead identical paths. **The
surface is NOT built in `internal/contextpack/builder.go`** (the originally-named site is wrong to
the code): it is rendered from each bead's `metadata.file_paths` in
`internal/contextpack/beadctx.go:83-94` (and `budgeter.go:229-240`), which is populated at approve
time in `createImplementationBeads` via `contextpack.ExtractFilePathsFromText(workChunk)` at
`internal/approve/plan.go:323-330`; `internal/contextpack` never receives `PlanFrontmatter`. So the
real producer switch lands in `internal/approve/plan.go` (workflow): derive each bead's
`metadata.file_paths` from `work_chunks[N-1].key_file_paths` instead of the prose scan.
`ExtractFilePathsFromText` then has NO remaining non-test caller (verified by grep: only
`internal/approve/plan.go:327`) and is DELETED from `internal/contextpack/builder.go`
(context-system) — retiring the heuristic at its definition keeps `internal/contextpack` a genuinely
touched package/domain, so the spec's R4→context-system/ADR-0033 mapping stays honest with NO spec
edit. The harness site (`internal/harness/analyzer.go` `extractMentionedPaths`) is DEFERRED to
follow-up bead `mindspec-097-harness-paths` and is NOT touched here. Depends on Bead 4 (extends Bead
4's `WorkChunk` type and reuses its id↔section alignment guard; serialized per constraint (e)). (S.)

**Steps**
1. Add `key_file_paths []string` to the per-bead `WorkChunk` type introduced by Bead 4
   (`internal/validate/plan.go`) — NOT a plan-level field. Parser support comes for free with Bead
   4's nested-slice unmarshal.
2. Switch the approve-side PRODUCER at `internal/approve/plan.go:323-330`: derive each bead's
   `metadata.file_paths` (fed to `buildBeadMetadata` at `:330`, later rendered by beadctx) from the
   aligned `work_chunks[N-1].key_file_paths` instead of `ExtractFilePathsFromText(workChunk)`, using
   Bead 4's id↔section alignment so chunk `id N` feeds the Nth bead. When a chunk declares no paths
   the surface is empty (acceptable — non-gating context enrichment).
3. DELETE the now-dead `ExtractFilePathsFromText` from `internal/contextpack/builder.go:35-37` (its
   only non-test caller was `internal/approve/plan.go:327`, removed in step 2) AND remove its
   orphaned tests `TestExtractFilePathsFromText*` (`internal/contextpack/builder_test.go:57-98`),
   which would otherwise leave `internal/contextpack` non-compiling.
4. Emit `key_file_paths` PER WORK-CHUNK in BOTH plan templates, nested under each `work_chunks[*]`
   entry (`internal/instruct/templates/plan.md` and `.mindspec/docs/user/templates/plan.md`).
5. Add tests on the END-TO-END rendered surface: a work-chunk that declares `key_file_paths`
   produces a bead whose `metadata.file_paths` reaches the `## Key File Paths` surface — hang the
   assertion where the surface is already asserted, `internal/contextpack/beadctx_test.go:64`; a
   prose-only `internal/...` path in the chunk body is NOT scraped (RED on revert to the prefix scan
   at the approve producer).
6. (Handoff) FILE the deferred follow-up bead `mindspec-097-harness-paths` (named in the spec/plan
   but not yet a real bead) for the untouched `internal/harness/analyzer.go` `extractMentionedPaths`
   site — flag for impl / session-close so it is not silently dropped.

**Verification**
- [ ] `go build ./... && go test ./internal/approve/... ./internal/contextpack/... ./internal/validate/...` green
- [ ] `key_file_paths` is a per-bead `WorkChunk` field; each bead's `metadata.file_paths` is sourced
      from `work_chunks[N-1].key_file_paths` at `internal/approve/plan.go` (not the prefix scan)
- [ ] `ExtractFilePathsFromText` is DELETED from `internal/contextpack/builder.go` with its tests
      removed; no non-test caller remains; the harness site is untouched (deferred to
      `mindspec-097-harness-paths`)
- [ ] A declared `key_file_paths` reaches the rendered `## Key File Paths` surface; a prose-only path
      does not; golangci-lint clean

**Acceptance Criteria**
- [ ] `## Key File Paths` for the contextpack site is sourced from a per-bead
      `work_chunks[*].key_file_paths` frontmatter field (added to the `WorkChunk` shape, consumed at
      `internal/approve/plan.go` into each bead's `metadata.file_paths`, emitted by both plan
      templates); `ExtractFilePathsFromText`'s hard-coded prefix scan is DELETED from
      `internal/contextpack/builder.go` and no longer feeds it. The harness site is OUT of scope
      (follow-up bead `mindspec-097-harness-paths`, which must be FILED).
- [ ] A test proves a declared `key_file_paths` value reaches the rendered surface and that a
      prose-only path is not scraped.

**Depends on**
Bead 4 (this bead EXTENDS Bead 4's per-bead `WorkChunk` type with `key_file_paths` and reuses its
id↔section alignment guard; the R2→R3→R4 lane is serialized per Decomposition Constraint (e) to
avoid struct/parser conflicts)

## Bead 6: Merge `internal/speclist` into `internal/spec` (R5)

Pure refactor, no behavior change: merge `internal/speclist` ({`speclist.go`, `speclist_test.go`})
INTO the core-owned `internal/spec` package, update the one real caller
(`cmd/mindspec/spec_list.go`), and delete `internal/speclist/*.go`. Merge DIRECTION matters for
ownership — `internal/spec` is already core-owned, and Bead 1's `internal/speclist/**` claim covers
the deletion-attribution of the OLD paths. Identical public results, all callers updated, all tests
green. Depends on Bead 1 (the speclist claim must be on the spec tip before the deletions). (S.)

**Steps**
1. Move the contents of `internal/speclist/speclist.go` (and its tests) into `internal/spec`,
   reconciling package name and any exported identifiers; preserve identical public behavior.
   Exported identifiers do NOT collide (`internal/spec` exports `Run`/`Result`; `internal/speclist`
   exports `List`/`SpecEntry`), but the merged TEST set has a helper-name collision: both
   `internal/spec/create_test.go:37` and `internal/speclist/speclist_test.go:118` define
   `setupTestRoot` — rename one (e.g. the speclist variant) when merging the test files.
2. Update the one real caller `cmd/mindspec/spec_list.go` to import/call the merged `internal/spec`
   API instead of `internal/speclist`.
3. Delete `internal/speclist/speclist.go` and `internal/speclist/speclist_test.go` (the emptied
   dir's `internal/speclist/**` claim from Bead 1 attributes the deletions to core).
4. Run `go build ./...` and the full suite to confirm no behavior change and no remaining
   `internal/speclist` import anywhere in the tree.

**Verification**
- [ ] `go build ./... && go vet ./... && go test ./internal/spec/... ./cmd/mindspec/...` green
- [ ] `internal/speclist` is deleted; no import of it remains; `cmd/mindspec/spec_list.go` compiles
      against the merged `internal/spec`
- [ ] `mindspec complete` for this bead raises NO `adr-divergence-unowned` (Bead 1's speclist claim
      attributes the deletions to core); golangci-lint clean

**Acceptance Criteria**
- [ ] `internal/speclist` is merged into `internal/spec`; the one real caller
      (`cmd/mindspec/spec_list.go`) compiles; the package builds and the full test suite passes with
      no behavior change.

**Depends on**
Bead 1 (Bead 1's `internal/speclist/**` ownership claim must be on the spec tip before this bead
deletes the OLD speclist paths, so the deletions attribute to core and pass `adr-divergence`)

## Bead 7: Tighten `domainNamePattern` + unify idvalidate error wording (R6 + R7)

Two findings in ONE bead — both edit `internal/idvalidate/ids.go` and R7 re-words the very
`DomainName` error string that R6's regex change rewrites (splitting them is a guaranteed same-line
conflict). R6: tighten `domainNamePattern` (`:32`, `^[a-z][a-z0-9-]*$`) to REJECT trailing and
consecutive hyphens (`cli-`, `a--b`) while still accepting valid kebab names, plus a POSIX backslash
test pin. R7: unify the four validators' (`SpecID`, `ADRID`, `BeadID`, `DomainName`) error wording
to one human-readable convention (no raw regex leaked) and document the `BeadID` flat-ID format
assumption. Depends on Bead 1 (idvalidate is newly core-claimed there). (S.)

**Steps**
1. (R6) Tighten `domainNamePattern` (`internal/idvalidate/ids.go:32`) so it rejects a trailing
   hyphen and consecutive hyphens while still accepting `security`, `cli-handlers`,
   `context-system` (e.g. `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`). Migration is confirmed safe: existing
   domain dirs `{context-system, core, execution, workflow}` have no trailing/double hyphen.
2. (R6) Add a POSIX backslash test pin asserting a backslash/glob-metacharacter domain name is
   rejected (per the precedent already used for ADRID/BeadID glob-metacharacter rejection).
3. (R7) Unify the four validators' error wording (`ids.go:36-118`) to one human-readable
   "must match <format>" convention so `DomainName` stops leaking the raw regex; keep every
   existing accept/reject decision unchanged (wording-only beyond R6's tightening).
4. (R7) Extend the `BeadID` doc comment (`:78-83`) to document the `<project-slug>-<4+alnum>`
   flat-ID / leading-segment assumption baked into `beadIDPattern` (`:29`).
5. Add/extend tests: trailing-hyphen (`cli-`) and double-hyphen (`a--b`) names are newly rejected;
   valid kebab names still accepted; the backslash pin passes; the unified messages carry no raw
   regex.

**Verification**
- [ ] `go build ./... && go test ./internal/idvalidate/...` green
- [ ] `domainNamePattern` rejects trailing/consecutive hyphens, still accepts `security`,
      `cli-handlers`, `context-system`; the POSIX backslash pin passes (RED on revert)
- [ ] The four validators share one human-readable wording convention with no raw regex; the
      `BeadID` doc comment documents the flat-ID assumption
- [ ] golangci-lint clean (American spelling; no new gosec)

**Acceptance Criteria**
- [ ] `domainNamePattern` rejects trailing and consecutive hyphens, still accepts valid kebab names
      (`security`, `cli-handlers`, `context-system`), and has a POSIX backslash test pin.
- [ ] The four idvalidate validators share one error-wording convention (no raw regex in
      user-facing messages), and the `BeadID` doc comment documents the flat-ID format assumption.

**Depends on**
Bead 1 (Bead 1's `internal/idvalidate/**` ownership claim must be on the spec tip before this bead
edits `internal/idvalidate/ids.go`, so the edit attributes to core and passes `adr-divergence`)

## Provenance

| Acceptance Criterion (spec) | Bead | Verified By |
|-----------------------------|------|-------------|
| R0: OWNERSHIP.yaml claims both `internal/idvalidate/**` + `internal/speclist/**`; only bead to edit it; lands first; attributes deletions to core through impl-approve | Bead 1 | Steps 1–3 + verification |
| R1: `-`-prefixed operand errors with ADR-0035 recovery line; `--` on single-ref subcommands; no `--` on range operands; controlled refs still succeed | Bead 2 | Steps 1–4 + verification |
| R2: `--design` ADR list from `PlanFrontmatter.ADRCitations` (via `buildDesignField` signature change); prose scrape retired; orphaned `parseADRIDs` tests removed; prose-only ID not harvested, declared one is | Bead 3 | Steps 1–4 + verification |
| R3: structured `work_chunks` (`id` + `depends_on []int`) parsed by `PlanFrontmatter` (approve exports/parses frontmatter) + consumed by both approve and validate; id↔section alignment guard; both regexes removed; both templates updated; RED-on-revert | Bead 4 (L) | Steps 1–7 + verification |
| R4: `## Key File Paths` from per-bead `work_chunks[*].key_file_paths`; approve producer switched (`plan.go:323-330`); `ExtractFilePathsFromText` deleted from `contextpack/builder.go`; harness site deferred; declared path reaches rendered surface, prose-only not scraped | Bead 5 | Steps 1–6 + verification |
| R5: `internal/speclist` merged into `internal/spec`; caller compiles; suite green, no behavior change | Bead 6 | Steps 1–4 + verification |
| R6: `domainNamePattern` rejects trailing/double hyphens, accepts valid kebab; POSIX backslash pin | Bead 7 | Steps 1,2,5 + verification |
| R7: four validators share one wording convention (no raw regex); `BeadID` doc documents flat-ID assumption | Bead 7 | Steps 3,4,5 + verification |
| `go build` + `go test ./...` + golangci-lint green | All beads | Each bead's verification |
| `mindspec validate spec 097-code-review-cleanup` passes (adr-coverage: every impacted domain mapped to a cited Accepted ADR) | All beads | ADR Fitness + frontmatter `adr_citations` |
