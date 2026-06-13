# ADR-0036: Ownership Discovery — Zero Framework Cognition for OWNERSHIP.yaml and source_globs

- **Date**: 2026-06-11
- **Status**: Accepted
- **Domain(s)**: workflow, validation, doc-sync, ownership
- **Deciders**: Max
- **Supersedes**: [ADR-0031](ADR-0031-doc-sync-gate.md) (in part — the
  silent `internal/<domain>/**` fallback semantics only; the manifest
  schema and the warning-to-error promotion ADR-0031 records remain
  authoritative)
- **Superseded-by**: n/a
- **Related**: [ADR-0031](ADR-0031-doc-sync-gate.md)

---

> **Amended by spec 095 (mindspec-vvs9) — the absent-→claims-nothing
> rule is preserved under a ref read, and an OPERATIONAL error is NOT
> "absent".** Decision (c) defines `Source() == "missing"` for an absent
> manifest, established for the on-disk `os.ReadFile` loader where
> `os.IsNotExist` cleanly separates "not there" from a real I/O error.
> Spec 095 added a ref-anchored sibling loader (`LoadOwnershipAtRef`)
> that reads the manifest from the diffed git ref (see the ADR-0031
> amend note). At the git boundary that separation is NOT free: `git
> show <ref>:<path>` returns a GENERIC error for both a path absent in a
> valid tree AND an invalid ref / git failure. Collapsing both to
> "absent → claims nothing" would let a transient git glitch silently
> un-attribute a changed file and un-gate doc-drift — the inverse of
> this ADR's no-silent-guessing stance. The boundary therefore makes the
> distinction explicit: `Executor.FileAtRefOrAbsent` probes existence
> with `git ls-tree` (empty output at a VALID ref ⇒ absent → present
> false, nil error; a failed ref ⇒ a hard error), and
> `LoadOwnershipAtRef` maps **path-absent-at-ref → claims-nothing
> Ownership (`Source() == "missing"`), NO error** — identical to
> absent-on-disk — while an **operational git/executor failure
> propagates as a HARD error, never claims-nothing.** The same
> existence-probe split backs the ref-aware domain enumeration
> (`Executor.TreeDirsAtRef`). The on-disk loader and its
> `os.IsNotExist` semantics are unchanged. **Example (placeholder):**
> reading `widget`'s OWNERSHIP at `bead/x` when the manifest was never
> committed there → claims nothing (gate proceeds); reading it at a
> non-existent ref → a hard error that blocks the gate rather than
> silently passing.

## Status

Created in spec 091 Bead 1 alongside the fallback removal, the
derived `Ownership.Source()` method, and the `source_globs:` config
field. The populate subcommands, doctor Warns, and complete/approve
warning printing land in spec 091's later beads.

## Context

Spec 086 / ADR-0031 introduced per-domain `OWNERSHIP.yaml` for
doc-sync attribution, but left two pieces of framework cognition in
the binary:

1. **A silent loader fallback** (`internal/validate/ownership.go:48-53`
   pre-091): when a domain's manifest file was absent, the loader
   synthesized `paths: [internal/<domain>/**]` — the framework
   guessing, from the domain's NAME, which source paths it owns.
2. **A hard-coded source classifier**
   (`internal/validate/docsync.go` `isSourceFile`): "source" means
   `.go` files under `cmd/` or `internal/`, excluding `_test.go` —
   with no operator override.

Both violate the **Zero Framework Cognition (ZFC)** stance (Yegge,
2024): heuristic classification is forbidden — semantic decisions
(which paths a domain owns; what counts as source in THIS repo)
belong to a coding agent or human operator who has inspected the
repo, never to framework guesswork. The domain name is a semantic
label; the source paths are an empirical question about the specific
repo (a domain named "payments" may live in `internal/ledger/`).

## Decision

### (a) ZFC stance + empty-stub default

The framework never proposes ownership paths or source globs. The
ONLY content it ever writes into an `OWNERSHIP.yaml` is the empty
stub — `paths: []` plus a populate-this comment pointing the operator
at `mindspec ownership populate <domain>` — written by
`mindspec doctor --fix` (for domain dirs lacking a manifest) and by
`mindspec domain add` (at scaffold time). Population is cognitive
work routed to the resident coding agent via emitted prompts.

### (b) Populate-prompt designs

Two ZFC-compliant prompt emitters (the framework prints a templated
prompt; the agent does the cognitive work):

- **`mindspec ownership populate [<domain>]`** (per-domain): the
  prompt tells the agent to read the domain's `overview.md` /
  `architecture.md`, inspect the actual repo layout, and propose
  `paths:` globs. It explicitly provides NO pattern hints, warns that
  the domain name need not match any directory name, restates the
  manifest schema (`paths:` + optional `exclude:`; `viz`/`agentmind`/
  `bench` first segments are a hard error), and ends with a
  verify-via-`mindspec doctor` step. With no argument it emits one
  prompt per missing-or-empty manifest; with an explicit domain it
  emits regardless of populated state.
- **`mindspec source populate`** (repo-wide): the prompt tells the
  agent to inspect the tree and propose `source_globs:` entries
  covering all hand-authored source, excluding docs, generated
  artifacts, vendored deps, and test fixtures. It states the
  FULL-OVERRIDE semantics prominently: a non-empty list completely
  replaces the built-in classifier, so a too-narrow list narrows the
  gate — coverage responsibility shifts to the declarer.

The full templated prompt texts are specified in spec 091 Reqs 10
and 12.

### (c) Fallback removal + heuristic demotion to disclosed default

- The silent loader fallback is REMOVED: a missing manifest now
  returns `Ownership{Paths: []}` and claims NOTHING. A derived
  `Source()` method (not a stored field — panel D2) distinguishes
  the three post-load states: `"missing"` (no manifest file),
  `"empty-stub"` (file with `paths: []`), `"manifest"` (populated).
  This closes the ZFC violation inherited from spec 086.
- The `cmd/**` + `internal/**` source heuristic is DEMOTED, not
  deleted: it becomes a DISCLOSED in-code default that an
  operator-declared non-empty `source_globs:` list in
  `.mindspec/config.yaml` fully overrides. While `source_globs` is
  empty or absent, classification is byte-identical to pre-091
  behavior (HC-7) and the disclosure lives in the scaffolded config
  comment, the `missing-source-globs` Warn, and the migration-status
  line. Full deletion of the in-code classifier is an explicit
  deferral, recorded under (h).
- Two attribution fallbacks survive as DISCLOSED defaults: the
  zero-domains branch in `checkInternalPackages` (the only drift
  coverage for bare checkouts; its error text carries the literal
  `<fallback: internal/<pkg>/**>` marker as the disclosure, and a
  test pins the branch), and — audited dead — the per-domain
  empty-`ManifestPath` marker, which was removed (post-removal an
  attributed domain always has a manifest-backed, non-empty
  `ManifestPath`; a test pins the deletion).

### (d) Migration path (HC-6)

The fallback removal is an accepted, documented behavioral break. On
the first `mindspec doctor` run after the spec lands, domains that
silently relied on the fallback fire the rewritten missing-OWNERSHIP
Warn naming `mindspec doctor --fix` as the remedy. Until populated,
such domains claim nothing — diffs that would have failed under the
old fallback now pass; this regression is surfaced directly at
doctor time and indirectly (two-step chain) at complete/approve.
`doctor --fix` scaffolds empty stubs and the `source_globs` config
block; `dead-manifest` then fires until the stubs are populated.
Repos without `source_globs` lose nothing at upgrade: classification
is byte-identical (HC-7). No deprecation window (panel D1): mindspec
is primarily a single-operator tool; one `doctor --fix` run on
upgrade is cheaper than a deprecation period.

**The break runs in BOTH directions.** The paragraph above covers
the doc-sync LOOSENING (manifest-less domains claim nothing, so
previously-failing diffs now pass). The same removal simultaneously
TIGHTENS the adr-divergence lane: pre-091, a spec-declared impacted
domain with no manifest auto-claimed `internal/<domain>/**` via the
loader fallback, so changed files under that tree attributed cleanly
and the lane could pass; post-removal those files attribute to no
domain and fire blocking `adr-divergence-unowned` errors —
previously-passing diffs now FAIL until the domain's manifest is
populated (remedy: `mindspec doctor --fix` + `mindspec ownership
populate <domain>`, same migration path as above). Proof from this
spec's own implementation (commit 5f39a94): the
`writeADRDivergenceFixture` helper in
`internal/complete/complete_test.go` relied on exactly that fallback
for attribution and had to be adapted to write a real
`OWNERSHIP.yaml` for its "core" domain. That fixture modification is
itself a disclosed exception to HC-3's enumerated test-update list —
the enumeration named only `TestOwnershipFallback` and
`TestValidateDocsErrorsOnInternalDocSkew_Fallback` and missed this
fixture's indirect dependence on the removed fallback; the edit is
test-fixture-only (no production code in `internal/complete/` was
touched) and is recorded here as part of the accepted break's full
disclosure.

### (e) Continuous-accuracy Warn loop

Three advisory Warns keep manifests and globs honest over time, none
of which promotes to error in spec 091:

- **`unclaimed-source`** (diff-time): files matching `source_globs`
  but claimed by no domain's manifest; computed by the doc-sync
  validator and printed at `mindspec complete` / `mindspec approve
  impl` per spec 091 Req 22 (both flows print every warning-severity
  issue as `WARN <name>: <message>`, including on success — Req 22
  also adds the recurring, stateless `missing-source-globs`
  migration-status line to those flows while `source_globs` is
  unset; deliberately no one-time/seen-marker state per HC-2).
- **`dead-manifest`** (static, doctor): an EXISTING manifest whose
  `paths` set resolves to zero workspace files (the empty stub
  trivially included). Missing manifests do NOT fire it — one state,
  one Warn, paired remedies (missing → `doctor --fix`; dead →
  `ownership populate`). The workspace walk skips `.git/`,
  `.worktrees/`, `.beads/`.
- **`missing-source-globs`** (static, doctor): config file absent,
  field absent, or list empty; discloses the active built-in default
  and names `mindspec source populate`.

### (f) Hygiene Warns

Three static, advisory, hand-edit-only doctor Warns for malformed
manifest content: **`duplicate-entry`** (same literal path twice in
one list), **`redundant-subpath`** (a `paths` entry strictly implied
by a wider sibling entry; literal prefix matching after stripping
trailing `**`), and **`domain-overlap`** (the same literal path
claimed by two or more domains; literal-string comparison only — see
(i)).

### (g) No-auto-mutation policy

The framework writes only the empty stub on creation and NEVER
overwrites an existing manifest. This includes the `--fix --force`
carve-out: even `mindspec doctor --fix --force` is read-only against
existing manifests (and existing `source_globs` config content — the
config fixer is append-only when the field is absent and leaves the
file untouched when present). Hand-authored manifests are never
rewritten, reordered, or normalized.

### (h) Deliberate deferrals

Out of v1 scope, recorded here as the named deferral list:

- canonical (alphabetical) ordering of manifest entries — manifests
  work in any order; no auto-normalization;
- trailing-slash and absolute-path style nits — not flagged;
- case-sensitivity validation — documented in the scaffold comment
  but not validator-enforced (matches the underlying filesystem
  semantics; a portability hazard the operator owns);
- FULL DELETION of the in-code source classifier — the disclosed
  default from (c) stays in the binary until a future spec removes
  it;
- resolved-file-set `domain-overlap` — see (i); per-entry
  `dead-manifest` evaluation is the sibling candidate.

### (i) Accepted gaps — wrong-but-resolving and partial-dead

Two states are caught by NO check in spec 091, accepted knowingly:

- **Wrong-but-RESOLVING globs**: a populated manifest whose glob
  matches real files, just not the domain's files. `dead-manifest`
  needs zero matches; `domain-overlap` compares literal strings
  only. The misclaim surfaces only indirectly, as `unclaimed-source`
  Warns on the files the domain SHOULD have claimed.
- **Partial-dead manifests**: `dead-manifest` evaluates the `paths`
  set as a WHOLE, so one dead glob among live ones (e.g.
  `paths: [internal/real/**, internal/deleted/**]`) fires nothing.

Verification of populate output is therefore on the operator/agent.
Extending `domain-overlap` to resolved-file-set intersection (with
per-entry `dead-manifest` evaluation alongside) is the named
follow-up candidate.

## Consequences

- (+) The framework no longer guesses: every ownership claim and
  every source-classification override is operator/agent-declared,
  and every surviving default is disclosed in error/Warn text.
- (+) The operator can finally override what counts as "source" —
  the framework no longer has the last word.
- (+) The Warn loop catches drift at rest (doctor) and at diff time
  (complete/approve), without ever blocking a gate.
- (−) Accepted breaking change (HC-6): manifest-less domains claim
  nothing until populated; previously-failing diffs can pass during
  the migration window. Surfaced by the doctor Warn chain.
- (−) Accepted gaps per (i): wrong-but-resolving globs and
  partial-dead manifests surface only indirectly.
- (−) A repo that populates `source_globs` too narrowly narrows its
  own gate — coverage responsibility shifts with the override; the
  populate prompt says so explicitly.

## Rollback

Revert the spec 091 merge. The loader fallback and marker branch
return with the reverted code; `source_globs:` keys left in
config.yaml are ignored by the older binary (unknown YAML fields are
dropped by the typed loader); empty stub manifests remain harmless —
under the reverted loader an empty `paths: []` manifest claims
nothing, same as post-091. ADR-0031's superseded-in-part note and
this ADR remain in the tree as historical record.

## Related

- [ADR-0031](ADR-0031-doc-sync-gate.md) — doc-sync gate, manifest
  schema, warning-to-error promotion; superseded in part by this ADR
  (fallback semantics only).
- Spec 091 (`.mindspec/docs/specs/091-ownership-discovery/spec.md`)
  — full requirement text, including the templated populate prompts
  (Reqs 10, 12) and the migrate-prompt phases (Req 14).
- Yegge (2024) — Zero Framework Cognition: heuristic classification
  is forbidden; route semantic decisions to agents/operators.
