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
			for _, leg := range []panel.DecisionLeg{panel.LegUnresolved, panel.LegStale, ""} {
				name := "nonbead/" + string(leg) + "/" + slug
				if bead {
					name = "bead/" + string(leg) + "/" + slug
				}
				t.Run(name, func(t *testing.T) {
					d := panel.Decision{Action: panel.Block, Leg: leg, Message: "blocked"}
					var err error
					var want []string
					if bead {
						err = tallyExitAction(d, tallyRecoveryFacts{Slug: slug, SpecID: "900-demo", Target: "main", BeadID: "mindspec-test", NextRound: 2, Gate: "bead"})
						want = []string{slug, "--spec", "900-demo", "--gate", "bead", "--target", "main", "--bead", "mindspec-test", "--round", "2", "--brief-file", "/tmp/brief.md"}
					} else {
						err = tallyExitActionNonBead(d, tallyRecoveryFacts{Slug: slug, Target: "main", Gate: "adhoc", NextRound: 2})
						want = []string{slug, "--gate", "adhoc", "--target", "main", "--round", "2", "--brief-file", "/tmp/brief.md"}
					}
					if err == nil {
						t.Fatal("blocked decision must refuse")
					}
					// Every panel create line of the Next block keeps the slug as one argument.
					rest := err.Error()
					found := 0
					for {
						var command string
						var ok bool
						_, rest, ok = strings.Cut(rest, "mindspec panel create ")
						if !ok {
							break
						}
						found++
						command, _, _ = strings.Cut(rest, "\n")
						command = strings.ReplaceAll(command, "<absolute path>", "/tmp/brief.md")
						got, shellErr := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+command).CombinedOutput()
						expected := strings.Join(want, "\x00") + "\x00"
						if shellErr != nil || string(got) != expected {
							t.Fatalf("recovery=%q got=%q want=%q error=%v", command, got, expected, shellErr)
						}
					}
					if found == 0 {
						t.Fatalf("missing panel create command: %v", err)
					}
					// The one recovery line re-runs the tally of the same slug.
					_, command, ok := strings.Cut(err.Error(), "recovery: mindspec panel tally ")
					if !ok {
						t.Fatalf("missing tally recovery: %v", err)
					}
					command, _, _ = strings.Cut(command, "   (")
					got, shellErr := exec.Command("/bin/sh", "-c", `capture() { printf '%s\000' "$@"; }; capture `+command).CombinedOutput()
					if shellErr != nil || string(got) != slug+"\x00" {
						t.Fatalf("tally recovery=%q got=%q error=%v", command, got, shellErr)
					}
				})
			}
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
