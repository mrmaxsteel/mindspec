package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"gopkg.in/yaml.v3"
)

// runtimePatterns are Beads runtime filenames that should not be git-tracked.
var runtimePatterns = map[string]bool{
	"bd.sock":         true,
	"daemon.lock":     true,
	"daemon.log":      true,
	"daemon.pid":      true,
	"sync-state.json": true,
	"last-touched":    true,
	".local_version":  true,
	"db.sqlite":       true,
	"bd.db":           true,
	"redirect":        true,
	".sync.lock":      true,
}

// runtimeExtensions are file extensions for Beads runtime artifacts.
var runtimeExtensions = []string{".db", ".db-wal", ".db-shm", ".db-journal"}

// durableFiles are expected Beads durable state files.
var durableFiles = []string{"issues.jsonl", "config.yaml", "metadata.json"}

func checkBeads(r *Report, root string) {
	beadsDir := filepath.Join(root, ".beads")

	if !dirExists(beadsDir) {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads",
			Status:  Missing,
			Message: ".beads/ directory not found — run `beads init`",
		})
		return
	}

	r.Checks = append(r.Checks, Check{Name: "Beads", Status: OK, Message: ".beads/ directory exists"})

	// Check durable state files
	var found []string
	for _, f := range durableFiles {
		if fileExists(filepath.Join(beadsDir, f)) {
			found = append(found, f)
		}
	}
	if len(found) > 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads durable state",
			Status:  OK,
			Message: fmt.Sprintf("(%s)", strings.Join(found, ", ")),
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads durable state",
			Status:  Missing,
			Message: "no durable state files found (issues.jsonl, config.yaml, metadata.json)",
		})
	}

	// Check for git-tracked runtime artifacts
	checkTrackedRuntime(r, root)
}

func checkTrackedRuntime(r *Report, root string) {
	out, err := gitutil.LsFiles(root, ".beads/")
	if err != nil {
		// git not available or not a git repo — skip with warning
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  Warn,
			Message: "could not run git ls-files (git not available or not a repo)",
		})
		return
	}

	tracked := strings.TrimSpace(out)
	if tracked == "" {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  OK,
			Message: "none tracked by git",
		})
		return
	}

	var violations []string
	for _, line := range strings.Split(tracked, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		filename := filepath.Base(line)
		if isRuntimeArtifact(filename) {
			violations = append(violations, line)
		}
	}

	if len(violations) > 0 {
		msg := fmt.Sprintf("tracked by git: %s — add to .beads/.gitignore and run `git rm --cached <file>`",
			strings.Join(violations, ", "))
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  Error,
			Message: msg,
		})
	} else {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  OK,
			Message: "none tracked by git",
		})
	}
}

func isRuntimeArtifact(filename string) bool {
	if runtimePatterns[filename] {
		return true
	}
	for _, ext := range runtimeExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

// bdVersionFloor is the minimum supported bd version. v1.0.2 shipped the
// worktree-redirect fixes mindspec relies on (and is the floor below which
// `bd list --json` may emit non-JSON, which bead.ListJSON rejects rather
// than fall back to scraping). v1.0.4 ships embedded Dolt mode — mindspec
// (including the test harness) assumes embedded mode and no longer carries
// server-mode plumbing, so the floor is 1.0.4. Keep bead.minBdVersionMsg
// in sync.
const bdVersionFloor = "1.0.4"

// checkBeadsConfigDrift reports missing or drifted mindspec-required keys in
// .beads/config.yaml. When a drift exists, a FixFunc is attached that calls
// bead.EnsureBeadsConfig with the caller-supplied force flag:
//   - force=false: adds missing keys, leaves user-authored drift alone
//   - force=true: also replaces user-authored values for required keys
func checkBeadsConfigDrift(r *Report, root string, force bool) {
	// Skip silently when .beads/ itself is absent — checkBeads already flagged that.
	if !dirExists(filepath.Join(root, ".beads")) {
		return
	}

	res, err := bead.ScanBeadsConfig(root)
	if err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads config drift",
			Status:  Warn,
			Message: fmt.Sprintf("cannot scan .beads/config.yaml: %v", err),
		})
		return
	}

	fix := func() error {
		_, err := bead.EnsureBeadsConfig(root, force)
		return err
	}

	if res.CreatedFile {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads config drift",
			Status:  Warn,
			Message: ".beads/config.yaml not found — run `mindspec doctor --fix` to create one",
			FixFunc: fix,
		})
		return
	}

	if len(res.Added) == 0 && len(res.UserAuthored) == 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads config drift",
			Status:  OK,
			Message: "all mindspec-required keys present",
		})
		return
	}

	var parts []string
	for _, k := range res.Added {
		parts = append(parts, fmt.Sprintf("missing %s", k))
	}
	for _, d := range res.UserAuthored {
		parts = append(parts, fmt.Sprintf("%s=%q (want %v)", d.Key, d.HaveRaw, d.Want))
	}
	msg := strings.Join(parts, "; ")

	switch {
	case len(res.UserAuthored) > 0 && len(res.Added) > 0:
		msg += " — run `mindspec doctor --fix` to add missing keys; `--fix --force` to also replace user-authored values"
	case len(res.UserAuthored) > 0:
		msg += " — run `mindspec doctor --fix --force` to replace user-authored values"
	default:
		msg += " — run `mindspec doctor --fix` to add them"
	}

	r.Checks = append(r.Checks, Check{
		Name:    "Beads config drift",
		Status:  Warn,
		Message: msg,
		FixFunc: fix,
	})
}

// checkStrayRootJSONL warns when <root>/issues.jsonl is tracked by git. This
// is GIT_DIR-pollution leakage from bd v1.0.2's default auto-add behavior
// (see .beads/config.yaml header). The canonical location is
// .beads/issues.jsonl.
func checkStrayRootJSONL(r *Report, root string) {
	if !dirExists(filepath.Join(root, ".git")) && !fileExists(filepath.Join(root, ".git")) {
		return
	}

	out, err := gitutil.LsFilesFullName(root, "issues.jsonl")
	if err != nil {
		return
	}
	if strings.TrimSpace(out) == "" {
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:   "Stray root issues.jsonl",
		Status: Warn,
		Message: "root-level issues.jsonl is tracked by git — run `git rm --cached issues.jsonl` " +
			"(cross-branch cleanup out of scope; the canonical file is .beads/issues.jsonl)",
	})
}

// checkDurabilityRisk warns when auto-export is disabled AND no Dolt remote
// is configured. In that state, ad-hoc `bd create` sessions outside
// mindspec's approve/complete flow won't refresh .beads/issues.jsonl and
// won't push to Dolt either, so work is only durable on the local machine.
func checkDurabilityRisk(r *Report, root string) {
	if !dirExists(filepath.Join(root, ".beads")) {
		return
	}

	autoExport, autoKnown := readExportAuto(root)
	if !autoKnown || autoExport {
		return
	}

	remoteKnown, hasRemote := detectDoltRemote(root)
	if !remoteKnown {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads durability",
			Status:  OK,
			Message: "skipped — could not determine Dolt remote configuration",
		})
		return
	}
	if hasRemote {
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:   "Beads durability",
		Status: Warn,
		Message: "export.auto: false AND no Dolt remote configured — ad-hoc `bd create` outside " +
			"mindspec's approve/complete flow won't refresh issues.jsonl or push; configure a Dolt " +
			"remote or revert `export.auto` to true",
	})
}

// checkBdVersionFloor warns when `bd --version` reports below the minimum
// supported version. Skips silently on parse failure — do not false-warn.
func checkBdVersionFloor(r *Report, root string) {
	cmd := exec.Command("bd", "--version")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return
	}

	ver, ok := parseBdVersion(string(out))
	if !ok {
		r.Checks = append(r.Checks, Check{
			Name:    "bd version floor",
			Status:  OK,
			Message: "skipped — could not parse `bd --version` output",
		})
		return
	}

	if compareSemver(ver, bdVersionFloor) < 0 {
		r.Checks = append(r.Checks, Check{
			Name:   "bd version floor",
			Status: Warn,
			Message: fmt.Sprintf("bd %s is below minimum %s — mindspec relies on worktree redirect fixes "+
				"(v1.0.2) and embedded Dolt mode (v1.0.4); upgrade with `brew upgrade beads`", ver, bdVersionFloor),
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:    "bd version floor",
		Status:  OK,
		Message: fmt.Sprintf("bd %s >= %s", ver, bdVersionFloor),
	})
}

// bdSchemaDriftRE matches the schema-error class a stale bd binary emits when
// its compiled-in schema expectation diverges from the on-disk DB — e.g.
// `column "depends_on_id" could not be found`. The version-floor check
// (checkBdVersionFloor) cannot catch this: a binary can report a fresh
// `--version` yet still query columns the live DB lacks (or vice versa).
//
// Anchored on a small set of DISTINCTIVE missing-column/table signatures so
// unrelated runtime failures don't masquerade as drift:
//   - `(column|table) "<name>" could not be found` — the Dolt phrasing.
//   - `no such column` / `no such table`           — SQLite.
//   - `unknown column` / `unknown table`            — MySQL / Dolt.
//   - `Error 1054`                                  — the MySQL unknown-column
//     error code (often emitted without the word "column" in front).
//
// Each alternative names "column"/"table" (or the 1054 code) explicitly, so a
// benign transient error (connection refused, db locked, deadline exceeded,
// "some unrelated runtime failure") still does NOT match and the probe stays
// OK/skip rather than false-warning.
var bdSchemaDriftRE = regexp.MustCompile(`(?i)((column|table)\s+"?[\w.]+"?\s+could not be found|no such (column|table)|unknown (column|table)|error 1054)`)

// checkBdSchemaDrift runs a cheap read-only bd probe and, when it fails with a
// recognizable schema-error class, warns that the bd binary's schema
// expectation has drifted from the DB. This complements checkBdVersionFloor,
// which only compares `bd --version` against a floor and never executes a
// schema-touching query. Behavior:
//   - probe succeeds                → OK (schema healthy)
//   - probe fails with a drift sig  → Warn (surface bd's real output)
//   - probe missing / fails without
//     a drift signature             → OK/skip (never false-warn)
func checkBdSchemaDrift(r *Report, root string) {
	if _, err := exec.LookPath("bd"); err != nil {
		r.Checks = append(r.Checks, Check{
			Name:    "bd schema drift",
			Status:  OK,
			Message: "skipped — bd not found on PATH",
		})
		return
	}

	// `bd list --json` reads issues (touching the schema) without mutating
	// anything; it's the same path bead.ListJSON relies on.
	cmd := exec.Command("bd", "list", "--json")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		r.Checks = append(r.Checks, Check{
			Name:    "bd schema drift",
			Status:  OK,
			Message: "bd schema matches the DB (read probe succeeded)",
		})
		return
	}

	combined := strings.TrimSpace(string(out))
	if !bdSchemaDriftRE.MatchString(combined) {
		// A failure with no recognizable schema signature is not drift —
		// skip rather than false-warn on an unrelated transient error.
		r.Checks = append(r.Checks, Check{
			Name:    "bd schema drift",
			Status:  OK,
			Message: "skipped — bd read probe failed without a schema-drift signature",
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:   "bd schema drift",
		Status: Warn,
		Message: fmt.Sprintf("the `bd` binary's schema expectation has drifted from the DB: %s — "+
			"the version floor check cannot catch this; upgrade/reinstall bd so its schema matches "+
			"(e.g. `brew upgrade beads`) and check for a stale shadowing binary", combined),
	})
}

// checkMultipleBdOnPath warns when more than one `bd` executable is reachable
// on PATH. The 2026 incident root cause was a stale `~/.local/bin/bd`
// shadowing the Homebrew binary, so the resolved `bd` was an old schema-drifted
// build even though `brew upgrade` had run. One bd → OK; zero → skip. Each PATH
// entry is counted once (duplicate dirs collapse) so a doubled PATH doesn't
// false-warn.
func checkMultipleBdOnPath(r *Report, _ string) {
	pathEnv := os.Getenv("PATH")
	var found []string
	seenDir := map[string]bool{}
	seenResolved := map[string]bool{}
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		if seenDir[dir] {
			continue
		}
		seenDir[dir] = true

		cand := filepath.Join(dir, "bd")
		info, err := os.Stat(cand)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		// Collapse symlinks/hardlinks pointing at the same real file so a
		// single binary reachable via two PATH entries isn't double-counted.
		resolved := cand
		if rp, err := filepath.EvalSymlinks(cand); err == nil {
			resolved = rp
		}
		if seenResolved[resolved] {
			continue
		}
		seenResolved[resolved] = true
		found = append(found, cand)
	}

	if len(found) <= 1 {
		msg := "exactly one `bd` on PATH"
		if len(found) == 0 {
			msg = "skipped — no `bd` found on PATH"
		}
		r.Checks = append(r.Checks, Check{
			Name:    "bd on PATH",
			Status:  OK,
			Message: msg,
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:   "bd on PATH",
		Status: Warn,
		Message: fmt.Sprintf("%d `bd` binaries on PATH (%s) — the first shadows the rest, so an upgrade of a "+
			"later one is silently ignored (the stale-shadowing root cause); remove the stale binary so a single "+
			"`bd` resolves", len(found), strings.Join(found, ", ")),
	})
}

// beadsMergeDriverScript is the repo-relative path of the tracked wrapper
// that regenerates .beads/issues.jsonl from the Dolt DB on merge (ADR-0025:
// the jsonl is a deterministic projection, so regenerate-from-DB is the
// correct merge). It replaced the orphaned `bd merge` driver removed in
// bd 1.0.x (incident 2026-06-11, mindspec-oe0u).
const beadsMergeDriverScript = "scripts/bd-jsonl-merge-driver.sh"

// beadsMergeAttrPattern is the exact .gitattributes pattern field that maps
// the beads jsonl to the merge=beads driver. Detection requires fields[0] to
// equal this so a corrupted/concatenated line (e.g. a newline-unsafe append
// producing `*.png binary.beads/issues.jsonl merge=beads`) is NOT falsely
// reported as a valid mapping (mindspec-oe0u).
const beadsMergeAttrPattern = ".beads/issues.jsonl"

// recoveryLine formats the agent-contract failure footer: a final line of
// the form `recovery: <command>` naming the exact command to run. Matches
// the convention used by scripts/bd-jsonl-merge-driver.sh; hand-rolled here
// because internal/guard exports no formatter for it (doctor is not an
// ADR-0030 enforcement package, so the import itself would be legal — the
// helper just doesn't exist there).
func recoveryLine(command string) string {
	return "\nrecovery: " + command
}

// checkBeadsMergeDriver validates the merge.beads.driver git config against
// the merge=beads attribute in .gitattributes. Two failure classes from the
// 2026-06-11 incident (mindspec-oe0u):
//
//  1. A configured driver whose command does not resolve/execute — e.g. the
//     orphaned `bd merge %A %O %A %B` form after the bd 1.0.x upgrade
//     removed that subcommand. Every both-sides-changed merge of
//     .beads/issues.jsonl fails loudly (path left unmerged with 3 stages).
//  2. A merge=beads attribute with NO driver configured — git silently
//     falls back to a plain TEXT merge of the jsonl. The more dangerous
//     sibling: silent semantic corruption instead of a loud failure. This
//     one is fixable: --fix writes the config pointing at the tracked
//     wrapper script via a PORTABLE repo-relative path (mindspec-oe0u),
//     when that script exists in the repo; when it doesn't, the check stays
//     ERROR with the manual command.
//
// GitHub-PR-merge residual: PR merges performed on GitHub's servers never
// run a local merge driver, so a both-sides-changed .beads/issues.jsonl can
// still land text-merged after a web merge. That is compensated by the
// post-merge beads-sync pattern (regenerate-from-DB on pull), not by this
// check — documented here, not fixed (mindspec-oe0u, ADR-0025).
//
// The inverse hole is flagged too: a configured driver with NO merge=beads
// attribute in .gitattributes — git text-merges the jsonl despite the
// configured driver. A repo with neither the attribute nor the driver is
// silent — there is nothing to validate.
func checkBeadsMergeDriver(r *Report, root string) {
	hasAttr := gitattributesHasBeadsMerge(root)
	driver, configured := readGitConfig(root, "merge.beads.driver")

	_, scriptExists := mergeDriverScriptPath(root)
	// Write a PORTABLE repo-relative driver value (mindspec-oe0u), NOT a
	// machine-specific absolute path: resolveDriverCommand resolves a
	// relative path containing '/' against the worktree top-level, and git
	// runs merge drivers from the worktree root, so this single shared
	// .git/config value is valid from every linked worktree AND every fresh
	// clone. --fix therefore CONVERGES an existing absolute value to the
	// portable form instead of re-baking an absolute one. Single-quoted so
	// the value round-trips through driverTokens and survives a repo path
	// that contains spaces.
	wantDriver := "'" + beadsMergeDriverScript + "' %A %O %B"

	if !configured || strings.TrimSpace(driver) == "" {
		if !hasAttr {
			return
		}
		if scriptExists {
			r.Checks = append(r.Checks, Check{
				Name:   "Beads merge driver",
				Status: Error,
				Message: ".gitattributes maps .beads/issues.jsonl to merge=beads but merge.beads.driver is not " +
					"configured — git silently falls back to a plain text merge of the jsonl (silent semantic " +
					"corruption of same-record divergence); run `mindspec doctor --fix` to point it at " +
					beadsMergeDriverScript +
					recoveryLine(fmt.Sprintf("git config merge.beads.driver %q", wantDriver)),
				FixFunc: func() error {
					return writeGitConfig(root, "merge.beads.driver", wantDriver)
				},
			})
			return
		}
		// No FixFunc here — the wrapper script is missing, so there is
		// nothing safe to point the config at. Still ERROR, not Warn: this
		// unfixable state is strictly worse than the fixable sibling above.
		r.Checks = append(r.Checks, Check{
			Name:   "Beads merge driver",
			Status: Error,
			Message: ".gitattributes maps .beads/issues.jsonl to merge=beads but merge.beads.driver is not " +
				"configured and " + beadsMergeDriverScript + " does not exist in this repo — git silently " +
				"text-merges the jsonl; restore the wrapper script, then configure the driver manually" +
				recoveryLine("git config merge.beads.driver '<abs-path-to>/bd-jsonl-merge-driver.sh %A %O %B'"),
		})
		return
	}

	// Inverse hole: driver configured but the merge=beads attribute is gone
	// from .gitattributes — git never consults the driver and silently
	// text-merges the jsonl despite the healthy-looking config.
	if !hasAttr {
		r.Checks = append(r.Checks, Check{
			Name:   "Beads merge driver",
			Status: Error,
			Message: fmt.Sprintf("merge.beads.driver is configured (%q) but .gitattributes has no merge=beads "+
				"mapping for .beads/issues.jsonl — git silently text-merges the jsonl despite the configured "+
				"driver", driver) +
				recoveryLine(`printf '.beads/issues.jsonl merge=beads\n' >> .gitattributes`),
		})
		return
	}

	recovery := recoveryLine(fmt.Sprintf("git config merge.beads.driver %q", wantDriver))
	if !scriptExists {
		recovery = recoveryLine("git config merge.beads.driver '<abs-path-to>/bd-jsonl-merge-driver.sh %A %O %B'")
	}

	toks := driverTokens(driver)
	if len(toks) == 0 {
		r.Checks = append(r.Checks, Check{
			Name:    "Beads merge driver",
			Status:  Error,
			Message: "merge.beads.driver is configured but empty — merges of .beads/issues.jsonl will fail" + recovery,
		})
		return
	}

	// The incident's exact shape: `bd merge ...` survived the bd 1.0.x
	// upgrade in .git/config even though the subcommand was removed. The
	// `bd` binary itself resolves on PATH, so the executable-existence
	// gate below would pass — flag the dead subcommand explicitly.
	if filepath.Base(toks[0]) == "bd" && len(toks) > 1 && toks[1] == "merge" {
		r.Checks = append(r.Checks, Check{
			Name:   "Beads merge driver",
			Status: Error,
			Message: fmt.Sprintf("merge.beads.driver %q invokes `bd merge`, which was removed in bd 1.0.x — "+
				"every both-sides-changed merge of .beads/issues.jsonl fails, leaving the path unmerged "+
				"(incident 2026-06-11)", driver) + recovery,
		})
		return
	}

	if err := resolveDriverCommand(root, toks[0]); err != nil {
		r.Checks = append(r.Checks, Check{
			Name:   "Beads merge driver",
			Status: Error,
			Message: fmt.Sprintf("merge.beads.driver %q: %v — merges of .beads/issues.jsonl will fail",
				driver, err) + recovery,
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:    "Beads merge driver",
		Status:  OK,
		Message: fmt.Sprintf("merge.beads.driver resolves (%s)", toks[0]),
	})
}

// gitattributesHasBeadsMerge reports whether <root>/.gitattributes assigns
// the merge=beads attribute to any pattern. Only the top-level file is
// scanned — that is where mindspec (and bd) write it.
func gitattributesHasBeadsMerge(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != beadsMergeAttrPattern {
			continue
		}
		for _, attr := range fields[1:] {
			if attr == "merge=beads" {
				return true
			}
		}
	}
	return false
}

// mergeDriverScriptPath returns the absolute path of the tracked wrapper
// script and whether it exists as a regular file in this repo.
func mergeDriverScriptPath(root string) (abs string, exists bool) {
	p := filepath.Join(root, beadsMergeDriverScript)
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return abs, false
	}
	return abs, true
}

// driverTokens splits a git merge-driver command line into tokens with
// shell-style single/double quoting honored (git runs the driver via sh).
// No escape processing beyond quote pairing — driver lines are simple.
func driverTokens(s string) []string {
	var toks []string
	var cur strings.Builder
	var quote rune // 0 when outside quotes
	inTok := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inTok = true
		case r == ' ' || r == '\t':
			if inTok {
				toks = append(toks, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	if inTok {
		toks = append(toks, cur.String())
	}
	return toks
}

// resolveDriverCommand checks that the first token of a merge-driver
// command resolves to an executable. Tokens containing a path separator
// are treated as paths (relative ones resolve against the worktree
// top-level, which is where git runs merge drivers); bare names go
// through PATH lookup.
func resolveDriverCommand(root, tok string) error {
	if strings.Contains(tok, "/") {
		path := tok
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("command %s does not exist", path)
		}
		if info.IsDir() {
			return fmt.Errorf("command %s is a directory", path)
		}
		if info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("command %s is not executable", path)
		}
		return nil
	}
	if _, err := exec.LookPath(tok); err != nil {
		return fmt.Errorf("command %q not found on PATH", tok)
	}
	return nil
}

// readGitConfig returns the value of a git config key for the repo at
// root. ok=false when the key is unset or git is unavailable.
func readGitConfig(root, key string) (string, bool) {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// writeGitConfig sets a git config key for the repo at root. In a linked
// worktree this lands in the shared .git/config, covering main and all
// worktrees at once.
func writeGitConfig(root, key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config %s: %v: %s", key, err, strings.TrimSpace(string(out)))
	}
	return nil
}

var bdVersionRE = regexp.MustCompile(`\bv?([0-9]+)\.([0-9]+)\.([0-9]+)`)

// parseBdVersion extracts the first dotted triple from `bd --version` output.
// Accepts both `bd version 1.0.2 (Homebrew)` and `v1.0.2` shapes.
func parseBdVersion(s string) (string, bool) {
	m := bdVersionRE.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3]), true
}

// compareSemver returns -1, 0, or 1 for a vs b. Both inputs must be
// three-part dotted numeric versions; non-numeric components sort as 0.
func compareSemver(a, b string) int {
	pa := splitSemver(a)
	pb := splitSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func splitSemver(s string) [3]int {
	var out [3]int
	parts := strings.SplitN(s, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}

// readExportAuto parses .beads/config.yaml for the export.auto key.
// Returns (value, known). `known=false` means the file doesn't exist, can't
// be parsed, or doesn't declare the key. When the distinction matters,
// callers should treat `known=false` as "assume bd's default (true)" — we
// return the zero value (false) rather than a misleading true so a caller
// that forgets the `known` check doesn't silently flip behavior.
func readExportAuto(root string) (bool, bool) {
	path := filepath.Join(root, ".beads", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false, false
	}
	raw, ok := cfg["export.auto"]
	if !ok {
		return false, false
	}
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "on":
			return true, true
		case "false", "no", "off":
			return false, true
		}
	}
	return false, false
}

// detectDoltRemote reports whether a Dolt remote is configured for this
// repo. Fallback order:
//  1. `sync.remote` in .beads/config.yaml (bd's own remote-sync config)
//  2. .beads/dolt/.dolt/repo_state.json → `remotes` map (the canonical
//     location on bd 1.0.2; verified against a live install)
//  3. .beads/dolt/.dolt/config.json → `remotes` map (legacy / per-repo
//     global config fallback — some Dolt versions and fresh repos leave
//     repo_state absent)
//
// Returns (known=false, ...) only when none of the three surfaces yielded
// a parseable answer. The hasRemote return is meaningful only when known.
func detectDoltRemote(root string) (known bool, hasRemote bool) {
	cfgPath := filepath.Join(root, ".beads", "config.yaml")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg map[string]any
		if yaml.Unmarshal(data, &cfg) == nil {
			if v, ok := cfg["sync.remote"]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return true, true
				}
			}
		}
	}

	for _, rel := range []string{
		filepath.Join(".beads", "dolt", ".dolt", "repo_state.json"),
		filepath.Join(".beads", "dolt", ".dolt", "config.json"),
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		var state struct {
			Remotes map[string]any `json:"remotes"`
		}
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}
		// Only treat config.json as authoritative when it declares remotes;
		// an empty config.json tells us nothing and shouldn't stop the
		// next fallback. repo_state.json is always authoritative when
		// parseable — an empty remotes map there means "no remotes."
		if strings.HasSuffix(rel, "repo_state.json") {
			return true, len(state.Remotes) > 0
		}
		if len(state.Remotes) > 0 {
			return true, true
		}
	}

	return false, false
}
