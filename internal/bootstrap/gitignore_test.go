package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrap_ReviewsIgnoredGreenfield(t *testing.T) {
	for _, emptyGitignore := range []bool{false, true} {
		name := "absent-gitignore"
		if emptyGitignore {
			name = "empty-gitignore"
		}
		t.Run(name, func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			// With no file, starterGitignore already supplies the runtime
			// block. An empty file exercises the order of both ensure calls.
			if emptyGitignore {
				if err := os.WriteFile(filepath.Join(root, ".gitignore"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			data := assertReviewsIgnored(t, root)
			runtimeHeader := strings.Index(string(data), "# MindSpec local runtime files (not version-controlled)")
			reviewHeader := strings.Index(string(data), "# MindSpec panel reviews (local, not version-controlled; spec 129 R8)")
			if runtimeHeader < 0 || reviewHeader < 0 || runtimeHeader >= reviewHeader {
				t.Fatalf("runtime header must precede review header:\n%s", data)
			}
		})
	}
}

func TestBootstrap_ReviewsIgnoredExistingGitignore(t *testing.T) {
	root := reviewBootstrapRepo(t)
	seed := "# project rules\nnode_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, false); err != nil {
		t.Fatal(err)
	}
	if got := assertReviewsIgnored(t, root); !strings.HasPrefix(string(got), seed) {
		t.Fatalf("existing .gitignore bytes changed:\n%s", got)
	}
}

func TestBootstrap_ReviewsIgnoredIdempotent(t *testing.T) {
	for _, seed := range []string{"", "# existing\n*.log\n"} {
		t.Run(seed, func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			if seed != "" {
				if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			first := assertReviewsIgnored(t, root)
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			if second := assertReviewsIgnored(t, root); !bytes.Equal(first, second) {
				t.Fatalf("second init changed .gitignore:\nbefore:\n%s\nafter:\n%s", first, second)
			}
		})
	}
}

func reviewBootstrapRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	return root
}

func assertReviewsIgnored(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	header := "# MindSpec panel reviews (local, not version-controlled; spec 129 R8)"
	want := header + "\n.mindspec/specs/*/reviews/**\n.mindspec/reviews/**\n"
	if !strings.Contains(string(data), want) {
		t.Errorf("missing literal review entries under their header:\n%s", data)
	}
	for _, comment := range []string{header, "# MindSpec local runtime files (not version-controlled)"} {
		count := 0
		for _, line := range lines {
			if line == comment {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected one %q header, got %d:\n%s", comment, count, data)
		}
	}
	for _, path := range []string{
		".mindspec/specs/001-x/reviews/a/panel.json",
		".mindspec/reviews/a/panel.json",
		"src/reviews/x.go",
	} {
		cmd := exec.Command("git", "check-ignore", "-q", "--", path)
		cmd.Dir = root
		err := cmd.Run()
		if path != "src/reviews/x.go" {
			if err != nil {
				t.Errorf("review %s must be ignored: %v", path, err)
			}
		} else {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Errorf("project source %s must not be ignored; got %v", path, err)
			}
		}
	}
	return data
}

// TestRun_GitignoreAppendsRuntimeEntries pins spec 123 AC-5 (R4a): a repo
// with a pre-existing, unrelated .gitignore gets exactly the two runtime
// entries appended, with every prior byte and line preserved (order
// included); `git check-ignore` matches both; a second `mindspec init`
// leaves the file byte-identical. RED on pre-spec-123 main: the .gitignore
// manifest item was create-only (Skipped when the file already existed),
// so `git check-ignore` missed both entries entirely.
func TestRun_GitignoreAppendsRuntimeEntries(t *testing.T) {
	root := t.TempDir()
	original := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Errorf("prior .gitignore bytes not preserved at the head; got:\n%s", content)
	}
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore missing runtime entry %q; got:\n%s", entry, content)
		}
	}

	// git check-ignore must match both (needs an actual git repo).
	runGit(t, root, "init", "-q")
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		cmd := exec.Command("git", "check-ignore", "--quiet", "--", entry)
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			t.Errorf("git check-ignore %s: not ignored (err=%v)", entry, err)
		}
	}

	afterFirst := content

	// Second init: byte-identical (idempotent).
	if _, err := Run(root, false); err != nil {
		t.Fatalf("second Run() error: %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != afterFirst {
		t.Errorf("second Run() changed .gitignore; before:\n%s\nafter:\n%s", afterFirst, data2)
	}
}

// TestRun_GitignoreCreatedFromScratchAlreadyIgnores is a sanity guard: the
// greenfield create-from-scratch path (no pre-existing .gitignore) already
// contains both runtime entries, so the R4a ensure-on-existing-file call is
// a true no-op immediately after — never a double append.
func TestRun_GitignoreCreatedFromScratchAlreadyIgnores(t *testing.T) {
	root := t.TempDir()

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
		if strings.Count(content, entry) != 1 {
			t.Errorf("expected exactly one occurrence of %q, got %d in:\n%s",
				entry, strings.Count(content, entry), content)
		}
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

func TestBootstrap_ReviewsLedgerTracked(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			root := t.TempDir()
			runGit(t, root, "init", "-q")
			if existing {
				if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# existing\n.mindspec/specs/*/reviews/**\n.mindspec/reviews/**\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range []string{"!.mindspec/specs/*/reviews/*/", "!.mindspec/specs/*/reviews/*/dispositions.jsonl"} {
				if strings.Count(string(before), entry+"\n") != 1 {
					t.Fatalf("negation missing or duplicated: %q", before)
				}
			}
			for _, name := range []string{"dispositions.jsonl", "panel.json", "BRIEF.md", "R1-round-1.json", "dispositions.lock"} {
				file := ".mindspec/specs/130-test/reviews/p/" + name
				cmd := exec.Command("git", "-C", root, "check-ignore", "-q", "--", file)
				err := cmd.Run()
				if name == "dispositions.jsonl" {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != 1 {
						t.Fatalf("ledger still ignored: %v", err)
					}
				} else if err != nil {
					t.Fatalf("%s not ignored: %v", name, err)
				}
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatalf("second init changed gitignore:\nbefore: %s\nafter: %s", before, after)
			}
		})
	}
}
