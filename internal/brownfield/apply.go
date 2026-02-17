package brownfield

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LineageEntry maps one source file to canonical and archive targets.
type LineageEntry struct {
	Source    string `json:"source"`
	SourceSHA string `json:"source_sha256"`
	Category  string `json:"category"`
	Canonical string `json:"canonical"`
	Archive   string `json:"archive"`
}

func applyTransactional(root string, report *Report, opts RunOptions) error {
	runDir := filepath.Join(root, ".mindspec", "migrations", report.RunID)
	stagingRoot := filepath.Join(runDir, "staging")
	stagingMindspec := filepath.Join(stagingRoot, ".mindspec")
	stagingDocs := filepath.Join(stagingMindspec, "docs")
	if err := os.MkdirAll(stagingDocs, 0o755); err != nil {
		return fmt.Errorf("create staging docs dir: %w", err)
	}

	shaByPath := make(map[string]string, len(report.Inventory))
	for _, inv := range report.Inventory {
		shaByPath[inv.Path] = inv.SHA256
	}

	var lineage []LineageEntry
	for _, c := range report.Classification {
		canonical, ok := canonicalTarget(c.Path, c.Category)
		if !ok {
			continue
		}

		srcAbs := filepath.Join(root, filepath.FromSlash(c.Path))
		dstAbs := filepath.Join(stagingRoot, filepath.FromSlash(canonical))
		if err := copyFile(srcAbs, dstAbs); err != nil {
			return fmt.Errorf("stage %s -> %s: %w", c.Path, canonical, err)
		}

		lineage = append(lineage, LineageEntry{
			Source:    c.Path,
			SourceSHA: shaByPath[c.Path],
			Category:  c.Category,
			Canonical: canonical,
			Archive:   filepath.ToSlash(filepath.Join("docs_archive", report.RunID, c.Path)),
		})
	}

	// Policies are migrated outside markdown discovery.
	policyLineage, err := stagePolicyMigration(root, stagingRoot, report.RunID)
	if err != nil {
		return err
	}
	if policyLineage != nil {
		lineage = append(lineage, *policyLineage)
	}

	if len(lineage) == 0 {
		return fmt.Errorf("brownfield apply produced no canonical targets from discovered markdown files")
	}

	sort.Slice(lineage, func(i, j int) bool { return lineage[i].Source < lineage[j].Source })
	report.Lineage = lineage
	if err := writeJSON(filepath.Join(runDir, "lineage.json"), lineage); err != nil {
		return err
	}

	if err := promoteCanonical(root, stagingRoot, report.RunID); err != nil {
		return err
	}
	if err := archiveSources(root, lineage, opts.ArchiveMode); err != nil {
		return err
	}
	if err := writeLineageManifest(root, report.RunID, lineage); err != nil {
		return err
	}
	return nil
}

func canonicalTarget(path, category string) (string, bool) {
	switch category {
	case "adr":
		if idx := strings.Index(strings.ToLower(path), "docs/adr/"); idx >= 0 {
			rel := path[idx+len("docs/adr/"):]
			return filepath.ToSlash(filepath.Join(".mindspec", "docs", "adr", rel)), true
		}
	case "spec":
		if idx := strings.Index(strings.ToLower(path), "docs/specs/"); idx >= 0 {
			rel := path[idx+len("docs/specs/"):]
			return filepath.ToSlash(filepath.Join(".mindspec", "docs", "specs", rel)), true
		}
	case "domain":
		if idx := strings.Index(strings.ToLower(path), "docs/domains/"); idx >= 0 {
			rel := path[idx+len("docs/domains/"):]
			return filepath.ToSlash(filepath.Join(".mindspec", "docs", "domains", rel)), true
		}
	case "core":
		if idx := strings.Index(strings.ToLower(path), "docs/core/"); idx >= 0 {
			rel := path[idx+len("docs/core/"):]
			return filepath.ToSlash(filepath.Join(".mindspec", "docs", "core", rel)), true
		}
	case "context-map":
		return filepath.ToSlash(filepath.Join(".mindspec", "docs", "context-map.md")), true
	case "glossary":
		return filepath.ToSlash(filepath.Join(".mindspec", "docs", "glossary.md")), true
	case "user-docs":
		lower := strings.ToLower(path)
		rel := path
		if strings.HasPrefix(lower, "docs/") {
			rel = path[len("docs/"):]
		}
		return filepath.ToSlash(filepath.Join(".mindspec", "docs", "user", rel)), true
	}
	return "", false
}

func stagePolicyMigration(root, stagingRoot, runID string) (*LineageEntry, error) {
	legacyPolicy := filepath.Join(root, "architecture", "policies.yml")
	data, err := os.ReadFile(legacyPolicy)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read legacy policies: %w", err)
	}

	rewritten := strings.ReplaceAll(string(data), "reference: \"docs/", "reference: \".mindspec/docs/")
	rewritten = strings.ReplaceAll(rewritten, "reference: 'docs/", "reference: '.mindspec/docs/")

	dst := filepath.Join(stagingRoot, ".mindspec", "policies.yml")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("create policy staging dir: %w", err)
	}
	if err := os.WriteFile(dst, []byte(rewritten), 0o644); err != nil {
		return nil, fmt.Errorf("write staged policy: %w", err)
	}

	sum := sha256.Sum256(data)
	return &LineageEntry{
		Source:    filepath.ToSlash(filepath.Join("architecture", "policies.yml")),
		SourceSHA: hex.EncodeToString(sum[:]),
		Category:  "policy",
		Canonical: filepath.ToSlash(filepath.Join(".mindspec", "policies.yml")),
		Archive:   filepath.ToSlash(filepath.Join("docs_archive", runID, "architecture", "policies.yml")),
	}, nil
}

func promoteCanonical(root, stagingRoot, runID string) error {
	targetMindspec := filepath.Join(root, ".mindspec")
	targetDocs := filepath.Join(targetMindspec, "docs")
	stagedDocs := filepath.Join(stagingRoot, ".mindspec", "docs")
	stagedPolicy := filepath.Join(stagingRoot, ".mindspec", "policies.yml")

	if err := os.MkdirAll(targetMindspec, 0o755); err != nil {
		return fmt.Errorf("ensure .mindspec dir: %w", err)
	}

	backupDocs := filepath.Join(root, ".mindspec", "migrations", runID, "preexisting-docs")
	hadDocs := false
	if _, err := os.Stat(targetDocs); err == nil {
		hadDocs = true
		if err := os.Rename(targetDocs, backupDocs); err != nil {
			return fmt.Errorf("backup existing canonical docs: %w", err)
		}
	}

	if err := os.Rename(stagedDocs, targetDocs); err != nil {
		if hadDocs {
			_ = os.Rename(backupDocs, targetDocs)
		}
		return fmt.Errorf("promote staged docs: %w", err)
	}

	if data, err := os.ReadFile(stagedPolicy); err == nil {
		if err := os.WriteFile(filepath.Join(targetMindspec, "policies.yml"), data, 0o644); err != nil {
			return fmt.Errorf("promote policies.yml: %w", err)
		}
	}

	return nil
}

func archiveSources(root string, lineage []LineageEntry, archiveMode string) error {
	for _, entry := range lineage {
		if entry.Category == "policy" {
			// Policy is not part of markdown archive handling in this phase.
			continue
		}

		src := filepath.Join(root, filepath.FromSlash(entry.Source))
		dst := filepath.Join(root, filepath.FromSlash(entry.Archive))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create archive dir for %s: %w", entry.Source, err)
		}

		switch archiveMode {
		case "move":
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("archive move %s -> %s: %w", entry.Source, entry.Archive, err)
			}
		default:
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("archive copy %s -> %s: %w", entry.Source, entry.Archive, err)
			}
		}
	}
	return nil
}

func writeLineageManifest(root, runID string, entries []LineageEntry) error {
	lineageDir := filepath.Join(root, ".mindspec", "lineage")
	if err := os.MkdirAll(lineageDir, 0o755); err != nil {
		return fmt.Errorf("create lineage dir: %w", err)
	}
	manifest := struct {
		RunID   string         `json:"run_id"`
		Entries []LineageEntry `json:"entries"`
	}{
		RunID:   runID,
		Entries: entries,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lineage manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(lineageDir, "manifest.json"), data, 0o644); err != nil {
		return fmt.Errorf("write lineage manifest: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
