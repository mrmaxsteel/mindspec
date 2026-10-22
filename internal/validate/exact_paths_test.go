package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func exactPathsGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %q: %v\n%s", args, err, out)
	}
	return strings.TrimSuffix(string(out), "\n")
}

func exactPathsWrite(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePaths_RenameEndpointsAndExactBytes(t *testing.T) {
	for _, suffix := range []string{"", " ", "\t", "\n"} {
		t.Run("suffix="+suffix, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
			t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
			root := t.TempDir()
			exactPathsGit(t, root, "init", "-q", "-b", "main")
			exactPathsGit(t, root, "config", "user.name", "Test")
			exactPathsGit(t, root, "config", "user.email", "test@example.invalid")
			exactPathsGit(t, root, "config", "diff.renames", "true")
			specDir := filepath.Join(root, ".mindspec/specs/132-paths")
			writeSpecAndPlan(t, root, specDir, "132-paths", []string{"widget"}, nil)
			exactPathsWrite(t, root, ".mindspec/config.yaml", "source_globs:\n  - internal/**\n")
			exactPathsWrite(t, root, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
			exactPathsWrite(t, root, ".mindspec/domains/widget/architecture.md", "Widget owns its source.\n")
			source := "internal/widget/widget.go" + suffix
			exactPathsWrite(t, root, source, "package widget\n\nconst Value = 1\n")
			exactPathsGit(t, root, "add", "-A")
			exactPathsGit(t, root, "commit", "-qm", "owned source baseline")
			base := exactPathsGit(t, root, "rev-parse", "HEAD")
			destination := ".mindspec/specs/132-paths/reviews/move/dispositions.jsonl"
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, destination)), 0o755); err != nil {
				t.Fatal(err)
			}
			exactPathsGit(t, root, "mv", source, destination)
			exactPathsGit(t, root, "commit", "-qm", "rename onto artifact")
			head := exactPathsGit(t, root, "rev-parse", "HEAD")
			ex := executor.NewMindspecExecutor(root)
			paths, err := getChangedFiles(ex, base, head, "")
			want := []string{destination, source}
			if err != nil || !reflect.DeepEqual(paths, want) {
				t.Errorf("committed paths = %q, %v; want %q", paths, err, want)
			}
			snapshotPaths, err := getChangedFiles(ex, base, head+"^{tree}", root)
			if err != nil || !reflect.DeepEqual(paths, snapshotPaths) {
				t.Errorf("range/snapshot disagree: range=%q snapshot=%q err=%v", paths, snapshotPaths, err)
			}
			docs := ValidateDocsRange(root, base, head, head, ex)
			found := false
			for _, issue := range docs.Issues {
				if issue.Name == "internal-docs" && strings.Contains(issue.Message, source) {
					found = true
				}
			}
			if !docs.HasFailures() || !found {
				t.Errorf("source deletion escaped doc-sync: %+v", docs.Issues)
			}
			// Exercise both the completion caller and the approval backstop.
			for _, implApprove := range []bool{false, true} {
				result, findings := ValidateDivergence(ex, root, specDir, "", base, head, head, implApprove)
				if !result.HasFailures() || len(findings) != 1 || findings[0].Path != source || findings[0].Domain != "widget" || findings[0].Kind != "uncovered" {
					t.Errorf("implApprove=%v: source deletion escaped divergence: issues=%+v findings=%+v", implApprove, result.Issues, findings)
				}
			}
		})
	}
}

func TestValidateDocs_WorktreeRenameUsesExactPaths(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root := t.TempDir()
	exactPathsGit(t, root, "init", "-q", "-b", "main")
	exactPathsGit(t, root, "config", "user.name", "Test")
	exactPathsGit(t, root, "config", "user.email", "test@example.invalid")
	exactPathsGit(t, root, "config", "diff.renames", "true")
	exactPathsWrite(t, root, ".mindspec/config.yaml", "source_globs:\n  - internal/**\n")
	exactPathsWrite(t, root, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	exactPathsWrite(t, root, ".mindspec/domains/widget/architecture.md", "Widget docs.\n")
	const source = "internal/widget/widget.go\t"
	exactPathsWrite(t, root, source, "package widget\n")
	exactPathsGit(t, root, "add", "-A")
	exactPathsGit(t, root, "commit", "-qm", "owned source baseline")
	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	exactPathsGit(t, root, "mv", source, ".beads/issues.jsonl")
	// The single-ref API compares the current checkout, by contract.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	ex := executor.NewMindspecExecutor(root)
	paths, err := getChangedFiles(ex, "HEAD", "", "")
	if err != nil || !reflect.DeepEqual(paths, []string{".beads/issues.jsonl", source}) {
		t.Errorf("worktree paths = %q, %v", paths, err)
	}
	result := ValidateDocs(root, "HEAD", ex)
	found := false
	for _, issue := range result.Issues {
		if issue.Name == "internal-docs" && strings.Contains(issue.Message, source) {
			found = true
		}
	}
	if !result.HasFailures() || !found {
		t.Fatalf("working-tree rename escaped doc-sync: %+v", result.Issues)
	}
}
