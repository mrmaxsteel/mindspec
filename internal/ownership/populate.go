package ownership

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// populatePromptTemplate is the spec 091 Requirement 10 agent prompt,
// verbatim. The <domain> token is the only substitution. ZFC: the
// framework provides NO pattern hints and pre-fills NO globs — the
// prompt says so explicitly; regression tests assert the emitted text
// never contains a framework-proposed `internal/<domain>/**` value.
const populatePromptTemplate = `Populate .mindspec/docs/domains/<domain>/OWNERSHIP.yaml
for the "<domain>" domain.

Read ` + "`.mindspec/docs/domains/<domain>/overview.md`" + ` and
` + "`architecture.md`" + ` to understand what this domain owns.
Then inspect THIS repo's actual layout — ` + "`ls`, `find`" + `,
` + "`go list ./...`" + `, or whatever discovery commands fit your
tools — and identify the source globs that implement the
behaviour described in those docs.

The framework deliberately provides no pattern hints. The
domain name is a semantic label; the source paths are an
empirical question about this specific repo. Do not assume
the domain name matches any directory name (e.g. a domain
named "payments" may correspond to ` + "`internal/ledger/`" + `,
or to something else entirely — only the repo can tell you).

Manifest schema: a ` + "`paths:`" + ` list of globs, plus an
optional ` + "`exclude:`" + ` list of globs subtracted from
` + "`paths`" + ` (see spec 086 / ADR-0031). Entries whose first
path segment is ` + "`viz`, `agentmind`, or `bench`" + ` are a
HARD ERROR — those subsystems are out of doc-sync scope;
never claim paths under them.

When done, edit the manifest's ` + "`paths:`" + ` list. Verify each
path resolves to at least one file (` + "`mindspec doctor`" + `
will Warn ` + "`dead-manifest`" + ` if it does not). Run
` + "`mindspec doctor`" + ` to confirm no ` + "`dead-manifest`" + ` /
` + "`redundant-subpath` / `duplicate-entry` / `domain-overlap`" + `
Warns remain. (` + "`unclaimed-source`" + ` is a diff-time Warn
surfaced by ` + "`mindspec complete` / `mindspec approve impl`" + `,
not by doctor.)
`

// BuildPopulatePrompt returns the Requirement 10 agent prompt for the
// named domain. Prompt emission only — this function never reads or
// writes the manifest, so it works identically for missing, empty-stub,
// and populated manifests (the explicit-arg re-emit behavior that the
// Requirement 16 widen-hint relies on).
func BuildPopulatePrompt(domain string) string {
	return strings.ReplaceAll(populatePromptTemplate, "<domain>", domain)
}

// DomainsNeedingPopulate returns the lexicographically-sorted domain
// directory names under .mindspec/docs/domains/ whose OWNERSHIP.yaml
// is missing or an empty stub (Ownership.Source() ∈ {"missing",
// "empty-stub"}). This drives the no-arg `mindspec ownership populate`
// enumeration (spec 091 Req 10): one prompt per unpopulated manifest
// so the agent can fill them all in one pass. Populated manifests are
// skipped — re-emission for those requires an explicit domain arg.
// Manifest state comes from validate.LoadOwnership (Bead 1's loader);
// this package must NOT reimplement manifest parsing.
func DomainsNeedingPopulate(root string) ([]string, error) {
	dir := filepath.Join(root, ".mindspec", "docs", "domains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading domains dir %s: %w", dir, err)
	}

	var needing []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		o, err := validate.LoadOwnership(root, name)
		if err != nil {
			return nil, err
		}
		switch o.Source() {
		case "missing", "empty-stub":
			needing = append(needing, name)
		}
	}
	sort.Strings(needing)
	return needing, nil
}
