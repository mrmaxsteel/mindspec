package main

// cmd/mindspec/version_test.go — spec 096 Req 5 (bug 2b4n): the
// `mindspec version` subcommand must succeed and emit stdout that is
// BYTE-EQUAL to `mindspec --version`.
//
// The proof captures the ACTUAL stdout of `mindspec --version` (cobra's
// default version template rendered against the live rootCmd — the
// "<name> version <version> (<commit>) <date>\n" decorated string) and
// the ACTUAL stdout of `mindspec version`, then asserts they are
// byte-identical. It deliberately does NOT compare against a hand-built
// literal: comparing the two real outputs is what pins the cobra
// decoration (the "<name> version " prefix and the trailing newline) so
// the subcommand can never silently drift to a different format.
//
// RED-on-revert: unregister/remove versionCmd and `mindspec version`
// exits non-zero with `unknown command "version"`, so runMindspec below
// returns an error and the test fails at the require-no-error step.

import (
	"bytes"
	"os/exec"
	"testing"
)

// runMindspec execs the freshly built binary with args and returns its
// stdout, stderr, and run error. Both invocations build from the same
// source with identical (default) ldflags, so the version vars match.
func runMindspec(t *testing.T, bin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func TestVersionSubcommandByteEqualToFlag(t *testing.T) {
	bin := buildMindspecBinary(t)

	flagOut, flagErr, err := runMindspec(t, bin, "--version")
	if err != nil {
		t.Fatalf("mindspec --version failed: %v\nstderr=%q", err, flagErr)
	}
	if flagOut == "" {
		t.Fatalf("mindspec --version produced empty stdout; nothing to compare against")
	}

	subOut, subErr, err := runMindspec(t, bin, "version")
	if err != nil {
		// The RED-on-revert signal: without the registered subcommand,
		// cobra exits non-zero with `unknown command "version"`.
		t.Fatalf("mindspec version failed: %v\nstderr=%q", err, subErr)
	}

	if subOut != flagOut {
		t.Errorf("mindspec version stdout is NOT byte-equal to mindspec --version stdout\n version: %q\n--version: %q", subOut, flagOut)
	}
}
