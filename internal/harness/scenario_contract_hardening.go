package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file owns the five Spec 092 (agent-contract-hardening) regression
// scenarios. Each pins one 2026-06-10 field-note failure and is REQUIRED to
// fail against the pre-fix baseline (spec 092 Req 22 / HC-6):
//
//	stale_phase_impl_approve       — mindspec-3smk
//	complete_from_doomed_worktree  — mindspec-qxsy
//	precommit_reexport_complete    — mindspec-i4ad
//	wrong_directory_guard_recovery — mindspec-tjat
//	approval_gate_discovery        — mindspec-v7ez

// argsInOrder reports whether first and second both appear in args with
// first occurring before second (exact-token match). Used for order-aware
// command assertions where `mindspec impl approve` (canonical) must be
// distinguished from the deprecated `mindspec approve impl`.
func argsInOrder(args []string, first, second string) bool {
	firstIdx := -1
	for i, a := range args {
		if a == first {
			firstIdx = i
			break
		}
	}
	if firstIdx == -1 {
		return false
	}
	for _, a := range args[firstIdx+1:] {
		if a == second {
			return true
		}
	}
	return false
}

// firstEventIndex returns the index of the first event matching the
// predicate, or -1.
func firstEventIndex(events []ActionEvent, match func(ActionEvent) bool) int {
	for i, e := range events {
		if match(e) {
			return i
		}
	}
	return -1
}

// ScenarioStalePhaseImplApprove pins field note mindspec-3smk: the epic's
// stored mindspec_phase metadata is stale ("implement") while every child
// bead is already closed (child-derived phase = review). Pre-fix,
// `mindspec impl approve` trusts the stored phase and fails with
// "expected review mode" — the only way out is raw `bd update --metadata`
// surgery, which the assertions forbid. Post-fix (spec 092 Req 1) the gate
// re-derives the phase from children and self-heals.
//
// Topology is spec-worktree-only (no bead branches/worktrees) so the stale
// phase is the ONLY blocking condition — otherwise the unmerged-bead gate
// would mask the pin (cf. ScenarioUnmergedBeadGuard).
func ScenarioStalePhaseImplApprove() Scenario {
	var epicID, beadID string
	return Scenario{
		Name:        "stale_phase_impl_approve",
		Description: "impl approve self-heals a stale mindspec_phase when children say review",
		MaxTurns:    25,
		TimeoutMin:  10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-stale"

			// Epic + one child bead, claimed then closed via raw `bd close`
			// (replaying the field note — the close path that skips the
			// mindspec_phase sync).
			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)
			sandbox.runBDMust("close", beadID)

			// Stale phase cache: stored phase says "implement" while every
			// child is closed (child-derived phase = review). bd update
			// --metadata REPLACES the whole map, so spec_num/spec_title are
			// repeated to preserve ADR-0023 epic binding.
			sandbox.runBDMust("update", epicID, "--metadata",
				`{"spec_num":1,"spec_title":"stale","mindspec_phase":"implement"}`)

			// Spec-worktree-only topology: spec branch + worktree, NO bead
			// branches — the stale phase is the only blocking condition.
			wt := setupWorktrees(sandbox, specID, "", "plan")

			// Spec/plan/impl artifacts committed on the spec branch (doc-sync
			// gate and FinalizeEpic need real content; bead_ids lets the
			// plan-bead gate verify the closed bead).
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Stale Phase Feature
status: Approved
---
# Stale Phase Feature
A completed feature whose epic phase cache went stale.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Implement feature
Create stale.go.
`, specID, beadID))
			sandbox.WriteFile(wt.SpecWtDir+"/stale.go", `package main

func Stale() string { return "stale" }
`)
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "impl: implement feature")

			sandbox.Commit("setup: review-by-children, stale stored phase")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The implementation for spec 001-stale is finished: every bead is closed and
the human has reviewed and approved the implementation. Approve the
implementation so the project returns to idle.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// Discriminating assertion (3smk): the approval gate succeeded —
			// either command order is accepted here (order discovery is
			// approval_gate_discovery's pin, not this scenario's).
			assertCommandRanEither(t, events, "mindspec",
				[]string{"impl", "approve"}, []string{"approve", "impl"})

			// No raw metadata surgery: the gate must self-heal, not the agent.
			for _, e := range events {
				args := eventArgs(e)
				if e.Command == "bd" && containsAll(args, "update") {
					for _, a := range args {
						// Catches --metadata, --metadata=..., --set-metadata,
						// and --set-metadata=... — all raw surgery paths.
						if strings.HasPrefix(a, "--metadata") || strings.HasPrefix(a, "--set-metadata") {
							t.Errorf("agent performed raw bd metadata surgery: %v", args)
							break
						}
					}
				}
				// No `mindspec repair` needed — the gate heals in-line.
				if e.Command == "mindspec" && containsAll(args, "repair") {
					t.Errorf("agent needed `mindspec repair` — phase gate did not self-heal: %v", args)
				}
			}
		},
	}
}

// ScenarioCompleteFromDoomedWorktree pins field note mindspec-qxsy: the
// agent's working directory IS the bead worktree that `mindspec complete`
// removes on success. In the field the invoking shell is left in a deleted
// directory: getcwd errors and a spurious exit 1 AFTER the command fully
// succeeded, so agents re-run complete or reach for git worktree repair.
//
// Baseline redesign (spec 092 Req 22): the spec's original discriminators
// were the no-retry/no-repair event assertions, but the recorded pre-fix
// baseline run showed Claude Code's Bash tool transparently self-heals the
// deleted cwd in this harness — the agent never observes the field failure,
// so those assertions cannot go red here. The discriminating assertion is
// therefore a DETERMINISTIC post-session probe pinning mindspec-qxsy's own
// acceptance criterion (spec 092 Req 4): a `mindspec complete` invoked from
// inside the bead worktree it removes must exit 0 AND emit the cd-back
// NOTE ("your shell's working directory was removed — run: cd <root>") —
// absent pre-fix, emitted post-Bead-4. The LLM half (StartDir = bead
// worktree, no retry/no repair) is retained as the behavioral envelope.
func ScenarioCompleteFromDoomedWorktree() Scenario {
	var epicID, beadID string
	return Scenario{
		Name:        "complete_from_doomed_worktree",
		Description: "mindspec complete run from inside the bead worktree it deletes",
		MaxTurns:    25,
		TimeoutMin:  10,
		Model:       "haiku",
		StartDir:    ".worktrees/worktree-spec-001-doomed/.worktrees/worktree-*",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-doomed"

			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Implement doomed", "task", epicID)
			// Deferred keepalive sibling keeps the epic from auto-closing
			// when the task bead closes (see ScenarioBeadsArtifactPassthrough).
			keepaliveID := sandbox.CreateBead("["+specID+"] future: follow-up", "task", epicID)
			sandbox.runBDMust("defer", keepaliveID)
			sandbox.ClaimBead(beadID)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Doomed Worktree Feature
status: Approved
---
# Doomed Worktree Feature
Add a doomed function.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Implement doomed
Create doomed.go with a Doomed function.
`, specID, beadID))
			sandbox.Commit("setup: approved spec and plan")

			wt := setupWorktrees(sandbox, specID, beadID, "implement")

			// Implementation is already committed in the bead worktree — the
			// agent's only job is to run the lifecycle close from INSIDE the
			// worktree that complete will remove.
			sandbox.WriteFile(wt.BeadWtDir+"/doomed.go", `package main

func Doomed() string { return "doomed" }
`)
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.BeadWtDir, "commit", "-m", "impl: doomed feature")

			sandbox.Commit("setup: implement mode, work committed")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

You are inside the bead worktree for the currently claimed bead. The
implementation is already committed — do not write any code. Finish the bead
through the MindSpec lifecycle (mindspec complete). Afterwards, verify the
project state and report the current mode.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// The complete itself must succeed and close the bead.
			assertCommandSucceeded(t, events, "mindspec", "complete")
			assertBeadsState(t, sandbox, epicID, map[string]string{
				beadID: "closed",
			})

			// Behavioral envelope (qxsy): after the first successful
			// complete, the agent must NOT re-run complete or reach for
			// git worktree repair surgery.
			firstOK := firstEventIndex(events, func(e ActionEvent) bool {
				return e.Command == "mindspec" && e.ExitCode == 0 && containsAll(eventArgs(e), "complete")
			})
			if firstOK < 0 {
				return // already reported by assertCommandSucceeded
			}
			for _, e := range events[firstOK+1:] {
				args := eventArgs(e)
				if e.Command == "mindspec" && containsAll(args, "complete") {
					t.Errorf("agent re-ran mindspec complete after success (exit=%d): %v", e.ExitCode, args)
				}
				if e.Command == "git" && containsAll(args, "worktree") {
					if containsAll(args, "add") || containsAll(args, "remove") ||
						containsAll(args, "prune") || containsAll(args, "repair") ||
						containsAll(args, "move") {
						t.Errorf("agent issued git worktree repair after successful complete: %v", args)
					}
				}
			}

			// DISCRIMINATING assertion (qxsy / spec 092 Req 4, deterministic):
			// run a second `mindspec complete` from INSIDE a fresh bead
			// worktree that the command will remove. The mindspec process
			// must exit 0 (the terminal mutation succeeded) and its output
			// must carry the cd-back NOTE, because the invoking shell cannot
			// be repaired by the process — the NOTE is the only channel.
			assertDoomedCompleteEmitsCdNote(t, sandbox, epicID, "001-doomed")
		},
	}
}

// assertDoomedCompleteEmitsCdNote claims a fresh probe bead, builds its
// worktree nested under the spec worktree, commits work there, and runs
// `mindspec complete` with the process cwd INSIDE that worktree. It asserts
// exit 0 and the Req 4 cd-back NOTE ("working directory was removed") in
// the combined output. Pre-fix the NOTE does not exist anywhere in the
// binary, so this fails deterministically at the pinned baseline.
func assertDoomedCompleteEmitsCdNote(t *testing.T, sandbox *Sandbox, epicID, specID string) {
	t.Helper()

	specWt := ".worktrees/worktree-spec-" + specID
	if !sandbox.FileExists(specWt) {
		t.Errorf("spec worktree %s missing — cannot run the doomed-complete probe", specWt)
		return
	}

	probeID := sandbox.CreateBead("["+specID+"] cd-note probe", "task", epicID)
	sandbox.ClaimBead(probeID)

	probeBranch := "bead/" + probeID
	probeWt := specWt + "/.worktrees/worktree-" + probeID
	mustRunGit(sandbox, "branch", probeBranch, "spec/"+specID)
	mustRunGit(sandbox, "worktree", "add", probeWt, probeBranch)
	sandbox.WriteFile(probeWt+"/probe.go", `package main

func Probe() string { return "probe" }
`)
	mustRunGit(sandbox, "-C", probeWt, "add", "-A")
	mustRunGit(sandbox, "-C", probeWt, "commit", "-m", "impl: cd-note probe")

	cmd := exec.Command(filepath.Join(sandbox.mindspecBinDir, "mindspec"),
		"complete", probeID, "cd-note probe")
	cmd.Dir = filepath.Join(sandbox.Root, probeWt) // the directory complete will remove
	cmd.Env = sandbox.Env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("mindspec complete from inside its own bead worktree exited non-zero (qxsy): %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "working directory was removed") {
		t.Errorf("complete output lacks the cd-back NOTE (\"your shell's working directory was removed — run: cd <root>\", spec 092 Req 4); got:\n%s", out)
	}
}

// precommitReexportHook chains the standard MindSpec pre-commit shim with a
// `bd export` whose output path is anchored to the COMMITTING worktree's
// top level. The absolute path is what defeats the harness's pinned bd shim
// (recorder.go cd's to sandbox root before exec, so a relative -o would
// land at the sandbox root instead of the worktree being committed).
// The "pre-commit hook v5" marker keeps githooks.InstallPreCommit from
// overwriting it mid-session.
const precommitReexportHook = `#!/usr/bin/env bash
# MindSpec pre-commit hook v5 (thin shim) — chained with bd export
# (scenario precommit_reexport_complete, field note mindspec-i4ad).
mindspec hook pre-commit "$@"
status=$?
if [ $status -ne 0 ]; then
  exit $status
fi
bd export -o "$(git rev-parse --show-toplevel)/.beads/issues.jsonl"
exit 0
`

// ScenarioPrecommitReexportComplete pins field note mindspec-i4ad: a
// pre-commit hook re-exports .beads/issues.jsonl during `mindspec
// complete`'s auto-commit, leaving the tree dirty again immediately after
// committing. Pre-fix, complete's plain IsTreeClean check fails with
// "workspace has uncommitted changes" and the field workarounds were
// `--no-verify` / `core.hooksPath` — both forbidden here. Post-fix (spec
// 092 Reqs 6-7) artifact dirt is classified per ADR-0025 and never blocks.
func ScenarioPrecommitReexportComplete() Scenario {
	var epicID, beadID string
	return Scenario{
		Name:        "precommit_reexport_complete",
		Description: "mindspec complete succeeds despite a pre-commit hook re-exporting the beads JSONL",
		MaxTurns:    30,
		TimeoutMin:  10,
		Model:       "haiku",
		StartDir:    ".worktrees/worktree-spec-001-reexport/.worktrees/worktree-*",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-reexport"

			// Track .beads/issues.jsonl (default sandbox .gitignore ignores
			// .beads/ entirely, which would make the scenario pass today).
			// Pattern from ScenarioBeadsArtifactPassthrough.
			sandbox.WriteFile(".gitignore", ".beads/*\n!.beads/issues.jsonl\n.harness/\n.worktrees/\n.mindspec/session.json\n.mindspec/focus\n.mindspec/current-spec.json\n")

			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Create reexport.go", "task", epicID)
			keepaliveID := sandbox.CreateBead("["+specID+"] future: follow-up", "task", epicID)
			sandbox.runBDMust("defer", keepaliveID)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Reexport Feature
status: Approved
---
# Reexport Feature
Add a reexport.go file.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Create reexport.go
Create reexport.go with a Reexport() function.
`, specID, beadID))

			// Baseline JSONL tracked by git BEFORE the bead is claimed, so
			// the claim leaves Dolt state ahead of every checked-out copy —
			// the hook's re-export during the session deterministically
			// dirties the worktree complete checks.
			if _, err := sandbox.runBD("export", "-o", ".beads/issues.jsonl"); err != nil {
				return fmt.Errorf("bd export baseline: %w", err)
			}
			sandbox.Commit("setup: spec + plan + baseline JSONL")

			// Claim AFTER the baseline commit (Dolt now differs from the
			// committed JSONL), then build the implement topology — the bead
			// worktree's JSONL is the stale baseline copy.
			sandbox.ClaimBead(beadID)
			setupWorktrees(sandbox, specID, beadID, "implement")

			// Install the chained pre-commit hook LAST so setup commits run
			// under the stock hook. Worktrees share .git/hooks via the
			// common git dir, so this fires for bead-worktree commits too.
			hookPath := filepath.Join(sandbox.Root, ".git", "hooks", "pre-commit")
			if err := os.WriteFile(hookPath, []byte(precommitReexportHook), 0o755); err != nil {
				return fmt.Errorf("installing chained pre-commit hook: %w", err)
			}
			return nil
		},
		// Prescriptive prompt, like ScenarioBeadsArtifactPassthrough: the
		// friction under test lives inside complete's clean-tree check, not
		// in command discovery. Steering the agent to complete's documented
		// auto-commit path (instead of a manual git commit) keeps the first
		// complete attempt on the exact field-note path.
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

You are inside the bead worktree for the claimed bead of spec 001-reexport.
1. Create reexport.go with a function Reexport() string that returns "reexport".
2. Finish the bead by running mindspec complete with the bead id and a one-line
   description of what you did — it auto-commits your work. Do not run git
   commit yourself.

Do not close beads directly with bd commands.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			assertCommandSucceeded(t, events, "mindspec", "complete")
			assertBeadsState(t, sandbox, epicID, map[string]string{
				beadID: "closed",
			})

			// Field workarounds are forbidden (i4ad): no --no-verify, no
			// core.hooksPath bypass anywhere in the event stream.
			for _, e := range events {
				for _, a := range eventArgs(e) {
					if a == "--no-verify" || strings.Contains(a, "hooksPath") {
						t.Errorf("agent used a hook bypass workaround: %s %v", e.Command, eventArgs(e))
					}
				}
			}

			// Discriminating assertion (Req 6): artifact dirt never blocks —
			// no `mindspec complete` invocation may fail. Pre-fix, the hook's
			// re-export during the auto-commit re-dirties the tree and the
			// plain IsTreeClean check rejects the first attempt.
			firstFailed := firstEventIndex(events, func(e ActionEvent) bool {
				return e.Command == "mindspec" && e.ExitCode != 0 && containsAll(eventArgs(e), "complete")
			})
			if firstFailed >= 0 {
				t.Errorf("mindspec complete failed (exit=%d) — artifact dirt blocked completion: %v",
					events[firstFailed].ExitCode, eventArgs(events[firstFailed]))

				// Loophole closure (spec 092 NEW-6): the eventual success must
				// not be a manual artifact-commit recovery.
				for _, e := range events[firstFailed+1:] {
					if e.Command != "git" {
						continue
					}
					args := eventArgs(e)
					if !containsAll(args, "add") && !containsAll(args, "commit") {
						continue
					}
					for _, a := range args {
						if strings.Contains(a, ".beads") {
							t.Errorf("agent manually committed the beads artifact after the failed complete: %v", args)
							break
						}
					}
				}
			}
		},
	}
}

// ScenarioWrongDirectoryGuardRecovery pins field note mindspec-tjat: the
// agent starts at the repo root where the HUMAN has unrelated uncommitted
// work, while the ready bead's claim should happen from (or be steered to)
// the spec worktree. Pre-fix, `mindspec next`'s dirty-tree failure names no
// worktree context and its recovery steps suggest committing or DISCARDING
// the user's changes — so agents stash, commit, or `git restore .` the
// human's dirt. The only passing path is the guard steering the agent to
// the worktree while main's dirt survives untouched.
func ScenarioWrongDirectoryGuardRecovery() Scenario {
	var epicID, beadID string
	const dirtyContent = "user WIP: do not touch\nline two of the human's draft\n"
	return Scenario{
		Name:        "wrong_directory_guard_recovery",
		Description: "dirty-main guard failure steers the agent to the worktree without touching user dirt",
		MaxTurns:    30,
		TimeoutMin:  10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-wrongdir"

			epicID = sandbox.CreateSpecEpic(specID)
			beadID = sandbox.CreateBead("["+specID+"] Create wrongdir.go", "task", epicID)
			keepaliveID := sandbox.CreateBead("["+specID+"] future: follow-up", "task", epicID)
			sandbox.runBDMust("defer", keepaliveID)

			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/spec.md", `---
title: Wrong Directory Feature
status: Approved
---
# Wrong Directory Feature
Add a wrongdir.go file.
`)
			sandbox.WriteFile(".mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Create wrongdir.go
Create wrongdir.go with a WrongDir() function.
`, specID, beadID))

			// Tracked file committed clean, then modified: the human's
			// work-in-progress dirt on main.
			sandbox.WriteFile("notes.txt", "clean baseline notes\n")
			sandbox.Commit("setup: approved spec/plan + notes baseline")

			// Plan-phase worktree only — the agent must claim via
			// mindspec next, which is where the dirty-tree guard fires.
			setupWorktrees(sandbox, specID, "", "plan")
			sandbox.Commit("setup: plan mode")

			// Pre-seed the user dirt LAST so it stays uncommitted.
			sandbox.WriteFile("notes.txt", dirtyContent)
			if sandbox.GitStatusClean() {
				return fmt.Errorf("precondition: expected notes.txt dirty on main, but tree is clean")
			}
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The file notes.txt contains the user's unrelated work-in-progress — leave it
exactly as it is: never stash, commit, restore, or modify it.

Spec 001-wrongdir has one ready bead. Claim it with mindspec next, create
wrongdir.go with a function WrongDir() string that returns "ok", and finish
the bead through the MindSpec lifecycle. If a command fails, read the error
carefully and follow its guidance.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// A later mindspec next succeeded — the agent found the passing path.
			assertCommandSucceeded(t, events, "mindspec", "next")

			// No git stash ANYWHERE in the event stream (any exit code).
			for _, e := range events {
				if e.Command == "git" && containsAll(eventArgs(e), "stash") {
					t.Errorf("agent ran git stash on the user's dirt: %v", eventArgs(e))
				}
			}

			// Main's pre-seeded dirt survives byte-identical...
			if got := sandbox.ReadFile("notes.txt"); got != dirtyContent {
				t.Errorf("notes.txt content was modified:\n got: %q\nwant: %q", got, dirtyContent)
			}
			// ...and is still UNCOMMITTED at root (committing it away would
			// also be touching the human's work).
			cmd := exec.Command("git", "status", "--porcelain", "--", "notes.txt")
			cmd.Dir = sandbox.Root
			out, err := cmd.Output()
			if err != nil {
				t.Errorf("git status notes.txt: %v", err)
			} else if !strings.Contains(string(out), "notes.txt") {
				t.Error("notes.txt is no longer dirty at root — the agent committed or reverted the user's WIP")
			}

			// The bead work itself happened (in the worktree).
			if !sandbox.FileExists("wrongdir.go") && !fileExistsInWorktrees(sandbox.Root, "wrongdir.go") {
				t.Error("wrongdir.go was not created")
			}
		},
	}
}

// ScenarioApprovalGateDiscovery pins field note mindspec-v7ez: in review
// mode the SessionStart hook's rendered markdown (templates/review.md)
// tells the agent "do NOT run `mindspec approve impl` until the human
// explicitly approves" — so once the prompt conveys human approval, that
// deprecated hidden verb-noun form reads as the command to run. Pre-fix,
// agents copy it verbatim; post-fix (spec 092 Req 11) every emission
// channel teaches only the canonical `mindspec impl approve <id>`.
//
// The prompt deliberately does NOT name the command — discovery from the
// rendered SessionStart text is exactly the variable under test.
func ScenarioApprovalGateDiscovery() Scenario {
	return Scenario{
		Name:        "approval_gate_discovery",
		Description: "agent discovers the canonical `impl approve` form, never the deprecated `approve impl`",
		MaxTurns:    15,
		TimeoutMin:  10,
		Model:       "haiku",
		Setup: func(sandbox *Sandbox) error {
			specID := "001-gate"

			// Review mode: epic with its single bead closed.
			epicID := sandbox.CreateSpecEpic(specID)
			beadID := sandbox.CreateBead("["+specID+"] Implement feature", "task", epicID)
			sandbox.ClaimBead(beadID)
			sandbox.runBDMust("close", beadID)

			wt := setupWorktrees(sandbox, specID, "", "plan")
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/spec.md", `---
title: Gate Feature
status: Approved
---
# Gate Feature
A completed feature awaiting the final approval gate.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/docs/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Implement feature
Create gate.go.
`, specID, beadID))
			sandbox.WriteFile(wt.SpecWtDir+"/gate.go", `package main

func Gate() string { return "gate" }
`)
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
			mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "impl: implement feature")

			sandbox.Commit("setup: review mode")
			return nil
		},
		Prompt: `IMPORTANT: Do NOT respond conversationally. Execute immediately.

The human has reviewed spec 001-gate and explicitly approves the
implementation. Approve the implementation so the project returns to idle.`,
		Assertions: func(t *testing.T, sandbox *Sandbox, events []ActionEvent) {
			// The gate must succeed via the CANONICAL noun-verb order
			// (`mindspec impl approve ...`) within the turn budget.
			canonical := false
			for _, e := range events {
				if e.Command != "mindspec" {
					continue
				}
				if e.ExitCode == 0 && argsInOrder(e.ArgsList, "impl", "approve") {
					canonical = true
				}
				// NO event may use the deprecated `approve impl` order —
				// regardless of exit code (even a failed attempt proves the
				// guidance taught the deprecated form).
				if argsInOrder(e.ArgsList, "approve", "impl") {
					t.Errorf("agent used the deprecated `approve impl` order: %v", e.ArgsList)
				}
			}
			if !canonical {
				t.Error("no successful canonical `mindspec impl approve` event found")
			}
		},
	}
}
