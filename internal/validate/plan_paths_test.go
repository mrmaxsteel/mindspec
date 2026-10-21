package validate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

// unlistedOne runs UnlistedPlanPaths on body with one unrelated key path.
func unlistedOne(body string, keys ...string) []string {
	if len(keys) == 0 {
		keys = []string{"other/w.go"}
	}
	return UnlistedPlanPaths(planWithPaths(keys, body+"\n"), []WorkChunk{{ID: 1, KeyFilePaths: keys}})
}

// TestUnlistedPlanPaths_ReviewerProbesWarn pins edit sentences that a
// whole-sentence marker test used to hide: each path is judged by its
// own clause, and a marker word elsewhere in the sentence does not veto
// a verb that stands right before the path.
func TestUnlistedPlanPaths_ReviewerProbesWarn(t *testing.T) {
	cases := map[string]string{
		"Update `internal/q/w.go`.":                                                           "internal/q/w.go",
		"Update the existing `internal/q/w.go`.":                                              "internal/q/w.go",
		"Update `internal/q/w.go` in the same bead.":                                          "internal/q/w.go",
		"Update `internal/q/w.go` so no missing path is lost.":                                "internal/q/w.go",
		"Update `internal/q/w.go` to behave like the spec validator.":                         "internal/q/w.go",
		"`internal/q/w.go` is updated to detect missing paths.":                               "internal/q/w.go",
		"Add `internal/q/readiness/fixtures.go`.":                                             "internal/q/readiness/fixtures.go",
		"Add `cmd/q/config.go`: a `config` command with a read-only `show` subcommand.":       "cmd/q/config.go",
		"4. Add `internal/q/evidence.go`, read before any mutation:":                          "internal/q/evidence.go",
		"Create `internal/q/w.go`, a no-op shim.":                                             "internal/q/w.go",
		"Add a test in `internal/q/w_test.go` that the command does not panic.":               "internal/q/w_test.go",
		"Update `internal/q/w.go` so existing callers keep working.":                          "internal/q/w.go",
		"Extend `internal/q/w.go` to read the new config key.":                                "internal/q/w.go",
		"Implement `internal/q/w.go`.":                                                        "internal/q/w.go",
		"Replace the loop in `internal/q/w.go`.":                                              "internal/q/w.go",
		"Fix the bug in `internal/q/w.go`.":                                                   "internal/q/w.go",
		"Refactor `internal/q/w.go` to use the helper.":                                       "internal/q/w.go",
		"Update `internal/q/w.go` to match the new schema.":                                   "internal/q/w.go",
		"Create `internal/q/w.go` (no new deps).":                                             "internal/q/w.go",
		"Add `internal/q/w.go` following the existing pattern.":                               "internal/q/w.go",
		"- New file: `internal/q/w.go`":                                                       "internal/q/w.go",
		"- **Files:** `internal/q/w.go`":                                                      "internal/q/w.go",
		"| File | Change |\n|---|---|\n| `internal/q/w.go` | new |":                           "internal/q/w.go",
		"| File | Change |\n|---|---|\n| `internal/q/w.go` | New |":                           "internal/q/w.go",
		"**Create `internal/q/w.go`** - exported `Sanitize`; callers stay unchanged.":         "internal/q/w.go",
		"**Create `internal/q/w.go`** \u2014 exported `Sanitize`, other callers unchanged.":   "internal/q/w.go",
		"### Files to modify\n\n- `internal/q/w.go`":                                          "internal/q/w.go",
		"Add these tests:\n\n- `internal/q/w_test.go` covers the empty case":                  "internal/q/w_test.go",
		"Add two files: `internal/q/w.go`.":                                                   "internal/q/w.go",
		"In `internal/q/w.go`, add the flag.":                                                 "internal/q/w.go",
		"Wire `mindspec doctor` into `.github/workflows/ci.yml` so a non-zero exit fails CI.": ".github/workflows/ci.yml",
	}
	for body, want := range cases {
		if got := unlistedOne(body); !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("body %q: got %v, want [%s]", body, got, want)
		}
	}
}

// TestUnlistedPlanPaths_NonEditSentencesStaySilent pins the negative side
// of the clause rule: citations, narration in the past tense, a negated
// verb, and verbs that only occur inside file names do not warn.
func TestUnlistedPlanPaths_NonEditSentencesStaySilent(t *testing.T) {
	assertNoPlanPathWarning(t, []string{
		"The implementation lives in `internal/q/w.go`.",
		"Bead 1 added `internal/q/w.go`.",
		"The `internal/q/create_update.go` helper is described in `docs/fix-add.md`.",
		"See internal/q/add_fix.go and internal/q/remove_edit.go for the layout.",
		"[create.go](internal/q/create.go) holds the parser.",
		"Update the caller; do not edit `internal/q/w.go`.",
		"Update the caller \u2014 `internal/q/w.go` stays untouched.",
		"The loader reads `internal/q/w.go`, which the fix does not change.",
		// Reference sentences: the path is not the object of the verb.
		"Implement the interface defined in `internal/q/w.go`.",
		"Add tests for `internal/q/w.go`.",
		"Make sure `internal/q/w.go` still passes.",
		"Update the docs to describe `internal/q/w.go`.",
		"Register the hook the same way `internal/q/w.go` does.",
		"Rename the type, as `internal/q/w.go` did in bead 1.",
		"Add the test, then run `go test ./internal/q/w.go`.",
		"The change in `internal/q/w.go` is small.",
		"Write the result to `internal/q/out.json`.",
		"Add a flag like the one in `internal/q/w.go`.",
		"Update the docs to mention what `internal/q/w.go` does.",
		"Move on to `internal/q/w.go` after the tests pass.",
		"Port the logic from `internal/q/w.go`.",
		"Add a check against `internal/q/w.go`.",
		"Write the report summarising `internal/q/w.go`.",
		"Fix round 2 added `TestX` in `internal/q/w_test.go`.",
		"The probes touch `internal/q/w.go` only in throwaway trees.",
		"Add a link to `docs/q/w.md`.",
		"Remove the call to `internal/q/w.go`.",
		"Add a reference to `docs/adr/w.md` in the README.",
		// Negation vetoes every edit signal for the paths after it.
		"Do not add a new file `internal/pkg/w.go`.",
		"- `internal/pkg/w.go`: do not change it.",
		"Do not ever change `internal/pkg/w.go`.",
		"Never directly edit `internal/pkg/w.go`.",
		// A path-leading label alone is not an edit.
		"- `internal/pkg/w.go`: read-only reference.",
		"- `internal/pkg/w.go` \u2014 the parser that the loader calls.",
		"`internal/pkg/w.go`: the entry point.",
	})
}

// TestUnlistedPlanPaths_ReferencePathStaysSilent pins sentences with one
// edit target and one reference path: only the target warns.
func TestUnlistedPlanPaths_ReferencePathStaysSilent(t *testing.T) {
	for _, body := range []string{
		"Refactor `internal/q/a.go` to reuse the parser from `internal/q/w.go`.",
		"Add `internal/q/a.go` in the style of `internal/q/w.go`.",
		"Add the new check to `internal/q/a.go`, modelled on `internal/q/w.go`.",
		"Edit `internal/q/a.go` so its output is identical to `internal/q/w.go`.",
		"Create `internal/q/a.go`; copy the layout of `internal/q/w.go`.",
		"Update `internal/q/a.go` to import `internal/q/w.go`.",
		"Update `internal/q/a.go` to call into `internal/q/w.go`.",
	} {
		if got := unlistedOne(body); !reflect.DeepEqual(got, []string{"internal/q/a.go"}) {
			t.Errorf("body %q: got %v, want [internal/q/a.go]", body, got)
		}
	}
}

// TestValidatePlan_ControlBytesDoNotPanic pins that plan text cannot crash
// the advisory check: control bytes and digit runs in the body are plain
// text, and the plan still validates with at most a warning.
func TestValidatePlan_ControlBytesDoNotPanic(t *testing.T) {
	for _, extra := range []string{
		"Edit the \x0199\x01 thing.",
		"Text \x01 and \x0199\x01 and \x01999999999999999999999\x01.",
		"Add `internal/q/\x010\x01.go` and \x00\x02\x1b[31m `internal/q/w.go`.",
		"\x01\x01\x01: \x01-1\x01 \u2014 new \x010\x01 (new)",
	} {
		root := writePlanPathsFixture(t, []string{"internal/validate/plan.go", "internal/validate/adr_new_test.go"})
		path := filepath.Join(root, "docs", "specs", "999-test", "plan.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte("\n"+extra+"\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		r := ValidatePlan(root, "999-test")
		if r.HasFailures() {
			t.Errorf("body %q: want no failure, got %+v", extra, r.Issues)
		}
		_ = UnlistedPlanPaths(string(data)+extra, []WorkChunk{{ID: 1}})
	}
}

// TestCheckUnlistedPlanPaths_PanicBecomesOneWarning pins the recover: a
// panic inside the check gives one warning that the check was skipped.
func TestCheckUnlistedPlanPaths_PanicBecomesOneWarning(t *testing.T) {
	r := &Result{}
	checkUnlistedPlanPaths(r, "body", []WorkChunk{{ID: 1, KeyFilePaths: nil}, {ID: 2}})
	if len(r.Issues) != 0 {
		t.Fatalf("plain body: want no issue, got %+v", r.Issues)
	}
	r = &Result{}
	func() {
		defer func() {
			if recover() != nil {
				t.Fatal("panic escaped checkUnlistedPlanPaths")
			}
		}()
		checkUnlistedPlanPathsWith(r, func() []string { panic("boom") })
	}()
	if len(r.Issues) != 1 || r.Issues[0].Severity != SevWarning || !strings.Contains(r.Issues[0].Message, "skipped") {
		t.Fatalf("want one skipped warning, got %+v", r.Issues)
	}
}

// TestUnlistedPlanPaths_TemplatePath pins template paths: "<name>"
// segments match key_file_paths as a glob in both directions, and an
// uncovered template is reported as written.
func TestUnlistedPlanPaths_TemplatePath(t *testing.T) {
	body := "Each bead adds its own pin test file `internal/validate/adr_spec131_<adr>_test.go` with a function named `TestADRAmendments_Spec131<ADR>`."
	want := []string{"internal/validate/adr_spec131_<adr>_test.go"}
	if got := unlistedOne(body, "internal/approve/plan.go"); !reflect.DeepEqual(got, want) {
		t.Fatalf("uncovered template: got %v, want %v", got, want)
	}
	for _, key := range []string{
		"internal/validate/adr_spec131_*_test.go",
		"internal/validate/adr_spec131_0040_test.go",
		"internal/validate/",
		"internal/validate/**",
	} {
		if got := unlistedOne(body, key); len(got) != 0 {
			t.Errorf("key %q: want no warning, got %v", key, got)
		}
	}
	// A placeholder in a directory segment, or a stem that is only a
	// placeholder, is too vague to check.
	assertNoPlanPathWarning(t, []string{
		"Edit internal/<pkg>/file.go.",
		"Add `internal/q/<name>.go`.",
	})
}

// TestUnlistedPlanPaths_FenceClosesOnMatchingRun pins fence closing: a
// shorter or different run inside a fence does not end it.
func TestUnlistedPlanPaths_FenceClosesOnMatchingRun(t *testing.T) {
	assertNoPlanPathWarning(t, []string{
		"````md\n```\nAdd internal/q/w.go\n````",
		"~~~\n```\nAdd internal/q/w.go\n```\n~~~",
		"```\n~~~\n```\nx\n```\nAdd internal/q/v.go\n```",
	})
	if got := unlistedOne("````md\n```\nskip\n```\n````\n\nAdd `internal/q/w.go`."); !reflect.DeepEqual(got, []string{"internal/q/w.go"}) {
		t.Fatalf("path after a closed fence: got %v", got)
	}
}

// TestUnlistedPlanPaths_CreateStepWithLaterUnchangedWarns pins a shipped
// step whose sentence says "unchanged" about a value, far from the file
// the step creates.
func TestUnlistedPlanPaths_CreateStepWithLaterUnchangedWarns(t *testing.T) {
	body := "1. **Create `internal/termsafe/termsafe.go`** \u2014 exported\n" +
		"   `Escape(s string) string` carrying the EXACT relocated implementation from\n" +
		"   `escapeConfigValue` (`cmd/mindspec/config.go:114-121`): iterate runes; if\n" +
		"   any rune is outside printable ASCII `[0x20, 0x7e]`, return\n" +
		"   `strconv.Quote(s)`; else return `s` unchanged."
	if got := unlistedOne(body); !reflect.DeepEqual(got, []string{"internal/termsafe/termsafe.go"}) {
		t.Fatalf("got %v, want [internal/termsafe/termsafe.go]", got)
	}
}

// TestValidatePlan_LongSentenceIsFast pins that the check is linear in
// the length of a sentence: one sentence of about 300 KB with many code
// spans, interleaved paths and a long run of paths validates fast.
// The bound is 20 s because it has to absorb -race and coverage
// instrumentation on CI runners; the quadratic implementation takes well
// over a minute on this input, so the bound still catches a regression.
func TestValidatePlan_LongSentenceIsFast(t *testing.T) {
	var b strings.Builder
	b.WriteString("\nEdit `internal/q/a.go` ")
	for i := 0; i < 15500; i++ {
		if i%200 == 0 {
			b.WriteString("`internal/q/p" + strings.Repeat("x", i%7) + ".go` ")
		}
		b.WriteString("`SomeIdentifier` ")
	}
	for i := 0; i < 2000; i++ {
		b.WriteString("`internal/q/r.go` ")
	}
	b.WriteString("new (new) file.\n")
	root := writePlanPathsFixture(t, []string{"internal/validate/plan.go", "internal/validate/adr_new_test.go"})
	path := filepath.Join(root, "docs", "specs", "999-test", "plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, b.String()...), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	r := ValidatePlan(root, "999-test")
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("ValidatePlan on a %d-byte sentence took %v, want under 20s (linear check; quadratic takes over a minute)", b.Len(), d)
	}
	if r.HasFailures() {
		t.Fatalf("want no failure, got %+v", r.Issues)
	}
}
