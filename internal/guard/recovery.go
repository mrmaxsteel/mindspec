package guard

import (
	"errors"
	"fmt"
	"strings"
)

// Recovery-line convention (spec 092-agent-contract-hardening, Req 12;
// documented in ADR-0035-agent-error-contract).
//
// Every guard failure ends with one or more machine-greppable lines of
// the form:
//
//	recovery: <command>
//
// One command per line, the LAST line of the message always being a
// recovery line. Agents (and humans) can extract the fix with
// `grep '^recovery: '` and paste it verbatim.
//
// All guard failures touched by spec 092 route through FormatFailure /
// NewFailure, and NEW guards added after that spec MUST do the same —
// enforced by the Req 21 convention test in recovery_convention_test.go.
//
// Safety contract (HC-5 / Req 19): emitted commands must be safe to
// paste. In particular, raw `bd update ... --metadata` is banned — it
// REPLACES the entire metadata map (internal/bead/bdcli.go), silently
// wiping mindspec_migrated_at, doc-skew audit keys, and ADR-override
// keys. Callers needing a phase metadata fix emit
// `mindspec repair phase <spec-id>` instead. FormatFailure panics on a
// banned or malformed command: that is a programmer error, caught at
// development time by the convention test and the formatter's unit
// tests, never reachable from user input.

// RecoveryPrefix is the machine-greppable prefix of a recovery line.
const RecoveryPrefix = "recovery: "

// FormatFailure formats a guard-failure message per the Req 12
// convention: the message body followed by one `recovery: <command>`
// line per command, the final line always being a recovery line.
//
// It panics when no command is given, when a command is empty or spans
// multiple lines, or when a command carries replace/destructive
// semantics (the Req 19 `bd update --metadata` ban) — all programmer
// errors, see the package convention comment above.
func FormatFailure(msg string, commands ...string) string {
	if len(commands) == 0 {
		panic("guard: FormatFailure requires at least one recovery command (spec 092 Req 12)")
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(msg, "\n"))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			panic("guard: FormatFailure recovery command must not be empty (spec 092 Req 12)")
		}
		if strings.Contains(command, "\n") {
			panic(fmt.Sprintf("guard: FormatFailure recovery command must be a single line (spec 092 Req 12): %q", command))
		}
		if IsBannedRecoveryCommand(command) {
			panic(fmt.Sprintf("guard: recovery command %q is banned: raw `bd update --metadata` REPLACES the whole metadata map; emit `mindspec repair phase <spec-id>` instead (spec 092 Req 19, HC-5)", command))
		}
		b.WriteString("\n")
		b.WriteString(RecoveryPrefix)
		b.WriteString(command)
	}
	return b.String()
}

// NewFailure returns FormatFailure(msg, commands...) as an error.
func NewFailure(msg string, commands ...string) error {
	return errors.New(FormatFailure(msg, commands...))
}

// HasFinalRecoveryLine reports whether the final line of msg is a
// non-empty recovery line. This is the predicate the Req 21 convention
// test applies to every guard-failure constructor.
func HasFinalRecoveryLine(msg string) bool {
	msg = strings.TrimRight(msg, "\n")
	if msg == "" {
		return false
	}
	lines := strings.Split(msg, "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, RecoveryPrefix) {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(last, RecoveryPrefix)) != ""
}

// IsBannedRecoveryCommand reports whether command falls under the
// Req 19 ban: raw `bd update ... --metadata` has replace semantics over
// the entire metadata map and must never be pasted by an agent.
func IsBannedRecoveryCommand(command string) bool {
	return strings.Contains(command, "bd update") && strings.Contains(command, "--metadata")
}
