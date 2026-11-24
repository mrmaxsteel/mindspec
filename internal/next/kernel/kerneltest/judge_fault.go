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

// JudgeRefusalFault runs one refusal-table row with a failed IO operation.
// Only the IO expression is replaced: the production error branch, wrapping,
// recovery and CLI rendering execute unchanged. No runtime fault hook is added.
// It returns true in the child that must execute the row's fixture.
func JudgeRefusalFault(t *testing.T, pkg, producer string) bool {
	t.Helper()
	if os.Getenv("S134_JUDGE_FAULT") == producer {
		return true
	}
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate fault fixture source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "../../../.."))
	file, expression, fault := "judge_round.go", "", ""
	cause := `fmt.Errorf("hostile\nrecovery: forged\x1b")`
	switch producer {
	case "judge.changed-paths":
		file = "judge.go"
		expression = `actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", old, newest, "--")`
		fault = `func() ([]byte, error) { return nil, ` + cause + ` }()`
	case "judge.prior-read":
		expression = `os.ReadFile(filepath.Join(res.Dir, panel.FileName))`
		fault = `func() ([]byte, error) { _ = os.ReadFile; return nil, ` + cause + ` }()`
	case "judge.prior-locate":
		expression = `filepath.Rel(root, filepath.Join(res.Dir, panel.FileName))`
		fault = `func(string, string) (string, error) { return "", ` + cause + ` }(root, filepath.Join(res.Dir, panel.FileName))`
	case "judge.round-read":
		expression = `panel.Tally(p.Dir)`
		fault = `func() (*panel.Result, error) { return nil, ` + cause + ` }()`
	default:
		t.Fatalf("unknown judge fault %s", producer)
	}
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
	target := filepath.Join(root, "internal/next/kernel", file)
	source := target
	if replacement, ok := replacements[target]; ok {
		source = replacement
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), expression) != 1 {
		t.Fatalf("IO expression absent or ambiguous in %s: %s", source, expression)
	}
	dir := t.TempDir()
	copy := filepath.Join(dir, file)
	if err := os.WriteFile(copy, []byte(strings.Replace(string(data), expression, fault, 1)), 0600); err != nil {
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
	name := "TestS134Submit_RefusalProducers"
	if pkg == "./cmd/mindspec" {
		name = "TestS134SubmitCmd_RefusalProducers"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "-count=1", pkg, "-run", "^"+name+"$/^"+regexp.QuoteMeta(producer)+"$", "-v")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "S134_JUDGE_FAULT="+producer, "GOFLAGS="+os.Getenv("GOFLAGS")+" -overlay="+overlay, "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fault row %s: %v\n%s", producer, err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+name+"/"+producer) {
		t.Fatalf("fault row did not run: %s", out)
	}
	t.Logf("%s", out)
	return false
}

// JudgeRefusalText gives the contract for each failed IO operation.
func JudgeRefusalText(producer string) (string, string) {
	switch producer {
	case "judge.changed-paths":
		return "cannot read changed paths: ", "restore the subject commits and retry submit"
	case "judge.prior-read":
		return "cannot read prior panel: ", "restore the panel directory and retry"
	case "judge.prior-locate":
		return "cannot locate prior panel: ", "restore the panel directory and retry"
	case "judge.round-read":
		return "cannot read round panel: ", "restore the panel directory and retry"
	}
	panic(fmt.Sprintf("unknown judge refusal %s", producer))
}
