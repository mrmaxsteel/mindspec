// Package version is the normalized-version helper for the
// self-improvement loop (spec 094, Bead 1 / DQ4).
//
// The motivating problem (plan §Design Question 4): mindspec's
// cobra `--version` string is DECORATED — `version + " (" + commit +
// ") " + date` (cmd/mindspec/root.go:46) — and its commit hash is
// exactly the high-entropy run the redaction entropy catch-all will
// SCRUB. The bare `version` package var (root.go:35) defaults to
// "dev" on every non-release/local/test build (the builds the
// motivating autopilot evidence came from). A naive `version >= X`
// comparison on the decorated string is therefore UNDEFINED exactly
// where the loop must work.
//
// This helper captures and compares the BARE semver and pins the
// dev/unparseable policy so the Req 3 regression/stale loop is defined
// on dev builds.
//
// # Current() and the bare version var
//
// The §API Contract pins `Current()` to return the bare `version`
// package var (root.go:35), NOT the decorated cobra string. Bead 1
// CANNOT import `package main` (cmd/mindspec) and MUST NOT edit
// root.go (that is Bead 2's file), so the single source of truth lives
// here as `current` (defaulting to "dev", matching root.go:35's
// default) with a `Set` seam. Bead 2 wires the ldflags-injected
// `main.version` into it via `version.Set(version)` at startup. See
// the report's DESIGN-CALL flag.
package version

import (
	"strconv"
	"strings"
	"sync"
)

// current is the bare semver, single source of truth for Current().
// Defaults to "dev" to match cmd/mindspec/root.go:35 on every
// non-release/local/test build. Bead 2 wires main.version in via Set.
//
// It is guarded by mu so Current()/Set() are safe to call from multiple
// goroutines. The §API Contract documents Set as a startup-only seam,
// but a privacy keystone must not race regardless of caller discipline
// (codex-correctness, r2-correctness): a torn read of the version that
// is stamped on every journal entry is unacceptable, so the access is
// synchronized rather than relying on the precondition.
var (
	mu      sync.RWMutex
	current = "dev"
)

// Current returns the bare semver string (e.g. "1.4.2" or "dev"), NOT
// the decorated cobra `--version` string. This is the value stamped on
// every journal entry and friction report (Req 3). Safe for concurrent
// use with Set.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set wires the build's bare version (cmd/mindspec/root.go:35's
// ldflags-injected `version` var) into the helper. Idempotent; a blank
// argument is ignored so a caller that passes an unset var cannot
// clobber the "dev" default. Bead 2 calls this once at startup. Safe for
// concurrent use with Current.
func Set(v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}
	mu.Lock()
	current = v
	mu.Unlock()
}

// Semver is a parsed major.minor.patch version. Pre-release and build
// metadata (the `-rc1` / `+meta` suffixes) are intentionally discarded:
// the loop compares release ordering only.
type Semver struct {
	Major int
	Minor int
	Patch int
}

// Parse parses a bare semver string into a Semver. It accepts an
// optional leading "v" and tolerates a `-prerelease` / `+build`
// suffix (the core x.y.z is parsed, the suffix discarded).
//
// ok == false for "dev", the empty string, or any unparseable input —
// the DQ4 "dev/unparseable" class the caller must treat as
// unbounded-newest (fail toward surfacing a regression).
func Parse(s string) (Semver, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "dev") {
		return Semver{}, false
	}
	s = strings.TrimPrefix(s, "v")
	// Drop any pre-release / build metadata suffix.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return Semver{}, false
	}
	nums := make([]int, 3)
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			nums[i] = 0
			continue
		}
		p := parts[i]
		if p == "" {
			return Semver{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Semver{}, false
		}
		nums[i] = n
	}
	return Semver{Major: nums[0], Minor: nums[1], Patch: nums[2]}, true
}

// Compare compares two bare version strings.
//
// When BOTH parse as concrete semver, it returns -1, 0, or +1 (a < b,
// a == b, a > b) with ok == true.
//
// When EITHER operand is "dev"/unparseable, the comparison is not
// well-defined and Compare returns (0, false). This is the DQ4
// dev→unbounded-newest seam: a `false` second return is the signal the
// caller (Bead 3) resolves as "fail toward surfacing" — i.e. classify
// REGRESSION, never stale. Concretely the four DQ4 outcomes are:
//
//   - running dev vs resolved vX        → ok=false → REGRESSION
//   - resolved dev vs running vX        → ok=false → REGRESSION
//   - running == resolved (concrete)    → cmp==0  → REGRESSION (≥ boundary)
//   - running > resolved / running < X  → cmp>0 / cmp<0 → regression / stale
//
// Pinning the dev case as not-cleanly-comparable (rather than mapping
// dev to a single point on the order) is the only self-consistent
// reading of the two DQ4 statements ("incoming dev → regression" AND
// "stored dev → any later concrete is a regression"), which a total
// order cannot satisfy simultaneously. See the report's DESIGN-CALL.
func Compare(a, b string) (int, bool) {
	av, aok := Parse(a)
	bv, bok := Parse(b)
	if !aok || !bok {
		return 0, false
	}
	switch {
	case av.Major != bv.Major:
		return sign(av.Major - bv.Major), true
	case av.Minor != bv.Minor:
		return sign(av.Minor - bv.Minor), true
	case av.Patch != bv.Patch:
		return sign(av.Patch - bv.Patch), true
	default:
		return 0, true
	}
}

func sign(d int) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	default:
		return 0
	}
}
