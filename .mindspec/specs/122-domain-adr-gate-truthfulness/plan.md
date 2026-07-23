---
status: Draft
spec_id: 122-domain-adr-gate-truthfulness
version: "1"
adr_citations:
    - ADR-0032
    - ADR-0041
    - ADR-0036
    - ADR-0039
    - ADR-0035
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/validate/ownership_resolve.go
      - internal/validate/spec.go
      - internal/validate/plan.go
      - internal/validate/hint_root.go
      - internal/validate/spec_test.go
      - internal/validate/plan_test.go
      - internal/validate/corpus_guard_test.go
      - internal/validate/adr0032_amendment_test.go
      - .mindspec/adr/ADR-0032-adr-semantic-gates.md
  - id: 2
    depends_on:
        - 1
    key_file_paths:
      - internal/validate/adr_domain_resolve.go
      - internal/validate/adr_domain_resolve_test.go
      - internal/validate/plan.go
      - internal/validate/divergence.go
      - internal/validate/plan_test.go
      - internal/validate/divergence_test.go
  - id: 3
    depends_on:
        - 2
    key_file_paths:
      - internal/validate/plan.go
      - internal/validate/divergence.go
      - internal/validate/docsync.go
      - internal/validate/plan_test.go
      - internal/validate/divergence_test.go
      - internal/validate/docsync_test.go
  - id: 4
    depends_on:
        - 1
        - 2
    key_file_paths:
      - internal/validate/corpus_guard_test.go
      - internal/approve/spec_test.go
      - cmd/mindspec/ceremony_guard_test.go
---
# Plan: 122-domain-adr-gate-truthfulness

Four beads implement the gate-truthfulness spec. The decomposition
follows the spec's issue clusters with **one deliberate re-cut against
the spec-approve sketch** (which sized "R5-pins + R6" as one bead): the
**ADR-0032 third-amendment section lands in Bead 1**, not the R5 bead —
because beads land one at a time in ready order and Bead 1's forward-only
Rule-2 reject is the FIRST Requirement-1/2 code to merge, and AC-13's
falsifier ("lands in a different bead than the implementing code")
pins the amendment to exactly that bead. This mirrors spec 121's Bead-1
§2 precedent (amendment with the first citing code; later beads cite the
already-landed text). Bead 4 keeps the R5 evidence map, the genuinely-new
non-R2 pins, and the AC-14 ceremony guard.

A second small re-cut: the **layout-aware hint-root helper is
introduced in Bead 1** (whose R1 error text is the FIRST message that
must print a true domains root) and consumed/sweep-guarded across the
remaining three hint sites in Bead 3 — otherwise Bead 1 would hard-code
the very literal Bead 3's guard then bans.

**Dependency graph (acyclic), waves, and the shared-file seam.**
Edges: `1→2`, `2→3`, `1→4`, `2→4`. Waves: W1 = {1}, W2 = {2},
W3 = {3, 4} (parallel). Longest serial chain: 3 (`1→2→3`), at the
heuristic ceiling — justified because each link is genuine
produced-then-consumed state, not file adjacency:

- **Bead 2 depends on Bead 1**: Bead 2's resolver doc comment cites the
  ADR-0032 third amendment's §(b) — text that exists only after Bead 1
  finalizes it (the 121 Bead-4 merge-order-pin pattern). Additionally,
  AC-5's premise ("the spec side resolves fine — this is NOT the
  spec-side failure") is only meaningful once R1's authoring gate is
  the landed behavior for the Draft-spec fixtures.
- **Bead 3 depends on Bead 2**: R3's trigger is "an uncited Accepted
  ADR whose **resolved** `Domain(s)` (per Requirement 2) cover `d`" —
  Bead 3's covering-ADR scan CONSUMES Bead 2's ADR-side resolution
  (AC-8 explicitly admits a covering ADR "by name **or resolvable
  path**"). This edge also serializes the real shared-file seam.
- **Bead 4 depends on Beads 1 and 2**: AC-14's corpus polarity guard
  asserts the FINAL pass/fail behavior of the gates, and only R1
  (new authoring error) and R2 (new passes) move pass/fail — R3/R4 are
  hint-text-only by spec pin ("the gate's PASS/FAIL boundary is
  unchanged"), so Bead 4 does NOT need Bead 3 and runs parallel to it.

**Shared-file seam resolution (the 117 false-independence lesson),
stated explicitly** — and `key_file_paths` is declared as the true
EDIT set per bead (it feeds bead metadata + `bead_scope.go`, so a
read-only/revert-probe target is NEVER declared there): the only
non-test SOURCE files edited by more than one bead are
`internal/validate/plan.go` — Beads 1 (the `:143`-area severity-gate
CALL of the new resolvability helper), 2 (the store wrap at `:157`
feeding `:485`/`:641`), and 3 (the `:548-551` hint) — and
`internal/validate/divergence.go` — Beads 2 (`:137`/`:230`) and 3
(`:219-222`). Under the `1→2→3` chain these beads are STRICTLY
SEQUENTIAL, so no two edit either file concurrently and the
3-beads-one-file hazard is resolved by ordering, not hope.
`internal/validate/ownership_resolve.go` is edited ONLY by Bead 1
(the new SIGNATURE-PRESERVING resolvability helper — see PF-2 below —
so `normalizeImpactedDomains`'s 4 existing production callers stay
untouched; Bead 2's ADR-side resolver lives in the NEW file
`adr_domain_resolve.go` and only READS the existing `newOwnershipCache`
/`matchesAny` helpers). `internal/validate/hint_root.go` is CREATED by
Bead 1 and Bead 3 only READS it (Bead 3 edits the three call sites in
`divergence.go`/`docsync.go`, not the helper). Bead 4's edit set is
TEST-ONLY (`internal/validate/corpus_guard_test.go`,
`internal/approve/spec_test.go`, `cmd/mindspec/ceremony_guard_test.go`)
— none co-edited with any Bead-3 source or test file, so the W3
parallelism is safe (`corpus_guard_test.go` is shared with Bead 1, but
1→4 is a real edge, so that access is sequential and test-file anyway).
Bead 4's AC-12 revert probes touch shipped source files
(`cmd/mindspec/adr.go`, `plan.go`, `parse.go`) ONLY in throwaway trees,
committing nothing — hence they are NOT in its `key_file_paths`.

**Plan-level choices the spec delegates (Open Questions), resolved:**

- **Forward-only plumbing: a SEPARATE signature-preserving helper +
  caller-side severity gate** (the spec's second option, PF-2's cleanest
  fix). `normalizeImpactedDomains`'s SIGNATURE IS UNCHANGED — its 4
  existing production callers (`plan.go:143`, `spec.go:225`,
  `ownership.go:490` `ResolveCandidateDomains`, `divergence.go:155`)
  are untouched, so `go build ./...` stays clean and the divergence
  consumer is unchanged BY CONSTRUCTION (AC-3/AC-4's anti-overreach).
  Instead Bead 1 adds a NEW helper in `ownership_resolve.go`,
  `bareUnresolvedImpactedDomains(exec, root, ownerRef string, entries
  []string) []string`, that reuses the SAME domain enumeration +
  `ownCache` load path and returns the Rule-2 entries (bare tokens
  naming no domain dir) reported ONLY when the ownership model is in
  use (≥1 domain whose OWNERSHIP.yaml LOADS via `ownCache`, not mere
  dir presence — the spec's resolved boundary). Only the two AUTHORING
  callers (`checkImpactedDomainsResolutionParity` in `spec.go`,
  `ValidatePlan` in `plan.go`) invoke it, ALONGSIDE their existing
  `normalizeImpactedDomains` call, and promote each returned entry to an
  error when the SPEC's status — read via `validate.SpecStatusAt(specDir)`
  (`internal/validate/frontmatter.go:33`), NOT `plan.go`'s `isApproved`
  (which reads the PLAN's status) — is an explicit case-folded `Draft`.
  Empty status (no frontmatter / no `status:` key) and every non-`Draft`
  explicit status emit NOTHING. This keeps Bead 1's non-test source edit
  set to exactly `ownership_resolve.go` (new helper), `spec.go` (call +
  retire the carve-out comment), `plan.go` (call), and `hint_root.go`
  (new).
- **ADR-side resolution home: a decorating `adr.Store`**
  (`newDomainResolvingStore(inner, exec, root, ownerRef)`) layered at
  the two gate-lane store constructions (`plan.go:157`,
  `divergence.go:137`), whose `Get`/`List` return ADRs with `Domains`
  mapped through the deterministic resolver. Rationale: ONE wrap site
  per lane feeds ALL comparison sites R2 enumerates (`intersectFold`
  `:485`, `coverageOf`/`IsDomainCovered` `:641`/`:628`, the bead-time
  probe `:230`) with zero exported-signature churn — a
  resolve-at-comparison helper would have to be threaded into four call
  shapes and could silently miss one. The cmd-side `adrReadStore`
  (`cmd/mindspec/adr.go:89-113`) is deliberately NOT wrapped: `adr
  show`/`adr list` keep rendering the author's literal `Domain(s)` line
  (AC-12's user-facing verbs stay truthful to the document).
  Memoization: the decorator caches resolved sets per ADR ID per run,
  layered over the existing `newMemoStore` — the non-observable
  performance choice the spec permits. The ADR-side resolver reuses the
  IDENTICAL cache plumbing as the spec side per lane: the plan lane
  constructs it with `exec == nil` / `ownerRef == ""` (the working-tree
  read `ValidatePlan` already uses — it builds no executor), and the
  divergence lane passes the lane's `exec` + ref-anchored `ownerRef`,
  so both comparison sides always see the same tree via one
  `newOwnershipCache`.
- **Directory-shape matching: glob-normalization with a synthetic
  child probe.** Trim any trailing `/`; an ADR-side label resolves to a
  domain when the trimmed label glob-matches the domain's explicit
  `paths:` (via the existing `matchesAny`, honoring `exclude:` — the
  Rule-3 mechanics, so a bare file path like `src/orders/api.py`
  resolves exactly as the spec side does), OR when the probe path
  `<trimmed-label>/x` matches (so the directory forms `src/orders` and
  `src/orders/` both resolve against a glob like `src/orders/**`).
  The probe evaluates the EXPLICIT manifest glob against a synthetic
  child of the DECLARED label — no path-prefix inference over unclaimed
  paths, no synthesized ownership (the ADR-0036 line the spec-100
  amendment drew). Exactly-one owner ⇒ dir-name; zero or multiple ⇒
  verbatim, NO error (the ADR-side no-new-error doctrine). The
  mechanism may be refined at implementation only within AC-5's
  observable both-slash-form contract.
- **Unowned-split naming: ONE finding code, two message bodies.**
  `adr-divergence-unowned` keeps its code identity for both the
  owned-by-undeclared-domain and the genuinely-unowned cases (message
  text differs; same error severity; same `--override-adr` /
  `--supersede-adr` escapes). Rationale: zero risk to any consumer
  keying on finding codes, and R7's no-new-surface posture; AC-11 fixes
  the observable either way.
- **Hint-root helper home: `internal/validate`** (new
  `internal/validate/hint_root.go`), a thin label helper over the
  EXISTING `workspace.DomainsDir` precedence (`workspace.go:617`) that
  returns the workspace-RELATIVE domains-root label the loaders
  actually resolve (`.mindspec/domains` flat / `.mindspec/docs/domains`
  canonical / `docs/domains` legacy). No new `internal/workspace`
  accessor is needed — `DomainsDir` already encodes the precedence —
  so the spec's CONDITIONAL `core` declaration stays unexercised:
  `internal/workspace` is untouched by every bead (declared-but-
  untouched is the polarity the spec's Impacted Domains anticipated).

**Delivery housekeeping (orchestrator close-out, not a bead)**: per the
spec's In Scope list — close GH #147, #178, #145, #197 and bead
`mindspec-6ou2` against the landed ACs; update `mindspec-6ou2`'s design
note to record the evidenced supersession; comment #181 with the
Non-Goal disposition and the reviewer-lens follow-up reference; and
FILE the deferred FX-3 follow-up bead (the 6ou2-item-1 contextpack
backtick-strip regression pin, deferred per PF-3 so it does not force
the spec-excluded `context-system` domain into this spec).

**Dogfood note (R7)**: every bead below touches only
workflow-owned paths (`internal/validate/**`, `internal/adr/**`,
`cmd/**`) plus process artifacts, under a spec whose resolved Impacted
Domains and cited ADRs cover them — so each bead's own
`mindspec complete` MUST pass the divergence gate with ZERO
`--override-adr`, which is itself a live check of the spec's thesis.

## ADR Fitness

- **ADR-0032 (Semantic ADR Coverage Gates) — AMENDED by this spec
  (R6/AC-13), the only ADR change.** A NEW (third) `## Amendment`
  section — "authoring-time resolvability + symmetric name-resolution"
  — carrying: R1's forward-only rule with BOTH carve-outs
  (explicit-`Draft`-only gating / grandfathered Approved + status-less
  corpus; manifest-less workspaces), R2's symmetric name-resolution
  rule with directory-shape completeness and the ADR-side no-new-error
  doctrine, the evidenced-supersession paragraph naming bead
  `mindspec-6ou2`'s 6/6 panel decision (2026-06-26) with its
  three-objection refutation, and the new-ADR trigger sentence. Per the
  spec's lifecycle pin and the spec-117/ADR-0043 precedent, the full
  amendment text is **PRE-DRAFTED at plan time** — it sits in this
  worktree's `.mindspec/adr/ADR-0032-adr-semantic-gates.md` now, under
  an explicit `PRE-DRAFT` marker comment — and is FINALIZED by Bead 1
  (marker removed; wording adjusted only where the concrete
  implementation forces it), so the amendment is reviewable at
  plan-approve and lands with the first Requirement-1/2 code.
  **New-ADR trigger check (required by the Touchpoints): NOT pulled.**
  This plan implements symmetry strictly as "resolve to dir-name, then
  compare" — no path-overlap/transitive intersection, no new owner
  identity — so a third amendment (not a superseding ADR) is the
  correct vehicle.
- **ADR-0036 (Ownership Discovery — zero framework cognition) —
  unchanged, applied.** Every new resolution leg (the Rule-2
  distinguished finding, the ADR-side resolver, the real-owner
  attribution in Bead 3's split) reads EXPLICIT per-domain manifests
  through the existing `ownCache`/`resolveDomains` plumbing. The
  directory child-probe evaluates declared globs against the declared
  label — it is glob evaluation, not path-PREFIX inference, so the
  ZFC line the spec-100 amendment drew is preserved. The manifest-less
  carve-out is this ADR's doctrine at the gate.
- **ADR-0041 (Gate-Before-Mutate) — unchanged, applied.** The gates
  keep validating the committed bead tip after artifact-materialization
  and before the merge/close mutation (`complete.go:910`,
  `impl.go:366`); this plan changes only what they evaluate. Moving the
  Rule-2 failure to `validate spec`/`spec approve`/`validate plan` is
  the earliest-derivable-refusal principle applied one stage earlier,
  and every new refusal carries an ADR-0035 single-lever recovery line.
- **ADR-0039 (Flat layout) — unchanged, applied.** The hint-root
  helper renders through `workspace.DomainsDir`'s flat → canonical →
  legacy precedence, so every printed path is the one that resolves in
  the operator's workspace (AC-9 asserts both layouts).
- **ADR-0035 (Agent Error Contract) — unchanged, applied.** The R1
  refusal text carries the complete two-remedy recovery in one message
  (AC-2 proves the first remedy works verbatim); the R3 hint names the
  actual `adr_citations` fix first; the R4 findings each name one true
  remedy.
- **ADR-0031 (Doc-Sync gate) — evaluated, sound, unchanged —
  deliberately NOT in `adr_citations`.** Only its `internal-docs` hint
  TEXT becomes layout-aware (R4); enforcement semantics are untouched.
  It is omitted from the frontmatter citations because its legacy
  `Domain(s)` line (`validation, doc-sync, lifecycle, ownership`) has
  zero intersection with this spec's impacted domains
  (`workflow`, `core`) and citing it would trip `adr-cite-irrelevant`
  — the exact 6ou2-class literal-comparison phenomenon this spec
  documents, and one that R2 deliberately does NOT change (those are
  prose tokens, not resolvable paths; they stay literal by the
  no-new-error doctrine). Its domain intersection is carried by the
  cited ADRs (the spec-121 ADR-0030 omission precedent).

No ADR is superseded; no divergence requiring a human stop.

## Testing Strategy

- **Unit fixtures in `internal/validate` (primary proof surface).**
  Table-driven tests over throwaway temp workspaces (the existing
  `divergence_test.go`/`plan_test.go` fixture patterns: real domain
  dirs + OWNERSHIP.yaml + spec/plan files; the divergence lane driven
  through the same executor-fixture shape
  `TestValidateDivergenceFilePathImpactedDomainResolves` uses, i.e.
  `complete`-shaped `CheckADRDivergence`/`ValidateDivergence`, never a
  mocked resolver). Fixture shapes are pinned to the FILED repros:
  AC-1/AC-2 (#178: `orders` owning `src/orders/**`, Draft spec label
  `api (orders — models)`), AC-5 (6ou2: both ADR `Domain(s)` slash
  forms), AC-7 (#147: `genevieve` full shape), AC-8 (#145: uncited
  Accepted ADR-0001), AC-9/AC-11 (#197/#178: flattened AND pre-flatten
  workspaces).
- **RED-today discipline, tagged honestly.** ACs marked *RED today* in
  the spec (AC-1, AC-5, AC-7 coverage tail, AC-8, AC-9, AC-11) must
  fail on the spec-init SHA with zero product changes and fail again on
  revert; the deliberate deviations are stated in-test exactly as the
  spec pins them: AC-1b, AC-3, AC-4, AC-6 PASS today (anti-overreach
  guards that go red only against a non-conforming implementation),
  and AC-14 passes today by definition (polarity guard).
- **Real-corpus guard tests.** AC-1b(ii) (Bead 1) and AC-14's polarity
  half (Bead 4) run `ValidateSpec`/`ValidatePlan` over THIS repo's own
  `.mindspec/specs/*` against the real `.mindspec/domains` +
  `.mindspec/adr`, locating the repo root from the test file
  (`runtime.Caller`), read-only. Each guard asserts its cohort is
  NON-EMPTY per class (Approved bare-label; frontmatter-less;
  `status:`-key-less) so the guard cannot go vacuous, and each pins the
  067 disposition the spec chose (excluded from AC-1b's green-staying
  set; allowed-additional-error in AC-14's already-red set).
- **Existing pins are CITED, not re-authored (R5).** Five of R5's
  regression pins already exist as named tests (enumerated in Bead 4);
  the genuinely-new test deliverables of this spec are the #147
  end-to-end fixture (Bead 2, AC-7) and the 6ou2 AC-5 repro (Bead 2),
  plus the guard/sweep tests (AC-1b, AC-10, AC-14) and a strengthening
  of the existing scaffold-comment pin (AC-12b). **The 6ou2-item-1
  contextpack backtick/bold-strip fixture ("FX-3") is DEFERRED out of
  this spec (PF-3):** it would land in `internal/contextpack/spec_test.go`
  (the `context-system` domain, which this spec's Impacted Domains
  explicitly EXCLUDE), so Bead 4's own zero-override `mindspec complete`
  would self-inflict an `adr-divergence-unowned` on it (the divergence
  lane has no test-file exemption). 6ou2 item 1 is ALREADY-SHIPPED
  behavior (`contextpack/spec.go:79-81` already strips); the pin is a
  nice-to-have regression net, not a fix, so it is filed as a
  workflow-external follow-up bead at spec close rather than forced into
  a domain this spec does not touch. R5's remaining evidence-map
  obligations are unaffected.
- **Sweep guards as tests.** AC-10's hint-literal guard scans
  `internal/validate` gate-message format strings for hard-coded
  domains roots and includes a fixture-of-the-guard demonstrating red
  against a deliberately reintroduced literal; the Validation Proofs
  `rg -n '\.mindspec/docs/domains' internal/validate/` sweep (only
  comments/tests may remain) runs in every bead's verification from
  Bead 3 on.
- **Integration gates (every bead).** `go build ./...`,
  `go test ./...` (no new red; known `mindspec-z4ps` flake is the only
  tolerated exception, byte-identical to the spec-init SHA),
  `go vet ./...`, `gofmt -l` clean, `golangci-lint run ./...`,
  `mindspec validate spec 122-domain-adr-gate-truthfulness`, and a
  zero-override `mindspec complete` (the dogfood note above). Review
  evidence maps every AC-1..AC-14 (incl. AC-1b, AC-12b) to exact
  `go test <package> -run <test>` commands per the spec's Validation
  Proofs.

## Bead 1: Forward-only Rule-2 authoring reject + ADR-0032 third amendment

R1 in full plus R6 (the amendment lands with the FIRST Requirement-1/2
code — see the preamble re-cut rationale). Makes a non-resolving bare
`## Impacted Domains` label a hard authoring-time error on explicitly
`Draft` specs only, with a fix hint that mechanically works, while
every `Approved`/non-`Draft`/status-less spec and every manifest-less
workspace stays byte-identical.

**Steps**
1. `internal/validate/ownership_resolve.go`: add the SIGNATURE-
   PRESERVING helper `bareUnresolvedImpactedDomains(exec, root,
   ownerRef string, entries []string) []string` (PF-2) — it computes
   the ownership-model-in-use predicate (≥1 enumerated domain dir whose
   OWNERSHIP.yaml LOADS through the existing per-run `ownCache` — not
   mere dir presence, per the spec's resolved boundary) and returns the
   Rule-2 entries (`:119-125` — bare token, no `/`, not a domain dir)
   ONLY when the model is in use. `normalizeImpactedDomains`'s
   SIGNATURE IS UNCHANGED — its 4 existing production callers
   (`plan.go:143`, `spec.go:225`, `ownership.go:490`,
   `divergence.go:155`) stay untouched, so `go build ./...` is clean
   and the divergence consumer + manifest-less carve-out are unchanged
   BY CONSTRUCTION. Document the helper's contract (in-use predicate,
   the empty-return-when-manifest-less guarantee).
2. Add `internal/validate/hint_root.go`: `domainsRootLabel(root)` — the
   workspace-relative domains-root label derived from the EXISTING
   `workspace.DomainsDir(root)` precedence (`workspace.go:617`), i.e.
   `.mindspec/domains` when flat, `.mindspec/docs/domains` canonical,
   `docs/domains` legacy. Introduced here because this bead's error
   text is the first gate message that must print a true root; Bead 3
   wires the three remaining hint sites and the sweep guard.
3. Caller-side severity gate at BOTH authoring consumers, each calling
   the new helper ALONGSIDE its existing `normalizeImpactedDomains`
   call (no signature change to the latter):
   `checkImpactedDomainsResolutionParity` (`spec.go:217`; retire the
   `:212-214` "Rule 2 still passes" carve-out comment, replacing it
   with the forward-only contract note) and `ValidatePlan`'s
   normalization site (`plan.go:143` area). Each reads the SPEC's own
   status via `validate.SpecStatusAt(specDir)` / `SpecStatus`
   (`internal/validate/frontmatter.go:21`/`:33` — the
   parse-the-contract signal; explicitly NOT `plan.go`'s `isApproved`,
   which reads the PLAN's status) and promotes each
   `bareUnresolvedImpactedDomains` entry to ONE
   `impacted-domains-resolve` ERROR iff the status is an explicit
   case-folded `Draft`. Empty status (no frontmatter / no `status:`
   key) and every other explicit status emit NOTHING — no error, no
   WARN (the strictly-cleaner-than-fallback pin).
4. Error text, one message per offending entry: the entry verbatim
   (escaped per the existing `termsafe` discipline at this site — the
   fl91/SessionStart lesson), why it failed (neither a domain-dir name
   nor a path claimed by any OWNERSHIP `paths:`), the enumerated
   available domain-dir names (sorted, escaped PER ELEMENT — the
   multi-owner-branch precedent at `ownership_resolve.go`), and both
   remedies: replace the label with one of the listed names, or declare
   a claimed path instead (claiming it in
   `<domainsRootLabel>/<name>/OWNERSHIP.yaml` if not yet claimed).
5. Finalize the ADR-0032 third `## Amendment` section (pre-drafted at
   plan time in this worktree — remove the `PRE-DRAFT` marker comment;
   adjust wording only where the implementation forces it). It must
   keep all AC-13 anchors: the forward-only in-use predicate + BOTH
   carve-outs, symmetric deterministic name-resolution with
   directory-shape completeness, the ADR-side no-new-error doctrine,
   the new-ADR trigger sentence, and the `mindspec-6ou2` 6/6 2026-06-26
   evidenced-supersession paragraph with the three-objection
   refutation. Add the AC-13 anchor test
   (`internal/validate/adr0032_amendment_test.go`) asserting the ADR
   file contains the anchor strings (`authoring-time`, `symmetric`,
   `carve-out`, `mindspec-6ou2`, `2026-06-26`, the trigger sentence)
   and no residual `PRE-DRAFT` marker.
6. Fixtures (per Testing Strategy): AC-1 (#178 repro — Draft spec,
   `api (orders — models)`, domain `orders` owning `src/orders/**`):
   `ValidateSpec` AND `ValidatePlan` each emit exactly ONE
   `impacted-domains-resolve` error naming the entry, listing `orders`,
   containing both remedies — assertions SCOPED to the
   `impacted-domains-resolve` lane. AC-2: applying the first remedy
   verbatim (entry → `orders`) yields zero lane errors from both
   verbs. AC-1b(i): `Approved`, no-frontmatter, and no-`status:`-key
   variants of the same fixture each emit NO lane finding of any
   severity. AC-1b(ii) corpus guard
   (`internal/validate/corpus_guard_test.go`, the AC-1b half): walk
   this repo's real specs; assert zero `impacted-domains-resolve` on
   every `Approved` bare-label spec AND every status-less legacy spec
   (both cohorts asserted non-empty; `067-harness-adr023-compat`
   excluded per the spec's disposition), red against a status-blind
   `!= "Approved"` or hard-fail-Approved implementation. AC-3:
   `Approved` spec + bare label forced through the `complete`-shaped
   divergence lane with a `src/orders/models.py` diff — behavior
   byte-identical to today (Rule-2 verbatim-keep, same per-file
   attribution). AC-4: manifest-less workspace, Draft spec, bare names
   — `ValidateSpec`/`ValidatePlan`/divergence domain-lane results
   identical to today's build.

**Verification**
- [ ] `go test ./internal/validate/ -run 'TestImpactedDomains|TestSpecStatusForwardOnly|TestCorpusGuard|TestADR0032Amendment'` passes (final names recorded in review evidence per AC)
- [ ] AC-1 subtests RED on the spec-init SHA (both verbs pass the #178 repro clean today); AC-2 red→green completes by following the message alone
- [ ] AC-1b/AC-3/AC-4 subtests pass today AND after (anti-overreach; deviation tags stated in-test); corpus-guard cohorts asserted non-empty
- [ ] `rg -n 'Rule 2, no error' internal/validate/spec.go` empty (carve-out comment retired); error text contains entry + available-domain list + both remedies (string-asserted)
- [ ] `rg -n 'authoring-time|symmetric|carve-out' .mindspec/adr/ADR-0032-adr-semantic-gates.md` non-empty; `rg -n 'PRE-DRAFT' .mindspec/adr/ADR-0032-adr-semantic-gates.md` empty; amendment cites `mindspec-6ou2` + `2026-06-26`
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat); `golangci-lint run ./...` clean; `mindspec validate spec 122-domain-adr-gate-truthfulness` passes; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-1 — #178 Draft repro: exactly one lane-scoped `impacted-domains-resolve` error at BOTH authoring verbs, naming entry + domains + both remedies (RED today)
- [ ] AC-1b — grandfather by explicit status: Approved / no-frontmatter / no-`status:`-key fixtures emit nothing; real-corpus guard green over both cohorts, 067 excluded
- [ ] AC-2 — the first remedy applied verbatim completes the red→green transition
- [ ] AC-3 — Approved spec at bead time byte-identical to today (forward-only keyed on authoring status, not the bead path)
- [ ] AC-4 — manifest-less workspace: no new failure anywhere
- [ ] AC-13 — ADR-0032 third amendment finalized in THIS bead with all anchors incl. the 6ou2 evidenced-supersession paragraph

**Depends on**
None (foundational; sole W1 root). (Human-readable narration only — bd
edges are wired exclusively from `work_chunks[].depends_on`.)

## Bead 2: ADR-side symmetric name-resolution + the 6ou2 and #147 end-to-end repros

R2 in full plus R5a (the one genuinely-new #147 pin). Resolves the
cited ADR's `Domain(s)` entries through the same deterministic
explicit-manifest owner resolution the spec side already gets, so both
sides of every coverage comparison meet as domain names — closing
6ou2 items 3/4 and the still-red coverage tail of #147, with NO new
ADR-side error class.

**Steps**
1. Add `internal/validate/adr_domain_resolve.go`: the deterministic
   entry resolver (Rule-1 keep domain-dir names verbatim; path-shaped
   entries owner-resolved through the shared `newOwnershipCache` +
   `matchesAny` mechanics honoring `exclude:`; directory-shape
   completeness via trailing-slash trim + the `<label>/x` synthetic
   child probe per the plan choice above; zero/ambiguous/non-path
   tuple-or-prose entries kept VERBATIM with no error) and the
   decorating store `newDomainResolvingStore(inner adr.Store, exec
   executor.Executor, root, ownerRef string) adr.Store` whose
   `Get`/`List` return ADRs with `Domains` mapped through it, memoized
   per ADR ID per run. Doc comment cites ADR-0032's third amendment
   §(b) (landed by Bead 1 — the merge-order edge) and states the
   no-new-error doctrine verbatim.
2. Wrap the two gate-lane store constructions:
   `plan.go:157` (`newMemoStore(adrStoreForSpecFn(root, specDir))` →
   additionally wrapped, `exec nil`/`ownerRef ""` — the working-tree
   read) and `divergence.go:137` (wrapped with the lane's `exec` +
   `ownerRef`, so ADR-side resolution reads the SAME ref-anchored tree
   as the spec side). This feeds every comparison site
   (`intersectFold` `:485` → `adr-cite-irrelevant`;
   `coverageOf`/`IsDomainCovered` `:641`/`:628` →
   `adr-coverage-missing`/`-proposed`; the bead-time probe `:230`)
   with zero signature churn. The cmd-side `adrReadStore` is NOT
   wrapped (`adr show`/`list` render the literal document). Note in a
   code comment that `adr-cite-irrelevant`'s message now renders the
   RESOLVED domain set (it only fires when the intersection is still
   empty).
3. AC-5 fixtures (6ou2's actual filed scenario, both slash forms):
   domain `orders` claiming `src/orders/**`; spec Impacted Domains
   `src/orders/api.py` (spec-side resolves via spec 100); plan citing
   Accepted ADR-0090 with `- **Domain(s)**: src/orders/` in one sibling
   fixture and `- **Domain(s)**: src/orders` in the other. Assert:
   today both emit `adr-cite-irrelevant` AND `adr-coverage-missing` at
   `validate plan` (RED-today probe); after, ZERO errors from both
   lanes for BOTH forms; same fixtures through the `complete`-shaped
   divergence probe with changed file `src/orders/api.py` pass with no
   `--override-adr`.
4. AC-6 both polarities: (i) cited Accepted ADR whose `Domain(s)` is
   only legacy prose (`validation, lifecycle`) vs impacted `orders` —
   the SAME error CODES fire as today (`adr-cite-irrelevant` /
   `adr-coverage-missing`), no new class; the assertion is scoped to
   error CODES + count (the `adr-cite-irrelevant` message body now
   renders the RESOLVED impacted-domain set, which for a prose-only ADR
   is byte-identical to today because prose tokens stay literal — so
   the test asserts codes/count and that no NEW code appears, not exact
   message bytes). (ii) an ADR-side path claimed by NO domain resolves
   to nothing, compares literally, produces no resolve-style error.
5. AC-7, the #147 END-TO-END fixture (the genuinely-new R5a pin):
   spec Impacted Domains `genevieve/review.py` +
   `genevieve/summarizer.py`; domain `genevieve` claiming
   `genevieve/**/*.py` + `.github/workflows/code-review.yaml`; cited
   Accepted ADR whose `Domain(s)` lists those same file-path strings;
   bead diff touching `genevieve/summarizer.py` +
   `.github/workflows/code-review.yaml`. The `complete`-shaped
   divergence lane returns ZERO errors (no unowned, no coverage
   failure, no override). RED today at the coverage step; the
   resolution half stays pinned standalone by the EXISTING
   `TestValidateDivergenceFilePathImpactedDomainResolves`
   (`internal/validate/divergence_test.go:546` — cited, not
   re-authored; red if the `divergence.go:155` normalization is
   reverted).

**Verification**
- [ ] `go test ./internal/validate/ -run 'TestADRDomainResolve|TestSixOUTwo|Test147EndToEnd'` passes (final names in review evidence); both AC-5 slash forms green through plan AND complete-shaped lanes
- [ ] AC-5 + AC-7 subtests RED on the spec-init SHA at the pinned steps (6ou2 items 3/4; the #147 coverage tail); AC-6 passes today and red only against an ADR-side error-gating implementation (deviation tag in-test)
- [ ] `TestValidateDivergenceFilePathImpactedDomainResolves` still green and cited in the AC-7 evidence as the standalone resolution-half pin
- [ ] `adr show`/`adr list` output unchanged for a path-`Domain(s)` ADR (cmd store un-wrapped — asserted)
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat); `golangci-lint run ./...` clean; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-5 — 6ou2 end-to-end, both slash forms: zero `adr-cite-irrelevant`/`adr-coverage-missing` at plan time AND a passing complete-shaped coverage probe (RED today)
- [ ] AC-6 — ADR-side no-new-error, both polarities (legacy prose compares literally; unclaimed path resolves to nothing silently)
- [ ] AC-7 — the full #147 shape through the complete-shaped lane returns zero errors (coverage tail RED today); existing resolution pin cited

**Depends on**
Bead 1 (cites the ADR-0032 §(b) amendment text Bead 1 finalizes — a
merge-order pin; also sequences the `plan.go` seam). (bd edges wired
from `work_chunks[].depends_on`.)

## Bead 3: Truthful hints — uncited-covering-ADR remedy, layout-aware roots, owned-vs-unowned split

R3 + R4. Every remaining gate message names a path, an owner, and a
remedy that are TRUE in the operator's workspace: the coverage error
names the governing uncited ADR first; the three pre-flatten hint
paths render through the shared layout-aware root; the unowned finding
splits into scope-drift (names the real owner) vs genuinely-unowned.
Pass/fail boundaries and override surfaces unchanged everywhere.

**Steps**
1. R3 in `checkADRCoverage` (`plan.go:548-551`): whenever domain `d` is
   `notCovered`, scan the SAME in-hand store (`List`, already
   memoized + domain-resolving via Bead 2) for UNCITED Accepted ADRs
   whose resolved `Domain(s)` cover `d`. When ≥1 exists, the
   `adr-coverage-missing` error names those ADR IDs (sorted,
   `termsafe`-escaped per element) with the actual remedy — add them to
   the plan's `adr_citations` frontmatter — as the FIRST hint, ahead of
   the spec-100 amend-a-cited-Accepted-ADR remedy, with
   `mindspec adr create --domain <d>` LAST as the create-new fallback.
   The trigger is the EXISTENCE of an uncited covering ADR, not the
   emptiness of the citation list. Degenerate case (no covering ADR
   anywhere): the existing `:548`/`:550` messages stand byte-identical.
2. R4(a): render the three hard-coded pre-flatten hint paths through
   Bead 1's `domainsRootLabel` helper — the `adr-divergence-unowned`
   claim-it remedy (`divergence.go:221`) and both `internal-docs`
   doc-sync templates (`docsync.go:517`, `:585`, replacing the
   `filepath.Join(".mindspec", "docs", "domains", …)` literals).
3. R4(b), the truthful unowned split at `divergence.go:219-222`: when
   `attributeDomainCached` over the resolved DECLARED candidate set
   returns `""`, re-attribute the file against the FULL enumeration
   (`resolveDomains` + `attributeDomainCached` over all domains with
   the same per-run `ownCache` — the `docsync.go` pattern, no new
   discovery mechanism). Owner found ⇒ the message names the real
   owning domain and the remedy "add `<domain>` to
   `## Impacted Domains`" (the #178-option-2 scope-drift signal);
   no owner ⇒ the genuinely-unowned message with the (now
   layout-aware) claim-it remedy. ONE finding code
   (`adr-divergence-unowned`), two message bodies (the plan choice);
   both remain ERRORS under the same `--override-adr`/`--supersede-adr`
   escapes — the PASS/FAIL boundary is unchanged.
4. AC-10 sweep guard (`internal/validate` test): scan the package's
   gate-message format strings (AddError/AddWarning call sites) for
   hard-coded domains-root literals (`.mindspec/docs/domains`,
   `.mindspec/domains`, `docs/domains` embedded in operator-facing
   format strings; comments and _test.go files exempt), and include the
   guard-of-the-guard fixture: a deliberately reintroduced literal (in
   a test-local sample source) demonstrably turns the guard red.
5. Fixtures: AC-8 both citation states (#145 repro: store holds
   Accepted ADR-0001 covering `orders`; (i) plan cites nothing; (ii)
   plan cites one OTHER non-covering Accepted ADR — both messages name
   `ADR-0001` + the `adr_citations` remedy FIRST, `adr create` only as
   fallback) + the negative half (no covering ADR ⇒ today's message).
   AC-9 in a FLATTENED workspace (`.mindspec/domains/` present,
   `.mindspec/docs/domains/` absent): forced genuinely-unowned
   divergence failure + forced `internal-docs` failure both print
   `.mindspec/domains/…` and NEITHER output contains the substring
   `.mindspec/docs/domains`; the same forced failures in a PRE-flatten
   workspace print `.mindspec/docs/domains/…`. AC-11: diff touching
   `internal/gitutil/x.go`, spec resolved domains `[workflow]`,
   `execution` manifest claiming `internal/gitutil/**` ⇒ finding names
   `execution` + the add-to-Impacted-Domains remedy; a file claimed by
   NO manifest keeps the genuinely-unowned message; both assert
   ERROR severity and the unchanged override path shape.

**Verification**
- [ ] `go test ./internal/validate/ -run 'TestCoverageHintUncited|TestHintRootLayout|TestUnownedSplit|TestHintLiteralSweep'` passes (final names in review evidence)
- [ ] AC-8 (both citation states), AC-9 (both layouts, both lanes), AC-11 (both owner polarities) subtests RED on the spec-init SHA; negative/degenerate halves byte-identical to today
- [ ] AC-10 guard red against the deliberately reintroduced literal (fixture-of-the-guard demonstrated) and green on the shipped tree
- [ ] `rg -n '\.mindspec/docs/domains' internal/validate/` leaves only comments/tests — no message format strings
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat); `golangci-lint run ./...` clean; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-8 — `adr-coverage-missing` names the uncited covering ADR + `adr_citations` remedy first, in BOTH citation states; truthful degenerate case (RED today)
- [ ] AC-9 — flattened workspace hints print `.mindspec/domains/…` with zero `.mindspec/docs/domains` substrings; pre-flatten prints the canonical path (RED today)
- [ ] AC-10 — hint-literal sweep guard exists and its red state is demonstrated
- [ ] AC-11 — owned-but-undeclared files name their real owner + add-to-Impacted-Domains remedy; genuinely-unowned message retained; both still overridable errors (RED today)

**Depends on**
Bead 2 (the covering-ADR scan consumes Bead 2's resolved ADR
`Domain(s)`; also serializes the `plan.go`/`divergence.go` seam).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 4: Regression evidence map, scaffold pin, ceremony non-inflation guard

R5 evidence obligations (minus the AC-7 fixture that travels with R2,
and minus the deferred FX-3 contextpack pin — see PF-3 in Testing
Strategy) + R7/AC-14. Closes #147/#145 on EVIDENCE — citing the pins
that already exist by name — and pins that this spec's net ceremony
went DOWN (no new flags/keys; no corpus spec crosses green→red). Edit
set is TEST-ONLY, all in workflow/core-owned packages the spec's
Impacted Domains cover, so the bead's own zero-override
`mindspec complete` passes.

**Steps**
1. AC-12 issue→test evidence map, citing the FIVE existing pins (the
   plan is honest that these ship today and are NOT re-authored):
   #147 resolution core —
   `TestValidateDivergenceFilePathImpactedDomainResolves`
   (`internal/validate/divergence_test.go:546`); #145 friction 2
   (worktree ADR visibility, user-facing verbs) —
   `TestAdrShowWorktree_FindsWorktreeLocalADR` (`cmd/mindspec/adr_test.go:204`),
   `TestAdrListWorktree_ListsWorktreeLocalADR` (`:225`),
   `TestAdrShowWorktree_StillFindsMainOnlyADR` (`:246`),
   `TestPlanSpecWorktreeADRVisible` (`internal/validate/plan_test.go:1497`),
   plus `TestADRCreate_WritesIntoInvokingWorktree`
   (`cmd/mindspec/adr_test.go:92`); #145 friction 3 (both `Domain(s)`
   header forms) — `TestParseADR_NonListDomainLine`
   (`internal/adr/parse_test.go:480`). Record the AC-12
   revert-red evidence per the spec's falsifier: temporarily reverting
   the `adrReadStore` overlay (`cmd/mindspec/adr.go:89-113`), the
   `plan.go:236` overlay store, and the `parse.go:74` Contains-match —
   each in a throwaway working tree — turns its cited pin red
   (transcripts in review evidence; nothing committed).
2. AC-12b: strengthen the EXISTING scaffold pin
   `TestScaffoldPlanEmitsADRCitations` (`internal/approve/spec_test.go:159`
   — shipped by spec 100 R4, so the key-presence half already exists;
   stated honestly) to ALSO pin the commented remedy-guidance sentence
   (`cite the Accepted ADRs whose Domain(s) cover`) emitted by
   `scaffoldPlan` (`internal/approve/spec.go:255`), beside the sibling
   scaffold-content assertions — so the documented-key remedy text
   cannot silently drop.
3. AC-14(a) surface guard: tests pinning that `mindspec complete
   --help`, `mindspec impl approve --help`, `mindspec validate --help`,
   and `mindspec config` output gain no new flag or key versus the
   pre-spec baseline (assert the rendered flag/key SETS equal the
   pinned baseline sets — additions fail, so any future lane/flag
   inflation trips the guard).
4. AC-14(b) corpus polarity guard (`internal/validate/corpus_guard_test.go`,
   the AC-14 half, beside Bead 1's AC-1b half): run
   `ValidateSpec`/`ValidatePlan` over EVERY `.mindspec/specs/*` with a
   spec.md against the real `.mindspec/domains` + `.mindspec/adr`;
   assert no spec outside the PINNED already-red baseline set is red
   (baseline computed once against the spec-init SHA and hard-pinned
   in the test — the corpus contains already-red specs, so polarity,
   not exact-superset, is the rule). Pin the 067 disposition exactly
   as the spec chose: `067-harness-adr023-compat` (already red on
   `section-empty`/`criteria-count`/`open-question`) is IN the gate
   and MAY additionally carry the new `impacted-domains-resolve` error
   (R1 correctly firing on its `Draft` + placeholder label — asserted
   present, crossing no green→red boundary).

**Verification**
- [ ] `go test ./internal/approve/ ./internal/validate/ ./cmd/mindspec/ ./internal/adr/` passes; the five cited pins green by exact name (`go test <pkg> -run <name>` transcripts in review evidence)
- [ ] AC-12 revert probes recorded: each of the three shipped-code reverts turns its cited pin red (throwaway tree, nothing committed)
- [ ] AC-12b: guidance-sentence assertion red when the scaffold comment is dropped (mutation probe recorded)
- [ ] AC-14(a) flag/key sets byte-equal to baseline; AC-14(b) corpus polarity green with the 067 disposition asserted
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat); `golangci-lint run ./...` clean; bead completes with zero `--override-adr` (edit set is test-only, all workflow/core-owned — no context-system file touched)

**Acceptance Criteria**
- [ ] AC-12 — the issue→test evidence map cites only EXISTING, passing, revert-red pins by exact name (user-facing verbs included)
- [ ] AC-12b — the scaffold `adr_citations` comment pin covers the remedy-guidance text (existing key-presence pin strengthened, stated honestly)
- [ ] AC-14 — no new flag/key on any pinned surface; no corpus spec crosses green→red; 067 dispositioned exactly per the spec

**Depends on**
Beads 1 and 2 (AC-14's polarity guard asserts the FINAL pass/fail
behavior, which only R1/R2 move; R3/R4 are hint-text-only by spec pin,
so Bead 3 is NOT required and this bead runs parallel to it in W3).
(bd edges wired from `work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly ONE owning bead (the spec's rule); the
AC-1b/AC-14 corpus guards live in one shared test file but assert
DISJOINT properties (grandfather-set stays finding-free — Bead 1;
whole-corpus green→red polarity — Bead 4), so ownership stays single.

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (#178 authoring-time reject, both verbs; RED today) | Bead 1 Steps 1, 3–4, 6 | Bead 1 verification: lane-scoped repro subtests |
| AC-1b (explicit-status grandfather + real-corpus guard) | Bead 1 Steps 3, 6 | Bead 1 verification: status-variant fixtures + corpus cohorts non-empty |
| AC-2 (hint mechanically works, red→green by message alone) | Bead 1 Steps 4, 6 | Bead 1 verification: remedy-applied re-run |
| AC-3 (forward-only; grandfathered bead time unchanged) | Bead 1 Steps 1, 6 | Bead 1 verification: complete-shaped divergence fixture (passes today, deviation tagged) |
| AC-4 (manifest-less carve-out) | Bead 1 Steps 1, 6 | Bead 1 verification: manifest-less fixture (anti-overreach) |
| AC-5 (6ou2 repro end-to-end, both slash forms; RED today) | Bead 2 Steps 1–3 | Bead 2 verification: plan + complete-shaped lanes, both forms |
| AC-6 (ADR-side no-new-error, both polarities) | Bead 2 Steps 1, 4 | Bead 2 verification: prose-literal + unclaimed-path subtests |
| AC-7 (#147 end-to-end; coverage tail RED today) | Bead 2 Step 5 | Bead 2 verification: full-shape fixture + cited existing resolution pin |
| AC-8 (#145 hint, both citation states; RED today) | Bead 3 Steps 1, 5 | Bead 3 verification: both states + degenerate half |
| AC-9 (#197 layout-aware hints, both layouts; RED today) | Bead 3 Steps 2, 5 (consuming Bead 1 Step 2's helper) | Bead 3 verification: flattened + pre-flatten fixtures, substring assertions |
| AC-10 (hint-literal sweep guard + demonstrated red) | Bead 3 Step 4 | Bead 3 verification: guard + fixture-of-the-guard |
| AC-11 (truthful unowned split; RED today) | Bead 3 Steps 3, 5 | Bead 3 verification: owner-named + genuinely-unowned subtests, override shape unchanged |
| AC-12 (#145/#147 shipped-fix evidence map, user-facing verbs) | Bead 4 Step 1 | Bead 4 verification: five named pins green + revert-red probes |
| AC-12b (scaffold `adr_citations` comment pin) | Bead 4 Step 2 | Bead 4 verification: strengthened `TestScaffoldPlanEmitsADRCitations` + mutation probe |
| AC-13 (ADR-0032 third amendment + 6ou2 supersession, same bead as first R1/R2 code) | Bead 1 Step 5 (pre-drafted at plan time; finalized here) | Bead 1 verification: anchor test + rg proofs, PRE-DRAFT marker gone |
| AC-14 (ceremony non-inflation; green→red polarity; 067 disposition) | Bead 4 Steps 4–5 | Bead 4 verification: flag/key set equality + corpus polarity guard |

R5's 6ou2-item-1 contextpack backtick pin ("FX-3") is DEFERRED out of
this spec (PF-3 — it would force the spec-excluded `context-system`
domain into a bead and self-trip its divergence gate); it is filed as a
workflow-external follow-up bead at spec close, since 6ou2 item 1 is
already-shipped behavior and the pin is a nice-to-have net, not a fix.
The spec's Validation Proofs commands are distributed per-bead (each
bead runs its package subset + the rg sweeps from Bead 3 on); the
delivery housekeeping (closing #147/#178/#145/#197/`mindspec-6ou2`, the
6ou2 design-note supersession update, the #181 comment, AND filing the
FX-3 follow-up bead) is orchestrator close-out work, not a bead.
