package guard

import (
	"strings"
	"testing"
)

func TestFormatFailure_SingleCommand(t *testing.T) {
	t.Parallel()
	got := FormatFailure("blocked: tree is dirty", "git add -A && git commit")
	want := "blocked: tree is dirty\nrecovery: git add -A && git commit"
	if got != want {
		t.Errorf("FormatFailure mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatFailure_MultipleCommandsOnePerLine(t *testing.T) {
	t.Parallel()
	got := FormatFailure("merge conflict in spec worktree",
		"cd /repo/.worktrees/worktree-spec-001-x",
		"git merge --continue",
	)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if lines[1] != "recovery: cd /repo/.worktrees/worktree-spec-001-x" {
		t.Errorf("line 1 = %q", lines[1])
	}
	if lines[2] != "recovery: git merge --continue" {
		t.Errorf("line 2 = %q", lines[2])
	}
}

func TestFormatFailure_FinalLineIsRecoveryLine(t *testing.T) {
	t.Parallel()
	got := FormatFailure("msg with trailing newlines\n\n", "mindspec repair phase 092-x")
	if !HasFinalRecoveryLine(got) {
		t.Errorf("expected final recovery line, got: %q", got)
	}
	lines := strings.Split(got, "\n")
	last := lines[len(lines)-1]
	if last != "recovery: mindspec repair phase 092-x" {
		t.Errorf("final line = %q", last)
	}
}

func TestFormatFailure_MachineGreppable(t *testing.T) {
	t.Parallel()
	got := FormatFailure("guard failed", "mindspec complete bead-1", "mindspec impl approve 092-x")
	var recovered []string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, RecoveryPrefix) {
			recovered = append(recovered, strings.TrimPrefix(line, RecoveryPrefix))
		}
	}
	if len(recovered) != 2 || recovered[0] != "mindspec complete bead-1" || recovered[1] != "mindspec impl approve 092-x" {
		t.Errorf("grep extraction mismatch: %#v", recovered)
	}
}

func TestNewFailure_ErrorMessageMatchesFormatFailure(t *testing.T) {
	t.Parallel()
	err := NewFailure("blocked", "cd /repo")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if got, want := err.Error(), FormatFailure("blocked", "cd /repo"); got != want {
		t.Errorf("NewFailure = %q, want %q", got, want)
	}
}

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func TestFormatFailure_PanicsOnProgrammerError(t *testing.T) {
	t.Parallel()
	mustPanic(t, "no commands", func() {
		FormatFailure("guard failed with no recovery")
	})
	mustPanic(t, "empty command", func() {
		FormatFailure("guard failed", "   ")
	})
	mustPanic(t, "multi-line command", func() {
		FormatFailure("guard failed", "cd /repo\nrm -rf /")
	})
}

// Req 19 / HC-5: raw `bd update --metadata` has replace semantics over
// the entire metadata map and can never be emitted through the helper.
func TestFormatFailure_BansRawMetadataUpdate(t *testing.T) {
	t.Parallel()
	banned := []string{
		`bd update mindspec-abc --metadata '{"mindspec_phase":"review"}'`,
		`bd update --metadata '{}' mindspec-abc`,
	}
	for _, command := range banned {
		mustPanic(t, command, func() {
			FormatFailure("phase gate failed", command)
		})
	}
	// The sanctioned merge-semantics alternative passes through.
	got := FormatFailure("phase gate failed", "mindspec repair phase 092-agent-contract-hardening")
	if !strings.HasSuffix(got, "recovery: mindspec repair phase 092-agent-contract-hardening") {
		t.Errorf("sanctioned command rejected: %q", got)
	}
}

func TestIsBannedRecoveryCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		command string
		banned  bool
	}{
		{`bd update mindspec-abc --metadata '{"k":"v"}'`, true},
		{`bd update --metadata '{}'`, true},
		{`bd update mindspec-abc --claim`, false},
		{`mindspec repair phase 092-x`, false},
		{`cd /repo`, false},
	}
	for _, tc := range cases {
		if got := IsBannedRecoveryCommand(tc.command); got != tc.banned {
			t.Errorf("IsBannedRecoveryCommand(%q) = %v, want %v", tc.command, got, tc.banned)
		}
	}
}

func TestHasFinalRecoveryLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"single recovery line", "recovery: cd /repo", true},
		{"message then recovery", "blocked\nrecovery: cd /repo", true},
		{"trailing newline tolerated", "blocked\nrecovery: cd /repo\n", true},
		{"no recovery line", "blocked: tree is dirty", false},
		{"recovery not final", "recovery: cd /repo\nblocked", false},
		{"empty recovery command", "blocked\nrecovery:   ", false},
		{"empty message", "", false},
		{"prefix without space", "blocked\nrecovery:cd /repo", false},
	}
	for _, tc := range cases {
		if got := HasFinalRecoveryLine(tc.msg); got != tc.want {
			t.Errorf("%s: HasFinalRecoveryLine(%q) = %v, want %v", tc.name, tc.msg, got, tc.want)
		}
	}
}
