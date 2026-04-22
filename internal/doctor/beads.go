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
	cmd := exec.Command("git", "ls-files", ".beads/")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// git not available or not a git repo — skip with warning
		r.Checks = append(r.Checks, Check{
			Name:    "Beads runtime artifacts",
			Status:  Warn,
			Message: "could not run git ls-files (git not available or not a repo)",
		})
		return
	}

	tracked := strings.TrimSpace(string(out))
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

// bdVersionFloor is the minimum supported bd version. Earlier versions lack
// worktree-redirect fixes that mindspec relies on (v1.0.2 bundles the fixes).
const bdVersionFloor = "1.0.2"

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
// is GIT_DIR-pollution leakage from bd v1.0.2's default auto-add behaviour
// (see .beads/config.yaml header). The canonical location is
// .beads/issues.jsonl.
func checkStrayRootJSONL(r *Report, root string) {
	if !dirExists(filepath.Join(root, ".git")) && !fileExists(filepath.Join(root, ".git")) {
		return
	}

	cmd := exec.Command("git", "ls-files", "--full-name", "--", "issues.jsonl")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return
	}
	if strings.TrimSpace(string(out)) == "" {
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
				"introduced in v1.0.2; upgrade with `brew upgrade beads`", ver, bdVersionFloor),
		})
		return
	}

	r.Checks = append(r.Checks, Check{
		Name:    "bd version floor",
		Status:  OK,
		Message: fmt.Sprintf("bd %s >= %s", ver, bdVersionFloor),
	})
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
// be parsed, or doesn't declare the key — treat as bd's default (true) in
// callers where the distinction matters.
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
		return true, false
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
// repo. It tries, in order: `sync.remote` in .beads/config.yaml, then
// .beads/dolt/.dolt/repo_state.json (which lists remote branches). Returns
// (known=false, ...) when detection cannot be performed confidently.
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

	repoState := filepath.Join(root, ".beads", "dolt", ".dolt", "repo_state.json")
	if data, err := os.ReadFile(repoState); err == nil {
		var state struct {
			Remotes map[string]any `json:"remotes"`
		}
		if json.Unmarshal(data, &state) == nil {
			return true, len(state.Remotes) > 0
		}
	}

	return false, false
}
