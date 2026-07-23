package main

// greenfield_e2e_test.go — spec 123 Bead 3: the AC-1/AC-19 first-run
// integration fixtures. These drive the ACTUAL cobra commands as a
// subprocess against a freshly built binary (the established
// cmd/mindspec temp-dir command-driving precedent — record_test.go,
// testhelpers_test.go's buildMindspecBinary) rather than in-process
// rootCmd.Execute(): `mindspec doctor` calls os.Exit(1) whenever any
// check is Error/Missing (cmd/mindspec/doctor.go), which a genuinely
// fresh greenfield fixture ALWAYS has (the permitted beads-not-
// initialized Missing) — an in-process Execute() would kill the test
// binary itself.
//
// AC-1's doctor assertion is scoped to four governed lanes (context-map,
// gitignore, models, managed-block/commands): no Error/Missing from
// those lanes, with the two DESIGNED first-run advisory Warns
// (missing-models R6c, missing-commands R7c) asserted PRESENT — not
// forbidden — exactly like the permitted beads-not-initialized Missing.
// `mindspec doctor` itself is allowed to exit non-zero in this fixture
// (the beads-Missing state), so its exit code is never asserted.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// runGitInit runs a real `git init -q` in dir — doctor's git-lane checks
// (unignored-untracked, tracked-runtime-file) shell out to real git.
func runGitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// runMindspecIn runs the built mindspec binary bin with args in dir
// (distinct from version_test.go's cwd-only runMindspec), returning
// combined stdout+stderr and the run error (nil on exit 0).
func runMindspecIn(t *testing.T, bin, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// gitCheckIgnore reports whether `git check-ignore` matches rel under
// dir (a real git subprocess — the same authority doctor's own git-lane
// checks and the R4 acceptance criteria use).
func gitCheckIgnore(dir, rel string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", rel)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// doctorCheckLine is one parsed line of `mindspec doctor`'s rendered
// output ("<name>: [STATUS] <message>", printDoctorChecks's format).
type doctorCheckLine struct {
	Name    string
	Status  string
	Message string
}

var doctorLineRE = regexp.MustCompile(`^(.*): \[(OK|WARN|ERROR|MISSING|FIXED)\](?: (.*))?$`)

// parseDoctorOutput parses `mindspec doctor`'s combined stdout+stderr
// into structured check lines, skipping the leading "Workspace Root:"
// line and any non-matching output.
func parseDoctorOutput(out string) []doctorCheckLine {
	var checks []doctorCheckLine
	for _, line := range strings.Split(out, "\n") {
		m := doctorLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		checks = append(checks, doctorCheckLine{Name: m[1], Status: m[2], Message: m[3]})
	}
	return checks
}

// isGovernedLaneCheck reports whether name belongs to one of AC-1's four
// governed lanes: context-map (docs), gitignore (git), models (config),
// managed-block/commands (config) — the lanes this spec's checks own.
func isGovernedLaneCheck(name string) bool {
	switch {
	case strings.Contains(name, "context-map.md"):
		return true
	case strings.HasSuffix(name, "git tracking"):
		return true
	case name == ".mindspec/config.yaml models":
		return true
	case name == ".mindspec/config.yaml commands":
		return true
	}
	return false
}

// assertGovernedLanesClean fails the test if any governed-lane check
// carries Error or Missing status — AC-1's core scoped assertion.
func assertGovernedLanesClean(t *testing.T, checks []doctorCheckLine) {
	t.Helper()
	for _, c := range checks {
		if !isGovernedLaneCheck(c.Name) {
			continue
		}
		if c.Status == "ERROR" || c.Status == "MISSING" {
			t.Errorf("governed-lane check %q has forbidden status %s: %s", c.Name, c.Status, c.Message)
		}
	}
}

// assertWarnPresent fails the test unless some check's Warn message
// contains substr — AC-1's designed-Warn-present assertions
// (missing-models, missing-commands).
func assertWarnPresent(t *testing.T, checks []doctorCheckLine, substr string) {
	t.Helper()
	for _, c := range checks {
		if c.Status == "WARN" && strings.Contains(c.Message, substr) {
			return
		}
	}
	t.Errorf("expected a Warn check containing %q, got: %+v", substr, checks)
}

// assertMissingPresent fails the test unless some check named name
// carries Missing status — the permitted beads-not-initialized pin.
func assertMissingPresent(t *testing.T, checks []doctorCheckLine, name string) {
	t.Helper()
	for _, c := range checks {
		if c.Name == name && c.Status == "MISSING" {
			return
		}
	}
	t.Errorf("expected check %q to be Missing (permitted — beads init is a Non-Goal), got: %+v", name, checks)
}

// TestGreenfieldFirstRun_AC1 pins AC-1 (#207 restored, R1/R2 first-run
// integration): empty dir -> git init -> mindspec init -> mindspec
// domain add alpha exits 0 at every step; the context-map gains an
// ### Alpha entry under ## Bounded Contexts BEFORE the --- separator
// (not tail-appended); and `mindspec doctor`'s governed-lane scoping
// holds with the two designed first-run Warns present. RED on
// pre-spec-123 main: `domain add` errors "reading context map" before
// any of this state exists.
func TestGreenfieldFirstRun_AC1(t *testing.T) {
	bin := buildMindspecBinary(t)
	root := t.TempDir()
	runGitInit(t, root)

	if out, err := runMindspecIn(t, bin, root, "init"); err != nil {
		t.Fatalf("mindspec init failed: %v\n%s", err, out)
	}
	if out, err := runMindspecIn(t, bin, root, "domain", "add", "alpha"); err != nil {
		t.Fatalf("mindspec domain add alpha failed: %v\n%s", err, out)
	}

	cm, err := os.ReadFile(filepath.Join(root, ".mindspec", "context-map.md"))
	if err != nil {
		t.Fatalf("reading context-map.md: %v", err)
	}
	content := string(cm)
	boundedIdx := strings.Index(content, "## Bounded Contexts")
	alphaIdx := strings.Index(content, "### Alpha")
	sepIdx := strings.Index(content, "\n---")
	if boundedIdx == -1 {
		t.Fatalf("context-map.md missing \"## Bounded Contexts\":\n%s", content)
	}
	if alphaIdx == -1 || alphaIdx < boundedIdx {
		t.Fatalf("### Alpha entry missing or not under ## Bounded Contexts:\n%s", content)
	}
	if sepIdx != -1 && alphaIdx > sepIdx {
		t.Fatalf("### Alpha entry landed AFTER the --- separator (tail-append), not before it:\n%s", content)
	}

	out, _ := runMindspecIn(t, bin, root, "doctor") // doctor's own exit code is not asserted — see file doc comment
	checks := parseDoctorOutput(out)
	if len(checks) == 0 {
		t.Fatalf("failed to parse any doctor check lines from output:\n%s", out)
	}
	assertGovernedLanesClean(t, checks)
	assertWarnPresent(t, checks, "missing-models")
	assertWarnPresent(t, checks, "missing-commands")
	assertMissingPresent(t, checks, "Beads")
}

// TestGreenfieldFirstRun_AC19 pins AC-19 (FR-4, the full cross-verb
// first-run E2E): empty dir -> git init -> mindspec init -> mindspec
// domain add alpha -> mindspec adr create "First decision" --domain
// alpha -> mindspec setup codex -> mindspec doctor: every step exits 0;
// the ADR lands as the slugged ADR-0001-first-decision.md reporting ID
// ADR-0001 (consuming Bead 2's slugged emission + ID normalization);
// init-then-setup operate on the SAME AGENTS.md with exactly one managed
// block and no framework leak after the setup refresh; runtime files
// are gitignored; and final doctor holds AC-1's governed-lane scoping.
// RED on pre-spec-123 main: the sequence breaks at `domain add`.
func TestGreenfieldFirstRun_AC19(t *testing.T) {
	bin := buildMindspecBinary(t)
	root := t.TempDir()
	runGitInit(t, root)

	steps := [][]string{
		{"init"},
		{"domain", "add", "alpha"},
		{"adr", "create", "First decision", "--domain", "alpha"},
		{"setup", "codex"},
	}
	for _, args := range steps {
		out, err := runMindspecIn(t, bin, root, args...)
		if err != nil {
			t.Fatalf("mindspec %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	// The ADR lands slugged, reporting the canonical ID (Bead 2).
	adrPath := filepath.Join(root, ".mindspec", "adr", "ADR-0001-first-decision.md")
	if _, err := os.Stat(adrPath); err != nil {
		t.Fatalf("expected slugged ADR file %s: %v", adrPath, err)
	}
	listOut, err := runMindspecIn(t, bin, root, "adr", "list")
	if err != nil {
		t.Fatalf("adr list failed: %v\n%s", err, listOut)
	}
	if !strings.Contains(listOut, "ADR-0001") {
		t.Errorf("adr list must report the canonical ADR-0001, got:\n%s", listOut)
	}
	if strings.Contains(listOut, "ADR-0001-first-decision") {
		t.Errorf("adr list must report the canonical ID, not the slugged stem, got:\n%s", listOut)
	}

	// init-then-setup: SAME AGENTS.md, exactly one managed block, no leak.
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	a := string(agents)
	if n := strings.Count(a, "<!-- BEGIN mindspec:managed -->"); n != 1 {
		t.Errorf("expected exactly one managed block marker after init+setup, got %d:\n%s", n, a)
	}
	for _, bad := range []string{"make build", "make test", "MindSpec Project"} {
		if strings.Contains(a, bad) {
			t.Errorf("AGENTS.md leaks framework fact %q after the init->setup seam:\n%s", bad, a)
		}
	}

	// Runtime files gitignored.
	for _, f := range []string{".mindspec/session.json", ".mindspec/focus"} {
		if !gitCheckIgnore(root, f) {
			t.Errorf("git check-ignore %s: expected ignored", f)
		}
	}

	// Final doctor: AC-1's governed-lane scoping.
	doctorOut, _ := runMindspecIn(t, bin, root, "doctor")
	checks := parseDoctorOutput(doctorOut)
	if len(checks) == 0 {
		t.Fatalf("failed to parse any doctor check lines from output:\n%s", doctorOut)
	}
	assertGovernedLanesClean(t, checks)
	assertWarnPresent(t, checks, "missing-models")
	assertWarnPresent(t, checks, "missing-commands")
	assertMissingPresent(t, checks, "Beads")
}

// TestGreenfieldFirstRun_AC1_BdInitAndPopulated is AC-1's OPTIONAL
// stronger variant: with `bd init` run AND both models: and commands:
// populated, doctor's four governed lanes AND the beads lane are all
// clean (fully-clean doctor is only reachable once those operator
// declarations exist — the governed-lane scoping above is the
// load-bearing check; this variant is additional confidence). Skips if
// `bd`/`beads` is not on PATH.
func TestGreenfieldFirstRun_AC1_BdInitAndPopulated(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		if _, err2 := exec.LookPath("beads"); err2 != nil {
			t.Skip("bd/beads not available on PATH")
		}
	}

	bin := buildMindspecBinary(t)
	root := t.TempDir()
	runGitInit(t, root)

	if out, err := runMindspecIn(t, bin, root, "init"); err != nil {
		t.Fatalf("mindspec init failed: %v\n%s", err, out)
	}
	if out, err := runMindspecIn(t, bin, root, "domain", "add", "alpha"); err != nil {
		t.Fatalf("mindspec domain add alpha failed: %v\n%s", err, out)
	}

	bdCmd := exec.Command("bd", "init")
	bdCmd.Dir = root
	if out, err := bdCmd.CombinedOutput(); err != nil {
		t.Skipf("bd init failed (skipping e2e): %v\n%s", err, out)
	}

	cfgPath := filepath.Join(root, ".mindspec", "config.yaml")
	cfgYAML := "models:\n  authoring: claude-opus-4-8\ncommands:\n  build: go build ./...\n  test: go test ./...\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runMindspecIn(t, bin, root, "doctor")
	if err != nil {
		t.Fatalf("mindspec doctor expected exit 0 once bd is initialized and models:/commands: are populated: %v\n%s", err, out)
	}
	checks := parseDoctorOutput(out)
	for _, c := range checks {
		if c.Status == "ERROR" || c.Status == "MISSING" {
			t.Errorf("expected fully-clean doctor, got %s on %q: %s", c.Status, c.Name, c.Message)
		}
	}
}
