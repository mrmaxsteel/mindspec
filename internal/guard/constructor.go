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
// confirmed it (G-r5-3). SAME-package construction is sealed by the
// internal/lint AST invariant instead (destructive_guidance_test.go):
// the scan asserts every composite literal or unexported-field write
// of this type appears only inside NewDestructiveCommand's own body.
// unsafe/reflect.NewAt bypasses of the struct are recorded OUTSIDE
// this convention's threat model (R5(b)) — no Go-level guard survives
// unsafe, and the convention targets convenient drift, not deliberate
// obfuscation.
//
// The zero value (valid == false) is INVALID and renders as a
// fail-closed placeholder, never as a usable command — a caller that
// writes `var d guard.DestructiveCommand` and skips the constructor
// gets an inert value, not a silently blank recovery line.
type DestructiveCommand struct {
	command string
	valid   bool
}

// NewDestructiveCommand is the ONLY sanctioned way to produce a
// DestructiveCommand. Two requirements, both enforced here:
//
//   - command must be text the classifier (classifier.go) recognizes
//     as a reviewed floor-family match. This constructor exists for
//     DESTRUCTIVE commands only; a non-destructive recovery line
//     (including `mindspec complete`, made safe upstream by R4) never
//     needs it — pass it to guard.FormatFailure/NewFailure as a plain
//     string, exactly as before this spec. A command that doesn't
//     match any floor family is a programmer error here, not a
//     runtime condition: it means this constructor was reached for
//     the wrong kind of command.
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
// Panics on an empty command or a non-floor-match command — both
// programmer errors caught at development time, the same posture
// FormatFailure already takes on a banned `bd update --metadata`
// (Req 19).
func NewDestructiveCommand(command string, outcome DestructionOutcome) DestructiveCommand {
	_ = outcome // mandatory by signature; evidentiary gating is the caller's (R2/R4's) job
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		panic("guard: NewDestructiveCommand requires a non-empty command")
	}
	if !IsFloorMatch(trimmed) {
		panic(fmt.Sprintf(
			"guard: NewDestructiveCommand: %q matches no reviewed destructive floor family (spec 127 R5a) — "+
				"this constructor is for destructive commands only; pass a plain string to FormatFailure/NewFailure "+
				"for a non-destructive recovery line",
			trimmed))
	}
	return DestructiveCommand{command: trimmed, valid: true}
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
