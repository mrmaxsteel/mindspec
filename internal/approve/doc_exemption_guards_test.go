package approve

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func TestS133NoDocImpact_DeclaringMergeGuards(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		sha                   string
		parents               []string
		boundSHA, boundSecond string
		want                  string
	}{
		{"valid", "merge", []string{"first", "second"}, "merge", "second", "bead-1"},
		{"no-parents", "merge", nil, "merge", "second", ""},
		{"single-parent", "merge", []string{"second"}, "merge", "second", ""},
		{"extra-parent", "merge", []string{"first", "second", "third"}, "merge", "second", ""},
		{"empty-sha", "", []string{"first", "second"}, "", "second", ""},
		{"empty-second", "merge", []string{"first", ""}, "merge", "", ""},
		{"other-merge", "other", []string{"first", "second"}, "merge", "second", ""},
		{"other-second", "merge", []string{"first", "other"}, "merge", "second", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := map[string]map[string]interface{}{"bead-1": {"mindspec_landed_merge_sha": tc.boundSHA, "mindspec_landed_second_parent": tc.boundSecond}}
			got := declaringMergeBead(executor.MergeIdentity{SHA: tc.sha, Parents: tc.parents, Subject: "Merge bead/bead-1"}, meta)
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestS133NoDocImpact_ImplExemptionGuards(t *testing.T) {
	for _, action := range []string{"valid", "empty-domain", "empty-reason", "blank-reason", "empty-actor", "empty-time", "bad-declaration", "bad-deferrals", "missing-id", "missing-source", "missing-owner", "missing-domains", "owner-copy", "source-copy", "metadata-error", "owner-metadata-error", "state-error", "no-parent", "undeclared-first", "undeclared-last", "empty-step"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			declaration := validate.DocDeclaration{Domain: "workflow", Reason: "internal only", By: "actor", At: "now"}
			d := validate.DocDeferral{ID: "debt", Source: "bead-1", Owner: "doc-owner", Domains: []string{"workflow"}}
			want := "doc-sync:"
			pass := action == "valid" || action == "owner-copy" || action == "empty-step"
			switch action {
			case "empty-domain":
				declaration.Domain = ""
			case "empty-reason":
				declaration.Reason = ""
			case "blank-reason":
				declaration.Reason = "  "
			case "empty-actor":
				declaration.By = ""
			case "empty-time":
				declaration.At = ""
			case "missing-id":
				d.ID = ""
			case "missing-source":
				d.Source = ""
			case "missing-owner":
				d.Owner = ""
			case "missing-domains":
				d.Domains = nil
			case "owner-copy":
				d.Source = "another-source"
			case "metadata-error", "owner-metadata-error":
				want = "injected metadata error"
			case "state-error":
				want = "injected state error"
			case "no-parent":
				want = "landed change has no parent"
			}
			meta := map[string]interface{}{validate.NoDocImpactKey: []validate.DocDeclaration{declaration}, "mindspec_landed_merge_sha": "merge", "mindspec_landed_second_parent": "second"}
			if action == "bad-declaration" {
				meta[validate.NoDocImpactKey] = "bad"
				want = "invalid mindspec_no_doc_impact"
			}
			if action == "bad-deferrals" {
				meta[validate.DocDeferralsKey] = "bad"
				want = "invalid mindspec_doc_deferrals"
			}
			if strings.HasPrefix(action, "missing-") || action == "owner-copy" || action == "source-copy" || action == "owner-metadata-error" {
				meta[validate.DocDeferralsKey] = []validate.DocDeferral{d}
				if strings.HasPrefix(action, "missing-") {
					want = "invalid doc deferral record"
				}
				if action == "source-copy" {
					want = "open doc deferrals:"
				}
			}
			old := implRunBDFn
			implRunBDFn = func(args ...string) ([]byte, error) {
				if action == "metadata-error" || (action == "owner-metadata-error" && args[1] == "doc-owner") {
					return nil, fmt.Errorf("injected metadata error")
				}
				if args[1] == "doc-owner" {
					return json.Marshal([]map[string]interface{}{{"metadata": map[string]interface{}{}}})
				}
				return json.Marshal([]map[string]interface{}{{"metadata": meta}})
			}
			t.Cleanup(func() { implRunBDFn = old })
			commits := []executor.MergeIdentity{{SHA: "merge", Parents: []string{"first", "second"}}}
			if action == "no-parent" {
				commits[0].Parents = nil
			}
			if action == "undeclared-first" {
				commits = append([]executor.MergeIdentity{{SHA: "direct", Parents: []string{"first"}}}, commits...)
			}
			if action == "undeclared-last" {
				commits = append(commits, executor.MergeIdentity{SHA: "direct", Parents: []string{"first"}})
			}
			if action == "empty-step" {
				commits = append(commits, executor.MergeIdentity{SHA: "empty", Parents: []string{"merge"}})
			}
			ex := &executor.MockExecutor{
				CompletionStateFn: func(_, _, _ string) (executor.CompletionState, error) {
					if action == "state-error" {
						return executor.CompletionState{}, fmt.Errorf("injected state error")
					}
					return executor.CompletionState{NewCommits: commits}, nil
				},
				ChangedFilesFn: func(_, head string) ([]string, error) {
					if head == "empty" {
						return nil, nil
					}
					return []string{"internal/workflow/source.go"}, nil
				},
			}
			docs := validate.ValidateDocsRange(root, "base", "tip", "tip", ex)
			_, err := implDocGate(root, "base", "tip", "010-test", ex, []string{"bead-1"}, docs, "")
			if pass {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %q, got %v", want, err)
			}
		})
	}
}

func TestS133NoDocImpact_NoBeadsNeedsNoAttribution(t *testing.T) {
	ex := &executor.MockExecutor{CompletionStateFn: func(_, _, _ string) (executor.CompletionState, error) {
		return executor.CompletionState{}, fmt.Errorf("no attribution was needed")
	}}
	if _, err := implDocGate(t.TempDir(), "base", "tip", "010-test", ex, nil, &validate.Result{}, ""); err != nil {
		t.Fatal(err)
	}
}
