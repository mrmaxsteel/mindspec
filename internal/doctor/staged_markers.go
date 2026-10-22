package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/rules"
)

func checkStagedMarkers(r *Report, root string) {
	table, err := rules.Load()
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "Conduct rules", Status: Error, Message: err.Error()})
		return
	}
	known := map[string]bool{}
	var enforced []string
	for _, rule := range table {
		known[rule.ID] = true
		if rule.EnforcedBy != "advisory" {
			enforced = append(enforced, rule.ID+" ("+rule.EnforcedBy+")")
		}
	}
	r.Checks = append(r.Checks, Check{Name: "Enforced rules", Status: Info, Message: strings.Join(enforced, ", ")})
	today := time.Now().UTC().Format("2006-01-02")
	for _, base := range []string{".claude/skills", ".agents/skills"} {
		_ = filepath.WalkDir(filepath.Join(root, base), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if !os.IsNotExist(walkErr) {
					r.Checks = append(r.Checks, Check{Name: "Staged rules", Status: Warn, Message: walkErr.Error()})
				}
				return nil
			}
			if entry.IsDir() || entry.Name() != "SKILL.md" {
				return nil
			}
			data, err := os.ReadFile(path) //nolint:gosec // Go 1.23 has no root-scoped read API. This check only parses local text.
			if err != nil {
				r.Checks = append(r.Checks, Check{Name: "Staged rules", Status: Warn, Message: err.Error()})
				return nil
			}
			for i, marker := range rules.ParseStagedMarkers(string(data)) {
				var issues []string
				if marker.Err != nil {
					issues = append(issues, marker.Err.Error())
				}
				if marker.ID == "" {
					issues = append(issues, "missing id")
				} else if !known[marker.ID] {
					issues = append(issues, "unknown id "+marker.ID)
				}
				if !marker.RemoveBy.IsZero() && marker.RemoveBy.Format("2006-01-02") < today {
					issues = append(issues, "past remove-by date "+marker.RemoveBy.Format("2006-01-02"))
				}
				if len(issues) > 0 {
					rel, _ := filepath.Rel(root, path)
					r.Checks = append(r.Checks, Check{Name: "Staged rule", Status: Warn, Message: fmt.Sprintf("%s marker %d: %s", rel, i+1, strings.Join(issues, "; "))})
				}
			}
			return nil
		})
	}
}
