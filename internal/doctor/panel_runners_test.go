package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/rules"
)

const doctorConfigBase = "source_globs: [internal/**]\nmodels: {authoring: test-model}\ncommands: {test: go-test}\n"

func doctorRunnerRoot(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	doctorGit(t, root, "init", "-q")
	writeConfig(t, root, doctorConfigBase+contents)
	writeFile(t, filepath.Join(root, ".gitignore"), ".mindspec/config.local.yaml\n.mindspec-scratch/\n")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	return root
}

func doctorGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Track only in the fixture index. No worktree or user repository is modified.
func doctorTrackLocal(t *testing.T, root string) {
	t.Helper()
	doctorTrack(t, root, localConfigPath)
}

func doctorTrack(t *testing.T, root, path string) {
	t.Helper()
	sha := doctorGit(t, root, "hash-object", "-w", path)
	doctorGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+sha+","+path)
}

func doctorConfigReport(root string, opts Options) *Report {
	r := &Report{}
	checkConfiguration(r, root, opts)
	return r
}

func doctorChecks(r *Report, name string) []Check {
	var checks []Check
	for _, check := range r.Checks {
		if check.Name == name {
			checks = append(checks, check)
		}
	}
	return checks
}

func requireDoctorCheck(t *testing.T, r *Report, name string, status Status, text string) {
	t.Helper()
	for _, c := range doctorChecks(r, name) {
		if c.Status == status && strings.Contains(c.Message, text) {
			return
		}
	}
	t.Fatalf("missing %q status %d containing %q; checks=%+v", name, status, text, r.Checks)
}

func doctorProbeConfig(command, probe string) string {
	return fmt.Sprintf("panel:\n  reviewers: [{family: process-a, count: 2}]\n  runners:\n    - family: process-a\n      kind: process\n      argv: [%q]\n      prompt: stdin\n      output: stdout\n      probe: [%q]\n", command, probe)
}

func doctorScript(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	writeFile(t, path, "#!/bin/sh\n"+body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertDoctorMarker(t *testing.T, path string, exists bool) {
	t.Helper()
	_, err := os.Stat(path)
	if exists && err != nil {
		t.Fatalf("expected probe marker: %v", err)
	}
	if !exists && !os.IsNotExist(err) {
		t.Fatalf("probe must not create marker; stat=%v", err)
	}
}

func TestDoctor_Launchers(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	root := doctorRunnerRoot(t, `panel:
  reviewers: [{family: harness, count: 2}]
  gates:
    plan_approve:
      reviewers: [{family: selected, count: 2}]
  runners:
    - {family: selected, kind: process, argv: [missing-review-command], prompt: stdin, output: stdout}
    - {family: unused, kind: process, argv: [unused-missing-command], prompt: stdin, output: stdout}
`)
	r := doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel launcher", Error, `runner "selected"`)
	if checks := doctorChecks(r, "Panel launcher"); len(checks) != 1 {
		t.Fatalf("check used runners once, skip unused runner: %+v", checks)
	}
	bin := t.TempDir()
	doctorScript(t, bin, "missing-review-command", "exit 99")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	r = doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel launcher", OK, `runner "selected"`)
}

func TestDoctor_ProbeNotRunFromTrackedConfig(t *testing.T) {
	root := doctorRunnerRoot(t, "")
	probe := doctorScript(t, root, "probe", ": > main-probe-marker")
	writeConfig(t, root, doctorConfigBase+doctorProbeConfig("/bin/sh", probe))
	doctorTrack(t, root, ".mindspec/config.yaml")
	config.ResetCache()
	r := doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel probe", Info, "was not run")
	if len(doctorChecks(r, "Panel probe")) != 1 {
		t.Fatal("a skipped probe must have exactly one Info line")
	}
	assertDoctorMarker(t, filepath.Join(root, "main-probe-marker"), false)
	r = doctorConfigReport(root, Options{Probe: true})
	requireDoctorCheck(t, r, "Panel probe", OK, "probe passed")
	assertDoctorMarker(t, filepath.Join(root, "main-probe-marker"), true)

	// A local override is eligible automatically, and its failure is an Error.
	localProbe := doctorScript(t, root, "local-probe", ": > local-probe-marker\nexit 1")
	writeFile(t, filepath.Join(root, localConfigPath), doctorProbeConfig("/bin/sh", localProbe))
	// Local config can set runners only; the reviewers remain in main config.
	local, err := os.ReadFile(filepath.Join(root, localConfigPath))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, localConfigPath), strings.Replace(string(local), "  reviewers: [{family: process-a, count: 2}]\n", "", 1))
	config.ResetCache()
	r = doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel probe", Error, "process-a")
	assertDoctorMarker(t, filepath.Join(root, "local-probe-marker"), true)
	if r.HasFailures() == false {
		t.Fatal("a failed probe must fail doctor")
	}
}

func TestDoctor_TrackedLocalProbe(t *testing.T) {
	root := doctorRunnerRoot(t, "panel:\n  reviewers: [{family: process-a, count: 2}]\n")
	probe := doctorScript(t, root, "probe", ": > tracked-probe-marker")
	contents := strings.Replace(doctorProbeConfig("/bin/sh", probe), "  reviewers: [{family: process-a, count: 2}]\n", "", 1)
	writeFile(t, filepath.Join(root, localConfigPath), contents)
	doctorTrackLocal(t, root)
	config.ResetCache()
	r := doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Local panel config", Warn, "is tracked")
	requireDoctorCheck(t, r, "Panel probe", Info, "was not run")
	assertDoctorMarker(t, filepath.Join(root, "tracked-probe-marker"), false)
	r = doctorConfigReport(root, Options{Probe: true})
	requireDoctorCheck(t, r, "Panel probe", OK, "passed")
	assertDoctorMarker(t, filepath.Join(root, "tracked-probe-marker"), true)
}

func TestDoctor_UntrackedMainProbe(t *testing.T) {
	root := doctorRunnerRoot(t, "")
	probe := doctorScript(t, root, "probe", ": > untracked-probe-marker")
	writeConfig(t, root, doctorConfigBase+doctorProbeConfig("/bin/sh", probe))
	config.ResetCache()
	if files := doctorGit(t, root, "ls-files", "--", ".mindspec/config.yaml"); files != "" {
		t.Fatalf("fixture config must be untracked: %q", files)
	}
	for _, probe := range []bool{false, true} {
		t.Run(fmt.Sprintf("probe=%v", probe), func(t *testing.T) {
			r := doctorConfigReport(root, Options{Probe: probe})
			assertDoctorMarker(t, filepath.Join(root, "untracked-probe-marker"), probe)
			if probe {
				requireDoctorCheck(t, r, "Panel probe", OK, "probe passed")
				return
			}
			requireDoctorCheck(t, r, "Panel probe", Info, "was not run")
			if len(doctorChecks(r, "Panel probe")) != 1 {
				t.Fatal("a skipped probe must have exactly one Info line")
			}
		})
	}
}

func TestDoctor_LoadErrorReported(t *testing.T) {
	for _, tc := range []struct{ name, main, local, recovery string }{
		{"retired", "panel:\n  substitution: {}\n", "", "set on_quota on the runner in panel.runners"},
		{"local unknown key", "", "unknown_key: true\n", "set only panel.runners and models"},
		{"local oversized", "", strings.Repeat("x", 65537), "set only panel.runners and models"},
		{"invalid yaml", "panel: [", "", "repair the config file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := doctorRunnerRoot(t, tc.main)
			if tc.local != "" {
				writeFile(t, filepath.Join(root, localConfigPath), tc.local)
			}
			r := doctorConfigReport(root, Options{})
			if len(r.Checks) != 1 || r.Checks[0].Status != Error || !strings.Contains(r.Checks[0].Message, "recovery: "+tc.recovery) {
				t.Fatalf("one load Error with recovery required: %+v", r.Checks)
			}
		})
	}
}

func TestDoctor_ConfigWarnings(t *testing.T) {
	root := doctorRunnerRoot(t, "panel:\n  isolation_root: custom-scratch\n  reviewers: [{family: harness, count: 2}]\n")
	writeFile(t, filepath.Join(root, localConfigPath), "models: {reviewer.harness: local-model}\n")
	writeFile(t, filepath.Join(root, ".gitignore"), "")
	r := doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel isolation ignore", Warn, "custom-scratch is not ignored\nrecovery: mindspec init")
	requireDoctorCheck(t, r, "Local config ignore", Warn, "config.local.yaml is not ignored\nrecovery: mindspec init")
	writeFile(t, filepath.Join(root, ".gitignore"), ".mindspec/config.local.yaml\ncustom-scratch/\n")
	r = doctorConfigReport(root, Options{})
	if len(doctorChecks(r, "Local config ignore"))+len(doctorChecks(r, "Panel isolation ignore")) != 0 {
		t.Fatalf("ignored paths must not warn: %+v", r.Checks)
	}

	// Detect a known tool via PATH, without ever starting it.
	bin := t.TempDir()
	tool := config.ReviewTools()[0]
	doctorScript(t, bin, tool.Command, ": > should-not-run\nexit 99")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	r = doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Review diversity", Info, tool.Name)
	if len(doctorChecks(r, "Review diversity")) != 1 || r.HasFailures() {
		t.Fatalf("suggestion must be one advisory line: %+v", r.Checks)
	}
	assertDoctorMarker(t, filepath.Join(root, "should-not-run"), false)
}

func TestDoctor_StagedMarkers(t *testing.T) {
	root := t.TempDir()
	for _, base := range []string{".claude/skills", ".agents/skills"} {
		writeFile(t, filepath.Join(root, base, "sample", "SKILL.md"), `
<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=2020-01-01 -->
<!-- staged-rule: id=unknown.rule; remove-by=2999-01-01 -->
<!-- staged-rule: remove-by=2999-01-01 -->
<!-- staged-rule: id=sequence.bead-impl-prompt; remove-by=2999-01-01 -->
`)
	}
	r := &Report{}
	checkStagedMarkers(r, root)
	if checks := doctorChecks(r, "Staged rule"); len(checks) != 6 {
		t.Fatalf("one Warn per invalid marker, valid marker stays quiet: %+v", checks)
	}
	for _, text := range []string{"past remove-by date 2020-01-01", "unknown id unknown.rule", "missing id"} {
		requireDoctorCheck(t, r, "Staged rule", Warn, text)
	}
	table, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	check := findCheck(r, "Enforced rules")
	if check == nil || check.Status != Info {
		t.Fatalf("missing enforced rule Info: %+v", r.Checks)
	}
	for _, rule := range table {
		want := rule.EnforcedBy != "advisory"
		if got := strings.Contains(check.Message, rule.ID+" ("); got != want {
			t.Errorf("enforced rule %s listed=%v, want=%v", rule.ID, got, want)
		}
	}
}

func doctorSlot(t *testing.T, root, spec, slug string, round int) panel.SlotLayout {
	t.Helper()
	layout, err := panel.NewSlotLayout(root, ".mindspec-scratch", spec, slug, round, "R1")
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: spec, Slug: slug, Round: round, Slot: "R1", MainRoot: root}); err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestDoctor_StaleCheckouts(t *testing.T) {
	for _, tc := range []struct {
		name, spec, reason string
		isolationRoot      string
		round              int
		abandoned, missing bool
	}{
		{"abandoned", "adhoc", "panel is abandoned", ".mindspec-scratch", 1, true, false},
		{"missing", "adhoc", "panel is missing", ".mindspec-scratch", 1, false, true},
		{"later", "adhoc", "panel is at a later round", ".mindspec-scratch", 2, false, false},
		{"current", "adhoc", "", ".mindspec-scratch", 1, false, false},
		{"spec abandoned", "131-test", "panel is abandoned", ".mindspec-scratch", 1, true, false},
		{"trailing slash", "adhoc", "panel is missing", ".mindspec-scratch/", 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := doctorRunnerRoot(t, "panel:\n  isolation_root: "+tc.isolationRoot+"\n")
			layout := doctorSlot(t, root, tc.spec, "p", 1)
			if !tc.missing {
				dir := filepath.Join(root, "review", "p")
				if tc.spec != "adhoc" {
					dir = filepath.Join(root, ".mindspec", "specs", tc.spec, "reviews", "p")
				}
				p := panel.Panel{Spec: tc.spec, Round: tc.round, Abandoned: tc.abandoned}
				if err := panel.WriteSnapshot(dir, p); err != nil {
					t.Fatal(err)
				}
			}
			r := doctorConfigReport(root, Options{})
			if tc.reason == "" {
				if len(doctorChecks(r, "Stale panel checkout")) != 0 {
					t.Fatalf("current checkout must stay quiet: %+v", r.Checks)
				}
				return
			}
			requireDoctorCheck(t, r, "Stale panel checkout", Warn, ": "+tc.reason+"\nrecovery:")
			requireDoctorCheck(t, r, "Stale panel checkout", Warn, "mindspec panel checkout p --prune")
			if len(doctorChecks(r, "Stale panel checkout")) != 1 {
				t.Fatal("one Warn required for one stale slot")
			}
			if _, err := os.Stat(layout.Metadata); err != nil {
				t.Fatalf("doctor must not prune: %v", err)
			}
		})
	}
}

func TestDoctor_StaleCheckoutsInvalidSpec(t *testing.T) {
	for _, spec := range []string{"../x", "invalid-spec"} {
		t.Run(spec, func(t *testing.T) {
			root := doctorRunnerRoot(t, "")
			layout := doctorSlot(t, root, "131-test", "p", 1)
			metadata := panel.SlotMetadata{Spec: spec, Slug: "p", Round: 1, Slot: "R1", MainRoot: root}
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, layout.Metadata, string(data))
			r := doctorConfigReport(root, Options{})
			requireDoctorCheck(t, r, "Panel checkouts", Warn, "invalid spec ID")
			if len(doctorChecks(r, "Panel checkouts")) != 1 || len(doctorChecks(r, "Stale panel checkout")) != 0 {
				t.Fatalf("invalid metadata must warn once and skip panel path lookup: %+v", r.Checks)
			}
			if len(doctorChecks(r, "MindSpec config")) != 0 {
				t.Fatalf("fixture config must load: %+v", r.Checks)
			}
		})
	}
}

func TestDoctor_CISkips(t *testing.T) {
	root := doctorRunnerRoot(t, "")
	probe := doctorScript(t, root, "probe", ": > ci-probe-marker")
	writeConfig(t, root, doctorConfigBase+doctorProbeConfig("not-on-path", probe))
	config.ResetCache()
	doctorSlot(t, root, "adhoc", "missing", 1)
	bin := t.TempDir()
	doctorScript(t, bin, config.ReviewTools()[0].Command, "exit 99")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	full := doctorConfigReport(root, Options{})
	for _, name := range []string{"Panel launcher", "Panel probe", "Review diversity", "Stale panel checkout"} {
		if len(doctorChecks(full, name)) == 0 {
			t.Fatalf("fixture must trigger %s before --ci: %+v", name, full.Checks)
		}
	}
	ci := doctorConfigReport(root, Options{SkipLocalEnv: true, Probe: true})
	for _, name := range []string{"Panel launcher", "Panel probe", "Review diversity", "Stale panel checkout"} {
		if len(doctorChecks(ci, name)) != 0 {
			t.Fatalf("--ci must omit %s: %+v", name, ci.Checks)
		}
	}
	assertDoctorMarker(t, filepath.Join(root, "ci-probe-marker"), false)
	// Config errors and model-only warnings remain under --ci.
	writeConfig(t, root, doctorConfigBase+"panel:\n  reviewers: [{family: absent, count: 2}]\n")
	config.ResetCache()
	requireDoctorCheck(t, doctorConfigReport(root, Options{SkipLocalEnv: true}), "Panel reviewers", Error, "no runner")
}

func TestDoctor_UnrunnableFamily(t *testing.T) {
	root := doctorRunnerRoot(t, "panel:\n  reviewers: [{family: absent, count: 2}]\n")
	r := doctorConfigReport(root, Options{})
	requireDoctorCheck(t, r, "Panel reviewers", Error, `panel.reviewers[0] names family "absent"`)
	requireDoctorCheck(t, r, "Panel reviewers", Error, "recovery: add a runner for the family to panel.runners")
	if len(doctorChecks(r, "Panel reviewers")) != 1 || !r.HasFailures() {
		t.Fatalf("one entry must give one Error across all gates: %+v", r.Checks)
	}
}

func TestDoctor_ModelOnlyWarn(t *testing.T) {
	root := doctorRunnerRoot(t, "panel:\n  reviewers: [{model: my-model, count: 2}]\n")
	r := doctorConfigReport(root, Options{SkipLocalEnv: true})
	requireDoctorCheck(t, r, "Panel reviewer", Warn, "panel.reviewers[0] runs on the harness runner; add a family and a runner to run it elsewhere")
	if len(doctorChecks(r, "Panel reviewer")) != 1 || r.HasFailures() {
		t.Fatalf("one model-only entry gives one advisory Warn: %+v", r.Checks)
	}
}

func TestDoctor_ProbeTimeout(t *testing.T) {
	root := doctorRunnerRoot(t, "")
	probe := doctorScript(t, root, "probe", "exec /bin/sleep 30")
	writeConfig(t, root, doctorConfigBase+doctorProbeConfig("/bin/sh", probe))
	config.ResetCache()
	start := time.Now()
	r := doctorConfigReport(root, Options{Probe: true})
	requireDoctorCheck(t, r, "Panel probe", Error, "did not exit within 10 seconds")
	if elapsed := time.Since(start); elapsed < 9*time.Second || elapsed > 15*time.Second {
		t.Fatalf("probe deadline not enforced: %v", elapsed)
	}
}
