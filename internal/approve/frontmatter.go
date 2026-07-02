package approve

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/frontmatter"

	"gopkg.in/yaml.v3"
)

// mutateFrontmatterFile reads the YAML frontmatter at path, applies mutate to
// the parsed field map, re-marshals it, and rewrites the file with the new
// block spliced ahead of the exact post-fence body bytes.
//
// This is the single consolidation of the two historical near-duplicate
// mutate-rewrite scanners (updatePlanApproval and writeBeadIDsToFrontmatter),
// which differed only in the mutation applied. Frontmatter location goes
// through the canonical internal/frontmatter.Parse (ARCH-6 / mindspec-npd2)
// rather than a hand-rolled `---` fence scan, so a space-padded fence reads as
// no-frontmatter here exactly as it does in every other migrated reader.
//
// For well-formed inputs the emitted bytes are identical to the historical
// splice: "---\n" + TrimRight(marshaled, "\n") + "\n---\n" + <original bytes
// after the closing fence>. The body is copied verbatim from data[bodyOffset:]
// (not re-joined from split lines), so any post-fence byte sequence is
// preserved.
func mutateFrontmatterFile(path string, mutate func(map[string]interface{})) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading plan: %w", err)
	}

	block, bodyOffset, ok := frontmatter.Parse(data)
	if !ok {
		return fmt.Errorf("no frontmatter found")
	}

	var fmMap map[string]interface{}
	if err := yaml.Unmarshal(block, &fmMap); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}
	if fmMap == nil {
		fmMap = make(map[string]interface{})
	}

	mutate(fmMap)

	newFm, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	output := "---\n" + strings.TrimRight(string(newFm), "\n") + "\n---\n" + string(data[bodyOffset:])
	return os.WriteFile(path, []byte(output), 0644)
}
