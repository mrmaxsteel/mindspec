package panel

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestRoundSchema_RecoveryFailure(t *testing.T) {
	_, err := roundSchema([][]byte{[]byte(`{"record":"round","verdict_schema":2}`), []byte(`{"record":"round"}`)})
	var failure *termsafe.Failure
	if !errors.As(err, &failure) || err.Error() != "round registration removed the verdict schema\nrecovery: create a new round" {
		t.Fatalf("schema recovery changed: %v", err)
	}
}

func TestIntactError_RecoveryFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reviewers int
		integrity error
		message   string
		suffix    string
	}{
		{"reviewer count", 1, nil, "panel registration requires at least 2 reviewers", " --reviewers 2"},
		{"integrity", 2, errors.New("changed\x1b\nrecovery: forged"), "panel.json changed outside the panel verbs (changed\\x1b\\nrecovery: forged)", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := &Result{Dir: "review/réview's", Panel: &Panel{Spec: "adhoc", Gate: "adhoc", Round: 1, ExpectedReviewers: tc.reviewers}, IntegrityErr: tc.integrity}
			err := res.intactError()
			var failure *termsafe.Failure
			if !errors.As(err, &failure) {
				t.Fatalf("untyped registration recovery: %v", err)
			}
			want := tc.message + "\nrecovery: mindspec panel create 'réview'\"'\"'s' --gate adhoc --round 2" + tc.suffix
			if err.Error() != want || strings.Count(err.Error(), "\nrecovery:") != 1 || strings.Contains(err.Error(), "\x1b") {
				t.Fatalf("registration recovery changed: got %q want %q", err, want)
			}
		})
	}
}
