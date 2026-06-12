package hook

import "testing"

// TestMatchMindspecComplete is the Spec 093 Req 9 / S3-6 anchored-match
// table: legit command-position forms match; quoted mentions and
// non-command-position mentions never match (false positives are the pinned
// bug class).
func TestMatchMindspecComplete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// --- legit forms (MUST match) ---
		{"bare", "mindspec complete mindspec-bd01", true},
		{"cd-prefix", "cd wt && mindspec complete mindspec-bd01", true},
		{"cd-abs-prefix", "cd /repo/.worktrees/wt && mindspec complete X", true},
		{"env-prefix", "FOO=1 mindspec complete X", true},
		{"multi-env-prefix", "FOO=1 BAR=2 mindspec complete X", true},
		{"after-and", "echo hi && mindspec complete X", true},
		{"after-semicolon", "echo hi; mindspec complete X", true},
		{"after-or", "false || mindspec complete X", true},
		{"after-pipe", "true | mindspec complete X", true},
		{"path-binary", "/usr/local/bin/mindspec complete X", true},
		{"cd-and-env", "cd wt && FOO=1 mindspec complete X", true},
		{"newline-separated", "echo a\nmindspec complete X", true},
		{"command-subst-dollar", "echo $(mindspec complete X)", true},

		// --- quoted / non-command-position mentions (MUST NOT match) ---
		{"commit-msg-double-quote", `git commit -m "panel approved; mindspec complete next"`, false},
		{"grep-single-quote", `grep 'mindspec complete' SKILL.md`, false},
		{"echoed-panel-state", `echo "run mindspec complete <id>"`, false},
		{"as-argument-not-command", "echo mindspec complete X", false},
		{"flag-then-text", "ls --help mindspec complete", false},
		{"different-subcommand", "mindspec next", false},
		{"mindspec-without-complete", "mindspec status", false},
		{"complete-without-mindspec", "complete mindspec-bd01", false},
		{"substring-binary", "notmindspec complete X", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMindspecComplete(tt.command); got != tt.want {
				t.Errorf("matchMindspecComplete(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestCompleteBeadID extracts the bead-id argument (first non-flag token
// after `complete`); bare `complete` yields "" (Req 10).
func TestCompleteBeadID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		command string
		want    string
	}{
		{"mindspec complete mindspec-bd01", "mindspec-bd01"},
		{"cd wt && mindspec complete mindspec-bd01 \"done\"", "mindspec-bd01"},
		{"mindspec complete --spec 093 mindspec-bd01", "mindspec-bd01"},
		{"FOO=1 mindspec complete X", "X"},
		{"mindspec complete", ""},
		{"mindspec complete --dry-run", ""},
		{"echo 'mindspec complete X'", ""},
	}
	for _, tt := range tests {
		if got := completeBeadID(tt.command); got != tt.want {
			t.Errorf("completeBeadID(%q) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

// TestCdPrefixPath resolves the cd target of the complete segment against
// the session cwd (Req 10 scan-root (a)).
func TestCdPrefixPath(t *testing.T) {
	t.Parallel()
	if got := cdPrefixPath("cd wt && mindspec complete X", "/session"); got != "/session/wt" {
		t.Errorf("relative cd: got %q", got)
	}
	if got := cdPrefixPath("cd /abs/wt && mindspec complete X", "/session"); got != "/abs/wt" {
		t.Errorf("absolute cd: got %q", got)
	}
	if got := cdPrefixPath("mindspec complete X", "/session"); got != "" {
		t.Errorf("no cd prefix should yield empty, got %q", got)
	}
}
