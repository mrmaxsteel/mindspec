package kernel_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// briefRuleRows renders the named rules in the row form of the rules command.
func briefRuleRows(t *testing.T, ids ...string) string {
	t.Helper()
	var b strings.Builder
	for _, id := range ids {
		r, err := rules.Lookup(id)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s: %s — %s\n", termsafe.Escape(r.ID), termsafe.Escape(r.Prohibition), termsafe.Escape(r.Replacement))
	}
	return b.String()
}

// The adapters no longer print scope rules, so each actor brief carries them.
func TestS134Brief_ActorRules(t *testing.T) {
	fixer := []string{"panel.verbs-only", "panel.scratch-in-slot", "review.no-vote", "review.derive", "lifecycle.complete-by-orchestrator"}
	implementer := []string{"panel.verbs-only", "review.no-vote", "review.derive", "lifecycle.complete-by-orchestrator"}
	for _, scope := range []struct {
		name string
		ids  []string
	}{{"fixer", fixer}, {"implementer", implementer}} {
		selected, err := rules.Filter(scope.name, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(selected) != len(scope.ids) {
			t.Fatalf("%s rule table has %d rows, the brief pins %d: %+v", scope.name, len(selected), len(scope.ids), selected)
		}
	}
	f := reducerFixture(t)
	for _, gate := range []string{f.Items[0] + ".review", f.Epic + ".final"} {
		id := gate + ".r2.fix"
		f.Create(id, gate)
		f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeySubject: kernel.Subject{Commit: gateHead(t, f, gate)}, kernel.KeyReproduced: []kernel.Finding{{Slot: "R1", Round: 1, Finding: "ccr[0]", Text: "reproduced finding"}}}))
		body, err := kernel.Brief(f.State(), id)
		f.Must(err)
		got, _, _ := strings.Cut(briefSection(t, body, "Rules (fixer)"), "\nBefore returning")
		if want := "\n" + briefRuleRows(t, fixer...); got != want {
			t.Fatalf("%s fix brief rules:\n%s\nwant:\n%s", gate, got, want)
		}
		rulesAt := strings.Index(body, "\n## Rules (fixer)\n")
		if findingAt := strings.Index(body, "reproduced finding"); findingAt < 0 || findingAt > rulesAt {
			t.Fatalf("%s fix brief puts rules before the findings: %s", gate, body)
		}
		if gate == f.Items[0]+".review" && !strings.HasSuffix(body, "mindspec complete '"+f.Items[0]+"' --check\n") {
			t.Fatalf("fix brief does not end with the check: %s", body)
		}
	}
	briefImpl(t, f, gateHead(t, f, f.Items[0]+".review"))
	body, err := kernel.Brief(f.State(), f.Items[0]+".impl.a1")
	f.Must(err)
	if got, want := briefSection(t, body, "Rules (implementer)"), "\n"+briefRuleRows(t, implementer...); got != want {
		t.Fatalf("implement brief rules:\n%s\nwant:\n%s", got, want)
	}
}
