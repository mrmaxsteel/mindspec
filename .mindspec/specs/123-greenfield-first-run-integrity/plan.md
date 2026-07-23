---
adr_citations:
    - ADR-0040
    - ADR-0036
    - ADR-0035
    - ADR-0032
    - ADR-0037
    - ADR-0039
    - ADR-0015
approved_at: "2026-07-23T11:44:38Z"
approved_by: user
bead_ids:
    - mindspec-ud0w.1
    - mindspec-ud0w.2
    - mindspec-ud0w.3
    - mindspec-ud0w.4
spec_id: 123-greenfield-first-run-integrity
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/bootstrap/bootstrap.go
        - internal/bootstrap/bootstrap_test.go
        - internal/domain/contextmap.go
        - internal/domain/scaffold.go
        - internal/domain/scaffold_test.go
        - internal/doctor/docs.go
        - internal/doctor/docs_test.go
        - internal/doctor/git.go
        - internal/gitutil/gitignore.go
        - internal/setup/claude.go
        - internal/setup/codex.go
        - internal/setup/copilot.go
    - depends_on: []
      id: 2
      key_file_paths:
        - internal/adr/create.go
        - internal/adr/create_test.go
        - internal/adr/parse.go
        - internal/adr/parse_test.go
        - internal/adr/show.go
        - internal/adr/show_test.go
        - internal/adr/supersede.go
        - internal/workspace/workspace.go
        - internal/workspace/workspace_test.go
        - cmd/mindspec/adr.go
    - depends_on:
        - 1
        - 2
      id: 3
      key_file_paths:
        - internal/config/config.go
        - internal/config/config_test.go
        - cmd/mindspec/config.go
        - cmd/mindspec/models.go
        - cmd/mindspec/commands.go
        - cmd/mindspec/greenfield_e2e_test.go
        - internal/doctor/config.go
        - internal/doctor/config_test.go
        - internal/bootstrap/bootstrap.go
        - internal/setup/codex.go
        - internal/setup/codex_test.go
        - internal/setup/claude.go
        - .mindspec/adr/ADR-0040-orchestration-layering-ratchet.md
    - depends_on: []
      id: 4
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
        - cmd/mindspec/config.go
        - internal/complete/panel_gate_layout_test.go
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - internal/setup/skills_test.go
---
# Plan: 123-greenfield-first-run-integrity

Four beads, following the spec's own suggested decomposition — (1)
first-run workspace integrity R1–R4, (2) ADR filename/ID convention
R5, (3) declared-config parity + consumer identity R6+R7+R9, (4)
ad-hoc panel path R8 — with the spec's PLAN-TIME NOTE (the O2-1
file-overlap constraint) resolved by **option (a): sequencing**.

**The R4↔R7 file-overlap resolution (O2-1 — the 117 seam-leak
class).** Bead 1 (R4) and Bead 3 (R7) both edit
`internal/bootstrap/bootstrap.go` (Bead 1: the manifest item + the
`.gitignore` entry-granular append path; Bead 3: the
`starterAgentsMD`/`appendAgentsBlock` region, `:275-365`) and the
`internal/setup/*.go` verbs (Bead 1: the gitignore-ensure leg in all
three; Bead 3: `codex.go`'s `agentsMDManagedBlock` and the
config-sourced block rendering). These two beads are therefore NOT
declared parallel: `work_chunks` wires an explicit **3-depends-on-1
edge**, so Bead 3 branches only after Bead 1's edits to
`bootstrap.go`/`setup/*.go` have merged to the spec branch and no
concurrent editing of the shared regions can occur. The edge is
doubly load-bearing — it is also a genuine produced-state edge:
Bead 3 owns the AC-1/AC-19 integration fixtures, whose command
sequences consume Bead 1's context-map scaffold, `domain add`
convergence, gitignore ensure, and doctor checks (the fixtures
cannot pass without Bead 1's landed code).

**Dependency graph (explicit).** Edges: 3←1 (file-overlap sequencing
per O2-1 + AC-1/AC-19 consumed state), 3←2 (consumed state: AC-19
asserts the ADR lands as slugged `ADR-0001-first-decision.md`
reporting ID `ADR-0001` — Bead 2's emission + ID normalization).
Beads 1, 2, and 4 have no incoming edges. The graph is acyclic by
construction: every edge points from chunk 3 to a lower id, and no
other chunk declares dependencies.

- **Waves**: W1 = {1, 2, 4} (mutually independent — disjoint files:
  Bead 1 owns bootstrap/domain/doctor-docs+git/gitutil/setup-gitignore;
  Bead 2 owns internal/adr + the workspace ADR resolver +
  cmd/mindspec/adr.go; Bead 4 owns cmd/mindspec/panel.go + the skill
  text), W2 = {3}.
- **Longest dependency chain**: 2 (1→3, 2→3) — within the ≤3 limit.
- **Bead count**: 4 — within the 3–5 target.

**Shared-file adjacencies WITHOUT edges** (shared source files are
not dependencies; flagged for the implementers):

- Beads 3 ∥ 4 both touch `cmd/mindspec/config.go` — disjoint hunks
  (Bead 3: the `commands:`/`models:` rendering near the key table,
  `:204-215` area; Bead 4: `configShowReviewRoots`, `:561`).
  Whichever merges second rebases trivially.
- Bead 1 and Bead 3 both touch `internal/bootstrap/bootstrap.go` and
  `internal/setup/*.go` — this is exactly the O2-1 overlap, and it
  carries an EDGE (3←1), so it is sequential, never parallel.
- No other file is touched by more than one bead.

**AC-ownership re-cut (one conscious placement, stated for the plan
panel).** AC-1 and AC-19 are owned by **Bead 3**, not Bead 1, even
though their RED-today trigger is Bead 1's R1/R2 code. Reason: both
criteria's assertion sets are only EXPRESSIBLE once Bead 3's checks
exist — AC-1 asserts the `missing-models` Warn (R6c) and the
unset-build-guidance Warn (R7c) PRESENT, and scopes no-Error/Missing
across all four governed lanes including the models and managed-block
lanes; AC-19 additionally consumes Bead 2's slugged ADR and Bead 3's
managed-block refresh seam. A Bead-1-time test asserting those Warns
present would fail against checks that do not yet exist. Bead 1
therefore runs the AC-1 command sequence as a lane-scoped SMOKE in
its own verification (exit 0, `### Alpha` entry, no Error/Missing
from the context-map and gitignore lanes — the same trigger, subset
assertions), and Bead 3 hardens it into the full AC-1 pin. The
RED-on-revert property is preserved: reverting Bead 1's fix makes
Bead 3's AC-1/AC-19 tests red (the sequence breaks at `domain add`).
Every AC has exactly one owning bead (Provenance table below).

**Transient-window note (for the Bead 1 review panel — F2 INFO).**
Between Bead 1 merging and Bead 3 merging, R1/R2's end-to-end
first-run behavior is pinned in the spec branch ONLY by Bead 1's
lane-scoped smoke (the AC-1/AC-19 full pins land with Bead 3). This
is acceptable — Bead 1's own AC-2/AC-2b/AC-3/AC-5/AC-6/AC-7 already
pin every R1–R4 unit, and the smoke covers the greenfield
`init`→`domain add` sequence — but the Bead 1 panel MUST spot-check
that Bead 1's smoke is genuinely revert-RED (reverting the
context-map scaffold breaks it), so the R1/R2 regression floor is
never merely the not-yet-present Bead 3 pin during the window.

**Plan-level choices the spec delegates (Open Questions), resolved:**

- **Slug rules (R5a, fixed observably by AC-8)**: lowercase; every
  run of non-`[a-z0-9]` characters collapses to a single hyphen;
  leading/trailing hyphens trimmed; **cap 48 characters**, truncating
  at the last hyphen boundary at-or-under the cap (then trimming any
  trailing hyphen) so a truncation never splits a word; an empty
  derived slug falls back to the bare `ADR-NNNN.md` form. A supplied
  `--slug` is validated so that the composed stem `ADR-NNNN-<slug>`
  passes `idvalidate.ADRID` (`internal/idvalidate/ids.go:33`);
  invalid values are refused with an ADR-0035 recovery line, never
  silently corrected.
- **Consumer build/test declaration key (R7b)**: `commands:` — a
  free-form `map[string]string` (task → shell command) beside
  `models:` in `internal/config/config.go`, with the documented
  vocabulary keys `build` and `test` (documented vocabulary, not an
  enforced enum — the same posture as `models:`' phase keys). The
  managed Build & Test section renders the populated entries as a
  fenced code block in stable order (`build`, `test`, then any
  remaining keys sorted); UNSET ⇒ the section is omitted entirely
  (never a placeholder). Populate verb: **`mindspec commands
  populate`** (mirroring `mindspec source populate` /
  `mindspec models populate`); doctor check id: `missing-commands`.
- **Shared-helper homes (anti-drift)**: (i) the context-map skeleton
  emitter + entry predicate live in a new
  `internal/domain/contextmap.go` — `ContextMapSkeleton()`,
  `EntryHeading(name)`, `HasEntry(content, name)` — consumed by
  `appendContextMap` (emission), `bootstrap.Run`'s manifest item, and
  doctor's two docs-lane checks (detection), satisfying R3's
  single-shared-helper constraint; import direction is
  bootstrap→domain and doctor→domain (domain imports only workspace —
  no cycle). (ii) the gitignore entry-granular append moves to a new
  `internal/gitutil/gitignore.go` — `EnsureGitignoreEntries(root,
  entries...)` — one implementation for its three consumers
  (bootstrap, the three setup verbs, doctor's `--fix`), retiring
  doctor's private `appendGitignoreEntry` body
  (`internal/doctor/git.go:66-83`) into a call through the shared
  helper.
- **ADR resolution (R5c)**: ONE resolver, `workspace.ResolveADRFile
  (root, id)` beside today's exact-join `ADRFilePath`
  (`internal/workspace/workspace.go:578-583`): accepts the canonical
  `ADR-NNNN` or the full slugged form; resolves exact `<id>.md` plus
  the `<canonical>-*.md` glob; when more than one file carries the
  number (bare + slugged, or two slugged), errors naming BOTH paths
  with the ADR-0035 recovery line. Both red exact-join surfaces
  (`--supersedes` and `show`'s exact-match short-circuit,
  `internal/adr/show.go:45-58`) rewire through it, so collision
  detection cannot drift between callers.
- **Doctor check homes**: missing-context-map + unmapped-domain in
  `internal/doctor/docs.go` (the docs lane, beside the sibling dir
  checks `:19-39`); the unignored-untracked runtime-file check in
  `internal/doctor/git.go` (extending the existing runtime-file walk
  `:24-46`); `missing-models` + `missing-commands` in
  `internal/doctor/config.go` (beside `checkSourceGlobs`, `:63-96`).
- **Config-block scaffolder reuse (R6c/R7c)**: `scaffoldSourceGlobs`'
  three-state byte-preserving contract
  (`internal/doctor/config.go:98-145`) is generalized into one shared
  scaffolder parameterized by key + literal block, consumed by
  `source_globs` (behavior unchanged — pinned by existing tests),
  `models`, and `commands` — three blocks, one write discipline.
- **Ad-hoc panel routing on legacy layouts (R8)**: the `panelDirFor`
  ad-hoc branch routes flat layouts to `.mindspec/reviews/<slug>`
  (the skill's documented home) and legacy layouts to the existing
  repo-root `review/<slug>` convention — the Open Questions
  resolution; AC-15's fixture is flat.
- **Tally reach (R8c)**: `panel tally`/`panel show`-class readers
  locate panels via `findPanelRegistration` →
  `configShowReviewRoots(root)` (`cmd/mindspec/config.go:561`), which
  today returns the repo root + spec dirs — `panel.Scan` globs
  `review/`+`reviews/` under each, so `.mindspec/reviews/` is
  unreachable. Bead 4 appends the workspace dir (`.mindspec`) to that
  root list; `internal/complete`'s `panelGateRoots` is NOT touched
  (AC-16's isolation guard).
- **`init` reads config best-effort (R7b/AC-14)**: `bootstrap.Run`
  loads `.mindspec/config.yaml` when present (absent ⇒ zero-value ⇒
  Build & Test omitted) so init's managed block is config-sourced
  exactly like setup's — the FR-3 asymmetry guard AC-14 pins.
- **AC-1/AC-19 fixture home**: `cmd/mindspec/greenfield_e2e_test.go`,
  driving the cobra commands in a temp-dir `git init` fixture (the
  established `cmd/mindspec` temp-dir command-driving precedent —
  `init_test.go`, `migrate_test.go`).

**R9 authoring note.** The ADR-0040 amendment is AUTHORED DURING
IMPLEMENTATION, inside Bead 3's single commit — the bead whose code
(managed-block generation, `models:`/`commands:` blocks) first cites
the consumer-identity clause, per R9's "same bead as the first citing
code" rule — and is pinned by AC-18's anchor test. The amendment text
may be pre-drafted at plan/orchestration time for citation stability,
but it LANDS in Bead 3 so the ADR-divergence gate sees the declared
touchpoint with the citing code.

## ADR Fitness

- **ADR-0040 (Orchestration Layering Ratchet) — AMENDED by this spec
  (R9), the only ADR change.** The consumer-identity clause extends
  the layering doctrine outward to content mindspec generates INTO
  consuming repos: managed/scaffolded artifacts carry only
  framework-generic guidance or values sourced from the consumer's L2
  declared config — never mindspec-the-framework's own repo facts —
  and a declared-but-inert key is surfaced honestly, its guidance
  stack naming today's authoritative consumer. This is the ratchet's
  own logic (hardcoded prose drifts; declared config has one parsed
  representation) applied to generated content; amend-not-supersede
  is right because no existing clause is displaced. Lands in Bead 3
  (see the R9 note), cited from the managed-block generation sites
  and both config blocks; AC-18 anchors it.
- **ADR-0036 (Ownership Discovery / ZFC) — unchanged, APPLIED
  twice.** The `source_globs` guidance stack (literal commented
  schema block + ZFC populate-prompt emitter + doctor nudge +
  three-state byte-preserving `--fix` scaffold; framework proposes no
  values) is extended verbatim to `models:` (R6) and `commands:`
  (R7c) in Bead 3. The framework never guesses a consumer's build
  system or model protocol — unset means omitted plus a nudge
  (Non-Goals: no auto-detection). Remains the best-fit pattern; this
  plan strengthens it by making the three blocks share one
  scaffolder.
- **ADR-0035 (Agent Error Contract) — unchanged, applied.** Every new
  finding, warning, and refusal carries a single-lever recovery line:
  unmapped-domain → `mindspec domain add <name>`; missing-context-map
  / unignored-runtime-file / missing-models / missing-commands →
  `mindspec doctor --fix` or the named populate verb; the ADR
  collision error → rename/remove so exactly one file carries the
  number; the `--spec`+`--gate adhoc` refusal → both valid
  invocation forms; the invalid `--slug` refusal → the accepted slug
  shape.
- **ADR-0032 (Semantic ADR Coverage Gates) — unchanged, protected.**
  Bead 2's ID normalization guarantees every surface reports and
  accepts canonical `ADR-NNNN` regardless of filename slugging
  (AC-9), so citation gates and `--supersedes` chains keep matching;
  slugs are filename ergonomics only (Non-Goals: no slug backfill of
  citation sites).
- **ADR-0037 (Panel Gate as Enforced Contract) — unchanged,
  protected.** Bead 4 adds REACH (ad-hoc panels become creatable and
  deterministically talliable), not AUTHORITY: `mindspec complete`'s
  panel scanning (`panelGateRoots`) never consults
  `.mindspec/reviews/`, pinned by AC-16's byte-identical
  gate-evaluation guard.
- **ADR-0039 (Flat Layout v2) — unchanged, applied.** The scaffolded
  context map lives at the flat `.mindspec/context-map.md`
  (`workspace.ContextMapPath`, resolution untouched); ad-hoc panels
  live at the flat `.mindspec/reviews/<slug>/`; legacy layouts route
  to the legacy review root (the Open Questions resolution).
- **ADR-0015 — unchanged, finally enforced by scaffolding.**
  `session.json`/`focus` are local runtime state (the classification
  `internal/doctor/git.go:22-23` already cites); Bead 1's R4 makes
  init/setup/doctor enforce that classification proactively instead
  of detecting its violation after the fact.

No divergence from any accepted ADR is proposed; the one amendment
(ADR-0040) is the spec's own R9 requirement, not a plan-detected
divergence.

## Testing Strategy

- **Temp-dir real-git fixtures for every scaffolding path.** All
  init/setup/domain-add/doctor fixtures run in throwaway `git init`
  dirs; idempotency is asserted by FULL-FILE BYTE COMPARISON after a
  second run (AC-2, AC-5, AC-6, AC-14's outside-marker preservation),
  and ignore-ness by real `git check-ignore` (AC-5/AC-6).
- **Greenfield E2E at the cmd layer.** AC-1 and AC-19 drive the
  actual cobra commands in sequence
  (`cmd/mindspec/greenfield_e2e_test.go`). Doctor assertions use
  AC-1's governed-lane scoping: NO Error/Missing from the
  context-map, gitignore, models, and managed-block lanes; the
  beads-not-initialized `Missing` and the two DESIGNED first-run
  Warns (`missing-models`, `missing-commands`) asserted
  PRESENT-but-permitted (the fixture populates neither key, and ZFC
  forbids guessing them).
- **Anti-drift seam pins (the 121 AC-17 pattern).** AC-4:
  `appendContextMap`'s emission and doctor's detection both route
  through package seam vars defaulting to the SAME exported
  `internal/domain` helpers (`EntryHeading`/`HasEntry`); a
  function-identity test asserts both defaults are those symbols, so
  a private reimplementation at either site fails the test.
- **Content/grep pins.** AC-12 pins the `models:` block's
  declared-but-inert honesty text; AC-17 greps the SHIPPED
  `ms-panel-run` skill content (via `pluginmindspec.SkillFiles()`)
  for the `panel create --gate adhoc` invocation shape; AC-18 greps
  the amended ADR-0040 for the consumer-identity anchors; AC-13
  greps produced `AGENTS.md` for the forbidden `make build` /
  `make test` / `MindSpec Project` strings.
- **RED-on-revert discipline, deviations tagged in-test.** Every AC
  test reproduces its issue's trigger and fails on today's `main`,
  EXCEPT the spec's explicit *guard* legs: AC-7(iii) (tracked→Error
  precedence), AC-9's full-stem `show` glob-fallback leg, AC-10's
  mixed-directory numbering floor, AC-16 (gate isolation), and
  AC-14's `--check`-writes-nothing leg. Bead panels spot-check
  revert-RED where cheap and record it in review evidence.
- **Validation proofs.** Each bead's verification runs its package
  subset of the spec's Validation Proofs commands plus
  `golangci-lint run ./...`; the spec-end review evidence maps every
  criterion AC-1..AC-19 (including AC-2b, AC-14b) to exact
  `go test <pkg> -run <test>` commands. Known pre-existing flakiness
  (`mindspec-z4ps`) is the only tolerated red, byte-identical to the
  spec-init SHA.

## Bead 1: First-run workspace integrity — context-map scaffold, domain-add convergence, gitignore ensure, doctor detection

R1–R4 in full (#207 + #208): `init` scaffolds a first-run-complete
workspace; `domain add` converges from every partial state; runtime
state is gitignored by init AND all three setup verbs; doctor detects
the missing/incomplete context map and unignored runtime files with
named recoveries. Tool-grouped deliberately: one bead owns every edit
to `bootstrap.go`, `scaffold.go`, the doctor docs/git lanes, and the
setup verbs' gitignore leg (the R4 half of the O2-1 overlap — Bead 3
sequences after this bead).

**Steps**

1. Add `internal/domain/contextmap.go`: `ContextMapSkeleton()` (a
   title line, a `## Bounded Contexts` section, then a `---`
   separator — so `appendContextMap`'s insertion scan
   (`internal/domain/scaffold.go:118-150`) inserts entries at the
   intended place, per R1), `EntryHeading(name)` (the exact
   `### <Title>` heading the writer emits), and
   `HasEntry(content, name)` (the R3 "mapped" predicate). Rewire
   `appendContextMap` (`scaffold.go:96-100`, `:118-150`) through
   them: create the skeleton when `context-map.md` is absent (R2a),
   insert via `EntryHeading`, skip-without-duplicate when `HasEntry`
   already holds.
2. R2(b/c) convergence in `internal/domain/scaffold.go`: replace the
   dir-exists refusal (`:40-42`) with per-file create-if-missing —
   each of the four templates + `OWNERSHIP.yaml` written only if
   absent, existing files never overwritten — plus the context-map
   entry backfill; refuse "already exists" ONLY when the domain is
   fully scaffolded AND mapped. Write order (R2c): domain files
   first, context-map entry last, so any failure after the dir
   exists leaves a state a bare re-run of the same command repairs
   (create-if-missing + backfill are both idempotent).
3. R1: add the `.mindspec/context-map.md` manifest item to
   `bootstrap.Run` (`internal/bootstrap/bootstrap.go:218-235`) with
   content `domain.ContextMapSkeleton()`, following the manifest's
   additive discipline (create-only, never overwrites an existing
   file).
4. R4a: add `internal/gitutil/gitignore.go` —
   `EnsureGitignoreEntries(root, entries...)`, exact-line presence
   check + entry-granular append, never reordering or rewriting
   existing bytes (the `appendGitignoreEntry` discipline,
   `internal/doctor/git.go:66-83`). `bootstrap.Run` calls it for the
   two runtime entries when `.gitignore` already exists (the
   greenfield create-from-scratch item `bootstrap.go:232` is
   unchanged); doctor's `--fix` path rewires through the shared
   helper.
5. R4b: `setup claude`, `setup codex`, `setup copilot`
   (`internal/setup/claude.go`, `codex.go`, `copilot.go`) each
   ensure the same two entries via `EnsureGitignoreEntries`;
   `--check` mode writes nothing.
6. R3 + R4c doctor checks: (a) docs lane
   (`internal/doctor/docs.go`): **missing-context-map** —
   `context-map.md` absent at the layout-resolved path → `Missing`,
   `--fix` scaffolds `domain.ContextMapSkeleton()` (mechanical,
   structure-only, ZFC-safe); **unmapped-domain** — each `domains/`
   dir with no `domain.HasEntry` match → `Warn` naming the domain
   with recovery `mindspec domain add <name>`; detection routes
   through the shared-helper seam vars (AC-4). (b) git lane
   (`internal/doctor/git.go:24-46` extension): per runtime file,
   not-tracked AND `git check-ignore` misses → `Warn` ("runtime file
   not gitignored — one `git add .mindspec/` from being committed")
   with `--fix` appending via the shared helper; ignored → `OK`;
   tracked → the existing `Error` + untrack `--fix` unchanged and
   taking precedence.
7. AC-4 anti-drift pin + fixtures. The pin: a function-identity test
   asserting BOTH seam defaults (scaffold emission, doctor
   detection) are the exported `internal/domain` helpers. The
   fixtures: AC-2 (the exact #207 aftermath: dir scaffolded, no
   context map → backfill succeeds, five domain files byte-identical,
   second re-run refuses with the map byte-identical); AC-2b
   (partial file set: `runbook.md` + `OWNERSHIP.yaml` absent →
   backfilled, present files byte-identical, entry backfilled); AC-3
   (i) missing map → finding + `--fix` clears, (ii) unmapped alpha →
   Warn + `domain add alpha` clears, (iii) fully mapped → both OK;
   AC-5 (pre-existing unrelated `.gitignore` → exactly the two
   entries appended, prior bytes/order preserved, `git check-ignore`
   matches both, second `init` byte-identical); AC-6 (one test per
   setup verb, never-ran-init repo: entries ignored, re-run
   byte-idempotent, `--check` writes nothing); AC-7 (i)
   unignored-untracked → Warn + `--fix` → OK, (ii) entry present →
   OK, (iii) tracked → existing Error + untrack `--fix` unchanged
   (guard). Plus the lane-scoped AC-1-shape SMOKE: empty dir →
   `git init` → `mindspec init` → `mindspec domain add alpha` exits
   0 with the `### Alpha` entry under `## Bounded Contexts` before
   the `---` separator, and doctor reports no Error/Missing from the
   context-map and gitignore lanes (the full AC-1 pin, including the
   R6c/R7c present-Warn legs, lands in Bead 3 — see the preamble
   re-cut).

**Verification**

- [ ] `go test ./internal/bootstrap/... ./internal/domain/... ./internal/doctor/... ./internal/gitutil/... ./internal/setup/...` passes; `golangci-lint run ./...` clean
- [ ] AC-2 fixture RED on today's `main` ("already exists" refusal, entry never backfilled); green with five domain files byte-identical + duplicate-free re-run
- [ ] AC-2b fixture RED on today's `main` (`scaffold.go:40-42` refuses before any create-if-missing); missing files backfilled, present files byte-identical
- [ ] AC-3 subtests (i)/(ii) RED on today's `main` (doctor silent); `--fix` and `domain add alpha` clear their findings; (iii) fully-mapped OK
- [ ] AC-4 identity pin fails when either the scaffold emission or the doctor detection is rewired to a private reimplementation
- [ ] AC-5 RED on today's `main` (`.gitignore` item `Skipped`, `git check-ignore` misses); prior bytes preserved; second `init` byte-identical
- [ ] AC-6 per-verb subtests RED on today's `main`; re-run byte-idempotent; `--check` writes nothing
- [ ] AC-7(i) RED on today's `main` (currently "OK: not tracked"); (ii) OK; (iii) tracked→Error + untrack `--fix` assertions from `git.go:24-46` preserved (guard)
- [ ] Lane-scoped greenfield smoke: `tmp=$(mktemp -d) && cd "$tmp" && git init -q . && mindspec init && mindspec domain add alpha && grep -q '### Alpha' .mindspec/context-map.md` succeeds (RED on today's `main` — "reading context map" error)
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-2 — legacy partial-state convergence, byte-identical domain files, duplicate-free refusal on re-run
- [ ] AC-2b — missing-standard-file backfill with present files byte-identical
- [ ] AC-3 — missing-context-map + unmapped-domain doctor checks with clearing recoveries
- [ ] AC-4 — one shared "mapped" helper, emission + detection identity-pinned
- [ ] AC-5 — entry-granular gitignore append on init, byte-idempotent
- [ ] AC-6 — gitignore ensure on all three setup verbs, `--check` writes nothing
- [ ] AC-7 — doctor ignore-ness check (unignored-untracked Warn + `--fix`; tracked→Error guard)

**Domain:** workflow (primary — `internal/bootstrap`,
`internal/domain`, `internal/doctor`, `internal/setup` gitignore leg,
`internal/gitutil` helper) — per the spec's Impacted Domains.

**Depends on**
None (W1 root; Bead 3 sequences after it for the O2-1
`bootstrap.go`/`setup/*.go` overlap). (Human-readable narration only —
bd edges are wired exclusively from `work_chunks[].depends_on`.)

## Bead 2: ADR filename convention — slugged create, canonical IDs, collision-safe resolution

R5 in full (#206): `adr create` emits `ADR-NNNN-<slug>.md`; every
surface reports and accepts the canonical `ADR-NNNN` ID; exact-join
resolvers gain bare-or-slugged resolution; a bare/slugged number
collision errors instead of silently short-circuiting. Emission +
resolution only — never a rename of existing files (Out of Scope).

**Steps**

1. Slug derivation in `internal/adr/create.go` (`:139` today): the
   kebab rules from the plan choice (lowercase; non-alnum runs →
   single hyphen; trim; 48-char cap truncated at a hyphen boundary);
   empty derived slug → bare-filename fallback; `--slug` flag added
   in `cmd/mindspec/adr.go`, validated so the composed stem passes
   `idvalidate.ADRID`, refused (ADR-0035 recovery naming the
   accepted shape) otherwise. Write `ADR-NNNN-<slug>.md`.
2. R5(b) canonical ID: `ParseADR` (`internal/adr/parse.go:47`)
   derives `ADR.ID` as the `ADR-<digits>` prefix of the stem, never
   the full slugged stem — `list`/`show` report `ADR-0001` for
   `ADR-0001-integrate-at-contracts.md`.
3. R5(c) resolution: add `workspace.ResolveADRFile(root, id)` beside
   the exact-join `ADRFilePath`
   (`internal/workspace/workspace.go:578-583`) per the plan choice —
   canonical or full-slugged input; exact + `<canonical>-*.md` glob;
   multi-file number collision → error naming both paths + the
   ADR-0035 recovery line (rename or remove the redundant file so
   exactly one carries the number). **Rewire set (READ-resolution
   callers — enumerated so no caller keeps the drifting exact-join):**
   `internal/adr/show.go:30` (`Show`), `internal/adr/create.go:100`
   (the `--supersedes` predecessor read), `internal/adr/supersede.go:23`
   (`Supersede`'s `oldID` read) and `internal/adr/supersede.go:56`
   (`CopyDomains`, reading the superseded ADR's domain list). **KEEP
   exact-join `ADRFilePath` for the WRITE-target sites** —
   `create.go:139` (the new file's emission path) and
   `create.go:185` (the collision-check existence probe for the new
   `id`, whose slug-variant scan is emission-side and must not resolve
   to an unrelated pre-existing number): the writer composes the
   canonical new path, it does not resolve an existing one. `Show`'s
   own glob fallback (`show.go:45-58`, guarded by AC-9) is subsumed by
   routing `show.go:30` through the resolver, so `show` gains
   collision detection it lacked.
4. R5(c) collision fix in `internal/adr/show.go` (`:45-58`): route
   `show`'s lookup through the same resolver so the exact-`<id>.md`
   branch no longer returns BEFORE the glob runs — a directory
   holding both `ADR-0002.md` and `ADR-0002-foo.md` errors naming
   both; the existing slug glob-fallback behavior for a
   single-match slugged file is preserved (guard).
5. R5(d) fixtures: AC-8 (`adr create "Integrate at contracts, not
   tools" --domain alpha` writes
   `ADR-0001-integrate-at-contracts-not-tools.md`, stem passes
   `idvalidate.ADRID`, heading reads `# ADR-0001:`; `--slug my-slug`
   overrides; punctuation-only title → bare fallback); AC-9 ((i)
   `list`/`show ADR-0001` report exactly `ADR-0001` — RED today;
   (ii) `create "Successor" --supersedes ADR-0001` resolves the
   slugged predecessor and writes the chain against `ADR-0001` — RED
   today; full-slugged `show` input resolving asserted as guard);
   AC-10 (mixed dir bare `ADR-0001.md` + slugged `ADR-0002-foo.md`:
   canonical IDs listed and next `create` allocates `0003` — guard,
   `maxADRNum` `parse.go:260-300` already slug-aware and kept
   covered; planted `ADR-0002.md` + `ADR-0002-foo.md`: `show
   ADR-0002` errors naming both paths — RED today).

**Verification**

- [ ] `go test ./internal/adr/... ./internal/workspace/... ./cmd/mindspec/...` passes; `golangci-lint run ./...` clean
- [ ] AC-8 RED on today's `main` (bare `ADR-0001.md` written); derivation, `--slug` override, and punctuation-only fallback subtests green; composed stems pass `idvalidate.ADRID`
- [ ] AC-9(i)/(ii) RED on today's `main` (stem-ID reporting `parse.go:47`; exact-join miss `workspace.go:578-583`); full-slugged `show` guard green (glob fallback `show.go:45-58` preserved)
- [ ] AC-10 collision subtest RED on today's `main` (silent short-circuit to the bare file); error names BOTH paths + the ADR-0035 recovery line; numbering-floor guard green with mixed-directory `maxADRNum` coverage retained
- [ ] Invalid `--slug` refused with the recovery line, never silently corrected
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-8 — slugged emission, `--slug` override, bare fallback, `idvalidate.ADRID`-passing stems
- [ ] AC-9 — canonical ID reporting + exact-join bare-or-slugged resolution (+ full-slugged `show` guard)
- [ ] AC-10 — mixed-directory numbering guard + collision error naming both paths

**Domain:** workflow (primary — `internal/adr`, `cmd/mindspec/adr.go`)
+ core (`internal/workspace` resolver) — per the spec's Impacted
Domains.

**Depends on**
None (W1; disjoint files from Beads 1 and 4). Bead 3's AC-19 consumes
this bead's slugged emission — the 3←2 edge is declared on Bead 3.
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 3: Declared-config parity + consumer identity — models/commands stacks, managed-block rewrite, ADR-0040 amendment, first-run integration pins

R6 + R7 + R9 (#210 + #211), plus the AC-1/AC-19 integration fixtures
(the preamble re-cut). Sequenced AFTER Bead 1 (the O2-1 edge — this
bead rebases on Bead 1's landed `bootstrap.go`/`setup/*.go` edits)
and after Bead 2 (AC-19 consumes the slugged ADR). Carries the
ADR-0040 amendment — the first citing code lives here (R9).

**Steps**

1. Config surface (R6d/R7b): add `commands:` (`map[string]string`,
   the plan-choice shape) beside `Models`
   (`internal/config/config.go:54-66` area); render both in
   `mindspec config` (`cmd/mindspec/config.go` key table area,
   `:204-215`) — `models:` KEEPS the `declared, not yet enforced`
   inert annotation (`:22`) until an enforcement spec removes it;
   the existing advisory `renderKnownModelWarnings` (`:344`) is
   unchanged (Non-Goals).
2. ADR-0036 stacks (R6a/c, R7c) in `internal/doctor/config.go`:
   generalize `scaffoldSourceGlobs` (`:98-145`) into the shared
   three-state byte-preserving block scaffolder (source_globs
   behavior unchanged, existing tests pin it); add `modelsBlock`
   (mirroring `sourceGlobsBlock` `:18-29`) documenting the free-form
   phase→model-id map shape, the advisory vocabulary
   (`authoring`, `implementation`, `review` — documented, not
   enforced), and — verbatim honesty — that the key is
   declared-and-inert today, nothing in the binary changes behavior
   based on it, the authoritative consumers remain the orchestration
   skills, and wiring enforcement is a named follow-up; add
   `commandsBlock` documenting the task→command shape and the
   `build`/`test` vocabulary. Checks: `missing-models` Warn when
   `len(cfg.Models) == 0` (disclosing inert status, hinting
   `mindspec models populate`, `--fix` scaffolding the block) and
   `missing-commands` Warn when `len(cfg.Commands) == 0` (hinting
   `mindspec commands populate`, `--fix` scaffolding the block),
   both mirroring `checkSourceGlobs` (`:63-96`). Both new checks'
   `--fix` legs route through the SAME generalized three-state
   scaffolder as `source_globs` (parameterized by key + literal
   block) — no private per-key reimplementation; this shared routing
   is what PF-2's identity test (Step 6) pins for all three
   consumers.
3. ZFC populate emitters (R6b/R7c): `cmd/mindspec/models.go` +
   `cmd/mindspec/commands.go` — `mindspec models populate` and
   `mindspec commands populate` print agent prompts to declare the
   respective keys in `.mindspec/config.yaml` and WRITE NOTHING
   (mirroring `mindspec source populate`,
   `cmd/mindspec/source.go:23-45`).
4. R7(a/b) managed-content rewrite: `starterAgentsMD` +
   `appendAgentsBlock` (`internal/bootstrap/bootstrap.go:275-365`)
   and `agentsMDManagedBlock` (`internal/setup/codex.go:53-104`)
   become config-sourced renderers: neutral `# AGENTS.md` title
   (never "MindSpec Project"), "This project uses MindSpec" framing,
   ZERO framework repo facts (no `make build`/`make test` —
   `bootstrap.go:287-288`, `:364-365`, `codex.go:62-63`); Build &
   Test section rendered from `cfg.Commands` when populated (fenced,
   stable order per the plan choice), OMITTED entirely when unset.
   `bootstrap.Run` loads config best-effort (the plan choice) so
   init's block is config-sourced too (the FR-3 asymmetry guard).
   The wholesale BEGIN/END replacement (`claude.go:573-582`) is
   RETAINED — a `setup` re-run re-renders from current config, which
   is exactly what heals the AC-14b leaked-block state. Audit
   `claude.go`/`copilot.go` managed docs stay framework-fact-free
   (expected clean — `claude.go:789`, `copilot.go:140` delegate to
   AGENTS.md; asserted by AC-13's grep).
5. R9: amend
   `.mindspec/adr/ADR-0040-orchestration-layering-ratchet.md` with
   the consumer-identity clause exactly as the spec's ADR
   Touchpoints state it (managed/scaffolded consumer content is
   framework-generic or L2-sourced, never framework-repo-specific; a
   repo fact with no L2 home gets one or is omitted; declared-inert
   keys are surfaced honestly, their guidance stacks naming today's
   authoritative consumer). Cite it from the managed-block renderers
   and both config blocks. AC-18 anchor test
   (`rg -n 'consumer-identity|framework-generic'` non-empty +
   citation-site assertions).
6. Fixtures — parity + identity. AC-11 ((i) empty-`models:` Warn
   disclosing inertness + populate hint; (ii) `--fix` three-state
   scaffold, each state pinned, operator bytes never rewritten;
   (iii) `models populate` prints and writes nothing; (iv) populated
   `models:` clears the Warn and renders with the inert annotation
   intact). **Commands-scaffolder parity (PF-2 / R7c / R10 —
   proving the shared scaffolder's `commands` consumer, not just
   `models`):** a `missing-commands` `doctor --fix` three-state
   test mirroring AC-11(ii) — file-absent (block created),
   key-absent (block appended, other keys' bytes preserved),
   key-present (byte-preserving no-op / operator bytes never
   rewritten); a `mindspec commands populate` test asserting it
   prints the agent prompt and writes NOTHING (mirroring AC-11(iii));
   and the `missing-commands` Warn clearing once `commands:` is
   populated. **Shared-scaffolder identity pin (PF-2, the 121 AC-17
   anti-drift pattern):** a structural/identity test asserting the
   `source_globs`, `models`, AND `commands` `--fix` legs ALL route
   through the one generalized scaffolder function (e.g. all three
   `--fix` handlers hold the same scaffolder seam symbol, or a
   table-driven test drives one scaffolder across all three keys), so
   a private per-key reimplementation of any consumer fails the test
   — closing PF-2's "commands could be omitted or privately
   reimplemented while every listed check passes" gap. AC-12
   (block-honesty text pin — the block asserts
   declared-but-inert and names the follow-up; test fails if it
   claims enforcement); AC-13 (greenfield `init` AND `setup codex`,
   no Makefile/go.mod: produced `AGENTS.md` has NO `make build`,
   `make test`, or `MindSpec Project`; unset key ⇒ no Build & Test
   section at all); AC-14 (populated `commands:` e.g. `npm test`:
   `init` AND `setup codex` both render it; config change + `setup`
   re-run re-renders inside the markers, outside-marker bytes
   untouched; `--check` reports without writing); AC-14b (the
   primary #211 exposure: an existing consumer `AGENTS.md` carrying
   the exact 0.12.0 leaked block — `make build`/`make test` +
   "# AGENTS.md — MindSpec Project", no Makefile: after
   `setup codex`, the framework facts are GONE, the build section is
   consumer-declared-or-omitted per the key, outside-marker content
   untouched, and with the key unset the `missing-commands` Warn
   fires).
7. Fixtures — first-run integration (`cmd/mindspec/
   greenfield_e2e_test.go`): AC-1 (empty dir → `git init` →
   `mindspec init` → `mindspec domain add alpha`: exit 0; the
   `### Alpha` entry under `## Bounded Contexts` before the `---`
   separator; doctor per the governed-lane scoping — NO
   Error/Missing from the context-map/gitignore/models/managed-block
   lanes; the `missing-models` and `missing-commands` Warns asserted
   PRESENT; the beads-not-initialized `Missing` asserted
   present-but-permitted; plus the optional variant that runs
   `bd init` and populates both keys asserting fully-clean doctor);
   AC-19 (the full cross-verb E2E: … → `mindspec adr create "First
   decision" --domain alpha` → `mindspec setup codex` →
   `mindspec doctor`: every step exit 0; the ADR lands as
   `ADR-0001-first-decision.md` reporting ID `ADR-0001` — consuming
   Bead 2; init-then-setup operate on the SAME `AGENTS.md` with no
   duplicate managed block and no framework leak after the setup
   refresh (the O3-3 seam); runtime files gitignored; final doctor
   per the AC-1 scoping).

**Verification**

- [ ] `go test ./internal/config/... ./internal/doctor/... ./internal/bootstrap/... ./internal/setup/... ./cmd/mindspec/...` passes; `golangci-lint run ./...` clean
- [ ] AC-11 subtests RED on today's `main` (no command, no check, no block); three-state scaffold pinned per state; `rg -n 'missing-models' internal/doctor/` non-empty
- [ ] Commands-scaffolder parity (PF-2): `missing-commands` `doctor --fix` three-state test green (file-absent / key-absent / key-present, operator bytes never rewritten); `mindspec commands populate` prints and writes nothing; `missing-commands` Warn clears once `commands:` populated; `rg -n 'missing-commands' internal/doctor/` non-empty
- [ ] Shared-scaffolder identity pin (PF-2): a structural/identity test fails if `source_globs`, `models`, or `commands` is rewired to a private scaffolder — all three route through the one generalized function
- [ ] AC-12 honesty pin: the block text asserts declared-but-inert + names the follow-up; test fails on any enforcement claim
- [ ] AC-13 RED on today's `main` for BOTH verbs; `! grep -RnE 'make (build|test)|MindSpec Project' AGENTS.md` holds on both fixtures; unset ⇒ no Build & Test section
- [ ] AC-14 RED on today's `main` (no mechanism; init's block hardcoded); both verbs render the populated key; re-render preserves outside-marker bytes; `--check` writes nothing
- [ ] AC-14b RED on today's `main` (`setup` regenerates the leaked block); framework facts gone after refresh; `missing-commands` Warn fires with the key unset
- [ ] AC-1 RED on today's `main` (sequence breaks at `domain add`); governed-lane scoping + present-Warn assertions green; `bd init`+populated variant fully clean
- [ ] AC-19 RED on today's `main`; slugged ADR + same-AGENTS.md no-duplicate-block seam + gitignored runtime files + scoped final doctor all asserted in one fixture
- [ ] AC-18: `rg -n 'consumer-identity|framework-generic' .mindspec/adr/ADR-0040-orchestration-layering-ratchet.md` non-empty; citation sites assert the clause; the ADR-divergence gate sees the declared touchpoint
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-1 — greenfield first-run with governed-lane doctor scoping + designed-Warn presence (full-form pin; trigger fixed by Bead 1)
- [ ] AC-11 — models parity: Warn, three-state `--fix`, ZFC populate, inert annotation
- [ ] AC-12 — schema-block honesty pin
- [ ] AC-13 — no framework leak at both verbs; unset ⇒ omitted section
- [ ] AC-14 — declared build guidance round-trips at both verbs; refresh re-renders; `--check` writes nothing
- [ ] AC-14b — already-onboarded leaked-block consumer healed by the refresh; unset-key Warn fires
- [ ] AC-18 — ADR-0040 amendment anchors + citations from the managed-content and config-block sites
- [ ] AC-19 — full cross-verb first-run E2E incl. the init→setup managed-block seam and the slugged ADR

**Domain:** workflow (primary — `internal/bootstrap`,
`internal/setup`, `internal/doctor`, `cmd/mindspec`) + core
(`internal/config` keys) — per the spec's Impacted Domains.

**Depends on**
Bead 1 (the O2-1 file-overlap sequencing edge on
`bootstrap.go`/`setup/*.go`, PLUS consumed state: AC-1/AC-19 require
Bead 1's context-map scaffold, convergence, gitignore ensure, and
doctor checks) and Bead 2 (consumed state: AC-19 asserts the slugged
`ADR-0001-first-decision.md` reporting `ADR-0001`). (bd edges wired
from `work_chunks[].depends_on`.)

## Bead 4: Ad-hoc panel path — `panel create --gate adhoc`, tally reach, skill alignment

R8 in full (#209): the documented ad-hoc panel becomes creatable at
`.mindspec/reviews/<slug>/`, deterministically talliable, refused
when misfiled, isolated from every lifecycle gate, and the shipped
skill text states the now-real invocation. Independent of the other
beads (disjoint files; the `cmd/mindspec/config.go` adjacency with
Bead 3 is disjoint hunks — see the preamble).

**Steps**

1. R8(a) ad-hoc branch in `panelDirFor`
   (`cmd/mindspec/panel.go:352-366`): when gate is `adhoc`, resolve
   WITHOUT a spec — flat layout → `.mindspec/reviews/<slug>` (the
   location `ms-panel-run` documents,
   `plugins/mindspec/skills/ms-panel-run/SKILL.md:43-52`), legacy
   layout → the repo-root `review/<slug>` convention (the Open
   Questions resolution). Existing slug/target/round validation and
   `panel.json` stamping (expected reviewers, threshold, gate
   `adhoc` — already in `CanonicalGateKeys`,
   `internal/panel/disposition.go:93`) apply unchanged.
2. R8(b) flag contract (`panel.go:113-115`): `--spec` remains
   REQUIRED for every non-adhoc gate (guard preserved); `--spec`
   together with `--gate adhoc` is REFUSED with an ADR-0035 recovery
   line naming both valid forms (`panel create <slug> --spec <id>
   --target <ref>` for gated panels; `panel create <slug> --gate
   adhoc --target <ref>` for ad-hoc) — never a silent ignore.
3. R8(c) tally/reader reach: append the workspace dir (`.mindspec`)
   to `configShowReviewRoots` (`cmd/mindspec/config.go:561`) so
   `panel.Scan`'s `review/`+`reviews/` globbing registers
   `.mindspec/reviews/<slug>` — `findPanelRegistration` then serves
   `panel tally` (and `config show`'s panel listing) against the
   ad-hoc dir with zero tally-side changes.
4. R8(d) gate isolation: `internal/complete`'s `panelGateRoots` is
   NOT modified; AC-16's test plants an ad-hoc `panel.json`
   (including a REJECT verdict) under `.mindspec/reviews/` in a
   `complete`-gate fixture and asserts the panel-gate evaluation is
   byte-identical to the same fixture without it (beside the
   existing `internal/complete` panel-gate layout tests).
5. R8(d) skill alignment: update the `ms-panel-run` SKILL.md ad-hoc
   section to state the now-real `mindspec panel create <slug>
   --gate adhoc --target <ref>` invocation (no `--spec`) and the
   `.mindspec/reviews/<slug>/` location. The `--target <ref>` uses
   the BARE-ref form the CLI actually rev-parses — never the
   `commit <sha>` placeholder notation (PF-1) — matching the skill's
   existing Step-0 `panel create --spec <id> --target <ref>` block
   (SKILL.md:57); the semantic `bead <id>`/`pr <n>`/`commit <sha>`
   vocabulary at SKILL.md:29 is the `/ms-panel` WORKFLOW input
   (resolved to a bare ref before the CLI), left untouched.
   AC-17's test greps the SHIPPED skill content (through
   `pluginmindspec.SkillFiles()`, the `internal/setup` skills
   surface) for the `--gate adhoc` invocation shape and the
   `.mindspec/reviews/<slug>/` location so the skill cannot drift
   back to an uncreatable path — and, being bare-ref, it stays
   consistent with what `panel.go:149` accepts.
6. Fixtures: AC-15 (the filed repro, target normalized to a BARE
   resolvable ref — `mindspec panel create adr-review --gate adhoc
   --target "$(git rev-parse HEAD)"`, no `--spec`, in a repo with
   ZERO specs. #209's `"commit <sha>"` is placeholder notation:
   `panel create` rev-parses the RAW `--target` string
   (`cmd/mindspec/panel.go:149` `revParseForPanelFn(root, target)`),
   so the literal `commit `+sha fails `git rev-parse` — the CLI takes
   a bare ref, and #209's own command errored at the `--spec` guard
   BEFORE target-resolution, so the prefix form was never exercised.
   Bead 4 only removes the `--spec` requirement for `adhoc`; it does
   NOT add a `commit <sha>`/`bead <id>`/`pr <n>` semantic-target
   parser (that is #212 scope, Out of Scope). Expected: exit 0;
   `panel.json` under
   `.mindspec/reviews/adr-review/` with gate `adhoc` + configured
   reviewer/threshold stamps; `panel tally adr-review` runs against
   it; `--spec`+`--gate adhoc` refused with the recovery line;
   non-adhoc gate without `--spec` still errors — guard); AC-16 (as
   step 4); AC-17 (as step 5).

**Verification**

- [ ] `go test ./cmd/mindspec/... ./internal/complete/... ./internal/setup/...` passes; `golangci-lint run ./...` clean
- [ ] AC-15 RED on today's `main` ("--spec is required"); `mindspec panel create adr-review --gate adhoc --target "$(git rev-parse HEAD)"` (bare ref, no `--spec`) then `test -f .mindspec/reviews/adr-review/panel.json`; `panel tally adr-review` exits per its decision contract; both refusal legs asserted (spec+adhoc refused with recovery; non-adhoc specless still errors — guard)
- [ ] AC-16: `complete`-gate evaluation byte-identical with/without the planted ad-hoc REJECT panel; `panelGateRoots` diff is zero-byte
- [ ] AC-17: shipped-skill grep pins the `--gate adhoc` invocation shape and the `.mindspec/reviews/<slug>/` location (RED against today's skill text, which documents an unreachable path)
- [ ] `go build ./... && go test ./...` — no new red (z4ps caveat)

**Acceptance Criteria**

- [ ] AC-15 — verbatim ad-hoc repro succeeds; tally runs; both flag-contract refusals
- [ ] AC-16 — gate isolation: ad-hoc panels never influence `mindspec complete` (ADR-0037 guard)
- [ ] AC-17 — skill↔binary contract pinned by a shipped-content grep test

**Domain:** workflow (primary — `cmd/mindspec/panel.go`,
`cmd/mindspec/config.go`, skill text via `internal/setup`;
`internal/complete` isolation guard test) — per the spec's Impacted
Domains.

**Depends on**
None (W1; disjoint files from Beads 1 and 2). The
`cmd/mindspec/config.go` adjacency with Bead 3 is disjoint hunks
(Bead 4: `configShowReviewRoots` `:561`; Bead 3: the key-table
rendering `:204-215`) — a shared-file adjacency, NOT a dependency:
no state Bead 4 needs is produced by Bead 3, so no 3→4 (or 4→3) edge
is wired; whichever of the two merges second rebases the other's
disjoint hunk trivially. (bd edges wired exclusively from
`work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly ONE owning bead. AC-1 and AC-19 are
consciously owned by Bead 3 (the integration bead) even though their
RED-today trigger is Bead 1's code — their assertion sets require
Bead 3's R6c/R7c checks to exist (see the preamble re-cut); Bead 1's
verification carries the lane-scoped smoke of the same trigger. R10's
regression pinning is distributed per-bead (every RED-today leg in
each bead's verification), with its shared-helper anti-drift half
owned by Bead 1 as AC-4; R9 is owned by Bead 3 via AC-18.

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (greenfield first-run, governed-lane doctor scoping, designed Warns present) | Bead 3 Step 7 (trigger fixed by Bead 1 Steps 1–3; Warn checks from Bead 3 Step 2) | Bead 3 verification: AC-1 fixture (RED today) + Bead 1's lane-scoped smoke |
| AC-2 (legacy partial-state convergence) | Bead 1 Steps 1–2, 7 | Bead 1 verification: AC-2 fixture (RED today), byte-compares |
| AC-2b (missing-standard-file backfill) | Bead 1 Steps 2, 7 | Bead 1 verification: AC-2b fixture (RED today) |
| AC-3 (doctor context-map checks i/ii/iii) | Bead 1 Steps 6(a), 7 | Bead 1 verification: AC-3 subtests (i/ii RED today) |
| AC-4 (shared "mapped" helper anti-drift) | Bead 1 Steps 1, 6(a), 7 | Bead 1 verification: seam-identity pin |
| AC-5 (gitignore append on init) | Bead 1 Steps 4, 7 | Bead 1 verification: AC-5 fixture (RED today), byte-idempotent |
| AC-6 (gitignore ensure on all three setup verbs) | Bead 1 Steps 5, 7 | Bead 1 verification: per-verb subtests (RED today) |
| AC-7 (doctor ignore-ness check + tracked guard) | Bead 1 Steps 6(b), 7 | Bead 1 verification: AC-7 (i RED today; iii guard) |
| AC-8 (slugged creation, --slug, bare fallback) | Bead 2 Steps 1, 5 | Bead 2 verification: AC-8 subtests (RED today) |
| AC-9 (canonical ID + exact-join resolution + show guard) | Bead 2 Steps 2–3, 5 | Bead 2 verification: AC-9 (i/ii RED today; guard leg) |
| AC-10 (mixed-directory guard + collision error) | Bead 2 Steps 3–5 | Bead 2 verification: AC-10 (collision RED today; numbering guard) |
| AC-11 (models parity stack) | Bead 3 Steps 1–3, 6 | Bead 3 verification: AC-11 (RED today), three-state pins |
| AC-12 (schema-block honesty) | Bead 3 Steps 2, 6 | Bead 3 verification: honesty text pin |
| AC-13 (no framework leak, both verbs, omitted-when-unset) | Bead 3 Steps 4, 6 | Bead 3 verification: AC-13 greps (RED today) |
| AC-14 (declared build guidance round-trips, both verbs) | Bead 3 Steps 1, 4, 6 | Bead 3 verification: AC-14 (RED today), outside-marker bytes |
| AC-14b (already-onboarded leaked-block consumer healed) | Bead 3 Steps 4, 6 | Bead 3 verification: AC-14b fixture (RED today) |
| AC-15 (ad-hoc panel create + tally + flag contract) | Bead 4 Steps 1–3, 6 | Bead 4 verification: verbatim repro (RED today) + refusal legs |
| AC-16 (gate isolation — ad-hoc never gates) | Bead 4 Step 4 | Bead 4 verification: byte-identical gate evaluation (guard) |
| AC-17 (skill↔binary contract grep) | Bead 4 Step 5 | Bead 4 verification: shipped-content grep (RED today) |
| AC-18 (ADR-0040 amendment anchors + citations) | Bead 3 Step 5 | Bead 3 verification: rg anchors + citation-site assertions |
| AC-19 (full cross-verb first-run E2E) | Bead 3 Step 7 (consuming Bead 1's R1–R4 code and Bead 2's R5 code — the two declared edges) | Bead 3 verification: AC-19 fixture (RED today) |
