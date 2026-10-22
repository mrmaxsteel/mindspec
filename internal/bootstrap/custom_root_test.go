package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func TestBootstrap_CustomRootIgnored(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom"}[custom], func(t *testing.T) {
			root := reviewBootstrapRepo(t)
			isolation := ".mindspec-scratch"
			if custom {
				isolation = "review-scratch"
			}
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
				t.Fatal(err)
			}
			if custom {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel: {isolation_root: review-scratch}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			seed := "# existing ignore\n*.log\n"
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(seed), 0o644); err != nil {
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
			if !strings.HasPrefix(string(before), seed) || !strings.Contains(string(before), "\n"+isolation+"\n") {
				t.Fatalf("ignore contents: %s", before)
			}
			if err := os.MkdirAll(filepath.Join(root, isolation), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := (&executor.MindspecExecutor{Root: root}).CheckIgnore(root, isolation); err != nil {
				t.Fatalf("checkout ignore precondition: %v", err)
			}
			if _, err := Run(root, false); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("init rerun changed custom root ignore")
			}
		})
	}
}
