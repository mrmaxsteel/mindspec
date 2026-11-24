package kernel

import (
	"fmt"
	"path/filepath"
)

func repairable(r Record, lease LeaseRecord, valid bool) error {
	if r.Status != "closed" {
		return actorFailure("repair requires a closed step")
	}
	if lease.Generation < 1 {
		return actorFailure("repair refused: step was never leased")
	}
	if valid {
		return actorFailure("repair refused: step already has a valid receipt")
	}
	pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
	if err != nil {
		return err
	}
	if has && (pin.Conflict != nil || pin.Generation != lease.Generation) {
		return actorFailure("repair refused: mindspec_closing has a conflict or a different generation")
	}
	return nil
}

// repairEvidence uses durable evidence only. It never runs a close-write hook.
func repairEvidence(s State, r Record) (SubmitOptions, error) {
	o := SubmitOptions{Step: r.Key(), Repair: true, Reason: "<text>"}
	lease, _, err := Decode[LeaseRecord](r.Metadata, KeyLease)
	if err != nil {
		return o, err
	}
	if err := repairable(r, lease, validClose(s, r)); err != nil {
		return o, err
	}
	t, _, _ := r.Tuple()
	receipt := Receipt{Kind: "passed", Generation: lease.Generation}
	switch t.Kind {
	case "lens", "verify", "change-judge":
		data, err := recordBytes(r)
		if err != nil {
			return o, err
		}
		receipt.RecordSHA256 = recordDigest(data)
		pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
		if err != nil {
			return o, err
		}
		if has && pin.RecordSHA256 != receipt.RecordSHA256 {
			return o, actorFailure("repair record differs from mindspec_closing")
		}
	case "review-slot":
		registration, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
		if err != nil {
			return o, err
		}
		if !has {
			return o, actorFailure("repair needs a registered verdict path")
		}
		for _, p := range s.Panels {
			if p.Slug() == registration.Slug {
				o.Artifact = filepath.Join(p.Dir, fmt.Sprintf("R%d-round-%d.json", t.Slot, t.Round))
				break
			}
		}
		if o.Artifact == "" {
			return o, actorFailure("repair needs a registered verdict path")
		}
	case "implement", "fix":
		o.Commit = rawSubject(s, r.Key())
		if o.Commit == "" {
			return o, actorFailure("repair needs the current branch commit")
		}
	default:
		return o, actorFailure("step kind has no repair evidence")
	}
	if !recordKind(t.Kind) {
		receipt, err = resolveArtifact(s, o)
		if err != nil {
			return o, err
		}
	}
	check := artifactChecks[t.Kind]
	if check == nil {
		return o, actorFailure("artifact check is not available for " + t.Kind)
	}
	return o, check(s, r, receipt)
}

func repairRun(o SubmitOptions) Run {
	argv := []string{"mindspec", "submit", o.Step, "--repair", "--reason", o.Reason}
	if o.Artifact != "" {
		argv = append(argv, "--artifact", o.Artifact)
	}
	if o.Commit != "" {
		argv = append(argv, "--commit", o.Commit)
	}
	return Run{Actor: "binary", Argv: argv}
}
