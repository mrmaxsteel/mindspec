package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/templates"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

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

	// If superseding, verify old ADR exists and optionally copy domains
	if opts.Supersedes != "" {
		oldPath := filepath.Join(workspace.ADRDir(root), opts.Supersedes+".md")
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

	content := templates.ADR()
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
