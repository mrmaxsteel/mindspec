package main

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func judgeRefusalFault(t *testing.T, producer string) bool {
	t.Helper()
	fault := kerneltest.JudgeFault{Package: "./cmd/mindspec", Test: "TestS134SubmitCmd_RefusalProducers", File: "internal/next/kernel/judge_round.go"}
	cause := `fmt.Errorf("hostile\nrecovery: forged\x1b")`
	switch producer {
	case "judge.changed-paths":
		fault.File = "internal/next/kernel/judge.go"
		fault.Expression = `actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", old, newest, "--")`
		fault.Replacement = `func() ([]byte, error) { return nil, ` + cause + ` }()`
	case "judge.prior-read":
		fault.Expression = `os.ReadFile(filepath.Join(res.Dir, panel.FileName))`
		fault.Replacement = `func() ([]byte, error) { _ = os.ReadFile; return nil, ` + cause + ` }()`
	case "judge.prior-locate":
		fault.Expression = `filepath.Rel(root, filepath.Join(res.Dir, panel.FileName))`
		fault.Replacement = `func(string, string) (string, error) { return "", ` + cause + ` }(root, filepath.Join(res.Dir, panel.FileName))`
	case "judge.round-read":
		fault.Expression = `TallyPanel(s, p)`
		fault.Replacement = `func() (*panel.Result, error) { return nil, ` + cause + ` }()`
	default:
		t.Fatalf("unknown judge fault %s", producer)
	}
	return kerneltest.JudgeRefusalFault(t, producer, fault)
}
