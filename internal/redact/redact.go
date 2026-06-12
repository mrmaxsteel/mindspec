// Package redact is the privacy keystone for the self-improvement loop
// (spec 094, Bead 1 — Req 1, HC-1/HC-2/HC-7).
//
// Everything that could emit collected data (the journal in Bead 2, the
// friction report in Bead 3) routes through this package FIRST. The
// design is structured-enum-fields-first (HC-2): the primary defense is
// collecting only closed-set, mindspec-emitted tokens, never free-text
// scrubbing. Every collected STRING is tainted-by-default and the full
// scrub (Scrub / ScrubError) is the backstop.
//
// # Fail-closed (HC-7)
//
// The load-bearing cross-bead contract is the DROP signal: Scrub returns
// (clean, ok) where ok == false means the field CANNOT be confidently
// classified and MUST be dropped by the caller — the raw value is NEVER
// returned in clean when ok is false, and there is no raw-string
// fallback path anywhere. RedactEvent returns false to drop a whole
// entry the same way. Beads 2/3 build their HC-7 behavior on these
// signals.
package redact

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
)

// maxScrubInput caps the input Scrub will attempt to classify. An input
// larger than this is treated as unclassifiable and DROPPED (HC-7
// fail-closed) rather than risk a partial-scrub leak after capping.
const maxScrubInput = 16 << 10 // 16 KiB

// maxCleanLen length-caps scrubbed output (the §API Contract "%w-chain
// length cap" rule, applied to all scrubbed strings as defense).
const maxCleanLen = 512

// EscapeHatchTokens is the closed-set enum of escape-hatch signals the
// allowlist recognises (plan §Storage Contract). Anything else in an
// Event's EscapeHatch field is TAINTED and drops the entry (M4).
var EscapeHatchTokens = map[string]struct{}{
	"":               {}, // no escape hatch (the common, non-friction case)
	"override-adr":   {},
	"allow-doc-skew": {},
	"supersede-adr":  {},
	"repair-phase":   {},
}

// scrubPanicHook is a test-only seam: when non-nil it is invoked at the
// top of the scrub so a test can force a panic and prove the
// recover→DROP (HC-7) path. Production leaves it nil.
var scrubPanicHook func(string)

// --- scrub pattern passes -------------------------------------------------
//
// Order is load-bearing: secrets/emails/IPs first (before a slash or
// dot split could fragment them), then branch names (before the generic
// path pass would eat `bead/<id>` as a path), then identifiers, paths,
// files, and finally the entropy catch-all.

type pass struct {
	name string
	re   *regexp.Regexp
	repl string
}

var scrubPasses = []pass{
	// Secrets — provider-prefixed tokens and KEY/TOKEN/PASSWORD assignments.
	{"secret-ghp", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{16,}\b`), "<secret>"},
	{"secret-github-pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), "<secret>"},
	{"secret-aws", regexp.MustCompile(`\bAKIA[0-9A-Z]{12,}\b`), "<secret>"},
	{"secret-openai", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`), "<secret>"},
	{"secret-bearer", regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]{8,}=*`), "Bearer <secret>"},
	{"secret-jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\b`), "<secret>"},
	{"secret-assign", regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:TOKEN|KEY|PASSWORD|SECRET))=\S+`), "${1}=<secret>"},
	// Emails before IPs/paths (the @ and dots are distinctive).
	{"email", regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`), "<email>"},
	// URLs (also neutralises markdown auto-link bait).
	{"url", regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s)\]]+`), "<url>"},
	// IPv4 / IPv6.
	{"ipv4", regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`), "<ip>"},
	{"ipv6", regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}\b`), "<ip>"},
	// Branch names BEFORE the path passes (so bead/<id> and spec/<slug>
	// become <branch>, not <path>).
	{"branch", regexp.MustCompile(`\b(?:bead|spec)/[A-Za-z0-9._/-]+`), "<branch>"},
	// Bead ids (mindspec-xxxx[.N]).
	{"bead-id", regexp.MustCompile(`\bmindspec-[a-z0-9]+(?:\.[0-9]+)?\b`), "<bead>"},
	// Spec slugs (NNN-word-word...).
	{"spec-slug", regexp.MustCompile(`\b\d{3}-[a-z0-9]+(?:-[a-z0-9]+)+\b`), "<spec>"},
	// Windows absolute paths.
	{"abspath-win", regexp.MustCompile(`[A-Za-z]:\\[^\s"']*`), "<path>"},
	// Any slash-bearing token (absolute OR relative: /a/b, ../../x,
	// .mindspec/docs/x, internal/next/guard.go) → <path>. The char class
	// excludes whitespace, quotes, brackets and the placeholder angle
	// brackets so it never eats an already-emitted <placeholder>.
	{"slashpath", regexp.MustCompile(`[^\s"'()\[\]<>]*/[^\s"'()\[\]<>]+`), "<path>"},
	// Worktree directory basenames (defense if one appears unrooted).
	{"worktree-dir", regexp.MustCompile(`\bworktree-[A-Za-z0-9._-]+`), "<path>"},
	// OWNERSHIP domain-slice render: `domains [core git state]` → `<domains>`.
	{"domains-slice", regexp.MustCompile(`(?i)domains?\s+\[[^\]]*\]`), "domains <domains>"},
	// Quoted domain token: domain "core" → domain <domain>.
	{"domain-quoted", regexp.MustCompile(`(?i)\bdomain\s+["'][^"']+["']`), "domain <domain>"},
	// Bare file names with a known source/config extension (no slash).
	{"filename", regexp.MustCompile(`\b[\w.-]+\.(?:go|ya?ml|json|jsonl|md|toml|sh|ps1)\b`), "<file>"},
	// Entropy catch-all: long hex or base64-ish runs (token/secret backstop).
	{"entropy-hex", regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`), "<token>"},
	{"entropy-b64", regexp.MustCompile(`\b[A-Za-z0-9+/_-]{24,}={0,2}\b`), "<token>"},
}

// residualLeakPasses are the patterns that, if STILL present after
// scrubbing, mean the scrub could not confidently classify the field —
// the field is then DROPPED (HC-7). They are the same dangerous shapes
// the golden corpus asserts ZERO of, so this gate is the in-library
// mirror of the CI gate.
var residualLeakPasses = []*regexp.Regexp{
	regexp.MustCompile(`[\w.@-]/[\w.@-]`),               // any slash path token
	regexp.MustCompile(`[A-Za-z]:\\`),                   // windows abs path
	regexp.MustCompile(`\bmindspec-[a-z0-9]`),           // bead id
	regexp.MustCompile(`\b\d{3}-[a-z0-9]+-[a-z0-9]`),    // spec slug
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]`),       // gh secret
	regexp.MustCompile(`\bAKIA[0-9A-Z]`),                // aws key
	regexp.MustCompile(`\bsk-[A-Za-z0-9]`),              // openai key
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{6,}\.`),      // jwt
	regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`),          // entropy hex
	regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}={0,2}`),    // entropy b64
	regexp.MustCompile(`@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), // email
}

// Scrub is the full tainted-string scrub (Req 1). It runs the
// structured-enum allowlist's backstop — every pass over the input —
// and returns (clean, ok).
//
// ok == false signals the field CANNOT be confidently classified and
// MUST be dropped by the caller (HC-7 fail-closed). The raw value is
// NEVER returned in clean when ok is false; clean is empty. This
// happens when: the input exceeds maxScrubInput; the scrub panics
// (recovered); or a dangerous pattern survives every pass (residual
// leak). Otherwise ok == true and clean is the redacted output,
// length-capped to maxCleanLen.
func Scrub(s string) (clean string, ok bool) {
	// HC-7: any panic in classification is a DROP, never a raw fallback.
	defer func() {
		if r := recover(); r != nil {
			clean, ok = "", false
		}
	}()
	if hook := scrubPanicHook; hook != nil {
		hook(s)
	}
	// Oversized input is unclassifiable — drop rather than risk a
	// post-cap partial leak.
	if len(s) > maxScrubInput {
		return "", false
	}
	out := s
	for _, p := range scrubPasses {
		out = p.re.ReplaceAllString(out, p.repl)
	}
	if len(out) > maxCleanLen {
		out = out[:maxCleanLen]
	}
	if hasResidualLeak(out) {
		return "", false
	}
	return out, true
}

// errClassRe recognises a sentinel error class/code at the head of a
// chain (e.g. "Dolt Error 1105", "Error 1105", "Error Code 23505").
var errClassRe = regexp.MustCompile(`(?i)\b((?:[a-z]+ )?error(?: code)? \d+)\b`)

// ScrubError is the `%w`/`%v` error-chain rule (Req 1). An error chain
// (errors.Error() flattens every wrapped layer) frequently drags a
// free-text payload the scrub cannot classify token-by-token — e.g. a
// Dolt-1105 carrying a bead DESCRIPTION. The rule:
//
//   - If a sentinel error class/code is present, UNWRAP to it: keep the
//     chain only UP TO AND INCLUDING the code, scrub that head, and
//     DISCARD the wrapped free-text message entirely. This is the
//     "unwrap to the sentinel error class/code and discard the wrapped
//     message" branch — the only way to guarantee an arbitrary
//     description cannot survive.
//   - Otherwise run the full scrub + entropy pass over the WHOLE chain
//     and length-cap it (the "OR" branch); the residual-leak gate in
//     Scrub still DROPS (ok=false) anything it cannot classify.
//
// A nil error scrubs to ("", true). A raw wrapped chain is NEVER shipped.
func ScrubError(err error) (clean string, ok bool) {
	if err == nil {
		return "", true
	}
	text := err.Error()
	if loc := errClassRe.FindStringIndex(text); loc != nil {
		// Keep only the head up to the end of the recognised code; the
		// wrapped free-text tail (the description) is discarded.
		return Scrub(text[:loc[1]])
	}
	return Scrub(text)
}

func hasResidualLeak(s string) bool {
	for _, re := range residualLeakPasses {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// --- structured-event redaction ------------------------------------------

// Identity is the canonical normalized event identity (Req 3 / DQ5):
// closed-set enum tokens only — NO error-class, NO override reason, NO
// user-supplied value. It is the Fingerprint input AND is persisted
// alongside the hash (Bead 2) so a hash collision cannot silently alias
// two distinct events.
type Identity struct {
	Command     string
	EscapeHatch string
	Subcommand  string
}

// Event is the structured, enum-first input to RedactEvent. Argv0 is the
// raw os.Args[0]; it is reduced to basename and scrubbed (M3). Command,
// EscapeHatch and Subcommand are mindspec's own closed-set tokens.
type Event struct {
	Argv0       string
	Command     string
	EscapeHatch string
	Subcommand  string
	Version     string
	OS          string
}

// RedactedEvent is the fail-closed result of RedactEvent: enum fields
// validated/scrubbed, argv0 reduced to a scrubbed basename, plus the
// canonical Identity and its Fingerprint.
type RedactedEvent struct {
	Argv0       string
	Command     string
	EscapeHatch string
	Subcommand  string
	Version     string
	OS          string
	Identity    Identity
	Fingerprint string
}

// RedactEvent scrubs an Event over its structured enum fields and
// returns (RedactedEvent, ok). ok == false drops the WHOLE entry (HC-7):
// it fires when the escape-hatch is not a closed-set token (a tainted
// value smuggled into the enum — M4) or when any string field fails the
// scrub. Argv0 is reduced to basename BEFORE the scrub (M3) so the
// verbatim home-dir/username invocation path is never returned.
func RedactEvent(ev Event) (RedactedEvent, bool) {
	// M4: the escape-hatch MUST be a closed-set token; anything else is
	// a tainted value, not an allowlisted enum — drop the entry.
	if _, allowed := EscapeHatchTokens[ev.EscapeHatch]; !allowed {
		return RedactedEvent{}, false
	}

	argv0, ok := Scrub(filepath.Base(ev.Argv0)) // M3: basename then scrub.
	if !ok {
		return RedactedEvent{}, false
	}
	cmd, ok := Scrub(ev.Command)
	if !ok {
		return RedactedEvent{}, false
	}
	sub, ok := Scrub(ev.Subcommand)
	if !ok {
		return RedactedEvent{}, false
	}
	// OS is a closed-set token (runtime.GOOS); scrub defensively.
	osTok, ok := Scrub(ev.OS)
	if !ok {
		return RedactedEvent{}, false
	}

	id := Identity{Command: cmd, EscapeHatch: ev.EscapeHatch, Subcommand: sub}
	return RedactedEvent{
		Argv0:       argv0,
		Command:     cmd,
		EscapeHatch: ev.EscapeHatch,
		Subcommand:  sub,
		Version:     ev.Version,
		OS:          osTok,
		Identity:    id,
		Fingerprint: Fingerprint(id),
	}, true
}

// Fingerprint is the canonical dedup key (Req 3): a deterministic hash
// over the structured enum tuple ONLY — command + which-escape-hatch +
// subcommand. It is reason-INVARIANT (no override reason, no user value,
// no error-class) and DISTINCT when any structured input differs (the
// dedup key is falsifiable — a constant-returning fingerprint must fail
// the distinctness tests).
//
// A NUL delimiter and explicit field lengths cannot be forged across
// field boundaries, so {complete, override-adr, ""} and {complete-,
// override, -adr, ""} cannot collide.
func Fingerprint(id Identity) string {
	h := sha256.New()
	for _, f := range []string{id.Command, id.EscapeHatch, id.Subcommand} {
		// length-prefix + NUL so no concatenation can alias two tuples.
		h.Write([]byte(itoa(len(f))))
		h.Write([]byte{0})
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
