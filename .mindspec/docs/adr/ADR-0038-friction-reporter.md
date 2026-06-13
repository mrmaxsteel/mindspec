# ADR-0038: Owner-Local Friction Self-Improvement Loop — Privacy-First Capture, Isolated Store, and the Fingerprint/Version Loop

- **Date**: 2026-06-13
- **Status**: Accepted
- **Domain(s)**: core, execution, workflow
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0023](ADR-0023.md) (beads as single state authority — the friction store is a review/diagnostic artifact, NOT workflow state, isolated from the bd/dolt egress path), [ADR-0027-mindspec-otel-only](ADR-0027-mindspec-otel-only.md) (the trace emitter is opt-in/noop; the friction journal is a net-new always-on sink, deliberately NOT reusing the OTEL path), [ADR-0035](ADR-0035-agent-error-contract.md) (the recovery/error-line convention whose templates are the adversarial golden corpus the redaction library must neutralise)

---

## Context

The autopilot evidence kept surfacing the same friction — escape-hatch
overrides (`--override-adr`, `--allow-doc-skew`, `--supersede-adr`) and
manual `repair phase` invocations — but that signal lived only in
transient session logs. Spec 094 closes a self-improvement loop:
mindspec records its own gated-override admissions to an always-on
local journal, the owner consolidates them into friction reports, and
triages them with a version-aware regression loop so a fixed friction
that recurs is surfaced again.

The gating constraint is **privacy** (spec HC-1): the captured data is
mindspec's own command surface, but the escape-hatch *reasons*, the
invocation paths, and the error/recovery strings it sits next to drag
absolute paths, secrets, bead descriptions, spec slugs, and branch
names. A self-reporting tool that leaks those would be worse than no
tool. Every decision below exists because the alternative looks
reasonable to a future editor who does not have this record — and each
relaxation reopens a leak or an egress path.

This ADR FREEZES the concrete decisions later specs build on: the store
path and filenames, the two-file schema, the concurrency/locking model,
the hash + normalized-identity + version-comparison strategy (including
the `dev` policy), the retention stance, and the owner-local v1 scope.

All examples in this ADR use PLACEHOLDERS / synthetic fixtures only
(`<path>`, `bead/<id>`, `<fingerprint>`). The ADR is git-committed
OUTSIDE the redaction sink, so a pasted real captured string would be a
committed leak — never paste one here.

## Decision

### 1. Privacy-first redaction: structured-enum-FIRST, tainted-by-default, fail-closed

The primary defense is **collecting only closed-set, mindspec-emitted
enum fields**, NOT free-text scrubbing (HC-2). The journal stores the
leaf command token, *which* escape-hatch flag was set (as an enum, not
its value), the optional subcommand token, the bare version, and the
OS — all validated against an allowlist of mindspec's own tokens.

- **The allowlist NEVER holds a value copied from argv/env/user input**
  (M4): a flag's VALUE (an override reason, a path arg, an ADR id, a
  glob), an env VALUE, or `argv[0]`'s invocation path are TAINTED —
  excluded from the allowlist, dropped or passed through the full
  scrub. `argv[0]` is reduced to `basename` and scrubbed before any
  return (M3).
- **Every collected STRING is tainted-by-default** and passes the full
  scrub (`internal/redact.Scrub`): absolute/relative paths, secrets,
  emails, IPs, `*.go`/path-shaped tokens, identifiers (spec slugs, bead
  ids, `bead/<id>`/`spec/<slug>` branch names, OWNERSHIP domain names),
  and a high-entropy hex/base64 catch-all all redact to typed
  placeholders.
- **The `%w`/`%v` error-chain rule** (`ScrubError`): unwrap to the
  sentinel error class/code and DISCARD the wrapped free-text, or drop
  the chain — a raw wrapped chain (e.g. a Dolt-1105 carrying a
  `bead/<id>` description) is NEVER shipped.
- **Fail-closed (HC-7)**: a field the redactor cannot CONFIDENTLY
  classify is DROPPED — the raw value is never returned, logged, or
  emitted as a fallback. The drop is signalled mechanically:
  `Scrub(s) (clean string, ok bool)` returns `ok == false`;
  `RedactEvent` returns `false` to drop the whole entry. There is no
  raw-string fallback path.
- **Human review is a backstop, never load-bearing.**

The adversarial golden-corpus test (`go test ./internal/redact/...`),
built from the REAL `ADR-0035` recovery/error templates, is wired as a
CI gate that FAILS the build on any leakage — not an advisory.

### 2. Success-path self-emit capture — NOT a hook, NOT OTEL

Capture happens in mindspec's own `PersistentPostRunE`
(`cmd/mindspec/root.go`), which cobra runs ONLY on a leaf command's
SUCCESS. A friction admission is an escape-hatch override flag that was
`Changed` on a SUCCEEDING leaf, or a completed `repair phase`. This is
deliberately:

- **NOT a PostToolUse / git hook**: those run outside a capturable
  mindspec leaf and cannot attribute the signal to a command.
- **NOT the OTEL emitter (ADR-0027)**: the trace sink is opt-in
  (`--trace`/`MINDSPEC_TRACE`) and defaults to noop; the friction
  journal is a net-new, always-on, separate sink, independent of trace
  state.
- **SUCCESS-path only**: failed/gate-blocked commands `os.Exit` BEFORE
  `PersistentPostRunE` and are structurally uncapturable in v1. Failed-
  path capture is a deferred follow-on.
- **`MINDSPEC_ALLOW_MAIN` is NOT bound in v1**: it is a raw-git bypass
  consumed in the pre-commit hook path that never runs a capturable
  leaf, and an ambient `os.Getenv` check would fire a FALSE friction
  event on every command in any shell that exported it.

Capture is **BEST-EFFORT / NON-FATAL**: a journal error or a redaction
drop is swallowed and never returned from the hook, so an already-
successful side-effecting command never becomes a post-mutation
failure. Fail-closed governs DATA EMISSION (drop the datum), not
command exit.

### 3. The signal taxonomy is escape-hatch-centered

The closed set of friction signals is exactly the escape-hatch
admissions: `override-adr`, `allow-doc-skew`, `supersede-adr`, and
`repair-phase`. These are the points where the workflow's own gates
were deliberately bypassed — the highest-value, lowest-noise friction
signal. No error-class, no general command telemetry.

### 4. Isolated 0600 store, symlink-safe git-tree guard — egress-proof

Both files live in a **dedicated, machine-global, NON-SYNCED state
dir**: `os.UserConfigDir()/mindspec/` (XDG-honoring on Linux, with the
documented `~/.config/mindspec/` → `~/.mindspec/` fallback chain). It is
**NEVER** under any project/git tree, NEVER the beads DB
(`.beads/issues.jsonl`), and NEVER swept by `bd`/`dolt push` (HC-3 /
HC-8). Files are created `0600` (owner-only); the mode is re-asserted
via the open fd against a permissive umask.

- The `MINDSPEC_STATE_DIR` test/operator override is subjected to the
  SAME guard: the path is CANONICALIZED (`Abs` + `EvalSymlinks`,
  nearest-existing-ancestor) BEFORE the git-tree check, so an
  out-of-tree symlink whose TARGET is inside the repo cannot slip the
  journal under a committable tree. Resolution failure FAILS CLOSED
  (nothing written).
- The store-isolation egress proof asserts a report's `<fingerprint>`
  is ABSENT from the provably-implementable surface set covering
  everything `bd dolt push` would send (`.beads/issues.jsonl`, the dolt
  working set, `bd` query output) — never an assumed dry-run payload.

### 5. Two-file schema: append-only journal + consolidated reports

The §Storage Contract pins two files in the isolated store:

- **`journal.jsonl`** — APPEND-ONLY, immutable history. One REDACTED
  event per line, written in a SINGLE `O_APPEND` `write(2)` of a
  sub-`PIPE_BUF` JSONL record. NO in-file collapse, NO count field, NO
  read-modify-rewrite — each line preserves its OWN version stamp.
  Record:

  ```json
  { "v": 1, "ts": "<rfc3339>", "argv0": "<basename>",
    "command": "<leaf token>", "escape_hatch": "<enum>",
    "subcommand": "<enum>", "fingerprint": "<fingerprint>",
    "identity": {"command": "<token>", "escape_hatch": "<enum>",
                 "subcommand": "<enum>"},
    "version": "<bare semver|dev>" }
  ```

- **`reports.jsonl`** — the CONSOLIDATED, mutable VIEW. One Report per
  fingerprint, built by collapsing the journal. Record:

  ```json
  { "v": 1, "fingerprint": "<fingerprint>",
    "identity": {"command": "<token>", "escape_hatch": "<enum>",
                 "subcommand": "<enum>"},
    "command": "<token>", "escape_hatch": "<enum>",
    "subcommand": "<enum>", "count": <int>,
    "first_version": "<semver|dev>", "first_seen_ts": "<rfc3339>",
    "last_version": "<semver|dev>", "last_seen_ts": "<rfc3339>",
    "resolved_in_version": "<semver|dev|empty>" }
  ```

  `count` and `first/last_version` are DERIVED over the version-stamped
  journal lines by OCCURRENCE ORDER, NOT by semver extrema:
  `first_version`/`first_seen_ts` come from the chronologically-EARLIEST
  event (the earliest `ts`, tie-broken by append order) and
  `last_version`/`last_seen_ts` from the LATEST — each version moves
  together with its paired timestamp. (Deriving by semver min/max would
  report the wrong first/last for an out-of-order or downgrade stream;
  the spec says "first/last version SEEN" and this ADR says "the earliest
  EVENT", so occurrence order is authoritative.) The triage `status` is
  one of `{open, regression, stale}` (see §7) and is ALWAYS re-derived at
  list time from `resolved_in_version` vs `last_version`, never persisted
  as ground truth, so a stored status can never lie.

### 6. Concurrency / locking model

- **In-process**: a single `sync.Mutex` serializes journal appends (so
  two goroutines never interleave a partial line) and makes the
  per-session storm counter race-free.
- **Cross-process**: the journal append is line-atomic via the
  `O_APPEND` single `write(2)` of a sub-`PIPE_BUF` line — NO lost
  update, NO file lock. Consolidation tolerates interleaved/duplicate/
  malformed lines (a torn line is SKIPPED, never fatal).
- **`reports.jsonl`** is the one mutable file: it is rewritten WHOLESALE
  (write-temp + `rename`, `0600`) under the same mutex — a
  consolidate/resolve never half-writes the view.
- **Cross-process resolve preservation**: the in-process mutex does NOT
  span processes, so a stale consolidator (`report`:
  Consolidate→WriteReports) could otherwise clobber a concurrent
  `report list --resolve` running in another process and erase its
  `resolved_in_version`. `WriteReports` therefore performs a
  COMPARE-AND-MERGE under the lock IMMEDIATELY before the temp+rename: it
  re-reads the current `reports.jsonl` and, for every fingerprint, a
  NON-EMPTY on-disk `resolved_in_version` WINS over an empty slot in the
  slice being written. A concurrent resolve is never lost.

### 7. Fingerprint + version loop — hash + normalized identity + `dev` policy

- **Fingerprint** = a deterministic SHA-256 hash over the FULL normalized
  `Identity` tuple (`command + which-escape-hatch [+ subcommand]`),
  length-prefixed and NUL-framed per field — reason-INVARIANT (no
  override reason, no user value, no error-class) and DISTINCT when any
  structured input differs (the length-prefix + NUL delimiter prevents
  cross-field aliasing, so `{complete, override-adr, ""}` and
  `{complete-, override, -adr, ""}` cannot collide).
- **Fingerprint = H(identity) → fingerprint-keying IS identity-keying by
  construction (DQ5 collision safety)**: because the fingerprint is a
  strong hash of the COMPLETE normalized identity, two DISTINCT identities
  yield DISTINCT fingerprints (modulo a cryptographic SHA-256 collision,
  treated as impossible). Consolidation and `MarkResolved` therefore key
  by fingerprint ALONE and are still collision-safe — keying by
  fingerprint is exactly keying by `H(identity)`. The `identity` tuple is
  ALSO persisted on each record, but as a DISPLAY/audit field, not as part
  of the dedup/resolve key; it does not need to be re-checked at
  dedup/resolve time. A regression test asserts two distinct identities
  produce distinct fingerprints, pinning this invariant.
- **Version** = the BARE `version` package var (`version.Current()`),
  NOT the decorated cobra `--version` string (whose commit hash the
  entropy scrub eats). Non-release/local/test builds report `dev`.
- **`dev`/unparseable policy (DQ4)**: a running/last version that is
  `dev`/unparseable is treated as **unbounded-newest** — it classifies
  REGRESSION, never stale (fail TOWARD surfacing). `Compare` returns
  `(0, false)` for any `dev`/unparseable operand; the caller resolves
  `false` as regression. This is the only self-consistent reading of
  the two DQ4 statements (incoming `dev` → regression AND stored `dev`
  → any later concrete is a regression), which no total order satisfies.
- **Regression/stale at the `>=` boundary**: a report
  `resolved_in_version = X` that recurs at running/last version `>= X`
  is a REGRESSION (the `==X` boundary is regression — a re-occurrence at
  the resolving version is the loop not closing); `< X` is stale
  (suppressed, kept for the record).
- **The status model is exactly `{open, regression, stale}`** — the
  faithful realization of spec Req-3's two-way resolved split plus the
  unresolved case: `open` (no `resolved_in_version`), `regression`
  (resolved at X, last `>= X`, or a dev/unparseable operand), `stale`
  (resolved at X, last `< X`). There is NO separate `resolved` status —
  a resolved report with no later recurrence is already `stale` (resolved
  at X with `last_version < X`). An earlier draft advertised a fourth
  `resolved` token that `Classify` never returned (it was dead); the
  language across code, help text, and this ADR is reconciled to the
  3-state model so code, docs, and spec agree.
- **Resolve-version normalization (source-side slot neutralisation,
  Req 7 / HC-4)**: `MarkResolved` NORMALIZES the resolve `--version` at
  the SOURCE — only a concrete semver (canonicalized to bare
  `major.minor.patch`) or the explicit dev/current policy token is ever
  PERSISTED as `resolved_in_version`. Anything else (a non-semver string,
  or a shell-metacharacter payload like `1.0.0; rm -rf /`) is REJECTED
  with an error and never written, so a live executable user string can
  never reach the copy-pasteable resolve-echo or the RESOLVED-IN render
  column. This closes the one user-controlled free-text slot in v1 at the
  source rather than relying on the render scrub (which does not
  neutralise shell metacharacters).

### 8. Untrusted-corpus render stance (Req 7 / HC-4)

The store is enum-only, so most fields are structurally safe. But the
RENDER surfaces — the `mindspec report` body and `report list` terminal
output — treat every printed field as UNTRUSTED defense-in-depth:

- Each field is re-scrubbed via `redact.Scrub`; a residual-leak or
  otherwise-unclassifiable value renders as the `<redacted>` placeholder,
  NEVER raw (fail-closed). The fingerprint is **scrubbed at FULL length
  BEFORE any display truncation** — an oversized/unclassifiable
  fingerprint renders `<redacted>`, never a raw prefix.
- ALL Unicode control runes are stripped — C0 (U+0000–001F), DEL, AND C1
  (U+0080–009F, including CSI U+009B); `\n`/`\r`/`\t` collapse to a
  space. So neither an injected `recovery:` line nor a raw terminal
  escape can reach a downstream agent or the user's terminal.
- Markdown auto-linking and bare URL schemes are neutralised, the output
  is code-fenced, and every field is length-capped.

This is the explicit backstop the spec asks for, so a future free-text
field cannot regress the render contract.

### 9. Capability-based, fail-closed identity; owner-local v1 scope

v1 is **OWNER-LOCAL only**: `mindspec report` writes the local report
store and attempts NO remote push, NO bead write, NO `bd`/`dolt`/git
path. The owner's cross-install push is DEFERRED. The feedback-remote
config contract (a sibling bead) is **global/user-scoped ONLY** — a
project-committed feedback-remote config is IGNORED (a committed remote
leaks the URL even though creds gate the push); identity is possession
of the machine-global push credential, enforced at the write
destination. Absent the credential, the contract is fail-closed:
no-push, no wrong-remote fallback. No CLI-level per-user hiding (you
cannot hide a subcommand in a shared binary) — the gate is the
credential.

The deferred channels (owner Dolt push, public GitHub draft-and-submit)
plug into this fingerprint/version mechanism later WITHOUT reopening any
decision above.

### 10. Retention: NONE in v1 (decided, not deferred-by-omission)

The append-only `0600` journal grows unbounded across sessions (Req 8's
cap is per-fingerprint-PER-SESSION, bounding only a runaway in-process
loop). A redacted + fail-closed + `0600` stale entry is low at-rest
risk. Rotation/compaction is an explicit follow-on, NOT an oversight.

### 11. Bootstrap paradox (Req 9)

**Install-failure friction is structurally UNREPORTABLE in-tool**: if
the install fails, mindspec is not present to self-report it — the
always-on `PersistentPostRunE` sink never runs. This entire loop can
only capture friction from a *working* installation. The deferred
out-of-band home for install-failure friction is the **installer side**
(an installer-emitted failure signal) and a **manual GitHub issue** —
explicitly OUTSIDE this in-tool loop. A future enabler may wire the
installer path; it does not change any decision here.

A standalone user-facing note restates this boundary outside the ADR:
[friction-bootstrap-paradox.md](../user/guides/friction-bootstrap-paradox.md)
(placeholder-only, cross-linked back to this section).

## Consequences

### Positive

- The escape-hatch friction signal stops being transient log noise and
  becomes a triageable, version-aware loop with a regression alarm.
- Privacy is structural (enum-first + fail-closed + isolated 0600
  store + git-tree guard), not a code-review promise; the golden-corpus
  CI gate makes a leak fail the build.
- The store is provably isolated from the `bd dolt push` egress path —
  a redaction MISS cannot reach the shared remote.
- Later channels (owner Dolt, public GitHub draft) reuse this
  fingerprint/version mechanism without reopening the frozen decisions.

### Negative / Tradeoffs

- v1 captures ONLY success-path escape-hatch friction; failed/
  `os.Exit`/install-failure friction is out of scope (named as
  deferred, not silently dropped).
- The journal grows unbounded (no retention) until the rotation
  follow-on lands.
- Enum-first means a NEW top-level command/subcommand must be added to
  the redaction allowlist (enforced by the cobra drift-guard test) or
  its friction events drop — a deliberate fail-closed default.
- The render surface re-scrubs already-redacted data (a small cost) as
  a defense-in-depth backstop, even though v1 emits no free-text field.

## Alternatives Considered

### 1. Free-text scrubbing as the PRIMARY defense

Rejected (HC-2): scrubbing is a backstop, not a primary mechanism. A
scrubber is a denylist that always loses to the next un-enumerated
pattern. Collecting only closed-set enum tokens means there is no
free-text to leak in the first place; the scrub is the second line.

### 2. Reuse the OTEL/trace emitter (ADR-0027) for capture

Rejected: the trace sink is opt-in and defaults to noop, so it would
miss the always-on friction signal, and routing self-improvement data
through the telemetry path conflates two concerns ADR-0027 separated.
The friction journal is a deliberately net-new sink.

### 3. Store friction in the beads tracker

Rejected (ADR-0023 / HC-3): the beads DB is git-tracked and swept by
the mandatory `bd dolt push`, so a redaction MISS would egress to the
shared remote. Friction reports are review/diagnostic artifacts, not
workflow state; they live in the isolated non-synced store.

### 4. A single mutable journal file (collapse-in-place with a count)

Rejected: an in-file read-modify-rewrite reintroduces cross-process
lost-update and a file-lock requirement, and loses per-event version
history (needed for first/last-seen). The append-only journal +
consolidated reports two-file split keeps appends lock-free and
preserves the version timeline.

### 5. Map `dev` to a single point on the version order

Rejected (DQ4): no total order satisfies BOTH "incoming `dev` →
regression" AND "stored `dev` → any later concrete is a regression".
Pinning `dev`/unparseable as not-cleanly-comparable and resolving it as
"fail toward surfacing" is the only self-consistent reading.

## Validation / Rollout

1. Bead 1 lands `internal/redact` (enum-first + fail-closed scrub +
   fingerprint helper) and `internal/version` (bare semver + `dev`
   policy), with the adversarial golden-corpus CI gate.
2. Bead 2 lands `internal/journal` (the append-only 0600 journal +
   symlink-safe git-tree guard + storm cap) and the `PersistentPostRunE`
   success-path self-emit.
3. Bead 3 (this ADR) lands `mindspec report` + `report list`, the
   `reports.jsonl` consolidation, the real `MarkResolved`, the
   regression/stale loop, and the untrusted-corpus render backstop.
4. The feedback-remote config contract (global-scoped, fail-closed)
   lands in parallel; the actual cross-install push is the deferred
   follow-on that plugs into this mechanism.
