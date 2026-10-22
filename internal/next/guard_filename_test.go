package next

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func filenameGuardRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		argv := []string{"-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}
		out, err := exec.Command("git", append(argv, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	git("config", "core.quotePath", "true")
	git("config", "status.renames", "true")
	return root, git
}

func writeGuardFilename(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("tracked user content\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyDirty_PreservesDecodedFilenameWhitespace(t *testing.T) {
	for _, base := range []string{".beads/issues.jsonl", ".mindspec/specs/132-test/reviews/café/dispositions.jsonl", ""} {
		for _, suffix := range []string{" ", "\t", "\n"} {
			rel := base + suffix
			t.Run(fmt.Sprintf("%q", rel), func(t *testing.T) {
				root, git := filenameGuardRepo(t)
				writeGuardFilename(t, root, rel)
				status := git("status", "--porcelain", "--no-renames", "--untracked-files=all")
				paths := parsePorcelain(status)
				if !reflect.DeepEqual(paths, []string{rel}) {
					t.Fatalf("decoder changed filename bytes: status=%q got=%q want=%q", status, paths, rel)
				}
				artifacts, user := classifyDirty(paths)
				if len(artifacts) != 0 || !reflect.DeepEqual(user, []string{rel}) {
					t.Fatalf("filename lost during exemption: artifacts=%q user=%q want=%q", artifacts, user, rel)
				}
				t.Logf("porcelain=%q user=%q", status, user)
			})
		}
	}
}

func TestCheckDirtyTreeProportional_RenameOntoArtifactNamesUserSource(t *testing.T) {
	for _, destination := range []string{".beads/issues.jsonl", ".mindspec/specs/132-test/reviews/cleanup/dispositions.jsonl"} {
		t.Run(destination, func(t *testing.T) {
			root, git := filenameGuardRepo(t)
			const source = "user.txt"
			writeGuardFilename(t, root, source)
			git("add", "--", source)
			git("commit", "-qm", "tracked user file")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, destination)), 0755); err != nil {
				t.Fatal(err)
			}
			git("mv", "--", source, destination)
			status := git("status", "--porcelain", "-z", "--untracked-files=all")
			entries := parsePorcelainZ(status)
			want := []dirtyEntry{{Status: "R ", Path: destination, OrigPath: source}}
			if !reflect.DeepEqual(entries, want) {
				t.Fatalf("fixture must detect staged rename: status=%q entries=%+v", status, entries)
			}
			oldStatus, oldExport := statusPorcelainZFn, exportBeadsFn
			t.Cleanup(func() { statusPorcelainZFn, exportBeadsFn = oldStatus, oldExport })
			statusPorcelainZFn = defaultStatusPorcelainZ
			exports := 0
			exportBeadsFn = func(string) error { exports++; return nil }
			label := source + " -> " + destination
			blocking, ignored, err := CheckDirtyTreeProportional(root, root, "132-test")
			if err != nil || len(blocking) != 0 || !reflect.DeepEqual(ignored, []string{label}) || exports != 0 {
				t.Errorf("rename user source hidden: blocking=%q ignored=%q exports=%d err=%v", blocking, ignored, exports, err)
			}
			restore := SetEntangleableClaimPathsForTest([]string{source})
			defer restore()
			blocking, ignored, err = CheckDirtyTreeProportional(root, root, "132-test")
			if err != nil || !reflect.DeepEqual(blocking, []string{label}) || len(ignored) != 0 || exports != 0 {
				t.Errorf("entangled source hidden: blocking=%q ignored=%q exports=%d err=%v", blocking, ignored, exports, err)
			}
			t.Logf("status=%q blocking=%q ignored=%q exports=%d", status, blocking, ignored, exports)
		})
	}
}

func TestClassifyDirtyZ_RequiresEveryEndpointToBeArtifact(t *testing.T) {
	const artifact = ".beads/issues.jsonl"
	const ledger = ".mindspec/specs/132-test/reviews/cleanup/dispositions.jsonl"
	for _, status := range []string{"R ", " C"} {
		for _, tc := range []struct {
			path, orig string
			artifact   bool
		}{
			{artifact, "user.txt", false},
			{"user.txt", artifact, false},
			{artifact, ledger, true},
			{artifact, artifact + "\t", false},
		} {
			e := dirtyEntry{Status: status, Path: tc.path, OrigPath: tc.orig}
			artifacts, user := classifyDirtyZ([]dirtyEntry{e})
			if tc.artifact {
				if !reflect.DeepEqual(artifacts, []dirtyEntry{e}) || len(user) != 0 {
					t.Errorf("artifact endpoints changed: entry=%+v artifacts=%+v user=%+v", e, artifacts, user)
				}
			} else if len(artifacts) != 0 || !reflect.DeepEqual(user, []dirtyEntry{e}) {
				t.Errorf("user endpoint exempt: entry=%+v artifacts=%+v user=%+v", e, artifacts, user)
			}
		}
	}
}
