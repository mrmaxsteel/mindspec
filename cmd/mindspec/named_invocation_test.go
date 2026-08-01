package main

// named_invocation_test.go is spec 127 AC-11(a): every `mindspec` command
// string a message or replacement-guidance artifact in AC-1..AC-10 names —
// the adopt invocation and its two flags, `mindspec complete` (bare and
// with `--resolve-merge`), `mindspec impl approve` (with `--resolve-merge`
// and `--allow-net-deletion`), `mindspec repair phase`, `mindspec release`
// (with and without `--force`), and `mindspec panel create` (R5(e)'s
// #186-interim-recovery replacement guidance, spec-orchestrator.md) —
// resolves against the REAL command tree at LEAF IDENTITY
// (`resolveCommand`, `cmd.Name()` equality — ceremony_guard_test.go's
// pattern) with every named flag asserted via flag-set membership on the
// resolved leaf. A bare `rootCmd.Find` err==nil check is forbidden
// (O2-1's reproduced leniency: cobra's `legacyArgs` resolves an unknown
// sub-path to the nearest ancestor without error), which is exactly why
// this file reuses `resolveCommand` rather than calling `rootCmd.Find`
// directly.
//
// AC-11(b) (the foreign-CLI leg: `bd delete <id> --force` pinned to the
// single constructor-produced TEMPLATE, never passed to `resolveCommand`)
// is bead 5's deliverable — see internal/guard/registries_test.go's
// TestBdDeleteForceTemplate_SingleSourceOfTruth — and is not re-verified
// here; this file owns AC-11(a) only.
//
// What makes namedInvocations (below) COMPLETE, stated honestly rather
// than assumed: there is no text-extraction or AST mechanism that reads
// spec.md's AC-1..AC-10 prose and derives this table automatically — no
// such mechanism could, short of NLP over free-form requirement text.
// This table is a REVIEWED ENUMERATION, built by reading every ACs'
// invocation-naming clause at bead-implementation time (R1(a)'s adopt
// invocation, R2(b)/AC-3's `mindspec complete <bead>`, R4(b)'s
// `--allow-net-deletion`, R5(d)/AC-9(v)'s `--resolve-merge` re-entry,
// R5(e)/AC-10's agent-template replacement invocations) and cross-checked
// against the real, current `mindspec --help` command tree (every row
// below was run against the built binary before landing) — the SAME
// review-then-list discipline this spec's own R5(a) known-sites
// exemption list and R5(c) opaque-operand registry already use for an
// analogous claim ("this exact set was reviewed", not "no other instance
// can possibly exist"). A FUTURE requirement naming a new `mindspec`
// invocation without a matching row here is a REVIEW-CAUGHT gap — the
// same in-diff extension obligation R5(a)/R5(b) already rely on for
// their own closed, reviewed (not closure-claimed) sets — not something
// this file's mechanism can catch on its own. What this file's mechanism
// DOES guarantee, mechanically, for every row that IS listed: the
// invocation resolves at real leaf identity (not a typo, not a renamed
// command silently swallowed by cobra's ancestor-fallback leniency) and
// every named flag is genuinely present on that leaf today.

import "testing"

// namedInvocation is one AC-1..AC-10-named `mindspec` command string,
// resolved at leaf identity with its named flags checked for presence
// (membership, not full-set equality — that stricter, cumulative check is
// TestCeremonyNonInflation_HelpFlags's job, a distinct guard).
type namedInvocation struct {
	// label is the human-readable name for t.Run and failure messages.
	label string
	// path is the leaf command's argv path off rootCmd, e.g.
	// []string{"impl", "approve"}.
	path []string
	// wantFlags are the long-flag names ("--foo") this invocation's own
	// AC-cited form names; every one must be present on the resolved
	// leaf's flag set (LocalFlags ∪ InheritedFlags, commandFlagSet's own
	// union — the same one ceremony_guard_test.go's guard reads).
	wantFlags []string
}

// namedInvocations is AC-11(a)'s own table, one row per AC-1..AC-10-named
// invocation shape. Every row is resolved AND flag-checked by
// TestNamedInvocations_ResolveAtLeafIdentity below.
var namedInvocations = []namedInvocation{
	{
		// R1(a): "an explicit, operator-invoked surface ... every AC that
		// names the invocation resolves it in full, including flags";
		// AC-2's refusal legs name --reason and --attest-unverified.
		label:     "mindspec impl adopt",
		path:      []string{"impl", "adopt"},
		wantFlags: []string{"--reason", "--attest-unverified"},
	},
	{
		// R2(b)/AC-3: the normal-unmerged orphan hint remains exactly
		// `mindspec complete <bead>` — the bare form, no flags named.
		label: "mindspec complete (bare)",
		path:  []string{"complete"},
	},
	{
		// R5(d)/AC-9(v): the conflict-recovery re-entry surface on the
		// bead→spec leg; R4(b): the audited work-destruction override.
		label:     "mindspec complete --resolve-merge / --allow-net-deletion",
		path:      []string{"complete"},
		wantFlags: []string{"--resolve-merge", "--allow-net-deletion"},
	},
	{
		// R5(d)/AC-9(v): the conflict-recovery re-entry surface on the
		// spec→main leg; R4(b): the audited work-destruction override
		// (also named at this leg — R4(a) covers all three producers).
		label:     "mindspec impl approve --resolve-merge / --allow-net-deletion",
		path:      []string{"impl", "approve"},
		wantFlags: []string{"--resolve-merge", "--allow-net-deletion"},
	},
	{
		// R5(e): the sanctioned phase-drift repair, replacing the
		// runtime-banned raw `bd update --metadata` workaround.
		label: "mindspec repair phase",
		path:  []string{"repair", "phase"},
	},
	{
		// R5(e)/AC-10(ii): release's pasteable recovery line (the safe,
		// no-flag form) and its separately-labeled `--force` discard.
		label: "mindspec release (bare)",
		path:  []string{"release"},
	},
	{
		label:     "mindspec release --force",
		path:      []string{"release"},
		wantFlags: []string{"--force"},
	},
	{
		// R5(e)'s #186 stale-SHA interim recovery, named in the
		// spec-orchestrator.md replacement guidance this bead ships:
		// re-panel to co-bump round + reviewed_head_sha.
		label:     "mindspec panel create --spec/--target/--round",
		path:      []string{"panel", "create"},
		wantFlags: []string{"--spec", "--target", "--round"},
	},
}

// TestNamedInvocations_ResolveAtLeafIdentity is AC-11(a). Each row's leaf
// must resolve (resolveCommand fails the test otherwise, via its own
// t.Fatalf) and every named flag must be present in the resolved leaf's
// real flag set.
func TestNamedInvocations_ResolveAtLeafIdentity(t *testing.T) {
	for _, inv := range namedInvocations {
		t.Run(inv.label, func(t *testing.T) {
			cmd := resolveCommand(t, inv.path...)
			flags := commandFlagSet(cmd)
			for _, want := range inv.wantFlags {
				if !flags[want] {
					t.Errorf("%s: resolved leaf %q has no flag %s (real flag set: %v)", inv.label, cmd.Name(), want, flags)
				}
			}
		})
	}
}

// TestNamedInvocations_BareFindIsForbidden is a guard-of-the-guard: it
// documents WHY this file never calls rootCmd.Find directly (O2-1's
// reproduced leniency) by proving the failure mode resolveCommand's own
// Name()-equality check exists to catch. cobra's legacyArgs resolves an
// unknown sub-path to the nearest ancestor WITHOUT error, so a bare
// `rootCmd.Find(path); err == nil` check would silently pass for a
// misspelled or renamed leaf.
func TestNamedInvocations_BareFindIsForbidden(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"impl", "definitely-not-a-real-leaf"})
	if err != nil {
		t.Fatalf("rootCmd.Find returned an error for a bogus leaf — this test's own premise (silent ancestor fallback) no longer holds, re-examine whether resolveCommand's Name() check is still needed: %v", err)
	}
	if cmd.Name() == "definitely-not-a-real-leaf" {
		t.Fatal("rootCmd.Find resolved a bogus leaf name to itself — this test's own premise is wrong")
	}
	// cmd silently resolved to the nearest ancestor ("impl") with no
	// error — exactly the false-positive resolveCommand's Name()-equality
	// check catches, and exactly why every row above uses resolveCommand.
}
