package brownfield

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LineageEntry maps one source file to canonical and archive targets.
type LineageEntry struct {
	Source    string `json:"source"`
	SourceSHA string `json:"source_sha256"`
	Category  string `json:"category"`
	Canonical string `json:"canonical"`
	Archive   string `json:"archive"`
}

type applySummary struct {
	RunID              string `json:"run_id"`
	AppliedAt          string `json:"applied_at"`
	ArchiveMode        string `json:"archive_mode"`
	OperationsApplied  int    `json:"operations_applied"`
	CanonicalApplied   int    `json:"canonical_operations_applied"`
	ArchivedSources    int    `json:"archived_sources"`
	LineageEntries     int    `json:"lineage_entries"`
	SourceDriftChecked int    `json:"source_drift_checked"`
	PlanSHA256         string `json:"plan_sha256"`
}

func applyTransactional(root string, report *Report, opts RunOptions, plan *MigrationPlan) error {
	runDir := filepath.Join(root, ".mindspec", "migrations", report.RunID)
	stagingRoot := filepath.Join(runDir, "staging")
	stagingDocs := filepath.Join(stagingRoot, ".mindspec", "docs")
	if err := os.MkdirAll(stagingDocs, 0o755); err != nil {
		return fmt.Errorf("create staging docs dir: %w", err)
	}

	lineage := make([]LineageEntry, 0, len(plan.Operations))
	canonicalApplied := 0
	operationsApplied := 0

	for _, op := range plan.Operations {
		switch op.Action {
		case planActionCreate, planActionUpdate, planActionMerge, planActionSplit:
			if strings.TrimSpace(op.Target) == "" {
				return fmt.Errorf("apply %s: target is required for action %q", op.ID, op.Action)
			}
			if len(op.Sources) == 0 {
				return fmt.Errorf("apply %s: sources are required for action %q", op.ID, op.Action)
			}
			dstAbs := filepath.Join(stagingRoot, filepath.FromSlash(op.Target))
			if op.Action == planActionMerge && len(op.Sources) > 1 {
				if err := stageMergedSources(root, op, dstAbs); err != nil {
					return fmt.Errorf("apply %s: stage merge %s: %w", op.ID, op.Target, err)
				}
			} else {
				if err := stageSourceToTarget(root, op.Sources[0], op.Target, dstAbs); err != nil {
					return fmt.Errorf("apply %s: stage %s: %w", op.ID, op.Target, err)
				}
			}
			canonicalApplied++
		case planActionArchiveOnly:
			if len(op.Sources) == 0 {
				return fmt.Errorf("apply %s: archive-only operation has no sources", op.ID)
			}
		default:
			return fmt.Errorf("apply %s: unsupported action %q", op.ID, op.Action)
		}

		for _, src := range op.Sources {
			archive := filepath.ToSlash(filepath.Join("docs_archive", report.RunID, src.Path))
			lineage = append(lineage, LineageEntry{
				Source:    src.Path,
				SourceSHA: src.SHA256,
				Category:  src.Category,
				Canonical: op.Target,
				Archive:   archive,
			})
		}
		operationsApplied++
	}

	if len(lineage) == 0 {
		return fmt.Errorf("migrate apply produced no lineage entries")
	}

	sort.Slice(lineage, func(i, j int) bool {
		if lineage[i].Source == lineage[j].Source {
			return lineage[i].Canonical < lineage[j].Canonical
		}
		return lineage[i].Source < lineage[j].Source
	})
	report.Lineage = lineage
	if err := writeJSON(filepath.Join(runDir, "lineage.json"), lineage); err != nil {
		return err
	}

	if canonicalApplied > 0 {
		if err := promoteCanonical(root, stagingRoot, report.RunID); err != nil {
			return err
		}
	}
	archived, err := archiveSources(root, lineage, opts.ArchiveMode, report.RunID)
	if err != nil {
		return err
	}
	if err := writeLineageManifest(root, report.RunID, lineage); err != nil {
		return err
	}

	planHash, err := hashPlan(plan)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(runDir, "apply.json"), applySummary{
		RunID:              report.RunID,
		AppliedAt:          time.Now().UTC().Format(time.RFC3339),
		ArchiveMode:        opts.ArchiveMode,
		OperationsApplied:  operationsApplied,
		CanonicalApplied:   canonicalApplied,
		ArchivedSources:    archived,
		LineageEntries:     len(lineage),
		SourceDriftChecked: countUniqueSources(lineage),
		PlanSHA256:         planHash,
	}); err != nil {
		return err
	}

	return nil
}

func stageSourceToTarget(root string, src PlanSource, target, dstAbs string) error {
	srcAbs := filepath.Join(root, filepath.FromSlash(src.Path))
	data, err := os.ReadFile(srcAbs)
	if err != nil {
		return err
	}
	transformed := transformContentForTarget(string(data), target)

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstAbs, []byte(transformed), 0o644)
}

func stageMergedSources(root string, op PlanOperation, dstAbs string) error {
	if len(op.Sources) == 0 {
		return fmt.Errorf("merge op has no sources")
	}
	if len(op.Sources) == 1 {
		return stageSourceToTarget(root, op.Sources[0], op.Target, dstAbs)
	}

	var b strings.Builder
	b.WriteString("# Consolidated Migration Document\n\n")
	for i, src := range op.Sources {
		srcAbs := filepath.Join(root, filepath.FromSlash(src.Path))
		data, err := os.ReadFile(srcAbs)
		if err != nil {
			return fmt.Errorf("read merge source %s: %w", src.Path, err)
		}
		content := transformContentForTarget(string(data), op.Target)
		fmt.Fprintf(&b, "## Source %d: %s\n\n", i+1, src.Path)
		b.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstAbs, []byte(b.String()), 0o644)
}

func transformContentForTarget(content, target string) string {
	if strings.HasSuffix(strings.ToLower(target), "/glossary.md") {
		content = strings.ReplaceAll(content, "(docs/", "(.mindspec/docs/")
		content = strings.ReplaceAll(content, "(./docs/", "(.mindspec/docs/")
	}
	if target == filepath.ToSlash(filepath.Join(".mindspec", "policies.yml")) {
		content = strings.ReplaceAll(content, "reference: \"docs/", "reference: \".mindspec/docs/")
		content = strings.ReplaceAll(content, "reference: 'docs/", "reference: '.mindspec/docs/")
	}
	return content
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
	case "policy":
		return filepath.ToSlash(filepath.Join(".mindspec", "policies.yml")), true
	}
	return "", false
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

func archiveSources(root string, lineage []LineageEntry, archiveMode, runID string) (int, error) {
	moveMode := archiveMode == "move"
	archivedCount := 0
	seen := map[string]struct{}{}

	for _, entry := range lineage {
		if _, ok := seen[entry.Source]; ok {
			continue
		}
		seen[entry.Source] = struct{}{}

		src := filepath.Join(root, filepath.FromSlash(entry.Source))
		dst := filepath.Join(root, filepath.FromSlash(entry.Archive))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return archivedCount, fmt.Errorf("create archive dir for %s: %w", entry.Source, err)
		}

		if moveMode && shouldMoveSource(entry.Source) {
			if err := os.Rename(src, dst); err != nil {
				return archivedCount, fmt.Errorf("archive move %s -> %s: %w", entry.Source, entry.Archive, err)
			}
			archivedCount++
			continue
		}

		if err := copyFile(src, dst); err != nil {
			return archivedCount, fmt.Errorf("archive copy %s -> %s: %w", entry.Source, entry.Archive, err)
		}
		archivedCount++
	}

	if moveMode {
		if err := relocateRemainingLegacyDocs(root, runID); err != nil {
			return archivedCount, err
		}
		if err := pruneLegacyPath(filepath.Join(root, "docs")); err != nil {
			return archivedCount, err
		}
		if err := pruneLegacyPath(filepath.Join(root, "architecture")); err != nil {
			return archivedCount, err
		}
	}

	return archivedCount, nil
}

func relocateRemainingLegacyDocs(root, runID string) error {
	legacyDocs := filepath.Join(root, "docs")
	if _, err := os.Stat(legacyDocs); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat legacy docs root: %w", err)
	}

	return filepath.WalkDir(legacyDocs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relFromRoot, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("legacy rel path: %w", err)
		}
		relFromRoot = filepath.ToSlash(relFromRoot)

		archiveDst := filepath.Join(root, "docs_archive", runID, filepath.FromSlash(relFromRoot))
		if err := copyFile(path, archiveDst); err != nil {
			return fmt.Errorf("archive legacy residual %s: %w", relFromRoot, err)
		}

		// Keep spec recordings/bench artifacts with their canonical spec directories.
		if strings.HasPrefix(relFromRoot, "docs/specs/") {
			relUnderDocs := strings.TrimPrefix(relFromRoot, "docs/")
			canonicalDst := filepath.Join(root, ".mindspec", "docs", filepath.FromSlash(relUnderDocs))
			if err := moveFile(path, canonicalDst); err != nil {
				return fmt.Errorf("move legacy spec residual %s: %w", relFromRoot, err)
			}
			return nil
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy residual %s: %w", relFromRoot, err)
		}
		return nil
	})
}

func shouldMoveSource(source string) bool {
	switch {
	case strings.HasPrefix(source, "docs/"):
		return true
	case source == "GLOSSARY.md":
		return true
	case source == filepath.ToSlash(filepath.Join("architecture", "policies.yml")):
		return true
	default:
		return false
	}
}

func pruneLegacyPath(root string) error {
	if err := removeDSStore(root); err != nil {
		return err
	}
	return removeEmptyDirs(root)
}

func removeDSStore(root string) error {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", filepath.ToSlash(root), err)
	}
	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", filepath.ToSlash(root), err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if err := removeDSStore(path); err != nil {
				return err
			}
			continue
		}
		if entry.Name() == ".DS_Store" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", filepath.ToSlash(path), err)
			}
		}
	}
	return nil
}

func removeEmptyDirs(root string) error {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", filepath.ToSlash(root), err)
	}
	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", filepath.ToSlash(root), err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := removeEmptyDirs(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}

	entries, err = os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("re-read dir %s: %w", filepath.ToSlash(root), err)
	}
	if len(entries) == 0 {
		if err := os.Remove(root); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty dir %s: %w", filepath.ToSlash(root), err)
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

func hashPlan(plan *MigrationPlan) (string, error) {
	data, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("marshal plan hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func countUniqueSources(entries []LineageEntry) int {
	seen := map[string]struct{}{}
	for _, e := range entries {
		seen[e.Source] = struct{}{}
	}
	return len(seen)
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

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		// Best-effort fallback for cross-device renames.
		if copyErr := copyFile(src, dst); copyErr != nil {
			return err
		}
		if rmErr := os.Remove(src); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
		return nil
	}
	return os.ErrNotExist
}
