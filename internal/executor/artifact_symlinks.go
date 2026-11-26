package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

// checkArtifactSymlinks refuses pending symlinks before the artifact commit.
func checkArtifactSymlinks(root string) error {
	status, err := gitutil.StatusZWithStderr(root)
	if err != nil {
		return err
	}
	entries := strings.Split(status, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		name := entry[3:]
		if strings.ContainsAny(entry[:2], "RC") {
			i++
		}
		current := root
		for _, part := range strings.Split(filepath.FromSlash(name), string(filepath.Separator)) {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("cannot commit remaining spec artifacts: symlink %s", name)
			}
		}
	}
	return nil
}
