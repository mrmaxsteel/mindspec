package complete

import (
	"fmt"
	"io"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// beadPanelPresence keeps one warning across the unmerged-to-merged handoff.
// The absence audit follows a successful terminal mutation, like other bead
// panel audits. Reconciliation of a missing branch retains its prior behavior.
type beadPanelPresence struct {
	policy, specID string
	closed         bool
	absent         bool
	ignore         bool
}

func (p *beadPanelPresence) check(beadID string, enabled bool, out io.Writer) error {
	if p.ignore || !enabled || panelSkipEnvFn() {
		return nil
	}
	specID := "<spec-id>"
	if p.specID != "" {
		specID = idrender.Spec(p.specID)
	}
	recovery := fmt.Sprintf("mindspec panel create <slug> --spec %s --gate bead --target bead/%s", specID, idrender.Bead(beadID))
	const message = "gate bead panel <slug>: no registration"
	if p.policy == "enforce" {
		if p.closed {
			return guard.NewFailure(message, "run a panel for the bead at its tip", fmt.Sprintf("mindspec complete %s", idrender.Bead(beadID)))
		}
		return guard.NewFailure(message, recovery)
	}
	if !p.absent && out != nil {
		fmt.Fprintf(out, "warning: %s; recovery: %s\n", message, recovery)
	}
	p.absent = true
	return nil
}

func (p *beadPanelPresence) audit(beadID string, out io.Writer) {
	if !p.absent {
		return
	}
	meta := map[string]interface{}{
		"panel_absent":      true,
		"panel_absent_gate": "bead",
		"panel_absent_at":   time.Now().UTC().Format(time.RFC3339),
	}
	if err := completeMergeMetadataFn(beadID, meta); err != nil && out != nil {
		fmt.Fprintf(out, "warning: could not record panel_absent on %s: %s\n", idrender.Bead(beadID), termsafe.Escape(err.Error()))
	}
}

// panelGate retains the check-only API. Completion supplies its resolved spec
// and policy directly to panelGateWithPresence and keeps the audit outcome.
func panelGate(beadID string, roots []string, wtPath string, enabled bool, out io.Writer) ([]PanelGateResult, error) {
	p := &beadPanelPresence{policy: config.DefaultConfig().Enforcement.PanelPresence}
	if len(roots) > 0 {
		root := roots[0]
		if found, err := workspace.FindRoot(root); err == nil {
			root = found
		}
		cfg, err := config.Load(root)
		if err != nil {
			return nil, err
		}
		p.policy = cfg.Enforcement.PanelPresence
	}
	return panelGateWithPresence(beadID, roots, wtPath, enabled, out, p)
}
