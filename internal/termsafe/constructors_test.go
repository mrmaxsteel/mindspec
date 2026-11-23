package termsafe_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

type rawCause struct {
	text  string
	cause error
}

func (e *rawCause) Error() string { return e.text }
func (e *rawCause) Unwrap() error { return e.cause }

func TestRefusalConstructors(t *testing.T) {
	cause := errors.New("cause")
	messages := []struct{ name, raw, flat, lines string }{
		{"recovery", "recovery: x", " recovery: x", " recovery: x"},
		{"newline-recovery", "a\nrecovery: x", `a\nrecovery: x`, "a\n recovery: x"},
		{"escape", "a\x1b", `a\x1b`, `a\x1b`},
		{"nul", "a\x00", `a\x00`, `a\x00`},
		{"newline", "\n", `\n`, "\n"},
	}
	for _, constructor := range []string{"new", "wrapped", "prefix", "join", "lines", "guard", "guard-prefix", "guard-prefix-multiple"} {
		for _, msg := range messages {
			t.Run(constructor+"/"+msg.name, func(t *testing.T) {
				var err error
				var guardCause error
				want := msg.flat
				switch constructor {
				case "new":
					err = termsafe.NewFailure(msg.raw, "inspect 'José'")
				case "wrapped":
					err = termsafe.NewWrappedFailure(cause, msg.raw, "inspect 'José'")
				case "prefix":
					err = termsafe.PrefixFailure(termsafe.NewWrappedFailure(cause, "", "inspect 'José'"), msg.raw)
				case "join":
					err = termsafe.JoinFailures([]error{&rawCause{msg.raw, cause}}, "inspect 'José'")
					if msg.name != "recovery" {
						want = `"` + msg.flat + `"`
					}
				case "lines":
					err = termsafe.NewLinesFailure(cause, strings.Split(msg.raw, "\n"), "inspect 'José'")
					want = msg.lines
				case "guard":
					err = guard.NewFailure(msg.raw, "inspect 'José'")
					want = strings.TrimRight(msg.lines, "\n")
				case "guard-prefix-multiple":
					guardCause = guard.NewFailure(msg.raw, "inspect first", "inspect 'José'")
					err = guard.PrefixFailure(guardCause, "", "fallback")
					want = strings.TrimRight(msg.lines, "\n") + "\n recovery: inspect first"
				case "guard-prefix":
					err = guard.PrefixFailure(guard.NewFailure("", "inspect 'José'"), msg.raw, "fallback")
				}
				if guardCause != nil && !errors.Is(err, guardCause) {
					t.Fatal("guard cause lost")
				}
				want += "\nrecovery: inspect 'José'"
				if err.Error() != want {
					t.Fatalf("refusal changed: got %q want %q", err.Error(), want)
				}
				lines := strings.Split(err.Error(), "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, "recovery:") && i != len(lines)-1 {
						t.Fatalf("forged recovery: %q", err)
					}
				}
				if constructor == "wrapped" || constructor == "prefix" || constructor == "join" || constructor == "lines" {
					if !errors.Is(err, cause) {
						t.Fatal("cause lost")
					}
				}
			})
		}
	}
	// A prefix can create the forbidden prefix only after concatenation.
	for _, err := range []error{
		termsafe.PrefixFailure(termsafe.NewFailure("very: x", "inspect"), "reco"),
		guard.PrefixFailure(guard.NewFailure("very: x", "inspect"), "reco", "fallback"),
		termsafe.JoinFailures([]error{errors.New("recovery: x")}, "inspect"),
	} {
		if err.Error() != " recovery: x\nrecovery: inspect" {
			t.Fatalf("joined prefix forged recovery: %q", err)
		}
	}
}
