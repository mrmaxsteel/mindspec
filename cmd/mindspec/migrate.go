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
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/layout"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
MindSpec documentation structure: lifecycle/authored artifacts under the flat
.mindspec/{specs,adr,domains,core} children, and user/dogfood docs under
top-level project-docs/.

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

// migrateLayoutCmd drives the deterministic, transactional, idempotent flatten
// of .mindspec/docs/{specs,adr,domains,core} + context-map.md into the flat
// .mindspec/ layout (spec 106, Reqs 4/5/11/14). It runs the precondition
// branch/PR discovery scan first (blocks on an unmerged pre-flatten branch/PR
// and on a dirty tree; tolerates locked worktrees / external forks; offline it
// degrades + WARNs), then drives the internal/layout mover. `--abort`
// hard-resets a pre-publish run to its pre-run ref.
var migrateLayoutCmd = &cobra.Command{
	Use:   "layout",
	Short: "Flatten .mindspec/docs/{specs,adr,domains,core} into .mindspec/ (irreversible)",
	Long: `Flattens the canonical .mindspec/docs/{specs,adr,domains,core} +
context-map.md tree into the flat .mindspec/ layout via a deterministic,
two-commit-per-move (pure git mv, then link-rewrite), crash-resumable mover.

Precondition: a clean working tree and no unmerged pre-flatten branch/PR.
Locked agent worktrees and external forks are tolerated. Offline, the scan
degrades to local + remote-tracking refs and warns that hosted PRs could not
be consulted.

--abort hard-resets a pre-publish run to its pre-run ref (refused after
publish; the lifecycle is forward-only, ADR-0023).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		abort, _ := cmd.Flags().GetBool("abort")
		runID, _ := cmd.Flags().GetString("run-id")
		target, _ := cmd.Flags().GetString("target")
		force, _ := cmd.Flags().GetBool("force")
		allowBranches, _ := cmd.Flags().GetStringArray("allow-branch")

		root, err := gitutil.RevParseShowToplevel()
		if err != nil {
			cwd, wdErr := os.Getwd()
			if wdErr != nil {
				return fmt.Errorf("resolving repo root: %w", err)
			}
			root = cwd
		}

		exec := executor.NewMindspecExecutor(root)

		if abort {
			if runID == "" {
				return fmt.Errorf("--abort requires --run-id <id> of the run to roll back")
			}
			mover := layout.NewMover(exec, root, runID)
			if err := mover.Abort(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "migrate layout: aborted run %s (hard-reset to pre-run ref)\n", runID)
			return nil
		}

		// RESUME detection (panel R5): an in-progress run is resumed BEFORE the
		// fresh-run clean-tree precondition is enforced. A crash AFTER the
		// rewrite-but-before-commit step leaves dirty moved markdown; enforcing
		// the clean-tree check first would refuse to resume it. The clean-tree
		// precondition applies only to a FRESH start.
		resumeID := runID
		if resumeID == "" {
			id, found, ferr := layout.FindResumableRun(root)
			if ferr != nil {
				return ferr
			}
			if found {
				resumeID = id
			}
		}
		resuming := false
		if resumeID != "" {
			ok, rerr := layout.IsResumable(root, resumeID)
			if rerr != nil {
				return rerr
			}
			resuming = ok
		}

		if resuming {
			runID = resumeID
			fmt.Fprintf(cmd.ErrOrStderr(), "migrate layout: resuming in-progress run %s (skipping fresh-run clean-tree precondition)\n", runID)
		} else {
			// Fresh run: clean tree + branch/PR discovery scan.
			allowlist := make(map[string]bool, len(allowBranches))
			for _, b := range allowBranches {
				if b = strings.TrimSpace(b); b != "" {
					allowlist[b] = true
				}
			}
			remoteDefault := ""
			if gitutil.HasRemote() {
				if d, derr := gitutil.DetectDefaultBranch("origin"); derr == nil {
					remoteDefault = d
				}
			}
			locked, _ := gitutil.LockedWorktreeBranches(root)
			lockedSet := make(map[string]bool, len(locked))
			for _, b := range locked {
				lockedSet[b] = true
			}
			res, perr := layout.CheckPrecondition(exec, root, layout.PreconditionOptions{
				Target:           target,
				RemoteDefault:    remoteDefault,
				LockedWorktrees:  lockedSet,
				Allowlist:        allowlist,
				Force:            force,
				Offline:          !gitutil.HasRemote(),
				RequireCleanTree: true,
			})
			if perr != nil {
				return perr
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARN: %s\n", w)
			}
			if len(res.Blocking) > 0 {
				return layout.BlockingError(res)
			}

			if runID == "" {
				runID = time.Now().UTC().Format("20060102T150405Z")
			}
		}

		mover := layout.NewMover(exec, root, runID)
		if err := mover.Run(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "migrate layout: flatten complete (run %s)\n", runID)
		return nil
	},
}

func init() {
	migrateCmd.Flags().Bool("json", false, "Output file inventory as JSON instead of a prompt")

	migrateLayoutCmd.Flags().Bool("abort", false, "Hard-reset a pre-publish run to its pre-run ref")
	migrateLayoutCmd.Flags().String("run-id", "", "Run identifier (default: UTC timestamp; required with --abort)")
	migrateLayoutCmd.Flags().String("target", "main", "Merge target the discovery scan evaluates refs against")
	migrateLayoutCmd.Flags().Bool("force", false, "Bypass the unmerged pre-flatten branch/PR blockers (for known-irrelevant old/abandoned branches)")
	migrateLayoutCmd.Flags().StringArray("allow-branch", nil, "Branch name to treat as known-irrelevant (repeatable); tolerated, never a blocker")
	migrateCmd.AddCommand(migrateLayoutCmd)
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
			if name == "beads" || strings.HasPrefix(name, workspace.BeadWorktreePrefix) {
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

You are setting up this repository's documentation structure for MindSpec. This is a
multi-phase process: first understand the codebase, then identify domains, then organize
existing documentation.

## Phase 1 — Codebase Analysis

Scan the source code structure to understand the project's natural boundaries:

1. Examine top-level directories, package/module layout, and service boundaries
2. Look for architectural signals: package naming conventions, module boundaries,
   service directories, API groupings, shared libraries
3. Identify what code changes together vs. what has independent interfaces
4. Note any existing documentation that describes architecture or domain boundaries

## Phase 2 — Domain Identification

Based on Phase 1, propose bounded domains. For each domain:

- **Slug**: short kebab-case name (e.g., ` + "`auth`" + `, ` + "`billing`" + `, ` + "`core-api`" + `)
- **Responsibilities**: what this domain owns
- **Boundaries**: where this domain starts and ends in the codebase
- **Key files/packages**: primary source locations

Create each domain using:

` + "```bash" + `
mindspec domain add <slug>
` + "```" + `

This auto-scaffolds the domain directory (overview.md, architecture.md, interfaces.md,
runbook.md) and updates the context map.

## Phase 3 — Source-Globs Population

Declare which path globs count as "source" for the doc-sync gate. Run (once,
repo-wide):

` + "```bash" + `
mindspec source populate
` + "```" + `

This prints an agent prompt; follow it to populate the ` + "`source_globs:`" + ` field in
` + "`.mindspec/config.yaml`" + ` from this repo's actual layout. The framework proposes no
globs — the prompt instructs you to derive them by inspecting the tree.

## Phase 4 — Ownership-Manifest Population

For each domain created in Phase 2, populate the empty-stub OWNERSHIP.yaml that
` + "`mindspec domain add`" + ` scaffolded. Run, per domain:

` + "```bash" + `
mindspec ownership populate <slug>
` + "```" + `

This prints an agent prompt; follow it to fill the domain's ` + "`paths:`" + ` list from
this repo's actual layout. The framework proposes no paths — the prompt
instructs you to derive them by inspecting the tree.

## Phase 5 — Context Map Population

After domains exist, populate ` + "`.mindspec/context-map.md`" + `:

1. Identify upstream/downstream relationships between domains
2. Document peer relationships and shared-kernel patterns
3. Record integration contracts (APIs, events, shared types)

Use this format for each relationship:

` + "```markdown" + `
### <Source> → <Target> (<relationship-type>)

<description of the contract>

**Contract**: [interfaces](domains/<source>/interfaces.md)
` + "```" + `

## Phase 6 — Domain Doc Population

For each domain, fill in the scaffolded files with real content from the codebase:

- **overview.md**: What this domain does, why it exists, key concepts
- **architecture.md**: Internal structure, patterns, key types, data flow
- **interfaces.md**: Public API surface, exported functions, contracts with other domains
- **runbook.md**: How to build, test, debug, and operate this domain

Write from the actual codebase — not placeholders. Read the source files to populate
accurate documentation.

## Phase 7 — File Classification

Finally, classify and move any stray documentation files into canonical locations.

### Canonical Structure

` + "```" + `
.mindspec/                # Lifecycle/authored artifacts (FLAT — no docs/ nesting)
├── adr/                  # Architecture Decision Records (ADR-NNNN.md)
├── core/                 # Project-wide architecture, conventions, modes, usage
├── domains/              # Bounded domain docs (overview.md, architecture.md, interfaces.md, runbook.md)
├── specs/                # Feature specs (NNN-slug/spec.md, plan.md)
└── context-map.md        # Bounded-context map and cross-context contracts

project-docs/             # User/dogfood docs — TOP-LEVEL, NOT under .mindspec/
├── user/                 # READMEs, guides, onboarding, operational notes
├── installation/         # Install/setup notes
└── research/             # Background research
` + "```" + `

### Category Rubric

| Category | Description | Target |
|----------|-------------|--------|
| adr | Architecture Decision Records (ADR-NNNN, decision/status content) | .mindspec/adr/ |
| spec | Feature specs, plans, acceptance criteria, context packs | .mindspec/specs/ |
| domain | Docs scoped to a bounded domain (overview, architecture, interfaces, runbook) | .mindspec/domains/<domain-name>/ |
| core | Project-wide architecture, process, conventions (not domain-specific) | .mindspec/core/ |
| context-map | Bounded-context map and cross-context relationships | .mindspec/context-map.md |
| user-docs | READMEs, guides, operational notes, onboarding/help content | project-docs/ |
| agent | Agent/tool instruction files (CLAUDE.md, agents.md, .cursorrules, copilot configs) | (repo root — typically category: skip) |
| skip | Files that should stay where they are (e.g., root README.md, CHANGELOG.md) | (no move) |

### Decision Rules

1. Content outweighs path when they conflict
2. If a file contains mixed content that should be split, split it into separate files
3. Root-level README.md and CHANGELOG.md typically stay in place (category: skip)
4. Files already under .mindspec/ (specs/, adr/, domains/, core/) or project-docs/ are already canonical — skip them
5. Preserve relative links between documents (update paths after moving)

`)

	b.WriteString("## Source Files to Classify\n\n")
	if len(sourceFiles) == 0 {
		b.WriteString("No source markdown files found outside the canonical .mindspec/ + project-docs/ locations.\n\n")
	} else {
		b.WriteString("These markdown files were found outside the canonical docs location:\n\n")
		for _, f := range sourceFiles {
			b.WriteString("- `" + f + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Existing Canonical Docs\n\n")
	if len(canonicalFiles) == 0 {
		b.WriteString("No existing canonical docs found. The flat .mindspec/ structure will be created.\n\n")
	} else {
		b.WriteString("These files already exist in the canonical location:\n\n")
		for _, f := range canonicalFiles {
			b.WriteString("- `" + f + "`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(`## Instructions

1. Complete Phases 1-6 first — domain discovery and population is the priority
2. Then classify and move stray files per Phase 7
3. Do NOT delete original files until you have verified the migration is correct
4. After migration, run ` + "`mindspec doctor`" + ` to verify the structure is valid
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
	if !gitutil.IsInsideWorkTree(root) {
		return c
	}
	c.enabled = true
	return c
}

func (c *migrateIgnoreChecker) isIgnored(relPath string) bool {
	if !c.enabled {
		return false
	}
	return gitutil.CheckIgnore(c.root, relPath) == nil
}
