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
//
// # Accepted residual (spec 120-trust-boundary-render-audit R9(b), OQ3 —
// RESOLVED)
//
// Two shapes are a DOCUMENTED, accepted residual — not newly scrubbed by
// this bead: a bare `user:pass`-WITHOUT-an-`@` (the dsn-credentials pass
// only fires on the `user:pass@host` authority shape; a lone
// `user:pass` with no host has no distinguishing signal from ordinary
// "label:value" prose, so a new pass here risks over-scrubbing
// legitimate text) and internal hostnames appearing bare (with no
// credential authority or scheme to key off). The HC-7 fail-closed DROP
// already bounds the exposure for anything that also fails
// classification; this residual class is accepted as the resolved
// baseline per OQ3, with plan-refinement latitude retained (a scrub
// pass MAY be added later if golden-corpus measurement shows an
// acceptable false-positive rate).
//
// # IPv6 over-scrub tradeoff (spec 120 R9, AC-21)
//
// The ipv6 pass is deliberately permissive (HC-7: prefer over-scrub to a
// missed leak) and therefore also matches two non-address shapes:
// a bare timestamp like "12:34:56" (three hex-looking colon-separated
// groups — indistinguishable from a compressed/full-form IPv6 address
// by pattern alone) becomes "<ip>", and a C++ scope-resolution token
// like "std::vector" has its trailing letter-plus-"::" fragment
// (e.g. "d::") consumed, yielding a garbled but non-leaking
// "st<ip>vector". Both are pinned, not fixed, by
// TestScrub_IPv6OverScrubTradeoffsPinned — todays behavior is the
// accepted baseline this bead records, not a defect to chase.
package redact

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/mrmaxsteel/mindspec/internal/version"
)

// maxScrubInput caps the input Scrub will attempt to classify. An input
// larger than this is treated as unclassifiable and DROPPED (HC-7
// fail-closed) rather than risk a partial-scrub leak after capping.
const maxScrubInput = 16 << 10 // 16 KiB

// maxCleanLen length-caps scrubbed output (the §API Contract "%w-chain
// length cap" rule, applied to all scrubbed strings as defense).
const maxCleanLen = 512

// EscapeHatchTokens is the closed-set enum of escape-hatch signals the
// allowlist recognizes (plan §Storage Contract). Anything else in an
// Event's EscapeHatch field is TAINTED and drops the entry (M4).
var EscapeHatchTokens = map[string]struct{}{
	"":               {}, // no escape hatch (the common, non-friction case)
	"override-adr":   {},
	"allow-doc-skew": {},
	"supersede-adr":  {},
	"repair-phase":   {},
}

// CommandTokens is the closed-set enum of mindspec's TOP-LEVEL command
// names (the cobra commands registered on rootCmd in cmd/mindspec —
// root.go AddCommand block + ownership.go/source.go, plus the hidden
// backward-compat aliases `approve`/`spec-init` and the deprecated
// `agentmind`/`viz`/`bench` shims). RedactEvent (HC-2 structured-enum
// FIRST) accepts a Command ONLY if it is in this set; ANY other token
// (a flag value, a path arg smuggled into the field, anything) DROPS the
// whole event — a Command is never passed through as an arbitrary
// scrubbed string. "" is allowed (no command, e.g. bare `mindspec`).
//
// Keep this in lockstep with cmd/mindspec command registration; a new
// top-level command must be added here or its success events drop.
var CommandTokens = map[string]struct{}{
	"":          {},
	"adr":       {},
	"approve":   {}, // hidden backward-compat alias
	"bead":      {},
	"cleanup":   {},
	"commands":  {}, // spec 123 R7c: `commands populate` (consumer build/test declaration)
	"complete":  {},
	"config":    {}, // spec 109 R9: read-only `config show`
	"context":   {},
	"doctor":    {},
	"domain":    {},
	"hook":      {},
	"impl":      {},
	"init":      {},
	"instruct":  {},
	"migrate":   {},
	"models":    {}, // spec 123 R6b: `models populate` (declared per-phase model protocol)
	"next":      {},
	"otel":      {},
	"ownership": {},
	"panel":     {}, // spec 110 R1/R2/R3: `panel create|verify|tally`
	"plan":      {},
	"record":    {},
	"release":   {}, // spec 101 bead 3cj0.2: `mindspec release <bead-id>` verb
	"repair":    {},
	"report":    {}, // spec 094 Bead 3: the owner-invoked friction report loop

	"setup":     {},
	"source":    {},
	"spec":      {},
	"spec-init": {}, // hidden backward-compat alias
	"state":     {},
	"trace":     {},
	"validate":  {},
	"version":   {}, // spec 096 Req 5: `version` subcommand == `--version`
	// cobra-generated built-ins (always registered, not disabled): the
	// `help` and `completion` top-level commands are dispatchable, so a
	// success/friction event can carry them — keep them allowlisted or
	// RedactEvent would silently DROP those events (drift-guard caught).
	"completion": {},
	"help":       {},
	// Deprecated top-level shims (still dispatchable, so still emittable).
	"agentmind": {},
	"viz":       {},
	"bench":     {},
}

// SubcommandTokens is the closed-set enum of mindspec leaf SUBCOMMAND
// names — the first word of every child cobra command's Use string
// across cmd/mindspec. As with CommandTokens, RedactEvent accepts a
// Subcommand ONLY if it is here, else it DROPS the event. "" is allowed
// (commands with no subcommand). Verbs shared across parents (add, list,
// show, set, start, status, …) appear once.
var SubcommandTokens = map[string]struct{}{
	"":                 {},
	"add":              {},
	"adr":              {},
	"append":           {}, // panel disposition append (spec 117 Bead 2 R6(b) transactional write op)
	"approve":          {},
	"bash":             {}, // completion bash (cobra built-in)
	"bead":             {}, // context bead
	"check":            {}, // panel disposition check (spec 117 Bead 2 R1(b) completeness floor)
	"claude":           {},
	"cleanup":          {},
	"codex":            {},
	"complete":         {},
	"context":          {},
	"copilot":          {},
	"create":           {},
	"create-from-plan": {},
	"disposition":      {}, // panel disposition (spec 117 telemetry store)
	"docs":             {},
	"fish":             {}, // completion fish (cobra built-in)
	"help":             {}, // cobra `help` subcommand under any parent
	"hygiene":          {},
	"impl":             {}, // approve impl (Bead 2's PRIMARY override-on-impl capture)
	"layout":           {}, // migrate layout (spec 106 flatten mover)
	"list":             {},
	"phase":            {},
	"plan":             {},
	"populate":         {},
	"powershell":       {}, // completion powershell (cobra built-in)
	"query":            {}, // panel disposition query (spec 117 Bead 3 Q1-Q5 surface)
	"record":           {},
	"repair":           {},
	"replay":           {}, // agentmind replay shim
	"serve":            {}, // agentmind serve shim
	"set":              {},
	"setup":            {},
	"show":             {},
	"source":           {},
	"spec":             {}, // approve spec / bead spec / validate spec
	"spec-title":       {}, // repair spec-title (spec 120 R3 lever)
	"start":            {},
	"status":           {},
	"summary":          {},
	"tally":            {}, // panel tally
	"validate":         {},
	"verify":           {}, // panel verify
	"worktree":         {},
	"write-session":    {},
	"zsh":              {}, // completion zsh (cobra built-in)
}

// osTokens is the closed-set enum of OS values — exactly the runtime.GOOS
// constants. An Event.OS not in this set DROPS the event (it can only
// legitimately be runtime.GOOS at the emit site). Built once at init.
var osTokens = map[string]struct{}{
	"aix": {}, "android": {}, "darwin": {}, "dragonfly": {}, "freebsd": {},
	"hurd": {}, "illumos": {}, "ios": {}, "js": {}, "linux": {}, "nacl": {},
	"netbsd": {}, "openbsd": {}, "plan9": {}, "solaris": {}, "wasip1": {},
	"windows": {}, "zos": {},
	"": {}, // OS may legitimately be unset.
}

// canonicalVersion reports whether v is an acceptable version token and,
// if so, returns the CANONICAL value to STORE (A1). The empty string and
// "dev" map to themselves. A parseable version maps to its reconstructed
// core "Major.Minor.Patch" — NEVER the raw input — so any prerelease
// ("-…") or build ("+…") suffix is structurally discarded, tainted or
// not (repanel-leak: `version.Parse` validates only the core x.y.z and
// discards the suffix UNVALIDATED, so a secret/path/email smuggled after
// a '-'/'+' — e.g. "1.2.3-ghp_…", "v0.0.0+/Users/victim/.ssh/id_rsa",
// "1.0.0-secret@victim.com" — would otherwise survive verbatim in an
// ok=true event). ANYTHING else — a decorated cobra string, a path, a
// bare secret — is rejected (ok=false) so RedactEvent drops the event
// fail-closed. This closes the demonstrated `Version:"ghp_… /key …"`
// leak AND its prerelease/build-suffix resurrection.
func canonicalVersion(v string) (canon string, ok bool) {
	if v == "" || v == "dev" {
		return v, true
	}
	sv, parsed := version.Parse(v)
	if !parsed {
		return "", false
	}
	// Re-emit the parsed core ONLY; the raw input (and any tainted
	// prerelease/build suffix) is never stored. The suffix is never needed
	// downstream — resolved_in_vX compares core semver (Compare ignores the
	// suffix too).
	return itoa(sv.Major) + "." + itoa(sv.Minor) + "." + itoa(sv.Patch), true
}

// init guards against drift: runtime.GOOS must be in the OS allowlist on
// the build's own platform, else this build would drop all its own
// events.
func init() {
	if _, ok := osTokens[runtime.GOOS]; !ok {
		panic("redact: runtime.GOOS not in osTokens allowlist: " + runtime.GOOS)
	}
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
	// PEM private-key blocks FIRST — a multi-line "BEGIN ... PRIVATE KEY"
	// envelope (RSA/EC/OPENSSH/ENCRYPTED/generic). Matched before anything
	// can fragment the body on whitespace/newlines (R3, codex-leak). (?s)
	// so `.` spans the newline-broken base64 body.
	//
	// HARDENED (repanel-codex): a MALFORMED/PARTIAL envelope — e.g. a
	// "-----BEGIN OPENSSH PRIVATE KEY" line MISSING its trailing dashes, or
	// a truncated block with no END marker at all — leaked its body under
	// the old pass (which required the literal `PRIVATE KEY-----`). We now
	// (a) allow the trailing dashes after the marker to be ABSENT, and
	// (b) scrub from the BEGIN marker through the matching END marker OR,
	// if there is no END, to END-OF-STRING. Over-scrubbing a partial key
	// envelope is safe (HC-7: prefer DROP/over-scrub on uncertainty), and a
	// private-key body is never legitimately needed downstream.
	{"secret-pem", regexp.MustCompile(`(?is)-----BEGIN[ A-Z0-9]*PRIVATE KEY(?:-+)?.*?(?:-----END[ A-Z0-9]*PRIVATE KEY(?:-+)?|$)`), "<secret>"},
	// Authorization headers — Basic AND Bearer (R3, codex-leak: short
	// Basic credentials sit under the entropy backstop). Matched before
	// the entropy passes; the credential body is dropped regardless of
	// length. The scheme word is kept for context, the secret replaced.
	// Tolerate optional whitespace BEFORE and after the colon
	// (`Authorization : Basic …` bypassed the old `Authorization:` token —
	// repanel-codex), case-insensitive, and the Proxy-Authorization
	// variant. The header name is normalized to a canonical "<Name>: " in
	// the replacement so the residual gate's scheme check still matches.
	{"secret-auth-header", regexp.MustCompile(`(?i)\b(Proxy-Authorization|Authorization)\s*:\s*(Basic|Bearer)\s+\S+`), "$1: $2 <secret>"},
	// Secrets — provider-prefixed tokens and KEY/TOKEN/PASSWORD assignments.
	{"secret-ghp", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{16,}\b`), "<secret>"},
	{"secret-github-pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), "<secret>"},
	{"secret-gitlab-pat", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{16,}\b`), "<secret>"},
	{"secret-slack", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{8,}\b`), "<secret>"},
	{"secret-aws", regexp.MustCompile(`\bAKIA[0-9A-Z]{12,}\b`), "<secret>"},
	// AWS temporary/session credential ids (STS AssumeRole, instance-profile
	// creds) — same shape as AKIA long-term keys but the ASIA prefix (spec
	// 120 R9/AC-21).
	{"secret-aws-session", regexp.MustCompile(`\bASIA[0-9A-Z]{12,}\b`), "<secret>"},
	// GCP service-account JSON key files: the "private_key_id" field is not
	// itself secret, but its presence signals a service-account key blob
	// whose sibling "private_key" field IS (caught by the PEM pass above
	// when present); scrub the field+value together defensively (spec 120
	// R9/AC-21).
	{"secret-gcp-key-id", regexp.MustCompile(`(?i)"private_key_id"\s*:\s*"[^"]*"`), `"private_key_id": "<secret>"`},
	// Google API keys (AIza-prefixed, spec 120 R9/AC-21).
	{"secret-gcp-api-key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}\b`), "<secret>"},
	{"secret-openai", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`), "<secret>"},
	// Bearer outside an Authorization header (e.g. bare "Bearer <tok>").
	{"secret-bearer", regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]{8,}=*`), "Bearer <secret>"},
	// NOTE: there is deliberately NO bare "Basic <word>" pass — the word
	// "Basic" is common English ("Basic validation") and a bare matcher
	// over-scrubs prose. The real leak class is the Authorization header
	// (handled by secret-auth-header above); any long bare base64
	// credential is caught by the entropy backstop.
	{"secret-jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\b`), "<secret>"},
	// secret-assign is case-INSENSITIVE, so it already covers Azure
	// Storage's mixed-case `AccountKey=` connection-string credential
	// (spec 120 R9/AC-21 — verified by TestScrub_AzureAccountKey rather
	// than a new dedicated pass, since this one is fully redundant).
	{"secret-assign", regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:TOKEN|KEY|PASSWORD|SECRET))=\S+`), "${1}=<secret>"},
	// DSN / connection-string credential authority BEFORE email (the email
	// pass would otherwise eat only the `pass@host` half and leak the
	// username — confirm-codex-leak). Matches the `user:pass@host[:port][/db]`
	// authority in BOTH `scheme://user:pass@host…` (the URL pass below would
	// catch the rest, but the credentials must die first) AND bare
	// `user:pass@host…` forms (postgres/mysql/redis/mongodb URIs and the bare
	// `svc:pass@db` shape). The WHOLE authority — username, password, host,
	// optional port and database path — collapses to <dsn>, so no identifier
	// survives. The password run forbids ` :@/` so a non-credential email
	// ("max@cloudlete.ai", no `user:pass@`) does NOT match and falls through
	// to the email pass. (HC-7: over-scrub the whole authority on a DSN-shaped
	// match rather than risk a username/host leak.)
	{"dsn-credentials", regexp.MustCompile(`\b[A-Za-z0-9._%+-]+:[^\s:@/]+@[A-Za-z0-9.-]+(?::\d+)?(?:/[^\s]*)?`), "<dsn>"},
	// Emails before IPs/paths (the @ and dots are distinctive).
	{"email", regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`), "<email>"},
	// URLs (also neutralizes markdown auto-link bait).
	{"url", regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s)\]]+`), "<url>"},
	// IPv4.
	{"ipv4", regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`), "<ip>"},
	// IPv6 — full, compressed (`::`), and zone-scoped (`%en0`/`%eth0`) forms
	// (confirm-codex-leak: the old pass only matched fully-expanded colon
	// groups, so `fe80::1%en0`, `2001:db8::dead:beef`, and `::1` survived).
	//   - full:        (hex:){2,7}hex          e.g. fe80:0:0:0:1:2:3:4
	//   - compressed:  <left>?::<right>?        a LITERAL `::` is required, so
	//                  a lone `host:5432`, `a:b`, or `C:\…` cannot match (only
	//                  the timestamp shape `12:34:56` — already scrubbed by
	//                  the old full-form pass — and real `::` addresses do).
	//   - optional zone id `%iface` on either form.
	// No `\b` anchors: `::` legitimately abuts non-word chars at the edges.
	{"ipv6", regexp.MustCompile(`(?:(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4})*)?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4})*)?)(?:%[A-Za-z0-9._-]+)?`), "<ip>"},
	// Branch names BEFORE the path passes (so bead/<id> and spec/<slug>
	// become <branch>, not <path>).
	{"branch", regexp.MustCompile(`\b(?:bead|spec)/[A-Za-z0-9._/-]+`), "<branch>"},
	// Bead ids (mindspec-xxxx[.N]).
	{"bead-id", regexp.MustCompile(`\bmindspec-[a-z0-9]+(?:\.[0-9]+)?\b`), "<bead>"},
	// Spec slugs (NNN-word-word...).
	{"spec-slug", regexp.MustCompile(`\b\d{3}-[a-z0-9]+(?:-[a-z0-9]+)+\b`), "<spec>"},
	// ADR ids (ADR-NNNN) — a real identifier surfaced by the
	// adr-divergence templates (divergence.go:204-226).
	{"adr-id", regexp.MustCompile(`\bADR-\d{3,}\b`), "<adr>"},
	// UNC paths (\\server\share\...) BEFORE the drive-letter pass. A
	// leading \\ then backslash-separated segments that MAY contain
	// spaces. Terminates at a colon, quote, newline, or angle/pipe so
	// trailing prose ("...: permission denied") is not eaten as path
	// (R3, codex-leak). The whole path is replaced — never a suffix
	// fragment.
	{"abspath-unc", regexp.MustCompile(`\\\\[^\\\n:"'<>|?*]+(?:\\[^\\\n:"'<>|?*]*)*`), "<path>"},
	// Windows absolute paths WITH SPACES: drive letter then
	// backslash-separated segments that may contain spaces (e.g.
	// `C:\Users\Max\Secret Plans\domains`). The OLD pass stopped at the
	// first space, leaking suffix fragments like "Plans\domains"
	// (codex-leak). Same terminator set as UNC. The whole path is
	// replaced — never a fragment.
	{"abspath-win", regexp.MustCompile(`[A-Za-z]:(?:\\[^\\\n:"'<>|?*]*)+`), "<path>"},
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
	// Bare / space-separated OWNERSHIP domain list: an unbracketed,
	// unquoted rendering like `domains core git state for spec` leaks the
	// domain names (R3, codex-leak). Match `domain(s)` followed by a run
	// of bare lowercase words and collapse them to <domains>. Bounded to a
	// short run of words (the known domain set is small) so it does not
	// devour an entire sentence.
	{"domains-bare", regexp.MustCompile(`(?i)\bdomains?\s+(?:[a-z][a-z0-9-]*)(?:\s+[a-z][a-z0-9-]*){0,7}`), "domains <domains>"},
	// Bare file names with a known source/config extension (no slash).
	{"filename", regexp.MustCompile(`\b[\w.-]+\.(?:go|ya?ml|json|jsonl|md|toml|sh|ps1|xlsx|docx|pdf|csv|txt)\b`), "<file>"},
	// Entropy catch-all: long hex or base64-ish runs (token/secret
	// backstop). Thresholds tightened by one (15 hex / 23 b64) to catch a
	// realistic secret sitting one char under the old 16/24 cutoffs
	// (R3, codex-leak: TestEntropyUnderThreshold). 15 hex chars = 60 bits,
	// 23 b64 chars ≈ 137 bits — both squarely secret-length; legitimate
	// short ids (commit-short 7-12, NNN-prefixes) stay under 15.
	{"entropy-hex", regexp.MustCompile(`\b[0-9a-fA-F]{15,}\b`), "<token>"},
	{"entropy-b64", regexp.MustCompile(`\b[A-Za-z0-9+/_-]{23,}={0,2}\b`), "<token>"},
}

// residualLeakPasses are the patterns that, if STILL present after
// scrubbing, mean the scrub could not confidently classify the field —
// the field is then DROPPED (HC-7). They are the same dangerous shapes
// the golden corpus asserts ZERO of, so this gate is the in-library
// mirror of the CI gate.
var residualLeakPasses = []*regexp.Regexp{
	regexp.MustCompile(`[\w.@-]/[\w.@-]`),                                               // any slash path token
	regexp.MustCompile(`[A-Za-z]:\\`),                                                   // windows abs path
	regexp.MustCompile(`\\\\[^\\\s]`),                                                   // UNC path (\\server\…)
	regexp.MustCompile(`[^\s<>]\\[^\s<>\\]`),                                            // any residual backslash path segment
	regexp.MustCompile(`\bmindspec-[a-z0-9]`),                                           // bead id
	regexp.MustCompile(`\b\d{3}-[a-z0-9]+-[a-z0-9]`),                                    // spec slug
	regexp.MustCompile(`\bADR-\d{3,}`),                                                  // adr id
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]`),                                       // gh secret
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]`),                                     // github fine-grained pat
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]`),                                         // gitlab pat
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]`),                                     // slack token
	regexp.MustCompile(`\bAKIA[0-9A-Z]`),                                                // aws key
	regexp.MustCompile(`\bASIA[0-9A-Z]`),                                                // aws session key (spec 120 R9)
	regexp.MustCompile(`(?i)"private_key_id"\s*:\s*"[^<]`),                              // residual gcp private_key_id value (spec 120 R9)
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]`),                                           // gcp api key (spec 120 R9)
	regexp.MustCompile(`\bAccountKey=[^<]`),                                             // azure storage account key — defense-in-depth alongside secret-assign (spec 120 R9)
	regexp.MustCompile(`\bsk-[A-Za-z0-9]`),                                              // openai key
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{6,}\.`),                                      // jwt
	regexp.MustCompile(`(?i)BEGIN[ A-Z0-9]*PRIVATE KEY`),                                // residual PEM envelope — flags a surviving BEGIN…PRIVATE KEY marker even when the trailing dashes are absent (repanel-codex: malformed/partial envelope)
	regexp.MustCompile(`(?i)\bAuthorization\s*:\s*(Basic|Bearer)\s+[A-Za-z0-9+/=._~-]`), // residual auth-header credential (Authorization-scoped — also matches the Proxy-Authorization suffix; tolerates whitespace before/after the colon; avoids flagging the English word "Basic"; the emitted "<secret>" placeholder lacks the trailing credential char so it is not re-flagged)
	regexp.MustCompile(`\b[0-9a-fA-F]{15,}\b`),                                          // entropy hex (tightened)
	regexp.MustCompile(`[A-Za-z0-9+/_-]{23,}={0,2}`),                                    // entropy b64 (tightened)
	regexp.MustCompile(`@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),                                 // email
	// Residual DSN credential authority — a surviving `user:pass@host` (the
	// confirm-codex-leak class where the email pass left the username). The
	// scrubbed placeholder `<dsn>` has no `:pass@` so it is not re-flagged.
	regexp.MustCompile(`[A-Za-z0-9._%+-]+:[^\s:@/]+@[A-Za-z0-9.-]`),
	// Residual IPv6 — full, compressed (`::`) and zone-scoped forms. Mirrors
	// the ipv6 scrub pass so a surviving compressed/zoned address (the
	// confirm-codex-leak class) DROPS the field. The `<ip>` placeholder
	// contains no colon-hex run, so it is never self-flagged.
	regexp.MustCompile(`(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4})*)?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4})*)?`),
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

// errClassRe recognizes a sentinel error class/code at the head of a
// chain (e.g. "Dolt Error 1105", "Error 1105", "Error Code 23505").
var errClassRe = regexp.MustCompile(`(?i)\b((?:[a-z]+ )?error(?: code)? \d+)\b`)

// ScrubError is the `%w`/`%v` error-chain rule (Req 1). An error chain
// (errors.Error() flattens every wrapped layer) frequently drags a
// free-text payload the scrub cannot classify token-by-token — e.g. a
// Dolt-1105 carrying a bead DESCRIPTION, or wrapper prose BEFORE the
// code. The rule (hardened — R2, R3: the old behavior kept everything up
// to the code, leaking leading prose, and shipped wrapped free-text when
// no code was present):
//
//   - If a sentinel error class/code is present, UNWRAP to it: keep ONLY
//     the matched code token itself (e.g. "Dolt Error 1105"), discarding
//     ALL surrounding prose — both the leading wrapper context AND the
//     trailing wrapped message. This honors the docstring guarantee that
//     an arbitrary description cannot survive REGARDLESS of where the
//     wrappers sit relative to the code.
//   - Otherwise (NO recognizable code) the chain is arbitrary free-text
//     prose we cannot token-classify. Fail closed: scrub the WHOLE string
//     and let the residual-leak gate DROP it. Untokenizable English prose
//     has no dangerous shape to trip the gate, so it would otherwise
//     survive — we therefore DROP unconditionally (ok=false) to match the
//     "%w-discard" guarantee instead of shipping wrapper prose.
//
// A nil error scrubs to ("", true). A raw wrapped chain is NEVER shipped.
func ScrubError(err error) (clean string, ok bool) {
	if err == nil {
		return "", true
	}
	text := err.Error()
	if m := errClassRe.FindString(text); m != "" {
		// Keep ONLY the recognized code token; every wrapper (leading and
		// trailing) is discarded. Scrub the token itself defensively.
		return Scrub(m)
	}
	// No sentinel code: this is unclassifiable wrapper prose. Prefer DROP
	// (HC-7 fail-closed) over shipping arbitrary free text — untokenizable
	// English prose has no dangerous shape for the residual gate to catch,
	// so it would otherwise survive. Drop the whole chain.
	return "", false
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

// RedactEvent is the structured-enum-FIRST redaction (HC-2): the journal
// stores closed-set ENUM tokens, never arbitrary scrubbed strings.
// RedactEvent VALIDATES every enum field against its allowlist and DROPS
// the whole event (returns ok=false, HC-7 fail-closed) if ANY field
// carries a non-enum token — it never passes an arbitrary string through.
// This closes the enum-masquerade leaks (a secret/path smuggled into
// Command/Subcommand/Version/OS that merely "passed Scrub").
//
// Field rules:
//   - EscapeHatch ∈ EscapeHatchTokens else DROP (M4).
//   - Command ∈ CommandTokens else DROP.
//   - Subcommand ∈ SubcommandTokens else DROP.
//   - OS ∈ osTokens (runtime.GOOS values) else DROP.
//   - Version is a bare semver or "dev"/"" (validVersion) else DROP — a
//     decorated/tainted version string never survives (A1).
//   - Argv0 is reduced to basename BEFORE the scrub (M3) so the verbatim
//     home-dir/username invocation path is never returned; a non-
//     classifiable basename DROPS the event.
//
// Because the enum fields are validated (not scrubbed-then-trusted), the
// stored value is the verbatim closed-set token; no free text reaches the
// journal through this path.
func RedactEvent(ev Event) (RedactedEvent, bool) {
	// M4 / A2: every structured enum field MUST be a closed-set token;
	// anything else is a tainted value smuggled into the enum — drop.
	if _, allowed := EscapeHatchTokens[ev.EscapeHatch]; !allowed {
		return RedactedEvent{}, false
	}
	if _, allowed := CommandTokens[ev.Command]; !allowed {
		return RedactedEvent{}, false
	}
	if _, allowed := SubcommandTokens[ev.Subcommand]; !allowed {
		return RedactedEvent{}, false
	}
	if _, allowed := osTokens[ev.OS]; !allowed {
		return RedactedEvent{}, false
	}
	// A1: Version must be a bare semver or "dev"/""; a decorated or
	// tainted version string DROPS the event fail-closed. The STORED value
	// is the CANONICAL core "Major.Minor.Patch" reconstructed from the
	// parse — never the raw input — so any prerelease/build suffix
	// (tainted or not) is discarded (repanel-leak).
	canonVersion, okVersion := canonicalVersion(ev.Version)
	if !okVersion {
		return RedactedEvent{}, false
	}

	// M3: argv[0] → basename → scrub. argv[0] is the one field that is
	// genuinely user-influenced (the invocation path), so it is still
	// scrubbed rather than enum-validated.
	argv0, ok := Scrub(filepath.Base(ev.Argv0))
	if !ok {
		return RedactedEvent{}, false
	}

	id := Identity{Command: ev.Command, EscapeHatch: ev.EscapeHatch, Subcommand: ev.Subcommand}
	return RedactedEvent{
		Argv0:       argv0,
		Command:     ev.Command,
		EscapeHatch: ev.EscapeHatch,
		Subcommand:  ev.Subcommand,
		Version:     canonVersion,
		OS:          ev.OS,
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
