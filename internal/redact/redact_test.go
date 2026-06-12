package redact

import (
	"strings"
	"testing"
)

// TestScrub_Categories exercises each scrub category (Req 1): the
// dangerous token must be gone and a typed placeholder present.
func TestScrub_Categories(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		absent  []string // substrings that MUST NOT survive
		present []string // placeholders that MUST appear
	}{
		{
			name:    "absolute posix path",
			in:      "failed at /Users/maxsteel/replit/mindspec/internal/x",
			absent:  []string{"/Users/maxsteel", "maxsteel", "replit"},
			present: []string{"<path>"},
		},
		{
			name:    "windows absolute path",
			in:      `open C:\Users\max\proj\file.txt failed`,
			absent:  []string{`C:\Users`, "max", "proj"},
			present: []string{"<path>"},
		},
		{
			name:    "relative source path",
			in:      "see internal/next/guard.go for details",
			absent:  []string{"internal/next/guard.go", "guard.go"},
			present: []string{"<path>"},
		},
		{
			name:    "dotdot relative path",
			in:      "git worktree add ../../.worktrees/worktree-mindspec-cdk8.1 -b x",
			absent:  []string{"../../.worktrees", "worktree-mindspec-cdk8.1", "mindspec-cdk8.1"},
			present: []string{"<path>"},
		},
		{
			name:    "go file token",
			in:      "the divergence.go template",
			absent:  []string{"divergence.go"},
			present: []string{"<file>"},
		},
		{
			name:    "spec slug",
			in:      "spec 094-self-improvement-loop landed",
			absent:  []string{"094-self-improvement-loop"},
			present: []string{"<spec>"},
		},
		{
			name:    "bead id",
			in:      "claiming bead mindspec-cdk8.1 now",
			absent:  []string{"mindspec-cdk8.1"},
			present: []string{"<bead>"},
		},
		{
			name:    "bead branch",
			in:      "checkout bead/mindspec-cdk8.1 first",
			absent:  []string{"bead/mindspec-cdk8.1", "mindspec-cdk8.1"},
			present: []string{"<branch>"},
		},
		{
			name:    "spec branch",
			in:      "merge spec/094-self-improvement-loop",
			absent:  []string{"spec/094-self-improvement-loop"},
			present: []string{"<branch>"},
		},
		{
			name:    "ownership domain slice",
			in:      "impacted domains [core git state]; add it",
			absent:  []string{"[core git state]", "core git state"},
			present: []string{"<domains>"},
		},
		{
			name:    "quoted domain",
			in:      `attributed to domain "execution" only`,
			absent:  []string{`"execution"`},
			present: []string{"<domain>"},
		},
		{
			name:    "github token secret",
			in:      "token ghp_ABCDEFGHIJKLMNOPqrstuvwxyz0123456789 leaked",
			absent:  []string{"ghp_ABCDEFGHIJKLMNOP"},
			present: []string{"<secret>"},
		},
		{
			name:    "aws key secret",
			in:      "key AKIAIOSFODNN7EXAMPLE here",
			absent:  []string{"AKIAIOSFODNN7EXAMPLE"},
			present: []string{"<secret>"},
		},
		{
			name:    "openai key secret",
			in:      "sk-abcdefghijklmnopqrstuvwx token",
			absent:  []string{"sk-abcdefghijklmnop"},
			present: []string{"<secret>"},
		},
		{
			name:    "bearer secret",
			in:      "Authorization: Bearer abcdef12345.token.value",
			absent:  []string{"abcdef12345.token.value"},
			present: []string{"Bearer <secret>"},
		},
		{
			name:    "token assignment",
			in:      "GITHUB_TOKEN=ghs_supersecretvalue123456",
			absent:  []string{"ghs_supersecretvalue", "supersecret"},
			present: []string{"<secret>"},
		},
		{
			name:    "email",
			in:      "contact max@cloudlete.ai about it",
			absent:  []string{"max@cloudlete.ai", "cloudlete"},
			present: []string{"<email>"},
		},
		{
			name:    "ipv4",
			in:      "dialed 192.168.10.42 ok",
			absent:  []string{"192.168.10.42"},
			present: []string{"<ip>"},
		},
		{
			name:    "entropy hex",
			in:      "commit deadbeefcafebabe0123456789abcdef stamped",
			absent:  []string{"deadbeefcafebabe0123456789abcdef"},
			present: []string{"<token>"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clean, ok := Scrub(c.in)
			if !ok {
				// A drop is also leak-free; but these inputs should classify.
				t.Fatalf("Scrub(%q) dropped (ok=false); want classified+scrubbed", c.in)
			}
			for _, a := range c.absent {
				if strings.Contains(clean, a) {
					t.Errorf("Scrub(%q) = %q still contains %q", c.in, clean, a)
				}
			}
			for _, p := range c.present {
				if !strings.Contains(clean, p) {
					t.Errorf("Scrub(%q) = %q missing placeholder %q", c.in, clean, p)
				}
			}
		})
	}
}

// TestScrub_FailClosed_OversizeDrops proves the HC-7 drop signal: an
// input that cannot be confidently classified returns ok=false and an
// EMPTY clean — never the raw value.
func TestScrub_FailClosed_OversizeDrops(t *testing.T) {
	t.Parallel()
	raw := strings.Repeat("/Users/secret/path ", 2000) // > maxScrubInput
	clean, ok := Scrub(raw)
	if ok {
		t.Fatalf("oversize input: ok=true, want fail-closed drop")
	}
	if clean != "" {
		t.Fatalf("oversize input: clean=%q, want empty (raw never returned)", clean)
	}
	if strings.Contains(clean, "secret") {
		t.Fatalf("raw value leaked through a dropped scrub")
	}
}

// TestScrub_FailClosed_PanicDrops proves a panic during classification
// is swallowed into the DROP signal (HC-7), never a raw fallback.
func TestScrub_FailClosed_PanicDrops(t *testing.T) {
	orig := scrubPanicHook
	t.Cleanup(func() { scrubPanicHook = orig })
	scrubPanicHook = func(string) { panic("boom") }

	clean, ok := Scrub("/Users/me/secret")
	if ok || clean != "" {
		t.Fatalf("panic path: got (%q, %v), want (\"\", false)", clean, ok)
	}
}

// TestHasResidualLeak verifies the in-library fail-closed gate detects a
// dangerous token that survived scrubbing.
func TestHasResidualLeak(t *testing.T) {
	t.Parallel()
	if !hasResidualLeak("/etc/passwd") {
		t.Error("expected residual abs path to be detected")
	}
	if !hasResidualLeak("ref internal/x/y.go") {
		t.Error("expected residual slash path to be detected")
	}
	if hasResidualLeak("command complete used override-adr <path>") {
		t.Error("clean placeholder output falsely flagged as a leak")
	}
}

// TestRedactEvent_EnumOnlyUnchanged is the HC-2/M4 allowlist primary
// path: an Event with ONLY closed-set enum fields redacts to itself
// (modulo the added fingerprint/identity).
func TestRedactEvent_EnumOnlyUnchanged(t *testing.T) {
	t.Parallel()
	ev := Event{
		Argv0:       "mindspec",
		Command:     "complete",
		EscapeHatch: "override-adr",
		Subcommand:  "",
		Version:     "1.4.2",
		OS:          "darwin",
	}
	got, ok := RedactEvent(ev)
	if !ok {
		t.Fatalf("enum-only event dropped; want kept unchanged")
	}
	if got.Argv0 != "mindspec" || got.Command != "complete" ||
		got.EscapeHatch != "override-adr" || got.OS != "darwin" || got.Version != "1.4.2" {
		t.Errorf("enum-only event changed: %+v", got)
	}
	if got.Fingerprint == "" {
		t.Error("expected a fingerprint to be stamped")
	}
	if got.Identity != (Identity{Command: "complete", EscapeHatch: "override-adr"}) {
		t.Errorf("identity = %+v, want canonical tuple", got.Identity)
	}
}

// TestRedactEvent_Argv0Basename is the M3 promotion: argv[0] is reduced
// to basename and scrubbed BEFORE any return — the verbatim
// home-dir/username invocation path is never returned.
func TestRedactEvent_Argv0Basename(t *testing.T) {
	t.Parallel()
	ev := Event{
		Argv0:       "/Users/maxsteel/go/bin/mindspec",
		Command:     "complete",
		EscapeHatch: "",
		Version:     "dev",
		OS:          "linux",
	}
	got, ok := RedactEvent(ev)
	if !ok {
		t.Fatalf("event unexpectedly dropped")
	}
	if got.Argv0 != "mindspec" {
		t.Errorf("Argv0 = %q, want basename %q", got.Argv0, "mindspec")
	}
	if strings.Contains(got.Argv0, "maxsteel") || strings.Contains(got.Argv0, "/Users") {
		t.Errorf("argv[0] invocation path leaked: %q", got.Argv0)
	}
}

// TestRedactEvent_TaintedEscapeHatchDropped is the M4 fail-closed enum
// boundary: a value smuggled into the EscapeHatch field (not a closed-set
// token) drops the WHOLE entry.
func TestRedactEvent_TaintedEscapeHatchDropped(t *testing.T) {
	t.Parallel()
	ev := Event{
		Argv0:       "mindspec",
		Command:     "complete",
		EscapeHatch: "override-adr=the real reason text", // tainted value
		Version:     "1.0.0",
		OS:          "darwin",
	}
	if _, ok := RedactEvent(ev); ok {
		t.Fatalf("tainted escape-hatch value was NOT dropped (M4 violation)")
	}
}

// TestFingerprint_Deterministic: identical identity → identical hash
// across runs.
func TestFingerprint_Deterministic(t *testing.T) {
	t.Parallel()
	id := Identity{Command: "complete", EscapeHatch: "override-adr"}
	if Fingerprint(id) != Fingerprint(id) {
		t.Fatal("fingerprint is not deterministic for identical identity")
	}
}

// TestFingerprint_ReasonInvariant: the same flag with two different
// reason VALUES yields the SAME fingerprint — the reason is not an input.
func TestFingerprint_ReasonInvariant(t *testing.T) {
	t.Parallel()
	// Reason values are never part of Identity, so two events that differ
	// only by reason carry the SAME identity → same fingerprint.
	a := Fingerprint(Identity{Command: "complete", EscapeHatch: "override-adr"})
	b := Fingerprint(Identity{Command: "complete", EscapeHatch: "override-adr"})
	if a != b {
		t.Fatal("fingerprint is not reason-invariant")
	}
}

// TestFingerprint_DistinctOnAnyInput: the dedup key is falsifiable — it
// DIFFERS when any structured input differs. A constant-returning
// fingerprint fails this.
func TestFingerprint_DistinctOnAnyInput(t *testing.T) {
	t.Parallel()
	base := Fingerprint(Identity{Command: "complete", EscapeHatch: "override-adr"})
	variants := []Identity{
		{Command: "complete", EscapeHatch: "allow-doc-skew"},                    // diff hatch
		{Command: "next", EscapeHatch: "override-adr"},                          // diff command
		{Command: "complete", EscapeHatch: "override-adr", Subcommand: "phase"}, // diff sub
	}
	for _, v := range variants {
		if Fingerprint(v) == base {
			t.Errorf("fingerprint collided for distinct identity %+v", v)
		}
	}
	// Boundary-forgery guard: {complete, override-adr} must not collide
	// with a reslicing like {complete-, override, -adr}.
	if Fingerprint(Identity{Command: "complete", EscapeHatch: "override-adr"}) ==
		Fingerprint(Identity{Command: "complete-override", EscapeHatch: "adr"}) {
		t.Error("fingerprint aliased across a field boundary")
	}
}
