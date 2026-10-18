package main

// frozen_contract_guidance_test.go — spec 128 (lifecycle-verb-state-
// honesty) bead 10, AC-12 (R7): idle- and review-mode instruct output
// contain the new-spec-for-frozen-contract guidance and the literal
// invocation `mindspec impl adopt` with `--reason`; a template test pins
// the command name against the REGISTERED cobra command so a future
// rename is red.
//
// The pin is mechanized, not duplicated prose: the expected invocation
// substring is DERIVED from rootCmd's own registered `impl adopt`
// command (its CommandPath()) rather than hardcoded a second time here.
// If `impl.go` ever renames the `adopt` leaf (or moves it under a
// different parent), CommandPath() changes, the derived substring no
// longer matches the templates' hardcoded text, and this test goes red
// — exactly the rename-is-red clause AC-12 requires. A SEPARATE
// assertion pins the `--reason` flag's existence and name against the
// same registered command, so a flag rename is caught too.

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
)

func TestFrozenContractGuidance_ImplAdoptInvocationPinnedToCobraTree(t *testing.T) {
	adoptCmd := findCommandByPath(t, rootCmd, "mindspec impl adopt")

	// The invocation substring every guidance surface must carry, built
	// FROM the live command tree — never a second hardcoded literal.
	invocation := adoptCmd.CommandPath()
	if invocation != "mindspec impl adopt" {
		t.Fatalf("registered command path changed to %q — this test's own\n"+
			"derivation still works, but re-check the templates by hand", invocation)
	}

	reasonFlag := adoptCmd.Flags().Lookup("reason")
	if reasonFlag == nil {
		t.Fatal("registered `mindspec impl adopt` command lost its `reason` flag — AC-12's guidance names --reason explicitly")
	}
	if reasonFlag.Name != "reason" {
		t.Fatalf("reason flag's registered name is %q, want \"reason\"", reasonFlag.Name)
	}

	for _, tc := range []struct {
		mode string
		name string
	}{
		{mode: "idle", name: "idle.md"},
		{mode: "review", name: "review.md"},
	} {
		ctx := &instruct.Context{Mode: tc.mode, ActiveSpec: "999-frozen-contract-demo"}
		out, err := instruct.Render(ctx)
		if err != nil {
			t.Fatalf("rendering %s: %v", tc.name, err)
		}
		if !strings.Contains(out, invocation) {
			t.Errorf("%s does not contain the registered invocation %q:\n%s", tc.name, invocation, out)
		}
		if !strings.Contains(out, "--"+reasonFlag.Name) {
			t.Errorf("%s does not contain the registered --%s flag:\n%s", tc.name, reasonFlag.Name, out)
		}
		if !strings.Contains(out, "new spec") {
			t.Errorf("%s does not state that new implementation work (including a frozen contract) goes through a new spec:\n%s", tc.name, out)
		}
	}
}
