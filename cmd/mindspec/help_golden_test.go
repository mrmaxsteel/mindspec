package main

// Spec 084 (mindspec-otel-only) Bead 3: help-surface gate.
//
// The pre-bead-3 version of this file pinned `agentmind serve --help`,
// `agentmind replay --help`, and `viz --help` to spec-083 golden files.
// Those commands are deleted in this bead, and the golden files with
// them. The new gate enforces spec 084 Hard Constraint #6 ("mindspec
// --help is observability-name-free"):
//
//   - `mindspec --help` (the default visible-subcommand listing) must
//     not contain any of the tokens: agentmind, bench, serve, replay,
//     viz. The deprecation stubs in deprecated_commands.go are
//     registered as hidden cobra commands so they do not appear in
//     `--help`.
//   - `mindspec otel --help` must list exactly two subcommands: setup
//     and status.
//
// Bead 4 will harden the gate further with the AST scan in the
// permanent specgate test; until then this subprocess assertion is the
// load-bearing check.

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestRootHelpOmitsObservabilityNames(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mindspec --help: %v\nstderr=%q", err, stderr.String())
	}
	out := stdout.String()

	banned := []string{"agentmind", "bench", "serve", "replay", "viz"}
	for _, tok := range banned {
		if containsToken(out, tok) {
			t.Errorf("mindspec --help contains banned token %q (spec 084 HC #6)\n--- output ---\n%s\n--- end ---", tok, out)
		}
	}
}

func TestOtelHelpListsExactlySetupAndStatus(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin, "otel", "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mindspec otel --help: %v\nstderr=%q", err, stderr.String())
	}
	out := stdout.String()

	// Extract subcommand names from the "Available Commands:" section
	// only. cobra also emits indented lines for Usage (e.g.
	// "  mindspec otel [command]") and the cmd's Long description that
	// would otherwise false-match an "indented two-space + word"
	// regex. Slice between "Available Commands:" and the next
	// non-indented section header (Flags:, Global Flags:, etc).
	availIdx := strings.Index(out, "Available Commands:")
	if availIdx < 0 {
		t.Fatalf("mindspec otel --help: no 'Available Commands:' section\n--- output ---\n%s\n--- end ---", out)
	}
	availSection := out[availIdx:]
	// Stop at the first blank line OR the next "Flags:" / "Global
	// Flags:" header, whichever comes first.
	if endIdx := strings.Index(availSection, "\n\n"); endIdx > 0 {
		availSection = availSection[:endIdx]
	}
	subRe := regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*)\s`)
	matches := subRe.FindAllStringSubmatch(availSection, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		seen[m[1]] = true
	}

	// Filter to user-defined subcommands. cobra also reports
	// "completion" and "help" as built-ins; drop those before
	// enforcing exact set-equality so a future user-defined third
	// subcommand (e.g. `mindspec otel export`) fails the test.
	builtins := map[string]bool{"completion": true, "help": true}
	userDefined := map[string]bool{}
	for name := range seen {
		if !builtins[name] {
			userDefined[name] = true
		}
	}

	wantSet := map[string]bool{"setup": true, "status": true}
	for name := range wantSet {
		if !userDefined[name] {
			t.Errorf("mindspec otel --help missing subcommand %q\n--- output ---\n%s\n--- end ---", name, out)
		}
	}
	for name := range userDefined {
		if !wantSet[name] {
			t.Errorf("mindspec otel --help has unexpected user-defined subcommand %q (expected exactly setup+status)\n--- output ---\n%s\n--- end ---", name, out)
		}
	}

	// Hard-reject any agentmind/bench/replay/serve/viz that might have
	// leaked into the otel subtree. (Redundant with the set-equality
	// check above, but explicit so the failure message names the
	// banned token.)
	bannedHere := []string{"agentmind", "bench", "replay", "serve", "viz"}
	for _, b := range bannedHere {
		if seen[b] {
			t.Errorf("mindspec otel --help unexpectedly lists banned subcommand %q\n--- output ---\n%s\n--- end ---", b, out)
		}
	}
}

// TestRootHelpListsReattest pins the spec 125 R4 verb on the help
// surface: `mindspec --help` lists `reattest` as a visible top-level
// command (it is an EXPLICIT operator-invoked recovery, so unlike the
// deprecation shims it must be discoverable, never hidden).
func TestRootHelpListsReattest(t *testing.T) {
	bin := buildMindspecBinary(t)

	cmd := exec.Command(bin, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mindspec --help: %v\nstderr=%q", err, stderr.String())
	}
	if !containsToken(stdout.String(), "reattest") {
		t.Errorf("mindspec --help must list the reattest verb (spec 125 R4)\n--- output ---\n%s\n--- end ---", stdout.String())
	}
}

// containsToken returns true if s contains tok as a word-ish token
// (surrounded by non-alphanumeric / start-or-end). Without this guard
// "service" would falsely match "serve". Both inputs are lowercased
// before matching so a mixed-case token like "AgentMind" is found in
// any-case input.
func containsToken(s, tok string) bool {
	re := regexp.MustCompile(`(^|[^a-z0-9_])` + regexp.QuoteMeta(strings.ToLower(tok)) + `($|[^a-z0-9_])`)
	return re.MatchString(strings.ToLower(s))
}
