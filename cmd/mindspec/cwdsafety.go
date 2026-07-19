package main

// Terminal-command cwd safety (spec 092-agent-contract-hardening,
// Reqs 3b/3c/4, field note mindspec-qxsy).
//
// The two terminal lifecycle commands — `mindspec complete` and
// `mindspec impl approve` — remove the very worktree the user's shell
// may be sitting in. The mindspec process can repair its OWN cwd
// (os.Chdir to the repo root after the terminal mutation), but it can
// never change the invoking shell's cwd. The cd-back NOTE below is the
// only available channel: when the invocation directory is gone after
// the mutation, the LAST line of stdout tells the agent exactly what to
// paste.

import (
	"fmt"
	"io"
	"os"

	"github.com/mrmaxsteel/mindspec/internal/workspace/containment"
)

// captureInvocationCwd records the directory the shell invoked mindspec
// from. It MUST be called at command entry, BEFORE any auto-chdir
// (spec 092 Req 4). Best-effort: an empty return disables the cd-back
// NOTE (a process that cannot resolve its own cwd at entry has nothing
// meaningful to stat later).
func captureInvocationCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// emitCdBackNote writes the Req 4 cd-back NOTE when the invocation cwd
// no longer exists after a terminal mutation. Callers invoke it as the
// FINAL stdout write of the command so the NOTE is the last line of
// stdout. No-op while the invocation directory still exists (or was
// never captured).
//
// The wording is pinned by the Bead-2 regression probe
// (internal/harness/scenario_contract_hardening.go,
// assertDoomedCompleteEmitsCdNote), which asserts the substring
// "working directory was removed".
//
// root here is the trusted, operator-chosen repo root (a ROOT-ONLY sink,
// R5/ADR-0042 §4): it is never subjected to containment.CheckContainment
// and this function never refuses. It still routes through
// containment.EmitCd for the same conditional shell-safe quoting every
// other executable-`cd` render gets — defense-in-depth against a
// metacharacter-bearing root, not a validation claim.
func emitCdBackNote(w io.Writer, invocationCwd, root string) {
	if invocationCwd == "" {
		return
	}
	if _, err := os.Stat(invocationCwd); err == nil {
		return
	}
	fmt.Fprintf(w, "NOTE: your shell's working directory was removed — run: %s\n", containment.EmitCd(root))
}
