package validate

import (
	"reflect"
	"testing"
)

func TestDocExemptionsGuards(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sources    map[string]string
		exemptions map[string]bool
		want       []string
	}{
		{"owned", map[string]string{"internal/widget/source.go": "widget"}, map[string]bool{"widget": true}, []string{"infrastructure"}},
		{"undeclared", map[string]string{"internal/widget/source.go": "widget"}, map[string]bool{"other": true}, []string{"general", "domain", "infrastructure"}},
		{"unowned", map[string]string{"internal/widget/source.go": "widget", "cmd/tool.go": ""}, map[string]bool{"widget": true}, []string{"general", "infrastructure"}},
		{"empty-domain-is-not-an-exemption", map[string]string{"cmd/tool.go": ""}, map[string]bool{"": true, "widget": true}, []string{"general", "infrastructure"}},
		{"empty-sources", map[string]string{}, map[string]bool{"widget": true}, []string{"general", "infrastructure"}},
		{"no-exemptions", map[string]string{"internal/widget/source.go": "widget"}, nil, []string{"general", "domain", "infrastructure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Result{DocImpact: &DocImpact{Sources: tc.sources, domainErrors: map[int]string{1: "widget"}, generalError: 0}}
			for _, name := range []string{"general", "domain", "infrastructure"} {
				r.AddError(name, name)
			}
			before := append([]Issue(nil), r.Issues...)
			got := ApplyDocExemptions(r, tc.exemptions)
			var names []string
			for _, issue := range got.Issues {
				names = append(names, issue.Name)
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, names)
			}
			if !reflect.DeepEqual(r.Issues, before) {
				t.Fatal("exemption changed the input issues")
			}
			// A second pass must use the retained issue indices.
			if again := ApplyDocExemptions(got, tc.exemptions); !reflect.DeepEqual(again.Issues, got.Issues) {
				t.Fatalf("repeated exemption removed another issue: %+v", again.Issues)
			}
		})
	}
	t.Run("nil-impact", func(t *testing.T) {
		r := &Result{}
		r.AddError("git-diff", "diff failed")
		if got := ApplyDocExemptions(r, map[string]bool{"widget": true}); got != r || !got.HasFailures() {
			t.Fatalf("missing impact lost the diff error: %+v", got)
		}
	})
}

func TestDocExemptionsRetainedDomainIndices(t *testing.T) {
	r := &Result{DocImpact: &DocImpact{Sources: map[string]string{"one": "widget", "two": "other"}, domainErrors: map[int]string{1: "widget", 2: "other"}, generalError: 0}}
	for _, name := range []string{"general", "widget", "other", "infrastructure"} {
		r.AddError(name, name)
	}
	got := ApplyDocExemptions(ApplyDocExemptions(r, map[string]bool{"widget": true}), map[string]bool{"other": true})
	if len(got.Issues) != 2 || got.Issues[0].Name != "general" || got.Issues[1].Name != "infrastructure" {
		t.Fatalf("retained indices did not name their original lanes: %+v", got.Issues)
	}
}

func TestDocExemptionsRetainedGeneralIndex(t *testing.T) {
	r := &Result{DocImpact: &DocImpact{Sources: map[string]string{"one": "widget", "two": "other"}, domainErrors: map[int]string{0: "widget"}, generalError: 1}}
	for _, name := range []string{"widget", "general", "infrastructure"} {
		r.AddError(name, name)
	}
	got := ApplyDocExemptions(ApplyDocExemptions(r, map[string]bool{"widget": true}), map[string]bool{"widget": true, "other": true})
	if len(got.Issues) != 1 || got.Issues[0].Name != "infrastructure" {
		t.Fatalf("retained general index did not name its original lane: %+v", got.Issues)
	}
}
