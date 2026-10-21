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
	body := "See `b/z.go`, a/y.go and b/z.go again. Also c/listed.go (bead 2).\n"
	chunks := []WorkChunk{{ID: 1, KeyFilePaths: []string{"x/x.go"}}, {ID: 2, KeyFilePaths: []string{"c/listed.go"}}}
	got := UnlistedPlanPaths(planWithPaths(nil, body), chunks)
	want := []string{"a/y.go", "b/z.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnlistedPlanPaths_PrefixGlobAndSuffixCoverage(t *testing.T) {
	body := strings.Join([]string{
		"- internal/foo/a.go",
		"- internal/foo/sub/b_test.go",
		"- internal/bar/c.go",
		"- cmd/mindspec/d_test.go",
		"- approve/plan.go",
		"- ./internal/foo/e.go",
	}, "\n") + "\n"
	keys := []string{"internal/foo/", "internal/bar", "cmd/mindspec/*_test.go", "internal/approve/plan.go"}
	if got := UnlistedPlanPaths(planWithPaths(keys, body), []WorkChunk{{ID: 1, KeyFilePaths: keys}}); len(got) != 0 {
		t.Fatalf("want all paths covered, got %v", got)
	}
	// The directory prefix must match whole segments only.
	got := UnlistedPlanPaths(planWithPaths(nil, "- internal/foobar/x.go\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"internal/foo"}}})
	if !reflect.DeepEqual(got, []string{"internal/foobar/x.go"}) {
		t.Fatalf("segment-prefix: got %v", got)
	}
	// A glob must not cover a path it does not match.
	got = UnlistedPlanPaths(planWithPaths(nil, "- cmd/mindspec/d.go\n"), []WorkChunk{{ID: 1, KeyFilePaths: []string{"cmd/mindspec/*_test.go"}}})
	if !reflect.DeepEqual(got, []string{"cmd/mindspec/d.go"}) {
		t.Fatalf("glob: got %v", got)
	}
}

func TestUnlistedPlanPaths_IgnoredTokens(t *testing.T) {
	body := strings.Join([]string{
		"https://github.com/org/repo/blob/main/internal/x/y.go",
		"/usr/local/bin/tool.sh and ~/home/file.md",
		"`internal/*/plan.go` and internal/validate/**/x.go",
		"<path>/file.go and internal/<pkg>/file.go",
		"`.mindspec/specs/999-test/plan.md` and .beads/issues.jsonl",
		"internal/panel.Scan and and/or and 1/2.5",
		"../outside/file.go and plugins/.../SKILL.md",
		"spec.md/plan.md",
		"$ROOT/internal/x.go",
	}, "\n") + "\n"
	got := UnlistedPlanPaths(planWithPaths(nil, body), []WorkChunk{{ID: 1, KeyFilePaths: []string{"other/z.go"}}})
	if len(got) != 0 {
		t.Fatalf("want no paths from ignored tokens, got %v", got)
	}
}

func TestUnlistedPlanPaths_TrailingPunctuationAndLineRefs(t *testing.T) {
	body := "Edit internal/a/b.go. Then internal/c/d.go:41-49, then [link](internal/e/f.md#L3), then `internal/g/h.go`!\n"
	got := UnlistedPlanPaths(planWithPaths(nil, body), []WorkChunk{{ID: 1, KeyFilePaths: []string{"z/z.go"}}})
	want := []string{"internal/a/b.go", "internal/c/d.go", "internal/e/f.md", "internal/g/h.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnlistedPlanPaths_NoWorkChunks(t *testing.T) {
	if got := UnlistedPlanPaths(planWithPaths(nil, "internal/a/b.go\n"), nil); got != nil {
		t.Fatalf("want nil without work_chunks, got %v", got)
	}
}

func TestUnlistedPlanPathsMessage_NoInternalRefs(t *testing.T) {
	msg := UnlistedPlanPathsMessage([]string{"a/b.go", "c/d_test.go"})
	if !strings.Contains(msg, "a/b.go, c/d_test.go") || !strings.Contains(msg, "key_file_paths") {
		t.Fatalf("message lacks paths or hint: %q", msg)
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
