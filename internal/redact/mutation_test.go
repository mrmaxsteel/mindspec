package redact

import (
	"regexp"
	"strings"
	"testing"
)

// neverMatch is a regex that matches nothing — used to NEUTER a pass and
// prove the build goes RED if any scrub pass is removed (C1 / R3:
// previously the 8 mutation-survivor passes had no test that went red
// when neutered, because the residual drop-gate masked the regression).
var neverMatch = regexp.MustCompile(`\x00NEVER_MATCHES\x00`)

// runPasses applies the scrub passes ONLY (no residual gate, no length
// cap) so a per-pass mutation test can observe whether a specific pass is
// load-bearing for its token — the residual gate is defense-in-depth and
// must NOT mask a neutered primary pass.
func runPasses(s string) string {
	out := s
	for _, p := range scrubPasses {
		out = p.re.ReplaceAllString(out, p.repl)
	}
	return out
}

// passFixtures maps each scrub pass name to an input that EXERCISES that
// pass, plus the substring that pass UNIQUELY removes (a substring no
// OTHER pass touches, so neutering the pass measurably regresses the
// output). Every shipped pass MUST appear here (the completeness check
// below fails the build if a new pass is added without a fixture), so
// neutering ANY pass goes RED (C1 / R3 — no surviving mutations).
//
// `unique` is a substring of the INPUT that survives the pass pipeline
// when this pass is neutered, but is gone when it is active. It is chosen
// so that NO other pass would remove it (verified by the test: with the
// pass active `unique` is absent; with it neutered `unique` is present).
var passFixtures = map[string]struct {
	in     string
	unique string
}{
	"secret-pem":         {"-----BEGIN RSA PRIVATE KEY-----\nMIIabcdefghij\n-----END RSA PRIVATE KEY-----", "PRIVATE KEY"},
	"secret-auth-header": {"Authorization: Basic dXNlcjpwYXNz extra", "dXNlcjpwYXNz"},
	"secret-ghp":         {"x ghp_ABCDEFGHIJKLMNOPq y", "ghp_ABCDEFGHIJKLMNOPq"},
	"secret-github-pat":  {"x github_pat_11ABCDEFG0123 y", "github_pat_11ABCDEFG0123"},
	"secret-gitlab-pat":  {"x glpat-AbCdEfGhIjKlMnOp y", "glpat-AbCdEfGhIjKlMnOp"},
	"secret-slack":       {"x xoxb-1234-abcdefgh y", "xoxb-1234-abcdefgh"},
	"secret-aws":         {"x AKIAIOSFODNN7EXAM y", "AKIAIOSFODNN7EXAM"},
	"secret-openai":      {"x sk-abcdefghijklmnop y", "sk-abcdefghijklmnop"},
	"secret-bearer":      {"got Bearer shorttok99 here", "Bearer shorttok99"},
	"secret-jwt":         {"x eyJabcdef.eyJghijkl.SflKxwRk y", "eyJabcdef.eyJghijkl.SflKxwRk"},
	"secret-assign":      {"API_KEY=shortval here", "API_KEY=shortval"},
	"email":              {"contact max@cloudlete.io now", "max@cloudlete.io"},
	"url":                {"see https://host.example/p here", "host.example/p"},
	"ipv4":               {"dialed 10.0.0.7 ok", "10.0.0.7"},
	"ipv6":               {"addr fe80:0:0:0:1:2:3:4 up", "fe80:0:0:0:1:2:3:4"},
	// branch: only the `bead/` prefix render is unique to this pass;
	// bead-id would still turn the id into <bead>, so the unique signal is
	// the literal `bead/` (the branch pass renders the whole thing <branch>).
	"branch":        {"checkout bead/feature-x now", "bead/feature-x"}, // covered by slashpath
	"bead-id":       {"claiming mindspec-zzz9 now", "mindspec-zzz9"},
	"spec-slug":     {"spec 094-loop-name-here landed", "094-loop-name-here"},
	"adr-id":        {"cited ADR-0042 now", "ADR-0042"},
	"abspath-unc":   {`copy \\hostsrv\confidential done`, `\\hostsrv\confidential`},
	"abspath-win":   {`open D:\proj\f failed`, `D:\proj\f`},
	"slashpath":     {"see lib/util for it", "lib/util"},
	"worktree-dir":  {"in worktree-feature-x dir", "worktree-feature-x"},
	"domains-slice": {"domains [alpha beta] add", "[alpha beta]"},
	"domain-quoted": {`domain "execution" only`, `"execution"`},
	"domains-bare":  {"domain alpha beta gamma end", "alpha beta gamma"},
	"filename":      {"the report.md template", "report.md"},
	"entropy-hex":   {"commit f0e1d2c3b4a5968 here", "f0e1d2c3b4a5968"},
	"entropy-b64":   {"cred AbCdEfGhIjKlMnOpQrStUvW now", "AbCdEfGhIjKlMnOpQrStUvW"},
}

// redundantPasses documents the scrub passes whose token is FULLY covered
// by a coarser backstop pass, so neutering the provider-specific pass
// alone does NOT re-leak (the backstop still fires). They are kept for a
// clearer placeholder (`<secret>` vs `<token>`) and ordering safety, but
// they are NOT independently load-bearing. For each, the named COVERING
// pass IS independently load-bearing (asserted below), so the leak class
// is still gated — only the cosmetic placeholder regresses if removed.
//
//   - secret-github-pat: github_pat_ mandates a 20+ char body, so the run
//     is always ≥31 chars and entropy-b64 (≥23) always catches it.
var redundantPasses = map[string]string{
	"secret-github-pat": "entropy-b64",
	// A URL and a bead/spec branch both contain a `/`, so the generic
	// slashpath pass already redacts them to <path>; the url / branch
	// passes only refine the placeholder (<url>/<branch>). slashpath is the
	// load-bearing backstop for the leak itself.
	"url":    "slashpath",
	"branch": "slashpath",
}

// neuter swaps a pass's regex for neverMatch and returns a restore func.
func neuter(name string) (restore func()) {
	for i := range scrubPasses {
		if scrubPasses[i].name == name {
			orig := scrubPasses[i].re
			scrubPasses[i].re = neverMatch
			return func() { scrubPasses[i].re = orig }
		}
	}
	return func() {}
}

// TestPerPassMutation_RedWhenNeutered proves EVERY scrub pass is
// load-bearing: with all passes active the pass's UNIQUE substring is
// gone; neutering JUST that pass makes the substring reappear in the pass
// pipeline output. A removed/regressed pass therefore turns this test RED
// (C1 / R3 — no surviving mutations). The pass pipeline (not full Scrub)
// is used so the residual drop-gate cannot MASK a neutered primary pass.
//
// For the documented redundantPasses, neutering the pass alone is NOT
// expected to re-leak; instead the test asserts the named COVERING pass
// is load-bearing for the same token, so the leak class stays gated.
func TestPerPassMutation_RedWhenNeutered(t *testing.T) {
	// Completeness: every shipped pass MUST have a fixture.
	for _, p := range scrubPasses {
		if _, ok := passFixtures[p.name]; !ok {
			t.Errorf("scrub pass %q has no mutation fixture — add one to passFixtures", p.name)
		}
	}

	for i := range scrubPasses {
		p := scrubPasses[i]
		fx, ok := passFixtures[p.name]
		if !ok {
			continue
		}
		t.Run(p.name, func(t *testing.T) {
			// Active: the unique substring is removed by the pipeline.
			active := runPasses(fx.in)
			if strings.Contains(active, fx.unique) {
				t.Fatalf("pass %q: unique substring %q was NOT removed with all "+
					"passes active (fixture wrong?): %q", p.name, fx.unique, active)
			}

			if covering, redundant := redundantPasses[p.name]; redundant {
				// Neutering the redundant pass alone must NOT re-leak (the
				// covering backstop still holds the token) …
				restore := neuter(p.name)
				stillGated := runPasses(fx.in)
				restore()
				if strings.Contains(stillGated, fx.unique) {
					t.Errorf("redundant pass %q re-leaked %q when neutered — it is "+
						"actually load-bearing; remove it from redundantPasses and "+
						"give it a unique fixture", p.name, fx.unique)
				}
				// … but with BOTH the pass and its named covering backstop
				// neutered, the token MUST re-leak — proving the class is gated
				// only by these two and nothing silently else (so removing the
				// covering pass is caught by ITS own fixture/this assertion).
				r1 := neuter(p.name)
				r2 := neuter(covering)
				bothOff := runPasses(fx.in)
				r2()
				r1()
				if !strings.Contains(bothOff, fx.unique) {
					t.Errorf("redundant pass %q + covering %q both neutered did NOT "+
						"re-leak %q — a third pass also covers it; update "+
						"redundantPasses. got=%q", p.name, covering, fx.unique, bothOff)
				}
				return
			}

			// Neuter ONLY this pass; the unique substring MUST reappear,
			// proving this pass is the one responsible (no other pass masks it).
			restore := neuter(p.name)
			mutated := runPasses(fx.in)
			restore()
			if !strings.Contains(mutated, fx.unique) {
				t.Errorf("neutering pass %q did NOT bring back %q — another pass "+
					"masks it, so removing %q would go undetected. Pick a substring "+
					"only %q removes (or document it in redundantPasses). mutated=%q",
					p.name, fx.unique, p.name, p.name, mutated)
			}
		})
	}
}
