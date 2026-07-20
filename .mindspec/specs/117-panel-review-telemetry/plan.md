---
status: Draft
spec_id: 117-panel-review-telemetry
version: "1"
adr_citations:
  - ADR-0037
  - ADR-0023
  - ADR-0040
  - ADR-0041
  - ADR-0042
  - ADR-0043
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/panel/disposition.go
      - internal/panel/disposition_test.go
      - cmd/mindspec/panel_disposition.go
      - .mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md
  - id: 2
    depends_on: [1]
    key_file_paths:
      - internal/panel/disposition.go
      - internal/panel/disposition_store.go
      - internal/panel/disposition_store_test.go
      - cmd/mindspec/panel_disposition_store.go
      - cmd/mindspec/panel_disposition.go
  - id: 3
    depends_on: [1]
    key_file_paths:
      - internal/panel/disposition.go
      - internal/panel/disposition_query.go
      - internal/panel/disposition_query_test.go
      - cmd/mindspec/panel_disposition_query.go
      - cmd/mindspec/panel_disposition.go
      - internal/panel/testdata/seed116/
  - id: 4
    depends_on: [1, 2]
    key_file_paths:
      - internal/panel/disposition.go
      - internal/panel/disposition_store.go
      - internal/panel/disposition_migrate.go
      - internal/panel/disposition_migrate_test.go
      - plugins/mindspec/skills/ms-panel-tally/SKILL.md
      - .claude/skills/ms-panel-tally/SKILL.md
      - .mindspec/specs/116-panel-message-escaping/reviews/
---
# Plan: 117-panel-review-telemetry

Turns the approved spec into four implementation beads that build a durable,
queryable panel-disposition telemetry store as **per-panel JSONL** under each
spec's `reviews/` dir, written and queried by a **new Go verb family**
(`mindspec panel disposition …`). Bead 1 is foundational (schema + validator +
ADR-0043); Beads 2 and 3 are the parallel Wave 2 (each depends only on Bead 1);
Bead 4 (migration + skill) is Wave 3 and depends on Bead 1 + Bead 2 (it writes
migrated records through Bead 2's append op). Graph: `1→2→4`, `1→3`; acyclic,
serial depth 3.

## Write-mechanism decision — Go verb, not a tracked script

The spec left one sub-choice open (Requirement 6 / OQ1): the validator + append +
query mechanism is a `mindspec` verb OR a tracked jq/bash script. **This plan
binds it to a Go verb family, `mindspec panel disposition` with leaves
`validate` / `append` / `check` / `query`.** A Go verb — and ONLY a Go verb —
inherits the five mechanisms the spec mandates:

1. **Safe-render surfaces (ADR-0042).** Every rendered `summary`/`note`/`reviewer`
   value is agent-writable untrusted-provenance text; a Go verb routes them through
   the shipped `internal/termsafe.Escape` + `internal/idvalidate/idrender` sinks
   (as `internal/next/select.go:104` already does). A bash/jq script has no access
   to those sinks and would re-introduce the fl91 leak class.
2. **`internal/lint` render-ratchet (spec 120, `internal/lint/ratchet_render_test.go`).**
   New Go code is mechanically scanned for raw-id / raw-string renders; a script is
   invisible to the ratchet.
3. **Gate-before-mutate machinery (ADR-0041).** R6(b)'s "validate + hygiene BEFORE
   any file mutation; exit-non-zero ⇒ nothing written" is the preflight/commit
   discipline the binary already implements; a script's ordering is unverifiable.
4. **Transactional-write / atomicity contract (R6(b), AC7 T1/T2/T3).** A single
   lock spanning validate → uniqueness-check → atomic append needs real
   cross-process file-locking + `-race`-testable concurrency; jq/bash cannot express
   it safely. The mechanism is the shipped **`internal/journal` lockfile idiom**
   (spec 094): a DEDICATED `dispositions.lock` file (never the data file), acquired
   via the build-tagged `acquireFileLock` (unix `syscall.Flock` `LOCK_EX` /
   windows `O_EXCL`-retry) — so lock acquisition never opens/creates the data file
   before validation, and the manifest's temp+rename cannot invalidate the held lock.
5. **Unit-testability of the pinned AC3 numbers.** Q1–Q4's exact values and the
   exhaustive negative-fixture matrix are Go table tests, runnable in CI; a script's
   correctness would rest on hand jq one-liners with no regression net.

ADR-0043 records this: per-spec JSONL + per-panel files + coverage manifest + the
Go-verb append contract, rejecting Dolt, "both", **and** the script alternative.

## ADR Fitness

All five pre-existing ADRs remain the best architectural choice for this work —
each is *cited, not amended* (the spec verified every posture at the gate):

- **ADR-0037 (panel-gate enforced contract).** Sound. Disposition rows/manifests
  live inside §8's trust boundary (agent-writable, forgeable-by-content; telemetry,
  not tamper-proofing) and the gate's decision matrix consumes NONE of them
  (decision-inert, like `panel.json`'s `gate` field). No amendment; the gate code
  (`internal/panel/gate.go`) is byte-unchanged.
- **ADR-0023 (beads = single lifecycle authority).** Sound. The store is an analysis
  dataset, never a second state authority; `deferred` rows POINT at follow-up beads
  via `evidence_ref` and no lifecycle verb reads the store.
- **ADR-0040 (orchestration-layering ratchet).** Sound and directly honored:
  disposition JUDGMENT stays in the `/ms-panel-tally` skill; the binary contributes
  only mechanism (schema/validation/storage/query), matching the existing
  mechanized-vote / judgment-in-skill split.
- **ADR-0041 (gate-before-mutate).** Sound; the Go-verb decision (above) lets R6(b)'s
  append op honor it literally.
- **ADR-0042 (render/derivation provenance).** Sound; the Go-verb decision lets every
  Q1–Q5 render and validator-error render route through the shipped safe sinks and
  fall under the `internal/lint` ratchet.

**NEW — ADR-0043 (panel-disposition telemetry store).** Records OQ1's resolution
(owner decision 2026-07-20) + the panel-refined layout: append-only JSONL, one file
per panel at `.mindspec/specs/<spec>/reviews/<panel>/dispositions.jsonl`, each file
carrying its disposition rows plus one `record:"panel"` coverage manifest line;
the transactional Go-verb append contract (via the `internal/journal` dedicated-
lockfile idiom — a `dispositions.lock` file, build-tagged flock/O_EXCL, so the
lock never touches the data file before validation); rejected alternatives (Dolt
table — worktree/embedded-Dolt-sharing + upstream-schema; "both" — ingest-drift;
tracked script — no safe-render/ratchet/gate-before-mutate/atomicity). Domain(s): **workflow**
(so mirroring it into this plan's `adr_citations` cannot trip `adr-cite-irrelevant`).
Authored in Bead 1. No ADR is superseded; no divergence requiring a human stop.

## Testing Strategy

- **Unit (primary).** Go table tests in `internal/panel/` are the proof surface for
  every pinned falsifier. Fixtures live under `internal/panel/testdata/`.
  - Bead 1: `disposition_test.go` — validator accept/reject matrix + hygiene predicate.
  - Bead 2: `disposition_store_test.go` — append/idempotency/atomicity (`-race`,
    T1/T2/T3) + completeness-floor mutation test.
  - Bead 3: `disposition_query_test.go` — Q1–Q5 over a checked-in seed fixture.
  - Bead 4: `disposition_migrate_test.go` — round-trip fidelity vs the seed file.
- **Cross-check.** AC3's numbers are additionally re-derivable with the documented jq
  one-liners over the seed, independent of the Go implementation.
- **Fixtures = MIGRATED form (P4).** Bead 1's `valid/` fixtures AND Bead 3's
  `testdata/seed116/` are the MIGRATED/canonicalized records — `record`/`id`/
  `created_at`(RFC3339)/`backfilled` present, canonical gate keys, `S-tests` (not
  `Sonnet-tests`) — NOT the raw archive rows (which lack those fields and would fail
  the validator). These small migrated fixtures are either hand-authored JSONL or
  generated once by the Bead-4 migration transform and checked in; each is the
  self-contained input its bead needs, so Bead 3 computes the pinned Q-numbers
  WITHOUT waiting on Bead 4's live migration write. The RAW seed
  (`/Users/Max/replit/mindspec-panel-verdicts/spec-116/DISPOSITIONS.jsonl`) stays the
  migration SOURCE (Bead 4) and the jq cross-check input only. The sole real
  dependency edges are Bead 1's schema/validator API (consumed by 2/3/4) and Bead 2's
  `AppendRecord`/`check` (consumed by 4).
- **Integration gates (every bead).** `go build ./...` + **`GOOS=windows go build ./...`
  (cross-compile smoke — the release build cross-compiles windows; a tag-less flock
  regression must fail a PR gate, not surface at the next tag)** + `go vet ./...` +
  `gofmt -l` clean + `golangci-lint run` (incl. the `internal/lint` render ratchet)
  + `mindspec validate spec 117-panel-review-telemetry`.

## Wave structure & dependency edges

- **Wave 1:** Bead 1 (foundational schema + validator + ADR).
- **Wave 2 (parallel):** Bead 2 + Bead 3 — each `depends_on: [1]`, mutually
  independent (neither consumes the other's output).
- **Wave 3:** Bead 4 — `depends_on: [1, 2]`: it writes migrated records through
  Bead 2's `AppendRecord` and integration-tests against Bead 2's `check` leaf, so it
  cannot compile/verify on a Bead-1-only base.
- **Edges:** `1→2`, `2→4`, `1→3`. The graph is ACYCLIC; longest serial chain = 3
  (`1→2→4`, at the heuristic ceiling). Bead count = 4 (within 3–5).
- **Shared-file note (heuristic 2).** The beads DO NOT co-edit one cobra file:
  Bead 1 creates the `disposition` parent command in `cmd/mindspec/panel_disposition.go`;
  Beads 2/3 register their leaves from their OWN cmd files
  (`panel_disposition_store.go`, `panel_disposition_query.go`) via `init()`
  `AddCommand`, and Bead 4 touches no cmd file. `R_scope` across beads is low; the
  substantive logic lives in disjoint `internal/panel/disposition_*.go` files.

---

## Bead 1: Schema + validator + hygiene predicate + ADR-0043

Foundational. Defines the on-disk JSON contract for both record kinds and the
pure-function validator every other bead consumes, plus the store-choice ADR.
Satisfies **R2**, **R6(a) schema literal**, **R5 hygiene predicate**, and the
validator half of **AC3**, plus **AC5** and **AC6**.

**Steps**
1. Add `internal/panel/disposition.go`: Go structs for the two records —
   `DispositionRow{Record,ID,Spec,Gate,Panel,Reviewer,Model,Severity,Summary,
   ConvergentWith[],Disposition,EvidenceRef?,Note?,CreatedAt,Round?,Backfilled}`
   and `CoverageManifest{Record,Spec,Gate,Panel,Round,Slots[]{Slot,Model,Verdict},
   Backfilled}`; pin the closed enums as package vars — `record ∈ {"disposition",
   "panel"}`, `gate ∈ config.PanelGateKeys`, `disposition ∈ {confirmed-fixed,
   confirmed-deferred,confirmed-scope-trim,deferred,false-contamination,
   audited-refuted}`, slot `verdict ∈ {APPROVE,REQUEST_CHANGES,REJECT}`; pin the
   `genuine` / `false-positive` derived sets as exported constants (single source of
   truth for Bead 3).
2. Implement `Validate(record) error`: discriminate on `record`; enforce all
   required fields present + correctly typed (RFC-3339 `created_at`, array
   `convergent_with` of strings, boolean `backfilled`, integer `round`), reject
   wrong-type PRESENT optionals, and for a manifest validate every nested `slots[]`
   entry (`slot`/`model` string, `verdict` in the enum); reject a bad `record`
   discriminator and out-of-enum `gate`/`disposition`. Route every error string
   through `termsafe.Escape` / `idrender` so the render ratchet passes.
3. Implement `HygienePredicate(record) error` (R5): reject any string field
   containing a `/Users/`-prefixed or `/tmp/`-prefixed path token, over rows AND
   manifests.
4. Build the EXHAUSTIVE negative-fixture matrix under
   `internal/panel/testdata/disposition/{valid,invalid}/` — `valid/` = the 21
   **MIGRATED** disposition rows (canonicalized: `record:"disposition"`, minted `id`,
   `created_at:"2026-07-11T00:00:00Z"`, `backfilled:true`, canonical gate keys,
   `S-tests` not `Sonnet-tests`) + 8 **MIGRATED** coverage manifests (small tracked
   fixtures — hand-authored or generated once by the Bead-4 transform and checked in;
   NOT the raw archive rows, which lack `record`/`id`/`created_at`/`backfilled` and
   would fail the validator). `invalid/` = one file per fixture:
   out-of-enum `disposition:"fixed"` and uncanonical `gate:"gapfix"` (the AC3
   representative set); bad `record:"other"`; missing EACH required disposition field
   (`record,spec,gate,panel,reviewer,model,severity,summary,convergent_with,
   disposition,created_at,backfilled,id`) and EACH required manifest field
   (`record,spec,gate,panel,round,slots,backfilled`); wrong-type for EACH string
   field, `convergent_with` non-array + non-string element, `backfilled` non-bool,
   `round` non-int, `slots` non-array, nested `slots[]` missing/wrong-type
   `slot`/`model` and out-of-enum `verdict`; non-RFC-3339 `created_at`; wrong-type
   present optional (`evidence_ref`/`note`); a `/Users/…` and a `/tmp/…` hygiene
   fixture.
5. Add `cmd/mindspec/panel_disposition.go`: the `disposition` parent command
   (attached to `panelCmd`) + the `validate <file|glob>` leaf that runs
   `Validate`+`HygienePredicate` over each JSONL line and exits non-zero on the
   first failure with a termsafe-rendered message.
6. Finalize `.mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md` (drafted
   at plan time so the plan can cite it; Status: Accepted, Domain(s): workflow) —
   confirm it records the decision, the coverage-manifest + append contract, and the
   rejected Dolt / both / script options with the worktree/merge/public-repo
   evidence; adjust only if Bead 1's concrete API forces wording changes.

**Verification**
- [ ] `go test ./internal/panel/ -run 'TestDispositionValidate|TestHygiene'` passes:
      every `valid/` fixture (21 rows + 8 manifests) ACCEPTS; every `invalid/`
      fixture REJECTS (table-driven, one case per fixture file).
- [ ] `go run ./cmd/mindspec panel disposition validate internal/panel/testdata/disposition/valid/*.jsonl`
      exits 0; the same over any `invalid/` file exits non-zero.
- [ ] `go build ./...` AND `GOOS=windows go build ./...`, `go vet ./...`,
      `gofmt -l internal/panel cmd/mindspec` empty, `golangci-lint run`
      (incl. `internal/lint` render ratchet) clean.
- [ ] `.mindspec/adr/ADR-0043-panel-disposition-telemetry-store.md` exists, Status
      Accepted, Domain(s) includes `workflow`; `mindspec validate spec 117-panel-review-telemetry` passes.

**Acceptance Criteria**
- [ ] **AC3 (validator half)**: validator accepts all 21 seed rows + every manifest
      and rejects the full negative-fixture matrix (bad `record`, out-of-enum
      `disposition`/`gate`, missing/wrong-type any field, malformed nested `slots[]`,
      non-RFC-3339 `created_at`).
- [ ] **AC5**: the hygiene predicate finds zero `/Users/`/`/tmp/` tokens across the
      valid fixtures and REJECTS a `/Users/Max/replit/x` fixture (both record kinds).
- [ ] **AC6**: ADR-0043 records the per-panel-JSONL decision + coverage manifest +
      Go-verb append contract + rejected Dolt/both/script options; the plan's
      `adr_citations` reference it.

**Depends on**
None.

## Bead 2: Transactional append op + terminal manifest write + completeness check

The single canonical write path (R6(b)) and the coverage-manifest write (R6(a)),
plus the R1(b) completeness floor read from the durable manifest (AC2). Satisfies
**R1(b)**, **R6**, **AC2**, and **AC7**.

**Steps**
1. Add the build-tagged lock helper (reuse/replicate the `internal/journal` idiom —
   read `internal/journal/lock.go`, `lock_unix.go`, `lock_windows.go` first): a
   `withDispositionLock(panelDir, fn)` that acquires `acquireFileLock` on a DEDICATED
   `<panelDir>/dispositions.lock` file (unix `syscall.Flock` `LOCK_EX`, BLOCKING;
   windows `O_EXCL`-lockfile with bounded retry), `MkdirAll` the panel dir 0700
   first, `defer unlock()`. The lock is held on the lockfile, NEVER on
   `dispositions.jsonl` — so lock acquisition never opens/creates the data file
   before validation (closes the ADR-0041 gate-before-mutate hole), and any
   temp+rename on the data file cannot invalidate the held descriptor. Invariant
   (pinned): each op OPENS the lockfile per call (a fresh file description) and takes
   a BLOCKING `LOCK_EX` — flock binds to the open file description, so two separate
   descriptors of the same lockfile contend even within one process; the concurrency
   tests depend on this.
2. Add `internal/panel/disposition_store.go`: `AppendRecord(specDir, panel, record)`
   performing, inside `withDispositionLock`, as one indivisible unit: (a)
   `Validate`+`HygienePredicate` (Bead 1) of the record, BEFORE touching the data
   file; (b) read the CURRENT `dispositions.jsonl` (rows + manifest) and run the
   uniqueness/idempotency check — a disposition row keyed on its stable content-
   derived `id` (hash of `{spec,panel,reviewer,summary}`; `round` is NOT part of the
   key since migrated rows carry no `round`), a manifest keyed on `{spec,panel,round}`;
   (c) the mutation: a disposition row is written by atomic `O_APPEND`; the manifest
   is **no-op-if-exists** — if a `record:"panel"` line for `{spec,panel,round}` is
   already present, do nothing (its terminal content is deterministic, so no in-place
   "update" is ever needed), else append it. A validation/hygiene refusal exits
   before any mutation (data file byte-unchanged); a duplicate row `id` is a no-op;
   there is never a second `record:"panel"` line for a key.
3. Add `WriteTerminalManifest(specDir, panel, manifest)` (thin wrapper over
   `AppendRecord` with `record:"panel"`) — EVERY terminal panel writes exactly one
   manifest, including a finding-less all-APPROVE panel (zero rows).
4. Add `CheckCompleteness(specDir, panel)` (R1(b) floor): read the panel's
   `dispositions.jsonl` (its manifest line + its rows — the "reads only the manifest"
   phrasing is shorthand for "reads only this one durable file, never a raw verdict
   file"); for every manifest slot whose `verdict` is `REQUEST_CHANGES`/`REJECT`,
   require ≥1 disposition row naming that slot token in `reviewer` or
   `convergent_with[]`; on violation return an error naming the panel and uncovered
   slot.
5. Add `cmd/mindspec/panel_disposition_store.go`: `append` leaf
   (`--spec --panel --data @file|-`, dispatches to `AppendRecord`) and `check` leaf
   (`--spec [--panel]`, runs `CheckCompleteness`, exits non-zero naming panel+slot).
6. Tests in `disposition_store_test.go` — all built on INLINE testdata the test
   constructs itself in a temp dir (NOT the `--spec 116` live store, which Bead 4
   lands; Bead 2 must verify on a Bead-1+2 base only):
   - **Completeness (AC2)**: construct an INLINE bead-2 panel file (manifest with
     slot `S1` verdict `REQUEST_CHANGES` + its covering row) plus the two
     C1-canonicalized coverages (`panel-116-bead3a` slot `G1-codex`, `gapfix-panel`
     slot `S-tests`); assert `CheckCompleteness` PASSES on all; delete the `S1` row;
     assert it FAILS naming `panel-116-bead2` + slot `S1`; assert the check never
     opens a raw verdict file.
   - **Idempotency (AC7d)**: append the same row twice → one row; write terminal
     capture twice for one `{spec,panel,round}` → one manifest (no-op-if-exists).
   - **Gate-before-mutate (AC7c)**: append a schema-invalid then a hygiene-violating
     record → non-zero exit, target data file byte-identical (checksum before/after)
     AND no `dispositions.jsonl` created if none existed (lock is on the lockfile).
   - **Finding-less panel (AC7b)**: a zero-row all-APPROVE panel yields a file with
     exactly one `record:"panel"` line; its slots count toward Q4.
   - **Concurrency (AC7e), `-race`**: **T1** N goroutines append the same `id` → 1
     row; **T2** N goroutines write the same `{spec,panel,round}` manifest → 1
     manifest; **T3** N goroutines append N DISTINCT records → all N persist, every
     line valid JSON, no interleave/corruption. **T4 (cross-PROCESS, M4)**: spawn
     two `go run ./cmd/mindspec panel disposition append …` SUBPROCESSES racing the
     same lockfile (distinct records → both persist; same `id` → one) — proving the
     lock serializes across processes, not just goroutines (flock binds to the open
     file description).

**Verification**
- [ ] `go test ./internal/panel/ -run 'TestCompleteness|TestAppend|TestManifest' -race` passes.
- [ ] `go test ./internal/panel/ -run TestAppendConcurrent -race` passes (T1/T2/T3/T4).
- [ ] `go run ./cmd/mindspec panel disposition check --spec <tmp-inline-store>` (an
      INLINE fixture the test writes, NOT `--spec 116`) exits 0; exits non-zero naming
      `panel-116-bead2` + `S1` after the S1-row deletion.
- [ ] `go build ./...` AND `GOOS=windows go build ./...` (the lock is build-tagged;
      the windows cross-compile MUST pass), `go vet ./...`, `golangci-lint run` clean.

**Acceptance Criteria**
- [ ] **AC2**: completeness check passes on the inline seed-shaped fixture (incl. the
      two canonicalized coverages) reading only `dispositions.jsonl`; fails naming
      `panel-116-bead2` + `S1` after deleting the covering row.
- [ ] **AC7**: (a) each panel file has exactly one manifest reproducing its slot count
      (over inline fixtures; the migrated-store slot-count fidelity is Bead 4's AC4);
      (b) finding-less panel still writes its manifest; (c) invalid/hygiene refusal
      leaves the data file byte-unchanged (gate-before-mutate; the lockfile, not the
      data file, is opened); (d) row idempotent on `id`, manifest idempotent
      (no-op-if-exists) on `{spec,panel,round}`; (e) T1/T2/T3/T4 concurrency proofs.

**Depends on**
Bead 1.

## Bead 3: Q1–Q5 query surface

The read side (R3) computing AC3's pinned numbers with `0/N` zero-count rendering.
Satisfies **R3** and the query half of **AC3**.

**Steps**
1. Add `internal/panel/disposition_query.go`: load rows+manifests via glob
   (`.mindspec/specs/*/reviews/*/dispositions.jsonl`), then compute — **Q1**
   per-model genuine/total; **Q2** per-model false-positive/total (using Bead 1's
   pinned `genuine`/`false-positive` sets); **Q3** convergence (rows with non-empty
   `convergent_with` / total + the row list); **Q4** per-gate genuine-per-slot where
   the denominator is summed from the coverage-manifest `slots` rosters and gates use
   canonical keys; **Q5** finding listing filterable on gate/severity/disposition.
   Q1/Q2 render `0/N` explicitly for a zero-count model (never drop / divide-by-empty).
   Route all rendered text through `termsafe`/`idrender`.
2. Add `cmd/mindspec/panel_disposition_query.go`: `query --metric Q1..Q5
   [--spec --gate --severity --disposition]` leaf.
3. Check in `internal/panel/testdata/seed116/` — the **MIGRATED** records (21
   canonicalized disposition rows with `record`/`id`/`created_at`(RFC3339)/
   `backfilled` present, canonical gate keys, `S-tests` not `Sonnet-tests`) + the 8
   coverage manifests (slot counts 9/9/8/8/8/8/12/4) as a self-contained fixture — a
   small tracked JSONL set (hand-authored or generated once by the Bead-4 transform),
   NOT the raw archive rows. The pinned numbers are then computable WITHOUT Bead 4.
4. Tests in `disposition_query_test.go` asserting EXACTLY: Q1 fable 3/5, opus 4/6,
   sonnet 6/7, gpt-5.6-sol 2/3; Q2 fable 0/5, opus 1/6, sonnet 1/7, gpt-5.6-sol 1/3;
   Q3 convergent rows = 4 of 21 (G1-codex, O1, S1, S-tests); Q4 spec_approve 5/9,
   plan_approve 3/9, bead 3/32, final_review 2/12, adhoc 2/4. Note the seed already
   exercises a real zero-NUMERATOR (`fable` false-positive 0/5); the `0/N` render test
   ADDITIONALLY targets the divide-by-EMPTY guard — a synthetic model with ZERO rows
   (N=0) must render `0/0` (or the pinned zero form) rather than panic or drop the
   model, distinct from fable's 0/5.

**Verification**
- [ ] `go test ./internal/panel/ -run TestQueryMetrics` passes with the exact pinned
      Q1–Q4 values above and the `0/N` assertion.
- [ ] `go run ./cmd/mindspec panel disposition query --metric Q4 --dir internal/panel/testdata/seed116`
      (the migrated fixture, NOT the live `--spec 116` store) prints the pinned
      per-gate pairs.
- [ ] jq cross-check documented & green over the RAW archive seed (model counts are
      migration-invariant): `jq -r .model /Users/Max/replit/mindspec-panel-verdicts/spec-116/DISPOSITIONS.jsonl | sort | uniq -c`
      → 5 fable/6 opus/7 sonnet/3 gpt-5.6-sol.
- [ ] `go build ./...` AND `GOOS=windows go build ./...`, `go vet ./...`,
      `golangci-lint run` clean.

**Acceptance Criteria**
- [ ] **AC3 (query half)**: Q1–Q4 over the seed fixture return EXACTLY the pinned
      numbers (Q1 3/5·4/6·6/7·2/3; Q2 0/5·1/6·1/7·1/3; Q3 4/21; Q4
      5/9·3/9·3/32·2/12·2/4); Q1/Q2 render `0/N` for zero-count models.

**Depends on**
Bead 1.

## Bead 4: Seed migration + `/ms-panel-tally` capture step

Lands the spec-116 seed as the live store (R4) and wires the mandated capture step
into BOTH skill surfaces (R1). Satisfies **R1 (skill)**, **R4**, **AC1**, **AC4**,
and the spec-level **AC-global**.

**Steps**
1. Add `internal/panel/disposition_migrate.go`: read
   `/Users/Max/replit/mindspec-panel-verdicts/spec-116/DISPOSITIONS.jsonl`, split the
   21 rows into per-panel files under
   `.mindspec/specs/116-panel-message-escaping/reviews/<panel>/dispositions.jsonl`
   (panels `panel-116-spec/plan/bead1/bead2/bead3a/bead3b`, `final-116`,
   `gapfix-panel`); map `gate` per the R4 table (`spec→spec_approve, plan→plan_approve,
   bead→bead, final→final_review, gapfix→adhoc`); carry `note/summary/severity/model`
   verbatim; set `record:"disposition"`, `backfilled:true`, backfill
   `created_at:"2026-07-11T00:00:00Z"` (full RFC 3339 — a date-only value would fail
   the validator), and mint the stable `id` (hash of `{spec,panel,reviewer,summary}`;
   migrated ROWS carry NO `round` field — only manifests get `round`); canonicalize
   skewed slot names `G-codex→G1-codex` (panel-116-bead3a) and `Sonnet-tests→S-tests`
   (gapfix-panel) in `reviewer`/`convergent_with`, leaving already-matching names
   verbatim.
2. Synthesize the 8 coverage manifests from the archive's per-panel verdict files
   (`verdict-<slot>.json`) — one `slots[]` entry per file (token, model, terminal
   `verdict`), `round:1`, `backfilled:true`, slot counts 9/9/8/8/8/8/12/4. Write every
   record through Bead 2's `AppendRecord` so validation + hygiene + idempotency apply.
3. Commit the resulting 8 `dispositions.jsonl` files as the landed seed store.
4. Edit BOTH `plugins/mindspec/skills/ms-panel-tally/SKILL.md` AND
   `.claude/skills/ms-panel-tally/SKILL.md` (kept byte-identical): add the mandated
   capture step — as the tally authority resolves each finding, append one
   `record:"disposition"` row per distinct finding via `mindspec panel disposition
   append`; at terminal state write the `record:"panel"` coverage manifest and run
   `mindspec panel disposition check`; state the slot-identity contract and the
   R1(b) floor.
5. Add `disposition_migrate_test.go`: field-by-field round-trip vs the seed file.

**Verification**
- [ ] `go test ./internal/panel/ -run TestSeedMigration` passes: exactly 21 rows with
      `spec=116` across the 8 files (+ one manifest each); `summary/note/severity/
      model/disposition/spec/panel` byte-identical to the source seed (P6 — including
      `disposition`, the central metric field, plus `spec`/`panel`); `gate` mapped
      only per the R4 table; `reviewer`/`convergent_with` byte-identical EXCEPT the
      two documented canonicalizations; every row+manifest `backfilled:true`; rows
      carry NO `round`; every manifest `round:1` + correct slot count.
- [ ] AC1 chain RED→GREEN:
      `grep -q 'disposition' plugins/mindspec/skills/ms-panel-tally/SKILL.md && grep -q 'disposition' .claude/skills/ms-panel-tally/SKILL.md && diff -q plugins/mindspec/skills/ms-panel-tally/SKILL.md .claude/skills/ms-panel-tally/SKILL.md`
      exits 0.
- [ ] AC1 SUBSTANCE (P7 — beyond the bare grep): a test/grep asserts the capture step
      text actually mandates ALL of — the `mindspec panel disposition append` call
      per distinct finding, the terminal `record:"panel"` manifest write, the
      `mindspec panel disposition check` completeness run, and the slot-identity +
      R1(b) floor contract (e.g. `grep -Eq 'panel disposition append' … && grep -Eq
      'record.*panel|coverage manifest' … && grep -Eq 'panel disposition check' … &&
      grep -Eq 'slot|floor' …` on BOTH surfaces).
- [ ] `go run ./cmd/mindspec panel disposition check --spec 116-panel-message-escaping`
      exits 0 over the LANDED store (integration with Bead 2's `check` leaf — the
      landed-store check lives here, in the bead that owns the migration).
- [ ] `go build ./...` AND `GOOS=windows go build ./...`, `go vet ./...`,
      `golangci-lint run` clean; `mindspec validate spec 117-panel-review-telemetry` passes.

**Acceptance Criteria**
- [ ] **AC1**: both `/ms-panel-tally` surfaces contain the capture step and remain
      byte-identical (grep+diff chain green), AND the step's SUBSTANCE mandates the
      append-verb / terminal manifest / `check` / slot-identity+floor contract (P7).
- [ ] **AC4**: the 8 per-panel files hold exactly 21 `spec=116` rows + one manifest
      each; round-trip byte-identical (incl. `disposition`, `spec`, `panel`) except
      the two C1 canonicalizations; every migrated record `backfilled:true`, rows
      carry no `round`, each manifest `round:1` + correct slot count.
- [ ] **AC7(a) slot-count fidelity over the MIGRATED store**: each landed panel file's
      manifest `slots` roster reproduces the archive's verdict-file counts
      (9/9/8/8/8/8/12/4) — the migrated-store half of AC7(a) (Bead 2 proves it over
      inline fixtures).
- [ ] **AC-global**: `mindspec validate spec 117-panel-review-telemetry` passes;
      `go build ./...` (+ `GOOS=windows go build ./...`) and the touched packages'
      tests pass.

**Depends on**
Bead 1, Bead 2.

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| AC1 (skill mandates capture, both surfaces byte-identical) | Bead 4 — grep/diff chain + skill edits |
| AC2 (completeness floor from durable manifest; mutation fails naming panel+slot) | Bead 2 — `TestCompleteness` + `check` leaf |
| AC3 (validator accepts 21 rows/manifests + rejects full negative matrix) | Bead 1 — `TestDispositionValidate`/`TestHygiene` |
| AC3 (Q1–Q4 pinned numbers + `0/N` rendering) | Bead 3 — `TestQueryMetrics` + jq cross-check |
| AC4 (migration fidelity: 21 rows, canonicalization, backfilled, round:1) | Bead 4 — `TestSeedMigration` round-trip |
| AC5 (hygiene predicate: zero `/Users//tmp` tokens; `/Users/…` fixture rejected) | Bead 1 — `TestHygiene` (predicate) **+ Bead 2 — AC7c (predicate enforced pre-write by the append op)** |
| AC6 (ADR-0043 records decision + rejected Dolt/both/script) | Bead 1 — ADR authored; `mindspec validate` |
| AC7 (finding-less manifest; gate-before-mutate; idempotency; T1–T4) | Bead 2 — `TestAppend*`/`TestAppendConcurrent -race` (op behavior over inline fixtures) **+ Bead 4 — AC7(a) slot-count fidelity over the MIGRATED store** |
| AC-global (`mindspec validate` + `go build ./...` + `GOOS=windows` + touched tests) | Bead 4 — spec validate + full build/test |
| R1 (capture at tally authority; slot-identity; floor) | Bead 4 (skill capture, both surfaces) **+** Bead 2 (floor check) |
| R2 (row + manifest schema + validator) | Bead 1 |
| R3 (Q1–Q5 query surface) | Bead 3 |
| R4 (seed migration) | Bead 4 |
| R5 (path hygiene, public-repo posture) | Bead 1 (predicate) **+** Bead 2 (enforced pre-write) |
| R6 (coverage manifest + transactional append op) | Bead 2 (op + manifest + lockfile) **+** Bead 1 (schema) |
