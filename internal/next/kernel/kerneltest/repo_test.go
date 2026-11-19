package kerneltest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestS134Probe_PinnedVersion(t *testing.T) {
	for output, want := range map[string]bool{"bd version 1.3.4": true, "bd version 1.3.4 (release)": true, "bd version 1.3.40": false, "bd version 1.3.3": false, "bd version 1.3.4-dev": false} {
		if got := pinnedVersion(output); got != want {
			t.Errorf("version %q accepted=%t want=%t", output, got, want)
		}
	}
}

func TestS134Fixture_Isolation(t *testing.T) {
	outer := gitRepo(t)
	// A missing path exposes accidental remote discovery.
	poison := "sync.remote: " + filepath.Join(outer, "missing-remote") + "\n"
	write(t, filepath.Join(outer, ".beads", "config.yaml"), []byte(poison))
	home := t.TempDir()
	write(t, filepath.Join(home, ".config", "beads", "config.yaml"), []byte(poison))
	write(t, filepath.Join(home, ".beads", "config.yaml"), []byte(poison))
	write(t, filepath.Join(home, ".gitconfig"), []byte("[init]\n templateDir = /missing-kernel-template\n"))
	tmp := filepath.Join(outer, "tmp")
	if err := os.Mkdir(tmp, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("BEADS_DIR", filepath.Join(outer, ".beads"))
	t.Setenv("BEADS_SYNC_REMOTE", filepath.Join(outer, "missing-env-remote"))
	root := NewRepo(t)
	if _, err := Runner(t, root)("list", "--all", "--json"); err != nil {
		t.Fatal(err)
	}
}
