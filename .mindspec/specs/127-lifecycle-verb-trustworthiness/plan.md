---
adr_citations:
    - ADR-0035
    - ADR-0041
    - ADR-0042
    - ADR-0023
approved_at: "2026-07-25T21:44:19Z"
approved_by: user
bead_ids:
    - mindspec-2vtk.1
    - mindspec-2vtk.2
    - mindspec-2vtk.3
    - mindspec-2vtk.4
    - mindspec-2vtk.5
    - mindspec-2vtk.6
    - mindspec-2vtk.7
spec_id: 127-lifecycle-verb-trustworthiness
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/guard/outcome.go
        - internal/gitutil/workdestruction.go
        - internal/gitutil/workdestruction_test.go
        - internal/gitutil/neteffect.go
        - internal/gitutil/neteffect_test.go
        - internal/lifecycle/gitquery.go
        - internal/lifecycle/gitquery_test.go
        - internal/executor/merge_golden_test.go
        - internal/executor/testdata/ac8i_ordinary_merge_golden.json
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/guard/classifier.go
        - internal/guard/classifier_test.go
        - internal/guard/constructor.go
        - internal/guard/constructor_test.go
        - internal/guard/registries.go
        - internal/guard/registries_test.go
        - internal/lint/destructive_guidance_test.go
        - internal/lint/testdata/destructive_seed_manifest.txt
        - internal/setup/claude.go
        - internal/setup/codex.go
        - .mindspec/adr/ADR-0035-agent-error-contract.md
        - cmd/mindspec/adr0035_amendment_test.go
    - depends_on:
        - 1
        - 2
      id: 3
      key_file_paths:
        - internal/approve/adopt.go
        - internal/approve/adopt_test.go
        - internal/approve/adopt_lattice.go
        - internal/approve/adopt_lattice_test.go
        - internal/gitutil/gitops.go
        - internal/gitutil/gitops_test.go
        - internal/lifecycle/gitquery.go
        - cmd/mindspec/impl.go
        - cmd/mindspec/impl_adopt_test.go
        - cmd/mindspec/help_golden_test.go
    - depends_on:
        - 2
        - 3
        - 5
      id: 4
      key_file_paths:
        - internal/lifecycle/orphans.go
        - internal/lifecycle/orphan_hints.go
        - internal/lifecycle/orphan_hints_test.go
        - internal/lifecycle/outcome_oracle_test.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/adopt.go
        - internal/doctor/orphaned_beads.go
        - internal/doctor/orphaned_beads_test.go
        - internal/guard/registries.go
    - depends_on:
        - 2
        - 3
      id: 5
      key_file_paths:
        - internal/approve/impl.go
        - internal/approve/impl_test.go
        - internal/approve/plan.go
        - internal/approve/plan_provenance_test.go
        - internal/guard/registries.go
    - depends_on:
        - 1
        - 2
        - 4
        - 5
      id: 6
      key_file_paths:
        - internal/executor/mindspec_executor.go
        - internal/executor/merge_preflight_test.go
        - internal/executor/reentry_test.go
        - internal/complete/complete.go
        - internal/approve/impl.go
        - internal/approve/spec.go
        - internal/approve/plan.go
        - cmd/mindspec/complete.go
        - cmd/mindspec/impl.go
        - cmd/mindspec/selfemit.go
        - internal/redact/redact.go
        - internal/guard/registries.go
    - depends_on:
        - 2
        - 4
        - 5
        - 6
      id: 7
      key_file_paths:
        - .claude/agents/spec-orchestrator.md
        - cmd/mindspec/release.go
        - cmd/mindspec/release_test.go
        - cmd/mindspec/named_invocation_test.go
        - internal/guard/registries.go
        - internal/guard/registries_test.go
        - internal/lint/destructive_guidance_test.go
---
# Plan: 127-lifecycle-verb-trustworthiness

**Revision 2, after the two-slot plan gate (P1/P2: 2/2
REQUEST_CHANGES).** Two mechanisms are REPLACED, not repaired: the
`guard.MergeClearance` producer-backstop design is deleted (it rested on
a false import-graph premise, left the clearance mintable from inside
the producer's own package, and carried a mint-to-verify staleness
window — P1-1/-2/-3/-4/-7; the predicate now lives in `internal/gitutil`
and every producer consults it LIVE at the merge moment), and the
stale-deletion discriminator is re-derived (the prior set-subtraction
form is provably empty and returned CLEAN on the destructive shape —
P1-5, reviser-reproduced with real git; the corrected snapshot-revert
signature was probed on six shapes at plan time). The #218 fixture
class moves to SUPERSEDED per the spec's own mapping (P1-6); the AC-3(iii)
convergence mechanism and the oracle independence check are pinned to
implementable forms (P2-plan-1/-2); and three MINOR clauses are fixed
in place (P2-plan-3/-4/-5).

Seven beads implement the #218-cluster spec. **The decomposition is LIFTED
from the spec's own Decomposition section, not re-derived** — the gate fixed
three dependency inversions into that section by construction (predicate
first; adopt before every hint that names it; bead 7 last), and this plan's
job is to turn those beads into wired `work_chunks`, VERIFY the edges, and
resolve the choices the spec explicitly delegates. Bead boundaries, bead
content, and the spec-declared edges are unchanged. Two **seam-serialization
edges are ADDED** (5→4 and 4→6, below) — scheduling edges only, changing no
bead's content.

**Dependency graph (acyclic), verification method, and the added edges.**
Spec-declared edges: `1→2`, `1→3`, `2→3`, `2→4`, `3→4`, `2→5`, `3→5`,
`1→6`, `2→6`, `5→6`, `2→7`, `4→7`, `5→7`, `6→7`. Added edges: `5→4`,
`4→6`. Verification: every edge in the union points from a lower position
to a higher position in the order **1, 2, 3, 5, 4, 6, 7**, which is
therefore a topological order and the graph is acyclic; the spec-edge set
is a subset of `work_chunks[].depends_on` (checked edge-by-edge against
the spec's "After beads …" clauses); and every chunk id 1..7 appears
exactly once with contiguous ids, mapping positionally to the `## Bead N`
sections below.

**Why the two added edges (the 117 false-independence lesson):** the spec's
edge set alone leaves {4,5} and {4,6} unordered, but they are NOT
independent — `internal/approve/impl.go` is edited by beads 4 (the
orphan-hint consumer conversion: `implOrphanRefusal`, declared `:842`,
emitter `:845-850`), 5 (the R3a branch-missing preflight and the §1
preflight-phase structure C-r4-7 assigns bead 5), and 6 (the
finalize-merge preflight consultation and the R5(d)(v) preserved-merge
precondition); `internal/complete/complete.go` is edited by beads 4 (the
`:520`/`:552` consumers) and 6 (complete's §1 preflight siting). The spec
itself says "three beads touch that seam, so the order is declared, not
discovered" — it declared `5→6`; this plan declares the remaining two.
Direction `5→4`: bead 5 owns the impl.go §1 preflight-phase restructure,
so bead 4's localized `implOrphanRefusal` conversion lands inside a
settled structure rather than the structure landing around it. Direction
`4→6`: bead 6 is the heavyweight producer bead and consumes the settled
consumer sites (its AC-8(v)/AC-9(v) fixtures drive `complete` paths whose
refusal text bead 4 finalizes). Consequence, stated honestly: the graph is
a **single serial chain 1→2→3→5→4→6→7** — no parallel wave exists. The
spec's own edges already forced 6 of 7 beads into one chain
(1→2→3→5→6→7); the apparent {4,5}/{4,6} parallelism was false
independence over co-edited files, so nothing real is lost. The
shared-file seam map is exhaustive over non-test source files:
`impl.go` (4,5,6 — serialized), `complete.go` (4,6 — serialized),
`plan.go` (5,6 — serialized; symbols `checkExistingBeadsSafety` at
`:810` and `beadCreateFailure` at `:703`), `adopt.go` (3,4 —
serialized), `cmd/mindspec/impl.go` (3,6 — serialized via 3→5→6),
`internal/lifecycle/gitquery.go` (1,3 — serialized via the 1→3 edge;
bead 1 adds the predicate wrapper, bead 3 the fetch wrapper),
`internal/guard/registries.go` (2,4,5,6,7 — all on the chain, each
conversion bead exits its own seeded entries). No file is edited by two
beads that can run concurrently.

**Rejected-mechanism compliance, checked bead-by-bead:** no bead executes
an emitted recovery line (bead 4's oracle runs only the pinned probe set
and one test-authored in-process `CompleteBead` call); no bead prints an
OID-pinned merge (bead 6's conflict recovery is the R5(d) re-entry
invocation — neither conflict emitter prints any raw `git merge` line);
no bead builds a prescriptive/pasteable-form discriminator or a
deny-by-default partition (bead 2 ships the reviewed finite floor + the
known-sites exemption list that claims only known-and-reviewed). Nothing
in this plan required resurrecting any of the four.

**Interim states, priced here rather than discovered at review:** between
bead 3 and bead 4, adopt's orphan-present refusal is inspection-first with
no destructive command (R1g's declared interim); between bead 2 and bead
7, the destructive-guidance allowlist holds its full seeded membership and
the exemption list holds 8+4 entries — the tree is green throughout
because AC-9(ii)'s exactly-one-entry assertion is **bead 7's** deliverable
(bead 2 lands membership pinning, rationale + obligation discipline, and
the bootstrap fixtures over the seeded set); the four seed-only
orchestrator-block exemption entries exit in bead 7 with named exit
records.

## Plan-level choices the spec delegates, resolved

- **Adopt surface naming (R1a): a sibling verb, `mindspec impl adopt
  <spec-id> --reason "<text>"`.** Registered beside `approve <id>` under
  the existing `impl` parent (`cmd/mindspec/impl.go:20-28`), following the
  125 `reattest` precedent: an explicit, audited, one-spec-per-invocation
  surface, deliberately NOT a flag on `impl approve` — R1's
  only-caller-is-its-own-handler enumeration is cleanest when the
  entrypoint has its own leaf, and a flag on `approve` would put a
  merge-free terminal transition behind the same leaf as the merging one.
  `--reason` is required (refuse before mutation without it — AC-2(i)).
  The attestation escape (R1c) is `--attest-unverified` on the same leaf:
  honoured in exactly the three pinned aggregate states, audit marker
  records NOT-verified + which trigger (`no-source` | `negative` |
  `error`). Every AC that names the adopt invocation resolves
  `impl adopt` at leaf identity with `--reason`/`--attest-unverified`
  flag-set membership (AC-11).
- **Conflict-resolution re-entry naming (R5d): `--resolve-merge`** on both
  owning verbs — `mindspec complete <bead> --resolve-merge` (the `:1688`
  bead→spec leg) and `mindspec impl approve <spec-id> --resolve-merge`
  (the `:1721` spec→main leg). One flag name, two leaves, both resolving
  per AC-11; the printed recovery lines name these invocations and
  nothing merge-shaped.
- **Predicate home and the boundaries (bead 1 / R4) — REVISED per
  P1-1/-2/-3/-4/-7: the clearance design is DELETED; the predicate lives
  in `internal/gitutil` and every producer consults it live at the merge
  moment.** Two premises of the prior resolution were false, corrected
  on the record: (a) `internal/guard` is NOT a leaf — it directly
  imports `internal/config`, `internal/phase`, `internal/termsafe`,
  `internal/workspace`, and `internal/workspace/containment` (verified
  `go list`); what licenses homing the outcome enum there is only that
  guard is directly imported by both sides (executor, lifecycle, and by
  gitutil itself) and the enum file adds zero NEW imports. (b) The
  `mindspec_executor.go:1457` comment's transitive rationale for the
  executor→lifecycle ban ("transitively pulls internal/phase") is
  already violated in the tree: `go list -deps ./internal/executor`
  includes `internal/phase` today, via executor→guard→phase and
  executor→gitutil→guard→phase. The operative rule is the DIRECT-import
  boundary in `executor.go`'s package doc ("must NOT import any
  enforcement package" — internal/{validate,approve,complete,state,
  phase}); the stale comment is filed as `mindspec-d8di` and is neither
  fixed nor built on here. A second boundary the prior design (and the
  gate briefs) missed outright: the **ADR-0030 `internal/lint` boundary**
  (`internal/lint/boundary_test.go`) bans `internal/gitutil` and
  `os/exec` imports from the enforcement packages — so the verb layer
  could never call a gitutil predicate directly either; it must ride an
  `internal/lifecycle` thin wrapper, the exact `lifecycle.IsAncestor`/
  `lifecycle.BranchExists` house pattern (`gitquery.go:9-16`, whose doc
  comment names this boundary). Resolution (adopting P1-7's simpler
  home): the predicate `gitutil.EvaluateWorkDestruction(workdir, branch,
  target string) (guard.DestructionOutcome, WorkDestructionEvidence,
  error)` and its evidence struct live in `internal/gitutil` — which
  already hosts the classification-shaped primitives (`NetEffectLanded`,
  `ContentSubsumedOutcome`), imports only guard/termsafe/containment (no
  cycle), and owns every decision primitive the predicate composes; the
  closed outcome enum stays in `internal/guard`;
  `internal/lifecycle/gitquery.go` gains the boundary wrapper as a
  package-level **`var EvaluateWorkDestruction =
  gitutil.EvaluateWorkDestruction`** — a var, not a func, so a
  pointer-equality pin test asserts wrapper ≡ implementation (S3-r2-6's
  no-second-rewirable-seam discipline, machine-checked; joins the
  spec-121 AC-17 anti-drift consumer set). Consumption: the verb-layer
  §1 preflights (`internal/complete`, `internal/approve`) consult the
  lifecycle wrapper through in-package seam vars (the `implIsAncestorFn`
  pattern, `impl.go:75-87`) — this is where the user-facing refusal
  fires, before the materialization subphase (AC-7(v) unchanged); the
  three executor producers consult `gitutil.EvaluateWorkDestruction`
  directly at the merge moment through an in-package seam
  `workDestructionFn` default-pinned to the same symbol (the
  `netEffectLandedFn` pattern, `mindspec_executor.go:1426`). **What this
  deletes and what it buys:** no `guard.MergeClearance` type and no
  exported constructor a producer could call to self-certify (P1-2
  dissolves — there is nothing to mint); no Executor-interface or
  `mock.go` plumbing (P1-3 dissolves — no value crosses the package
  boundary; `executor.go` and `mock.go` are untouched); and no
  mint-to-verify staleness window: the producer evaluates the LIVE
  operand tips, so target-side drift — which provably occurs inside
  FinalizeEpic (`:576` commits into the spec worktree above the
  auto-merge loop, and each `:703` merge advances the target for the
  next bead) — is inside the evaluation by construction, not argued
  away (P1-4 dissolves into bead 6's target-drift fixture). The
  producer-site consultation satisfies R4(a)'s literal "consults …
  before mutating" and strictly subsumes its backstop reading (the
  evaluation runs at the producer, in this run, on this pair); the §1
  preflight is not redundant — it owns the user-facing refusal siting
  and its own AC-7(v) tests. Priced costs, recorded: the predicate runs
  twice per merge (two read-only previews — accepted); a producer-site
  refusal can fire only on state that changed between §1 and the merge
  (branch-side self-drift is class-invariant per R4(a)'s disposition;
  target-side drift is the NEW coverage), and it fails closed exactly
  like the `:640-659` ancestry-error precedent — post-materialization,
  mutating nothing further — with the disposition stated in bead 6
  (mid-loop partial state named; re-run convergence via the `ancestor`
  no-op leg). Residual TOCTOU is the merge-moment window only — the
  same window every existing probe-then-merge site carries (e.g. the
  `:819` protected-main probe) — strictly narrower than any design
  separating evaluation from action.
- **Which primitive answers which evidence class (the spec's "plan
  chooses mechanics"):** (1) *ancestry* — `gitutil.IsAncestor` (B onto T,
  then B onto `main`); (2) *supersession* — `gitutil.NetEffectLanded` /
  `gitutil.ContentSubsumedOutcome` against T then `main` (content truth,
  branch-keyed, importable everywhere); `lifecycle.FindLandedMerge` is
  consumed for **bead attribution** where hints and the adopt lattice
  need to name the landed merge (R2's derivation, R1b's per-bead
  coverage) — it enriches, never decides, the R4 refusal, which keeps the
  predicate's decision legs on symbols the whole consumer set can share;
  (3) *stale-deletion* — **REVISED per P1-5/P1-6; the prior mechanic is
  provably empty and is replaced by the snapshot-revert signature,
  probed with real git at plan time.** The prior form —
  `PreviewDeletedPaths` minus `ChangedPathsInRange(merge-base(B,T)..B)`
  — is empty by construction: a clean preview-deletion of a T-present
  path requires a B-side deletion relative to the merge base, so the
  subtracted set always contains the D-set; and on the destructive
  recreated-branch shape the computed merge-base LIES (it is T's tip,
  so the reverting commit "authors" the deletions), which made the
  prior leg certify the destruction as CLEAN. Reviser-reproduced (not
  taken on P1's word) in isolated `/tmp/core1-planrev-scratch/` repos:
  seven probes, both destructive shapes CLEAN under the old form.
  **Corrected discriminator:** with D = `gitutil.PreviewDeletedPaths
  (workdir, target, branch)` non-empty (the read-only helper the spec's
  In Scope bullet anticipates — the non-mutating `git merge-tree
  --write-tree` preview, `neteffect.go:92-107`, plus a name-status diff
  of the preview tree against T's tip, **rename detection pinned
  `--find-renames`** so AC-8(ii)'s moves are R-paths, never D-paths; a
  conflicted preview never reaches this leg — conflicts route to
  R5(d)): compute N = the paths B adds relative to T's tip, strip N
  from B's tip tree in a temporary `GIT_INDEX_FILE` (refs, real index,
  and worktree untouched; writes only unreferenced loose tree objects,
  exactly as the house preview already does), and scan `git rev-list
  --format='%H %T' <target>` for an ancestor whose tree OID equals the
  stripped tree. A hit means B reconstructs a prior state of T plus
  novel work — its deletions are staleness artifacts →
  `DestructionStaleDeletion`, evidence carrying the D-set and the
  reconstructed ancestor; no hit → the deletions are authored → clean.
  Plan-time probe results: recreated-from-tip-carrying-an-old-tree
  (single- and multi-commit recreation) → FIRES; honest cleanup
  (AC-8(iii) shape), large rename/move (AC-8(ii)), honest stale branch
  → CLEAN; modify/delete → conflict path, outside the leg. **One
  conservative corner, stated:** a cleanup whose deletions are exactly
  the whole delta since some T-ancestor (the result tree equals that
  ancestor's tree) fires the signature and refuses, override available
  — that shape is byte-indistinguishable from un-landing the tip's own
  change, and the arc's ruling ("is this text dangerous?" is not
  decidable from the text) applies one level down: intent is not
  decidable from the git state, so the tie breaks fail-closed; bead 1
  fixtures the corner by name, and AC-8(iii)'s fixture is the generic
  cleanup shape (an older file deleted with later content landed
  since), which stays clean. **A hand-edited recreation (old tree plus
  manual tweaks) produces no exact tree match and is MISSED by this
  leg** — a stated limitation, review-caught, the same honesty posture
  as R5(a)'s finite floor; the supersession leg remains the primary
  catcher of the real #218 branches (below). **Spec-text deviation, flagged
  for and RESOLVED at the plan-approve ruling (spec amended,
  `c43e3c93`):** the spec's class-3 parenthetical originally pinned
  the authored range as `merge-base(B,T)..B`, under exactly which
  definition the class is empty (the probes above); the ruling amended
  the class-3 sentence to ground authorship in the branch's novel
  content rather than range membership — the definition this plan
  implements, preserving the class's stated intent ("deletions that are
  artifacts of B's staleness, not authored changes").
  `ChangedPathsInRange` is CUT (its only consumer was the empty
  subtraction). **The #218 shape itself classifies SUPERSEDED, not
  stale-deletion** (P1-6, matching the spec's mapping: AC-3(i)
  "superseded / landed-via-another-route (#218 shape)" vs AC-3(v)'s
  separately-defined stale-deletion): the incident's stale bead branch
  and a recreated-at-old-snapshot spec branch carry genuinely old fork
  points and content that landed via another route — the supersession
  primitives answer them; the stale-deletion class's constructible
  witness is the recreated-from-current-tip-carrying-an-old-tree shape
  (the probe above), which supersession does NOT catch (its net effect
  — mass deletion — never landed anywhere). **No numeric leg is added**
  (the spec's MAY): the AC-8(ii)/(iii) fixtures make any bare
  deleted-line floor unimplementable, and a conservative tripwire would
  add a false-refusal class for zero discriminating power over the
  three evidence classes — declined, recorded here.
- **Outcome enum mechanics (B-r4-3, pinned by the spec, named here so
  every bead builds the same thing):** `guard.DestructionOutcome` is an
  iota enum — `DestructionAncestor`, `DestructionSuperseded`,
  `DestructionStaleDeletion`, `DestructionClean`,
  `DestructionEvidenceError` — with exported count sentinel
  `DestructionOutcomeCount`; every consumer's fixture table asserts
  `len(table) == guard.DestructionOutcomeCount`; consumer switches carry
  no `default` arm over the type.
- **Adopt audit-marker keys (the 125 flat-key style; adjustable at
  implementation only within AC-1/AC-2's inspectability assertions):**
  `mindspec_adopt_reason`, `mindspec_adopt_actor` (os/user + host +
  argv0), `mindspec_adopt_at` (RFC3339 UTC), `mindspec_adopt_op`,
  `mindspec_adopt_evidence` ∈ {`verified`, `attested`} (distinct pinned
  wording in the marker per AC-1 vs AC-2(iii)),
  `mindspec_adopt_attest_trigger` ∈ {`no-source`, `negative`, `error`}
  (present iff attested — F-r5-4), `mindspec_adopt_sources` (compact
  per-source three-valued summary). Written through the existing bead
  metadata helpers (`bead.MergeMetadata` family) behind approve's
  in-package seams; epic close + finalize export through the existing
  machinery (`bead.Export` / the executor's export-artifact shape),
  with AC-1's marker-for-marker comparison fixture against a normal
  `impl approve` as the no-third-terminal-shape enforcement (R1) —
  the comparison fixture, not this key list, is the authority.
- **Refusal-class markers (F-r5-3):** the four adopt refusal classes
  carry distinct literal markers in their message text —
  `adopt-evidence-negative`, `adopt-sources-conflict`,
  `adopt-evidence-error`, `adopt-coverage-unavailable` — and each
  AC-2(ix) row asserts its own marker string.
- **Status-set resolution for the lattice's epic enumeration (F-r5-2 —
  the trap is LIVE in the helper an implementer would reach for):**
  `bead.CustomStatuses` returns nil on an unreadable or unparseable
  `<root>/.beads/config.yaml` (`internal/bead/config.go:435-450`), so
  `bead.AllStatuses` silently degrades to built-ins — the exact fail-open
  narrowing F-r5-2 forbids. Bead 3 therefore resolves the status set
  strictly in `internal/approve`: root is passed explicitly (never
  cwd-derived); a **missing** config file is the legitimate no-customs
  state; a **present but unreadable/unparseable** file is evidence-error
  for the aggregate (checked by reading + YAML-parsing the file first,
  then delegating to `bead.AllStatuses(root)` for the union so the parse
  rule keeps one home). Enumeration is the single comma-joined
  `--status=` `bd list --parent` call (the `phase.fetchChildren`
  precedent, `cache.go:272-291`, minus its cwd-degrade).
- **Registry/allowlist/exemption-list representation (R5):** typed Go
  tables in `internal/guard/registries.go` — entry = {surface key, quoted
  string, occurrence count, rationale, obligation-test name}, at R5(a)'s
  content grain (`file:line` advisory only). Seed manifests are committed
  `internal/lint/testdata/` files, sorted, keyed by (surface + quoted
  matched line + occurrence count), header recording input SHA
  `09f62bd9`, scanner revision, and generation command; the three
  bootstrap fixtures (reconciliation-vs-Background, registry-equals-
  manifest-minus-exits, content regeneration) live beside the scan in
  `internal/lint` (G-r5-4).
- **Canonical-surface seam (R5c):** `internal/setup` gains one exported
  read-only accessor, `setup.CanonicalGuidanceSurfaces() map[string]
  string`, returning the evaluated `skillFiles()`/`lifecycleSkillFiles()`
  content plus the `claudeMDManagedBlock` and `agentsMDBlockTemplate`
  literals under their builder-name surface keys — consumed by the
  `internal/lint` sweep (an in-package `export_test.go` cannot cross
  packages) and by AC-10(iv)'s mutation fixtures via a test seam over the
  same map. No guidance content changes.

**Valves (the spec's, restated so the orchestrator prices them):**
primary — the `internal/lint` convention scan out of bead 2 as its own
bead (declared, not expected to fire after the round-6 cut); secondary —
the AC-3 outcome-oracle harness out of bead 4 (it would land directly
after bead 1); tertiary — the R5(d) re-entry recovery-surface work out of
bead 6 (the AC-7(iv) chokepoint universal is NOT staged — it stays whole
in bead 6 under every valve outcome). Bead 6 must NOT be split
producer-wise. Bead 5 is light and carries no valve.

**Model tiering (standing protocol):** Sonnet implements by default,
escalate to Fable. **Beads 2, 3, and 6 should go to (or escalate early
to) Fable**: bead 2 is the classifier/constructor/scan machinery whose
three predecessors died to empirical attack — its floor precision
(non-match fixtures) and same-package AST invariant are the delicate
parts; bead 3 is the three-valued lattice with seven adversarial table
rows; bead 6 is the re-entry/resumption surface whose round-5 findings
came from real-git labs. Beads 1, 5, 7 are Sonnet-suitable; bead 4 is
Sonnet-suitable with an early-escalate note on the oracle's independence
clause (the ground-truth computation must not import the code under
test).

**Dogfood note:** every bead touches only paths the spec's Impacted
Domains claim (workflow, execution, core/redact) — each bead's own
`mindspec complete` MUST pass the divergence gate with ZERO
`--override-adr`. The recursion hazard 125 named applies doubly: bead 6
changes the very merge path that completes bead 6 — the orchestrator
should treat bead 6's own completion as the first live run of the
preflight (a refusal on bead 6's own clean merge would falsify AC-8(i)
before the panel does) — and bead 4 changes the orphan hint the
orchestrator would see if any of this spec's beads is ever closed
without `complete`.

## ADR Fitness

- **ADR-0035 (Agent Error Contract) — AMENDED by this spec; the only ADR
  change.** The guidance non-destructiveness clause per the spec's
  Touchpoints: destructive-class commands anywhere in an emitted
  lifecycle diagnostic only via the evidence-carrying constructor or a
  registered allowlist entry with rationale AND tested obligation;
  evidence-negative → safe disposition named; evidence-absent →
  inspection-first. The amendment text names its four enforcement
  mechanisms (constructor, classifier, convention scan, allowlist) and
  states both the in-diff extension obligation and the finite-floor
  limitation. Per the 122/123/125 precedent the amendment is
  **PRE-DRAFTED at plan time** — it sits in this worktree's
  `.mindspec/adr/ADR-0035-agent-error-contract.md` under an explicit
  `PRE-DRAFT (Spec 127)` marker comment — and is FINALIZED by bead 2
  (marker removed; wording adjusted only where implementation forces
  it), with its pinning test (`cmd/mindspec/adr0035_amendment_test.go`,
  the adr0041 precedent) landing in the same bead (AC-9(iv)).
- **ADR-0041 (Gate-Before-Mutate) — APPLIED, no amendment.** R3's
  branch-existence fact and R4's work-destruction facts are §1
  preflight-phase facts (for `complete`, resolved before the step-2.5/3
  materialization subphase — the live-consultation design above keeps
  exactly this siting); evidence-computation errors refuse fail-closed retryable per
  §2(ii), mirroring `mindspec_executor.go:640-659`; adopt is a
  forward-reconcile, never a rollback.
- **ADR-0042 (Render/Derivation Provenance) — unchanged, applied.**
  Every new message renders branch names, bead IDs, paths, and evidence
  summaries via `termsafe.Escape`/`idrender` (the existing discipline at
  `orphans.go:70`'s doc comment and `plan.go:818-825`); the outcome
  oracle's byte-match templates are parameterized only by validated
  identifiers/OIDs, which is the same discipline enforced from the test
  side.
- **ADR-0023 (Beads/Dolt as Single State Authority) — unchanged,
  respected.** Adopt transitions state through the existing bd/epic
  machinery (epic close + metadata + finalize export); audit markers are
  bd metadata; no filesystem state authority, no new derivation rows,
  and no consumer gains lifecycle authority from any new value type.

No ADR is superseded; no divergence requiring a human stop. Both import
boundaries are preserved without a new edge: the executor's only new
consumption is `internal/gitutil` (already a direct import), and the
ADR-0030 lint boundary is respected by routing the verb layer through
`internal/lifecycle/gitquery.go` wrappers (the house pattern).
`internal/gitutil` gains read-only surfaces only —
`EvaluateWorkDestruction` (a composition of the shipped 125 primitives,
consumed as shipped; no primitive's discrimination is extended, per Out
of Scope), `PreviewDeletedPaths`, and the workdir-taking fetch variant
(bead 3) — non-mutating throughout (unreferenced loose objects only,
same as the existing preview).

## Testing Strategy

- **Real-git temp-repo fixtures, existing house patterns**, with git
  identity set per fixture (CI has no global identity): executor beads
  reuse the `merge_binding_test.go`/`merge_conflict_test.go` shapes;
  lifecycle beads the `initLandedRepo`/`mergeBead` builders
  (`landed_test.go`); approve beads the in-package bd-runner seams.
  Remote corroboration (bead 3) uses **real bare local origins**
  (`finalize_orphan_test.go:65` pattern) — filesystem-path fetch, fully
  hermetic, no network, no stub. **Fetch mechanism pinned (P2-plan-5):**
  the existing `gitutil.FetchRemoteBranch` is cwd-bound by its own doc
  (`gitops.go:250` — "runs … from the current working directory"),
  which contradicts the explicit-root posture; bead 3 adds the
  read-only workdir-taking variant `gitutil.FetchRemoteBranchIn(workdir,
  remote, branch)` (a `cmd.Dir` plumbing wrapper beside it, same
  `noPrompt` + `rejectOptionLike` hygiene — covered by the In-Scope
  "new read-only gitutil helper" allowance), exposed to `internal/
  approve` through a `lifecycle/gitquery.go` wrapper var (the ADR-0030
  boundary bans a direct gitutil import from approve — a constraint the
  prior revision's "approve calls gitutil.FetchRemoteBranch" wording
  violated outright). Adopt fetches at the explicit root; fixtures
  never chdir; the poisoned-cache / absent-branch / unreachable-remote
  rows run against the bare path (unreachable = a bare path that does
  not exist).
- **CI-parity is a per-bead obligation**: everything runs in the bd-less
  `go test -short ./...` lane (GREEN at `09f62bd9`, R6). bd-touching
  legs ride in-package seams (`orphans.go`'s
  `branchExistsFn`/`isAncestorFn`/`findEpicBySpecIDFn`/
  `listClosedBeadsFn`; approve's bd-runner seams; doctor's
  `runMindspecCompleteFn`). No new gating test skips when bd is missing
  — a test genuinely requiring its declared environment FATALs (spec 125
  Requirement 6 convention, worked example
  `internal/executor/landed_e2e_test.go:98-100`). New seams get
  pointer-equality default pins (the `netEffectLandedFn` AC-17 pattern)
  so no gate goes hollow. `internal/harness` is untouched by every bead
  (nothing here reaches it); the Validation Proofs package list is the
  test surface.
- **The AC-3 outcome oracle (bead 4) is in-package test infrastructure
  in `internal/lifecycle`** (`outcome_oracle_test.go`): fixtures built
  through the real orphan scan; ground truth re-derived ONLY via the
  pinned probe set (`git merge-base --is-ancestor`, `git rev-list`,
  `git for-each-ref`, `git diff --name-status`, test-side raw
  `git merge-tree --write-tree`). **Independence mechanization pinned
  (P2-plan-2 — an imports check alone is structurally vacuous for
  same-package surfaces, and this arc has already cut one mechanism for
  asserting enforcement no fixture performs):** two legs, both
  red-on-violation. (1) A **file-scoped AST call/identifier scan of
  `outcome_oracle_test.go`** asserting no CallExpr or selector/identifier
  reference to a NAMED forbidden set — `EvaluateWorkDestruction` (the
  gitutil symbol AND the same-package lifecycle wrapper var), the hint-
  derivation entrypoint(s) bead 4 lands in `orphan_hints.go`,
  `FindLandedMerge`, `NetEffectLanded`, `ContentSubsumedOutcome`,
  `PreviewDeletedPaths` — plus (2) **no `internal/gitutil` import in the
  oracle file** (the imports leg is meaningful only for the cross-package
  classifiers; the call-scan leg covers the same-package wrapper and
  derivation). **Confinement rule:** every ground-truth helper the
  oracle consumes is defined IN the oracle file itself, so a sibling
  test file (which legitimately calls the derivation for the AC-3
  table) can never become an indirection outside the fence — the scan
  asserts the oracle file's helper closure is file-local. Byte-match
  templates consume whole lines, shell metacharacters fail, and the two
  injection fixtures (compound line, resolvable-but-wrong mutating
  leaf) pin fail-closed. **No test anywhere executes a string the code
  under test emitted.** **AC-3(iii) convergence mechanism pinned
  (P2-plan-1 — the real `CompleteBead`'s bd seams (`execBeadExportFn`
  `:1412`, `mergeBindingFn`/`mergeBindingReadFn` `:1441-1442`) are
  unexported in `internal/executor` with no setters, and the binding
  write is fail-closed, so a cross-package test cannot stub them and a
  bare bd-less run cannot go green):** the convergence leg drives the
  REAL in-process `MindspecExecutor.CompleteBead` with test-authored
  arguments under the **fake-bd PATH shim** (the
  `internal/executor/landed_e2e_test.go:88-104` precedent — shim
  resolved first on PATH, FATAL-not-skip, hermetic and `-short`-safe),
  so the export and binding writes succeed for real. **Explicitly
  forbidden:** satisfying the leg through an `executor.Executor`
  wrapper/mock that fabricates the merge (the
  `internal/complete/fault_injection_realgit_test.go:90`
  `killAfterExecutor` embedding shape, repurposed as a stub) — that is
  R6's named stub-hollowing ("an AC's stated mechanism is substituted
  by a stub that fabricates the value the AC then asserts") and the
  bead prompt carries the prohibition verbatim.
- **AC-8(i) golden determinism (bead 1):** the ordinary-merge golden is
  captured by a deterministic fixture builder — pinned
  `GIT_AUTHOR_NAME/EMAIL/DATE` + `GIT_COMMITTER_*` env, fixed content,
  fixture-root path templated out of the transcript, **and git-config
  isolation on every git invocation, applied identically at capture and
  replay (P2-plan-4): `GIT_CONFIG_GLOBAL=/dev/null` +
  `GIT_CONFIG_NOSYSTEM=1`** — the golden records parent tips, so any
  commit-object-altering config (`commit.gpgsign=true` in a dev's
  global config is the live example) would otherwise capture local
  state and fail everywhere else; the house per-repo defense
  (`internal/complete/fault_injection_realgit_test.go:152`,
  `internal/doctor/git_test.go:91`,
  `internal/bootstrap/mergedriver_test.go:350` all set
  `commit.gpgsign false`) is subsumed by the env-level isolation —
  recording exit
  code, normalized output, merge-commit subject, parent count/tips, and
  result-tree paths; committed under `internal/executor/testdata/` with
  a provenance header (captured at the bead-1 tree, descended from
  `09f62bd9`, before any producer change — O2-r2-7). The guard test
  replays the same fixture and asserts byte-identity against the golden
  on every later tree.
- **cmd-layer tests** are registration/flag/anti-drift only, using the
  leaf-identity `resolveCommand` pattern
  (`ceremony_guard_test.go:59-73`, `cmd.Name()` equality); bare
  `rootCmd.Find` err==nil checks are forbidden (R6). AC-11's bd form
  never goes near `resolveCommand`.
- **RED discipline, tagged honestly in-test.** RED-on-spec-init set
  (fail at `09f62bd9`, fail again on fix-revert): AC-1, AC-2 (all legs),
  AC-3 legs (i)/(ii)/(iv)/(v)/(vi), AC-4(i), AC-5, AC-6(i), AC-7 (all
  legs), AC-9(i)/(v)/(vi), AC-10(i)/(ii)/(iii)/(iv). RED-via-new-
  mechanism set (the seam or artifact is introduced by this spec, so the
  evidence is the fix-revert leg, never a spec-init-SHA claim — the 125
  F1-MINOR honesty rule): AC-2's seam-forced error legs, AC-4(ii),
  AC-8(iv), AC-9(ii)'s registry discipline, the bootstrap fixtures.
  Guards (green before and after): AC-3(iii), AC-8(i), AC-9(iii),
  AC-8(ii)/(iii) once landed. Anti-drift: AC-2(vii), AC-3's emitter
  enumeration, AC-7(iv), AC-11, seam pointer pins.
- **Integration gates (every bead):** `go build ./...`,
  `go test -short ./...` (no new red, zero skips among new tests),
  `go vet ./...`, `gofmt -l` clean, `golangci-lint run ./...`,
  `mindspec validate spec 127-lifecycle-verb-trustworthiness`, and a
  zero-`--override-adr` `mindspec complete`. Review evidence maps every
  AC leg to exact `go test <package> -run <test>` commands.

## Bead 1: The shared work-destruction predicate + the AC-8(i) golden

The root of the graph. The closed outcome type, the three evidence
classes over the 125 primitives, the authored-range stale-deletion leg,
and the committed ordinary-merge golden — captured before any producer
changes.

**Steps**
1. `internal/guard/outcome.go` (new): the `DestructionOutcome` iota enum
   (`DestructionAncestor`, `DestructionSuperseded`,
   `DestructionStaleDeletion`, `DestructionClean`,
   `DestructionEvidenceError`) with exported `DestructionOutcomeCount`
   sentinel and a `String()` method. The FILE adds zero imports (the
   guard package itself is NOT a leaf — the preamble's P1-1 correction).
2. `internal/gitutil/neteffect.go`: add the read-only helper
   `PreviewDeletedPaths(workdir, target, branch string) ([]string,
   error)` — non-mutating `git merge-tree --write-tree` preview + a
   name-status diff of the preview tree against target's tip, D-paths
   only, **rename detection pinned `--find-renames`** (plumbing diffs
   do not detect renames by default; without the pin a large move
   reads as D-paths). Propagates every git error (never classify on
   infra failure — the 125 O2-1 discipline); mutates no refs, index,
   or worktree (unreferenced loose objects only, like the existing
   preview). Fixtures in `neteffect_test.go` incl. the rename/move
   shape (R-paths, never D-paths). `ChangedPathsInRange` is NOT built
   (cut with the empty subtraction — preamble P1-5 disposition).
3. `internal/gitutil/workdestruction.go` (new):
   `EvaluateWorkDestruction(workdir, branch, target string)
   (guard.DestructionOutcome, WorkDestructionEvidence, error)` — the
   predicate home per the preamble's revised resolution. Evaluation
   order and mechanics: ancestry via `IsAncestor` (→
   `DestructionAncestor`); supersession via same-package DIRECT calls
   to `NetEffectLanded` (+ `ContentSubsumedOutcome` for the
   subsumed-vs-conflict distinction) against target then `main` (→
   `DestructionSuperseded`) — no cross-package seam exists for a
   decision leg at all (a strictly stronger form of S3-r2-6's
   no-second-rewirable-seam demand); stale-deletion via the
   **snapshot-revert signature** (preamble mechanics item (3), probed
   at plan time: D-set from `PreviewDeletedPaths`; strip the branch's
   novel paths from its tip tree in a temp `GIT_INDEX_FILE`; scan
   `rev-list --format='%H %T'` of the target for an equal ancestor
   tree) → `DestructionStaleDeletion`, evidence carrying the D-set and
   the reconstructed ancestor; else `DestructionClean`. ANY infra
   error → (`DestructionEvidenceError`, evidence naming the failed
   probe, non-nil err) — absence of evidence is never safety.
   Error-forcing for tests rides unexported in-package seam vars with
   pointer-equality default pins (no exported knob).
   `WorkDestructionEvidence` (same file — it is gitutil-shaped; in
   guard it would cycle) carries merge-base, per-class findings, and
   the landed-merge attribution slot bead 4 fills via
   `lifecycle.FindLandedMerge` (enrichment, never the decision — the
   preamble's mechanics split; lifecycle imports gitutil, so the
   attribution lives with the derivation, not in the predicate). Doc
   comment states the per-consumer outcome table pointer (R4c/F3-r2-1).
   `internal/lifecycle/gitquery.go` gains the ADR-0030 boundary
   wrapper `var EvaluateWorkDestruction = gitutil.EvaluateWorkDestruction`
   with a pointer-equality pin test in `gitquery_test.go` (wrapper ≡
   implementation; joins the AC-17 anti-drift set).
4. `internal/gitutil/workdestruction_test.go`: real-git table fixtures
   over ALL FIVE outcomes with `len(table) ==
   guard.DestructionOutcomeCount` (B-r4-3's sentinel discipline, the
   model every consumer copies), including the AC-8 boundary fixtures as
   this bead's own panel evidence: **AC-8(ii)** large rename/directory-
   move with no net content loss → `DestructionClean`; **AC-8(iii)**
   honest cleanup bead whose authored range carries its deletions (the
   generic shape: an older file deleted with later content landed
   since) → `DestructionClean` (these two make a bare deleted-line
   floor unimplementable — red against a numeric-floor impl, named
   in-test); **the #218 shape (stale branch whose content landed via
   another route) → `DestructionSuperseded`** (P1-6 — the spec's
   AC-3(i) mapping; the prior revision filed it under stale-deletion,
   contradicting the spec); **the stale-deletion witness** (P1-5):
   branch recreated from the target's tip carrying an old tree plus
   novel work → `DestructionStaleDeletion` — in BOTH the single-commit
   and multi-commit recreation variants, red-on-revert against a
   discriminator that returns clean for them (the plan-time probes,
   committed as fixtures); **the conservative corner, fixtured by
   name**: a cleanup whose result tree exactly equals a target-ancestor
   tree → `DestructionStaleDeletion` (fail-closed by ruling, override
   available — in-test comment cites the preamble); probe forced to
   fail (seam) → `DestructionEvidenceError`.
5. `internal/executor/merge_golden_test.go` + testdata: the AC-8(i)
   golden capture per the Testing Strategy (deterministic env, ordinary
   bead merge through production `CompleteBead`, transcript + merge
   shape committed with provenance header). This bead adds ONLY a test
   + testdata to executor — no production executor edit.

**Verification**
- [ ] `go test -short ./internal/gitutil/... ./internal/lifecycle/... ./internal/executor/...` passes; test names per AC recorded in review evidence
- [ ] AC-8(ii)/(iii) fixtures green AND demonstrated red against a deleted-line-floor impl (deviation target named in-test); outcome-count sentinel test in place
- [ ] Stale-deletion witness fixtures (single- and multi-commit recreation) FIRE and are red-on-revert; the #218 fixture asserts `DestructionSuperseded`; the corner fixture asserts the fail-closed ruling
- [ ] Evidence-error leg proves error propagation (no infra failure classified); `PreviewDeletedPaths` and the predicate mutate nothing (`git for-each-ref` + status before/after identical in-test)
- [ ] Wrapper pointer-pin green (`lifecycle.EvaluateWorkDestruction == gitutil.EvaluateWorkDestruction`)
- [ ] AC-8(i) golden committed with provenance header (config-isolated env per Testing Strategy); replay green
- [ ] Full gates: build/vet/gofmt/golangci-lint/`validate spec` clean; bead completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-8(ii) — large rename/move completes clean at the predicate (boundary pinned; red-on-revert)
- [ ] AC-8(iii) — honest cleanup clean, no override toll (boundary pinned; red-on-revert)
- [ ] AC-8(i) — golden captured pre-producer-change with provenance (guard, consumed by bead 6)
- [ ] Predicate outcome set closed + sentinel-counted; evidence-error never folded into a boolean (mechanism for AC-2(ix)/AC-3/AC-7 consumers)

**Depends on**
None (graph root). (Human-readable narration only — bd edges are wired
exclusively from `work_chunks[].depends_on`.)

## Bead 2: R5a/b/c/f guard machinery — classifier, constructor, scan, seeds, ADR amendment

The enforcement lane: reviewed-finite-floor classifier with global-option
normalization, the opaque evidence-carrying constructor, the repo-wide
`internal/lint` convention scan with both guidance legs, every seeded
artifact with its pinned bootstrap manifest, and the ADR-0035 amendment +
pinning test. The round-6 cut applies: NO discriminator, NO polarity
registry, NO deny-by-default partition — the floor + exemption list claim
exactly what they can support.

**Steps**
1. `internal/guard/classifier.go`: the R5(a) floor as named token-aware
   families — the full closed list the spec pins (merge-starting `git
   merge` forms; reset; restore; `clean -f`; `branch -D`; exact-unsafe
   force-push (`--force`/`-f`, lease-guarded forms EXCLUDED at floor
   level); stash drop/clear; `worktree remove` forced long+short;
   pathspec-discard checkout / `switch --discard-changes`; forced
   checkout; `update-ref -d`; `tag -d`; `push --delete` + refspec-
   deletion spelling; `reflog expire`/`gc --prune`; `rm -rf`;
   `bd delete … --force`) with option-cluster equivalence and the pinned
   **global-option normalization** set (`-C <path>`, `-c <key=val>`,
   `--git-dir=`, `--work-tree=`, `--exec-path=`, `-p`/`--paginate`,
   `--no-pager`) stripped before family matching (H-r6-8). Recorded
   floor-level exclusions each with rationale + non-match fixture:
   `git merge --abort` (the `:1641` disposition), `git worktree prune`,
   `git rm --cached` (doctor emits it — `internal/doctor/beads.go:113`),
   `git push --force-with-lease`/`--force-if-includes`.
2. `internal/guard/constructor.go`: the opaque struct type (unexported
   fields, zero value renders invalid/fail-closed) + constructor whose
   destructive variants require a non-defaultable
   `guard.DestructionOutcome` (bead 1's type — the declared 1→2 edge);
   consumed by the failure/message renderers. Opacity seals cross-package
   construction; same-package construction is REVIEW-CAUGHT, with the
   scan's AST invariant (step 4) as defence-in-depth, not a seal (G-r5-3,
   narrowed by bead-2 rework round 4's RULING 1 — matches spec.md's
   amended R5(b) wording verbatim; this line was the one propagation
   miss the amendment did not reach).
3. `internal/guard/registries.go`: the destructive-guidance allowlist
   (seeded — every Background-inventory *emitter* site as-is; the three
   merge producers never seed), the opaque-operand registry (seeded from
   the scan's pinned first run, e.g. `bead_ready.go:57`'s spread), and
   the known-sites exemption list (**eight content-pinned live entries +
   four seed-only orchestrator-block entries**, per R5(a)'s scan-derived
   enumeration; entries pin surface key + quoted string + count, lines
   advisory). Every entry: rationale + named obligation test; additions
   red; entry disciplines per R5(a)'s three machine assertions.
4. `internal/lint/destructive_guidance_test.go`: the repo-wide AST scan
   (cached-universe `ratchet_universe_test.go` + repo-root
   `boundary_test.go` precedents — NOT a Req-21 extension): flags
   classifier-matching literals/foldable operands in `NewFailure`/
   `FormatFailure` argument positions, `RecoveryCommand`-shaped method
   returns, and rendered diagnostic bodies, over methods AND unexported
   functions; provable shapes per R5(c) (literals, literal-template
   Sprintf, package-level const folds, function-local single-assignment
   folds — the last built here); outside those shapes → red unless
   constructor-provenance (dataflow to the approved constructor call —
   never the static type) or opaque-operand-registered. Same-package
   invariant: populated composite literals / unexported-field writes of
   the opaque type only inside the approved constructor; helper forges,
   wrapper-forges, callee-name spoofs, visible `unsafe`/`reflect.NewAt`
   → red. Guidance legs: the three globs (`.claude/agents/**`,
   `.claude/skills/**`, `plugins/*/skills/**`) with new-file anti-drift,
   PLUS `setup.CanonicalGuidanceSurfaces()` (step 5). `historical_skills`
   excluded with the recorded refresh-snapshot rationale (verify the
   rationale against `internal/setup/skills.go:18-19` — if false, it
   joins the sweep instead). Teeth-in-a-consumer: AC-9(i)'s injection
   table in `internal/approve` (one probe per floor family, cluster
   re-spellings, both `git -C` normalization probes, all four non-match
   fixtures), asserting the scan names the offending file:line; plus
   the AC-9(iii) guard that the Req-19 `bd update --metadata` panic
   behaviour is unchanged.
5. `internal/setup/claude.go`/`codex.go`: the exported read-only
   `CanonicalGuidanceSurfaces()` accessor (skill map + both managed-block
   literals under builder-name surface keys) + the mutation-fixture test
   seam (AC-10(iv) plants a floor match in a lifecycle-skill literal and
   a managed-block literal and asserts the sweep REDs each). No guidance
   content change.
6. Bootstrap manifests + fixtures (G-r5-4, H-r6-5): committed sorted
   seed manifests (input SHA `09f62bd9`, scanner revision, generation
   command in header) for all three seeded artifacts; fixtures (α)
   manifest reconciled against the Background-derived expected
   enumeration plus its one named, count-pinned widening — NOT exact
   equality (bead-2 rework round 3/4 narrowing, RULING 4 then RULING 3:
   this reconciliation catches accidental drift against the named
   expectation set, not a deliberate coordinated insertion, which is
   review-caught rather than machine-prevented) — at quoted-string
   grain for the eight Background-cited live exemption entries plus the
   two named-widening entries (ten live total); command-family grain
   for the four seed-only block entries per J-r7-1's honest scope), (β)
   registry ≡ manifest − named bead-exit records, (γ) classifier
   re-matches every entry's quoted line.
7. ADR-0035 amendment: finalize the PRE-DRAFT (marker removed) +
   `cmd/mindspec/adr0035_amendment_test.go` pinning constructor,
   classifier, scan, allowlist AND the extension obligation + finite-
   floor limitation sentences (AC-9(iv)).

**Verification**
- [ ] `go test -short ./internal/guard/... ./internal/lint/... ./internal/setup/... ./cmd/mindspec/...` passes
- [ ] AC-9(i): every floor family REDs its probe; `git -C <path> reset --hard` and `git -C <path> worktree remove -f` red; all four recorded exclusions produce NO match; option-cluster probes red
- [ ] AC-10(iii)/(iv) at seeded state: zero unlisted floor matches across globs + canonical surfaces; unlisted-match fixtures red in all three rendering forms; warning-deletion/reword/duplication/hollow-entry fixtures red; mutation fixtures on skill-map + managed-block literals red
- [ ] Bootstrap fixtures (α)/(β)/(γ) green; seed manifests carry provenance headers; AC-9(iv) amendment pinning test green; AC-9(iii) guard green
- [ ] Constructor bypass set red (raw string, conversion, zero value, same-package forge, wrapper, spoof, visible unsafe); full gates clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-9(i) — red-on-introduction per family in a consumer package, normalization + precision fixtures (RED today)
- [ ] AC-9(iii) — Req-19 panic unchanged (guard)
- [ ] AC-9(iv) — amendment pinning test incl. obligation + limitation sentences (RED today)
- [ ] AC-9(vi) — constructor opacity + same-package bypasses red (RED today)
- [ ] AC-10(iii)/(iv) — sweep green at seeded membership; exemption-list machine assertions + canonical-surface mutation fixtures (RED today until the scan exists)
- [ ] AC-9(ii)'s registry disciplines (membership pinning, rationale+obligation, additions-red, bootstrap fixtures) — final burn-down count is bead 7's

**Depends on**
Bead 1 (the constructor's evidence parameter is `guard.DestructionOutcome`).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 3: R1 adopt surface + the R1b aggregation lattice

`mindspec impl adopt` — audited, evidence-gated, merge-free — with the
three-valued lattice, fetch-route corroboration, epic coverage, the
attestation escape with trigger recording, and the full AC-2(ix) table.
Interim orphan-present refusal is inspection-first (no destructive
command); the R2-derived form arrives with bead 4 (R1g, declared).

**Steps**
1. `internal/approve/adopt.go` (new): the adopt entrypoint —
   preconditions (review-state spec, `--reason` required), the R1
   branch-present legs (current → names the normal path; stale per
   bead 1's predicate → inspection + stale-branch deletion + adopt
   rerun, constructor-derived from birth, never seeded), the R1g
   orphan-present interim refusal (inspection-first), and the terminal
   transition: epic close + finalize export + audit marker through the
   existing helpers (no spec→main merge; main-side delta = the export
   artifact only). Evidence-error → fail-closed retryable, wording
   distinct, retry named first, attestation stated (AC-2(vi)).
2. `internal/approve/adopt_lattice.go` (new): the R1b aggregation —
   per-source three-valued results over (i) the remote spec-branch ref,
   **fetch route ONLY** (the new workdir-taking
   `gitutil.FetchRemoteBranchIn(root, remote, branch)` via its
   `lifecycle/gitquery.go` wrapper — the Testing Strategy's P2-plan-5
   pin; the existing `FetchRemoteBranch` is cwd-bound and approve may
   not import gitutil (ADR-0030) — then every probe at the freshly
   fetched tip; the pre-fetch tracking ref is never evidence; absent
   remote branch / unreachable remote → error), and
   (ii) the epic's bead branches at strict AllStatuses breadth (the
   plan-preamble status resolution; resolution failure →
   evidence-error, never narrowing — F-r5-2), each surviving branch
   evaluated per-branch via the shared predicate +
   `lifecycle.FindLandedMerge` over the corroborated ref for coverage
   attribution. Lattice pinned: any negative → negative; no negative +
   any error → evidence-error; positive requires epic coverage (every
   bead positively evidenced; one-of-N → unavailable/attestation);
   conflict (corroborated-positive spec ref vs negative bead branch) →
   sources-conflict. Four distinct refusal-class markers per the
   preamble (F-r5-3).
3. Attestation escape: `--attest-unverified`, honoured in exactly
   {no-source, negative, error}; refused over all-positive-with-coverage
   (nothing to attest past); marker records NOT-verified + the trigger
   aggregate (F-r5-4).
4. `cmd/mindspec/impl.go`: register `adopt <spec-id>` beside
   `approve <id>` with `--reason`/`--attest-unverified`;
   `help_golden_test.go` updated; R1(e)'s call-site enumeration test
   (the adopt entrypoint's only *direct* call site — a `CallExpr`
   resolving by import-path identity to the imported selector — is its
   registered handler; reachability through a function value, method
   value, or wrapper-satisfied interface is not mechanically detected
   and is review-caught, per R1(e)'s bead-3-fix-round-2 amendment);
   `impl_adopt_test.go` leaf-identity registration/flag checks.
5. Tests: **AC-1** happy path (bare-local-origin remote at the landed
   tip, corroboration asserted; epic-coverage asserted; audit marker
   VERIFIED wording; no merge commit; durable post-state ≡ a normal
   `impl approve` fixture marker-for-marker — R1). **AC-2** legs
   (i)-(vii) + the full **(ix) lattice table** — all seven rows, each
   asserting its own class marker, incl. the poisoned-cache row
   (evaluated at the fetched tip, never the stale cache), the
   AllStatuses row (custom-status bead poisons), and the
   resolution-failure row (evidence-error, never narrowed); deleting the
   poison, coverage, or corroboration rule REDs its row (red-on-revert
   built in). AC-2(iii)'s two attestation fixtures (no-source trigger +
   attest-past-negative trigger, marker records which).

**Verification**
- [ ] `go test -short ./internal/approve/... ./cmd/mindspec/...` passes; AC-1/AC-2 legs mapped to test names in review evidence
- [ ] Lattice table: 7 rows, each with its own refusal-class marker assertion; rule-deletion revert checks red; corroboration always fetch-then-evaluate (probe asserted at the fetched OID)
- [ ] Status-set strict resolution: corrupt `.beads/config.yaml` fixture → evidence-error (red against a `bead.AllStatuses`-only impl, named in-test)
- [ ] Call-site enumeration green; help golden updated; no mutation on any refusal leg (`git for-each-ref`/epic status/metadata before-after identical)
- [ ] Full gates clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-1 — adopt happy path, verified evidence, no merge, post-state parity (RED today)
- [ ] AC-2 (i)-(vii), (ix) — refusal legs + the lattice table (RED today; leg (viii) is bead 4's)
- [ ] Interim R1g refusal inspection-first with no destructive command (upgraded by bead 4, declared)

**Depends on**
Beads 1 (consumes the predicate) and 2 (its stale-branch refusal lines
are constructor-derived from birth — never seeded). (bd edges wired from
`work_chunks[].depends_on`.)

## Bead 4: R2 hints + the outcome oracle + doctor FixFunc gating

One evidence-carrying derivation behind all four orphan-hint consumers,
the doctor FixFunc gated by the same outcome value, the AC-3 oracle
harness, and AC-2(viii)'s composite incident fixture. Exits its seeded
allowlist entries (the four consumer sites) with named records.

**Steps**
1. `internal/lifecycle/orphan_hints.go` (new): the single derivation —
   takes a `guard.DestructionOutcome` + `gitutil.WorkDestructionEvidence`
   (cannot be called without one — O2-7's structural leg), returns the
   closed outcome plus rendered lines via bead 2's constructor. Pinned
   outcomes per R2: ancestor-of-main-not-of-spec → deletion hint
   (the reachable case — an ancestor-of-spec branch never becomes an
   `Orphan`, `orphans.go:55-57`; no dead fixture); superseded → stale-
   branch deletion + the full `mindspec impl adopt <spec-id> --reason
   "<text>"` invocation; stale-deletion → inspection-first with a
   concrete `git diff` command + the merging-would-delete-landed-work
   sentence; normal unmerged → exactly `mindspec complete <bead>`
   (byte-unchanged); evidence-error → inspection-first. Deletion hints
   carry their proof (evidence class named; unlanded-commits →
   preserve-first, e.g. tag before delete). Supersession attribution
   enriched via `FindLandedMerge` (the preamble's mechanics split).
2. Consumers converted — all four render the derivation output:
   `internal/complete/complete.go:520`/`:552`,
   `internal/approve/impl.go:850` (`implOrphanRefusal`),
   `internal/doctor/orphaned_beads.go:100` — plus
   `internal/approve/adopt.go`'s R1g refusal upgraded from the interim
   inspection-first form to the derived form (AC-2(viii)). Emitter-
   enumeration anti-drift test covers all five. Consumer fixture tables
   each assert `len(table) == guard.DestructionOutcomeCount`.
3. `internal/doctor/orphaned_beads.go:99-101`: `FixFunc` attached ONLY
   on the normal-unmerged outcome (where it remains
   `runMindspecCompleteFn`); all other outcomes carry no FixFunc —
   `doctor --fix` mutates nothing there (AC-3(vi)); output and action
   derive from one value.
4. `internal/lifecycle/outcome_oracle_test.go`: the AC-3 oracle per the
   Testing Strategy — byte-match per-outcome templates (whole-line,
   metacharacters fail, the two injection fixtures), probe-derived
   ground truth with the independence clause asserted mechanically per
   the P2-plan-2 pin: the file-scoped AST call/identifier scan over the
   named forbidden set PLUS the no-gitutil-import leg, with every
   ground-truth helper confined to the oracle file; divergence =
   failure; harness executes only probes.
5. AC-3 table over all five outcomes through the REAL scan; leg (iii)
   convergence drives the REAL in-process `MindspecExecutor.CompleteBead`
   (test-authored call) under the fake-bd PATH shim per the Testing
   Strategy's P2-plan-1 pin (`landed_e2e_test.go:88-104` precedent,
   FATAL-not-skip) — satisfying the leg through an `executor.Executor`
   wrapper/mock that fabricates the merge is FORBIDDEN (R6
   stub-hollowing, stated in the bead prompt);
   consumer-parity leg (all four production surfaces render identical
   derivation output). **AC-2(viii)** composite fixture: recreated spec
   branch + closed bead's stale branch → adopt refuses before mutation,
   renders the derived hint (inspection `git diff`, evidence-proven
   deletion + preserve-first when unlanded commits exist, adopt rerun),
   never `mindspec complete`; oracle proves the deletion hint right.
6. Registry exits: the four converted consumer sites exit the seeded
   allowlist with named bead-4 exit records (fixture β stays green).

**Verification**
- [ ] `go test -short ./internal/lifecycle/... ./internal/complete/... ./internal/approve/... ./internal/doctor/...` passes
- [ ] AC-3 legs (i)/(ii)/(iv)/(v) RED at `09f62bd9` (static string today), leg (iii) guard green, leg (vi) RED (FixFunc unconditional today); every leg oracle-judged; parity + emitter-enumeration green
- [ ] AC-2(viii) RED today; oracle independence red on a test-side REFERENCE to any forbidden-set symbol (call-scan leg, same-package-capable) and on a gitutil import (imports leg) — both demonstrated by revert-shaped probes; convergence leg green under the fake-bd shim, with no Executor wrapper/mock in the leg's call path
- [ ] Registry exit records named; sweep still green; full gates clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-3 — all legs incl. FixFunc gating and oracle discipline (legs i/ii/iv/v/vi RED today)
- [ ] AC-2(viii) — the composite incident state, adopt refusal renders the derived hint (RED today)

**Depends on**
Beads 2 and 3 (names adopt — a shipped surface), plus bead 5
(seam-serialization on `internal/approve/impl.go`/`complete.go` — a
scheduling edge added by this plan, not a spec dependency; rationale in
the preamble). (bd edges wired from `work_chunks[].depends_on`.)

## Bead 5: R3a/R3c approve refusals

Branch-missing/indeterminate preflight on `impl approve`; plan re-approve
closed-child deletion gated by positive provenance passed as a value.
R3b is deliberately NOT here (bead 6 owns it with the R4 preflight —
O3-r2-5); this bead's panel does not judge AC-5. Owns the impl.go §1
preflight-phase structure bead 6 consumes (C-r4-7).

**Steps**
1. `internal/approve/impl.go`: branch existence resolved as a §1
   preflight fact before any merge-base/plumbing touches the branch
   name (structuring the preflight phase — C-r4-7). Missing branch →
   refusal naming the absent branch, the external-merge likelihood, and
   `mindspec impl adopt <spec-id> --reason "<text>"` in full; no raw
   `exit status 128`, no wrapped `*exec.ExitError` escapes. Probe error
   → its own fail-closed retryable leg, distinct wording, does NOT name
   adopt ("could not determine" never becomes "absent") — AC-4(ii).
   Existence probe behind the existing seam family
   (`branchExistsFn`-style) with pointer pin.
2. `internal/approve/plan.go:810-828`: provenance resolved in the
   `ApprovePlan` preflight (where children are already resolved) and
   passed INTO `checkExistingBeadsSafety` as a value (purity preserved —
   S3-r2-5). Closed child: deletion hint ONLY on positive
   partial/interrupted provenance (mechanics: no bead branch exists AND
   no landed-merge evidence AND, when present, supersede-run markers —
   the `plan.go:715-728` by-construction model); completed-work OR
   ambiguous/unavailable evidence → preserve + inspection/
   reconciliation, no `bd delete`. Both plan.go emitters route through
   bead 2's constructor (never allowlisted — O1-r2-6), sharing ONE
   constructor-produced `bd delete <id> --force` form — AC-11's
   single source of truth, with bd-CLI drift covered by the existing
   `bead.IsUnsupportedFlagError` convention and no `bd` string ever
   passed to `resolveCommand`.
3. `internal/approve/plan_provenance_test.go`: AC-6 as a pure hermetic
   table test over the provenance value (no bd, no git, no identity —
   CI-parity by construction). AC-4 fixtures: real-git deleted-branch
   repo (leg i), seam-forced probe error (leg ii).
4. Registry exits — TWO named bead-5 records (P2-plan-3: step 2
   converts BOTH `plan.go` emitters, so both seeded entries must exit
   or fixture β carries an unfalsifiable leftover): the closed-child
   site (`checkExistingBeadsSafety`, seed at `:820-825`) AND the
   partial-create site (`beadCreateFailure`, `:703-733` — its literal
   `bd delete %s --force` Sprintf is a provable-shape floor match the
   seed scan captures). Fixture β stays green with no dead entry
   surviving to bead 7's burn-down.

**Verification**
- [ ] `go test -short ./internal/approve/...` passes; AC-4(i) RED at `09f62bd9` (raw merge-base error today), AC-6(i) RED (`plan.go:823` unconditional today); AC-4(ii) red-on-revert (new seam leg)
- [ ] `exit status 128` appears nowhere in AC-4 output; adopt named in leg (i) only; AC-6 table pure (no I/O — asserted by the test's own construction)
- [ ] Both registry exits named (closed-child + partial-create); sweep green; full gates clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-4 — branch-missing + indeterminate preflight legs (leg i RED today)
- [ ] AC-6 — closed-child deletion needs positive provenance, pure table (leg i RED today)
- [ ] AC-11 — the single constructor-produced bd form (anti-drift)

**Depends on**
Beads 2 and 3 (per the spec; the R3a refusal names adopt in full).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 6: R4 merge preflight on every producer + R3b + R5(d) re-entry

The single mandatory work-destruction preflight (live-consultation
design, plan preamble) on all three producers; `--allow-net-deletion` with friction
registration; the chokepoint enumeration; the `:1688`/`:1721` conversion
to the `--resolve-merge` resumption surface with the preserved-merge
precondition across every committing path; AC-5. NOT split
producer-wise (AC-7(iv) is a whole-set universal).

**Steps**
1. Verb-layer §1 preflights EVALUATE the predicate through the
   `lifecycle.EvaluateWorkDestruction` wrapper behind in-package seam
   vars (the `implIsAncestorFn` pattern; ADR-0030 — no clearance is
   minted, nothing crosses a package boundary):
   `internal/complete/complete.go` (before the step-2.5/3
   materialization subphase — the user-facing R4 refusal fires here,
   so no refusal leaves a tool-generated commit, AC-7(v));
   `internal/approve/impl.go` for FinalizeEpic's per-bead auto-merge
   set and the direct spec→main leg (consuming bead 5's
   preflight-phase structure). Per-class outcomes (F3-r2-1):
   ancestor → proceed no-op; superseded/stale-deletion → refuse naming
   evidence + `--allow-net-deletion "<reason>"`; evidence-error →
   fail-closed retryable (the `:640-659` precedent). Spec→main leg
   hoisted above the cleanup block (F3-r2-3 — the `guardMergeLayout`
   precedent at `:845-857`); refusal fires with every worktree/branch
   still present (AC-7(iii)).
2. `internal/executor/mindspec_executor.go`: the three producers
   (`:411`, `:703`, `:916`) consult the predicate LIVE at the merge
   moment via the in-package seam `workDestructionFn =
   gitutil.EvaluateWorkDestruction` (pointer-pinned, AC-17 set) over
   the CURRENT operand tips, immediately before
   `MergeInto`/`MergeBranch` — the preamble's revised R4 resolution.
   A producer-site refusal is the fail-closed backstop for state that
   changed since §1 (branch-side self-drift is class-invariant per
   R4(a); target-side drift is the new coverage): it aborts exactly
   like the `:640-659` ancestry-error precedent, mutating nothing
   further, and its disposition is STATED — for `complete`, recovery
   is re-running the verb, whose §1 preflight then refuses user-facing
   before any new commit; for FinalizeEpic's loop at bead N, beads
   1..N−1 are already merged with their landed bindings written — a
   durable, legitimate partial state — and the re-run converges over
   them via the `ancestor` no-op leg while §1 surfaces the refusal for
   bead N. Chokepoint anti-drift test enumerates every
   `MergeInto`/`MergeBranch` call site in lifecycle/executor packages
   and asserts each CONSULTS the preflight — dataflow from a
   `workDestructionFn` evaluation of that call's operand pair to the
   merge call, not call-site adjacency — AC-7(iv), whole and unstaged.
3. Override + friction: `--allow-net-deletion "<reason>"` recorded on
   epic metadata (the `--allow-doc-skew` pattern) AND registered in
   `cmd/mindspec/selfemit.go`'s `escapeHatchFlags` + `detectFriction`
   order (position stated in code — S3-r2-7) and
   `internal/redact.EscapeHatchTokens`.
4. R5(d) conversion: neither conflict emitter prints any raw
   `git merge` — payloads become the `--resolve-merge` re-entry
   invocation + pinned resolution steps, still constructor-routed. The
   re-entry surface per R5(d)(i)-(vi): fresh preflight at invocation
   over current operand tips; resolves its own worktree/branch (no
   scrollback operand); resumption-not-start-only (unmerged index →
   re-print steps and exit; resolved index + MERGE_HEAD → complete the
   product-initiated merge with the seeded subject, never
   checkout/abort); `abortMergeState` (`:1634-1643`) gains the
   own-run-only precondition; seeded subjects (bead leg names the bead
   branch — identity-load-bearing per `landed.go:255-276`; spec→main
   subject human-facing, recorded per leg); completion via
   `git commit --no-edit` (E-r5-5's recorded mechanics); worktree-state
   refusal for no-merge-state failures (names blocking paths, never
   "conflict", never an unchanged re-entry loop — E-r5-4); resolution
   steps byte-match templates with add-step operands = the emitter-
   computed conflicted set (`-A`/`.`/`-u` forbidden in emitted lines —
   E-r5-3). Correct the stale `:1662-1670` "belt-and-suspenders"
   comment (A-r4-5). `cmd/mindspec/complete.go`/`impl.go` register the
   `--resolve-merge` flags, with leaf-identity + flag-membership tests.
5. Preserved-merge precondition (R5(d)(v) — universal): every
   `CommitAll`/`commitWithExport` call site (`mindspec_executor.go:370`
   /`:576`/`:623`/`:1041`/`:1126`; `approve/spec.go:122`/`:131`;
   `approve/plan.go:399`/`:409` — the verified full set at this base)
   and the three producers check the target worktree for an in-progress
   merge their run did not create → refuse fail-closed naming the
   preserved conflict, its paths, and the re-entry invocation;
   FinalizeEpic's warn-and-continue on the `:576` commit failure
   becomes a refusal for exactly this class (E-r5-2). Shared helper +
   call-site enumeration test so a new committing path cannot bypass.
6. Tests: **AC-7** all five legs (producer parity via the same evidence
   shape on all three; override fixture binds ONLY `--allow-net-
   deletion` — S3-r2-7; tracker-path refusal leaves the bead tip at the
   panel-reviewed SHA). **Target-drift backstop fixture (P1-4's
   dissolution, demonstrated not asserted):** the target branch is
   advanced to a destruction-shaped state between the §1 evaluation
   and the producer's merge (seam/hook-forced) — the producer refuses,
   bead N is unmerged, prior merges and bindings are intact, and the
   re-run converges with the §1 refusal and no new tool-generated
   commit. **AC-5** (R3b): recreated stale spec branch →
   finalize merge refused before mutation. **AC-8(i)** golden replay
   byte-identical (the common path untouched); **AC-8(iv)** seam-forced
   evidence error → retryable refusal naming the override; **AC-8(v)**
   post-conflict convergence (re-entry driven in-process, resolve,
   complete, re-run converges via ancestor-no-op — no refusal, no
   override). **AC-9(v)** all eight sub-legs (α-η) per the spec,
   red-on-revert on both E-r5-1 destruction mechanisms (δ) and the
   E-r5-2 two-parent-`chore:` resurrection (ε).
7. Registry exits: the `:1688`/`:1721` seeded emitter entries exit with
   named bead-6 records (converted through the constructor).

**Verification**
- [ ] `go test -short ./internal/executor/... ./internal/complete/... ./internal/approve/... ./cmd/mindspec/... ./internal/redact/...` passes
- [ ] AC-7(i)-(v) RED at `09f62bd9`; AC-5 RED; AC-9(v) legs each mapped to a named test; AC-8(i) golden byte-identical (guard); AC-8(v) convergence with zero override
- [ ] No raw `git merge` string in either emitter's output (string-asserted + the internal/lint scan stays green); friction admission + epic metadata both present on the override fixture; redact token registered
- [ ] Preserved-merge enumeration test covers the full call-site set; `abortMergeState` precondition fixture (never aborts a foreign merge)
- [ ] Target-drift backstop fixture green (producer-site refusal on mid-run target drift; partial state + re-run convergence asserted); `workDestructionFn` pointer pin green
- [ ] Full gates clean; zero `--override-adr` (this bead's own completion is the preflight's first live run — orchestrator note)

**Acceptance Criteria**
- [ ] AC-7 — preflight refuses on every producer, override audited, chokepoint whole (RED today)
- [ ] AC-5 — stale recreated spec branch refused via the preflight (RED today)
- [ ] AC-8(i)/(iv)/(v) — golden guard + error leg + post-conflict convergence
- [ ] AC-9(v) — re-entry/resumption legs α-η (RED today)

**Depends on**
Beads 1, 2, and 5 (the spec's edges: predicate; constructor; impl.go
preflight structure), plus bead 4 (seam-serialization on
`impl.go`/`complete.go` — a scheduling edge added by this plan;
preamble). (bd edges wired from `work_chunks[].depends_on`.)

## Bead 7: R5d/e sweep and burn-down — last by construction

Agent-template replacement, release.go recovery split, allowlist burned
down to the single pinned final entry, seed-only exemption entries
exited, and the AC-11 named-invocation anti-drift table over every
invocation this spec's messages and guidance name.

**Steps**
1. `.claude/agents/spec-orchestrator.md:163-175`: the raw bypass block
   (stash / raw merge / forced worktree removal / `branch -D` / raw
   `bd update --metadata`) removed; replacement names the PINNED
   invocations (O2-r2-6): `mindspec complete <bead-id> "<msg>"` for the
   `.beads/issues.jsonl`-churn case, `mindspec repair phase <spec-id>`
   for phase drift, and the documented #186 stale-SHA interim recovery
   per R5(e)/Out-of-Scope's 127→128 seam (the converging
   non-destructive path against behaviour at this base). AC-10(i)'s
   absence greps + pinned-invocation assertions.
2. `cmd/mindspec/release.go:257`: recovery line = the safe action
   (commit and re-run); the `--force` discard becomes a separately-
   labeled operator choice the classifier still sees (O3-r2-7) —
   the allowlist's ONE final entry, rationale + the tested obligation
   (discard never in the `recovery:` line — O2-r2-5).
3. Burn-down: every remaining seeded allowlist entry removed —
   `internal/guard/registries.go` ends at exactly ONE entry; AC-9(ii)'s
   exactly-one assertion + leftover/rationale-free/obligation-untested/
   second-entry red fixtures land here. The four seed-only
   orchestrator-block exemption entries exit with named bead-7 records
   (fixture β final state).
4. `cmd/mindspec/named_invocation_test.go` (AC-11): every `mindspec`
   invocation named by AC-1..AC-10 messages/guidance (`impl adopt` +
   both flags, `complete` + `--resolve-merge`, `impl approve` +
   `--resolve-merge` + `--allow-net-deletion`, `repair phase`,
   `release` with/without `--force`) resolves at leaf identity with
   flag-set membership; bare `Find` forbidden.
5. Final whole-tree sweep re-run as review evidence: zero floor matches
   outside the eight-entry exemption list across globs + canonical
   surfaces.

**Verification**
- [ ] `go test -short ./cmd/mindspec/... ./internal/guard/... ./internal/lint/...` passes
- [ ] AC-10(i)/(ii) RED at `09f62bd9`, green after; every replacement invocation resolves per AC-11
- [ ] AC-9(ii): exactly one allowlist entry; all four red fixtures fire on revert-shaped mutations; exemption list at eight live entries, four named exits
- [ ] internal/lint sweep green over the final tree; full gates clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-10(i)/(ii) — template replacement + release split (RED today)
- [ ] AC-9(ii) — burn-down to the exactly-one pinned entry (final state)
- [ ] AC-11 — named-invocation anti-drift (anti-drift)
- [ ] AC-10(iii)'s final membership (eight live entries, seed-only exits recorded)

**Depends on**
Beads 2, 4, 5, and 6 — last by construction (C-r4-4): the burn-down
cannot pass while any conversion bead's family is still seeded.
(bd edges wired from `work_chunks[].depends_on`.)

## Provenance

| Acceptance Criterion | Delivered / Verified By |
|---------------------|-------------------------|
| AC-1 | Bead 3 (happy path, parity fixture) |
| AC-2 (i)-(vii), (ix) | Bead 3 (refusal legs + lattice table) |
| AC-2 (viii) | Bead 4 (composite incident fixture, oracle-judged) |
| AC-3 (all legs) | Bead 4 (derivation, consumers, FixFunc, oracle) |
| AC-4 (i)/(ii) | Bead 5 (branch-missing/indeterminate preflight) |
| AC-5 | Bead 6 (R3b via the R4 preflight — spec-assigned) |
| AC-6 (i)-(iii) | Bead 5 (pure provenance table) |
| AC-7 (i)-(v) | Bead 6 (preflight, override, parity, chokepoint) |
| AC-8 (i) | Bead 1 captures golden; Bead 6 holds it (guard) |
| AC-8 (ii)/(iii) | Bead 1 (predicate boundary fixtures) |
| AC-8 (iv)/(v) | Bead 6 (error leg; post-conflict convergence) |
| AC-9 (i)/(iii)/(iv)/(vi) | Bead 2 (classifier, guard, amendment, opacity) |
| AC-9 (ii) | Bead 2 (discipline over seeds) + Bead 7 (final burn-down) |
| AC-9 (v) | Bead 6 (re-entry legs α-η) |
| AC-10 (iii)/(iv) | Bead 2 (sweep + exemption list + canonical leg); Bead 7 (seed-only exits) |
| AC-10 (i)/(ii) | Bead 7 (template + release split) |
| AC-11 (a) | Bead 7 (named-invocation table) |
| AC-11 (b) | Bead 5 (single constructor-produced bd form) |

Every spec AC row maps to at least one owning bead; no AC is unowned; no
bead ships without a named falsifying test (each bead's RED/red-on-revert
set above).
