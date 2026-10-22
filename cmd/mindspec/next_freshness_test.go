package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// writeStaleSession records a compacted session, which the freshness gate
// treats as not fresh.
func writeStaleSession(t *testing.T, root string) {
	t.Helper()
	raw, err := json.Marshal(state.Session{SessionSource: "compact", SessionStartedAt: "2026-10-22T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	path := workspace.SessionPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNext_FreshnessGate_ExplicitIDWarnsOnly: a claim that names the bead
// gets a warning from the session freshness gate, not a refusal, so lanes
// need no --force. The old gate refused every stale claim without --force.
func TestNext_FreshnessGate_ExplicitIDWarnsOnly(t *testing.T) {
	f := newNextHonestyFixture(t)
	writeStaleSession(t, f.root)
	out, err := f.run(t, "--spec", "129-fixture", "epic-129.1")
	if err != nil && strings.Contains(err.Error(), "session freshness gate") {
		t.Fatalf("explicit-ID claim was refused by the freshness gate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "claiming the named bead anyway") {
		t.Fatalf("explicit-ID claim must print the freshness warning; err=%v\n%s", err, out)
	}
}

// TestNext_FreshnessGate_UnselectedClaimStillRefused: without a bead ID and
// without --force, a stale session still stops the claim.
func TestNext_FreshnessGate_UnselectedClaimStillRefused(t *testing.T) {
	f := newNextHonestyFixture(t)
	writeStaleSession(t, f.root)
	out, err := f.run(t, "--spec", "129-fixture")
	if err == nil || !strings.Contains(err.Error(), "session freshness gate") {
		t.Fatalf("unselected stale claim must be refused by the freshness gate: %v\n%s", err, out)
	}
}

func TestSessionFreshnessGate_Table(t *testing.T) {
	fresh := &state.Session{SessionSource: "startup", SessionStartedAt: "2026-10-22T00:00:00Z"}
	claimed := &state.Session{SessionSource: "startup", SessionStartedAt: "2026-10-22T00:00:00Z", BeadClaimedAt: "2026-10-22T01:00:00Z"}
	for _, tc := range []struct {
		name            string
		sess            *state.Session
		force, explicit bool
		wantErr, warn   bool
	}{
		{"no session", &state.Session{}, false, false, false, false},
		{"fresh", fresh, false, false, false, false},
		{"claimed, unselected", claimed, false, false, true, false},
		{"claimed, explicit", claimed, false, true, false, true},
		{"claimed, force", claimed, true, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warn, err := sessionFreshnessGate(tc.sess, tc.force, tc.explicit)
			if (err != nil) != tc.wantErr || (warn != "") != tc.warn {
				t.Fatalf("warn=%q err=%v", warn, err)
			}
		})
	}
}
