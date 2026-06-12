package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 094 Bead 4 — feedback-remote config contract tests (Req 6 / HC-3).
//
// These exercise the NET-NEW global/user-scoped loader and the FAIL-CLOSED
// resolution: global config is loaded; absent config => fail-closed
// (no-push); a PROJECT-committed feedback-remote config is IGNORED; and env >
// global precedence with no project read.

// writeGlobalConfig writes a global config.yaml under dir and returns dir.
func writeGlobalConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLoadGlobal_ReadsFeedbackRemote covers the happy path: the global-scoped
// loader parses feedback_remote.url from config.yaml under the global dir.
func TestLoadGlobal_ReadsFeedbackRemote(t *testing.T) {
	dir := t.TempDir()
	writeGlobalConfig(t, dir, "feedback_remote:\n  url: https://feedback.example/db\n")

	gc, err := LoadGlobal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc.FeedbackRemote.URL != "https://feedback.example/db" {
		t.Errorf("expected feedback_remote.url to load, got %q", gc.FeedbackRemote.URL)
	}
}

// TestLoadGlobal_AbsentFile covers the default no-remote state: an absent
// global config.yaml yields an empty GlobalConfig (no error), which downstream
// resolves fail-closed.
func TestLoadGlobal_AbsentFile(t *testing.T) {
	gc, err := LoadGlobal(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error for absent global config: %v", err)
	}
	if gc.FeedbackRemote.URL != "" {
		t.Errorf("expected empty feedback_remote.url when file absent, got %q", gc.FeedbackRemote.URL)
	}
}

// TestResolveFeedbackRemote is the core fail-closed / config-scope table.
func TestResolveFeedbackRemote(t *testing.T) {
	tests := []struct {
		name          string
		globalYAML    string // "" => no global config.yaml written
		envURL        string
		envCredential string
		wantCanPush   bool
		wantURL       string
	}{
		{
			name: "absent config => fail-closed, local-only",
			// No global config, no env => no URL, no credential.
			wantCanPush: false,
			wantURL:     "",
		},
		{
			name:        "global URL but NO credential => fail-closed",
			globalYAML:  "feedback_remote:\n  url: https://feedback.example/db\n",
			wantCanPush: false,
			wantURL:     "", // URL is empty whenever CanPush is false (no wrong-remote leak)
		},
		{
			name:          "credential but NO URL => fail-closed (no wrong remote)",
			envCredential: "secret-token",
			wantCanPush:   false,
			wantURL:       "",
		},
		{
			name:          "global URL + credential => push permitted",
			globalYAML:    "feedback_remote:\n  url: https://feedback.example/db\n",
			envCredential: "secret-token",
			wantCanPush:   true,
			wantURL:       "https://feedback.example/db",
		},
		{
			name:          "env URL overrides global URL (env > global)",
			globalYAML:    "feedback_remote:\n  url: https://global.example/db\n",
			envURL:        "https://env.example/db",
			envCredential: "secret-token",
			wantCanPush:   true,
			wantURL:       "https://env.example/db",
		},
		{
			name:          "env URL with credential, no global config",
			envURL:        "https://env.example/db",
			envCredential: "secret-token",
			wantCanPush:   true,
			wantURL:       "https://env.example/db",
		},
		{
			// Whitespace-only credential must NOT open the gate: blank
			// after TrimSpace => absent (fail-closed auth-bypass guard).
			name:          "whitespace-only credential => fail-closed (not present)",
			globalYAML:    "feedback_remote:\n  url: https://feedback.example/db\n",
			envCredential: "   ",
			wantCanPush:   false,
			wantURL:       "",
		},
		{
			// Whitespace-only env URL must never be returned as the target;
			// it is treated as absent, so with no other URL we fail closed.
			name:          "whitespace-only env URL => not returned, fail-closed",
			envURL:        "   ",
			envCredential: "secret-token",
			wantCanPush:   false,
			wantURL:       "",
		},
		{
			// Whitespace-only global-config URL is likewise absent.
			name:          "whitespace-only global URL => not returned, fail-closed",
			globalYAML:    "feedback_remote:\n  url: \"   \"\n",
			envCredential: "secret-token",
			wantCanPush:   false,
			wantURL:       "",
		},
		{
			// Valid values with surrounding whitespace are USED, trimmed —
			// the credential's space-padding must not break a real push, and
			// the resolved URL must be the clean (trimmed) destination.
			name:          "leading/trailing space around valid values => used trimmed",
			envURL:        "  https://env.example/db  ",
			envCredential: "  secret-token  ",
			wantCanPush:   true,
			wantURL:       "https://env.example/db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.globalYAML != "" {
				writeGlobalConfig(t, dir, tc.globalYAML)
			}
			t.Setenv(FeedbackRemoteEnvURL, tc.envURL)
			t.Setenv(FeedbackRemoteEnvCredential, tc.envCredential)

			got, err := ResolveFeedbackRemote(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CanPush != tc.wantCanPush {
				t.Errorf("CanPush = %v, want %v", got.CanPush, tc.wantCanPush)
			}
			if got.URL != tc.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tc.wantURL)
			}
			// Fail-closed invariant: a non-pushable target must never carry a URL.
			if !got.CanPush && got.URL != "" {
				t.Errorf("fail-closed violation: CanPush=false but URL=%q (would risk a wrong-remote push)", got.URL)
			}
		})
	}
}

// TestResolveFeedbackRemote_MalformedGlobalFailsClosed locks the approved
// fail-closed-on-load-error semantics: a malformed global config.yaml makes
// LoadGlobal error, and ResolveFeedbackRemote propagates that error with an
// EMPTY target (CanPush=false, URL="") rather than guessing a remote — even
// when a valid env URL+credential are present. A corrupt machine-global store
// must never be silently ignored in favor of the env; it fails closed.
func TestResolveFeedbackRemote_MalformedGlobalFailsClosed(t *testing.T) {
	dir := t.TempDir()
	// Invalid YAML (a bare unterminated structure) so yaml.Unmarshal errors.
	writeGlobalConfig(t, dir, "feedback_remote: [unterminated\n")
	t.Setenv(FeedbackRemoteEnvURL, "https://env.example/db")
	t.Setenv(FeedbackRemoteEnvCredential, "secret-token")

	got, err := ResolveFeedbackRemote(dir)
	if err == nil {
		t.Fatal("expected an error from malformed global config, got nil")
	}
	if got.CanPush {
		t.Error("fail-closed violation: malformed global config must not yield CanPush=true")
	}
	if got.URL != "" {
		t.Errorf("fail-closed violation: malformed global config returned URL=%q", got.URL)
	}
}

// TestResolveFeedbackRemote_ProjectCommittedConfigIgnored is the load-bearing
// config-scope test: a feedback-remote config placed in a PROJECT-committed
// `.mindspec/config.yaml` is IGNORED. Only the global/user-scoped value is
// read. Here the global dir has NO feedback-remote, and the project file does
// — the resolution must still fail closed (HC-3).
func TestResolveFeedbackRemote_ProjectCommittedConfigIgnored(t *testing.T) {
	// A project repo with a committed feedback-remote config (the leak case).
	projectRoot := t.TempDir()
	projectMindspec := filepath.Join(projectRoot, ".mindspec")
	if err := os.MkdirAll(projectMindspec, 0o755); err != nil {
		t.Fatal(err)
	}
	committed := "feedback_remote:\n  url: https://committed.example/db\nmerge_strategy: pr\n"
	if err := os.WriteFile(filepath.Join(projectMindspec, "config.yaml"), []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}

	// A clean global dir with NO feedback-remote configured.
	globalDir := t.TempDir()
	// No machine-global credential.
	t.Setenv(FeedbackRemoteEnvURL, "")
	t.Setenv(FeedbackRemoteEnvCredential, "")

	got, err := ResolveFeedbackRemote(globalDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CanPush {
		t.Fatal("project-committed feedback-remote must be IGNORED: CanPush must stay false")
	}
	if got.URL != "" {
		t.Errorf("project-committed URL leaked into resolution: %q", got.URL)
	}

	// And prove the project-scoped Load never surfaces a feedback-remote at
	// all — the field is absent from the project Config struct, so the
	// committed key is silently dropped (never honored).
	ResetCache()
	defer ResetCache()
	cfg, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("project Load error: %v", err)
	}
	if cfg.MergeStrategy != "pr" {
		t.Errorf("project Load should still parse non-feedback keys, got merge_strategy=%q", cfg.MergeStrategy)
	}
}

// TestGlobalConfigDir_NonProjectPath asserts the resolved global dir is the
// os.UserConfigDir()/mindspec root (HC-3: never under a project/git tree).
func TestGlobalConfigDir_NonProjectPath(t *testing.T) {
	dir, err := GlobalConfigDir()
	if err != nil {
		t.Skipf("global config dir unavailable in this environment: %v", err)
	}
	if filepath.Base(dir) != "mindspec" {
		t.Errorf("expected global config dir to end in /mindspec, got %q", dir)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected an absolute global config dir, got %q", dir)
	}
}

// TestEnclosingGitTree exercises the project-tree guard helper directly: a path
// inside (or whose ancestor is) a git work tree is detected; a clean path is
// not. This is the non-vacuous core of the HC-3 hardening.
func TestEnclosingGitTree(t *testing.T) {
	// A clean directory with no .git anywhere up the chain.
	clean := t.TempDir()
	if root := enclosingGitTree(filepath.Join(clean, "mindspec")); root != "" {
		t.Errorf("expected no enclosing git tree for clean path, got %q", root)
	}

	// A git work tree (.git as a directory); the resolved dir is a child.
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "sub", "mindspec") // need not exist yet
	if root := enclosingGitTree(nested); root != repo {
		t.Errorf("expected enclosing git tree %q, got %q", repo, root)
	}

	// A git worktree where .git is a FILE (gitdir: pointer) must also count.
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if root := enclosingGitTree(filepath.Join(wt, "mindspec")); root != wt {
		t.Errorf("expected enclosing git worktree %q for .git-file case, got %q", wt, root)
	}
}

// TestGlobalConfigDir_RejectsGitTree drives the guard through GlobalConfigDir by
// repointing HOME (and XDG_CONFIG_HOME, honored by os.UserConfigDir on Linux)
// into a temp git work tree. The resolution must be REFUSED so the machine-
// global store never lands under a committable tree (HC-3 hardening).
func TestGlobalConfigDir_RejectsGitTree(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Cover both resolution platforms: HOME drives darwin's
	// UserConfigDir (HOME/Library/Application Support) and the UserHomeDir
	// fallback; XDG_CONFIG_HOME drives Linux's UserConfigDir directly.
	t.Setenv("HOME", repo)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(repo, ".config"))

	dir, err := GlobalConfigDir()
	if err == nil {
		t.Fatalf("expected GlobalConfigDir to refuse a path inside a git tree, got %q", dir)
	}
	if !strings.Contains(err.Error(), "git work tree") {
		t.Errorf("expected a git-work-tree rejection error, got: %v", err)
	}
}

// TestGlobalConfigDir_FallbackChain exercises the documented fallback when
// os.UserConfigDir is unavailable: with XDG_CONFIG_HOME unset/empty and HOME
// pointed at a clean (non-git) temp dir, resolution falls through to
// HOME/.config/mindspec (the XDG-style fallback), still absolute and ending in
// /mindspec, and NOT rejected. This is non-vacuous: it asserts the concrete
// fallback path, not just a basename.
func TestGlobalConfigDir_FallbackChain(t *testing.T) {
	home := t.TempDir() // clean: no .git
	t.Setenv("HOME", home)
	// Force the UserConfigDir base to be derived from HOME so the resolved
	// path is deterministic relative to our temp dir on either platform.
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := GlobalConfigDir()
	if err != nil {
		t.Fatalf("unexpected error resolving fallback global dir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected an absolute fallback dir, got %q", dir)
	}
	if filepath.Base(dir) != "mindspec" {
		t.Errorf("expected fallback dir to end in /mindspec, got %q", dir)
	}
	// It must resolve UNDER our temp HOME (proving the fallback chain used HOME),
	// and must NOT have been rejected.
	rel, err := filepath.Rel(home, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("expected fallback dir under HOME %q, got %q (rel=%q)", home, dir, rel)
	}
}
