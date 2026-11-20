package main

import (
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"strings"
	"testing"
)

func TestS134Final_PanelEvidenceOnSpecBranch(t *testing.T) {
	e := finalKernelFixture(t)
	e.apply()
	e.create(e.implement(0))
	e.review("APPROVE")
	e.ok("complete", e.f.Items[0])
	e.create(e.apply())
	// Both the registration artifacts and the verdicts live on the spec branch.
	e.git(e.cwd, "add", "-f", ".mindspec/specs/001-x/reviews/001-x")
	e.git(e.cwd, "commit", "-qm", "panel registration")
	e.proof = e.cwd
	e.review("APPROVE")
	r := e.row(e.f.Epic + ".final")
	receipt, ok, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	e.must(err)
	if r.Status != "closed" || !ok || receipt.Kind != "passed" {
		t.Fatalf("final review did not pass: %+v", r)
	}
	// Interrupt the real finalizer after its export and recorded publication.
	t.Setenv("MINDSPEC_TEST_FAIL_WORKTREE_REMOVE", "1")
	first, code := e.run("impl", "approve", e.f.SpecID)
	if code == 0 || !strings.Contains(first, "finalize stage cleanup") || !strings.Contains(first, "injected cleanup interruption") || !strings.Contains(first, "recovery: mindspec impl approve "+e.f.SpecID) {
		t.Fatalf("missing cleanup recovery: exit=%d %s", code, first)
	}
	epic := e.row(e.f.Epic)
	before, _ := epic.Metadata["mindspec_pre_finalize_tip"].(string)
	published, _ := epic.Metadata["mindspec_finalize_tip"].(string)
	if before == "" || published == "" || before == published || e.git(e.cwd, "rev-parse", "HEAD") != published {
		t.Fatalf("real finalizer did not record publication: %+v", epic.Metadata)
	}
	t.Setenv("MINDSPEC_TEST_FAIL_WORKTREE_REMOVE", "0")
	again := e.ok("impl", "approve", e.f.SpecID)
	for _, output := range []string{first, again} {
		for _, name := range []string{"takeovers", "decision-answers"} {
			if !strings.Contains(output, fmt.Sprintf("%s | 0", name)) {
				t.Fatalf("audit changed on replay: %s", output)
			}
		}
	}
	if e.row(e.f.Epic).Status != "closed" {
		t.Fatal("approval did not finalize epic")
	}
}
