package panel

import (
	"strings"
	"testing"
)

// TestVoteDecision is the deterministic vote-only subset shared by the
// complete-side advisory and the hook (Spec 093 Req 13d). It excludes
// staleness/dirty (the hook's git work) but pins the same threshold,
// completeness, REJECT/hard_block, and abandoned semantics.
func TestVoteDecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		res     *Result
		want    VoteVerdict
		summary string
	}{
		{
			name: "no panel → Pass",
			res:  &Result{},
			want: VotePass,
		},
		{
			name: "unreadable registration → Block",
			res:  &Result{PanelErr: errString("bad")},
			want: VoteBlock,
		},
		{
			name: "abandoned → Abandoned with reason",
			res: &Result{
				Panel:       &Panel{ExpectedReviewers: 6, Round: 1, Abandoned: true, AbandonReason: "max: dropped"},
				LatestRound: 1,
			},
			want:    VoteAbandoned,
			summary: "dropped",
		},
		{
			name: "round mismatch → Block",
			res: &Result{
				Panel: &Panel{ExpectedReviewers: 6, Round: 1}, LatestRound: 2, RoundMismatch: true,
			},
			want: VoteBlock,
		},
		{
			name: "incomplete → Block",
			res: &Result{
				Panel: &Panel{ExpectedReviewers: 6, Round: 1}, LatestRound: 1,
				Verdicts: makeVerdicts(4, 0),
			},
			want:    VoteBlock,
			summary: "incomplete",
		},
		{
			name: "REJECT → Block",
			res: &Result{
				Panel: &Panel{ExpectedReviewers: 6, Round: 1}, LatestRound: 1,
				Verdicts: makeVerdicts(5, 1), Approves: 5, Rejects: 1,
			},
			want: VoteBlock,
		},
		{
			name: "threshold met 5/6 with 6 present → Pass",
			res: &Result{
				Panel: &Panel{ExpectedReviewers: 6, Round: 1}, LatestRound: 1,
				Verdicts: makeVerdicts(6, 0), Approves: 5,
			},
			want: VotePass,
		},
		{
			name: "sub-threshold 4/6 with 6 present → Block",
			res: &Result{
				Panel: &Panel{ExpectedReviewers: 6, Round: 1}, LatestRound: 1,
				Verdicts: makeVerdicts(6, 0), Approves: 4,
			},
			want:    VoteBlock,
			summary: "threshold is 5/6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, s := tt.res.VoteDecision()
			if v != tt.want {
				t.Errorf("verdict = %v, want %v (%s)", v, tt.want, s)
			}
			if tt.summary != "" && !strings.Contains(s, tt.summary) {
				t.Errorf("summary %q missing %q", s, tt.summary)
			}
		})
	}
}

func makeVerdicts(n, _ int) []Verdict {
	out := make([]Verdict, n)
	for i := range out {
		out[i] = Verdict{Slot: string(rune('a' + i)), Round: 1}
	}
	return out
}

type errString string

func (e errString) Error() string { return string(e) }

// TestVoteDecision_BeadIDForAbandonedWithoutReason: abandoned with empty
// reason still classifies as Abandoned (the missing reason is surfaced, not
// upgraded to a Block — enforcement of who/why lives in the consumers).
func TestVoteDecision_AbandonedNoReason(t *testing.T) {
	t.Parallel()
	res := &Result{Panel: &Panel{ExpectedReviewers: 6, Round: 1, Abandoned: true}, LatestRound: 1}
	v, s := res.VoteDecision()
	if v != VoteAbandoned {
		t.Fatalf("want VoteAbandoned, got %v", v)
	}
	if !strings.Contains(s, "no abandon_reason") {
		t.Errorf("summary should flag missing reason: %s", s)
	}
}
