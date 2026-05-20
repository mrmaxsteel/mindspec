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

	// The "Available Commands:" section lists subcommands one per
	// line. We scan all visible subcommand names by matching the
	// indented "  <name>" pattern. cobra's default help renders e.g.
	//   "  setup       …"
	subRe := regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*)\s`)
	matches := subRe.FindAllStringSubmatch(out, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		seen[m[1]] = true
	}

	// Filter to the two subcommands we expect (cobra also reports
	// "completion" and "help" as built-ins; those are acceptable —
	// only the user-defined surface matters).
	wantPresent := []string{"setup", "status"}
	for _, name := range wantPresent {
		if !seen[name] {
			t.Errorf("mindspec otel --help missing subcommand %q\n--- output ---\n%s\n--- end ---", name, out)
		}
	}

	// Hard-reject any agentmind/bench/replay/serve/viz that might have
	// leaked into the otel subtree.
	bannedHere := []string{"agentmind", "bench", "replay", "serve", "viz"}
	for _, b := range bannedHere {
		if seen[b] {
			t.Errorf("mindspec otel --help unexpectedly lists banned subcommand %q\n--- output ---\n%s\n--- end ---", b, out)
		}
	}
}

// containsToken returns true if s contains tok as a word-ish token
// (surrounded by non-alphanumeric / start-or-end). Without this guard
// "service" would falsely match "serve".
func containsToken(s, tok string) bool {
	re := regexp.MustCompile(`(?i)(^|[^a-z0-9_])` + regexp.QuoteMeta(tok) + `($|[^a-z0-9_])`)
	return re.MatchString(strings.ToLower(s))
}
