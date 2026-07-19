package harness

// Spec 121 Bead 3: the protected-main finalize-PR automation's harness
// end-to-end scenario. Unlike the agent-driven Scenario_* fixtures in
// this package, this is a DETERMINISTIC scenario (no LLM turn loop): it
// drives the REAL built `mindspec` binary via Sandbox.Run and asserts on
// what the finalize-PR automation (cmd/mindspec/finalize_pr.go) actually
// did — the same "recording gh shim + a scripted delegate on PATH"
// pattern the writeFakeBD precedent uses for `bd`
// (r7_hostile_test.go:127-160), applied to `gh` (recorder.go's
// DefaultShimCommands already lists it).
//
// The recording shim (recorder.go) RECORDS-AND-DELEGATES: it logs the
// invocation, then execs whatever `findRealBinary` resolves for the
// command name at shim-install time. Prepending a directory holding a
// SCRIPTED FAKE `gh` to PATH before NewSandbox runs makes that resolve to
// the fake, not any real `gh` on the host — so the fake's own per-call
// argv log (below) becomes the reliable end-to-end assertion source (the
// shim's own JSONL args_list reconstruction line-splits on ANY embedded
// newline in an arg value — e.g. this automation's multi-line PR body —
// a pre-existing recorder.go limitation unrelated to spec 121, so
// assertions here read the fake gh's own flat argv log instead).

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGHScript is a stateful POSIX-shell fake `gh` covering the four
// subcommand shapes the finalize-PR automation issues: `pr list` (both
// the unfiltered R1 lookup and the --base-filtered R3 reconcile query),
// `pr create`, `pr checks`, and `pr merge`. State (per-head PR
// number/state) persists in flat files under stateDir so a SECOND
// `mindspec impl approve` invocation (the AC-2 re-run/adoption case)
// observes the first invocation's PR as already open. Every invocation
// additionally appends its full argv, one arg per line with a
// "---CALL---" record separator, to callLogPath — this is the assertion
// source (see the package doc above for why).
const fakeGHScript = `#!/bin/sh
STATE_DIR="%[1]s"
CALL_LOG="%[2]s"
mkdir -p "$STATE_DIR"
{
  printf '%%s\n' "$@"
  echo "---CALL---"
} >> "$CALL_LOG"

head=""
prev=""
for a in "$@"; do
  if [ "$prev" = "--head" ]; then head="$a"; fi
  prev="$a"
done
STATE_FILE="$STATE_DIR/pr-$(printf '%%s' "$head" | tr -c 'a-zA-Z0-9._-' '_')"

case "$1 $2" in
  "pr create")
    if [ -f "$STATE_FILE" ]; then
      echo "gh: a pull request for branch \"$head\" into branch \"main\" already exists" >&2
      exit 1
    fi
    echo "OPEN" > "$STATE_FILE"
    echo "https://github.com/acme/mindspec-fixture/pull/121"
    exit 0
    ;;
  "pr list")
    st=""
    [ -f "$STATE_FILE" ] && st=$(cat "$STATE_FILE")
    if [ -z "$st" ]; then
      echo "[]"
    else
      printf '[{"number":121,"state":"%%s","url":"https://github.com/acme/mindspec-fixture/pull/121","headRefName":"%%s","baseRefName":"main"}]\n' "$st" "$head"
    fi
    exit 0
    ;;
  "pr checks")
    echo '[{"name":"ci/build","state":"SUCCESS"}]'
    exit 0
    ;;
  "pr merge")
    for f in "$STATE_DIR"/pr-*; do
      [ -f "$f" ] && echo "MERGED" > "$f"
    done
    exit 0
    ;;
  *)
    echo "fixture fake gh: unhandled invocation: $*" >&2
    exit 1
    ;;
esac
`

// writeFakeGH installs the scripted fake `gh` at the FRONT of PATH (the
// writeFakeBD precedent, adapted): findRealBinary resolves the FIRST
// match on PATH, so this must run BEFORE NewSandbox so its own PATH-
// prepend + InstallShims call picks up the fake, not any real `gh` the
// host has installed. Returns the state/call-log paths and a restore
// func.
func writeFakeGH(t *testing.T) (stateDir, callLogPath string, restore func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shim script assumes a POSIX shell")
	}
	binDir := t.TempDir()
	stateDir = t.TempDir()
	callLogPath = filepath.Join(t.TempDir(), "gh-calls.log")

	script := fmt.Sprintf(fakeGHScript, stateDir, callLogPath)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake gh: %v", err)
	}
	origPath := os.Getenv("PATH")
	newPath := binDir
	// Defense against a stale globally-installed `mindspec` shadowing the
	// project's own build on some developer PATH ahead of binDir: Go's
	// exec.Command resolves PATH at CALL time from the process's own env
	// (not from Sandbox.Env()'s cmd.Env), so Sandbox.Run("mindspec", ...)
	// below is exposed to whatever "mindspec" this TEST process's ambient
	// PATH resolves — pin the project's freshly-built one ahead of it too.
	if mindspecBin := projectBinDir(); mindspecBin != "" {
		newPath += string(os.PathListSeparator) + mindspecBin
	}
	newPath += string(os.PathListSeparator) + origPath
	os.Setenv("PATH", newPath)
	return stateDir, callLogPath, func() { os.Setenv("PATH", origPath) }
}

// fakeGHCalls parses callLogPath into one []string per recorded
// invocation (see fakeGHScript's "---CALL---" record separator).
func fakeGHCalls(t *testing.T, callLogPath string) [][]string {
	t.Helper()
	data, err := os.ReadFile(callLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading fake gh call log: %v", err)
	}
	var calls [][]string
	var cur []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "---CALL---" {
			calls = append(calls, cur)
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return calls
}

// addBareOriginRemote creates a real bare git repo, wires it as
// sandbox's `origin` remote, and pushes local `main` to it — the
// precondition `gitutil.HasRemote()` (mindspec_executor.go's finalize
// probe) needs to route through the "pr" merge-strategy leg at all.
func addBareOriginRemote(t *testing.T, sandbox *Sandbox) (originDir string) {
	t.Helper()
	originDir = t.TempDir()
	mustRun(t, "", "git", "init", "--bare", "--initial-branch=main", originDir)
	mustRunGit(sandbox, "remote", "add", "origin", originDir)
	mustRunGit(sandbox, "push", "origin", "main")
	return originDir
}

// pushBranchToOriginMain simulates "the impl PR already merged" (the
// protected-main finalize orphan trigger, mindspec-uxl4): specBranch was
// branched from main, so pushing it directly onto origin's main ref is a
// genuine fast-forward — preFinalizeTip becomes an ancestor of
// origin/main exactly as a real merged PR would leave it, with NO faked
// ancestry.
func pushBranchToOriginMain(t *testing.T, sandbox *Sandbox, specBranch string) {
	t.Helper()
	mustRunGit(sandbox, "push", "origin", specBranch+":main")
}
