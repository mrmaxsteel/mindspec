package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestBootstrap_ScratchIgnoredExisting(t *testing.T) {
	for _, isolation := range []string{".mindspec-scratch", "review-scratch"} {
		t.Run(isolation, func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
				t.Fatal(err)
			}
			if isolation != ".mindspec-scratch" {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel: {isolation_root: "+isolation+"}\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			seed := "# existing project\n*.log\n"
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0644); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(before), seed) {
				t.Fatal("existing ignore text changed")
			}
			for _, path := range []string{isolation, ".mindspec/config.local.yaml"} {
				if !strings.Contains(string(before), "\n"+path+"\n") {
					t.Fatalf("missing ignore %s: %s", path, before)
				}
				mustGit(t, root, "check-ignore", "--", path+"/probe")
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("second init changed ignores")
			}
		})
	}
}
