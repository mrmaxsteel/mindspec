package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// MigrateInventory is the JSON output of mindspec migrate --json.
type MigrateInventory struct {
	SourceFiles    []string `json:"source_files"`
	CanonicalFiles []string `json:"canonical_files"`
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Emit a prompt instructing the coding agent to reorganize docs",
	Long: `Scans the repository for markdown files and emits a structured prompt
that instructs the coding agent to reorganize them into the canonical
MindSpec documentation structure under .mindspec/docs/.

Use --json to output just the file inventory for programmatic use.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")

		root, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		sourceFiles, err := scanSourceMarkdown(root)
		if err != nil {
			return fmt.Errorf("scanning markdown files: %w", err)
		}

		canonicalFiles, err := scanCanonicalDocs(root)
		if err != nil {
			return fmt.Errorf("scanning canonical docs: %w", err)
		}

		if jsonFlag {
			inv := MigrateInventory{
				SourceFiles:    sourceFiles,
				CanonicalFiles: canonicalFiles,
			}
			data, err := json.MarshalIndent(inv, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Println(buildMigratePrompt(sourceFiles, canonicalFiles))
		return nil
	},
}

func init() {
	migrateCmd.Flags().Bool("json", false, "Output file inventory as JSON instead of a prompt")
}

// scanSourceMarkdown walks the repo for .md files outside the canonical docs area.
func scanSourceMarkdown(root string) ([]string, error) {
	ignored := newMigrateIgnoreChecker(root)
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", ".beads", ".claude", "docs_archive", "node_modules", "vendor":
				return filepath.SkipDir
			}
			// Skip .mindspec/docs and .mindspec/migrations
			if name == "docs" || name == "migrations" {
				if filepath.Base(filepath.Dir(path)) == ".mindspec" {
					return filepath.SkipDir
				}
			}
			// Skip vendored/dependency repos and worktree clones
			if name == "beads" || strings.HasPrefix(name, "worktree-") {
				return filepath.SkipDir
			}
			// Skip nested git repos
			if path != root {
				if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)

		// Skip Go template files
		if strings.HasPrefix(strings.ToLower(rel), "internal/instruct/templates/") {
			return nil
		}

		if ignored.isIgnored(rel) {
			return nil
		}

		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// scanCanonicalDocs lists existing files under .mindspec/docs/.
func scanCanonicalDocs(root string) ([]string, error) {
	docsDir := filepath.Join(root, ".mindspec", "docs")
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func buildMigratePrompt(sourceFiles, canonicalFiles []string) string {
	var b strings.Builder

	b.WriteString(`# MindSpec Documentation Migration

You are reorganizing this repository's documentation into the canonical MindSpec structure.

## Canonical Structure

All documentation lives under` + " `.mindspec/docs/`" + ` with this layout:

` + "```" + `
.mindspec/docs/
├── adr/              # Architecture Decision Records (ADR-NNNN.md)
├── core/             # Project-wide architecture, conventions, modes, usage
├── domains/          # Bounded domain docs (each domain has overview.md, architecture.md, interfaces.md, runbook.md)
├── specs/            # Feature specs (NNN-slug/spec.md, plan.md)
├── user/             # READMEs, guides, onboarding, operational notes
├── agent/            # Agent instruction files (CLAUDE.md, .cursorrules, etc.)
├── context-map.md    # Bounded-context map and cross-context contracts
└── glossary.md       # Term definitions and concept index
` + "```" + `

## Category Rubric

Classify each source file into exactly one category:

| Category | Description | Target |
|----------|-------------|--------|
| adr | Architecture Decision Records (ADR-NNNN, decision/status content) | .mindspec/docs/adr/ |
| spec | Feature specs, plans, acceptance criteria, context packs | .mindspec/docs/specs/ |
| domain | Docs scoped to a bounded domain (overview, architecture, interfaces, runbook) | .mindspec/docs/domains/<domain-name>/ |
| core | Project-wide architecture, process, conventions (not domain-specific) | .mindspec/docs/core/ |
| context-map | Bounded-context map and cross-context relationships | .mindspec/docs/context-map.md |
| glossary | Term-definition/index docs mapping concepts to references | .mindspec/docs/glossary.md |
| user-docs | READMEs, guides, operational notes, onboarding/help content | .mindspec/docs/user/ |
| agent | Agent/tool instruction files (CLAUDE.md, agents.md, .cursorrules, copilot configs) | .mindspec/docs/agent/ |
| skip | Files that should stay where they are (e.g., root README.md, CHANGELOG.md) | (no move) |

## Decision Rules

1. Content outweighs path when they conflict
2. If a file clearly belongs to a single category, move it to the target location
3. If a file contains mixed content that should be split, split it into separate files in the appropriate locations
4. If a file is already in the right place, skip it
5. Root-level README.md and CHANGELOG.md typically stay in place (category: skip)
6. Files already under .mindspec/docs/ are already canonical — skip them
7. Preserve relative links between documents (update paths after moving)

`)

	b.WriteString("## Source Files to Classify\n\n")
	if len(sourceFiles) == 0 {
		b.WriteString("No source markdown files found outside .mindspec/docs/.\n\n")
	} else {
		b.WriteString("These markdown files were found outside the canonical docs location:\n\n")
		for _, f := range sourceFiles {
			b.WriteString("- `" + f + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Existing Canonical Docs\n\n")
	if len(canonicalFiles) == 0 {
		b.WriteString("No existing canonical docs found. The .mindspec/docs/ directory will be created.\n\n")
	} else {
		b.WriteString("These files already exist in the canonical location:\n\n")
		for _, f := range canonicalFiles {
			b.WriteString("- `" + f + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(`## Instructions

1. Read each source file and classify it using the rubric above
2. For each file, decide: move, merge into existing canonical file, split, or skip
3. Create the target directories if they don't exist
4. Move/copy files to their canonical locations
5. Update internal links in moved files to reflect new paths
6. If merging content into an existing canonical file, append or integrate thoughtfully
7. Do NOT delete the original files until you have verified the migration is correct
8. After migration, run ` + "`mindspec doctor`" + ` to verify the structure is valid
`)

	return b.String()
}

// migrateIgnoreChecker uses git check-ignore to skip gitignored files.
type migrateIgnoreChecker struct {
	root    string
	enabled bool
}

func newMigrateIgnoreChecker(root string) *migrateIgnoreChecker {
	c := &migrateIgnoreChecker{root: root}
	if _, err := exec.LookPath("git"); err != nil {
		return c
	}
	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return c
	}
	c.enabled = true
	return c
}

func (c *migrateIgnoreChecker) isIgnored(relPath string) bool {
	if !c.enabled {
		return false
	}
	cmd := exec.Command("git", "-C", c.root, "check-ignore", "--quiet", "--", relPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
