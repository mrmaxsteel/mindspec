package setup

import (
	"embed"
	"strings"
)

// historicalWorkflowsFS holds byte-exact snapshots of workflow scripts
// MindSpec shipped in prior versions. installWorkflows byte-matches an
// on-disk workflow against them: a match is refreshed in place, anything
// else is user-modified and left alone with a notice.
//
//go:embed historical_workflows/*.js
var historicalWorkflowsFS embed.FS

// previouslyShippedWorkflows returns, for each workflow basename, the
// byte-exact contents MindSpec has shipped. A snapshot is named
// historical_workflows/<stem>.<tag>.js and belongs to workflow <stem>.js.
func previouslyShippedWorkflows() map[string][]string {
	out := make(map[string][]string)
	entries, _ := historicalWorkflowsFS.ReadDir("historical_workflows")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		stem, _, _ := strings.Cut(e.Name(), ".")
		data, err := historicalWorkflowsFS.ReadFile("historical_workflows/" + e.Name())
		if err != nil {
			continue
		}
		out[stem+".js"] = append(out[stem+".js"], string(data))
	}
	return out
}

// matchesShippedWorkflow reports whether current is byte-identical to one
// of the previously shipped bodies.
func matchesShippedWorkflow(current string, shipped []string) bool {
	for _, s := range shipped {
		if current == s {
			return true
		}
	}
	return false
}
