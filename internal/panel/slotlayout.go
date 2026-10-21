package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SlotLayout contains absolute scratch paths. Lock files are outside slot trees.
type SlotLayout struct {
	Dir      string
	Metadata string
	Tree     string
	Lock     string
}
type SlotMetadata struct {
	Spec     string `json:"spec"`
	Slug     string `json:"slug"`
	Round    int    `json:"round"`
	Slot     string `json:"slot"`
	MainRoot string `json:"main_root"`
}

func layoutComponent(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
}
func NewSlotLayout(mainRoot, isolationRoot, spec, slug string, round int, slot string) (SlotLayout, error) {
	if spec == "" {
		spec = "adhoc"
	}
	if !filepath.IsAbs(mainRoot) || isolationRoot == "" || filepath.IsAbs(isolationRoot) || filepath.Clean(isolationRoot) == "." {
		return SlotLayout{}, submitRefusal("invalid slot root")
	}
	for _, part := range strings.Split(filepath.ToSlash(isolationRoot), "/") {
		if !layoutComponent(part) {
			return SlotLayout{}, submitRefusal("invalid isolation root")
		}
	}
	if !layoutComponent(spec) || !layoutComponent(slug) || !layoutComponent(slot) || round < 1 {
		return SlotLayout{}, submitRefusal("invalid slot identity")
	}
	base := filepath.Join(mainRoot, isolationRoot)
	suffix := filepath.Join(spec, slug, fmt.Sprintf("r%d", round))
	dir := filepath.Join(base, suffix, slot)
	return SlotLayout{Dir: dir, Metadata: filepath.Join(dir, "slot.json"), Tree: filepath.Join(dir, "tree"), Lock: filepath.Join(base, ".locks", suffix, slot+".lock")}, nil
}
func (s SlotLayout) AttemptDir(k int) string {
	return filepath.Join(s.Dir, fmt.Sprintf("attempt-%d", k))
}
func WriteSlotMetadata(layout SlotLayout, m SlotMetadata) error {
	if m.Spec == "" {
		m.Spec = "adhoc"
	}
	if err := os.MkdirAll(layout.Dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(layout.Metadata, append(raw, '\n'), 0o600)
}
func attemptNumber(name string) int {
	if !strings.HasPrefix(name, "attempt-") {
		return 0
	}
	value := strings.TrimPrefix(name, "attempt-")
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || strconv.Itoa(n) != value {
		return 0
	}
	return n
}

// AttemptFromPath binds only files under the target slot's attempt directory.
func AttemptFromPath(slotDir, file string) (int, error) {
	if file == "" || file == "-" {
		return 0, nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return 0, err
	}
	if slotDir != "" {
		root, err := filepath.Abs(slotDir)
		if err != nil {
			return 0, err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return 0, err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) > 1 {
			return attemptNumber(parts[0]), nil
		}
		return 0, nil
	}
	// Without a verified target slot directory, an arbitrary attempt-named
	// ancestor does not establish attempt ownership.
	return 0, nil
}

// NextAttemptNumber inspects this slot directory and this round only.
func NextAttemptNumber(dir string, round int, slot, slotDir string) (int, error) {
	attempts, err := SlotAttempts(dir, round)
	if err != nil {
		return 0, err
	}
	high := 0
	for _, e := range attempts[slot] {
		if e.Attempt > high {
			high = e.Attempt
		}
	}
	entries, err := os.ReadDir(slotDir)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	for _, entry := range entries {
		if k := attemptNumber(entry.Name()); entry.IsDir() && k > high {
			high = k
		}
	}
	return high + 1, nil
}
