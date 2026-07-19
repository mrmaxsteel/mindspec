package main

import (
	"os"
	"strings"
	"testing"
)

// TestImplApproveRejectsInvalidSpecIDArg is spec 120 AC-7 (R3 specID
// ingress): a hostile args[0] produces the early clean CLI refusal
// BEFORE any SpecWorktreePath composition or os.Chdir — asserted here by
// confirming the process cwd is unchanged after the call (the early gate
// fires before findRoot/os.Chdir ever run).
func TestImplApproveRejectsInvalidSpecIDArg(t *testing.T) {
	cwdBefore, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	hostileIDs := []string{
		"x;evil",
		"--help",
		"x\x00\x1b[31m\nrecovery: forged",
	}
	for _, hostile := range hostileIDs {
		err := approveImplRunE(implApproveCmd, []string{hostile})
		if err == nil {
			t.Errorf("approveImplRunE(%q) accepted a hostile spec ID", hostile)
			continue
		}
		if !strings.Contains(err.Error(), "mindspec spec list") {
			t.Errorf("approveImplRunE(%q) error must name the `mindspec spec list` lever, got: %v", hostile, err)
		}
		cwdAfter, cwdErr := os.Getwd()
		if cwdErr != nil {
			t.Fatalf("os.Getwd after call: %v", cwdErr)
		}
		if cwdAfter != cwdBefore {
			t.Errorf("approveImplRunE(%q): cwd changed (%q -> %q) — the early gate must refuse BEFORE any os.Chdir", hostile, cwdBefore, cwdAfter)
		}
	}
}
