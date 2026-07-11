---
adr_citations:
    - ADR-0037
    - ADR-0035
approved_at: "2026-07-11T17:21:36Z"
approved_by: user
bead_ids:
    - mindspec-s2mf.1
    - mindspec-s2mf.2
    - mindspec-s2mf.3
    - mindspec-s2mf.4
spec_id: 116-panel-message-escaping
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/termsafe/termsafe.go
        - internal/termsafe/termsafe_test.go
        - cmd/mindspec/config.go
        - .mindspec/domains/workflow/OWNERSHIP.yaml
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - internal/panel/gate.go
        - internal/panel/hostile_fields_test.go
        - internal/panel/leaf_imports_test.go
        - internal/panel/panel_test.go
        - .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md
    - depends_on:
        - 2
      id: 3
      key_file_paths:
        - cmd/mindspec/panel.go
        - cmd/mindspec/panel_test.go
    - depends_on:
        - 2
      id: 4
      key_file_paths:
        - internal/complete/panel_advisory.go
        - internal/complete/panel_advisory_test.go
        - internal/instruct/panelstate.go
        - internal/instruct/panelstate_test.go
---
# Plan: 116-panel-message-escaping

Close `mindspec-fl91`: no byte an attacker can plant in a panel directory can
forge a terminal line or a SessionStart transcript line through any panel-gate
render path, bead or non-bead — and every clean (printable-ASCII) panel renders
byte-identically to today. Mechanism per OQ1's unanimous resolution (Option A):
the safe-set/quote rule relocates to a new stdlib-only `internal/termsafe`
leaf, `internal/panel/gate.go` escapes every attacker-influenceable field at
its construction/interpolation point (including inside `RawMergeFence`), and
the sink-local sibling renders that bypass `Decision.Message` (R3) are escaped
where they render. No gate decision (`Action`) changes for any fact set. All
line references below are pinned against branch HEAD `02a39c8f`.

**ADR-0030 frontmatter caveat (spec M5, deliberate):** ADR-0030 is prose-only
context in this plan — its `Domain(s)` header (`execution, validation,
lifecycle, lint`) does not intersect this spec's sole impacted domain
(workflow), so listing it in `adr_citations` would trip `adr-cite-irrelevant`
(`checkADRCitations`' literal case-folded intersection — the `intersectFold`
helper, `internal/validate/plan.go:756-781`, called at `:481`;
`checkADRCitations` itself heads at `:466`). ADR-0037 (`workflow, execution`) and
ADR-0035 (`workflow, execution, core`) both cover workflow and are cited.

## Decomposition and land order

Four beads: serial spine **Bead 1 → Bead 2**, then two parallel siblings
**Bead 3a ∥ Bead 3b**, each depending ONLY on Bead 2 (longest chain 3,
within the ≤3 bound, with a wider parallel base at Bead 2 → {3a, 3b};
`work_chunks` declares the edges — chunk ids 3 and 4 map positionally to
Bead 3a and Bead 3b):

- **Bead 1 (the escaper home — substrate)** — the new `internal/termsafe`
  leaf package (the relocated safe-set/quote rule + AC6 property tests),
  `cmd/mindspec`'s `escapeConfigValue` reduced to a thin delegation, and the
  workflow OWNERSHIP claim of `internal/termsafe/**`. Zero behavior change
  for every existing caller; every other bead imports what this bead creates.
- **Bead 2 (the construction boundary — the core fix)** — depends on Bead 1:
  `gate.go` field escaping at every R2 interpolation site + inside
  `RawMergeFence`, the leaf-invariant doc-comment amendment (`gate.go:19-23`,
  folding the `:14-17` retired-hook correction) and the dated ADR-0037
  amendment recording the A′ rejection, the AC4 field-sweep matrix, and the
  AC7 import-pin test. `Decision.Message` becomes safe-by-construction for
  all four sinks.
- **Bead 3a (the CLI bypass sinks)** — depends on Bead 2, parallel sibling
  of 3b: the R3(a)-(d) sibling-site escapes in `cmd/mindspec/panel.go` + the
  AC1 headline pin. Proves AC1.
- **Bead 3b (the complete + instruct bypass sinks)** — depends on Bead 2,
  parallel sibling of 3a: the `internal/complete` R3(f)/(g) escapes + the
  AC2 pin, and the `internal/instruct` R3(e)/(h) escapes + the AC3 pin.
  Proves AC2 and AC3. Whichever of 3a/3b merges LAST runs the AC-global
  sweep on the final tree (each runs it for its own packages regardless).

**The Bead 2 ↔ Bead 3a/3b dependency (decided: both siblings depend on
Bead 2; AC1 lands in Bead 3a, AC2/AC3 in Bead 3b).** The AC1 fixture plants
hostility in BOTH halves of the render: the `panel.json` `bead_id` flows
through `PanelGateDecision` into `d.Message` (clean only after Bead 2's
construction-boundary escaping), while the sibling `reviewed_head_sha` /
verdict-slot / verdict-string fields are rendered directly by
`renderPanelVerify`/`renderPanelTally` from `Result`/`GateFacts`, bypassing
`Message` entirely (clean only after Bead 3a's sink-local escapes). So AC1
goes green only when both land — placing the AC1 test in Bead 2 would strand
a signature test that cannot pass inside its own bead (the mis-cut the
decomposition rule forbids), and landing Bead 3b before Bead 2 would leave
its AC2/AC3 tests equally stranded (complete's Block body IS `d.Message`
with the hostile slug/abandon-reason interpolated by `gate.go`, and
instruct's `verdict()` passes `d.Message` through at `panelstate.go:135` —
both assert full-text cleanliness that only Bead 2's escaping provides).
The dependencies are real produced-state data flow, not just AC1's
convenience. **Merging 2+3a/3b was considered and REJECTED** on the merge
heuristic: zero production-file overlap (Bead 2 touches only
`internal/panel` + the ADR; the 3a/3b pair touches `cmd/mindspec`,
`internal/complete`, `internal/instruct` — R_scope = 0), and the two sides
have different review characters (a security-boundary change inside a
dependency-clean leaf plus an invariant amendment, vs mechanical
same-pattern sink sweeps across three consumer packages). Bead 2 is
independently green at its own merge point: its signature ACs (AC4's
field-sweep over the pure `PanelGateDecision`, AC7's import pin) live
entirely inside `internal/panel`, and the AC5 pins stay green under Bead 2
alone (the R4 fixture-audit + fence-strip/rebuild analysis — escaping is a
no-op on every clean fixture, and `sanitizeNonBeadDecision` rebuilds legs
5/5b wholesale from `target`/`gitErr`, discarding the constructed text).

**Bead 1 is NOT merged into Bead 2** (also considered): merging would shrink
the chain to 2 but puts `cmd/mindspec/config.go` churn inside the
`internal/panel` security bead (zero file overlap between them — nothing for
the merge heuristic to claim), and Bead 1's output is consumed by EVERY
downstream bead (`cmd/mindspec`'s delegation lands in Bead 1 and serves
Bead 3a; Bead 2 imports `termsafe` in `gate.go`;
`internal/complete`/`internal/instruct` import `termsafe` in Bead 3b),
making it genuinely shared substrate — the 115 Bead-1 pattern. It is not
trivial-work-only: the relocation carries the AC6 property suite (identity,
quoting, idempotence) and the ownership claim + proof.

**The bypass-sink bead IS split into 3a ∥ 3b (ADOPTED at the plan panel —
S3 major; this plan's first draft rejected the split, and that call is
FLIPPED here).** The first draft reasoned "all three sub-parts are the same
class, each is small, and a split buys extra panel rounds without decoupling
anything." The panel's S3 finding showed that undercounts the decoupling and
overcounts the cost: the split has NO chain-depth cost (both sub-beads
depend only on Bead 2, so the longest chain stays 3 — the split only widens
the parallel base) and the decoupling is genuine — zero file overlap (3a
touches only `cmd/mindspec`; 3b only `internal/complete` +
`internal/instruct`), zero inter-dependency (neither imports nor tests
anything the other produces; they share only the Bead-1 escaper), and two
different sink families with different test idioms (CLI stdout/exit-action
drives vs seam-driven gate errors and the transcript composite). Each
sub-bead lands its own packages green independently at its own merge point
with its own signature AC (3a: AC1; 3b: AC2+AC3) — no proof test spans the
3a/3b boundary, so neither half is the stranded mis-cut the decomposition
rule forbids. The split line is exactly the first draft's own sanctioned
fallback: cmd/mindspec (AC1) vs complete+instruct (AC2+AC3), both
`depends_on: [2]`.

**Reviewer/fixer scratch discipline (inherit into every bead brief and
reviewer prompt):** reviewers and fixers MUST use ABSOLUTE `/tmp` scratch
paths (or `t.TempDir()` inside Go tests) for any file they create, and must
NEVER write relative `.mindspec/` (or any relative repo) paths — the agent
harness resets cwd between bash calls, and a relative write from a reviewer
has previously corrupted SIBLING worktrees, which `mindspec complete` then
auto-committed past review. Reviewer verdict paths must be ABSOLUTE. Verify
the bead worktree is CLEAN (`git status --porcelain` empty) before every
`mindspec complete`.

**Toolchain note:** run the CI-matched `gofmt` (go.mod pins go 1.23.0 —
Go 1.19+ gofmt reformats doc comments). In doc comments, avoid backtick code
spans containing shell-escape sequences; the relocated `termsafe` doc contract
describes escapes in plain ASCII prose (the spec-113 gofmt gotcha).

**AC discriminator SHA is restated as-written.** Every AC proof anchors to
the spec-init SHA `8f9c9ccf`, where all six new test names and the string
`termsafe` have ZERO `*.go` hits (re-verified at plan time: repo-wide grep,
0 hits). Bead briefs must NOT "fix" it to a newer SHA — the RED-today claims
are made against that exact commit.

## Bead 1: The escaper home — `internal/termsafe` leaf, `escapeConfigValue` delegation, workflow ownership claim (AC6 + OWNERSHIP proof)

**Goal:** the safe-set/quote rule exists exactly once, in a home every
consumer can import, with `cmd/mindspec`'s observable behavior byte-unchanged.

**Scope**

New package `internal/termsafe` (production file + test file),
`cmd/mindspec/config.go` (delegation only),
`.mindspec/domains/workflow/OWNERSHIP.yaml` (one claim line). No other file
changes; no behavior change anywhere.

**Steps**

1. **Create `internal/termsafe/termsafe.go`** — exported
   `Escape(s string) string` carrying the EXACT relocated implementation from
   `escapeConfigValue` (`cmd/mindspec/config.go:114-121`): iterate runes; if
   any rune is outside printable ASCII `[0x20, 0x7e]`, return
   `strconv.Quote(s)`; else return `s` unchanged. The package imports EXACTLY
   `strconv` (AC6's stdlib-only falsifier) and carries the relocated semantic
   contract as its doc comment (safe set = printable ASCII, strictly tighter
   than unicode.IsPrint so Trojan-Source bidi/zero-width/homoglyph runes
   escape too; whole-string single-line quote on any violation; NUL, ESC,
   DEL, C1 incl. U+009B, newline, invalid UTF-8 all covered; idempotent on
   the control-byte class — quote output for ASCII input is itself printable
   ASCII; printable non-ASCII re-quotes cosmetically on a second pass, a
   display concern, not a safety one). **AC6 discriminator caution
   (spec-pinned):** the relocated loop must carry the literal comparison
   `r < 0x20 || r > 0x7e` — the AC6 single-home grep keys on that exact
   code-level condition (today exactly one non-test hit,
   `cmd/mindspec/config.go:116`); reformulating the condition requires
   updating the AC6 proof in the same commit.
2. **Reduce `escapeConfigValue` to a thin delegation** — its body becomes
   `return termsafe.Escape(s)`; the doc contract at `config.go:94-113` stays
   byte-unchanged (it documents observable behavior, which does not change —
   R5's falsifier covers every input, and `config show` output is pinned by
   the existing `cmd/mindspec` config tests, which must pass untouched).
3. **New `internal/termsafe/termsafe_test.go`** — named test
   **`TestTermsafeEscape_SafeSetQuoteAndNoOp`** (AC6) pinning: (a)
   printable-ASCII identity, byte-for-byte, including punctuation-heavy
   values (the spec's `approve_threshold: n-1` style); (b) NUL, ESC, CSI
   (U+009B), DEL, newline, and invalid-UTF-8 inputs each quoted into a single
   line whose every byte is printable ASCII; (c) idempotence on the
   control-byte class (`Escape(Escape(x)) == Escape(x)` for every control-byte
   fixture).
4. **Claim `internal/termsafe/**` in `.mindspec/domains/workflow/OWNERSHIP.yaml`**
   — consumers-based placement (all four consumers are workflow-owned per
   that file's existing `internal/complete/**`, `internal/instruct/**`,
   `internal/panel/**`, `cmd/**` claims; the spec 115 AC10 rationale).
5. **Package sweep** — `go build ./...`, the `cmd/mindspec` suite green with
   zero semantic modification (delegation is observationally identity), and
   CI-matched `gofmt` clean.

**Verification**

- [ ] AC6 proof EXACTLY as written in the spec: `grep -q 'func TestTermsafeEscape_SafeSetQuoteAndNoOp' internal/termsafe/*_test.go && go test ./internal/termsafe -run 'TestTermsafeEscape_SafeSetQuoteAndNoOp' -v && grep -q 'termsafe' cmd/mindspec/config.go && [ "$(grep -rl 'r < 0x20 || r > 0x7e' --include='*.go' cmd internal | grep -v _test | wc -l | tr -d ' ')" = "1" ] && grep -rl 'r < 0x20 || r > 0x7e' --include='*.go' cmd internal | grep -v _test | grep -q '^internal/termsafe/'` (run from the repo root — the path-prefix check assumes relative `grep -rl` output).
- [ ] OWNERSHIP single-claimant: `grep -l 'internal/termsafe' .mindspec/domains/*/OWNERSHIP.yaml` prints exactly `.mindspec/domains/workflow/OWNERSHIP.yaml`.
- [ ] `go build ./... && go test ./internal/termsafe ./cmd/mindspec` — green with NO semantic expectation change in existing tests.
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- The safe-set/quote rule has exactly one non-test implementation, in
  `internal/termsafe`, importing nothing beyond the standard library (AC6).
- `escapeConfigValue`'s observable behavior is unchanged for every input;
  every `config show` byte unchanged (R5).
- `internal/termsafe/**` is claimed by exactly one domain: workflow.

**Depends on**
None (first bead).

## Bead 2: The construction boundary — `gate.go` + `RawMergeFence` field escaping, leaf-invariant amendment (ADR-0037 + doc comments), AC4 field-sweep + AC7 import pin

**Goal:** `Decision.Message` is safe-by-construction for all present and
future renderers: every attacker-influenceable string is escaped exactly
once, at its interpolation point, with no `Action` change for any fact set
and every existing pin green unmodified.

**Scope**

`internal/panel/gate.go` (the escaping + both doc-comment revisions), the
`internal/panel/panel_test.go:388-394` leaf-note comment revision (comment
only — outside the pinned function body, so AC5's no-removed-assertion-lines
check is untouched), two new test files (suggested
`internal/panel/hostile_fields_test.go`, `internal/panel/leaf_imports_test.go`),
and the dated ADR-0037 amendment. `internal/panel`'s decision matrix,
thresholds, and `GateFacts` semantics byte-unchanged.

**Steps**

1. **Escape every R2-enumerated field at its `gate.go` interpolation site**
   via `termsafe.Escape` (the new import — the one leaf-invariant change):
   - **`f.BeadID`** — leg 0 (`:148-149`), leg 5 (`:199-201`, both
     interpolations), leg 5b (`:212-215`, both interpolations).
   - **`f.GitErr`** — leg 5b: interpolate
     `termsafe.Escape(f.GitErr.Error())` via `%s` (replacing the raw `%v` at
     `:213-215`; on the bead path this error wraps
     `rev-parse bead/<BeadID>: …`, re-embedding a hostile bead ID).
   - **`slug`** (`f.Reg.Slug()` = `filepath.Base(dir)`,
     `internal/panel/panel.go:210` — a disk directory name, NOT validated for
     `internal/complete`/`internal/instruct`'s scan-matched dirs) — legs
     2/3/4/8/9/9.5/10.
   - **`p.AbandonReason`** — leg 3 (`:176-181`; escape the trimmed non-empty
     reason — the mindspec-authored `(no reason recorded — …)` fallback is
     template prose, exempt).
   - **`short(p.ReviewedHeadSHA)`** — leg 6: escape AFTER `short()`, at the
     leg-6 interpolation site (`:225`; `short()` itself is defined at
     `:346-352`) — the 7-byte truncation still admits a 5-byte CSI intro, so
     what renders is `termsafe.Escape(short(...))`.
   - **`f.WorktreePath` and each `f.UserDirt` entry** — leg 7 (`:234-238`):
     per-entry escaping; the template's `\n  ` per-path separators stay REAL
     newlines (the templates own their newlines, interpolated fields own
     none).
   - **Verdict-file names** — leg 8: escape each `v.File` and each
     `res.Malformed` element INSIDE `presentVerdictFiles` before joining —
     per element, never the joined whole (escaping a joined string with one
     hostile element would quote the separators too, the per-field-not-
     per-message discipline).
   - **Slot names** — leg 9 (`f.Res.HardBlocks`) and leg 9.5
     (`UnresolvedVerdicts` slots): per-element before join, same discipline.
   - **`f.HeadSHA`** — escaped for uniformity (DECIDED; the spec offers this
     at zero cost): it is always hex from `rev-parse` so escaping is a no-op,
     and taking it makes the reviewable invariant clean — "every interpolated
     runtime string in `PanelGateDecision` passes `termsafe.Escape` exactly
     once"; only mindspec-authored constants (e.g. `SkipPanelEnv`), template
     prose, and ints remain unescaped.
   - **Exempt, with evidence (spec R2):** `round`, all counts/thresholds
     (ints); `ConsolidatedName(round)` (pure function of an int,
     `internal/panel/tally.go:38`).
2. **Escape inside `RawMergeFence`** (`gate.go:67-72`): the `beadID`
   parameter escapes at its interpolation, covering every Block leg's fence
   append (legs 2/4/6/7/8/9/9.5/10). `termsafe.Escape("") == ""` keeps
   `RawMergeFence("")` byte-identical, so `sanitizeNonBeadDecision`'s
   fence-strip suffix match (`cmd/mindspec/panel.go:829-831`) still matches —
   the R4 non-interaction argument, pinned by AC5.
3. **Doc-comment revisions** (one commit-level work item, spec ADR
   Touchpoints): revise the `gate.go:19-23` leaf note — `internal/panel` now
   imports exactly ONE internal package, the stdlib-only pure-string
   `internal/termsafe`; the invariant's recorded purpose (no config coupling,
   no git/status I/O, decision purity) is preserved while its letter changes,
   and the invariant is now machine-checked (AC7) rather than comment-only.
   Fold in the one-line `:14-17` correction: the PreToolUse hook is RETIRED
   (`internal/complete/panel_advisory.go:45-47`) — the in-binary gate is the
   sole enforcer; drop the "hook as backstop invokes the identical decision"
   claim. Also revise the `internal/panel/panel_test.go:388-394`
   config-free-leaf comment to name the amended invariant (comment lines
   above the func — no assertion-line removal inside the six pinned
   functions).
4. **ADR-0037 dated amendment** — append to the amendment chain following
   the file's `> **Amendment (YYYY-MM-DD, spec NNN — label):**` convention,
   stamped with the bead's actual land date: (a) `internal/panel` now imports
   exactly one internal package, the stdlib-only pure-string
   `internal/termsafe`; letter changed, recorded purpose preserved; (b) the
   A′ rejection, verbatim intent from OQ1 — the leaf's own local-copy
   precedent (`artifactPaths`, `gate.go:486-492`) duplicates INERT DATA whose
   drift is low-stakes and locally visible, whereas a SECURITY PREDICATE
   duplicated in two homes drifts silently and weakens the guarantee at all
   four sinks at once; the AC6 single-home grep + AC7 import-pin test replace
   the comment-only invariant with machine checks; (c) the §8 posture — this
   is output hygiene inside the trust boundary, not tamper-proofing; decision
   matrix, thresholds, hatches (§7), and the fail-open/fail-closed asymmetry
   (§6) all byte-unchanged.
5. **AC4 field-sweep matrix** — NEW named test
   **`TestPanelGateDecision_HostileFieldsEscaped`** in `internal/panel`
   (suggested new file `hostile_fields_test.go`): a table over legs
   0/2/3/4/5/5b/6/7/8/9/9.5/10 planting the hostile pattern
   (`NUL + ESC/CSI + newline + a forged "recovery: …" line`) in each
   R2-enumerated field for the leg(s) that interpolate it, asserting (a)
   every returned `Message` passes the clean triple — no raw `0x00`, no raw
   `0x1b`, no forged standalone line — with the templates' REAL newlines
   exempt; (b) `Action` matches the clean-fixture baseline leg-for-leg; (c)
   leg 7's intended multi-line layout survives — the per-path `\n  ` lines
   and the fence's leading `\n` are REAL newlines in the message (the
   whole-message-collapse falsifier); (d) the no-double-escape falsifier,
   operationalized: for each hostile field, the rendered `Message` contains
   the single-quoted `termsafe.Escape(field)` literal at each of that leg's
   interpolation sites, and the nested form
   `termsafe.Escape(termsafe.Escape(field))` appears nowhere.
6. **AC7 import pin** — NEW named test
   **`TestPanelLeafImports_StdlibPlusTermsafeOnly`** in `internal/panel`
   (suggested new file `leaf_imports_test.go`): parse the package's non-test
   `*.go` files (`go/parser`, imports-only mode) and assert the ONLY
   `github.com/mrmaxsteel/mindspec`-prefixed import across them is
   `internal/termsafe` — any future second internal import fails a test, the
   same way ADR-0030's boundary is test-enforced by
   `internal/lint/boundary_test.go`.
7. **AC5 pins re-run** — the six R4-named existing tests pass with NO
   semantic modification (the R4 argument: every existing fixture is
   printable ASCII, so construction-boundary escaping is a no-op; the
   non-bead sanitizer's leg-5/5b detection keys on template prose
   (`cmd/mindspec/panel.go:839`, `:854`) that field-escaping never alters,
   and rebuilds those legs wholesale).

**Verification**

- [ ] AC4: `grep -q 'func TestPanelGateDecision_HostileFieldsEscaped' internal/panel/*_test.go && go test ./internal/panel -run 'TestPanelGateDecision_HostileFieldsEscaped' -v`.
- [ ] AC7: `grep -q 'func TestPanelLeafImports_StdlibPlusTermsafeOnly' internal/panel/*_test.go && go test ./internal/panel -run 'TestPanelLeafImports_StdlibPlusTermsafeOnly' -v`.
- [ ] AC5 EXACTLY as written: `go test ./cmd/mindspec -run 'TestSanitizeNonBeadDecision|TestPanelVerbs_NonBeadGitErrHostileTargetEscaped|TestPanelTally_NonBeadHostileTargetEscapedAndQuoted|TestPanelVerbs_DecisionIsPanelGateDecision|TestPanelTally_ExitCodeTracksDecision' -v && go test ./internal/panel -run 'TestPanel_GateFieldDecisionInertAllEnumValues' -v`, and `git diff 8f9c9ccf -- cmd/mindspec/panel_test.go internal/panel/panel_test.go` shows no removed assertion lines within those six functions.
- [ ] ADR amendment present: `grep -n 'termsafe' .mindspec/adr/ADR-0037-panel-gate-enforced-contract.md` ≥ 1 hit (0 today), and the amendment text names the A′ rejection.
- [ ] `go build ./... && go test ./internal/panel ./cmd/mindspec ./internal/complete ./internal/instruct` — full consumer sweep green with zero semantic expectation change outside the new suites.
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- Every R2-enumerated field escapes exactly once at construction; no clean
  bead panel's rendered output differs by one byte; `Action` unchanged for
  every fact set (AC4, R2).
- The amended leaf invariant is machine-checked (AC7) and recorded in
  ADR-0037 with the A′ rejection (M-level, spec ADR Touchpoints).
- All six R4 pins green without semantic modification (AC5).

**Depends on**
Bead 1 (imports `termsafe.Escape` — the package does not exist before
Bead 1; AC4's assertion (d) is phrased against `termsafe.Escape` output).

## Bead 3a: The CLI bypass sinks — `cmd/mindspec` sibling renders + the AC1 headline pin

**Goal:** every `cmd/mindspec` render of a panel-dir-sourced field that
bypasses `Decision.Message` is escaped at its render site with the shared
escaper, and the fl91 headline pin (AC1) goes green end-to-end.

**Scope**

`cmd/mindspec/panel.go` (R3(a)-(d) sibling sites; header/argv/exit-action
sites explicitly untouched — fenced inline below) and
`cmd/mindspec/panel_test.go` (the AC1 test). Parallel sibling of Bead 3b:
zero file overlap, zero inter-dependency; this sub-bead lands its own
packages green independently at its own merge point, with Bead 2 (not 3b)
as its only prerequisite.

**Steps**

1. **`cmd/mindspec/panel.go` — R3(a)-(d) sibling escapes** (all via
   `escapeConfigValue`, which now delegates to `termsafe`; these fields come
   from `Result`/`GateFacts` directly, never from `Message`, so no nesting is
   possible):
   - (a) the **bead-path** raw `%v` of `facts.GitErr` at `:531`
     (`could not verify live tip: %v`) — mirror its non-bead twin at `:529`:
     `escapeConfigValue(facts.GitErr.Error())` via `%s`.
   - (b) `res.Panel.ReviewedHeadSHA` rendered raw in every
     `renderPanelVerify` branch (`:517-535`).
   - (c) `renderPanelVerify`'s malformed-file list (`:501-503`) — per-element
     escape before the `strings.Join`.
   - (d) `renderPanelTally`'s per-slot lines — `v.Slot` and `v.Verdict` at
     `:649` (`Verdict` is `ToUpper(TrimSpace(<raw JSON string>))`,
     `internal/panel/tally.go:383` — ESC survives `ToUpper`), `res.Malformed`
     at `:652` (per-element), and the raw `sc.Slot` labels at `:678`/`:682`
     (whose *changes* are already escaped — only the label is added).

   **Inline scope fence (do NOT touch — same file, pages below the (d)
   edits, machine-checked in this bead's own Verification):** the two
   exit-action renderers are OUT of scope and render same-looking `%s`
   slugs/messages a pattern-matching implementer could wrongly "make
   consistent": `tallyExitAction`'s `slug` interpolation (`:707-708`) is
   complete's/tally's ARGV, control-byte-validated at `:299-334`, never
   `panel.json` content; `tallyExitActionNonBead` (`:736-754`) is already
   escaped + shell-quoted (spec-113-final G2); and `:756-770` is
   `shellQuoteTarget`'s doc comment + body (`func` at `:768-770`) — spec Out
   of Scope. The header `slug` at `:500`/`:640` is inside the two functions
   being edited but is equally NOT escaped: for the CLI verbs it is always
   byte-equal to the validated argv (`findPanelRegistration` matches
   `reg.Slug() == slug`, `:371-380`).
2. **AC1 headline pin** — NEW named test
   **`TestPanelVerbs_BeadHostileBeadIDEscaped`** in
   `cmd/mindspec/panel_test.go`, the bead-path analogue of
   `TestPanelVerbs_NonBeadGitErrHostileTargetEscaped` (`:966`). **Helper
   discipline (DECIDED — duplicate, do not hoist):** `assertClean` is a LOCAL
   closure inside the `:966` function (`:971`); hoisting it out would remove
   lines within an AC5-pinned function and trip AC5's
   no-removed-assertion-lines diff check — so the new test defines its OWN
   copy of the clean-triple closure (no raw `0x00`, no raw `0x1b`, no
   standalone forged `recovery: forged` line — over combined stdout+error),
   leaving the six pinned functions byte-untouched. Fixture: a **bead**
   `panel.json` with `bead_id = "mindspec-x\x00\x1b[31m\nrecovery: forged"`
   plus hostile R3 siblings (a `reviewed_head_sha` carrying ESC, a verdict
   file whose slot name carries ESC, a hostile verdict string), driven
   through `panel verify` and `panel tally` — Warn AND Block legs, so both
   `tallyExitAction` branches (`:701-712`) render — asserting clean on every
   captured stream.

   **Fixture physics (what hostility each field can PHYSICALLY carry):**
   filename-derived fields — the panel slug (= directory name) and verdict
   filenames (whose slot portion feeds `v.Slot`/`res.Malformed`) — CANNOT
   carry NUL (the filesystem rejects it), and a NEWLINE in a verdict
   filename silently defeats the fixture: `verdictFileRE`'s `(.+)` slot
   group (`internal/panel/tally.go:33`) does not match `\n`, so a
   newline-bearing verdict filename is skipped, the hostile slot never
   renders, and the assertion passes VACUOUSLY. Filename-derived hostility
   is therefore ESC/CSI bytes only; the full NUL + newline + forged-line
   pattern rides in the JSON-sourced fields (`bead_id`,
   `reviewed_head_sha`, verdict strings).

   **Presence assertions (no vacuous fixture — AC4(d)'s discipline ported
   to the sink tests):** for each hostile sibling field, assert its ESCAPED
   form (the `termsafe.Escape` output literal) is PRESENT in the captured
   output — or at minimum a rendered-branch guard in the existing `:997`
   style (`if !strings.Contains(combined, …) { t.Fatalf("expected the …
   branch to render …") }`) proving the hostile-slot verdict was parsed and
   its render leg taken — so a degenerate fixture (skipped file, unreached
   branch) fails loudly instead of passing vacuously.

   **Malformed-file + slot-label coverage (the R3(c)/(d) escapes must
   actually render):** the fixture set includes (i) a verdict file whose
   name MATCHES `verdictFileRE` with hostile ESC bytes in the slot portion
   but whose body is INVALID JSON, so it lands in `res.Malformed` and
   renders through `renderPanelVerify` (`:501-503`) AND `renderPanelTally`
   (`:652`) under the clean-triple assertion; and (ii) a hostile-slot
   REQUEST_CHANGES verdict carrying a `concrete_changes_required` entry
   (and/or a decode-error shape), so `collectSlotChanges` (`:577-614`)
   attributes items (or a DecodeErr) to that slot and the `:678`/`:682`
   `sc.Slot` label lines render and are asserted clean. Without (i)/(ii)
   those two escapes would regress green silently — spec R3's falsifier
   names "malformed-filename" explicitly.

   **Leg-5b coverage (DECIDED — spec option (i) as primary):** the
   bead-path rev-parse has no stub seam (`resolvePanelGateFacts` hard-wires
   `newExecutor(root).RevParseRef`, `panel.go:405-407`), so leg 5b is
   driven by building `panel.GateFacts` directly with a
   `fmt.Errorf("rev-parse %s: simulated", hostileID)` wrap and asserting on
   the pure `renderPanelVerify`/`renderPanelTally`/`tallyExitAction` (the
   `TestPanelVerbs_DecisionIsPanelGateDecision` precedent — deterministic,
   no reliance on `exec.Command` pre-spawn behavior; the spec's option (ii)
   NUL-argv route remains an acceptable end-to-end supplement).

   **Block legs under the full-command drive (corrected + strengthened):**
   the rev-parse-DEPENDENT Block leg (6, stale SHA) needs a successful
   `rev-parse bead/<bead_id>`, so its full-command drive uses a CLEAN
   `bead_id` with hostile sibling fields. But a hostile `bead_id` DOES
   reach a Block leg with zero git: `ResolveGateFacts` short-circuits
   BEFORE rev-parse on the abandoned/round-mismatch paths (`gate.go:406-408`),
   and round mismatch (leg 4) IS a Block — so the test ALSO drives a
   round-mismatch Block subtest (`panel.json` `round` disagreeing with the
   filename-derived latest round) with the HOSTILE `bead_id`, exercising
   `RawMergeFence(f.BeadID)` and the leg-4 interpolations (`gate.go:186-189`)
   end-to-end through the real CLI sinks, deterministically, with no git
   repo needed. (The first draft's "a NUL `bead_id` can never rev-parse to
   a Block leg" was wrong as a blanket claim — it holds only for the
   rev-parse-dependent legs; this subtest is the free strengthening the
   correction buys.)
3. **Scope-fence machine check + AC5 re-run + AC-global + optional manual
   smoke.** The R3-excluded sites in this package are NOT modified (spec
   R3's falsifier includes scope discipline) — and that is MACHINE-CHECKED
   in this bead's own Verification (below), not merely reviewable in the
   diff. `internal/redact` untouched. Then the build + this bead's package
   sweep; the six AC5 pins re-run (this bead edits the sanitizer's file,
   `cmd/mindspec/panel.go`); if Bead 3a merges after Bead 3b, also run the
   full AC-global suite on the final tree (Provenance). Optionally (spec
   Validation Proofs): in a scratch repo, hand-edit a bead panel's
   `panel.json` `bead_id` to the hostile pattern, run
   `mindspec panel verify <slug>` / `panel tally <slug>` in a terminal, and
   confirm via `| od -c` that no raw `\033` and no forged line appears.

**Verification**

- [ ] AC1: `grep -q 'func TestPanelVerbs_BeadHostileBeadIDEscaped' cmd/mindspec/panel_test.go && go test ./cmd/mindspec -run 'TestPanelVerbs_BeadHostileBeadIDEscaped' -v`.
- [ ] AC5 re-run EXACTLY as written (same command pair as Bead 2), plus the `git diff 8f9c9ccf` no-removed-assertion check.
- [ ] Scope fence, MACHINE-CHECKED — each excluded function is byte-identical to the spec-init tree, extracted by FUNCTION ANCHOR rather than raw line range (this bead's own edits above `:701` shift later line numbers, so fixed `sed -n 'N,Mp'` ranges would false-fail): each of
  `diff <(git show 8f9c9ccf:cmd/mindspec/panel.go | sed -n '/^func tallyExitAction(/,/^}/p') <(sed -n '/^func tallyExitAction(/,/^}/p' cmd/mindspec/panel.go)`,
  the same pair for `/^func tallyExitActionNonBead(/,/^}/` and `/^func shellQuoteTarget(/,/^}/`,
  prints NOTHING; and `git diff 8f9c9ccf -- internal/redact/` is empty.
- [ ] `go build ./... && go test ./cmd/mindspec ./internal/panel` — green with zero semantic expectation change outside the new test.
- [ ] If this sub-bead merges last: AC-global — `go build ./... && go test ./cmd/mindspec ./internal/panel ./internal/complete ./internal/instruct ./internal/termsafe` all pass; `mindspec validate spec 116-panel-message-escaping` passes (advisory WARN acceptable).
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- The hostile-bead-panel fixture drives NO raw NUL, raw ESC, or forged
  standalone line through the CLI sinks — `panel verify` stdout, `panel
  tally` stdout, `tallyExitAction`'s Warn writer and Block failure body
  (AC1 — R1(a)/(b), the fl91 headline), including the leg-4
  hostile-`bead_id` Block and the malformed-file/slot-label renders.
- Every R3(a)-(d) sibling site escapes at its render site; every excluded
  `cmd/mindspec` site is byte-unchanged, machine-checked (R3's
  scope-discipline falsifier).
- The six AC5 pins green without semantic modification.

**Depends on**
Bead 2 (AC1 asserts full-text cleanliness of renders that carry
`d.Message` — clean only once `gate.go` escapes at construction; see the
Decomposition section. Transitively Bead 1: `escapeConfigValue`'s
delegation landed there). Independent of Bead 3b — may merge before or
after it.

## Bead 3b: The complete + instruct bypass sinks — R3(f)/(g) (AC2) and R3(e)/(h) (AC3)

**Goal:** every `internal/complete`- and `internal/instruct`-built render of
a panel-dir- or checkout-plantable field that bypasses `Decision.Message` is
escaped at its render site with the shared escaper, and the AC2/AC3
sink-level hostile-fixture pins go green.

**Scope**

`internal/complete/panel_advisory.go` (R3(f)/(g)),
`internal/instruct/panelstate.go` (R3(e)/(h)), plus the AC2/AC3 tests. Both
packages gain a `termsafe` import (legal for every consumer under the
ADR-0030 lint, which bans only `os/exec`/`internal/gitutil` imports and
git/bd exec literals). Under Bead 2's construction-boundary fix,
`panelGate`'s two `Decision.Message` sinks (`panel_advisory.go:225-227`,
`:258-260`) need NO change — they render an already-safe Message. Parallel
sibling of Bead 3a: zero file overlap, zero inter-dependency; this sub-bead
lands its own packages green independently at its own merge point, with
Bead 2 (not 3a) as its only prerequisite.

**Steps**

1. **`internal/complete` — R3(f)/(g)** (complete-built messages no `gate.go`
   escaping reaches):
   - (f) the refutation-persist-failure Block (`panel_advisory.go:251-254`,
     re-verified at `02a39c8f`: the `guard.NewFailure` construction spans
     exactly `:251-254`, with its slot collection at `:246-250`): escape
     each slot before the `strings.Join(slots, ", ")` at `:253` (the slots
     come from `panel.json`'s parse-lenient `refutations` array and from
     filename-derived RC slots via `AppliedRefutations`' byte-equality
     match, `tally.go:227`) and interpolate `termsafe.Escape(err.Error())`
     via `%s` in place of the raw `%v` of the metadata error.
   - (g) `CheckPendingObligations`' refusal (`:662-664`): render `e.Slot` via
     `termsafe.Escape` — closing the asymmetry with the sibling shape-error
     messages that already use control-safe `%q` (`:599`, `:624`). The
     consumer `internal/approve` is untouched.

   **Inline scope fence (do NOT touch — same file, adjacent to these edits,
   machine-checked in this bead's own Verification):** the THREE
   recovery-line `beadID` interpolations must NOT be escaped or otherwise
   modified: the panel-Block recovery line at `:226-227`
   (`"re-run the panel (/ms-panel-run step 0 for %s), then 'mindspec
   complete %s'", beadID, beadID`); the R3(f) Block's OWN recovery arg at
   `:254` (`fmt.Sprintf("mindspec complete %s", beadID)` — the (f) edit
   touches the `:251-253` message construction, NEVER the `:254` recovery
   line one token away); and `reconcilePendingRefutations`' `refuse`
   closure at `:714` (the same `fmt.Sprintf("mindspec complete %s",
   beadID)` shape). All three interpolate complete's ARGV, never
   `panel.json` content — the gate fires only on a `ForBead` byte-equal
   match (`internal/panel/panel.go:275`), so a hostile value there requires
   a hostile argv: the general argv-echo class the spec's Non-Goals
   exclude. They are structurally near-identical
   `guard.NewFailure(fmt.Sprintf(...), fmt.Sprintf("mindspec complete %s",
   beadID))` shapes sitting NEXT TO the (f)/(g) edit sites; do not "make
   them consistent" with the new escapes. `panelAdvisory` +
   `Result.VoteDecision` (production-dead / lockstep-covered) are equally
   untouched.
2. **AC2 pin** — NEW named test
   **`TestCompletePanelGate_HostilePanelEscaped`** in `internal/complete`,
   seam-driven beside the existing `panelGate` suite. **Fixture discipline
   (spec-pinned, grill-corrected):** the bead ID passed to `panelGate` is an
   HONEST printable one that byte-equals the panel's `bead_id` — mirroring
   production (`panel.ForBead`'s byte-equality, `internal/panel/panel.go:275`)
   and keeping the R3-excluded recovery-line argv interpolations
   (`:226-227`) out of the assertion's blast radius. Hostility lives in the
   SIBLING fields: slug (panel dir name), `abandon_reason`,
   `reviewed_head_sha`, verdict filename/slot, verdict string. **Fixture
   physics (as in Bead 3a's AC1):** the filename-derived fields — the slug
   (a directory name) and verdict filename/slot — carry ESC/CSI hostility
   only (NUL is filesystem-impossible; a newline in a verdict filename
   fails `verdictFileRE` and silently skips the file — a vacuous fixture);
   the full NUL + newline + forged-line pattern rides in the JSON-sourced
   fields (`abandon_reason`, `reviewed_head_sha`, verdict strings,
   `refutations` slots). **Presence assertions (no vacuous fixture):**
   assert each hostile field's ESCAPED form is PRESENT in the captured
   error/advisory text (or a rendered-branch guard proving the leg fired),
   so a degenerate fixture fails loudly. Subtests: (a)
   a Block leg — the returned `guard.NewFailure`'s full error text is clean;
   (b) a Warn leg (abandoned, hostile reason) — the bytes written to the
   advisory writer are clean, including the `"panel gate: "` line; (c) the
   R3(f) Block — a hostile-slot RC verdict + matching `refutations` entry
   with the `completeMergeMetadataFn` seam forced to fail → that failure's
   text is clean; plus a direct `CheckPendingObligations` call over a
   hostile-slot metadata double → its refusal text is clean (R3(g)). Plus a
   clean-fixture subtest asserting the rendered output equals the pre-change
   literal (the spec's plan-level F3 note — belt-and-suspenders for R2's
   "no clean byte differs" falsifier).
3. **`internal/instruct` — R3(e)/(h) sink-local escapes** (the SessionStart
   transcript composite, `renderFullPanelState`, `panelstate.go:621-642`):
   `e.Slug` at `:177`; `renderStaleWorktrees`' `e.Path` at `:518` (worktree-
   list and fs-glob directory names — a hostile committed tree materializes a
   hostile dir name on checkout); `renderInProgressBeads`' `Title`/`Worktree`/
   `LastCommit` at `:362`/`:367`/`:372` (agent-authored commit subject;
   bd-sourced but agent-writable title). All via `termsafe.Escape` at the
   render site. `verdict()`'s pass-through of `d.Message` (`:135`) needs no
   change — safe by construction after Bead 2. No render outside the
   composite is touched (spec Non-Goals).
4. **AC3 pin** — NEW named test
   **`TestInstructPanelState_HostilePanelEscaped`** in `internal/instruct`:
   (a) `PanelStateEntry` fixtures whose `Slug` and tally fields (hostile
   `bead_id` via `Panel.BeadID`, hostile `abandon_reason`, a Block-producing
   fact set) carry the hostile pattern → `renderPanelState` output contains
   no raw `0x00`/`0x1b` and no forged standalone markdown line (a hostile
   newline cannot mint a new `- **…**` bullet or a bare `recovery:` line);
   (b) the R3(h) renders, driven through `renderFullPanelState`: a
   `StaleWorktreeEntry` whose `Path` carries the hostile pattern and a
   `BeadStateEntry` whose `LastCommit`, `Title`, and `Worktree` carry it →
   the full composite output is clean by the same triple. In-memory fixtures
   (`PanelStateEntry`/`StaleWorktreeEntry`/`BeadStateEntry` structs, not
   files) carry the FULL hostile pattern — the filename-physics constraint
   does not apply to them — with the same presence-assertion discipline
   (each hostile field's escaped form appears in the composite). Plus a
   clean-fixture subtest asserting the rendered output equals the pre-change
   literal (F3 note, as in AC2).
5. **Scope-fence machine check + package sweep + AC-global if last.** The
   fenced sites in step 1 are MACHINE-CHECKED (Verification below). Then
   `go build ./...` + this bead's package sweep; if Bead 3b merges after
   Bead 3a, also run the full AC-global suite on the final tree
   (Provenance).

**Verification**

- [ ] AC2: `grep -q 'func TestCompletePanelGate_HostilePanelEscaped' internal/complete/*_test.go && go test ./internal/complete -run 'TestCompletePanelGate_HostilePanelEscaped' -v`.
- [ ] AC3: `grep -q 'func TestInstructPanelState_HostilePanelEscaped' internal/instruct/*_test.go && go test ./internal/instruct -run 'TestInstructPanelState_HostilePanelEscaped' -v`.
- [ ] Scope fence, MACHINE-CHECKED (the `:226-227`/`:254` fenced lines sit INSIDE the function the (f) edit touches, so grep-shape assertions pin them rather than a whole-function diff): `! grep -q 'termsafe.Escape(beadID)' internal/complete/panel_advisory.go` (beadID is argv — escaped NOWHERE in this file); `[ "$(grep -c 'fmt.Sprintf("mindspec complete %s", beadID)' internal/complete/panel_advisory.go)" = "2" ]` (the `:254` and `:714` shapes, byte-unchanged); `grep -q 'beadID, beadID))' internal/complete/panel_advisory.go` (the `:226-227` recovery line's raw argv pair intact); `diff <(git show 8f9c9ccf:internal/complete/panel_advisory.go | sed -n '/^func reconcilePendingRefutations(/,/^}/p') <(sed -n '/^func reconcilePendingRefutations(/,/^}/p' internal/complete/panel_advisory.go)` prints nothing (the `:714` fence, function-anchored — that function is not edited at all); and `grep -q 'return mapGateAction(d.Action), d.Message' internal/instruct/panelstate.go` (the `:135` pass-through stays a raw pass-through — no sink-side re-escaping of an already-safe Message).
- [ ] `go build ./... && go test ./internal/complete ./internal/instruct` — green with zero semantic expectation change outside the new tests.
- [ ] If this sub-bead merges last: AC-global — `go build ./... && go test ./cmd/mindspec ./internal/panel ./internal/complete ./internal/instruct ./internal/termsafe` all pass; `mindspec validate spec 116-panel-message-escaping` passes (advisory WARN acceptable).
- [ ] `gofmt -l ./cmd ./internal` prints nothing.

**Acceptance Criteria**

- The hostile-panel fixtures drive NO raw NUL, raw ESC, or forged
  standalone line through the complete gate's error/advisory text —
  including the R3(f) persist-failure and R3(g) obligation-refusal
  messages — (AC2) or through the SessionStart transcript composite,
  including the R3(h) stale-worktree and in-progress-bead renders (AC3).
- Every R3(e)-(h) site escapes at its render site; every excluded site
  (the three recovery-line `beadID` interpolations among them) is
  byte-unchanged, machine-checked (R3's scope-discipline falsifier).
- Both packages green with zero semantic expectation change outside the
  new tests.

**Depends on**
Bead 2 (AC2/AC3 assert full-text cleanliness of renders that carry
`d.Message` — complete's Block body IS `d.Message` with the hostile
slug/abandon-reason interpolated by `gate.go`, and instruct's `verdict()`
passes `d.Message` through at `panelstate.go:135` — clean only once
`gate.go` escapes at construction; see the Decomposition section.
Transitively Bead 1 for the `termsafe` imports). Independent of Bead 3a —
may merge before or after it.

## ADR Fitness

- **ADR-0037 (panel gate as enforced contract) — AMENDED; remains the right
  home.** 116 changes what gate messages CARRY, never what the gate DECIDES:
  the decision matrix, thresholds, §7 hatches, and §6 fail-open/fail-closed
  asymmetry are byte-unchanged, and the fix sits squarely inside §8's trust
  boundary — §8 rules out tamper-proofing (signing/hashing agent-writable
  files), not output hygiene; a panel artifact that forges terminal or
  transcript lines is the accidental-footgun class the gate exists to stop.
  The one contract-level change is the `internal/panel` leaf invariant: its
  letter ("imports NO internal package") breaks; its recorded purpose (no
  config coupling, no I/O, decision purity) is preserved by a stdlib-only
  pure-string import. That is recorded as a dated amendment (Bead 2), which
  also records the A′ local-copy rejection — a duplicated security predicate
  drifts silently, unlike the inert-data local copies the leaf's own
  precedents duplicate — and the invariant graduates from comment-only to
  machine-checked (AC7 import pin, AC6 single-home grep). A new ADR was
  considered and rejected: the leaf invariant lives in ADR-0037's spec-099
  amendment lineage (ADR line 80), and recording its evolution anywhere else
  would split the contract across two homes. No divergence; the amendment is
  Bead 2's job.
- **ADR-0035 (agent error contract) — best choice, unchanged.** Every Block
  stays a `guard.NewFailure` with a genuine final recovery line: escaping
  applies to interpolated FIELDS, never to the recovery template prose, so
  `guard.HasFinalRecoveryLine` discipline and the recovery lines'
  copyability are unchanged (and the copyable-command shell-quoting layer,
  `shellQuoteTarget`, is explicitly out of scope). The AC4/AC1 assertions
  that template newlines and prose survive verbatim are this ADR's contract
  made falsifiable. No amendment.
- **ADR-0030 (executor boundary) — cited as prose context only, not amended,
  and deliberately NOT in this plan's `adr_citations` (see the frontmatter
  caveat above).** Its relevance is permissive, not directive: the boundary
  lint governs `internal/complete` but bans only `os/exec`/`internal/gitutil`
  imports and git/bd exec literals (`internal/lint/boundary_test.go:44-56`,
  package list `:111-117`), so the pure-string `termsafe` import every bead
  adds is legal for every consumer; `internal/panel` and `internal/instruct`
  are not lint-governed at all — the constraint on `internal/panel` is the
  self-imposed leaf invariant (ADR-0037's, above), not this ADR. Nothing in
  116 moves enforcement or I/O across the executor boundary. No amendment.

No ADR divergence is proposed anywhere in this plan; no ADR is superseded.

## Testing Strategy

- **Unit — `internal/termsafe` (Bead 1).** Property-style pins on the
  relocated rule: printable-ASCII identity, single-line printable-ASCII-safe
  quoting for every control class (NUL/ESC/CSI U+009B/DEL/newline/invalid
  UTF-8), and idempotence on the control-byte class. The AC6 single-home
  grep is the anti-drift machine check: exactly one non-test implementation
  of the `r < 0x20 || r > 0x7e` loop, and it lives in `internal/termsafe`.
- **Unit — `internal/panel` (Bead 2).** The AC4 field-sweep matrix drives
  the PURE `PanelGateDecision` (no I/O — `GateFacts` constructed directly,
  the package's existing convention) over all twelve legs with per-field
  hostility, asserting the clean triple, leg-for-leg `Action` parity with the
  clean baseline, leg 7's real multi-line layout, and the operationalized
  no-double-escape check (single-escaped literal present, nested form absent).
  The AC7 import pin (`go/parser`, imports-only) makes the amended leaf
  invariant RED on any future second internal import.
- **Unit — `cmd/mindspec` (Bead 3a).** AC1 mixes two drive styles
  deliberately: full-command drives through `panel verify`/`panel tally`
  (Warn and Block legs — both `tallyExitAction` branches) for the
  end-to-end sinks, and direct `GateFacts` construction against the pure
  renderers for leg 5b (no bead-path rev-parse seam exists —
  `resolvePanelGateFacts` hard-wires the executor, `panel.go:405-407`; the
  `TestPanelVerbs_DecisionIsPanelGateDecision` precedent). The
  rev-parse-dependent Block leg (6) under the full-command drive uses a
  clean `bead_id` with hostile siblings; the round-mismatch Block (leg 4)
  short-circuits BEFORE rev-parse (`gate.go:406-408`) and is driven
  end-to-end with the HOSTILE `bead_id`, no git needed. Fixture physics:
  filename-derived hostility is ESC/CSI-only (NUL is
  filesystem-impossible; a newline fails `verdictFileRE` and skips the
  file); presence assertions / rendered-branch guards make a vacuous
  fixture RED; the malformed-file (invalid-JSON body, hostile filename)
  and hostile-slot `concrete_changes_required` fixtures force the R3(c)/(d)
  render legs to actually fire.
- **Unit — `internal/complete` (Bead 3b).** AC2 is seam-driven beside the
  existing `panelGate` suite: fixture panels on disk (hostile slug =
  directory name, hostile `panel.json` fields, hostile verdict filenames —
  ESC/CSI-only where filename-derived, per the same fixture physics),
  the advisory `warnOut` writer captured for the Warn leg, and the
  `completeMergeMetadataFn` seam forced to fail for the R3(f) leg; R3(g) via
  a direct `CheckPendingObligations` call over an injected metadata reader
  (its dependency-injected `getMeta` signature). The honest-bead-ID fixture
  discipline keeps the R3-excluded argv interpolations out of the blast
  radius; presence assertions keep every hostile field's render leg
  provably fired.
- **Unit — `internal/instruct` (Bead 3b).** AC3 drives the unexported
  `renderPanelState`/`renderFullPanelState` in-package with hostile
  `PanelStateEntry`/`StaleWorktreeEntry`/`BeadStateEntry` fixtures — the
  transcript sink's forged-markdown falsifier (no minted bullet, no bare
  `recovery:` line) alongside the byte-level triple. In-memory struct
  fixtures carry the full hostile pattern (no filename physics apply).
- **Clean-fixture literal subtests (spec F3 plan-level note).** AC2's and
  AC3's new tests each include a clean-fixture subtest asserting the rendered
  output equals the pre-change literal — belt-and-suspenders for R2's "no
  clean byte differs" falsifier, on top of AC5's six untouched pins.
- **Regression net — AC5 (Beads 2 and 3a).** The six R4-named existing
  tests run unmodified at Bead 2's merge point (the owner — the
  construction change is what could break them) and re-run at Bead 3a's
  (whose `cmd/mindspec/panel.go` edits touch the sanitizer's file); the
  `git diff 8f9c9ccf` check pins that no assertion line inside them was
  removed. Bead 3b touches neither `cmd/mindspec` nor `internal/panel`, so
  the pins cannot move there; they run again anyway inside whichever
  sub-bead's AC-global sweep lands last. This is the double-escape
  non-interaction argument (fence-strip on `RawMergeFence("")`, template-
  prose leg detection, wholesale leg-5/5b rebuild) made falsifiable.
- **Scope-fence machine checks (Beads 3a and 3b).** Each sub-bead's OWN
  verification asserts its excluded sites are byte-unchanged — Bead 3a via
  function-anchored extraction diffs against `8f9c9ccf`
  (`tallyExitAction`, `tallyExitActionNonBead`, `shellQuoteTarget`; anchors
  rather than fixed line ranges, since the bead's own edits shift later
  line numbers), Bead 3b via grep-shape pins (no `termsafe.Escape(beadID)`
  anywhere in `panel_advisory.go`; the two
  `fmt.Sprintf("mindspec complete %s", beadID)` shapes and the
  `beadID, beadID))` recovery pair intact; `reconcilePendingRefutations`
  function-diff empty; instruct's `:135` raw pass-through intact) — so an
  accidental touch fails the bead itself, not just a later panel.
- **RED-on-revert discipline.** Every named new test is ABSENT at the
  spec-init SHA `8f9c9ccf` (all six names and the string `termsafe`: zero
  `*.go` hits, re-verified at plan time), and every AC proof chains an
  existence discriminator (`grep -q 'func Test<Name>'`) with the exact-named
  `go test -run '<Name>'` — `go test -run` exits 0 on a no-match, so the grep
  is what makes each proof fail before the test lands. No package-wide run
  stands in for a named run. AC6's discriminator is additionally CODE-level
  (the exact loop condition), immune to the doc-comment `0x7e` false-count
  the spec's grill caught.
- **Shared test infrastructure (named, reused, never forked):**
  `assertClean`'s clean-triple assertions (a LOCAL closure in
  `cmd/mindspec/panel_test.go:971` inside an AC5-pinned function — Bead 3a
  DUPLICATES its assertions into the new test rather than hoisting, so the
  pinned function stays byte-untouched); the pure-`GateFacts` construction
  precedent of `TestPanelVerbs_DecisionIsPanelGateDecision`
  (`panel_test.go:499`); the `panelGate` fixture/seam family in
  `internal/complete` (`completeGetMetadataFn`/`completeMergeMetadataFn`,
  the `warnOut` writer); and the in-package render-function drive convention
  of `internal/instruct/panelstate_test.go`. The one new shared production
  surface, `termsafe.Escape`, is itself pinned by AC6 before any consumer
  lands.

## Provenance

Legend: every AC is completed inside a single bead (no proof test spans an
edge — in particular none spans the 3a ∥ 3b boundary); AC5 is OWNED by
Bead 2 (the construction change is what could break the pins) and RE-RUN by
Bead 3a (whose `cmd/mindspec/panel.go` edits touch the sanitizer's file).
AC-global is OWNED by whichever of Bead 3a/3b merges LAST (the final tree);
every bead additionally runs `go build ./...` + its own packages at its own
merge point, so each sub-bead is independently green regardless of merge
order.

| Spec AC | Bead | Verification step (named, runnable) |
|---|---|---|
| AC1 (bead-path CLI sinks — verify + tally, Warn AND Block incl. the hostile-bead_id leg-4 round-mismatch Block, hostile bead_id + R3 siblings, malformed-file + slot-label renders forced) | Bead 3a | `TestPanelVerbs_BeadHostileBeadIDEscaped`; chained proof exactly as spec'd (`grep -q 'func …' && go test ./cmd/mindspec -run … -v`) |
| AC2 (complete sinks — Block body, Warn advisory, R3(f) persist-failure, R3(g) obligation refusal) | Bead 3b | `TestCompletePanelGate_HostilePanelEscaped`; chained proof as spec'd; clean-fixture literal subtest (F3) |
| AC3 (SessionStart transcript composite — panel block + R3(h) stale-worktree/bead renders) | Bead 3b | `TestInstructPanelState_HostilePanelEscaped`; chained proof as spec'd; clean-fixture literal subtest (F3) |
| AC4 (construction-boundary field-sweep matrix, Option A shape) | Bead 2 | `TestPanelGateDecision_HostileFieldsEscaped` (legs 0/2/3/4/5/5b/6/7/8/9/9.5/10; clean triple + Action parity + leg-7 layout + no-double-escape) |
| AC5 (six pins + non-bead path unchanged on clean inputs) | Bead 2 (re-run Bead 3a) | The spec's exact two-command run over the six named tests + the `git diff 8f9c9ccf` no-removed-assertion check |
| AC6 (one escaper home: termsafe tests + delegation + single-home grep) | Bead 1 | `TestTermsafeEscape_SafeSetQuoteAndNoOp` + the full chained AC6 proof (delegation grep, loop-condition single-home count = 1, home = `internal/termsafe/`) |
| AC7 (amended leaf invariant, machine-checked) | Bead 2 | `TestPanelLeafImports_StdlibPlusTermsafeOnly` (go/parser imports-only pin; only internal import = termsafe) |
| AC-global (build + five-package sweep + spec validate) | Whichever of Bead 3a/3b merges last (final tree; every bead runs the build + its packages) | `go build ./... && go test ./cmd/mindspec ./internal/panel ./internal/complete ./internal/instruct ./internal/termsafe`; `mindspec validate spec 116-panel-message-escaping` |
| R3 scope-discipline falsifier (excluded sites untouched) | Bead 3a + Bead 3b (each for its own file) | Machine-checked: 3a's function-anchored diffs (`tallyExitAction`/`tallyExitActionNonBead`/`shellQuoteTarget` vs `8f9c9ccf`) + `git diff 8f9c9ccf -- internal/redact/` empty; 3b's grep-shape pins (no `termsafe.Escape(beadID)`; both `mindspec complete %s` recovery shapes intact; `reconcilePendingRefutations` diff empty; `:135` pass-through intact) |
| Validation Proofs: OWNERSHIP single-claimant | Bead 1 | `grep -l 'internal/termsafe' .mindspec/domains/*/OWNERSHIP.yaml` prints exactly the workflow file |
| Validation Proofs: manual terminal smoke (optional) | Bead 3a | Scratch-repo hand-edited hostile `bead_id`; `panel verify`/`panel tally` piped to `od -c` — no raw `\033`, no forged line |
