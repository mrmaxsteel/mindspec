package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

func setupCreateIntegrity(t *testing.T) string {
	t.Helper()
	resetPanelCreateFlags(t)
	root := mkPanelTestRoot(t, "")
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	root = resolved
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	stubNonBeadRevParse(t, func(string, string) (string, error) { return "abc123", nil })
	return root
}

// Include bytes as well as paths: refusing after rewriting panel.json is a bug.
func panelTreeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, rel := range snapshotTree(t, root) {
		path := filepath.Join(root, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			out[rel] = string(b)
		} else {
			out[rel] = "<directory>"
		}
	}
	return out
}

func assertPanelRefusal(t *testing.T, root string, args ...string) string {
	t.Helper()
	before := panelTreeBytes(t, root)
	out, err := runPanelVerbCmd(args...)
	if err == nil {
		t.Fatalf("expected refusal for %q, got success: %s", args, out)
	}
	if !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("missing recovery: %v", err)
	}
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal wrote files:\nbefore=%v\nafter=%v", before, after)
	}
	return err.Error()
}

func TestPanelCreate_RefusesSameRoundWithVerdicts(t *testing.T) {
	for _, gate := range []string{"bead", "final_review", "adhoc", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			args := []string{"create", "demo", "--gate", gate, "--target", "bead/probe-x.1"}
			if gate != "adhoc" {
				args = append(args, "--spec", "900-demo")
			}
			if out, err := runPanelVerbCmd(args...); err != nil {
				t.Fatalf("seed: %v %s", err, out)
			}
			dir, _, err := panelDirFor(root, "900-demo", "demo", gate)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(`{"verdict":"APPROVE"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			msg := assertPanelRefusal(t, root, args...)
			if !strings.Contains(msg, "--round 2") || !strings.Contains(msg, "mindspec panel rebind") {
				t.Fatalf("wrong recovery: %s", msg)
			}
			if gate == "adhoc" && strings.Contains(msg, "--spec") {
				t.Fatalf("adhoc recovery has --spec: %s", msg)
			}
		})
	}
}

func TestPanelCreate_RefusesOlderRound(t *testing.T) {
	root := setupCreateIntegrity(t)
	dir := writePanelFixture(t, root, "demo", panel.Panel{Spec: "900-demo", Round: 1})
	// Filenames, rather than recorded round or JSON validity, determine LatestRound.
	if err := os.WriteFile(filepath.Join(dir, "R1-round-3.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg := assertPanelRefusal(t, root, "create", "demo", "--spec", "900-demo", "--target", "main", "--round", "2")
	if !strings.Contains(msg, "--round 4") {
		t.Fatalf("wrong next round: %s", msg)
	}
}

func TestPanelCreate_SameRoundNoVerdictsAllowed(t *testing.T) {
	root := setupCreateIntegrity(t)
	args := []string{"create", "demo", "--spec", "900-demo", "--target", "main", "--round", "2"}
	for i := 0; i < 2; i++ {
		if out, err := runPanelVerbCmd(args...); err != nil {
			t.Fatalf("create %d: %v %s", i, err, out)
		}
	}
	dir := filepath.Join(root, "review", "demo")
	// Prior-round verdicts and markdown archives do not prohibit this round.
	for _, name := range []string{"R1-round-1.json", "BRIEF-round-2.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := runPanelVerbCmd(args...); err != nil {
		t.Fatalf("recreate empty round 2: %v %s", err, out)
	}
}

func TestPanelCreate_BeadIDDerivedFromTarget(t *testing.T) {
	root := setupCreateIntegrity(t)
	if out, err := runPanelVerbCmd("create", "demo", "--spec", "900-demo", "--gate", "bead", "--target", "bead/probe-x.1"); err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	regs := panel.ForBead(panel.Scan(root), "probe-x.1")
	if len(regs) != 1 || regs[0].Panel.Target != "bead/probe-x.1" {
		t.Fatalf("derived panel missing: %+v", regs)
	}
	if len(regs[0].Panel.Roster) != 4 || regs[0].Panel.ExpectedReviewers != 4 {
		t.Fatalf("bead needs 4 slots: %+v", regs[0].Panel)
	}

}

func TestPanelCreate_BeadTargetRefusals(t *testing.T) {
	for _, tc := range []struct{ name, gate, target, bead string }{
		{"main", "bead", "main", ""},
		{"empty id", "bead", "bead/", ""},
		{"invalid id", "bead", "bead/probe/x", ""},
		{"option id", "bead", "bead/-probe", ""},
		{"mismatch", "bead", "bead/probe-x.1", "probe-y.1"},
		{"spec gate", "spec_approve", "bead/probe-x.1", "probe-x.1"},
		{"plan gate", "plan_approve", "bead/probe-x.1", "probe-x.1"},
		{"final gate", "final_review", "bead/probe-x.1", "probe-x.1"},
		{"adhoc gate", "adhoc", "bead/probe-x.1", "probe-x.1"},
		{"omitted gate", "", "bead/probe-x.1", "probe-x.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			args := []string{"create", "demo", "--target", tc.target}
			if tc.gate != "adhoc" {
				args = append(args, "--spec", "900-demo")
			}
			if tc.gate != "" {
				args = append(args, "--gate", tc.gate)
			}
			if tc.bead != "" {
				args = append(args, "--bead", tc.bead)
			}
			msg := assertPanelRefusal(t, root, args...)
			recovery := msg[strings.LastIndex(msg, "recovery:"):]
			if tc.gate != "" && tc.gate != "bead" {
				if strings.Contains(recovery, "--bead") || strings.Contains(recovery, "--gate bead") ||
					!strings.Contains(recovery, "--target '"+tc.target+"'") || !strings.Contains(recovery, "--round 1") {
					t.Fatalf("non-bead recovery changed the requested panel: %s", recovery)
				}
				if tc.gate == "adhoc" {
					if strings.Contains(recovery, "--spec") || !strings.Contains(recovery, "--gate adhoc") {
						t.Fatalf("adhoc recovery: %s", recovery)
					}
				} else if !strings.Contains(recovery, "--gate '"+tc.gate+"'") || !strings.Contains(recovery, "--spec '900-demo'") {
					t.Fatalf("recovery lost gate/spec: %s", recovery)
				}
			} else if !strings.Contains(recovery, "--gate bead --spec <id> --target bead/<id>") {
				t.Fatalf("bead-target recovery: %s", recovery)
			}
		})
	}
}

func writeRegistrationAt(t *testing.T, checkout, spec, slug string) string {
	t.Helper()
	dir := filepath.Join(checkout, ".mindspec", "specs", spec, "reviews", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(panel.Panel{Spec: spec, Target: "main", Gate: "final_review", Round: 3, ExpectedReviewers: 2, ReviewedHeadSHA: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, panel.FileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindPanelRegistration_AmbiguousRefuses(t *testing.T) {
	root := setupCreateIntegrity(t)
	a := writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-900"), "900-demo", "b1")
	b := writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-901"), "901-demo", "b1")
	_, err := findPanelRegistration(root, "b1")
	if err == nil || !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) || !strings.Contains(err.Error(), "--spec") {
		t.Fatalf("ambiguity must list both matches: %v", err)
	}
	reg, err := findPanelRegistration(root, "b1", "901-demo")
	if err != nil || reg.Dir != b {
		t.Fatalf("--spec selected %+v: %v", reg, err)
	}
	assertPanelRefusal(t, root, "tally", "b1")
}

func TestFindPanelRegistration_CheckoutCopyWins(t *testing.T) {
	root := setupCreateIntegrity(t)
	local := writeRegistrationAt(t, root, "900-demo", "b1")
	writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-900"), "900-demo", "b1")
	for _, spec := range []string{"", "900-demo"} {
		reg, err := findPanelRegistration(root, "b1", spec)
		if err != nil || reg.Dir != local {
			t.Fatalf("checkout must win: %+v %v", reg, err)
		}
	}
}

func TestFindPanelRegistration_SameSpecWorktreeCopiesRefuse(t *testing.T) {
	root := setupCreateIntegrity(t)
	writeRegistrationAt(t, filepath.Join(root, ".worktrees", "a"), "900-demo", "b1")
	writeRegistrationAt(t, filepath.Join(root, ".worktrees", "b"), "900-demo", "b1")
	if _, err := findPanelRegistration(root, "b1", "900-demo"); err == nil {
		t.Fatal("--spec must not pick arbitrarily among same-spec worktrees")
	}
}

func TestFindPanelRegistration_PrintsResolvedDir(t *testing.T) {
	root := setupCreateIntegrity(t)
	writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-900"), "900-demo", "b1")
	dir := writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-901"), "901-demo", "b1")
	for _, cmd := range []*cobra.Command{panelTallyCmd, panelVerifyCmd} {
		flag := cmd.Flags().Lookup("spec")
		t.Cleanup(func() { _ = flag.Value.Set(flag.DefValue); flag.Changed = false })
		out, err := runPanelVerbCmd(cmd.Name(), "b1", "--spec", "901-demo")
		if cmd == panelVerifyCmd && err != nil {
			t.Fatalf("verify: %v", err)
		}
		if cmd == panelTallyCmd && err == nil {
			t.Fatal("incomplete tally must block")
		}
		if !strings.Contains(out, "panel directory: "+dir) || !strings.Contains(out, "resolved round: 3") {
			t.Fatalf("resolved registration absent: %s", out)
		}
	}
}

func TestPanelCreate_SlugTakenByOtherSpec(t *testing.T) {
	root := setupCreateIntegrity(t)
	writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-900"), "900-demo", "b1")
	assertPanelRefusal(t, root, "create", "b1", "--spec", "902-demo", "--target", "main")
}

func TestPanelCreate_AdhocSlugCollision(t *testing.T) {
	for _, existingAdhoc := range []bool{false, true} {
		t.Run(map[bool]string{false: "spec owns", true: "adhoc owns"}[existingAdhoc], func(t *testing.T) {
			root := setupCreateIntegrity(t)
			writeRegistrationAt(t, root, "900-demo", "other") // flat layout
			if existingAdhoc {
				if out, err := runPanelVerbCmd("create", "b1", "--gate", "adhoc", "--target", "main"); err != nil {
					t.Fatalf("seed: %v %s", err, out)
				}
				resetPanelCreateFlags(t)
				assertPanelRefusal(t, root, "create", "b1", "--spec", "900-demo", "--target", "main")
			} else {
				writeRegistrationAt(t, root, "900-demo", "b1")
				msg := assertPanelRefusal(t, root, "create", "b1", "--gate", "adhoc", "--target", "main")
				if strings.Contains(msg[strings.LastIndex(msg, "recovery:"):], "--spec") {
					t.Fatalf("adhoc recovery: %s", msg)
				}
			}
		})
	}
}

func TestTallyExitActionNonBead_AdhocNoSpec(t *testing.T) {
	err := tallyExitActionNonBead(panel.Decision{Action: panel.Block, Message: "blocked"}, "demo", "main", "adhoc", "")
	if err == nil || strings.Contains(err.Error(), "--spec") || !strings.Contains(err.Error(), "--gate 'adhoc'") {
		t.Fatalf("wrong adhoc recovery: %v", err)
	}
	// The create error adapter also preserves the spec-less round recovery.
	err = panelCreateFailure(&panel.CreateRoundConflict{Round: 1, LatestRound: 1}, "demo", panel.CreateInput{Gate: "adhoc", Target: "main", Round: 1})
	if strings.Contains(err.Error(), "--spec") || !strings.Contains(err.Error(), "--round 2") || !strings.Contains(err.Error(), "panel rebind") {
		t.Fatalf("wrong adhoc round recovery: %v", err)
	}
}

func TestTallyExitActionNonBead_RecordedSpec(t *testing.T) {
	err := tallyExitActionNonBead(panel.Decision{Action: panel.Block, Message: "blocked"}, "demo", "main", "final_review", "901-demo")
	if err == nil || !strings.Contains(err.Error(), "--spec '901-demo'") || strings.Contains(err.Error(), "<id>") {
		t.Fatalf("wrong spec recovery: %v", err)
	}
}

func TestPanelSubcommands_SilenceUsage(t *testing.T) {
	var check func(*cobra.Command)
	check = func(cmd *cobra.Command) {
		if !cmd.SilenceUsage {
			t.Errorf("%s lacks SilenceUsage", cmd.CommandPath())
		}
		for _, child := range cmd.Commands() {
			check(child)
		}
	}
	check(panelCmd)
	root := setupCreateIntegrity(t)
	writePanelFixture(t, root, "demo", panel.Panel{Spec: "900-demo", Target: "main", Gate: "final_review", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: "abc123"})
	// Ensure the executed child is what suppresses Cobra usage.
	old := rootCmd.SilenceUsage
	rootCmd.SilenceUsage = false
	t.Cleanup(func() { rootCmd.SilenceUsage = old })
	out, err := runPanelVerbCmd("tally", "demo")
	if err == nil {
		t.Fatalf("expected incomplete Block: %s", out)
	}
	if strings.Contains(out, "Usage:") {
		t.Fatalf("Block printed usage: %s", out)
	}
}

func TestPanelCreate_AdhocRepanel(t *testing.T) {
	root := setupCreateIntegrity(t)
	args := []string{"create", "demo", "--gate", "adhoc", "--target", "main"}
	if out, err := runPanelVerbCmd(args...); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}
	// A round without verdict files may also be recreated.
	if out, err := runPanelVerbCmd(args...); err != nil {
		t.Fatalf("recreate empty round: %v %s", err, out)
	}
	dir, _, err := panelDirFor(root, "", "demo", "adhoc")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(`{"verdict":"REJECT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runPanelVerbCmd(append(args, "--round", "2")...); err != nil {
		t.Fatalf("re-panel: %v %s", err, out)
	}
	reg, err := findPanelRegistration(root, "demo")
	if err != nil || reg.Panel.Round != 2 || reg.Panel.Gate != "adhoc" || reg.Panel.Spec != "" {
		t.Fatalf("wrong re-panel registration: %+v %v", reg, err)
	}
}

func TestPanelCreate_AdhocOwnDestinationSymlink(t *testing.T) {
	root := setupCreateIntegrity(t)
	dir := writePanelFixture(t, root, "demo", panel.Panel{Gate: "adhoc", Round: 1})
	alias := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := checkPanelSlugOwner(root, "demo", "", "adhoc", filepath.Join(alias, "review", "demo")); err != nil {
		t.Fatalf("own canonical destination %s refused: %v", dir, err)
	}
}

func TestPanelTally_AdhocBlockRecoveryRegistersNextRound(t *testing.T) {
	root := setupCreateIntegrity(t)
	if out, err := runPanelVerbCmd("create", "demo", "--gate", "adhoc", "--target", "main"); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}
	dir, _, err := panelDirFor(root, "", "demo", "adhoc")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "R1-round-1.json"), []byte(`{"verdict":"REJECT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runPanelVerbCmd("tally", "demo")
	if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("expected Block recovery: %v %s", err, out)
	}
	recovery := err.Error()
	start := strings.LastIndex(recovery, "mindspec panel create")
	if start < 0 || !strings.Contains(recovery[start:], "<N+1>") || strings.Contains(recovery[start:], "--spec") {
		t.Fatalf("wrong adhoc recovery: %s", recovery)
	}
	command := strings.ReplaceAll(recovery[start:], "<N+1>", "2")
	tokens := splitPosixSingleQuoted(t, command)
	resetPanelCreateFlags(t)
	if out, err := runPanelVerbCmd(tokens[2:]...); err != nil {
		t.Fatalf("printed recovery failed: %v %s", err, out)
	}
	reg, err := findPanelRegistration(root, "demo")
	if err != nil || reg.Panel.Round != 2 || reg.Panel.Gate != "adhoc" || reg.Panel.Spec != "" {
		t.Fatalf("recovery did not register round 2: %+v %v", reg, err)
	}
}

func TestFindPanelRegistration_CheckoutDifferentSpecRefuses(t *testing.T) {
	root := setupCreateIntegrity(t)
	local := writeRegistrationAt(t, root, "900-demo", "b1")
	other := writeRegistrationAt(t, filepath.Join(root, ".worktrees", "worktree-spec-901"), "901-demo", "b1")
	_, err := findPanelRegistration(root, "b1")
	if err == nil || !strings.Contains(err.Error(), "spec 900-demo: "+local) || !strings.Contains(err.Error(), "spec 901-demo: "+other) {
		t.Fatalf("cross-spec ambiguity must list both matches: %v", err)
	}
}

func TestPanelCreate_AdhocCollisionWithMatchingSpec(t *testing.T) {
	for _, gate := range []string{"adhoc", "final_review"} {
		t.Run(gate, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			existingGate := "adhoc"
			if gate == "adhoc" {
				existingGate = "final_review"
			}
			// Match the spec even on an imported adhoc registration so the
			// gate collision cannot be masked by the cross-spec check.
			dir := writePanelFixture(t, root, "demo", panel.Panel{Spec: "900-demo", Gate: existingGate, Round: 1})
			before := panelTreeBytes(t, root)
			if err := checkPanelSlugOwner(root, "demo", "900-demo", gate, dir); err == nil {
				t.Fatal("matching spec must not bypass an adhoc/spec collision")
			}
			assertPanelRefusal(t, root, "create", "demo", "--gate", gate, "--spec", "900-demo", "--target", "main")
			if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("collision changed the panel tree")
			}
		})
	}
}

func TestPanelVerify_AdhocMissingRefRecoveryNoSpec(t *testing.T) {
	for _, verb := range []string{"verify", "tally"} {
		t.Run(verb, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			writePanelFixture(t, root, "demo", panel.Panel{Gate: "adhoc", Target: "deleted", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: "abc123"})
			stubNonBeadRevParse(t, func(string, string) (string, error) { return "", gitutil.ErrRefNotFound })
			out, err := runPanelVerbCmd(verb, "demo")
			if err != nil || !strings.Contains(out, "no longer exists") ||
				!strings.Contains(out, "mindspec panel create demo --gate adhoc --target 'deleted'") || strings.Contains(out, "--spec") {
				t.Fatalf("missing-ref adhoc advisory: %v %s", err, out)
			}
		})
	}
}
