package main

// Spec 119 final-review O2: `mindspec doctor`'s rendered check lines carry
// agent-writable content (spec-dir names, branch names, bead IDs inside
// check Names and Messages). Both are escaped through internal/termsafe AT
// THE RENDER SINK (printDoctorChecks) — the spec 116 safe-set/quote rule —
// so a hostile name can never forge extra display lines or smuggle raw
// ESC/control bytes to the terminal.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
)

func TestPrintDoctorChecks_EscapesHostileNameAndMessage(t *testing.T) {
	hostileName := "stale-open bead: evil\x1b[2J\x1b[1;1H"
	hostileMsg := "run this\nFAKE-LINE: all checks passed \x07"
	var buf bytes.Buffer
	printDoctorChecks(&buf, []doctor.Check{
		{Name: hostileName, Status: doctor.Error, Message: hostileMsg},
	})
	out := buf.String()

	if strings.ContainsAny(out, "\x1b\x07") {
		t.Errorf("rendered doctor output must not contain raw ESC/BEL bytes, got %q", out)
	}
	// The hostile message's forged line must not appear at line start —
	// strconv.Quote keeps the whole value on one quoted line.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "FAKE-LINE:") {
			t.Errorf("hostile message forged a display line: %q", out)
		}
	}
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("status tag must render unescaped (fixed literal), got %q", out)
	}
}

func TestPrintDoctorChecks_PlainASCIIUnchanged(t *testing.T) {
	var buf bytes.Buffer
	printDoctorChecks(&buf, []doctor.Check{
		{Name: "beads config", Status: doctor.OK, Message: "all keys present"},
	})
	if got, want := buf.String(), "beads config: [OK] all keys present\n"; got != want {
		t.Errorf("plain ASCII check line must render byte-identically: got %q, want %q", got, want)
	}
}
