---
adr_citations:
    - ADR-0019
    - ADR-0034
    - ADR-0036
approved_at: "2026-06-17T11:11:00Z"
approved_by: user
bead_ids:
    - mindspec-pn3x.1
    - mindspec-pn3x.2
    - mindspec-pn3x.3
spec_id: 105-spec-grill-skill
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - plugins/mindspec/skills/ms-spec-grill/SKILL.md
      title: Author the ms-spec-grill plugin SKILL.md
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - bench/grill/fixtures/spec1.md
        - bench/grill/fixtures/spec2.md
        - bench/grill/fixtures/spec3.md
        - bench/grill/fixtures/spec4-heldout.md
        - bench/grill/ground_truth.tsv
        - bench/grill/det_detect.sh
        - bench/grill/run_eval.sh
      title: Track the grill detection eval harness under bench/grill/
    - depends_on:
        - 1
      id: 3
      key_file_paths:
        - internal/setup/claude.go
        - internal/setup/skills_test.go
        - internal/setup/claude_test.go
        - .mindspec/domains/workflow/OWNERSHIP.yaml
      title: Wire ms-spec-create auto-chain + setup counts + ownership
---
# Plan: 105-spec-grill-skill

Ship an LLM-backed **grill** as a `SKILL.md` prompt (no Go behavior), a
repo-tracked detection eval that proves the grill beats the deterministic
ceiling on the semantic/synonym/contradiction classes, and the wiring that
auto-invokes it from `ms-spec-create` while keeping the binary non-interactive.
Three beads: author the skill (1), track the eval that scores it (2), wire it
into setup/ownership (3). Beads 2 and 3 both depend on the skill text from
bead 1.

## ADR Fitness

The spec declares ONE impacted domain — **workflow** — and all three cited ADRs
are Accepted and cover it, so `adr-coverage` passes and none is irrelevant
(every cite intersects the single declared domain):

- **ADR-0019 (Deterministic Worktree and Branch Enforcement for Agent
  Workflows)** — Accepted; Domain(s) **workflow, git, agent-integration**.
  Establishes the agent-integration boundary this spec leans on: advisory,
  judgment-bearing prompt-layer guidance for agents vs deterministic binary
  enforcement. The grill is deliberately placed in the agent/skill (prompt)
  layer (Beads 1, 3) while `mindspec spec create` stays a non-interactive
  scaffold (AC9) — a direct extension of 0019's boundary, not a new error
  shape.
- **ADR-0034 (Ceremony Collapse)** — Accepted; Domain(s) **workflow**.
  Precedent for collapsing redundant workflow surface rather than duplicating
  it. It justifies shipping ONE lean `ms-spec-grill` skill that `ms-spec-create`
  invokes (Req 10/11, Bead 1+3) instead of inlining grill logic into
  `ms-spec-create`, and treating spec 104's deterministic interview engine as
  superseded rather than carried forward.
- **ADR-0036 (Ownership Discovery)** — Accepted; Domain(s) **workflow,
  validation, doc-sync, ownership**. Defines how `## Impacted Domains`
  file-path entries normalize to owning domains. The grill's domain-alignment
  technique (Req 2, Bead 1) reality-checks declared domains against
  `.mindspec/domains/*/OWNERSHIP.yaml` using exactly this model, and the
  `.agents/skills/**` ownership claim (Req 12, Bead 3) is what keeps the
  git-tracked `.agents/` replica from tripping `adr-divergence-unowned`.

ADR-coverage check: workflow → {0019, 0034, 0036}. The single declared domain
is covered by three Accepted ADRs → `mindspec validate plan` adr-coverage
passes; every cite intersects `workflow` → no `adr-cite-irrelevant`.

> Deferred (NOT implemented here): the principle "interactive, judgment-bearing
> authoring lives in the skill/agent layer; the binary stays non-interactive"
> plausibly warrants its own ADR (next free id at authoring time was
> **ADR-0039**). The spec defers this to Open Questions and does NOT cite an
> unwritten id as a touchpoint; this plan does the same — no bead writes
> ADR-0039.

## Testing Strategy

The decisive nuance of this spec is that its deliverable is a non-deterministic
LLM prompt, yet the bead-cycle gate requires "tests PASS." We therefore split
the proof set into two tiers and state explicitly which one gates a bead.

### HERMETIC / deterministic (bead-blocking — these MUST pass for the bead)

Every one of these is reproducible byte-for-byte with no live LLM, no network,
no `claude` CLI. They are the gating set the bead-cycle "tests pass" contract
covers:

- **AC2** — after `mindspec setup`, the grill exists at all three paths
  (`plugins/…`, `.claude/…`, `.agents/…`) with parseable `name:`/`description:`
  frontmatter. Hermetic: it is `mindspec setup` (a binary, no LLM) plus
  `test -f` + `grep`.
- **AC3** — protocol-coverage grep over the plugin SKILL.md (each technique
  name present in the prompt text).
- **AC5** — `det_detect.sh` scores exactly **0/M** on the LLM-only classes
  (SEMANTIC+SYNONYM+CONTRADICTION) over the cleaned fixtures, while still
  catching 100% of GROUNDING/EXACT_PHRASE/STRUCTURAL. Pure shell + grep/awk,
  no LLM.
- **AC9** — `mindspec spec create <fresh-id> </dev/null` exits 0 (binary stays
  non-interactive, no blocking stdin read).
- **AC10** — `grep -qE 'ms-spec-grill'` over every `ms-spec-create` replica AND
  the `lifecycleSkillFiles()` literal source (the grep-provable handoff).
- **AC11** — `grep` proves `.agents/skills/**` is owned by workflow,
  `! grep bench/` proves no `bench/**` glob was added, and
  `go test ./internal/setup/...` passes with the updated 12-skill counts.

⚠️ **Go-test scope:** the only Go tests this spec touches are the setup count
assertions. Run `go build ./...` then the filtered
`go test ./internal/setup/...`. **NEVER run `go test ./internal/harness/...`**
(per AGENTS.md; the harness suite is out of scope and slow/flaky).

### `mindspec setup` propagation note

AC2's three-path install is **only** green AFTER `mindspec setup` runs. The
single source of truth is `plugins/mindspec/skills/ms-spec-grill/SKILL.md`;
setup copies it to `.claude/skills/ms-spec-grill/SKILL.md` and
`.agents/skills/ms-spec-grill/SKILL.md` via the plugin `//go:embed
skills/*/SKILL.md` → `pluginmindspec.SkillFiles()` mechanism. The bead author
MUST NOT hand-author the `.claude/`/`.agents/` copies and MUST NOT add the
grill to `lifecycleSkillFiles()` (that is the lifecycle-gate inline-literal
mechanism `ms-spec-create` uses; the grill is a PLUGIN skill). Any test or
proof that checks the propagated copies runs `mindspec setup` first.

### ADVISORY / LLM eval (NOT bead-blocking — run on demand, reported, never gated)

- **AC1** — `run_eval.sh` LLM-grill recall on M. This is non-deterministic,
  `claude`-CLI-gated, and SKIPS-with-notice (exit 0) when `claude` is absent.
  The spec's Hard Constraints make it explicit: the LLM recall figure is
  ADVISORY; a single bad sample MUST NOT red a panel or block a bead. The
  reproducibility pin (**FIXED full model id** via `claude -p --model <id>`,
  N≥5 runs, MIN/median recall ≥ ⌈0.9·M⌉ where M is computed from the tracked
  `ground_truth.tsv`) makes the figure as stable as an LLM allows, but it is
  still demonstrated on demand and reported — it is **not** part of the hermetic
  "tests pass" gate. The behavioral ACs realized through the same harness (AC6
  grounding, AC7 invented-domain, AC8 scenario) are likewise ADVISORY: their
  deterministic *match rule* is hermetic, but they require a live `claude` run
  to produce the findings being matched, so they ride with AC1, not the gating
  set.

> **Reproducibility mechanism note (no temperature lever).** The Claude Code CLI
> exposes **no `--temperature` flag** (verified against v2.1.178: `--model`
> exists, a temperature option does not). The spec's Hard-Constraint
> "reproducibility pin" INTENT is therefore realized via **`--model <pinned full
> model id>` + N≥5 runs with MIN/median aggregation** — not a temperature knob.
> This plan faithfully interprets the approved spec's reproducibility intent
> within the tool's real capability; it does NOT re-open the spec, it merely
> records the mechanism the CLI actually permits.

### Known limits of the hermetic proof (the deliverable is a non-deterministic prompt)

The hermetic/bead-blocking set above proves *wiring and shape*, never *efficacy*.
State this plainly, mirroring the AC1-is-advisory honesty:

- **AC3 is a keyword-PRESENCE grep, not a quality gate.** A SKILL.md that merely
  lists the six technique words — even in a degenerate, hollow prompt that never
  actually grills — passes AC3. The hermetic set NEVER proves the grill actually
  *works*; prompt efficacy is only ever demonstrated by the ADVISORY eval
  (AC1/AC6/AC7/AC8 via `run_eval.sh`).
- **AC10 is a presence-grep proving wiring-PRESENCE, NOT runtime invocation.** A
  skill naming another skill in prose is a *soft handoff* the binary never
  executes — an inherent limit of the prompt layer. AC10 proves the token
  `ms-spec-grill` is present in `ms-spec-create`; it cannot prove the agent
  actually runs the grill at author time.
- **The "all hermetic gates green but the prompt is hollow" path is real.** A
  non-firing grill can pass every gating test: AC2 (file present + frontmatter),
  AC3 (technique words present), AC5 (det baseline 0/M — about the *fixtures*,
  not the grill), AC9 (non-interactive exit), AC10 (token present), AC11
  (ownership + counts). Whether the grill *fires* is observable ONLY via the
  advisory eval, never the hermetic gate. Bead 1's required-but-advisory
  fire-demonstration (below) is the lightweight guard against this hollow path.

### Behavioral-coverage gaps (per-requirement honesty)

Only **R1→AC6**, **R2→AC7**, and **R6→AC8** have any behavioral AC, and even those
three are ADVISORY (live-`claude` eval, never gating). **R3 (synonym/fuzzy), R4
(falsifiability), R5 (contradiction), and R7 (thinness/AC-floor) have NO
behavioral AC of their own** — their firing is observable only as part of AC1's
*aggregate* recall over M. A per-technique regression in R3/R4/R5/R7 (e.g. the
prompt silently stops catching contradictions) is therefore **invisible** to
every gate: it would only dent the aggregate recall figure, and only then if it
crosses the ⌈0.9·M⌉ threshold. This is named here rather than implied as
per-technique coverage by the Provenance table — the table maps ACs to proofs,
not techniques to ACs.

**Bead gate statement:** each bead's "tests pass" obligation is satisfied by
its HERMETIC subset above (det baseline 0/M, install-after-setup,
protocol-coverage grep, non-interactive exit, handoff grep, ownership glob +
no-bench + `go test ./internal/setup/...`). The LLM recall (AC1) and the
LLM-backed behavioral ACs (AC6–AC8) are demonstrated on demand by running
`bench/grill/run_eval.sh` and **reported**, never used to fail CI or a panel.

## Decomposition rationale (3 beads)

The work splits along three non-overlapping artifact surfaces, each
independently reviewable:

| Bead | Reqs | Primary surface | Owns ACs |
|:-----|:-----|:----------------|:---------|
| 1 — author the grill | 1–7, 11 | `plugins/mindspec/skills/ms-spec-grill/SKILL.md` | AC3 (and the prompt AC1/AC6/AC7/AC8 exercise) |
| 2 — track the eval | 14 | `bench/grill/**` (fixtures, ground_truth.tsv, det_detect.sh, run_eval.sh) | AC1 (advisory), AC5, AC6/AC7/AC8 (advisory, via harness) |
| 3 — wire + setup + ownership | 8, 9, 10, 12, 13 | `internal/setup/**`, `ms-spec-create` replicas, `OWNERSHIP.yaml` | AC2, AC9, AC10, AC11 |

Heuristic justification:

- **Single concern each.** Bead 1 is the prompt (a SKILL.md, no Go). Bead 2 is
  the scoring harness (shell + fixtures + TSV). Bead 3 is the Go/setup wiring
  and the ownership/count plumbing. Combining any two would mix the prompt-craft
  review with a shell-harness review or a Go-test review — three different
  lenses.
- **Right-sized.** None is large enough to split further; each is one coherent
  artifact tree plus its hermetic proof.
- **Why a NEW skill (Req 11), not an inline rewrite of `ms-spec-create`:** a
  single-responsibility grill is independently eval-able by Bead 2, reusable
  beyond create, and matches ADR-0034's collapse-don't-duplicate posture.
- **Scope redundancy WARN is expected and acceptable.** The three beads share
  almost no file paths (the grill source, the bench tree, and the setup tree
  are disjoint), so `mindspec validate plan` may emit a non-blocking
  `decomposition-scope-redundancy` WARN ("R below threshold … beads may lack
  shared context"). That is correct here by design — these are genuinely
  independent surfaces unified only by depending on the same prompt — and is a
  WARN, not an error.

### work_chunks depends_on graph

```
Bead 1 (grill SKILL.md)         depends_on: []
Bead 2 (bench/grill eval)       depends_on: [1]
Bead 3 (wire + setup + owner)   depends_on: [1]
```

Longest path = 2 (1 → 2 and 1 → 3 in parallel after 1). No cycle; a DAG.

**Merge-order note.** Bead 1 MUST merge first: both Bead 2 and Bead 3 reference
and test the grill skill authored in Bead 1.

- Bead 2's `run_eval.sh` drives the grill (AC1) and its det baseline (AC5) is
  scored against the fixtures it ships, but the *fixtures and TSV* it tracks
  are also the corpus the grill prompt was tuned against — so the grill text
  (Bead 1) must exist first for the eval to mean anything.
- Bead 3's AC2 (`mindspec setup` propagates the grill to three paths) and AC10
  (`ms-spec-create` auto-invokes `ms-spec-grill`) both reference the grill that
  Bead 1 creates: without Bead 1, the plugin SKILL.md setup propagates does not
  exist and the handoff points at a non-existent skill.

After Bead 1 lands, Beads 2 and 3 are independent of each other (disjoint file
sets: `bench/**` vs `internal/setup/**` + `OWNERSHIP.yaml` + `ms-spec-create`
replicas) and may be cycled in either order or in parallel.

## Bead 1: author the ms-spec-grill plugin SKILL.md (Reqs 1–7, 11)

**Scope:** Create the single source-of-truth plugin skill
`plugins/mindspec/skills/ms-spec-grill/SKILL.md` encoding the grill protocol
and every coaching technique. No Go code, no other file.

**Changed files:** `plugins/mindspec/skills/ms-spec-grill/SKILL.md` (new)

**Steps**
1. Write valid plugin frontmatter — `name: ms-spec-grill` and a `description:`
   line — matching the shape of `plugins/mindspec/skills/ms-panel-run/SKILL.md`
   (a `---` block with `name`/`description`), so the plugin-discovery walk and
   AC2's frontmatter grep both pass.
2. Encode the **grill protocol (R1):** instruct the agent to ask exactly ONE
   question at a time (never batch), refuse to advance until the current answer
   is concrete, and ground every claim live — cross-check each declared domain
   against the directories under `.mindspec/domains/`, surface the real
   ADRs under `.mindspec/adr/` relevant to each impacted domain, suggest
   the next-free ADR number when a new principle appears, and reality-check
   "X already does Y" claims against the actual tree before accepting them.
   - ⚠️ **IMPL NOTE:** the AC3 grep is literal — the prompt text MUST contain a
     phrase matching `one (question )?at a time` (e.g. "ask ONE question at a
     time"). Write the technique names as plain words the grep can find.
3. Encode the **coaching techniques** so AC3's grep set all match:
   `domain` alignment (R2 — validate each `## Impacted Domains` entry against
   the real domain set, reject invented domains like "caching"/"scheduling",
   map file-paths to owners per ADR-0036); `synonym|fuzzy` detection (R3 —
   reject support/enable/improve/handle verbs with no falsifiable behavior);
   `falsifiab`ility coaching (R4 — refuse "it works"/"is fast"/"reasonable
   time", coach to a threshold + observable); `contradiction` detection (R5 —
   pairwise scan, e.g. "run concurrently" vs "at most one at a time", force
   resolution); `scenario|edge[- ]case` probing (R6 — drive failure /
   concurrency / empty-input / boundary scenarios into requirements or explicit
   Non-Goals).
4. Encode the **thinness refusal & AC floor (R7):** refuse thin/empty answers,
   require ≥3 falsifiable acceptance criteria each pairing an assertion with a
   runnable proof, and resolve or explicitly defer every Open Question before
   the grill is considered complete.
5. State (R11) that this is a NEW single-responsibility skill — the prompt does
   the grilling; `ms-spec-create` only auto-invokes it (the invocation edit is
   Bead 3, not here).
6. **Verbatim-span instruction (supports the eval anchor matcher, Bead 2 §C):**
   instruct the agent that for EACH finding it surfaces it MUST echo the
   **verbatim quoted span copied from the spec/fixture text** that the finding is
   about, prefixed with a `[CATEGORY]` tag (one of
   SEMANTIC/SYNONYM/CONTRADICTION/GROUNDING/EXACT_PHRASE/STRUCTURAL). This makes
   findings deterministically matchable against `ground_truth.tsv` anchors
   without an LLM judge. The span MUST be quoted from the source text as written,
   NOT paraphrased.

**RED tests** (hermetic; bead-blocking)
- **AC3 protocol-coverage grep** over the new file, ALL must succeed:
  `grep -qiE 'domain'`, `grep -qiE 'synonym|fuzzy'`, `grep -qiE 'falsifiab'`,
  `grep -qiE 'contradiction'`, `grep -qiE 'scenario|edge[- ]case'`,
  `grep -qiE 'one (question )?at a time'`. RED before the file exists / before
  the techniques are named; GREEN once steps 2–3 land.
- **AC2 frontmatter (source side):** `grep -qE '^name:'` and
  `grep -qE '^description:'` over the plugin source succeed. (The propagated
  three-path check is Bead 3's AC2, after `mindspec setup`.)

**Acceptance Criteria**
- [ ] `plugins/mindspec/skills/ms-spec-grill/SKILL.md` exists with valid
      `name:` + `description:` frontmatter (AC2 source side).
- [ ] The AC3 protocol-coverage grep set ALL succeed over that file (each
      technique named in the prompt).

**Verification**
- [ ] `test -f plugins/mindspec/skills/ms-spec-grill/SKILL.md` PASS
- [ ] `grep -qiE 'domain' && grep -qiE 'synonym|fuzzy' && grep -qiE 'falsifiab' && grep -qiE 'contradiction' && grep -qiE 'scenario|edge[- ]case' && grep -qiE 'one (question )?at a time'` over the file (AC3) — all PASS
- [ ] `grep -qE '^name:' && grep -qE '^description:'` over the file PASS
- [ ] **REQUIRED-but-ADVISORY (non-gating, reported) fire-demonstration:** a
      single `claude -p` run of the grill prompt against ≥1 thin fixture surfaces
      **≥1 real SEMANTIC or CONTRADICTION finding** (a substantive finding, not a
      restatement). The transcript/output is attached to **Bead 1's panel
      evidence** so a hollow/non-firing prompt cannot silently pass on the AC3 +
      frontmatter greps alone. This is a LIGHTWEIGHT demonstration — it does NOT
      require Bead 2's full `run_eval.sh` (authored later) — and **SKIPS-with-notice
      if `claude` is absent**. It is SEEN/REPORTED in panel evidence, **not** a
      hermetic gate; the AC3 grep alone is a presence check that a degenerate
      prompt would pass (see Testing Strategy § Known limits).

**Depends on**
None

## Bead 2: track the grill detection eval under bench/grill/ (Req 14)

**Scope:** Bring the cleaned `/tmp/grill-bench` prototype into the repo at
`bench/grill/` and complete it to the spec's contract: fixtures (the cleaned 3
+ ≥1 held-out/blind, CONTRADICTION raised to ≥3 items), `ground_truth.tsv`,
the deterministic baseline `det_detect.sh`, and the model-pinned,
deterministic-matching, `claude`-gated `run_eval.sh`. ⚠️ `bench` is an excluded
first-segment — this tree gets NO OWNERSHIP glob (that is Bead 3's negative AC).

**Changed files:** `bench/grill/fixtures/spec1.md`, `bench/grill/fixtures/spec2.md`,
`bench/grill/fixtures/spec3.md`, `bench/grill/fixtures/spec4-heldout.md` (new held-out/blind),
`bench/grill/ground_truth.tsv`, `bench/grill/det_detect.sh`, `bench/grill/run_eval.sh` (new)

**Steps**
1. Copy the cleaned fixtures from the prototype, KEEPING the deterministic-leak
   fix already applied: spec2-P4 and spec3-P4 are re-phrased so the
   `functions? properly` / `handles .* correctly` regexes in `det_detect.sh`
   no longer accidentally catch them — this is what makes the AC5 baseline a
   TRUE 0/M on the LLM-only classes rather than 2/M.
   - ⚠️ **IMPL NOTE:** verify the leak is closed by running `det_detect.sh`
     over spec2/spec3 and confirming it flags ZERO SEMANTIC/SYNONYM items.
2. Add **≥1 HELD-OUT/blind fixture** (`spec4-heldout.md`) NOT used while
   authoring or tuning the grill (Bead 1), and **raise CONTRADICTION to ≥3
   items** total across the fixture set (the prototype had n=1, which is
   unfalsifiable). Record every planted problem in `ground_truth.tsv` with its
   `category` (SEMANTIC/SYNONYM/CONTRADICTION/GROUNDING/EXACT_PHRASE/STRUCTURAL),
   `catchable_by` (llm/both), and the **`anchor`** column (see § Eval matcher
   below).
   - ⚠️ **IMPL NOTE:** the held-out fixture's LLM-only items and all ≥3
     CONTRADICTION items MUST be INCLUDED in M — they may not be excluded from
     the scored denominator. M = count of all SEMANTIC+SYNONYM+CONTRADICTION
     rows in the FINAL tracked `ground_truth.tsv`.
3. Track `det_detect.sh` (the prototype's deterministic ceiling: empty/
   placeholder detection, exact bad-phrase lists, domain-set membership, <3
   ACs). It MUST still catch 100% of GROUNDING/EXACT_PHRASE/STRUCTURAL and 0/M
   of the LLM-only classes.
   - ⚠️ **ANTI-GAMING (det_detect is a FIXED baseline, NOT tunable):**
     `det_detect.sh`'s heuristic regex set is the **faithful deterministic
     ceiling** — it is FIXED, not a lever. The AC5 `0/M` result MUST be achieved
     by **FIXTURE RE-PHRASING ONLY** (closing the leak in spec2-P4/spec3-P4 etc.,
     per step 1), NEVER by narrowing `det_detect.sh`'s regexes to dodge the
     LLM-only items. An impl that narrows a regex to fake `0/M` would break the
     POSITIVE recall assertion in AC5 (below), not just the negative one.
4. Author `run_eval.sh` (R14):
   - Invoke `claude -p` with a **FIXED full model id** (`--model <pinned id>`) so
     the recall is as reproducible run-to-run as the CLI allows. **There is NO
     `--temperature` flag** in the Claude Code CLI (verified v2.1.178) — do NOT
     attempt to pass one; determinism is realized by the model-pin + N-run
     aggregation below, not a temperature knob (see Testing Strategy §
     Reproducibility mechanism note).
   - Loop **N≥5** times; compute **M** from the tracked `ground_truth.tsv`
     (count the SEMANTIC+SYNONYM+CONTRADICTION rows — NOT a frozen literal);
     score each run's grill findings against `ground_truth.tsv` by the
     **DETERMINISTIC STRUCTURED-ANCHOR match** defined in § Eval matcher below
     (NO LLM judging an LLM); report MIN (or median) recall over the runs and the
     deterministic baseline (0/M) for comparison; MAY additionally print the
     held-out fixture's recall separately. Exit 0 only when min/median ≥
     **⌈0.9·M⌉**.
   - **`claude` is a PRECONDITION:** if `claude` is not installed/authenticated
     (logged-in CLI or `ANTHROPIC_API_KEY`), **SKIP-with-notice** (print a skip
     message, exit 0) — never hard-fail — so the eval is runnable on fresh
     machines/CI.
   - ⚠️ **IMPL NOTE:** make `run_eval.sh` and `det_detect.sh` executable
     (`chmod +x`) and shellcheck-clean.

**§ Eval matcher — STRUCTURED-ANCHOR design (replaces hand-waved keyword/line matching)**

The match rule is the load-bearing part of the eval: it must credit a planted
problem deterministically without an LLM judge, and it must survive the LLM
paraphrasing its own prose run-to-run. The design:

- **C1 — `anchor` column.** `ground_truth.tsv` gains a unique **`anchor`** per
  row: a unique substring of the **FIXTURE'S OWN text** for that problem (e.g.
  the requirement/AC phrase exactly as written in the fixture — NOT the LLM's
  paraphrase). Paired with this, the grill prompt (Bead 1, step 6) instructs the
  agent to echo, for each finding, the **verbatim quoted fixture span** plus a
  `[CATEGORY]` tag.
- **C2 — credit rule.** The matcher credits a problem-id **IFF** (the finding's
  `[CATEGORY]` tag matches the row's `category`) **AND** (that row's `anchor`
  appears as a **NORMALIZED substring** in a finding line). Normalization =
  case-fold + strip punctuation/quotes (so `"Function Properly,"` matches
  `function properly`). One-to-one: **each P-id is credited at most once**;
  **surplus findings** (the LLM finds more than were planted) **neither credit
  nor penalize**. Recall = matched-planted / M.
- **C3 — anchor is pinned to the FIXTURE, not the paraphrase.** The anchor MUST
  be the fixture phrase (stable across runs), never the LLM's wording. Real
  prototype runs proved paraphrase drift — the model wrote "function properly"
  where the fixture said "behave as expected", and "handles concurrency
  correctly" where the fixture said "robust under load" — which an
  LLM-paraphrase anchor would have scored as FALSE NEGATIVES. Pinning the anchor
  to the fixture text (which the prompt instructs the agent to quote verbatim)
  removes that drift.
- **C4 — author-time anchor/fixture self-check.** `run_eval.sh` (or a tiny
  companion check, run at author time and on every eval invocation) MUST assert
  that **every `ground_truth.tsv` row's `anchor` is actually a substring of its
  own fixture file**. This catches anchor/fixture skew at author time (a typo'd
  or stale anchor would otherwise silently fail to ever match). This check is
  hermetic (no `claude`) and may run even in the SKIP-with-notice path.

**RED tests**
- **AC5 (hermetic, bead-blocking) — NEGATIVE 0/M AND POSITIVE recall:**
  `bench/grill/det_detect.sh bench/grill/fixtures/*.md` (a) flags ZERO
  SEMANTIC/SYNONYM/CONTRADICTION problems → assert
  `deterministic recall (semantic+synonym+contradiction) = 0/M` (RED if the leak
  fixtures aren't cleaned; GREEN after step 1); AND (b) **POSITIVE assertion**
  still catches the concretely-planted STRUCTURAL/EXACT_PHRASE/GROUNDING items
  — INCLUDING the planted items in the held-out fixture — at 100%. The positive
  half is what stops an impl from faking `0/M` by NARROWING a regex instead of
  re-phrasing a fixture: a narrowed regex breaks this POSITIVE test.
- **AC1 / AC6 / AC7 / AC8 (ADVISORY, NOT bead-blocking):**
  `bench/grill/run_eval.sh` run on demand with `claude` available reports LLM
  min/median recall ≥ ⌈0.9·M⌉ on M (via the structured-anchor match), the
  GROUNDING item detected (AC6), the invented-domain item detected (AC7), and a
  scenario/edge-case finding (AC8). With `claude` ABSENT the script
  SKIPS-with-notice and exits 0. These are demonstrated and **reported**, not
  used to gate the bead.
- **C4 anchor self-check (hermetic):** every `ground_truth.tsv` row's `anchor`
  is a substring of its own fixture file — runs even when `claude` is absent.

**Acceptance Criteria**
- [ ] `det_detect.sh` over the cleaned fixtures scores 0/M on the LLM-only
      classes (achieved by FIXTURE RE-PHRASING, not regex narrowing) AND 100% of
      GROUNDING/EXACT_PHRASE/STRUCTURAL including the held-out fixture's planted
      items (AC5, both halves).
- [ ] `ground_truth.tsv` includes ≥1 held-out fixture's LLM-only items and ≥3
      CONTRADICTION items, all counted in M, and an `anchor` column whose every
      value is a substring of its own fixture (C1/C4).
- [ ] `run_eval.sh` is model-pinned (fixed full model id via `--model`; NO
      temperature flag — does not exist), loops N≥5, computes M from the TSV,
      matches via the STRUCTURED-ANCHOR rule (category-tag AND normalized-anchor
      substring, one-to-one, surplus neither credits nor penalizes), exits 0 only
      on recall ≥ ⌈0.9·M⌉, runs the C4 anchor self-check, and SKIPS-with-notice
      (exit 0) when `claude` is absent (AC1 advisory; AC6/AC7/AC8 advisory).

**Verification**
- [ ] `bench/grill/det_detect.sh bench/grill/fixtures/*.md` → 0/M on
      SEMANTIC+SYNONYM+CONTRADICTION AND 100% on the schema-catchable classes
      including held-out planted items (AC5 negative + positive, hermetic) PASS
- [ ] anchor self-check (C4): every `ground_truth.tsv` `anchor` is a substring of
      its fixture — PASS (hermetic, no `claude`)
- [ ] `bash -n bench/grill/run_eval.sh` parses; on-demand run with `claude`
      present reports min/median recall ≥ ⌈0.9·M⌉ via the structured-anchor match
      (AC1, advisory — reported)
- [ ] `bench/grill/run_eval.sh` with `claude` ABSENT exits 0 with a skip notice (hermetic)

**Depends on**
Bead 1 (the eval scores the grill skill authored there; merge-order: Bead 1 first).

## Bead 3: wire ms-spec-create auto-chain + setup counts + ownership (Reqs 8, 9, 10, 12, 13)

**Scope:** Make `mindspec setup` propagate the new plugin skill, auto-invoke
the grill from `ms-spec-create` (all replicas + the `lifecycleSkillFiles()`
literal source), update the setup count assertions for the 8th plugin skill,
and claim `.agents/skills/**` in the workflow OWNERSHIP.yaml. No `bench/**`
glob (hard schema error).

**Changed files:** `internal/setup/claude.go` (the `ms-spec-create` inline
literal in `lifecycleSkillFiles()`), `internal/setup/skills_test.go`,
`internal/setup/claude_test.go`, `.mindspec/domains/workflow/OWNERSHIP.yaml`

**Steps**
1. **Auto-chain handoff (R10):** edit the `ms-spec-create` SKILL.md — which is
   defined as an inline raw-string literal in `lifecycleSkillFiles()` in
   `internal/setup/claude.go` (around line 610) — to add an explicit,
   grep-provable handoff: after `mindspec spec create <id>` scaffolds, the skill
   auto-invokes `ms-spec-grill` by default (the grill fires UNLESS the author
   explicitly opts out). The literal text MUST contain the token `ms-spec-grill`
   so AC10's grep matches.
   - ⚠️ **IMPL NOTE:** `ms-spec-create` is a LIFECYCLE-gate skill — its source
     is the inline literal in `lifecycleSkillFiles()`, NOT a file under
     `plugins/`. `mindspec setup` regenerates `.claude/skills/ms-spec-create/`
     and `.agents/skills/ms-spec-create/` from that literal, so editing the
     literal updates every replica. Do NOT add `ms-spec-grill` to
     `lifecycleSkillFiles()` — the grill is a PLUGIN skill (Bead 1) that setup
     discovers via `//go:embed skills/*/SKILL.md`. Auto-chain-by-default is
     chosen over opt-in because the failure mode being fixed is authors skipping
     rigor.
   - ⚠️ **LITERAL-EDIT SAFETY (G):** editing the `lifecycleSkillFiles()` Go
     raw-string literal round-trips idempotently through `mindspec setup` ONLY if
     any backtick the new prose needs is emitted via the existing
     `` + "`" + `` concat pattern already used in that literal (a bare backtick
     inside a Go raw string is impossible and would not compile). Keep the added
     handoff prose backtick-free or use that concat pattern. Additionally, the
     auto-chain prose MUST use **imperative phrasing** — e.g. "automatically run
     ms-spec-grill" — NOT a passive "see ms-spec-grill", so the text actually
     INSTRUCTS invocation rather than merely cross-referencing.
2. **Non-interactive binary (R8 / AC9):** confirm `mindspec spec create` adds
   no blocking stdin read — the grilling is agent reasoning, not a binary
   prompt. No Go behavior change is expected; AC9 is a regression assertion.
   - ⚠️ **IMPL NOTE:** AC9 is proved by `mindspec spec create <fresh-id>
     </dev/null` exiting 0. If anything in the create path ever read stdin this
     would hang; the proof is that it does not.
3. **Setup count assertions (R13)** — adding the 8th plugin skill shifts the
   hardcoded counts. The edits MUST be COMPLETE — numeric literals AND the prose
   comments, func names, and `want`/verified-name slices — not just the numbers:
   - `internal/setup/skills_test.go`:
     - `len(all)` **11 → 12**;
     - add `"ms-spec-grill"` to the `want` **slice**;
     - **rename/retag** the `TestSkillInventory_Eleven` func (the "Eleven" name
       is now wrong → e.g. `TestSkillInventory_Twelve`) AND its
       "exactly 11 / 4 lifecycle + 7 plugin" comment → "exactly 12 / 4 lifecycle
       + 8 plugin".
   - `internal/setup/claude_test.go`:
     - `TestRunClaude_FreshSetup` — `len(r.Created)` **13 → 14**; add
       `"ms-spec-grill"` to the verified skill-**name list**; fix the "7 plugin
       skills … 13 items" / "7 plugin = 11" prose comments → **8 / 14 / 12**.
     - `TestRunClaude_Idempotent` — `len(r2.Skipped)` **13 → 14**.
   - ⚠️ **IMPL NOTE:** the "+1" propagates: 1 new plugin skill = +1 created item
     AND +1 skipped item on the idempotent second run (13 → 14 in both). Grep the
     two test files for every `13`/`11`/`7` count tied to the skill surface AND
     every `Eleven`/"7 plugin" textual token; leave hook/CLAUDE.md counts
     untouched.
   - ⚠️ **IMPL NOTE (stale DOC comments, cosmetic):** `plugins/mindspec/embed.go`
     carries "7 skills"/"N=7" DOC COMMENTS that NO test asserts — update them to
     8 for accuracy, but they are cosmetic (not gated). No Go skill *list* edit is
     needed for AC2: `embed` auto-discovers via `WalkDir` over
     `//go:embed skills/*/SKILL.md`, so the new plugin skill is picked up
     automatically.
   - ⚠️ **NO OTHER COUNT SURFACES:** verified clean — README, AGENTS.md, the
     `mindspec instruct` output, and the CLAUDE-managed block carry NO hardcoded
     skill count that this bead must touch. The ONLY count surfaces are the two
     `internal/setup/*_test.go` files (gated) + `embed.go`'s doc comments
     (cosmetic).
4. **Ownership claim (R12 / AC11):** add `- .agents/skills/**` to
   `.mindspec/domains/workflow/OWNERSHIP.yaml` so the git-tracked
   `.agents/skills/ms-spec-grill/SKILL.md` normalizes to workflow and does not
   trip `adr-divergence-unowned` on `mindspec complete`.
   - ⚠️ **IMPL NOTE (hard constraint):** do NOT add any `bench/**` glob to any
     OWNERSHIP.yaml — `bench` is an excluded first-segment; the divergence gate
     skips it and adding it is a hard `LoadOwnership` schema error. AC11
     asserts `! grep -qE 'bench/'` over the file.

**RED tests** (all hermetic; bead-blocking)
- **R13 / count tests:** `go test ./internal/setup/...` — RED against the old
  11/13 counts the moment the 8th plugin skill (Bead 1) is on disk; GREEN after
  step 3 updates the literals to 12/14.
- **AC2 (propagated, three-path):** `mindspec setup` then
  `test -f plugins/mindspec/skills/ms-spec-grill/SKILL.md && test -f .claude/skills/ms-spec-grill/SKILL.md && test -f .agents/skills/ms-spec-grill/SKILL.md`,
  and for each `grep -qE '^name:' && grep -qE '^description:'`.
- **AC10:** `grep -qE 'ms-spec-grill'` over `.claude/skills/ms-spec-create/SKILL.md`,
  `.agents/skills/ms-spec-create/SKILL.md`, AND the `lifecycleSkillFiles()`
  literal in `internal/setup/claude.go`.
- **AC9:** `mindspec spec create <fresh-id> </dev/null` exits 0.
- **AC11:** `grep -qE '^\s*- \.agents/skills/\*\*'` succeeds and
  `! grep -qE 'bench/'` succeeds over the workflow OWNERSHIP.yaml.

**Acceptance Criteria**
- [ ] `ms-spec-create` (literal + both replicas after setup) auto-invokes
      `ms-spec-grill` — grep-provable (AC10).
- [ ] After `mindspec setup`, the grill exists at all three paths with valid
      frontmatter (AC2).
- [ ] `mindspec spec create <fresh-id> </dev/null` exits 0 — binary stays
      non-interactive (AC9).
- [ ] `.agents/skills/**` is workflow-owned, NO `bench/**` glob, and
      `go test ./internal/setup/...` passes with 12-skill (4 lifecycle + 8
      plugin) counts (AC11, R13).

**Verification**
- [ ] `go build ./...` passes
- [ ] `go test ./internal/setup/...` PASS (12/14 counts) — ⚠️ NEVER `./internal/harness/...`
- [ ] `mindspec setup` then the three-path `test -f` + per-file `grep -qE '^name:'`/`'^description:'` (AC2) PASS
- [ ] `grep -qE 'ms-spec-grill'` over both `ms-spec-create` replicas + the literal source (AC10) PASS
- [ ] `mindspec spec create <fresh-id> </dev/null`; `echo $?` = 0 (AC9)
- [ ] `grep -qE '^\s*- \.agents/skills/\*\*' .mindspec/domains/workflow/OWNERSHIP.yaml` PASS and `! grep -qE 'bench/' .mindspec/domains/workflow/OWNERSHIP.yaml` PASS (AC11)

**Depends on**
Bead 1 (setup propagates the grill plugin skill; the handoff points at it; the
count tests go red once it is on disk). Merge-order: Bead 1 first.

## Provenance

| Acceptance Criterion (spec) | Bead | Tier | Proof |
|:----------------------------|:-----|:-----|:------|
| **AC1** — LLM grill recall ≥ ⌈0.9·M⌉ over N≥5 runs on M (incl. held-out + ≥3 CONTRADICTION), beats det baseline | Bead 2 | ADVISORY | `bench/grill/run_eval.sh` (model-pinned via `--model`, NO temperature flag, STRUCTURED-ANCHOR deterministic match, M from TSV); SKIPS-with-notice when `claude` absent. Fire-presence also demonstrated lightly in Bead 1 panel evidence |
| **AC2** — install at all 3 paths + valid frontmatter AFTER `mindspec setup` | Bead 3 (source: Bead 1) | HERMETIC | `mindspec setup` then three-path `test -f` + `grep '^name:'`/`'^description:'` |
| **AC3** — protocol coverage named in the prompt | Bead 1 | HERMETIC | the AC3 6-way `grep -qiE` set over the plugin SKILL.md |
| **AC4** — vague intent → 0-error spec (`mindspec validate spec`) | Bead 1 (prompt) + Bead 2 (recorded scenario) | ADVISORY | run grill on recorded vague-intent scenario, then `mindspec validate spec <id>` = 0 errors (on demand) |
| **AC5** — det baseline TRUE 0/M on LLM-only classes (cleaned fixtures), achieved by fixture re-phrasing not regex narrowing | Bead 2 | HERMETIC | `bench/grill/det_detect.sh bench/grill/fixtures/*.md` → 0/M LLM-only (negative) AND 100% schema-catchable incl. held-out planted items (positive recall — guards against regex-narrowing) |
| **AC6** — R1 live-repo grounding fires | Bead 2 (harness) / Bead 1 (prompt) | ADVISORY | `run_eval.sh` deterministic match records the GROUNDING problem id |
| **AC7** — R2 invented domain rejected | Bead 2 (harness) / Bead 1 (prompt) | ADVISORY | `run_eval.sh` deterministic match records the invented-domain problem id |
| **AC8** — R6 scenario probing surfaces an edge case | Bead 2 (harness) / Bead 1 (prompt) | ADVISORY | `run_eval.sh` deterministic match records a scenario/edge-case finding |
| **AC9** — binary stays non-interactive (no blocking read) | Bead 3 | HERMETIC | `mindspec spec create <fresh-id> </dev/null` exits 0 |
| **AC10** — grep-provable `ms-spec-create` → `ms-spec-grill` handoff (all replicas + literal) | Bead 3 | HERMETIC | `grep -qE 'ms-spec-grill'` over both replicas + the `lifecycleSkillFiles()` literal |
| **AC11** — `.agents/skills/**` owned, no `bench/**`, setup counts green | Bead 3 | HERMETIC | `grep '- .agents/skills/**'` + `! grep 'bench/'` over OWNERSHIP.yaml + `go test ./internal/setup/...` (12/14) |
