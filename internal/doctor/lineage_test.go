package doctor

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lineageDoctorFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "metadata.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
case "$1" in
list)
 case "$*" in
 *--type=epic*) echo '[{"id":"epic-a","title":"[SPEC 129-x] work","status":"open","issue_type":"epic","metadata":{"spec_id":"129-x"}},{"id":"epic-b","status":"closed","issue_type":"epic","metadata":{"spec_id":"129-x"}}]' ;;
 *) echo '[{"id":"epic-a.1","title":"[129] old","status":"open","issue_type":"task"}]' ;;
 esac ;;
show) echo '[{"id":"epic-a.1","title":"[129] old","dependencies":[]}]' ;;
*) echo '[]' ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root
}
func assertLineageReports(t *testing.T, r *Report, wantGit bool) {
	t.Helper()
	title, duplicate, git := false, false, false
	for _, check := range r.Checks {
		if strings.HasPrefix(check.Name, "Title-only lineage:") {
			title = check.Status == Warn && strings.Contains(check.Message, "recovery: bd update epic-a.1 --parent epic-a")
		}
		if check.Name == "Duplicate spec epics" {
			duplicate = check.Status == Warn && strings.Contains(check.Message, "epic-a, epic-b")
		}
		if strings.HasSuffix(check.Name, " git tracking") {
			git = true
		}
	}
	if !title || !duplicate || (wantGit && !git) {
		t.Fatalf("missing reports: title=%t duplicate=%t git=%t: %+v", title, duplicate, git, r.Checks)
	}
}
func TestDoctor_TitleOnlyLineage(t *testing.T) {
	root := lineageDoctorFixture(t)
	r := &Report{}
	checkLineage(r, root)
	assertLineageReports(t, r, false)
}
func TestRunWithOptions_LineageFindingsReachReport(t *testing.T) {
	root := lineageDoctorFixture(t)
	r := RunWithOptions(root, Options{SkipLocalEnv: true})
	assertLineageReports(t, r, true)
}

func TestDoctor_TitleNumberDiagnosisMatchesRemovedRule(t *testing.T) {
	for _, title := range []string{"[129] work", "[129-fixture] work", "[129b-fixture] work", "[129 extra] work"} {
		number, ok := lineageTitleNumber(title)
		if !ok || number != 129 {
			t.Fatalf("old title parse lost for %q: %d %t", title, number, ok)
		}
	}
	if _, ok := lineageTitleNumber("[first] [129]"); ok {
		t.Fatal("diagnosis parsed a later tag that the old rule ignored")
	}
}

func TestDoctor_LineageWithoutStoreDoesNotQuery(t *testing.T) {
	for _, exported := range []bool{false, true} {
		t.Run(fmt.Sprintf("exported=%t", exported), func(t *testing.T) {
			root := t.TempDir()
			if exported {
				if err := os.MkdirAll(filepath.Join(root, ".beads"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".beads", "issues.jsonl"), []byte(`{}`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			t.Cleanup(phase.SetListJSONForTest(func(...string) ([]byte, error) { calls++; return []byte(`[]`), nil }))
			r := &Report{}
			checkLineage(r, root)
			if calls != 0 || len(r.Checks) != 0 {
				t.Fatalf("lineage queried a repo without a store: calls=%d checks=%+v", calls, r.Checks)
			}
		})
	}
}
