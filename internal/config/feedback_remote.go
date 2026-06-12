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
	if root := enclosingGitTree(candidate); root != "" {
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
