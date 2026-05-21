package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

const adrTemplate = `# ADR-NNNN: <Title>

- **Date**: <YYYY-MM-DD>
- **Status**: Proposed
- **Domain(s)**: <comma-separated list>
- **Deciders**: <who decides>
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

<What is the issue that we're seeing that motivates this decision or change?>

## Decision

<What is the change that we're proposing and/or doing?>

## Decision Details

<Detailed breakdown of the decision. Use subsections as needed.>

## Consequences

### Positive

- <Positive consequence 1>
- <Positive consequence 2>

### Negative / Tradeoffs

- <Negative consequence or tradeoff 1>
- <Negative consequence or tradeoff 2>

## Alternatives Considered

### 1. <Alternative name>

<Description and why it was rejected.>

## Validation / Rollout

1. <Validation step 1>
2. <Validation step 2>
`

// CreateOpts configures ADR creation.
type CreateOpts struct {
	Domains    []string
	Supersedes string
}

// Create generates a new ADR file from the template.
// Returns the path to the created file.
func Create(root, title string, opts CreateOpts) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("title must not be empty")
	}

	// If superseding, verify old ADR exists and optionally copy domains.
	//
	// SEC-1 hot path (bead mindspec-x1qr): opts.Supersedes is user-controlled
	// and previously flowed straight into filepath.Join, enabling a
	// "../../../tmp/poisoned" traversal that mutated arbitrary *.md files.
	// We validate BEFORE any join so the error message is precise and no
	// path-construction code runs on attacker input.
	if opts.Supersedes != "" {
		if err := idvalidate.ADRID(opts.Supersedes); err != nil {
			return "", fmt.Errorf("invalid --supersedes value: %w", err)
		}
		oldPath, err := workspace.ADRFilePath(root, opts.Supersedes)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			return "", fmt.Errorf("%s not found", opts.Supersedes)
		}
		if len(opts.Domains) == 0 {
			domains, err := CopyDomains(root, opts.Supersedes)
			if err != nil {
				return "", fmt.Errorf("copying domains from %s: %w", opts.Supersedes, err)
			}
			opts.Domains = domains
		}
	}

	id, err := NextID(root)
	if err != nil {
		return "", fmt.Errorf("generating next ID: %w", err)
	}

	content := adrTemplate
	content = strings.ReplaceAll(content, "NNNN", id)
	content = strings.ReplaceAll(content, "<Title>", title)
	content = strings.ReplaceAll(content, "<YYYY-MM-DD>", time.Now().Format("2006-01-02"))

	if len(opts.Domains) > 0 {
		content = strings.Replace(content, "<comma-separated list>", strings.Join(opts.Domains, ", "), 1)
	}

	if opts.Supersedes != "" {
		content = strings.Replace(content, "**Supersedes**: n/a", fmt.Sprintf("**Supersedes**: %s", opts.Supersedes), 1)
	}

	adrDir := workspace.ADRDir(root)
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return "", fmt.Errorf("creating ADR directory: %w", err)
	}

	outPath := filepath.Join(adrDir, fmt.Sprintf("ADR-%s.md", id))
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing ADR file: %w", err)
	}

	// If superseding, update the old ADR
	if opts.Supersedes != "" {
		newID := fmt.Sprintf("ADR-%s", id)
		if err := Supersede(root, opts.Supersedes, newID); err != nil {
			return outPath, fmt.Errorf("updating superseded ADR: %w", err)
		}
	}

	return outPath, nil
}

// CreateWithID generates a new ADR file at a caller-supplied ID
// (e.g. "ADR-0099") instead of auto-allocating via NextID. Used by the
// Spec 087 Bead 3 supersede flow which pre-creates a placeholder ADR
// with Status: Proposed so the divergence-gate skip path is
// deterministic (revision 1 of the panel-revised plan: the file MUST
// exist on disk at the user-supplied ID verbatim).
//
// Contract:
//   - id MUST already have passed idvalidate.ADRID at the call site
//     (the CLI layer enforces this before reaching here).
//   - If a file at workspace.ADRFilePath(root, id) — or any
//     `<id>-<slug>.md` collision — already exists, returns an error
//     whose message contains the substring "already exists" and writes
//     nothing.
//   - Reuses the existing template-fill logic from Create but
//     SUBSTITUTES the user-supplied id verbatim. Status is Proposed,
//     Domains seed from opts.Domains.
//   - The Supersedes/Superseded-by chain update from Create does NOT
//     apply: CreateWithID is for placeholder creation, which has no
//     "old" ADR to update.
func CreateWithID(root, id, title string, opts CreateOpts) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("title must not be empty")
	}

	// Validate the ID defensively in case the caller forgot.
	if err := idvalidate.ADRID(id); err != nil {
		return "", fmt.Errorf("invalid ADR ID: %w", err)
	}

	outPath, err := workspace.ADRFilePath(root, id)
	if err != nil {
		return "", err
	}

	// Collision check: exact path AND slug variants. Mirrors adr.Show's
	// lookup discipline so we don't create ADR-0099.md when an
	// ADR-0099-<slug>.md already exists.
	if _, statErr := os.Stat(outPath); statErr == nil {
		return "", fmt.Errorf("ADR %s already exists at %s", id, outPath)
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	slugMatches, globErr := filepath.Glob(filepath.Join(workspace.ADRDir(root), id+"-*.md"))
	if globErr != nil {
		return "", globErr
	}
	if len(slugMatches) > 0 {
		return "", fmt.Errorf("ADR %s already exists at %s", id, slugMatches[0])
	}

	// Strip the "ADR-" prefix to obtain the bare ID for the template
	// header substitution (the NextID-driven path uses just digits).
	bareID := strings.TrimPrefix(id, "ADR-")

	content := adrTemplate
	content = strings.ReplaceAll(content, "NNNN", bareID)
	content = strings.ReplaceAll(content, "<Title>", title)
	content = strings.ReplaceAll(content, "<YYYY-MM-DD>", time.Now().Format("2006-01-02"))

	if len(opts.Domains) > 0 {
		content = strings.Replace(content, "<comma-separated list>", strings.Join(opts.Domains, ", "), 1)
	}

	adrDir := workspace.ADRDir(root)
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return "", fmt.Errorf("creating ADR directory: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing ADR file: %w", err)
	}

	return outPath, nil
}
