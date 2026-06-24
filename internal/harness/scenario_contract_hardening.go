package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// writeSandboxDomainCoverage writes the file half of the minimal
// ADR-divergence coverage triple for sandbox fixtures (run-1
// adjudication, review/scenario1-fix-design.md §1): an OWNERSHIP.yaml
// claiming the given fixture source files for the `sandbox` domain,
// plus an Accepted ADR-0001 covering that domain. Callers must
// (a) COMMIT these at the sandbox ROOT BEFORE the worktree fork so
// every branch carries them (ownership + ADR resolve root-relative:
// internal/validate/ownership.go, adr.NewFileStore(root)), and
// (b) add `adr_citations:\n- ADR-0001` to the fixture plan.md
// frontmatter — IsDomainCovered requires the covering ADR to be
// plan-CITED. Without the triple, the pre-existing spec-087
// ADR-divergence gate flags any committed fixture .go file as
// `adr-divergence-unowned`, and a perfectly-behaved `impl approve` /
// `complete` exits 1 on a gate unrelated to the scenario's pin.
func writeSandboxDomainCoverage(sandbox *Sandbox, files ...string) {
	var b strings.Builder
	b.WriteString("paths:\n")
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	sandbox.WriteFile(".mindspec/domains/sandbox/OWNERSHIP.yaml", b.String())
	sandbox.WriteFile(".mindspec/adr/ADR-0001-sandbox-domain.md", `# ADR-0001: Sandbox Domain

- **Date**: 2026-06-11
- **Status**: Accepted
- **Domain(s)**: sandbox

## Decision

Scenario fixture source files at the sandbox root belong to the
sandbox domain. This is the minimal coverage triple that lets
lifecycle gates pass on the merits in harness sandboxes, mirroring how
a compliant field repo carries ownership + ADR coverage.
`)
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
// surgery. Post-fix (spec 092 Req 1) the gate re-derives the phase from
// children and self-heals; the load-bearing discriminators are the
// deterministic assertStaleApproveSelfHeals probe, the no-gate-bypass
// guard, and the exit-0-on-the-merits success assertion (surgery events
// are logged informationally — stop-#2 adjudication fallback).
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

			// ADR-divergence coverage triple (run-1 adjudication): stale.go
			// on the spec branch must pass the spec-087 gate on the merits,
			// otherwise every clean `impl approve` exits 1 on a gate
			// unrelated to this scenario's pin. Committed pre-fork so both
			// main and the spec branch carry it.
			writeSandboxDomainCoverage(sandbox, "stale.go")
			sandbox.Commit("setup: sandbox domain coverage (ownership + ADR-0001)")

			// Spec-worktree-only topology: spec branch + worktree, NO bead
			// branches — the stale phase is the only blocking condition.
			wt := setupWorktrees(sandbox, specID, "", "plan")

			// Spec/plan/impl artifacts committed on the spec branch (doc-sync
			// gate and FinalizeEpic need real content; bead_ids lets the
			// plan-bead gate verify the closed bead).
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", `---
title: Stale Phase Feature
status: Approved
---
# Stale Phase Feature
A completed feature whose epic phase cache went stale.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
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

			for _, e := range events {
				args := eventArgs(e)
				// Raw metadata surgery — INFORMATIONAL ONLY (stop-#2
				// adjudication fallback, DQ-7/doomed precedent): in three
				// post-fix runs (1a/1b/1c, the last after the R1-M1
				// guidance fix) the haiku agent gratuitously ran `bd update
				// --metadata` during orientation while sanctioned paths
				// existed, worked, and were ALSO used — model
				// belt-and-braces disposition, not a product gap (it did
				// NOT fire at the Bead 2 baseline). The load-bearing 3smk
				// discriminators are deterministic: the
				// assertStaleApproveSelfHeals probe below (the gate
				// self-heals; the reconcile event fires) plus the hard
				// no-bypass guard and the exit-0-on-the-merits success
				// assertion. Logged for run-report color instead of failed.
				if e.Command == "bd" && containsAll(args, "update") {
					for _, a := range args {
						// Catches --metadata, --metadata=..., --set-metadata,
						// and --set-metadata=... — all raw surgery paths.
						if strings.HasPrefix(a, "--metadata") || strings.HasPrefix(a, "--set-metadata") {
							t.Logf("informational (downgraded per stop-#2 adjudication): agent performed raw bd metadata surgery: %v", args)
							break
						}
					}
				}
				// No gate bypass: with the coverage fixture in place nothing
				// legitimately diverges, so an override-assisted approval
				// means a fixture/gate confound is being masked, not healed.
				if e.Command == "mindspec" {
					for _, a := range args {
						if a == "--override-adr" || a == "--supersede-adr" || a == "--allow-doc-skew" {
							t.Errorf("agent bypassed an approval gate (%s) — the gate must pass on the merits: %v", a, args)
						}
					}
				}
				// NOTE (Req 22 redesign, run-1 adjudication): `mindspec
				// repair phase` is NOT forbidden — it is the recovery
				// command Req 2/19 themselves advertise on every
				// phase-deriving command while the stored phase is stale.
				// Punishing it would test against the product's own
				// contract. Req 1's gate self-heal is pinned
				// deterministically by assertStaleApproveSelfHeals below
				// (the doomed-worktree probe precedent) and by the 3smk
				// unit AC.
				if e.Command == "mindspec" && containsAll(args, "repair") {
					t.Logf("informational: agent used sanctioned `mindspec repair`: %v", args)
				}
			}

			// DISCRIMINATING deterministic probe (3smk / spec 092 Req 1): a
			// fresh stale-phase spec approved by the binary directly must
			// exit 0 and emit the lifecycle.phase_reconciled self-heal
			// event line.
			assertStaleApproveSelfHeals(t, sandbox)
		},
	}
}

// assertStaleApproveSelfHeals is the deterministic post-session probe for
// spec 092 Req 1 (mindspec-3smk), per the doomed-worktree probe precedent
// (assertDoomedCompleteEmitsCdNote): build a SECOND stale-phase spec with
// real bd state and a docs-only spec branch (so the ADR-divergence gate
// no-ops and needs no coverage triple), then run the sandbox binary's
// `mindspec impl approve` directly. Pre-fix the phase gate trusts the
// stale stored phase and exits non-zero, and the string
// `event=lifecycle.phase_reconciled` does not exist in the binary — so
// this fails deterministically at the pinned baseline regardless of
// anything the LLM half did.
func assertStaleApproveSelfHeals(t *testing.T, sandbox *Sandbox) {
	t.Helper()

	specID := "002-staleprobe"
	epicID := sandbox.CreateSpecEpic(specID)
	beadID := sandbox.CreateBead("["+specID+"] Probe feature", "task", epicID)
	sandbox.ClaimBead(beadID)
	sandbox.runBDMust("close", beadID)
	// Stale stored phase while every child is closed (derived = review).
	// Test-side setup write — only the agent event stream is scanned by
	// the no-surgery assertion. spec_num/spec_title repeated to preserve
	// the ADR-0023 epic binding (bd update --metadata replaces the map).
	sandbox.runBDMust("update", epicID, "--metadata",
		`{"spec_num":2,"spec_title":"staleprobe","mindspec_phase":"implement"}`)

	wt := setupWorktrees(sandbox, specID, "", "plan")
	sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", `---
title: Stale Probe Feature
status: Approved
---
# Stale Probe Feature
Deterministic Req 1 self-heal probe (docs-only spec branch).
`)
	sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
---
# Plan
## Bead 1: Probe feature
Docs-only probe; the work is the lifecycle close itself.
`, specID, beadID))
	mustRunGit(sandbox, "-C", wt.SpecWtDir, "add", "-A")
	mustRunGit(sandbox, "-C", wt.SpecWtDir, "commit", "-m", "docs: staleprobe spec+plan")

	cmd := exec.Command(filepath.Join(sandbox.mindspecBinDir, "mindspec"),
		"impl", "approve", specID)
	cmd.Dir = sandbox.Root
	cmd.Env = sandbox.Env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("stale-phase `mindspec impl approve %s` must self-heal and exit 0 (3smk / spec 092 Req 1): %v\nstdout:\n%s\nstderr:\n%s",
			specID, err, stdout.String(), stderr.String())
	}
	// The Req 1 self-heal demonstrably executed end-to-end: the HC-3
	// structured event line names the stored→derived reconcile.
	for _, want := range []string{"event=lifecycle.phase_reconciled", "stored=implement", "derived=review"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("impl approve stderr lacks %q — the Req 1 phase reconcile did not execute; stderr:\n%s\nstdout:\n%s",
				want, stderr.String(), stdout.String())
		}
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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Doomed Worktree Feature
status: Approved
---
# Doomed Worktree Feature
Add a doomed function.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
---
# Plan
## Bead 1: Implement doomed
Create doomed.go with a Doomed function.
`, specID, beadID))
			// ADR-divergence coverage triple (run-1 adjudication, applied
			// scenario-wide): doomed.go and the cd-note probe's probe.go
			// must pass the spec-087 gate on the merits if complete's diff
			// range ever includes them (HEAD-resolution dependent).
			writeSandboxDomainCoverage(sandbox, "doomed.go", "probe.go")
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

// lastNonEmptyLine returns the last non-empty (after TrimSpace) line of s,
// or "" when none exists.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// assertDoomedCompleteEmitsCdNote claims a fresh probe bead, builds its
// worktree nested under the spec worktree, commits work there, and runs
// `mindspec complete` with the process cwd INSIDE that worktree. It asserts
// exit 0 and the Req 4 cd-back NOTE as the LAST non-empty line of STDOUT,
// carrying the copy-pastable `run: cd <root>` command (post-panel B7: the
// unit half is pinned in cmd/mindspec/cwdsafety_test.go — completeTail
// emits the NOTE after FormatResult and the instruct tail; this closes the
// end-to-end half). Pre-fix the NOTE does not exist anywhere in the
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("mindspec complete from inside its own bead worktree exited non-zero (qxsy): %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "working directory was removed") {
		t.Errorf("complete stdout lacks the cd-back NOTE (\"your shell's working directory was removed — run: cd <root>\", spec 092 Req 4); stdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}
	// Post-panel B7 tightening: the NOTE is the LAST non-empty stdout line
	// (after FormatResult and the instruct tail — the placement the agent's
	// shell actually sees) and carries the copy-pastable `run: cd` command.
	note := lastNonEmptyLine(stdout.String())
	if !strings.Contains(note, "working directory was removed") || !strings.Contains(note, "run: cd ") {
		t.Errorf("last non-empty stdout line is not the cd-back NOTE with its `run: cd <root>` command (spec 092 Req 4); last line: %q\nfull stdout:\n%s",
			note, stdout.String())
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

// isHooksPathAssignment reports whether a git invocation's args ASSIGN
// core.hooksPath — either the inline `-c core.hooksPath=<path>` override
// or `git config [scope] core.hooksPath <path>`. Read-only forms
// (`--get`, `--get-all`, `--get-regexp`, `--list`) and unsets are NOT
// assignments (post-panel B8b: flagging bare "hooksPath" substrings
// false-failed on read-only diagnostics).
func isHooksPathAssignment(args []string) bool {
	for i, a := range args {
		if strings.Contains(a, "core.hooksPath=") {
			return true // -c core.hooksPath=<path>
		}
		if a != "core.hooksPath" {
			continue
		}
		readOnly := false
		for _, b := range args {
			switch b {
			case "--get", "--get-all", "--get-regexp", "--list", "--unset", "--unset-all":
				readOnly = true
			}
		}
		if !readOnly && i+1 < len(args) {
			return true // config core.hooksPath <path>
		}
	}
	return false
}

// assertNoManualArtifactCommit flags any agent-issued git add/commit that
// touches (or sweeps up) the beads artifact before the first SUCCESSFUL
// `mindspec complete` (post-panel B8). Matched forms:
//   - explicit `.beads` pathspec on add or commit;
//   - commit with all-tracked staging flags (-a / -am / --all);
//   - bare commit after a blanket stage (`git add -A` / `--all` / `.`)
//     earlier in the window — the staged artifact rides along.
//
// A blanket `git add` ALONE is not flagged: `mindspec complete`'s
// sanctioned auto-commit path re-exports and commits whatever is staged
// through the executor, so staging without a manual commit is not the
// manual-recovery loophole.
func assertNoManualArtifactCommit(t *testing.T, events []ActionEvent) {
	t.Helper()
	for _, v := range manualArtifactCommitViolations(events) {
		t.Errorf("%s", v)
	}
}

// manualArtifactCommitViolations is the pure evidence-gating core behind
// assertNoManualArtifactCommit, extracted so the attribution logic is
// unit-testable against synthetic event streams (panel R2-M1/R3). It
// SCANS only events before the first successful complete, but
// ATTRIBUTES against the UNTRUNCATED stream: the shim logs each command
// at exit, so the parent mindspec-complete event lands AT (or after)
// the scan boundary, AFTER its own git children — truncating the
// attribution slice to the scan window excluded the parent and made
// attribution work only by accident of the hook-children's ±1s slack
// (panel R3-MAJ-1).
func manualArtifactCommitViolations(events []ActionEvent) []string {
	window := events
	if firstOK := firstEventIndex(events, func(e ActionEvent) bool {
		return e.Command == "mindspec" && e.ExitCode == 0 && containsAll(eventArgs(e), "complete")
	}); firstOK >= 0 {
		window = events[:firstOK]
	}

	namesBeads := func(args []string) bool {
		for _, a := range args {
			if strings.Contains(a, ".beads") {
				return true
			}
		}
		return false
	}

	var violations []string
	artifactStaged := false
	for _, e := range window {
		if e.Command != "git" {
			continue
		}
		args := eventArgs(e)
		switch {
		case containsAll(args, "add"):
			// Explicit .beads pathspec: only the agent ever passes one
			// (mindspec's executor stages via blanket `git add -A`, bd
			// never git-adds) — flag unconditionally.
			if namesBeads(args) {
				violations = append(violations,
					fmt.Sprintf("agent staged the beads artifact before the first successful complete: %v", args))
				artifactStaged = true
				continue
			}
			// Blanket stage: attribution matters — mindspec's own
			// CommitAll runs `git add -A` as a recorded subprocess
			// (run-3 false-positive fix, see mindspecSpawnedGit). The
			// FULL stream is passed so the parent's window is visible.
			if !mindspecSpawnedGit(events, e) {
				for _, a := range args {
					if a == "-A" || a == "--all" || a == "." || a == "-u" {
						artifactStaged = true
					}
				}
			}
		case containsAll(args, "commit"):
			// Explicit .beads pathspec on a commit: agent-only, flag
			// unconditionally.
			if namesBeads(args) {
				violations = append(violations,
					fmt.Sprintf("agent manually committed the beads artifact before the first successful complete: %v", args))
				continue
			}
			// Blanket-form commits need attribution: mindspec's
			// auto-commit and `chore: sync beads artifact` follow-up are
			// recorded git subprocesses of the complete event.
			if mindspecSpawnedGit(events, e) {
				continue
			}
			allTracked := false
			for _, a := range args {
				if a == "-a" || a == "-am" || a == "--all" {
					allTracked = true
				}
			}
			if allTracked || artifactStaged {
				violations = append(violations,
					fmt.Sprintf("agent manually committed the beads artifact before the first successful complete: %v", args))
			}
		}
	}
	return violations
}

// mindspecSpawnedGit reports whether git event g falls within the
// execution window of a recorded mindspec command in all. The PATH shim
// records mindspec's OWN git subprocesses (complete's auto-commit, the
// Req 7 `chore: sync beads artifact` follow-up) indistinguishably from
// agent-issued ones — the run-3 false positive that failed a perfectly
// sanctioned completion. Shim timestamps are END times (recorder.go
// logs after the real binary exits), so a child's end lies within
// [parentEnd-parentDuration, parentEnd]; ±1s slack absorbs the shim's
// second-resolution timestamps.
//
// `all` MUST be the untruncated event stream (panel R3-MAJ-1): exit-
// time logging puts the parent mindspec event AFTER its git children in
// the stream — at or beyond any scan-window boundary — so a slice
// truncated at the first successful complete excludes the very parent
// whose duration window performs the attribution. Used ONLY for the
// fuzzy blanket-form heuristics — explicit `.beads` pathspecs are
// flagged regardless of attribution (mindspec never passes one).
func mindspecSpawnedGit(all []ActionEvent, g ActionEvent) bool {
	if g.Timestamp.IsZero() {
		return false
	}
	gEnd := g.Timestamp
	for _, m := range all {
		if m.Command != "mindspec" || m.Timestamp.IsZero() || m.DurationMS <= 0 {
			continue
		}
		mEnd := m.Timestamp
		mStart := mEnd.Add(-m.Duration())
		if !gEnd.Before(mStart.Add(-time.Second)) && !gEnd.After(mEnd.Add(time.Second)) {
			return true
		}
	}
	return false
}

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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Reexport Feature
status: Approved
---
# Reexport Feature
Add a reexport.go file.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
---
# Plan
## Bead 1: Create reexport.go
Create reexport.go with a Reexport() function.
`, specID, beadID))

			// ADR-divergence coverage triple (run-1 adjudication, applied
			// scenario-wide): the agent-authored reexport.go must pass the
			// spec-087 gate on the merits if complete's diff range ever
			// includes it (HEAD-resolution dependent).
			writeSandboxDomainCoverage(sandbox, "reexport.go")

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

			// Field workarounds are forbidden (i4ad): no --no-verify
			// anywhere, and no core.hooksPath ASSIGNMENT (post-panel B8b:
			// read-only diagnostics like `git config --get core.hooksPath`
			// are legitimate orientation, not a bypass — only setting the
			// hooks path reroutes the pre-commit hook under test).
			for _, e := range events {
				args := eventArgs(e)
				for _, a := range args {
					if a == "--no-verify" {
						t.Errorf("agent used a hook bypass workaround: %s %v", e.Command, args)
					}
				}
				if e.Command == "git" && isHooksPathAssignment(args) {
					t.Errorf("agent reassigned core.hooksPath (hook bypass workaround): %s %v", e.Command, args)
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
			}

			// Loophole closure (spec 092 NEW-6, post-panel B8 tightening):
			// no agent-issued git add/commit may touch the beads artifact
			// BEFORE the first successful complete — regardless of whether
			// a complete failed first. The old detector only scanned events
			// after a failed complete and only matched explicit `.beads`
			// args, so `git add -A` + bare `git commit -am` (or a
			// pre-emptive commit before the first complete attempt) evaded
			// it.
			assertNoManualArtifactCommit(t, events)
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

			sandbox.WriteFile(".mindspec/specs/"+specID+"/spec.md", `---
title: Wrong Directory Feature
status: Approved
---
# Wrong Directory Feature
Add a wrongdir.go file.
`)
			sandbox.WriteFile(".mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
---
# Plan
## Bead 1: Create wrongdir.go
Create wrongdir.go with a WrongDir() function.
`, specID, beadID))

			// ADR-divergence coverage triple (run-1 adjudication, applied
			// scenario-wide): the agent-authored wrongdir.go must pass the
			// spec-087 gate on the merits if complete's diff range ever
			// includes it (HEAD-resolution dependent).
			writeSandboxDomainCoverage(sandbox, "wrongdir.go")

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

			// ADR-divergence coverage triple (run-1 adjudication): gate.go
			// on the spec branch hits the IDENTICAL impl-approve confound
			// the stale_phase run exposed — without coverage, no canonical
			// `impl approve` can succeed un-bypassed. Committed pre-fork.
			writeSandboxDomainCoverage(sandbox, "gate.go")
			sandbox.Commit("setup: sandbox domain coverage (ownership + ADR-0001)")

			wt := setupWorktrees(sandbox, specID, "", "plan")
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/spec.md", `---
title: Gate Feature
status: Approved
---
# Gate Feature
A completed feature awaiting the final approval gate.
`)
			sandbox.WriteFile(wt.SpecWtDir+"/.mindspec/specs/"+specID+"/plan.md", fmt.Sprintf(`---
status: Approved
spec_id: %s
bead_ids:
- %s
adr_citations:
- ADR-0001
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
				// Post-panel B8c: use the order-preserving accessor
				// (ArgsList when shim-recorded, else the positional
				// flatArgs of the Args map) instead of raw e.ArgsList, so
				// Args-map-only events can't silently escape the
				// deprecated-order check.
				args := eventArgsList(e)
				if e.ExitCode == 0 && argsInOrder(args, "impl", "approve") {
					canonical = true
				}
				// NO event may use the deprecated `approve impl` order —
				// regardless of exit code (even a failed attempt proves the
				// guidance taught the deprecated form).
				if argsInOrder(args, "approve", "impl") {
					t.Errorf("agent used the deprecated `approve impl` order: %v", args)
				}
			}
			if !canonical {
				t.Error("no successful canonical `mindspec impl approve` event found")
			}
		},
	}
}
