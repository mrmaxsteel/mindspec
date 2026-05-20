package main

// Spec 084 (mindspec-otel-only) Bead 3 / Test D: each removed
// top-level command emits exactly one stderr line matching the
// per-command migration table (spec lines 411-417) and exits with
// code 2.
//
// The stderr lines are asserted by **byte-equality** (no template
// matching, no regex permissiveness) so any future drift in the
// deprecation messages is caught immediately. The five strings are
// the source of truth for the migration table; deprecated_commands.go
// must produce these and nothing else on the matching invocation.

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestDeprecatedCommands_ExitCodeAndMessage(t *testing.T) {
	bin := buildMindspecBinary(t)

	cases := []struct {
		name     string
		argv     []string
		wantLine string
	}{
		{
			name:     "bench-run",
			argv:     []string{"bench", "run"},
			wantLine: "bench moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0028 for rationale)",
		},
		{
			name:     "bench-bare",
			argv:     []string{"bench"},
			wantLine: "bench moved: install agentmind from https://github.com/mrmaxsteel/agentmind (see ADR-0028 for rationale)",
		},
		{
			name:     "agentmind-serve",
			argv:     []string{"agentmind", "serve"},
			wantLine: "agentmind serve moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind serve' (see ADR-0027)",
		},
		{
			name:     "agentmind-replay",
			argv:     []string{"agentmind", "replay"},
			wantLine: "agentmind replay moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind replay' (see ADR-0027)",
		},
		{
			name:     "agentmind-setup",
			argv:     []string{"agentmind", "setup"},
			wantLine: "agentmind setup renamed: use 'mindspec otel setup' (see ADR-0027 for rationale)",
		},
		{
			name:     "viz",
			argv:     []string{"viz"},
			wantLine: "viz moved: install agentmind from https://github.com/mrmaxsteel/agentmind and run 'agentmind viz' (see ADR-0027)",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, tc.argv...)
			cmd.Env = strippedEnv(t)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			// Exit code MUST be 2. cobra/Go reports this via ExitError.
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected ExitError (code 2), got %v\nstdout=%q\nstderr=%q",
					err, stdout.String(), stderr.String())
			}
			if got := ee.ExitCode(); got != 2 {
				t.Errorf("exit code = %d, want 2\nstderr=%q", got, stderr.String())
			}

			// Stderr MUST be exactly one line (plus trailing newline)
			// matching the per-command migration table verbatim.
			gotStderr := strings.TrimRight(stderr.String(), "\n")
			if gotStderr != tc.wantLine {
				t.Errorf("stderr mismatch (byte-equality)\nwant: %q\ngot:  %q",
					tc.wantLine, gotStderr)
			}

			// Stdout MUST be empty — the deprecation stub does not
			// print to stdout.
			if stdout.Len() != 0 {
				t.Errorf("expected empty stdout, got %q", stdout.String())
			}
		})
	}
}
