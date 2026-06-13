package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec 094 Bead 4 — feedback-remote config contract (Req 6 / HC-3).
//
// This is the GLOBAL/USER-SCOPED feedback-remote config surface. It is a
// DISTINCT, net-new API from the repo-local Load(root) above: today's Load
// reads ONLY the project `.mindspec/config.yaml` under the repo root, and the
// feedback-remote setting is DELIBERATELY absent from that project-scoped
// Config struct so a project-committed feedback-remote config is IGNORED,
// never honored (a committed remote would leak the URL even though the
// credential gates the push — HC-3).
//
// v1 ships the CONTRACT only: the actual cross-install REMOTE PUSH is deferred
// (spec §Non-Goals). What ships now is (a) the global-only config loading and
// (b) the FAIL-CLOSED resolution — absent the machine-global push credential
// the owner/remote path is unavailable, so the caller writes local-only and
// NEVER falls back to "push anyway" or a wrong remote. The scope + fail-closed
// properties land NOW so the follow-on push cannot regress them.
//
// This package collects and renders NO friction data — it only defines and
// loads the config and resolves the fail-closed push target. That privacy
// isolation is why Bead 4 is dependency-free (no internal/redact, no
// internal/journal).

// FeedbackRemoteEnvURL overrides the global-config feedback-remote URL from the
// environment. Precedence is env > global config; the project-committed config
// is NEVER consulted for this setting (HC-3).
const FeedbackRemoteEnvURL = "MINDSPEC_FEEDBACK_REMOTE_URL"

// FeedbackRemoteEnvCredential carries the machine-global feedback-remote push
// credential. Possession of this credential IS the identity (Req 6): the gate
// is enforced at the WRITE DESTINATION, not at the CLI (you cannot hide a
// subcommand per-user in a shared binary). It is read from the environment
// only — never from any committed file.
const FeedbackRemoteEnvCredential = "MINDSPEC_FEEDBACK_REMOTE_CREDENTIAL"

// FeedbackRemote holds the global/user-scoped feedback-remote settings parsed
// from the machine-global config.yaml. Only the non-secret destination lives
// here; the push credential is supplied out-of-band via the environment so the
// gating secret is never committed to any config file.
type FeedbackRemote struct {
	URL string `yaml:"url"`
}

// GlobalConfig is the machine-global, user-scoped mindspec config read from
// os.UserConfigDir()/mindspec/config.yaml. It is intentionally separate from
// the project-scoped Config: the feedback-remote contract is global-only.
type GlobalConfig struct {
	FeedbackRemote FeedbackRemote `yaml:"feedback_remote"`
}

// FeedbackRemoteTarget is the resolved push destination. CanPush is the
// FAIL-CLOSED gate: when false, the owner/remote path is unavailable and the
// caller MUST write local-only with no push attempt. URL is non-empty ONLY
// when CanPush is true, so a caller can never accidentally push to a partially
// resolved (wrong) remote.
type FeedbackRemoteTarget struct {
	// URL is the resolved push destination, populated ONLY when CanPush is
	// true. It is empty whenever the contract fails closed.
	URL string
	// CanPush reports whether a push is permitted: true ONLY when BOTH a
	// destination URL (env > global config) AND the machine-global push
	// credential are present. Absent either, this is false (fail-closed).
	CanPush bool
}

// GlobalConfigDir resolves the machine-global mindspec config/state directory:
// os.UserConfigDir()/mindspec. On Linux os.UserConfigDir honors
// $XDG_CONFIG_HOME (falling back to ~/.config); if os.UserConfigDir is
// unavailable, this falls back to os.UserHomeDir()/.config/mindspec and then
// os.UserHomeDir()/.mindspec. This is the SAME root as the spec §Storage
// Contract state dir; the feedback-remote config.yaml lives directly under it.
//
// It is NEVER under any project/git tree and NEVER swept by bd/dolt (HC-3): the
// resolved dir is guarded against landing inside a git work tree (which could
// otherwise be steered via XDG_CONFIG_HOME/HOME pointing at a repo), so the
// machine-global store is never under a committable tree.
func GlobalConfigDir() (string, error) {
	var candidate string
	switch {
	case userConfigDir() != "":
		candidate = filepath.Join(userConfigDir(), "mindspec")
	case userHomeDir() != "":
		home := userHomeDir()
		// Prefer the XDG-style ~/.config/mindspec, matching what
		// os.UserConfigDir would have produced on Linux; fall back to the
		// documented ~/.mindspec if even that base is somehow unusable.
		if base := filepath.Join(home, ".config"); base != "" {
			candidate = filepath.Join(base, "mindspec")
		} else {
			candidate = filepath.Join(home, ".mindspec")
		}
	default:
		return "", fmt.Errorf("cannot resolve global config dir: neither UserConfigDir nor UserHomeDir available")
	}

	// Hardening (HC-3): the machine-global store must never live inside a
	// project/git work tree, where its config.yaml could be tracked/committed.
	// XDG_CONFIG_HOME/HOME are operator-owned, but an operator who points them
	// at a repo would otherwise let project-tracked files masquerade as the
	// global config. Refuse such a resolution rather than silently honoring it.
	//
	// Run the guard against the CANONICAL (symlink-resolved) candidate so a
	// HOME/XDG_CONFIG_HOME that is itself a symlink into a repo cannot slip a
	// committable target past the literal-path walk — the same symlink-into-
	// repo bypass closed for the MINDSPEC_STATE_DIR override. The RETURNED
	// value stays the un-resolved candidate (so existing callers/tests that
	// reason about the path relative to HOME are unaffected); only the guard
	// looks through the symlink. If canonicalization fails closed, refuse.
	guardTarget := candidate
	if resolved, err := CanonicalPath(candidate); err == nil {
		guardTarget = resolved
	} else {
		return "", fmt.Errorf("refusing global config dir %q: cannot canonicalize it for the HC-3 git-tree guard (fail closed): %w", candidate, err)
	}
	if root := enclosingGitTree(guardTarget); root != "" {
		return "", fmt.Errorf("refusing global config dir %q: it resolves inside the git work tree at %q (HC-3: the global store must never be under a committable tree); unset or repoint XDG_CONFIG_HOME/HOME", candidate, root)
	}
	return candidate, nil
}

// userConfigDir returns os.UserConfigDir() or "" when unavailable. Split out so
// the fallback chain reads as a simple switch.
func userConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return ""
}

// userHomeDir returns os.UserHomeDir() or "" when unavailable.
func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// EnclosingGitTree is the exported HC-3 isolation guard: it reports the
// nearest enclosing git work tree of dir (the directory containing a ".git"
// entry), or "" if dir resolves outside any git tree. It is reused by
// internal/journal to apply the SAME guard GlobalConfigDir() uses to the
// MINDSPEC_STATE_DIR override, so the friction store can never land inside a
// project/.beads/committable tree even via the explicit override seam.
//
// NOTE: this walks dir as a LITERAL path. Callers that accept an
// operator-supplied path which may be (or contain) a symlink MUST first run
// it through CanonicalPath, otherwise a symlink whose PATH is outside any git
// tree but whose TARGET is inside the repo slips past this string-only walk
// (the HC-3 symlink-into-repo bypass). See journal.Dir.
func EnclosingGitTree(dir string) string {
	return enclosingGitTree(dir)
}

// CanonicalPath resolves dir to an absolute, symlink-free path so the HC-3
// git-tree guard (EnclosingGitTree) checks the REAL on-disk location rather
// than a symlink whose path is innocent but whose target is inside the repo.
//
// It applies filepath.Abs then filepath.EvalSymlinks. Because the
// MINDSPEC_STATE_DIR override (and the lazily-created global config dir) need
// not exist yet, a leaf that does not exist is handled by resolving the
// NEAREST EXISTING ANCESTOR (the same lazily-created-dir tolerance
// enclosingGitTree already grants) and re-appending the non-existent tail —
// so a symlinked PARENT is still caught even when the leaf is absent.
//
// It FAILS CLOSED: if Abs fails, or symlink resolution of every ancestor up to
// the filesystem root fails (a genuinely broken/ambiguous path), it returns an
// error so the caller writes nothing rather than guessing.
func CanonicalPath(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("canonicalize path: empty path")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("canonicalize path %q: %w", dir, err)
	}
	// Fast path: the whole path exists → resolve it directly.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	// The leaf (or some suffix) does not exist yet. Walk up to the nearest
	// EXISTING ancestor, resolve THAT (so a symlinked parent is still caught),
	// then re-append the non-existent tail. This mirrors how enclosingGitTree
	// tolerates a lazily-created dir.
	cur := filepath.Clean(abs)
	var tail []string
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			parts := append([]string{resolved}, tail...)
			return filepath.Join(parts...), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the filesystem root without any existing ancestor that
			// resolved — fail closed rather than guess.
			return "", fmt.Errorf("canonicalize path %q: no resolvable ancestor", dir)
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}

// enclosingGitTree walks up from dir (and its existing ancestors) looking for a
// ".git" entry (dir or gitfile). It returns the directory that contains the
// ".git" entry, or "" if none is found. dir itself need not exist yet — the
// machine-global config dir is often created lazily — so the walk starts at the
// nearest existing ancestor.
func enclosingGitTree(dir string) string {
	if dir == "" {
		return ""
	}
	cur := filepath.Clean(dir)
	for {
		if _, err := os.Lstat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "" // reached the filesystem root
		}
		cur = parent
	}
}

// LoadGlobal reads the global/user-scoped config.yaml under dir (the
// GlobalConfigDir()). It returns an empty *GlobalConfig when the file is
// absent — the default, no-remote state. Pass GlobalConfigDir()'s result in
// production; tests pass a temp dir.
//
// This loader reads ONLY the global scope. The feedback-remote setting is
// NEVER read from a project-committed `.mindspec/config.yaml` (the project
// Config struct does not even carry the field), so a committed remote is
// ignored (HC-3).
func LoadGlobal(dir string) (*GlobalConfig, error) {
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("reading global config: %w", err)
	}

	gc := &GlobalConfig{}
	if err := yaml.Unmarshal(data, gc); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return gc, nil
}

// ResolveFeedbackRemote computes the fail-closed feedback-remote push target.
//
// Resolution (all inputs are TrimSpace'd; blank-after-trim => ABSENT):
//   - URL: env MINDSPEC_FEEDBACK_REMOTE_URL, else the global config's
//     feedback_remote.url. The project-committed config is NEVER consulted.
//   - Credential: env MINDSPEC_FEEDBACK_REMOTE_CREDENTIAL (the identity gate).
//
// A whitespace-only credential or URL is therefore NOT "present": it can never
// open the gate (CanPush stays false) nor be returned as the target.
//
// FAIL-CLOSED: CanPush is true ONLY when BOTH a URL and the credential are
// present. Absent either, the returned target has CanPush=false and an EMPTY
// URL — the caller writes local-only and NEVER falls back to "push anyway" or
// a wrong remote (Req 6 / HC-3). The remote push itself is deferred to the
// follow-on; this only resolves WHETHER one is permitted and to WHERE.
func ResolveFeedbackRemote(globalDir string) (FeedbackRemoteTarget, error) {
	gc, err := LoadGlobal(globalDir)
	if err != nil {
		// Fail closed on a load error: never guess a remote.
		return FeedbackRemoteTarget{}, err
	}

	// Normalize all inputs: a value that is empty OR whitespace-only after
	// TrimSpace is treated as ABSENT. This closes a fail-closed bypass where a
	// whitespace-only credential ("   ") would satisfy the authorization gate
	// (CanPush=true without a real credential) and a whitespace-only URL would
	// be returned verbatim as a structurally invalid push target. Both env and
	// the global-config URL fallback are trimmed.
	url := strings.TrimSpace(os.Getenv(FeedbackRemoteEnvURL))
	if url == "" {
		url = strings.TrimSpace(gc.FeedbackRemote.URL)
	}
	credential := strings.TrimSpace(os.Getenv(FeedbackRemoteEnvCredential))

	// Fail-closed: a push requires BOTH a destination and the credential.
	// (url/credential are already blank-after-trim => absent here.)
	if url == "" || credential == "" {
		return FeedbackRemoteTarget{}, nil
	}
	return FeedbackRemoteTarget{URL: url, CanPush: true}, nil
}
