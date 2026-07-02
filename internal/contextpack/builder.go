package contextpack

import (
	"path/filepath"
	"strings"
)

// ExtractSection extracts the content under a markdown ## heading, collecting
// lines until the next ## heading or EOF. Returns empty string if not found.
func ExtractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var collecting bool
	var result []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if collecting {
				break
			}
			h := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if strings.EqualFold(h, heading) {
				collecting = true
				continue
			}
		}
		if collecting {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func relPath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return rel
}
