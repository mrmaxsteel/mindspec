# Scenario-1 (stale_phase_impl_approve) Failure Adjudication — Fix Design

**Adjudicator**: spec-092 scenario-1 failure adjudicator (Bead 9 STOP, run 1)
**Evidence base**: `review/prep/bead9_verification_evidence.md` + `bead9_run1_stale_phase_FAIL.log`
at bead-worktree tip `0281dc3`; scenario source `internal/harness/scenario_contract_hardening.go`;
spec `092-agent-contract-hardening/spec.md` (Req 1/2/19/22, HC-6, 3smk harness AC at lines 698-705);
Bead 3 shipped behavior `internal/approve/impl.go`, `internal/phase/derive.go`.
**Authority invoked**: spec 092 Req 22 redesign principle — "A scenario that cannot be made to
fail pre-fix is not a regression pin and must be redesigned or replaced" — already exercised once
for `complete_from_doomed_worktree` (commit 1603723; spec.md annotated in 8123c70). The symmetric
case here: a scenario that cannot be made to PASS post-fix for reasons unrelated to its pin is
equally not a regression pin and is redesigned under the same principle.

---

## 1. Fixture confound — VERIFIED

**Claim** (implementer's diagnosis, part 2): the sandbox fixture makes ANY clean `impl approve`
exit 1 via `adr-divergence-unowned` on `stale.go`, independent of agent behavior.

**Verified by two independent channels:**

*Channel A — run-log forensics* (`bead9_run1_stale_phase_FAIL.log`):
- `[93] mindspec repair phase 001-stale (exit=0)` — the stored phase was re-derived to `review`
  BEFORE the first gate attempt, so the phase gate cannot be what failed next.
- `[144] mindspec approve impl 001-stale (exit=1)` — first gate attempt fails post-repair.
- `[287] mindspec approve impl 001-stale --override-adr "stale.go is a new file ..." (exit=0)` —
  the IDENTICAL command succeeds with the ADR-gate bypass as the only delta, and the override
  reason names `stale.go`. The blocker at [144] was therefore the spec-087 ADR-divergence gate.

*Channel B — code trace* (this branch):
1. `internal/approve/impl.go:233` runs `validate.CheckADRDivergence(root, base, exec, specDir, "")`
   as enforcement gate 3/3 and at `:254-257` returns non-zero on any finding unless
   `--override-adr`/`--supersede-adr` is given.
2. The gate is LIVE in this fixture: `workspace.SpecDir` (workspace.go:153) resolves worktree-first
   to `.worktrees/worktree-spec-001-stale/.mindspec/docs/specs/001-stale/`, where Setup wrote
   spec.md + plan.md — so `ValidateDivergence`'s "missing spec.md → silent no-op" degrade does
   NOT apply.
3. Diff range = merge-base(main, spec/001-stale)..spec/001-stale (adr_divergence.go:40-45) and the
   Setup commit "impl: implement feature" puts `stale.go` in that range.
4. `stale.go` is not a process artifact (divergence.go:154/213-217: only `.mindspec/docs/**`,
   `docs/`, `.beads/`, `review/`, CLAUDE/AGENTS.md are skipped).
5. The sandbox has NO `.mindspec/docs/domains/` anywhere (grep over `internal/harness/` finds zero
   OWNERSHIP references; `sandbox.go` bootstraps none), so `candidateDomains` (the
   `listDomainDirs` fallback, divergence.go:115-123) is empty, `attributeDomain` returns `""`,
   and divergence.go:165-173 emits `adr-divergence-unowned` for `stale.go`.

So with the phase healed by any means — Bead 3's Req 1 self-heal, sanctioned `repair phase`, or
surgery — a perfectly-behaved single `mindspec impl approve 001-stale` exits 1 in this fixture.
The confound is real, deterministic, and a Bead-2 fixture defect, not a Bead-3 code regression.

**Why the Bead-2 baseline didn't see it**: at `c4a1c7e` the run died earlier at the (then
unhealed) phase gate; the ADR gate was never reached. Confirmed consistent with
`bead2_baseline_evidence.md` §1 ("no successful approval event possible").

### Minimal fixture fix (chosen variant)

Neither option in the charge is sufficient ALONE:
- Owning `stale.go` in an OWNERSHIP.yaml merely flips the finding from `unowned` to `uncovered`:
  `IsDomainCovered` (plan.go:488/512-) returns true only when a plan-CITED ADR with
  `Status: Accepted` lists the owning domain — the fixture plan has no `adr_citations`.
- "Scoping the spec to not impact that domain" cannot help: with no impacted-domains section the
  gate falls back to enumerating ALL domain dirs, and with zero domains everything non-artifact
  is `unowned` regardless of spec scope.

No other scenario fixture "handles" this — none has OWNERSHIP/ADR coverage; the legacy
impl-approve scenarios are simply equally exposed (see §5). So the fix must be authored fresh.
**Pick: full minimal coverage triple**, because it reproduces how a compliant field repo looks
(the 3smk field note occurred in the real mindspec repo, which HAS ownership + ADR coverage) and
is reusable as a shared helper by the other exposed fixtures:

In `ScenarioStalePhaseImplApprove.Setup`, BEFORE the worktree fork (so both main and the spec
branch carry them; commit alongside the existing root setup commit):

1. `sandbox.WriteFile(".mindspec/docs/domains/sandbox/OWNERSHIP.yaml", "paths:\n- stale.go\n")`
   (globMatch handles the literal path; ownership.go:43 reads
   `.mindspec/docs/domains/<domain>/OWNERSHIP.yaml` relative to ROOT — it must exist at the
   sandbox root checkout, not only in the spec worktree).
2. A sandbox ADR at the root ADR dir (path via the same location `adr.NewFileStore(root)` reads,
   `.mindspec/docs/adr/ADR-0001-sandbox-domain.md`) with frontmatter `Status: Accepted` and
   `Domains: [sandbox]` (match the field names `adr.FileStore` parses — copy an existing
   fixture-ADR shape from `internal/validate` tests).
3. plan.md frontmatter gains `adr_citations:\n- ADR-0001` (scalar citation form is accepted,
   plan.go:41-43).

Result: `stale.go` → owned by `sandbox` → `IsDomainCovered(store, [ADR-0001], "sandbox")` → true
→ zero findings → the ADR gate passes on the merits. Doc-sync is unaffected: `stale.go` at repo
root is not an `internal/`/`cmd/` source file (docsync.go:108-112) and the new files are doc-class.

Fallback variant (if the triple proves brittle): drop `stale.go` from the fixture so the spec
branch is docs-only (the divergence lane then has nothing governable to scan; CommitCount preflight
still sees ≥1 commit). Rejected as primary because it makes the spec branch impl-free — less
faithful to the field note and it silently de-fangs the ADR gate instead of satisfying it.

---

## 2. Assertion alignment with the spec's 3smk contract

**What the AC actually mandates** (spec.md:698-705): "Assert success via `assertCommandRanEither`
... with no `bd update --metadata` surgery event and no `mindspec repair` needed." The ESSENTIAL
contract per the field note and Goal: approve eventually succeeds against a stale stored phase
WITHOUT raw metadata surgery (the banned replace-semantics `bd update --metadata`, Req 19/HC-5).

Adjudication per current assertion:

**(a) No-surgery assertion (scenario_contract_hardening.go:141-152) — KEEP, verbatim.**
Verified it fires on genuine surgery only: it matches `bd update` events whose args carry a
`--metadata`/`--set-metadata` prefix. In the failing run it flagged exactly [92], [214], [270] —
all three genuine raw replace-writes of the epic's metadata map — and nothing else (orientation
`bd show/list`, `bd close` [200] are untouched; `bd close` remains a logged wrong-action, not a
pin). This is the heart of the 3smk pin and stays.

**(b) "agent needed `mindspec repair` — phase gate did not self-heal" (lines 153-156) — DROP.**
It contradicts the spec's own shipped Req 2/19 behavior. Bead 3 deliberately taught the
derive-consistency warning (derive.go:136-137) to end with
`recovery: mindspec repair phase <spec-id>`, and that warning is emitted by EVERY phase-deriving
command while the fixture's stale state persists — the agent saw it at `[55] mindspec state show`,
i.e. during orientation, before any gate attempt. ADR-0035 / Req 12 doctrine is precisely that
agents grep-and-paste the final recovery line. An assertion that fails the run when the agent
follows the binary's own advertised, sanctioned, merge-semantics recovery command tests AGAINST
the product contract. The AC sentence "no `mindspec repair` needed" predates Bead 3's Req 2
guidance and is superseded by it — same status as the doomed-worktree discriminator, and it gets
the same treatment: annotate both spec.md sites (Req 16 bullet and the 3smk Harness AC) as
superseded-by-redesign, the 8123c70 pattern. A conditional variant ("no repair AFTER a failed
gate") was considered and rejected: Req 2's both-phases-fail error ALSO advertises repair, so even
post-failure repair use is sanctioned product guidance; restricting it re-creates the same
contradiction one step later.

**(c) Self-heal evidence — ADD a deterministic post-session probe; do NOT demand it of the LLM
half.** Two facts force this shape:
- The harness event stream records command/args/exit only (event.go:13-27, no stdout/stderr), so
  the Req 1 HC-3 stderr line `event=lifecycle.phase_reconciled ...` (impl.go:282-283) cannot be
  asserted from agent events at all — including the suggested "assert the reconcile event when the
  agent's first approve happens with stale metadata".
- The agent's route is its own choice between two sanctioned paths (gate self-heal vs advertised
  repair); Req 1's mechanism is already deterministically pinned by the 3smk UNIT AC
  (spec.md:621-632: ApproveImpl succeeds, writes forward via MergeMetadata only after the last
  pre-terminal gate, logs `lifecycle.phase_reconciled`, end-state `done`).
So the LLM half must NOT require the self-heal specifically — but per HC-6 the harness scenario
should still evidence Req 1 end-to-end. The doomed-worktree precedent
(`assertDoomedCompleteEmitsCdNote`) supplies the architecture: a deterministic in-assertion probe.

**Probe — `assertStaleApproveSelfHeals`** (new helper, mirrors lines 301-347): in the Assertions
func, build a SECOND stale spec deterministically (`002-staleprobe`): `CreateSpecEpic` + one child
bead claimed and `bd close`d; `bd update <epic> --metadata
'{"spec_num":2,"spec_title":"staleprobe","mindspec_phase":"implement"}'` (test-side setup writes
are exempt from the agent-event assertions by construction — only `events` is scanned);
spec-worktree-only topology via `setupWorktrees(sandbox, "002-staleprobe", "", "plan")` with
spec.md + plan.md (bead_ids listing the closed bead) committed on the spec branch — docs-only, so
the ADR gate no-ops for the probe and it needs no coverage triple. Then run the sandbox binary
directly: `mindspec impl approve 002-staleprobe` with `cmd.Dir = sandbox.Root`, capturing
stdout+stderr. Assert:
  1. exit 0 — the gate healed the stale phase rather than deadlocking (the 3smk acceptance
     criterion itself);
  2. stderr contains `event=lifecycle.phase_reconciled` AND `stored=implement` AND
     `derived=review` — the Req 1 self-heal demonstrably executed end-to-end with real bd, real
     topology, the shipped binary.

**(d) No-bypass guard — ADD.** No agent `mindspec` approve event may carry `--override-adr`,
`--supersede-adr`, or `--allow-doc-skew`. With the fixed fixture there is nothing to override; an
override-assisted green would mean the confound (or a sibling gate defect) has returned and is
being masked — the exact failure mode of run 1's event [287]. This converts a regressed fixture
from a silent green into a loud red.

### Aligned assertion set (verbatim, replacing the current Assertions func body)

```go
Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
    // Discriminating assertion (3smk): the approval gate succeeded —
    // either command order is accepted here (order discovery is
    // approval_gate_discovery's pin, not this scenario's).
    assertCommandRanEither(t, events, "mindspec",
        []string{"impl", "approve"}, []string{"approve", "impl"})

    for _, e := range events {
        args := eventArgs(e)
        // No raw metadata surgery (3smk's essential ban, Req 19/HC-5):
        // the lifecycle must offer a sanctioned path — never replace-
        // semantics surgery over the epic's metadata map.
        if e.Command == "bd" && containsAll(args, "update") {
            for _, a := range args {
                if strings.HasPrefix(a, "--metadata") || strings.HasPrefix(a, "--set-metadata") {
                    t.Errorf("agent performed raw bd metadata surgery: %v", args)
                    break
                }
            }
        }
        // No gate bypass: with the coverage fixture in place nothing
        // legitimately diverges, so an override-assisted approval means
        // a fixture/gate confound is being masked, not healed.
        if e.Command == "mindspec" {
            for _, a := range args {
                if a == "--override-adr" || a == "--supersede-adr" || a == "--allow-doc-skew" {
                    t.Errorf("agent bypassed an approval gate (%s) — the gate must pass on the merits: %v", a, args)
                }
            }
        }
        // NOTE (Req 22 redesign, run-1 adjudication): `mindspec repair
        // phase` is NOT forbidden — it is the recovery command Req 2/19
        // themselves advertise on every phase-deriving command while the
        // stored phase is stale. Punishing it would test against the
        // product's own contract. Req 1's gate self-heal is pinned
        // deterministically by assertStaleApproveSelfHeals below (the
        // doomed-worktree probe precedent) and by the 3smk unit AC.
    }

    // DISCRIMINATING deterministic probe (3smk / spec 092 Req 1): a
    // fresh stale-phase spec approved by the binary directly must exit
    // 0 and emit the lifecycle.phase_reconciled self-heal event line.
    assertStaleApproveSelfHeals(t, sandbox)
},
```

(`assertStaleApproveSelfHeals` per §2(c); optionally `t.Logf` when a `mindspec repair` event is
present, for run-report color — informational only.)

---

## 3. Red-pin integrity at baseline `c4a1c7e` — HOLDS

Concrete baseline facts (verified against the git history of this branch):
- `cmd/mindspec/repair.go` does not exist at `c4a1c7e` (`git cat-file -e` fails) — `mindspec
  repair phase` is unknown and exits non-zero, mutating nothing.
- The string `phase_reconciled` does not exist anywhere at `c4a1c7e` (`git grep` empty) — no
  self-heal, no reconcile event.
- The baseline phase gate trusts the stored phase outright ("expected review mode", recorded red
  in bead2_baseline_evidence.md §1), and the fixture fix only touches gates DOWNSTREAM of the
  phase gate, so baseline behavior at the phase gate is unchanged by the new fixture.

With the redesigned assertions + fixed fixture, every baseline path is red:
1. **Deterministic floor**: `assertStaleApproveSelfHeals` runs the baseline binary against a stale
   probe spec → the phase gate rejects (exit 1) AND stderr cannot contain
   `event=lifecycle.phase_reconciled` (string absent from the binary). The probe fails REGARDLESS
   of anything the agent did — exactly the determinism the doomed redesign bought.
2. **LLM half**: the agent's only routes are (a) raw `bd update --metadata` surgery → no-surgery
   assertion red; (b) attempt the advertised-but-nonexistent... nothing is advertised at baseline
   (the Req 2 warning text doesn't exist either) — any `repair` attempt exits non-zero and the
   stored phase stays `implement`, so every approve fails → `assertCommandRanEither` (which
   requires ExitCode 0, asserts.go:57-81) red; (c) give up → same red. There is no green path:
   succeeding at the gate without surgery requires the Req 1 reconcile, which is not in the binary.
3. The dropped no-repair assertion removes nothing from the baseline pin: at `c4a1c7e` repair
   cannot succeed anyway, so it never carried baseline discriminating power — the discriminators
   were always no-surgery + gate success, and both remain.

Obligation: per Req 22's letter, the REDESIGNED scenario (new fixture + probe) must be re-run once
against the `c4a1c7e` binary and the fresh red recorded next to the doomed redesign's record in
the Bead 2 evidence trail (scenario code from the spec branch, binary from baseline — the 1603723
procedure).

---

## 4. Ownership — Bead 9 scope addition, CONFIRMED

The defect set is entirely test-harness + spec-text: scenario Setup (fixture coverage triple),
scenario Assertions (drop no-repair, add probe + no-bypass), and two spec.md annotation sites
(Req 16's 3smk bullet, the 3smk Harness AC) marking "no `mindspec repair` needed" as superseded.
ZERO production code changes: Bead 3's shipped Req 1/2/19 behavior is verified correct by code
trace (impl.go reconcile sequencing + reconcile event; derive.go recovery-line warning) and is
what the redesigned probe pins. Therefore:
- A Bead 2/3 fix round would reopen two merged, panel-approved beads to change test-only code —
  ceremony with no reviewable production delta.
- Bead 9 already owns exactly this class of change on this branch: punch-list scenario tightenings
  B7/B8/B8b/B8c landed in `145c335`, and the spec.md superseded-discriminator annotation pattern
  landed in `8123c70`. The Req-22 redesign precedent was likewise executed inside the spec without
  reopening beads.
- The one Bead-3 touchpoint is interpretive, not code: Bead 3 remains the OWNING REQUIREMENT bead
  for Req 1 (the probe is its end-to-end proof); record in Bead 3's bd notes that its harness
  proof now flows through the redesigned scenario.

Conditions on the Bead-9 landing: (i) this adjudication constitutes the orchestrator sanction that
run 1's stop-condition required before any scenario edit; (ii) the baseline re-run obligation of
§3 is part of the same commit's evidence; (iii) the verification sequence then RESTARTS at run 1
per the bead's serial no-fix-forward discipline.

---

## 5. Same-class exposures flagged for the orchestrator (not actioned here)

- **`approval_gate_discovery` (run 5) WILL hit the identical impl-approve confound**: same
  spec-worktree-only topology, `gate.go` committed on the spec branch, no coverage — its
  "successful canonical `impl approve`" assertion cannot pass post-fix. Apply the same coverage
  triple (or docs-only branch) BEFORE run 5; recommend extracting the §1 fixture fix as a shared
  helper (e.g. `writeSandboxDomainCoverage(sandbox, file string)`).
- **`complete_from_doomed_worktree` / `precommit_reexport_complete` (runs 2/3)**: `complete` also
  hard-blocks on the divergence gate (complete.go:302-305), but its diff is
  merge-base(specBranch, HEAD)..HEAD, which was empty in the baseline doomed run (complete invoked
  with HEAD=main) — hence its baseline exit 0. The doomed scenario's StartDir is the bead worktree
  (HEAD=bead branch), so the bead's `doomed.go` commit MAY enter the gate's range depending on
  where the executor resolves HEAD. Watch runs 2/3 for `adr-divergence` text in any failure;
  same fixture treatment if it fires.
- **Latent, pre-092**: legacy scenarios committing .go files and running `impl approve`
  (`stop_does_not_block_approve_impl`, `single_bead` family) carry the same exposure since spec
  087 landed; they are outside Bead 9's five-scenario charter — file a follow-up bead.
