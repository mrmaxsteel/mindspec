package complete

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func TestLedger_GlobalIgnoreRefuses(t *testing.T) {
	root := t.TempDir()
	excludes := filepath.Join(t.TempDir(), "excludes")
	global := filepath.Join(t.TempDir(), "gitconfig")
	for path, content := range map[string]string{
		excludes: "dispositions.jsonl\n",
		global:   "[core]\nexcludesFile = " + strconv.Quote(excludes) + "\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	cmd := exec.Command("git", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", root, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init fixture: %v %s", err, out)
	}
	file := "dispositions.jsonl"
	if err := os.WriteFile(filepath.Join(root, file), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	realExec := &executor.MindspecExecutor{Root: root}
	if err := realExec.CheckSlotIgnore(root, file); err == nil {
		t.Fatal("slot ignore check read the global excludes file")
	}
	err := CheckPanelLedgers(root, []string{file}, realExec)
	if err == nil || !strings.Contains(err.Error(), "panel ledger dispositions.jsonl is ignored") {
		t.Fatalf("ledger preflight did not refuse the global ignore: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	if err := CheckPanelLedgers(root, []string{file}, realExec); err != nil {
		t.Fatalf("ledger without the global ignore was refused: %v", err)
	}
}
