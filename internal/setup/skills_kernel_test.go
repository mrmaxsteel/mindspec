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

// s134AdapterNames are the five loop skills that are harness adapters.
// Each value is the line count of its SKILL.md at the spec base 371dc360.
var s134AdapterNames = map[string]int{
	"ms-bead-cycle":     166,
	"ms-spec-autopilot": 83,
	"ms-bead-impl":      163,
	"ms-bead-fix":       47,
	"ms-panel-tally":    59,
}

// s134AdapterWords are the words that state a rule of order, a pick or a
// halt. That logic belongs to the binary, so no adapter may contain them.
var s134AdapterWords = regexp.MustCompile(`(?i)\b(must|never|only after|round|pick|order|halt|first|before)\b`)

// s134AdapterVerb matches each mindspec verb that an adapter names, with
// the word after it for the two panel verbs.
var s134AdapterVerb = regexp.MustCompile(`mindspec\s+([a-z-]+)(?:\s+([a-z-]+))?`)

var s134AllowedVerbs = map[string]bool{
	"next-action": true, "lease": true, "submit": true, "brief": true,
	"panel prompt": true, "panel tally": true, "instruct": true,
}

// s134AdapterProblems returns each way that body breaks the adapter
// contract: line cap, base length, required verbs, verb allowlist, word
// scan, removed guidance and the panel-before-loop sequence.
func s134AdapterProblems(name, body string) []string {
	var problems []string
	lines := strings.Count(body, "\n")
	if lines > 50 || lines > s134AdapterNames[name] {
		problems = append(problems, fmt.Sprintf("%d lines; the cap is 50 and the base is %d", lines, s134AdapterNames[name]))
	}
	for _, verb := range []string{"next-action", "lease", "submit"} {
		if !regexp.MustCompile(`\b` + verb + `\b`).MatchString(body) {
			problems = append(problems, "does not name "+verb)
		}
	}
	for _, m := range s134AdapterVerb.FindAllStringSubmatch(body, -1) {
		verb := m[1]
		if verb == "panel" {
			verb += " " + m[2]
		}
		if !s134AllowedVerbs[verb] {
			problems = append(problems, "names a verb outside the allowlist: mindspec "+verb)
		}
	}
	for _, verb := range []string{"complete", "approve", "merge"} {
		if regexp.MustCompile(`(?i)\b` + verb + `\b`).MatchString(body) {
			problems = append(problems, "names a lifecycle verb outside the allowlist: "+verb)
		}
	}
	if hit := s134AdapterWords.FindString(body); hit != "" {
		problems = append(problems, "contains the word "+hit)
	}
	if hit := s134RemovedGuidance.FindString(body); hit != "" {
		problems = append(problems, "removed guidance: "+hit)
	}
	tally := strings.Index(body, "mindspec panel tally")
	apply := strings.Index(body, "next-action --apply")
	if tally < 0 || apply < 0 || apply < tally {
		problems = append(problems, "the panel tally step must come ahead of the next-action --apply loop")
	}
	return problems
}

func s134AdapterCopies(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	copies := map[string]string{}
	for name := range s134AdapterNames {
		for _, dir := range []string{"plugins/mindspec/skills", ".claude/skills"} {
			raw, err := os.ReadFile(filepath.Join(root, dir, name, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			copies[dir+"/"+name] = string(raw)
		}
	}
	return copies
}

// TestS134Skills_AdapterShape checks each copy of the five adapters against
// the adapter contract and checks that the two copies of each are equal.
func TestS134Skills_AdapterShape(t *testing.T) {
	copies := s134AdapterCopies(t)
	for key, body := range copies {
		name := filepath.Base(key)
		t.Run(key, func(t *testing.T) {
			if problems := s134AdapterProblems(name, body); len(problems) != 0 {
				t.Fatal(problems)
			}
		})
	}
	for name := range s134AdapterNames {
		if copies["plugins/mindspec/skills/"+name] != copies[".claude/skills/"+name] {
			t.Errorf("%s: the .claude copy differs from the plugin copy", name)
		}
		if copies["plugins/mindspec/skills/"+name] != pluginmindspec.SkillFiles()[name] {
			t.Errorf("%s: the embedded copy differs from the plugin file", name)
		}
	}
	cycle := copies["plugins/mindspec/skills/ms-bead-cycle"]
	for mutation, body := range map[string]string{
		"7-line adapter that names complete": "---\nname: ms-bead-cycle\n---\n\nRun `mindspec next-action --apply`, `mindspec lease` and `mindspec submit`.\nThen run `mindspec complete <id>`.\n\n",
		"pick order":                         cycle + "Pick the step with the lowest order.\n",
		"loop ahead of the panel tally":      strings.Replace(cycle, "## Panel", "Run `mindspec next-action --apply <spec-slug> --json` now.\n\n## Panel", 1),
		"verb outside the allowlist":         cycle + "Run `mindspec panel create <panel-slug>`.\n",
		"no submit":                          strings.ReplaceAll(cycle, "submit", "send"),
		"over the base length":               cycle + strings.Repeat("\n", 7),
		"removed guidance":                   cycle + "Set max_rounds_per_bead in the config.\n",
	} {
		t.Run("mutation/"+mutation, func(t *testing.T) {
			if len(s134AdapterProblems("ms-bead-cycle", body)) == 0 {
				t.Fatal("the mutation passed the adapter contract")
			}
		})
	}
	if len(s134AdapterProblems("ms-bead-fix", copies["plugins/mindspec/skills/ms-bead-fix"]+strings.Repeat("\n", 4))) == 0 {
		t.Error("ms-bead-fix grew past its base length and passed")
	}
}

// TestS134Skills_CycleContract checks that each adapter states the loop in
// sequence: the panel tally and its routes, then the next-action loop with
// lease, brief, fresh agent, submit by kind, run entries, decisions and
// status. The old pre-kernel text fails the same check.
func TestS134Skills_CycleContract(t *testing.T) {
	for key, body := range s134AdapterCopies(t) {
		t.Run(key, func(t *testing.T) {
			if err := s134CycleError(body); err != nil {
				t.Fatal(err)
			}
			for _, fragment := range s134CycleSequence {
				wrong := strings.Replace(body, fragment, "WRONG", 1)
				if wrong == body || s134CycleError(wrong) == nil {
					t.Errorf("removing %q passed the contract check", fragment)
				}
			}
		})
	}
	for _, old := range []string{"ms-bead-cycle.pre134.md", "ms-bead-cycle.pre134-kernel.md"} {
		prior, err := historicalSkillsFS.ReadFile("historical_skills/" + old)
		if err != nil {
			t.Fatal(err)
		}
		if s134CycleError(string(prior)) == nil {
			t.Errorf("%s passed the adapter contract", old)
		}
	}
}

var s134CycleSequence = []string{
	"run `mindspec panel tally <panel-slug>`.",
	"prints no `decision:` line: report the error to the user and stop.",
	"It prints a `Next` block: run the lines of that block above `For the operator, run one of these:`, then stop. Do not run `next-action --apply`.",
	"It prints no `Next` block, which only a kernel gate does: go to the loop.",
	"1. Run `mindspec next-action --apply <spec-slug> --json`.",
	"If it refuses, run the commands that the refusal and `mindspec instruct` print.",
	"run its `claim` argv (`mindspec lease <step>`) and keep the token.",
	"Run its `brief` argv. Start a fresh agent with that output as its prompt.",
	"If the `brief` argv exits non-zero, start no agent",
	"run the action's `submit` argv with the token and the evidence:",
	"`--commit <oid>` for an implement or fix step",
	"`--record <path>` for a lens, verify or\n   change-judge step",
	"`--artifact <path>` for a review slot.",
	"If `submit` refuses, run its recovery line.",
	"4. Run each `run` entry's `argv` as printed.",
	"5. For each entry in `decisions`, follow the `mindspec instruct` rules.",
	"- `work`: go to step 1.",
	"- `done`: report the result to the user and stop.",
	"any other status: report it and each flag to the user, and stop.",
	"Run the `repair` line of each flag.",
}

func s134CycleError(body string) error {
	last := -1
	for _, fragment := range s134CycleSequence {
		pos := strings.Index(body, fragment)
		if pos < 0 || pos <= last {
			return fmt.Errorf("missing or out-of-sequence instruction: %s", fragment)
		}
		last = pos
	}
	return nil
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
	// Pin the body shipped before the leased-check correction as refreshable too.
	prior, err := historicalSkillsFS.ReadFile("historical_skills/ms-bead-cycle.pre134-fix.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(prior)); got != "4a11dd849813a08dea1b65a7158f0868a6b66508ca1e6a61dc0c42ba7e1ae757" {
		t.Fatalf("pre134-fix snapshot digest %s", got)
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
