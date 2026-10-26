package panel

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveSubjectHead_CommitPinsUseExactSpecBranch(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, target := range []string{"cafe", "ABCDEF1", strings.Repeat("a", 40), strings.Repeat("b", 64)} {
			t.Run(gate+"/"+target, func(t *testing.T) {
				var refs []string
				sha, err := ResolveSubjectHead(Panel{Gate: gate, Spec: "900-demo", Target: target}, "repo",
					func(spec string) (string, error) {
						if spec != "900-demo" {
							t.Fatalf("unexpected spec %s", spec)
						}
						return "spec/900-demo", nil
					},
					func(root, ref string) (string, error) {
						if root != "repo" {
							t.Fatalf("unexpected workdir %s", root)
						}
						refs = append(refs, ref)
						return "spec-tip", nil
					},
					func(root, ref string) (bool, error) {
						if root != "repo" || ref != "refs/heads/spec/900-demo" {
							t.Fatalf("inexact existence check: %s %s", root, ref)
						}
						return true, nil
					})
				if err != nil || sha != "spec-tip" || !reflect.DeepEqual(refs, []string{"refs/heads/spec/900-demo"}) {
					t.Fatalf("pin must follow exact branch: sha=%s err=%v refs=%v", sha, err, refs)
				}
			})
		}
	}
}

func TestResolveSubjectHead_OnlyConfirmedAbsenceAllowsFallback(t *testing.T) {
	missing := errors.New("missing commit")
	broken := errors.New("cannot read object database")
	for _, tc := range []struct {
		name                  string
		exists                bool
		existsErr, resolveErr error
	}{
		{name: "absent"},
		{name: "present", exists: true},
		{name: "missing object", exists: true, resolveErr: missing},
		{name: "unreadable object", exists: true, resolveErr: broken},
		{name: "unknown existence", existsErr: broken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var refs []string
			target := strings.Repeat("a", 40)
			sha, err := ResolveSubjectHead(Panel{Gate: "plan_approve", Spec: "900-demo", Target: target}, "repo",
				func(string) (string, error) { return "spec/900-demo", nil },
				func(_, ref string) (string, error) {
					refs = append(refs, ref)
					if ref == target {
						return target, nil
					}
					return "spec-tip", tc.resolveErr
				},
				func(_, _ string) (bool, error) { return tc.exists, tc.existsErr })
			switch {
			case tc.existsErr != nil:
				if !errors.Is(err, ErrSubjectHead) || len(refs) != 0 || sha != "" {
					t.Fatalf("unknown existence must fail closed: %s %v %v", sha, err, refs)
				}
			case tc.resolveErr != nil:
				if !errors.Is(err, ErrSubjectHead) || !errors.Is(err, tc.resolveErr) || sha != "" || !reflect.DeepEqual(refs, []string{"refs/heads/spec/900-demo"}) {
					t.Fatalf("present but unreadable must fail closed: %s %v %v", sha, err, refs)
				}
			case tc.exists:
				if err != nil || sha != "spec-tip" || !reflect.DeepEqual(refs, []string{"refs/heads/spec/900-demo"}) {
					t.Fatalf("present branch: %s %v %v", sha, err, refs)
				}
			default:
				if err != nil || sha != target || !reflect.DeepEqual(refs, []string{target}) {
					t.Fatalf("absent branch must fall back: %s %v %v", sha, err, refs)
				}
			}
		})
	}
}

func TestResolveSubjectHead_OtherTargetsResolveRecordedTarget(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve", "bead", "adhoc", "final_review"} {
		for _, target := range []string{"main", "spec/900-demo", "HEAD", "refs/heads/deadbeef", "abc", strings.Repeat("a", 40)} {
			if (gate == "spec_approve" || gate == "plan_approve") && looksLikeCommitSHA(target) {
				continue
			}
			t.Run(gate+"/"+target, func(t *testing.T) {
				var refs []string
				sha, err := ResolveSubjectHead(Panel{Gate: gate, Target: target}, "repo",
					func(string) (string, error) { t.Fatal("must not compose spec branch"); return "", nil },
					func(_, ref string) (string, error) { refs = append(refs, ref); return "target-tip", nil },
					func(string, string) (bool, error) { t.Fatal("must not inspect spec branch"); return false, nil })
				if err != nil || sha != "target-tip" || !reflect.DeepEqual(refs, []string{target}) {
					t.Fatalf("recorded target changed: %s %v %v", sha, err, refs)
				}
			})
		}
	}
}

func TestResolveSubjectHead_BranchCompositionErrorBlocks(t *testing.T) {
	broken := errors.New("invalid spec")
	_, err := ResolveSubjectHead(Panel{Gate: "plan_approve", Target: "cafe"}, "repo",
		func(string) (string, error) { return "", broken },
		func(string, string) (string, error) { t.Fatal("must not resolve invalid branch"); return "", nil },
		func(string, string) (bool, error) { t.Fatal("must not inspect invalid branch"); return false, nil })
	if !errors.Is(err, ErrSubjectHead) || !errors.Is(err, broken) {
		t.Fatalf("branch error hidden: %v", err)
	}
}

func TestResolveGateFacts_UnreadablePinnedBranchIsNotMissingRef(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "review/p")
	// Historical commit-pinned registrations remain readable.
	if err := WriteSnapshot(dir, Panel{Round: 1, Spec: "900-demo", Gate: "plan_approve", Target: "cafe", Roster: createRoster(2), ExpectedReviewers: 2, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:recorded"}); err != nil {
		t.Fatal(err)
	}
	missing := errors.New("missing commit")
	facts := ResolveGateFacts(Registration{Dir: dir}, "", filepath.Dir(dir), GateIO{
		RevParse:      func(string, string) (string, error) { return "", fmt.Errorf("%w: %w", ErrSubjectHead, missing) },
		IsRefNotFound: func(err error) bool { return errors.Is(err, missing) },
	})
	if facts.MissingRef || facts.GitErr != nil || !errors.Is(facts.SubjectErr, ErrSubjectHead) {
		t.Fatalf("present ref with missing commit must be a subject error: %+v", facts)
	}
	if d := PanelGateDecision(facts); d.Action != Block {
		t.Fatalf("unreadable pinned subject must block: %+v", d)
	}
}
