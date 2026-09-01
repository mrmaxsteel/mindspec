package phase

// sanctioned_sync.go — spec 128 (lifecycle-verb-state-honesty) Bead 1,
// R3.3 (round-1 G4.2 pin): the ONE sanctioned backward mindspec_phase
// write surface.
//
// `mindspec repair phase` (cmd/mindspec/repair.go) is the OPERATOR-facing
// backward-move guard: it refuses a backward write unless
// --force-backward "<reason>" is passed. SanctionedDerivedSync is the
// distinct INTERNAL surface a tool-performed sync may use to re-derive
// and write the child-derived phase — backward permitted, no force flag,
// no recorded reason — because the caller (currently: bead 2's `release`
// step-6.5 sync) is re-deriving from FRESH evidence (the epic's own
// children) inside this function's own lock critical section, not
// asking a human to override a guard against possibly-stale evidence.
//
// Exported NOW (bead 1) so bead 2 can wire its ONE production caller
// without touching this file again; its exported-ness is fenced by
// TestSanctionedDerivedSync_NoCallersOutsideAllowlist
// (sanctioned_sync_test.go), which at THIS bead's gate asserts ZERO
// production callers outside the allowlist {cmd/mindspec/release.go} —
// satisfiable before release is wired — and bead 2 TIGHTENS the same
// test to exactly-one when it adds the real call site.

// SanctionedDerivedSync re-derives epicID's child-derived lifecycle phase
// and writes it to mindspec_phase, backward move permitted, entirely
// inside WithPhaseWriteLock's critical section — so the re-derivation
// (children read) and the write happen atomically with respect to every
// other mindspec_phase writer.
//
// This is the ONLY sanctioned backward write surface. It performs NO
// value comparison against the stored phase and takes NO force/reason —
// it always writes whatever it freshly derives. Any other caller wanting
// a backward move MUST go through the operator-facing
// `mindspec repair phase --force-backward "<reason>"` guard instead.
func SanctionedDerivedSync(root, epicID string) error {
	return WithPhaseWriteLock(root, epicID, mergeMetadataFn, func(string) (map[string]interface{}, error) {
		c := NewCache()
		children, _ := c.GetChildren(epicID)
		derived := DerivePhaseFromChildren(children)
		if derived == "" {
			return nil, nil
		}
		return map[string]interface{}{"mindspec_phase": derived}, nil
	})
}
