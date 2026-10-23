package executor

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestChangedFiles_QuotedDefaultAndExactPaths(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	g, _, root := newRepoExecutor(t)
	wantExact := []string{`".beads/issues.jsonl"`, "tab\t.txt"}
	var wantQuoted []string
	for _, name := range wantExact {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("user change\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wantQuoted = append(wantQuoted, strconv.Quote(name))
	}
	runGitIn(t, root, "add", "-A")
	runGitIn(t, root, "commit", "-qm", "quoted filenames")
	for _, tc := range []struct {
		name    string
		options []ChangedFilesOptions
		want    []string
	}{
		{"default", nil, wantQuoted},
		{"no-renames", []ChangedFilesOptions{{NoRenames: true}}, wantQuoted},
		{"exact", []ChangedFilesOptions{{ExactPaths: true}}, wantExact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.ChangedFiles("HEAD^", "HEAD", tc.options...)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("paths = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestTreeDirsAtRef_ExactDomainBytes(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root := t.TempDir()
	runGitIn(t, root, "init", "-q", "-b", "main")
	runGitIn(t, root, "config", "user.name", "Test")
	runGitIn(t, root, "config", "user.email", "test@example.invalid")
	want := []string{"line\n", "space ", "tab\t", "é"}
	for _, name := range want {
		dir := filepath.Join(root, ".mindspec/domains", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte("paths: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitIn(t, root, "add", "-A")
	runGitIn(t, root, "commit", "-qm", "domain names with exact bytes")
	got, err := NewMindspecExecutor(root).TreeDirsAtRef("HEAD", ".mindspec/domains")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("domain names = %q, %v; want %q", got, err, want)
	}
}
