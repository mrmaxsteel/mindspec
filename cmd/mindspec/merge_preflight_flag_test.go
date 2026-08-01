package main

// Spec 127 bead 6: internal/executor and internal/lifecycle each declare
// their OWN AllowNetDeletionFlag const (internal/executor may not import
// internal/lifecycle — its own executor/enforcement boundary — so the two
// layers cannot share one symbol). This is the pinning test both
// packages' doc comments promise: cmd/mindspec already imports both, so
// it is the natural home to prove the two independently-declared
// spellings never silently diverge.

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

func TestAllowNetDeletionFlag_ExecutorAndLifecycleAgree(t *testing.T) {
	if executor.AllowNetDeletionFlag != lifecycle.AllowNetDeletionFlag {
		t.Fatalf("executor.AllowNetDeletionFlag=%q != lifecycle.AllowNetDeletionFlag=%q — the two independently-declared spellings have diverged", executor.AllowNetDeletionFlag, lifecycle.AllowNetDeletionFlag)
	}
}
