package setup

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestRepoAgentDocs_ScopeSessionStartToTopLevelSession pins that the
// checked-in agent-instruction docs of this repo carry the subagent scope
// note. The repo root is found from this test file's path.
func TestRepoAgentDocs_ScopeSessionStartToTopLevelSession(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to resolve this test file's path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	for _, rel := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		".github/copilot-instructions.md",
		"project-docs/user/AGENTS.md",
		"project-docs/user/CLAUDE.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		d := string(data)
		if !strings.Contains(d, "for the top-level session only") ||
			!strings.Contains(d, "A subagent that works from a dispatch brief does not run `mindspec instruct`") {
			t.Errorf("%s lacks the subagent scope note", rel)
		}
	}
}
