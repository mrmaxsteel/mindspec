package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findCheck returns the first Check with a given Name, or nil.
func findCheck(r *Report, name string) *Check {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			return &r.Checks[i]
		}
	}
	return nil
}

// writeFile is a terse helper for setting up test fixtures.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// beadsRoot returns a temp dir with an empty .beads/ and inits a git repo
// so git-dependent checks can run. If initGit is false, no git setup runs.
func beadsRoot(t *testing.T, initGit bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	if initGit {
		runGit(t, root, "init", "-q")
		runGit(t, root, "config", "user.email", "test@example.com")
		runGit(t, root, "config", "user.name", "Test")
		runGit(t, root, "config", "commit.gpgsign", "false")
	}
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

// ─── checkBeadsConfigDrift ────────────────────────────────────────────────

func TestCheckBeadsConfigDrift_MissingFile(t *testing.T) {
	root := beadsRoot(t, false)

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	c := findCheck(r, "Beads config drift")
	if c == nil {
		t.Fatal("missing check")
	}
	if c.Status != Warn {
		t.Errorf("status = %d, want Warn", c.Status)
	}
	if c.FixFunc == nil {
		t.Error("expected FixFunc to be set so --fix can create the file")
	}
}

func TestCheckBeadsConfigDrift_AllCorrect(t *testing.T) {
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"proj\"\n"+
			"types.custom: \"gate\"\n"+
			"status.custom: \"resolved\"\n"+
			"export.git-add: false\n")

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	c := findCheck(r, "Beads config drift")
	if c == nil || c.Status != OK {
		t.Fatalf("got %+v, want OK", c)
	}
}

func TestCheckBeadsConfigDrift_MissingKey(t *testing.T) {
	root := beadsRoot(t, false)
	// Missing types.custom.
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"proj\"\n"+
			"status.custom: \"resolved\"\n"+
			"export.git-add: false\n")

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	c := findCheck(r, "Beads config drift")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
	if !strings.Contains(c.Message, "types.custom") {
		t.Errorf("message should name missing key; got %q", c.Message)
	}
	if c.FixFunc == nil {
		t.Error("expected FixFunc")
	}
}

func TestCheckBeadsConfigDrift_UserAuthoredDrift(t *testing.T) {
	root := beadsRoot(t, false)
	// User flipped export.git-add to true on purpose.
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"proj\"\n"+
			"types.custom: \"gate\"\n"+
			"status.custom: \"resolved\"\n"+
			"export.git-add: true\n")

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	c := findCheck(r, "Beads config drift")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
	if !strings.Contains(c.Message, "export.git-add") {
		t.Errorf("message should name drifted key; got %q", c.Message)
	}
	if !strings.Contains(c.Message, "--force") {
		t.Errorf("message should mention --force; got %q", c.Message)
	}
}

func TestCheckBeadsConfigDrift_FixForceReplacesUserAuthored(t *testing.T) {
	root := beadsRoot(t, false)
	cfgPath := filepath.Join(root, ".beads", "config.yaml")
	writeFile(t, cfgPath,
		"issue-prefix: \"proj\"\n"+
			"types.custom: \"gate\"\n"+
			"status.custom: \"resolved\"\n"+
			"export.git-add: true\n")

	// --fix (force=false) leaves drift alone.
	r1 := &Report{}
	checkBeadsConfigDrift(r1, root, false)
	c1 := findCheck(r1, "Beads config drift")
	if c1 == nil || c1.FixFunc == nil {
		t.Fatal("expected FixFunc")
	}
	if err := c1.FixFunc(); err != nil {
		t.Fatalf("fix: %v", err)
	}
	got, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(got), "export.git-add: true") {
		t.Errorf("--fix alone should preserve user-authored true; got:\n%s", got)
	}

	// --fix --force (force=true) replaces it.
	r2 := &Report{}
	checkBeadsConfigDrift(r2, root, true)
	c2 := findCheck(r2, "Beads config drift")
	if c2 == nil || c2.FixFunc == nil {
		t.Fatal("expected FixFunc")
	}
	if err := c2.FixFunc(); err != nil {
		t.Fatalf("fix --force: %v", err)
	}
	got, _ = os.ReadFile(cfgPath)
	if strings.Contains(string(got), "export.git-add: true") {
		t.Errorf("--fix --force should replace user-authored true; got:\n%s", got)
	}
	if !strings.Contains(string(got), "export.git-add: false") {
		t.Errorf("--fix --force should write false; got:\n%s", got)
	}
}

func TestCheckBeadsConfigDrift_MalformedYAML(t *testing.T) {
	root := beadsRoot(t, false)
	// Unterminated flow mapping — yaml.v3 refuses to parse.
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"proj\"\ntypes.custom: {oops\n")

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	c := findCheck(r, "Beads config drift")
	if c == nil {
		t.Fatal("expected a check for malformed config")
	}
	if c.Status != Warn {
		t.Errorf("status = %d, want Warn", c.Status)
	}
	if !strings.Contains(c.Message, "cannot scan") {
		t.Errorf("message should explain the scan failure; got %q", c.Message)
	}
	// No FixFunc is attached — we don't auto-repair a file we can't parse.
	if c.FixFunc != nil {
		t.Error("FixFunc must be nil on parse failure (refuse to blindly overwrite user YAML)")
	}
}

func TestCheckBeadsConfigDrift_NoBeadsDirSilent(t *testing.T) {
	root := t.TempDir()

	r := &Report{}
	checkBeadsConfigDrift(r, root, false)

	if len(r.Checks) != 0 {
		t.Errorf("expected no checks when .beads/ absent, got %d", len(r.Checks))
	}
}

// ─── checkStrayRootJSONL ──────────────────────────────────────────────────

func TestCheckStrayRootJSONL_Clean(t *testing.T) {
	root := beadsRoot(t, true)

	r := &Report{}
	checkStrayRootJSONL(r, root)

	if findCheck(r, "Stray root issues.jsonl") != nil {
		t.Error("clean repo should produce no check")
	}
}

func TestCheckStrayRootJSONL_Tracked(t *testing.T) {
	root := beadsRoot(t, true)
	writeFile(t, filepath.Join(root, "issues.jsonl"), "{}\n")
	runGit(t, root, "add", "issues.jsonl")
	runGit(t, root, "commit", "-m", "stray")

	r := &Report{}
	checkStrayRootJSONL(r, root)

	c := findCheck(r, "Stray root issues.jsonl")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
	if !strings.Contains(c.Message, "git rm --cached issues.jsonl") {
		t.Errorf("expected remediation in message; got %q", c.Message)
	}
}

// ─── checkDurabilityRisk ──────────────────────────────────────────────────

func TestCheckDurabilityRisk_AutoExportOn(t *testing.T) {
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"p\"\nexport.auto: true\n")

	r := &Report{}
	checkDurabilityRisk(r, root)

	if findCheck(r, "Beads durability") != nil {
		t.Error("export.auto=true should produce no warn")
	}
}

func TestCheckDurabilityRisk_AutoExportOff_NoRemote(t *testing.T) {
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"p\"\nexport.auto: false\n")
	// Write a repo_state.json with no remotes so detection is deterministic.
	writeFile(t, filepath.Join(root, ".beads", "dolt", ".dolt", "repo_state.json"),
		`{"remotes":{}}`)

	r := &Report{}
	checkDurabilityRisk(r, root)

	c := findCheck(r, "Beads durability")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
}

func TestCheckDurabilityRisk_AutoExportOff_WithRemote(t *testing.T) {
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"p\"\nexport.auto: false\nsync.remote: \"origin\"\n")

	r := &Report{}
	checkDurabilityRisk(r, root)

	if findCheck(r, "Beads durability") != nil {
		t.Error("remote configured should suppress warn")
	}
}

func TestCheckDurabilityRisk_AutoExportOff_RemoteViaConfigJSON(t *testing.T) {
	// When repo_state.json is absent but config.json declares remotes,
	// detectDoltRemote should still recognize the remote.
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"p\"\nexport.auto: false\n")
	writeFile(t, filepath.Join(root, ".beads", "dolt", ".dolt", "config.json"),
		`{"remotes":{"origin":{"url":"https://example.com/foo"}}}`)

	r := &Report{}
	checkDurabilityRisk(r, root)

	if findCheck(r, "Beads durability") != nil {
		t.Error("config.json with remotes should suppress the durability warn")
	}
}

func TestCheckDurabilityRisk_AutoExportOff_RemoteUnknown(t *testing.T) {
	// export.auto: false but no repo_state.json and no sync.remote → skip
	// the check with an info-level OK rather than false-warn.
	root := beadsRoot(t, false)
	writeFile(t, filepath.Join(root, ".beads", "config.yaml"),
		"issue-prefix: \"p\"\nexport.auto: false\n")

	r := &Report{}
	checkDurabilityRisk(r, root)

	c := findCheck(r, "Beads durability")
	if c == nil {
		t.Fatal("expected skipped-info check, got nothing")
	}
	if c.Status != OK {
		t.Errorf("status = %d, want OK (skipped)", c.Status)
	}
	if !strings.Contains(c.Message, "skipped") {
		t.Errorf("message should note skip; got %q", c.Message)
	}
}

// ─── checkBdVersionFloor ──────────────────────────────────────────────────

// stubBd writes a fake `bd` into a temp dir and returns a PATH that resolves
// it first. The script prints `output` on stdout.
func stubBd(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho " + shellQuote(output) + "\n"
	path := filepath.Join(dir, "bd")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func shellQuote(s string) string {
	// Single-quote and escape embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func TestParseBdVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"bd version 1.0.2 (Homebrew)", "1.0.2", true},
		{"v1.0.2", "1.0.2", true},
		{"beads 0.62.1", "0.62.1", true},
		{"2.0.0-beta", "2.0.0", true},
		{"unknown", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := parseBdVersion(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseBdVersion(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.2", "1.0.2", 0},
		{"1.0.1", "1.0.2", -1},
		{"1.0.3", "1.0.2", 1},
		{"0.62.4", "1.0.2", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tc := range cases {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckBdVersionFloor_BelowFloor(t *testing.T) {
	path := stubBd(t, "bd version 1.0.1 (Homebrew)")
	t.Setenv("PATH", path)
	root := beadsRoot(t, false)

	r := &Report{}
	checkBdVersionFloor(r, root)

	c := findCheck(r, "bd version floor")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
	if !strings.Contains(c.Message, "1.0.1") {
		t.Errorf("message should include actual version; got %q", c.Message)
	}
}

func TestCheckBdVersionFloor_AtOrAbove(t *testing.T) {
	path := stubBd(t, "bd version 1.0.2 (Homebrew)")
	t.Setenv("PATH", path)
	root := beadsRoot(t, false)

	r := &Report{}
	checkBdVersionFloor(r, root)

	c := findCheck(r, "bd version floor")
	if c == nil || c.Status != OK {
		t.Fatalf("got %+v, want OK", c)
	}
}

// ─── integration: full doctor flow on a bd-default config ────────────────

// bdDefaultConfig is a config.yaml shape similar to what `bd init` produces
// before mindspec's required keys are added.
const bdDefaultConfig = `# Beads Configuration File
issue-prefix: "proj"
events-export: true
sync-branch: "beads-sync"
`

func TestDoctor_IntegrationBdDefaultThenFix(t *testing.T) {
	root := beadsRoot(t, false)
	cfgPath := filepath.Join(root, ".beads", "config.yaml")
	writeFile(t, cfgPath, bdDefaultConfig)

	// First run: should report config drift (missing types.custom,
	// status.custom, export.git-add).
	r1 := &Report{}
	checkBeadsConfigDrift(r1, root, false)
	c1 := findCheck(r1, "Beads config drift")
	if c1 == nil || c1.Status != Warn {
		t.Fatalf("pre-fix: got %+v, want Warn", c1)
	}
	for _, key := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !strings.Contains(c1.Message, key) {
			t.Errorf("message should report missing %q; got %q", key, c1.Message)
		}
	}

	// Apply the fix.
	if err := c1.FixFunc(); err != nil {
		t.Fatalf("fix: %v", err)
	}

	// Second run: should now be clean.
	r2 := &Report{}
	checkBeadsConfigDrift(r2, root, false)
	c2 := findCheck(r2, "Beads config drift")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("post-fix: got %+v, want OK", c2)
	}

	// User-authored issue-prefix should be preserved.
	got, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(got), `issue-prefix: "proj"`) {
		t.Errorf("fix should preserve user-authored issue-prefix; got:\n%s", got)
	}
}

func TestCheckBdVersionFloor_UnknownFormatSkips(t *testing.T) {
	path := stubBd(t, "unrecognized version output")
	t.Setenv("PATH", path)
	root := beadsRoot(t, false)

	r := &Report{}
	checkBdVersionFloor(r, root)

	c := findCheck(r, "bd version floor")
	if c == nil {
		t.Fatal("expected skipped-info check, got nothing")
	}
	if c.Status != OK {
		t.Errorf("status = %d, want OK (skipped)", c.Status)
	}
	if !strings.Contains(c.Message, "skipped") {
		t.Errorf("message should say skipped; got %q", c.Message)
	}
}
