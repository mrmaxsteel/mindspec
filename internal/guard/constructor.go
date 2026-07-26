package guard

import (
	"fmt"
	"strings"
)

// DestructiveCommand is the opaque, evidence-carrying wrapper spec 127
// R5(b) requires around every destructive-class command that reaches a
// lifecycle diagnostic — recovery line, message body, or labeled
// choice block. It is deliberately a STRUCT with unexported fields,
// NOT a named string type: a named string type compile-enforces
// nothing (untyped string constants are assignable to it, and an
// explicit conversion from a plain string is accepted — D-r4-2's
// compile probe on the rejected alternative). A raw string literal is
// neither assignable NOR explicitly convertible to this struct type,
// so a call site that forgets the constructor fails to compile — for
// callers OUTSIDE this package.
//
// Opacity only seals CROSS-package construction. Any file inside
// package guard can populate command/valid directly via a composite
// literal — Go does not scope unexported-field access below the
// package boundary, and a compiled probe during this bead's authoring
// confirmed it (G-r5-3).
//
// SAME-package construction is REVIEW-CAUGHT by the internal/lint AST
// invariant instead (destructive_guidance_test.go's
// scanSamePackageInvariant), NOT sealed, NOT prevented, and NOT made
// impossible — spec 127 bead-2 rework round 3's RULING 1 supersedes
// every prior round's "comprehensive, bounded honestly" framing of
// this mechanism, after three consecutive rounds of adversarial review
// each found NEW same-package escape shapes the previous round's fix
// did not close (an arbitrary-depth alias/defined-type chain in round
// 1; an elided-type slice/map/struct-field literal in round 2; a
// function-local type declaration, a named container type over the
// sealed type, and an elided pointer element, all in round 3). The
// scan's own fixture list — destructive_guidance_test.go's
// samePackageEscapeShapes, held to that file by a manifest test so it
// cannot silently drift from what scanSamePackageInvariant is actually
// exercised against — is the single source of truth for which
// concrete shapes this mechanism is proven to catch; this comment
// does not restate that list as a count, because every count written
// here or in spec 127's own text has gone stale at least once (bead-2
// rework rounds 4 and 5). The root cause is not a gap this scan can
// eventually close by finding one more shape: Go permits same-package
// code to write unexported fields BY DESIGN, and offers unboundedly
// many ways to name a type — aliases at any depth, defined types,
// conversions, elided literals in slice/map/struct-field/pointer
// context, function-local declarations, and more this bead has not
// yet closed: round 3's confirm pass (O3) found a NAMED POINTER TYPE
// (`type dcNamedPtr *DestructiveCommand` used as a slice element with
// an elided literal) that compiles, forges a live value, and this
// scan does NOT catch — filed as bd mindspec-erpg, deliberately NOT
// fixed here (chasing one more spelling repeats the exact pattern
// that produced this history). A SYNTACTIC (AST-level, no go/types)
// check cannot enumerate that set from the outside, so no future
// round should claim this mechanism "comprehensive" again, and no
// future round should assume a shape named in prose is therefore
// caught — only samePackageEscapeShapes's own fixtures are.
//
// What this scan actually does: flag every concrete escape shape
// listed in destructive_guidance_test.go's samePackageEscapeShapes —
// a real, useful defense-in-depth catch for convenient drift, layered
// UNDER human review of any same-package change to this file, never a
// substitute for it. Not resolved, and not claimed: the named-
// pointer-type shape above, a value that reaches this package only
// after passing through an interface{}/generic-type indirection, or a
// same-named type re-exported through another package's alias —
// either would need actual type-checking (go/types) to resolve, which
// this scan deliberately does not add (Go has indefinitely many
// spellings, and a syntactic scan closes the SHAPES adversarial
// review has actually found AND FIXTURED, not an unbounded claim it
// cannot back).
// unsafe/reflect.NewAt bypasses of the struct are recorded OUTSIDE
// this convention's threat model (R5(b)) — no Go-level guard survives
// unsafe, and the convention targets convenient drift, not deliberate
// obfuscation. reflect alone, WITHOUT unsafe, cannot write an
// unexported field (verified, O2-r2-1) and is therefore not a
// separate escape to defend against.
//
// The zero value (valid == false) is INVALID and renders as a
// fail-closed placeholder, never as a usable command — a caller that
// writes `var d guard.DestructiveCommand` and skips the constructor
// gets an inert value, not a silently blank recovery line. This is a
// deliberately RENDER-time fail-closed leg, not a development-time
// red: nothing in the same-package invariant above flags a bare
// zero-value construction reaching a scanned diagnostic operand
// (O3-r2-9) — the recorded, reviewed decision is that this one bypass
// ships as a fail-closed render, not as an AST-caught compile-time
// error, because the value it produces is inert (never a floor-
// matching command) rather than silently wrong.
type DestructiveCommand struct {
	command string
	valid   bool
}

// NewDestructiveCommand is the ONLY sanctioned way to produce a
// (valid) DestructiveCommand. Two requirements:
//
//   - command must be text the classifier (classifier.go) recognizes
//     as a match on the reviewed floor. This constructor's error path
//     is for a command genuinely OFF that closed, reviewed floor —
//     NOT a claim that such a command is safe (spec 127 bead-2
//     rework, RULING 3/O1-r2-2): the floor is deliberately
//     incomplete, so an off-floor command is caught by review under
//     the ADR-0035 in-diff extension obligation, never proven
//     non-destructive by this constructor's refusal. A genuinely
//     non-destructive recovery line (including `mindspec complete`,
//     made safe upstream by R4) never needs this constructor at all —
//     pass it to guard.FormatFailure/NewFailure as a plain string,
//     exactly as before this spec.
//   - outcome is a mandatory guard.DestructionOutcome parameter — Go
//     has no default arguments, so no call site can construct a
//     DestructiveCommand without at least naming an evaluated
//     DestructionOutcome value ("non-defaultable" per spec 127 R5(b)).
//     This constructor does not itself decide WHICH outcome licenses
//     WHICH command — that mapping is the R2 hint-derivation's and
//     R4 preflight's job, downstream of this bead. Passing
//     DestructionAncestor (this type's zero value, per bead 1's F1-3)
//     is a legitimate, real outcome value here: the enforcement is
//     the mandatory PARAMETER, not a non-zero check on its value —
//     mere possession of a DestructionOutcome is not proof the
//     predicate ran (bead 1's own outcome.go doc comment), and this
//     constructor does not claim otherwise.
//
// Returns a non-nil error — NEVER panics — on an empty command or a
// command that matches no reviewed floor family (spec 127 bead-2
// rework, RULING 1: a panic inside a guard is itself a denial of
// service, and the floor is deliberately incomplete, so unmatched-but-
// destructive input is an EXPECTED input shape, not a programmer
// error; two independent real triggers were verified — a joined
// multi-ID `bd delete <ids...> --force` line at 8+ IDs, and a
// legitimately quoted, whitespace-bearing `git -C <path>` operand,
// both now FIXED at the classifier level, classifier.go, but the
// constructor itself must not re-introduce the same failure mode for
// whatever off-floor destructive input review has not yet caught).
// On error, the returned DestructiveCommand is the type's own INVALID
// zero value — Valid() is false and String() renders the fail-closed
// placeholder below — so a caller that erroneously ignores the error
// still gets an inert value, never a silently wrong command.
func NewDestructiveCommand(command string, outcome DestructionOutcome) (DestructiveCommand, error) {
	_ = outcome // mandatory by signature; evidentiary gating is the caller's (R2/R4's) job
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return DestructiveCommand{}, fmt.Errorf("guard: NewDestructiveCommand requires a non-empty command")
	}
	if !IsFloorMatch(trimmed) {
		return DestructiveCommand{}, fmt.Errorf(
			"guard: NewDestructiveCommand: %q matches no family on the reviewed destructive floor (spec 127 R5a). "+
				"This is NOT a claim that it is safe: the floor is a reviewed finite set. If the command IS "+
				"destructive, extend AllFamilies in this same change (the ADR-0035 in-diff extension obligation); "+
				"if it is genuinely non-destructive, pass it to FormatFailure/NewFailure as a plain string",
			trimmed)
	}
	return DestructiveCommand{command: trimmed, valid: true}, nil
}

// String renders the wrapped command. The zero value (never produced
// by NewDestructiveCommand) renders a fail-closed placeholder instead
// of silently producing an empty or nonsensical recovery line.
func (d DestructiveCommand) String() string {
	if !d.valid {
		return "<invalid-destructive-command: use guard.NewDestructiveCommand>"
	}
	return d.command
}

// Valid reports whether d was produced by NewDestructiveCommand (as
// opposed to being the type's zero value).
func (d DestructiveCommand) Valid() bool {
	return d.valid
}
