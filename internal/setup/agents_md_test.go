package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// agents_md_test.go — spec 123 R7: setup codex's managed AGENTS.md block
// is config-sourced, never mindspec-the-framework's own identity/build
// (AC-13's setup half; AC-14's round-trip + refresh; AC-14b's leaked-
// block healing, the primary #211 exposure).

// TestRunCodex_AgentsMDNoFrameworkLeak_NoConfig pins AC-13's setup half:
// a fresh repo with NO .mindspec/config.yaml, no Makefile, no go.mod —
// after `setup codex`, AGENTS.md carries no mindspec build/title leak and
// no Build & Test section at all.
func TestRunCodex_AgentsMDNoFrameworkLeak_NoConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)
	for _, forbidden := range []string{"make build", "make test", "MindSpec Project"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md leaks framework fact %q:\n%s", forbidden, content)
		}
	}
	if strings.Contains(content, "Build & Test") {
		t.Errorf("AGENTS.md must omit the Build & Test section when commands: is unset:\n%s", content)
	}
}

// TestRunCodex_AgentsMDRendersDeclaredCommands pins AC-14's setup half:
// a declared commands: key renders in the managed Build & Test section.
func TestRunCodex_AgentsMDRendersDeclaredCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "commands:\n  build: npm run build\n  test: npm test\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## Build & Test") {
		t.Errorf("AGENTS.md must render the declared Build & Test section:\n%s", content)
	}
	if !strings.Contains(content, "npm run build") || !strings.Contains(content, "npm test") {
		t.Errorf("AGENTS.md must render the declared commands:\n%s", content)
	}
}

// TestRunCodex_AgentsMDRefreshesOnConfigChange pins AC-14's re-render
// guarantee: `setup codex` run once (no config), then again after
// commands: is declared, re-renders the managed block to the new value
// via the retained wholesale BEGIN/END replacement — while content
// OUTSIDE the markers is left untouched.
func TestRunCodex_AgentsMDRefreshesOnConfigChange(t *testing.T) {
	// Not t.Parallel(): shares config.Load's process-lifetime cache, which
	// this test explicitly resets between the two on-disk config states.
	root := t.TempDir()
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("first RunCodex: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(first), "Build & Test") {
		t.Fatalf("precondition: first run must have no Build & Test section:\n%s", first)
	}

	// Plant outside-marker content the refresh must not touch.
	withMarginalia := string(first) + "\n<!-- operator note: do not remove -->\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(withMarginalia), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "commands:\n  build: make build-this-repo\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache() // the on-disk config changed; Load's cache must not serve the stale value

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("second RunCodex: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(second)
	if !strings.Contains(content, "make build-this-repo") {
		t.Errorf("refresh must render the newly-declared command:\n%s", content)
	}
	if !strings.Contains(content, "<!-- operator note: do not remove -->") {
		t.Errorf("refresh must preserve outside-marker content:\n%s", content)
	}
}

// TestRunCodex_CheckModeWritesNothingForAgentsMD is a narrow --check
// guard specific to the new config-load path in ensureAgentsMD: --check
// must still report without writing even though loading config is now
// part of the AGENTS.md path.
func TestRunCodex_CheckModeWritesNothingForAgentsMD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := RunCodex(root, true); err != nil {
		t.Fatalf("RunCodex(check): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("--check must not create AGENTS.md, stat err=%v", err)
	}
}

// TestRunCodex_HealsLeakedFrameworkBlock pins AC-14b, the primary #211
// exposure: an EXISTING consumer AGENTS.md already carrying the exact
// leaked v0.12.0 managed block (make build/make test +
// "# AGENTS.md — MindSpec Project" title, no Makefile present). After
// `mindspec setup codex`, the wholesale block replacement rewrites the
// managed block so the framework facts are GONE; the build section is
// consumer-declared-or-omitted per the commands: key; and content
// OUTSIDE the markers is untouched. RED on pre-spec-123 main: `setup`
// regenerated the identical leaked block every time.
func TestRunCodex_HealsLeakedFrameworkBlock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	leaked := `# AGENTS.md — MindSpec Project
<!-- BEGIN mindspec:managed -->

This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

## Build & Test

` + "```bash" + `
make build    # Build binary
make test     # Run all tests
` + "```" + `
<!-- END mindspec:managed -->

## Operator-authored section

Do not touch this part.
`
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(leaked), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, forbidden := range []string{"make build", "make test", "MindSpec Project"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("leaked framework fact %q survived the refresh:\n%s", forbidden, content)
		}
	}
	if strings.Contains(content, "Build & Test") {
		t.Errorf("unset commands: must omit the Build & Test section post-refresh:\n%s", content)
	}
	if !strings.Contains(content, "## Operator-authored section") || !strings.Contains(content, "Do not touch this part.") {
		t.Errorf("content outside the managed markers must be untouched:\n%s", content)
	}
}

// leakedAgentsMDWithCommand builds a pre-123 leaked AGENTS.md whose
// managed block declares a consumer build command, so a data-loss
// regression (rewriting from defaults) is observable: the operator's
// build line must survive.
func leakedAgentsMDWithCommand(t *testing.T, root, buildLine string) {
	t.Helper()
	content := "# AGENTS.md\n" + mindspecMarkerBegin + `

This project uses [MindSpec](https://github.com/mrmaxsteel/mindspec), a spec-driven development framework.

## Build & Test

` + "```bash" + `
` + buildLine + `
` + "```" + `
` + mindspecMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunCodex_MalformedConfigDoesNotClobberAgentsMD pins spec 123 FX-1
// (data-loss, G-1): an UNRELATED malformed config key (`runner: typoo`)
// alongside a valid `commands.build` must NOT cause setup to silently
// rewrite AGENTS.md from a DefaultConfig fallback — which would ERASE the
// consumer's declared build guidance. Setup must FAIL LOUDLY and leave
// the existing managed block (with its build line) byte-untouched.
func TestRunCodex_MalformedConfigDoesNotClobberAgentsMD(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A VALID commands.build alongside an INVALID unrelated key.
	badCfg := "runner: typoo\ncommands:\n  build: npm run build\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(badCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	leakedAgentsMDWithCommand(t, root, "npm run build   # build")
	before, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	_, runErr := RunCodex(root, false)
	if runErr == nil {
		t.Fatal("expected RunCodex to FAIL LOUDLY on a malformed config, got nil error")
	}
	if !strings.Contains(runErr.Error(), "config.yaml") {
		t.Errorf("error should name the config as the actionable cause, got: %v", runErr)
	}

	after, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("AGENTS.md must be byte-untouched when config load fails.\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if !strings.Contains(string(after), "npm run build") {
		t.Errorf("consumer's declared build line must be preserved, not clobbered to defaults:\n%s", after)
	}
}

// TestHealLegacyTitle_ProvenanceGated pins spec 123 FX-3 (provenance,
// G-2): an AGENTS.md with the EXACT legacy title but NO mindspec managed
// markers is a legitimately operator-authored file (a fork, or a project
// genuinely named "MindSpec Project") — the title heal MUST NOT touch it.
func TestHealLegacyTitle_ProvenanceGated(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Exact legacy title, but no BEGIN/END markers → not mindspec-generated.
	legit := "# AGENTS.md — MindSpec Project\n\nA project I genuinely named this. No mindspec markers here.\n"
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(legit), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := healLegacyAgentsMDTitle(root); err != nil {
		t.Fatalf("healLegacyAgentsMDTitle: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != legit {
		t.Errorf("marker-less file with legacy title must be PRESERVED (no provenance), got:\n%s", got)
	}
}

// TestHealLegacyTitle_HealsWhenProvenanceProven confirms the positive
// path: the exact legacy title WITH a well-formed managed pair IS healed.
func TestHealLegacyTitle_HealsWhenProvenanceProven(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	leaked := "# AGENTS.md — MindSpec Project\n" + mindspecMarkerBegin + "\n\nmanaged body\n" + mindspecMarkerEnd + "\n"
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(leaked), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := healLegacyAgentsMDTitle(root); err != nil {
		t.Fatalf("healLegacyAgentsMDTitle: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "# AGENTS.md\n") || strings.Contains(string(got), "MindSpec Project") {
		t.Errorf("provenance-proven leaked title must be healed to the neutral title, got:\n%s", got)
	}
}

// TestEnsureManagedDoc_RefusesMalformedMarkers pins spec 123 FX-4 (marker
// topology, G-3): an AGENTS.md with malformed marker topology (END before
// BEGIN, and a duplicate BEGIN) must be REFUSED without any write — no
// corrupted/duplicated operator content, no half-rewritten file.
func TestEnsureManagedDoc_RefusesMalformedMarkers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "END before BEGIN",
			content: "# AGENTS.md\n" + mindspecMarkerEnd + "\noperator content\n" + mindspecMarkerBegin + "\n",
		},
		{
			name:    "duplicate BEGIN",
			content: "# AGENTS.md\n" + mindspecMarkerBegin + "\nfirst\n" + mindspecMarkerBegin + "\nsecond\n" + mindspecMarkerEnd + "\n",
		},
		{
			name:    "BEGIN with no END",
			content: "# AGENTS.md\n" + mindspecMarkerBegin + "\nbody, never closed\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			config.ResetCache()
			t.Cleanup(config.ResetCache)
			path := filepath.Join(root, "AGENTS.md")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)

			r := &Result{}
			err := ensureAgentsMD(root, false, r)
			if err == nil {
				t.Fatalf("expected a refusal on malformed marker topology (%s), got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "marker") {
				t.Errorf("error should name the marker topology problem, got: %v", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Errorf("file must be byte-unchanged on refusal.\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

// TestRunClaude_HealsLeakedAgentsMDTitle pins spec 123 FX-5 (heal
// coverage, G-4): `init` writes AGENTS.md for every consumer, so a
// pre-123 leaked AGENTS.md title must be healed on the claude-only
// onboarding path too — not only codex.
func TestRunClaude_HealsLeakedAgentsMDTitle(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	leaked := "# AGENTS.md — MindSpec Project\n" + mindspecMarkerBegin + "\n\nmanaged body\n" + mindspecMarkerEnd + "\n"
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(leaked), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunClaude(root, false); err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "MindSpec Project") {
		t.Errorf("setup claude must heal the leaked AGENTS.md title:\n%s", got)
	}
	if !strings.HasPrefix(string(got), "# AGENTS.md\n") {
		t.Errorf("title must be healed to the neutral form:\n%s", got)
	}
}

// TestRunCopilot_HealsLeakedAgentsMDTitle pins spec 123 FX-5 for the
// copilot onboarding path.
func TestRunCopilot_HealsLeakedAgentsMDTitle(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	leaked := "# AGENTS.md — MindSpec Project\n" + mindspecMarkerBegin + "\n\nmanaged body\n" + mindspecMarkerEnd + "\n"
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(leaked), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunCopilot(root, false); err != nil {
		t.Fatalf("RunCopilot: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "MindSpec Project") {
		t.Errorf("setup copilot must heal the leaked AGENTS.md title:\n%s", got)
	}
}

// TestRunCodex_HostileCommandValueEscapedInAgentsMD is the S-slot
// coverage nicety: an end-to-end setup that WRITES AGENTS.md from a
// hostile commands.build value inspects the resulting bytes and confirms
// they are termsafe-escaped (no raw control byte, no forged line).
func TestRunCodex_HostileCommandValueEscapedInAgentsMD(t *testing.T) {
	root := t.TempDir()
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A hostile build command carrying ESC + an embedded forged line.
	cfgYAML := "commands:\n  build: \"echo hi\\x1b[2Jinjected\\nFAKE: forged\"\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunCodex(root, false); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.ContainsRune(content, 0x1b) {
		t.Errorf("AGENTS.md must not contain a raw ESC byte from the hostile command value:\n%q", content)
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "FAKE:") {
			t.Errorf("hostile command value forged a display line in AGENTS.md:\n%s", content)
		}
	}
}
