package kerneltest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// JudgeFault describes the source rewrite and the test that consumes it.
// Callers keep producer-specific source text in their test files.
type JudgeFault struct {
	Package, Test, File, Expression, Replacement string
}

// JudgeRefusalFault runs one refusal-table row with a failed IO operation.
// Only the IO expression is replaced: the production error branch, wrapping,
// recovery and CLI rendering execute unchanged. No runtime fault hook is added.
// It returns true in the child that must execute the row's fixture.
func JudgeRefusalFault(t *testing.T, producer string, fault JudgeFault) bool {
	t.Helper()
	if os.Getenv("S134_JUDGE_FAULT") == producer {
		return true
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate fault fixture source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "../../../.."))
	replacements := map[string]string{}
	// Keep any caller's mutation overlay when this fixture adds its IO fault.
	if name := os.Getenv("S134_SOURCE_OVERLAY"); name != "" {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var inherited struct{ Replace map[string]string }
		if err := json.Unmarshal(data, &inherited); err != nil {
			t.Fatal(err)
		}
		for k, v := range inherited.Replace {
			replacements[k] = v
		}
	}
	target := filepath.Join(root, fault.File)
	source := target
	if replacement, ok := replacements[target]; ok {
		source = replacement
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), fault.Expression) != 1 {
		t.Fatalf("IO expression absent or ambiguous in %s: %s", source, fault.Expression)
	}
	dir := t.TempDir()
	copy := filepath.Join(dir, filepath.Base(fault.File))
	if err := os.WriteFile(copy, []byte(strings.Replace(string(data), fault.Expression, fault.Replacement, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	replacements[target] = copy
	overlay := filepath.Join(dir, "overlay.json")
	data, err = json.Marshal(struct{ Replace map[string]string }{replacements})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "-count=1", fault.Package, "-run", "^"+fault.Test+"$/^"+regexp.QuoteMeta(producer)+"$", "-v")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "S134_JUDGE_FAULT="+producer, "GOFLAGS="+os.Getenv("GOFLAGS")+" -overlay="+overlay, "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fault row %s: %v\n%s", producer, err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+fault.Test+"/"+producer) {
		t.Fatalf("fault row did not run: %s", out)
	}
	t.Logf("%s", out)
	return false
}

// JudgeRefusalText gives the contract for each failed IO operation.
func JudgeRefusalText(producer string) (string, string) {
	switch producer {
	case "judge.changed-paths":
		return "cannot read changed paths: ", "restore the subject commits, then run mindspec next-action"
	case "judge.prior-read":
		return "cannot read prior panel: ", "restore the panel directory and retry"
	case "judge.prior-locate":
		return "cannot locate prior panel: ", "restore the panel directory and retry"
	case "judge.round-read":
		return "cannot read round panel: ", "restore the panel directory and retry"
	}
	panic(fmt.Sprintf("unknown judge refusal %s", producer))
}
