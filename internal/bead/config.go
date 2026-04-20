package bead

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// builtinStatuses lists the statuses bd recognises out of the box. Callers
// that need to iterate every possible bead status combine this with
// CustomStatuses for a complete picture.
var builtinStatuses = []string{"open", "in_progress", "blocked", "closed"}

// BuiltinStatuses returns a fresh copy of the built-in bd status set.
func BuiltinStatuses() []string {
	out := make([]string, len(builtinStatuses))
	copy(out, builtinStatuses)
	return out
}

// CustomStatuses reads `status.custom` from <root>/.beads/config.yaml and
// returns the declared custom statuses. bd accepts either a scalar string
// ("resolved") or a comma-separated list ("resolved,paused"); both are
// normalised into individual trimmed entries here.
//
// An empty slice is returned if the file is missing, malformed, or
// declares no custom statuses — callers should tolerate that gracefully
// and fall back to BuiltinStatuses.
func CustomStatuses(root string) []string {
	path := filepath.Join(root, ".beads", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	raw, ok := cfg["status.custom"]
	if !ok {
		return nil
	}
	return splitCustomList(raw)
}

// AllStatuses returns the union of built-in and custom statuses for the
// project rooted at root. Order is stable: built-ins first, customs in
// declaration order, with case-insensitive dedup.
func AllStatuses(root string) []string {
	all := BuiltinStatuses()
	seen := make(map[string]bool, len(all))
	for _, s := range all {
		seen[strings.ToLower(s)] = true
	}
	for _, s := range CustomStatuses(root) {
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		all = append(all, s)
	}
	return all
}

// splitCustomList accepts either a single string or a []interface{} of
// strings from the YAML decoder and returns a normalised slice.
func splitCustomList(raw interface{}) []string {
	var out []string
	switch v := raw.(type) {
	case string:
		for _, piece := range strings.Split(v, ",") {
			if s := strings.TrimSpace(piece); s != "" {
				out = append(out, s)
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}
