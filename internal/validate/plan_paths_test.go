package validate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// planWithPaths builds a plan whose frontmatter declares one work chunk
// with keyPaths and whose body names bodyPaths.
func planWithPaths(keyPaths []string, body string) string {
	var b strings.Builder
	b.WriteString("---\nstatus: Draft\nspec_id: 999-test\nversion: \"1\"\nwork_chunks:\n  - id: 1\n    key_file_paths:\n")
	for _, p := range keyPaths {
		b.WriteString("      - " + p + "\n")
	}
	b.WriteString("    # frontmatter mention internal/front/only.go must not count\n")
	b.WriteString("---\n\n# Plan\n\n")
	b.WriteString(body)
	return b.String()
}

func TestUnlistedPlanPaths_TestFileMissingWarns(t *testing.T) {
	body := "Bead 1 edits `internal/validate/plan.go` and adds `internal/validate/adr_new_test.go`.\n"
	got := UnlistedPlanPaths(planWithPaths([]string{"internal/validate/plan.go"}, body), []WorkChunk{{ID: 1, KeyFilePaths: []string{"internal/validate/plan.go"}}})
	want := []string{"internal/validate/adr_new_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnlistedPlanPaths_ListedDoesNotWarn(t *testing.T) {
	keys := []string{"internal/validate/plan.go", "internal/validate/adr_new_test.go"}
	body := "Bead 1 edits `internal/validate/plan.go` and adds internal/validate/adr_new_test.go.\n"
	if got := UnlistedPlanPaths(planWithPaths(keys, body), []WorkChunk{{ID: 1, KeyFilePaths: keys}}); len(got) != 0 {
		t.Fatalf("want no paths, got %v", got)
	}
}

func TestUnlistedPlanPaths_SortedDeduplicatedAcrossChunks(t *testing.T) {
	body := "Edit `b/w.go`, a/v.go and b/w.go again. Also edit c/listed.go (bead 2).\n"
	chunks := []WorkChunk{{ID: 1, KeyFilePaths: []string{"q/q.go"}}, {ID: 2, KeyFilePaths: []string{"c/listed.go"}}}
	got := UnlistedPlanPaths(planWithPaths(nil, body), chunks)
	want := []string{"a/v.go", "b/w.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnlistedPlanPaths_PrefixGlobAndSuffixCoverage(t *testing.T) {
	body := strings.Join([]string{
		"- Edit internal/pkg/a.go",
		"- Edit internal/pkg/sub/b_test.go",
		"- Edit internal/lib/c.go",
		"- Edit cmd/mindspec/d_test.go",
		"- Edit approve/plan.go",
		"- Edit ./internal/pkg/e.go",
	}, "\n") + "\n"
	keys := []string{"internal/pkg/", "internal/lib", "cmd/mindspec/*_test.go", "internal/approve/plan.go"}
	if got := UnlistedPlanPaths(planWithPaths(keys, body), []WorkChunk{{ID: 1, KeyFilePaths: keys}}); len(got) != 0 {
		t.Fatalf("want all paths covered, got %v", got)
	}
	// The directory prefix must match whole segments only.
	got := UnlistedPlanPaths(planWithPaths(nil, "- Edit internal/pkgbar/w.go\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"internal/pkg"}}})
	if !reflect.DeepEqual(got, []string{"internal/pkgbar/w.go"}) {
		t.Fatalf("segment-prefix: got %v", got)
	}
	// A glob must not cover a path it does not match.
	got = UnlistedPlanPaths(planWithPaths(nil, "- Edit cmd/mindspec/d.go\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"cmd/mindspec/*_test.go"}}})
	if !reflect.DeepEqual(got, []string{"cmd/mindspec/d.go"}) {
		t.Fatalf("glob: got %v", got)
	}
}

// assertNoPlanPathWarning runs the check over each body (one work chunk,
// no listed files) and fails if any body produces a warning.
func assertNoPlanPathWarning(t *testing.T, bodies []string) {
	t.Helper()
	for _, body := range bodies {
		r := &Result{}
		checkUnlistedPlanPaths(r, planWithPaths(nil, body+"\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"other/w.go"}}})
		if len(r.Issues) != 0 {
			t.Errorf("body %q: want no warning, got %+v", body, r.Issues)
		}
	}
}

func TestUnlistedPlanPaths_IgnoredTokens(t *testing.T) {
	assertNoPlanPathWarning(t, []string{
		"Edit https://github.com/org/repo/blob/main/internal/q/w.go",
		"Edit /usr/local/bin/tool.sh and ~/home/file.md",
		"Edit `internal/*/plan.go` and internal/validate/**/w.go",
		"Edit <path>/file.go and internal/<pkg>/file.go",
		"Edit `.mindspec/specs/999-test/plan.md` and .beads/issues.jsonl",
		"Edit internal/panel.Scan and and/or and 1/2.5",
		"Edit ../outside/file.go and plugins/.../SKILL.md",
		"Edit spec.md/plan.md",
		"Edit $ROOT/internal/w.go",
		"Edit `C:/repo/internal/w.go` and `C:\\repo\\internal\\w.go`",
	})
}

// TestUnlistedPlanPaths_GlobAndPlaceholderTokens pins that a glob or
// placeholder token is rejected whole, so no suffix of it can warn, and
// that conventional example paths and "a/b" filename prose do not warn.
// Each input is tried as given and with an edit verb in front.
func TestUnlistedPlanPaths_GlobAndPlaceholderTokens(t *testing.T) {
	inputs := []string{
		"Use `path/to/file.go` as a placeholder.",
		"Inspect `internal/[ab]foo/bar.go`.",
		"Inspect `[ab]src/file.go`.",
		"Example `{module}src/file.go`.",
		"`internal/[ab]pkg/w.go`",
		"`{module}/src/w.go`",
		"`src/path/to/w.go`",
		"`internal/foo/w.go`",
		"`internal/pkg/bar.go`",
		"`project-docs/baz.md`",
		"`internal/gitutil/x.go`",
		"`internal/example/w.go`",
		"Makefile/go.mod",
	}
	var bodies []string
	for _, in := range inputs {
		bodies = append(bodies, in, "Edit "+in)
	}
	assertNoPlanPathWarning(t, bodies)
}

// TestUnlistedPlanPaths_ContextMentionsDoNotWarn pins the edit-context
// rule: a path named as read-only context, as a citation, or inside a
// fenced block does not warn.
func TestUnlistedPlanPaths_ContextMentionsDoNotWarn(t *testing.T) {
	assertNoPlanPathWarning(t, []string{
		"Do not change `internal/x/y.go`.",
		"Do not change `internal/pkg/w.go`.",
		"- Don't edit internal/pkg/w.go; it stays as is.",
		"Leave `internal/pkg/w.go` doc comments alone and update nothing else.",
		"Read and left unchanged (by inspection): `internal/pkg/w.go`.",
		"Update the caller; `internal/pkg/w.go` is byte-unchanged.",
		"Change the message, e.g. in `internal/pkg/w.go`.",
		"Add a check for example to `internal/pkg/w.go`.",
		"Add the helper next to the one in `internal/pkg/w.go:55-61`.",
		"Update the call at internal/pkg/w.go:41.",
		"Edit the section at [the doc](docs/w.md#L3).",
		"Change `internal/pkg/w.go#L12`.",
		"`internal/pkg/w.go` holds the parser.",
		"A genuinely-new fixture names `internal/pkg/w.go`.",
		"```go\n// Add internal/pkg/w.go\nfunc f() {} // edit internal/pkg/v.go\n```",
		"~~~\nCreate internal/pkg/w.go\n~~~",
		"- Steps:\n  ```\n  add internal/pkg/w.go\n  ```",
	})
}

// TestUnlistedPlanPaths_EditMentionsWarn pins the positive side: an edit
// verb (or a "(new)" tag) with a path warns, including bold paths and a
// "label:path" token.
func TestUnlistedPlanPaths_EditMentionsWarn(t *testing.T) {
	cases := map[string]string{
		"Add `internal/validate/new_test.go`.":          "internal/validate/new_test.go",
		"Create **internal/a.go**.":                     "internal/a.go",
		"Create __internal/b.go__ and wire it in.":      "internal/b.go",
		"- Modify file:internal/c.go to add a flag.":    "internal/c.go",
		"- `internal/d.go` (new): the helper.":          "internal/d.go",
		"| `internal/e.go` | update the parser |":       "internal/e.go",
		"Edit internal/f.go. Then run the tests.":       "internal/f.go",
		"Edit [the parser](internal/g.go) to log.":      "internal/g.go",
		"```\nskip\n```\n\nRename `internal/h.go` now.": "internal/h.go",
	}
	for body, want := range cases {
		got := UnlistedPlanPaths(planWithPaths(nil, body+"\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"other/w.go"}}})
		if !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("body %q: got %v, want [%s]", body, got, want)
		}
	}
}

func TestUnlistedPlanPaths_NoWorkChunks(t *testing.T) {
	if got := UnlistedPlanPaths(planWithPaths(nil, "Edit internal/a/b.go\n"), nil); got != nil {
		t.Fatalf("want nil without work_chunks, got %v", got)
	}
}

func TestUnlistedPlanPathsMessage_NoInternalRefs(t *testing.T) {
	msg := UnlistedPlanPathsMessage([]string{"a/b.go", "c/d_test.go"})
	if !strings.Contains(msg, "a/b.go, c/d_test.go") || !strings.Contains(msg, "key_file_paths") {
		t.Fatalf("message lacks paths or hint: %q", msg)
	}
	if strings.Contains(msg, "remove the path") {
		t.Fatalf("message must not ask to remove the path from the plan: %q", msg)
	}
	for _, bad := range []string{"ADR-", "spec 1", "Spec 1", "R1", "mindspec-"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("message carries internal ref %q: %q", bad, msg)
		}
	}
}

// writePlanPathsFixture writes a structurally valid Draft plan that names
// internal/validate/adr_new_test.go in Bead 1's steps.
func writePlanPathsFixture(t *testing.T, keys []string) string {
	t.Helper()
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "999-test")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## ADR Fitness\n\nNo relevant ADRs.\n\n## Testing Strategy\n\nUnit tests.\n\n## Provenance\n\nN/A.\n\n" +
		"## Bead 1: Test\n\n**Steps**:\n1. Edit `internal/validate/plan.go`\n2. Add `internal/validate/adr_new_test.go`\n3. Run tests\n\n" +
		"**Verification**:\n- [ ] `go test ./...` passes\n\n**Acceptance Criteria**:\n- [ ] the new test passes\n\n**Depends on**: nothing\n"
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(planWithPaths(keys, body)), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func planPathIssues(r *Result) []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Name == PlanPathsWarningName {
			out = append(out, i)
		}
	}
	return out
}

func TestValidatePlan_UnlistedPlanPathWarnsNotFails(t *testing.T) {
	root := writePlanPathsFixture(t, []string{"internal/validate/plan.go"})
	r := ValidatePlan(root, "999-test")
	got := planPathIssues(r)
	if len(got) != 1 {
		t.Fatalf("want exactly one %s issue, got %+v (all: %+v)", PlanPathsWarningName, got, r.Issues)
	}
	if got[0].Severity != SevWarning {
		t.Fatalf("want warning severity, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, "internal/validate/adr_new_test.go") || strings.Contains(got[0].Message, "internal/validate/plan.go") {
		t.Fatalf("warning lists the wrong paths: %q", got[0].Message)
	}
	// The warning alone must not turn the result into a failure: the same
	// plan with the path listed has the same failure state.
	listed := ValidatePlan(writePlanPathsFixture(t, []string{"internal/validate/plan.go", "internal/validate/adr_new_test.go"}), "999-test")
	if len(planPathIssues(listed)) != 0 {
		t.Fatalf("listed path still warns: %+v", listed.Issues)
	}
	if r.HasFailures() != listed.HasFailures() {
		t.Fatalf("warning changed failure state: with=%v without=%v (%+v)", r.HasFailures(), listed.HasFailures(), r.Issues)
	}
	if r.HasFailures() {
		t.Fatalf("fixture plan should pass validation, got %+v", r.Issues)
	}
}
