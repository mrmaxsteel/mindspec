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
	isolationRoot = strings.TrimSuffix(filepath.ToSlash(isolationRoot), "/")
	if spec == "" {
		spec = "adhoc"
	}
	if !filepath.IsAbs(mainRoot) || isolationRoot == "" || filepath.IsAbs(isolationRoot) || filepath.Clean(isolationRoot) == "." {
		return SlotLayout{}, submitRefusal("invalid slot root", "select an absolute main root and a relative isolation root")
	}
	for _, part := range strings.Split(isolationRoot, "/") {
		if !layoutComponent(part) {
			return SlotLayout{}, submitRefusal("invalid isolation root", "select an isolation root within the main root")
		}
	}
	if !layoutComponent(spec) || !layoutComponent(slug) || !layoutComponent(slot) || round < 1 {
		return SlotLayout{}, submitRefusal("invalid slot identity", "select a panel, positive round and roster slot")
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
// Ancestors of the slot do not count as attempt directories. An attempt path
// that cannot bind to the slot is refused. If the slot is absent or cannot
// resolve, only an immediate attempt-<n> parent causes refusal.
func AttemptFromPath(slotDir, file string) (int, error) {
	if file == "" || file == "-" {
		return 0, nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return 0, err
	}
	hasAttempt := func(path string) bool {
		for _, part := range strings.Split(path, string(filepath.Separator)) {
			if attemptNumber(part) > 0 {
				return true
			}
		}
		return false
	}
	unbound := func(hasAttempt bool) (int, error) {
		if hasAttempt {
			return 0, submitRefusal("cannot bind attempt path to the target slot", "submit a file from the target slot's attempt directory")
		}
		return 0, nil
	}
	immediateAttempt := attemptNumber(filepath.Base(filepath.Dir(abs))) > 0
	if slotDir == "" {
		return unbound(immediateAttempt)
	}
	root, err := filepath.EvalSymlinks(slotDir)
	if err != nil {
		return unbound(immediateAttempt)
	}
	slotAbs, err := filepath.Abs(slotDir)
	if err != nil {
		return 0, err
	}
	inside := func(rel string) bool {
		return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	// Keep the slot-local path check if a symlink leaves an attempt directory.
	lexicalAttempt := false
	if rel, err := filepath.Rel(slotAbs, abs); err == nil && inside(rel) {
		lexicalAttempt = hasAttempt(filepath.Dir(rel))
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return unbound(lexicalAttempt || immediateAttempt)
	}
	// Canonicalize directories, since a caller can supply verdict bytes before
	// the verdict file exists. Slot discovery also resolves directory symlinks.
	root, err = filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	abs = filepath.Join(dir, filepath.Base(abs))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return unbound(lexicalAttempt || immediateAttempt)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 1 && attemptNumber(parts[0]) > 0 {
		return attemptNumber(parts[0]), nil
	}
	return unbound(lexicalAttempt || hasAttempt(filepath.Dir(rel)) || (!inside(rel) && immediateAttempt))
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
