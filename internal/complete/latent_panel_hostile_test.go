package complete

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

// TestLatentPanelSinksHostileFieldsEscaped pins AC-17: the production-dead
// panelAdvisory (this package) and panel.VoteDecision (internal/panel,
// exercised here via panelAdvisory's call chain) escape a hostile reviewer
// Slot (derived from a verdict filename) before it reaches the advisory
// line, with clean-fixture byte-identity preserved.
func TestLatentPanelSinksHostileFieldsEscaped(t *testing.T) {
	root := t.TempDir()
	hostileSlot := "evil\x1b[31mFAKE"
	writePanel(t, root, "093-bd01", panel.Panel{
		BeadID: bp("mindspec-bd01"), Spec: "093", Round: 1, ExpectedReviewers: 3,
	}, map[string]string{
		"a-round-1.json":              "APPROVE",
		"b-round-1.json":              "APPROVE",
		hostileSlot + "-round-1.json": "REQUEST_CHANGES",
	})
	var buf bytes.Buffer
	panelAdvisory("mindspec-bd01", []string{root}, &buf)
	out := buf.String()
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("advisory output contains a raw ESC control byte:\n%q", out)
	}
	if !strings.Contains(out, "would BLOCK") {
		t.Errorf("advisory should say would-BLOCK on an unresolved dissent: %q", out)
	}

	// Clean-fixture byte-identity: TestPanelAdvisory_Dissent_WouldBlock
	// (same package) already pins a clean slot's unresolved-verdict line
	// renders byte-identical; not duplicated here.
}
