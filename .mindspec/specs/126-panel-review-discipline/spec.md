---
approved_at: "2026-07-24T17:27:13Z"
approved_by: user
status: Approved
---
# Spec 126-panel-review-discipline: Panel / review-discipline hardening

## Goal

Promote the portable review-panel discipline that today lives only in this
project's operator memory into the shipped product — the skill files, the
lifecycle-skill literals, the ADR record, and the config-default story — so
that a fresh `mindspec setup <agent>` install carries the same panel
discipline the origin project practices. The binary's gate is already strict
(canonical verdict enum `internal/panel/tally.go:20-24`; any-unresolved-RC
Block, gate leg 9.5 at `internal/panel/gate.go:299`; audited refutation,
`gate.go` + ADR-0037 §7 amendment; the findings-never-out-voted doctrine named
in ADR-0043:17). The gap is in the SKILLS, DOCS, and CONFIG-DEFAULT story.
Everything this spec ships at the document gates (R4) is skill-layer
GUIDANCE a compliant agent follows — the approve verbs themselves remain
un-gated (ADR-0040 layering; mechanization is tracked separately as
`mindspec-0pij`). This spec closes exactly that gap for the eight filed
concerns — `mindspec-yt96`, `-0skg`, `-8hvn`, `-s05u`, `-ljfz`, `-f8ra`,
`-fr0o`, `-oxnr` — and nothing else.

## Background

**Origin.** A 2026-07-24 audit ("do local memories make mindspec behave
better than a fresh install?") compared the orchestrator's accumulated
review discipline against what the product ships. Field evidence: another
install freelances a non-canonical verdict vocabulary
(`approved/concerns/reject`) because the product's own story is inconsistent
at the point of use. See `SPEC-BRIEF-panel-review-discipline.md` (repo root)
and the eight beads above.

**Corrected audit finding (panel-size).** The audit brief claimed the binary
default reviewer mix is "empty (`config.go:45`)". That claim is STALE:
`internal/config/config.go:45` is the `SourceGlobs` empty-default comment;
the panel default is NOT empty. `DefaultConfig`
(`internal/config/config.go:432`; panel block at `config.go:452-457`) ships
`reviewers: [{family: claude, count: 3}, {family: codex, count: 3}]` with
`approve_threshold: "n-1"` — i.e. 6 reviewers, threshold 5. The real
inconsistency is therefore TWO-way, not three-way:

- Skills + shipped default say **6 / N−1**: `ms-panel-run/SKILL.md` lines 3,
  6, 8, 32, 143; `ms-panel-tally/SKILL.md:26` ("5-of-6 for the standard
  6-reviewer panel") and `:125`; `ms-spec-final-review/SKILL.md:40`
  ("`expected_reviewers` 6").
- ADR-0043:16-17 says "spec/plan ≈ 9–12 slots, bead = 8, final = 12" — which
  is this project's OPERATOR-scaled configuration (the model-tiering
  protocol), presented as if it were the product's shape.

R1 reconciles these into one authoritative story. Spec 112 (`kkcw`) shipped
the per-gate mixed-model CONFIG capability (`Panel.Gates`, `config.go:102`;
`PanelGateKeys` including `spec_approve`/`plan_approve`, `config.go:129`);
spec 113 R3 (`mindspec-zw81`) shipped the `mindspec panel create --gate
<name>` FLAG that stamps the decision-inert `gate` field at creation time.
This spec ships the defaults-and-docs residual only.

**Incident evidence for the conduct rules.** (Provenance below is spec-level
and ADR-0044-level record ONLY — shipped skill text describes each failure
MODE portably, never a mindspec incident ID; see the portability Non-Goal.)

- spec-116 Bead3a: mutation/empirical reviewers probing the SHARED bead
  worktree contaminated each other (one deletes an escape to probe it while
  another greps the same file) → 3 FALSE REQUEST_CHANGES (→ R2).
- spec-119 beads 4/5/6: three false codex "suite is red in clean checkout"
  claims, all refuted by reproducing in a clean detached checkout, plus 3
  wrong "hollow fixture" BLOCKINGs — yet `ms-bead-cycle/SKILL.md:135` ships
  only the dangerous half ("trust the empirical check") (→ R3).
- spec-121 Bead-2: a summarised BRIEF silently dropped an AC clause; the
  8-slot panel reviewed the summary, not the AC, and the gap sailed 8/8 —
  while `ms-bead-impl/SKILL.md:51,155` mandates verbatim quoting for impl
  prompts and `ms-panel-run/SKILL.md:78` says the opposite for the BRIEF
  ("Don't paste the plan; summarise it.") (→ R5).
- Reviewer relative-path scratch writes + harness cwd-reset corrupted
  SIBLING worktrees during the 2026-07-08 run; the gate's dirty-tree Block
  (leg 7, `gate.go` decision matrix comment at `gate.go:139-156`) catches
  the stray at complete time, but only AFTER the mess exists (→ R7).
- spec-119 + PR #217: local-green ≠ CI-green; no shipped skill tells any
  reviewer to reproduce the project's declared CI invocation (→ R8).

**Layering.** Per ADR-0040, review-conduct doctrine is skill-layer judgment;
the binary contributes mechanism only. This spec deliberately ships doctrine
as skill/ADR text and touches the binary only for (a) the R1 consistency
test, (b) the R5 `briefStubBody` heading (`internal/panel/create.go:26`,
string-constant edit plus its in-package guard test), (c) the R8
`commandOrder` vocabulary line (`config.go:233`) with its render-order guard
test, and (d) the pre-126 historical snapshots that make the skill edits
reach EXISTING installs (see Delivery reach). Gate/tally decision logic is
untouched.

**Delivery reach.** All five orchestration skills are compiled into the
binary via `go:embed` (`plugins/mindspec/embed.go:19`,
`pluginmindspec.SkillFiles()` at `embed.go:25`); the four lifecycle gate
skills are raw-string literals in `internal/setup/claude.go:784`
(`lifecycleSkillFiles`, `ms-spec-approve` at `:808`, `ms-plan-approve` at
`:825`). There is NO `refreshManagedSkill` function — that name is only a
stale comment at `claude.go:783`. The real upgrade mechanism is
`installSkills` (`internal/setup/skills.go:105-143`) +
`previouslyShippedSkills` (`skills.go:41`, embedded snapshots at
`skills.go:18` via `go:embed historical_skills/*.md`): on setup, an
already-installed skill is refreshed to canonical ONLY if its on-disk bytes
match the new canonical OR a byte-exact snapshot under
`internal/setup/historical_skills/`; anything else is classified
user-modified and LEFT IN PLACE (HC-6). Editing a shipped skill WITHOUT
capturing its pre-edit bytes as a snapshot therefore strands every existing
install on the old text. This spec accordingly ships a byte-exact pre-126
snapshot (`<name>.pre126.md`, per the `<name>.<tag>.md` convention
documented at `skills.go:26-36` and the spec-106 `*.pre106.md` precedent)
for EVERY shipped skill it edits — the 5 plugin skills AND the
`ms-spec-approve`/`ms-plan-approve` lifecycle literals — so a fresh install
and an upgraded install both receive every change in this spec (AC-16).

## Impacted Domains

- **workflow**: `internal/setup/**` — the `ms-spec-approve`/`ms-plan-approve`
  raw-string literals (R4), the new review-discipline consistency test
  beside the existing fragment-guard precedent in
  `internal/setup/skills_test.go` (pinned-fragment comment at `:482`, const
  block at `:483`), the pre-126 snapshots under
  `internal/setup/historical_skills/`, and the upgrade-refresh test (AC-16).
  `internal/panel/**` — the `briefStubBody` heading (`create.go:26`,
  R5/AC-15), a string-constant edit, plus its in-package guard test in
  `internal/panel/create_test.go` (the constant is unexported, so no other
  package can assert on it); `renderBriefHeader` (`create.go:186-208`), the
  gate (`gate.go`), and the tally (`tally.go`) are not modified.
  `plugins/mindspec/skills/*/SKILL.md` — the 5 edited plugin skills are
  ALSO workflow-owned: `.mindspec/domains/workflow/OWNERSHIP.yaml:23`
  declares `plugins/mindspec/**` (spec-106 flatten), so these edits carry
  the normal workflow-domain divergence obligations: `ms-panel-run`,
  `ms-panel-tally`, `ms-bead-cycle`, `ms-spec-final-review` (and
  `ms-bead-fix` for the R7 scratch mandate). `ms-bead-impl` is referenced
  (its verbatim-quote rule is the R5 precedent) but expected unchanged.
- **core**: `internal/config/**` — comment/vocabulary-level only: document
  the `ci` key in the `Commands` field comment (`config.go:80`), add
  `"ci"` to `commandOrder` (`config.go:233`), and add the render-order
  guard test against the exported `RenderBuildTestSection` (`config.go:320`)
  so the `commandOrder` entry is RED-on-revert (R8, AC-17; no schema
  change). `DefaultConfig`'s panel values (`config.go:452-457`) are PINNED
  by the R1 test, not changed — settled: zero-config installs keep 3+3 /
  `n-1`. **No `internal/redact` change**: this spec adds no CLI verb, no
  render surface, and no new core-owned token or grammar — verified against
  `internal/redact/` (redact.go + falsifiability/mutation/golden-corpus
  tests untouched).
Not impacted (no domain edits): `execution`, `context-system`; `internal/panel`
decision logic; the panel.json/verdict schema; `bd`.

## ADR Touchpoints

- [ADR-0037](../../adr/ADR-0037-panel-gate-enforced-contract.md): the N−1
  threshold single-home (§3), refutations (§7 amendment), and the trust
  boundary are UNCHANGED — R1/R6 cite it; nothing here alters gate
  semantics.
- [ADR-0043](../../adr/ADR-0043-panel-disposition-telemetry-store.md):
  context lines 16-17 receive a clarifying amendment distinguishing the
  SHIPPED default (6 / N−1, config-resolved) from the operator-scaled ladder
  (bead 8, spec/plan ≈ 9, final 12) that this project runs — the ladder
  becomes a documented `panel.gates` EXAMPLE, not the product default (R1).
- [ADR-0040](../../adr/ADR-0040-orchestration-layering-ratchet.md): the
  layering ratchet is the reason R2/R3/R4/R6/R7/R8 ship as skill/ADR text
  rather than binary enforcement; R4 explicitly leaves the approve verbs and
  `loop.gate_authority` wiring inert (L4 staging — mechanization is
  `mindspec-0pij`).
- **NEW ADR-0044 "panel-review-conduct" (lands Accepted, not Proposed —
  impl-approve's divergence lane errors on Proposed-only citations)**: the
  single home for the FULL normative reviewer-conduct doctrine
  (mutation-isolation, verify-never-confirm, reviewer-facing
  findings-never-out-voted, absolute-scratch, CI-parity kernel) and for the
  incident provenance behind it. Doctrine/citation boundary: ADR-0044
  carries the complete normative text; each touched skill carries ONLY its
  role-local operational fragment (the sentence a reviewer/orchestrator
  acts on at that point of use) plus a citation to ADR-0044 — never a
  restatement of another role's rules and never the incident IDs. ADR-0037
  stays gate-mechanics; ADR-0044 is conduct (ADR-0040 layering).

## Requirements

1. **R1 — One authoritative panel-size story, pinned defaults, consistency
guard** (bead `mindspec-yt96`).
The configured panel mix is the single authority. (a) Every skill sentence
that hard-codes "6" / "5-of-6" (`ms-panel-run` name/description/lines 3, 6,
8, 32, 143; `ms-panel-tally:26,125`; `ms-spec-final-review:40`) is rephrased
to derive from the configured mix, with the shipped default stated once in
canonical form: "the configured panel mix (shipped default: 6 reviewers — 3
`claude` + 3 `codex` — threshold `n-1`)". (b) The scale-by-gate-cost ladder
(bead 8, spec/plan ≈ 9, final 12) ships as a commented `panel.gates` EXAMPLE
in `ms-panel-run` (and the ADR-0043 amendment), explicitly labelled operator
preference, family-level only — reviewer entries carry ONLY `family` +
`count` keys, no model values (structurally enforced by AC-2). (c) A Go
consistency test (in `internal/setup`, beside the `skills_test.go:482-483`
fragment-guard precedent) asserts all three surfaces agree:
`config.DefaultConfig().Panel` sums to 6 reviewer slots across families
{claude, codex} with `ApproveThreshold == "n-1"`; the embedded
`ms-panel-run` + `ms-panel-tally` bytes (`pluginmindspec.SkillFiles()`)
contain the canonical default sentence; and `.mindspec/adr/ADR-0043-*.md`
contains the amendment marker. Changing any one surface REDs the test until
all agree. (d) `DefaultConfig` panel VALUES are unchanged, full stop
(zero-config installs keep their sizes, per the spec-112 note at
`config.go:459-462`); the scale-by-gate-cost ladder ships ONLY as the (b)
documented `panel.gates` example, labelled operator preference.

2. **R2 — Reviewer mutation-isolation** (bead `mindspec-0skg`).
`ms-panel-run` (a new "Reviewer conduct" section + a note on the
empirical-prober row of the Slot lens defaults table, `SKILL.md:206-217`)
and `ms-spec-final-review` (F2 row) instruct: any reviewer that EXECUTES or
MUTATES code (runs tests, deletes an escape to probe it, edits a fixture)
must do so in its OWN isolated checkout — `git worktree add --detach
/tmp/rev-<panel-slug>-<slot> <reviewed_head_sha>` (or `git archive`) —
never in the shared bead worktree; read-only inspection of the shared tree
remains fine; the reviewer removes its worktree when done. `ms-panel-tally`
Step 2 gains the orchestrator half: before acting on a CONTESTED finding
(an empirical claim another slot or the fix author disputes), re-verify it
yourself in a fresh detached checkout at the reviewed SHA; the reproduction
(or its failure) is the refutation `evidence`. The in-skill motivation is
stated PORTABLY (failure mode, no incident ID): "reviewers that mutate a
shared checkout contaminate each other's probes — one slot's deleted escape
becomes another slot's false REQUEST_CHANGES"; the spec-116 provenance
lives in ADR-0044 only.

3. **R3 — Verify-never-confirm for CLI-reviewer findings** (bead
`mindspec-8hvn`).
The doctrine is stated family-symmetric — an empirical finding from ANY
reviewer is a VERIFY, never a CONFIRMED — and the codex rows are its
documented instance. (a) `ms-bead-cycle`'s family-asymmetry table row at
`SKILL.md:135` ("trust the empirical check") is rewritten: treat the codex
REQUEST_CHANGES as verify-first — reproduce the claim (test-execution AND
code-reasoning findings alike) in an isolated checkout (R2 mechanics)
before treating it as confirmed; the shipped text carries BOTH directions
of the field evidence, phrased portably (CLI reviewers catch real bugs the
interactive reviewers miss, AND CLI reviewers false-flag "red suite in
clean checkout" / "hollow fixture" claims that a clean-checkout
reproduction refutes) — the spec-119 provenance lives in ADR-0044 only.
(b) Never out-vote by count in either direction; weight the reviewer that
did the deeper code trace (extends the existing `ms-panel-tally:127-128`
confidence/single-dissent rules). (c) `ms-panel-tally` requires every
BLOCKING finding from a non-interactive CLI reviewer to be reproduced in
isolation before fix-dispatch or refutation. (d) `ms-panel-run`'s
substitution material documents the security-classifier fallback: when a
reviewer family's safety classifier refuses an adversarial/security-framed
lens, re-dispatch the SAME slot with a correctness-framed persona (keep the
slot id, mark the substitution in `reviewer_id`, as the existing quota-sub
convention at `SKILL.md:187` already does).

4. **R4 — Document-gate panel doctrine: guidance, not enforcement** (bead
`mindspec-s05u`; GH #212, crosslinks #182/#181).
R4 ships the document-panel DOCTRINE — the lens tables and the invocation
step — NOT a gate: the approve verbs have NO binary preflight
(`cmd/mindspec/spec.go:111` calls `approve.ApproveSpec` and
`cmd/mindspec/plan_cmd.go:52` calls `approve.ApprovePlan` with no panel
check; the panel.json `gate` field is decision-inert), so a compliant agent
runs the document panel because the installed skill says to, and a
non-compliant agent CAN skip it — by design, per ADR-0040's L4 staging.
(a) The `ms-spec-approve` and `ms-plan-approve` literals
(`internal/setup/claude.go:808,:825`) gain a pre-approve panel step: run
`/ms-panel-run` with `mindspec panel create <slug> --spec <id> --gate
spec_approve|plan_approve --target <ref>` (per-gate config keys from spec
112 `PanelGateKeys`, `config.go:129`; the `--gate` flag itself from spec
113 R3) against the spec/plan DOCUMENT, and do not run `mindspec spec
approve` / `plan approve` until `/ms-panel-tally` returns Allow — as
instructions in the installed skill text. (b) `ms-panel-run` gains
document-review lens defaults for these gates (falsifiability of ACs,
scope/domain honesty, contradiction hunting, feasibility/blast-radius)
parallel to the existing bead table. (c) Escalation guidance: a
high-blast-radius document (multiple domains, security surface, public
contract) scales the mix via `panel.gates` — family-level guidance only.
(d) Enforcement is OUT OF SCOPE: mechanized gate-before-mutate on the
spec/plan approve verbs (and any `loop.gate_authority` wiring) is tracked
as **`mindspec-0pij`** (the L1 ratchet, sibling to spec 115's
complete-side ratchet). Bead `mindspec-s05u` is therefore PARTIALLY
delivered by this spec (the guidance half); the mechanization half is
`mindspec-0pij`. (e) NO new skill files: R4 ships entirely inside the
existing `ms-spec-approve`/`ms-plan-approve` lifecycle literals plus the
document-gate lens tables in `ms-panel-run` — panel mechanics stay
single-homed in `ms-panel-run`, and existing installs receive the edits
via the snapshot-refresh mechanism (Delivery reach, AC-16).

5. **R5 — BRIEF quotes acceptance criteria verbatim** (bead `mindspec-ljfz`).
`ms-panel-run` step 0.2 replaces the `SKILL.md:78` guidance ("Don't paste
the plan; summarise it.") with: the BRIEF MUST paste each Rn/ACn the work
claims to satisfy VERBATIM from `spec.md` (byte-exact, with its Rn/ACn id);
the plain-English summary is additive, never a replacement — aligning the
BRIEF with the impl-prompt rule (`ms-bead-impl:51,155`). One slot's lens
carries the AC-provenance duty: trace every claimed AC clause to a
landed+passing test against the SPEC text, not the BRIEF summary
(final-review F5 keeps its artifact-path half). This targets the
SKILL-authored BRIEF body; the machine-managed header (`renderBriefHeader`,
`create.go:186-208`) is unchanged. Additionally, `briefStubBody`
(`create.go:26`) gains an `## Acceptance Criteria (verbatim from spec.md)`
heading (with its `<!-- TODO(skill): ... -->` stub comment) — a
string-constant-only edit, guarded by AC-15 — so a fresh install's very
first BRIEF skeleton demands the verbatim ACs.

6. **R6 — Findings-never-out-voted, reviewer-facing** (bead `mindspec-f8ra`).
`ms-panel-run` (in the reviewer-prompt composition guidance and the R2
"Reviewer conduct" section) and `ms-spec-final-review` gain a short "how
your verdict is adjudicated" statement addressed TO reviewers: the doctrine
by name (findings-never-out-voted, ADR-0043:17); every finding is fixed or
evidence-refuted via the audited `refutations` procedure
(`ms-panel-tally:109-117`), never dropped because the APPROVE count cleared
the threshold; the threshold is a floor, not a license (the gate Blocks on
any unresolved REQUEST_CHANGES — `gate.go:299`, `ms-panel-tally:125`); a
"minor" finding still gets adjudicated. This is teaching text for what the
binary already enforces — no gate change.

7. **R7 — Absolute scratch for ALL reviewers/fixers + clean-worktree check +
portability sweep** (bead `mindspec-fr0o`).
`ms-panel-run`, `ms-bead-fix`, and `ms-spec-final-review` generalize the
existing codex-only `/tmp` plumbing (`ms-panel-run:97-201`) into a blanket
rule: every reviewer/fixer scratch file lives at an ABSOLUTE path under
`/tmp` (or, for verdicts, the panel directory) — never a relative path,
because harness cwd-resets turn relative writes into sibling-worktree
corruption, and stray files reach `mindspec complete`'s dirty-tree Block
(gate leg 7, `gate.go:139-156` matrix) only after the mess exists.
`ms-bead-cycle` gains, adjacent to its merge terminal (`SKILL.md:116,146`):
verify the bead worktree is CLEAN (`git status --porcelain` empty) before
every `mindspec complete`, and REMOVE strays rather than letting them be
committed — stated as the advisory complement of the mechanized leg-7
Block, not a replacement. As part of the same edit, the five skills are
SWEPT for operator-environment leaks: the pre-existing
`/Users/Max/.codex/memories` at `ms-panel-run:195` is generalized to
`$HOME/.codex/memories`, and every other absolute home path or
project-incident reference found by the AC-10 scan is either generalized
to a portable form or entered in the AC-10 allowlist table with a
one-line filed reason (e.g. the pre-existing `lola spec-050` / `lola-f4a8`
case-history citations are per-hit decisions for the plan: generalize the
prose or allowlist the intentional provenance).

8. **R8 — Portable CI-parity kernel** (bead `mindspec-oxnr`).
(a) The project declares its CI invocation via the existing free-form
`commands:` map (`config.go:80`, spec 123 R7b): `commands.ci` is the
documented vocabulary key beside `build`/`test`, with `commands.test` as
the fallback when `ci` is undeclared (Commands field comment + a `"ci"`
entry in `commandOrder`, `config.go:233`; NO schema change and no new
config field — the map already accepts any key, and populated entries
already render into the managed AGENTS.md "Build & Test" section via
`RenderBuildTestSection`, `config.go:320`). Because the sorted-rest path in
`CommandLines` already renders ANY declared key, the `commandOrder` entry
alone is not observable by output presence — its effect is ORDER (`ci`
renders in vocabulary position before lexically-sorted extension keys), so
it gets its own render-order guard test (AC-17).
(b) `ms-spec-final-review` instructs one designated slot (the F2
full-regression lens is the natural home) to reproduce the project-declared
CI invocation VERBATIM (`commands.ci`, falling back to `commands.test`) on
the spec branch before sign-off, and to record an explicit advisory finding
("no declared CI invocation — CI parity not reproduced") when neither key
is declared. The existing "Not a CI substitute" line
(`ms-spec-final-review:62`) is retained and extended with the local-green ≠
CI-green rationale. (c) NO mindspec-specific invocation ships in any skill
(no `go test -short`, no `bd`-off-PATH choreography) — the rule is
abstract; the value is per-project config.

## Scope

### In Scope
- `plugins/mindspec/skills/ms-panel-run/SKILL.md` (R1, R2, R3d, R5, R6, R7)
- `plugins/mindspec/skills/ms-panel-tally/SKILL.md` (R1, R2, R3b/c)
- `plugins/mindspec/skills/ms-bead-cycle/SKILL.md` (R3a/b, R7)
- `plugins/mindspec/skills/ms-bead-fix/SKILL.md` (R7)
- `plugins/mindspec/skills/ms-spec-final-review/SKILL.md` (R1, R2, R6, R7, R8b)
- `internal/setup/claude.go` — `lifecycleSkillFiles` literals for
  `ms-spec-approve`/`ms-plan-approve` (R4)
- `internal/setup/historical_skills/` — byte-exact pre-126 snapshots
  (`<name>.pre126.md`) of EVERY shipped skill this spec edits: all 5 plugin
  skills above PLUS `ms-spec-approve` and `ms-plan-approve`, so
  `installSkills`/`previouslyShippedSkills` refresh existing installs
  (Delivery reach; AC-16)
- `internal/setup/` — new consistency/fragment-guard test (R1c, AC-12) AND
  the upgrade-refresh test (AC-16)
- `internal/config/config.go` — `Commands` comment + `commandOrder` `"ci"`
  entry (R8a) + the `RenderBuildTestSection` render-order guard test
  (AC-17); NO `DefaultConfig` value change (settled — zero-config installs
  keep 3+3 / `n-1`)
- `internal/panel/create.go` — `briefStubBody` gains the
  `## Acceptance Criteria (verbatim from spec.md)` heading (R5), plus the
  AC-15 guard test in `internal/panel/create_test.go`
- `.mindspec/adr/ADR-0043-*.md` amendment; new
  `ADR-0044-panel-review-conduct` (lands Accepted)

### Out of Scope
- `internal/panel/gate.go`, `tally.go` decision logic; the panel.json /
  verdict-file schema; `renderBriefHeader`
- `DefaultConfig` panel values (settled: unchanged — pinned by the R1c test)
- New skill directories (no `ms-spec-gate`/`ms-plan-gate`; R4 lives in the
  lifecycle literals + `ms-panel-run`)
- **Mechanized enforcement of the document-gate panels** — a binary
  preflight on `mindspec spec approve` / `plan approve` (gate-before-mutate)
  and any `loop.gate_authority` wiring stay inert per ADR-0040 L4; tracked
  as **`mindspec-0pij`** (L1 ratchet, sibling to spec 115). R4/AC-6 assert
  installed guidance text only, never that a panel actually ran.
- `internal/redact`, new CLI verbs, new config schema fields
- `AGENTS.md` / repo-local memory content (mindspec-self material stays put)

## Non-Goals

- **The portability hard filter.** Only PORTABLE discipline — true for ANY
  project on the mindspec lifecycle — ships in skill text. Explicitly
  EXCLUDED from every shipped skill: `go test -short`, `bd`-off-PATH
  testing, `internal/harness`/`internal/instruct` pre-existing-RED lore,
  the `internal/lint` argv-ratchet, gofmt/git-ref-probe findings, SPECIFIC
  model values (Opus/Sonnet/Haiku/Fable/GPT-N assignments — operator
  preference, not product doctrine), **absolute operator home paths**
  (`/Users/...`, `/home/...`), and **this project's incident IDs**
  (`spec-NNN` references to mindspec's own history — the failure MODE ships
  portably; the provenance lives in ADR-0044, which setup never installs).
  The product stays model-family-agnostic (`claude`/`codex` family split
  only); operators configure specifics via `panel:`/`models:`. Guarded by
  AC-10's scan-plus-allowlist.
- Not a redesign of panel mechanics, tally, or the gate — the binary is
  already correct; this spec ships teaching and defaults.
- Not the GH #186 complete-stale-SHA bug, #179/#146/#187 verb
  trustworthiness, or disposition-telemetry work (spec 117 / ADR-0043 store).
- Not per-model quality tiering or any model roster.

## Acceptance Criteria

Legend: **[CI]** = enforced by a Go test, RED on revert of the
corresponding edit — AC-1/2/4/5/6/7/8/9/10/11/12/13 via the R1c
consistency/fragment-guard test in `internal/setup` (table-driven over `pluginmindspec.SkillFiles()` +
`lifecycleSkillFiles` content, plus `config.DefaultConfig()`); AC-15 via an
in-package test in `internal/panel/create_test.go`; AC-16 via the
upgrade-refresh test in `internal/setup`; AC-17 via the render-order test
in `internal/config`. Pinned fragments are SHORT DISTINCTIVE strings, not
byte-exact doctrine sentences (settled; the plan fixes the exact fragment
table). **Pre-edit-absence rule**: for EVERY pinned positive fragment, the
plan's fragment table MUST record a grep proof that the fragment is ABSENT
from the target file at the parent commit — a fragment already present
pre-edit makes the check vacuous (it would pass without the edit).
Known-present tokens are therefore BANNED as fragments: `reviewed_head_sha`
(already in all five skills), bare `verbatim` (already in
ms-panel-run/tally/bead-cycle), bare `absolute path` (already in
ms-bead-fix); the affected ACs below name distinctive replacements.
**[doc]** = doc-review-only (ADR prose judgment).

- [ ] AC-1 **[CI]** (R1) `config.DefaultConfig().Panel`: reviewer slots sum
  to 6 across exactly the families {`claude`, `codex`} (3+3) and
  `ApproveThreshold == "n-1"`; the embedded `ms-panel-run` and
  `ms-panel-tally` bytes each contain the canonical fragment
  `shipped default: 6 reviewers` and `n-1`; the test fails if any surface
  changes alone.
- [ ] AC-2 **[CI]** (R1) STRUCTURAL, not a denylist grep: the R1c test
  extracts the commented `panel.gates` ladder EXAMPLE from the embedded
  `ms-panel-run` bytes (the fenced example block, comment markers
  stripped), parses it, and asserts (i) it names gates `bead`,
  `spec_approve`, `final_review`, and (ii) every reviewer entry carries
  ONLY the keys `family` and `count` — ANY other key (in particular
  `model:`) fails the test. This does not delegate to AC-10; AC-10 remains
  the broad backstop over the full skill bytes.
- [ ] AC-3 **[doc]** (R1) ADR-0043's context no longer presents "8–12
  slots" as the product's shape: lines 16-17 (or an amendment block)
  distinguish the shipped 6 / `n-1` default from the operator-configured
  ladder, and the file contains the marker fragment the AC-1 test greps.
- [ ] AC-4 **[CI]** (R2) `ms-panel-run` and `ms-spec-final-review` each
  contain the fragment `git worktree add --detach` (verified absent at
  parent) inside a mutation-isolation instruction; `ms-panel-tally`
  contains a distinctive contested-finding re-verify fragment (candidate:
  `re-verify it yourself in a fresh detached checkout`) — NOT
  `reviewed_head_sha`, which pre-exists in all five skills; the plan's
  fragment table proves the chosen fragment absent at parent.
- [ ] AC-5 **[CI]** (R3) `ms-bead-cycle` no longer contains the bare
  `trust the empirical check` row; it and `ms-panel-tally` contain the
  short distinctive fragment `verify, never confirmed` (verified absent at
  parent) plus the reproduce-in-isolation requirement; `ms-panel-run`
  documents the security-classifier same-slot substitution fallback.
- [ ] AC-6 **[CI]** (R4) Installed-guidance assertion — this checks that
  the TEXT ships, not that any panel ran (the verbs are un-gated; see Out
  of Scope / `mindspec-0pij`): `lifecycleSkillFiles()["ms-spec-approve"]`
  and `["ms-plan-approve"]` each contain a panel-invocation step naming
  `--gate spec_approve` / `--gate plan_approve` respectively and the
  instruction not to run the approve verb before a tally Allow;
  `ms-panel-run` contains a document-gate lens table.
- [ ] AC-7 **[CI]** (R5) `ms-panel-run` no longer contains
  `Don't paste the plan; summarise it.`; it contains the BRIEF
  verbatim-AC instruction pinned by a distinctive fragment (candidate:
  `additive, never a replacement`) — NOT bare `verbatim`, which pre-exists
  in three of the skills; plan proves absence at parent — and names the
  AC-provenance reviewer duty.
- [ ] AC-8 **[CI]** (R6) `ms-panel-run` and `ms-spec-final-review` contain
  the fragment `findings-never-out-voted` (verified absent at parent) in
  reviewer-facing text stating fixed-or-evidence-refuted and
  threshold-is-a-floor.
- [ ] AC-9 **[CI]** (R7) `ms-panel-run`, `ms-bead-fix`, and
  `ms-spec-final-review` contain the generalized absolute-scratch mandate
  pinned by a distinctive fragment (candidate: `never a relative path` or
  `harness cwd-resets`) — NOT bare `absolute path`, which pre-exists in
  ms-bead-fix; plan proves absence at parent — applied to ALL
  reviewer/fixer scratch, not only codex files; `ms-bead-cycle` contains
  the `git status --porcelain`-clean check before `mindspec complete`; and
  `ms-panel-run` no longer contains `/Users/Max` (the `:195` sandbox note
  reads `$HOME/.codex/memories` or equivalent).
- [ ] AC-10 **[CI]** (Non-Goal filter) Scan-plus-allowlist guard over ALL
  embedded plugin-skill bytes AND all four lifecycle literals. Pattern
  classes (one table row each): (i) model values —
  `(?i)\b(opus|sonnet|haiku|fable|claude-[0-9][.\w-]*|gpt-[0-9][.\w-]*|o[0-9]+(-[a-z]+)?)\b`
  (categorical — covers `gpt-4o`, `gpt-5.6-sol`, `o4-mini`, `claude-4.5`, not
  just single-char suffixes); (ii) mindspec-lore
  invocations/findings — `go test -short`, `argv-ratchet`,
  `internal/harness`, `internal/instruct`, gofmt-corruption and
  git-ref-probe lore markers, `bd`-off-PATH choreography markers; (iii)
  absolute operator-home paths — `/Users/`, `/home/`, `/root/` (POSIX; Windows
  `C:\Users\` explicitly out of scope); (iv) project-incident IDs —
  `spec-[0-9]+`. Every match must appear in an explicit ALLOWLIST table in
  the test, each entry carrying the file, the matched text, and a one-line
  filed reason (e.g. an intentionally retained neutral example slug or a
  deliberately kept cross-project case history per R7's sweep); any
  non-allowlisted match REDs the test. Prefer generalizing a hit over
  allowlisting it; the allowlist exists so the guard can be comprehensive
  without forcing the sweep to rewrite text that is genuinely intentional.
  The guard MUST include negative test cases proving `gpt-4o`, `o4-mini`, and
  `/root/agent/…` each RED it (categorical-policy enforcement, per spec-gate G2).
- [ ] AC-11 **[CI]** (R8) `ms-spec-final-review` instructs a named slot to
  run the project-declared CI invocation, referencing `commands.ci` with
  `commands.test` fallback and the explicit no-declaration advisory;
  `internal/config/config.go`'s `Commands` comment documents the `ci`
  vocabulary key (asserted by a source-grep row in the R1c `internal/setup`
  test, or the `internal/config` render-order test file — plan picks one).
  Negative (honest tripwire, not a proof of absence): the
  CI-parity text names only config keys, never a concrete invocation —
  [CI]-checked for the known-risk patterns `go test`, `npm test`,
  `pytest`, and `make test` over the `ms-spec-final-review` bytes (plus
  AC-10's `go test -short` row); concrete commands outside this pattern
  table are caught by doc review, and the AC claims no more than the
  listed patterns.
- [ ] AC-12 **[CI]** The R1c test file exists in `internal/setup`, runs
  under plain `go test ./internal/setup/`, and is table-driven so each of
  AC-1/2/4/5/6/7/8/9/10/11's fragments is one row; reverting any single
  skill edit REDs AT LEAST its row (a file-granular revert of a multi-row
  skill such as `ms-panel-run` legitimately REDs every row pinned to that
  file — the requirement is per-fragment traceability, not
  one-revert-one-row).
- [ ] AC-13 **[CI]** (conduct home, citation half) Every touched skill
  surface — `ms-panel-run`, `ms-panel-tally`, `ms-bead-cycle`,
  `ms-bead-fix`, `ms-spec-final-review`, and the `ms-spec-approve` /
  `ms-plan-approve` literals — contains the string `ADR-0044` (a citation
  to the conduct home), and `.mindspec/adr/ADR-0044-*.md` exists with a
  `status: Accepted` (or equivalent Accepted marker) — asserted by grep
  rows in the R1c table.
- [ ] AC-14 **[doc]** (conduct home, boundary half)
  `ADR-0044-panel-review-conduct` carries the FULL normative doctrine
  (mutation-isolation, verify-never-confirm, reviewer-facing
  findings-never-out-voted, absolute-scratch, CI-parity kernel) plus the
  incident provenance; each skill carries ONLY its role-local operational
  fragment + the AC-13 citation — no duplicated normative text across the
  skills (the per-skill fragments pinned by AC-4/5/7/8/9 are role-local
  instructions, not doctrine restatements); ADR-0037 stays gate-mechanics,
  cross-referenced only.
- [ ] AC-15 **[CI]** (R5) The `internal/panel` `briefStubBody` constant
  contains the heading `## Acceptance Criteria (verbatim from spec.md)`,
  asserted by a test in `internal/panel/create_test.go` (in-package
  `package panel` — the constant is unexported and `internal/setup` does
  not import `internal/panel`, so the R1c table cannot reach it); RED on
  revert of the `create.go:26` constant edit; `renderBriefHeader` output
  unchanged.
- [ ] AC-16 **[CI]** (Delivery reach) `internal/setup/historical_skills/`
  contains a byte-exact pre-126 snapshot (`<name>.pre126.md`) for EVERY
  shipped skill this spec edits — `ms-panel-run`, `ms-panel-tally`,
  `ms-bead-cycle`, `ms-bead-fix`, `ms-spec-final-review`,
  `ms-spec-approve`, `ms-plan-approve` — and an upgrade test in
  `internal/setup` proves the refresh path: for each edited skill, seed
  the pre-126 canonical body on disk, run the setup skill-install path
  (`installSkills`), and assert the file is classified Refreshed and now
  matches the new canonical; PLUS a user-modified-preservation assertion
  (a body matching NO shipped snapshot is left in place with the HC-6
  notice). Deleting any one snapshot REDs the corresponding seed row.
- [ ] AC-17 **[CI]** (R8a) A test in `internal/config` calls the exported
  `RenderBuildTestSection` (`config.go:320`) on a fixture whose `commands`
  map declares `build`, `test`, `ci`, AND an extension key that sorts
  lexically BEFORE `ci` (e.g. `aa`), and asserts the rendered line order
  is exactly build, test, ci, then `aa`-then-remaining-sorted — i.e. `ci`
  in vocabulary position. Reverting the `commandOrder` `"ci"` entry
  (`config.go:233`) reorders `ci` after `aa` and REDs the test; presence
  of the `ci` line alone (the sorted-rest fallback) cannot pass it.

## Validation Proofs

- `go test ./internal/setup/ -run 'ReviewDiscipline|SkillFragment|UpgradeRefresh'`
  → PASS; then `git stash` any one skill edit → the matching row(s) FAIL →
  unstash → PASS (RED-on-revert demonstration for AC-12/AC-16).
- Pre-edit-absence proof (AC legend): at the parent commit,
  `grep -rn '<each pinned positive fragment>' plugins/mindspec/skills/ internal/setup/claude.go`
  → no match, recorded in the plan's fragment table.
- `go test ./... ` → PASS (no gate/tally behavior change; existing
  `internal/panel` + `internal/setup` + `internal/config` suites green).
- `go build ./... && grep -c 'shipped default: 6 reviewers' plugins/mindspec/skills/ms-panel-run/SKILL.md` → ≥1 (AC-1 surface).
- `grep -n "Don't paste the plan" plugins/mindspec/skills/ms-panel-run/SKILL.md` → no match (AC-7).
- `grep -rn '/Users/Max' plugins/mindspec/skills/` → no match (AC-9/AC-10).
- `grep -rinE '\b(opus|sonnet|haiku|fable|gpt-[0-9o])\b|go test -short|spec-[0-9]+|/Users/|/home/' plugins/mindspec/skills/`
  → every remaining match corresponds to an AC-10 allowlist row with a
  filed reason.
- Upgrade proof (AC-16): in a throwaway repo, install the PRE-126 skill
  bodies under `.claude/skills/`, run post-126 `mindspec setup claude` →
  setup reports each edited skill Refreshed and the files now carry the
  new fragments; a hand-edited skill body is left in place with the
  user-modified notice.
- Fresh-install proof: in a throwaway repo, `mindspec setup claude` →
  `.claude/skills/ms-spec-approve/SKILL.md` contains the `--gate
  spec_approve` panel step and `.claude/skills/ms-panel-run/SKILL.md`
  contains the mutation-isolation + verbatim-AC + doctrine fragments
  (AC-4/6/7/8 reach a stock install).
- `mindspec config show | grep -A2 'panel:'` on a zero-config repo → the
  3+3 / `n-1` default unchanged (R1d, settled).
- `go test ./internal/panel/ -run 'BriefStub'` → PASS;
  `grep -c 'Acceptance Criteria (verbatim from spec.md)' internal/panel/create.go`
  → ≥1 (AC-15 surface).
- `go test ./internal/config/ -run 'RenderBuildTest'` → PASS; then revert
  the `commandOrder` `"ci"` entry → FAIL (AC-17 RED-on-revert).

## Open Questions

- [x] grill (self-answered, headless): OQ-1 ADR home → new ADR-0044 (conduct ≠ gate-mechanics; ADR-0040 layering); lands Accepted per the divergence-lane rule.
- [x] grill (self-answered, headless): OQ-2 default size → keep 3+3 / `n-1`; ladder ships only as operator-preference example.
- [x] grill (self-answered, headless): OQ-3 stub heading → add to `briefStubBody`; the stub is where fresh installs look.
- [x] grill (self-answered, headless): OQ-4 R4 shape → no new skill files; lifecycle literals + `ms-panel-run` lens tables; guidance only, mechanization = `mindspec-0pij`.
- [x] grill (self-answered, headless): OQ-5 CI declaration → `commands.ci` vocabulary key, `commands.test` fallback; no schema change.
- [x] grill (self-answered, headless): OQ-6 fragment pinning → short distinctive fragments; same revert-detection, less brittle; every fragment proven absent at parent.

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-07-24
- **Notes**: Approved via mindspec approve spec