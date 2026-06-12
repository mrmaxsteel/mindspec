package config

import (
	"fmt"
	"os"
	"path/filepath"

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
// ~/.mindspec. This is the SAME root as the spec §Storage Contract state dir;
// the feedback-remote config.yaml lives directly under it.
//
// It is NEVER under any project/git tree and NEVER swept by bd/dolt (HC-3).
func GlobalConfigDir() (string, error) {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "mindspec"), nil
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		// Prefer the XDG-style ~/.config/mindspec, matching what
		// os.UserConfigDir would have produced on Linux.
		return filepath.Join(home, ".config", "mindspec"), nil
	}
	return "", fmt.Errorf("cannot resolve global config dir: neither UserConfigDir nor UserHomeDir available")
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
// Resolution:
//   - URL: env MINDSPEC_FEEDBACK_REMOTE_URL, else the global config's
//     feedback_remote.url. The project-committed config is NEVER consulted.
//   - Credential: env MINDSPEC_FEEDBACK_REMOTE_CREDENTIAL (the identity gate).
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

	url := os.Getenv(FeedbackRemoteEnvURL)
	if url == "" {
		url = gc.FeedbackRemote.URL
	}
	credential := os.Getenv(FeedbackRemoteEnvCredential)

	// Fail-closed: a push requires BOTH a destination and the credential.
	if url == "" || credential == "" {
		return FeedbackRemoteTarget{}, nil
	}
	return FeedbackRemoteTarget{URL: url, CanPush: true}, nil
}
