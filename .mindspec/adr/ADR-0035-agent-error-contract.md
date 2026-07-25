# ADR-0035: Agent Error Contract — Recovery Lines and Exit Codes

- **Date**: 2026-06-11
- **Status**: Accepted
- **Domain(s)**: workflow, execution, core
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0023](ADR-0023.md) (beads as single state authority), [ADR-0030](ADR-0030-executor-boundary.md) (executor boundary), [ADR-0034](ADR-0034-ceremony-collapse.md) (`mindspec_phase` metadata cache)

---

## Context

Spec 092-agent-contract-hardening collected five field notes where an
agent hit a mindspec guard failure and either stalled, improvised a
destructive workaround (`git stash`, `--no-verify`, raw
`bd update --metadata`), or trusted a misleading exit code. Two defects
were common to all of them:

1. Guard failures described what was wrong but not what to RUN. Each
   agent re-derived the fix, sometimes wrongly. In the worst case an
   agent pasted a raw `bd update <id> --metadata '{...}'`, which
   REPLACES the epic's entire metadata map (`internal/bead/bdcli.go`),
   silently wiping `mindspec_migrated_at`, doc-skew audit keys, and
   ADR-override keys.
2. Exit codes did not reliably encode whether a terminal mutation had
   happened, so agents retried commands that had already mutated state,
   or gave up on commands that had succeeded and merely failed cleanup.

This ADR pins the contract that spec 092 implements (Reqs 12, 19, 21
and HC-4/HC-5) so that future guards and lifecycle commands have a
stable rule to follow and the doc-sync gate has a stable doc target.

## Decision

Two sub-decisions:

1. **Recovery-line convention (Req 12).** Every guard-failure message
   ends with one or more lines of the exact form
   `recovery: <command>` — one copy-pastable command per line, the
   FINAL line of the message always being a recovery line, extractable
   with `grep '^recovery: '`. All guard failures route through the
   shared formatter in `internal/guard` (`guard.FormatFailure` /
   `guard.NewFailure`); new guards added after spec 092 MUST do the
   same. Emitted commands must be safe to paste: no command may carry
   replace/destructive semantics over state the agent did not name. In
   particular, raw `bd update ... --metadata` is banned from ALL
   emitted output (Req 19); callers needing a phase metadata fix emit
   `mindspec repair phase <spec-id>` (merge semantics via
   `bead.MergeMetadata`) instead. The formatter panics on a banned or
   malformed command — a programmer error caught at development time,
   never reachable from user input.

2. **Exit-code contract (HC-4).** Every lifecycle command WITH a
   terminal mutation exits 0 iff that mutation succeeded; commands that
   mutate nothing exit 0 iff they completed their read/guard
   evaluation. Post-terminal cleanup failures warn (with recovery
   commands) but do not change the exit code; pre-terminal gate
   failures exit non-zero having mutated nothing. Two pre-existing
   `impl approve` behaviors violate this contract and are explicitly
   grandfathered out of spec 092's scope: (i) `phase.EnsureMigrated`
   writes migration metadata before any gate when the
   `mindspec_phase` key is absent; (ii) the Spec-086-pinned
   `CommitCount` preflight can fail non-zero after the epic close.
   Neither may be cited as precedent for new code.

## Decision Details

- The machine-greppable prefix is the exported constant
  `guard.RecoveryPrefix` (`"recovery: "`).
- A recovery "command" may be a short prose alternative containing
  commands (e.g. `recovery: close remaining beads with 'mindspec
  complete <bead-id>', or if bead states are already correct run:
  mindspec repair phase <spec-id>`), as long as it stays on one line
  and every embedded command is safe to paste.
- Guard failures additionally name the caller's worktree context where
  location confusion is plausible (`workspace.ContextLine`, Req 8):
  `you are in the <main|spec|bead> worktree (<dir>); this check
  evaluated <checkedPath>`.

## Consequences

### Positive

- Agents recover from guard failures by pasting the emitted command
  instead of improvising; destructive workarounds lose their trigger.
- The convention is forward-enforced, not aspirational: the Req 21
  convention test (`internal/guard/recovery_convention_test.go`) walks
  every exported guard-failure constructor in `internal/guard` and
  fails the build when any produced message lacks a final `recovery:`
  line or emits a banned command.
- Exit codes become a trustworthy signal for retry/abort decisions in
  agent loops.

### Negative / Tradeoffs

- Guard authors must phrase a recovery command even when the fix is
  ambiguous; where genuinely ambiguous, the line offers alternatives.
- The formatter's panic-on-misuse stance means a guard wired up with a
  banned command crashes in development rather than degrading — strict,
  but that is the point of the contract.

## Alternatives Considered

### 1. Grep-based lint instead of a convention test

Rejected: mirrors ADR-0030's reasoning — string greps miss
constructed messages and false-positive on prose. The convention test
invokes the real constructors and checks the real output.

### 2. Formatter returns an error on banned commands

Rejected: the formatter is called on failure paths whose callers would
have to handle a second-order error; misuse is a programmer error, not
a runtime condition, so panic + test coverage is the honest treatment.

### 3. Per-command ad hoc recovery text without a shared helper

Rejected: this is the status quo that produced the field notes —
unparseable, inconsistent, and unenforceable.

## Validation / Rollout

1. Spec 092 Bead 1 lands the formatter, `workspace.ContextLine`, and
   the Req 21 convention test (runs under `go test -short`).
2. Spec 092 Beads 3-8 route every guard touched by Reqs 1-8 through
   the formatter; each converted call site adds a unit test asserting
   `guard.HasFinalRecoveryLine` on its failure message.
3. Spec 092 Bead 9 re-runs the five LLM-harness regression scenarios
   green, demonstrating agents recover via the emitted commands.

<!-- PRE-DRAFT (Spec 127): pre-drafted at plan time per the 122/123/125
amendment-lifecycle precedent; FINALIZED by spec-127 bead 2 (this marker
removed there; wording adjusted only where the concrete implementation
forces it). Reviewable at plan-approve. -->
## Amendment (Spec 127): Guidance non-destructiveness

Every refusal keeps the copy-pastable final recovery line. This
amendment adds the guidance non-destructiveness clause over the whole
emitted lifecycle diagnostic — recovery line, message body, and labeled
choice block alike:

1. A destructive-class command — a match of the reviewed
   destructive-family floor in `internal/guard`'s classifier
   (global-option-normalized, exact unsafe forms, recorded safe-form
   exclusions) — may be produced only (a) through the evidence-carrying
   constructor in `internal/guard`, whose destructive variants cannot
   be invoked without a non-defaultable work-destruction-predicate
   outcome, or (b) from a registered destructive-guidance allowlist
   entry carrying an in-code rationale AND a testable obligation; an
   entry whose obligation has no test is on the same footing as an
   unregistered site.
2. Where evidence says the action would destroy work, the recovery
   line names the safe disposition instead. Where evidence is
   unavailable or uncomputable, hints degrade to inspection-first —
   never to the destructive action.
3. **Enforcement, named** (a clause with no red mechanism is a
   slogan): the classifier and constructor (`internal/guard`), the
   repo-wide destructive-guidance convention scan (`internal/lint`,
   covering emitted diagnostics, the shipped-guidance globs, and the
   canonical setup guidance builders), and the exactly-pinned
   allowlist/registries are this clause's red mechanisms; the
   `cmd/mindspec` amendment pinning test asserts this section names
   all four.
4. **In-diff extension obligation and the finite-floor limitation,
   stated plainly**: the floor is a reviewed finite set, not closure —
   whether a piece of text is dangerous is not decidable from the
   text. A change that introduces a use or emission of a destructive
   command family not on the floor MUST extend the floor in the same
   change. This is a review-time convention with NO red mechanism
   while the new family is off the floor: a novel destructive family
   or program is caught by review under this obligation, not by the
   guard. The known-sites exemption list claims only that its exact
   quoted strings, at their exact surfaces, are known and were
   reviewed — never that they are safe because non-prescriptive.
