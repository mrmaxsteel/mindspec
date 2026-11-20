package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS134E2E_FinalApprovalPendingSource(t *testing.T) {
	testFinalApprovalPendingSource(t, newKernelE2E(t, kernelE2EBinary(t), 1), "review/evil/x.go")
}

func TestS134Final_PendingSourceBeforeMutation(t *testing.T) {
	testFinalApprovalPendingSource(t, finalKernelFixture(t), "review/evil/x.go", "review/evil/x_test.go", "reviews/evil/x.go", "reviews/evil/x_test.go")
}

func testFinalApprovalPendingSource(t *testing.T, e *kernelE2E, extraModes ...string) {
	t.Helper()
	e.apply()
	step := e.f.Items[0] + ".impl.a1"
	token := e.lease(step)
	head := e.commit(e.tree(e.f.Items[0]), "source.go", "package source\n")
	e.ok("submit", step, "--lease", token, "--commit", head)
	e.create(e.apply())
	e.review("APPROVE")
	e.ok("complete", e.f.Items[0])
	e.create(e.apply())
	e.review("APPROVE")
	reviewed := e.git(e.cwd, "rev-parse", "HEAD")
	main := e.git(e.f.Root, "rev-parse", "main")
	for _, mode := range append([]string{"untracked", "modified", "deleted"}, extraModes...) {
		t.Run(mode, func(t *testing.T) {
			name := "source.go"
			if mode == "untracked" {
				name = "unreviewed.go"
			}
			if strings.Contains(mode, "/") {
				name = mode
			}
			full := filepath.Join(e.cwd, name)
			if mode == "deleted" {
				e.must(os.Rename(full, filepath.Join(t.TempDir(), name)))
			} else {
				e.write(e.cwd, name, "package unreviewed\n")
			}
			before := e.row(e.f.Epic)
			out, code := e.run("impl", "approve", e.f.SpecID)
			if code == 0 || !strings.Contains(out, name) || !strings.Contains(out, "normal workflow") || !strings.Contains(out, "move") {
				t.Fatalf("pending %s source was not refused: exit=%d %s", mode, code, out)
			}
			if e.git(e.cwd, "rev-parse", "HEAD") != reviewed || e.git(e.f.Root, "rev-parse", "main") != main || e.row(e.f.Epic).Status != before.Status {
				t.Fatal("refusal committed, merged, or closed the epic")
			}
			if e.git(e.cwd, "diff", "--cached", "--name-only") != "" {
				t.Fatal("refusal staged pending source")
			}
			if mode == "untracked" || strings.Contains(mode, "/") {
				e.must(os.Rename(full, filepath.Join(t.TempDir(), filepath.Base(name))))
			} else {
				e.write(e.cwd, name, "package source\n")
			}
		})
		if t.Failed() {
			return
		}
	}
	e.ok("impl", "approve", e.f.SpecID)
}
