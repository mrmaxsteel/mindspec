package bootstrap

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// TestStarterDocs_ScopeSessionStartToTopLevelSession pins that every
// starter and appended agent-instruction text tells a dispatched subagent
// not to run the session-start and bd steps.
func TestStarterDocs_ScopeSessionStartToTopLevelSession(t *testing.T) {
	cfg := config.DefaultConfig()
	docs := map[string]string{
		"starter AGENTS.md": renderStarterAgentsMD(cfg),
		"append AGENTS.md":  renderAppendAgentsBlock(cfg),
		"starter CLAUDE.md": starterClaudeMD,
		"append CLAUDE.md":  appendClaudeBlock,
		"starter copilot":   starterCopilotInstructionsMD,
		"append copilot":    appendCopilotBlock,
	}
	for name, d := range docs {
		if !strings.Contains(d, "for the top-level session only") ||
			!strings.Contains(d, "A subagent that works from a dispatch brief does not run `mindspec instruct` or `bd`") {
			t.Errorf("%s lacks the subagent scope note:\n%s", name, d)
		}
	}
}
