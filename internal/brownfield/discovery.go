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
	Resume      bool
}

const (
	stageClassified = "classified"
	stageApplying   = "applying"
	stageApplied    = "applied"
)

type migrationState struct {
	RunID           string `json:"run_id"`
	Mode            string `json:"mode"`
	ArchiveMode     string `json:"archive_mode,omitempty"`
	LLMProvider     string `json:"llm_provider"`
	LLMModel        string `json:"llm_model"`
	LLMAvailable    bool   `json:"llm_available"`
	UnresolvedCount int    `json:"unresolved_count"`
	Stage           string `json:"stage"`
	Resumed         bool   `json:"resumed"`
	UpdatedAt       string `json:"updated_at"`
}

// Report captures deterministic brownfield discovery output.
type Report struct {
	RunID          string                `json:"run_id"`
	Mode           string                `json:"mode"`
	MarkdownFiles  []string              `json:"markdown_files"`
	Inventory      []InventoryEntry      `json:"inventory"`
	Classification []ClassificationEntry `json:"classification"`
	Lineage        []LineageEntry        `json:"lineage,omitempty"`
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
			if name == ".git" || name == ".beads" || name == ".claude" || name == "docs_archive" {
				return filepath.SkipDir
			}
			if name == "migrations" && filepath.Base(filepath.Dir(path)) == ".mindspec" {
				return filepath.SkipDir
			}
			if name == "docs" && filepath.Base(filepath.Dir(path)) == ".mindspec" {
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
		if strings.HasPrefix(strings.ToLower(rel), "internal/instruct/templates/") {
			return nil
		}
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
	if opts.Resume && opts.RunID == "" {
		return nil, fmt.Errorf("--resume requires a run ID")
	}
	if opts.RunID == "" {
		opts.RunID = time.Now().UTC().Format("20060102T150405Z")
	}

	report, stage, resumed, err := loadResumeArtifacts(root, opts.RunID, opts.Resume)
	if err != nil {
		return nil, err
	}
	if !resumed {
		report, err = DiscoverMarkdown(root)
		if err != nil {
			return nil, err
		}
		report.Classification = classify(report.Inventory)
		stage = stageClassified
	}
	if stage == "" {
		stage = stageClassified
	}

	report.RunID = opts.RunID
	if opts.Apply {
		report.Mode = "apply"
	} else {
		report.Mode = "report-only"
	}
	report.LLM = ResolveLLMConfig()
	report.Unresolved = unresolvedPaths(report.Classification)

	if err := writeRunArtifacts(root, report, opts, stage, resumed); err != nil {
		return nil, err
	}

	if !opts.Apply {
		return report, nil
	}
	if stage == stageApplied {
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

	if err := writeRunState(root, report, opts, stageApplying, resumed); err != nil {
		return nil, err
	}
	if err := applyTransactional(root, report, opts); err != nil {
		return report, err
	}
	if err := writeRunState(root, report, opts, stageApplied, resumed); err != nil {
		return nil, err
	}
	return report, nil
}

func loadResumeArtifacts(root, runID string, strict bool) (*Report, string, bool, error) {
	runDir := runDir(root, runID)
	inventoryPath := filepath.Join(runDir, "inventory.json")
	classificationPath := filepath.Join(runDir, "classification.json")
	statePath := filepath.Join(runDir, "state.json")

	stage := ""
	stateExists := false
	if _, err := os.Stat(statePath); err == nil {
		stateExists = true
	} else if !os.IsNotExist(err) {
		return nil, "", false, fmt.Errorf("stat resume state: %w", err)
	}
	if strict && !stateExists {
		return nil, "", false, fmt.Errorf("--resume %s requested but state.json is missing", runID)
	}
	if stateExists {
		var state migrationState
		if err := readJSON(statePath, &state); err != nil {
			return nil, "", false, fmt.Errorf("load resume state: %w", err)
		}
		stage = state.Stage
	}

	if _, err := os.Stat(inventoryPath); os.IsNotExist(err) {
		if strict {
			return nil, "", false, fmt.Errorf("--resume %s requested but inventory.json is missing", runID)
		}
		return nil, stage, false, nil
	} else if err != nil {
		return nil, "", false, fmt.Errorf("stat resume inventory: %w", err)
	}
	if _, err := os.Stat(classificationPath); os.IsNotExist(err) {
		if strict {
			return nil, "", false, fmt.Errorf("--resume %s requested but classification.json is missing", runID)
		}
		return nil, stage, false, nil
	} else if err != nil {
		return nil, "", false, fmt.Errorf("stat resume classification: %w", err)
	}

	var (
		inventory      []InventoryEntry
		classification []ClassificationEntry
	)
	if err := readJSON(inventoryPath, &inventory); err != nil {
		return nil, "", false, fmt.Errorf("load resume inventory: %w", err)
	}
	if err := readJSON(classificationPath, &classification); err != nil {
		return nil, "", false, fmt.Errorf("load resume classification: %w", err)
	}

	files := make([]string, 0, len(inventory))
	for _, inv := range inventory {
		files = append(files, inv.Path)
	}
	sort.Strings(files)
	sort.Slice(inventory, func(i, j int) bool { return inventory[i].Path < inventory[j].Path })
	sort.Slice(classification, func(i, j int) bool { return classification[i].Path < classification[j].Path })

	return &Report{
		MarkdownFiles:  files,
		Inventory:      inventory,
		Classification: classification,
	}, stage, true, nil
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
	case strings.Contains(lower, "/templates/"):
		return "user-docs", 0.90, "path-contains-templates"
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
	case lower == "agents.md", lower == "claude.md", strings.HasPrefix(lower, "docs/archive/"), strings.HasPrefix(lower, "docs/roadmap.md"):
		return "user-docs", 0.85, "path-user-docs-operational"
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

func writeRunArtifacts(root string, report *Report, opts RunOptions, stage string, resumed bool) error {
	if err := os.MkdirAll(runDir(root, report.RunID), 0o755); err != nil {
		return fmt.Errorf("create migration run dir: %w", err)
	}

	runPath := runDir(root, report.RunID)
	if err := writeJSON(filepath.Join(runPath, "inventory.json"), report.Inventory); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(runPath, "classification.json"), report.Classification); err != nil {
		return err
	}

	return writeRunState(root, report, opts, stage, resumed)
}

func writeRunState(root string, report *Report, opts RunOptions, stage string, resumed bool) error {
	if stage == "" {
		stage = stageClassified
	}
	state := migrationState{
		RunID:           report.RunID,
		Mode:            report.Mode,
		ArchiveMode:     opts.ArchiveMode,
		LLMProvider:     report.LLM.Provider,
		LLMModel:        report.LLM.Model,
		LLMAvailable:    report.LLM.Available,
		UnresolvedCount: len(report.Unresolved),
		Stage:           stage,
		Resumed:         resumed,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	return writeJSON(filepath.Join(runDir(root, report.RunID), "state.json"), state)
}

func runDir(root, runID string) string {
	return filepath.Join(root, ".mindspec", "migrations", runID)
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

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
