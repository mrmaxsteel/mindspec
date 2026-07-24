---
status: Draft
spec_id: 126-panel-review-discipline
version: "1"
adr_citations:
  - ADR-0044
  - ADR-0043
  - ADR-0040
  - ADR-0037
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
      - plugins/mindspec/skills/ms-bead-fix/SKILL.md
      - plugins/mindspec/skills/ms-spec-final-review/SKILL.md
      - .mindspec/adr/ADR-0044-panel-review-conduct.md
      - internal/setup/review_discipline_test.go
  - id: 2
    depends_on: []
    key_file_paths:
      - internal/setup/claude.go
      - internal/setup/lifecycle_gate_step_test.go
      - internal/panel/create.go
      - internal/panel/create_test.go
      - internal/config/config.go
      - internal/config/commands_test.go
  - id: 3
    depends_on:
      - 1
      - 2
    key_file_paths:
      - plugins/mindspec/skills/ms-panel-run/SKILL.md
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - plugins/mindspec/skills/ms-spec-final-review/SKILL.md
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
      - internal/setup/review_discipline_test.go
  - id: 4
    depends_on:
      - 3
    key_file_paths:
      - plugins/mindspec/skills/ms-spec-final-review/SKILL.md
      - internal/setup/historical_skills/ms-panel-run.pre126.md
      - internal/setup/historical_skills/ms-panel-tally.pre126.md
      - internal/setup/historical_skills/ms-bead-cycle.pre126.md
      - internal/setup/historical_skills/ms-bead-fix.pre126.md
      - internal/setup/historical_skills/ms-spec-final-review.pre126.md
      - internal/setup/historical_skills/ms-spec-approve.pre126.md
      - internal/setup/historical_skills/ms-plan-approve.pre126.md
      - internal/setup/upgrade_refresh_pre126_test.go
      - internal/setup/review_discipline_test.go
---
# Plan: 126-panel-review-discipline

Four beads implement the panel/review-discipline spec. The decomposition
is driven by ONE dominating constraint: the edits are concentrated in a
few SHARED files — `ms-panel-run/SKILL.md` alone is touched by six
requirements (R1, R2, R3d, R4b/c, R5, R6, R7), and `ms-spec-final-review`
by five (R1, R2, R6, R7, R8b). Two beads editing the same SKILL.md
concurrently is the spec-125 `landed.go` shared-file lesson (and the 117
false-independence lesson before it), so the plan cuts along
REQUIREMENT-COHERENT clusters and pays for the shared files with explicit
serialization instead of splitting any requirement's skill text across
beads. Four beads, not three or five, because: (a) the reviewer-conduct
cluster (R2/R3/R6/R7) is one coherent doctrine authored against one new
ADR and should be reviewed as one thing; (b) the three Go product-surface
edits (lifecycle literals, `briefStubBody`, `commandOrder`) share NO
files with the skill beads and form a genuinely parallel second Wave-1
bead with its own in-package [CI] pins — collapsing it into a skill bead
would be false coupling; (c) the panel-size/document-gate/BRIEF cluster
(R1/R4/R5-skill-half) re-edits the same three skills as bead 1 and must
serialize anyway; (d) delivery reach (snapshots + upgrade test) is only
MEANINGFUL after every skill edit has landed (below), so it anchors the
tail. A fifth bead would either split a requirement across skill files or
manufacture a test-only bead; three would merge the conduct doctrine into
the panel-size rewrite and produce one review-resistant mega-diff.

**Dependency graph (acyclic), waves, and serial depth.** Edges: `1→3`,
`2→3`, `3→4`. Waves: W1 = {1, 2} (parallel — zero shared files), W2 =
{3}, W3 = {4}. Longest serial chain: 3 (`1→3→4`, `2→3→4`) — at the
heuristic ceiling, and every link is a genuine seam, not file adjacency:

- **Bead 3 depends on Bead 1** (shared files + shared artifact): Bead 3
  re-edits `ms-panel-run`, `ms-panel-tally`, and `ms-spec-final-review`,
  all three of which Bead 1 edits first; and Bead 3 APPENDS rows to the
  table-driven guard test file Bead 1 CREATES
  (`internal/setup/review_discipline_test.go`). Bead 3's document-gate
  lens table and R1 rewrite also cite ADR-0044, which Bead 1 authors.
- **Bead 3 depends on Bead 2** (assertion-target dependency): Bead 3
  appends the AC-6 and literal-AC-13 rows asserting fragments INSIDE the
  `ms-spec-approve`/`ms-plan-approve` literals — those fragments exist
  only after Bead 2's `claude.go` edit lands. No file is co-edited
  (Bead 3 touches no Go source), but the rows would be RED without
  Bead 2.
- **Bead 4 depends on Bead 3** (shared file + a semantic precondition):
  Bead 4 edits `ms-spec-final-review` (R8b), serialized behind Beads 1
  and 3's edits to the same file; and AC-16's Refreshed assertion is
  only non-vacuous once the canonical skills have DIVERGED from the
  pre-126 snapshots — `installSkills` classifies a seeded body
  `Refreshed` via `matchesShipped` (`internal/setup/skills.go:130-137`,
  match-snapshot ≠ match-canonical) only when canonical ≠ snapshot,
  which requires Beads 1–3 landed. Bead 4 also completes the guard
  table (AC-11 rows, AC-12 completeness).

**Shared-SKILL.md serialization edges, stated explicitly** (the true
EDIT set per bead; `key_file_paths` above is authoritative):

| File | Bead 1 | Bead 2 | Bead 3 | Bead 4 |
|:-----|:------:|:------:|:------:|:------:|
| `ms-panel-run/SKILL.md` | R2, R3d, R6, R7 + sweep | — | R1a/b, R4b/c, R5 | — |
| `ms-panel-tally/SKILL.md` | R2, R3b/c | — | R1a | — |
| `ms-bead-cycle/SKILL.md` | R3a, R7 | — | — | — |
| `ms-bead-fix/SKILL.md` | R7 | — | — | — |
| `ms-spec-final-review/SKILL.md` | R2, R6, R7 | — | R1a | R8b |
| `internal/setup/claude.go` (literals + CLAUDE.md skills-table template) | — | R4a + R1 residual (F1-3 fold) | — | — |
| `internal/setup/review_discipline_test.go` | creates | — | appends | appends |

So: `ms-panel-run` ∈ {1, 3}, `ms-panel-tally` ∈ {1, 3},
`ms-spec-final-review` ∈ {1, 3, 4}, and the guard-test file ∈ {1, 3, 4}
— all strictly sequential under `1→3→4`. W1's two beads co-edit NOTHING
(Bead 1: five SKILL.mds + a new ADR + a new test file; Bead 2:
`claude.go`, `internal/panel/*`, `internal/config/*` — Bead 2's own
[CI] pin lives in a SEPARATE new file,
`internal/setup/lifecycle_gate_step_test.go`, so the two beads still
co-edit nothing). One W1 cross-check, stated ACCURATELY: because W1 is
parallel, Bead 1's AC-10 scan runs over the BASE lifecycle-literal
values — it cannot see Bead 2's edits, and there is NO guarantee about
which W1 bead merges second or what its `go test` gate saw. The real
contract is: (a) Bead 1 scans the base literals; (b) Bead 2's literal
additions (the `--gate …` panel step, the two ADR-0044 citations, the
portable enforcement sentence, and the CLAUDE.md-template reword)
introduce no scannable pattern —
verified by inspection at Bead 2's own gate, and Bead 2's dispatch
prompt forbids introducing any; (c) **Bead 3 (depends_on: 1, 2) is the
first bead whose `go test` gate is GUARANTEED to run the AC-10 scan
over the full union** of Bead 1's skill bytes and Bead 2's literal
values — any scannable pattern Bead 2 had introduced REDs there, before
anything downstream consumes it.

**Snapshot-timing decision (AC-16), settled: snapshots land in Bead 4
(LAST), captured from the BASE ref — never from a sibling bead's edited
tree.** The seven `internal/setup/historical_skills/<name>.pre126.md`
files must be the BYTE-EXACT bodies a pre-126 install carries. Capture
rule: for the five plugin skills,
`git show $(git merge-base origin/main HEAD):plugins/mindspec/skills/<name>/SKILL.md`
(base = `7ec96295` at plan time) — byte-exact by construction and
IMMUNE to bead ordering, so no early "snapshot bead" is needed; for the
two lifecycle literals (`ms-spec-approve`, `ms-plan-approve`), the
snapshot is the exact bytes the BASE binary installs — captured by
building the base ref and running `mindspec setup claude` into a
throwaway dir (the on-disk artifact is precisely what
`previouslyShippedSkills` will be compared against), cross-checked
against the literal bodies in `git show <base>:internal/setup/claude.go`
(the `ms-spec-approve.pre0uur.md` precedent). The
dedicated-early-snapshot-bead alternative was REJECTED: base-ref capture
is equally byte-exact whenever it runs, but an early bead's AC-16
upgrade test would be vacuous (canonical == snapshot →
`installSkills` classifies `Skipped`, never `Refreshed`,
`skills.go:129-137`), forcing the test to split away from its fixtures —
and the leftover would be a near-trivial asset-only bead.

**Snapshot PROVENANCE is pinned by digest, not inferred from history
(plan-gate correction).** The seed/Refreshed legs alone cannot prove the
snapshots were captured correctly: `installSkills` trusts whatever
`.pre126.md` bytes sit in `historical_skills/` — a WRONGLY-captured
snapshot still seeds, still `matchesShipped` (the bad fixture is itself
in the shipped-history set by construction), and still classifies
`Refreshed`. The earlier draft's claim that "a wrong snapshot matches NO
shipped revision → REDs" was FALSE and is withdrawn. Therefore the [CI]
AC-16 test (`upgrade_refresh_pre126_test.go`) **pins a reviewed SHA-256
digest for each of the seven base artifacts and asserts each embedded
`.pre126.md` body hashes to its pinned digest BEFORE seeding** — an
external oracle the fixture cannot satisfy by merely existing. Digest
provenance, recorded here as the proof-of-record:

- Base commit: **`7ec96295`** (= `git merge-base origin/main HEAD` at
  plan time).
- Five plugin skills — capture command
  `git show 7ec96295:plugins/mindspec/skills/<name>/SKILL.md | shasum -a 256`,
  reviewed values (computed at plan time; the test pins these):
  - `ms-panel-run` `1ae112ef731ee1b489dac7226b73684d5357678a913900fe27e45a1d2afbb11f`
  - `ms-panel-tally` `687dc90613f067b88bc7e3c1b7c490f93a8a99993904bb4489ae55b9c197354a`
  - `ms-bead-cycle` `679b580e33412870fe5449d0ceef8240a4ed5cbbebbd41722bf142ce41796ba8`
  - `ms-bead-fix` `d481f5297960b45f53404ddba3a4ad69be975fb205bb85746f7c9fed005aaf8e`
  - `ms-spec-final-review` `26c09c05b9d083495d54de1f0b99393c822037bb6026dd68a2194a45829a9ce3`
- Two lifecycle literals (`ms-spec-approve`, `ms-plan-approve`) —
  capture: build the base binary from a detached `7ec96295` worktree,
  run `mindspec setup claude` into a throwaway dir, `shasum -a 256` the
  two installed SKILL.md files (the on-disk artifact is precisely what
  `previouslyShippedSkills` compares against), CROSS-CHECKED against the
  literal bodies in `git show 7ec96295:internal/setup/claude.go` (the
  `ms-spec-approve.pre0uur.md` precedent). Their digests are computed at
  Bead 4 capture time by this recorded procedure and pinned in the same
  test; the cross-check evidence lands in Bead 4's review record.

The one-time `git show <base>:<path> | diff -` rows in Bead 4's
verification remain Validation Proofs; the digest assertion is the
standing [CI] guard. The Refreshed, canonical-output-equality,
delete-one-snapshot, and user-modified-preservation legs all stay.

**The pinned-fragment table (AC legend's pre-edit-absence rule), proofs
recorded.** Every positive [CI] fragment below was grepped at the parent
commit (`6ad5478d`, whose skill/Go bytes equal base `7ec96295` — no
bead has touched them) across `plugins/mindspec/skills/`,
`internal/setup/claude.go`, and `internal/panel/create.go`: **no match**
for every row marked ABSENT. The spec's BANNED known-present tokens are
avoided throughout: `reviewed_head_sha` (present in all five skills),
bare `verbatim` (present in panel-run/tally/bead-cycle), bare
`absolute path` (present at `ms-bead-fix:15`).

| AC | Pinned fragment (positive unless noted) | Target surface(s) | Absent at parent? |
|:---|:---|:---|:---|
| AC-1 | `shipped default: 6 reviewers` | ms-panel-run, ms-panel-tally | ABSENT ✓ |
| AC-1 | `n-1` (ASCII; existing text uses `N − 1`/`N−1` unicode forms only) | ms-panel-run, ms-panel-tally | ABSENT ✓ |
| AC-2 | (structural: parse the fenced `panel.gates` example; no grep fragment) | ms-panel-run | n/a (example block itself is new) |
| AC-3 | `Amendment (Spec 126)` marker heading | ADR-0043 | ABSENT ✓ (`grep -n 'Amendment' ADR-0043-*` empty) |
| AC-4 | `git worktree add --detach` | ms-panel-run, ms-spec-final-review | ABSENT ✓ |
| AC-4 | `re-verify it yourself in a fresh detached checkout` | ms-panel-tally | ABSENT ✓ |
| AC-5 | `verify, never confirmed` | ms-bead-cycle, ms-panel-tally | ABSENT ✓ |
| AC-5 | negative: `trust the empirical check` must VANISH | ms-bead-cycle | present at `:135` today ✓ (real removal) |
| AC-6 | `--gate spec_approve` / `--gate plan_approve` | claude.go literals | ABSENT ✓ (`grep -n '\-\-gate' claude.go` empty) |
| AC-7 | `additive, never a replacement` | ms-panel-run | ABSENT ✓ |
| AC-7 | negative: `Don't paste the plan; summarise it.` must VANISH | ms-panel-run | present at `:78` today ✓ |
| AC-8 | `findings-never-out-voted` | ms-panel-run, ms-spec-final-review | ABSENT from all skills ✓ (present only in ADR-0043 — not a target file) |
| AC-9 | `never a relative path` AND `harness cwd-resets` | ms-panel-run, ms-bead-fix, ms-spec-final-review | ABSENT ✓ |
| AC-9 | `git status --porcelain` | ms-bead-cycle | ABSENT ✓ |
| AC-9 | negative: `/Users/Max` must VANISH | ms-panel-run (`:195`) | present today ✓ |
| AC-11 | `commands.ci` | ms-spec-final-review | ABSENT ✓ |
| AC-11 | negative tripwire patterns `go test`, `npm test`, `pytest`, `make test` stay absent | ms-spec-final-review | already absent ✓ (tripwire, not a proof of the edit) |
| AC-13 | `ADR-0044` | all 5 plugin skills + both literals + `.mindspec/adr/ADR-0044-*.md` exists w/ Accepted | ABSENT everywhere ✓; no ADR-0044 file exists (next free number ✓) |
| AC-15 | `## Acceptance Criteria (verbatim from spec.md)` | internal/panel `briefStubBody` | ABSENT ✓ |
| AC-17 | (behavioral: render ORDER, not presence — no fragment by design) | internal/config | n/a |

**AC-10 scan inventory at base + per-hit dispositions (the R7 sweep,
executed by Bead 1; the implementer finalizes the table from the test's
own scan output).** Scan SURFACE, stated precisely: the scan iterates
`pluginmindspec.SkillFiles()` embedded bytes AND the four
`lifecycleSkillFiles()` **map VALUES** — it must NEVER grep the
`claude.go` SOURCE text, which carries `spec-072` in Go comments at
`:370`/`:437`/`:448`, all OUTSIDE the literal values and none of them
shipped skill text. Class (i) model values: ZERO hits across that
surface (verified — the product text is already family-agnostic), so
the class-(i) allowlist starts EMPTY and the mandated negative cases
(`gpt-4o`, `o4-mini`, `/root/agent/…`) carry the
categorical-enforcement burden. Class (ii) lore markers
(`go test -short`, `argv-ratchet`, `internal/harness`,
`internal/instruct`, …): zero hits. Class (iii) home paths: exactly one
— `ms-panel-run:195` `/Users/Max/.codex/memories` → **GENERALIZE** to
`$HOME/.codex/memories` (mandated by AC-9; never allowlisted). Class
(iv) `spec-[0-9]+`: six locators, EIGHT occurrences — VERIFIED at base
`7ec96295` via `git show 7ec96295:<path> | grep -noE 'spec-[0-9]+'`:
`ms-panel-run:28` carries TWO (`spec-050-bead2` and `spec-050-bead2-r2`
on the one Inputs example line) and `ms-panel-tally:88` carries TWO
(both on the one postmortem line); the other four locators carry one
each.

**The allowlist is OCCURRENCE-ACCOUNTED, not membership-based
(plan-gate tightening).** Every retained entry carries (file, stable
locator — the containing line's anchor text, robust to line-number
drift — the EXACT matched text/token, plus expected occurrence COUNT
within that file), and the filed reason; the matched-text identity is
part of the entry KEY, as spec.md:491-510 requires — an entry
authorizes only ITS token at ITS locator, never "any hit here". The
scan consumes allowlist entries ONE-TO-ONE against scan
hits — matched text AND count must both agree — and FAILS on BOTH
residuals: (a) any scan hit not matched by an
entry — so an extra `spec-050` beyond its entry's pinned count in an
already-allowlisted file
REDs, and (b) any allowlist entry left UNUSED — so a stale entry after
a future cleanup REDs and gets deleted rather than lingering as a
silent hole. Dispositions:

| Hit | Matched text | Expected count | Disposition | Filed reason |
|:---|:---|:---:|:---|:---|
| `ms-panel-run:189` retry note | `spec-050` (in `lola spec-050`) | 0 (removed) | **GENERALIZE** (R3d rewrites this substitution material anyway): "quota-tripped CLI slots stay tripped on immediate retry — account quotas refresh on a clock, not per-process" | incident ID adds nothing portable |
| `ms-panel-run:28` example-slug line | `spec-050` ×2 (inside `spec-050-bead2` and `spec-050-bead2-r2`) | 2 | **ALLOWLIST** | intentionally retained neutral example slugs — the round-1 AND round-2 forms on the one Inputs line (AC-10's own worked example) |
| `ms-bead-cycle:27` bd-vs-plan | `spec-050` (in `lola spec-050`) | 1 | **ALLOWLIST** | deliberately kept cross-project case history (per R7's spec text; not a mindspec incident ID) |
| `ms-panel-tally:88` (lola-f4a8 $417 postmortem) | `spec-050` ×2 | 2 | **ALLOWLIST** | cross-project provenance powering the artifact-gate HARD-block rationale; both occurrences on the one postmortem line |
| `ms-spec-final-review:73` fix commits | `spec-050` (in `lola spec-050`) | 1 | **ALLOWLIST** | cross-project provenance for the escape-hatch legitimacy rule |
| `ms-bead-impl:10` case history | `spec-050` (in `lola spec-050`) | 1 | **ALLOWLIST** | pre-existing case history in a skill this spec does NOT edit but which the AC-10 scan COVERS (the scan runs over ALL embedded skills; spec: ms-bead-impl expected unchanged — hence it still needs its own entry) |

Retained allowlist total: 7 occurrences across five locators (the 8
base occurrences minus the one GENERALIZED away at `ms-panel-run:189`);
every count above re-verified at plan-fix time with
`git show 7ec96295:<path> | grep -noE 'spec-[0-9]+'`.

(The spec's Non-Goal bans THIS project's incident IDs; the retained
`lola`/`spec-050` citations are another project's case histories, which
R7 explicitly admits as per-hit allowlist-with-reason decisions.) The
negative cases prove FIVE distinct failure paths RED: `gpt-4o`,
`o4-mini`, `/root/agent/…` (categorical classes), a fixture that
DUPLICATES an already-allowlisted token in the same file (e.g. a 3rd
`spec-050` in the tally bytes) proving the occurrence accounting
bites, and a COUNT-PRESERVING SUBSTITUTION fixture: replace ONE
allowed `spec-050` occurrence at an allowlisted locator with `gpt-4o`
in a test-bytes copy — the locator's total hit count unchanged — and
the scan MUST still RED, proving a disallowed class can never consume
an allowed locator/count slot (this is exactly what the entry's
matched-text key buys).

**Model tiering (standing protocol):** Sonnet implements by default,
escalate to Fable. **Beads 1 and 3 should go to (or escalate early to)
Fable** — Bead 1 authors the ADR-0044 normative doctrine plus five
role-local skill fragments under the AC-14 no-duplication boundary and
makes the AC-10 allowlist judgment calls; Bead 3 rewrites the
panel-size story across three skills plus the ADR-0043 amendment
without changing any pinned default. Beads 2 and 4 are Sonnet-suitable
(string-constant edits + mechanical snapshot capture, each with sharp
in-package tests).

**Dogfood note:** every bead touches only workflow-owned
(`plugins/mindspec/**`, `internal/setup/**`, `internal/panel/**`,
`.mindspec/adr/**`) and core-owned (`internal/config/**`) paths under a
spec whose Impacted Domains cover both; each bead's own
`mindspec complete` must pass the divergence gate with ZERO
`--override-adr`. Reviewer/fixer prompts for these beads must ALREADY
practice what the beads preach: absolute `/tmp` scratch only, and the
orchestrator verifies the bead worktree is clean before every
`mindspec complete` — this spec is the codification of exactly those
rules, and its own panels are the first live demonstration.

**Delivery housekeeping (orchestrator close-out, not a bead):** the
eight origin concern-beads map to chunks as: `yt96`→Bead 3 (R1),
`0skg`→Bead 1 (R2), `8hvn`→Bead 1 (R3), `s05u`→Beads 2+3 (R4 —
PARTIAL: guidance half only; the mechanization half stays open as
`mindspec-0pij` per the spec), `ljfz`→Beads 2+3 (R5), `f8ra`→Bead 1
(R6), `fr0o`→Bead 1 (R7), `oxnr`→Beads 2+4 (R8). Close each origin
bead (except `s05u`, which gets a comment pointing at `0pij` for its
open half) when its owning chunk merges; do NOT close them from inside
bead subagents.

## ADR Fitness

- **NEW ADR-0044 "panel-review-conduct" — authored by Bead 1, landing
  Accepted via the repo's house `- **Status**: Accepted` bullet (the
  form every ADR in `.mindspec/adr/` uses — e.g. ADR-0043:4 — and the
  form the [CI] Accepted-marker row greps; NOT `status:` YAML
  frontmatter, which no repo ADR carries)** (spec-settled:
  impl-approve's divergence lane errors on Proposed-only citations). It is the single home for the
  FULL normative reviewer-conduct doctrine (mutation-isolation,
  verify-never-confirm, reviewer-facing findings-never-out-voted,
  absolute-scratch, CI-parity kernel) AND the incident provenance
  (spec-116 Bead3a contamination, spec-119 false-red claims, spec-121
  BRIEF summarization gap, the 2026-07-08 cwd-reset corruption, the
  PR #217 local-green ≠ CI-green case). The doctrine/citation boundary
  (AC-14) is enforced editorially per bead: each touched skill carries
  ONLY its role-local operational sentence + an `ADR-0044` citation —
  never another role's rules, never an incident ID. Authored in the
  conduct bead (not a separate docs bead) because that is where four of
  its five doctrine sections gain their skill-side fragments in the
  same diff — the reviewer sees doctrine and point-of-use text
  together. Beads 2/3/4 add citations only, never normative text.
- **ADR-0043 — amended by Bead 3 (R1/AC-3), the only existing-ADR
  edit.** Context lines 15-17 ("spec/plan ≈ 9–12 slots, bead = 8,
  final = 12") gain an `## Amendment (Spec 126): shipped default vs
  operator ladder` block (the 125 ADR-0041 heading convention; this
  heading IS the AC-1/AC-3 grep marker) distinguishing the SHIPPED
  6 / `n-1` config-resolved default from the operator-scaled ladder,
  which becomes a documented `panel.gates` EXAMPLE.
- **ADR-0040 — unchanged, applied.** The layering ratchet is WHY
  R2/R3/R4/R6/R7/R8 ship as skill/ADR text; R4's approve verbs stay
  un-gated (L4 staging; mechanization = `mindspec-0pij`). No bead adds
  binary enforcement.
- **ADR-0037 — unchanged, cited.** N−1 single-home, refutation
  procedure, and the trust boundary are referenced by the R6 teaching
  text; gate semantics untouched (guarded by the existing
  `internal/panel` suites staying green).

No ADR is superseded; no divergence requiring a human stop.

## Testing Strategy

- **Everything is pure and CI-parity-hermetic (spec-119: bd-less
  `go test -short` lane, NO skip-gating).** The five new/extended test
  surfaces execute NO subprocess, NO git, NO bd:
  - the R1c consistency/fragment/scan table
    (`internal/setup/review_discipline_test.go`) reads
    `pluginmindspec.SkillFiles()` (embedded bytes,
    `plugins/mindspec/embed.go:25`), `lifecycleSkillFiles()`
    (in-package), `config.DefaultConfig()` (pure), and the two ADR
    files via `os.ReadFile` of the repo-tracked
    `.mindspec/adr/ADR-0043-*.md`/`ADR-0044-*.md` (path resolved by
    walking up from the package dir to `go.mod` — plain FS on files
    present in every CI checkout; no git);
  - Bead 2's self-pin (`internal/setup/lifecycle_gate_step_test.go`)
    reads `lifecycleSkillFiles()` values in-package — pure string
    assertions, same class as the R1c table;
  - the AC-16 upgrade test drives `installSkills` against `t.TempDir()`
    (the exact `skills_test.go:236-244` seeding precedent), after
    asserting each embedded snapshot body against its pinned SHA-256
    digest (`crypto/sha256`, stdlib, no subprocess);
  - AC-15 asserts on the in-package `briefStubBody` constant;
  - AC-17 calls the exported `RenderBuildTestSection`
    (`internal/config/config.go:320`) on a literal fixture.
  None of them may skip under any environment; there is nothing to
  skip FOR.
- **The lint ratchet is NOT implicated — verified, not assumed.** This
  spec adds ZERO exec/git argv sites: the only non-test Go edits are a
  raw-string literal (`claude.go`), a string constant (`create.go:26`),
  a slice-literal entry + comment (`config.go:233`/`:80`); the new
  tests use `os`/`filepath`/`strings` only. `internal/lint`'s
  argv-ratchet tables are untouched and `go test ./internal/lint/`
  green is part of every bead's gate.
- **Row-lands-with-edit discipline for the R1c table.** The table file
  is CREATED by Bead 1 (its rows: AC-4/5/8/9 fragments, the
  occurrence-accounted AC-10 scan-plus-allowlist with its five
  negative cases — three categorical + the duplicate-token occurrence
  negative + the count-preserving substitution negative, the five
  plugin AC-13 citation rows, the
  ADR-0044-Accepted bullet-marker row) and
  APPENDED by Bead 3 (AC-1/2/6/7 rows incl. the NEGATIVE structural
  fixed-six sweep guard, ADR-0043 marker row, the two literal AC-13
  rows) and Bead 4 (AC-11 rows incl. the four negative
  tripwire patterns; AC-12 completeness sweep). The file is co-edited
  ONLY along the strictly-serial `1→3→4` chain, and every skill AND
  literal edit gains its RED pin in the SAME bead that makes the edit —
  no bead lands unpinned doc changes: Beads 1/3/4 pin in the R1c table,
  and Bead 2 SELF-PINS its lifecycle-literal edits in its own separate
  in-package file `internal/setup/lifecycle_gate_step_test.go` (a
  sibling file, NOT the Bead-1-created table, so W1's zero-shared-files
  parallelism is preserved). Table shape: one row per pinned fragment
  (AC-12's per-fragment traceability), each row naming its AC id, the
  surface (embedded-skill key or literal key or ADR path), and the
  fragment; a file-granular revert of `ms-panel-run` REDding many rows
  is expected and compliant.
- **RED-on-revert demonstrations (per bead, 125-style):**
  `go test ./internal/setup/ -run 'ReviewDiscipline|UpgradeRefresh'` →
  PASS; `git stash` one skill/literal/snapshot edit → matching row(s)
  FAIL → unstash → PASS. For AC-17 the revert leg is the
  `commandOrder` `"ci"` entry (order flips after `aa` → FAIL); for
  AC-15 the `create.go:26` heading; for AC-16, deleting any one
  `.pre126.md` REDs its seed row.
- **Guards that must NOT change:** `internal/panel` gate/tally suites
  (decision logic untouched), `renderBriefHeader` output
  (`create.go:186-208` — AC-15 asserts the stub constant only),
  `DefaultConfig` panel VALUES (pinned by AC-1, per the spec-112 note
  at `config.go:459-462`), existing `internal/setup` skill-refresh
  suites, existing `internal/config` command-render tests, and the
  `internal/redact` suites (no core token/grammar change).
- **Integration gates (every bead):** `go build ./...`,
  `go test -short ./...` (no new red), `go vet ./...`, `gofmt -l`
  clean, `golangci-lint run ./...`,
  `mindspec validate spec 126-panel-review-discipline`, bead worktree
  CLEAN (`git status --porcelain` empty) before `mindspec complete`,
  zero `--override-adr`. The spec's Validation Proofs are distributed:
  fragment greps + stash demos per owning bead; the throwaway-repo
  fresh-install and upgrade proofs at Bead 4; the
  `mindspec config show` zero-config panel check at Bead 3.

## Bead 1: Reviewer-conduct doctrine — ADR-0044 (Accepted) + the five-skill conduct edits + the guard-table scaffold

R2, R3, R6, R7 in full, plus ADR-0044 and the creation of the R1c
guard-test file with this bead's rows. All five plugin skills are
edited HERE for their conduct fragments (and each gains its one-line
ADR-0044 citation); no other W1 work touches a SKILL.md.

**Steps**
1. Author `.mindspec/adr/ADR-0044-panel-review-conduct.md`, landing
   **Accepted**: the five normative sections (mutation-isolation,
   verify-never-confirm — family-symmetric with the codex rows as the
   documented instance, reviewer-facing findings-never-out-voted,
   absolute-scratch, CI-parity kernel) plus the incident provenance
   (spec-116 Bead3a, spec-119 beads 4/5/6, spec-121 Bead-2, the
   2026-07-08 cwd-reset corruption, spec-119/PR #217). Cross-reference
   ADR-0037 (gate mechanics stay there) and ADR-0040 (why this is
   skill-layer). Incident IDs live ONLY here (AC-14 boundary).
2. `ms-panel-run/SKILL.md` (R2, R3d, R6, R7): new "Reviewer conduct"
   section — mutation-isolation (`git worktree add --detach
   /tmp/rev-<panel-slug>-<slot> <reviewed_head_sha>` or `git archive`;
   read-only inspection of the shared tree stays fine; remove the
   worktree when done), with the portable failure-mode motivation
   ("reviewers that mutate a shared checkout contaminate each other's
   probes — one slot's deleted escape becomes another slot's false
   REQUEST_CHANGES") and a note on the empirical-prober row of the
   Slot-lens table (`:206-217`); the reviewer-facing "how your verdict
   is adjudicated" statement (`findings-never-out-voted`,
   fixed-or-evidence-refuted via the audited refutations procedure,
   threshold-is-a-floor — citing `gate.go:299` semantics by behavior,
   ADR-0043:17 by name) in the reviewer-prompt composition guidance;
   the security-classifier same-slot substitution fallback beside the
   existing quota-sub convention (`:187` area), and the GENERALIZED
   retry note (drop `lola spec-050`, keep the portable lesson); the
   blanket absolute-scratch rule generalizing the codex-only `/tmp`
   plumbing (`:97-201`) — `never a relative path`, `harness
   cwd-resets` rationale — and `:195` reworded to
   `$HOME/.codex/memories`; one-line ADR-0044 citation.
3. `ms-panel-tally/SKILL.md` (R2 orchestrator-half, R3b/c): Step 2
   gains the contested-finding rule — before acting on an empirical
   claim another slot or the fix author disputes,
   `re-verify it yourself in a fresh detached checkout` at the
   reviewed SHA; the reproduction (or its failure) IS the refutation
   `evidence`; every BLOCKING finding from a non-interactive CLI
   reviewer must be reproduced in isolation before fix-dispatch or
   refutation (`verify, never confirmed`); extend the
   confidence/single-dissent weighting (`:127-128`) with
   never-out-vote-by-count-in-either-direction / weight the deeper
   code trace; ADR-0044 citation.
4. `ms-bead-cycle/SKILL.md` (R3a, R7): REWRITE the `:135` asymmetry
   row — delete `trust the empirical check`; the codex
   REQUEST_CHANGES is a `verify, never confirmed`: reproduce the claim
   (test-execution AND code-reasoning findings alike) in an isolated
   checkout before treating it as confirmed, carrying BOTH field
   directions portably (CLI reviewers catch real bugs interactive
   reviewers miss AND false-flag clean-checkout-red / hollow-fixture
   claims that a clean reproduction refutes). Adjacent to the merge
   terminal (`:116`/`:146`): verify the bead worktree is CLEAN
   (`git status --porcelain` empty) before every `mindspec complete`
   and REMOVE strays rather than letting them be committed — stated as
   the advisory complement of the mechanized dirty-tree Block (gate
   leg 7). ADR-0044 citation.
5. `ms-bead-fix/SKILL.md` (R7): the generalized absolute-scratch
   mandate for fix authors (same fragments); ADR-0044 citation.
6. `ms-spec-final-review/SKILL.md` (R2, R6, R7): F2-row
   mutation-isolation instruction (`git worktree add --detach`), the
   reviewer-facing adjudication statement
   (`findings-never-out-voted`), the absolute-scratch rule; ADR-0044
   citation. (R8b's CI-parity slot text is Bead 4's, NOT here — one
   concern per edit pass.)
7. CREATE `internal/setup/review_discipline_test.go`: the table-driven
   guard over `pluginmindspec.SkillFiles()` + `lifecycleSkillFiles()`
   + `config.DefaultConfig()` + repo-ADR reads. This bead's rows:
   AC-4 (both fragments), AC-5 (positive fragments + the
   `trust the empirical check` MUST-NOT-CONTAIN row + the
   substitution-fallback fragment), AC-8, AC-9 (all four fragments +
   the `/Users/Max` MUST-NOT-CONTAIN row), AC-13 partial (five plugin
   `ADR-0044` rows + the ADR-0044-file-exists row asserting the repo's
   real `- **Status**: Accepted` bullet form — the marker every ADR in
   `.mindspec/adr/` actually uses (e.g. ADR-0043:4), NOT a `status:`
   frontmatter key, which no repo ADR has),
   and the FULL AC-10 scan-plus-allowlist: the four pattern classes
   (categorical model regex, lore markers, POSIX home paths,
   `spec-[0-9]+`) over ALL embedded plugin skills AND the four
   `lifecycleSkillFiles()` map VALUES — never the `claude.go` SOURCE
   text (its Go comments carry `spec-072` at `:370`/`:437`/`:448`,
   outside the literal values). The allowlist is OCCURRENCE-ACCOUNTED
   exactly as dispositioned above: each entry = (file, stable locator
   anchor text, EXACT matched text/token, expected occurrence count,
   filed reason — the matched-text key is spec-mandated,
   spec.md:491-510); entries are
   consumed ONE-TO-ONE against scan hits, matched text and count both
   agreeing; the scan FAILS on any
   unmatched hit AND on any unused entry. Negative cases: the three
   categorical fixtures proving `gpt-4o`, `o4-mini`, and
   `/root/agent/…` each RED the scan, PLUS the occurrence-negative
   fixture duplicating an already-allowlisted token in the same file
   (a 3rd `spec-050` in a tally-bytes fixture copy) proving the scan
   REDs on the count overrun, PLUS the count-preserving substitution
   fixture (ONE allowed `spec-050` at an allowlisted locator replaced
   with `gpt-4o` in a fixture copy — count unchanged) proving a
   disallowed class cannot consume an allowed locator/count slot.

**Verification**
- [ ] `go test -short ./internal/setup/ -run ReviewDiscipline` PASS; `git stash` any one skill edit → its row(s) FAIL → unstash → PASS (AC-12 discipline for this bead's rows)
- [ ] `grep -n 'trust the empirical check' plugins/mindspec/skills/ms-bead-cycle/SKILL.md` → no match; `grep -rn '/Users/Max' plugins/mindspec/skills/` → no match; `grep -rn 'ADR-0044' plugins/mindspec/skills/` → 5 files
- [ ] AC-10 negative cases demonstrably RED — all FIVE: inject `gpt-4o`, `o4-mini`, `/root/agent/…`, a duplicate of an allowlisted token (3rd `spec-050` in a tally fixture copy), AND the count-preserving substitution (ONE allowed `spec-050` at an allowlisted locator replaced with `gpt-4o` — count unchanged, scan still REDs) inside test fixture copies, never the real files; allowlist rows each carry file + locator + exact matched text + expected count + reason; an intentionally-unused extra allowlist entry also REDs (unused-entry leg demonstrated)
- [ ] ADR-0044 exists with the `- **Status**: Accepted` bullet (repo ADR house form); incident IDs appear in ADR-0044 ONLY (`grep -rn 'spec-116\|spec-119\|spec-121' plugins/mindspec/skills/` → no match)
- [ ] AC-14 editorial check recorded in review evidence: each skill fragment is role-local, no doctrine restatement across skills
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 126-…`; worktree clean before `mindspec complete`; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-4 — mutation-isolation fragments in panel-run/final-review + tally re-verify fragment (fragments proven absent at parent)
- [ ] AC-5 — verify-never-confirm replaces the trust-the-empirical-check row; substitution fallback documented
- [ ] AC-8 — reviewer-facing findings-never-out-voted statement in panel-run + final-review
- [ ] AC-9 — absolute-scratch mandate (all reviewers/fixers), clean-worktree check in bead-cycle, `/Users/Max` gone
- [ ] AC-10 — occurrence-accounted, matched-text-keyed scan-plus-allowlist guard live over embedded skills + `lifecycleSkillFiles()` VALUES; three categorical negatives + the duplicate-token occurrence negative + the count-preserving substitution negative all RED
- [ ] AC-14 **[doc]** — ADR-0044 carries full doctrine + provenance; skills carry role-local fragments + citations only

**Depends on**
None (W1 root; parallel with Bead 2 — zero shared files).
(Human-readable narration only — bd edges are wired exclusively from
`work_chunks[].depends_on`.)

## Bead 2: Binary/literal product surfaces — lifecycle-literal panel step, BRIEF stub heading, `commands.ci` vocabulary

R4a, R5's `briefStubBody` half (AC-15), R8a (AC-17), plus the folded
R1-residual reword of the CLAUDE.md skills-table template (Step 1,
F1-3). Pure Go edits: two literal surfaces in `claude.go` (lifecycle
skills + the CLAUDE.md template), one string constant, one slice
entry + comment — each with its own in-package [CI] pin landing in
THIS bead. No SKILL.md is touched.

**Steps**
1. `internal/setup/claude.go` — the `ms-spec-approve` (`:808`) and
   `ms-plan-approve` (`:825`) literals in `lifecycleSkillFiles()`
   (`:784`) gain the pre-approve panel step: run `/ms-panel-run` with
   `mindspec panel create <slug> --spec <id> --gate spec_approve`
   (resp. `--gate plan_approve`) `--target <ref>` against the
   spec/plan DOCUMENT, and do not run `mindspec spec approve` /
   `mindspec plan approve` until `/ms-panel-tally` returns Allow —
   guidance text, honestly framed: the verbs are un-gated by design
   (ADR-0040 L4), and the SHIPPED literal text says only "binary
   enforcement of these gates is a separate, tracked enhancement" —
   PORTABLE phrasing, because a `mindspec-0pij` bead-ID reference
   inside the shipped literal would be a bead-ID leak that no AC-10
   scan class catches; the `mindspec-0pij` pointer lives in this plan,
   the spec, and the ADR only, NEVER in the literal. Point at
   `ms-panel-run`'s document-gate lens defaults (Bead 3 lands that
   table; the pointer is by section name, not a restatement — panel
   mechanics stay single-homed in `ms-panel-run` per R4e). One-line
   `ADR-0044` citation in each literal. NO new skill files. ALSO in
   this file — an adjudicated FOLD (panel finding F1-3): the CLAUDE.md
   skills-table template (`claude.go:918` at base; seeded to consumer
   `CLAUDE.md:30`) still reads "then launch 6 reviewers and collect
   verdicts" — a SHIPPED surface that would contradict R1's
   single-authority story if left behind. Reword it topology-neutral
   ("then launch the configured reviewer panel and collect verdicts").
   The TEXT edit lands HERE because `claude.go` is exclusively this
   bead's file (Bead 3 stays Go-source-free and W1 parallelism is
   preserved); the standing sweep-guard row over this template is
   Bead 3's (AC-1's owner), and Step 6's self-pin covers it until
   then.
2. `internal/panel/create.go:26` — `briefStubBody` gains the
   `## Acceptance Criteria (verbatim from spec.md)` heading with its
   `<!-- TODO(skill): paste each claimed Rn/ACn verbatim from spec.md -->`
   stub comment, positioned with the other section headings the skill
   fills in. String-constant-only; `renderBriefHeader`
   (`create.go:186-208`) untouched.
3. `internal/panel/create_test.go` (in-package `package panel` — the
   constant is unexported): AC-15 test asserting `briefStubBody`
   contains the heading, plus a guard that `renderBriefHeader` output
   is unchanged for a fixed input (golden-ish string compare against
   the current rendering).
4. `internal/config/config.go` — `Commands` field comment (`:80` block)
   documents the `ci` vocabulary key beside `build`/`test` with the
   `commands.test`-fallback semantics; `commandOrder` (`:233`) becomes
   `[]string{"build", "test", "ci"}`. NO schema change, NO
   `DefaultConfig` change.
5. `internal/config/commands_test.go` — the AC-17 render-order test:
   `RenderBuildTestSection` (`:320`) on a fixture declaring `build`,
   `test`, `ci`, and `aa`; assert rendered line order is exactly
   build, test, ci, aa (vocabulary position — the sorted-rest fallback
   alone would place `aa` before `ci`, so reverting the `commandOrder`
   entry REDs). Plus the AC-11 config-half pin (plan picks the
   `internal/config` home the spec offers): a source-level assertion
   that the `Commands` comment names `ci` (read the field's doc via a
   grep-style check over the package source file, same technique as
   existing comment pins — keeping the R1c table skill-only).
6. NEW `internal/setup/lifecycle_gate_step_test.go` — Bead 2's OWN
   [CI] pin for the literal edits, in a SEPARATE file (NOT the
   Bead-1-created `review_discipline_test.go`, so W1's
   zero-shared-files parallelism holds): assert against the
   `lifecycleSkillFiles()` map VALUES that the `ms-spec-approve` value
   contains `--gate spec_approve`, the `ms-plan-approve` value
   contains `--gate plan_approve`, both contain the
   not-before-tally-Allow instruction and an `ADR-0044` citation, and
   NEITHER contains `mindspec-0pij` (the bead-ID-leak
   MUST-NOT-CONTAIN row backing Step 1's portable phrasing). Pure
   in-package string assertions, no subprocess. Reverting any Step-1
   literal edit REDs HERE, in this bead — Bead 3's later R1c rows
   re-assert the same fragments as part of the table's completeness,
   which is redundant coverage, not the primary pin. The same file
   also self-pins the Step-1 CLAUDE.md-template reword: the template
   value must NOT match the case-insensitive separator-tolerant
   `6[- ]reviewers?` pattern (Bead 3's AC-1 sweep guard later
   re-covers this surface as the standing row).

**Verification**
- [ ] `go test -short ./internal/panel/ -run BriefStub` PASS; revert `create.go:26` heading → FAIL (AC-15 RED-on-revert)
- [ ] `go test -short ./internal/config/` PASS; revert the `commandOrder` `"ci"` entry → order test FAIL (AC-17 RED-on-revert); `grep -c 'Acceptance Criteria (verbatim from spec.md)' internal/panel/create.go` ≥ 1
- [ ] `go test -short ./internal/setup/ -run LifecycleGateStep` PASS; `git stash` the `claude.go` literal edits → the self-pin FAILs → unstash → PASS (AC-6 literal-half RED-on-revert lands in THIS bead)
- [ ] `grep -n '\-\-gate spec_approve' internal/setup/claude.go` and `--gate plan_approve` each ≥ 1 (fragment absent at parent — recorded above); `grep -c 'mindspec-0pij' internal/setup/claude.go` → 0; `grep -icE '6[- ]reviewers?' internal/setup/claude.go` → 0 (CLAUDE.md-template reword landed; RED-on-revert via the Step-6 self-pin); existing `internal/setup` suites green (literal edits keep frontmatter + `managed-by` marker intact)
- [ ] No exec/argv site added (`internal/lint` suites untouched and green); `renderBriefHeader` golden guard green
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 126-…`; worktree clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-15 — `briefStubBody` heading, in-package test, `renderBriefHeader` unchanged (RED on constant revert)
- [ ] AC-17 — `ci` renders in vocabulary position before lexically-earlier extension keys; presence alone cannot pass (RED on `commandOrder` revert)
- [ ] AC-6 literal-half SELF-PIN — `lifecycle_gate_step_test.go` REDs on any Step-1 literal revert and on a `mindspec-0pij` leak; AC-6 OWNERSHIP stays with Bead 3 (lens-table half + R1c rows); the same file self-pins the CLAUDE.md-template reword (`6[- ]reviewers?` MUST-NOT-MATCH — AC-1 sweep OWNERSHIP stays with Bead 3)

**Depends on**
None (W1; parallel with Bead 1 — Bead 1 edits SKILL.mds + a new test
file, this bead edits `claude.go`/`internal/panel`/`internal/config`).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 3: One authoritative panel-size story + document-gate lens doctrine + verbatim-AC BRIEF

R1 in full (incl. the ADR-0043 amendment), R4b/c (the document-gate
lens tables + escalation guidance in `ms-panel-run`), and R5's skill
half. Re-edits the three shared skills AFTER Bead 1 and appends this
cluster's rows to the guard table.

**Steps**
1. `ms-panel-run/SKILL.md` (R1a/b) — **FULL-INVENTORY sweep, not just
   the headline sentences (plan-gate expansion).** The R1 rewrite must
   remove EVERY fixed-six/R1–R6/3+3 EXECUTION instruction, not only
   the named fragment lines — otherwise the skill states "the
   configured mix is the single authority" while its operating steps
   still hard-code the topology. Inventory at parent, each rewritten
   to DERIVE from the configured mix:
   - `:3` (description "launch 6 reviewers (3 Claude Agents + 3 Codex
     CLI sessions)") and `:8` ("fan out three `Agent` calls … and
     three `codex exec` … Wait for all six") → launch one reviewer
     per configured slot, per-family dispatch derived from the mix;
     wait for ALL configured slots.
   - `:6` (the H1 heading `# Run a 6-Reviewer Panel`) — named by
     spec.md:181's explicit line list but MISSED by this inventory's
     first draft (panel finding G1-1-R2) → derive from the configured
     mix or go topology-neutral (e.g. `# Run a Review Panel`).
   - `:32` (parenthetical "(6 reviewers, N−1 threshold)") → becomes
     the ONE canonical labelled sentence: `shipped default: 6
     reviewers` — 3 `claude` + 3 `codex` — threshold `n-1`.
   - `:97` (`/tmp/codex_<panel-slug>_r{4,5,6}.md`) → one pre-staged
     prompt file per configured codex-family slot (`r<slot>` from the
     mix, not a literal 4–6 range).
   - `:108` ("For each codex slot R4, R5, R6") and `:133` ("For each
     claude slot R1, R2, R3") → "for each configured codex-family /
     claude-family slot"; slot ids come from the configured mix.
   - `:143` ("Verify all six JSON files exist") → verify ONE verdict
     JSON per configured reviewer (the expected-file list is derived
     from `panel.json`'s `expected_reviewers`, never a literal six).
   - `:208-217` (the 6-row Slot-lens table + "six distinct lenses,
     not six clones") → the table is KEPT concrete but relabelled the
     **DEFAULT lens assignment for the shipped 6-slot mix**, with one
     sentence on lens assignment when the configured mix differs
     (assign each configured slot a distinct lens, reusing/splitting
     this default set); "six distinct lenses" → "N distinct lenses,
     not clones".
   - `:224` ("Don't run all six reviewers as foreground tool calls")
     → "all reviewers".
   After the sweep, `6` / `3+3` / `R1–R3`+`R4–R6` topology appears
   ONLY in (a) the explicitly-labelled shipped-DEFAULT sentence and
   (b) the ladder example — a fresh install still reads a fully
   concrete 6-reviewer default flow; what is removed is the
   CONTRADICTION, not the concreteness. Add the commented, fenced
   `panel.gates` ladder EXAMPLE (bead 8, spec/plan ≈ 9, final 12),
   explicitly labelled operator preference, reviewer entries carrying
   ONLY `family` + `count` keys (the AC-2 structural contract — no
   `model:` key anywhere in the example).
2. `ms-panel-run/SKILL.md` (R4b/c): document-review lens defaults
   table for the `spec_approve`/`plan_approve` gates (falsifiability
   of ACs, scope/domain honesty, contradiction hunting,
   feasibility/blast-radius) parallel to the bead Slot-lens table;
   escalation guidance — a high-blast-radius document scales the mix
   via `panel.gates`, family-level only.
3. `ms-panel-run/SKILL.md` (R5): step 0.2 — REPLACE
   `Don't paste the plan; summarise it.` (`:78`) with: the BRIEF MUST
   paste each Rn/ACn the work claims to satisfy VERBATIM from
   `spec.md` (byte-exact, with its id); the plain-English summary is
   `additive, never a replacement` (aligning with
   `ms-bead-impl:51,155`). Assign one slot's lens the AC-provenance
   duty: trace every claimed AC clause to a landed+passing test
   against the SPEC text, not the BRIEF summary. Machine-managed
   header untouched (Bead 2 already landed the stub heading).
4. `ms-panel-tally/SKILL.md` (R1a) — same FULL-INVENTORY rule:
   `:26` and `:125` rephrased to the configured-mix +
   canonical-default form (same `shipped default: 6 reviewers`
   sentence), PLUS the execution instructions the first draft missed:
   `:69-72` (the report template's "APPROVEs among R1–R3 claude vs
   R4–R6 codex" and the `<claude>/3 claude, <codex>/3 codex`
   denominators) → family split reported per configured family with
   denominators DERIVED from the mix (`<claude>/<claude-slots>`,
   `<codex>/<codex-slots>`; slot sets from `panel.json`, not a
   hard-coded R1–R3/R4–R6 partition); `:126` ("Six verdicts × ~3
   items each = ~18 lines") → "N verdicts × ~3 items each" with the
   arithmetic derived, not literal.
5. `ms-spec-final-review/SKILL.md` (R1a) — same FULL-INVENTORY rule:
   `:40`'s "`expected_reviewers` 6" rephrased to the configured mix
   (final-review commonly operator-scaled via `panel.gates` —
   family-level example pointer, no model values), PLUS `:44`'s
   "**Fan out 6 reviewers with the FINAL-REVIEW lenses**" → fan out
   one reviewer per configured slot with the final-review lens set;
   the F1–F6 lens table (`:46-55`) is KEPT concrete but relabelled
   the DEFAULT lens assignment for the shipped mix, with the same
   one-sentence assignment rule for scaled mixes as panel-run's
   Slot-lens table.
6. `.mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md`:
   append the `## Amendment (Spec 126): shipped default vs operator
   ladder` block — context lines 15-17's "9–12 / 8 / 12" identified as
   the ORIGIN PROJECT's operator-scaled configuration; the product
   ships 6 / `n-1` (config-resolved); the ladder is a documented
   `panel.gates` example. The heading is the AC-1/AC-3 marker
   fragment.
7. Append to `internal/setup/review_discipline_test.go`: AC-1 rows
   (DefaultConfig structural check — slots sum to 6 over exactly
   {claude, codex}, 3+3, `ApproveThreshold == "n-1"`; the
   `shipped default: 6 reviewers` + ASCII `n-1` fragments in the
   embedded panel-run + tally bytes; the ADR-0043 amendment-marker
   row); **the NEGATIVE/structural AC-1 sweep guard (plan-gate
   addition)**: scan the embedded bytes of the three R1 skills
   (panel-run, tally, final-review) for residual fixed-six-topology
   patterns presented as execution instructions — the pattern set
   covers at least `all six`, `six reviewers`, a SEPARATOR-TOLERANT
   case-insensitive `6[- ]reviewers?` (which subsumes `6 reviewers`
   AND catches the hyphenated `6-Reviewer` heading form at
   `ms-panel-run:6` — G1-1-R2),
   `six distinct`, `Six verdicts`, `r{4,5,6}`, `R1, R2, R3` /
   `R4, R5, R6` slot enumerations, `R1–R3`/`R4–R6` (both hyphen and
   en-dash) family partitions, and `/3 claude`-style hard-coded
   family denominators — with an explicit ALLOWLIST of exactly the
   labelled shipped-DEFAULT sentence(s), the DEFAULT-labelled lens
   tables, and the fenced ladder example; any hit outside the
   allowlist REDs, so reverting ANY part of the Step 1/4/5 sweep is
   RED, not just the headline fragments; the same pattern set ALSO
   runs over the in-package CLAUDE.md skills-table template in
   `claude.go` (the `/ms-panel-run` row Bead 2 rewords — same
   `package setup`, direct constant access; NO allowlist entries for
   that surface); the AC-2 STRUCTURAL check (extract the fenced example from
   the embedded panel-run bytes, strip comment markers, YAML-parse;
   assert gates {bead, spec_approve, final_review} present and every
   reviewer entry carries ONLY {family, count} — any other key,
   `model:` in particular, fails); AC-6 rows (literal fragments
   `--gate spec_approve`/`--gate plan_approve` + the
   not-before-tally-Allow instruction + the panel-run document-gate
   lens-table marker); AC-7 rows (positive `additive, never a
   replacement` + MUST-NOT-CONTAIN `Don't paste the plan; summarise
   it.` + the AC-provenance duty fragment); the two literal AC-13
   citation rows (completing the seven-surface set).

**Verification**
- [ ] `go test -short ./internal/setup/ -run ReviewDiscipline` PASS; stash the ADR-0043 amendment alone → AC-1 marker row FAIL (all-surfaces-agree demonstrated); stash a panel-run edit → its rows FAIL
- [ ] NEGATIVE sweep guard demonstrated: stash the Step-1 rewrite of any ONE inventoried execution instruction (e.g. the `:143` all-six verification line) → the structural AC-1 sweep row REDs; the allowlisted default sentence + lens tables + ladder example do NOT trip it. Heading-form RED demo: restoring `# Run a 6-Reviewer Panel` in a fixture copy of the panel-run bytes REDs the `6[- ]reviewers?` pattern while the labelled `shipped default: 6 reviewers` sentence stays green; restoring "launch 6 reviewers" in the `claude.go` CLAUDE.md template likewise REDs
- [ ] `grep -n "Don't paste the plan" plugins/mindspec/skills/ms-panel-run/SKILL.md` → no match; `grep -c 'shipped default: 6 reviewers' plugins/mindspec/skills/ms-panel-run/SKILL.md` ≥ 1; `grep -inE 'all six|Six verdicts|r\{4,5,6\}|R4, R5, R6|6[- ]reviewers?' plugins/mindspec/skills/ms-panel-run/SKILL.md plugins/mindspec/skills/ms-panel-tally/SKILL.md plugins/mindspec/skills/ms-spec-final-review/SKILL.md` → no match outside allowlisted default/example text (in particular the `:6` H1 heading no longer reads `6-Reviewer`)
- [ ] AC-2 structural: hand-inject a `model: opus` line into the example inside a test fixture copy → parser row REDs (demonstrated in-test, not on the real file)
- [ ] `mindspec config show | grep -A2 'panel:'` on a zero-config repo → 3+3 / `n-1` unchanged (R1d settled; DefaultConfig VALUES untouched in the diff)
- [ ] `grep -rn 'ADR-0044' plugins/mindspec/skills/ internal/setup/claude.go` → 5 skills + 2 literals (AC-13 complete)
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 126-…`; worktree clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-1 — three surfaces agree (config structural + skill fragments + ADR marker); any one alone REDs; PLUS the negative structural sweep guard — no residual fixed-six execution instruction outside the labelled-default/example allowlist, RED on any sweep revert; pattern set incl. the separator-tolerant case-insensitive `6[- ]reviewers?` (catching the `:6` heading form) and the claude.go CLAUDE.md-template surface
- [ ] AC-2 — ladder example structurally family+count-only, parsed not grepped
- [ ] AC-3 **[doc]** — ADR-0043 no longer presents the ladder as the product's shape
- [ ] AC-6 — literal panel-step fragments + document-gate lens table asserted (literal TEXT landed AND self-pinned by Bead 2 — split-property note below; this bead's rows complete the R1c table)
- [ ] AC-7 — verbatim-AC BRIEF rule replaces the summarise-only guidance; AC-provenance duty named
- [ ] AC-13 — all seven touched surfaces cite ADR-0044; ADR-0044 Accepted (citations WRITTEN by Beads 1/2 — this bead completes and owns the assertion)

**Depends on**
Beads 1 and 2 (re-edits three SKILL.mds after Bead 1 and appends to
Bead 1's test file; asserts fragments inside Bead 2's literals).
(bd edges wired from `work_chunks[].depends_on`.)

## Bead 4: Portable CI-parity slot + delivery reach — pre-126 snapshots and the upgrade-refresh proof

R8b, AC-16, and guard-table completion (AC-11 rows, AC-12). Runs last:
final-review is its third sequential editor, and the Refreshed
assertions are only non-vacuous once Beads 1–3 have diverged every
edited surface from its base bytes.

**Steps**
1. `ms-spec-final-review/SKILL.md` (R8b): the F2 full-regression slot
   is instructed to reproduce the project-declared CI invocation
   VERBATIM — `commands.ci`, falling back to `commands.test` — on the
   spec branch before sign-off, and to record the explicit advisory
   finding ("no declared CI invocation — CI parity not reproduced")
   when neither key is declared. Extend the retained "Not a CI
   substitute" line (`:62`) with the local-green ≠ CI-green rationale.
   ABSTRACT only: config keys, never a concrete invocation (no
   `go test`, no `bd` choreography — R8c).
2. Capture the seven byte-exact pre-126 snapshots into
   `internal/setup/historical_skills/`: `ms-panel-run.pre126.md`,
   `ms-panel-tally.pre126.md`, `ms-bead-cycle.pre126.md`,
   `ms-bead-fix.pre126.md`, `ms-spec-final-review.pre126.md` from
   `git show <base>:plugins/mindspec/skills/<name>/SKILL.md`
   (base = `git merge-base origin/main HEAD`, `7ec96295` at plan
   time); `ms-spec-approve.pre126.md` and `ms-plan-approve.pre126.md`
   as the exact bytes the BASE binary installs (base-build
   `mindspec setup claude` into a scratch dir, copy the installed
   SKILL.md files; cross-check against `git show
   <base>:internal/setup/claude.go`). The `<name>.<tag>.md` convention
   (`skills.go:26-36`) resolves them automatically — no code change
   needed for pickup.
3. NEW `internal/setup/upgrade_refresh_pre126_test.go` (AC-16),
   following the `skills_test.go:236-244` seeding precedent. **FIRST
   leg — the provenance-digest oracle (plan-gate correction):** for
   each of the seven snapshots, assert the embedded `.pre126.md` body
   hashes (SHA-256) to its PINNED digest — the five plugin-skill
   digests recorded in this plan's snapshot-provenance block
   (computed from base `7ec96295`), the two lifecycle-literal digests
   computed by the recorded base-binary install-capture procedure and
   pinned in the same test — BEFORE any seeding. This is what makes
   AC-16 non-vacuous: without it, a wrongly-captured snapshot passes
   every downstream leg, because `matchesShipped` trusts whatever
   bytes sit in `historical_skills/` (the bad fixture is itself in
   the shipped-history set). THEN the behavioral legs: for EACH
   of the seven edited skills, seed the `.pre126.md` bytes on disk in
   `t.TempDir()`, run `installSkills` with the current canonical
   `wanted` set, assert the file is classified **Refreshed**
   (`Result.Refreshed`) and its bytes now equal the new canonical;
   PLUS the user-modified-preservation leg (a body matching no shipped
   snapshot → left in place, `Result.Notices` HC-6 message). Deleting
   any one snapshot REDs TWICE: the digest assertion fails on the
   missing embedded body, and the seeded pre-126 bytes then match
   nothing shipped → classified user-modified, not Refreshed → the
   seed row REDs too. (Note the asymmetry the digest leg exists for:
   DELETION is caught by the behavioral legs, but a WRONGLY-CAPTURED
   snapshot is caught ONLY by the digest oracle.)
4. Append to `internal/setup/review_discipline_test.go`: AC-11 rows —
   the `commands.ci` fragment (+ fallback + no-declaration-advisory
   fragments) in the embedded final-review bytes, and the four
   negative tripwire rows (`go test`, `npm test`, `pytest`,
   `make test` MUST NOT appear in the final-review bytes; AC-10's
   `go test -short` row already backstops). AC-12 completeness sweep:
   confirm every AC-1/2/4/5/6/7/8/9/10/11 fragment is one traceable
   row; record the full stash-matrix demonstration.
5. Update the AC-10 allowlist ONLY if this bead's R8b text introduces
   a new scan hit (it must not — write it clean instead).

**Verification**
- [ ] `go test -short ./internal/setup/ -run 'ReviewDiscipline|UpgradeRefresh'` PASS; `git rm` any one `.pre126.md` → its digest row AND seed row FAIL → restore → PASS (AC-16 RED-on-revert); corrupt one byte of a snapshot in a scratch copy → digest row alone REDs (the wrong-capture case the behavioral legs cannot catch)
- [ ] Digest pins verified against the plan's snapshot-provenance block: the five plugin digests in the test byte-match the plan-recorded `7ec96295` values; the two literal digests match the install-capture, cross-checked against `git show 7ec96295:internal/setup/claude.go`
- [ ] Byte-exactness recorded (one-time Validation Proofs — the standing [CI] guard is the digest assertion, NOT these diffs): `git show <base>:plugins/mindspec/skills/<name>/SKILL.md | diff - internal/setup/historical_skills/<name>.pre126.md` → empty, ×5; literal-snapshot capture method + diff evidence recorded for the two lifecycle files
- [ ] Throwaway-repo upgrade proof (spec Validation Proofs): install PRE-126 bodies, run post-126 `mindspec setup claude` → each edited skill reported Refreshed and carrying the new fragments; a hand-edited body left in place with the HC-6 notice. Fresh-install proof: stock `mindspec setup claude` → `ms-spec-approve` carries the `--gate spec_approve` step; `ms-panel-run` carries the AC-4/7/8 fragments
- [ ] `grep -nE 'go test|npm test|pytest|make test' plugins/mindspec/skills/ms-spec-final-review/SKILL.md` → no match; `commands.ci` present
- [ ] `go build ./... && go test -short ./... && golangci-lint run ./...` clean; `mindspec validate spec 126-…`; worktree clean; zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-11 — named-slot CI-reproduction instruction with `commands.ci`/`commands.test` fallback + advisory; concrete-invocation tripwires green (config-comment half landed and pinned by Bead 2 — split-property note below)
- [ ] AC-12 — the R1c table complete: per-fragment rows for AC-1/2/4/5/6/7/8/9/10/11, plain `go test ./internal/setup/`
- [ ] AC-16 — seven byte-exact pre-126 snapshots, each pinned to a reviewed SHA-256 digest from base `7ec96295` (asserted before seeding) + Refreshed/user-modified upgrade proof; delete-one-snapshot REDs digest + seed rows; wrong-capture REDs the digest row

**Depends on**
Bead 3 (third sequential editor of `ms-spec-final-review` and of the
guard table; canonical-vs-snapshot divergence precondition for the
Refreshed assertions requires Beads 1–3 landed).
(bd edges wired from `work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly ONE owning bead. Four narrated
split-property notes, disjoint by construction (the 125 precedent):
AC-6's literal TEXT lands AND self-pins in Bead 2 (Steps 1, 6 —
`lifecycle_gate_step_test.go`) while Bead 3 OWNS the AC and asserts
both halves in the R1c table (literal fragments + lens table); AC-11's
`Commands`-comment half lands and is pinned in Bead 2 (Steps 4–5)
while Bead 4 OWNS the AC and lands the skill half + table rows; AC-13's
citations are WRITTEN where each surface is edited (Bead 1: five
plugins + ADR-0044; Bead 2: two literals, self-pinned in Step 6) while
Bead 3 OWNS the AC and lands the complete seven-surface assertion.
AC-12's table file is created by Bead 1, appended by Bead 3, and OWNED
(completeness) by Bead 4; and AC-1's CLAUDE.md-template residual (the
F1-3 fold) has its TEXT landed and self-pinned in Bead 2 (Steps 1, 6)
while Bead 3 OWNS the AC-1 sweep row that covers the template surface.

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (three-surface panel-size agreement + full-inventory sweep) | Bead 3 Steps 1, 4–7 (CLAUDE.md-template text: Bead 2 Step 1, self-pinned Bead 2 Step 6) | Bead 3: structural DefaultConfig + fragment + ADR-marker rows; single-surface stash REDs; NEGATIVE structural sweep guard (no fixed-six execution instruction outside the labelled-default/example allowlist; separator-tolerant case-insensitive `6[- ]reviewers?` catches the `:6` hyphenated heading; guard also runs over the claude.go CLAUDE.md skills-table template) |
| AC-2 (ladder example family+count-only, structural) | Bead 3 Steps 1, 7 | Bead 3: parse-the-example row + injected-`model:` negative |
| AC-3 [doc] (ADR-0043 amendment) | Bead 3 Step 6 | Bead 3: amendment block + marker grep |
| AC-4 (mutation-isolation fragments) | Bead 1 Steps 2, 3, 6, 7 | Bead 1: fragment rows, absence-at-parent recorded in plan table |
| AC-5 (verify-never-confirm; row rewritten) | Bead 1 Steps 3, 4, 7 | Bead 1: positive + MUST-NOT-CONTAIN rows |
| AC-6 (lifecycle-literal panel step + doc-gate lens table) | Bead 3 Steps 2, 7 (literal text: Bead 2 Step 1; self-pinned: Bead 2 Step 6) | Bead 2: `lifecycle_gate_step_test.go` RED-on-revert; Bead 3: literal-fragment + lens-table rows |
| AC-7 (BRIEF verbatim-AC rule) | Bead 3 Steps 3, 7 | Bead 3: positive + summarise-line-gone rows |
| AC-8 (findings-never-out-voted, reviewer-facing) | Bead 1 Steps 2, 6, 7 | Bead 1: fragment rows |
| AC-9 (absolute scratch + clean-worktree + `/Users/Max` gone) | Bead 1 Steps 2, 4, 5, 6, 7 | Bead 1: four positive rows + MUST-NOT-CONTAIN row |
| AC-10 (occurrence-accounted, matched-text-keyed scan-plus-allowlist over embedded skills + `lifecycleSkillFiles()` values) | Bead 1 Step 7 (sweep: Steps 2–6) | Bead 1: scan table (one-to-one entry consumption, matched text + count both agreeing; unmatched-hit AND unused-entry both RED) + three categorical negatives + the duplicate-allowlisted-token occurrence negative + the count-preserving substitution negative |
| AC-11 (CI-parity slot text + `ci` comment; tripwires) | Bead 4 Steps 1, 4 (comment half: Bead 2 Steps 4–5) | Bead 4: fragment + tripwire rows; Bead 2: config-source pin |
| AC-12 (table-driven R1c file, per-fragment rows) | Bead 4 Step 4 (created Bead 1 Step 7, extended Bead 3 Step 7) | Bead 4: completeness sweep + stash matrix |
| AC-13 (seven ADR-0044 citations + Accepted ADR) | Bead 3 Step 7 (citations: Bead 1 Steps 2–6, Bead 2 Step 1 self-pinned in Step 6) | Bead 3: seven grep rows + the `- **Status**: Accepted` bullet-marker row (repo ADR house form, not frontmatter) |
| AC-14 [doc] (doctrine/citation boundary) | Bead 1 Steps 1–6 | Bead 1: editorial check in review evidence |
| AC-15 (`briefStubBody` heading, in-package test) | Bead 2 Steps 2–3 | Bead 2: BriefStub test + revert demo + header-golden guard |
| AC-16 (digest-pinned byte-exact snapshots + Refreshed/user-modified proof) | Bead 4 Steps 2–3 | Bead 4: pinned SHA-256 digest assertions (base `7ec96295`) BEFORE seeding + seed rows + delete-one-REDs + wrong-capture-REDs; base-ref diffs + throwaway-repo proofs as Validation Proofs |
| AC-17 (`ci` render-order, not presence) | Bead 2 Steps 4–5 | Bead 2: order test + `commandOrder`-revert demo |

The spec's Validation Proofs distribute: fragment greps + stash demos
at each owning bead; `mindspec config show` zero-config check at
Bead 3; fresh-install and upgrade throwaway-repo proofs at Bead 4;
`go test ./...` green at every bead.

## Open Questions — RESOLVED (plan-gate round 1)

All six OQs were adjudicated at the plan gate; each confirms the
plan's position, recorded here as the decided record:

- [x] OQ-1 **RESOLVED — serialized single R1c guard file.** One
  table-driven `review_discipline_test.go`, created by Bead 1 and
  appended by Beads 3/4 along the strictly-serial `1→3→4` chain. (Not
  in tension with OQ-2's split: Bead 2's self-pin is a SIBLING file
  precisely because Bead 2 is not on that chain.)
- [x] OQ-2 **RESOLVED — keep W1 parallelism.** The AC-6/AC-13 split
  ownership stands; Bead 2 now SELF-PINS its literal edits in its own
  `internal/setup/lifecycle_gate_step_test.go` (Step 6), closing the
  unpinned-until-B3 window without collapsing the wave.
- [x] OQ-3 **RESOLVED — `git show <base>:` source extraction is the
  primary proof-of-record** for the two lifecycle-literal snapshots;
  the base-binary install-capture is the cross-check (and the byte
  source for what `previouslyShippedSkills` actually compares). Both
  feed the pinned digests in the AC-16 test.
- [x] OQ-4 **RESOLVED — in-process `t.TempDir()` fixture** (the
  `skills_test.go:230-298` precedent) is the [CI] test; the real
  two-binary end-to-end stays a recorded Bead-4 Validation Proof. No
  hermetic two-binary harness.
- [x] OQ-5 **RESOLVED — keep-and-allowlist**, WITH the
  occurrence-accounted allowlist (file + locator + expected count +
  reason; one-to-one consumption; unused entries RED) replacing the
  membership-based draft. The keep-vs-generalize line stands as
  dispositioned; `/Users/Max` is generalized, never allowlisted.
- [x] OQ-6 **RESOLVED — Bead 1 authors ADR-0044 (Accepted, house
  `- **Status**: Accepted` bullet) at bead time**; no plan-gate
  pre-draft round. Bead 1's dispatch BRIEF pastes AC-14 verbatim so
  the doctrine/citation boundary is in the author's and the panel's
  hands from the first line.
