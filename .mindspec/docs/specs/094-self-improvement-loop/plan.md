---
adr_citations:
    - id: ADR-0023
    - id: ADR-0035
    - id: ADR-0038
approved_at: "2026-06-12T23:06:18Z"
approved_by: user
bead_ids:
    - mindspec-cdk8.1
    - mindspec-cdk8.2
    - mindspec-cdk8.3
    - mindspec-cdk8.4
spec_id: 094-self-improvement-loop
status: Approved
version: "1"
---
# Plan: 094-self-improvement-loop

This plan decomposes spec 094 (Reqs 1-9, HC-1..HC-8) into **four** beads.
The spec's §Proposed bead decomposition pinned three, with the
dependency chain redaction FIRST, then capture, then report
(Bead 1 → Bead 2 → Bead 3). The plan-approve panel (r4-decomposition +
codex-decomposition, cross-confirmed) re-cut the report bead: Req 6 (the
feedback-remote config contract) has NO functional dependency on the
redaction lib or the journal in v1 (v1 does no push — `report` writes
local-only regardless of config), so it is peeled into a new **Bead 4**
that runs in PARALLEL (Depends on: None). The keystone redaction-first
chain is preserved unchanged: **Bead 1 → Bead 2 → Bead 3**, with Bead 4
off to the side. Final chain: B1←none, B2←1, B3←1,2, B4←none. Bead
descriptions cite requirement, AC, and HC numbers from the spec rather
than inlining their text (per the mindspec-lawq rule: bead payloads stay
lean; full text lives in `spec.md`). All file:line anchors below carry
the spec's own draft-time line-drift caveat — they are starting
coordinates re-verified at bead-claim time, not pins.

The single load-bearing invariant the dependency chain encodes is
HC-1: **privacy is the gating requirement, not a feature.** Bead 1
(the redaction library + its adversarial golden-corpus CI gate) MUST
merge before any code that could emit collected data (Beads 2, 3),
and the golden-corpus test is wired as a CI gate, not an advisory.

## ADR Fitness

The spec's impacted domains are **core** (`internal/redact`,
`internal/journal`, the fingerprint/version stamping, the
feedback-remote config contract in `internal/config`), **execution**
(`cmd/mindspec/root.go`'s `PersistentPostRunE` self-emit, the new
`cmd/mindspec/report.go`), and **workflow** (`internal/next/guard.go`,
`internal/complete/complete.go`, `internal/validate/divergence.go` —
READ-ONLY: their recovery/error templates are the source of the
adversarial fixtures, no behavior changes). Frontmatter cites two
Accepted ADRs that together cover all three impacted domains:

- **ADR-0035** (agent error contract — Accepted, domains: workflow,
  execution, core). Covers ALL three impacted domains. Load-bearing
  here: the recovery/error strings the redaction golden-corpus must
  neutralise (`guard.FormatFailure`/`NewFailure` recovery-line
  convention — `next/guard.go` ClaimFailure + worktree recipe,
  `complete.go` adr-divergence fold) are produced BY this ADR's
  convention; that convention is exactly why those strings are
  templates interpolated with user data (paths, slugs, bead ids,
  branch names, domain names). This spec adds NO new error contract —
  it builds adversarial fixtures FROM the existing one. Sound; adhere.
- **ADR-0023** (beads as single state authority — Accepted, domains:
  workflow, git, state). Covers the workflow domain. REINFORCED, not
  bent: the friction journal AND the consolidated friction reports are
  **review/diagnostic artifacts** in a dedicated, non-synced local
  store (precedent: spec-093's `panel.json`), isolated from the beads
  DB (`.beads/issues.jsonl`) and from the `bd`/`dolt push` egress path
  (HC-3 / Req 4). `mindspec report` writes NOTHING to the beads
  tracker; bd statuses remain the single workflow-state authority and
  this spec adds no new bead-tracked state. Sound; adhere — Bead 3's
  store-isolation egress proof is the executable guarantee.

**ADR-0027** (mindspec-is-OTEL-only — Accepted, domains: observability,
telemetry, recording, extraction) is cited by the spec's ADR
Touchpoints and is load-bearing for the design narrative — this spec
deliberately does NOT reuse the OTEL emitter for capture (the trace
emitter defaults to `noopTracer{}` and `trace.Init` fires only under
`MINDSPEC_TRACE`/`--trace`, `cmd/mindspec/root.go:53-56`), so the
always-on redacted journal is a net-new, separate sink. It is
intentionally NOT in the frontmatter `adr_citations`: its declared
domains (observability/telemetry/recording/extraction) do not
intersect this spec's impacted domains, so a frontmatter citation
would (correctly) trip the `adr-cite-irrelevant` semantic gate. The
"don't reuse OTEL" decision is recorded in the proposed new ADR and in
Bead 2's contrast note instead. No ADR-0027 behavior changes.

**Proposed new ADR — owner-local friction self-improvement loop** (spec
ADR Touchpoints): Bead 3 authors it (domains: core, execution,
workflow) recording (a) the privacy-first redaction architecture
(structured-enum-fields-first, tainted-by-default, the scrub-category
list, entropy catch-all, `%w`-chain rule, human-review-as-backstop);
(b) the success-path self-emit capture decision (NOT a PostToolUse
hook, NOT OTEL per ADR-0027; failed/`os.Exit` capture deferred); (c)
the escape-hatch-centered signal taxonomy; (d) the fingerprint +
version loop-closing scheme (no error-class, no override reason);
(e) the capability-based fail-closed identity model; (f) the
untrusted-corpus stance; and (g) the v1 owner-local scope with the
deferred channels. **Number resolution**: NOT 0035/0036/0037 (taken).
As of plan-fill this branch tops at `ADR-0037-panel-gate-enforced-
contract.md`, so the next free integer is expected to be **0038** — but
the ADR lane may carry concurrent in-flight claims, so Bead 3 re-checks
`.mindspec/docs/adr/` on the current branch AND main at bead-claim
time, claims the next free integer, hand-creates the file following the
existing ADR format (do NOT use `mindspec adr create` — two live bugs:
mindspec-8lzq worktree mis-write, mindspec-bn3u colliding IDs, hit by
specs 091/092/093), and adds the landed ID to this frontmatter
post-creation. Per the §Open Questions note the spec leaves
ADR-vs-doc-section home for the panel to confirm at gate time; the
draft position taken here is a new ADR.

No accepted ADR is unfit for this work; no superseding ADR is proposed.

## Design Question Resolutions

Spec §Design Questions — draft positions are BINDING for planning
unless the plan-approve panel explicitly overrides (the spec's own
preamble). The location/perms (HC-8) and the capture/fingerprint
mechanisms (Req 2/Req 3) arrived already RESOLVED into the
requirements; the remaining plan-phase items are settled here against
the spec's stated draft positions:

1. **Journal/store format + retention (DQ1)**: adopt the draft
   position — append-only JSONL under the dedicated `0600` non-synced
   state dir (HC-8); the journal is one appended file (not per-session)
   with within-session fingerprint collapse (Req 8); a retention/
   rotation cap is DEFERRED to the follow-on (a stale redacted `0600`
   entry is low risk given fail-closed redaction). Encoded in Bead 2.
2. **Exact success-path event enum (DQ2)**: adopt the draft position,
   CORRECTED against real code (r2/r6/codex-feasibility). The override
   flags `--override-adr`/`--allow-doc-skew`/`--supersede-adr` are NOT
   root persistent flags — they are **leaf-local** flags registered on
   `completeCmd` (`complete.go:145-147`), `approveImplCmd`
   (`approve.go:56-58`), and `implApproveCmd` (`impl.go:36-38`); the only
   root persistent flag is `--trace`. The audit therefore targets the
   **leaf success commands** — `complete`, `impl approve`, the hidden
   `approve impl`, and a completed `repair phase` (`repair.go:32-55`;
   the command is `mindspec repair phase <spec-id>`, parent+child) — and
   the hook reads each via `cmd.Flags().Changed(...)` on the LEAF `cmd`
   (in `PersistentPostRunE` `cmd` is the leaf), dispatching on
   `cmd.Name()`/`cmd.CommandPath()` because the SAME flag name lives on
   three commands at two scopes (bead-level `complete` vs spec-epic
   `approve`/`impl`). **`MINDSPEC_ALLOW_MAIN` is REMOVED from v1
   success-path capture** (see DQ6 below). The audit + the bound set are
   recorded as Bead 2 evidence. Encoded in Bead 2.
3. **Friction-report write ceremony (DQ3)**: adopt the draft position —
   silent local write is acceptable (nothing egresses; the store never
   leaves the machine, HC-3; HC-6 already forces journal-only in CI).
   No interactive confirm in v1. Encoded in Bead 3.
4. **`resolved_in_vX` storage + regression computation (DQ4)**: adopt
   the draft position, SHARPENED for version semantics (r6/r2/codex-
   feasibility — the loop-closing landmine). `mindspec --version`
   (`root.go:46`) is a DECORATED string (`version + " (" + commit + ") "
   + date`) whose commit hash is exactly the high-entropy run Req 1's
   entropy catch-all will SCRUB, and the bare semver `version` var
   defaults to `"dev"` (`root.go:35`) on every non-release/local/test
   build — the builds the motivating autopilot evidence came from. So a
   naive `version ≥ X` comparison on the decorated string is UNDEFINED
   exactly where the loop must work. RESOLUTION: capture and stamp the
   **bare `version` package var (`root.go:35`)**, NOT the cobra
   `--version` string; the §API Contract pins a normalized-version helper
   (`version.Parse`/`Compare`) that parses semver and defines the
   **non-semver/"dev" policy** (a `dev`/unparseable running version is
   treated as **unbounded-newest** for regression: an incoming `dev`
   report against a `resolved_in_vX` record classifies REGRESSION, never
   stale — fail toward surfacing, not suppressing — and a stored `dev`
   resolved-version is non-comparable, so any later concrete version is a
   regression). Store `resolved_in_vX` on the friction-report record
   keyed by the normalized event identity + fingerprint (DQ5); compute
   regression-vs-stale via the helper at `report list` time (parsed
   version ≥ X → regression; < X → stale; `dev` → regression). A test
   injects a fake semver since dev builds cannot exercise the ordering.
   Encoded in Bead 1 (version helper + fingerprint helper) + Bead 2
   (stamping) + Bead 3 (regression/stale).
5. **Fingerprint stability across versions (DQ5)**: adopt the draft
   position — the fingerprint inputs are mindspec's own closed-set
   tokens (`command + which-escape-hatch [+ subcommand]`, no
   error-class), so drift is bounded to renamed commands/flags; a
   normalization/version map is DEFERRED to the follow-on if observed.
   The fingerprint is keyed by the **normalized EVENT IDENTITY** (the
   canonical tuple `{command, escape-hatch, subcommand}`) PLUS its hash,
   and BOTH are persisted (the tuple, not the opaque hash alone) so a
   hash collision cannot silently alias two distinct events and poison
   dedup counts + `resolved_in_vX` (codex-completeness collision risk).
   Encoded in Bead 1 (fingerprint helper) + Bead 2 (stamping).
6. **`MINDSPEC_ALLOW_MAIN` excluded from v1 capture (DQ6, NEW)**: the
   env var is consumed in `internal/hook/dispatch.go:51` (the `mindspec
   hook` git-pre-commit path), NOT in `root.go`, and its motivating use
   is `MINDSPEC_ALLOW_MAIN=1 git commit ...` — a raw-git bypass that never
   runs a capturable `mindspec` leaf command, so `PersistentPostRunE`
   cannot attribute it. Worse, env-present ≠ flag-was-set: a naive
   `os.Getenv` check in the always-on hook fires a FALSE friction event on
   every successful command in any shell that exported the var (ambient
   contamination). There is no ambient-safe in-`mindspec` detection in v1,
   so `MINDSPEC_ALLOW_MAIN` is **REMOVED from the v1 success-path capture
   set** and explicitly scoped to the deferred failure/hook-path capture
   bead (§Non-Goals; a future enabler that wraps the `hook`/`os.Exit`
   paths). Encoded in Bead 2 (audit/binding) + reflected in §Non-Goals.

## Storage Contract

Privacy-first storage cannot leave path/scope/schema/concurrency to
bead-time guesswork (codex-completeness/r6). These are PINNED now:

- **State-dir path + resolution**: the dedicated, non-synced, non-committed
  store roots at **`os.UserConfigDir()/mindspec/`** (honoring XDG on
  Linux: `$XDG_CONFIG_HOME/mindspec/`, falling back to
  `os.UserHomeDir()/.config/mindspec/` then `~/.mindspec/` if
  `UserConfigDir` is unavailable). It is NEVER under any project/git tree
  and NEVER swept by `bd`/`dolt` (HC-3/HC-8). NOTE: spec-093's
  `review/<slug>/panel.json` is a JSONL *layout* precedent only — it lives
  UNDER the project tree (synced), so it is NOT a non-synced-location
  precedent; this store is net-new.
- **Scoping**: the store is **GLOBAL per machine-user**, NOT
  per-project — friction signals are owner-local and span repos. Filenames
  are fixed: the journal is one appended file `journal.jsonl`; the
  consolidated reports are `reports.jsonl`. Both created `0600` (HC-8).
- **0600 append sink (the `internal/ndjson` primitive)**: the
  append+0600 primitive `internal/ndjson` (`writer.go` — `O_APPEND`,
  `FileMode: 0o600`, used by `internal/recording/markers.go`) exemplifies is
  what Bead 2 uses: a SINGLE `O_APPEND` `write(2)` of one redacted JSONL
  record per event (the journal opens `O_CREATE|O_WRONLY|O_APPEND` 0600,
  re-asserts the mode via the fd, writes one line, closes — it does NOT
  re-derive a mutex-guarded read-modify-rewrite). An in-process `sync.Mutex`
  serializes only to keep two goroutines' lines from interleaving and to
  make the per-session storm counter race-free. For **cross-process**
  concurrent append (two `mindspec` processes both reaching
  `PersistentPostRunE`), the append is line-atomic via the `O_APPEND` single
  `write(2)` of one redacted JSONL record below `PIPE_BUF`, so there is NO
  cross-process lost-update and NO file lock is needed; consolidation
  (`report`, Bead 3) tolerates interleaved/duplicate lines by collapsing on
  the normalized event identity + fingerprint.
- **Journal record schema** (`journal.jsonl`, enum-only — no free text):
  `{ "v": <schema-int>, "ts": <rfc3339>, "argv0": "<basename>",
  "command": "<leaf command path token>",
  "escape_hatch": "<closed-set enum: override-adr|allow-doc-skew|
  supersede-adr|repair-phase>", "subcommand": "<optional enum token>",
  "fingerprint": "<hash>", "identity": {command, escape_hatch,
  subcommand}, "count": <int>, "version": "<bare semver or 'dev'>" }`.
  NEVER a raw command string, NEVER `argv[0]`'s full path, NEVER a flag
  VALUE (M3/M4).
- **Friction-report record schema** (`reports.jsonl`): `{ "v":
  <schema-int>, "fingerprint": "<hash>", "identity": {command,
  escape_hatch, subcommand}, "command": "<token>", "escape_hatch":
  "<enum>", "count": <int>, "first_version": "<semver|dev>",
  "last_version": "<semver|dev>", "resolved_in_version":
  "<semver|empty>", "status": "open|regression|stale" }`.
  `mindspec report` derives `first_version`/`last_version` by OCCURRENCE
  ORDER (earliest/latest event by `ts`, append-order tiebreak), NOT by
  semver min/max, so an out-of-order/downgrade stream reports the true
  first/last seen with paired timestamps. The status model is
  `{open, regression, stale}` (a resolved-with-no-recurrence report is
  `stale`; there is no separate `resolved` token). `resolved_in_version`
  is keyed by fingerprint, which is `H(identity)` — a strong hash over the
  FULL normalized identity, so fingerprint-keying IS identity-keying by
  construction (DQ5 collision-safe); the `identity` tuple is persisted as
  a display/audit field.
- **Retention**: NONE in v1 — DECIDED, not deferred-by-omission. The
  append-only `0600` journal grows unbounded across sessions (Req 8's cap
  is per-fingerprint-PER-SESSION); a redacted + fail-closed + `0600`
  stale entry is low at-rest risk. Rotation/compaction is a follow-on.

## API Contract

The cross-bead seams (Bead 1's `internal/redact`/version helper consumed
by Beads 2/3; Bead 2's `internal/journal` read API consumed by Bead 3)
are PINNED now so later beads integrate against a settled signature, not
a guess (r2/r6/codex-feasibility/codex-completeness). Exact names may be
adjusted at impl for Go idiom, but the SHAPES — especially the
fail-closed return — are binding:

- **`internal/redact` (Bead 1)**:
  - `func Scrub(s string) (clean string, ok bool)` — full tainted-string
    scrub; `ok == false` signals the field CANNOT be confidently
    classified and MUST be DROPPED by the caller (HC-7 fail-closed; the
    raw value is NEVER returned in `clean` when `ok` is false). This `ok`
    drop-signal is the load-bearing cross-bead contract Beads 2/3 build
    their HC-7 behavior on.
  - `func RedactEvent(ev Event) (RedactedEvent, bool)` — per-entry scrub
    over the structured enum fields; `false` ⇒ drop the whole entry.
  - `func Fingerprint(identity Identity) string` where `Identity =
    struct{ Command, EscapeHatch, Subcommand string }` (closed-set enum
    tokens only; deterministic; reason-invariant; no error-class, no user
    value). The canonical normalized tuple is `Identity`; both it and the
    returned hash are persisted (DQ5).
- **`internal/version` helper (Bead 1)**: `func Current() string` returns
  the **bare `version` package var** (`root.go:35`), NOT the decorated
  cobra string; `func Parse(s string) (Semver, bool)` (`ok == false` for
  `"dev"`/unparseable); `func Compare(a, b string) (int, bool)`
  implementing the DQ4 `dev`→unbounded-newest policy.
- **`internal/journal` (Bead 2; read API consumed by Bead 3)**:
  - `func AppendSuccessEvent(ev Event) error` — scrubs at WRITE time via
    `redact`, stamps the bare version + an rfc3339 `ts`, and APPENDS exactly
    ONE redacted event line to the APPEND-ONLY `journal.jsonl` in a single
    atomic `O_APPEND` write (no in-file collapse, no read-modify-rewrite —
    each line preserves its own version so Bead 3 can derive
    first/last-seen). The per-fingerprint-PER-SESSION storm cap (Req 8) is
    enforced IN PROCESS (drop appends past the cap for a fingerprint within
    one invocation), not by a stored count. **Fail-closed** (a
    non-classifiable field — or a `MINDSPEC_STATE_DIR` that resolves inside
    a git/project tree — is dropped, never written raw).
  - `func ListReports() ([]Record, error)` (alias `ReadEvents`) returns the
    raw append-only journal event Records Bead 3 collapses into
    `reports.jsonl`; `func MarkResolved(fp string, ver string) error` is the
    Bead-3 resolve SEAM — it operates on the reports layer, NEVER mutating
    the append-only journal (a minimal stub here Bead 3 completes).
    The COUNT-collapse + first/last/`resolved_in_version` live on Bead 3's
    `reports.jsonl`, per the §Storage Contract's 2-file design.
- **Best-effort / non-fatal contract**: `PersistentPostRunE` journaling
  is **BEST-EFFORT and NON-FATAL to command success** — an
  `AppendSuccessEvent` error (or a redaction drop) is swallowed (logged
  at most to the trace sink, never with raw text) and NEVER converts an
  already-successful, side-effecting command (`complete`, `impl approve`)
  into a post-mutation failure. The hook never returns the journal error.
  Fail-closed governs DATA EMISSION (drop the datum), not command exit.

## Testing Strategy

- **Unit tests are the primary gate**: every behavior change lands with
  unit tests asserting the exact spec AC for that requirement.
  `go build ./... && go test -short ./...` green on every commit (HC-5),
  no test skipped vs the branch base.
- **The adversarial golden-corpus gate (HC-1, the keystone)**: Bead 1
  ships `internal/redact` golden-corpus fixtures derived from REAL
  mindspec error/recovery strings — `next/guard.go` ClaimFailure
  (`:176`) + the `git -C <SpecWorktreePath> worktree add` recovery
  recipe (`:180`/`:210`), the `divergence.go:186` adr-divergence-unowned
  message as folded by `joinResultErrorMessages`/`adrDivergenceFailure`
  (`complete.go:340`/`:612`/`:650`), and a `%w`-wrapped Dolt-1105-style
  chain carrying a bead description — asserting ZERO leakage of absolute
  OR relative paths, `*.go` tokens, spec slugs, branch names, bead ids,
  OWNERSHIP domain names, bead descriptions, or high-entropy hex/base64
  runs. `go test ./internal/redact/...` is wired as a CI gate that
  FAILS the build on any leakage — not an advisory.
- **Allowlist/value boundary tests (HC-2/M4)**: a fixture with ONLY
  closed-set enum fields redacts to itself unchanged; a fixture with any
  free text / flag VALUE / `argv[0]` invocation path passes the full
  scrub. Asserts the allowlist NEVER holds a value copied from
  argv/env/user input.
- **Write-time redaction + fail-closed (HC-7) tests**: Bead 2 asserts
  redaction runs at journal-WRITE time (not draft time); a
  redaction-failure fixture yields NO on-disk entry for that field and
  NEVER the raw string (dropped, not fallen-back); a grep of the on-disk
  journal for a planted home-dir path / secret / override-reason returns
  absent.
- **At-rest + isolation tests (HC-8/HC-3)**: `0600` perms assertion on
  the journal + friction-report store; the store path is non-project,
  non-`bd`/`dolt`. Bead 3's egress proof runs `report`, then asserts the
  report's fingerprint is ABSENT from a **provably-implementable surface
  set** that covers everything `bd dolt push` would send: `grep
  .beads/issues.jsonl` (the implementable floor) AND the dolt
  working-set / tracked tables AND `bd` query output — NOT an assumed
  `bd dolt push` dry-run payload (no such inspectable dry-run is
  guaranteed to exist; naming the full set keeps the proof from silently
  degrading to issues.jsonl-only).
- **Clean-success → no-entry test (Req 2 privacy boundary, the
  load-bearing negative case)**: a SUCCESSFUL command with NO bound
  escape-hatch flag / override env / `repair phase` (e.g. `mindspec
  status`) appends ZERO journal entries — `PersistentPostRunE` runs on
  EVERY success, so without this a regression could silently start
  journaling all activity. Bead 2.
- **Fingerprint determinism + storm-cap + regression/stale tests**:
  `fingerprint(command, which-escape-hatch [, subcommand])` is
  deterministic across runs and identical for two runs of the same
  event; the SAME override flag with two different reason VALUES yields
  the SAME fingerprint (Bead 1), AND the fingerprint DIFFERS when any
  structured input differs (Bead 1). A storm of `L+1` same-fingerprint
  fires yields ONE entry whose `count` caps at the named
  `journalStormCapL` (`count==L`, per-fingerprint-per-session; Bead 2). A
  report marked `resolved_in_v2` then re-reported classifies REGRESSION
  at version == v2 (the `≥` boundary) and > v2, stale at < v2, and
  REGRESSION for a `dev`/unparseable version (unbounded-newest; Bead 3).
- **Fail-closed config-scope tests (Req 6/HC-3, Bead 4)**: the
  feedback-remote config loader is global/user-scoped ONLY — a
  feedback-remote config in a PROJECT-committed file is IGNORED; only
  global/user-scoped is read; absent the machine-global push credential
  the contract returns fail-closed (no "push anyway", no wrong remote) —
  asserted by a unit test on the new global-config loader (a DISTINCT
  net-new API; today's `config.Load` is repo-local only). Bead 3's
  `report` separately asserts it attempts no push/network call regardless.
- **Untrusted-corpus tests (Req 7/HC-4, bound to the v1 RENDERING
  surfaces)**: v1 journals structured ENUMS only (Req 2), so there is no
  free-text journal field; Req 7 is bound to the concrete surfaces where
  any text COULD surface — the `mindspec report` body output and the
  `mindspec report list` terminal rendering. A synthetic fixture feeds a
  markdown auto-link / injection payload through the report renderer and
  asserts it renders fenced, auto-linking neutralised, field length-
  capped, no `recovery:` line auto-executed; a slot value with a shell
  metachar in any copy-pasteable recovery line is escaped/placeholdered
  (Bead 3). (If v1 emits NO free-text field at all, the test is an
  explicit defense-in-depth backstop on the renderer, stated as such.)
- **CI no-op test (HC-6)**: with `GITHUB_ACTIONS` set, `mindspec report`
  is a no-op beyond the journal (no report write, no prompt, no push).
- **No new test frameworks**: existing Go `testing` + the existing
  `cmd/mindspec` cobra-command test seams; existing setup/test golden
  patterns for the config-scope tests.

## Decomposition Notes

Four beads sits in the 3-5 "optimal" band (no `decomposition-bead-count`
warning). The spec pinned three; the panel re-cut peels Req 6 into a
parallel Bead 4 (rationale below). The load-bearing dependency chain
stays a strict line — Bead 1 → Bead 2 → Bead 3, depth 3 (at the
`decomposition-chain-depth` threshold of 3, not over) — with Bead 4 a
parallel depth-1 bead off the line (it does NOT extend the chain, so the
depth-3 threshold is unaffected). Every chain edge is a spec-mandated
HARD gate, not granularity:

- **Bead 1 must merge before 2 and 3** (HC-1): privacy is a hard
  requirement; the golden-corpus gate must be green before any code can
  write a journal entry or a friction report from collected data.
  Folding Bead 1 into 2 would ship a sink before its redaction gate.
- **Bead 2 before Bead 3** (Req 4 single-sink architecture): the
  journal is the source of truth and the consolidation point; `report`
  consolidates what the self-emit WROTE. Bead 3's `report list`
  regression/stale logic reads the version-stamped entries Bead 2
  produces. Folding them re-couples the always-on capture path with the
  owner-invoked dispatch path.

- **Bead 4 is parallel, NOT on the chain** (r4/codex-decomposition): Req
  6's feedback-remote config contract is net-new global/user-scoped
  config plumbing in `internal/config` with NO functional dependency on
  the redaction lib or the journal in v1 — v1 does no push, so `report`
  writes local-only regardless of the config. Bundling it behind 1→2→3
  was a borderline-FALSE serialization edge; peeling it gives the
  config-scope/fail-closed contract its own focused adversarial diff and
  rightsizes the report bead.

Parallelism is 2/4 = 0.50 zero-inbound (Bead 1 AND Bead 4 start
immediately), above the 0.25 floor — no parallelism warning expected.
File overlap is deliberate and serialized by the dependency edges:
`internal/redact` (Bead 1) is consumed by Beads 2 and 3;
`internal/journal` (Bead 2) is read by Bead 3; `cmd/mindspec/root.go` is
touched only by Bead 2; `internal/config` is touched only by Bead 4 (no
overlap with the chain). Dependency wiring at plan-approve time is
best-effort; the orchestrator verifies all edges (1←nothing, 2←1, 3←1,2,
4←nothing) post-approve.

## Bead 1: Redaction library + adversarial golden corpus

**Scope**
Req 1; HC-1, HC-2, HC-7 (the redaction analog of fail-closed), plus the
Req 3 canonical fingerprint helper AND the normalized-version helper
(DQ4 — the loop-closing keystone). Lands FIRST so no sink exists before
its privacy gate is green (HC-1). NEW package `internal/redact` (core)
and a small `internal/version` helper (core). Pins the exported §API
Contract signatures (`Scrub`/`RedactEvent`/`Fingerprint`/`Identity` +
the version helper) that Beads 2/3 consume — the load-bearing cross-bead
seams, especially the fail-closed DROP signal.
READ-ONLY source files for the fixtures: `internal/next/guard.go`
(ClaimFailure `:176`, worktree recipe `:180`/`:210`),
`internal/validate/divergence.go` (`:186` adr-divergence-unowned),
`internal/complete/complete.go` (`joinResultErrorMessages`/
`adrDivergenceFailure` fold at `:340`/`:612`/`:650`) — no behavior
change to any of these; their templates are the adversarial corpus.

**Steps**
1. Create `internal/redact` with the **structured enum allowlist** as
   the PRIMARY defense (HC-2): the allowlist holds ONLY closed-set
   mindspec-emitted tokens — command/subcommand name, the escape-hatch
   flag NAME as an enum/boolean "was-set", the bare `version` semver
   (`version.Current()`, NOT the decorated `--version` string), OS. It
   NEVER holds a value copied from argv/env/user input (a flag VALUE,
   env VALUE, ADR id, glob, path arg, or `argv[0]`'s invocation path are
   TAINTED, not allowlisted — M4). `argv[0]` is reduced to `basename`
   and passed through the scrub before any return (M3).
2. Implement the tainted-by-default full scrub: absolute paths +
   secrets/emails/IPs; **relative source paths** and `*.go`/path-shaped
   tokens → `<path>`; **identifiers** (spec slugs, bead ids,
   `bead/<id>`/`spec/<slug>` branch names, OWNERSHIP domain names, file
   names) → typed placeholders; an **entropy catch-all** for long hex /
   base64 runs; and the **`%w`/`%v` chain rule** — unwrap to the
   sentinel error class/code and discard the wrapped message, OR run the
   full scrub + entropy pass over the WHOLE chain (not just line 1) and
   length-cap it; never ship a raw wrapped chain.
3. Make redaction **fail-closed (HC-7)**: if the redactor errors,
   panics, or cannot CONFIDENTLY classify a field, the field (or the
   whole entry) is DROPPED and the RAW value is NEVER returned, logged,
   or emitted as a fallback. No raw-string fallback path exists. The
   drop is SIGNALLED to the caller mechanically per the §API Contract —
   `Scrub(s) (clean string, ok bool)` returns `ok == false` (and an empty
   `clean`) so Bead 2/3 can implement "drop the field" deterministically;
   `RedactEvent` returns `false` to drop the whole entry. Pinning this
   return shape is a step deliverable (it is the cross-bead seam).
4. Expose the canonical **fingerprint helper** (Req 3) per the §API
   Contract: `Fingerprint(Identity{Command, EscapeHatch, Subcommand})`
   over the structured enum fields ONLY — NO error-class, NO override
   reason, NO user-supplied value. Deterministic, reason-INVARIANT, and
   **DISTINCT when any structured input differs** (the dedup key must be
   falsifiable). Return BOTH the hash and keep the `Identity` tuple as
   the normalized event identity (persisted by Bead 2 alongside the hash,
   so a collision cannot alias two events — DQ5).
4b. Expose the **normalized-version helper** `internal/version` (DQ4):
   `Current()` returns the bare `version` package var (`root.go:35`), NOT
   the decorated cobra `--version` string; `Parse`/`Compare` parse semver
   and implement the `dev`/unparseable → unbounded-newest policy. This is
   what makes the Req 3 regression/stale loop defined on the dev builds
   the evidence came from; without it the loop is inert.
5. Build the **adversarial golden corpus** from the REAL strings named
   in Scope (guard ClaimFailure + worktree recipe, divergence `:186` as
   folded by complete.go, a `%w`-wrapped Dolt-1105 chain carrying a bead
   description) and assert ZERO leakage of paths/slugs/branch-names/
   bead-ids/domain-names/descriptions/high-entropy tokens.
6. Wire `go test ./internal/redact/...` as the **HC-1 CI gate** (fails
   the build on any leakage), and add the allowlist-boundary fixtures
   (enum-only → unchanged; free text → scrubbed) + fingerprint
   determinism/reason-invariance tests.

**Verification**
- [ ] `go build ./... && go test -short ./...` green (HC-5)
- [ ] `go test ./internal/redact/...` passes; golden corpus asserts zero
      leakage on the real-string fixtures; it is wired as a CI gate that
      fails the build on any leakage (HC-1)
- [ ] An enum-only fixture redacts to itself unchanged; a free-text /
      tainted-value fixture passes the full scrub (HC-2/M4)
- [ ] A redaction-failure fixture returns DROPPED, never the raw value
      (HC-7); `argv[0]` is reduced to `basename` before any return (M3)
- [ ] `fingerprint` is deterministic and reason-invariant (same flag +
      two different reason VALUES → same fingerprint); contains no
      error-class

**Acceptance Criteria**
- [ ] Spec AC "Redaction (Req 1 / HC-1)": the adversarial golden-corpus
      test feeds the real `guard.go` ClaimFailure (`:176`) + worktree
      recipe (`:180`/`:210`), `divergence.go:186` as folded by
      `complete.go` (`:340`/`:612`/`:650`), and a `%w`-wrapped Dolt-1105
      chain, and asserts ZERO of: absolute/relative paths, `*.go`
      tokens, spec slugs, branch names, bead ids, OWNERSHIP domain
      names, bead descriptions, high-entropy hex/base64 runs
- [ ] Spec AC: the structured enum allowlist is the primary path
      (enum-only fixture unchanged; free-text fixture fully scrubbed)
- [ ] Spec AC (CI gate, FALSIFIABLE not declarative): (a) a deliberately
      leaky/mutated fixture — one carrying an un-neutralised abs path or
      bead description — makes `go test ./internal/redact/...` exit
      NON-ZERO (a negative/mutation assertion), AND (b) the CI workflow
      file lists `go test ./internal/redact/...` in a required,
      non-skippable job (HC-1)
- [ ] **HC-7 fail-closed (promoted from Verification)**: a
      redaction-failure fixture returns the DROP signal (`ok == false`,
      empty `clean`) and NEVER the raw value — no raw-string fallback path
- [ ] **M3 basename (promoted from Verification)**: `argv[0]` is reduced
      to `basename` and passed through the scrub before any return; the
      verbatim home-dir/username invocation path is never returned
- [ ] Spec AC "Fingerprint" (Req 3): `Fingerprint(Identity{command,
      escape-hatch, subcommand})` deterministic across two runs;
      reason-invariant (same override flag + two different reason VALUES →
      SAME fingerprint) with NO error-class / user value; **AND DISTINCT
      when any structured input differs** (`Fingerprint(complete,
      override-adr) != Fingerprint(complete, allow-doc-skew)` and `!=
      Fingerprint(next, override-adr)`) — a constant-returning fingerprint
      must FAIL
- [ ] **Version helper (DQ4)**: `version.Current()` returns the bare
      semver (not the decorated `--version` string); `Parse("dev")`
      reports unparseable; `Compare` implements the `dev`→unbounded-newest
      policy (an injected fake semver exercises the ordering)

**Depends on**
None

## Bead 2: Self-emit capture + always-on isolated 0600 journal

**Scope**
Req 2 (SUCCESS-path capture), Req 3 (version stamping consuming Bead 1's
fingerprint + version helpers), Req 8 (friction-storm cap); HC-1, HC-6,
HC-7, HC-8. NEW package `internal/journal` (core) implementing the §API
Contract (`AppendSuccessEvent`/`ListReports`/`MarkResolved`) and the §
Storage Contract (`journal.jsonl` at `os.UserConfigDir()/mindspec/`,
`0600`, schema, REUSING `internal/ndjson`'s `O_APPEND`+mutex+`0600` sink
for concurrent append — NOT re-deriving concurrency). The store is
DEDICATED, NON-SYNCED (NOT under any project/git tree, NOT
`.beads/issues.jsonl`, NOT swept by `bd`/`dolt push`); the append API
scrubs at WRITE time via Bead 1's `internal/redact`, persists the
normalized event identity + fingerprint + per-fingerprint count + cap,
and stamps the **bare `version` var** (Bead 1's `version.Current()`),
NOT the decorated `--version` string. EDIT `cmd/mindspec/root.go`
(`PersistentPostRunE`, `:67`) for the success-path self-emit — reading
the LEAF command's escape-hatch flags and dispatching on
`cmd.Name()`/`cmd.CommandPath()`. Journaling is BEST-EFFORT / NON-FATAL
to command success (§API Contract). The opt-in `--trace`/
`MINDSPEC_TRACE` path (`:53-56`) is untouched (ADR-0027).

**Steps**
1. Plan-phase **success-path event audit (DQ2, CORRECTED — C1/C2)**:
   the override flags are **leaf-local**, NOT root persistent — audit the
   leaf success commands `complete` (`complete.go:145-147`), `impl
   approve` (`impl.go:36-38`), the hidden `approve impl`
   (`approve.go:56-58`), and a completed `repair phase`
   (`repair.go:32-55` — the command is `mindspec repair phase <spec-id>`,
   parent+child). In `PersistentPostRunE` `cmd` is the LEAF, so read each
   flag via `cmd.Flags().Changed("override-adr")` /
   `Changed("allow-doc-skew")` / `Changed("supersede-adr")` on the leaf,
   and **dispatch on `cmd.Name()`/`cmd.CommandPath()`** because the same
   flag NAME lives on three commands at two scopes (bead-level `complete`
   vs spec-epic `approve`/`impl`). A literal builder following the old
   "audit root.go persistent flags" text would look in the wrong place
   (root has only `--trace`). Bind ONLY these confirmed leaf signals.
   **`MINDSPEC_ALLOW_MAIN` is NOT bound in v1** — it is consumed in
   `internal/hook/dispatch.go:51` (the `mindspec hook` git-pre-commit
   path, motivated by raw `MINDSPEC_ALLOW_MAIN=1 git commit`), which never
   runs a capturable leaf, and an ambient `os.Getenv` check would fire a
   false friction event on every command in any shell that exported the
   var; it is deferred to the failure/hook-path capture bead (DQ6 /
   §Non-Goals). Record the bound set + audit as bead evidence.
2. Implement `internal/journal`: an append-only JSONL store under a
   dedicated mindspec state dir (NOT a project/git tree, NOT the beads
   DB), files created **`0600`** (HC-8). The append API scrubs every
   field through `internal/redact` at WRITE time (HC-1) and is
   **fail-closed** (HC-7): a field the redactor cannot classify is
   dropped, never written raw.
3. Persist ONLY the §Storage Contract journal record — `basename(argv[0])`
   + the leaf command token + the escape-hatch enum token(s) + the Bead 1
   fingerprint + the normalized `Identity` tuple (so a hash collision
   can't alias two events) + count + the **bare `version.Current()`
   semver** (NOT the decorated `--version` string, whose commit hash the
   entropy scrub would eat — V1) — NEVER a raw command string, NEVER
   `argv[0]`'s full invocation path, NEVER a flag VALUE (M3/M4).
4. Implement the **friction-storm cap (Req 8)**: collapse by the
   normalized identity + fingerprint into ONE entry carrying an
   occurrence `count`, with a **named per-fingerprint-per-session cap
   `journalStormCapL` (= 50)** so a runaway loop cannot bloat the
   journal — firing `M < L` times yields one entry with `count == M`;
   firing `L+1` times yields one entry whose `count` stops growing past
   `L` (`count == L`, not `L+1`). The cap is a named constant a
   deterministic test can assert against.
5. Wire the self-emit into `PersistentPostRunE` (`:67`): on a SUCCESS
   whose LEAF command (`cmd.Name()`/`CommandPath()`) used a bound
   escape-hatch flag or is a completed `repair phase`, append exactly one
   redacted journal entry; a SUCCESS with NO bound friction signal (the
   common case — `PersistentPostRunE` runs on EVERY success) appends
   NOTHING (A1 — the load-bearing privacy boundary). The append is
   **BEST-EFFORT / NON-FATAL**: an `AppendSuccessEvent` error or a
   redaction drop is swallowed and NEVER returned from the hook, so an
   already-successful side-effecting command (`complete`, `impl approve`)
   never becomes a post-mutation failure. Failed / gate-blocked commands
   `os.Exit` BEFORE this hook and are structurally uncapturable — out of
   scope; leave the `--trace`/`MINDSPEC_TRACE` path untouched.
6. Tests: a `complete --override-adr`/completed-`repair phase` SUCCESS
   appends exactly one entry; a **clean SUCCESS with no bound friction
   signal (e.g. `status`) appends NONE** (A1); a gate-blocked `os.Exit`
   run appends NONE; the on-disk grep finds no home-dir path / secret /
   flag-value; the HC-7 drop test; the best-effort test (a forced journal
   error leaves the parent command's exit code unchanged); the
   `0600`/non-committed perms test; the storm-cap boundary test (`L+1`
   fires → `count == L`); a version-stamp test asserting the bare semver
   (not the decorated `--version`) is recorded.

**Verification**
- [ ] `go build ./... && go test -short ./...` green (HC-5)
- [ ] `go test ./internal/journal/... ./cmd/mindspec/...` passes
- [ ] A success-path leaf-flag/`repair phase` event appends exactly ONE
      redacted entry from `PersistentPostRunE`; a CLEAN success with no
      bound friction signal appends NONE; a gate-blocked `os.Exit` run
      appends NONE (structurally uncapturable, not merely filtered)
- [ ] Grep of the on-disk journal for a planted home-dir path / secret /
      override-reason returns ABSENT; the entry holds only
      `basename(argv[0])` + leaf command token + escape-hatch enum +
      identity tuple + fingerprint + count + bare version (M3/M4)
- [ ] HC-7 fail-closed: a redaction-failure fixture yields no on-disk
      entry for that field and never the raw string; a forced journal
      error leaves the parent command exit code unchanged (best-effort)
- [ ] HC-8: journal files are `0600` under a non-project,
      non-`bd`/`dolt` state dir (`os.UserConfigDir()/mindspec/`)

**Acceptance Criteria**
- [ ] Spec AC "Self-emit + journal" (Req 2): a SUCCESS-path leaf
      escape-hatch event (nil `RunE` with `complete --override-adr` /
      completed `repair phase`) appends exactly one redacted entry; a
      gate-blocked `os.Exit` run appends none
- [ ] **Clean-success → NO-ENTRY (A1, the privacy boundary)**: a
      SUCCESSFUL command with NO bound escape-hatch flag, no
      `repair phase` (e.g. `mindspec status`) appends ZERO entries —
      `PersistentPostRunE` runs on every success, so this negative case
      bounds the always-on sink and blocks a journal-everything regression
- [ ] Spec AC: the persisted entry contains `basename(argv[0])` + leaf
      command token + escape-hatch enum token(s) + identity tuple +
      fingerprint + count + bare version, and NO raw command, NO
      `file_path`, NO `argv[0]` full path, NO flag VALUE
- [ ] Spec AC "Fail-closed redaction (HC-7)": redaction-failure fixture
      yields no on-disk entry and never the raw string; journaling is
      best-effort (a forced append error never fails the parent command)
- [ ] Spec AC "At-rest (HC-8)": journal/store files `0600` under a
      non-project, non-`bd`/`dolt` dir (`os.UserConfigDir()/mindspec/`);
      planted-path grep absent
- [ ] Spec AC "storm" (Req 8): firing the same fingerprint `M < L` times
      → ONE entry with `count == M`; firing `L+1` times → ONE entry whose
      `count` caps at the named `journalStormCapL` (`count == L`, not
      `L+1`), per-fingerprint-per-session
- [ ] Spec AC "trace + always-on" (split, falsifiable): (a) ALWAYS-ON —
      with `MINDSPEC_TRACE` unset a success-path friction event still
      appends exactly one entry, and toggling `--trace` changes neither
      entry count nor content; (b) FRICTION-GATED — a success-path command
      that used NO escape-hatch/`repair phase` appends ZERO entries
- [ ] Spec AC "Fingerprint + version" (Req 3): every journal entry
      carries the **bare `version.Current()` semver** (not the decorated
      `--version` string) + the fingerprint + the normalized identity

**Depends on**
Bead 1

## Bead 3: `mindspec report` + `report list` + resolved_in_vX + ADR

**Scope** — the REPORT LOOP (Req 6 is peeled to Bead 4).
Req 4 (`mindspec report` → friction report in the non-synced store),
Req 5 (`report list` triage + `resolved_in_vX`), Req 7 (untrusted-corpus
rules bound to the report/report-list rendering surfaces), Req 9
(bootstrap-paradox doc), plus the regression/stale loop of Req 3; HC-3,
HC-4, HC-6, HC-7, HC-8. NEW `cmd/mindspec/report.go` (execution); EDIT
`internal/journal` (read/consolidate + friction-report store API per the
§Storage Contract `reports.jsonl` schema). Authors the NEW ADR (number
per the §ADR Fitness procedure — expected 0038, re-checked at claim time)
+ the ADR-0023/0027/0035 touchpoints. v1 is local-only: NO remote push,
NO bead write. (Req 6's feedback-remote config contract → Bead 4, which
runs in parallel; `report` here writes local-only regardless of it.)

**Steps**
1. Implement `mindspec report`: consolidate the journal (collapsed by
   the normalized identity + fingerprint, Req 3) into a redacted
   friction report (§Storage Contract `reports.jsonl` schema), deriving
   `first_version`/`last_version` by min/max over the version-stamped
   journal entries via the Bead 1 `version` helper, stamped with the
   **bare semver** + the fingerprint, written to the SAME dedicated
   non-synced `0600` store as the journal (HC-3/HC-8). The report body
   passes Bead 1's redaction. It MUST NOT write via `bd`, MUST NOT touch
   `.beads/issues.jsonl`, MUST NOT enter any `bd`/`dolt push`/git path
   (Req 4 / ADR-0023). In CI / non-interactive (`GITHUB_ACTIONS`),
   `report` is a no-op beyond the journal (HC-6). With no feedback-remote
   credential (the v1 default; Bead 4's contract) `report` writes
   local-only and attempts no push/network call.
2. Implement `mindspec report list`: a triage view that READS the
   friction store (NOT `bd`) showing fingerprint, command, escape-hatch,
   occurrence count, first/last version seen, and regression/stale
   status; plus a way to mark a report **`resolved_in_vX`** persisted
   back to the same non-synced store, keyed by the **normalized event
   identity + fingerprint** (NOT the opaque hash alone — collision
   safety). Compute regression/stale at `report list` time via the Bead 1
   version helper: parsed version > X → regression, == X → REGRESSION
   (the `≥` boundary), < X → stale, `dev`/unparseable → regression
   (unbounded-newest, DQ4).
3. Apply the **untrusted-corpus rules (Req 7/HC-4)** bound to the v1
   RENDERING surfaces — the `report` body output and the `report list`
   terminal rendering (v1 journals enums only, so there is no free-text
   journal field; these renderers are where any text could surface).
   Render any collected text inside fenced code blocks, neutralise
   markdown auto-linking, length-cap every field, ensure no
   agent/automation auto-executes a `recovery:` line; ESCAPE/placeholder
   any slot value (shell-metachars neutralised) that appears in
   copy-pasteable recovery text (P3). If v1 emits NO free-text field at
   all, this is an explicit defense-in-depth backstop on the renderer
   tested with a synthetic fixture — stated as such, not a phantom field.
4. **Document the bootstrap paradox (Req 9)** in the new ADR + a spec/
   doc note: install-failure friction is structurally UNREPORTABLE
   in-tool (if install fails, mindspec is not present to self-report);
   name the deferred out-of-band home (installer-emitted failure signal
   / manual report). Author the new ADR (number per §ADR Fitness;
   hand-create, NOT `mindspec adr create`; add the landed ID to this
   frontmatter) covering the redaction architecture, success-path
   capture, signal taxonomy, fingerprint/version scheme, fail-closed
   identity, untrusted-corpus stance, and v1 scope. The ADR MUST
   additionally FREEZE the concrete decisions later specs build on: the
   exact **store path + filenames**, the **journal + report record
   schemas**, the **concurrent-append/locking model**, the **hash +
   normalized-identity + version-comparison strategy** (incl. the `dev`
   policy), and the **retention stance (none in v1)**. ALL ADR examples
   and snippets MUST use PLACEHOLDERS / synthetic fixtures (`<path>`,
   `bead/<id>`) — NEVER live captured strings (the ADR is git-committed
   OUTSIDE the redaction sink; a pasted real string is a committed leak).
   Record the ADR-0023/0027/0035 touchpoints.
5. Tests: the store-isolation egress proof (run `report`, assert the
   fingerprint ABSENT from `.beads/issues.jsonl` AND the dolt
   working-set/tracked tables AND `bd` query output — NOT an assumed
   `bd dolt push` dry-run payload); the CI no-op (HC-6); the no-push test
   (report attempts no network call); the regression/stale boundary test
   (version == X → regression, > X → regression, < X → stale, injected
   semver); the injection-payload fenced/neutralised render test; the
   slot-escaping test; an ADR-example-placeholder-only inspection; an
   ADR/spec-text inspection for the bootstrap-paradox out-of-band path.

**Verification**
- [ ] `go build ./... && go test -short ./...` green (HC-5)
- [ ] `go test ./cmd/mindspec/... ./internal/journal/...` passes
- [ ] Store-isolation egress proof: a `report`-created friction report's
      fingerprint is ABSENT from `.beads/issues.jsonl`, the dolt
      working-set/tracked tables, AND `bd` query output (the full set,
      not a single assumed dry-run payload)
- [ ] With `GITHUB_ACTIONS` set, `report` is a no-op beyond the journal
      (HC-6); with no feedback-remote credential, `report` writes
      locally and attempts no push/network call (test FAILS on any push)
- [ ] The new ADR file exists with a non-0037 number (expected 0038);
      its Domain(s) line intersects {core, execution, workflow}; the
      bootstrap-paradox out-of-band path is documented (Req 9); its
      examples use PLACEHOLDERS only (no live captured strings) and it
      freezes the store path / schemas / locking / hash+version strategy

**Acceptance Criteria**
- [ ] Spec AC "Report → friction store" (Req 4): `mindspec report`
      consolidates the journal (collapsed by fingerprint) into a
      redacted, version+fingerprint-stamped report in the dedicated
      non-synced store; the body passes Req 1 redaction
- [ ] Spec AC "Store isolation (egress-proof)" (HC-3): the report's
      fingerprint is absent from `.beads/issues.jsonl` AND the dolt
      working-set/tracked tables AND `bd` query output (the provably-
      implementable full set, not an assumed `bd dolt push` dry-run)
- [ ] Spec AC "CI no-op" (HC-6): with `GITHUB_ACTIONS` set, `report` is
      a no-op beyond the journal; with no feedback-remote credential it
      writes local-only and attempts no push (test fails on any push)
- [ ] Spec AC "report list" (Req 5): reads the friction store (not
      `bd`), shows fingerprint/command/escape-hatch/count/version-range/
      regression-stale status, offers a `resolved_in_vX` mark
- [ ] Spec AC "Fingerprint + version loop" (Req 3): a report
      `resolved_in_v2` re-reported classifies REGRESSION at version == v2
      (the `≥` boundary) AND at version > v2, stale/suppressed at version
      < v2, and REGRESSION for a `dev`/unparseable version (unbounded-
      newest); `report list` reflects it (injected-semver test)
- [ ] Spec AC "Untrusted corpus" (Req 7/HC-4), bound to the report /
      report-list render surfaces: an injection/auto-link payload renders
      fenced + neutralised + length-capped, no `recovery:` auto-executed;
      a shell-metachar slot value in copy-pasteable recovery text is
      escaped/placeholdered (synthetic fixture if no live free-text field)
- [ ] Spec AC "New ADR landed" (ADR Touchpoints): the ADR file exists
      with a non-0037 number (expected 0038), its Domain(s) line
      intersects {core, execution, workflow}, its examples use
      PLACEHOLDERS only (no live captured strings), and it freezes the
      store path / record schemas / locking / hash+version strategy
- [ ] Spec AC "Bootstrap paradox" (Req 9): the spec/ADR documents
      install-failure friction as structurally unreportable in-tool and
      names the deferred out-of-band home

**Depends on**
Bead 1, Bead 2

## Bead 4: Feedback-remote config contract (global-scoped, fail-closed)

**Scope**
Req 6 (the feedback-remote config contract — capability-based,
fail-closed, global/user-scoped); HC-3 (the config-scope + fail-closed
identity half). EDIT/EXTEND `internal/config` (core) with **NET-NEW
global/user-scoped config loading** — today's `config.Load`
(`internal/config/config.go`) reads ONLY the project `.mindspec/config.yaml`
under the repo root; there is NO user-global loader, so this is a
DISTINCT new API, not an edit to an existing global surface. Peeled from
the report bead because in v1 no push occurs — `mindspec report` writes
local-only regardless of this config — so it has NO functional
dependency on the redaction lib (Bead 1) or the journal (Bead 2) and
runs in PARALLEL. v1 ships the CONTRACT only; the actual cross-install
push is deferred (§Non-Goals), but the scope + fail-closed properties
ship NOW so the follow-on cannot regress them.

**Steps**
1. Add a global/user-scoped feedback-remote config loader to
   `internal/config`: resolve from the machine-global location (the same
   `os.UserConfigDir()/mindspec/` root as the §Storage Contract state
   dir, e.g. `config.yaml` there), DISTINCT from the project-scoped
   `config.Load`. Define the precedence: the feedback-remote setting is
   read ONLY from the global/user scope and is NEVER honored from a
   project-committed file (a committed remote would leak the URL even
   though creds gate the push — HC-3).
2. Implement the **fail-closed contract**: identity is possession of the
   machine-global feedback-remote push credential, enforced at the WRITE
   DESTINATION. Absent the credential, the contract returns a
   no-push/local-only result — NEVER a silent fallback to "push anyway"
   or a wrong remote. No CLI-level per-user hiding (you cannot hide a
   subcommand in a shared binary); the gate is the credential.
3. Tests: a feedback-remote config in a PROJECT-committed
   `.mindspec/config.yaml` is IGNORED; only the global/user-scoped value
   is read; with no credential the contract resolves fail-closed
   (no-push) and a test FAILS if any push/network destination is
   returned/attempted.

**Verification**
- [ ] `go build ./... && go test -short ./...` green (HC-5)
- [ ] `go test ./internal/config/...` passes
- [ ] A project-committed feedback-remote config is ignored; only the
      global/user-scoped config is read (Req 6 / HC-3)
- [ ] With no feedback-remote credential the contract is fail-closed:
      no push, no wrong-remote fallback (test fails on any push/network)

**Acceptance Criteria**
- [ ] Spec AC "Identity / config" (Req 6 / HC-3): the feedback-remote
      config is global/user-scoped ONLY — a project-committed
      feedback-remote config is ignored, never honored
- [ ] Spec AC "fail-closed identity" (Req 6 / HC-3): no-credential
      default → no push, no wrong-remote fallback (capability gated at
      the write destination, not the CLI)
- [ ] Spec AC: the global/user-scoped loader is a DISTINCT net-new API
      from the repo-local `config.Load` (project config keeps its
      precedence; the feedback-remote key is never read project-scoped)

**Depends on**
None

## Provenance

Spec acceptance criterion → owning bead + verification:

| Spec AC | Bead | Verified by |
|---|---|---|
| Adversarial golden corpus zero-leakage (Req 1 / HC-1) | Bead 1 | `internal/redact` real-string golden-corpus CI gate |
| Enum-allowlist primary path (HC-2 / M4) | Bead 1 | enum-only-unchanged + free-text-scrubbed fixtures |
| Redaction CI gate fails on leakage (HC-1) | Bead 1 | mutated-fixture → non-zero exit + CI-job-listed assertion |
| Fingerprint determinism + reason-invariance + DISTINCTNESS (Req 3) | Bead 1 | deterministic + same-flag-two-reasons + differs-on-any-input fixtures |
| Fail-closed DROP signal + `argv[0]`→basename (HC-7 / M3) | Bead 1 | drop-signal (`ok=false`) fixture + basename reduction (promoted to AC) |
| Normalized-version helper + `dev` policy (Req 3 / DQ4) | Bead 1 | bare-semver `Current` + `Parse("dev")` unparseable + `Compare` injected-semver test |
| Success-path single-entry / `os.Exit` none (Req 2) | Bead 2 | `PersistentPostRunE` leaf-flag capture tests |
| Clean-success → NO entry (Req 2 privacy boundary, A1) | Bead 2 | no-friction success appends zero entries |
| Entry holds enum-only, no raw/flag-value (M3/M4) | Bead 2 | on-disk grep for planted path/secret/reason |
| Fail-closed drop + best-effort non-fatal (HC-7) | Bead 2 | redaction-failure fixture → no entry; forced append error doesn't fail parent |
| `0600` + non-committed path (HC-8) | Bead 2 | perms assertion + `os.UserConfigDir()/mindspec/` location test |
| Friction-storm cap (Req 8) | Bead 2 | `L+1`-fire → one entry `count==L` (named `journalStormCapL`) |
| Trace always-on + friction-gated (split) | Bead 2 | trace-state-independent single entry + no-friction-success zero entries |
| Version stamping — bare semver (Req 3) | Bead 2 | per-entry bare-`version.Current()` assertion |
| Consolidated redacted report + first/last version (Req 4) | Bead 3 | `report` body redaction + bare-version/fingerprint stamp + min/max derive |
| Store isolation egress-proof (HC-3) | Bead 3 | `.beads/issues.jsonl` + dolt working-set + `bd` query fingerprint-absent |
| CI no-op + no-push (HC-6 / Req 4) | Bead 3 | `GITHUB_ACTIONS` no-op + no-network-call test |
| `report list` triage + `resolved_in_vX` (Req 5) | Bead 3 | store-read shape + identity-keyed mark-persist test |
| Regression/stale loop incl. `==X` boundary + `dev` (Req 3) | Bead 3 | `resolved_in_v2` re-report classification, injected-semver |
| Untrusted corpus + slot escaping (Req 7 / HC-4) | Bead 3 | injection-payload fenced + slot-escape render-surface tests |
| Bootstrap-paradox doc (Req 9) | Bead 3 | ADR/spec-text inspection |
| New ADR landed + placeholder-only + frozen decisions (ADR Touchpoints) | Bead 3 | ADR file exists, non-0037 number, domains intersect, placeholder-example inspection |
| Fail-closed / config-scope (Req 6 / HC-3) | Bead 4 | no-push test + project-committed-config-ignored test (net-new global loader) |
| build/test green (HC-5) | all beads | per-bead build/test verification lines |
| Bead 1 merges before any data-emitting bead (HC-1) | Beads 1-3 | dependency chain 1 → 2 → 3 (Bead 4 parallel) |
