package brownfield

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InventoryEntry captures one discovered markdown source and content hash.
type InventoryEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// ClassificationEntry captures deterministic category scoring for one source.
type ClassificationEntry struct {
	Path        string  `json:"path"`
	SHA256      string  `json:"sha256"`
	Category    string  `json:"category"`
	Confidence  float64 `json:"confidence"`
	Rule        string  `json:"rule"`
	RequiresLLM bool    `json:"requires_llm"`
}

// LLMConfig captures resolved LLM provider settings from environment.
type LLMConfig struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Available bool   `json:"available"`
}

// RunOptions controls brownfield run behavior.
type RunOptions struct {
	Apply       bool
	ArchiveMode string
	RunID       string
}

// Report captures deterministic brownfield discovery output.
type Report struct {
	RunID          string                `json:"run_id"`
	Mode           string                `json:"mode"`
	MarkdownFiles  []string              `json:"markdown_files"`
	Inventory      []InventoryEntry      `json:"inventory"`
	Classification []ClassificationEntry `json:"classification"`
	LLM            LLMConfig             `json:"llm"`
	Unresolved     []string              `json:"unresolved_paths"`
}

// DiscoverMarkdown scans root for markdown files and returns deterministic output.
func DiscoverMarkdown(root string) (*Report, error) {
	var (
		files     []string
		inventory []InventoryEntry
	)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			name := d.Name()
			// Skip VCS and bead internals from brownfield corpus discovery.
			if name == ".git" || name == ".beads" || name == "docs_archive" {
				return filepath.SkipDir
			}
			if name == "migrations" && filepath.Base(filepath.Dir(path)) == ".mindspec" {
				return filepath.SkipDir
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
		files = append(files, rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		inventory = append(inventory, InventoryEntry{
			Path:   rel,
			SHA256: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	sort.Slice(inventory, func(i, j int) bool { return inventory[i].Path < inventory[j].Path })

	return &Report{
		MarkdownFiles: files,
		Inventory:     inventory,
	}, nil
}

// FormatSummary renders a compact report summary.
func (r *Report) FormatSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Brownfield discovery report (run-id: %s)\n", r.RunID)
	fmt.Fprintf(&b, "  Markdown files discovered: %d\n", len(r.MarkdownFiles))
	if len(r.Classification) > 0 {
		fmt.Fprintf(&b, "  Classified entries: %d\n", len(r.Classification))
		fmt.Fprintf(&b, "  Low-confidence (LLM-required): %d\n", len(r.Unresolved))
	}
	if r.Mode != "" {
		fmt.Fprintf(&b, "  Mode: %s\n", r.Mode)
	}
	fmt.Fprintf(&b, "  LLM provider: %s (available=%t)\n", r.LLM.Provider, r.LLM.Available)
	for i, f := range r.MarkdownFiles {
		if i >= 20 {
			remaining := len(r.MarkdownFiles) - 20
			fmt.Fprintf(&b, "  ... and %d more\n", remaining)
			break
		}
		fmt.Fprintf(&b, "  - %s\n", f)
	}
	return b.String()
}

// ToJSON renders report output as structured JSON.
func (r *Report) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ResolveLLMConfig resolves LLM provider settings from environment variables.
func ResolveLLMConfig() LLMConfig {
	provider := strings.TrimSpace(os.Getenv("MINDSPEC_LLM_PROVIDER"))
	model := strings.TrimSpace(os.Getenv("MINDSPEC_LLM_MODEL"))
	if model == "" {
		model = "default"
	}
	if provider == "" || strings.EqualFold(provider, "off") || strings.EqualFold(provider, "none") {
		return LLMConfig{
			Provider:  "off",
			Model:     model,
			Available: false,
		}
	}
	return LLMConfig{
		Provider:  provider,
		Model:     model,
		Available: true,
	}
}

// Run executes deterministic discovery + classification and writes run artifacts.
func Run(root string, opts RunOptions) (*Report, error) {
	report, err := DiscoverMarkdown(root)
	if err != nil {
		return nil, err
	}

	if opts.RunID == "" {
		opts.RunID = time.Now().UTC().Format("20060102T150405Z")
	}
	report.RunID = opts.RunID
	if opts.Apply {
		report.Mode = "apply"
	} else {
		report.Mode = "report-only"
	}
	report.LLM = ResolveLLMConfig()
	report.Classification = classify(report.Inventory)
	report.Unresolved = unresolvedPaths(report.Classification)

	if err := writeRunArtifacts(root, report, opts); err != nil {
		return nil, err
	}

	if !opts.Apply {
		return report, nil
	}

	if len(report.Unresolved) > 0 && !report.LLM.Available {
		return report, fmt.Errorf(
			"brownfield apply blocked: %d low-confidence docs require LLM classification, but no provider is configured (set MINDSPEC_LLM_PROVIDER and MINDSPEC_LLM_MODEL or re-run with --report-only)",
			len(report.Unresolved),
		)
	}
	if len(report.Unresolved) > 0 {
		return report, fmt.Errorf(
			"brownfield apply blocked: %d low-confidence docs still require LLM classification; provider %q is configured but LLM classification calls are not implemented yet",
			len(report.Unresolved),
			report.LLM.Provider,
		)
	}

	return report, fmt.Errorf("brownfield apply is not implemented yet: synthesis/archive stages pending (archive=%s)", opts.ArchiveMode)
}

func classify(entries []InventoryEntry) []ClassificationEntry {
	out := make([]ClassificationEntry, 0, len(entries))
	for _, e := range entries {
		category, confidence, rule := classifyPath(e.Path)
		out = append(out, ClassificationEntry{
			Path:        e.Path,
			SHA256:      e.SHA256,
			Category:    category,
			Confidence:  confidence,
			Rule:        rule,
			RequiresLLM: confidence < 0.70,
		})
	}
	return out
}

func classifyPath(path string) (string, float64, string) {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "/adr/") || strings.HasPrefix(lower, "docs/adr/") || strings.HasPrefix(lower, ".mindspec/docs/adr/"):
		return "adr", 0.98, "path-contains-adr"
	case strings.Contains(lower, "/specs/") || strings.HasSuffix(lower, "/spec.md") || strings.HasSuffix(lower, "/plan.md"):
		return "spec", 0.96, "path-contains-specs"
	case strings.Contains(lower, "/domains/"):
		return "domain", 0.95, "path-contains-domains"
	case strings.Contains(lower, "/core/"):
		return "core", 0.92, "path-contains-core"
	case strings.HasSuffix(lower, "context-map.md"):
		return "context-map", 0.95, "filename-context-map"
	case strings.HasSuffix(lower, "glossary.md"):
		return "glossary", 0.95, "filename-glossary"
	case strings.HasSuffix(lower, "readme.md"), strings.Contains(lower, "/guides/"), strings.Contains(lower, "/user"):
		return "user-docs", 0.80, "path-user-docs-heuristic"
	default:
		return "unknown", 0.40, "no-deterministic-rule"
	}
}

func unresolvedPaths(entries []ClassificationEntry) []string {
	var unresolved []string
	for _, e := range entries {
		if e.RequiresLLM {
			unresolved = append(unresolved, e.Path)
		}
	}
	return unresolved
}

func writeRunArtifacts(root string, report *Report, opts RunOptions) error {
	runDir := filepath.Join(root, ".mindspec", "migrations", report.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create migration run dir: %w", err)
	}

	if err := writeJSON(filepath.Join(runDir, "inventory.json"), report.Inventory); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(runDir, "classification.json"), report.Classification); err != nil {
		return err
	}

	state := struct {
		RunID           string `json:"run_id"`
		Mode            string `json:"mode"`
		ArchiveMode     string `json:"archive_mode,omitempty"`
		LLMProvider     string `json:"llm_provider"`
		LLMModel        string `json:"llm_model"`
		LLMAvailable    bool   `json:"llm_available"`
		UnresolvedCount int    `json:"unresolved_count"`
	}{
		RunID:           report.RunID,
		Mode:            report.Mode,
		ArchiveMode:     opts.ArchiveMode,
		LLMProvider:     report.LLM.Provider,
		LLMModel:        report.LLM.Model,
		LLMAvailable:    report.LLM.Available,
		UnresolvedCount: len(report.Unresolved),
	}
	if err := writeJSON(filepath.Join(runDir, "state.json"), state); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
