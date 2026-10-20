package complete

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// PanelCore3Failure gives both lifecycle callers the same actionable refusal.
func PanelCore3Failure(reg panel.Registration, err error) error {
	slots := make([]string, 0, len(reg.Panel.Roster))
	for _, slot := range reg.Panel.Roster {
		slots = append(slots, termsafe.Escape(slot.Slot))
	}
	scope := " --spec " + core3Quote(reg.Panel.Spec)
	if reg.Panel.Gate == "adhoc" {
		scope = ""
	}
	recovery := fmt.Sprintf("mindspec panel tally %s%s", core3Quote(reg.Slug()), scope)

	var uncovered *panel.UncoveredSlotError
	if errors.As(err, &uncovered) {
		recovery = fmt.Sprintf("mindspec panel refute %s%s --slot %s --round %d --reason '<why>' --ground disproven --evidence '<evidence>' --by '<who>'", core3Quote(reg.Slug()), scope, core3Quote(uncovered.Slot), reg.Panel.Round)
	}

	return guard.NewFailure(fmt.Sprintf("panel %s slots [%s]: coverage check failed: %s", termsafe.Escape(reg.Slug()), strings.Join(slots, ", "), termsafe.Escape(err.Error())), recovery)
}

func core3Quote(s string) string {
	return "'" + strings.ReplaceAll(termsafe.Escape(s), "'", "'\"'\"'") + "'"
}

// checkPanelCore3 consumes the actual per-registration gate decisions. A Warn
// never reads coverage, including abandoned, missing-ref and git-error paths.
func checkPanelCore3(results []PanelGateResult, enabled bool, out io.Writer) error {
	if !enabled || panelSkipEnvFn() {
		return nil
	}
	for _, result := range results {
		reg := result.Registration
		if result.Decision.Action == panel.Warn {
			if out != nil {
				fmt.Fprintf(out, "panel advisory: %s coverage check skipped after gate warning\n", termsafe.Escape(reg.Slug()))
			}
			continue
		}
		if result.Decision.Action != panel.Allow {
			continue
		}
		legacy, err := panel.LegacyPanel(reg.Dir)
		if err != nil {
			return PanelCore3Failure(reg, err)
		}
		if legacy {
			if out != nil {
				fmt.Fprintf(out, "panel advisory: %s legacy registration has no required coverage check\n", termsafe.Escape(reg.Slug()))
			}
			continue
		}
		if reg.Panel.Telemetry != "required" {
			continue
		}
		if err := panel.CheckCore3(reg); err != nil {
			return PanelCore3Failure(reg, err)
		}
	}
	return nil
}
