package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func s133CLIGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
func s133CLIWrite(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func s133CLIFixture(t *testing.T, policy, kind string) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
	root := t.TempDir()
	s133CLIWrite(t, root, ".mindspec/config.yaml", "source_globs:\n  - internal/**/*.go\nenforcement:\n  panel_presence: "+policy+"\n")
	s133CLIWrite(t, root, ".mindspec/specs/999-check/spec.md", "# Spec\n\n## Impacted Domains\n\n- core\n")
	citations := "  - ADR-9001\n"
	if kind == "both" {
		citations = "  []\n"
	}
	s133CLIWrite(t, root, ".mindspec/specs/999-check/plan.md", "---\nstatus: Approved\nadr_citations:\n"+citations+"---\n")
	s133CLIWrite(t, root, ".mindspec/domains/core/OWNERSHIP.yaml", "paths:\n  - internal/core/**\n")
	s133CLIWrite(t, root, ".mindspec/domains/core/architecture.md", "Core documentation.\n")
	s133CLIWrite(t, root, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Core\n\n- **Status**: Accepted\n- **Domain(s)**: core\n\n## Decision\nOwn core.\n")
	s133CLIWrite(t, root, "internal/core/core.go", "package core\nconst Value = 0\n")
	status := "in_progress"
	if kind == "closed" {
		status = "closed"
	}
	store, _ := json.Marshal([]map[string]interface{}{{"id": "bead-1", "status": status, "metadata": map[string]interface{}{"sentinel": "unchanged"}}})
	s133CLIWrite(t, root, ".beads/tracker.json", string(store))
	s133CLIGit(t, root, "init", "-q", "-b", "main")
	s133CLIGit(t, root, "add", "-A")
	s133CLIGit(t, root, "commit", "-qm", "base")
	s133CLIGit(t, root, "branch", "spec/999-check")
	s133CLIGit(t, root, "checkout", "-qb", "bead/bead-1")
	s133CLIWrite(t, root, "internal/core/core.go", "package core\nconst Value = 1\n")
	if kind != "both" && kind != "doc" {
		s133CLIWrite(t, root, ".mindspec/domains/core/interfaces.md", "Core change.\n")
	}
	s133CLIGit(t, root, "add", "-A")
	s133CLIGit(t, root, "commit", "-qm", "implementation")
	s133CLIGit(t, root, "checkout", "-q", "main")
	fake := t.TempDir()
	script := "#!/bin/sh\ncase \"$1 $2\" in\n'show bead-1') cat \"$S133_STORE\" ;;\n'worktree list'|'list --type=epic') echo '[]' ;;\n*) echo \"unexpected tracker operation: $*\" >&2; exit 99 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(fake, "bd"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("S133_STORE", filepath.Join(root, ".beads/tracker.json"))
	return root
}

func TestS133CompleteCheck_CLIReadOnly(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"pass", "both", "closed"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				root := s133CLIFixture(t, policy, kind)
				state := func() string {
					data, err := os.ReadFile(filepath.Join(root, ".beads/tracker.json"))
					if err != nil {
						t.Fatal(err)
					}
					return s133CLIGit(t, root, "status", "--porcelain") + s133CLIGit(t, root, "rev-parse", "HEAD") + string(data)
				}
				before := state()
				out, err := runBinary(t, bin, root, "complete", "bead-1", "--spec", "999-check", "--check")
				if state() != before {
					t.Fatal("CLI check changed git or tracker bytes")
				}
				if kind == "pass" {
					if err != nil || !strings.Contains(out, "Content gates passed.") {
						t.Fatalf("check should pass without panel: %v\n%s", err, out)
					}
					for _, bad := range []string{"gate bead", "panel_absent", "Closed bead", "Next mode", "mindspec next"} {
						if strings.Contains(out, bad) {
							t.Fatalf("check printed lifecycle/panel tail %q: %s", bad, out)
						}
					}
				} else {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != 1 {
						t.Fatalf("need exit 1: %v\n%s", err, out)
					}
					if kind == "both" && (!strings.Contains(out, "doc-sync:") || !strings.Contains(out, "adr-divergence:")) {
						t.Fatalf("must report both failures: %s", out)
					}
					if kind == "closed" && !strings.Contains(out, "closed") {
						t.Fatalf("must name closed state: %s", out)
					}
				}
			})
		}
	}
}

func TestS133CompleteCheck_CLIRefusedFlags(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, policy := range []string{"warn", "enforce"} {
		for _, flags := range [][]string{{"--merge-only"}, {"--resolve-merge"}, {"--override-adr", "reason"}, {"--supersede-adr", "ADR-9002"}, {"--allow-net-deletion", "reason"}} {
			t.Run(policy+"/"+flags[0], func(t *testing.T) {
				root := s133CLIFixture(t, policy, "pass")
				before := s133CLIGit(t, root, "status", "--porcelain") + s133CLIGit(t, root, "rev-parse", "HEAD")
				out, err := runBinary(t, bin, root, append([]string{"complete", "bead-1", "--check"}, flags...)...)
				if err == nil || !strings.Contains(out, "--check cannot be used with "+flags[0]) {
					t.Fatalf("must refuse flag: %v\n%s", err, out)
				}
				after := s133CLIGit(t, root, "status", "--porcelain") + s133CLIGit(t, root, "rev-parse", "HEAD")
				if !bytes.Equal([]byte(before), []byte(after)) {
					t.Fatal("flag refusal changed git state")
				}
			})
		}
	}
}
