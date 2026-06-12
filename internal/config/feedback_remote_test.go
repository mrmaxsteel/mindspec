package config

import (
	"os"
	"path/filepath"
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
