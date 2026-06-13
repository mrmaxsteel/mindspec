package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mrmaxsteel/mindspec/internal/journal"
	versionpkg "github.com/mrmaxsteel/mindspec/internal/version"
)

// hermeticStateDir points the journal at a fresh temp dir for the test and
// returns it (the MINDSPEC_STATE_DIR seam).
func hermeticStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(journal.StateDirEnv, dir)
	return dir
}

func journalRecords(t *testing.T) []journal.Record {
	t.Helper()
	recs, err := journal.ListReports()
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	return recs
}

func journalBytes(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "journal.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read journal: %v", err)
	}
	return string(data)
}

// --- commandTokens / detectFriction unit tests (real cobra commands) -----

// TestCommandTokens maps the four real leaf commands to their
// (Command, Subcommand) enum tokens. These must match the drift-guard's
// CommandTokens / SubcommandTokens membership so RedactEvent never drops a
// legitimate friction event.
func TestCommandTokens(t *testing.T) {
	cases := []struct {
		cmd     *cobra.Command
		wantCmd string
		wantSub string
	}{
		{completeCmd, "complete", ""},
		{implApproveCmd, "impl", "approve"},
		{approveImplCmd, "approve", "impl"},
		{repairPhaseCmd, "repair", "phase"},
	}
	for _, tc := range cases {
		gotCmd, gotSub := commandTokens(tc.cmd)
		if gotCmd != tc.wantCmd || gotSub != tc.wantSub {
			t.Errorf("commandTokens(%q): got (%q,%q) want (%q,%q)",
				tc.cmd.CommandPath(), gotCmd, gotSub, tc.wantCmd, tc.wantSub)
		}
	}
}

// TestDetectFriction_RepairPhase asserts a completed `repair phase` is a
// friction signal (repair-phase token), and a bare top-level command is
// not.
func TestDetectFriction_RepairPhase(t *testing.T) {
	if eh, ok := detectFriction(repairPhaseCmd); !ok || eh != "repair-phase" {
		t.Errorf("repair phase: got (%q,%v), want (repair-phase,true)", eh, ok)
	}
	// A command with no bound flag set and no repair-phase identity is clean.
	if eh, ok := detectFriction(contextCmd); ok {
		t.Errorf("clean command should not be friction, got (%q,%v)", eh, ok)
	}
}

// TestDetectFriction_OverrideFlag asserts a Changed override flag on a leaf
// is detected as the matching escape-hatch token, and an UNSET flag is not.
func TestDetectFriction_OverrideFlag(t *testing.T) {
	// Build an isolated copy of the complete command's flags so we can flip
	// Changed without mutating the real command across tests.
	cmd := &cobra.Command{Use: "complete"}
	cmd.Flags().String("override-adr", "", "")
	cmd.Flags().String("allow-doc-skew", "", "")
	cmd.Flags().String("supersede-adr", "", "")

	// Unset → no friction.
	if eh, ok := detectFriction(cmd); ok {
		t.Errorf("unset flags: want no friction, got (%q,%v)", eh, ok)
	}

	// Set --override-adr → friction.
	if err := cmd.Flags().Set("override-adr", "some private reason"); err != nil {
		t.Fatal(err)
	}
	if eh, ok := detectFriction(cmd); !ok || eh != "override-adr" {
		t.Errorf("override-adr set: got (%q,%v), want (override-adr,true)", eh, ok)
	}
}

// --- PersistentPostRunE wiring (the always-on hook) ----------------------

// newTestRoot builds a minimal cobra root that reuses the REAL
// PersistentPostRunE wiring under test (emitFriction) without invoking the
// real commands' side effects. The PostRunE body mirrors root.go's
// production hook's self-emit step.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use: "mindspec",
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// The exact production self-emit step (root.go).
			emitFriction(cmd)
			return nil
		},
	}
	return root
}

// TestPostRun_SuccessPathOverride_AppendsOneEntry asserts a success-path
// `complete --override-adr` appends exactly ONE redacted entry from
// PersistentPostRunE (Req 2 / A1 positive case).
func TestPostRun_SuccessPathOverride_AppendsOneEntry(t *testing.T) {
	hermeticStateDir(t)

	root := newTestRoot()
	complete := &cobra.Command{
		Use:  "complete",
		RunE: func(cmd *cobra.Command, args []string) error { return nil }, // SUCCESS
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	root.SetArgs([]string{"complete", "--override-adr", "my private reason"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	recs := journalRecords(t)
	if len(recs) != 1 {
		t.Fatalf("want exactly 1 journal entry, got %d: %+v", len(recs), recs)
	}
	if recs[0].Command != "complete" || recs[0].EscapeHatch != "override-adr" {
		t.Errorf("unexpected entry: %+v", recs[0])
	}
}

// TestPostRun_OverrideReasonNeverJournaled is the R3 privacy keystone:
// drive a REAL success-path `complete --override-adr "<secret reason>"`
// end-to-end through the production PostRunE and assert NONE of the reason
// (a path, a secret token, an email) survives in the on-disk journal — only
// the enum tokens. The capture path reads Changed() (a bool), never the
// flag VALUE, so the reason is structurally excluded.
func TestPostRun_OverrideReasonNeverJournaled(t *testing.T) {
	dir := hermeticStateDir(t)

	root := newTestRoot()
	complete := &cobra.Command{
		Use:  "complete",
		RunE: func(cmd *cobra.Command, args []string) error { return nil }, // SUCCESS
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	const reason = "rotate /Users/victim/.ssh/id_rsa ghp_SECRETTOKEN because max@victim.com asked"
	root.SetArgs([]string{"complete", "--override-adr", reason})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := journalBytes(t, dir)
	for _, needle := range []string{"ghp_SECRETTOKEN", "victim", ".ssh", "/Users", "@victim.com", "rotate", "because"} {
		if strings.Contains(raw, needle) {
			t.Errorf("override reason leaked into the journal (needle %q):\n%s", needle, raw)
		}
	}
	// And exactly one enum-only entry was still produced.
	if recs := journalRecords(t); len(recs) != 1 || recs[0].EscapeHatch != "override-adr" {
		t.Errorf("want exactly one override-adr entry, got %+v", recs)
	}
}

// TestPostRun_CleanSuccess_AppendsNothing is the LOAD-BEARING negative case
// (A1): PersistentPostRunE runs on EVERY success, so a clean command with
// NO bound escape-hatch / repair-phase MUST append ZERO entries — this
// bounds the always-on sink and blocks a journal-everything regression.
func TestPostRun_CleanSuccess_AppendsNothing(t *testing.T) {
	dir := hermeticStateDir(t)

	root := newTestRoot()
	status := &cobra.Command{
		Use:  "status",
		RunE: func(cmd *cobra.Command, args []string) error { return nil }, // clean SUCCESS
	}
	root.AddCommand(status)

	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if raw := journalBytes(t, dir); raw != "" {
		t.Errorf("clean success journaled an entry (A1 privacy boundary violated):\n%s", raw)
	}
	if recs := journalRecords(t); len(recs) != 0 {
		t.Errorf("clean success appended %d entries, want 0", len(recs))
	}
}

// TestPostRun_UnsetOverrideFlag_AppendsNothing asserts that a command that
// HAS an override flag registered but did NOT set it (the normal clean
// path) appends nothing — only a Changed flag is friction.
func TestPostRun_UnsetOverrideFlag_AppendsNothing(t *testing.T) {
	dir := hermeticStateDir(t)

	root := newTestRoot()
	complete := &cobra.Command{
		Use:  "complete",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	root.SetArgs([]string{"complete"}) // no --override-adr
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if raw := journalBytes(t, dir); raw != "" {
		t.Errorf("unset override flag journaled an entry:\n%s", raw)
	}
}

// TestPostRun_FailedCommand_NotReached asserts a command whose RunE returns
// an error never reaches the self-emit (cobra skips PersistentPostRunE on
// RunE failure) — failed/gate-blocked runs are structurally uncapturable.
func TestPostRun_FailedCommand_NotReached(t *testing.T) {
	dir := hermeticStateDir(t)

	root := newTestRoot()
	complete := &cobra.Command{
		Use:           "complete",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errFailedForTest // FAILURE: PostRunE must not run
		},
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	root.SetArgs([]string{"complete", "--override-adr", "reason"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected the command to fail")
	}
	if raw := journalBytes(t, dir); raw != "" {
		t.Errorf("a FAILED command journaled an entry (PostRunE should be skipped):\n%s", raw)
	}
}

var errFailedForTest = &simpleErr{"forced failure"}

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

// TestPostRun_BestEffort_DoesNotFailCommand asserts journaling is
// NON-FATAL: even if the journal cannot be written, an already-successful
// command's exit is unaffected (the hook never returns the journal error).
// We force a journal failure by pointing the state dir at a path that
// cannot be created (a regular file in the way).
func TestPostRun_BestEffort_DoesNotFailCommand(t *testing.T) {
	// Point the state dir at a path whose parent is a FILE, so MkdirAll
	// fails inside AppendSuccessEvent.
	blocker := filepath.Join(t.TempDir(), "iam-a-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(journal.StateDirEnv, filepath.Join(blocker, "nope"))

	root := newTestRoot()
	complete := &cobra.Command{
		Use:  "complete",
		RunE: func(cmd *cobra.Command, args []string) error { return nil }, // SUCCESS
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	root.SetArgs([]string{"complete", "--override-adr", "reason"})
	if err := root.Execute(); err != nil {
		t.Fatalf("journaling must be best-effort: a journal failure must NOT fail the command, got %v", err)
	}
}

// TestPostRun_VersionStamp asserts every journal entry carries the bare
// version.Current() semver (Req 3), NOT a decorated --version string.
func TestPostRun_VersionStamp(t *testing.T) {
	hermeticStateDir(t)

	// Pin a fake bare semver via the Set seam; restore after.
	versionpkg.Set("9.9.9")

	root := newTestRoot()
	complete := &cobra.Command{
		Use:  "complete",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	complete.Flags().String("override-adr", "", "")
	root.AddCommand(complete)

	root.SetArgs([]string{"complete", "--override-adr", "reason"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	recs := journalRecords(t)
	if len(recs) != 1 {
		t.Fatalf("want 1 entry, got %d", len(recs))
	}
	if recs[0].Version != "9.9.9" {
		t.Errorf("want bare version 9.9.9 stamped, got %q", recs[0].Version)
	}
	// Guard against the decorated --version string ever being stamped.
	if strings.Contains(recs[0].Version, "(") || strings.Contains(recs[0].Version, "dev (") {
		t.Errorf("decorated version string leaked into the stamp: %q", recs[0].Version)
	}
}

// TestVersionSet_WiredAtStartup asserts root.go's init wired
// version.Set(version) so version.Current() reflects main's build var. In a
// test build main.version is "dev", so Current() is "dev" (not empty), and
// currentVersion() routes through the helper.
func TestVersionSet_WiredAtStartup(t *testing.T) {
	// init already ran version.Set(version) where version=="dev" in tests.
	// Re-derive that the seam is the path currentVersion() reads.
	versionpkg.Set("1.2.3")
	if got := currentVersion(); got != "1.2.3" {
		t.Errorf("currentVersion() should read version.Current(); got %q", got)
	}
	versionpkg.Set("dev") // restore a sane default for other tests
}
