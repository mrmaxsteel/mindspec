package otel

// setup.go: file-writing wrappers around the pure renderers in
// config.go. Spec 084 Bead 1.
//
// All file I/O lives here, separate from rendering, so the renderers
// remain trivially testable. No network I/O is performed anywhere in
// this file (Hard Constraint #5 — enforced by Bead 4's specgate
// TestNoOtelNetCalls AST check).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SetupResult captures what changed on disk so callers (and tests) can
// assert idempotency and report to the user.
type SetupResult struct {
	// Path is the absolute file path that was written (or would have
	// been written, if NoOp is true).
	Path string

	// Written indicates a real disk mutation occurred.
	Written bool

	// NoOp indicates the on-disk content already matched the rendered
	// output; no write was performed. This is the canonical idempotent
	// path: re-running setup with identical inputs against an already-
	// configured file flips NoOp=true and Written=false.
	NoOp bool

	// PriorContent is the file's content before the write (empty if
	// the file did not exist). Useful for callers that want to diff.
	PriorContent string
}

// WriteClaudeSettingsLocal writes the OTEL configuration into
// <root>/.claude/settings.local.json, preserving any sibling keys.
//
// Idempotency: if the file already contains exactly the rendered
// output, no write is performed and SetupResult.NoOp is true.
func WriteClaudeSettingsLocal(root string, c Config) (SetupResult, error) {
	if err := c.Validate(); err != nil {
		return SetupResult{}, err
	}
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")

	existing, priorBytes, err := readSettingsIfExists(settingsPath)
	if err != nil {
		return SetupResult{}, err
	}

	merged, err := MergeClaudeSettingsLocal(existing, c)
	if err != nil {
		return SetupResult{}, err
	}

	out, err := MarshalClaudeSettings(merged)
	if err != nil {
		return SetupResult{}, err
	}

	res := SetupResult{Path: settingsPath, PriorContent: string(priorBytes)}
	if string(priorBytes) == string(out) {
		res.NoOp = true
		return res, nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return res, fmt.Errorf("otel: creating .claude dir: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return res, fmt.Errorf("otel: writing %s: %w", settingsPath, err)
	}
	res.Written = true
	return res, nil
}

// WriteCodexConfig writes the OTEL configuration into the user's
// ~/.codex/config.toml (or `path` if supplied non-empty).
//
// Parent directory creation: if the parent of `path` is absent, it is
// created with mode 0700 per spec 084 Hard Constraint #5 (Codex
// directory is treated as security-sensitive because it may contain
// the user's exporter headers including bearer tokens).
//
// Returns:
//   - SetupResult{Written:true} on a successful mutation.
//   - SetupResult{NoOp:true} when the on-disk file already matches
//     the rendered output (sha256-idempotent re-run).
//   - An error with exit-class "malformed" when the pre-existing TOML
//     fails the merge regex (caller maps to exit code 1).
func WriteCodexConfig(path string, c Config) (SetupResult, error) {
	if err := c.Validate(); err != nil {
		return SetupResult{}, err
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return SetupResult{}, fmt.Errorf("otel: resolving home dir: %w", err)
		}
		path = filepath.Join(home, ".codex", "config.toml")
	}

	existing := ""
	priorBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return SetupResult{}, fmt.Errorf("otel: reading %s: %w", path, err)
	}
	if err == nil {
		existing = string(priorBytes)
	}

	rendered, err := RenderCodexConfigToml(c, existing)
	if err != nil {
		return SetupResult{}, err
	}

	res := SetupResult{Path: path, PriorContent: existing}
	if existing == rendered {
		res.NoOp = true
		return res, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return res, fmt.Errorf("otel: creating codex config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
		return res, fmt.Errorf("otel: writing %s: %w", path, err)
	}
	res.Written = true
	return res, nil
}

// readSettingsIfExists reads a JSON settings file. Returns (nil, nil,
// nil) if the file does not exist; (nil, nil, err) on read/parse
// failure other than not-exist; (parsed, raw, nil) on success.
func readSettingsIfExists(path string) (map[string]any, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("otel: reading %s: %w", path, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, raw, fmt.Errorf("otel: parsing %s: %w", path, err)
	}
	return parsed, raw, nil
}
