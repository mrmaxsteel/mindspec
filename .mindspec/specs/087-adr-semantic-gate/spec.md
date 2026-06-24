---
approved_at: "2026-05-20T23:09:17Z"
approved_by: user
status: Approved
---
# Spec 087-adr-semantic-gate: ADR semantic gates — plan-time + complete-time enforcement with --override-adr / --supersede-adr

## Goal

Plan approval fails if cited ADRs are not applicable to the spec's
impacted domains, or if any impacted domain is uncovered by cited
ADRs. Bead completion fails when the diff touches a domain whose
ADRs weren't cited, unless `--override-adr "<reason>"` or
`--supersede-adr ADR-NNNN` is supplied — both flags BYPASS the
divergence gate, with `--supersede-adr` additionally creating a
placeholder ADR (`Status: Proposed`) for the user to upgrade to
`Accepted` later. `mindspec approve impl <spec>` performs the same
divergence check as a backstop with broader scope (`main` vs spec
branch). This spec is F1 of the converged transformation plan; F4
(spec 085, `executor.Executor` boundary) and F2 (spec 086, doc-sync
gate + per-domain `OWNERSHIP.yaml`) have landed, so the
path-to-domain machinery this spec depends on
(`internal/validate/ownership.go`) and the named placeholder this
spec fills (`internal/validate/adr_divergence.go::CheckADRDivergence`)
are already on `main`.

## Background

F1 sits atop F2 (086). F2 introduced per-domain `OWNERSHIP.yaml`
manifests at `internal/validate/ownership.go` with
`loadOwnership(root, domain string) (*Ownership, error)` and
`attributeDomain(root, sourcePath string, domains []string)
(string, *Ownership, error)`. F2 also landed an empty `*Result`
stub at `internal/validate/adr_divergence.go` named
`validate.CheckADRDivergence(root, diffRef string, exec
executor.Executor) *Result`, wired into the `approve impl` call
chain BEFORE the four mutating/terminal operations and into
`complete.Run` between the clean-tree check and `closeBeadFn`. F1
fills that stub with the real divergence body, WIDENS the signature
to accept `specDir` and `beadID` (both already in scope at the call
sites — see Requirement 10), and adds the override flags.

The required spec-section list at `internal/validate/spec.go` lines
13-22 already includes `Impacted Domains`. That section is parsed
for domain names (one per `- ` bullet, name = text before the first
`:`, case-folded) by `internal/contextpack/spec.go` lines 18-87. F1
consumes that parsed slice directly; no frontmatter migration. The
plan-time `checkADRCitations` helper lives at
`internal/validate/plan.go` line 366 (called at line 102) and is
the entry point F1 extends.

### Canonical "domain" identifier (revision 2)

The canonical "domain" identifier across this spec, OWNERSHIP.yaml,
and ADR `Domains` fields is the **OWNERSHIP.yaml directory name**
under `.mindspec/domains/<name>/`. As of spec 086, the seeded
set is exactly four:

- `context-system`
- `core`
- `execution`
- `workflow`

The `## Impacted Domains` section in every spec MUST list these
short tags (one per `- ` bullet), NOT file paths. ADR `Domains`
front-matter MUST use the same identifier set. The gate normalises
via case-fold + trim at comparison time (the contextpack parser
already lower-cases at `spec.go:70`). New domains are added by
creating `.mindspec/domains/<name>/OWNERSHIP.yaml`; once that
manifest exists, the name becomes a valid `Impacted Domains` value
and a valid ADR `Domains` value.

The transformation plan section that governs this spec is at
`/Users/Max/replit/mindspec-transformation-plan.md` lines ~87-128
and is converged after three rounds of adversarial debate. This
spec implements that block verbatim; it does not redesign.

## Impacted Domains

- context-system
- core
- execution
- workflow

### Affected packages (per domain)

- **`internal/validate/`** (domain: `core`) — the centerpiece.
  Three files change and one new file is added:
  - `plan.go` (line 366: `checkADRCitations`) is extended. For
    each cited ADR, the helper loads the ADR via the `adr.Store`
    already in scope, intersects `ADR.Domains` with the spec's
    parsed impacted-domains (obtained via `contextpack.ParseSpec`
    against the same spec directory), and emits
    `AddError("adr-cite-irrelevant", ...)` on empty intersection.
    A NEW `checkADRCoverage(r, store, citations, impactedDomains)`
    helper is added in the same file: every impacted domain must
    have at least one cited Accepted ADR whose `Domains` contains
    it. Missing → `AddError` with the hint `mindspec adr create
    --domain <d>`. Superseded ADRs do NOT satisfy coverage unless
    the superseding chain head is also cited (see Requirement 17).
  - `spec.go` — no body change; the required-section list at
    lines 13-22 already includes `Impacted Domains`, which is the
    input F1 reads.
  - `adr_divergence.go` — the empty `CheckADRDivergence` stub at
    lines 1-25 is FILLED IN and the signature is WIDENED to
    `CheckADRDivergence(root, diffRef string, exec
    executor.Executor, specDir string, beadID string) *Result`
    (see Requirement 10). The function delegates to the new
    `ValidateDivergence` helper and returns the same `*Result`.
    The `SubCommand: "adr-divergence"` label is preserved so the
    spec 086 AST call-order test continues to pass.
  - `divergence.go` (NEW) — exports
    `ValidateDivergence(exec executor.Executor, root, specDir,
    beadID string) *Result`. Computes `changed :=
    exec.ChangedFiles(base, head)`. Filters `changed` to drop any
    file whose first path segment is in {`viz`, `agentmind`,
    `bench`} (HC-4, revision 5). Maps each remaining file to its
    owning domain via `attributeDomain(root, file,
    impactedDomains)` from `ownership.go`. When `attributeDomain`
    returns `("", nil, nil)` (no manifest claims the file), the
    walker emits `AddError("adr-divergence-unowned", ...)` per
    Requirement 9b (revision 4). For each touched domain not
    covered by a cited ADR: `AddError("adr-divergence-uncovered",
    ...)` naming the file → manifest → uncovered domain chain so
    the operator knows exactly which `OWNERSHIP.yaml` decided
    ownership and which domain needs an ADR.
- **`internal/contextpack/`** (domain: `context-system`) —
  `spec.go` lines 18-87 (`ParseSpec`) is the source of truth for
  the spec's impacted-domains list. No structural change; F1
  reuses it from both the plan-time and complete-time call sites.
- **`internal/approve/`** (domain: `workflow`) — `impl.go` ALREADY
  calls `validate.CheckADRDivergence(root, base, exec)` at line
  138 per spec 086's reorder (between `validate.ValidateDocs` at
  line 125 and `implRunBDCombinedFn("close", epicID)` at line
  145). F1 widens the call to
  `validate.CheckADRDivergence(root, base, exec, specDir, "")`
  (passing empty `beadID` to trigger the broader-scope spec-branch
  diff). `specDir` is read from the in-scope spec-mode
  session/lookup that `impl.go` already uses. The
  `--override-adr "<reason>"` and `--supersede-adr ADR-NNNN` flags
  are wired into `ImplOpts` (additive to `AllowDocSkew` from 086);
  when EITHER is set, the divergence call is skipped (both flags
  bypass the gate — revision 1). Metadata writes happen AFTER
  `exec.FinalizeEpic` returns nil at line 171, matching the
  post-terminal-mutation discipline that spec 086 established for
  `mindspec_impl_skew_*` at line 182. Writes go through the
  `implMergeMetadataFn` test seam (revision 10).
- **`internal/complete/`** (domain: `workflow`) — `complete.go`
  ALREADY calls `validate.CheckADRDivergence(root, base, exec)` at
  line 165 (between `validate.ValidateDocs` at line 157 and
  `closeBeadFn` at line 174). F1 widens the call to
  `validate.CheckADRDivergence(root, base, exec, specDir,
  beadID)`. The `--override-adr` and `--supersede-adr` flags are
  wired into `CompleteOpts` (additive to `AllowDocSkew` from 086).
  Either flag skips the divergence call (revision 1) and records
  audit metadata via the `completeMergeMetadataFn` test seam
  (revision 10) AFTER the terminal mutation (`exec.CompleteBead`)
  succeeds, mirroring the 086 pattern at `complete.go` line 210
  onwards. `--supersede-adr ADR-NNNN` additionally pre-creates the
  named ADR via `internal/adr` (`Status: Proposed`, `Domains`
  deduced from the violated domain — revision 8) BEFORE the gate
  is skipped; the new ADR file lives in the repo for the user to
  upgrade to `Status: Accepted` in a follow-up bead.
- **`cmd/mindspec/`** (domain: `execution`) — `complete.go` and
  `impl.go` gain `--override-adr "<reason>"` and `--supersede-adr
  ADR-NNNN` flags. `--override-adr ""` (empty reason) is rejected
  at flag-parse / CLI-binding time with `"--override-adr requires
  a non-empty reason"`, matching 086's `--allow-doc-skew` empty
  rejection. `--supersede-adr` accepts an ADR id matching the
  existing `ADR-NNNN` format validation in `internal/adr`. The
  two flags are mutually exclusive on a single invocation:
  passing both returns `"--override-adr and --supersede-adr are
  mutually exclusive"`.
- **`internal/adr/`** (domain: `core`) — read for the existing
  API surface (`create.go`, `show.go`, `store.go`, `parse.go`,
  `supersede.go`, `list.go`, `filestore.go`). No structural
  change. `--supersede-adr` calls the existing creator
  (`adr.Create` or equivalent) with `Status: Proposed` and the
  deduced `Domains` slice; `Supersedes` is left empty in v1
  (out of scope, see `## Scope`). The supersede chain walker in
  `parse.go` / `supersede.go` is reused by Requirement 17.
- **`internal/bead/`** (domain: `core`) — read for
  `bead.MergeMetadata` (reused from 086 via the test seam) and
  `bead.GitUserEmail` from `internal/bead/identity.go` for the
  `_by` audit field.

## ADR Touchpoints

- [ADR-0032-adr-semantic-gates.md](../../adr/ADR-0032-adr-semantic-gates.md)
  (**new**): Records the plan-time and complete-time enforcement
  contract: cited-but-irrelevant ADRs error, uncovered impacted
  domains error, divergence-detected at complete error. Records
  the canonical domain identifier (OWNERSHIP.yaml directory name)
  and the domain-overlap algorithm (case-folded, trim-whitespace,
  exact set intersection — no hierarchy, no aliases in v1).
  Records the superseded-chain rule (a cited ADR with
  `Status: Superseded` does NOT satisfy coverage unless the
  superseding chain head is also cited) with cycle detection and
  max chain length 10. Records the `--override-adr "<reason>"` and
  `--supersede-adr ADR-NNNN` semantics (both bypass the gate; the
  latter additionally creates a placeholder ADR with `Status:
  Proposed`), the distinct audit-trail metadata namespaces
  (`mindspec_adr_override_reason`/`_at`/`_by` vs
  `mindspec_adr_supersede_id`/`_reason`/`_at`/`_by`), the
  post-terminal-mutation write discipline, the mutual-exclusivity
  of the two flags on a single invocation (both namespaces MAY
  co-exist across separate invocations on the same bead), and the
  no-env-var-escape-hatch rule.
- [ADR-0030-executor-boundary.md](../../adr/ADR-0030-executor-boundary.md):
  Prerequisite. F1 uses `executor.Executor.ChangedFiles` and
  `executor.Executor.MergeBase` (added in 085) inside the new
  `ValidateDivergence` helper. ADR-0032 cites ADR-0030 as the
  git/process I/O boundary it builds on; divergence reads go
  through `exec.ChangedFiles(base, head)`, never through
  `os/exec` or `internal/gitutil`.
- [ADR-0031-doc-sync-gate.md](../../adr/ADR-0031-doc-sync-gate.md):
  Sibling. F1 follows ADR-0031's enforcement pattern (gate
  before every mutating/terminal operation; explicit recorded
  override; post-terminal-mutation write for the audit row) and
  reuses the `OWNERSHIP.yaml` machinery ADR-0031 introduces.
  ADR-0032 cites ADR-0031 as the immediate precedent and
  explicitly notes that `--override-adr`, `--supersede-adr`, and
  `--allow-doc-skew` are independent overrides (any combination
  may be passed; `--override-adr` and `--supersede-adr` are the
  only mutually exclusive pair).
- Earlier ADRs about ADR processes: ADR-0014 through ADR-0024
  are scanned during ADR-0032 drafting for any prior decision on
  ADR citation semantics, domain tagging on ADRs, or supersede
  workflow. Any touchpoint found is recorded in ADR-0032's
  "Related ADRs" section.
- **ADR number reservation.** At spec-draft time the highest
  existing ADR is `ADR-0031-doc-sync-gate.md`, so `ADR-0032` is
  the intended number. The implementer MUST re-check
  `.mindspec/adr/` at PR-open time; if `ADR-0032` has been
  claimed by a sibling spec landing first, renaming the file and
  updating cross-references (Background, Impacted Domains, this
  section, Acceptance Criteria) is a **1-bead followup** under
  this spec, not a spec amendment.

## Requirements

### Hard Constraints (from converged plan)

1. **HC-1 F1 lands AFTER F2 (spec 086).** F2 has merged; the
   `OWNERSHIP.yaml` machinery (`loadOwnership`, `attributeDomain`
   in `internal/validate/ownership.go`) and the named
   `CheckADRDivergence` stub
   (`internal/validate/adr_divergence.go`) are on `main`. This
   spec is unblocked.
2. **HC-2 Solo-developer UX preserved.** Both overrides exist on
   both lifecycle commands: `--override-adr "<reason>"` and
   `--supersede-adr ADR-NNNN` on `mindspec complete` and on
   `mindspec approve impl`. Each is explicit, flag-driven, and
   recorded in audit metadata under distinct key namespaces. No
   env-var escape hatch.
3. **HC-3 Existing test suite preserved.** No test is skipped,
   excluded, or marked `t.Skip` relative to `main`. New tests are
   additive.
4. **HC-4 `viz/agentmind/bench` excluded.** F1 does not enforce
   on diffs under those trees. Enforcement is in TWO layers:
   - The `OWNERSHIP.yaml` schema rejection of `viz/`,
     `agentmind/`, `bench/` first-segment entries (already
     enforced by `ownership.go::checkExcludedSegment` lines
     86-92).
   - The `ValidateDivergence` walker MUST filter
     `exec.ChangedFiles` output to drop any file whose first path
     segment is in {`viz`, `agentmind`, `bench`} BEFORE
     `attributeDomain` is called (revision 5; schema rejection
     alone is insufficient because it only blocks manifest claims,
     not diff input).
5. **HC-5 Every commit `go build ./... && go test -short ./...`
   green.** Including the `checkADRCitations` extension commit,
   the new `checkADRCoverage` commit, the new `divergence.go`
   commit, the `CheckADRDivergence` body-fill + signature-widen
   commit, and the CLI-flag commits.
6. **HC-6 AST boundary lint from spec 085 stays green.** The new
   `divergence.go` MUST NOT import `os/exec` or
   `github.com/mrmaxsteel/mindspec/internal/gitutil`, and MUST
   NOT call `exec.Command("git", ...)` or `exec.Command("bd",
   ...)`. All git reads go through
   `executor.Executor.ChangedFiles` and
   `executor.Executor.MergeBase`.

### Spec-specific

7. **Plan-time check 1 — irrelevant citation.** Extend
   `internal/validate/plan.go::checkADRCitations` (line 366).
   For each cited ADR, load via the in-scope `adr.Store`,
   intersect `ADR.Domains` with the spec's parsed impacted-domains
   (obtained via `contextpack.ParseSpec` against the spec
   directory). Empty intersection → `AddError("adr-cite-irrelevant",
   fmt.Sprintf("cited ADR %s declares domains %v which do not
   intersect spec impacted domains %v", id, adr.Domains,
   spec.Domains))`. The existing Superseded/Proposed warning
   behaviour at lines 374-384 is preserved.
8. **Plan-time check 2 — uncovered domain.** Add
   `checkADRCoverage(r *Result, store adr.Store, citations
   []ADRCitation, impactedDomains []string)` in `plan.go`. For
   each impacted domain, scan the cited ADRs; the domain is
   covered when at least one cited ADR has `Status: Accepted`
   AND `Domains` contains the domain (case-folded set
   intersection per the canonical domain identifier defined in
   Background). A cited ADR with `Status: Superseded` does NOT
   satisfy coverage unless the superseding chain head (resolved
   per Requirement 17) is also cited. Uncovered domain →
   `AddError("adr-coverage-missing", fmt.Sprintf("impacted
   domain %q has no cited Accepted ADR; run: mindspec adr
   create --domain %s", d, d))`.
   *Annotation (2026-06-11): superseded in part by PR #126
   (tri-state coverage) — a cited Proposed ADR now satisfies
   plan-time coverage with an advisory `adr-coverage-proposed`
   warning; Accepted is enforced at impl-approve. See ADR-0032
   Amendment.*
9. **Divergence check at complete (and unowned-file detection).**
   New file `internal/validate/divergence.go` exporting
   `ValidateDivergence(exec executor.Executor, root, specDir,
   beadID string) *Result`. The function:
   - Resolves the spec branch's fork point as `base` and the
     bead branch tip as `head` (callers pass these in as the
     `diffRef` argument shape already established by spec 086 —
     `complete.go:157` passes `base` from
     `exec.MergeBase(specBranch, "HEAD")`, `impl.go:125` passes
     `base` from `exec.MergeBase("main", specBranch)`). When
     `beadID` is empty (approve-impl backstop path), the walker
     enumerates the entire spec-branch diff (main → spec/<id>
     HEAD).
   - Computes `changed := exec.ChangedFiles(base, head)` and
     FILTERS out any file whose first path segment is in {`viz`,
     `agentmind`, `bench`} (HC-4 layer 2, revision 5).
   - Loads the spec's impacted-domains list via
     `contextpack.ParseSpec(specDir)`.
   - Maps each remaining changed file to its owning domain via
     `attributeDomain(root, file, impactedDomains)` from
     `ownership.go`.
   - **(a) Uncovered domain.** When `attributeDomain` returns a
     non-empty domain that is NOT covered by any cited ADR:
     `AddError("adr-divergence-uncovered", fmt.Sprintf("file %s
     attributed to domain %q (manifest: %s) but no cited ADR
     covers %q", file, domain, manifestOrFallback, domain))`.
   - **(b) Unowned file (revision 4).** When `attributeDomain`
     returns `("", nil, nil)` (no manifest in the impacted-domains
     set claims the file): `AddError("adr-divergence-unowned",
     fmt.Sprintf("file %s is not claimed by any OWNERSHIP.yaml
     for the spec's impacted domains %v; add it to an existing
     manifest or create a new domain dir at
     .mindspec/domains/<name>/OWNERSHIP.yaml", file,
     impactedDomains))`.

   Both error subcommands name the file → manifest → domain
   chain so the operator can fix the right artefact.
10. **Fill and widen `CheckADRDivergence` (revision 3).** Replace
    the empty body of `internal/validate/adr_divergence.go::
    CheckADRDivergence` (lines 20-25) with a delegation to
    `ValidateDivergence`. The signature is WIDENED to
    `CheckADRDivergence(root, diffRef string, exec
    executor.Executor, specDir string, beadID string) *Result`.
    The `SubCommand: "adr-divergence"` label is preserved so the
    spec 086 AST call-order test continues to pass without
    modification. `specDir` is in scope at both call sites
    (`complete.go` and `impl.go` both read it from
    `mindspec.session.json` for the current spec). `beadID` is
    derived from `CompleteOpts.BeadID` at the `complete` call site
    and is passed as `""` at the `approve impl` call site (which
    triggers the broader spec-branch diff). The spec-086 AST
    call-order test asserts the symbol name and call ORDER, not
    the argument list, so the widening is non-breaking.
11. **`approve impl` backstop.** Per the spec-086 wiring at
    `impl.go:138`, `CheckADRDivergence` already runs between
    doc-sync and epic-close. F1's body-fill + signature-widen
    means this call now performs the real divergence check against
    the broader scope (`main` vs spec branch, triggered by empty
    `beadID`) as a backstop catching any drift between per-bead
    completes and the final merged epic.
12. **Override semantics — `--override-adr "<reason>"` (one-shot).**
    The flag SKIPS the divergence check entirely (revision 1) and
    records audit metadata under the `mindspec_adr_override_*`
    namespace (revision 7):
    - `mindspec_adr_override_reason`: the verbatim reason string
    - `mindspec_adr_override_at`: UTC RFC3339 timestamp
    - `mindspec_adr_override_by`: `bead.GitUserEmail()` (best
      effort; falls back to `"unknown"`)

    Writes go through the `completeMergeMetadataFn` /
    `implMergeMetadataFn` test seams (revision 10) AFTER the
    terminal mutation succeeds — `exec.CompleteBead` returns nil
    on the `complete` path (mirroring spec 086's discipline at
    `complete.go` line 210 onwards) and `exec.FinalizeEpic`
    returns nil on the `approve impl` path (mirroring spec 086's
    `mindspec_impl_skew_*` discipline at `impl.go:182`). Empty
    reason rejected at flag-parse time: `"--override-adr requires
    a non-empty reason"`.
13. **Override semantics — `--supersede-adr ADR-NNNN` (revisions
    1, 7, 8, 11).** The flag does TWO things:
    - **(a) Pre-create the named ADR.** Via the existing
      `internal/adr` create surface with `Status: Proposed` and
      `Domains` seeded from the first uncovered domain captured
      by the divergence walker before the supersede call
      (revision 8). When invoked defensively without a prior
      divergence violation, `Domains` is empty and the user
      populates it after creation. The optional `Supersedes`
      field is left empty in v1 (out of scope, see `## Scope`).
      The new ADR file lives in the repo for the user to upgrade
      to `Status: Accepted` later.
    - **(b) BYPASS the divergence gate (revision 1).** Same
      gate-skip semantics as `--override-adr`. `--supersede-adr`
      is a richer form of `--override-adr` that ALSO creates the
      placeholder ADR; both bypass the gate. The gate is NOT
      re-run after pre-creation — the new ADR has
      `Status: Proposed`, which would not satisfy Req 8's
      `Status: Accepted` requirement, so re-running would fail.
      *Annotation (2026-06-11): superseded in part by PR #126
      (tri-state coverage) — a cited Proposed placeholder now
      satisfies coverage at plan/bead time (warning), so the
      "re-running would fail" rationale no longer holds; the
      bypass semantics themselves are unchanged. See ADR-0032
      Amendment.*

    Audit metadata is written under the DISTINCT
    `mindspec_adr_supersede_*` namespace (revision 7), through
    the same test seam (revision 10), AFTER the terminal mutation
    succeeds:
    - `mindspec_adr_supersede_id`: the new ADR id (e.g.,
      `"ADR-0099"`)
    - `mindspec_adr_supersede_reason`: auto-filled to
      `"superseded by ADR-NNNN"` where NNNN is the new ADR id
      (revision 11); no user-facing prompt
    - `mindspec_adr_supersede_at`: UTC RFC3339 timestamp
    - `mindspec_adr_supersede_by`: `bead.GitUserEmail()` (best
      effort; falls back to `"unknown"`)
14. **No env-var escape hatch.** Neither override is reachable
    via environment variable. The CLI flags are the only entry
    points.
15. **Mutual exclusivity (single invocation only).**
    `--override-adr` and `--supersede-adr` on a SINGLE CLI
    invocation return `"--override-adr and --supersede-adr are
    mutually exclusive"` at flag-parse / CLI-binding time.
    However, the two metadata namespaces
    (`mindspec_adr_override_*` and `mindspec_adr_supersede_*`)
    MAY co-exist on a single bead/epic across separate
    invocations (revision 7) — for example, a multi-domain
    refactor that uses `--override-adr` on one bead and
    `--supersede-adr` on another within the same spec.
16. **Domain-overlap algorithm.** Case-folded, trim-whitespace,
    exact set intersection over the canonical domain identifier
    (OWNERSHIP.yaml directory name; see Background). No
    hierarchy. No aliases in v1. Implementation note: domain
    strings from both the spec parser (`contextpack.ParseSpec`
    already lower-cases at line 70) and from ADR `Domains` lists
    are normalised the same way at comparison time.
17. **Superseded-chain rule with cycle detection (revision 6).**
    A cited ADR with `Status: Superseded` does NOT satisfy
    coverage unless the superseding chain head is also cited in
    the same plan. The walker:
    - Starts at the cited ADR and follows `ADR.SupersededBy`
      links.
    - Tracks visited IDs in a set; on revisit →
      `AddError("adr-supersede-cycle", "superseded chain has a
      cycle starting at ADR-NNNN")` and aborts the walk.
    - Bounds chain length at 10; on overflow →
      `AddError("adr-supersede-chain-too-long", "superseded
      chain starting at ADR-NNNN exceeds max length 10")` and
      aborts.
    - On terminal: the head ADR MUST have `Status: Accepted` AND
      MUST be cited in the same plan for the original cite to
      satisfy coverage.
    The plan-time `checkADRCoverage` helper and the complete-time
    `ValidateDivergence` helper both apply this rule; the
    existing `adr-cite-superseded` warning at `plan.go:379` is
    preserved as additional signal.

## Scope

### In Scope

- Extension of
  `internal/validate/plan.go::checkADRCitations` (line 366) to
  add the irrelevant-citation check per Requirement 7.
- New `checkADRCoverage` helper in `plan.go` per Requirement 8.
- New file `internal/validate/divergence.go` exporting
  `ValidateDivergence(exec executor.Executor, root, specDir,
  beadID string) *Result` per Requirement 9, including both the
  uncovered-domain and unowned-file error paths.
- Filling the body AND widening the signature of
  `internal/validate/adr_divergence.go::CheckADRDivergence` per
  Requirement 10; the `SubCommand` label is preserved.
- `--override-adr "<reason>"` and `--supersede-adr ADR-NNNN`
  flags on `mindspec complete` and `mindspec approve impl` per
  Requirements 12, 13, 15. Both flags bypass the gate; the
  latter additionally pre-creates the placeholder ADR.
- Audit-trail metadata writes under two distinct namespaces
  (`mindspec_adr_override_*` and `mindspec_adr_supersede_*`) on
  the bead (on `complete`) or on the spec epic (on `approve
  impl`) per Requirements 12, 13, written AFTER the terminal
  mutation succeeds, via the test-seam indirection
  (`completeMergeMetadataFn` / `implMergeMetadataFn`).
- HC-4 viz/agentmind/bench filtering inside `ValidateDivergence`
  per HC-4 layer 2.
- Domain-overlap algorithm: case-folded, trim-whitespace, exact
  set intersection over the canonical domain identifier per
  Requirement 16.
- Superseded-chain rule with cycle detection and max chain
  length 10 per Requirement 17.
- ADR-0032 drafted and accepted as part of this spec.

### Out of Scope

- **Environment-variable escape hatch** — no env var bypass for
  either override.
- **Domain aliases or hierarchy in v1** — strict exact-match
  set intersection only. A future spec may introduce a domain
  alias registry or hierarchical tags; this spec does not.
- **F3 context-pack budgeter** — spec 088.
- **F5 ceremony collapse** — spec 089.
- **Cross-domain refactor heuristics beyond the override flag** —
  if a refactor legitimately spans many domains, the operator
  uses `--override-adr "<reason>"` (audit row captures the
  reason) or `--supersede-adr ADR-NNNN` (introducing a new ADR
  for the user to upgrade later). No heuristic auto-allowance.
- **Populating the `Supersedes` field on `--supersede-adr`** —
  v1 leaves the field empty; a future spec may add a companion
  flag.
- **Multi-domain single-invocation supersede** — a single
  `--supersede-adr` invocation creates a single ADR seeded with
  one domain. For multi-domain coverage the operator either
  uses `--override-adr` once (covering everything in audit) or
  invokes `--supersede-adr` on multiple sequential bead
  completes within the spec.
- **`mindspec doctor` override-count warning** — the
  "more than N overrides per spec" doctor warning mentioned in
  the plan's risk section is a follow-up; v1 records the audit
  metadata but does not aggregate or warn on it.

## Acceptance Criteria

- [ ] `TestPlanRejectsIrrelevantADRCitation` passes: a spec
  whose `## Impacted Domains` section parses to `["core"]`
  and whose plan cites an ADR with `Domains: [execution]`
  causes `ValidatePlan` (or the relevant entry point exercising
  `checkADRCitations`) to return a `*Result` whose `Errors`
  contain a `"adr-cite-irrelevant"` finding naming the ADR id,
  its `Domains`, and the spec's impacted domains.
- [ ] `TestPlanRejectsUncoveredDomain` passes: a spec whose
  `## Impacted Domains` parses to `["core"]` and whose plan
  cites no ADR with `Domains` containing `"core"` causes
  `ValidatePlan` to return a `*Result` whose `Errors` contain
  `"adr-coverage-missing"` with a message containing the hint
  string `mindspec adr create --domain core`.
- [ ] `TestCompleteRejectsUndeclaredDomainTouch` passes: a bead
  whose diff (`exec.ChangedFiles(base, head)`) touches a file
  under `internal/core/foo.go`, against a plan whose cited ADRs
  only have `Domains: [execution]`, causes `complete.Run` to
  return an error whose message contains
  `"adr-divergence-uncovered"`, names `internal/core/foo.go`,
  names the resolved manifest path or the `<fallback:
  internal/core/**>` marker, and names the uncovered domain. The
  bead is NOT closed; `exec.CompleteBead` is NOT invoked.
- [ ] `TestCompleteRejectsUnownedFile` passes (revision 4): a
  bead whose diff touches a file under
  `internal/some-new-dir/foo.go` (a path no OWNERSHIP.yaml in
  the spec's impacted-domains set claims) causes `complete.Run`
  to return an error whose message contains
  `"adr-divergence-unowned"` and names the file plus the
  impacted-domains set the walker consulted.
- [ ] `TestSupersedeUnblocks` passes (revision 1, 9): the bead
  invoked with `opts.SupersedeADR = "ADR-0099"` (and `ADR-0099`
  not yet existing) pre-creates `ADR-0099` with `Status:
  Proposed` and `Domains` containing the previously-violated
  domain, then BYPASSES the gate and closes the bead. The new
  ADR file MUST exist on disk at
  `.mindspec/adr/ADR-0099-*.md` with parseable frontmatter
  showing `Status: Proposed` (revision 9 falsifiability fix).
  The bead's metadata contains `mindspec_adr_supersede_id` =
  `"ADR-0099"`, `mindspec_adr_supersede_reason` containing
  `"ADR-0099"`, `mindspec_adr_supersede_at` parseable as RFC3339,
  and `mindspec_adr_supersede_by` non-empty (revision 7
  namespace).
- [ ] `TestOverrideUnblocks` passes: the same bead invoked with
  `opts.OverrideADR = "wip — core ADR coming in followup"`
  skips the divergence call, closes the bead, and writes
  `mindspec_adr_override_reason` (= verbatim reason),
  `mindspec_adr_override_at` (parseable RFC3339), and
  `mindspec_adr_override_by` (non-empty) into the bead metadata
  AFTER `exec.CompleteBead` returns nil. If `exec.CompleteBead`
  errors, no override metadata is written.
- [ ] `TestOverrideMetadataGoesThroughSeam` passes (revision 10):
  with `completeMergeMetadataFn` swapped for a recording stub,
  the override write is captured by the stub (not by direct
  `bead.MergeMetadata`).
- [ ] `TestSupersededADRDoesNotSatisfyCoverage` passes: a plan
  citing only an ADR with `Status: Superseded` (whose
  `SupersededBy` ADR exists and would cover the impacted domain
  but is NOT itself cited) causes `checkADRCoverage` to emit
  `"adr-coverage-missing"`. With the superseding chain head
  also cited in the same plan, the same scenario passes.
- [ ] `TestSupersedeChainCycleDetected` passes (revision 6): a
  plan citing an ADR whose `SupersededBy` chain forms a cycle
  causes `checkADRCoverage` to emit `"adr-supersede-cycle"` and
  abort the walk without infinite loop.
- [ ] `TestSupersedeChainTooLong` passes (revision 6): a plan
  citing an ADR whose `SupersededBy` chain exceeds length 10
  causes `checkADRCoverage` to emit
  `"adr-supersede-chain-too-long"` and abort.
- [ ] `TestApproveImplBackstopRunsDivergence` passes: a
  spec-branch diff against `main` that touches a domain not
  covered by any cited ADR causes `ApproveImpl` to return an
  error containing `"adr-divergence-uncovered"`. The epic is
  NOT closed; `mindspec_phase: done` is NOT written;
  `exec.FinalizeEpic` is NOT invoked. Same scenario with
  `opts.OverrideADR = "<reason>"` succeeds and writes
  `mindspec_adr_override_*` to the EPIC's metadata AFTER
  `exec.FinalizeEpic` returns nil.
- [ ] `TestVizAgentmindBenchFiltered` passes (revision 5): a
  bead diff touching `viz/foo.go`, `agentmind/bar.go`, and
  `bench/baz.go` (and no other files) causes `complete.Run` to
  pass the divergence gate (zero divergence errors) because all
  three files are filtered out before `attributeDomain` is
  called.
- [ ] `TestOverrideAndSupersedeMutuallyExclusive` passes: passing
  both flags on a single CLI invocation returns an error
  containing `"mutually exclusive"`.
- [ ] `TestOverrideEmptyReasonRejected` passes: passing
  `--override-adr ""` returns an error containing `"requires a
  non-empty reason"`.
- [ ] `TestCheckADRDivergenceSignatureWidened` passes (revision
  3): the symbol `validate.CheckADRDivergence` has signature
  `(root, diffRef string, exec executor.Executor, specDir
  string, beadID string) *Result` and the spec-086 AST
  call-order test still passes against the widened signature.
- [ ] `cmd/mindspec/complete.go` and `cmd/mindspec/impl.go` (or
  the relevant subcommand files identified by the AST audit at
  PR-open time) expose `--override-adr "<reason>"` and
  `--supersede-adr ADR-NNNN`. Both flags are independent of
  (and may co-exist with) `--allow-doc-skew` from spec 086.
- [ ] `ADR-0032-adr-semantic-gates.md` exists under
  `.mindspec/adr/`, status `Accepted`, citing ADR-0030 and
  ADR-0031, recording the algorithm, override semantics, and
  metadata namespaces per Requirements 7-17.
- [ ] All existing tests still pass; AST boundary lint from
  spec 085
  (`internal/lint/boundary_test.go::TestEnforcementHasNoGitLeaks`)
  stays green. The new `divergence.go` does not import
  `os/exec` or `internal/gitutil` and does not call
  `exec.Command("git", ...)` or `exec.Command("bd", ...)`.
- [ ] `go build ./... && go test -short ./...` is green on
  every commit of the F1 branch (verified by per-commit CI or
  by `git rebase -x`).

## Validation Proofs

- `go test ./internal/validate -run TestPlanRejectsIrrelevantADRCitation -v`
  — PASS; error message in log shows ADR id, ADR Domains, spec
  impacted domains.
- `go test ./internal/validate -run TestPlanRejectsUncoveredDomain -v`
  — PASS; hint string in log.
- `go test ./internal/complete -run TestCompleteRejectsUndeclaredDomainTouch -v`
  — PASS; file → manifest → domain chain in log.
- `go test ./internal/complete -run TestCompleteRejectsUnownedFile -v`
  — PASS; file + impacted-domains set in log.
- `go test ./internal/complete -run TestSupersedeUnblocks -v` —
  PASS; new `ADR-0099` file present on disk with `Status:
  Proposed`; bead metadata dump shows four
  `mindspec_adr_supersede_*` keys.
- `go test ./internal/complete -run TestOverrideUnblocks -v` —
  PASS; bead metadata dump shows three `mindspec_adr_override_*`
  keys.
- `go test ./internal/complete -run TestOverrideMetadataGoesThroughSeam -v`
  — PASS; recording stub captured the write.
- `go test ./internal/validate -run TestSupersededADRDoesNotSatisfyCoverage -v`
  — PASS.
- `go test ./internal/validate -run TestSupersedeChainCycleDetected -v`
  — PASS.
- `go test ./internal/validate -run TestSupersedeChainTooLong -v`
  — PASS.
- `go test ./internal/complete -run TestVizAgentmindBenchFiltered -v`
  — PASS.
- `go test ./internal/approve -run TestApproveImplBackstopRunsDivergence -v`
  — PASS; on failure path no `FinalizeEpic` call and no
  `mindspec_phase: done` write; on override path EPIC metadata
  contains the three `mindspec_adr_override_*` keys.
- `go test ./internal/validate -run TestCheckADRDivergenceSignatureWidened -v`
  — PASS.
- `go test ./internal/lint -run TestEnforcementHasNoGitLeaks -v` —
  PASS (new `divergence.go` does not regress 085's boundary).
- `go build ./... && go test -short ./...` — exit 0 on every
  commit of this spec's branch.
- Manual: `mindspec complete <bead>` against a bead touching an
  uncovered domain — exit non-zero, error names file + manifest +
  domain. Re-run with `--override-adr "test override"` — exit 0,
  bead closes, metadata records reason under
  `mindspec_adr_override_*`. Re-run a different uncovered bead
  with `--supersede-adr ADR-0099` — exit 0, bead closes, new
  ADR file on disk with `Status: Proposed`, metadata records id
  under `mindspec_adr_supersede_*`.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-05-20
- **Notes**: Approved via mindspec approve spec