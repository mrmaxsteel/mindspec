---
adr_citations:
    - id: ADR-0014
    - id: ADR-0023
    - id: ADR-0030
    - id: ADR-0034
approved_at: "2026-06-11T17:00:12Z"
approved_by: user
bead_ids:
    - mindspec-fpfh.1
    - mindspec-fpfh.2
    - mindspec-fpfh.3
    - mindspec-fpfh.4
    - mindspec-fpfh.5
spec_id: 091-ownership-discovery
status: Approved
version: "1"
---
# Plan: 091-ownership-discovery

This plan decomposes spec 091 (Reqs 8-22, HC-1..HC-7) into five beads
along the spec's natural surface seams: loader + config foundation,
doc-sync override semantics, prompt emitters + migrate phases, doctor
checks + fixers, and the complete/approve warnings pipe. Bead
descriptions cite requirement and AC numbers from the spec rather
than inlining their text (per the mindspec-lawq rule: bead payloads
stay lean; full text lives in `spec.md`).

## ADR Fitness

The spec's impacted domains are **workflow** (doctor, doc-sync
validation, complete/approve — all claimed by the workflow
OWNERSHIP manifest), **execution** (the `cmd/mindspec` command
surface: two new populate subcommands plus the migrate phases, per
the spec-092 precedent for command-layer changes), and **core**
(`internal/config` gains the `source_globs:` field). Frontmatter
citations cover each:

- **ADR-0034** (ceremony collapse — Accepted, domains: workflow).
  Covers the workflow domain. Established `mindspec doctor` as the
  pre-mutation diagnostic/migration surface
  (`--dry-run-migration`) and migration-on-first-touch semantics;
  spec 091 extends exactly that surface (new doctor checks +
  `--fix` fixers, Reqs 8, 15, 17, 18, 20, 21) and follows the same
  disclose-then-migrate pattern for the fallback removal (HC-6).
  Sound; adhere.
- **ADR-0023** (beads as single state authority — Accepted,
  domains: workflow, git, state). Covers the workflow domain.
  Spec 091 honours its no-sidecar-state stance: the Req 22
  migration-status nudge is deliberately stateless (HC-2 — no new
  persisted state, no new files outside `.mindspec/` scaffolds).
  Sound; adhere.
- **ADR-0030** (executor boundary — Accepted, domains: execution,
  validation, lifecycle, lint). Covers the execution domain.
  Doc-sync already consumes diffs via
  `Executor.ChangedFiles`; spec 091 adds no new git/process
  interaction (the Req 17 dead-manifest check is a plain filesystem
  walk, not a git operation), so no boundary change is needed.
  Sound; adhere — `go test ./internal/lint/...` stays green.
- **ADR-0014** (canonical document root under `.mindspec` —
  Accepted, domains: core, context-system, workflow). Covers the
  core domain. Every artifact this spec scaffolds or extends lives
  under the ADR-0014 canonical root: the empty-stub
  `OWNERSHIP.yaml` files under `.mindspec/domains/<domain>/`
  and the new `source_globs:` field in `.mindspec/config.yaml`
  (Req 11, parsed by `internal/config`). No legacy-fallback paths
  are introduced. Sound; adhere.

Evaluated but not citable in frontmatter:

- **ADR-0031** (doc-sync gate — Accepted, domains: validation,
  doc-sync, lifecycle, ownership). Prerequisite for everything in this
  spec: it records the OWNERSHIP.yaml schema (unchanged here) and the
  silent `internal/<domain>/**` fallback that Req 13 removes. Per the
  spec's ADR Touchpoints, Bead 1 amends it with a superseded-in-part
  note pointing at the new ownership-discovery ADR; the schema and the
  warning-to-error promotion it records stay authoritative.
  Its `Domain(s)` vocabulary (validation, doc-sync, lifecycle,
  ownership) predates the repo's manifest domain names and does not
  intersect the spec's impacted domains, so a frontmatter citation
  would register `adr-cite-irrelevant`; it is evaluated here in
  prose instead. (Bead 1's amendment may align its `Domain(s)` line
  with the manifest vocabulary if the gate owner wants it citable.)
- **Proposed new ADR — ownership discovery (ZFC)**: Bead 1 authors
  the spec's ADR Touchpoints items (a)-(i): ZFC stance, empty-stub
  scaffold, fallback removal, disclosed-default override semantics,
  continuous-accuracy Warns, hygiene Warns, no-auto-mutation policy,
  deferrals, and the accepted wrong-but-resolving gap.
  **Number resolution**: the spec reserves "ADR-0035 if still free at
  bead-claim time, else the next free integer". `ADR-0035` is NOT
  free — spec 092's Bead 1 created
  `ADR-0035-agent-error-contract.md` on the 092 branch (highest ADR
  on main and on the 091 branch is still `ADR-0034-ceremony-collapse.md`).
  Bead 1 therefore creates the ADR as
  **`ADR-0036-ownership-discovery.md`, or the next free integer
  found by re-checking `.mindspec/adr/` on BOTH the current
  branch and main at bead-claim time** — never 0035 — and updates
  every `ADR-0035` cross-reference in `spec.md` in the same commit,
  per the spec's reservation procedure.

## Design Question Resolutions

The spec's adjudicated decisions are already baked in (Q-A: option-2
full-override `source_globs` over a disclosed in-code fallback; Q-B:
`dead-manifest` for existing manifests only; Q-C: recurring stateless
nudge riding the Req 22 warnings pipe). The items the spec and the
gate panel deferred to plan phase resolve as follows:

1. **`buildMigratePrompt` insertion-site audit (spec Open Question
   1, Req 14)**: Bead 1's opening step. The audit's output (exact
   insertion sites + final phase numbers for the two new phases,
   honouring the two binding ordering constraints) is recorded as a
   bd comment on Bead 1 and consumed by Bead 3, which lands the
   migrate edits.
2. **Empty-`ManifestPath` marker reachability (spec Open Question 2,
   Req 13)**: Bead 1. Panel code-grounding (V2-4) found the marker at
   `docsync.go:256-258` is expected to be dead post-Req-13
   (attribution can only return a domain with non-empty `Paths`,
   which implies a non-empty `ManifestPath`); Bead 1 performs the
   mandated audit and deletes-or-relabels with a pinning test either
   way.
3. **Zero-domains branch coverage (panel V1-1)**: NO existing test
   reaches `len(domains)==0` — both `TestCheckInternalPackages_*`
   tests write ownership fixtures. Bead 1 ADDS a new test pinning the
   zero-domains legacy branch (blocking `internal-docs` errors with
   the `<fallback: internal/<pkg>/**>` marker) as part of the Req 13
   disclosure obligation.
4. **`--fix` append-vs-leave detection (panel V2-2)**: the Req 11
   three-state fixer MUST inspect raw YAML bytes (key-presence or
   `yaml.Node`) — `config.Load`'s typed struct cannot distinguish an
   absent `source_globs:` key from `source_globs: []`. The Warn side
   (Req 18) deliberately collapses both to `len(globs)==0` via
   `config.Load`. Encoded in Bead 4.
5. **Dead-manifest walk exclusions (panel V2-6, Req 17 binding
   text)**: the workspace walk skips `.git/`, `.worktrees/`, and
   `.beads/`; Bead 4 pins the exclusion set with a test where a glob
   matching ONLY a file under an excluded tree still fires
   `dead-manifest`.
6. **Partial-dead manifests (per-entry evaluation)**: stay
   whole-set for v1, exactly as Req 17's binding text and the HC-6
   accepted-gap documentation state; per-entry evaluation remains a
   named follow-up candidate recorded in the new ADR alongside
   resolved-file-set `domain-overlap`. No bead implements it.
7. **ADR number**: `ADR-0036`-or-next-free at claim time (see ADR
   Fitness; never 0035, which spec 092 took).

## Testing Strategy

- **Unit tests are the primary gate**: every behavior change lands
  with unit tests asserting the exact spec AC for that requirement —
  state assertions for the loader/config changes (Bead 1), classifier
  set/severity assertions for the override semantics (Bead 2), string
  assertions on emitted prompts and the migrate prompt's ordering
  indexes (Bead 3), doctor check/fixer assertions including
  byte-identity of untouched files (Bead 4), and output-layer WARN
  assertions for complete/approve (Bead 5).
  `go build ./... && go test -short ./...` green on every commit
  (HC-5), with no skipped/excluded tests vs main.
- **HC-3 enumerated updates** land in the SAME bead as the behavior
  change (Bead 1): `TestOwnershipFallback`
  (`internal/validate/ownership_test.go:76`) updated to assert
  `Paths: []` / `Source() == "missing"` — this update IS the
  authoritative regression gate for the fallback removal — and
  `TestValidateDocsErrorsOnInternalDocSkew_Fallback`
  (`internal/validate/docsync_test.go:188`) updated to the
  post-Req-13 semantics. The classifier tests
  (`TestClassifyChanges`/`TestIsSourceFile`) stay UNCHANGED: under
  option-2 override semantics they pin the empty-`source_globs`
  fallback path (HC-7).
- **Config-cache hygiene**: every test (and the Bead 4 fixer) that
  mutates `.mindspec/config.yaml` on disk calls `config.ResetCache()`
  before re-reading (per Req 11; cache at
  `internal/config/config.go:76-110`).
- **Shell validation proofs** from the spec's Validation Proofs
  section are run at spec close-out (autopilot final review); they
  complement, never replace, the unit tests — the spec itself marks
  the `TestOwnershipFallback` update, not the doctor-surface greps,
  as the authoritative proof of the loader change.
- **No new test frameworks**: existing `internal/doctor` check/fixer
  test patterns (`beads.go`/`git.go` FixFunc precedents) and
  `internal/validate` fixture helpers (`writeOwnershipFixture`) are
  reused.

## Decomposition Notes

Five beads, within the ≤6 advisory ceiling. The graph is a shallow
diamond: Bead 1 (foundation) unblocks Beads 2 and 3 in parallel;
Bead 4 (doctor) consumes Bead 1's exported loader/glob helpers AND
Bead 3's stub/prompt helpers; Bead 5 (warnings pipe) is
requirement-independent of the rest (it prints whatever
warning-severity issues the doc-sync `*Result` carries, and the
existing `AddWarning`/`SevWarning` infrastructure already exists) so
it runs parallel with Bead 1. Longest chain: Bead 1 → Bead 3 →
Bead 4 (depth 3, at the threshold, not over it). File overlap is
deliberate and small: Beads 1 and 2 both touch
`internal/validate/docsync.go` + its test (Bead 1 owns the
zero-domains/marker/stale-comment obligations and the two HC-3 test
updates; Bead 2 owns the override classifier and the new Warn) — the
edited regions are disjoint, and the orchestrator runs them serially
anyway (Bead 2 depends on Bead 1's `Source()`). The Req 22(b)
nudge-issue production lands in Bead 2 (validator side); Bead 5 only
prints, which keeps it independent.

## Bead 1: Foundation — fallback removal, Source(), config field, ADR, migrate audit

**Scope**
Reqs 13, 11 (schema half only — the fixer is Bead 4's), 14 (audit
obligation only), spec ADR Touchpoints, HC-3 test updates, panel
items V1-1/V2-4. Files: `internal/validate/ownership.go` (+ test),
`internal/validate/docsync.go` (+ test; zero-domains + marker +
stale comments only), `internal/config/config.go` (+ test), new ADR
file under `.mindspec/adr/`, ADR-0031 amendment, `spec.md`
cross-reference substitution (same commit as the ADR).

**Steps**
1. Opening step (Req 14 deferral): read `buildMigratePrompt`
   (`cmd/mindspec/migrate.go:167`) in full and record the audit — the
   exact insertion sites and final phase numbers for the two new
   populate phases, honouring the two binding ordering constraints
   (Source-Globs Population AFTER Domain Identification;
   Ownership-Manifest Population AFTER the phase carrying
   `mindspec domain add`, i.e. Phase 2) — as a bd comment for Bead 3.
2. Claim the ADR number: re-check `.mindspec/adr/` on this
   branch AND main; ADR-0035 is taken by spec 092
   (agent-error-contract), so create
   `ADR-0036-ownership-discovery.md` (or the next free integer if
   0036 is also taken by then) covering the spec's ADR Touchpoints
   items (a)-(i); update every ADR-0035 cross-reference in `spec.md`
   in the same commit; add the superseded-in-part note to ADR-0031
   (fallback semantics at its lines 54/81 only). Hand-create the ADR
   file following the existing ADR format; do NOT use
   `mindspec adr create` — it has two live bugs (mindspec-8lzq:
   writes to the main checkout from worktrees; mindspec-bn3u:
   returns colliding IDs). Spec 092's Bead 1 hit exactly this.
3. Remove the silent fallback at
   `internal/validate/ownership.go:48-53` (missing manifest returns
   `Ownership{Paths: []}`); add the derived `Source()` method per the
   Req 13 three-state table (no new stored field); update the stale
   struct-doc comment (ownership.go:18-21); export (or move to a
   shared package) `loadOwnership` and `globMatch` so
   `internal/doctor` can reuse them (Req 17/20 obligation: doctor
   must NOT reimplement loading or matching).
4. Update the two HC-3 enumerated tests in place:
   `TestOwnershipFallback` (ownership_test.go:76) asserts
   `Paths: []` + `Source() == "missing"`;
   `TestValidateDocsErrorsOnInternalDocSkew_Fallback`
   (docsync_test.go:188) asserts a manifest-less domain claims
   nothing.
5. Zero-domains branch (Req 13 disclosure): update the
   `checkInternalPackages` comment to state the branch is the
   deliberate no-domains disclosed default; ADD a new test reaching
   `len(domains)==0` asserting the blocking `internal-docs` error
   with the literal `<fallback: internal/<pkg>/**>` marker — per
   panel V1-1, no existing test reaches this branch.
6. Audit the empty-`ManifestPath` marker
   (`internal/validate/docsync.go:256-258`): if
   dead post-fallback-removal (expected per panel V2-4), delete it;
   if reachable, relabel as a disclosed-fallback marker; pin the
   chosen outcome with a test. Update the stale `listDomainDirs`
   comment (docsync.go:114-118).
7. Add the optional `SourceGlobs []string` field
   (`yaml:"source_globs"`) to `internal/config/config.go` with three
   tests per the spec AC: round-trip of
   `source_globs: [cmd/**, internal/**]`, empty default when the
   field is absent, and empty default when the config FILE is absent.

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/validate/... ./internal/config/...` passes,
      including the updated `TestOwnershipFallback`, the updated
      docsync fallback test, the NEW zero-domains test, the marker
      pinning test, and the three config-field tests
- [ ] `grep -n '"internal/" + domain' internal/validate/ownership.go`
      finds nothing — that string is the real fallback construction
      (ownership.go:50), whereas `internal/<domain>` only appears in
      comments; `Source()` exists with the three-state semantics. The
      updated `TestOwnershipFallback` remains the authoritative gate
      for the removal (this grep is supplementary)
- [ ] The new ADR file exists with a non-0035 number;
      `grep -c "ADR-0035" .mindspec/specs/091-ownership-discovery/spec.md`
      is 0 after the cross-reference substitution;
      `grep -qi "superseded in part" .mindspec/adr/ADR-0031-doc-sync-gate.md`
- [ ] The bd comment on this bead records the `buildMigratePrompt` (`cmd/mindspec/migrate.go`) audit
      (insertion sites + final phase numbers for Bead 3)

**Acceptance Criteria**
- [ ] Spec AC "silent fallback removed" (loader returns empty Paths,
      `Source() == "missing"`; `TestOwnershipFallback` updated as the
      authoritative regression gate)
- [ ] Spec AC "`source_globs:` field added to the config schema"
      (three-state parse tests)
- [ ] Spec AC "ownership-discovery ADR exists" + "ADR-0031 carries
      the superseded-in-part note" (number per claim-time
      next-free-integer procedure, expected ADR-0036)
- [ ] Req 13 disclosure obligations: zero-domains comment + NEW
      pinning test; marker audited with pinned outcome; all three
      stale comments updated

**Depends on**
None

## Bead 2: Doc-sync override semantics + unclaimed-source Warn + nudge issue

**Scope**
Reqs 16 and 22(b) (validator half: the warning-severity
`missing-source-globs` issue on the doc-sync `*Result`). Files:
`internal/validate/docsync.go` + `docsync_test.go` (regions disjoint
from Bead 1's edits). Uses Bead 1's `Source()` and `SourceGlobs`.

**Steps**
1. Implement the binding override semantics: `ValidateDocs` reads
   `source_globs` via `config.Load`; when NON-EMPTY it is the ONLY
   classifier (full override, never union — a matching `.js` file IS
   source, a non-matching `internal/*.go` file is NOT); when
   empty/absent the existing `isSourceFile` path
   (`internal/validate/docsync.go:108-112`) runs byte-identically (HC-7) and
   `unclaimed-source` is disabled. Doc-file precedence in
   `classifyChanges` (isDocFile checked first) is preserved under
   override.
2. Emit the `unclaimed-source` Warn per Req 16: fires when a diff
   touches glob-matched files no domain's resolved `paths` claims,
   regardless of each domain's `Source()` state; message lists the
   unclaimed files, the mechanical per-domain `Source()` state report
   (no ranking/guessing, per ZFC), and the `doctor --fix` +
   `ownership populate` hint; the all-`"manifest"` variant switches
   to the widen-or-`domain add` hint. Advisory only — never blocks.
3. Specified double-report (Req 16): populated globs + zero domain
   DIRECTORIES fires BOTH the blocking zero-domains branch AND the
   advisory Warn on the same files; do NOT suppress either side.
4. Add the warning-severity `missing-source-globs` issue to the
   `*Result` when `source_globs` is empty/absent (Req 22(b)),
   mirroring the Req 18 doctor Warn text (names
   `.mindspec/config.yaml`, discloses the built-in default, hints
   `mindspec source populate`), stateless by construction.
5. Tests per the spec ACs: empty-globs byte-identical classification
   + identical `HasFailures()` on a mixed fixture; populated-override
   two-sided test (`pkg/**` → `pkg/foo.js` in, `internal/foo/bar.go`
   out); unclaimed-source tests (a)/(b)/(c) with state-report
   annotations (`missing`/`empty-stub`/`manifest`); all-populated
   hint variant; Warn-does-not-block (`HasFailures()` false);
   purely-docs diff fires nothing; disabled-when-empty with the
   blocking lane still live; nudge recurrence (validate twice →
   issue present both times, no marker/state file; populated globs →
   issue absent).

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/validate/...` passes, including all Req 16
      AC tests, the two-sided override-semantics tests, and the
      Req 22(b) recurrence test
- [ ] `TestClassifyChanges` / `TestIsSourceFile` pass UNCHANGED
      (HC-3/HC-7: the empty-globs fallback path is byte-identical)
- [ ] No new persisted state: the recurrence test asserts no
      marker/state file is created (HC-2)

**Acceptance Criteria**
- [ ] Spec AC "unclaimed-source Warn" tests (a)/(b)/(c) including the
      honesty-note case (c)
- [ ] Spec ACs "empty source_globs preserves today's classification
      byte-for-byte" and "populated source_globs fully overrides —
      never union"
- [ ] Spec ACs "Warn does NOT block the gate", "Warn does NOT fire
      for purely-docs diffs", "unclaimed-source disabled when
      source_globs empty — built-in fallback still drives the gate",
      and "unclaimed-source with zero unpopulated domains names the
      right remedies"
- [ ] Spec AC "migration-status line recurs statelessly" (validator
      half: issue on the `*Result` every invocation, no marker file)

**Depends on**
Bead 1

## Bead 3: Prompt emitters — populate subcommands, domain add stub, migrate phases

**Scope**
Reqs 8 (templating helper only — the doctor fixer is Bead 4's), 9,
10, 12, 14 (migrate edits per Bead 1's audit), 19. Files: new
`internal/ownership/` package (stub renderer + both prompt builders
+ tests), new `cmd/mindspec/ownership.go`, new
`cmd/mindspec/source.go`, `internal/domain/scaffold.go` (+ test),
`cmd/mindspec/migrate.go` (+ test).

**Steps**
1. Create `internal/ownership/` with the single Req 8 templating
   helper (`renderStub(generatedBy string) []byte`): identical
   `paths: []` body, one `<command>` substitution parameter per the
   Req 8 table; no copy-paste of the comment body anywhere else.
2. Implement the Req 10 ownership-populate prompt builder and
   `cmd/mindspec/ownership.go` (`mindspec ownership populate
   [<domain>]`): prompt names the manifest path, domain, docs to
   read, schema + excluded-first-segments hard error, and the
   inspect-the-repo instruction with NO pattern hints; no-arg emits
   one prompt per missing-or-empty manifest (via Bead 1's
   `Source()`); an explicit domain arg emits regardless of populated
   state (the Req 16 hint relies on this).
3. Implement the Req 12 source-populate prompt builder and
   `cmd/mindspec/source.go` (`mindspec source populate`, repo-wide,
   no domain arg): includes the literal `source_globs:` reference,
   `.mindspec/config.yaml` path, the FULLY REPLACES coverage warning,
   and no pre-filled glob values.
4. Extend `internal/domain/scaffold.go:Add()` (Req 9): write the
   stub via `renderStub("mindspec domain add <name>")` after the four
   standard files; print the populate prompt; no new flag, no
   opt-out.
5. Insert the two migrate phases into `cmd/mindspec/migrate.go` at
   the insertion sites named by Bead 1's audit (Req 14), honouring
   the two binding ordering constraints; make no other changes to
   `buildMigratePrompt`.
6. Tests per the spec ACs: `internal/ownership/populate_test.go`
   (prompt contains domain name + manifest path, contains NO literal
   `internal/<domain>/**` proposal), source-populate test (contains
   `source_globs:`, config path, "FULLY REPLACES"; no pre-filled
   `cmd/**`/`internal/**` glob values), `scaffold_test.go` (stub file
   present with `paths: []` after `Add()`, prompt printed), and
   `migrate_test.go` index assertions (source-populate reference
   AFTER the Domain Identification heading; ownership-populate
   reference AFTER the `mindspec domain add` instruction; no
   assertions on relative order of the new phases or final numbers).

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/ownership/... ./internal/domain/...` and
      the migrate prompt test pass
- [ ] Negative assertions hold: neither emitted prompt contains a
      framework-proposed path/glob value (ZFC)
- [ ] `mindspec ownership populate --help` and
      `mindspec source populate --help` exit 0; no other new
      top-level subcommand was added (Req 19)

**Acceptance Criteria**
- [ ] Spec AC "`mindspec ownership populate <domain>` prints a
      templated agent prompt" (incl. no-arg and explicit-arg
      behaviors)
- [ ] Spec AC "`mindspec source populate` prints the source-globs
      prompt"
- [ ] Spec AC "`mindspec domain add <name>` scaffolds an empty-stub
      OWNERSHIP.yaml" + prints the populate prompt
- [ ] Spec AC "`mindspec migrate` includes BOTH populate phases,
      honouring the two binding ordering constraints"

**Depends on**
Bead 1

## Bead 4: Doctor — fixers, dead-manifest, missing-source-globs, hygiene, Warn rewrite

**Scope**
Reqs 8 (fixer behavior), 11 (three-state config fixer), 15, 17, 18,
20, 21; panel items V2-2 (raw-YAML detection) and V2-6 (walk
exclusions). Files: `internal/doctor/docs.go` (+ test), a new
doctor config check/fixer file (e.g. `internal/doctor/config.go`
+ test), following the existing `Check.FixFunc` pattern from
`beads.go`/`git.go`. Consumes Bead 1's exported
loader/`globMatch`/`Source()` and Bead 3's `renderStub` + populate
prompt builders.

**Steps**
1. REPLACE the missing-OWNERSHIP Warn message (docs.go:95) with the
   full Req 21 text (no stale "falls back" claim; verbatim
   `doctor --fix` + `ownership populate` remedies) and update the
   stale rationale comment (docs.go:84-86); mark the check
   `Fixable` with a `FixFunc` that writes the Req 8 stub via
   `internal/ownership`'s `renderStub("mindspec doctor --fix")`
   (Bead 3) and prints the populate
   prompt per scaffolded domain (Req 15). Idempotent: an existing
   manifest is never touched — including under `--fix --force` (the
   documented carve-out).
2. Add the `source_globs` config check + fixer (Req 11 three
   states): file absent → create `.mindspec/config.yaml` with
   exactly the documented comment + `source_globs: []` block; file
   present without the field → APPEND the block, prior bytes
   unchanged; field present (empty or populated) → untouched. The
   absent-vs-present decision inspects RAW YAML bytes (key-presence
   or `yaml.Node`) per panel V2-2 — the typed loader in
   `internal/config/config.go` cannot distinguish the states; call
   `config.ResetCache()` after every write.
3. Add the `dead-manifest` check (Req 17): per domain with an
   EXISTING manifest whose whole `paths` set resolves to zero files;
   workspace walk uses the shared `globMatch` exported from
   `internal/validate/ownership.go` (Bead 1) and skips
   `.git/`, `.worktrees/`, `.beads/` (binding); Warn names the
   manifest path, the suspect glob (or `(empty)`), and the
   `ownership populate` hint. A domain with NO manifest never fires
   it (one state, one Warn — Req 21 owns `missing`).
4. Add the `missing-source-globs` doctor Warn (Req 18) for all three
   states (no config file / no field / empty list), naming
   `.mindspec/config.yaml`, disclosing the built-in default
   classifier, hinting `mindspec source populate`; clears once one
   non-empty entry exists.
5. Add the three hygiene Warns (Req 20): `duplicate-entry`,
   `redundant-subpath` (prefix match after stripping trailing `**`,
   names both entries), `domain-overlap` (literal-string comparison
   across domains, names every claimant); all advisory,
   hand-edit-only, run after `dead-manifest`.
6. Tests per the spec ACs: dead-manifest fires / clears with one
   matching file / does NOT fire for a missing manifest (Req 21 Warn
   fires instead); the V2-6 exclusion test (glob matching ONLY a
   file under `.worktrees/` still fires); three config-fixer state
   tests with `config.ResetCache()` and byte-identity of prior
   content on append; hand-authored manifest byte-identical under
   `--fix` and `--fix --force`; second-doctor-run semantics (missing
   clears after `--fix`, dead-manifest fires on the empty stub);
   hygiene trio + doctor-exits-zero-with-only-Warns; Req 21 message
   greps (hint present, `falls back` absent — `! grep -q`, not
   `grep -qv`).

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/doctor/...` passes, covering every new
      check, the fixer three-state matrix, the no-overwrite
      byte-identity assertions, and the walk-exclusion pin
- [ ] `cmd/mindspec/doctor.go` is UNCHANGED (existing `--fix`
      plumbing dispatches via `FixFunc`)
- [ ] No reimplementation: doctor calls `globMatch` from `internal/validate/ownership.go` and `renderStub` from `internal/ownership` (shared helpers)
- [ ] In a fixture repo: `mindspec doctor --fix` output contains the
      populate prompt; the follow-up `mindspec doctor` run Warns
      `dead-manifest` (not missing-OWNERSHIP) for scaffolded stubs

**Acceptance Criteria**
- [ ] Spec ACs "`mindspec doctor --fix` scaffolds an empty-stub
      OWNERSHIP.yaml" and "scaffolds the `source_globs` config
      block" (three states, `config.ResetCache()` in tests)
- [ ] Spec AC "existing OWNERSHIP.yaml files are never modified"
      (byte-identical, incl. `--fix --force`)
- [ ] Spec ACs "dead-manifest Warn" / "does NOT fire for a missing
      manifest" / "clears once the manifest matches at least one
      file"
- [ ] Spec AC "doctor emits missing-source-globs Warn" (three
      states, discloses the built-in default)
- [ ] Spec ACs "duplicate-entry", "redundant-subpath",
      "domain-overlap", and "hygiene Warns do NOT block any gate"
- [ ] Spec AC "doctor Warn message for missing OWNERSHIP.yaml" is
      the full Req 21 replacement

**Depends on**
Bead 1, Bead 3

## Bead 5: Warnings pipe — complete/approve print doc-sync warnings

**Scope**
Req 22(a) (+ the printing half of 22(b)). Files:
`internal/complete/complete.go` (consumption site :201-204, plus
its output rendering) and `internal/approve/impl.go` (:157-158,
plus its output rendering), with their tests.
Requirement-independent of Beads 1-4: it prints whatever
warning-severity issues the doc-sync `*Result` carries (the
`AddWarning`/`SevWarning` infrastructure already exists); the
Req 22(b) nudge issue itself is produced by Bead 2 and rides this
pipe with no further changes here.

**Steps**
1. `mindspec complete`: print every warning-severity issue from the
   doc-sync `*Result` as `WARN <name>: <message>` (one line per
   issue, to stderr), INCLUDING when `HasFailures()` is false and
   the flow proceeds (today complete.go:201-204 drops warnings via
   `HasFailures()` + error-joining only).
2. `mindspec approve impl`: same printing at the impl.go:157-158
   consumption site.
3. Tests at the output-rendering layer of each flow: a `*Result`
   with one warning-severity issue and NO errors produces output
   containing `WARN` and the issue message AND the flow proceeds;
   companion: zero warning issues → no `WARN` line. (No double-print
   risk: the only other ValidateDocs consumer is
   `cmd/mindspec/validate.go:70`, a separate command.)

**Verification**
- [ ] `go build ./... && go test -short ./...` passes
- [ ] `go test ./internal/complete/... ./internal/approve/...`
      passes, including the WARN-printed-and-flow-proceeds and
      no-spurious-WARN tests for both flows
- [ ] WARN lines go to stderr in the `WARN <name>: <message>` format
      (string assertion)

**Acceptance Criteria**
- [ ] Spec AC "`mindspec complete` and `mindspec approve impl` print
      doc-sync warnings (Requirement 22(a))" with the companion
      zero-warn case
- [ ] Spec AC "migration-status line recurs statelessly" (printing
      half: once Bead 2's issue exists, every complete/approve run
      prints the `WARN missing-source-globs:` line)

**Depends on**
None

## Provenance

Spec acceptance criterion → owning bead + verification:

| Spec AC | Bead | Verified by |
|---|---|---|
| doctor --fix scaffolds empty-stub OWNERSHIP.yaml | Bead 4 | fixer tests + populate-prompt output assertion |
| doctor --fix scaffolds the source_globs config block (3 states) | Bead 4 | three state tests, raw-YAML detection, ResetCache |
| domain add scaffolds empty stub + prints prompt | Bead 3 | scaffold_test.go stub + stdout assertions |
| ownership populate prints prompt (no pre-filled paths) | Bead 3 | populate_test.go positive/negative assertions |
| migrate includes BOTH populate phases (ordering constraints) | Bead 3 | migrate prompt index assertions (sites from Bead 1 audit) |
| source_globs field added to config schema | Bead 1 | three-state parse tests in internal/config |
| source populate prints prompt (FULLY REPLACES; no pre-fill) | Bead 3 | source-populate prompt assertions |
| doctor missing-source-globs Warn (3 states, discloses default) | Bead 4 | doctor config-check tests |
| unclaimed-source disabled when empty + fallback drives gate | Bead 2 | disabled-when-empty + blocking-lane-identical test |
| silent fallback removed | Bead 1 | updated TestOwnershipFallback (authoritative gate) |
| empty source_globs byte-identical (HC-7, empty side) | Bead 2 | mixed-fixture classification-set + HasFailures test |
| populated source_globs fully overrides (populated side) | Bead 2 | pkg/** two-sided override test |
| Req 21 Warn message full replacement | Bead 4 | message greps (hint present, no "falls back") |
| unclaimed-source Warn fires across Source() states (a)/(b)/(c) | Bead 2 | three state-report tests in docsync_test.go |
| unclaimed-source all-populated hint variant | Bead 2 | hint-content test |
| Warn does NOT block the gate | Bead 2 | HasFailures()-false test |
| Warn does NOT fire for purely-docs diffs | Bead 2 | docs-only diff test |
| dead-manifest Warn fires | Bead 4 | docs_test.go dead-manifest test (+ V2-6 exclusion pin) |
| dead-manifest does NOT fire for missing manifest | Bead 4 | ghost-domain companion test |
| dead-manifest clears with one matching file | Bead 4 | clearing test |
| duplicate-entry / redundant-subpath / domain-overlap | Bead 4 | hygiene trio tests |
| hygiene Warns do NOT block | Bead 4 | doctor-exits-zero test |
| complete/approve print doc-sync warnings (22a) | Bead 5 | output-layer WARN tests, both flows |
| migration-status line recurs statelessly (22b) | Beads 2, 5 | recurrence + no-marker test (Bead 2); printing tests (Bead 5) |
| existing manifests never modified | Bead 4 | byte-identity tests incl. --fix --force |
| ownership-discovery ADR exists + ADR-0031 note | Bead 1 | ADR file (0036-or-next-free) + cross-ref substitution + grep |
| go build && go test -short green, HC-3 modulo-updates only | all beads | per-bead build/test verification lines; HC-3 updates in Bead 1 |
