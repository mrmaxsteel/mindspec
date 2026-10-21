package setup

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// TestManagedBlocks_ScopeSessionStartToTopLevelSession pins that every
// managed agent-instruction block tells a dispatched subagent not to run
// the session-start and bd steps.
func TestManagedBlocks_ScopeSessionStartToTopLevelSession(t *testing.T) {
	blocks := map[string]string{
		"CLAUDE.md":               claudeMDManagedBlock,
		"copilot-instructions.md": copilotManagedBlock,
		"AGENTS.md":               agentsMDManagedBlock(config.DefaultConfig()),
	}
	for name, b := range blocks {
		if !strings.Contains(b, "for the top-level session only") ||
			!strings.Contains(b, "A subagent that works from a dispatch brief does not run `mindspec instruct` or `bd`") {
			t.Errorf("%s managed block lacks the subagent scope note:\n%s", name, b)
		}
	}
}
