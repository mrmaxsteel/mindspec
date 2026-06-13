package redact

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestScrubError_KeepsCodeOnly is the B1 hardening (R2/R3): ScrubError
// keeps ONLY the sentinel code token and discards ALL surrounding prose —
// both leading wrapper context and the trailing wrapped message — and
// DROPS the whole chain (ok=false) when no code is present, rather than
// shipping arbitrary free-text prose.
func TestScrubError_KeepsCodeOnly(t *testing.T) {
	t.Parallel()

	// Prose BEFORE the code must NOT survive (the old keep-up-to-code bug).
	leadProse := errors.New("Dolt Error 1105: out of range")
	wrapped := fmt.Errorf("failed to persist the user's private note about their salary: %w", leadProse)
	clean, ok := ScrubError(wrapped)
	if !ok {
		t.Fatalf("coded chain dropped; want classified to the code token")
	}
	for _, leak := range []string{"salary", "private note", "persist", "out of range"} {
		if strings.Contains(clean, leak) {
			t.Errorf("ScrubError leaked surrounding prose %q in %q", leak, clean)
		}
	}
	if !strings.Contains(clean, "Dolt Error 1105") {
		t.Errorf("ScrubError dropped the sentinel code; got %q", clean)
	}

	// Trailing description after the code must NOT survive.
	desc := errors.New("Dolt Error 1105: Out of range value for column description: SECRET BEAD TEXT")
	clean, ok = ScrubError(fmt.Errorf("recording bead event: %w", desc))
	if !ok || strings.Contains(clean, "SECRET BEAD TEXT") || strings.Contains(clean, "description") {
		t.Errorf("ScrubError leaked trailing description: clean=%q ok=%v", clean, ok)
	}

	// NO sentinel code anywhere → DROP the whole chain (ok=false), never
	// ship wrapper prose.
	noCode := fmt.Errorf("complete: %w", fmt.Errorf("validate: %w", errors.New("could not open the confidential salary spreadsheet")))
	clean, ok = ScrubError(noCode)
	if ok {
		t.Errorf("no-code chain shipped free text instead of dropping: clean=%q", clean)
	}
	if clean != "" {
		t.Errorf("dropped chain returned non-empty clean=%q (raw must never be returned)", clean)
	}

	// nil error scrubs to ("", true).
	if c, o := ScrubError(nil); c != "" || !o {
		t.Errorf("ScrubError(nil) = (%q,%v), want (\"\",true)", c, o)
	}
}

// TestRedactEvent_VersionValidated is the A1 keystone fix (R3/codex-*):
// a tainted/decorated Version DROPS the whole event; only bare semver or
// "dev"/"" survives.
func TestRedactEvent_VersionValidated(t *testing.T) {
	t.Parallel()
	// The demonstrated leak: a PAT + abs path + bead id smuggled via Version.
	tainted := Event{
		Version: "ghp_ABCDEFGHIJKLMNOPQRST /Users/victim/key mindspec-cdk8.1",
		Command: "next",
	}
	if re, ok := RedactEvent(tainted); ok {
		t.Fatalf("tainted Version survived (ok=true): %+v", re)
	}
	// Decorated cobra version string also dropped.
	if _, ok := RedactEvent(Event{Version: "1.4.2 (abc1234) 2026-06-12", Command: "next"}); ok {
		t.Error("decorated cobra version string survived; want drop")
	}
	// Accepted forms.
	for _, v := range []string{"", "dev", "1.4.2", "v2.0.0", "0.0.1-rc1"} {
		if _, ok := RedactEvent(Event{Version: v, Command: "next"}); !ok {
			t.Errorf("valid version %q was dropped", v)
		}
	}
}

// TestRedactEvent_VersionCanonicalDropsSuffix is the repanel-leak fix:
// `version.Parse` validates ONLY the core x.y.z and DISCARDS the
// prerelease/build suffix UNVALIDATED, so a secret/path/email smuggled
// after a '-'/'+' would survive verbatim if RedactEvent stored the raw
// input. RedactEvent MUST store the CANONICAL reconstructed core semver
// instead, so the tainted suffix is gone — exactly, byte-for-byte.
func TestRedactEvent_VersionCanonicalDropsSuffix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string // tainted Version input (otherwise enum-valid event)
		want string // exact stored Version after canonicalization
		gone string // the tainted suffix payload that MUST NOT survive
	}{
		{"1.2.3-ghp_0123456789abcdefghijABCDEFGHIJ012345", "1.2.3", "ghp_0123456789abcdefghijABCDEFGHIJ012345"},
		{"v0.0.0+/Users/victim/.ssh/id_rsa", "0.0.0", "/Users/victim/.ssh/id_rsa"},
		{"1.0.0-secret@victim.com", "1.0.0", "secret@victim.com"},
		{"1.2.3+AKIAIOSFODNN7EXAMPLE", "1.2.3", "AKIAIOSFODNN7EXAMPLE"},
		{"1.2.3+meta./Users/max/.aws/credentials", "1.2.3", "/Users/max/.aws/credentials"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			re, ok := RedactEvent(Event{Version: c.in, Command: "next"})
			if !ok {
				// A drop is also leak-free and acceptable, but with a valid
				// core semver we expect canonical-store, not drop.
				t.Fatalf("Version %q dropped; want canonical-stored %q", c.in, c.want)
			}
			if re.Version != c.want {
				t.Errorf("Version %q → stored %q, want canonical %q", c.in, re.Version, c.want)
			}
			if strings.Contains(re.Version, c.gone) {
				t.Errorf("tainted suffix %q survived in stored Version %q", c.gone, re.Version)
			}
			if re.Version == c.in {
				t.Errorf("stored Version is the raw tainted input %q (must be canonical)", c.in)
			}
		})
	}
}

// TestRedactEvent_EnumClosedSets is the A2 enum-masquerade fix
// (codex-leak/codex-completeness): Command/Subcommand/OS are validated
// against closed sets; a non-enum token (even one that would pass Scrub)
// DROPS the event — it is never passed through as an arbitrary string.
func TestRedactEvent_EnumClosedSets(t *testing.T) {
	t.Parallel()
	drops := []struct {
		name string
		ev   Event
	}{
		{"non-enum command", Event{Command: "exfiltrate", Version: "dev"}},
		{"path-arg as command", Event{Command: "/Users/victim/secret", Version: "dev"}},
		{"secret as subcommand", Event{Command: "next", Subcommand: "ghp_ABCDEFGHIJKLMNOPQRST", Version: "dev"}},
		{"arbitrary subcommand", Event{Command: "bead", Subcommand: "rm -rf", Version: "dev"}},
		{"non-goos os", Event{Command: "next", OS: "/etc/passwd", Version: "dev"}},
		{"arbitrary os", Event{Command: "next", OS: "MyLeakyOS", Version: "dev"}},
	}
	for _, d := range drops {
		t.Run(d.name, func(t *testing.T) {
			if re, ok := RedactEvent(d.ev); ok {
				t.Errorf("%s survived (ok=true): %+v", d.name, re)
			}
		})
	}
	// Valid enum tuples are KEPT.
	keeps := []Event{
		{Command: "complete", Subcommand: "", OS: "darwin", Version: "1.0.0", EscapeHatch: "override-adr"},
		{Command: "bead", Subcommand: "start", OS: "linux", Version: "dev"},
		{Command: "spec", Subcommand: "create", OS: "windows", Version: "v2.1.0"},
		{Command: "", Subcommand: "", OS: "", Version: ""},
	}
	for _, ev := range keeps {
		if _, ok := RedactEvent(ev); !ok {
			t.Errorf("valid enum event dropped: %+v", ev)
		}
	}
}

// TestEntropyThresholds is the B3 off-by-one fix (R3/codex-leak): a
// realistic secret one char under the OLD 16-hex / 24-b64 cutoffs is now
// caught, while legitimate short ids stay unscrubbed.
func TestEntropyThresholds(t *testing.T) {
	t.Parallel()
	caught := []string{
		"f0e1d2c3b4a5968",         // 15 hex
		"AbCdEfGhIjKlMnOpQrStUvW", // 23 b64
	}
	for _, s := range caught {
		clean, ok := Scrub("val " + s + " end")
		if !ok {
			continue // a drop is also leak-free
		}
		if strings.Contains(clean, s) {
			t.Errorf("sub-threshold secret %q survived: %q", s, clean)
		}
	}
	// Legitimate short tokens must NOT be over-scrubbed.
	for _, s := range []string{"abc1234", "deadbee", "v1", "094"} {
		clean, ok := Scrub("ref " + s)
		if !ok || !strings.Contains(clean, s) {
			t.Errorf("legit short token %q over-scrubbed: clean=%q ok=%v", s, clean, ok)
		}
	}
}

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
		// --- hardening leak classes (R2/R3/codex-leak) ---
		{
			name:    "pem private key block",
			in:      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAxYZ\n-----END RSA PRIVATE KEY-----",
			absent:  []string{"MIIEpAIBAAKCAQEAxYZ", "BEGIN RSA"},
			present: []string{"<secret>"},
		},
		{
			name:    "pem openssh private key",
			in:      "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmU=\n-----END OPENSSH PRIVATE KEY-----",
			absent:  []string{"b3BlbnNzaC1rZXktdjEAAAAABG5vbmU", "OPENSSH"},
			present: []string{"<secret>"},
		},
		{
			name:    "authorization basic header",
			in:      "Authorization: Basic dXNlcjpzdXBlcnNlY3JldA==",
			absent:  []string{"dXNlcjpzdXBlcnNlY3JldA"},
			present: []string{"Basic <secret>"},
		},
		{
			name:    "authorization basic short credential",
			in:      "Authorization: Basic dXNlcjpwYXNz",
			absent:  []string{"dXNlcjpwYXNz"},
			present: []string{"Basic <secret>"},
		},
		// --- repanel-codex r2: auth-header whitespace-before-colon variants ---
		{
			name:    "authorization space before colon",
			in:      "Authorization : Basic Zm9vOmJhcg==",
			absent:  []string{"Zm9vOmJhcg"},
			present: []string{"Basic <secret>"},
		},
		{
			name:    "lowercase authorization space-colon",
			in:      "authorization  :  Basic dXNlcjpwYXNz",
			absent:  []string{"dXNlcjpwYXNz"},
			present: []string{"Basic <secret>"},
		},
		{
			name:    "proxy-authorization space-colon bearer",
			in:      "Proxy-Authorization : Bearer abc.def.ghi-secret-tok",
			absent:  []string{"abc.def.ghi-secret-tok"},
			present: []string{"Bearer <secret>"},
		},
		// --- repanel-codex r2: malformed/partial PEM (no trailing dashes) ---
		{
			name:    "openssh private key no trailing dashes",
			in:      "-----BEGIN OPENSSH PRIVATE KEY\nQUJDREVGR0hJSktMTU5PUA==\n-----END OPENSSH PRIVATE KEY",
			absent:  []string{"QUJDREVGR0hJSktMTU5PUA", "OPENSSH"},
			present: []string{"<secret>"},
		},
		{
			name:    "ec private key no trailing dashes",
			in:      "-----BEGIN EC PRIVATE KEY\nMHcCAQEEIabcdefghij\n-----END EC PRIVATE KEY",
			absent:  []string{"MHcCAQEEIabcdefghij", "BEGIN EC"},
			present: []string{"<secret>"},
		},
		{
			name:    "encrypted private key no trailing dashes",
			in:      "-----BEGIN ENCRYPTED PRIVATE KEY\nMIIFDjBABgkqhkiG9w0\n-----END ENCRYPTED PRIVATE KEY",
			absent:  []string{"MIIFDjBABgkqhkiG9w0", "ENCRYPTED"},
			present: []string{"<secret>"},
		},
		{
			name:    "truncated private key no end marker",
			in:      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAtruncatedbody",
			absent:  []string{"MIIEpAIBAAKCAQEAtruncatedbody", "BEGIN RSA"},
			present: []string{"<secret>"},
		},
		{
			name:    "unc path",
			in:      `copy from \\fileserver\confidential\payroll.xlsx failed`,
			absent:  []string{"fileserver", "confidential", "payroll"},
			present: []string{"<path>"},
		},
		{
			name:    "unc path with spaces",
			in:      `copy failed for \\corp-fs01\Payroll Share\June\alice bonus.xlsx`,
			absent:  []string{"corp-fs01", "Payroll Share", "alice bonus"},
			present: []string{"<path>"},
		},
		{
			name:    "windows path with spaces terminated by colon",
			in:      `reading domains dir C:\Users\Max\Secret Plans\domains: permission denied`,
			absent:  []string{"Secret Plans", `Plans\domains`, `Users\Max`},
			present: []string{"<path>"},
		},
		{
			name:    "bare space-separated ownership domains",
			in:      "OWNERSHIP domain core git state for spec",
			absent:  []string{"core git state"},
			present: []string{"<domains>"},
		},
		{
			name:    "entropy hex one under old threshold (15 chars)",
			in:      "key f0e1d2c3b4a5968 leaked",
			absent:  []string{"f0e1d2c3b4a5968"},
			present: []string{"<token>"},
		},
		{
			name:    "entropy b64 one under old threshold (23 chars)",
			in:      "cred AbCdEfGhIjKlMnOpQrStUvW now",
			absent:  []string{"AbCdEfGhIjKlMnOpQrStUvW"},
			present: []string{"<token>"},
		},
		{
			name:    "gitlab pat",
			in:      "token glpat-AbCdEfGhIjKlMnOpQrSt leaked",
			absent:  []string{"glpat-AbCdEfGhIjKlMnOpQrSt"},
			present: []string{"<secret>"},
		},
		{
			name:    "slack bot token",
			in:      "token xoxb-12345678-abcdefghij here",
			absent:  []string{"xoxb-12345678-abcdefghij"},
			present: []string{"<secret>"},
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
