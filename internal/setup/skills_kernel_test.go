package setup

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

var s134RemovedGuidance = regexp.MustCompile("(?i)max_rounds_per_bead|panel_deadlock_rounds|max_consecutive_impl_failures|gate_authority|Any bead REJECT|REJECT / HARD block|do not proceed to the next bead|on_reject|loop\\.halt|max-rounds|One halted bead|keep going after a halt|default .?3")

func s134CycleError(body string) error {
	// Require the dispatch sequence in its executable instructions, not a glossary.
	last := -1
	for _, instruction := range []string{
		"1. Run `mindspec next-action --apply <spec-slug> --json`",
		"2. For each selected action, execute its `claim` argv (`mindspec lease <step>`)",
		"3. Dispatch by the returned kind:",
		"4. Submit the evidence with the action's `submit` argv and current token.",
		"5. Execute each `run` entry's `argv` as returned.",
		"6. Return each open decision to the user with its listed options.",
		"7. Run the loop again after each submission or command.",
	} {
		pos := strings.Index(body, instruction)
		if pos < 0 || pos <= last {
			return fmt.Errorf("missing or out-of-order instruction: %s", instruction)
		}
		last = pos
	}
	for _, want := range []string{
		"`implement`", "`review-slot`", "`fix`", "`change-judge`",
		"mindspec submit <step> --lease <token> --commit <oid>",
		"mindspec submit <step> --lease <token> --artifact <path>",
		"Other eligible work can continue.",
		"Execute the action's `brief` argv",
		"Skip adapter panel creation and automatic fix or halt routing.",
		"Never merge a bead branch with raw `git merge bead/<id>`.",
		"If a check stopped between bd-close and the actual git merge, recover it before another dispatch.",
		"Stop on a failed merge or a dependency mismatch",
	} {
		if !strings.Contains(body, want) {
			return fmt.Errorf("missing actor or recovery contract: %s", want)
		}
	}
	if hit := s134RemovedGuidance.FindString(body); hit != "" {
		return fmt.Errorf("removed guidance: %s", hit)
	}
	return nil
}

func TestS134Skills_CycleContract(t *testing.T) {
	body := pluginmindspec.SkillFiles()["ms-bead-cycle"]
	if err := s134CycleError(body); err != nil {
		t.Fatal(err)
	}
	// Each wrong value must fail the same predicate that guards the shipped file.
	for _, fragment := range []string{
		"mindspec next-action --apply <spec-slug> --json",
		"execute its `claim` argv (`mindspec lease <step>`)",
		"Dispatch by the returned kind:",
		"Submit the evidence with the action's `submit` argv and current token.",
		"Execute each `run` entry's `argv` as returned.",
		"Return each open decision to the user with its listed options.",
		"Run the loop again after each submission or command.",
		"`change-judge`", "Stop on a failed merge or a dependency mismatch",
	} {
		t.Run(fragment, func(t *testing.T) {
			wrong := strings.Replace(body, fragment, "WRONG", 1)
			if wrong == body || s134CycleError(wrong) == nil {
				t.Fatal("wrong instruction passed the shipped contract check")
			}
		})
	}
	prior, err := historicalSkillsFS.ReadFile("historical_skills/ms-bead-cycle.pre134.md")
	if err != nil {
		t.Fatal(err)
	}
	if s134CycleError(string(prior)) == nil {
		t.Fatal("old independent selection rules passed")
	}
}

func TestS134Skills_CopiesAndDocs(t *testing.T) {
	root := repoRoot(t)
	for name, source := range pluginmindspec.SkillFiles() {
		got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte(source)) {
			t.Errorf("%s: tracked copy differs from plugin source", name)
		}
	}
	for _, rel := range []string{".claude/skills", "plugins/mindspec/skills", "plugins/mindspec/README.md", "README.md", "project-docs/user/guides"} {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if hit := s134RemovedGuidance.Find(body); hit != nil {
				t.Errorf("%s: removed guidance %q", path, hit)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"README.md", "plugins/mindspec/README.md", "project-docs/user/guides/autonomy.md"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"next-action", "mindspec lease", "mindspec submit", "bd ready --exclude-label mindspec"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s: missing %q", rel, want)
			}
		}
	}
	for _, wrong := range []string{"max_rounds_per_bead", "panel_deadlock_rounds", "max_consecutive_impl_failures", "gate_authority", "Any bead REJECT", "REJECT / HARD block", "do not proceed to the next bead", "on_reject", "loop.halt", "max-rounds", "One halted bead", "keep going after a halt", "default `3"} {
		if !s134RemovedGuidance.MatchString(wrong) {
			t.Errorf("scan missed planted guidance %q", wrong)
		}
	}
}

func TestS134Skills_SnapshotProvenance(t *testing.T) {
	// Digests from git show 70c68a7:plugins/mindspec/skills/<name>/SKILL.md.
	for name, want := range map[string]string{
		"ms-bead-cycle":     "27022594ebd2eb5a40edee9927ccc8cdfe1556e6809aec024038cf5438d38ce3",
		"ms-spec-autopilot": "70b2f91aebafe3923e52ae78c3b54ea61865c0851411fa0be5715b6e65c6ffcc",
	} {
		t.Run(name, func(t *testing.T) {
			prior, err := historicalSkillsFS.ReadFile("historical_skills/" + name + ".pre134.md")
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(prior)); got != want {
				t.Fatalf("snapshot digest %s, want %s", got, want)
			}
			if string(prior) == pluginmindspec.SkillFiles()[name] {
				t.Fatal("snapshot must differ from current source")
			}
		})
	}
}

func TestS134Skills_PlainSetupRefreshesHistory(t *testing.T) {
	// Every setup subprocess sees a local fake, never the operator's store.
	fakeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDir, "bd"), []byte("#!/bin/sh\n[ \"$1\" = setup ] || exit 91\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	entries, err := historicalSkillsFS.ReadDir("historical_skills")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		name, rel string
		run       func(string, bool) (*Result, error)
	}{
		{"claude", ".claude/skills", RunClaude},
		{"codex", ".agents/skills", RunCodex},
		{"copilot", ".agents/skills", RunCopilot},
	} {
		t.Run(target.name, func(t *testing.T) {
			count := 0
			for _, entry := range entries {
				name, _, _ := strings.Cut(entry.Name(), ".")
				want, active := skillFiles()[name]
				if !active {
					continue
				} // Retired skills have a separate removal contract.
				count++
				t.Run(entry.Name(), func(t *testing.T) {
					prior, err := historicalSkillsFS.ReadFile("historical_skills/" + entry.Name())
					if err != nil {
						t.Fatal(err)
					}
					if string(prior) == want {
						t.Fatal("refresh fixture already equals source")
					}
					root := t.TempDir()
					dir := filepath.Join(root, target.rel)
					writeExisting(t, dir, name, string(prior))
					result, err := target.run(root, false)
					if err != nil {
						t.Fatal(err)
					}
					if result.BeadsRan && result.BeadsMsg != "" {
						t.Fatalf("unexpected fake bd call: %s", result.BeadsMsg)
					}
					rel := filepath.Join(target.rel, name, "SKILL.md")
					if !containsPath(result.Refreshed, rel) {
						t.Fatalf("old shipped skill did not refresh: %+v", result)
					}
					for installedName, source := range skillFiles() {
						if got := readSkill(t, dir, installedName); got != source {
							t.Errorf("installed %s differs from source", installedName)
						}
					}
				})
			}
			if count == 0 {
				t.Fatal("no historical skills exercised")
			}
		})
	}
}
