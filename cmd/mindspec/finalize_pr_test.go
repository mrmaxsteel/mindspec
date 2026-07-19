package main

// Spec 121 Bead 3: unit fixtures at the cmd-side gh seam, pinning
// AC-1..AC-7, AC-20, AC-21. The harness gh-shim end-to-end scenario
// (internal/harness/scenario_finalize_pr.go) additionally exercises the
// same argv/adoption/degrade/reconcile shapes through the real built
// binary.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// --- test seam plumbing -----------------------------------------------

// withFinalizePRSeams overrides ghRunFn/ghAvailableFn/fetchRemoteBranchFn
// for the duration of the test, restoring the originals on cleanup.
// fetchRemoteBranchFn defaults to a no-op success — only the AC-4 real-
// merge fixture wants the genuine gitutil.FetchRemoteBranch behavior, and
// it restores that explicitly.
func withFinalizePRSeams(t *testing.T, gh func(ctx context.Context, args ...string) ([]byte, error), available bool) {
	t.Helper()
	origGH, origAvail, origFetch := ghRunFn, ghAvailableFn, fetchRemoteBranchFn
	origLeg, origChecks, origPoll := finalizePRLegTimeout, finalizePRChecksTimeout, finalizePRPollInterval
	t.Cleanup(func() {
		ghRunFn, ghAvailableFn, fetchRemoteBranchFn = origGH, origAvail, origFetch
		finalizePRLegTimeout, finalizePRChecksTimeout, finalizePRPollInterval = origLeg, origChecks, origPoll
	})
	if gh != nil {
		ghRunFn = gh
	}
	ghAvailableFn = func() bool { return available }
	fetchRemoteBranchFn = func(remote, branch string) error { return nil }
}

// ghScript dispatches a scripted gh seam by subcommand shape, recording
// every call's full argv for post-hoc assertions.
type ghScript struct {
	t         *testing.T
	calls     []string
	create    func(args []string) ([]byte, error)
	lookup    func(args []string) ([]byte, error) // pr list, no --base
	reconcile func(args []string) ([]byte, error) // pr list --base ...
	checks    func(args []string) ([]byte, error)
	merge     func(args []string) ([]byte, error)
}

func (s *ghScript) fn(ctx context.Context, args ...string) ([]byte, error) {
	s.calls = append(s.calls, strings.Join(args, " "))
	if len(args) < 2 {
		s.t.Fatalf("unexpected short gh invocation: %v", args)
		return nil, nil
	}
	switch {
	case args[0] == "pr" && args[1] == "create":
		if s.create != nil {
			return s.create(args)
		}
	case args[0] == "pr" && args[1] == "list":
		hasBase := false
		for _, a := range args {
			if a == "--base" {
				hasBase = true
			}
		}
		if hasBase {
			if s.reconcile != nil {
				return s.reconcile(args)
			}
		} else if s.lookup != nil {
			return s.lookup(args)
		}
	case args[0] == "pr" && args[1] == "checks":
		if s.checks != nil {
			return s.checks(args)
		}
	case args[0] == "pr" && args[1] == "merge":
		if s.merge != nil {
			return s.merge(args)
		}
	}
	s.t.Fatalf("unscripted gh invocation: %v", args)
	return nil, nil
}

func jsonEntries(t *testing.T, entries []finalizePREntry) []byte {
	t.Helper()
	if entries == nil {
		return []byte("[]")
	}
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries: %v", err)
	}
	return b
}

func jsonChecks(t *testing.T, checks []ghCheckEntry) []byte {
	t.Helper()
	b, err := json.Marshal(checks)
	if err != nil {
		t.Fatalf("marshal checks: %v", err)
	}
	return b
}

const (
	fprSpecID = "121-finalizepr"
	fprEpicID = "mindspec-fpr1"
)

func fprHead() string { return "chore/finalize-" + fprSpecID }

func runAutomation(cfg *config.Config) (stdout, stderr string) {
	var so, se bytes.Buffer
	runFinalizePRAutomation(&so, &se, cfg, fprSpecID, fprEpicID, fprHead())
	return so.String(), se.String()
}

// --- AC-1: templated auto-open --------------------------------------

func TestFinalizePR_AC1_AutoOpenTemplatedTitle(t *testing.T) {
	script := &ghScript{t: t}
	script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
	script.create = func(args []string) ([]byte, error) {
		// Assert argv shape: head, base, title (with epicID), body present.
		joined := strings.Join(args, "\x00")
		if !strings.Contains(joined, "--head\x00"+fprHead()) {
			t.Errorf("pr create missing --head %s: %v", fprHead(), args)
		}
		if !strings.Contains(joined, "--base\x00main") {
			t.Errorf("pr create missing --base main: %v", args)
		}
		wantTitle := "chore(beads): finalize epic " + fprEpicID + " for spec " + fprSpecID
		if !strings.Contains(joined, "--title\x00"+wantTitle) {
			t.Errorf("pr create title = %v, want containing %q", args, wantTitle)
		}
		return []byte("https://github.com/acme/mindspec/pull/42\n"), nil
	}
	withFinalizePRSeams(t, script.fn, true)

	stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true})
	if !strings.Contains(stdout, "https://github.com/acme/mindspec/pull/42") {
		t.Errorf("stdout missing PR URL: %s", stdout)
	}
	if len(script.calls) != 2 {
		t.Errorf("expected exactly 2 gh calls (lookup, create), got %d: %v", len(script.calls), script.calls)
	}
}

// --- AC-2: idempotent adoption + base pin ----------------------------

func TestFinalizePR_AC2_AdoptionAndBasePin(t *testing.T) {
	t.Run("adopts already-open same-head-main PR, no duplicate create", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) {
			return jsonEntries(t, []finalizePREntry{{
				Number: 7, State: "OPEN", URL: "https://github.com/acme/mindspec/pull/7",
				HeadRefName: fprHead(), BaseRefName: "main",
			}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)

		stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true})
		if !strings.Contains(stdout, "pull/7") {
			t.Errorf("stdout missing adopted PR URL: %s", stdout)
		}
		for _, c := range script.calls {
			if strings.HasPrefix(c, "pr create") {
				t.Errorf("adoption case must never call pr create: %v", script.calls)
			}
		}
	})

	t.Run("other-base same-head PR is never adopted or merged", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) {
			return jsonEntries(t, []finalizePREntry{{
				Number: 8, State: "OPEN", URL: "https://github.com/acme/mindspec/pull/8",
				HeadRefName: fprHead(), BaseRefName: "develop",
			}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)

		// Even with auto-merge true, no checks/merge call should ever fire.
		_, _ = runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		for _, c := range script.calls {
			if strings.HasPrefix(c, "pr create") || strings.HasPrefix(c, "pr merge") || strings.HasPrefix(c, "pr checks") {
				t.Errorf("other-base PR must never be created-around, checked, or merged: %v", script.calls)
			}
		}
		if len(script.calls) != 1 {
			t.Errorf("expected exactly 1 gh call (lookup only), got %v", script.calls)
		}
	})

	// O1-1 (panel round 1): --state all can return a historical
	// CLOSED/MERGED same-head/main row ALONGSIDE the live OPEN
	// candidate; classifyFinalizePREntries must prefer OPEN regardless
	// of result order, never just "the last match".
	t.Run("prefers OPEN over a same-head/main CLOSED row regardless of order", func(t *testing.T) {
		head := fprHead()
		closed := finalizePREntry{Number: 8, State: "CLOSED", URL: "https://x/pull/8", HeadRefName: head, BaseRefName: "main"}
		open := finalizePREntry{Number: 9, State: "OPEN", URL: "https://x/pull/9", HeadRefName: head, BaseRefName: "main"}

		same, other := classifyFinalizePREntries([]finalizePREntry{open, closed}, head, "main")
		if same == nil || same.Number != 9 {
			t.Errorf("OPEN-then-CLOSED order: expected the OPEN #9 entry, got %+v", same)
		}
		if other != nil {
			t.Errorf("expected no other-base entry, got %+v", other)
		}

		same, other = classifyFinalizePREntries([]finalizePREntry{closed, open}, head, "main")
		if same == nil || same.Number != 9 {
			t.Errorf("CLOSED-then-OPEN order: expected the OPEN #9 entry (not just the last match), got %+v", same)
		}
		if other != nil {
			t.Errorf("expected no other-base entry, got %+v", other)
		}
	})
}

// --- AC-3: gh-absent byte-identical degrade --------------------------

func TestFinalizePR_AC3_GHAbsentDegrades(t *testing.T) {
	withFinalizePRSeams(t, func(ctx context.Context, args ...string) ([]byte, error) {
		t.Fatalf("gh must never be invoked when unavailable: %v", args)
		return nil, nil
	}, false)

	stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true})
	if stdout != "" {
		t.Errorf("gh-absent path must add nothing to stdout (byte-identical to a no-gh run), got: %q", stdout)
	}
	if !strings.Contains(stderr, "gh CLI not found") {
		t.Errorf("expected a warning naming the skipped automation, got: %q", stderr)
	}
}

// --- AC-5: default-false boundary (auto-merge opt-in) -----------------

func TestFinalizePR_AC5_DefaultFalseNoMerge(t *testing.T) {
	script := &ghScript{t: t}
	script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
	script.create = func(args []string) ([]byte, error) {
		return []byte("https://github.com/acme/mindspec/pull/9\n"), nil
	}
	withFinalizePRSeams(t, script.fn, true)

	// AutoMergeFinalizePR left at zero value (false).
	stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true})
	if !strings.Contains(stdout, "pull/9") {
		t.Errorf("stdout missing opened PR URL: %s", stdout)
	}
	for _, c := range script.calls {
		if strings.HasPrefix(c, "pr checks") || strings.HasPrefix(c, "pr merge") {
			t.Errorf("auto_merge_finalize_pr defaults false: no checks/merge call may fire, got %v", script.calls)
		}
	}
}

// --- AC-6: per-leg fault matrix ---------------------------------------

func TestFinalizePR_AC6_FaultMatrix(t *testing.T) {
	t.Run("pr create fails, reconcile finds absent -> stranded NOTE", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return nil, errFake("create exploded") }
		script.reconcile = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true})
		if !strings.Contains(stderr, `"pr create"`) {
			t.Errorf("warning must name the failed leg: %s", stderr)
		}
		if !strings.Contains(stdout, "does not appear to have been created or merged") {
			t.Errorf("expected the stranded-carrier NOTE, got: %s", stdout)
		}
	})

	t.Run("existing-PR lookup fails, reconcile query also fails -> UNDETERMINED", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return nil, errFake("lookup exploded") }
		script.reconcile = func(args []string) ([]byte, error) { return nil, errFake("reconcile exploded too") }
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true})
		if !strings.Contains(stderr, `"existing-PR lookup"`) {
			t.Errorf("warning must name the lookup leg: %s", stderr)
		}
		if !strings.Contains(stderr, "UNDETERMINED") {
			t.Errorf("a failed reconcile query must be named UNDETERMINED: %s", stderr)
		}
		if !strings.Contains(stdout, "could not determine") {
			t.Errorf("expected the undetermined NOTE, got: %s", stdout)
		}
	})

	t.Run("checks non-green -> left open, no merge", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/1\n"), nil }
		script.checks = func(args []string) ([]byte, error) {
			return jsonChecks(t, []ghCheckEntry{{Name: "ci/build", State: "FAILURE"}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)

		stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		if !strings.Contains(stdout, "not all green") {
			t.Errorf("expected the not-green message, got: %s", stdout)
		}
		for _, c := range script.calls {
			if strings.HasPrefix(c, "pr merge") {
				t.Error("non-green checks must never merge")
			}
		}
	})

	t.Run("zero checks reported -> not green, never an error", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/2\n"), nil }
		script.checks = func(args []string) ([]byte, error) { return []byte("[]"), nil }
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		if !strings.Contains(stdout, "none configured") {
			t.Errorf("expected the zero-checks message, got: %s", stdout)
		}
		if strings.Contains(stderr, "leg") {
			t.Errorf("zero checks must never be reported as a failed leg: %s", stderr)
		}
	})

	t.Run("checks watch times out -> degrade + reconcile", func(t *testing.T) {
		finalizePRChecksTimeout = 20 * time.Millisecond
		finalizePRPollInterval = 5 * time.Millisecond
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/3\n"), nil }
		script.checks = func(args []string) ([]byte, error) {
			return jsonChecks(t, []ghCheckEntry{{Name: "ci/build", State: "PENDING"}}), nil
		}
		script.reconcile = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		if !strings.Contains(stderr, `"pr checks"`) {
			t.Errorf("timeout must be reported as the pr checks leg: %s", stderr)
		}
		if !strings.Contains(stdout, "does not appear to have been created or merged") {
			t.Errorf("expected the stranded NOTE after the checks timeout, got: %s", stdout)
		}
	})

	t.Run("pr merge fails, reconcile finds merged -> success", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/4\n"), nil }
		script.checks = func(args []string) ([]byte, error) {
			return jsonChecks(t, []ghCheckEntry{{Name: "ci", State: "SUCCESS"}}), nil
		}
		script.merge = func(args []string) ([]byte, error) { return nil, errFake("merge exploded") }
		script.reconcile = func(args []string) ([]byte, error) {
			return jsonEntries(t, []finalizePREntry{{State: "MERGED", URL: "https://x/pull/4", HeadRefName: fprHead(), BaseRefName: "main"}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		if !strings.Contains(stderr, `"pr merge"`) {
			t.Errorf("warning must name the merge leg: %s", stderr)
		}
		if !strings.Contains(stdout, "reconciled as merged") {
			t.Errorf("expected the reconciled-merged success line, got: %s", stdout)
		}
	})

	t.Run("per-leg timeout on create respects the bounded context", func(t *testing.T) {
		finalizePRLegTimeout = 10 * time.Millisecond
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) {
			return nil, errFake("simulated per-leg timeout")
		}
		script.reconcile = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		withFinalizePRSeams(t, script.fn, true)

		stdout, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: true})
		if !strings.Contains(stderr, `"pr create"`) {
			t.Errorf("expected the create leg to be named: %s", stderr)
		}
		if !strings.Contains(stdout, "does not appear to have been created or merged") {
			t.Errorf("expected the stranded NOTE: %s", stdout)
		}
	})
}

type errFake string

func (e errFake) Error() string { return string(e) }

// --- AC-7: config round-trip + inert-combination warning --------------

func TestFinalizePR_AC7_ConfigRoundTripAndInertWarning(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		def := config.DefaultConfig()
		if !def.AutoOpenFinalizePR {
			t.Error("auto_open_finalize_pr must default true")
		}
		if def.AutoMergeFinalizePR {
			t.Error("auto_merge_finalize_pr must default false")
		}
	})

	t.Run("round-trip from yaml", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(root+"/.mindspec", 0o755); err != nil {
			t.Fatal(err)
		}
		yamlBody := "auto_open_finalize_pr: false\nauto_merge_finalize_pr: true\n"
		if err := os.WriteFile(root+"/.mindspec/config.yaml", []byte(yamlBody), 0o644); err != nil {
			t.Fatal(err)
		}
		config.ResetCache()
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.AutoOpenFinalizePR {
			t.Error("explicit false must override the default true")
		}
		if !cfg.AutoMergeFinalizePR {
			t.Error("explicit true must be honored")
		}
	})

	t.Run("inert combination warns, invokes gh not at all", func(t *testing.T) {
		withFinalizePRSeams(t, func(ctx context.Context, args ...string) ([]byte, error) {
			t.Fatalf("inert combination must invoke zero gh commands: %v", args)
			return nil, nil
		}, true)

		_, stderr := runAutomation(&config.Config{AutoOpenFinalizePR: false, AutoMergeFinalizePR: true})
		if !strings.Contains(stderr, "auto_merge_finalize_pr is inert") {
			t.Errorf("expected the inert-combination warning naming auto_merge_finalize_pr, got: %s", stderr)
		}
	})

	t.Run("config show renders both keys", func(t *testing.T) {
		out, err := renderConfig(config.DefaultConfig())
		if err != nil {
			t.Fatalf("renderConfig: %v", err)
		}
		if !strings.Contains(out, "auto_open_finalize_pr: true") {
			t.Errorf("config show missing auto_open_finalize_pr: %s", out)
		}
		if !strings.Contains(out, "auto_merge_finalize_pr: false") {
			t.Errorf("config show missing auto_merge_finalize_pr: %s", out)
		}
	})
}

// --- AC-20: hostile input, both directions ----------------------------

func TestFinalizePR_AC20_HostileOutputEscaped(t *testing.T) {
	const hostile = "pwn\x1b[31m\x07"
	script := &ghScript{t: t}
	script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
	script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/" + hostile + "\n"), nil }
	script.checks = func(args []string) ([]byte, error) {
		return jsonChecks(t, []ghCheckEntry{{Name: hostile, State: "FAILURE"}}), nil
	}
	withFinalizePRSeams(t, script.fn, true)

	stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
	if strings.Contains(stdout, "\x1b") || strings.Contains(stdout, "\x07") {
		t.Errorf("hostile control bytes leaked into output unescaped: %q", stdout)
	}
}

func TestFinalizePR_AC20_MalformedIDConstructsZeroArgv(t *testing.T) {
	withFinalizePRSeams(t, func(ctx context.Context, args ...string) ([]byte, error) {
		t.Fatalf("malformed id must never reach a gh invocation: %v", args)
		return nil, nil
	}, true)

	t.Run("malformed spec id", func(t *testing.T) {
		var so, se bytes.Buffer
		runFinalizePRAutomation(&so, &se, &config.Config{AutoOpenFinalizePR: true}, "--evil;rm", fprEpicID, "chore/finalize---evil;rm")
		// S2-1: pin the EXPLICIT idvalidate.SpecID gate's own message
		// text, not just "invalid" — a redundant later gate
		// (workspace.FinalizeBranch's own idvalidate.SpecID call) would
		// ALSO fail closed on this input with a DIFFERENT message
		// ("could not derive the finalize branch name"), so a bare
		// "invalid" substring wouldn't uniquely prove THIS gate fired.
		if !strings.Contains(se.String(), "spec id") || !strings.Contains(se.String(), "is invalid") {
			t.Errorf("expected the explicit idvalidate.SpecID gate's own warning (\"spec id ... is invalid\"), got: %s", se.String())
		}
	})

	t.Run("malformed epic id", func(t *testing.T) {
		var so, se bytes.Buffer
		runFinalizePRAutomation(&so, &se, &config.Config{AutoOpenFinalizePR: true}, fprSpecID, "--evil;rm", fprHead())
		// S2-1: pin the EXPLICIT idvalidate.BeadID(epicID) gate's own
		// message text, distinguishing it from any other gate.
		if !strings.Contains(se.String(), "epic id") || !strings.Contains(se.String(), "is invalid") {
			t.Errorf("expected the explicit idvalidate.BeadID gate's own warning (\"epic id ... is invalid\"), got: %s", se.String())
		}
	})
}

// --- AC-21: landed-then-error reconcile --------------------------------

func TestFinalizePR_AC21_LandedThenErrorReconcile(t *testing.T) {
	t.Run("created then errored -> treated as success", func(t *testing.T) {
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return nil, errFake("client lost the response") }
		script.reconcile = func(args []string) ([]byte, error) {
			return jsonEntries(t, []finalizePREntry{{State: "OPEN", URL: "https://x/pull/5", HeadRefName: fprHead(), BaseRefName: "main"}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)

		stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true})
		if strings.Contains(stdout, "does not appear to have been created or merged") {
			t.Errorf("a server-side created PR must never be reported stranded: %s", stdout)
		}
		if !strings.Contains(stdout, "reconciled as open") {
			t.Errorf("expected the reconciled-open success line, got: %s", stdout)
		}
	})

	t.Run("merged then errored -> treated as success, post-merge refresh runs", func(t *testing.T) {
		refreshed := false
		script := &ghScript{t: t}
		script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
		script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/6\n"), nil }
		script.checks = func(args []string) ([]byte, error) {
			return jsonChecks(t, []ghCheckEntry{{Name: "ci", State: "SUCCESS"}}), nil
		}
		script.merge = func(args []string) ([]byte, error) { return nil, errFake("client lost the response") }
		script.reconcile = func(args []string) ([]byte, error) {
			return jsonEntries(t, []finalizePREntry{{State: "MERGED", URL: "https://x/pull/6", HeadRefName: fprHead(), BaseRefName: "main"}}), nil
		}
		withFinalizePRSeams(t, script.fn, true)
		fetchRemoteBranchFn = func(remote, branch string) error { refreshed = true; return nil }

		stdout, _ := runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})
		if !strings.Contains(stdout, "reconciled as merged") {
			t.Errorf("expected the reconciled-merged success line, got: %s", stdout)
		}
		if !refreshed {
			t.Error("a reconciled merge must still run the post-merge origin/main refresh")
		}
	})
}

// --- AC-4: real bare-origin merge; both suppressions clear -------------

func runGitFPR(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFileFPR(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := dir + "/" + rel
	if err := os.MkdirAll(full[:strings.LastIndex(full, "/")], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ac4Fixture holds the real bare-origin fixture setupAC4Fixture builds.
type ac4Fixture struct {
	root, origin, head string
}

// setupAC4Fixture builds a REAL bare origin remote and a real
// chore/finalize carrier (never pushed to origin — mirrors that the
// automation's OWN `pr create`/`pr merge` legs are what push/merge it,
// per the real FinalizeEpic flow this fixture stands in for), chdirs the
// test into root, and returns the identifiers AC-4's variants share.
func setupAC4Fixture(t *testing.T) ac4Fixture {
	t.Helper()
	origin := t.TempDir()
	runGitFPR(t, "", "init", "--bare", "--initial-branch=main", origin)

	root := t.TempDir()
	runGitFPR(t, "", "init", "--initial-branch=main", root)
	runGitFPR(t, root, "config", "user.email", "test@mindspec.dev")
	runGitFPR(t, root, "config", "user.name", "MindSpec Test")

	writeFileFPR(t, root, ".beads/issues.jsonl", `{"id":"`+fprEpicID+`","status":"in_progress"}`+"\n")
	runGitFPR(t, root, "add", "-A")
	runGitFPR(t, root, "commit", "-m", "init")
	runGitFPR(t, root, "remote", "add", "origin", origin)
	runGitFPR(t, root, "push", "origin", "main")

	head := fprHead()
	runGitFPR(t, root, "checkout", "-b", head)
	writeFileFPR(t, root, ".beads/issues.jsonl", `{"id":"`+fprEpicID+`","status":"closed"}`+"\n")
	runGitFPR(t, root, "add", "-A")
	runGitFPR(t, root, "commit", "-m", "chore(beads): finalize epic "+fprEpicID+" for spec "+fprSpecID)
	runGitFPR(t, root, "checkout", "main")
	runGitFPR(t, root, "push", "origin", head)

	origWD, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	return ac4Fixture{root: root, origin: origin, head: head}
}

// mergeServerSide models what `gh pr merge --merge` does on GitHub's own
// infrastructure: a TRUE merge commit landing on origin's main via a
// SEPARATE clone, never via `root` itself (panel round 1 O2-2). Doing
// the merge+push directly in root — the earlier draft's approach — was
// unintentionally self-defeating as an isolation test: a successful
// `git push` from root ALSO updates root's own local
// refs/remotes/origin/main tracking ref as a side effect, so
// postMergeRefresh's fetch leg could be deleted entirely and the AC-4
// assertion would still pass. Routing the merge through an independent
// clone means root's own origin/main tracking ref stays genuinely stale
// until postMergeRefresh's OWN fetch runs — isolating exactly the R2(c)
// fetch leg AC-4 is meant to pin.
func mergeServerSide(t *testing.T, fx ac4Fixture) {
	t.Helper()
	otherClone := t.TempDir()
	runGitFPR(t, "", "clone", fx.origin, otherClone)
	runGitFPR(t, otherClone, "config", "user.email", "test@mindspec.dev")
	runGitFPR(t, otherClone, "config", "user.name", "MindSpec Test")
	runGitFPR(t, otherClone, "fetch", "origin", fx.head)
	runGitFPR(t, otherClone, "merge", "--no-ff", "origin/"+fx.head, "-m", "Merge finalize PR")
	runGitFPR(t, otherClone, "push", "origin", "main")
}

func stubAC4EpicMetadata(t *testing.T) {
	t.Helper()
	epicJSON := `[{"id":"` + fprEpicID + `","title":"[SPEC ` + fprSpecID + `] fixture","status":"closed","issue_type":"epic","metadata":{"spec_num":121,"spec_title":"finalizepr"}}]`
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicJSON), nil
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) { return []byte("[]"), nil }))
}

// TestFinalizePR_AC4_RealMergeClearsBothSuppressions asserts that after a
// genuinely server-side merge (mergeServerSide) and the automation's
// REAL post-merge origin/main refresh, lifecycle.ScanIntegrityFindings
// reports neither a finalize_branch nor a stale_tracker finding for this
// spec, even though local main is never touched (AC-4's "even while
// LOCAL main still lags" clause; a pull_advisory finding is tolerated
// and not asserted against). Also pins O2-1: `pr merge` is invoked with
// the merge-commit strategy, never squash/rebase.
func TestFinalizePR_AC4_RealMergeClearsBothSuppressions(t *testing.T) {
	fx := setupAC4Fixture(t)

	var mergeArgs []string
	script := &ghScript{t: t}
	script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
	script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/99\n"), nil }
	script.checks = func(args []string) ([]byte, error) {
		return jsonChecks(t, []ghCheckEntry{{Name: "ci", State: "SUCCESS"}}), nil
	}
	script.merge = func(args []string) ([]byte, error) {
		mergeArgs = args
		mergeServerSide(t, fx)
		return nil, nil
	}

	origGH, origAvail, origFetch := ghRunFn, ghAvailableFn, fetchRemoteBranchFn
	t.Cleanup(func() { ghRunFn, ghAvailableFn, fetchRemoteBranchFn = origGH, origAvail, origFetch })
	ghRunFn = script.fn
	ghAvailableFn = func() bool { return true }
	// AC-4 wants the REAL post-merge refresh (not stubbed) so the
	// suppression evaluates against genuinely-refreshed refs.
	fetchRemoteBranchFn = func(remote, branch string) error {
		return runGitFetchFPR(t, fx.root, remote, branch)
	}

	runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})

	// O2-1: the merge-commit strategy, never squash/rebase — a
	// --squash/--rebase refactor would otherwise pass every OTHER
	// assertion here silently.
	if !containsAdjacentOrFlag(mergeArgs, "--merge") {
		t.Errorf("pr merge argv must carry --merge (true merge commit): %v", mergeArgs)
	}
	if containsAdjacentOrFlag(mergeArgs, "--squash") || containsAdjacentOrFlag(mergeArgs, "--rebase") {
		t.Errorf("pr merge argv must never carry --squash/--rebase: %v", mergeArgs)
	}

	stubAC4EpicMetadata(t)
	findings := lifecycle.ScanIntegrityFindings(fx.root, phase.NewCache())
	for _, fb := range findings.FinalizeBranches {
		if fb.SpecID == fprSpecID {
			t.Errorf("expected no finalize_branch finding for %s, got %+v", fprSpecID, fb)
		}
	}
	for _, st := range findings.StaleTrackers {
		if st.SpecID == fprSpecID && st.Kind == "stale_tracker" {
			t.Errorf("expected no stale_tracker finding for %s (only pull_advisory tolerated), got %+v", fprSpecID, st)
		}
	}
}

// TestFinalizePR_AC4_PostMergeRefreshIsolatesFetchLeg is the O2-2
// isolation control: identical fixture and server-side merge, but
// postMergeRefresh's fetch is stubbed to a no-op. Because
// mergeServerSide merges via a SEPARATE clone (never root), root's own
// refs/remotes/origin/main tracking ref stays stale WITHOUT the fetch —
// so the stale_tracker finding must still fire. This is the differential
// half proving TestFinalizePR_AC4_RealMergeClearsBothSuppressions above
// actually depends on the fetch leg, not on the fixture's own git
// plumbing silently refreshing the ref for it.
func TestFinalizePR_AC4_PostMergeRefreshIsolatesFetchLeg(t *testing.T) {
	fx := setupAC4Fixture(t)

	script := &ghScript{t: t}
	script.lookup = func(args []string) ([]byte, error) { return jsonEntries(t, nil), nil }
	script.create = func(args []string) ([]byte, error) { return []byte("https://x/pull/99\n"), nil }
	script.checks = func(args []string) ([]byte, error) {
		return jsonChecks(t, []ghCheckEntry{{Name: "ci", State: "SUCCESS"}}), nil
	}
	script.merge = func(args []string) ([]byte, error) {
		mergeServerSide(t, fx)
		return nil, nil
	}

	origGH, origAvail, origFetch := ghRunFn, ghAvailableFn, fetchRemoteBranchFn
	t.Cleanup(func() { ghRunFn, ghAvailableFn, fetchRemoteBranchFn = origGH, origAvail, origFetch })
	ghRunFn = script.fn
	ghAvailableFn = func() bool { return true }
	fetchRemoteBranchFn = func(remote, branch string) error { return nil } // no-op: fetch leg disabled

	runAutomation(&config.Config{AutoOpenFinalizePR: true, AutoMergeFinalizePR: true})

	stubAC4EpicMetadata(t)
	findings := lifecycle.ScanIntegrityFindings(fx.root, phase.NewCache())
	found := false
	for _, st := range findings.StaleTrackers {
		if st.SpecID == fprSpecID && st.Kind == "stale_tracker" {
			found = true
		}
	}
	if !found {
		t.Error("with postMergeRefresh's fetch disabled, root's origin/main tracking ref must stay stale and the stale_tracker finding must still fire — got none (the fixture is not actually isolating the fetch leg)")
	}
}

func containsAdjacentOrFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func runGitFetchFPR(t *testing.T, dir, remote, branch string) error {
	t.Helper()
	cmd := exec.Command("git", "fetch", remote, branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git fetch %s %s: %v\n%s", remote, branch, err, out)
	}
	return err
}
