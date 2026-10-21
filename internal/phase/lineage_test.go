package phase

import (
	"errors"
	"strings"
	"testing"
)

func TestFindEpicForBead_BlocksDepIgnored(t *testing.T) {
	stubBeadShow(t, map[string]string{
		"bead-1": `[{"dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"blocks"}]}]`,
		"epic-x": `[{"id":"epic-x","metadata":{"spec_num":129,"spec_title":"test"}}]`,
	}, nil, "[]", nil)
	_, _, err := FindEpicForBead("bead-1")
	if !errors.Is(err, ErrNoEpicLineage) {
		t.Fatalf("blocks edge supplied lineage: %v", err)
	}
}

func TestFindEpicForBead_TwoParentsRefuse(t *testing.T) {
	stubBeadShow(t, map[string]string{
		"bead-1": `[{"dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"parent-child"},{"id":"epic-y","issue_type":"epic","dependency_type":"parent-child"}]}]`,
		"epic-x": `[{"id":"epic-x","metadata":{"spec_num":129,"spec_title":"test"}}]`,
		"epic-y": `[{"id":"epic-y","metadata":{"spec_num":130,"spec_title":"test"}}]`,
	}, nil, "[]", nil)
	_, _, err := FindEpicForBead("bead-1")
	if err == nil || !strings.Contains(err.Error(), "recovery: bd dep remove bead-1 epic-y") || errors.Is(err, ErrNoEpicLineage) {
		t.Fatalf("want two-parent refusal, got %v", err)
	}
}

func TestFindEpicForBead_NoTitleFallback(t *testing.T) {
	stubBeadShow(t, map[string]string{"bead-1": `[{"title":"[129] work"}]`}, nil, "[]", nil)
	listCalls := 0
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) { listCalls++; return nil, errors.New("epic list must not run") }))
	_, _, err := FindEpicForBead("bead-1")
	if !errors.Is(err, ErrNoEpicLineage) || listCalls != 0 {
		t.Fatalf("title supplied lineage or epic list ran: %v", err)
	}
}
