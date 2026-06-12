package hook

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// writePanelFixture writes review/<slug>/panel.json + verdict files under
// root. panelJSON is the marshaled Panel; verdicts maps filename → verdict
// string.
func writePanelFixture(t *testing.T, root, slug string, p panel.Panel, verdicts map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "review", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, v := range verdicts {
		vd, _ := json.Marshal(map[string]string{"verdict": v})
		if err := os.WriteFile(filepath.Join(dir, name), vd, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// stubScanRoots pins runPreComplete's I/O seams for a test: root resolution,
// config (PanelGate on), the bead→spec lookup, rev-parse, and porcelain.
// Returns a restore func.
func stubScanRoots(t *testing.T, root, headSHA string, revErr error, porcelain string, statusErr error) func() {
	t.Helper()
	origFind := preCompleteFindRootFn
	origCfg := preCompleteConfigLoadFn
	origLookup := beadSpecLookupFn
	origRev := preCompleteRevParseFn
	origStatus := preCompleteStatusFn
	origWtList := worktreeListFn

	preCompleteFindRootFn = func(string) (string, error) { return root, nil }
	preCompleteConfigLoadFn = func(string) (*config.Config, error) { return config.DefaultConfig(), nil }
	beadSpecLookupFn = func(string) (string, error) { return "", nil } // force fallback to (a)/(c)
	preCompleteRevParseFn = func(string, string) (string, error) { return headSHA, revErr }
	preCompleteStatusFn = func(string) (string, error) { return porcelain, statusErr }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	return func() {
		preCompleteFindRootFn = origFind
		preCompleteConfigLoadFn = origCfg
		beadSpecLookupFn = origLookup
		preCompleteRevParseFn = origRev
		preCompleteStatusFn = origStatus
		worktreeListFn = origWtList
	}
}

// TestRunPreComplete_NonMatch_ZeroCost asserts HC-3: a non-matching Bash
// command exits Pass with NO config/git/fs/lookup work (every seam is wired
// to FAIL the test if invoked).
func TestRunPreComplete_NonMatch_ZeroCost(t *testing.T) {
	origFind := preCompleteFindRootFn
	origLookup := beadSpecLookupFn
	origRev := preCompleteRevParseFn
	defer func() {
		preCompleteFindRootFn = origFind
		beadSpecLookupFn = origLookup
		preCompleteRevParseFn = origRev
	}()
	preCompleteFindRootFn = func(string) (string, error) { t.Fatal("FindLocalRoot called on non-match"); return "", nil }
	beadSpecLookupFn = func(string) (string, error) { t.Fatal("bead lookup called on non-match"); return "", nil }
	preCompleteRevParseFn = func(string, string) (string, error) { t.Fatal("rev-parse called on non-match"); return "", nil }

	for _, cmd := range []string{
		"git commit -m \"mindspec complete next\"",
		"grep 'mindspec complete' SKILL.md",
		"ls -la",
		"",
	} {
		r := runPreComplete(&Input{Command: cmd})
		if r.Action != Pass {
			t.Errorf("non-match %q: expected Pass, got %v", cmd, r.Action)
		}
	}
}

// TestRunPreComplete_BareComplete_Pass: `mindspec complete` with no id
// passes (explicit-id only in v1, Req 10).
func TestRunPreComplete_BareComplete_Pass(t *testing.T) {
	r := runPreComplete(&Input{Command: "mindspec complete"})
	if r.Action != Pass {
		t.Errorf("bare complete: expected Pass, got %v (%s)", r.Action, r.Message)
	}
}

// TestRunPreComplete_EscapeHatch: env set → Pass+Warn naming the bead,
// before any config/git work; the Warn message never names the variable in
// a Block (it is a Warn here, but assert it does name the bead).
func TestRunPreComplete_EscapeHatch(t *testing.T) {
	t.Setenv(SkipPanelEnv, "1")
	orig := preCompleteFindRootFn
	defer func() { preCompleteFindRootFn = orig }()
	preCompleteFindRootFn = func(string) (string, error) { t.Fatal("config work despite skip env"); return "", nil }
	r := runPreComplete(&Input{Command: "mindspec complete mindspec-bd01"})
	if r.Action != Warn {
		t.Fatalf("escape hatch: expected Warn, got %v", r.Action)
	}
	if !strings.Contains(r.Message, "mindspec-bd01") {
		t.Errorf("escape-hatch warn should name the bead: %s", r.Message)
	}
}

// TestRunPreComplete_ConfigToggle: panel_gate:false → Pass before any panel
// scan (the scan/rev-parse seams would fail the test if reached).
func TestRunPreComplete_ConfigToggle(t *testing.T) {
	origFind := preCompleteFindRootFn
	origCfg := preCompleteConfigLoadFn
	origRev := preCompleteRevParseFn
	defer func() {
		preCompleteFindRootFn = origFind
		preCompleteConfigLoadFn = origCfg
		preCompleteRevParseFn = origRev
	}()
	preCompleteFindRootFn = func(string) (string, error) { return "/root", nil }
	preCompleteConfigLoadFn = func(string) (*config.Config, error) {
		c := config.DefaultConfig()
		c.Enforcement.PanelGate = false
		return c, nil
	}
	preCompleteRevParseFn = func(string, string) (string, error) { t.Fatal("git work despite toggle off"); return "", nil }

	r := runPreComplete(&Input{Command: "mindspec complete mindspec-bd01"})
	if r.Action != Pass {
		t.Errorf("config toggle off: expected Pass, got %v (%s)", r.Action, r.Message)
	}
}

// TestRunPreComplete_NoPanel_FailOpen: matched complete but no panel.json
// referencing the bead → Pass with no output (HC-4).
func TestRunPreComplete_NoPanel_FailOpen(t *testing.T) {
	root := t.TempDir()
	restore := stubScanRoots(t, root, "sha", nil, "", nil)
	defer restore()
	// review/ dir exists but no panel.json (legacy BRIEF-only).
	os.MkdirAll(filepath.Join(root, "review", "legacy"), 0o755)
	os.WriteFile(filepath.Join(root, "review", "legacy", "BRIEF.md"), []byte("x"), 0o644)

	r := runPreComplete(&Input{Command: "mindspec complete mindspec-bd01"})
	if r.Action != Pass {
		t.Errorf("no panel.json: expected fail-open Pass, got %v (%s)", r.Action, r.Message)
	}
}

// TestRunPreComplete_IncompletePanel_Block exercises the full wiring through
// to a Block: a registered incomplete panel for the bead.
func TestRunPreComplete_IncompletePanel_Block(t *testing.T) {
	root := t.TempDir()
	sha := "abc1234def5678abc1234def5678abc1234def56"
	restore := stubScanRoots(t, root, sha, nil, "", nil)
	defer restore()
	writePanelFixture(t, root, "093-bd01", panel.Panel{
		BeadID: ptr("mindspec-bd01"), Spec: "093", Round: 1,
		ExpectedReviewers: 6, ReviewedHeadSHA: sha,
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE",
		"c-round-1.json": "APPROVE", "d-round-1.json": "APPROVE",
	})

	r := runPreComplete(&Input{Command: "mindspec complete mindspec-bd01"})
	if r.Action != Block {
		t.Fatalf("incomplete panel: expected Block, got %v (%s)", r.Action, r.Message)
	}
	if !strings.Contains(r.Message, "incomplete") || !strings.Contains(r.Message, "4/6") {
		t.Errorf("block should cite incompleteness 4/6: %s", r.Message)
	}
}

// TestRunPreComplete_CwdIndependence_CdPrefix: panel.json lives in a "spec
// worktree" reached only via the command's `cd <worktree>` prefix; the
// session cwd (root) has no panel. cd-prefix scan root (a) must find it →
// Block (Spec 093 AC cwd independence).
func TestRunPreComplete_CwdIndependence_CdPrefix(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "wt")
	os.MkdirAll(worktree, 0o755)
	sha := "abc1234def5678abc1234def5678abc1234def56"

	origFind := preCompleteFindRootFn
	origCfg := preCompleteConfigLoadFn
	origLookup := beadSpecLookupFn
	origRev := preCompleteRevParseFn
	origStatus := preCompleteStatusFn
	origWtList := worktreeListFn
	defer func() {
		preCompleteFindRootFn = origFind
		preCompleteConfigLoadFn = origCfg
		beadSpecLookupFn = origLookup
		preCompleteRevParseFn = origRev
		preCompleteStatusFn = origStatus
		worktreeListFn = origWtList
	}()
	// FindLocalRoot returns its own argument (so the cd-prefix path resolves
	// to the worktree, and the session-cwd path resolves to root).
	preCompleteFindRootFn = func(p string) (string, error) {
		if p == "" {
			return root, nil
		}
		return p, nil
	}
	preCompleteConfigLoadFn = func(string) (*config.Config, error) { return config.DefaultConfig(), nil }
	beadSpecLookupFn = func(string) (string, error) { return "", nil }
	preCompleteRevParseFn = func(string, string) (string, error) { return sha, nil }
	preCompleteStatusFn = func(string) (string, error) { return "", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }

	// Panel only under the worktree, NOT under root (session cwd).
	writePanelFixture(t, worktree, "093-bd01", panel.Panel{
		BeadID: ptr("mindspec-bd01"), Spec: "093", Round: 1,
		ExpectedReviewers: 6, ReviewedHeadSHA: sha,
	}, map[string]string{"a-round-1.json": "APPROVE"})

	cmd := "cd " + worktree + " && mindspec complete mindspec-bd01"
	r := runPreComplete(&Input{Command: cmd})
	if r.Action != Block {
		t.Fatalf("cd-prefix cwd independence: expected Block (panel only in worktree), got %v (%s)", r.Action, r.Message)
	}
}

// --- a small real-git smoke test wiring the default rev-parse seam --------

func TestRunPreComplete_StaleSHA_RealGit(t *testing.T) {
	root := t.TempDir()
	mustGitInit(t, root)
	// Create the bead branch at a known commit.
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-b", "bead/mindspec-bd01")
	run("commit", "--allow-empty", "-m", "bead work")

	origFind := preCompleteFindRootFn
	origCfg := preCompleteConfigLoadFn
	origLookup := beadSpecLookupFn
	origStatus := preCompleteStatusFn
	origWtList := worktreeListFn
	defer func() {
		preCompleteFindRootFn = origFind
		preCompleteConfigLoadFn = origCfg
		beadSpecLookupFn = origLookup
		preCompleteStatusFn = origStatus
		worktreeListFn = origWtList
	}()
	preCompleteFindRootFn = func(p string) (string, error) { return root, nil }
	preCompleteConfigLoadFn = func(string) (*config.Config, error) { return config.DefaultConfig(), nil }
	beadSpecLookupFn = func(string) (string, error) { return "", nil }
	preCompleteStatusFn = func(string) (string, error) { return "", nil }
	worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
	// Leaves preCompleteRevParseFn as the real gitutil.RevParseRef.

	// panel reviewed a DIFFERENT sha than the live branch HEAD → stale.
	writePanelFixture(t, root, "093-bd01", panel.Panel{
		BeadID: ptr("mindspec-bd01"), Spec: "093", Round: 1,
		ExpectedReviewers: 6, ReviewedHeadSHA: "0000000000000000000000000000000000000000",
	}, map[string]string{
		"a-round-1.json": "APPROVE", "b-round-1.json": "APPROVE", "c-round-1.json": "APPROVE",
		"d-round-1.json": "APPROVE", "e-round-1.json": "APPROVE", "f-round-1.json": "APPROVE",
	})

	origDir, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origDir)

	r := runPreComplete(&Input{Command: "mindspec complete mindspec-bd01"})
	if r.Action != Block {
		t.Fatalf("stale SHA: expected Block, got %v (%s)", r.Action, r.Message)
	}
	if !strings.Contains(r.Message, "reviewed") || !strings.Contains(r.Message, "branch now at") {
		t.Errorf("stale-SHA block should cite reviewed vs current: %s", r.Message)
	}
}
