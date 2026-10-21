package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SlotRuntime supplies config and lock facts without coupling the leaf to config.
type SlotRuntime struct {
	Family, Model, Kind string
	TimeoutMinutes      *int
	SlotLockHeld        bool
	RunLockHeld         map[int]bool
	OriginalKind        string
}

type SlotStatus struct {
	Slot           string   `json:"slot"`
	Family         string   `json:"family"`
	Model          string   `json:"model"`
	Lens           string   `json:"lens"`
	Kind           string   `json:"kind"`
	Status         string   `json:"status"`
	Annotations    []string `json:"annotations"`
	HandoffPending bool     `json:"handoff_pending"`
}

// latestLaterAttempt selects the highest attempt after the latest submission.
// A later successful rerun can close a slot without adding another verdict.
func latestLaterAttempt(events []SlotAttempt) (SlotAttempt, bool) {
	var latest SlotAttempt
	found := false
	for _, e := range events {
		if e.AfterLatestSubmission && (!found || e.Attempt >= latest.Attempt) {
			latest, found = e, true
		}
	}
	return latest, found
}

func submissionClosed(sub SubmissionRecord, events []SlotAttempt) bool {
	e, later := latestLaterAttempt(events)
	return !later || (e.Status == "submitted" && e.Seq == sub.Seq)
}

// RoundSchema uses only round events. A downgrade from a v2 round fails closed.
func RoundSchema(dir string) (RoundRecord, error) {
	lines, err := ReadLedger(dir)
	if err != nil {
		return RoundRecord{}, err
	}
	var latest RoundRecord
	seenV2 := false
	for _, line := range lines {
		var r RoundRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return latest, err
		}
		if r.Record != RecordRound {
			continue
		}
		if seenV2 && r.VerdictSchema != 2 {
			return r, fmt.Errorf("round registration removed the verdict schema; recovery: create a new round")
		}
		seenV2 = seenV2 || r.VerdictSchema == 2
		latest = r
	}
	return latest, nil
}

// ReadSlotStatuses is read-only. Lock facts are probes taken by the adapter.
func ReadSlotStatuses(res *Result, runtime map[string]SlotRuntime, now time.Time) ([]SlotStatus, error) {
	if res == nil || res.Panel == nil {
		return nil, fmt.Errorf("panel registration unreadable")
	}
	p := res.Panel
	round, err := RoundSchema(res.Dir)
	if err != nil {
		return nil, err
	}
	subs, err := LatestSubmissions(res.Dir, p.Round)
	if err != nil {
		return nil, err
	}
	attempts, err := SlotAttempts(res.Dir, p.Round)
	if err != nil {
		return nil, err
	}
	lines, err := ReadLedger(res.Dir)
	if err != nil {
		return nil, err
	}
	var out []SlotStatus
	for _, roster := range p.Roster {
		rt := runtime[roster.Slot]
		if rt.Family == "" {
			rt.Family = roster.Family
		}
		if rt.Model == "" {
			rt.Model = roster.Model
		}
		s := SlotStatus{Slot: roster.Slot, Family: rt.Family, Model: rt.Model, Lens: roster.Lens, Kind: rt.Kind, Status: "missing", Annotations: []string{}}
		sub := subs[roster.Slot]
		events := attempts[roster.Slot]
		file := fmt.Sprintf("%s-round-%d.json", roster.Slot, p.Round)
		data, readErr := os.ReadFile(filepath.Join(res.Dir, file))
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, readErr
		}
		exists := readErr == nil
		matches := exists && sub.Seq > 0 && findingHash(data) == sub.FileSHA256
		if round.VerdictSchema != 2 {
			if exists {
				var v FindingVerdict
				s.Status = "final"
				if json.Unmarshal(data, &v) != nil {
					s.Status = "malformed"
				}
			}
		} else {
			switch {
			case sub.Seq > 0 && sub.Status == "final" && sub.Verdict != "VOID" && submissionClosed(sub, events):
				s.Status = "damaged"
				if matches {
					s.Status = "final"
				}
			case exists && !matches:
				s.Status = "unsubmitted"
			default:
				e, later := latestLaterAttempt(events)
				if !later || e.Status == "submitted" && e.Seq == sub.Seq {
					if sub.Seq > 0 {
						s.Status = "draft"
						if sub.Verdict == "VOID" {
							s.Status = "void"
						}
					}
				} else {
					switch e.Status {
					case "started":
						s.Status = "launch_failed"
						if rt.SlotLockHeld || rt.RunLockHeld[e.Attempt] {
							s.Status = "running"
						}
					case "dispatched":
						s.Status = "running"
						when, err := time.Parse(time.RFC3339Nano, e.CreatedAt)
						if err != nil {
							return nil, err
						}
						if rt.TimeoutMinutes != nil && now.Sub(when) > time.Duration(*rt.TimeoutMinutes)*time.Minute {
							s.Status = "launch_failed"
						}
					case "quota_wall", "launch_failed", "malformed":
						s.Status = e.Status
					}
				}
			}
		}
		var replaced string
		for _, line := range lines {
			var record struct {
				SubmissionRecord
				FromFamily string `json:"from_family"`
				ToFamily   string `json:"to_family"`
			}
			if err := json.Unmarshal(line, &record); err != nil {
				return nil, err
			}
			if record.Round != p.Round || record.Slot != roster.Slot {
				continue
			}
			switch record.Record {
			case RecordSubstitution:
				s.Annotations = append(s.Annotations, "substituted "+record.FromFamily+"->"+record.ToFamily)
				s.HandoffPending = rt.Kind == "harness"
			case RecordLaunch:
				var e LaunchRecord
				if err := json.Unmarshal(line, &e); err != nil {
					return nil, err
				}
				if e.Status == "dispatched" {
					s.HandoffPending = false
				}
			case RecordVerdict:
				if record.Via == "direct" && rt.OriginalKind == "process" && !containsAnnotation(s.Annotations, "direct on process family") {
					s.Annotations = append(s.Annotations, "direct on process family")
				}
				if record.Seq < sub.Seq && record.Status == "final" && (record.Verdict == VerdictRequestChanges || record.Verdict == VerdictReject || record.HardBlock) {
					replaced = record.Verdict
				}
			}
		}
		if sub.RunnerSource == "config.local.yaml" {
			s.Annotations = append(s.Annotations, "local runner")
		}
		if replaced != "" && sub.Seq > 0 && sub.Verdict != VerdictRequestChanges && sub.Verdict != VerdictReject && !sub.HardBlock {
			s.Annotations = append(s.Annotations, "replaced "+replaced+"->"+sub.Verdict)
		}
		out = append(out, s)
	}
	return out, nil
}

func containsAnnotation(values []string, value string) bool {
	for _, s := range values {
		if s == value {
			return true
		}
	}
	return false
}

func VerifyExitCode(slots []SlotStatus) int {
	code := 0
	for _, s := range slots {
		if s.Status == "running" || s.Status == "missing" || s.Status == "draft" && s.Kind == "harness" || s.Status == "quota_wall" && s.HandoffPending {
			return 1
		}
		if s.Status != "final" {
			code = 2
		}
	}
	return code
}

// ReplacementAnnotations is shared by tally and verify rendering.
func ReplacementAnnotations(slots []SlotStatus) []string {
	var out []string
	for _, s := range slots {
		for _, a := range s.Annotations {
			if strings.HasPrefix(a, "replaced ") {
				out = append(out, s.Slot+" "+a)
			}
		}
	}
	return out
}
