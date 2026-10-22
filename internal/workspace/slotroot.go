package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// SlotRoot is verified slot metadata and its exact directory.
type SlotRoot struct {
	Spec     string `json:"spec"`
	Slug     string `json:"slug"`
	Round    int    `json:"round"`
	Slot     string `json:"slot"`
	MainRoot string `json:"main_root"`
	Dir      string `json:"-"`
}

func slotRootComponent(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
}

// FindSlotRoot finds the nearest ancestor with both markers and verifies its
// full path against the main root's current isolation configuration.
func FindSlotRoot(cwd string, isolationRoot func(string) (string, error)) (SlotRoot, bool, error) {
	var slot SlotRoot
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return slot, false, err
	}
	// Resolve the host's temp-directory aliases and other symlink spellings.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	for {
		metadata := filepath.Join(dir, "slot.json")
		tree := filepath.Join(dir, "tree")
		_, metaErr := os.Lstat(metadata)
		treeInfo, treeErr := os.Stat(tree)
		if metaErr == nil && treeErr == nil && treeInfo.IsDir() {
			refuse := func() (SlotRoot, bool, error) {
				return SlotRoot{}, true, fmt.Errorf("invalid slot directory %s\nrecovery: use mindspec panel checkout to create the slot directory", termsafe.Escape(dir))
			}
			raw, err := ReadSlotMetadata(metadata)
			if err != nil || json.Unmarshal(raw, &slot) != nil {
				return refuse()
			}
			if !filepath.IsAbs(slot.MainRoot) || !slotRootComponent(slot.Spec) || !slotRootComponent(slot.Slug) || !slotRootComponent(slot.Slot) || slot.Round < 1 {
				return refuse()
			}
			if isolationRoot == nil {
				return refuse()
			}
			root, err := isolationRoot(slot.MainRoot)
			if err != nil {
				return slot, true, err
			}
			if root == "" || filepath.IsAbs(root) || filepath.Clean(root) == "." {
				return refuse()
			}
			for _, part := range strings.Split(filepath.ToSlash(root), "/") {
				if !slotRootComponent(part) {
					return refuse()
				}
			}
			expected := filepath.Join(slot.MainRoot, root, slot.Spec, slot.Slug, "r"+strconv.Itoa(slot.Round), slot.Slot)
			if real, err := filepath.EvalSymlinks(expected); err == nil {
				expected = real
			}
			if dir != filepath.Clean(expected) {
				return refuse()
			}
			slot.Dir = dir
			return slot, true, nil
		}
		if metaErr != nil && !os.IsNotExist(metaErr) {
			return slot, false, metaErr
		}
		if treeErr != nil && !os.IsNotExist(treeErr) {
			return slot, false, treeErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return SlotRoot{}, false, nil
		}
		dir = parent
	}
}
