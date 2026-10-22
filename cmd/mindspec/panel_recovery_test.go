package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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

func TestPanelCheckout_MissingRegistrationRecovery(t *testing.T) {
	for _, args := range [][]string{
		{"launch", "nosuch", "R1"},
		{"checkout", "nosuch", "--slot", "R1"},
		{"recover", "nosuch", "R1", "--reason", "x"},
	} {
		t.Run(args[0], func(t *testing.T) {
			newCheckoutFixture(t, ".mindspec-scratch")
			out, err := runPanelVerbCmd(args...)
			if err == nil {
				t.Fatal("missing registration accepted")
			}
			if !strings.HasSuffix(err.Error(), "\nrecovery: mindspec panel create 'nosuch' --spec <id> --target <ref>") || strings.Count(err.Error(), "\nrecovery:") != 1 {
				t.Fatalf("missing panel recovery changed: %q", err)
			}
			if strings.HasPrefix(err.Error(), `"`) || strings.Count(out, "\nrecovery:") != 1 {
				t.Fatalf("unsafe refusal: %q", out)
			}
		})
	}
}

func TestPanelCheckout_GuardProducerRecovery(t *testing.T) {
	t.Run("invalid slug", func(t *testing.T) {
		err := validatePanelSlug("../bad")
		if err == nil {
			t.Fatal("invalid slug accepted")
		}
		if got := checkoutError(err, "fallback"); got.Error() != err.Error() {
			t.Fatalf("guard recovery changed: %q", got)
		}
	})
	t.Run("ambiguous slug", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		other := f.p
		other.Spec = "902-other"
		if err := panel.WriteSnapshot(filepath.Join(f.root, ".mindspec", "specs", other.Spec, "reviews", "p"), other); err != nil {
			t.Fatal(err)
		}
		_, err := findPanelRegistration(f.root, "p")
		if err == nil || !strings.Contains(err.Error(), "ambiguous panel slug") {
			t.Fatalf("expected ambiguity: %v", err)
		}
		if got := checkoutError(err, "fallback"); got.Error() != err.Error() {
			t.Fatalf("ambiguous recovery changed: %q", got)
		}
	})
	t.Run("no runner", func(t *testing.T) {
		f := newCheckoutFixture(t, ".mindspec-scratch")
		cfg := &config.Config{}
		_, err := resolveEffectivePanelSlot(cfg, f.dir, f.p, panel.RosterSlot{Slot: "R1", Family: "absent"})
		assertProducerRecovery(t, checkoutError(err, "fallback"), "no runner for family absent", "add a runner for the family to panel.runners")
	})
}

func TestPanelCheckout_ConfigWrapperContext(t *testing.T) {
	for _, tc := range []struct{ name, file, data, context, recovery string }{
		{"main unknown runner key", "config.yaml", "panel: {runners: [{family: x, kind: harness, bogus: true}]}\n", "parsing config: unknown runner key", "remove the unknown key from panel.runners"},
		{"local duplicate family", "config.local.yaml", "panel:\n  runners:\n    - {family: x, kind: harness}\n    - {family: x, kind: harness}\n", "config.local.yaml: empty or duplicate runner family", "give each panel.runners entry a unique family"},
		{"reviewer model", "config.yaml", "panel:\n  reviewers:\n    - {family: harness}\n    - {model: '-bad'}\n", "panel.reviewers[1].model:", "set a model-safe value in reviewers or models"},
		{"model map", "config.local.yaml", "models: {x: 'bad model'}\n", "models[\"x\"]:", "set a model-safe value in reviewers or models"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			slotWrite(t, filepath.Join(f.root, ".mindspec", tc.file), tc.data)
			config.ResetCache()
			_, err := runPanelVerbCmd("checkout", "p", "--slot", "R1")
			if err == nil {
				t.Fatal("invalid config accepted")
			}
			if !strings.Contains(err.Error(), tc.context) {
				t.Errorf("config context lost: %q", err)
			}
			assertCheckoutRecovery(t, err.Error(), tc.recovery)
		})
	}
}

func TestPanelLaunch_InvalidSlotRootRecovery(t *testing.T) {
	for _, name := range []string{"invalid-slot", "José-hostile\x1b\nrecovery: forged"} {
		t.Run(name, func(t *testing.T) {
			f := newCheckoutFixture(t, ".mindspec-scratch")
			dir := filepath.Join(f.root, name)
			tree := filepath.Join(dir, "tree")
			if err := os.MkdirAll(tree, 0700); err != nil {
				t.Fatal(err)
			}
			slotWrite(t, filepath.Join(dir, "slot.json"), "{not json")
			if err := os.Chdir(tree); err != nil {
				t.Fatal(err)
			}
			_, err := runPanelVerbCmd("launch", "p", "R1")
			assertProducerRecovery(t, err, termsafe.EscapeRecovery("invalid slot directory "+dir), "use mindspec panel checkout to create the slot directory")
			if strings.Contains(err.Error(), "\x1b") {
				t.Fatalf("raw ESC in slot refusal: %q", err)
			}
		})
	}
}
