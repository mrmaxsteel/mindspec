package main

// Spec 120 R4 (converging pass): pins the `mindspec next` orphan-refusal
// render wiring directly. checkUnmergedBeads delegates to the shared
// lifecycle.FindOrphanedClosedBeads predicate, whose returned Orphan.BeadID
// is unvalidated bd-list data (see internal/lifecycle/orphans_test.go's
// hostile-BeadID pin on the underlying RecoveryCommand primitive) — this
// test pins the SAME guarantee at the call-site formatting function
// (unmergedBeadError), extracted from checkUnmergedBeads specifically so a
// hostile Orphan can be driven through it without a live bd/git repo.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

func TestUnmergedBeadError_HostileBeadIDForcedQuoted(t *testing.T) {
	hostileIDs := []string{
		"mindspec-1\n--help",
		"mindspec-1;evil",
		"mindspec-1 --force; rm -rf /",
	}
	for _, id := range hostileIDs {
		o := lifecycle.Orphan{BeadID: id, BeadBranch: "bead/" + id, SpecBranch: "spec/008-test"}
		err := unmergedBeadError(o)
		if err == nil {
			t.Fatalf("unmergedBeadError(%q) returned nil", id)
		}
		msg := err.Error()
		wantQuoted := strconv.Quote(id)
		if !strings.Contains(msg, wantQuoted) {
			t.Errorf("unmergedBeadError(%q) = %q, want the forced-quoted id %q present", id, msg, wantQuoted)
		}
		// The message template itself carries exactly one structural
		// newline (between the two sentences). A hostile newline embedded
		// in id must be folded into the escaped `\n` two-byte sequence by
		// strconv.Quote — never surface as a SECOND real newline that
		// forges an extra terminal line.
		if got := strings.Count(msg, "\n"); got != 1 {
			t.Errorf("unmergedBeadError(%q) has %d real newlines, want exactly 1 (structural); hostile newline leaked raw: %q", id, got, msg)
		}
	}
}

func TestUnmergedBeadError_CleanBeadIDByteIdentical(t *testing.T) {
	const clean = "mindspec-9cyu.1"
	o := lifecycle.Orphan{BeadID: clean, BeadBranch: "bead/" + clean, SpecBranch: "spec/008-test"}
	err := unmergedBeadError(o)
	if err == nil {
		t.Fatal("unmergedBeadError returned nil")
	}
	msg := err.Error()
	want := "bead " + clean + " was closed without `mindspec complete` — merge topology is broken.\nRun `mindspec complete " + clean + "` to recover, then retry `mindspec next`."
	if msg != want {
		t.Errorf("unmergedBeadError(clean) = %q, want byte-identical %q", msg, want)
	}
}
