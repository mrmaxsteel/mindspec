package main

// panel_adhoc_test.go: spec 123 Bead 4 (R8a/R8b/R8c) — the ad-hoc panel
// path's CLI-level pins. AC-15's verbatim repro (`panel create --gate
// adhoc`, no --spec, succeeds + tally reaches it) and the flag-contract
// refusals (--spec+--gate adhoc refused; a non-adhoc gate without --spec
// still errors — guard). AC-16 (gate isolation) lives in
// internal/complete; AC-17 (skill<->binary grep) lives in internal/setup
// — see those packages' own test files.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// mkFlatPanelTestRoot creates a FLAT-layout workspace root (a lifecycle
// child directory OTHER than "specs" — "adr" — so workspace.DetectLayout
// classifies it LayoutFlat via flatTreePresent) with ZERO specs: the
// AC-15 fixture shape is a repo that has never had a spec, proving the
// ad-hoc branch in panelDirFor never calls workspace.SpecDir.
func mkFlatPanelTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "adr"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec/adr: %v", err)
	}
	return root
}

// stubBareRefRevParse mimics real `git rev-parse`'s behavior on the two
// --target shapes AC-15 distinguishes (plan PF-1): a bare resolvable ref
// (any string not prefixed "commit ") resolves; the #209 issue's
// "commit <sha>" placeholder notation does not — `panel create` rev-parses
// the RAW --target string (panel.go's revParseForPanelFn seam), so a
// literal "commit "+sha is not a ref real `git rev-parse` would accept
// either. This stub proves the CLI takes the bare form, never the
// prefixed one, without spawning a real git subprocess.
func stubBareRefRevParse(_, target string) (string, error) {
	if strings.HasPrefix(target, "commit ") {
		return "", fmt.Errorf("fatal: ambiguous argument %q: unknown revision or path not in the working tree", target)
	}
	return "cafebabecafebabecafebabecafebabecafebabe", nil
}

// runPanelCmd executes `mindspec panel <args...>` against rootCmd and
// returns the combined stdout+stderr text and the RunE error.
func runPanelCmd(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(append([]string{"panel"}, args...))
	err := rootCmd.Execute()
	return stdout.String() + stderr.String(), err
}

// TestPanelCreate_AdHocNoSpec_Succeeds pins AC-15's verbatim repro:
// `mindspec panel create adr-review --gate adhoc --target <bare-ref>`, NO
// --spec, in a repo with ZERO specs succeeds (exit 0), writes panel.json
// under .mindspec/reviews/adr-review/ with gate "adhoc" and no spec/bead,
// and `panel tally` reaches it (R8c tally-reach through
// configShowReviewRoots's new .mindspec root — never
// "no registered panel found").
func TestPanelCreate_AdHocNoSpec_Succeeds(t *testing.T) {
	resetPanelCreateFlags(t)
	root := mkFlatPanelTestRoot(t)
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	stubWorktreeListEmpty(t)

	origRevParse := revParseForPanelFn
	t.Cleanup(func() { revParseForPanelFn = origRevParse })
	revParseForPanelFn = stubBareRefRevParse

	bareRef := "cafebabecafebabecafebabecafebabecafebabe"
	out, err := runPanelCmd("create", "adr-review", "--gate", "adhoc", "--target", bareRef)
	if err != nil {
		t.Fatalf("ad-hoc panel create (no --spec, zero specs in repo) must succeed: %v\noutput=%s", err, out)
	}

	panelPath := filepath.Join(root, ".mindspec", "reviews", "adr-review", panel.FileName)
	data, readErr := os.ReadFile(panelPath)
	if readErr != nil {
		t.Fatalf("expected panel.json at %s: %v", panelPath, readErr)
	}
	var got panel.Panel
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal panel.json: %v", err)
	}
	if got.Gate != "adhoc" {
		t.Errorf("gate = %q, want %q", got.Gate, "adhoc")
	}
	if got.BeadID != nil {
		t.Errorf("bead_id = %v, want nil (an ad-hoc panel has no bead)", got.BeadID)
	}
	if got.Spec != "" {
		t.Errorf("spec = %q, want empty (an ad-hoc panel has no owning spec)", got.Spec)
	}
	if got.ReviewedHeadSHA != bareRef {
		t.Errorf("reviewed_head_sha = %q, want %q", got.ReviewedHeadSHA, bareRef)
	}

	resetPanelCreateFlags(t)
	tallyOut, tallyErr := runPanelCmd("tally", "adr-review")
	if tallyErr != nil && strings.Contains(tallyErr.Error(), "no registered panel found") {
		t.Fatalf("panel tally must reach the ad-hoc panel under .mindspec/reviews/ (R8c tally-reach): %v\noutput=%s", tallyErr, tallyOut)
	}
}

// TestPanelCreate_SpecAdhocCombo_Refused pins AC-15's flag-contract guard
// (R8b): --spec together with --gate adhoc is refused with a final
// recovery line (ADR-0035) naming BOTH valid invocation forms — an ad-hoc
// panel has no owning spec by definition, so a silent ignore would
// misfile it. Nothing is written on the refused path.
//
// The whitespace-only case (FX-1) pins the mutual-exclusion guard against
// a --spec value that TrimSpace's to "": an EXPLICITLY-supplied --spec is
// refused regardless of its content, because the guard keys on
// cmd.Flags().Changed("spec"), not an emptiness probe — otherwise a
// `--spec '   '` would slip the guard and flow an untrimmed-whitespace
// spec into the create path.
func TestPanelCreate_SpecAdhocCombo_Refused(t *testing.T) {
	cases := []struct {
		name string
		spec string
	}{
		{"non-blank spec", "999-nope"},
		{"whitespace-only spec (FX-1)", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetPanelCreateFlags(t)
			root := mkFlatPanelTestRoot(t)
			withTestChdir(t, root)
			config.ResetCache()
			t.Cleanup(config.ResetCache)

			before := snapshotTree(t, root)

			out, err := runPanelCmd("create", "adr-review",
				"--spec", tc.spec,
				"--gate", "adhoc",
				"--target", "cafebabecafebabecafebabecafebabecafebabe")
			if err == nil {
				t.Fatalf("--spec + --gate adhoc must be refused; got success, output=%s", out)
			}
			if !guard.HasFinalRecoveryLine(err.Error()) {
				t.Errorf("refusal must carry a final recovery line (ADR-0035): %q", err.Error())
			}
			if !strings.Contains(err.Error(), "--spec <id> --target <ref>") {
				t.Errorf("refusal must name the gated invocation form: %q", err.Error())
			}
			if !strings.Contains(err.Error(), "--gate adhoc --target <ref>") {
				t.Errorf("refusal must name the ad-hoc invocation form: %q", err.Error())
			}

			after := snapshotTree(t, root)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("a refused create must write nothing:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

// TestPanelCreate_NonAdhocGateWithoutSpec_StillErrors pins AC-15's guard
// leg (R8b): the pre-existing --spec requirement is UNCHANGED — BYTE
// IDENTICAL error message — for every non-adhoc invocation, whether
// --gate is a non-adhoc value or omitted entirely (today's behavior).
func TestPanelCreate_NonAdhocGateWithoutSpec_StillErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"gate omitted", []string{"create", "demo", "--target", "cafebabecafebabecafebabecafebabecafebabe"}},
		{"gate bead", []string{"create", "demo", "--gate", "bead", "--target", "cafebabecafebabecafebabecafebabecafebabe"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetPanelCreateFlags(t)
			root := mkFlatPanelTestRoot(t)
			withTestChdir(t, root)
			config.ResetCache()
			t.Cleanup(config.ResetCache)

			out, err := runPanelCmd(tc.args...)
			if err == nil {
				t.Fatalf("expected \"--spec is required\", got success: %s", out)
			}
			if err.Error() != "--spec is required" {
				t.Errorf("error = %q, want byte-identical guard %q", err.Error(), "--spec is required")
			}
		})
	}
}
