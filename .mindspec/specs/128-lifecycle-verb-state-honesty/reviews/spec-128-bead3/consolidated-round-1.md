# spec-128-bead3 — consolidated concrete_changes_required, round 1

Tally: 1 APPROVE (R2) / 5 REQUEST_CHANGES (R1 ×3, R3 ×3, R4 ×1, R5 ×2, R6 ×3 with hard_block). NOTE: R4/R5/R6 ran as claude-subs (codex capacity outage; retries exhausted; family substitution explicit in reviewer_id). 12 asks in 6 groups. Judgment call A: ACCEPTED by R1/R3 with verification. Artifact-gate screen: clear. The R3-carried spec-branch RED (phase.lock fixture fallout) is dispatched SEPARATELY as a sanctioned spec-branch test fix — not this bead's scope.

## G1 — Flag-blind fallback: an explicit --spec may never be answered with a different spec or ambient state (R1 ×2 + R4 + R5 + R6-F1/F3 — FOUR-slot convergence, all with exit-0 repros) — MUST FIX
When specFlag is non-empty and ResolveTargetWithCache fails non-ambiguously: PROPAGATE the resolution error (after the health probe if failure-vs-empty matters) — never fall into unfiltered DiscoverDraftSpecsWithCache (repro: `--spec 999` + one unrelated draft → that draft's {"mode":"spec"} exit 0), never ambient idle (0-draft repro), never the F3 contradictory ambiguity advice. AND the success-path fabrication: `--spec <valid nonexistent ID>` currently prints {"mode":"idle","activeSpec":<id>,...} exit 0 — R1.1's banned shape via the explicit-flag pass-through; render not-found (or drop activeSpec). Regression fixtures: unresolvable prefix with 1 draft / 0 drafts; invalid --spec value; the flagged-nonexistent idle+activeSpec case. (Optional follow-up, not required: prefix resolution over drafts.)

## G2 — Error-vs-not-found discrimination + warm-cache truth (R6 F2a/F2b, hard-block core, both empirically reproduced) — MUST FIX
1. F2a: a genuine (non-not-found) FindEpicBySpecIDWithCache error routes into the draft probe; when the probe's AllEpics health-retry warms the cache and the epic IS found, isDraft=false collapses to ModeIdle — discarding the error AND the found epic (repro: fail-exactly-call-1 stub + in_progress epic → {"mode":"idle","activeSpec":...} exit 0 = AC-1(i)'s banned shape). Distinguish the not-found sentinel from genuine failures before the draft probe, or surface the probe's found-epic to derive the real phase.
2. F2b: after a successful c.AllEpics() health probe, RE-RUN target resolution (memoized, free) instead of concluding "healthy, nothing active" — the original failure may have been the flake the probe just disproved (repro: same stub no-flag → idle; healthy control → plan).

## G3 (R3, MAJOR) — Main-checkout draft discovery is real-world vacuous — MUST FIX (extend or adjudicate loudly)
`spec create` writes the spec dir ONLY into the workspace (create.go:89, :117) — the os.Stat probe can never see a fresh draft from main; AC-1(iii) passes only on a shape the tool never produces. Either extend probe reach (tree-scoped `git cat-file -e spec/<id>:.mindspec/specs/<id>/spec.md` against the visible spec/<id> ref, or configured worktree-path scan) — preferred, it serves the spec's goal — or adjudicate out loud (deviation note, ADR-0023 amendment sentence on per-checkout visibility, AC-1(iii) reframed to its real premise). Implementer decides with rationale; silence is the only wrong answer.

## G4 (R3, MAJOR) — run.go:134's Mode-"" window — MUST FIX IN THIS BEAD
`mode, _ := ResolveModeWithCache` under the changed contract → Mode "" → instruct.Render's accidental "loading template .md" failure. run.go is in NO bead's declared list, so the window has no owner — fix at the call site here (return the error or render the explicit refusal), disclosed as a scope addition. (Tally decision: in-bead fix over a paper assignment to bead 5.)

## G5 (R5 + R1, convergent) — Multi-draft ambiguity is delivered but untested — MUST FIX (cheap)
state.go-level fixture: two drafts, no --spec → ErrAmbiguousTarget listing both; plus/or a DiscoverDraftSpecsWithCache multi-draft unit pinning the sorted result (use newDraftFixtureRoot).

## G6 (R3, minor) — Draft-probe efficiency — MUST FIX (one-line reorder)
Consult memoized FindEpicBySpecIDWithCache BEFORE gitutil.BranchExistsIn in draftSpecStatusWithCache (idle main currently spawns ~1 git subprocess per historical spec dir).

## Carry-forwards recorded for bead 5's prompt (not this bead)
(a) The planned indeterminate trigger is UNREACHABLE for in-worktree bd failures (ResolveContextFromDir never errors; back-fills modes) — bead 5 must rewire or de-swallow. (b) Standalone `mindspec instruct` on main-with-draft renders idle while state show says spec — run.go ownership now partially addressed by G4; the instruct-template half stays bead 5's.
