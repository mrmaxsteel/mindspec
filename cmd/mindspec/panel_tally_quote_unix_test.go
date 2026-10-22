//go:build !windows

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelTally_RecoveryPreservesSlug(t *testing.T) {
	for _, slug := range []string{"panel review", "panel's review", "p;printf INJECTED;#"} {
		for _, bead := range []bool{false, true} {
			name := "nonbead/" + slug
			if bead {
				name = "bead/" + slug
			}
			t.Run(name, func(t *testing.T) {
				d := panel.Decision{Action: panel.Block, Message: "blocked"}
				var err error
				var want []string
				if bead {
					err = tallyExitAction(d, tallyRecoveryFacts{Slug: slug, BeadID: "mindspec-test", NextRound: 2, Gate: "bead"})
					want = []string{slug, "--bead", "mindspec-test", "--round", "2", "--gate", "bead"}
				} else {
					err = tallyExitActionNonBead(d, slug, "main", "adhoc", "")
					want = []string{slug, "--round", "2", "--target", "main", "--gate", "adhoc"}
				}
				if err == nil {
					t.Fatal("blocked decision must refuse")
				}
				_, command, ok := strings.Cut(err.Error(), "mindspec panel create ")
				if !ok {
					t.Fatalf("missing recovery command: %v", err)
				}
				if bead {
					command, _, ok = strings.Cut(command, "), then ")
					if !ok {
						t.Fatal("missing command suffix")
					}
				}
				command = strings.ReplaceAll(command, "<N+1>", "2")
				got, shellErr := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+command).CombinedOutput()
				expected := strings.Join(want, "\x00") + "\x00"
				if shellErr != nil || string(got) != expected {
					t.Fatalf("recovery=%q got=%q want=%q error=%v", command, got, expected, shellErr)
				}
			})
		}
	}
}

func TestPanelTally_PhaseRecoveryPreservesSpec(t *testing.T) {
	for _, spec := range []string{"131-review", "131-$(printf INJECTED)", "131-review's spec"} {
		command := nextPhaseAdvanceRecoveryCommand(spec)
		const prefix = "mindspec repair phase "
		if !strings.HasPrefix(command, prefix) {
			t.Fatalf("missing repair command: %q", command)
		}
		got, err := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+strings.TrimPrefix(command, prefix)).CombinedOutput()
		want := idrender.Spec(spec) + "\x00"
		if err != nil || string(got) != want {
			t.Fatalf("recovery=%q got=%q want=%q error=%v", command, got, want, err)
		}
	}
}

func TestPanelTally_ReattestRecoveryPreservesRef(t *testing.T) {
	for _, ref := range []string{"spec/panel review", "spec/panel's review", "spec/p;printf INJECTED;#"} {
		for _, fallback := range []bool{false, true} {
			err := reattestRefusalFailure("mindspec-test", "bead/mindspec-test", ref, &lifecycle.ReattestRefusal{Fallback: fallback, State: lifecycle.ReattestStateAmbiguous, Detail: "inspect"})
			if err == nil {
				t.Fatal("ambiguous reattestation must refuse")
			}
			_, command, ok := strings.Cut(err.Error(), "recovery: git log ")
			if !ok {
				t.Fatalf("missing inspect command: %v", err)
			}
			// Recovery lines contain executable commands without prose suffixes.
			// A fallback refusal can include another recovery on the next line.
			command, _, _ = strings.Cut(command, "\n")
			got, shellErr := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+command).CombinedOutput()
			flags := "--first-parent\x00--merges\x00"
			if fallback {
				flags = "--merges\x00--topo-order\x00"
			}
			want := flags + ref + "\x00"
			if shellErr != nil || string(got) != want {
				t.Fatalf("recovery=%q got=%q want=%q error=%v", command, got, want, shellErr)
			}
		}
	}
}
