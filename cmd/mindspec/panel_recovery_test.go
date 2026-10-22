package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
)

func assertProducerRecovery(t *testing.T, err error, message, recovery string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected refusal")
	}
	want := message + "\nrecovery: " + recovery
	if got := err.Error(); got != want || strings.Count(got, "\nrecovery:") != 1 {
		t.Fatalf("producer recovery changed: got %q want %q", got, want)
	}
}

func TestPanelSubmit_ProducerRecoveryThroughCheckoutError(t *testing.T) {
	f := newCLIV2(t)
	_, err := panel.Submit(f.target("R1"), paneltest.Bytes(t, f.verdict()), func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
		return panel.SubmitResolution{Family: "harness", Model: "m", RunnerKind: "harness", Landed: true}, nil
	})
	assertProducerRecovery(t, checkoutError(err, "generic recovery"), "panel round has landed", "mindspec panel create 'p' --round 2")
}

func TestPanelCheckout_ConfigProducerRecovery(t *testing.T) {
	for _, tc := range []struct{ name, file, data, message, recovery string }{
		{"local", "config.local.yaml", "{}\n---\n{}\n", "config.local.yaml: expected one YAML document", "set only panel.runners and models in .mindspec/config.local.yaml"},
		{"retired", "config.yaml", "panel: {substitution: {}}\n", "panel.substitution is retired", "set on_quota on the runner in panel.runners"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			slotWrite(t, filepath.Join(f.root, ".mindspec", tc.file), tc.data)
			config.ResetCache()
			_, producer := config.Load(f.root)
			if producer == nil {
				t.Fatal("expected config refusal")
			}
			out, err := runPanelVerbCmd("checkout", "p", "--slot", "R1")
			assertProducerRecovery(t, err, tc.message, tc.recovery)
			if strings.Count(out, "\nrecovery:") != 1 || !strings.Contains(out, err.Error()) {
				t.Fatalf("Cobra changed recovery: %q", out)
			}
		})
	}
}

func TestPanelCheckout_UnicodeRecoveryLiteralArguments(t *testing.T) {
	slug := "réview$(printf INJECTED)"
	if err := validatePanelSlug(slug); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{slug, "/Users/José/" + slug} {
		recovery := checkoutPruneRecovery(target, "R1")
		refusal := checkoutFailure("tree has another SHA", recovery)
		_, command, ok := strings.Cut(refusal.Error(), "\nrecovery: ")
		if !ok {
			t.Fatal(refusal)
		}
		out, err := exec.Command("sh", "-c", "set -- "+command+`; printf '%s\n' "$@"`).CombinedOutput()
		want := "mindspec\npanel\ncheckout\n" + target + "\n--prune\n--slot\nR1\n"
		if err != nil || string(out) != want {
			t.Fatalf("recovery changed literal arguments: recovery=%q output=%q want=%q error=%v", command, out, want, err)
		}
	}
}

func TestPanelCheckout_WindowsSupportedProducerRecovery(t *testing.T) {
	err := checkoutError(panellaunch.SupportedOn("windows"), "generic recovery")
	assertProducerRecovery(t, err, "process runners need a Unix host", "use a harness runner")
	if err := panellaunch.SupportedOn("linux"); err != nil {
		t.Fatalf("Unix process runners refused: %v", err)
	}
}
