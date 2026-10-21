package rules

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestRuleTable_MinimumSet(t *testing.T) {
	all := []string{"orchestrator", "implementer", "reviewer", "fixer"}
	want := map[string]struct {
		scope []string
		by    string
	}{
		"panel.verbs-only":                   {all, "complete"},
		"panel.submit":                       {[]string{"reviewer"}, "panel submit"},
		"panel.isolated-checkout":            {[]string{"reviewer"}, "advisory"},
		"panel.scratch-in-slot":              {[]string{"reviewer", "fixer"}, "advisory"},
		"panel.fix-after-final":              {[]string{"orchestrator"}, "advisory"},
		"panel.one-fixer":                    {[]string{"orchestrator"}, "advisory"},
		"panel.artifact-gate":                {[]string{"orchestrator"}, "advisory"},
		"panel.clean-before-complete":        {[]string{"orchestrator"}, "complete"},
		"review.basis":                       {[]string{"reviewer"}, "panel submit"},
		"review.property":                    {[]string{"reviewer"}, "advisory"},
		"review.mutation":                    {[]string{"reviewer"}, "advisory"},
		"review.stated-limit":                {[]string{"reviewer"}, "advisory"},
		"review.ci-parity":                   {[]string{"reviewer"}, "advisory"},
		"review.reproduce":                   {[]string{"orchestrator"}, "advisory"},
		"review.no-vote":                     {all, "complete"},
		"review.classifier-redispatch":       {[]string{"orchestrator"}, "advisory"},
		"review.derive":                      {[]string{"implementer", "fixer"}, "advisory"},
		"lifecycle.no-probe-complete":        {[]string{"orchestrator"}, "advisory"},
		"lifecycle.complete-by-orchestrator": {[]string{"implementer", "fixer"}, "advisory"},
		"sequence.bead-impl-prompt":          {[]string{"orchestrator"}, "advisory"},
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) < len(want) {
		t.Fatalf("got %d rules; need at least %d", len(loaded), len(want))
	}
	for id, expected := range want {
		r, err := Lookup(id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(r.Scope, expected.scope) || r.EnforcedBy != expected.by || r.Prohibition == "" || r.Replacement == "" {
			t.Errorf("%s: unexpected row %+v", id, r)
		}
		if id == "review.ci-parity" {
			if !reflect.DeepEqual(r.Gates, []string{"final_review"}) {
				t.Errorf("CI rule gates = %v", r.Gates)
			}
		} else if len(r.Gates) != 0 {
			t.Errorf("%s unexpectedly limited to %v", id, r.Gates)
		}
	}
	r, err := Lookup("sequence.bead-impl-prompt")
	if err != nil || r.Target != "next-action" {
		t.Fatalf("staged rule target: %+v, %v", r, err)
	}
	for _, gate := range gates {
		selected, err := Filter("reviewer", gate)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, r := range selected {
			ids = append(ids, r.ID)
			if !slices.Contains(r.Scope, "reviewer") {
				t.Fatalf("non-reviewer row: %+v", r)
			}
		}
		if slices.Contains(ids, "review.ci-parity") != (gate == "final_review") {
			t.Errorf("CI parity selection for %s: %v", gate, ids)
		}
		if !slices.Contains(ids, "review.basis") {
			t.Errorf("ungated rule excluded for %s: %v", gate, ids)
		}
	}
	for _, query := range [][2]string{{"nope", ""}, {"reviewer", "nope"}} {
		if _, err := Filter(query[0], query[1]); err == nil || !strings.Contains(err.Error(), "recovery:") {
			t.Errorf("unknown filter %v must refuse with recovery: %v", query, err)
		}
	}
	if _, err := Lookup("nope"); err == nil || !strings.Contains(err.Error(), "recovery:") {
		t.Errorf("unknown id must refuse with recovery: %v", err)
	}
	t.Run("exact-id-only", func(t *testing.T) {
		if _, err := Lookup("panel.submi"); err == nil {
			t.Fatal("prefix of panel.submit matched an exact-id lookup")
		}
	})
	t.Run("duplicate-id", func(t *testing.T) {
		bad := strings.Replace(string(table), "id: panel.submit", "id: panel.verbs-only", 1)
		if bad == string(table) {
			t.Fatal("duplicate-id mutation did not apply")
		}
		if _, err := parseTable([]byte(bad)); err == nil {
			t.Fatal("duplicate id passed validation")
		}
	})
	t.Run("unknown-key", func(t *testing.T) {
		bad := strings.Replace(string(table), "id: panel.submit", "id: panel.submit\n  unknown_key: true", 1)
		if bad == string(table) {
			t.Fatal("unknown-key mutation did not apply")
		}
		if _, err := parseTable([]byte(bad)); err == nil {
			t.Fatal("unknown YAML key passed validation")
		}
	})
	// Mutate a required scope in the same YAML decoder used by Load.
	bad := strings.Replace(string(table), "scope: [reviewer]", "scope: [reader]", 1)
	if bad == string(table) {
		t.Fatal("scope mutation did not apply")
	}
	if _, err := parseTable([]byte(bad)); err == nil {
		t.Fatal("wrong scope passed validation")
	}
}

func TestStagedMarker_Parse(t *testing.T) {
	for _, path := range []string{
		"../../plugins/mindspec/skills/ms-bead-impl/SKILL.md",
		"../../.claude/skills/ms-bead-impl/SKILL.md",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		markers := ParseStagedMarkers(string(raw))
		if len(markers) != 1 || markers[0].Err != nil || markers[0].ID != "sequence.bead-impl-prompt" || markers[0].RemoveBy.Format("2006-01-02") != "2027-06-30" {
			t.Fatalf("%s: parsed markers = %+v", path, markers)
		}
		if _, err := Lookup(markers[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{
		"remove-by=2027-06-30",
		"id=sequence.bead-impl-prompt; remove-by=2027-02-30",
		"target=next-action; remove-by=2027-06-30",
		"id=sequence.bead-impl-prompt; id=review.basis; remove-by=2027-06-30",
		"id=; id=sequence.bead-impl-prompt; remove-by=2027-06-30",
	} {
		markers := ParseStagedMarkers("<!-- staged-rule: " + body + " -->")
		if len(markers) != 1 || markers[0].Err == nil {
			t.Fatalf("malformed marker was not retained as invalid: %q: %+v", body, markers)
		}
	}
	markers := ParseStagedMarkers("<!-- staged-rule: id=unknown.rule; remove-by=2020-01-01 -->")
	if len(markers) != 1 || markers[0].Err != nil || markers[0].ID != "unknown.rule" || markers[0].RemoveBy.Year() != 2020 {
		t.Fatalf("unknown and expired marker must remain available for diagnosis: %+v", markers)
	}
	if got := ParseStagedMarkers("no marker"); len(got) != 0 {
		t.Fatalf("invented marker: %+v", got)
	}
}
