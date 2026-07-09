---
adr_citations:
    - ADR-0040
    - ADR-0037
    - ADR-0036
    - ADR-0035
    - ADR-0034
approved_at: "2026-07-09T10:31:42Z"
approved_by: user
bead_ids:
    - mindspec-9cyu.1
    - mindspec-9cyu.2
    - mindspec-9cyu.3
spec_id: 111-workflow-panel-runner
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - .mindspec/domains/workflow/OWNERSHIP.yaml
        - internal/validate/ownership_wave2_test.go
        - .mindspec/domains/workflow/architecture.md
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - plugins/mindspec/workflows/ms-panel.js
        - .claude/workflows/ms-panel.js
        - plugins/mindspec/embed.go
        - plugins/mindspec/workflow_test.go
        - internal/setup/claude.go
        - internal/setup/claude_test.go
        - .mindspec/domains/workflow/interfaces.md
    - depends_on:
        - 2
      id: 3
      key_file_paths:
        - plugins/mindspec/skills/ms-panel-run/SKILL.md
        - plugins/mindspec/skills/ms-panel-tally/SKILL.md
        - .claude/skills/ms-panel-run/SKILL.md
        - .claude/skills/ms-panel-tally/SKILL.md
        - .mindspec/domains/workflow/runbook.md
---
# Plan: 111-workflow-panel-runner

## ADR Fitness

The sole impacted domain is **workflow** (spec § Impacted Domains: every source
edit — the new `.claude/workflows/**` + `plugins/mindspec/workflows/**` workflow
artifact, `plugins/mindspec/embed.go`, `internal/setup/**`, `internal/validate/**`,
`plugins/mindspec/skills/**`, `.claude/skills/**`, and
`.mindspec/domains/workflow/OWNERSHIP.yaml` — lands under the `workflow`
OWNERSHIP globs; `internal/config` (the 109 `runner:`/`panel:` resolvers),
`internal/panel` + `cmd/**` (the 110 `mindspec panel` verbs + verdict schema),
and `internal/executor` are consumed **read-only as an existing CLI + artifact
contract** and are **not** impacted — 111 adds no source to any of them). Five
ADRs genuinely constrain the plan and are cited; three frequently-adjacent ADRs
were evaluated and **deliberately not cited**. **No bead diverges from any
accepted ADR** — the honest boundaries this spec draws (the workflow is an
adapter, never a second decision authority) are the accepted designs, not
departures from them.

- **ADR-0040 — Orchestration Layering Ratchet** (Domain(s): core, workflow;
  intersects workflow). The license and load-bearing frame: agents integrate at
  the **artifact + CLI contract** level (the `panel.json`/verdict-JSON schemas +
  the `mindspec panel` verbs), never at the prompt-format level, and
  orchestration **runners** are *adapters* behind those contracts with
  **degraded modes** for hosts lacking a capability. Spec 109 landed ADR-0040 on
  `main` (and declared the `runner:` key) before this branch rebased forward, so
  it is citable here (unlike 109's own plan). This plan makes it real: the
  workflow (Bead 2) is the first non-default runner adapter — it targets 110's
  verbs and verdict schema and never re-implements them (R2, R5); the runner
  dispatch (Bead 3) degrades a workflow-less host to the skills path (R6); and
  the workflow ships to the Claude Code target **only** (R8), codex/copilot
  staying skills-path per the capability tiers. This plan **adheres**.
- **ADR-0037 — Panel Gate as Enforced Contract** (Domain(s): workflow,
  execution; intersects workflow). The spine, binding Bead 2. §3's single home
  (`internal/panel.PanelGateDecision`, reached only through `mindspec complete`)
  is **not weakened**: the workflow is a read-side + write-side adapter that
  registers via `mindspec panel create` and returns `mindspec panel verify` /
  `mindspec panel tally` output verbatim, adding **no** second interpreter and
  **never** running `mindspec complete` (structurally pinned — the string
  appears nowhere in the file, AC4 + Bead 2's exact-set test). §8's
  "plain reviewable files, no signing" trust boundary is **extended, not
  changed**, to the codex transcription step: each codex slot persists its raw
  stdout to a tracked `<slot>-round-<N>.codex.log` audit artifact (R3b) so the
  wrapper's verdict is auditable against its source. This plan **adheres**.
- **ADR-0036 — Ownership Discovery** (Domain(s): workflow, validation, doc-sync,
  ownership; intersects workflow). Governs Bead 1's new `.claude/workflows/**`
  claim: `.claude/workflows/ms-panel.js` is governable source (`isDocFile` and
  `isProcessArtifact` both return false for it — verified against
  `internal/validate` below), so editing it unclaimed trips
  `adr-divergence-unowned`. The claim must be present **same-diff-or-earlier**
  than any `.claude/workflows/**` edit (the exact spec-108 `internal/trace` /
  `.golangci.yml` invariant), so it lands in Bead 1 (a real edge before Bead 2's
  workflow file). Also governs the gate-forward doc-sync every bead honors.
  Sound as-is.
- **ADR-0035 — Agent Error Contract** (Domain(s): workflow, execution, core;
  intersects workflow). Constrains Bead 2's failure surfaces: a codex quota
  wall, the substitution branch, a parse failure, and a MISSING slot must be
  guard-style, never silent. The workflow surfaces every failure through
  `mindspec panel verify`'s named-malformed / incomplete report and `mindspec
  panel tally`'s non-zero Block + recovery line (both already ADR-0035-compliant
  in 110), passed through **verbatim** in the workflow result — never as a
  dropped slot the workflow hides. Sound as-is.
- **ADR-0034 — Ceremony Collapse** (Domain(s): workflow; intersects workflow).
  The workflow adds **no new ceremony step and no new gate**: it replaces the
  launch mechanics of an existing panel with one invocation and hands off to the
  same `mindspec complete` merge terminal the skills path uses (the runner
  selection in Bead 3 is a dispatch, not a lifecycle transition). Sound as-is.

**Evaluated, deliberately NOT cited:**
- **ADR-0039 — Flat `.mindspec/` Layout v2** (Domain(s): core, workflow,
  execution, context-system). Real *context* — the workflow's verdict files land
  under the layout-aware `<spec-dir>/reviews/<slug>/` tree — but 111 introduces
  **no** `.mindspec/` layout logic: the panel-dir layout is 110's `mindspec
  panel create` concern (111 passes `<slug>` and the binary resolves the tree),
  and `.claude/workflows/` is a fixed Claude Code install location, not a
  `.mindspec/` layout tier. Citing it would add a non-constraining ADR; omitting
  it keeps `adr_citations` load-bearing (110 cited it because `panel create`
  *writes* the layout-aware dir; 111 does not). `workflow` coverage is
  unaffected — the five cited ADRs all name it.
- **ADR-0030 — Executor Boundary** (Domain(s): execution, validation, lifecycle,
  lint). The spec's ADR-0037 touchpoint draws the honest boundary vs 110
  explicitly: unlike 110's `panel create` (which rev-parses through the executor
  git-I/O boundary), 111 adds **no** executor code — the codex CLI is exec'd by a
  workflow agent's own shell tool, outside `internal/executor`. ADR-0030's
  Domain (`execution`) does **not** intersect `workflow`, so citing it would fire
  `adr-cite-irrelevant` at plan-approve. Mirrors 110's plan omitting ADR-0030.
- **ADR-0027 — MindSpec OTEL-Only** (Domain(s): observability, telemetry,
  recording, extraction). A spec Non-Goal reaffirmation ("no telemetry stream")
  — the workflow emits no metrics by adding none. It constrains no bead, and its
  Domain does not intersect `workflow`, so citing it would fire
  `adr-cite-irrelevant`.

**Coverage check.** `workflow` (the only impacted domain) is covered by all five
citations — ADR-0040/0037/0036/0035/0034 each name `workflow` in their
`Domain(s)` line and all are Accepted, so `checkADRCoverage` finds a cited
Accepted covering ADR and `checkADRCitations` finds no irrelevant citation.

**Divergence report: none.** No bead is better served by a design that departs
from an accepted ADR. The two designs a reviewer might probe are both refused by
the spec itself: (a) letting the workflow run `mindspec complete` / consolidate /
author `consolidated-round-<N>.md` — rejected, it breaks ADR-0037's single-home
invariant (R5, structurally pinned); (b) relying on the documented `schema`
option (a JSON Schema an `agent()` step's structured *return value* must
conform to) as a substitute for `mindspec panel verify` — rejected: `schema`
governs the calling step's in-memory return, not the on-disk
`<slot>-round-<N>.json` file `panel verify` reads and the gate consumes, so a
schema-conformant return is not itself an on-disk artifact guarantee;
conformance of the file is enforced by prompt + the `schema`-constrained
return **plus** deterministic post-hoc `mindspec panel verify` + the R3
same-reviewer re-serialize-or-MISSING ladder (Non-Goals; Bead 2 step 3 uses
`schema` on every verdict-producing `agent()` call for exactly this reason).

## Testing Strategy

**Approach.** The runner is a Claude Code dynamic workflow — a `.js`
orchestration script whose *behavior* (registration → fan-out → parse-retry →
quota-wall substitution → verify/tally-return) executes only inside the Claude
Code runtime with live agents, so it is **not** unit-testable in `go test`. The
automated gates therefore pin the workflow's **artifact identity, structure,
distribution, and ownership** deterministically, and the behavioral requirements
(R2–R5, R3b) are proven by the spec's **Manual e2e** (Bead 2 Validation Proof,
run against a built binary + live agents). Concretely, the Go-testable surface is:

- **Ownership** (Bead 1): `internal/validate.attributeDomain` resolves
  `.claude/workflows/ms-panel.js` to `workflow` — a pure path-glob test mirroring
  spec 108's `TestWorkflowOwnsTraceAndGolangci` (no filesystem read of the
  workflow file: `attributeDomain` matches path strings against the manifest, so
  the test passes on the claim alone, before Bead 2's file exists).
- **Embed + distribution** (Bead 2): `plugins/mindspec.WorkflowFiles()` returns
  the embedded workflow, and `internal/setup.RunClaude` installs it to
  `.claude/workflows/` byte-identical while `RunCodex`/`RunCopilot` do **not**.
- **Structural / anti-laundering** (Bead 2): the `ALLOWED_CLI` allowlist is a
  static literal array admitting **exactly** the four permitted commands (the
  codex entry sandboxed `--sandbox read-only`), `mindspec complete` appears
  nowhere, `.codex.log` and `claude-sub` are present, and a **positive
  enumeration** of every `mindspec`-/`codex`-bearing string literal in the file
  proves each one is exactly one of the four allowlisted forms — closing the
  indirection class a pattern blocklist could smuggle past.

**The AC4 exact-set strengthening (spec-approve round-2 carry-forward #1, further
strengthened by items 2–4 of the plan-panel round-1 fix, and by items 2 and 5 of
the plan-panel round-2 fix).** R5's requirement text
claims `ALLOWED_CLI` admits *exactly* four commands, but spec AC4 only proves the
four are **present** and `mindspec complete` is **absent** — a fifth admitted
command would pass AC4 while falsifying R5. Because ACs are floors, Bead 2 adds
`TestMsPanelWorkflow_AllowedCLIExactSet` (Go test over the embedded workflow
text) that extracts the `ALLOWED_CLI` array literal, parses its string elements,
and asserts the set **equals** the four-element list `{"mindspec panel create",
"codex exec --sandbox read-only --skip-git-repo-check", "mindspec panel verify",
"mindspec panel tally"}` — failing on any extra, any missing, or any rename. The
same test asserts (a) `mindspec complete` is absent from the whole file and (b) a
**positive enumeration** replaces the narrower blocklist approach a prior round
considered: it extracts **every** double-quoted, single-quoted, and
backtick-delimited string literal in the file and asserts every literal
containing `mindspec` — and, separately, every literal containing `codex exec`
— is exactly one of the four `ALLOWED_CLI` strings. This subsumes any
indirection syntax by construction rather than by anticipating each pattern: a
single-quoted concatenation fragment, a backtick template, or a smuggled
non-`complete` verb all fail because the literal that actually contains the
telltale substring isn't, itself, one of the four exact strings. The test also
asserts `buildCommand` (the file's single command-construction chokepoint,
Bead 2 step 1) is defined exactly once and that the enumeration above finds no
`mindspec`-/`codex`-bearing literal **outside** the `ALLOWED_CLI` array itself —
proving command construction routes through that one chokepoint by construction,
not merely by convention. **Round-2 items 2 and 5 reconcile this with Bead 2's
own prescribed call-site code:** every `buildCommand` call site passes one of
four verb identifiers **destructured** from `ALLOWED_CLI` (Bead 2 step 1), never
a retyped command-string literal, and the test additionally pins that exactly
four such identifiers are destructured in one binding — closing both the
round-1 snippet contradiction (a call-site literal would itself have been an
"occurrence outside the array") and the exact-match-bypass reading (a call site
that reused the literal text plus raw concatenation would still equal one of the
four strings, but is now caught by the occurrence-location assertion regardless
of textual match). This strengthens R5 to its full "exactly four" guarantee,
plus a construction-level command-routing guarantee, without a spec edit.

**Per-test proof discipline.** Every new Go test is verified with an anchored
PASS-line grep so a reviewer sees the specific test pass, not a bare package
green: `go test ./PKG -v -run 'TestName$' | grep -q -- '--- PASS: TestName'`. The
`$` anchor on `-run` stops a prefix sweeping a sibling. The three named test
functions use exactly the spec's Acceptance-Criteria names
(`TestWorkflowOwnsClaudeWorkflows`, `TestWorkflowFiles_EmbedsMsPanel`,
`TestClaudeSetup_InstallsWorkflowClaudeTargetOnly`); the fourth
(`TestMsPanelWorkflow_AllowedCLIExactSet`) is the carry-forward floor-raise.
**Grep note:** this machine's `grep` is ugrep; the fixed-string `grep -q` /
`grep -qF` forms below are ugrep-safe as written; any name-anchored file-membership
check uses `/usr/bin/grep -qxF` explicitly. **Any self-check that runs the binary
runs a BRANCH-BUILT binary** (`go build -o /tmp/ms111/mindspec ./cmd/mindspec` —
a directory containing the binary, so it can be prepended to `PATH` for the
Manual e2e's bare `mindspec …` agent-step calls, e.g. `PATH=/tmp/ms111:$PATH`),
never the pre-installed `~/.local/bin/mindspec`, which predates this branch.

**JS syntactic validity (advisory, not a CI gate).** A JS toolchain is present on
this machine (`node v25.2.1`), so Bead 2 records `node --check
.claude/workflows/ms-panel.js` (and the plugin copy) exiting `0`. Per the spec's
Validation Proofs this is **recorded when a JS toolchain is present, not a CI
gate** — CI has no guaranteed node, and the workflow's authority is Claude Code,
not node.

**Consumer ownership (the created-count pin).** Wiring `installWorkflows` into
`RunClaude` grows a fresh Claude setup from 14 to 15 created items, so Bead 2
**owns** its consumers in `internal/setup/claude_test.go`:
`TestRunClaude_FreshSetup`'s `len(r.Created) != 14` and `TestRunClaude_Idempotent`'s
`len(r2.Skipped) != 14` both move to `15` (with the comment updated) in the same
bead — without the update `go test ./internal/setup` goes red. This is why
`internal/setup/claude_test.go` is in Bead 2's `key_file_paths`. `skillFiles()`
still returns 12 skills (workflows are not skills), so
`TestSkillInventory_Twelve` is unaffected.

**Regression.** Full `go test ./...` runs once at **plan time** and again
**pre-`/ms-impl-approve`** — not per bead; per-bead gates run the touched packages
only. Plan-time result (2026-07-08, this post-109 worktree): `go build ./...`
green; the three packages this spec touches — `internal/validate`,
`internal/setup`, `plugins/mindspec` (currently `[no test files]`; Bead 2 adds
the first) — are green. The pre-existing `internal/instruct`
`TestRun_IdleNoBeads` environment-isolation failure (tracked as `z4ps`) is
unrelated and **not** in this spec's surface — 111 touches no
`internal/instruct` code. Git-touching tests run with `GIT_TERMINAL_PROMPT=0`. No
new external dependency, no network access. **Advisory WARN expected:** the repo
has no `source_globs` in `.mindspec/config.yaml`, so `validate plan` emits the
non-gating `missing-source-globs` migration nudge; this is pre-existing and
unrelated to 111.

**Dependency shape (decomposition / the DAG).** Three beads (within the 3–5
optimal band, ≤ the 6 advisory cap), forming the single chain the spec's
compile/gate facts force — **1 → 2 → 3**, depth 3 (= the advisory MaxChainDepth
of 3), with Bead 1 the sole root (parallelism 0.33, above the 0.25 floor). Both
edges are **real**, not ordering wishes:
- **Bead 2 `depends_on: [1]`** — Bead 2 adds `.claude/workflows/ms-panel.js`,
  which trips `adr-divergence-unowned` unless the `.claude/workflows/**` claim
  Bead 1 lands is visible at Bead 2's diffed ref. Divergence reads each domain's
  `OWNERSHIP.yaml` at the diffed ref, so the claim must appear in an **earlier**
  bead (or the same diff); an earlier bead is the cleaner, independently-reviewable
  form (the spec-108 precedent this spec cites). Not a false edge — a parallel
  split would let Bead 2 be diffed before Bead 1 and fail its own per-bead gate.
- **Bead 3 `depends_on: [2]`** — Bead 3's `ms-panel-run` runner branch invokes
  the `/ms-panel` workflow when `runner: claude-code-workflow`. That dispatch is
  only truthful once the workflow artifact exists and ships (Bead 2); merging the
  skill first would leave the dispatch pointing at an unshipped workflow. A real
  functional edge (mirrors 110's Bead 5 → Bead 4 "skills must reference verbs
  that exist").

The single-root chain is the honest shape of a **three-layer adapter** —
gate-enablement (ownership) → artifact + distribution → skill dispatch — not a
decomposition defect: each layer is a compile/gate prerequisite of the next, and
forcing parallelism would introduce a false edge or a dangling-reference hazard.
Because the three beads touch **disjoint** file sets (ownership manifest+test /
`.js`+embed+setup / skills), `validate plan` emits a non-gating
`decomposition-scope-redundancy` low-overlap WARN (R≈0.04 < 0.15) — the expected
shape of a cleanly separated three-domain-slice change, not a decomposition
defect (110's plan noted the same for its five-package split).
**Doc-sync is collision-free by construction:** the three beads append to
**disjoint** `workflow` domain-doc files — Bead 1 → `architecture.md` (the
ownership-claim lineage, joining spec 108's "Ownership claims" section), Bead 2 →
`interfaces.md` (the workflow adapter + embed/install distribution surface), Bead
3 → `runbook.md` (the runner-selection operator procedure) — so even a
hypothetical reordering never conflicts.

**Doc-sync footprint (precise).** The repo declares no `source_globs`, so the
built-in classifier drives doc-sync: only `.go` files under `internal/`/`cmd/`
(non-`_test.go`) count as source requiring a same-diff `workflow` doc region. By
that rule the **only** file strictly forcing a doc region is Bead 2's
`internal/setup/claude.go`; Bead 1's `OWNERSHIP.yaml` classifies as a doc
(`.mindspec/domains/**`) and its test is `_test.go`, and Bead 3's `.js`/SKILL.md
edits are non-`.go` — so those two beads' doc regions are added to honor the
spec's Scope gate-forward planning constraint (every workflow-source bead
documents its change) and are verified by a `git show --name-only` membership
grep, **not** because the doc-sync gate would block them. This mirrors 110's
skills-only Bead 5, which carried a `runbook.md` region under the same planning
constraint.

**Requirement → bead map.** R1 → Bead 2 (the tracked workflow artifact, both
byte-identical copies); R2 → Bead 2 (registration through `mindspec panel
create`); R3 → Bead 2 (fan-out + the anti-laundering re-serialize-or-MISSING
ladder); R3b → Bead 2 (the `.codex.log` audit artifact); R4 → Bead 2 (the
deterministic quota-wall substitution branch); R5 → Bead 2 (verify+tally return +
the `ALLOWED_CLI` allowlist, strengthened to exactly-four by
`TestMsPanelWorkflow_AllowedCLIExactSet`); R6 → Bead 3 (runner dispatch in
`ms-panel-run`); R7 → Bead 3 (skill slimming + the retained judgment sections);
R8 → Bead 2 (embed + Claude-target-only install); R9 → Bead 1 (the
`.claude/workflows/**` claim + `TestWorkflowOwnsClaudeWorkflows`). Every spec
requirement is delivered; the Provenance table maps every spec acceptance
criterion.

## Bead 1: ownership — claim `.claude/workflows/**` for the workflow domain + attribution test

Delivers R9. The `.claude/workflows/ms-panel.js` runner artifact (Bead 2) is
governable source, so its owning domain must claim it **before** it is edited.
This bead lands the claim and the attribution test as a small, independently
reviewable root — the spec-108 `internal/trace` / `.golangci.yml`
same-diff-or-earlier precedent. Doc-sync: `.mindspec/domains/workflow/architecture.md`
(the ownership-claim lineage).

**Steps**
1. Add `- .claude/workflows/**` to the `paths:` list of
   `.mindspec/domains/workflow/OWNERSHIP.yaml`. Place it adjacent to the existing
   `.claude/skills/**` claim (line 21) so the two `.claude/**` sub-tree claims sit
   together; `plugins/mindspec/workflows/**` needs **no** new claim (already
   covered by the existing `plugins/mindspec/**` glob). Verify against
   `internal/validate` that the target file classifies as governable source:
   `isDocFile(".claude/workflows/ms-panel.js")` is false (no `docs/`,
   `.mindspec/`, `project-docs/`, or root-operator-doc prefix) and
   `isProcessArtifact(...)` is false (no `/reviews/` segment, not under `.beads/`
   or `review/`) — so without the claim it would trip `adr-divergence-unowned`.
2. Append `TestWorkflowOwnsClaudeWorkflows` to
   `internal/validate/ownership_wave2_test.go` (reusing the file's existing
   `repoRootForWorkflowManifest(t)` helper, which walks up to the **live**
   committed `.mindspec/domains/workflow/OWNERSHIP.yaml` so removing the claim
   fails the test): call
   `attributeDomain(nil, root, "", ".claude/workflows/ms-panel.js",
   []string{"workflow"})` and assert it returns `("workflow", non-nil Ownership,
   nil err)` — the exact shape of the sibling `TestWorkflowOwnsTraceAndGolangci`.
   The test needs no on-disk workflow file: `attributeDomain` glob-matches the
   path string against the manifest, so it passes on the claim alone.
3. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/architecture.md` (under/after the existing
   "Ownership claims + carve-out cleanup — spec 108 wave 2" section) recording
   the spec-111 `.claude/workflows/**` claim — that dynamic workflows are
   governable source the workflow domain now owns, added same-diff-or-earlier
   than the workflow artifact per ADR-0036 (the spec-108 invariant).

**Verification**
- [ ] `grep -Eq '^[[:space:]]*-[[:space:]]+\.claude/workflows/\*\*' .mindspec/domains/workflow/OWNERSHIP.yaml` exits `0` (spec AC2, R9 — the claim is present)
- [ ] `go test ./internal/validate -v -run 'TestWorkflowOwnsClaudeWorkflows$' | grep -q -- '--- PASS: TestWorkflowOwnsClaudeWorkflows'` (spec AC3, R9)
- [ ] `go test ./internal/validate` exits `0` (whole package green — the existing ownership/divergence/doc-sync tests still pass, proving no manifest-parse regression)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/architecture.md'` (doc-sync: the ownership claim carries a workflow domain-doc region — spec Scope gate-forward constraint)
- [ ] `go build ./...` exits `0`

**Acceptance Criteria**
- [ ] `.mindspec/domains/workflow/OWNERSHIP.yaml` claims `.claude/workflows/**`
  and `attributeDomain` returns `"workflow"` for `.claude/workflows/ms-panel.js`,
  so no later bead editing it trips `adr-divergence-unowned` (spec AC2, AC3, R9)

**Depends on**
None

## Bead 2: the `/ms-panel` workflow adapter — tracked artifact (both copies) + embed + Claude-target-only install + structural/exact-set tests

Delivers R1, R2, R3, R3b, R4, R5, R8. The workflow `.js` is the runner adapter
behind 110's `mindspec panel` verbs + verdict schema and 109's `panel:` mix /
`substitution.claude_sub_on_quota` config; `plugins/mindspec/embed.go` ships it
and `internal/setup/claude.go` installs it to the Claude target only. **Depends
on Bead 1** — the `.claude/workflows/**` claim must be visible at this bead's
diffed ref before `.claude/workflows/ms-panel.js` is added (a real
adr-divergence edge). Doc-sync: `.mindspec/domains/workflow/interfaces.md`.

**Steps**
1. Author `plugins/mindspec/workflows/ms-panel.js` — the Claude Code dynamic
   workflow (a JavaScript orchestration script invocable as `/ms-panel`) whose
   **script coordinates agents and itself does no shell/file I/O** (the
   documented workflow limit; every CLI touch + file write is an `agent()`
   step). It accepts via the documented `args` input `{slug, spec, target,
   bead_id?, round, sha?, lenses[], mix}`.

   **Input hardening, at workflow entry, before any command or path is
   built:** validate `slug`, `spec`, `bead_id` (when present), and every
   `mix[].family` against the same clean-single-path-element contract 110's
   CLI validators apply (reject empty, `.`, `..`, `/`, `\`, and control
   bytes); validate `target` against a **branch-name-safe grammar** (round-2
   item 3 — reject empty, control bytes, a leading `-`, and any construct
   `git check-ref-format` disallows: `..`, `~`, `^`, `:`, `?`, `*`, `[`, a
   trailing `/` or `.lock`, or whitespace) and always pass it to
   `buildCommand` as its own **argv-safe** token — appended as a single
   element, never concatenated into a larger string — so no value that
   survives the grammar check can still widen past one argument; validate
   `target` and `bead_id` additionally reject shell metacharacters (`` ` ``,
   `$(`, `;`, `|`, `&`, newlines) since they flow into a built command line;
   validate `round` is a positive integer. Any
   failure aborts the workflow before Step 2 runs — no agent step, CLI call,
   or file path is constructed from an unvalidated value. The workflow's own
   `slot` ids (`R1`, `R2`, … — generated in Step 3, never user input) are
   drawn from a fixed internal enumeration, not interpolated from `args`.

   Declare the command allowlist as a **static literal array at the top of
   the file** — no template interpolation, no concatenation — so it is
   machine-parseable and cannot smuggle a fifth command:
   ```js
   // ALLOWED_CLI — the exact, exhaustive set of shell commands any /ms-panel
   // agent step may exec. Adding a command here (or building one dynamically)
   // is a gate-integrity change; TestMsPanelWorkflow_AllowedCLIExactSet enforces
   // this set is exactly these four. The lifecycle merge-terminal verb is
   // intentionally absent from this set and from this file entirely — this
   // workflow is an adapter, never a lifecycle mutator (ADR-0037/0040).
   const ALLOWED_CLI = [
     "mindspec panel create",
     "codex exec --sandbox read-only --skip-git-repo-check",
     "mindspec panel verify",
     "mindspec panel tally",
   ];
   const [CMD_PANEL_CREATE, CMD_CODEX_EXEC, CMD_PANEL_VERIFY, CMD_PANEL_TALLY] =
     ALLOWED_CLI;
   ```
   **(Round-2 item 2 — the destructured verb identifiers.)** Every later step
   invokes `buildCommand` with one of these four **identifiers**, never by
   retyping the command string — so the only place any of the four command
   strings exists as a source-level string literal is this one array
   declaration; `TestMsPanelWorkflow_AllowedCLIExactSet` (Step 6) pins both the
   array's exact four-string content and that exactly four verb identifiers
   are destructured from it in one binding.

   The codex entry pins a **read-only** sandbox (no writes, no network side
   effects) — correct because, per Step 3, codex itself never writes a file
   (the wrapper's own `Write` tool does), so `workspace-write` (the codex CLI
   default the skills-path today relies on) grants access this workflow never
   needs; pinning `read-only` makes "codex never writes files" a
   sandbox-enforced guarantee, not only a prompt instruction.

   **`buildCommand(verb, ...args)` — the single command-construction
   chokepoint.** Define one function, alongside `ALLOWED_CLI`, that every
   agent step invokes to obtain the exact string it is told to run: it
   asserts `verb` is one of the four `ALLOWED_CLI` entries (throwing
   otherwise), then assembles the command from a **fixed per-verb
   template** — the template alone supplies each verb's fixed option flags
   (`--spec`, `--target`, `--bead`, `--round` for the panel verbs;
   `--sandbox read-only --skip-git-repo-check` for the codex verb, already
   pinned in the `ALLOWED_CLI` entry itself) — interleaving into that
   template's value slots only the caller-passed **user-derived values**
   (`slug`, `spec`, `target`, `bead_id`, `round`). `args` therefore never
   carries a flag, only these values, and `buildCommand` **rejects any
   element of `args` that starts with a leading `-`** (round-2 item 2's
   argument-injection guard, scoped to values — since no legitimate
   `slug`/`spec`/`target`/`bead_id`/`round` value ever begins with `-`, a
   flag-shaped value such as a `slug` or `target` of `--json` — or an
   attempt to append a second `--sandbox` override after the codex
   prefix — is rejected as an injection attempt, while the template's own
   fixed flags, never passed through `args`, run untouched) — this step's
   validation applied to already-hardened argument values, never free
   operator text. No agent step's prompt hand-assembles or narrates a shell
   command in prose, and **no call site retypes a command string as a
   literal** — every call passes one of the four verb identifiers
   destructured from `ALLOWED_CLI` above, followed only by user-derived
   values; each embeds only `buildCommand(...)`'s return value (e.g. "Run
   exactly: `${buildCommand(CMD_PANEL_CREATE, slug, spec, target)}` — do
   not modify this command"). This is what makes `ALLOWED_CLI` enforcement
   structural: a step's runnable command is fully determined by code that
   can only ever select one of the four prefixes plus one fixed per-verb
   flag template, never by an agent's own interpretation of a looser
   instruction, and never by a call site that reintroduces the command
   string — or any of its flags — as its own literal — and it is what makes
   the positive-enumeration test below (Step 6) hold **by construction**,
   not by accident: since `buildCommand` is the sole assembler of any
   command string and every caller passes it a verb identifier plus values
   rather than a literal or a flag, no `mindspec`- or `codex`-bearing
   literal can exist anywhere in the file outside the `ALLOWED_CLI` array
   declaration itself for that test to miss.

   `sha?` is **advisory-only** (an optional BRIEF display hint); the authoritative
   `reviewed_head_sha` is self-resolved by `mindspec panel create` from
   `--target` at write time (110 R1), so the workflow **never** uses `sha?` to
   set the recorded SHA and works when it is omitted.
2. Registration step (R2): a single `agent()` step that runs
   `buildCommand(CMD_PANEL_CREATE, slug, spec, target, bead_id?, round?)`
   (Step 1's destructured identifier, never the literal string, followed
   only by user-derived values — the fixed `--spec`/`--target`/`--bead`/
   `--round` flags come from `buildCommand`'s own per-verb template, never
   from this call site) — the hardened equivalent of `mindspec panel create
   <slug> --spec <spec> --target <target> [--bead <bead_id>] [--round
   <round>]` — so `panel.json` (round + `reviewed_head_sha` co-bumped by
   construction, `expected_reviewers` / `approve_threshold` stamped from the 109
   resolvers) is written **by the binary**. A re-panel passes the round
   **value** `N+1` (the template supplies the `--round` flag; re-resolving
   the SHA in the same write; prior-round `<slot>-round-<K>.json` files
   untouched). The workflow contains **no** hand-typed `panel.json` schema
   and **no** re-implementation of the round+SHA co-bump — it reads back only the
   BRIEF path `panel create` reports. **This is the single reported layout every
   later step derives its write paths from:** the step captures the panel
   directory (containing `panel.json` + `BRIEF.md`) from that reported path once,
   here, and passes it forward as a workflow variable; Step 3's per-slot verdict
   and `.codex.log` writes are anchored to this one captured directory, never
   independently reconstructed from raw `spec`/`slug` in each slot.
3. Fan-out step + the anti-laundering ladder (R3, R3b): flatten `mix` (the
   resolved `[{family,count}]` from config `panel:`) into a per-slot descriptor
   list — one `{slotId, family, lens}` entry per unit of `count` (`slotId`
   drawn from the fixed `R1, R2, …` enumeration, Step 1), lens assigned from
   `lenses[]` — and fan out via the documented `pipeline(list, itemFn)`
   primitive (the workflows docs' named concurrent fan-out construct:
   "`pipeline()` runs one [agent] per item in a list", bounded by the
   runtime's 16-concurrent-agent cap) rather than an unnamed or undocumented
   parallel construct. Every slot's verdict and (for codex) log path is
   `<panel-dir>/<slot>-round-<N>.json` / `.codex.log`, where `<panel-dir>` is
   the **single directory Step 2's registration call resolved and captured**
   — no slot recomputes `<spec-dir>/reviews/<slug>/` independently from raw
   `spec`/`slug` (the Step 2 hardening note).

   `itemFn` dispatches on `descriptor.family`:
   - A **claude** slot is an `agent()` step prompted with the BRIEF path +
     its lens + the 110 verdict-JSON shape (`reviewer_id`, `verdict`,
     `confidence`, `rationale`, `concrete_changes_required`, `findings`;
     optional top-level `hard_block`), called with the documented `schema`
     option set to that shape so the step's **returned value** is
     schema-conformant, and instructed to also **write** that same
     schema-validated object to the 110 contract path
     `<panel-dir>/<slot>-round-<N>.json` via its `Write` tool. (`schema`
     constrains only the agent step's in-memory return — a value distinct
     from what the `Write` tool puts on disk — so the prompt instruction to
     also write the identical object is still required; see the ADR Fitness
     divergence-report note on the `schema` option's real scope.)
   - A **codex** slot is a **wrapper agent** whose own `agent()` call also
     carries the `schema` option (over its transcribed-verdict return) and
     that: execs the allowlisted `codex exec --sandbox read-only
     --skip-git-repo-check` invocation (`buildCommand(CMD_CODEX_EXEC, ...)`,
     Step 1's destructured identifier) with the
     BRIEF prompt + lens — **read-only**, since codex itself never writes
     (below); **persists codex's unmodified stdout** to
     `<panel-dir>/<slot>-round-<N>.codex.log` (R3b — the string `.codex.log`
     must appear literally in the file, AC5) **before/as** it transcribes;
     **parses that stdout deterministically**: the transcription is accepted
     only if the stdout contains **exactly one** JSON object matching the
     verdict shape — zero objects, more than one object, or a single object
     plus surrounding narrative text all count as **not accepted** and route
     into ladder step (a) below, identically to structurally-invalid JSON;
     and writes the accepted verdict file **itself** via its `Write` tool.
     **Codex never writes files** — neither the verdict nor the log; the
     wrapper writes both, and the `--sandbox read-only` pin (Step 1) makes
     this a sandbox-enforced guarantee, not only a `Write`-tool convention
     (eliminating the sandbox-file-write failure class by construction).

   Verdict conformance of the **on-disk file** — distinct from an agent
   step's `schema`-checked in-memory return — is not something `mindspec
   panel verify` can skip: it is enforced by the prompt shape plus the
   `schema`-constrained return plus deterministic post-hoc `mindspec panel
   verify` (step 4). A rendered verdict can never be replaced by a different
   reviewer's verdict or a re-review — the ladder is:
   - (a) **Parse failure on a *rendered* verdict** (content produced, but
     either not accepted per the one-object rule above or not valid schema
     JSON) → re-prompt the **same** reviewer **exactly once**, feeding back
     that reviewer's own rendered output (for a codex slot, the persisted
     `.codex.log`) with the instruction to re-emit **that same verdict** as
     valid JSON **without re-reviewing** — a serialization retry keeping the
     **same slot id and same family** `reviewer_id` (e.g. `R4 codex` stays
     `R4 codex`, **never** `claude-sub`).
   - (b) **Still unparseable (or still not exactly one object) after the
     single re-prompt** → the slot **fails CLOSED to a MISSING verdict** (no
     file written) → an incomplete panel → the gate Blocks. Never
     substituted, never replaced by another reviewer's verdict.
   - (c) **Substitution (step 4) is reserved EXCLUSIVELY** for a quota wall in
     which *no* verdict content was ever rendered — a rendered-but-malformed
     verdict is out of substitution's reach.
4. Quota-wall substitution branch (R4): when a codex wrapper reports a quota wall
   (the codex CLI usage-limit signal) **with no verdict rendered** (the step-3(c)
   precondition) **and** config `panel.substitution.claude_sub_on_quota == true`,
   substitute a **claude** `agent()` for that slot in the same round, **keeping
   the slot id** and writing `reviewer_id: "<slot> claude-sub"` (the string
   `claude-sub` must appear literally, AC4). When the flag is `false`, **leave the
   slot unfilled** → a missing verdict at `panel verify` → the gate Blocks (the
   workflow never fabricates or silently skips a slot). A code branch driven by
   the wrapper's returned status — **no** human judgment on the workflow path.
   Then the return step (R5): one `agent()` step runs
   `buildCommand(CMD_PANEL_VERIFY, slug)` and another runs
   `buildCommand(CMD_PANEL_TALLY, slug)` (Step 1's destructured identifiers,
   never the literal strings), and the workflow
   **returns their combined stdout verbatim** (unmodified, not re-rendered or
   paraphrased) as its single structured **result**. The workflow does **not**:
   run `mindspec complete` (the string appears nowhere in the file — not a
   command, not a comment, AC4); perform tally **consolidation** (semantic dedup
   + criticality ranking — 110 R8 skill judgment); author
   `consolidated-round-<N>.md`; or mutate `panel.json` beyond `panel create`.
5. Ship the byte-identical dogfood copy + embed + install (R1, R8): copy
   `plugins/mindspec/workflows/ms-panel.js` to `.claude/workflows/ms-panel.js`
   **byte-for-byte** (the tracked copy Claude Code reads). In
   `plugins/mindspec/embed.go` add `//go:embed workflows/*` into a new
   `workflowsFS embed.FS` var and a `WorkflowFiles() map[string]string` accessor
   mirroring `SkillFiles()` — walking `workflows/`, keyed by **file basename**
   (e.g. `"ms-panel.js"` → content), map rebuilt fresh per call. In
   `internal/setup/claude.go` add `installWorkflows(workflowsDir, workflowsRel,
   pluginmindspec.WorkflowFiles(), check, r)` mirroring `installSkills`'
   create/refresh/skip/notice disposition (write `<workflowsDir>/<basename>`),
   and call it from `RunClaude` after `installSkills` (writing to
   `.claude/workflows`). Do **not** add any workflow install to `RunCodex` /
   `RunCopilot` (they install only to `.agents/skills/` — dynamic workflows are a
   Claude Code capability, ADR-0040 tiers). Update the created-count consumers in
   `internal/setup/claude_test.go`: `TestRunClaude_FreshSetup` `14 → 15` and its
   comment, `TestRunClaude_Idempotent` `len(r2.Skipped) 14 → 15`.
6. Add `plugins/mindspec/workflow_test.go` (the package's first test file) with
   `TestWorkflowFiles_EmbedsMsPanel` (asserts `WorkflowFiles()["ms-panel.js"]` is
   non-empty and **equals** the on-disk `workflows/ms-panel.js` read via the
   embed FS — pinning embed == plugin copy) and
   `TestMsPanelWorkflow_AllowedCLIExactSet` (the carry-forward floor-raise, now
   strengthened per items 2–4 of the plan-panel round-1 fix and the
   identifier reconciliation of round-2 item 2): extract the
   `ALLOWED_CLI` array literal from `WorkflowFiles()["ms-panel.js"]`, parse its
   double-quoted string elements, assert the set **equals exactly**
   `{"mindspec panel create","codex exec --sandbox read-only
   --skip-git-repo-check","mindspec panel verify","mindspec panel tally"}`;
   assert `mindspec complete` is absent from the whole content; **pin the
   destructured identifier count** (round-2 item 2): assert the file declares
   exactly one destructuring binding of the shape `const [ID1, ID2, ID3, ID4]
   = ALLOWED_CLI;` — four identifiers wide, right-hand side literally
   `ALLOWED_CLI` (the identifier names themselves are not pinned, only that
   there are exactly four and the source is the allowlist array) — so every
   `buildCommand` call site's verb argument is provably a reference to one of
   these four identifiers, never a retyped literal; then run the
   **positive-enumeration anti-laundering check**, which replaces the prior
   blocklist-of-indirection-patterns approach: extract every double-quoted,
   single-quoted, and backtick-delimited string literal in the file, and for
   every extracted literal containing the substring `mindspec`, assert the
   **full literal**, trimmed, is exactly one of the three allowlisted
   `mindspec panel <verb>` strings above. This subsumes the narrower blocklist
   by construction — a single-quoted concatenation fragment (`'mindspec ' +
   verb`) fails because `'mindspec '` alone isn't one of the three exact
   strings, a backtick template (`` `mindspec ${verb}` ``) fails because its
   raw literal text isn't an exact match, and any smuggled non-`complete`
   lifecycle verb (e.g. a literal `"mindspec panel abandon"`) fails by set
   non-membership — without needing to anticipate the indirection syntax in
   advance. Extend the same extracted-literal pass to every literal containing
   `codex exec`: each must equal the one allowlisted codex string exactly (so a
   second, unsandboxed `codex exec` invocation elsewhere in the file also fails
   the test). Finally, assert **command construction routes through the single
   builder**: `buildCommand` is defined exactly once, and the two prior passes'
   full inventory of `mindspec`/`codex`-bearing literals contains **only** the
   four `ALLOWED_CLI` entries themselves (inside the array literal) and no
   other occurrence — proving, given `buildCommand` is the file's sole command
   assembler and every call site passes it a destructured identifier rather
   than a literal (Step 1), that every runnable command an agent step is told
   to run was built from one of the four fixed prefixes.

   **Closing the exact-match bypass (round-2 item 5).** A bare "every
   `mindspec`-bearing literal equals one of the four allowed strings" check is,
   on its own, satisfiable by a call site that reuses the literal text
   directly — e.g. `"mindspec panel create" + " " + rawArg`, bypassing
   `buildCommand` entirely — because the reused literal trivially equals one
   of the four exact strings; string equality alone would let it pass. The
   identifier-count pin above is the mechanism that closes this: since every
   legitimate call site references a destructured identifier and never
   retypes the command string, **any** occurrence of the literal text found
   outside the `ALLOWED_CLI` array declaration is itself a test failure —
   independent of whether that occurrence happens to match one of the four
   strings. A bypassing call site of this shape is caught by the
   occurrence-location assertion, not by string equality, so an exact-literal
   reuse can no longer masquerade as compliant.

   Add
   `TestClaudeSetup_InstallsWorkflowClaudeTargetOnly` to
   `internal/setup/claude_test.go`: `RunClaude(tmp, false)` writes
   `.claude/workflows/ms-panel.js` **byte-identical** to
   `pluginmindspec.WorkflowFiles()["ms-panel.js"]`, while `RunCodex(tmp2,false)`
   and `RunCopilot(tmp3,false)` create **no** `.claude/workflows/**` and no
   `.agents/workflows/**` file. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/interfaces.md` documenting the `/ms-panel` runner
   adapter — its `args` contract, the input-hardening validation, the
   `buildCommand` chokepoint, the `ALLOWED_CLI` exactly-four allowlist (incl.
   the codex read-only sandbox pin), the codex-wrapper `.codex.log` audit
   artifact, the same-reviewer re-serialize-or-MISSING ladder, the quota-wall
   substitution branch, and that the workflow embeds + installs to the Claude
   target only.

**Verification**
- [ ] `test -f .claude/workflows/ms-panel.js && test -f plugins/mindspec/workflows/ms-panel.js && diff -q .claude/workflows/ms-panel.js plugins/mindspec/workflows/ms-panel.js` exits `0` (spec AC1, R1/R8 — both tracked copies present and byte-identical; combined with the embed test [embed == plugin] and the install test [installed == embed], all four copies are transitively identical)
- [ ] `W=.claude/workflows/ms-panel.js; grep -q 'ALLOWED_CLI' "$W" && for c in 'mindspec panel create' 'mindspec panel verify' 'mindspec panel tally' 'codex exec --sandbox read-only --skip-git-repo-check'; do grep -qF "$c" "$W" || exit 1; done && grep -q 'claude-sub' "$W" && ! grep -qF 'mindspec complete' "$W"` exits `0` (spec AC4, R2/R4/R5; the codex entry's `--sandbox read-only` pin is item 4 of the plan-panel round-1 fix)
- [ ] `grep -qF '.codex.log' .claude/workflows/ms-panel.js` exits `0` (spec AC5, R3b)
- [ ] `go test ./plugins/mindspec -v -run 'TestWorkflowFiles_EmbedsMsPanel$' | grep -q -- '--- PASS: TestWorkflowFiles_EmbedsMsPanel'` (spec AC6, R8)
- [ ] `go test ./plugins/mindspec -v -run 'TestMsPanelWorkflow_AllowedCLIExactSet$' | grep -q -- '--- PASS: TestMsPanelWorkflow_AllowedCLIExactSet'` (carry-forward #1/#3, strengthened per items 2–4 of the plan-panel round-1 fix and items 2/5 of the round-2 fix: ALLOWED_CLI is EXACTLY the four commands including the sandboxed codex entry, no `mindspec complete`, exactly four verb identifiers are destructured from ALLOWED_CLI (round-2 item 2), a positive enumeration of every `mindspec`-/`codex`-bearing literal in the file matches only the four allowlisted strings with zero occurrences outside the array (closing the exact-match bypass, round-2 item 5), and command construction is proven to route through the single `buildCommand` chokepoint — strengthens AC4 beyond a present/absent grep or a pattern blocklist)
- [ ] `go test ./internal/setup -v -run 'TestClaudeSetup_InstallsWorkflowClaudeTargetOnly$' | grep -q -- '--- PASS: TestClaudeSetup_InstallsWorkflowClaudeTargetOnly'` (spec AC7, R8 — Claude target writes it byte-identical; codex/copilot do not)
- [ ] `go test ./internal/setup` exits `0` (whole package green — proves the created-count consumer updates [14→15] landed; a skipped update leaves this red)
- [ ] `go test ./plugins/mindspec` exits `0`
- [ ] `node --check .claude/workflows/ms-panel.js && node --check plugins/mindspec/workflows/ms-panel.js` exits `0` (advisory — recorded because `node v25.2.1` is present; NOT a CI gate, per spec Validation Proofs)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/interfaces.md'` (doc-sync)
- [ ] `go build ./...` exits `0`
- [ ] **Manual e2e (spec Validation Proof) — requires the Claude Code runtime + live agents; NOT automatable in CI.** A live codex reviewer cannot be forced to render malformed JSON or hit a quota wall on cue, so the malformed/multi-object/quota-wall branches below run against a **codex PATH-shim test double**, not the real codex CLI; the happy-path and result-passthrough check runs against the real codex CLI (or the shim's `healthy` scenario, interchangeably).

  **Setup (once), in a scratch repo (post-109/110 base) with `.mindspec/config.yaml` `runner: claude-code-workflow`:**
  1. `go build -o /tmp/ms111/mindspec ./cmd/mindspec` — a **directory** containing the branch-built binary (not a single file), so it can be prepended to `PATH`. Every bare `mindspec …` call the workflow's agent steps make must resolve to this build, never the pre-installed `~/.local/bin/mindspec`. Launch the Claude Code session that runs `/ms-panel` with `/tmp/ms111` prepended to `PATH` (e.g. `PATH=/tmp/ms111:$PATH claude`), so every agent-step shell inherits the resolution.
  2. Write a codex PATH-shim: an executable file named `codex` at `/tmp/ms111-shim/codex`, placed **earlier** on `PATH` than the real codex CLI for the scenario runs below (`PATH=/tmp/ms111-shim:/tmp/ms111:$PATH`). The shim reads a scenario name from an env var (`MS111_CODEX_SCENARIO`) and prints one canned stdout payload per scenario:
     - `healthy` — a single valid verdict JSON object matching the 110 shape.
     - `malformed-once` (round-2 item 1 — the exact verbatim payload pair, canonically comparable): on its **first** invocation for a given slot, prints ONE schema-valid verdict JSON object wrapped in disallowed surrounding narrative — exactly:
       ```
       Here is my review of the panel.

       {"reviewer_id": "R1 codex", "verdict": "APPROVE", "confidence": 0.9, "rationale": "Looks solid overall.", "concrete_changes_required": [], "findings": [], "hard_block": false}

       Let me know if further changes are needed.
       ```
       — rejected by Step 3's "exactly one object" rule (a single object plus surrounding narrative text counts as not accepted), but the embedded object is well-formed and canonically decodable, giving the first attempt a real operand to compare against the re-serialize below (the round-1 text's "narrative prose (no JSON)" framing left no JSON to canonically decode for the value-fidelity check — this is the round-2 fix). On the **second** invocation (the re-prompt), prints the same verdict as a clean, standalone re-serialize with no surrounding text — exactly:
       ```
       {"reviewer_id": "R1 codex", "verdict": "APPROVE", "confidence": 0.9, "rationale": "Looks solid overall.", "concrete_changes_required": [], "findings": [], "hard_block": false}
       ```
       — tests the successful-reserialize path with a canonically comparable pair.
     - `malformed-always` — narrative prose (no JSON) on every invocation — tests the fail-to-MISSING path only (no value-fidelity comparison implied; there is nothing to re-serialize).
     - `multi-object` — two concatenated JSON objects on stdout (each individually valid JSON, but not a single unambiguous object) — tests that Step 3's "exactly one object" rule routes this into the parse-failure ladder exactly like non-JSON prose.
     - `quota-wall` — prints the established usage-limit signal (`ERROR: You've hit your usage limit`) with **no** verdict content, on every invocation — tests substitution (R4), never the parse-failure ladder.
  3. **Seed `.mindspec/config.yaml`'s `panel:` block (round-2 item 4).** The
     zero-config default (`panel.reviewers: [{claude,3},{codex,3}]`,
     `internal/config`'s documented `TestLoad_ZeroConfigPanelModelsLoopDefaults`
     baseline) makes `mindspec panel create`'s config-resolved
     `expected_reviewers` **6**, independent of whatever `mix` a given
     `/ms-panel` invocation below actually fans out to — with no `panel:`
     block set, every run's `panel verify` would read short of 6 and report
     incomplete regardless of whether the reviewer ladder behaved correctly,
     so run 1 (which must read **complete** to prove the happy path) and run 3
     (which must read **incomplete** because of its own MISSING slot, not
     because of an unrelated count mismatch) can't be told apart. Before run 1,
     set:
     ```yaml
     panel:
       reviewers:
         - family: claude
           count: 2
         - family: codex
           count: 1
       approve_threshold: "3"
     ```
     (matching run 1's 3-reviewer `{claude:2,codex:1}` mix exactly). Before
     each of runs 2–7 (each a 1-reviewer mix), rewrite `panel.reviewers` to a
     single `{family: <that run's mix family>, count: 1}` entry with
     `approve_threshold: "1"`, so every run's `expected_reviewers` equals its
     own `mix` total and each run's completeness observable reflects the
     ladder's actual behavior on that run, not a config/mix count mismatch. (111
     reads none of 112's `gates:` config — out of scope per Risks/Sequencing —
     so no `gates:` block is seeded here.)

  **Scenario runs (each a separate `/ms-panel` invocation against a throwaway `--target` — a live codex process can't be forced to switch behavior mid-panel, so each failure mode gets its own slug/round):**
  1. **Happy path + result passthrough (R3, R3b, R5):** `/ms-panel {slug: "ms111-e2e-happy", spec: "<demo-spec-id>", target: "<throwaway-branch>", round: 1, lenses: ["L1","L2","L3"], mix: [{family:"claude",count:2},{family:"codex",count:1}]}` with `MS111_CODEX_SCENARIO=healthy` (or the real codex CLI, PATH-shim absent). Confirm: 3 verdict files land at `<panel-dir>/<slot>-round-1.json`; the codex slot also leaves `<slot>-round-1.codex.log`; the workflow **result** carries the `mindspec panel verify` report + `mindspec panel tally` preview **verbatim** (unmodified CLI stdout), with `mindspec complete` never invoked (confirm via the run transcript + `git log` showing no merge).
  2. **Parse-failure re-prompt, successful reserialize, with canonical VALUE fidelity (R3 steps 1–2, carry-forward #2 as strengthened by item 6, payload shapes fixed by round-2 item 1):** `/ms-panel {slug: "ms111-e2e-reserialize", spec: "<demo-spec-id>", target: "<throwaway-branch>", round: 1, lenses: ["L1"], mix: [{family:"codex",count:1}]}` with `MS111_CODEX_SCENARIO=malformed-once`. Confirm the re-emitted verdict keeps the **same** `reviewer_id` (same slot, same family — never `claude-sub`); extract the single embedded JSON object from the `.codex.log`-persisted first-invocation stdout (the narrative-wrapped payload above) and decode it **canonically as JSON** alongside the final verdict file (not by string equality — key order and whitespace may differ), and confirm `verdict`, `hard_block`, and `concrete_changes_required` are equal across the two — a re-serialize, not a re-review.
  3. **Parse-failure to MISSING (R3 step 2):** same shape as run 2 with `slug: "ms111-e2e-missing"` and `MS111_CODEX_SCENARIO=malformed-always`. Confirm **no** verdict file is written for the slot, `mindspec panel verify` reports the panel incomplete, and the gate Blocks.
  4. **Ambiguous multi-object stdout treated as a parse failure (Step 3's one-object rule, item 6):** same shape as run 2 with `slug: "ms111-e2e-multiobject"` and `MS111_CODEX_SCENARIO=multi-object`. Confirm this is **not** silently accepted as the first of the two objects — it is routed into the same re-prompt-once-then-MISSING ladder as `malformed-once`/`malformed-always`.
  5. **Quota-wall substitution, flag true (R4):** same shape as run 2 with `slug: "ms111-e2e-quota-true"` and `MS111_CODEX_SCENARIO=quota-wall`, config `panel.substitution.claude_sub_on_quota: true`. Confirm the slot's verdict shows `reviewer_id: "<slot> claude-sub"`.
  6. **Quota-wall, flag false (R4):** identical to run 5 but `slug: "ms111-e2e-quota-false"` and `claude_sub_on_quota: false`. Confirm the slot is left **missing** — no fabricated verdict, no skip.
  7. **Input hardening rejects unsafe args before any command runs (item 5; target-specific cases added per round-2 item 3):** run each of the following as a separate invocation and confirm, for every case, that the workflow aborts before Step 2's registration call runs — no `panel.json`, no `BRIEF.md`, no directory created under the traversal target (mirrors 110's own slug-validation rejection table) — and that no command containing the offending value was ever passed to `buildCommand`:
     - **7a — slug traversal:** `/ms-panel {slug: "../../etc", spec: "<demo-spec-id>", target: "<throwaway-branch>", round: 1, lenses: ["L1"], mix: [{family:"claude",count:1}]}`.
     - **7b — target argument injection / shell metacharacter:** identical shape with `target: "--upload-pack=touch /tmp/pwned;"` (a leading-dash flag-injection attempt fused with a shell metacharacter, rejected by both the leading-dash and metacharacter clauses of Step 1's hardening).
     - **7c — target control byte:** identical shape with `target: "main\u0000\u0007"` (an embedded NUL + BEL control-byte pair, written here as escape sequences since raw control bytes cannot be committed to a markdown source file; rejected by the branch-name-safe grammar's control-byte clause).

**Acceptance Criteria**
- [ ] The workflow exists at both tracked locations byte-identical; it accepts
  `{slug,spec,target,bead_id?,round,sha?,lenses[],mix}`, derives its mix only
  from `mix`, never uses `sha?` to set the recorded SHA, and never itself execs a
  CLI or writes a file (script vs agent) (spec AC1, R1)
- [ ] Registration is through `mindspec panel create` only — no hand-written
  `panel.json`, no re-implemented co-bump (R2)
- [ ] Each mix slot produces a verdict at `<slot>-round-<N>.json`; codex slots
  are wrapper agents that write both the verdict and the `.codex.log`, codex
  writing neither (sandbox-enforced by the `--sandbox read-only` pin, not only a
  prompt convention); a codex transcription is accepted only when its stdout
  contains **exactly one** verdict JSON object — zero, multiple, or
  narrative-wrapped objects are treated as a parse failure; a rendered-but-malformed
  verdict resolves only to the same reviewer's re-serialized verdict (value-fidelity
  checked on **all** gate-relevant fields — `verdict`, `hard_block`,
  `concrete_changes_required` — via canonical decode, not string equality) or a
  MISSING slot — never a substitution or a re-reviewed APPROVE (spec AC5, R3, R3b)
- [ ] Quota-wall substitution is a deterministic branch honoring
  `claude_sub_on_quota`, keeping the slot id + `claude-sub` `reviewer_id`; a
  `false` flag leaves the slot missing (R4)
- [ ] The workflow returns `panel verify` + `panel tally` output verbatim;
  `ALLOWED_CLI` admits **exactly** the four commands, the codex entry sandboxed
  `--sandbox read-only` (enforced by `TestMsPanelWorkflow_AllowedCLIExactSet`'s
  exact-set + positive-enumeration assertions, not just present/absent greps);
  every agent step's runnable command is constructed only via the single
  `buildCommand` chokepoint, called with a destructured `ALLOWED_CLI` verb
  identifier — never a retyped literal, never free-form per-step prose
  (round-2 item 2); `mindspec complete`
  appears nowhere; no consolidation / `consolidated-round-*.md` / `panel.json`
  mutation beyond `create` (spec AC4, R5)
- [ ] `WorkflowFiles()` embeds the workflow and `mindspec setup` (Claude target)
  installs it byte-identical to the embed while codex/copilot do not; the
  fresh-setup created-count consumers are updated in the same bead (spec AC6,
  AC7, R8)
- [ ] `slug`, `spec`, `bead_id`, `target`, `mix[].family`, and `round` are
  validated against a traversal/control-byte/shell-metacharacter contract at
  workflow entry, before any command or write path is built; verdict/log write
  paths for every slot derive from the single panel directory Step 2's
  registration call resolved, never from independent per-slot reconstruction
  (item 5, hardening)

**Depends on**
Bead 1

## Bead 3: skills — runner dispatch in `ms-panel-run` + the workflow-result note in `ms-panel-tally`, judgment sections retained

Delivers R6 and R7. Edited on the **post-110** skill structure (110's Bead 5
already slimmed step 0 to `mindspec panel create` and removed
`ms-panel-tally`'s decision-matrix table — see Risks/Sequencing): this bead adds
the runner branch and gates the mechanized launch prose behind it, without
deleting the skills-path mechanics or any judgment section. The four files are
the **workflow** domain (`plugins/mindspec/skills/**` + their byte-identical
`.claude/skills/**` mirrors — each pair edited identically); doc-sync lands in
`.mindspec/domains/workflow/runbook.md`. **Depends on Bead 2** — the runner
branch dispatches to a shipped `/ms-panel` workflow.

**Steps**
1. In `ms-panel-run/SKILL.md` (both the `plugins/…` file **and** its
   `.claude/skills/…` mirror, edited identically), add a **Runner dispatch**
   section near the top that reads the effective `runner:` value via `mindspec
   config show` (spec 109) and branches, **explicitly labelling each section
   workflow-path or skills-path**:
   - `claude-code-workflow` → **compose the slot lenses** (the retained judgment
     step, § Slot lens defaults) then invoke the `/ms-panel` workflow **once**
     with the resolved `{slug, spec, target, bead_id?, round, lenses[], mix}`;
   - `claude-code-skills` (the **default** until the workflow path is proven) →
     the existing manual launch path runs **unchanged**;
   - `external` → a documented out-of-scope **stub** (no adapter ships; the panel
     runs human/skills-path per ADR-0040 degraded modes).
   State that a host lacking workflow capability degrades to the skills path.
2. In `ms-panel-run/SKILL.md`, **gate behind the `claude-code-workflow` branch**
   (superseded-for-workflow-path, **not deleted**) the **Launch the panel**,
   **Codex failure detection (deterministic)**, **Working directory matters**,
   and the codex-specific **Anti-patterns**: label them as the
   `claude-code-skills` default-path mechanics, and state that on the workflow
   path they are superseded by the schema+wrapper mechanism (R3–R4) and the
   single `/ms-panel` invocation (R6) — do **not** re-state the bash detection
   for the workflow path. **Keep § Slot lens defaults intact** as the workflow
   path's sole authoring step. The grep AC requires the literal strings
   `claude-code-workflow`, `/ms-panel`, `Slot lens defaults`, and `Launch the
   panel` to all remain present.
3. In `ms-panel-tally/SKILL.md` (both files), add a note that on the workflow
   path the per-slot verdict table + decision arrive **pre-rendered in the
   workflow result** (an already-run `mindspec panel tally`), so the skill's job
   narrows to Step 4 consolidation + the merge terminal. **Leave unchanged** the
   judgment sections: **Consolidate** (semantic dedup + ranking, authoring
   `consolidated-round-<N>.md`), **§ Artifact gates**, **§ After a halt —
   recovery**, **§ Escape hatch**, and **§ Abandon procedure**. The grep AC
   requires the literal strings `workflow`, `## Artifact gates`, `Consolidate`,
   `After a halt`, and `Escape hatch` to all remain present.
4. Keep the two `.claude/skills/**` mirrors **byte-identical** to their
   `plugins/mindspec/skills/**` sources (they are today; a `diff -q` gate below
   pins it).
5. Doc-sync (workflow): add a bead-unique region to
   `.mindspec/domains/workflow/runbook.md` noting the panel operator procedure now
   selects the runner via the 109 `runner:` key — `claude-code-workflow` invokes
   the `/ms-panel` workflow once (registration + fan-out + verify/tally-return
   mechanized), `claude-code-skills` retains the hand-driven launch, `external`
   is a documented stub — with the judgment sections (Slot lens defaults,
   Consolidate, Artifact gates) retained in the skills.

**Verification**
- [ ] `R=plugins/mindspec/skills/ms-panel-run/SKILL.md; grep -q 'claude-code-workflow' "$R" && grep -Eq '/ms-panel([^-a-z]|$)' "$R" && grep -q 'Slot lens defaults' "$R" && grep -q 'Launch the panel' "$R"` exits `0` (spec AC8, R6/R7 — plugins copy; the `/ms-panel` check is word-boundary-anchored, item 9 of the plan-panel round-1 fix, so it can't false-pass on the pre-existing `/ms-panel-tally`/`/ms-panel-run` substrings)
- [ ] `T=plugins/mindspec/skills/ms-panel-tally/SKILL.md; grep -q 'workflow' "$T" && grep -q '## Artifact gates' "$T" && grep -q 'Consolidate' "$T" && grep -q 'After a halt' "$T" && grep -q 'Escape hatch' "$T"` exits `0` (spec AC9, R7 — plugins copy)
- [ ] `R=.claude/skills/ms-panel-run/SKILL.md; grep -q 'claude-code-workflow' "$R" && grep -Eq '/ms-panel([^-a-z]|$)' "$R" && grep -q 'Slot lens defaults' "$R" && grep -q 'Launch the panel' "$R"` exits `0` (the `.claude` mirror carries the same runner branch + retained sections)
- [ ] `T=.claude/skills/ms-panel-tally/SKILL.md; grep -q 'workflow' "$T" && grep -q '## Artifact gates' "$T" && grep -q 'Consolidate' "$T" && grep -q 'After a halt' "$T" && grep -q 'Escape hatch' "$T"` exits `0` (the `.claude` mirror)
- [ ] `diff -q plugins/mindspec/skills/ms-panel-run/SKILL.md .claude/skills/ms-panel-run/SKILL.md && diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md` exits `0` (the mirrors stay byte-identical — no drift between the two copies)
- [ ] `git show --name-only HEAD | /usr/bin/grep -qxF '.mindspec/domains/workflow/runbook.md'` (doc-sync)
- [ ] `go build ./...` exits `0` (no Go code touched — the tree still builds)

**Acceptance Criteria**
- [ ] `ms-panel-run` reads the runner value from config and branches:
  `claude-code-workflow` composes lenses then invokes `/ms-panel` once;
  `claude-code-skills` runs the existing launch **unchanged**; `external` is a
  documented stub — the mechanized launch prose is gated behind the runner branch
  (superseded for the workflow path), **not deleted**, and § Slot lens defaults
  and § Launch the panel both survive (spec AC8, R6, R7)
- [ ] `ms-panel-tally` notes the workflow-result pre-rendered path while all four
  named judgment sections (Consolidate, Artifact gates, After a halt — recovery,
  Escape hatch) survive; both `plugins` and `.claude` copies are edited
  identically and stay byte-identical (spec AC9, R7)

**Depends on**
Bead 2

## Risks / Sequencing

**Hard prerequisites: specs 109, 110, and 112 land on `main` before 111's
beads (spec Open Questions, resolved).** This plan is authored on a **post-109**
base and **assumes 110 and 112 have merged** before 111 enters Implementation
Mode — per the spec, `plan approve` for 111 may run only after both 109 and 110
have merged to `main` **and** `spec/111-workflow-panel-runner` has been
rebased/merged-forward onto that base (the spec-108→107 ordering pattern); the
standing model-tiering protocol adds 112 (per-gate `gates:` config) to the same
forward-rebase.

- **110 (`mindspec-fbel.1–.5`, plan-approved, not yet merged)** provides the
  `mindspec panel create | verify | tally` verbs + the verdict-file/slot schema
  the workflow consumes as an **unchanged CLI + artifact contract**, and 110's
  Bead 5 slims `ms-panel-run` step 0 (to `mindspec panel create`, removing the
  hand-typed `panel.json` schema and the skill-authored verdict-output block) and
  `ms-panel-tally` (removing the `| Condition | Action |` decision matrix). Bead 3
  branches **on top of that slimmed structure** — it adds the runner dispatch and
  gates the surviving launch mechanics behind it. **Rebase point:** if 111 is
  implemented before 110 merges, Bead 3's skill edits would target the pre-110
  (un-slimmed) skills; the forward-rebase resolves this so Bead 3 sees the
  post-110 sections named in its Steps.
- **112 (`mindspec-lma4.1–.3`, plan-approved, not yet merged)** adds the per-gate
  `gates:` map + `substitutes` + the recorded `panel.Panel.Gate` field. **111
  reads none of 112's new symbols**: the workflow consumes `runner:`,
  `panel:` mix, and `substitution.claude_sub_on_quota` (all 109 surfaces 112
  keeps), so a post-112 rebase touches no 111 line.
- **Mechanical-rebase safety.** 111's Go work is **additive on new files/symbols**:
  Bead 1 appends a test + one OWNERSHIP line + a doc region; Bead 2 adds two new
  `.js` files, a new `embed.go` var + accessor, a new `installWorkflows` call +
  its two count-consumer edits, and a new `plugins/mindspec` test file; Bead 3
  edits only skills + a doc. **No 111 bead edits a function 110 or 112 edits** —
  110 touches `internal/panel` / `cmd/mindspec` / `internal/instruct` /
  `internal/validate` (`ValidateSpec`), 112 touches `internal/config` /
  `internal/panel` / `internal/complete` / `cmd/mindspec`; 111's only overlap is
  the **file** `internal/validate/ownership_wave2_test.go` (110 does not touch it)
  and the workflow OWNERSHIP.yaml (additive line), so the forward-rebase is a
  clean fast-forward on every 111-owned path.

**Return-value schema is not an on-disk-artifact guarantee (honest limit).**
The Claude Code workflows docs document a `schema` option on `agent()` steps —
a JSON Schema the step's *returned* value must conform to — but no enforcement
of what an agent subsequently writes to disk with its `Write` tool; the two are
separate values. Bead 2 uses `schema` on every verdict-producing `agent()` call
(both claude reviewer steps and the codex wrapper's own transcription step) to
constrain the in-memory return, but does not treat that as a substitute for an
on-disk artifact guarantee: verdict conformance of the actual
`<slot>-round-<N>.json` file rests on the agent prompt + the
`schema`-constrained return + `mindspec panel verify`'s deterministic
parse-status report + the R3 same-reviewer re-serialize-or-MISSING ladder. The
`.js` extension is inferred from the documented "JavaScript script" language;
the `.claude/workflows/` location, the `/ms-panel` slash-command invocation,
and the `pipeline()` fan-out primitive (Bead 2 step 3) are all documented. If a
future Claude Code release changes the workflow file format, the adapter — not
the contract — is what updates (ADR-0040 portability).

**Spec-approve round-2 carry-forward #4 (`../../adr/` relative-link convention).**
Info-only, pre-existing since round 1, not a regression, known repo-wide. It
concerns anchored links inside `spec.md`'s `## ADR Touchpoints`; the plan writes
no such links (it cites ADRs by ID in frontmatter + prose), so there is **no
plan action** — recorded here for disposition completeness.

**Panel-substitution posture (orchestration note, not a plan constraint).** Per
the standing model-tiering protocol, the 9-reviewer spec/plan panels add 3×
Fable; when Codex is quota-walled, substitute Sonnet/Claude personas rather than
block.

## Provenance

Spec ACs are numbered in the order they appear in the spec's Acceptance Criteria
checklist. Every spec AC traces to a bead; every requirement R1–R9 is delivered
(R9 → Bead 1; R1–R5, R3b, R8 → Bead 2; R6, R7 → Bead 3).

| Acceptance Criterion (spec) | Verified By |
|---------------------------|-------------|
| AC1 — both tracked workflow copies present + byte-identical (`diff -q`) (R1, R8) | Bead 2 verification (`diff -q`; embed test [embed == plugin] + install test [installed == embed] make all four copies transitively identical) |
| AC2 — `.claude/workflows/**` claim present in workflow `OWNERSHIP.yaml` (R9) | Bead 1 verification (anchored `grep -Eq`) |
| AC3 — `TestWorkflowOwnsClaudeWorkflows`: `attributeDomain` → `"workflow"` for the workflow file (R9) | Bead 1 verification (PASS-line grep) |
| AC4 — workflow declares `ALLOWED_CLI` with the four commands present (codex sandboxed `read-only`), honors `claude-sub`, and `mindspec complete` appears nowhere (R2, R4, R5) | Bead 2 verification (structural grep) **strengthened** by `TestMsPanelWorkflow_AllowedCLIExactSet` (exactly-four exact-set incl. the sandboxed codex entry + destructured-identifier-count pin + a positive enumeration of every `mindspec`-/`codex`-bearing literal + proof that command construction routes through the single `buildCommand` chokepoint, closing the exact-match-bypass reading — carry-forwards #1/#3, items 2–4 of the round-1 plan-panel fix, items 2/5 of the round-2 plan-panel fix) |
| AC5 — workflow persists each codex slot's `<slot>-round-<N>.codex.log` (`grep -qF '.codex.log'`) (R3b) | Bead 2 verification (grep) |
| AC6 — `TestWorkflowFiles_EmbedsMsPanel`: `WorkflowFiles()` returns the embedded workflow (R8) | Bead 2 verification (PASS-line grep) |
| AC7 — `TestClaudeSetup_InstallsWorkflowClaudeTargetOnly`: Claude setup writes it byte-identical; codex/copilot do not (R8) | Bead 2 verification (PASS-line grep) |
| AC8 — `ms-panel-run` gains the runner branch + `/ms-panel`, retains § Slot lens defaults + § Launch the panel (R6, R7) | Bead 3 verification (plugins + `.claude` mirror greps, word-boundary-anchored per item 9) |
| AC9 — `ms-panel-tally` notes the workflow-result path; Consolidate / Artifact gates / After a halt / Escape hatch survive (R7) | Bead 3 verification (plugins + `.claude` mirror greps) |
| AC10 — tree builds + touched packages green (`go build ./... && go test ./internal/validate/... ./internal/setup/... ./plugins/mindspec/...`) | every bead's `go build ./...` + per-package `go test`; full `go test ./...` regression at plan time and pre-`/ms-impl-approve` (Testing Strategy) |
| Validation Proof — `node --check` on both copies (where a JS toolchain is present) | Bead 2 verification (advisory, `node v25.2.1` present; not a CI gate) |
| Validation Proof (Manual, live agents) — verdict + `.codex.log` files; parse-failure re-prompt with same `reviewer_id` **and canonical, all-gate-fields value fidelity** (`verdict`/`hard_block`/`concrete_changes_required`, carry-forward #2 as strengthened by item 6 and given a canonically-comparable narrative-wrapped-JSON payload pair by round-2 item 1); ambiguous multi-object stdout treated as a parse failure; MISSING-not-substituted; quota-wall `claude-sub` vs missing per flag; target metacharacter/argument-injection/control-byte rejection cases (round-2 item 3); result carries verify+tally verbatim, `mindspec complete` never invoked; run against a branch-built binary on `PATH`, a codex PATH-shim test double per scenario, and a scratch config seeded with a matching `panel:` reviewer mix so completeness observables discriminate correctly (round-2 item 4) (R2–R5, R3b; items 5–8 of the round-1 plan-panel fix; items 1, 3, 4 of the round-2 plan-panel fix) | Bead 2 verification (Manual e2e, per-scenario runs) |
| Structural workflow proof — calls the verbs, never mutates the lifecycle | Bead 2 verification (AC4 grep + `TestMsPanelWorkflow_AllowedCLIExactSet`) |
