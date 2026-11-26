package kernel

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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
	case "review-slot":
		if hasRecord(r) {
			return o, repairRecord(s, r, receipt, t.Kind)
		}
		// A raw close before a submit leaves no stored record. The verdict
		// file at the registered path supplies it, with the submit checks.
		path, err := slotVerdictPath(s, t)
		if err != nil {
			return o, err
		}
		data, err := slotRecord(s, t, path)
		if err != nil {
			return o, err
		}
		if err := slotRecordReceipted(s, r, []byte(data)); err != nil {
			return o, err
		}
		o.Artifact = path
		receipt.RecordSHA256 = recordDigest([]byte(data))
		r.Metadata = mergedMetadata(r.Metadata, Metadata(KeyRecord, data))
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
	case "implement", "fix":
		o.Commit = rawSubject(s, r.Key())
		if o.Commit == "" {
			return o, actorFailure("repair needs the current branch commit")
		}
	default:
		return o, actorFailure("step kind has no repair evidence")
	}
	if o.Commit != "" {
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

// repairRecord checks the stored record of r as the repair evidence.
func repairRecord(s State, r Record, receipt Receipt, kind string) error {
	data, err := recordBytes(r)
	if err != nil {
		return err
	}
	if kind == "review-slot" {
		if err := slotRecordReceipted(s, r, data); err != nil {
			return err
		}
	}
	receipt.RecordSHA256 = recordDigest(data)
	check := artifactChecks[kind]
	if check == nil {
		return actorFailure("artifact check is not available for " + kind)
	}
	return check(s, r, receipt)
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

// slotRecordReceipted refuses the repair of a slot when its verdict bytes
// differ from the hash that its earlier receipt or its closing pin names.
// Such bytes changed after the receipt, so only a retry can replace them.
func slotRecordReceipted(s State, r Record, data []byte) error {
	digest := recordDigest(data)
	receipt, has, err := Decode[Receipt](r.Metadata, KeyClose)
	differs := has && (err != nil || receipt.RecordSHA256 != digest)
	pin, pinned, err := Decode[Closing](r.Metadata, KeyClosing)
	differs = differs || pinned && (err != nil || pin.RecordSHA256 != digest)
	if !differs {
		return nil
	}
	recovery := "run mindspec next-action " + briefQuote(idrender.Spec(s.SpecID)) + " --apply; it opens a retry of this slot"
	if next, err := nextAttempt(r.Key()); err == nil {
		recovery = fmt.Sprintf("run mindspec next-action %s --apply; it opens the retry %s, then submit the verdict of that retry", briefQuote(idrender.Spec(s.SpecID)), next)
	}
	return termsafe.NewFailure("stored record differs from its receipt", recovery)
}
