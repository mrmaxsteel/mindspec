package complete

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// recordBdExport puts a bd on PATH that logs every export call and writes
// the export the way `bd export -o .beads/issues.jsonl` does.
func recordBdExport(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "exports.log")
	t.Setenv("BD_EXPORT_LOG", log)
	script := `#!/bin/sh
case "$1" in
  show)   echo '[{"metadata":{}}]' ;;
  update) ;;
  export)
    printf '%s\n' "$PWD" >> "$BD_EXPORT_LOG"
    mkdir -p .beads && printf '{"id":"window-1","status":"closed"}\n' > .beads/issues.jsonl ;;
  *)      echo "fake bd: unsupported subcommand $1" >&2; exit 1 ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func requireNoExport(t *testing.T, log string) {
	t.Helper()
	if data, err := os.ReadFile(log); !os.IsNotExist(err) {
		t.Fatalf("bd export ran in a repo that ignores the export: %q %v", data, err)
	}
}

func requireExportNotStaged(t *testing.T, f *windowFixture) {
	t.Helper()
	if _, err := f.ex.FileAtRef("spec/"+closingSpec, ".beads/issues.jsonl"); err == nil {
		t.Fatal("the ignored export was committed to the spec branch")
	}
	staged := gitOutput(t, f.specWT, "diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) != "" {
		t.Fatalf("index not clean after close: %q", staged)
	}
	if tracked := gitOutput(t, f.specWT, "ls-files", "--", ".beads/issues.jsonl"); strings.TrimSpace(tracked) != "" {
		t.Fatalf("the ignored export is in the index: %q", tracked)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestComplete_IgnoredExportNoLedgersMakesNoCommit(t *testing.T) {
	f := newWindowFixtureExport(t, true, true)
	f.ex.realPublish = true
	log := recordBdExport(t)
	m := f.merge()
	r, err := f.run(CompleteOpts{})
	if err != nil || r == nil || !r.BeadClosed || !r.WorktreeRemoved {
		t.Fatalf("close in a repo that ignores the export failed: %+v %v", r, err)
	}
	requireNoExport(t, log)
	if len(f.ex.publishCalls) != 0 {
		t.Fatalf("publication ran with nothing to publish: %v", f.ex.publishCalls)
	}
	if tip := gateRevParse(t, f.root, "spec/"+closingSpec); tip != m {
		t.Fatalf("close made a commit with no ledgers: tip %s, merge %s", tip, m)
	}
	requireExportNotStaged(t, f)
	t.Logf("closed=%v removed=%v spec_tip_unchanged=true", r.BeadClosed, r.WorktreeRemoved)
}

func TestComplete_IgnoredExportCommitsOnlyLedgers(t *testing.T) {
	f := newWindowFixtureExport(t, true, true)
	f.ex.realPublish = true
	log := recordBdExport(t)
	ledger := f.registerLedgerWithExclude(false)
	f.merge()
	r, err := f.run(CompleteOpts{})
	if err != nil || r == nil || !r.BeadClosed || !r.WorktreeRemoved {
		t.Fatalf("close in a repo that ignores the export failed: %+v %v", r, err)
	}
	requireNoExport(t, log)
	if len(f.ex.publishCalls) != 1 || len(f.ex.publishCalls[0]) != 1 || f.ex.publishCalls[0][0] != ledger {
		t.Fatalf("publication paths = %v, want only %s", f.ex.publishCalls, ledger)
	}
	committed, err := f.ex.FileAtRef("spec/"+closingSpec, ledger)
	if err != nil {
		t.Fatalf("ledger not committed: %v", err)
	}
	current, err := os.ReadFile(filepath.Join(f.specWT, ledger))
	if err != nil || string(committed) != string(current) {
		t.Fatalf("committed ledger differs from working copy: %v", err)
	}
	requireExportNotStaged(t, f)

	// The ledger now matches, so publication is done without any export.
	done, err := closePublicationDone(closingBead, "spec/"+closingSpec, f.specWT, []string{ledger}, false, f.ex)
	if err != nil || !done {
		t.Fatalf("matching ledger must count as published: %v %v", done, err)
	}
	writeFile(t, f.specWT, ledger, string(current)+"\n")
	done, err = closePublicationDone(closingBead, "spec/"+closingSpec, f.specWT, []string{ledger}, false, f.ex)
	if err != nil || done {
		t.Fatalf("changed ledger must not count as published: %v %v", done, err)
	}
	done, err = closePublicationDone(closingBead, "spec/"+closingSpec, f.specWT, nil, false, f.ex)
	if err != nil || !done {
		t.Fatalf("no ledgers must count as published: %v %v", done, err)
	}
}
