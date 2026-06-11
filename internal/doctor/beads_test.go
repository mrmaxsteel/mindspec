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
	// 1.0.3 is server-mode bd — below the 1.0.4 embedded-mode floor.
	path := stubBd(t, "bd version 1.0.3 (Homebrew)")
	t.Setenv("PATH", path)
	root := beadsRoot(t, false)

	r := &Report{}
	checkBdVersionFloor(r, root)

	c := findCheck(r, "bd version floor")
	if c == nil || c.Status != Warn {
		t.Fatalf("got %+v, want Warn", c)
	}
	if !strings.Contains(c.Message, "1.0.3") {
		t.Errorf("message should include actual version; got %q", c.Message)
	}
}

func TestCheckBdVersionFloor_AtOrAbove(t *testing.T) {
	path := stubBd(t, "bd version 1.0.4 (Homebrew)")
	t.Setenv("PATH", path)
	root := beadsRoot(t, false)

	r := &Report{}
	checkBdVersionFloor(r, root)

	c := findCheck(r, "bd version floor")
	if c == nil || c.Status != OK {
		t.Fatalf("got %+v, want OK", c)
	}
}

// ─── checkBeadsMergeDriver ────────────────────────────────────────────────

// gitConfigGet reads a git config key in a test repo, returning "" when unset.
func gitConfigGet(t *testing.T, root, key string) string {
	t.Helper()
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writeExecutable drops an executable shell stub at path.
func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

const beadsMergeAttr = ".beads/issues.jsonl merge=beads\n"

func TestCheckBeadsMergeDriver(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantNone   bool   // no check at all
		wantStatus Status // when !wantNone
		wantMsg    []string
		wantFix    bool
	}{
		{
			name:     "no attribute and no driver is silent",
			setup:    func(t *testing.T, root string) {},
			wantNone: true,
		},
		{
			name: "dead bd merge driver flagged",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				writeExecutable(t, filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh"))
				runGit(t, root, "config", "merge.beads.driver", "bd merge %A %O %A %B")
			},
			wantStatus: Error,
			wantMsg:    []string{"bd merge", "removed in bd 1.0.x", "recovery: git config merge.beads.driver"},
		},
		{
			name: "driver command not on PATH flagged",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				runGit(t, root, "config", "merge.beads.driver", "no-such-merge-driver-xyz %A %O %B")
			},
			wantStatus: Error,
			wantMsg:    []string{"not found on PATH", "recovery:"},
		},
		{
			name: "driver absolute path missing flagged",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				runGit(t, root, "config", "merge.beads.driver", "/nonexistent/bd-jsonl-merge-driver.sh %A %O %B")
			},
			wantStatus: Error,
			wantMsg:    []string{"does not exist", "recovery:"},
		},
		{
			name: "driver script without execute bit flagged",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				script := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
				writeFile(t, script, "#!/bin/sh\nexit 0\n") // 0644
				runGit(t, root, "config", "merge.beads.driver", script+" %A %O %B")
			},
			wantStatus: Error,
			wantMsg:    []string{"not executable", "recovery:"},
		},
		{
			name: "healthy wrapper with absolute path passes",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				script := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
				writeExecutable(t, script)
				runGit(t, root, "config", "merge.beads.driver", script+" %A %O %B")
			},
			wantStatus: OK,
			wantMsg:    []string{"resolves"},
		},
		{
			name: "healthy wrapper with relative path resolves against root",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				writeExecutable(t, filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh"))
				runGit(t, root, "config", "merge.beads.driver", "scripts/bd-jsonl-merge-driver.sh %A %O %B")
			},
			wantStatus: OK,
			wantMsg:    []string{"resolves"},
		},
		{
			name: "attribute without driver flagged fixable when script exists",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				writeExecutable(t, filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh"))
			},
			wantStatus: Error,
			wantMsg:    []string{"merge.beads.driver is not", "plain text merge", "recovery: git config merge.beads.driver"},
			wantFix:    true,
		},
		{
			name: "attribute without driver and missing script flagged manual",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
			},
			// The unfixable state must not be milder than the fixable one.
			wantStatus: Error,
			wantMsg:    []string{"does not exist in this repo", "recovery: git config merge.beads.driver"},
		},
		{
			name: "configured driver without merge=beads attribute flagged",
			setup: func(t *testing.T, root string) {
				script := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
				writeExecutable(t, script)
				runGit(t, root, "config", "merge.beads.driver", script+" %A %O %B")
			},
			wantStatus: Error,
			wantMsg:    []string{"no merge=beads", "despite the configured", "recovery: printf"},
		},
		{
			name: "bare command name resolving on PATH passes",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
				dir := t.TempDir()
				writeExecutable(t, filepath.Join(dir, "stub-bd-merge-driver"))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				runGit(t, root, "config", "merge.beads.driver", "stub-bd-merge-driver %A %O %B")
			},
			wantStatus: OK,
			wantMsg:    []string{"resolves"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := beadsRoot(t, true)
			tc.setup(t, root)

			r := &Report{}
			checkBeadsMergeDriver(r, root)

			c := findCheck(r, "Beads merge driver")
			if tc.wantNone {
				if c != nil {
					t.Fatalf("expected no check, got %+v", c)
				}
				return
			}
			if c == nil {
				t.Fatal("missing check")
			}
			if c.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d (message %q)", c.Status, tc.wantStatus, c.Message)
			}
			for _, want := range tc.wantMsg {
				if !strings.Contains(c.Message, want) {
					t.Errorf("message should contain %q; got %q", want, c.Message)
				}
			}
			if tc.wantFix && c.FixFunc == nil {
				t.Fatal("expected FixFunc")
			}
			if !tc.wantFix && c.FixFunc != nil {
				t.Error("FixFunc must be nil for this case")
			}
		})
	}
}

func TestCheckBeadsMergeDriver_FixWritesConfigAndRecheckPasses(t *testing.T) {
	root := beadsRoot(t, true)
	writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
	script := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
	writeExecutable(t, script)

	r1 := &Report{}
	checkBeadsMergeDriver(r1, root)
	c1 := findCheck(r1, "Beads merge driver")
	if c1 == nil || c1.FixFunc == nil {
		t.Fatalf("expected fixable check, got %+v", c1)
	}
	if err := c1.FixFunc(); err != nil {
		t.Fatalf("fix: %v", err)
	}

	got := gitConfigGet(t, root, "merge.beads.driver")
	toks := driverTokens(got)
	if len(toks) == 0 || !filepath.IsAbs(toks[0]) {
		t.Errorf("fix should write an absolute script path; got %q (tokens %v)", got, toks)
	}
	if !strings.HasSuffix(got, " %A %O %B") {
		t.Errorf("fix should pass %%A %%O %%B; got %q", got)
	}
	if !strings.Contains(got, "bd-jsonl-merge-driver.sh") {
		t.Errorf("fix should point at the wrapper script; got %q", got)
	}

	// Re-run: the repaired config must now pass.
	r2 := &Report{}
	checkBeadsMergeDriver(r2, root)
	c2 := findCheck(r2, "Beads merge driver")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("post-fix: got %+v, want OK", c2)
	}
}

// TestCheckBeadsMergeDriver_FixSurvivesPathWithSpaces is the PR #128 panel
// blocker regression test: git runs merge drivers via `sh -c`, so an
// unquoted absolute script path word-splits when the repo lives under a
// directory with spaces. The written config must single-quote the path,
// round-trip through driverTokens, re-validate OK, AND drive an actual
// both-sides-changed git merge in the fixture repo.
func TestCheckBeadsMergeDriver_FixSurvivesPathWithSpaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "pr128 space", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")

	writeFile(t, filepath.Join(root, ".gitattributes"), beadsMergeAttr)
	// Stub wrapper: writes a sentinel into %A so the merge result proves
	// the driver actually ran (a silent text merge could never produce it).
	script := filepath.Join(root, "scripts", "bd-jsonl-merge-driver.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'driver-merged\\n' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	issues := filepath.Join(root, ".beads", "issues.jsonl")
	writeFile(t, issues, "base\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "base")

	// Attribute present, no driver configured → fixable check.
	r1 := &Report{}
	checkBeadsMergeDriver(r1, root)
	c1 := findCheck(r1, "Beads merge driver")
	if c1 == nil || c1.FixFunc == nil {
		t.Fatalf("expected fixable check, got %+v", c1)
	}
	if err := c1.FixFunc(); err != nil {
		t.Fatalf("fix: %v", err)
	}

	// Written value must round-trip through driverTokens to the script path.
	got := gitConfigGet(t, root, "merge.beads.driver")
	toks := driverTokens(got)
	if len(toks) == 0 || toks[0] != script {
		t.Fatalf("written config %q does not round-trip through driverTokens to %q (tokens %v)", got, script, toks)
	}

	// Re-validation must pass despite the space in the path.
	r2 := &Report{}
	checkBeadsMergeDriver(r2, root)
	c2 := findCheck(r2, "Beads merge driver")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("post-fix: got %+v, want OK", c2)
	}

	// Live proof: a both-sides-changed merge must invoke the driver. With
	// an unquoted path this fails under sh word-splitting (reproduced on
	// the pre-fix code with root '/tmp/pr128 space/repo').
	runGit(t, root, "checkout", "-q", "-b", "side")
	writeFile(t, issues, "side\n")
	runGit(t, root, "commit", "-q", "-am", "side")
	runGit(t, root, "checkout", "-q", "main")
	writeFile(t, issues, "main\n")
	runGit(t, root, "commit", "-q", "-am", "main")
	runGit(t, root, "merge", "-q", "--no-edit", "side")

	data, err := os.ReadFile(issues)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "driver-merged\n" {
		t.Errorf("merge driver did not run; issues.jsonl = %q", data)
	}
}

func TestDriverTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"bd merge %A %O %A %B", []string{"bd", "merge", "%A", "%O", "%A", "%B"}},
		{"/abs/path/driver.sh %A %O %B", []string{"/abs/path/driver.sh", "%A", "%O", "%B"}},
		{"'/path with space/d.sh' %A", []string{"/path with space/d.sh", "%A"}},
		{`"/path with space/d.sh" %A`, []string{"/path with space/d.sh", "%A"}},
		{"  spaced   out  ", []string{"spaced", "out"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		got := driverTokens(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("driverTokens(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("driverTokens(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestGitattributesHasBeadsMerge(t *testing.T) {
	cases := []struct {
		name    string
		content string // "" means no file at all
		write   bool
		want    bool
	}{
		{name: "no file", write: false, want: false},
		{name: "attribute present", write: true, content: ".beads/issues.jsonl merge=beads\n", want: true},
		{name: "comment only", write: true, content: "# .beads/issues.jsonl merge=beads\n", want: false},
		{name: "other attributes", write: true, content: "*.go text eol=lf\n", want: false},
		{name: "different merge driver", write: true, content: "*.lock merge=ours\n", want: false},
		{name: "among other lines", write: true, content: "*.go text\n.beads/issues.jsonl merge=beads\n", want: true},
		// D2 mutation coverage: merge=beads must be found beyond fields[1].
		{name: "multi-attribute line", write: true, content: ".beads/issues.jsonl -text merge=beads\n", want: true},
		{name: "tab separated", write: true, content: ".beads/issues.jsonl\tmerge=beads\n", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.write {
				writeFile(t, filepath.Join(root, ".gitattributes"), tc.content)
			}
			if got := gitattributesHasBeadsMerge(root); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
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
