---
approved_at: "2026-06-12T22:29:43Z"
approved_by: user
drafted_at: "2026-06-12"
drafted_by: spec-drafting research agent
source_design: bd show mindspec-sot1 (description + AUTH/IDENTITY note) + panel-sot1-design verdicts (r1-soundness NEEDS_REVISION, r2-security CHANGES_REQUIRED, r3-alternatives APPROACH_SOUND), /Users/Max/replit/orchestrator-staging-2026-06-10/panel-sot1-design/{r1-soundness,r2-security,r3-alternatives}.json
status: Approved
---
# Spec 094-self-improvement-loop: Self-improvement loop: owner-local friction reporting (redaction-first, self-emit, version-fingerprinted)

## Goal

Make mindspec report its own friction so the tool can improve itself.
A v1 install must capture the highest-signal "the tool forced the wrong
action" moments — escape-hatch / override / `mindspec repair` events that
SUCCEEDED — as an always-on, redacted session journal, let the owner
consolidate that journal into a redacted, version-stamped, fingerprinted
**local friction report held in a dedicated, NON-SYNCED store** (NOT a
bead in the beads tracker — see Req 4 / HC-3), and triage those reports.
The motivating evidence (the 2026-06 autopilot run) is
entirely owner-side: `--override-adr`-with-committed-claim fired on
EVERY complete; `MINDSPEC_ALLOW_MAIN` was needed 3x; silent close-leg
recurred ~3x; none of these self-reported — a human/orchestrator had to
notice. v1 closes the minimum loop for exactly those signals.

v1 is deliberately **OWNER-LOCAL only**: capture → redact → local
friction report (dedicated non-synced store, NOT a bead) → triage, plus
the version-fingerprint mechanism that lets a later report be classified
as a regression or stale against a recorded fix. The public GitHub
draft-and-submit channel, cross-install dedup search, the owner
cross-install remote push, and the layer-3 subjective agent-instruction
are all explicitly deferred to a follow-on spec (§Non-Goals). This scope
follows the panel verdict: r3 (APPROACH_SOUND) and r1 (NEEDS_REVISION,
`mvp_v1`) both converge on "redaction lib + report-to-local-store +
escape-hatch self-emit first; defer the rest."

Three load-bearing design choices, each adopted directly from the panel:

1. **Privacy is the gating requirement, not a feature.** The redaction
   library ships FIRST, with adversarial fixtures derived from REAL
   mindspec error/recovery output, and its golden-corpus test must pass
   in CI before any capture or report code that could emit collected
   data merges (HC-1). Defense is **structured-enum-fields-first**: every
   collected string is tainted-by-default; the human review step is a
   BACKSTOP, never the primary defense (HC-2). The panel (r2,
   CHANGES_REQUIRED) demonstrated — not hypothesised — that mindspec's
   "own controlled strings" are templates interpolated with user data
   that leak spec slugs, bead ids, branch names, relative paths,
   OWNERSHIP domain names, and `%w`-wrapped bead descriptions.

2. **Capture belongs IN mindspec, not in a hook — on the SUCCESS path.**
   `PersistentPostRunE` (`cmd/mindspec/root.go:67`) runs ONLY when a
   command's `RunE` returns nil (success) and receives NO error and NO
   exit code; gate-blocked / failed commands `os.Exit(1)` inside `RunE`
   (`internal/complete/complete.go:58`/`:117` and ~12 other cmd files)
   BEFORE the hook runs, so it can capture neither a failure nor an exit
   code. v1 therefore captures the **success-path admission** that the
   tool forced the wrong action: an escape-hatch / override flag that was
   USED on a leaf command that nonetheless SUCCEEDED (read via
   `cmd.Flags().Changed` — `--override-adr` / `--allow-doc-skew` /
   `--supersede-adr`), or a completed `repair phase`. (The
   `MINDSPEC_ALLOW_MAIN` env var is DELIBERATELY NOT a v1 captured signal —
   it is a raw-git bypass consumed in the hook-dispatch path that never runs
   a capturable leaf, and an ambient `os.Getenv` check would fire a FALSE
   friction event on every command in any shell that exported it; see Req 2
   and ADR-0038 §2.) This is NOT a PostToolUse hook (fragile
   in compound commands, leaks the raw command/`file_path`) and NOT the
   OTEL emitter (opt-in / noop by default per ADR-0027) — see r3
   `capture_path`. Capturing FAILED / gate-blocked runs is a different
   mechanism (wrapped `RunE` / routing the `os.Exit` paths) and is OUT of
   v1 scope (§Non-Goals).

3. **The loop closes on version + fingerprint.** Every signal carries
   `mindspec --version` and a canonical FINGERPRINT =
   `hash(command + which-escape-hatch [+ subcommand])` — built ONLY from
   mindspec's own closed-set structured tokens, NEVER the override REASON
   or any user-supplied value. (There is no general typed-error taxonomy
   today, and the success-capture path sees no error at all, so
   error-class is deliberately NOT a fingerprint input in v1 — see
   Req 3.) Resolving a friction report records `resolved_in_vX`; a later
   report at version ≥ X is a REGRESSION, at version < X is stale.
   Without this the corpus is collect-only telemetry (r1's central
   finding).

## Background

The 2026-06 autopilot run surfaced repeated, structurally-invisible
friction: overrides forced on every `complete`, `MINDSPEC_ALLOW_MAIN`
needed repeatedly, a recurring silent close-leg, install-script blockers.
None self-reported. The original sot1 design proposed a 3-layer,
dual-channel system (public GitHub draft-and-submit + owner Dolt; a
PostToolUse+Stop hook journal; a managed-CLAUDE.md subjective trigger).

A 3-reviewer design panel revised that scope:

- **r1-soundness (NEEDS_REVISION):** the design built only the INGEST
  half of a loop it called "self-improving"; the loop closes only with
  version-aware attribution (fingerprint + `resolved_in_vX` +
  regression/stale). `mvp_v1`: build the journal + owner channel +
  the minimum back-half (a `report list` triage view) ONLY; defer public
  draft-and-submit, layer-3, and cross-install GitHub dedup.
- **r2-security (CHANGES_REQUIRED):** the "recovery line = mindspec's own
  controlled strings, safe to ship" premise is FALSE. Demonstrated leaks
  from current code (anchors re-verified at draft time, with line drift
  noted): `internal/next/guard.go` recovery (`:176` ClaimFailure wrap,
  `:180`/`:210` `git -C <SpecWorktreePath> worktree add <beadWorktreeRel>
  -b bead/<id> <specBranch>`) ships abs paths + branch names + bead ids +
  spec slugs; `internal/validate/divergence.go:186`
  (`file %s is not claimed by any OWNERSHIP.yaml ... impacted domains
  %v`) ships RELATIVE source paths + OWNERSHIP domain names, folded into
  the guard failure via `adrDivergenceFailure` /
  `joinResultErrorMessages` (`internal/complete/complete.go:340`, `:612`,
  `:650`); `%w` chains (the design's own Dolt-1105-on-large-descriptions
  case) drag bead descriptions into the error text. The journal/hook
  source data (`internal/hook/hook.go:122-133`) IS the raw Bash
  `command` + `file_path` — so redaction must run at JOURNAL-WRITE time,
  not just at draft time.
- **r3-alternatives (APPROACH_SOUND):** the trust-first instinct is
  right; the changes are scope/sequencing/capture-mechanism. Center the
  whole feature on escape-hatch/override telemetry (an override flag is a
  literal admission the tool forced the wrong action — highest signal,
  lowest noise); do NOT auto-journal every non-zero exit (most are
  correct blocks = noise). Capture via mindspec self-emit in
  `PersistentPostRunE`, not a hook. Sequence bead 1 as the redaction lib
  + adversarial fixtures, gated before any code that consumes it.

This spec encodes the panel-revised v1. It adds two new top-level
subcommands (`mindspec report`, `mindspec report list`), a self-emit
call in `PersistentPostRunE`, a new redaction library, and a new
session-journal store. It adds no public-network egress and no
cross-install push.

## Impacted Domains

- **core**: the new redaction library, the session-journal store, the
  fingerprint/version stamping, and the feedback-remote config contract
  (a new global/user-scoped config surface). These are the privacy- and
  loop-critical pure-ish components and are claimed by the core domain.
- **execution**: the CLI command surface (`cmd/mindspec`, per the
  spec-091/092 precedent for command-layer changes) gains the `mindspec
  report` and `mindspec report list` subcommands and the self-emit call
  in `PersistentPostRunE` (`cmd/mindspec/root.go`).
- **workflow**: the escape-hatch / override / `mindspec repair` event
  taxonomy is the friction SOURCE, and the recovery/error STRINGS the
  redaction golden-corpus is built from are produced here
  (`internal/next/guard.go`, `internal/complete/complete.go`,
  `internal/validate/divergence.go`). No behavior in these files
  changes; they are read to build the adversarial fixtures.

## Affected packages (per domain)

- **`internal/redact` (NEW, core)** — the redaction library: structured
  enum field allowlist, tainted-by-default scrub passes (abs paths,
  relative/`*.go`-shaped tokens, spec slugs, bead ids, branch names,
  OWNERSHIP domain names, file names, secrets), an entropy catch-all for
  long hex/base64 runs, `%w`-chain unwrap-or-whole-chain-scrub + length
  caps, and the canonical fingerprint helper. Ships with the adversarial
  golden corpus (Req 1).
- **`internal/journal` (NEW, core)** — the always-on session-journal
  store AND the consolidated friction-report store, both living in a
  DEDICATED, NON-SYNCED local state dir (NOT under any project/git tree,
  NOT the beads DB, NOT swept by `bd`/`dolt push` — see Req 4 / HC-3).
  Files are created `0600` (non-world-readable, M2). Append API that
  scrubs at write time, per-fingerprint occurrence count + cap, version
  stamping, read API for consolidation and `report list`. Stores only
  `basename(argv[0])` + the escape-hatch enum token(s) + fingerprint +
  count + version, never a raw command string and never a flag VALUE
  (r2 medium-severity fixes — no unredacted command, no home-dir invocation
  path, and no user-supplied override reason ever persisted to disk).
- **`cmd/mindspec/root.go` (execution)** — `PersistentPostRunE` (`:67`)
  gains the self-emit call on the SUCCESS path only: when a leaf command
  that SUCCEEDED used an escape-hatch/override flag (`cmd.Flags().Changed`
  — `--override-adr`/`--allow-doc-skew`/`--supersede-adr`) or is a
  completed `repair phase`, it appends a redacted journal entry (Req 2).
  (`MINDSPEC_ALLOW_MAIN` is NOT bound — see Req 2 / ADR-0038 §2.)
  Failed/gate-blocked runs
  `os.Exit` before this hook and are out of scope. The opt-in
  `--trace`/`MINDSPEC_TRACE` path (`:53-56`) is untouched — the journal
  is a separate, always-on, redacted sink (ADR-0027).
- **`cmd/mindspec/report.go` (NEW, execution)** — `mindspec report`
  (consolidate journal → redacted friction report in the dedicated
  non-synced store, Req 4) and `mindspec report list` (triage view over
  that store + mark `resolved_in_vX`, Req 5). Neither command writes to
  the beads tracker or pushes anything.
- **`internal/config` (core)** — the feedback-remote config contract:
  global/user-scoped only, fail-closed, never honored from a
  project-committed file (Req 6 / HC-3).
- **`internal/next/guard.go`, `internal/complete/complete.go`,
  `internal/validate/divergence.go` (workflow)** — READ-ONLY: their
  recovery/error templates are the source of the adversarial fixtures
  (Req 1). No behavior change.

## ADR Touchpoints

- **NEW ADR (proposed): owner-local friction self-improvement loop** —
  number claimed at implementation time per the standard next-free-number
  procedure (highest existing ADR at draft time is
  `ADR-0037-panel-gate-enforced-contract.md`; not pinned here — the ADR
  lane may have concurrent in-flight claims). Records: (a) the
  **privacy-first redaction architecture** — structured-enum-fields-first,
  tainted-by-default, the scrub-category list (relative paths +
  identifiers, not just abs paths/secrets), the entropy catch-all, the
  `%w`-chain rule, and human-review-as-backstop; (b) the **self-emit
  capture decision** — capture in `PersistentPostRunE` on the SUCCESS
  path (it runs only when `RunE` returns nil and sees no exit code;
  failed/gate-blocked commands `os.Exit` before it), reading escape-hatch
  flags via `cmd.Flags().Changed`
  (`--override-adr`/`--allow-doc-skew`/`--supersede-adr`) + a completed
  `repair phase` (`MINDSPEC_ALLOW_MAIN` is NOT bound — ADR-0038 §2); NOT a
  PostToolUse hook (fragile/leaky) and NOT OTEL
  (opt-in/noop, ADR-0027); capturing FAILED runs is deferred to a
  different mechanism (§Non-Goals); (c) the
  **escape-hatch-centered signal taxonomy** — overrides/`repair` are the
  high-signal events, blanket non-zero-exit capture is rejected as noise;
  (d) the **fingerprint + version loop-closing scheme** —
  `hash(command + which-escape-hatch [+ subcommand])` (NO error-class —
  none is available on the success-capture path and no typed-error
  taxonomy exists; NO override reason / user value), `resolved_in_vX`,
  regression-vs-stale; (e) the **capability-based fail-closed identity
  model** — the owner/remote path is gated by the machine-global
  feedback-remote push credential, config is global/user-scoped only, no
  silent fallback; (f) the **untrusted-corpus stance**; and (g) the
  **v1 owner-local scope** with the explicitly deferred channels
  (§Non-Goals). Panel to confirm ADR vs doc-section home at gate time.
- **[ADR-0027-mindspec-otel-only.md](../../adr/ADR-0027-mindspec-otel-only.md)**
  — cited because this spec deliberately does NOT reuse the OTEL emitter
  for capture. Per r3's correction, the trace emitter defaults to
  `noopTracer{}` and `trace.Init` fires only under `MINDSPEC_TRACE` /
  `--trace` (`cmd/mindspec/root.go:53-56`), so "OTEL already feeds the
  journal" is false; the always-on redacted journal is a net-new,
  separate sink. No ADR-0027 behavior changes.
- **[ADR-0035-agent-error-contract.md](../../adr/ADR-0035-agent-error-contract.md)**
  — cited because the recovery/error strings the redaction corpus must
  neutralise are produced by the `guard.FormatFailure` recovery-line
  convention this ADR governs (e.g. `internal/next/guard.go`,
  `internal/complete/complete.go`). The convention is the reason those
  strings are templates interpolated with user data; this spec adds no
  new error contract, it builds adversarial fixtures FROM the existing
  one. No ADR-0035 behavior changes.
- **[ADR-0023.md](../../adr/ADR-0023.md)** (beads as single state
  authority) — REINFORCED, not bent: the friction journal AND the
  consolidated friction reports are **review/diagnostic artifacts** in a
  dedicated, non-synced local store (precedent: spec-093's `panel.json`),
  and `mindspec report` writes NOTHING to the beads tracker. The friction
  store is fully ISOLATED from the beads DB (`.beads/issues.jsonl`) and
  from the `bd`/`dolt push` egress path (HC-3 / Req 4), so no friction
  datum is ever swept into the shared remote. bd statuses remain the
  single workflow-state authority; this spec adds no new bead-tracked
  state.

## Requirements

### Hard Constraints

- **HC-1 Privacy is the gating requirement.** The redaction library
  (Req 1) and its adversarial golden-corpus test land in bead 1 and pass
  in CI BEFORE any capture (Req 2) or report (Req 4) code that could
  emit collected data merges. No friction datum is ever written to the
  journal or to a friction report except through the redaction library.
  The golden-corpus test is a CI gate, not an advisory.
- **HC-2 Structured-enum-fields-first; tainted-by-default;
  review-as-backstop.** The primary defense is collecting only structured
  enum fields (command/subcommand position, WHICH escape-hatch flag was
  set as a closed-set token/boolean, version, OS) — NOT free-text
  scrubbing as the primary mechanism. **Allowlist/value boundary (M4):**
  the structured allowlist may hold ONLY closed-set, mindspec-emitted
  tokens — the command/subcommand name, the flag NAME (as an enum, or a
  boolean "was-set"), the version, the OS. It NEVER holds a value copied
  from argv/env/user input: a flag's VALUE (an override reason, a path
  arg, an ADR id, a glob), the env var's VALUE, or `argv[0]`'s invocation
  path are all TAINTED — excluded from the allowlist and either dropped
  or passed through the full scrub. Every collected STRING is treated as
  tainted and passes the full scrub. Any human review step is a backstop,
  never load-bearing.
- **HC-3 Fail-closed identity; config scope; store isolation.** The
  owner/remote path is gated by the machine-global feedback-remote push
  credential. Absent the credential, `mindspec report` NEVER attempts a
  push and NEVER falls back to "push anyway" or a wrong remote. The
  feedback-remote config is global/user-scoped ONLY — a project-committed
  feedback-remote config is ignored, never honored (a committed remote
  leaks the URL even though creds gate the push). **Store isolation
  (egress-proof):** the journal AND the consolidated friction reports
  live in a dedicated, non-synced local state dir and are NEVER written
  to the beads tracker (`.beads/issues.jsonl` is git-tracked and swept by
  the mandatory session-completion `bd dolt push`, AGENTS.md:83). The
  friction store is isolated from the beads DB and from the `bd`/`dolt
  push` path, so a redaction MISS can never egress to the shared remote
  unreviewed. In v1 (local friction store only) this manifests as: with
  no credential configured — the default — `report` writes to the local
  store and attempts no egress of any kind.
- **HC-4 The friction corpus is UNTRUSTED everywhere it is read.** All
  collected free text is rendered inside fenced code blocks; markdown
  auto-linking is neutralised; every field is length-capped; any
  slot/placeholder VALUE that appears in copy-pasteable recovery text is
  itself escaped/placeholdered (shell-metachars neutralised) so a
  reconstructed recovery line carries no live user value (P3 / Req 7); no
  agent or automation auto-executes a `recovery:` line or acts on
  body/recovery links (Req 7).
- **HC-5 Each commit `go build ./... && go test -short ./...` green.**
- **HC-6 No-human safety (CI / non-interactive).** When no human is
  present (CI / `GITHUB_ACTIONS` / non-interactive detected), the pipeline
  is **local-journal-only**: no friction-report write, no draft, no
  prompt, no push, no agent action. The journal is the sole sink in that
  mode.
- **HC-7 Fail-closed redaction.** If the redaction library errors,
  panics, or cannot CONFIDENTLY classify a field, that field is DROPPED
  (or the whole entry/report is dropped) and the RAW value is NEVER
  persisted to disk, written to the store, or emitted as a fallback.
  There is no raw-string fallback and no error-log of the unredacted
  text. (This is the redaction analog of HC-3's fail-closed identity.)
- **HC-8 At-rest protection; non-committed path.** The session journal
  and the consolidated friction-report store are created `0600`
  (owner-only, non-world-readable) under a dedicated mindspec state dir
  that is NEVER inside a project/committed tree and NEVER swept by
  `bd`/`dolt`. The exact path is a plan-phase decision (§Design
  Questions) but the perms + non-committed + non-synced properties are
  binding here.

### Numbered requirements

1. **Redaction library + adversarial fixtures — gated FIRST (bead 1).**
   A new `internal/redact` package whose primary defense is a
   **structured enum allowlist** (command/subcommand position, WHICH
   escape-hatch flag was set as a closed-set token/boolean,
   `mindspec --version`, OS) — NOT free-text scrubbing as the primary
   mechanism (HC-2). The allowlist holds ONLY these closed-set,
   mindspec-emitted tokens and NEVER a value copied from argv/env/user
   input — a flag VALUE (override reason, path arg, ADR id, glob), an env
   VALUE, or `argv[0]`'s invocation path are TAINTED, not allowlisted
   (HC-2 / M4). `argv[0]` is reduced to `basename` (i.e. `mindspec`) and
   passed through the scrub before any storage (M3) — the verbatim
   home-dir/username invocation path is never persisted. Every collected
   STRING is tainted-by-default and passes the full scrub, which MUST
   cover, beyond absolute paths and secrets:
   - **relative source paths** and `*.go`/path-shaped tokens →
     `<path>` (r2: `divergence.go:186` ships relative paths);
   - **identifiers**: spec slugs, bead ids, branch names
     (`bead/<id>` / `spec/<slug>`), OWNERSHIP domain names, and file
     names → typed placeholders (r2: recovery lines + the adr-divergence
     error ship exactly these, and no abs-path pass catches them);
   - an **entropy catch-all** for long hex / base64 runs (token/secret
     backstop);
   - **`%w`/`%v` error chains**: unwrap to the sentinel error
     class/code and discard the wrapped message, OR run the full
     scrub + entropy pass over the WHOLE chain (not just line 1) and
     length-cap it — never ship a raw wrapped chain (r2: Dolt-1105
     carries bead descriptions).
   The library also exposes the canonical fingerprint helper (Req 3).
   The human review step is a BACKSTOP, never the primary defense
   (HC-2). **Adversarial golden corpus**: fixtures derived from REAL
   mindspec error/recovery output — at minimum `internal/next/guard.go`
   ClaimFailure (`:176`) and the worktree-recovery recipe (`:180`/`:210`),
   `internal/validate/divergence.go:186`'s adr-divergence-unowned message
   as folded by `joinResultErrorMessages` /
   `adrDivergenceFailure` (`internal/complete/complete.go:340`/`:612`/
   `:650`), and a `%w`-wrapped Dolt-1105-style chain carrying a bead
   description — asserting ZERO leakage of paths, slugs, branch names,
   bead ids, domain names, descriptions, or high-entropy tokens in the
   redacted output. This test is the HC-1 CI gate. (Note: because the
   journal/store collect structured enums only — Req 2 — these error
   STRINGS are not themselves journaled in v1; the golden corpus is a
   defense-in-depth BACKSTOP that pins the redaction library against the
   day any free-text field — e.g. a `report` body — does carry them.)

2. **Self-emit capture from `PersistentPostRunE` → an always-on,
   redacted session journal — SUCCESS path only.** `cmd/mindspec/root.go`'s
   `PersistentPostRunE` (`:67`) appends a redacted entry to a new
   `internal/journal` session store on a friction event. **Mechanism
   reality (r3):** cobra runs `PersistentPostRunE` ONLY when a command's
   `RunE` returns nil (success); its signature receives NO error and NO
   exit code, and gate-blocked / failed commands `os.Exit(1)` inside
   `RunE` (`internal/complete/complete.go:58`/`:117` and ~12 other cmd
   files) BEFORE the hook runs — so it can observe neither a failure nor
   "the exit code." v1 therefore captures SUCCESS-PATH events ONLY. The
   plan-phase audit (DQ2/DQ6) resolved the exact bound set to the
   leaf-local override flags + a completed `repair phase`:
   - an **escape-hatch / override flag** that was set on a LEAF command that
     SUCCEEDED — detected via `cmd.Flags().Changed("override-adr")` /
     `Changed("allow-doc-skew")` / `Changed("supersede-adr")` (these are
     leaf-local flags on `complete` / `impl approve` / the hidden
     `approve impl`, NOT root persistent flags);
   - a completed **`repair phase`** invocation (a command that itself
     SUCCEEDED).
   **`MINDSPEC_ALLOW_MAIN` is DELIBERATELY EXCLUDED from v1 capture
   (DQ6 / ADR-0038 §2):** it is a raw-git bypass consumed in the
   hook-dispatch path (`internal/hook/dispatch.go`) that never runs a
   capturable leaf, and an ambient `os.Getenv` check would fire a FALSE
   friction event on every command in any shell that exported it. v1 binds
   NO override env var.
   Capture is **centered on these escape-hatch / `repair` admissions** —
   the literal "the tool forced the wrong action" signals — NOT every
   non-zero exit (most non-zero exits are the tool correctly BLOCKING =
   noise; r3 `over_built_areas`). The event enum is constrained to events
   OBSERVABLE on the success path (a `Changed`-flag, a nil-returning
   `RunE`). Capturing FAILED / gate-blocked commands (which `os.Exit`
   before the hook) needs a DIFFERENT mechanism — a wrapped `RunE` or
   routing the `os.Exit` paths through a deferred emitter — and is OUT of
   v1 scope (§Non-Goals; a future enabler). This is NOT a PostToolUse hook
   (fragile across compound `... && ...` commands, leaks the raw
   `command`/`file_path`, r3 `capture_path`) and NOT the OTEL emitter
   (opt-in / noop by default, ADR-0027). **Redaction runs at journal-WRITE
   time** (HC-1), the entry is written `0600` to the dedicated non-synced
   store (HC-8), and the journal stores only `basename(argv[0])` + the
   escape-hatch enum token(s) + fingerprint + count + version — never a
   raw command string, never `argv[0]`'s full invocation path, and never a
   flag VALUE (r2 medium-severity fixes; M3/M4).

3. **Canonical fingerprint + version attribution + regression/stale
   loop-closing.** Every journal entry and every friction report carries
   `mindspec --version` and a canonical FINGERPRINT =
   `hash(command + which-escape-hatch [+ subcommand])` computed from the
   structured enum fields (Req 1's allowlist — mindspec's own controlled
   closed-set values, so the fingerprint is stable and leak-free).
   **Error-class is deliberately NOT a fingerprint input in v1 (r3):**
   there is no general typed-error taxonomy (`guard.NewFailure` is
   free-text; only `validate.Issue.Name` + otel codes are stable), and the
   success-capture path (Req 2) sees no error at all, so error-class would
   be empty/vestigial. The fingerprint also NEVER includes the override
   REASON or any user-supplied flag value (that is tainted user data; M4).
   The fingerprint is the single dedup key (the `reports.jsonl`
   consolidation collapse + future cross-install matching). When a friction
   report is resolved it
   records `resolved_in_vX`; an incoming report with the same fingerprint
   and version ≥ X is a REGRESSION (high-signal, reopen), version < X is
   stale (suppress). This is the mechanism that makes "fewer reports over
   time" observable (r1's central finding). (A typed-error-code taxonomy
   is a FUTURE enabler — needed only if failure-capture is ever added per
   Req 2 — and is explicitly deferred, NOT a v1 bead.)

4. **`mindspec report` → a redacted friction report in a DEDICATED,
   NON-SYNCED local store (NOT a bead).** A new subcommand that
   consolidates the session journal (collapsed by fingerprint, Req 3)
   into a redacted friction report written to a dedicated local store,
   stamped with `mindspec --version` and the fingerprint. **The friction
   store is NOT the beads tracker.** It MUST NOT be written via `bd`,
   MUST NOT live in `.beads/issues.jsonl` (which is git-tracked and swept
   by the mandatory session-completion `bd dolt push`, AGENTS.md:83), and
   MUST NOT be carried by any `bd`/`dolt push`/git path — otherwise a
   redaction MISS would egress to the shared remote unreviewed (r2's
   sharpest hole). Instead it lives in the same dedicated, non-synced
   `0600` state dir as the journal (HC-3 / HC-8), isolated from the beads
   DB and the dolt push path. The journal is the source of truth and the
   consolidation point — Req 2's self-emit WRITES the journal; `report` is
   the dispatch point (r1 single-sink architecture). v1 writes to the
   local store only; it attempts no remote push and no bead write (HC-3,
   Non-Goals). In CI / non-interactive, `report` is a no-op beyond the
   journal (HC-6).

5. **`mindspec report list` triage view + mark `resolved_in_vX`.** A thin
   triage view that READS the dedicated friction store (Req 4) — NOT the
   beads tracker — showing fingerprint, command, escape-hatch, occurrence
   count, first/last version seen, and regression/stale status per Req 3,
   plus a way to mark a friction report `resolved_in_vX` (persisted back
   to that same non-synced store). Without this, v1 is collect-only and
   proves nothing (r1 `mvp_v1`).

6. **Feedback-remote config contract — capability-based, fail-closed,
   global-scoped (AUTH / IDENTITY).** Identity is possession of the
   feedback-remote push credential, enforced at the WRITE DESTINATION,
   not at the CLI (you cannot hide a subcommand per-user in a shared
   binary). The feedback-remote config MUST be global/user-scoped only
   and is NEVER honored from a project-committed file (HC-3). The
   owner/remote path fails CLOSED without the credential: no silent
   fallback to "push anyway" or a wrong remote. **v1 writes to the
   dedicated non-synced friction store only** (Req 4 — never the beads
   tracker, never a push), so the actual cross-install remote push is
   deferred (§Non-Goals); this requirement specifies the config-scope +
   fail-closed contract NOW so the follow-on cannot regress it, and so a
   non-owner running `mindspec report` in v1 simply writes a harmless
   report to THEIR OWN local, non-synced store that never reaches the
   owner and never enters any git/dolt egress path.

7. **Untrusted-corpus consumption rules (HC-4).** Everywhere the friction
   corpus is read (the friction-report body, `report list`, and any
   future channel): render all collected free text inside fenced code
   blocks; neutralise markdown auto-linking; length-cap every field; and
   guarantee no agent/automation auto-executes a `recovery:` line or acts
   on body/recovery links. **Slot escaping (P3 / r2):** any
   slot/placeholder value that appears in copy-pasteable recovery text
   MUST be escaped/placeholdered (shell-metachars neutralised, or replaced
   by a typed placeholder) so a reader who copies a recovery line cannot
   execute a smuggled user value. Document that any triage agent treats
   the corpus as untrusted input (r2 `injection`).

8. **Friction-storm cap.** One broken state firing N times must not
   bloat the store. The journal is APPEND-ONLY (one redacted event per
   line, NO count field, NO within-session collapse-to-one-entry); the
   per-fingerprint-per-session cap is enforced by DROPPING excess appends
   (within one process invocation, at most the named cap of lines are
   appended for a given fingerprint, further appends dropped) so a runaway
   loop cannot bloat the journal. The occurrence COUNT is DERIVED only at
   consolidation, on the `reports.jsonl` view (`count` = number of journal
   lines for that fingerprint) — it lives on the consolidated report, never
   on a journal record (ADR-0038 §5; r1 edge: friction storm).

9. **Bootstrap-paradox documentation.** Install-failure friction (the
   motivating evidence's biggest class — `install.sh` / `install.ps1`
   were total blockers) is structurally UNREPORTABLE by an in-tool
   reporter: if install fails, mindspec is not present to self-report.
   v1 does NOT pretend to cover it. The spec/ADR documents the
   out-of-band path (e.g. a separate installer-emitted failure signal or
   a manual report) as the acknowledged, deferred home for this class —
   it is explicitly out of v1's in-tool scope (r1 bootstrap paradox).

## Scope

### In Scope
- Requirements 1-9; Hard Constraints HC-1..HC-8.
- The new `internal/redact` and `internal/journal` packages (the journal
  AND the dedicated non-synced friction-report store); the
  success-path `PersistentPostRunE` self-emit call; the `mindspec report`
  and `mindspec report list` subcommands; the feedback-remote
  config-scope + fail-closed contract.
- Unit tests for every behavior change; the adversarial redaction
  golden-corpus CI gate (Req 1 / HC-1); fingerprint determinism +
  regression/stale tests; the journal storm-cap test; the
  fail-closed/config-scope tests; the store-isolation test (a friction
  report never appears in the beads DB / `bd dolt push` payload); the
  fail-closed-redaction (HC-7) and `0600`/non-committed-path (HC-8) tests.

### Out of Scope
- The public GitHub draft-and-submit channel (prefilled-URL,
  structured-only payload, the >6KB temp-file/clipboard fallback, the
  first-run consent disclosure). The design is captured in sot1 for the
  follow-on; v1 ships no public-network egress.
- Cross-install dedup-search (an undisclosed outbound GitHub call;
  rate-limited at scale — r2 low-severity, r1 offline edge).
- The owner cross-install REMOTE PUSH to a shared feedback DB. v1 writes
  to the dedicated non-synced local store only (HC-3, Req 4/6). The
  config-scope + fail-closed contract is in scope; the push itself is not.
- **Capturing FAILED / gate-blocked commands** (those that `os.Exit`
  inside `RunE` before `PersistentPostRunE` runs). v1 captures the
  SUCCESS path only (Req 2); failure-capture needs a different mechanism
  (wrapped `RunE` / routing the `os.Exit` paths) and is deferred.
- The layer-3 managed-CLAUDE.md / instruct subjective agent-instruction
  (un-mechanizable judgment layer; r1/r3 defer it).
- A claim-less / install-script self-report path (bootstrap paradox,
  Req 9 documents it as out-of-band).

## Non-Goals

- v1 does NOT aggregate friction across installs and does NOT call any
  network service. It is owner-local capture + triage only.
- v1 does NOT build the public draft-and-submit channel, the
  cross-install dedup search, the owner remote push, or the layer-3
  subjective trigger — all deferred to a follow-on spec (the panel's
  `mvp_v1` / `over_built_areas` scope cut). The fingerprint + version
  mechanism (Req 3) is built now precisely so those later channels plug
  into a loop that already closes.
- v1 does NOT relabel itself "friction telemetry": the back-half
  (fingerprint + `resolved_in_vX` + regression/stale + `report list`
  triage) is in scope, which is what makes it a loop and not a
  collect-only sink (r1).
- v1 does NOT auto-journal every non-zero exit; blanket exit-code capture
  is rejected as noise (r3). Capture is escape-hatch/override/`repair`
  centered.
- v1 does NOT capture FAILED / gate-blocked commands. `PersistentPostRunE`
  runs only on `RunE` success and never after an `os.Exit`, so
  failure-path friction is structurally uncapturable by the v1 mechanism;
  it needs a wrapped `RunE` / `os.Exit`-routing emitter and is deferred
  (a future enabler, which a typed-error-code taxonomy would also serve).
- v1 does NOT make the journal OR the friction reports beads-tracked
  entities, and does NOT write them through `bd` (ADR-0023: they are
  diagnostic artifacts in a dedicated non-synced store; bd statuses remain
  the single workflow-state authority, and `.beads/issues.jsonl` / the
  `bd dolt push` egress path never carries a friction datum).
- **v1 does NOT detect friction classes that are not escape-hatch-shaped
  or not mechanically observable on the success path** — specifically
  *silent-wrong-behavior* (exit 0 but the tool did the wrong thing),
  *latency / timeout*, *abandonment* (the human gives up mid-flow), and
  *doc-discovery* friction. RATIONALE: none of these emits a closed-set
  override/`repair` signal the success-path hook can read, and inferring
  them needs heuristics/outcome-labelling out of v1 scope; they are
  explicitly deferred, not overlooked (r1).
- v1 does NOT claim to cover install/bootstrap-failure friction in-tool
  (Req 9).

## Acceptance Criteria

### Redaction (Req 1 / HC-1)
- [ ] The adversarial golden-corpus test feeds REAL mindspec
  error/recovery strings — `next/guard.go` ClaimFailure (`:176`) and the
  `git -C <SpecWorktreePath> worktree add` recovery recipe
  (`:180`/`:210`), the `divergence.go:186` adr-divergence-unowned message
  as folded by `joinResultErrorMessages`/`adrDivergenceFailure`
  (`complete.go:340`/`:612`/`:650`), and a `%w`-wrapped
  Dolt-1105-style chain carrying a bead description — and asserts the
  redacted output contains ZERO of: absolute or relative paths, `*.go`
  tokens, spec slugs, branch names, bead ids, OWNERSHIP domain names,
  bead descriptions, or high-entropy hex/base64 runs.
- [ ] The structured enum allowlist is the primary path: a fixture with
  ONLY enum fields (no free text) redacts to itself unchanged; a fixture
  with free text passes the full scrub.
- [ ] The redaction test is wired as a CI gate that fails the build on
  any leakage (HC-1).

### Self-emit + journal (Req 2 / Req 8 / HC-7 / HC-8)
- [ ] A SUCCESS-path override/escape-hatch event (a LEAF command that
  returned nil `RunE` with `--override-adr` / `--allow-doc-skew` /
  `--supersede-adr` set, or a completed `repair phase`) appends exactly one
  redacted journal entry from `PersistentPostRunE`; a plain non-zero exit /
  gate-blocked run that `os.Exit`es appends NONE (it is structurally
  uncapturable, not merely filtered). `MINDSPEC_ALLOW_MAIN` is NOT a v1
  captured signal (DQ6 / ADR-0038 §2), so an `MINDSPEC_ALLOW_MAIN`-set
  command appends nothing on that basis alone.
- [ ] The persisted journal entry contains `basename(argv[0])` + the
  escape-hatch enum token(s) + fingerprint + count + version, and NO raw
  command string, NO `file_path`, NO `argv[0]` full path, and NO flag
  VALUE (grep the on-disk journal for a planted home-dir path / secret /
  override-reason string — absent).
- [ ] **Fail-closed redaction (HC-7):** a redaction-failure fixture (the
  redactor errors/cannot classify a field) yields NO on-disk entry for
  that field and NEVER the raw string — the datum is dropped, not
  fallen-back.
- [ ] **At-rest (HC-8):** the journal and friction-report store files are
  created `0600` (a perms assertion) under a non-project, non-`bd`/`dolt`
  state dir; a grep of the on-disk file for a planted absolute home-dir
  path returns absent.
- [ ] A storm (same fingerprint fired N times in a session) appends
  append-only journal lines capped at the per-fingerprint-per-session limit
  L (firing L+1 times → exactly L lines on disk; excess appends DROPPED,
  not collapsed into a count field on a record). The occurrence `count` is
  DERIVED later at `reports.jsonl` consolidation, not stored on the journal
  (Req 8 / ADR-0038 §5).
- [ ] The opt-in `--trace`/`MINDSPEC_TRACE` path is unaffected; the
  journal is written regardless of trace state (always-on) but only on a
  success-path friction event.

### Fingerprint + version loop (Req 3 / Req 5)
- [ ] `fingerprint(command, which-escape-hatch [, subcommand])` is
  deterministic and identical across two runs of the same friction event;
  differs when any input differs. The fingerprint contains NO error-class
  and NO override reason / user value (assert the same override flag with
  two different reason VALUES produces the SAME fingerprint).
- [ ] A friction report marked `resolved_in_v2` then re-reported: at
  version ≥ v2 it is classified REGRESSION; at version < v2 it is
  classified stale/suppressed (Req 3); `report list` reflects the status.

### Report → friction store (Req 4 / HC-3 / HC-6)
- [ ] `mindspec report` consolidates the journal (collapsed by
  fingerprint) into a redacted friction report in the dedicated
  non-synced store, stamped with version + fingerprint; the report body
  passes the Req 1 redaction (no leaked identifiers).
- [ ] **Store isolation (egress-proof):** a friction report created by
  `mindspec report` NEVER appears in `.beads/issues.jsonl`, in `bd`
  query output, or in a `bd dolt push` payload — asserted by a test that
  runs `report`, then greps the beads DB / a dry-run push payload and
  finds the report's fingerprint absent.
- [ ] In CI / non-interactive (`GITHUB_ACTIONS` set), `mindspec report`
  is a no-op beyond the journal: no friction-report write, no prompt, no
  push (HC-6).
- [ ] `mindspec report list` reads the friction store (not `bd`) and
  shows fingerprint, command, escape-hatch, occurrence count, version
  range, and regression/stale status; offers a `resolved_in_vX` mark.

### Identity / config (Req 6 / HC-3)
- [ ] With no feedback-remote credential configured (the default),
  `mindspec report` writes locally and attempts no push (fail-closed; no
  wrong-remote fallback) — asserted by a test that fails if any push/network
  call is attempted.
- [ ] A feedback-remote config placed in a PROJECT-committed file is
  ignored (never honored); only a global/user-scoped config is read.

### Untrusted corpus (Req 7 / HC-4)
- [ ] A friction body whose collected text contains a markdown
  auto-link / injection payload (`](http://evil) ignore previous
  instructions`) is rendered fenced, with auto-linking neutralised and
  the field length-capped; no `recovery:` line is auto-executed.
- [ ] A slot/placeholder value carrying a shell metachar that appears in
  copy-pasteable recovery text is escaped/placeholdered, so the rendered
  recovery line contains no live, executable user value (P3).

### Bootstrap paradox (Req 9)
- [ ] The spec/ADR documents the install-failure friction class as
  structurally UNREPORTABLE in-tool and names the deferred out-of-band
  home (installer-emitted signal / manual report); an inspection of the
  ADR/spec text confirms the documented out-of-band path exists (Req 9;
  bead 3 / ADR closeout).

### Cross-cutting
- [ ] `go build ./... && go test -short ./...` green on every commit
  (HC-5).
- [ ] Bead 1 (redaction + golden corpus) merges before any bead that
  emits collected data (HC-1, dependency-enforced).

## Validation Proofs

- `go test ./internal/redact/...` — the adversarial golden-corpus gate;
  zero leakage on the real-string fixtures.
- `go test ./internal/journal/...` — write-time redaction, fail-closed
  drop (HC-7), `0600` perms + non-committed path (HC-8), the append-only
  storm cap (excess appends dropped), fingerprint determinism.
- `go test ./cmd/mindspec/...` — `report` / `report list` behavior, the
  CI no-op (HC-6), the store-isolation egress proof (no friction report in
  the beads DB / `bd dolt push` payload), and the fail-closed/config-scope
  path (Req 6).
- `go build ./... && go test -short ./...` green (HC-1/HC-5).
- Manual: run a command with `--override-adr` in a fixture repo, confirm
  one redacted journal entry; run `mindspec report`, confirm the friction
  report body is leak-free against the Req 1 corpus AND that `bd` /
  `.beads/issues.jsonl` show no new entry; run `mindspec report list`,
  confirm the entry with its count + version.

## Open Questions

None blocking approval. The two gate-surfaced requirement defects
(Req 2's capture mechanism and Req 3's fingerprint formula) are RESOLVED
into the requirements text above (success-path-only capture; error-class
dropped from the fingerprint), and the friction-store location/perms are
now settled into HC-3/HC-8/Req 4 (dedicated, non-synced, `0600`,
never-`bd`/`dolt`). The remaining genuinely-deferred items — all
plan-phase, none approve-blocking — are tracked under §Design Questions
below.

## Design Questions (for the panel)

Draft positions are BINDING for planning unless the panel explicitly
overrides them — a plan agent decomposes against the stated position, not
the open question. The location/perms and the capture/fingerprint
mechanisms are RESOLVED into the requirements (see §Open Questions) and
are intentionally NOT relitigated here.

1. **Journal/store format + retention.** The PATH class is fixed
   (dedicated, non-synced, `0600`, non-committed — HC-8). Still open: the
   on-disk FORMAT (JSONL vs other), whether the journal is per-session or
   one appended file, and any rotation/retention policy for the unbounded
   append. Draft position: append-only JSONL under the state dir, with a
   retention cap deferred to the follow-on (a stale redacted entry is low
   risk given fail-closed redaction + `0600`).
2. **Exact success-path event enum.** The precise set of escape-hatch
   flags / override env vars / `repair` forms to treat as friction
   signals — a plan-phase audit of `cmd/mindspec/root.go`'s persistent
   flags + env vars, CONSTRAINED to events observable on the success path
   (a `cmd.Flags().Changed` flag, an `os.Getenv`, a nil-returning
   `RunE`). Draft position: start from the Req 2 candidate set
   (`--override-adr`, `--allow-doc-skew`, `--supersede-adr`,
   `MINDSPEC_ALLOW_MAIN`, completed `repair`) and bind only those the
   audit confirms are leaf-readable in the hook. **RESOLVED (DQ6 /
   ADR-0038 §2):** the audit bound exactly the three leaf-local override
   flags + a completed `repair phase`; `MINDSPEC_ALLOW_MAIN` was EXCLUDED
   (a raw-git bypass that never runs a capturable leaf; an ambient getenv
   would mis-fire on every command).
3. **Friction-report write ceremony.** Whether `mindspec report` writes
   the report to the dedicated non-synced store silently or stages a draft
   the human confirms. Note the store NEVER leaves the machine and never
   enters `bd`/`dolt` (Req 4), and HC-6 already forces journal-only in CI.
   Draft position: silent local write is acceptable (nothing egresses);
   no interactive confirm required in v1.
4. **`resolved_in_vX` storage + regression computation.** Whether the
   resolved version lives on the friction-report record vs a separate
   index in the store, and how `report list` computes
   regression-vs-stale across versions (Req 3/5). Draft position: store
   it on the report record keyed by fingerprint; compute regression/stale
   at `report list` time.
5. **Fingerprint stability across versions.** If the command set changes
   between mindspec versions, the fingerprint (now `command +
   which-escape-hatch [+ subcommand]`, no error-class) may drift and
   break regression detection. Draft position: the inputs are mindspec's
   own closed-set tokens, so drift is bounded to renamed commands/flags; a
   normalization/version map is deferred to the follow-on if observed.

## Proposed bead decomposition (dependency order)

The intended bead order (matching the panel's `mvp_sequencing` /
`mvp_v1`): redaction first, then capture, then report.

| Bead | Title | Depends on | Notes |
|:-----|:------|:-----------|:------|
| 1 | Redaction library + adversarial golden corpus (Req 1, HC-1/HC-2): `internal/redact` structured-enum allowlist + tainted scrub passes (rel paths, slugs, bead ids, branch names, domain names, file names) + entropy catch-all + `%w`-chain rule + canonical fingerprint helper + the real-string golden corpus CI gate | — | **Lands FIRST.** Privacy is the gating requirement; nothing that emits collected data merges before this passes CI. De-risks the privacy core before any sink exists (r3 `mvp_sequencing`). |
| 2 | Self-emit capture + session journal (Req 2 SUCCESS-path capture, Req 3 fingerprint/version, Req 8 storm cap, HC-1/HC-6/HC-7/HC-8): `internal/journal` write-time-redacted, `0600`, non-synced store + `PersistentPostRunE` self-emit on the SUCCESS path centered on escape-hatch/override/`repair` events (`cmd.Flags().Changed` on `--override-adr`/`--allow-doc-skew`/`--supersede-adr` + completed `repair phase`; `MINDSPEC_ALLOW_MAIN` NOT bound, DQ6); `hash(command + which-escape-hatch [+ subcommand])` fingerprint (no error-class) + version stamping; append-only journal with a per-fingerprint-per-session storm cap (excess appends dropped, no record count field); fail-closed redaction | 1 | Capture in mindspec, not a hook; not OTEL (ADR-0027). Stores `basename(argv[0])` + escape-hatch enum only — no error-class, no raw command, no `argv[0]` path, no flag value. Failed/`os.Exit` runs are out of scope. |
| 3 | `mindspec report` → redacted friction report in a DEDICATED, NON-SYNCED store (NOT a bead) + `report list` triage + `resolved_in_vX` + bootstrap-paradox doc/ADR closeout (Req 4, Req 5, Req 6 fail-closed/config-scope, Req 7 untrusted-corpus + slot-escaping, Req 9 bootstrap doc, regression/stale of Req 3, HC-3/HC-4/HC-6/HC-7/HC-8) | 1, 2 | Closes the minimum loop: consolidate journal → friction report in the non-synced store → triage. The store is isolated from the beads DB + `bd dolt push` (no egress). v1 local-only; the feedback-remote config contract is fail-closed + global-scoped. Folds in Req 9's bootstrap-paradox ADR/spec note (P1). |

### Sequencing risks

- **Bead 1 must merge before 2 and 3** — privacy is a hard requirement;
  the golden-corpus gate (HC-1) must be green before any code can write a
  journal entry or a friction report from collected data.
- **Bead 2's redaction runs at journal-WRITE time, not draft time** — a
  journal persisted before scrub would leave an unredacted raw command on
  disk (r2 medium-severity). The store must scrub on append, fail closed
  (HC-7), and write `0600` to the non-synced dir (HC-8).
- **Bead 3's friction store must stay isolated from the beads DB** —
  writing the report via `bd` / into `.beads/issues.jsonl` would let the
  mandatory `bd dolt push` egress a redaction miss to the shared remote
  unreviewed (r2's sharpest hole). The store is the dedicated non-synced
  dir, never `bd`/`dolt`.
- **Capture-event noise** — centering on escape-hatch/override/`repair`
  (not blanket non-zero exit) is load-bearing; over-capturing makes the
  corpus noise and the loop unobservable (r3).

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-06-12
- **Notes**: Approved via mindspec approve spec