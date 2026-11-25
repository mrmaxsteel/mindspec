package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// The Next block of `panel tally` on a gate that is not a kernel gate. The
// route comes from Decision.Leg, never from Decision.Message.

// panelOperatorActionLine is the operator-action line of the Next block.
const panelOperatorActionLine = "A REJECT or missing required measurement artifact needs operator action. A text edit does not supply an artifact."

// panelHaltLine starts the halt lines of the Next block.
const panelHaltLine = "Halt and report the panel state."

// panelIncompleteHaltLine follows the re-tally line of the incomplete route.
const panelIncompleteHaltLine = "If that tally still blocks: halt and report the panel state."

// panelAdhocPassLine is the Next block of a passing panel that has no pass
// verb. It names no command, so the agent does not go to the loop.
const panelAdhocPassLine = "The review passed. Report the result to the user and stop."

type nextRoute int

const (
	routeNone nextRoute = iota
	routePass
	routeIncomplete
	routeUnresolved
	routeOperator
	routeStale
	routeHalted
)

// panelNextRoute selects the route of the Next block from the decision leg.
// A Block with an unset or unknown leg takes the halted route.
func panelNextRoute(d panel.Decision, f tallyRecoveryFacts) nextRoute {
	switch d.Action {
	case panel.Allow:
		if d.Leg == panel.LegPass {
			return routePass
		}
		return routeNone
	case panel.Warn:
		return routeHalted
	case panel.Block:
		switch d.Leg {
		case panel.LegIncomplete:
			return routeIncomplete
		case panel.LegUnresolved:
			if f.ArtifactMissing {
				return routeOperator
			}
			return routeUnresolved
		case panel.LegOperator:
			return routeOperator
		case panel.LegStale:
			return routeStale
		}
		return routeHalted
	}
	return routeNone
}

// nextBlockOperands holds the shell-safe operands of the Next block. A value
// that the panel did not record renders as a placeholder.
type nextBlockOperands struct {
	slug, spec, bead, target, gate, round, nextRound string
}

func newNextBlockOperands(f tallyRecoveryFacts) nextBlockOperands {
	o := nextBlockOperands{
		slug:      "<panel-slug>",
		spec:      "<spec-id>",
		bead:      "<bead-id>",
		target:    "<target>",
		gate:      f.Gate,
		round:     "N",
		nextRound: "<N+1>",
	}
	if f.Slug != "" {
		o.slug = shellQuoteTarget(escapeConfigValue(f.Slug))
	}
	if f.SpecID != "" {
		o.spec = shellQuoteTarget(idrender.Spec(f.SpecID))
	}
	if f.BeadID != "" {
		o.bead = idrender.Bead(f.BeadID)
		if o.gate == "" {
			o.gate = "bead"
		}
	}
	if f.Target != "" {
		o.target = shellQuoteTarget(escapeConfigValue(f.Target))
	}
	if f.NextRound > 1 {
		o.round = strconv.Itoa(f.NextRound - 1)
		o.nextRound = strconv.Itoa(f.NextRound)
	}
	return o
}

// createArgv is the next `panel create` round with the flags of the gate.
func (o nextBlockOperands) createArgv() string {
	var b strings.Builder
	b.WriteString("mindspec panel create " + o.slug)
	if o.gate != "adhoc" {
		b.WriteString(" --spec " + o.spec)
	}
	if o.gate != "" {
		b.WriteString(" --gate " + shellQuoteTarget(escapeConfigValue(o.gate)))
	}
	b.WriteString(" --target " + o.target)
	if o.gate == "bead" {
		b.WriteString(" --bead " + o.bead)
	}
	b.WriteString(" --round " + o.nextRound + " --brief-file <absolute path>")
	return b.String()
}

// passVerb is the verb that a passing gate runs next. Adhoc has none.
func (o nextBlockOperands) passVerb() string {
	switch o.gate {
	case "bead":
		return "mindspec complete " + o.bead
	case "spec_approve":
		return "mindspec spec approve " + o.spec
	case "plan_approve":
		return "mindspec plan approve " + o.spec
	case "final_review":
		return "mindspec impl approve " + o.spec
	}
	return ""
}

func (o nextBlockOperands) recoveryLines() []string {
	return []string{
		"when the subject needs a new round: " + o.createArgv(),
		"for a passing panel whose changed paths need no new review: mindspec panel rebind " + o.slug + " --reason \"<who>: <why>\"",
		"by the operator only: mindspec panel abandon " + o.slug + " --reason \"<why>\" --by \"<name>\"",
	}
}

func (o nextBlockOperands) haltLines() []string {
	return o.haltLinesAfter(panelHaltLine)
}

// haltLinesAfter is the halt line given, then the operator recovery lines.
func (o nextBlockOperands) haltLinesAfter(halt string) []string {
	lines := []string{halt, "For the operator, run one of these:"}
	return append(lines, o.recoveryLines()...)
}

// dispositionLines are the disposition append lines. An adhoc panel has no
// spec, and disposition append needs one, so adhoc has none.
func (o nextBlockOperands) dispositionLines() []string {
	if o.gate == "adhoc" {
		return nil
	}
	return []string{"for each finding that no verb captured: mindspec panel disposition append --spec " + o.spec + " --panel " + o.slug + " --data -"}
}

// verdictVerbLines are the refute and defer lines. Adhoc has no defer.
func (o nextBlockOperands) verdictVerbLines() []string {
	lines := []string{
		"to refute a finding with evidence: mindspec panel refute " + o.slug + " --slot <slot> --round " + o.round + " --finding 'ccr[0]' --ground disproven --reason \"<why>\" --evidence <reference> --by \"<name>\"",
	}
	switch o.gate {
	case "adhoc":
	case "spec_approve":
		lines = append(lines, "to defer a finding to the plan: mindspec panel defer "+o.slug+" --slot <slot> --round "+o.round+" --finding 'ccr[0]' --to-plan --by \"<name>\"")
	default:
		lines = append(lines, "to defer a finding to an open follow-up: mindspec panel defer "+o.slug+" --slot <slot> --round "+o.round+" --finding 'ccr[0]' --bead <follow-up> --by \"<name>\"")
	}
	return lines
}

// renderPanelNextBlock returns the Next block lines for the route, or nil when
// the route prints none.
func renderPanelNextBlock(d panel.Decision, f tallyRecoveryFacts) []string {
	o := newNextBlockOperands(f)
	var lines []string
	switch panelNextRoute(d, f) {
	case routePass:
		verb := o.passVerb()
		if verb == "" {
			verb = panelAdhocPassLine
		}
		lines = append(lines, verb)
	case routeIncomplete:
		if f.SlotsErr != nil || len(f.MissingSlots) == 0 {
			reason := "the slot status is unknown"
			if f.SlotsErr != nil {
				reason = "the slot status could not be read: " + termsafe.Escape(f.SlotsErr.Error())
			}
			lines = append(lines, reason, "then: mindspec panel tally "+o.slug)
			lines = append(lines, o.haltLinesAfter(panelIncompleteHaltLine)...)
			break
		}
		for _, slot := range f.MissingSlots {
			s := shellQuoteTarget(escapeConfigValue(slot))
			lines = append(lines,
				"mindspec panel checkout "+o.slug+" --slot "+s+" --dispatch",
				"mindspec panel prompt "+o.slug+" --slot "+s+"   (give the output to a fresh reviewer agent)")
		}
		lines = append(lines, "then: mindspec panel tally "+o.slug)
		lines = append(lines, o.haltLinesAfter(panelIncompleteHaltLine)...)
	case routeUnresolved:
		lines = append(lines, o.dispositionLines()...)
		lines = append(lines, o.verdictVerbLines()...)
		lines = append(lines,
			"fix: mindspec panel verify "+o.slug,
			"fix: mindspec panel checkout "+o.slug+" --fixer",
			"fix: mindspec rules --scope fixer",
			"fix: give the reproduced findings to one fresh fixer agent",
		)
		if o.gate == "bead" {
			lines = append(lines, "fix: mindspec complete "+o.bead+" --check")
		}
		lines = append(lines,
			"fix: make one commit",
			"fix: "+o.createArgv(),
		)
	case routeOperator:
		lines = append(lines, panelOperatorActionLine)
		if d.Leg == panel.LegUnresolved {
			lines = append(lines, o.verdictVerbLines()...)
		}
		lines = append(lines, o.haltLines()...)
	case routeStale:
		lines = append(lines, "Run one of these:")
		lines = append(lines, o.recoveryLines()...)
	case routeHalted:
		lines = append(lines, o.haltLines()...)
	default:
		return nil
	}
	return lines
}

// formatPanelNextBlock renders the lines under a "Next:" heading.
func formatPanelNextBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Next:")
	for _, line := range lines {
		fmt.Fprintf(&b, "\n  %s", line)
	}
	return b.String()
}

// tallyRerunRecovery is the one recovery line of a blocked non-kernel tally.
func tallyRerunRecovery(slug string) string {
	rendered := "<panel-slug>"
	if slug != "" {
		rendered = shellQuoteTarget(escapeConfigValue(slug))
	}
	return "mindspec panel tally " + rendered + "   (after the Next steps above)"
}
