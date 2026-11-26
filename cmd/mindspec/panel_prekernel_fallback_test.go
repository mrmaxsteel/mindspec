package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/spf13/cobra"
)

// A panel of a spec that is not enrolled reads its slot verdicts from disk
// when the tracker cannot be read. Each reader of the slot records keeps
// that fallback, and verify, submit and launch read the tracker once for
// their slot status. Submit, defer and refute also read it once before the
// write, to find that the spec is not enrolled, so that read succeeds.
func TestS134PanelReaders_PreKernelTrackerFailure(t *testing.T) {
	f, head := panelPreKernelFixture(t)
	old := panelKernelStateFn
	t.Cleanup(func() { panelKernelStateFn = old })
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { return nil, nil }
	out, err := kernelPanelRun(t, panelCreateCmd, "legacy", "--spec", f.SpecID, "--gate", "final_review", "--target", "spec/"+f.SpecID)
	kernelPanelRequire(t, out, err)
	dir := filepath.Join(f.SpecDir, "reviews", "legacy")
	verdict := func(slot string) string {
		v := paneltest.Verdict(panel.VerdictApprove)
		if slot == "R1" {
			v.Verdict = panel.VerdictRequestChanges
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "missing test", Basis: "traced"}, {Text: "missing doc", Basis: "traced"}}
		}
		v.Slot, v.Round, v.Model, v.ReviewedSHA = slot, 1, "test-model", head
		file := filepath.Join(dir, slot+"-round-1.json")
		f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0o644))
		return file
	}
	out, err = kernelPanelRun(t, panelSubmitCmd, "legacy", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--file", verdict("R1"))
	kernelPanelRequire(t, out, err)
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "epics.json", fmt.Sprintf(`[{"id":%q,"issue_type":"epic","metadata":{"spec_id":%q}}]`, f.Epic, f.SpecID))
	rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
	reg, err := findPanelRegistration(f.Root, "legacy", f.SpecID)
	f.Must(err)
	launch := func(_ *testing.T) (string, error) {
		cmd := &cobra.Command{}
		var buf strings.Builder
		cmd.SetOut(&buf)
		err := launchStatus(cmd, f.Root, reg, "R1")
		return buf.String(), err
	}
	for _, c := range []struct {
		name string
		once bool
		want string
		run  func(*testing.T) (string, error)
	}{
		{"submit", true, "R2 final", func(t *testing.T) (string, error) {
			return kernelPanelRun(t, panelSubmitCmd, "legacy", "--spec", f.SpecID, "--slot", "R2", "--round", "1", "--file", verdict("R2"))
		}},
		{"verify", true, "R1", func(t *testing.T) (string, error) {
			return kernelPanelRun(t, panelVerifyCmd, "legacy", "--spec", f.SpecID)
		}},
		{"launch status", true, "R1 final", launch},
		{"defer", false, "", func(t *testing.T) (string, error) {
			return kernelPanelRun(t, panelDeferCmd, "legacy", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--finding", "ccr[0]", "--bead", "follow-up", "--by", "operator")
		}},
		{"refute", false, "", func(t *testing.T) (string, error) {
			return kernelPanelRun(t, panelRefuteCmd, "legacy", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--finding", "ccr[1]", "--ground", "disproven", "--reason", "covered", "--evidence", "proof.txt", "--by", "operator")
		}},
	} {
		calls := 0
		ok := 0
		if c.name == "submit" || c.name == "defer" || c.name == "refute" {
			ok = 1
		}
		panelKernelStateFn = func(_, _ string) (*kernel.State, error) {
			calls++
			if calls <= ok {
				return nil, nil
			}
			return nil, errors.New("tracker unavailable")
		}
		out, err := c.run(t)
		text := out
		if err != nil {
			text += err.Error()
		}
		if strings.Contains(text, "tracker unavailable") || !strings.Contains(out, c.want) {
			t.Errorf("%s lost the disk fallback: out=%s err=%v", c.name, out, err)
		}
		if c.name != "verify" && err != nil {
			t.Errorf("%s refused: out=%s err=%v", c.name, out, err)
		}
		if c.once && calls-ok != 1 {
			t.Errorf("%s read the tracker %d times", c.name, calls)
		}
	}
}
