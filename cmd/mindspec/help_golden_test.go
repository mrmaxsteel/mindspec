package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestHelpGolden enforces spec 083 Hard Constraint #5 ("CLI surface
// unchanged on the user side") for the three interactive AgentMind
// commands rewired into cobra re-exec wrappers in Bead 4. Each
// golden file at `testdata/help_golden/{serve,replay,viz}.txt`
// captures the exact `--help` output recorded BEFORE the Bead 4
// rewire. This test diffs the live `--help` output against the
// golden; any drift fails the test and forces the author to either
// (a) revert the surface change or (b) consciously regenerate the
// golden in the same commit and explain why in the commit message.
//
// The mindspec binary is built into a temp dir per t.TempDir(), so
// the test is hermetic against a stale `./bin/mindspec` on disk.
// The golden files live under `testdata/` which `go test` excludes
// from the package proper (per the testdata directory convention).
func TestHelpGolden(t *testing.T) {
	bin := buildMindspecBinary(t)

	cases := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "agentmind serve --help",
			args:       []string{"agentmind", "serve", "--help"},
			goldenFile: "serve.txt",
		},
		{
			name:       "agentmind replay --help",
			args:       []string{"agentmind", "replay", "--help"},
			goldenFile: "replay.txt",
		},
		{
			name:       "viz --help",
			args:       []string{"viz", "--help"},
			goldenFile: "viz.txt",
		},
	}

	repoRoot := repoRootFromTestDir(t)
	goldenDir := filepath.Join(repoRoot, "cmd", "mindspec", "testdata", "help_golden")

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(bin, tc.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: exec failed: %v\nstderr=%q", tc.name, err, stderr.String())
			}

			got := stdout.Bytes()

			goldenPath := filepath.Join(goldenDir, tc.goldenFile)
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("help output drifted for %q\n--- want (golden %s) ---\n%s\n--- got ---\n%s\n--- end ---\nIf this drift is intentional, regenerate via:\n  go build -o /tmp/ms ./cmd/mindspec && /tmp/ms %s > %s\nand justify the CLI surface change in the commit message per spec 083 HC#5.",
					tc.name, goldenPath, string(want), string(got),
					joinArgs(tc.args), goldenPath)
			}
		})
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
