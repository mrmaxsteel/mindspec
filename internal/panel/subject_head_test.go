package panel

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveSubjectHead_BranchErrors(t *testing.T) {
	missing := errors.New("missing ref")
	broken := errors.New("cannot read object database")
	for _, branchErr := range []error{missing, broken} {
		t.Run(branchErr.Error(), func(t *testing.T) {
			var refs []string
			target := strings.Repeat("a", 40)
			sha, err := ResolveSubjectHead(Panel{Gate: "plan_approve", Spec: "900-demo", Target: target}, "repo",
				func(specID string) (string, error) {
					if specID != "900-demo" {
						t.Fatalf("unexpected spec: %s", specID)
					}
					return "spec/900-demo", nil
				},
				func(_, ref string) (string, error) {
					refs = append(refs, ref)
					if ref == "spec/900-demo" {
						return "", branchErr
					}
					return target, nil
				}, func(err error) bool { return errors.Is(err, missing) })
			if branchErr == missing {
				if err != nil || sha != target || !reflect.DeepEqual(refs, []string{"spec/900-demo", target}) {
					t.Fatalf("missing branch must fall back: sha=%s err=%v refs=%v", sha, err, refs)
				}
			} else if !errors.Is(err, broken) || sha != "" || !reflect.DeepEqual(refs, []string{"spec/900-demo"}) {
				t.Fatalf("git error was hidden: sha=%s err=%v refs=%v", sha, err, refs)
			}
		})
	}
}

func TestResolveSubjectHead_MovingRefs(t *testing.T) {
	for _, target := range []string{"main", "spec/900-demo", "HEAD", "refs/heads/deadbeef"} {
		var refs []string
		_, err := ResolveSubjectHead(Panel{Gate: "spec_approve", Spec: "900-demo", Target: target}, "repo",
			func(string) (string, error) {
				t.Fatal("moving ref must not resolve the spec branch")
				return "", nil
			},
			func(_, ref string) (string, error) { refs = append(refs, ref); return "tip", nil },
			func(error) bool { return false })
		if err != nil || !reflect.DeepEqual(refs, []string{target}) {
			t.Fatalf("moving ref changed: %s %v %v", target, err, refs)
		}
	}
}
