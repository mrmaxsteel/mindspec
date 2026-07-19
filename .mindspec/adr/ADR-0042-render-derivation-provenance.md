# ADR-0042: Render + Derivation Provenance

- **Date**: 2026-07-19
- **Status**: Accepted
- **Domain(s)**: workflow, core, execution, context-system
- **Deciders**: Max
- **Supersedes**: n/a
- **Superseded-by**: n/a
- **Related**: [ADR-0037](ADR-0037-panel-gate-enforced-contract.md) (the termsafe single-home doctrine its 116 amendment records — this ADR adds consumers, changes nothing 0037 governs), [ADR-0035](ADR-0035-agent-error-contract.md) (every refusal this contract introduces carries a genuine final `recovery:` line; the spine strengthens recovery-executability — a printed recovery command embeds either a validated-clean operand or a trusted shell-quoted root, never a hostile value), [ADR-0023](ADR-0023.md) + [ADR-0025](ADR-0025-jsonl-as-build-artifact.md) (Dolt as READ authority and jsonl as projection — which establish read authority, NOT safe id provenance; see §2), [ADR-0030](ADR-0030-executor-boundary.md) (the executor boundary the render/gating changes leave untouched), [ADR-0041](ADR-0041-gate-before-mutate.md) (the preflight discipline the new derivation refusals slot into)

---

## Context

Spec 116 shipped the escaping *mechanism* (`internal/termsafe.Escape`:
printable ASCII unchanged, else single-line `strconv.Quote`) and closed the
panel-gate path. Ten review rounds on spec 120 then established, verified
against real code, that escaping alone cannot carry the trust boundary:

- **Escaping is structurally insufficient for executable and ID-typed
  positions.** `termsafe.Escape` is the IDENTITY on printable ASCII, so a
  hostile-but-printable operand (`120-x;evil`, `--help`) survives it
  byte-identically — it renders as if trusted, and executes on agent
  compliance or lands in argv as option injection.
- **Neither sink-enumeration nor source-enumeration converges.** Round 1→2
  found new render sinks; round 2→3 found five more derivation ingresses
  (CLI `--spec`, `impl approve args[0]`, lifecycle-scan bd enumeration,
  inline `"bead/"+` concat, direct `filepath.Join`); rounds 4–5 then proved
  reverse-derivation SHAPES unbounded — four parser shapes in two rounds
  (`strings.TrimPrefix`, `os.ReadDir`, `ReadFile`+`json.Unmarshal`,
  bracket/colon `Index`-slice). No syntactic ratchet over parser shapes can
  be complete by construction.
- **The validators' grammar was wrong.** `idvalidate.BeadID` rejected 489
  of 774 live bd IDs (every dotted epic-child, every short-suffix legacy
  ID) and `idvalidate.SpecID` rejected the live `008b`/`008c` spec dirs —
  so applying them unchanged at any waist would have bricked `complete`/
  `next`/worktree-detect for most beads. A validator stricter than the real
  grammar converts the spine into a denial-of-service on legitimate work.
- **bd ids are agent-writable — empirically proven, round 9.**
  `bd create --force --id="--help" --type=epic` SUCCEEDS on bd v1.1.0+Dolt
  (orchestrator-reproduced in a fresh sandbox; `bd list` then returns an
  epic whose id is `--help`). bd's `--id`+`--force` writes ARBITRARY ids
  directly to Dolt. ADR-0023/ADR-0025 establish Dolt as the READ authority
  and jsonl as a projection — they say nothing about safe id PROVENANCE.
  The round-8 "bd-minted, therefore safe" severity refutation was WRONG and
  is RETRACTED on the record: bd/epic/bead ids are agent-writable at every
  consumer, like every other agent-writable value.
- The root-cause path is concrete: `slugify` preserves shell
  metacharacters from the agent-writable bd `spec_title` metadata, and the
  derived specID flowed unvalidated into branch names, worktree paths, and
  executable `cd` renders. The agent-writable config `worktree_root`
  participates in every composed worktree path with no predicate at all.

The structural question this ADR answers: WHERE does validation live so
that the guarantee is by construction rather than by enumeration?

## Decision

The repo-wide provenance taxonomy is enforced as follows. It fits the
five-consumer taxonomy exactly: authority-bearing consumer APIs are finite
and enumerable — **structural composition, executable operands,
render/display, semantic lookup, structured persistence** — while sinks,
sources, derivation shapes, and callers are not; therefore every guarantee
below lands at a consumer boundary or a composition waist, never at an
enumeration of sinks, sources, shapes, or callers.

### 1. Structured identifiers: grammar-correct validation at every derivation boundary AND at the composition waist

`internal/idvalidate` is the single validation authority, and its patterns
are grammar-correct against the framework's REAL live ID inventory —
dotted numeric epic-children at any depth, any-length alnum segments,
one optional spec-number letter suffix — pinned by a committed
live-inventory fixture (774/774 bd IDs, 120/120 spec dirs). This is a
strict contract WIDENING: every previously-accepted ID still passes, and
every security rejection is preserved (empty, `.`, `..`, path separators,
glob metacharacters; by charset every shell metacharacter, whitespace,
control byte, uppercase). Grammar correctness is the PREREQUISITE of the
whole contract: the fixture is the ratchet against both future legacy
forms and future over-narrowing.

An untrusted string must pass `idvalidate` before it is promoted to an ID
at every authority-bearing consumer, in both directions of the ID↔string
boundary:

- **Forward (the composition waist)**: the ten `internal/workspace`
  composition helpers (`SpecBranch`, `BeadBranch`, the worktree
  name/path helpers, the finalize helpers, and `SpecDir`) validate their
  ID argument internally and fail closed with `(string, error)` returns —
  the `SpecDir` SEC-1 precedent. Every caller — CLI, metadata
  slugification, dir-name parse, lifecycle scan — is covered by
  construction; no caller can obtain a composed branch, worktree name, or
  path from an invalid ID. Downstream `cd`/`git merge`/`git branch -d`/
  template operands therefore carry only `[a-z0-9./-]` bytes by
  construction.
- **Reverse (the five consumer classes)**: a string parsed back OUT of an
  agent-writable source (a branch name, a dir name, a JSON field, a bd
  title, a plan frontmatter list, the bd store itself) validates before it
  exercises ID authority at ANY of the five classes — composed into a
  path/branch, embedded in `bd`/`git` argv, rendered in an ID-typed
  position, matched at a semantic-lookup boundary, or written into a
  durable ID field. Derivation-site gates and derivation-shape scans are
  DEFENSE-IN-DEPTH (earlier failure, better diagnostics), never the
  completeness guarantee — derivation shapes are unbounded; consumers are
  not.
- **Gate ALL ids — there is NO bd-minted exemption.** Because bd ids are
  agent-writable (the round-9 empirical proof above), no id operand is
  trusted by PROVENANCE anywhere: every ID-position operand at every
  `bd`/`git` exec site — however the variable is named, however the value
  was derived, whatever store it came from — passes `idvalidate` at or
  before the call. The class-2 boundary lives at the consumers
  (`internal/bead`'s id-taking bd calls gate in-package, covering every
  present and future caller), not at callers. A well-formed id passes for
  free, so gating costs nothing and ends per-site trust litigation: with
  the exemption abolished, no "is this id safe" question exists.
- Degrade-vs-error policy stays at the policy points: ambient scans and
  best-effort paths SKIP the malformed object with one escaped warning
  naming the convergent repair lever; explicit verbs targeting the
  malformed object REFUSE convergently with a single lever (ADR-0035).
  Validated IDs stay RAW thereafter — that is the point of validating.

### 2. Free-text display fields: escape at render, per field

Agent-writable fields that legitimately contain arbitrary printable text
(titles, descriptions, filenames, porcelain and git error bodies, slugs
read from disk) CANNOT be charset-gated. They route `termsafe.Escape` at
the render site, per field (per line for line-oriented bodies), never per
message. Multi-line bd body fields and the spec Goal remain fenced,
labeled markdown payload (116's inherited persuasion Non-Goal). There is
one escaper; no second implementation.

### 3. ID-typed render positions: the forced-safe `idrender` rule

No value is ever *presented as* an ID unless it validated. The single
render home `idrender.Spec`/`idrender.Bead` (an `internal/idvalidate`
sub-package, stdlib-only): a value passing `idvalidate` renders
BYTE-IDENTICAL; anything else renders `strconv.Quote`d — always quoted,
even when printable, precisely because `termsafe.Escape` is the identity
on printable ASCII and would present `spec=120-x;evil` byte-raw as if
trusted. Severity context, on the record: display alone executes nothing —
the forced quote is a trust-labeling defense against presenting
unvalidated bytes as a trusted ID, not a remote-execution fix. The empty
string is the established no-value sentinel and renders as identity.
Presenting an unvalidated value in an ID-labeled position is itself the
defect, independent of what the bytes are.

### 4. The config path component: `worktree_root` predicate + TOCTOU-bounded containment

The agent-writable `worktree_root` is predicate-validated at config
ingress (relative, conservative charset, no `..` segment — with lexical
checks explicitly named lexical-only) plus symlink-aware containment:
`filepath.EvalSymlinks` on the resolved root and on the deepest EXISTING
ancestor of the composed path, which must resolve under the root. The
containment predicate re-runs immediately before each USE of a composed
worktree path (worktree-create, `Chdir`, `MkdirAll` — check-at-use). This
contract does NOT claim atomic containment: the window between
check-at-use and the kernel/git operation is an ACCEPTED, honestly bounded
residual — an adversary who can win that race holds concurrent local
write access, the same capability plane as editing hooks or config
directly, already outside the threat model. Executable-`cd` paths are
emitted through one exported shell-safe emitter (byte-identical when
unquoted-safe, else POSIX single-quoted); root-only sinks quote-emit and
never refuse — the repo root is operator-chosen and trusted.

### 5. The enforcer: the whole-tree wrapper-agnostic lint

The R6(g) two-way `go/ast` lint in `internal/lint` is THE exhaustive,
by-construction enforcer of the gate-all-ids rule. It resolves every
`bd`/`git` invocation in `cmd/`+`internal/` SEMANTICALLY at the exec seam
/ wrapper-call graph — through the `internal/bead` helpers, the package
seam-vars, the harness wrappers, direct `exec.Command`/`LookPath` forms,
and any FUTURE wrapper function — never by a fixed wrapper-name list.
Every discovered call site must carry one of exactly two dispositions:
GATED (every id-position operand passes `idvalidate` at or before the
call) or AUDITED-ALLOWLISTED for genuine NON-id operands only
(framework-authored subcommands/flags, literals, waist-composed branch
operands, SHAs, `--`-separated pathspecs, free text). The allowlist
schema itself rejects id-provenance justifications: an entry justified as
"bd-minted" or "not agent-steerable" is a test failure. The prose site
enumerations in spec 120 are ILLUSTRATIVE audited seeds, not the
enforcement surface — a call site absent from the prose is not a spec
hole; the lint fails the build on any un-gated, un-allowlisted id-operand
site, so completeness is mechanical. The forward analog (the
composition-helper allowlist and the inline-concat/Join scans) closes the
composition class the same way; the raw-ID-render scan backs §3; the
derivation-shape scans (TrimPrefix, root-enumeration) are retained as
labeled defense-in-depth with no completeness claim.

### 6. The inertness theorem for exact-match lookups

Pure in-process exact-equality matching confers NO authority on malformed
bytes: a hostile ID can only fail to match. `phase.FindEpicBySpecID` is
the pinned exemplar (in-memory string equality; the specID never enters
argv, a path, or any byte-interpreting operation), and `validate.
SpecStatus` is waist-backstopped fail-closed. The semantic-lookup class is
therefore gated as one-line boundary posture (invalid → the existing
not-found/no-match semantics) — recorded here so matching cannot be
re-litigated as an open class.

### 7. The persistence doctrine

Durable ID-field writers (`recording.EmitBeadMarker`, `AddBeadToPhase`)
validate before write — trace-record integrity hygiene. But persisted
stores are agent-writable, so validation-at-write is NEVER trust: **any
future read of a persisted ID field into an ID role re-validates at
ingest.** The same rule is why bd's own store confers no provenance (§1).

### 8. The convergence stopping rule

The only admissible future reverse-derivation finding is **an unvalidated
value exercising ID authority at a consumer** — i.e. a consumer API
missing from the five-class inventory, or a gate failing its acceptance
criterion: a falsifiable claim against a finite list. A new parser shape,
a new caller, or a new wrapper is NOT a spec hole — its hostile product is
contained at every consumer class (waist error / argv gate / forced-quoted
render / no-match / skipped write), and the wrapper-agnostic lint discovers
the new site at build time. Such findings are non-blocking ratchet-hygiene
notes.

### 9. The newtype deferral (the terminal escalation)

A validated-ID newtype (opaque `SpecID`/`BeadID` types constructible only
via validating constructors) is the DEFERRED post-120 terminal escalation
(spec 120 OQ4-b), triggered ONLY by a post-ship consumer-boundary leak in
practice — a novel raw consumption site that the (a)–(h) ratchets and
review discipline all miss. If triggered, adoption starts at the
`workspace`/`phase`/`recording` API boundary. It is not adopted now:
partial adoption yields false confidence, and full adoption is a
repo-wide typed refactor that would reopen a settled, multiply-reviewed
core for a guarantee the waist + consumer gates + lint already carry.

## Consequences

### Positive

- The taxonomy's promise — "ingress-validated ⇒ stays raw" — is TRUE by
  construction instead of assumed: no composed branch/path, no `bd`/`git`
  id operand, no ID-labeled render, no semantic match, and no durable ID
  write can carry a value that failed the corrected `idvalidate`.
- Clean inputs are byte-identical to before: the corrected grammar accepts
  the entire live inventory, validated IDs render as identity through
  `idrender`, and no gate or command changes its decision for any
  well-formed input — the spine is behavior-invisible on a healthy repo.
- Review litigation ends structurally: the gate-all-ids rule removes the
  per-site "is this id safe" question; the stopping rule (§8) makes the
  only admissible future finding falsifiable against a finite consumer
  list; the wrapper-agnostic lint makes "the enumeration missed a site"
  a build failure instead of a review round.
- Refusals stay convergent (ADR-0035): every derivation refusal names one
  lever that changes or routes around the offending state
  (`mindspec repair spec-title`, `mindspec spec list`, `bd ready`, the
  worktree_root set-default lever), with the hostile value escaped-only.

### Negative / Tradeoffs

- The nine waist helpers gain `(string, error)` signatures — every future
  caller carries an explicit error obligation (the deliberate SEC-1
  compile-time-obligation tradeoff), and ~62 existing call sites were
  routed once to pay for it.
- The TOCTOU residual in §4 is accepted, not eliminated — check-at-use
  bounds it to the concurrent-local-writer capability plane; this ADR
  must not be cited as an atomic-containment guarantee.
- The lint allowlists carry real audit cost: every new bd/git invocation,
  composition-helper call, or ID render must name its gate or its non-id
  justification before the build passes — a constraint, not a suggestion.
- This is not tamper-resistance and not full prompt-injection safety:
  artifacts stay agent-writable, and a persuasive printable string inside
  a fenced payload still renders in its labeled context (116's inherited
  Non-Goal).

## Alternatives Considered

### 1. Refuse-or-escape at every render sink

The original spec-120 shape. Rejected as unsound at its root: sink
enumeration never converges (each review round found sinks the previous
matcher could not see), and escaping is the identity on printable ASCII —
a hostile printable operand survives escaping byte-identically and
executes on agent compliance. Rendering is a consumer class, not the
boundary.

### 2. Enumerate and gate every derivation source / parser shape

Rejected: source enumeration spiraled exactly as sink enumeration did
(five new ingresses in one round), and derivation shapes were proven
unbounded (four parser shapes in two rounds). The derivation-site gates
and the TrimPrefix/enumeration scans are retained as defense-in-depth
only; the guarantee lives where the API surface is finite — the
consumers.

### 3. Trust bd-minted ids (allowlist by provenance)

Rejected on empirical evidence: `bd create --force --id="--help"
--type=epic` succeeds on bd v1.1.0+Dolt, so the store confers no
provenance and a provenance-justified allowlist entry is a hole. Gating
every id costs nothing on well-formed ids and removes the trust question
entirely. The round-8 refutation that argued otherwise is retracted on
the record.

### 4. Silent sentinel returns at the waist (empty string on invalid)

Rejected outright: an empty composed path is fail-open — downstream code
happily operates on `""` or on a prefix-only branch name. Fail-closed
`(string, error)` makes the invalid case a compile-time obligation.

### 5. Adopt the validated-ID newtype now

Rejected for this spec (see §9): recorded as the deferred terminal
escalation with a named trigger, not a current mechanism.
