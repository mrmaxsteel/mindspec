package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const briefRecovery = "mindspec next-action"

// Brief renders an actor's durable inputs without running a reducer or writing files.
func Brief(s State, step string) (body string, err error) {
	defer func() {
		if err != nil {
			err = pathFailure(err, "cannot render brief: ", briefRecovery)
		}
	}()
	r, exists := s.Records[step]
	t, has, e := r.Tuple()
	if !s.Enrolled || !exists || !owned(s, step) || e != nil || !has {
		return "", termsafe.NewFailure("brief requires an actor step of an enrolled spec", briefRecovery)
	}
	switch t.Kind {
	case "implement", "review-slot", "lens", "verify", "fix", "change-judge":
	default:
		return "", termsafe.NewFailure("brief requires an actor step of an enrolled spec", briefRecovery)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\nStep: %s\n", t.Kind, termsafe.Escape(step))
	sub, _, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "Subject: %s\n", termsafe.Escape(sub.Commit))
	switch t.Kind {
	case "implement":
		fmt.Fprintf(&b, "Read the work-item brief:\nmindspec next %s --emit-only\n", briefQuote(t.WorkItem))
		if err := renderBriefRules(&b, "implementer"); err != nil {
			return "", err
		}
	case "review-slot":
		reg, _, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "Read the slot brief:\nmindspec panel prompt %s --slot R%d --spec %s\n", briefQuote(reg.Slug), t.Slot, briefQuote(idrender.Spec(s.SpecID)))
	case "verify":
		b.WriteString(verifyRecordFields)
		findings, has, err := Decode[[]Finding](r.Metadata, KeyFindings)
		if err != nil {
			return "", err
		}
		if !has || len(findings) == 0 {
			return "", fmt.Errorf("verify step has no mindspec_findings")
		}
		renderFindings(&b, findings)
	case "fix":
		if err := renderBriefContext(&b, s, t.WorkItem); err != nil {
			return "", err
		}
		findings, has, err := Decode[[]Finding](r.Metadata, KeyReproduced)
		if err != nil {
			return "", err
		}
		if !has || len(findings) == 0 {
			return "", fmt.Errorf("fix step has no mindspec_reproduced")
		}
		b.WriteString("\nFix these reproduced findings.\n")
		renderFindings(&b, findings)
		if err := renderBriefRules(&b, "fixer"); err != nil {
			return "", err
		}
		if t.WorkItem == "" {
			b.WriteString("\nBefore returning, run the verification commands in the approved plan.\n")
		} else {
			b.WriteString("\nBefore returning, run:\n")
			fmt.Fprintf(&b, "mindspec complete %s --check\n", briefQuote(t.WorkItem))
		}
	case "lens":
		if err := renderBriefContext(&b, s, t.WorkItem); err != nil {
			return "", err
		}
		gate := activeGate(s, t.Gate)
		round, _, err := Decode[GateRound](s.Records[gate].Metadata, KeyRound)
		if err != nil {
			return "", err
		}
		paths, err := briefScope(s, gate, round)
		if err != nil {
			return "", err
		}
		renderPaths(&b, "Files in Scope", paths)
		base, err := briefBase(s, t.WorkItem, round.Subject)
		if err != nil {
			return "", err
		}
		if err := renderBriefDiff(&b, s, base, sub.Commit, paths); err != nil {
			return "", err
		}
		b.WriteString("\nReturn a record with step and lenses; each lens has slot and lens. Give one nonempty lens to each non-adversarial slot and none to an adversarial slot.\n")
	case "change-judge":
		gate := activeGate(s, t.Gate)
		accepted, err := AcceptedSubject(s, gate)
		if err != nil {
			return "", err
		}
		source := judgementGate(s, s.Records[gate])
		round, _, err := Decode[GateRound](source.Metadata, KeyRound)
		if err != nil {
			return "", err
		}
		ceiling := round.SlotCount
		if ceiling == 0 {
			reg, _, err := Decode[RegistrationRecord](source.Metadata, KeyRegistration)
			if err != nil {
				return "", err
			}
			ceiling = len(reg.Slots)
		}
		fmt.Fprintf(&b, "Accepted subject: %s\nNew subject: %s\nSlot ceiling: %d\n", termsafe.Escape(accepted), termsafe.Escape(sub.Commit), ceiling)
		paths, err := subjectPaths(s, gate, accepted, sub.Commit)
		if err != nil {
			return "", err
		}
		renderPaths(&b, "Changed paths", paths)
		if err := renderBriefDiff(&b, s, accepted, sub.Commit, paths); err != nil {
			return "", err
		}
		b.WriteString("\nReturn carry-over with a reason, or re-panel with slot_count and paths covering every changed path. Include value, old_subject, and new_subject.\n")
	}
	if recordKind(t.Kind) {
		lease, _, err := Decode[LeaseRecord](r.Metadata, KeyLease)
		if err != nil {
			return "", err
		}
		dir, err := kernelTempDir(s.Root, false)
		if err != nil {
			return "", err
		}
		name := filepath.Join(dir, "records", fmt.Sprintf("%s.g%d.json", step, lease.Generation))
		fmt.Fprintf(&b, "\nRecord path: %s\nCreate its parent directory if needed. Write the record there; do not commit it.\nSubmit:\nmindspec submit %s --lease '<token>' --record %s\n", termsafe.Escape(name), briefQuote(step), briefQuote(name))
	}
	return b.String(), nil
}

// ReviewSlotBody renders the shared round body. Prior asks come only from metadata.
func ReviewSlotBody(s State, gate string) (body string, err error) {
	defer func() {
		if err != nil {
			err = pathFailure(err, "cannot render review brief: ", briefRecovery)
		}
	}()
	g := s.Records[gate]
	t, has, err := g.Tuple()
	if err != nil {
		return "", err
	}
	if !s.Enrolled || !owned(s, gate) || !has || (t.Kind != "bead-gate" && t.Kind != "final-gate") {
		return "", fmt.Errorf("review brief requires an enrolled gate")
	}
	round, has, err := Decode[GateRound](g.Metadata, KeyRound)
	if err != nil {
		return "", err
	}
	if !has {
		return "", fmt.Errorf("gate has no authorized round")
	}
	var b strings.Builder
	b.WriteString("## Summary\n\n")
	if err := renderBriefContext(&b, s, t.WorkItem); err != nil {
		return "", err
	}
	paths, err := briefScope(s, gate, round)
	if err != nil {
		return "", err
	}
	renderPaths(&b, "Files in Scope", paths)
	if round.Round > 1 && round.Prior != nil {
		paths, err := subjectPaths(s, gate, round.Prior.Subject, round.Subject)
		if err != nil {
			return "", err
		}
		renderPaths(&b, "Paths changed since the prior round subject", paths)
	}
	b.WriteString("\n## Prior-Round Asks\n\n")
	if round.Prior == nil || len(round.Prior.Asks) == 0 {
		b.WriteString("None.\n")
	} else {
		for _, ask := range round.Prior.Asks {
			renderFindings(&b, []Finding{ask.Finding})
			for _, ret := range ask.Returns {
				fmt.Fprintf(&b, "Return: %s\n", termsafe.Escape(ret.Return))
				for _, field := range []struct{ name, value string }{{"Ground", ret.Ground}, {"Acceptance criterion", ret.AC}, {"Reason", ret.Reason}, {"Evidence", ret.Evidence}, {"Target", ret.Target}} {
					if field.value != "" {
						fmt.Fprintf(&b, "%s: %s\n", field.name, termsafe.Escape(field.value))
					}
				}
			}
		}
	}
	b.WriteString("\nDo not raise a refuted or deferred finding again without new evidence.\n")
	return b.String(), nil
}

// renderBriefRules prints the conduct rules of one actor scope from the one
// section that rules.Section renders, so a fresh agent gets them from its brief.
func renderBriefRules(b *strings.Builder, scope string) error {
	section, err := rules.Section(scope)
	if err != nil {
		return err
	}
	b.WriteString(section)
	return nil
}

// verifyRecordFields names the fields of the verify record and their values.
const verifyRecordFields = `
Return a record with these fields:
- step: the Step value above
- subject: the Subject value above
- returns: one entry for each finding below
Each entry in returns has these fields:
- slot: the slot of the finding, a string
- round: the round of the finding, a number
- finding: the finding as a string, ccr[<i>] or veto, not a number
- finding_sha256: the Digest value of the finding
- return: reproduced, refuted, or deferred
A refuted entry also needs ground, reason, and evidence (the resolving evidence); an out_of_acceptance_scope ground also needs ac. A deferred entry also needs reason and target. A veto cannot be deferred.
`

func renderFindings(b *strings.Builder, findings []Finding) {
	for _, f := range findings {
		fmt.Fprintf(b, "\nFinding: %s round %d %s\nDigest: %s\nText: %s\n", termsafe.Escape(f.Slot), f.Round, termsafe.Escape(f.Finding), termsafe.Escape(f.FindingSHA256), termsafe.Escape(f.Text))
	}
}
func renderPaths(b *strings.Builder, title string, paths []string) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	if len(paths) == 0 {
		b.WriteString("None.\n")
	}
	for _, path := range paths {
		fmt.Fprintf(b, "- Path: %s\n", termsafe.Escape(path))
	}
}
func renderBriefContext(b *strings.Builder, s State, item string) error {
	fmt.Fprintf(b, "Spec: %s\nPlan: %s\n", termsafe.Escape(filepath.Join(s.SpecDir, "spec.md")), termsafe.Escape(filepath.Join(s.SpecDir, "plan.md")))
	items := []string{item}
	if item == "" {
		data, err := os.ReadFile(filepath.Join(s.SpecDir, "spec.md"))
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "Spec summary: %s\n", termsafe.Escape(string(data)))
		items = s.Items
	}
	for _, id := range items {
		r, ok := s.Records[id]
		if !ok {
			return fmt.Errorf("work-item context is absent")
		}
		fmt.Fprintf(b, "Work item: %s\nTitle: %s\nContext: %s\n", termsafe.Escape(id), termsafe.Escape(r.Title), termsafe.Escape(r.Description))
	}
	return nil
}
func briefScope(s State, gate string, round GateRound) ([]string, error) {
	if len(round.Paths) > 0 {
		return round.Paths, nil
	}
	t, _, _ := s.Records[gate].Tuple()
	base, err := briefBase(s, t.WorkItem, round.Subject)
	if err != nil {
		return nil, err
	}
	return subjectPaths(s, gate, base, round.Subject)
}
func briefBase(s State, item, subject string) (string, error) {
	if item == "" {
		return actorGit(s.Root, "merge-base", "main", subject)
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind == "implement" && t.WorkItem == item && passed(s, r) {
			lease, has, err := Decode[LeaseRecord](r.Metadata, KeyLease)
			if err != nil {
				return "", err
			}
			if has && lease.ClaimBase != "" {
				return lease.ClaimBase, nil
			}
		}
	}
	return "", fmt.Errorf("work item has no passed implementation claim base")
}
func renderBriefDiff(b *strings.Builder, s State, old, newest string, paths []string) error {
	b.WriteString("\n## Diff\n\n")
	if len(paths) == 0 {
		b.WriteString("None.\n")
		return nil
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", old, newest, "--"}
	data, err := actorGitBytes(s.Root, append(args, paths...)...)
	if err != nil {
		return err
	}
	// Label the escaped value so source text cannot become brief instructions.
	fmt.Fprintf(b, "Diff text: %s\n", termsafe.Escape(string(data)))
	return nil
}
func briefQuote(value string) string {
	return "'" + strings.ReplaceAll(termsafe.EscapeRecovery(value), "'", "'\"'\"'") + "'"
}
