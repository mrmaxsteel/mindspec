---
adr_citations:
    - ADR-0041
    - ADR-0040
    - ADR-0035
    - ADR-0034
    - ADR-0023
    - ADR-0037
    - ADR-0036
approved_at: "2026-07-23T21:54:35Z"
approved_by: user
bead_ids:
    - mindspec-8nhe.1
    - mindspec-8nhe.2
    - mindspec-8nhe.3
spec_id: 124-impl-readiness-gate
status: Approved
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/validate/readiness/readiness.go
        - internal/validate/readiness/report.go
        - internal/validate/readiness/fixtures.go
        - internal/validate/readiness/readiness_test.go
        - internal/bead/readiness.go
        - cmd/mindspec/bead_ready.go
        - cmd/mindspec/bead_ready_test.go
        - cmd/mindspec/ceremony_guard_test.go
    - depends_on:
        - 1
      id: 2
      key_file_paths:
        - cmd/mindspec/next.go
        - cmd/mindspec/next_ready_gate_test.go
        - cmd/mindspec/adr0041_amendment_test.go
        - internal/next/ready_gate.go
        - internal/next/ready_gate_test.go
        - .mindspec/adr/ADR-0041-gate-before-mutate.md
        - cmd/mindspec/ceremony_guard_test.go
    - depends_on:
        - 1
        - 2
      id: 3
      key_file_paths:
        - cmd/mindspec/bead_clarify.go
        - cmd/mindspec/bead_clarify_test.go
        - internal/bead/clarify.go
        - internal/bead/clarify_test.go
        - plugins/mindspec/skills/ms-bead-impl/SKILL.md
        - plugins/mindspec/skills/ms-bead-cycle/SKILL.md
        - plugins/mindspec/skills/ms-spec-autopilot/SKILL.md
        - internal/setup/readiness_skill_pin_test.go
        - internal/complete/readiness_isolation_test.go
        - cmd/mindspec/ceremony_guard_test.go
---
# Plan: 124-impl-readiness-gate

Three beads implement the two-layer readiness gate, following the spec's
own natural decomposition (spec § In Scope) exactly: (1) the deterministic
mechanical-floor engine + the read-only `bead ready-check` verb + the
committed fixture pair; (2) the `next` gate-before-mutate wiring +
`--allow-not-ready` + the ADR-0041 amendment; (3) the skill layer + the
`bead clarify` verb + the bounded R8 clarification loop + the ceremony
accounting close-out.

**Engine placement (the spec's delegated plan-level choice): the
mechanical floor lives in a new SUB-PACKAGE `internal/validate/readiness/`**,
NOT `internal/validate` itself and NOT a top-level `internal/readiness`.
Three forcing facts:
(a) **Ownership** — the workflow domain's OWNERSHIP.yaml enumerates its
`internal/**` directories explicitly and `internal/validate/**` is a glob,
so `internal/validate/readiness/**` is COVERED (no divergence self-trip on
any bead's zero-override `mindspec complete`); a top-level
`internal/readiness/**` would be claimed by NOTHING and self-trip
`adr-divergence-unowned` — the spec pinned placement "fixed by the domain
attributions".
(b) **Test-build import cycle (plan-gate O2-1/O3-1, reproduced
empirically)** — the engine must consume `lifecycle.FindLandedMerge` (MF-3),
i.e. an `<engine> → internal/lifecycle` edge. But
`internal/lifecycle/ownership_test.go` is a WHITE-BOX test
(`package lifecycle`) that imports `internal/validate` (for
`validate.GlobMatch`). Placing the engine IN `internal/validate` would form
`lifecycle[test] → validate → lifecycle` — Go's `import cycle not allowed
in test`, breaking `go test ./internal/lifecycle/` and `go test ./...` so
Bead 1 could never pass its own gate. The sub-package severs it: the
white-box `lifecycle` test imports `internal/validate` (which does NOT
import the sub-package), while `internal/validate/readiness → lifecycle`
is a leaf edge lifecycle's tests never close. Production imports were the
only leg the prior draft checked; the test build is the one that breaks,
so the sub-package is load-bearing, not cosmetic.
(c) **Substrate reuse** — the sub-package still consumes
`internal/validate`'s existing plan parsing (`WorkChunk`, the positional
`bead_ids[N-1]` mapping, the `## Bead N` splitter — spec 097 R3/R4) as a
normal same-domain import.

**What the sub-package hosts (one move, three findings resolved):**
`internal/validate/readiness/readiness.go` (the engine + the four
signals), `internal/validate/readiness/fixtures.go` (the EXPORTED
NEGATIVE/POSITIVE fixture builders — a NON-test file so all three test
surfaces (engine / verb / gate) can import them; Go cannot import a
`_test.go` across packages — plan-gate codex-G1), and
`internal/validate/readiness/report.go` (the per-signal `ReadinessReport`
type + its renderer as a reachable API — so Bead 2's `next` refusal
RE-USES the same renderer rather than restating gate output, ADR-0040
no-restate — plan-gate O3-2). The bd seam (below) also lives here.

**Dependency graph (acyclic), waves, and the shared-file seam.**
Edges: `1→2`, `1→3`, `2→3`. Waves: W1 = {1}, W2 = {2}, W3 = {3} — fully
serial, longest chain 3, at the decomposition heuristic ceiling. Each link
is genuine produced-then-consumed state, not file adjacency:

- **Bead 2 depends on Bead 1**: the `next` gate CALLS the Bead-1 engine
  (`EvaluateReadiness`) and its refusal prints the Bead-1 per-signal
  report format (R3: "prints the per-signal report" — the same rendering,
  not a re-implementation; ADR-0040 forbids restating gate logic). No
  engine, no gate.
- **Bead 3 depends on Bead 2**: `/ms-bead-impl`'s dispatch ingress must
  HONOR the durable override marker that Bead 2's `--allow-not-ready`
  writes (R4a / AC-5's proceed-with-warning path reads the exact metadata
  key + signal list Bead 2 defines the write for), and the
  `ms-bead-cycle` prose documents the clarification-vs-`--allow-not-ready`
  distinction (AC-13(iv)) against the SHIPPED flag semantics.
- **Bead 3 depends on Bead 1**: the ingress invocation the AC-5 content
  pin greps for is the Bead-1 verb; `bead clarify` registers on the same
  `beadCmd` family.

**Shared-file seam resolution (the spec-117 false-independence lesson),
stated explicitly** — `key_file_paths` above is the true EDIT set per
bead (read-only probe targets are never declared there):

- **`cmd/mindspec/bead.go` is edited by NO bead.** Both new verbs would
  naturally land there (Bead 1's `ready-check`, Bead 3's `clarify`) —
  a real two-beads-one-file overlap. It is designed out by file split:
  each verb ships as its own file (`cmd/mindspec/bead_ready.go`,
  `cmd/mindspec/bead_clarify.go`) with its own `init()` registering on
  the existing `beadCmd` (the cobra pattern `bead.go` itself uses), so
  `bead.go` stays byte-identical all spec.
- **`cmd/mindspec/ceremony_guard_test.go` is edited by all three beads**
  — forced by R7a/AC-9(i): the baseline update must land in the SAME
  bead that adds each surface (`ready-check` → Bead 1; `--allow-not-ready`
  → Bead 2; `clarify` → Bead 3). Under the `1→2→3` chain these edits are
  STRICTLY SEQUENTIAL — no two beads ever hold the file concurrently, so
  the three-beads-one-file hazard is resolved by ordering, not hope.
- No other source or test file appears in more than one bead's edit set:
  Bead 1's metadata-key constants file (`internal/bead/readiness.go`) is
  only READ by Beads 2/3; Bead 3's clarify logic is a NEW file
  (`internal/bead/clarify.go`); `cmd/mindspec/next.go` is Bead 2 only;
  the skills are Bead 3 only. Hostile-render and integration tests live
  in each bead's own new `_test.go` files (never appended to a shared
  existing test file).
- The `decomposition-scope-redundancy` WARN (R=0.03 < 0.15) this
  produces is DELIBERATE and accepted: shared context flows through the
  `1→2→3` dependency edges (Beads 2/3 consume Bead 1's engine, verb,
  and key constants as landed APIs), not through co-edited files —
  minimizing declared file overlap is the 117 lesson applied, and the
  one genuinely shared file (the ceremony guard) is serialized above.

**Plan-level choices the spec delegates, resolved:**

- **Metadata carriers** (R3/R8a — "a dedicated bd metadata key"): two
  keys, following the `mindspec_landed_*` naming precedent —
  `mindspec_readiness_override` (the R3 durable override marker: the
  overridden signal IDs + a UTC timestamp) and
  `mindspec_readiness_attempt` (the R8 append-only attempt record).
  Constants live in a new `internal/bead/readiness.go` (Bead 1) so all
  three beads share one definition; both are written only via the
  existing `internal/bead.MergeMetadata` (no bd schema change,
  ADR-0023-advisory). MF-1..MF-4 read the plan section, the bd
  description, and bd dependency edges — NEVER issue metadata — so the
  AC-12 invariance holds by construction, and Bead 1 pins it by seeding
  the key directly.
- **Override-marker write ordering** (Bead 2) — **superseded as shipped
  (Bead-2 codex fixup + final-review r1 F1-2/G3)**: this bullet's
  original AFTER-claim design was flipped to **marker-BEFORE-ClaimBead,
  FAIL-CLOSED**, because AC-4 makes `--allow-not-ready` success a
  GUARANTEE that the durable marker exists — a marker-write failure now
  refuses with nothing claimed and no worktree, and the refusal path
  stays zero-mutation (AC-4's byte-identical audit; a plain refusal
  without `--allow-not-ready` never reaches the write). If `ClaimBead`
  then FAILS after the marker landed (a claim lost to contention), the
  claim-failure branch ROLLS BACK the marker — best-effort delete, loud
  warning naming the orphaned key if the delete also fails — so a lost
  claim leaves no stray authorizing override (final-review r1
  G3-OVERRIDE-ORPHAN). If the process dies between marker write and
  claim, the documented recovery is re-running `mindspec next` (which
  re-gates and re-offers `--allow-not-ready`) — forward-reconcilable,
  never corrupting.
- **`bead clarify` invocation shape** (constrained by AC-9/AC-15):
  `mindspec bead clarify <bead-id> --file <record.json>` — one JSON file
  carrying the original NOT-READY report (ordinals, verbatim reasons,
  signal tags) plus the clarification entries
  (`{ordinal, reason, answer, span}`). A single-file payload keeps the
  argv paste-safe (ADR-0035 — no free prose on the command line), makes
  the one-write cap atomic (one `MergeMetadata` call), and gives AC-15's
  verb-level rejections (unknown ordinal / missing span / second
  attempt) a deterministic parse surface. No other flag; explicitly NO
  `--finalize` or terminal-stamp surface (R8e derive-don't-write).
- **MF-2 harvest heuristic** (spec: "harvest heuristic plan-level, pinned
  by the POSITIVE fixture's benign feature (iv)"), three plan-level rules,
  each fixture-pinned:
  - **(1) Foreign-citation exclusion — SPAN-scoped as shipped
    (final-review r1 G1-MF2-MIXED-CITATION-LINE narrowed this rule's
    original whole-line form).** An `R<n>`/`AC-<n>` token is EXCLUDED
    only when it belongs to a foreign-spec citation's own span: the
    `spec <digits>` / `Spec <digits>` reference (where `<digits>`
    differs from the owning spec's number) plus the chain of tokens
    strictly adjacent to it (separated by whitespace or the `/`, `+`,
    `&` token-chain separators only) — "the spec 123 AC-17 pattern"
    cites AC-17, never claims it. A token ELSEWHERE on the same line
    (across clause punctuation or intervening words) is the bead's own
    claim and IS harvested — so a dangling owning-spec token sharing a
    line with a benign foreign citation still FAILs MF-2. Pinned by
    AC-14(iv) plus the mixed-line fixture arms.
  - **(2) Code-span exclusion (plan-gate F2-3, consistent with the
    spec's MF-4 code-fence rule).** Token harvest scans OUTSIDE inline-code
    spans (`` `…` ``) and fenced code blocks — the SAME exclusion MF-4
    applies — so an `R<n>`/`AC-<n>` token appearing only as backtick-quoted
    fixture data or a code identifier is not harvested as a claim. This
    covers the plan's own beads, which name tokens like `` `AC-9` `` inside
    code spans when describing the gate.
  - **(3) Clause-enumerator normalization — classified by FORM, not by
    existence (plan-gate F2-1 + G2-ENUMERATOR-COLLISION).** A parenthetical
    immediately following an `R<n>`/`AC-<n>` token is classified
    DETERMINISTICALLY by its own form, never by whether a coincidental
    sub-token happens to exist:
    - A parenthetical matching the **lowercase-Roman-numeral sequence**
      `(i)|(ii)|(iii)|(iv)|(v)|(vi)|(vii)|(viii)|(ix)|(x)|…` is a CLAUSE
      ENUMERATOR → always degrades to the BASE token: `AC-9(i)` harvests
      as `AC-9`, `R8(iv)` as `R8`. This holds EVEN IF a token `AC-9i`
      coincidentally exists in spec.md — form wins, so the outcome is
      collision-free.
    - A parenthetical single alphabetic sub-letter that is NOT a
      Roman-numeral form (`(a)`, `(b)`, `(c)`, `(d)`, `(f)`, `(g)`, … —
      note `(i)`, `(v)`, `(x)` are Roman and are treated as enumerators,
      NOT sub-letters) is a SUB-LETTER → normalizes to the exact sub-token:
      `R5(b)` harvests as `R5b`, resolving against an exact spec `R5b`.
    Because the split is on the literal parenthetical form, it is
    unambiguous regardless of what tokens exist. AC-14 gains an
    ENUMERATOR-PARENTHETICAL fixture arm (Bead 1) that pins the
    Roman-vs-sub-letter classification INCLUDING the collision case: a
    fixture where a coincidental `AC-9i` sub-token exists in spec.md yet a
    claimed `AC-9(i)` STILL resolves to base `AC-9` (form-based, not
    existence-based), alongside `R5(b)`→`R5b` exact and a genuinely
    dangling literal `AC-9i`-as-written FAILing — so a strict "sub-lettered
    exact" impl, a lenient "any-parenthetical-is-base" impl, AND an
    existence-based disambiguator all go red on the wrong arm.
  Everything else harvested from the bead's bd description + its
  `## Bead N` plan section is a claim.
- **Ceremony-guard extension shape** (AC-9(i)): the spec-122 guard pins
  flag sets for `complete`/`impl approve`/`validate` + config keys, but
  NOT the `bead` subcommand set or `next` flag set — the new surfaces
  would not trip it. Each bead therefore ADDS its surface's pin in the
  same commit that adds the surface: Bead 1 adds a
  `mindspec bead` SUBCOMMAND-set pin (baseline recorded from the real
  `beadCmd` children: `spec`, `plan`, `worktree`, `hygiene`,
  `create-from-plan`, plus the new `ready-check`); Bead 2 adds a
  `mindspec next` FLAG-set pin (recorded from pflag metadata, plus the
  new `--allow-not-ready`); Bead 3 extends the bead subcommand pin with
  `clarify` and pins `clarify`'s own flag set (`--file`, `--help`,
  `--trace` only). The guard mechanics (`commandFlagSet`,
  `assertSetEqual`) are reused, never weakened; the three baseline
  diffs are exactly the AC-9(i) evidence.
- **Fixture strategy** (R6 "committed test fixtures"): fixtures are
  committed EXPORTED builders in `internal/validate/readiness/fixtures.go`
  (a non-test file, per the sub-package placement above) constructing
  hermetic temp workspaces (the `internal/validate`
  `plan_test.go`/`divergence_test.go` pattern: real spec/plan files + real
  git repos per `internal/lifecycle/landed_test.go`). The NEGATIVE/POSITIVE
  pair are two named builders imported by the engine tests AND the
  verb/gate tests, so every consumer exercises the identical planted
  defects / benign features and CI pins both directions (AC-1/AC-2).
- **bd-less CI hermeticity — the per-surface mechanism, explicit
  (plan-gate F3-1, the spec-119 lesson).** CI runs `go test -short -race
  ./...` with NO `bd` on PATH, and `internal/validate` calls
  `bead.RunBD`/`bead.BeadExists` DIRECTLY (no seam exists there today —
  only `internal/next` has the `runBDFn` func-var seam). A blanket
  `LookPath`-skip (the `greenfield_e2e_test.go:278` precedent) would
  SILENTLY un-enforce the RED-today ACs in CI. Two mechanisms, named per
  surface:
  - **Engine-level (Bead 1): a new injectable bd seam IN the readiness
    sub-package.** `internal/validate/readiness/readiness.go` declares
    package-level func vars for the two bd reads it needs — dependency
    status/closure and metadata reads — defaulting to the real
    `internal/bead` helpers (the `adrStoreForSpecFn`/`loadOwnershipForRefFn`
    in-package-func-var precedent, `internal/validate/adr_memo.go:20`).
    Engine unit tests (AC-1/2/3/12/14) SWAP these to deterministic
    in-memory returns — zero real bd, fully hermetic under `-short`.
  - **Cmd-level (Beads 1/2/3): a stateful fake-bd shim.** The bd-mutating
    flows that must run the REAL binary path — AC-4's override-claim
    write, AC-10's real `plan approve`→`complete`, AC-11/AC-15's
    fresh-process `bd show` — use a stateful fake-bd shim on PATH (the
    `writeFakeBD`/`fakeBdDir` precedent used across
    `internal/doctor`/`internal/harness`/`internal/setup` tests) so
    claims/metadata/status round-trip without a real bd.
  - **No-skip-gating rule (explicit):** the AC-pinning readiness tests
    MUST FAIL, not `t.Skip`, when their bd surface is unavailable — the
    engine tests are seam-swapped (bd never consulted) and the cmd tests
    install the fake-bd shim unconditionally, so absence is a hard error,
    never a silent skip. This is stated in each affected bead's
    verification so CI genuinely enforces the four RED-today ACs.

**Dogfood note (the spec's own gate, eaten) — re-confirmed against the
revised floor.** Every `## Bead N` section below PASSES the very floor
Bead 1 builds:
- **MF-1**: each bead's `key_file_paths` is non-placeholder and its
  `**Acceptance Criteria**` block carries real entries.
- **MF-2**: every claimed `R<n>`/`AC-<n>` token resolves in spec.md.
  The plan's own `AC-9(i)`/`AC-9(ii)` references (Bead-section prose and
  the ceremony steps) are written as CLAUSE-ENUMERATOR parentheticals and
  harvest to the base `AC-9` (rule 3 above), which resolves; and where the
  plan quotes a token purely as fixture data it is inside a code span
  (rule 2). No bead claims a dangling token.
- **MF-3**: dependency edges are wired in `work_chunks[].depends_on`
  (Bead 2→1, Bead 3→1,2); at claim time each dep is already landed under
  the serial `1→2→3` order.
- **MF-4**: no genuine blocking marker. Blocking-region HEADERS
  (`**Blocking Questions**`) and the `TBD`/`OPEN QUESTION` literals appear
  in this plan ONLY inside code spans / as described behavior, exactly the
  MF-4 code-span/code-fence exclusion the engine must honor (both the
  token scan AND the blocking-region-header detection exclude backtick
  spans — plan-gate F2-2), so they never trip the gate on the plan's own
  beads.

## ADR Fitness

- **ADR-0041 (Gate-Before-Mutate) — AMENDED by this spec (R9/AC-16), the
  only ADR change.** The fourth-verb clause — "preflight-leg-only
  addition", naming `mindspec next` — is **PRE-DRAFTED at plan time**: it
  sits in this worktree's `.mindspec/adr/ADR-0041-gate-before-mutate.md`
  §1 now, under an explicit `PRE-DRAFT` marker comment, and is FINALIZED
  by Bead 2 (marker removed; wording adjusted only where the concrete
  implementation forces it) — the spec-117/122 amendment lifecycle, so
  the amendment is reviewable at plan-approve and lands in the same bead
  as the `next`-gate code. The clause is a scope-deferral: preflight leg
  only, no certification of `next`'s success-path mutation chain.
- **ADR-0040 (Orchestration Layering Ratchet) — unchanged, applied as the
  spine.** Deterministic invariants (MF-1..MF-4, the verb-level clarify
  rejections) live in the binary; judgment (SR-1..SR-5, whether a
  clarification RESOLVES) lives in prompts; the skills invoke the verb
  and route outcomes, restating no gate logic (the content pins keep
  them honest — R7b).
- **ADR-0035 (Agent Error Contract) — unchanged, applied.** Every
  ready-check FAIL, `next` refusal, and `clarify` rejection routes
  through `internal/guard.NewFailure` with per-signal `recovery:` lines
  naming one copy-pastable lever; raw `bd update --metadata` is never
  emitted (the clarify/override writes go through the verbs).
- **ADR-0034 (Ceremony Collapse) — unchanged, respected.** Two verbs +
  one flag, each paid for by a same-bead ceremony-baseline update
  (AC-9(i)); no config key, no score, no threshold.
- **ADR-0023 (Beads/Dolt as Single State Authority) — unchanged,
  respected.** Both metadata keys are advisory audit annotations via
  existing helpers; lifecycle state stays derived from bd statuses; the
  readiness verdict is never read by any lifecycle verb's gate.
- **ADR-0037 (Panel Gate as Enforced Contract) — unchanged, PROTECTED.**
  Readiness gains no merge authority: Bead 3's
  `internal/complete/readiness_isolation_test.go` pins `mindspec
  complete`'s gate evaluation byte-identical with and without readiness
  state (AC-9(ii)).
- **ADR-0036 (Ownership Discovery) — unchanged, respected.** The floor
  reads only repo artifacts and mindspec's own scaffold literals;
  it proposes no content. (Cited for the workflow-domain intersection;
  no resolution mechanics change.)

No ADR is superseded; no divergence requiring a human stop. Every bead
touches only workflow-owned (`internal/validate/**`, `internal/next/**`,
`internal/complete/**`, `internal/setup/**`, `cmd/**`,
`plugins/mindspec/**`) and execution-owned (`internal/bead/**`) paths the
spec's Impacted Domains declare, so each bead's own zero-override
`mindspec complete` is a live check of the decomposition.

## Testing Strategy

- **Hermetic engine fixtures (primary proof surface,
  `internal/validate/readiness/readiness_test.go`).** Table-driven over
  temp workspaces built by the EXPORTED
  `internal/validate/readiness/fixtures.go` builders: real
  spec.md/plan.md files (scaffold literals taken from
  `internal/approve/spec.go` `scaffoldPlan`, not re-typed), real git
  repos with real `--no-ff` merges + branch deletion for MF-3 states
  (the `internal/lifecycle/landed_test.go` shapes), bd reads behind the
  sub-package's injectable func-var seam (swapped to in-memory returns —
  the tests never consult a real bd and never `t.Skip`). The NEGATIVE
  builder plants exactly one defect per signal (placeholder-only AC
  block; dangling `AC-99` AND prefix-dangling `AC-1`-vs-only-`AC-19`;
  closed-but-unmerged branch-PRESENT dependency; a genuine `TBD` token +
  an unchecked item under a `**Blocking Questions**` region). The
  POSITIVE builder models spec 123 Bead 1 faithfully and MUST carry all
  four benign features (scaffold `**Verification**` checklist;
  `- [ ] AC-n — …` entries; a bare-base claim resolving via a
  sub-lettered spec token; a foreign "the spec 123 AC-17 pattern"
  citation) plus a code-quoted `TBD`/`OPEN QUESTION` literal and an
  enumerator-parenthetical `AC-9(i)`-shaped claim (rule-3 base-resolve) —
  a fixture-shape assertion fails if any benign feature is stripped (the
  AC-2 sanitized-fixture trapdoor).
- **RED-today discipline.** Every AC except the guard slices is RED on
  the spec-init SHA with zero product changes (the verbs, flag, skill
  content, and amendment text do not exist) and goes red again on
  revert. AC-9(i) is a deliberate baseline-diff record (the guard trips
  red on each surface until its same-bead baseline update — that
  tripping IS the mechanism); AC-9(ii) and AC-12(ii) are anti-overreach
  guards that pass once written and go red only against a
  non-conforming implementation (deviation stated in-test).
- **Verb/gate integration at cmd level (stateful fake-bd shim, no
  skip).** `ready-check` exit codes, idempotence (two runs
  byte-identical), no-mutation audits (`git status --porcelain`, branch
  list, worktree list, bd status before/after), hostile-render (AC-8, the
  `cmd/mindspec/next_orphan_render_test.go` pattern), and the AC-10
  temporal flow (real `plan approve` → `ready-check` FAIL on MF-3 →
  real `mindspec complete` of the dep with branch deletion →
  `ready-check` PASS) live in `cmd/mindspec/bead_ready_test.go` /
  `next_ready_gate_test.go` — the cmd package imports the full verb
  stack, so the flow runs the real completion path. These bd-mutating
  flows install a stateful fake-bd shim on PATH (the
  `writeFakeBD`/`fakeBdDir` precedent) UNCONDITIONALLY — so a missing
  real bd is a hard failure of setup, never a `t.Skip` that would
  silently un-enforce the RED-today ACs (plan-gate F3-1). The
  no-mutation audit is scoped to the CLAIM/branch/worktree lifecycle
  state the gate prevents; see the AC-4 scoping note in Bead 2.
- **Skill content pins (`internal/setup/readiness_skill_pin_test.go`).**
  The spec-123 AC-17 pattern via `pluginmindspec.SkillFiles()` (the
  exact surface `mindspec setup` installs from — see
  `internal/setup/adhoc_panel_skill_test.go`): AC-5's
  unconditional-ingress invocation + override-honoring text, AC-6's
  Phase 0 block (five SR IDs, zero-commit rule, `NOT READY: <bead-id>`
  first-line + ordinal shape, clarification-handling rule), AC-7's
  NOT-READY routing, AC-13's loop contract (dispositions, grounding,
  categorical cap, clarify-vs-override distinction).
- **Integration gates (every bead).** `go build ./...`,
  `go test ./...` (no new red; the known `mindspec-z4ps` flake is the
  only tolerated exception), `go vet ./...`, `gofmt -l` clean,
  `golangci-lint run ./...`,
  `mindspec validate spec 124-impl-readiness-gate`, and a zero-override
  `mindspec complete`. Review evidence maps every AC-1..AC-16 to exact
  `go test <package> -run <test>` commands per the spec's Validation
  Proofs.

## Bead 1: Mechanical-floor engine + `bead ready-check` verb + the readiness fixture pair

R1 + R2 + R6 in full, plus R7a's first slice (the `ready-check` ceremony
pin). The deterministic core: four signals, a read-only report verb, and
the CI-pinned NEGATIVE/POSITIVE fixture pair that makes the gate
falsifiable both ways.

**Steps**
1. `internal/bead/readiness.go` (new, execution-domain): the two
   dedicated metadata key constants
   (`MetaKeyReadinessOverride = "mindspec_readiness_override"`,
   `MetaKeyReadinessAttempt = "mindspec_readiness_attempt"`) with doc
   comments stating the carrier contract: written only via
   `MergeMetadata` by their owning verbs (Beads 2/3), NEVER read by any
   mechanical signal, advisory-not-lifecycle (ADR-0023).
2. The sub-package `internal/validate/readiness/` (new — see the
   placement rationale in the preamble):
   - `report.go`: the `ReadinessReport` type (four per-signal results —
     stable IDs `MF-1`..`MF-4`, PASS/FAIL, evidence path +
     offending/missing element, one recovery lever per FAIL) and its
     renderer as a reachable exported API, so Bead 2's `next` refusal
     reuses it (ADR-0040 no-restate).
   - `readiness.go`: the engine. `EvaluateReadiness(root, beadID)` returns
     a `ReadinessReport`. Ingress: `idvalidate.BeadID`. Owning-spec
     resolution from LINEAGE (the `internal/complete/complete.go:369`
     fail-closed pattern via `phase` epic lookup — never cwd; real lookup
     errors refuse, they never degrade to cwd resolution). Bead index
     N = the ID's numeric suffix, mapped positionally to
     `work_chunks[N-1]` / the Nth `## Bead` section via
     `internal/validate`'s existing plan frontmatter + section parsing.
     It declares the **package-level bd seam covering EVERY bd read
     `EvaluateReadiness` performs** (plan-gate G2-BDLESS-ENGINE-SEAM),
     func vars defaulting to the real `internal/bead` helpers (the
     `adr_memo.go:20` precedent). The COMPLETE bd read-set, each routed
     through the seam so the engine is fully hermetic with bd absent (no
     `t.Skip`):
     - the **bead's OWN record** — its bd description (MF-2 harvest
       context + the MF-4 description scan) and, indirectly, the plan
       path resolved from lineage (MF-1's `## Bead N` section, MF-2's
       plan-section harvest, MF-4's plan-section scan all read the plan
       FILE, not bd — but the bead→spec LINEAGE lookup that locates the
       plan is a bd/phase read and is behind the seam);
     - each **dependency edge's bd record** — the dependency ID list, and
       per dep its bd STATUS/closure (MF-3);
     (MF-3's landed-merge leg is `lifecycle.FindLandedMerge`, a git read,
     not bd — it is exercised over real temp repos, not faked.) No MF
     signal reaches `internal/bead` except through these func vars, so
     the engine unit tests swap them ALL to in-memory returns and consult
     no real bd for the bead's-own-record reads OR the dependency reads.
3. The four signals, exactly as R2 pins them:
   - **MF-1**: section exists; `**Acceptance Criteria**` block has ≥1
     entry that is not the scaffold's angle-bracketed template literal
     (matched against the actual `scaffoldPlan` emission);
     `work_chunks[N-1].key_file_paths` non-empty, no `path/to/file.go`.
   - **MF-2**: harvest the bead's OWN claimed `R<n>`/`AC-<n>` tokens
     (bd description + its plan section) under the three harvest rules
     from the preamble — foreign-citation exclusion, code-span/code-fence
     exclusion, and clause-enumerator normalization (`AC-9(i)`→base
     `AC-9`). Resolve each against spec.md by the exact rule — whole-token
     OR base-of-sub-lettered for letterless claims (`R5` via `R5a`;
     `R5(b)`→`R5b` only when the letter form targets a real spec token),
     EXACT token only for genuinely sub-lettered claims, numeric-prefix
     never resolves (identifier-boundary matching).
   - **MF-3**: for every bd dependency edge (authoritative — the prose
     `**Depends on**` section is never parsed): dep closed in bd AND
     `lifecycle.FindLandedMerge(root, specBranch, depID)` returns a
     positive `*LandedMerge`. FAIL fail-closed on ANY error, with
     state-specific messages: open dep; `ErrLandedMergeNotFound` (the
     2u0u closed-but-unmerged split); `*LandedMergeNoEvidence` (the
     spec-121 uncorroborated candidate); infra errors verbatim. No
     file-existence leg (spec Out of Scope).
   - **MF-4**: whole-word `TBD` / whole-phrase `OPEN QUESTION`
     (case-insensitive) over the bead's plan section + bd description,
     scanned OUTSIDE inline-code spans and fenced code blocks; unchecked
     `- [ ]` items only under a designated blocking region
     (`**Blocking Questions**` / `**Open Questions**` / `**Blocked on**`
     sub-blocks). BOTH the token scan AND the blocking-region-HEADER
     detection exclude backtick-spanned/fenced text (plan-gate F2-2) — so
     a section that merely NAMES `**Blocking Questions**` inside a code
     span (as this plan's Bead-1 section does when describing MF-4) is not
     read as an actual blocking region. Routine
     `**Verification**`/`**Acceptance Criteria**` checklists never
     blocking.
4. `cmd/mindspec/bead_ready.go` (new file, own `init()` on `beadCmd` —
   `cmd/mindspec/bead.go` is NOT edited): the `ready-check <bead-id>`
   subcommand, calling `readiness.EvaluateReadiness` and rendering via
   the sub-package's `report.go` renderer. Prints one line per signal;
   all bead/plan-derived text through `termsafe.Escape`; exit 0 all-pass;
   on any FAIL exit non-zero via `guard.NewFailure` carrying one
   `recovery:` line per failing signal (edit the named plan section;
   `mindspec complete <dep-id>`; `--allow-not-ready` documented as the
   claim-time override). No bd write, no git write, no file write on ANY
   path.
5. Fixtures + engine tests: `internal/validate/readiness/fixtures.go`
   (the exported NEGATIVE/POSITIVE builders per Testing Strategy) and
   `internal/validate/readiness/readiness_test.go`: MF-2 all-classes
   table (AC-14 i-iv, both false-positive-refusal and
   false-negative-refusal arms) PLUS the enumerator-parenthetical arm
   pinning the FORM-based Roman-vs-sub-letter classification: a claimed
   `AC-9(i)` PASSes via base `AC-9` EVEN WHEN a coincidental `AC-9i`
   sub-token also exists in spec.md (form wins — collision-free), a
   claimed `R5(b)` resolves to exact `R5b`, and a genuinely
   dangling-as-written literal `AC-9i` FAILs — so strict-exact,
   any-parenthetical-is-base, AND existence-based-disambiguator impls
   each go red on the wrong arm; MF-3
   four variants (AC-3 i-iv: open; closed+branch-present+unmerged;
   closed+landed+branch-DELETED; closed+landed+branch-present) driven
   through real `FindLandedMerge` over real repos; MF-4 exclusion table
   (code-quoted literals PASS; a backtick-quoted `**Blocking Questions**`
   header is NOT a real region; a genuine blocking-region unchecked item
   FAILs; scaffold checklists PASS); AC-12 layer boundary BOTH
   directions — seed `MetaKeyReadinessAttempt` via `MergeMetadata` (the
   engine seam) with (i) a record "addressing" a planted MF-2 failure →
   still FAIL, and (ii) a record whose prose carries `AC-7`, `TBD`, and
   an unchecked `- [ ]` on the POSITIVE bead → still PASS, verdicts
   byte-identical with/without the record. Engine tests swap the bd seam
   to in-memory returns (hermetic, no `t.Skip`).
6. Verb tests (`cmd/mindspec/bead_ready_test.go`), all installing the
   stateful fake-bd shim unconditionally (no skip): AC-1 (negative:
   non-zero exit, four FAIL lines each naming its planted defect +
   recovery), AC-2 (positive: exit 0, benign features present —
   fixture-shape assertion — idempotent double-run, before/after
   no-mutation audit scoped to claim/branch/worktree state), AC-8
   hostile bd-description render via `termsafe` (the
   `next_orphan_render_test.go` pattern), malformed-ID ingress refusal,
   and AC-10's end-to-end temporal flow (real `plan approve` → bead 2
   FAILs MF-3 → real `mindspec complete` of bead 1 (branch deleted) →
   bead 2 PASSes).
7. `cmd/mindspec/ceremony_guard_test.go`: add the `mindspec bead`
   subcommand-set pin (existing five children + `ready-check`) and the
   `ready-check` flag-set pin (`--help`, `--trace` only) — this bead's
   deliberate AC-9(i) baseline addition, in the same commit as the verb.

**Verification**
- [ ] `go test ./internal/validate/readiness/ -run 'TestReadiness'` and
  `go test ./cmd/mindspec/ -run 'TestBeadReadyCheck'` pass (final names
  recorded in review evidence per AC)
- [ ] No test-build import cycle: `go test ./internal/lifecycle/ ./...`
  builds clean (the sub-package placement severs
  `lifecycle[test] → validate → lifecycle`)
- [ ] AC-1/AC-2/AC-3/AC-8/AC-10/AC-14 subtests RED on the spec-init SHA
  (the verb does not exist); AC-12's pass-stays-pass arm is an
  anti-overreach guard (deviation tagged in-test)
- [ ] AC-14 enumerator arm: `AC-9(i)` PASSes via base `AC-9` while a
  literal `AC-9i` claim FAILs (both wrong impls go red)
- [ ] bd-less hermeticity: engine tests green with `bd` OFF PATH (seam
  swapped); cmd tests install the fake-bd shim and FAIL (never skip)
  if it is unavailable
- [ ] POSITIVE-fixture shape assertion red when any of the four benign
  features is stripped (sanitized-fixture trapdoor demonstrated)
- [ ] No-mutation audit: `git status --porcelain` empty (excluding the
  gate-independent `.beads/` tracker-sync — AC-4 scoping), branch/
  worktree lists and bd status byte-identical before/after both fixture
  runs
- [ ] Ceremony guard: green with exactly the `bead` subcommand +
  `ready-check` flag pins added (baseline diff shows only those)
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat);
  `golangci-lint run ./...` clean; `gofmt -l` clean;
  `mindspec validate spec 124-impl-readiness-gate` passes; bead
  completes with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-1 — negative fixture refused: non-zero exit, FAIL line per
  MF-1..MF-4 naming each planted defect, each with a `recovery:` line
  (RED today)
- [ ] AC-2 — positive fixture passes with all four benign features +
  code-quoted `TBD`, read-only and idempotent; sanitized fixture fails
  the shape assertion (RED today)
- [ ] AC-3 — MF-3 via `FindLandedMerge`: all four dependency variants,
  branch-deletion-tolerant, fail-closed on both error states (RED today)
- [ ] AC-8 — hostile bd-description bytes escaped via `termsafe` in the
  report (RED today)
- [ ] AC-10 — temporal flow through real `plan approve` + real
  `complete`: readiness re-derived per invocation (RED today)
- [ ] AC-12 — layer boundary both directions: fail-stays-fail with a
  seeded attempt record; pass-stays-pass with hostile-token record
  prose; mechanical verdicts byte-identical with/without the record
- [ ] AC-14 — MF-2 exact resolution, all four classes + the
  clause-enumerator-parenthetical arm, paired refusal-both-ways fixtures
  (RED today)

**Depends on**
None (foundational; sole W1 root). (Human-readable narration only — bd
edges are wired exclusively from `work_chunks[].depends_on`.)

## Bead 2: `next` gate-before-mutate + `--allow-not-ready` + the ADR-0041 amendment

R3 + R9 in full, plus R7a's second slice. Wires the Bead-1 floor into
`mindspec next`'s claim path ahead of every mutation, adds the durable
override, and finalizes the pre-drafted ADR-0041 fourth-verb clause in
the same bead as the gate code (AC-16's same-bead pin).

**Steps**
1. `internal/next/ready_gate.go` (new): `GateReadiness(root, beadID,
   allowNotReady bool)` — calls `readiness.EvaluateReadiness` (the Bead-1
   sub-package) and renders refusals via its shared `report.go` renderer
   (ADR-0040 no-restate); on a failing floor with `allowNotReady=false`
   returns the refusal error (per-signal report + per-signal recovery
   lines + BOTH escape hatches: `mindspec bead ready-check <id>` and
   re-run with `--allow-not-ready`), built via `guard` so the refusal is
   ADR-0035-shaped; on `allowNotReady=true` returns the failing-signal
   list for the caller's warning + marker write; on a passing floor
   returns a single OK line's worth of state (no interactive step, no
   extra output). Also `RecordReadinessOverride(beadID, signals)` —
   one `MergeMetadata` write of `MetaKeyReadinessOverride`
   (signal IDs + UTC timestamp).
2. `cmd/mindspec/next.go`: add the `--allow-not-ready` flag; invoke the
   gate immediately after bead SELECTION (Step 4) and BEFORE any
   lifecycle mutation — before `formatClaimLine`/`ClaimBead`, the
   `bead/<id>` branch, and the worktree. (Note the ordering fact from the
   plan-gate: `next`'s Step 1 runs a pre-existing, idempotent,
   readiness-INDEPENDENT `.beads/` tracker-sync — the ADR-0025 dirty-tree
   normalization that may `bd export` on EVERY invocation regardless of
   the readiness outcome. That sync is NOT a mutation the readiness gate
   prevents or is responsible for, and it is explicitly OUT of AC-4's
   zero-mutation guarantee — see the AC-4 scoping note in step 5. The gate
   guarantees only that the CLAIM + branch + worktree never land on a
   refusal.) `--emit-only` performs no claim/branch/worktree mutation and
   is not gated. On refusal: exit non-zero, no claim/branch/worktree
   created. On override: stderr warning naming every failing signal, then
   claim, then `RecordReadinessOverride` immediately after `ClaimBead`
   succeeds and before `EnsureWorktree` (the write-ordering choice
   recorded in the preamble). `--force` keeps its session-freshness
   meaning at `next.go:104` untouched — it gains no readiness authority.
   The gate call site carries a code comment CITING the ADR-0041
   "preflight-leg-only addition" clause (AC-16: the code cites it).
3. Finalize the ADR-0041 §1 fourth-verb clause (pre-drafted at plan
   time in this worktree — remove the `PRE-DRAFT` marker comment;
   adjust wording only where the implementation forces it). It must
   keep the AC-16 anchors: the phrase "preflight-leg-only addition"
   co-located with `mindspec next`; the scope-deferral framing (no
   certification of `next`'s success-path claim/branch/worktree
   mutation chain, which retains its existing contract); the
   byte-identical-on-refusal statement; the `--allow-not-ready`-vs-
   `--force` distinction; the "read `next` as a fourth member for the
   preflight leg only" reconciliation of the three-verb wording.
4. `cmd/mindspec/adr0041_amendment_test.go` (new; the
   `adr0032_amendment_test.go` / `adr0040_anchor_test.go` pattern):
   asserts, over a WHITESPACE-NORMALIZED read of the shipped ADR (collapse
   runs of whitespace/newlines to single spaces before matching, so
   line-wrap never splits the anchor — plan-gate F3-2), that the file
   contains "preflight-leg-only addition" within the same clause as
   `mindspec next` (the discriminating anchor — vacuously ABSENT from the
   pre-spec ADR, so the assertion goes green only after the amendment),
   contains the scope-deferral sentence, and contains no residual
   `PRE-DRAFT` marker. (The ADR pre-draft is authored so the anchor phrase
   and `mindspec next` already sit on one raw line, but the test
   normalizes regardless so a future reflow cannot break it.)
5. Gate tests (`cmd/mindspec/next_ready_gate_test.go` +
   `internal/next/ready_gate_test.go`), installing the fake-bd shim
   unconditionally and reusing the Bead-1 fixture builders. **AC-4
   scoping (plan-gate G2-AC4-SCOPE, refuted-as-a-dodge — this IS the
   spec's own enumeration):** the "byte-identical" audit is pinned
   against EXACTLY the three items spec.md:137 enumerates — the bd STATUS
   of the selected bead, the `git branch --list 'bead/*'` set, and the
   worktree list — captured before/after. It is NOT pinned against
   `.beads/issues.jsonl`, and that is not a narrowing of the spec: the
   pre-gate ADR-0025 tracker-sync is idempotent and gate-INDEPENDENT (it
   syncs the dolt↔jsonl REPRESENTATION and runs on every `next`
   regardless of the readiness outcome), and it provably touches NONE of
   the three audited items — the bead's bd status, the branch list, and
   the worktree list are all unchanged by a representation sync. So the
   plan's pin is the spec's AC-4 text verbatim, not a scope dodge.
   Against the NEGATIVE bead, `next` exits non-zero and that
   audited state is byte-identical to pre-invocation; with
   `--allow-not-ready` the claim proceeds, stderr names all failing
   signals, and `bd show` reveals the durable marker naming those
   signals; `--force` alone does NOT bypass the gate (still refused);
   against the POSITIVE bead `next` claims normally, no prompt, no
   refusal, output beyond the existing claim lines limited to the single
   OK line.
6. `cmd/mindspec/ceremony_guard_test.go`: add the `mindspec next`
   flag-set pin (recorded from pflag metadata: the existing `--pick`,
   `--spec`, `--force`, `--emit-only`, `--help`, `--trace` set) plus
   the new `--allow-not-ready` — this bead's deliberate AC-9(i)
   baseline addition.

**Verification**
- [ ] `go test ./cmd/mindspec/ -run 'TestNextReadyGate|TestADR0041Amendment'`
  and `go test ./internal/next/ -run 'TestGateReadiness'` pass (final
  names in review evidence)
- [ ] AC-4 subtests RED on the spec-init SHA (`--allow-not-ready` does
  not exist; `next` claims the negative bead today); byte-identical
  refusal audit (claim/branch/worktree state; NOT `.beads/` jsonl) green
  after
- [ ] `rg -n 'preflight-leg-only addition' .mindspec/adr/ADR-0041-gate-before-mutate.md`
  non-empty and on a line also naming `mindspec next` (the amendment test
  uses whitespace-normalized matching regardless);
  `rg -n 'PRE-DRAFT' .mindspec/adr/ADR-0041-gate-before-mutate.md`
  empty; the `next.go` gate call site cites the clause
- [ ] `--force`-only invocation still refused on the negative bead
  (orthogonality pinned)
- [ ] Ceremony guard: green with exactly the `next` flag pin +
  `--allow-not-ready` added (baseline diff shows only those)
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat);
  `golangci-lint run ./...` clean; `gofmt -l` clean; bead completes
  with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-4 — gate-before-mutate: byte-identical refusal audit (scoped to
  claim/branch/worktree state, not the gate-independent `.beads/`
  tracker-sync); override claims + durable marker naming the failing
  signals; `--force` orthogonal; positive bead claims with no prompt
  (RED today)
- [ ] AC-16 — amended ADR-0041 §1 carries the "preflight-leg-only
  addition" anchor co-located with `mindspec next`, scope-deferral
  framing, code citation, `PRE-DRAFT` marker gone — landed in THIS bead
  with the gate code (RED today: the anchor is absent from the shipped
  ADR)

**Depends on**
Bead 1 (the gate calls the Bead-1 engine and renders its report; the
ceremony-guard seam is sequenced behind Bead 1's edit). (bd edges wired
from `work_chunks[].depends_on`.)

## Bead 3: Skill layer — dispatch ingress, Phase 0, NOT-READY routing, the bounded clarification loop + `bead clarify`

R4 + R5 + R8 in full, plus R7b and R7a's final slice. The judgment layer:
the unconditional dispatch-ingress re-check, the Phase 0 semantic review
in the staged prompt, the cycle's pre-damage NOT-READY routing, and the
once-per-bead grounded clarification loop with its thin verb — plus the
content pins that keep prose and binary attached, and the AC-9
close-out (full ceremony accounting + the complete-gate isolation
fixture).

**Steps**
1. `internal/bead/clarify.go` (new): the attempt-record schema
   (original NOT-READY report: `{ordinal, reason, signal}` entries;
   clarifications: `{ordinal, reason, answer, span}` entries; all
   free-text fields treated as agent-authored — escaped at render
   sites) and `WriteAttemptRecord(beadID, record)` enforcing the
   verb-level contract: REFUSE when `MetaKeyReadinessAttempt` already
   exists (the categorical one-round-per-bead cap — recovery: escalate
   to plan/spec revision); REFUSE a clarification naming an ordinal
   absent from the record's own report; REFUSE a clarification with an
   empty `span` (presence check ONLY — whether the span SUPPORTS the
   answer is the fresh Phase-0 subagent's judgment, per R8b/ADR-0040).
   Exactly one `MergeMetadata` write on success; no update/finalize
   API exists (R8e derive-don't-write).
2. `cmd/mindspec/bead_clarify.go` (new file, own `init()` on `beadCmd`
   — `bead.go` again untouched): `mindspec bead clarify <bead-id>
   --file <record.json>`. Ingress `idvalidate.BeadID`; parse + validate
   via `internal/bead/clarify.go`; ADR-0035 refusals with single-lever
   recovery lines (second attempt → "escalate: revise the plan/spec
   section the reasons quote"; unknown ordinal / missing span → fix the
   record file). No other flag; no terminal-stamp surface.
3. `plugins/mindspec/skills/ms-bead-impl/SKILL.md` (R4a/R4b/R8c): a new
   `## Ingress — readiness re-check (EVERY dispatch path)` section
   ABOVE Phase A, stating: run `mindspec bead ready-check <bead-id>`
   first, unconditionally — a supplied `prompt-path` (which skips
   Phase A) and the manual `bd update`/`git worktree add` fallback
   still hit it; on FAIL with no override marker STOP (no prompt
   staged, no dispatch); on FAIL where `bd show` reveals the
   `mindspec_readiness_override` marker covering the failing signals,
   proceed with a warning naming the overridden signals; the ingress
   also reads `mindspec_readiness_attempt` and injects its
   clarification entries (keyed by ordinal) into the staged prompt's
   Phase 0 section on re-dispatch. The Phase-A prompt skeleton gains
   the mandatory `## Phase 0 — readiness review (before any edit)`
   block: the five signals SR-1..SR-5 verbatim from R4b; the
   zero-commit rule; the report contract (`NOT READY: <bead-id>` first
   line; reasons numbered by ordinal, unique, each tagged with its SR
   ID, quoting the offending/missing span verbatim, plus the concrete
   unblocking question); the clarification-handling rule (any cited
   ordinal lacking a mapped, span-grounded entry is re-reported NOT
   READY; the reviewer judges whether the entry RESOLVES the ambiguity
   against its cited span — a bare "it's ready" never flips the
   verdict); and the one-line "Phase 0: READY" pass path.
4. `plugins/mindspec/skills/ms-bead-cycle/SKILL.md` (R5/R8): a new
   NOT-READY outcome section in the Sequence: a return whose first
   line is `NOT READY: <bead-id>` consumes NO panel round, does NOT
   count toward `loop.halt.max_consecutive_impl_failures` (pre-damage
   refusal, not an impl failure), is NEVER routed to `/ms-bead-fix`,
   and leaves the bead worktree intact. Exactly two dispositions:
   **ACCEPT** (default) — halt the bead, surface the reasons, route to
   plan/spec revision of the quoted sections, then re-dispatch;
   **DISAGREE/clarify** — once per bead, author the grounded
   reason-keyed record (ordinal + verbatim reason + concrete answer +
   authoritative source span; a clarification may DISAMBIGUATE existing
   authority, never CREATE new normative behavior — when no span
   supports the answer, ACCEPT is the correct disposition) and write it
   via `mindspec bead clarify <id> --file <record.json>`, then
   re-dispatch. The cap is categorical and durable: ANY prior attempt
   record forces the next NOT READY (new reasons included) to ACCEPT;
   the verb enforces it, restart-proof. Document the
   clarify-vs-`--allow-not-ready` distinction
   (resolve-with-evidence vs blunt recorded bypass; a clarification
   never moves a mechanical signal). In the same step,
   `plugins/mindspec/skills/ms-spec-autopilot/SKILL.md` gains the
   ACCEPTed-NOT-READY row in its existing halt table (bead-level halt;
   surface the ordinal report + revision routing; do not proceed to
   the next bead).
5. `internal/setup/readiness_skill_pin_test.go` (new; the
   `adhoc_panel_skill_test.go` pattern over
   `pluginmindspec.SkillFiles()`): AC-5's content half (the ingress
   invocation appears BEFORE the Phase A section and names both the
   `prompt-path` and manual-fallback paths + the override-marker
   proceed-with-warning rule); AC-6 (Phase 0 block: all five SR IDs,
   zero-commit rule, `NOT READY: <bead-id>` + ordinal-verbatim-span
   shape, clarification-handling rule); AC-7 (cycle routing: no panel
   round, `max_consecutive_impl_failures` exclusion, never
   `/ms-bead-fix`, ACCEPT halt-and-surface); AC-13 (both dispositions,
   grounded reason-keyed requirement, categorical per-bead cap
   wording, clarify-vs-override distinction).
6. Verb + record tests (`cmd/mindspec/bead_clarify_test.go`,
   `internal/bead/clarify_test.go`), installing the stateful fake-bd
   shim unconditionally (the `writeFakeBD`/`fakeBdDir` precedent; the
   fake persists metadata across invocations so a fresh-process
   `bd show` reads a prior write — no `t.Skip` if real bd is absent):
   AC-15 — after one write, a second `clarify` (including a
   renamed/renumbered-reason record) is refused by a FRESH process with
   no transcript state; the trail (report + entries) survives worktree
   teardown and a fresh `bd show` (AC-11 durability), written exactly
   once; no `--finalize` flag exists (asserted against the command's
   pflag set); unknown-ordinal and missing-span rejections pinned
   deterministically. **AC-5 behavioral evidence (plan-gate F1-1):** the
   ingress FAIL-stops-dispatch and FAIL-with-override-proceeds behaviors
   are pinned COMPOSITIONALLY — from the Bead-1 verb exit semantics
   (`ready-check` exits non-zero on FAIL, zero with the override marker
   honored is a skill-read, not a verb concern) plus Bead-2's AC-4
   override-marker write pin — with the skill's ingress ordering
   asserted by the step-5 content pin; the review evidence records a
   manual `ready-check`→dispatch flow transcript demonstrating the
   stop/proceed branches end-to-end (there is no separate binary that
   "dispatches", so AC-5's behavioral half is this composition, named
   explicitly here).
7. The AC-9 close-out, both halves: (ii)
   `internal/complete/readiness_isolation_test.go` (new) — a
   force-claimed bead carrying BOTH the override marker and an attempt
   record goes through `mindspec complete`'s gate evaluation
   byte-identically to the same bead without them (readiness has zero
   merge authority; ADR-0037 protected); and (i)
   `cmd/mindspec/ceremony_guard_test.go` — extend the `bead`
   subcommand pin with `clarify` and pin `clarify`'s flag set
   (`--file`, `--help`, `--trace`), the final AC-9(i) slice; the
   full-spec baseline diff across Beads 1-3 shows exactly the two
   verbs + one flag the spec pays for.

**Verification**
- [ ] `go test ./internal/setup/ -run 'TestReadinessSkillPins'`,
  `go test ./internal/bead/ -run 'TestClarify'`,
  `go test ./cmd/mindspec/ -run 'TestBeadClarify'`, and
  `go test ./internal/complete/ -run 'TestReadinessIsolation'` pass
  (final names in review evidence)
- [ ] AC-5/AC-6/AC-7/AC-11/AC-13/AC-15 subtests RED on the spec-init
  SHA (the skill content, verb, and record do not exist); AC-9(ii) is
  an anti-overreach guard (deviation tagged in-test)
- [ ] Second-clarify refusal demonstrated from a fresh process (no
  transcript memory) including the renamed-reason variant; `bd show`
  trail asserted after worktree teardown — the clarify/`bd show` tests
  install the fake-bd shim unconditionally and FAIL (never skip) if it
  is unavailable (bd-less CI enforcement, plan-gate F3-1)
- [ ] The shipped `ms-bead-impl` ingress section precedes Phase A and
  is reachable from BOTH the `prompt-path` and manual-fallback flows
  (content-pinned); removing the clarification-handling rule turns
  AC-6's pin red (mutation probe recorded)
- [ ] Ceremony guard: green with exactly `clarify` + its flag set
  added; cumulative baseline diff = `bead ready-check`, `bead clarify`,
  `--allow-not-ready` and nothing else
- [ ] `go build ./... && go test ./...` no new red (z4ps caveat);
  `golangci-lint run ./...` clean; `gofmt -l` clean; bead completes
  with zero `--override-adr`

**Acceptance Criteria**
- [ ] AC-5 — unconditional dispatch ingress on every path, override
  marker honored with a warning; behavioral evidence composed from the
  Bead-1 verb exit-semantics + Bead-2 AC-4 marker pins + the step-5
  ingress-ordering content pin + a recorded end-to-end flow transcript
  (RED today)
- [ ] AC-6 — Phase 0 contract content-pinned: five SR IDs, zero-commit
  rule, `NOT READY:` ordinal-verbatim-span shape, clarification-handling
  rule (RED today)
- [ ] AC-7 — NOT-READY routing content-pinned: no panel round, halt-
  counter exclusion, never `/ms-bead-fix`, ACCEPT halt-and-surface
  (RED today)
- [ ] AC-9 — ceremony accounting complete (exactly two verbs + one flag
  across the spec's baseline diffs) AND complete-gate isolation
  fixture (readiness state has zero effect on `mindspec complete`)
- [ ] AC-11 — attempt record durable across worktree teardown + fresh
  `bd show`; ingress injection of ordinal-keyed entries content-pinned
  (RED today)
- [ ] AC-13 — bounded-loop contract content-pinned: both dispositions,
  grounded entries, categorical per-bead cap, clarify-vs-override
  distinction (RED today)
- [ ] AC-15 — restart-proof cap + derive-don't-write: fresh-process
  second-clarify refusal (renamed reasons included), exactly-once
  durable trail, no terminal-stamp surface (RED today)

**Depends on**
Beads 1 and 2 (the ingress invokes Bead 1's verb; the override-marker
honoring consumes Bead 2's durable marker; the ceremony-guard seam is
sequenced last). (bd edges wired from `work_chunks[].depends_on`.)

## Provenance

Every spec AC maps to exactly ONE owning bead. AC-9 is owned by Bead 3
(the completing full-set assertion + the isolation fixture) while R7a's
same-bead baseline updates are distributed — each of Beads 1/2/3 lands
its own surface's pin in the commit that adds the surface, and Bead 3's
cumulative-diff check is the single verifying assertion. AC-12 is owned
by Bead 1 (the invariance is an engine property, pinned by seeding the
carrier key directly); Bead 3's clarify tests re-exercise it with a
verb-written record as belt-and-braces, asserting the same property,
not a new one.

| Acceptance Criterion | Satisfied By | Verified By |
|---------------------|--------------|-------------|
| AC-1 (negative fixture refused with evidence) | Bead 1 Steps 2-6 | Bead 1 verification: negative-fixture verb subtests |
| AC-2 (positive fixture passes, benign features, read-only, idempotent) | Bead 1 Steps 2-6 | Bead 1 verification: positive-fixture + shape-trapdoor subtests |
| AC-3 (MF-3 landed-merge predicate, branch-deletion-tolerant, fail-closed) | Bead 1 Steps 3, 5 | Bead 1 verification: four-variant real-repo subtests |
| AC-4 (gate-before-mutate in `next` + durable override + `--force` orthogonality) | Bead 2 Steps 1-2, 5 | Bead 2 verification: byte-identical state audit + marker subtests |
| AC-5 (unconditional dispatch ingress, override honored) | Bead 3 Steps 3, 5 | Bead 3 verification: ingress content pins |
| AC-6 (Phase 0 contract pinned) | Bead 3 Steps 3, 5 | Bead 3 verification: Phase 0 content pins + mutation probe |
| AC-7 (NOT-READY routing pinned) | Bead 3 Steps 4-5 | Bead 3 verification: cycle-routing content pins |
| AC-8 (hostile-input render safety) | Bead 1 Steps 4, 6 | Bead 1 verification: hostile-render subtest |
| AC-9 (ceremony accounting + complete-gate isolation) | Bead 3 Step 7 (slices in Bead 1 Step 7, Bead 2 Step 6) | Bead 3 verification: cumulative baseline diff + isolation fixture |
| AC-10 (end-to-end temporal flow via real approve/complete) | Bead 1 Step 6 | Bead 1 verification: temporal-flow integration subtest |
| AC-11 (clarification durability + injection, off the scan surface) | Bead 3 Steps 1-3, 6 | Bead 3 verification: durability subtests + injection content pin |
| AC-12 (layer boundary both directions) | Bead 1 Step 5 | Bead 1 verification: seeded-record invariance subtests |
| AC-13 (bounded loop + audit contract pinned) | Bead 3 Steps 4-5 | Bead 3 verification: loop-contract content pins |
| AC-14 (MF-2 exact resolution, all classes) | Bead 1 Steps 3, 5 | Bead 1 verification: paired-refusal MF-2 table |
| AC-15 (restart-proof cap, derive-don't-write) | Bead 3 Steps 1-2, 6 | Bead 3 verification: fresh-process refusal + exactly-once trail |
| AC-16 (ADR-0041 amendment anchor, same bead as gate code) | Bead 2 Steps 3-4 (pre-drafted at plan time; finalized there) | Bead 2 verification: anchor test + rg proofs, marker gone |

The spec's Validation Proofs commands are distributed per-bead (each
bead runs its package subset; the skill greps run from Bead 3 on; the
ADR anchor rg from Bead 2 on). Requirement coverage: R1/R2/R6 → Bead 1;
R3/R9 → Bead 2; R4/R5/R8/R7b → Bead 3; R7a → each bead's same-commit
ceremony-baseline slice.
