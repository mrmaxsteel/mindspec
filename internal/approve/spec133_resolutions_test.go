package approve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestS133Resolutions_ApprovalObligations(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, state := range []string{"live", "abandoned", "deleted ledger", "replaced", "ledger only", "outside section", "id prefix", "fenced ids", "comment ids", "all named"} {
			t.Run(policy+"/"+state, func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "unresolved")
				id := f.deferFinding(false)
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				// The epic and ledger must both contribute: the second id exists only
				// on the epic, and the first is removed from that list on ledger cases.
				f.meta["plan_deferral_ids"] = []string{"plan-epic-only"}
				if state == "deleted ledger" {
					f.meta["plan_deferral_ids"] = []string{id, "plan-epic-only"}
				}
				ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
				if state != "deleted ledger" {
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track spec ledger")
				}
				switch state {
				case "abandoned":
					dir := filepath.Dir(ledger)
					res, err := panel.Tally(dir)
					if err != nil {
						t.Fatal(err)
					}
					paneltest.Seed(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: s133DocID, Panel: "spec_approve", Reason: "review stopped", By: "operator", CreatedAt: paneltest.CreatedAt})
					res.Panel.Abandoned = true
					res.Panel.AbandonReason = "review stopped"
					if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
						t.Fatal(err)
					}
				case "deleted ledger":
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track before deletion")
					if err := os.Rename(ledger, filepath.Join(t.TempDir(), "deleted-ledger")); err != nil {
						t.Fatal(err)
					}
				case "ledger only":
					if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "old-snapshot")); err != nil {
						t.Fatal(err)
					}
				case "replaced":
					res, err := panel.Tally(filepath.Dir(ledger))
					if err != nil {
						t.Fatal(err)
					}
					p := res.Panel
					if err := panel.Create(filepath.Dir(ledger), panel.CreateInput{Spec: s133DocID, Target: p.Target, Gate: "spec_approve", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
						t.Fatal(err)
					}
				}
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				content += "\n## Resolutions\n\n"
				if state == "all named" {
					content += "Resolve `" + id + "` and `plan-epic-only`.\n"
				} else if state == "id prefix" {
					content += id + "-extra plan-epic-only-extra\n"
				} else if state == "fenced ids" {
					content += "````md\n```\n" + id + " plan-epic-only\n```\n````\n"
				} else if state == "comment ids" {
					content += "<!--\n" + id + " plan-epic-only\n-->\n"
				} else {
					content += "None.\n"
				}
				if state == "outside section" {
					content += "\n## Appendix\n\n" + id + " plan-epic-only\n"
				}
				f.writePlan(content)
				f.commit("ready plan")
				f.seed("plan_approve", "pass")
				f.exec.commits = 0
				f.mutations = nil
				if state == "all named" {
					if _, err := f.approve("plan_approve"); err != nil {
						t.Fatal(err)
					}
					if f.creates != 1 {
						t.Fatal("resolution-complete plan made no bead")
					}
					return
				}
				before := f.read("plan.md")
				head := preflightGit(t, f.root, "rev-parse", "HEAD")
				_, err := f.approve("plan_approve")
				if err == nil || !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "plan-epic-only") {
					t.Fatalf("missing obligations not named: %v", err)
				}
				if before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") || len(f.mutations) != 0 || f.closes != 0 || f.creates != 0 || f.exec.commits != 0 {
					t.Fatalf("resolution refusal mutated: %v", f.mutations)
				}
			})
		}
	}
}

func TestS133Resolutions_SectionBoundary(t *testing.T) {
	for _, text := range []string{"## Appendix\nplan-one", "## Resolutions\nNone\n## Appendix\nplan-one", "```md\n## Resolutions\nplan-one\n```", "## Resolutions\nplan-one-extra"} {
		if resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("accepted outside or partial id: %s", text)
		}
	}
	if !resolutionNamesID(planResolutionsSection("## Resolutions\n### Decision\nResolve `plan-one`.\n"), "plan-one") {
		t.Fatal("nested resolution did not name id")
	}
}

func TestS133Resolutions_HiddenMarkdown(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"shorter backticks", "````md\n```\n## Resolutions\nplan-one\n```\n````\n"},
		{"different fence character", "~~~md\n```\n## Resolutions\nplan-one\n~~~\n"},
		{"closing fence info", "```md\n```text\n## Resolutions\nplan-one\n```\n"},
		{"indented closing fence", "```md\n    ```\n## Resolutions\nplan-one\n```\n"},
		{"nonbreaking space closer", "```md\n```\u00a0\n## Resolutions\nplan-one\n```\n"},
		{"comment heading", "<!--\n## Resolutions\nplan-one\n-->\n"},
		{"single line comment heading", "<!-- ## Resolutions plan-one -->\n"},
		{"fenced id", "## Resolutions\n```\nplan-one\n```\n"},
		{"tilde fenced id", "## Resolutions\n~~~\nplan-one\n~~~\n"},
		{"comment id", "## Resolutions\n<!-- plan-one -->\n"},
		{"multiline comment id", "## Resolutions\n<!--\nplan-one\n-->\n"},
		{"comment boundary", "## Resolutions\n<!--\n## Appendix\n-->\nNone.\n"},
		{"comment inside fence", "```\n<!--\n```\n## Appendix\nplan-one\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if resolutionNamesID(planResolutionsSection(tc.text), "plan-one") {
				t.Fatalf("hidden id counted: %s", tc.text)
			}
			for _, control := range []string{"Resolve plan-one.\n", "- Resolve `plan-one`.\n"} {
				text := tc.text + "\n## Resolutions\n" + control
				if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
					t.Fatalf("visible control did not count: %s", text)
				}
			}
		})
	}
	for _, text := range []string{
		"````\n```\n````` \t\n## Resolutions\nplan-one\n",
		"   ~~~md\n~~~\r\n## Resolutions\nplan-one\n",
		"## Resolutions\n<!--\n## Appendix\n-->\nplan-one\n",
		"## Resolutions\n<!-- hidden -->Resolve plan-one.\n",
		"## Resolutions\n```bad`info\nplan-one\n",
	} {
		if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("visible id did not count: %s", text)
		}
	}
}

var resolutionBlockCases = []struct{ name, tail string }{
	{"lazy continuation ids", "\n## Resolutions\n> Context.\nlazy text\nplan-one\n"},
	{"empty bullet heading", "\n-\n  ## Resolutions\n  plan-one\n"},
	{"empty ordered heading", "\n1.\n   ## Resolutions\n   plan-one\n"},
	{"list fence blank", "\n## Plan\n- item\n  ```\n  code\n\n  ## Resolutions\n  R1 resolved\n  ```\n"},
	{"inline tag heading boundary", "\n## Resolutions\n\nR2 resolved\n\n## Bead 1: handle `<id>` input\n\nR1 resolved.\n"},
	{"list fence", "\n## Resolutions\n\n- item\n    ```\n    plan-one\n    ```\n"},
	{"quote fence", "\n## Resolutions\n\n> ```\n> plan-one\n> ```\n"},
	{"indented heading", "\n    ## Resolutions\n    plan-one\n"},
	{"indented text", "\n## Resolutions\n\n    plan-one\n"},
	{"list heading", "\n- item\n  ## Resolutions\n  plan-one\n"},
	{"list heading on marker", "\n- ## Resolutions\n  plan-one\n"},
	{"quote heading", "\n> ## Resolutions\n> plan-one\n"},
	{"list fence in quote", "\n## Resolutions\n\n> - item\n>     ```\n>     plan-one\n>     ```\n"},
	{"quote fence in list", "\n## Resolutions\n\n- item\n  > ~~~\n  > plan-one\n  > ~~~\n"},
	{"nested list fence", "\n## Resolutions\n\n- item\n  - child\n    ~~~~\n    plan-one\n    ~~~~\n"},
	{"ordered list fence", "\n## Resolutions\n\n10. item\n    ```\n    plan-one\n    ```\n"},
	{"ordered marker in paragraph", "\n## Resolutions\n\nParagraph.\n2. prose\n\n    ```\n    plan-one\n    ```\n"},
	{"ordered marker after continuation", "\n## Resolutions\n\nParagraph.\n    continuation\n2. prose\n\n    ```\n    plan-one\n    ```\n"},
	{"nonbreaking space in paragraph", "\n## Resolutions\n\nParagraph.\n\u00a0\n2. prose\n\n    plan-one\n"},
	{"ordered marker after lazy quote", "\n## Resolutions\n\n> Paragraph.\n2. prose\n\n    plan-one\n"},
	{"ordered marker after lazy list", "\n## Resolutions\n\n- Paragraph.\n2. prose\n\n    plan-one\n"},
	{"fence on list marker", "\n## Resolutions\n\n- ```\n  plan-one\n  ```\n"},
	{"code with list marker", "\n## Resolutions\n\n    - plan-one\n"},
	{"code with quote marker", "\n## Resolutions\n\n    > plan-one\n"},
	{"code in list", "\n## Resolutions\n\n- item\n\n      plan-one\n"},
	{"code after thematic break", "\n## Resolutions\n\n- - -\n\n    plan-one\n"},
	{"tab code", "\n## Resolutions\n\n\tplan-one\n"},
	{"tab heading", "\n\t## Resolutions\n\tplan-one\n"},
	{"carriage return code", "\r## Resolutions\r\r    plan-one\r"},
	{"comment list marker", "\n## Resolutions\n<!--\n- item\n-->\n    plan-one\n"},
	{"list comment before fence", "\n## Resolutions\n\n- <!-- hidden -->\n    ```\n  plan-one\n    ```\n"},
	{"split comment id", "\n## Resolutions\nA-<!-- -->1\n"},
	{"tab section boundary", "\n## Resolutions\nNone.\n##\tAppendix\nplan-one\n"},
	{"empty section boundary", "\n## Resolutions\nNone.\n##\nplan-one\n"},
	{"closing hashes boundary", "\n## Resolutions\nNone.\n## Appendix ##\nplan-one\n"},
	{"setext boundary", "\n## Resolutions\n\nAppendix\n--------\nplan-one\n"},
	{"multiline setext boundary", "\n## Resolutions\n\nplan-one\nAppendix\n--------\n"},
	{"link destination", "\n## Resolutions\n\n[decision](plan-one)\n"},
	{"multiline link destination", "\n## Resolutions\n\n[decision](\nplan-one\n)\n"},
	{"script heading after blank", "\n<script>\n\n## Resolutions\nplan-one\n</script>\n"},
	{"preformatted text after blank", "\n## Resolutions\n\n<pre>\n\nplan-one\n</pre>\n"},
}

var resolutionBlockControls = []string{
	"Resolve plan-one.\n",
	"- Resolve `plan-one`.\n",
	"- Decision\n  - Resolve plan-one.\n",
}

func TestS133Resolutions_BlockStructure(t *testing.T) {
	for _, tc := range resolutionBlockCases {
		t.Run(tc.name, func(t *testing.T) {
			id := "plan-one"
			if tc.name == "split comment id" {
				id = "A-1"
			} else if tc.name == "list fence blank" || tc.name == "inline tag heading boundary" {
				id = "R1"
			}
			if section := planResolutionsSection("# Plan\n\nBody.\n" + tc.tail); resolutionNamesID(section, id) {
				t.Fatalf("hidden id counted in %q", section)
			}
			for _, control := range resolutionBlockControls {
				text := tc.tail + "\n## Resolutions\n\n" + strings.ReplaceAll(control, "plan-one", id)
				if !resolutionNamesID(planResolutionsSection(text), id) {
					t.Fatalf("visible control did not count: %s", text)
				}
			}
		})
	}
	for _, text := range []string{
		"## Resolutions\n~~old~~\nResolve plan-one.\n",
		"   ## Resolutions ###\nResolve plan-one.\n",
		"##\tResolutions\r\nResolve plan-one.\r\n",
	} {
		if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("visible id did not count: %s", text)
		}
	}
}

func TestS133Resolutions_BlockStructureApprovePlan(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for i, tc := range resolutionBlockCases {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				f := s133Document(t, policy, "plan_approve", "absent")
				id := "plan-one"
				if tc.name == "split comment id" {
					id = "A-1"
				} else if tc.name == "list fence blank" || tc.name == "inline tag heading boundary" {
					id = "R1"
				}
				f.meta["plan_deferral_ids"] = []string{id}
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				tail := tc.tail
				if tc.name == "inline tag heading boundary" {
					content = strings.Replace(content, "## Bead 1: First thing", "## Resolutions\nR2 resolved\n\n## Bead 1: handle `<id>` input\n\nR1 resolved\n", 1)
					tail = ""
				}
				f.writePlan(content + tail)
				f.commit("block structure refusal")
				f.seed("plan_approve", "pass")
				f.exec.commits = 0
				f.mutations = nil
				before := f.read("plan.md")
				head := preflightGit(t, f.root, "rev-parse", "HEAD")
				_, err := f.approve("plan_approve")
				if err == nil || !strings.Contains(err.Error(), "missing deferred finding ids: "+id) {
					t.Errorf("hidden obligation did not refuse: %v (creates=%d)", err, f.creates)
				}
				if f.creates != 0 || f.closes != 0 || f.exec.commits != 0 || len(f.mutations) != 0 || before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") {
					t.Errorf("refusal mutated lifecycle: creates=%d closes=%d commits=%d mutations=%v", f.creates, f.closes, f.exec.commits, f.mutations)
				}
				if t.Failed() {
					return
				}
				control := strings.ReplaceAll(resolutionBlockControls[i%len(resolutionBlockControls)], "plan-one", id)
				f = s133Document(t, policy, "plan_approve", "absent")
				f.meta["plan_deferral_ids"] = []string{id}
				f.writePlan(content + tail + "\n## Resolutions\n\n" + control)
				f.commit("visible block structure control")
				f.seed("plan_approve", "pass")
				if _, err := f.approve("plan_approve"); err != nil {
					t.Fatalf("visible control refused: %v", err)
				}
				if f.creates != 1 {
					t.Fatalf("visible control created %d beads, want 1", f.creates)
				}
			})
		}
	}
}

func TestS133Resolutions_RefusalBeforeWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "plan_approve", "abandoned")
			f.children = []string{"old-one", "old-two"}
			f.meta["plan_deferral_ids"] = []string{"plan-missing"}
			delete(f.meta, "mindspec_phase")
			delete(f.meta, phase.PhaseSemanticsV2MarkerKey)
			migrations := 0
			t.Cleanup(phase.SetMergeMetadataForTest(func(_ string, updates map[string]interface{}) error {
				migrations++
				f.mutations = append(f.mutations, "migration")
				for key, value := range updates {
					f.meta[key] = value
				}
				return nil
			}))
			before := f.read("plan.md")
			head := preflightGit(t, f.root, "rev-parse", "HEAD")
			metadata, err := json.Marshal(f.meta)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.approve("plan_approve")
			if err == nil || !strings.Contains(err.Error(), "plan resolutions:") || !strings.Contains(err.Error(), "plan-missing") {
				t.Fatalf("missing resolution did not refuse: %v", err)
			}
			if f.closes != 0 || f.creates != 0 || len(f.mutations) != 0 || migrations != 0 {
				t.Errorf("resolution refusal wrote lifecycle: closes=%d creates=%d migrations=%d mutations=%v", f.closes, f.creates, migrations, f.mutations)
			}
			afterMetadata, err := json.Marshal(f.meta)
			if err != nil || string(metadata) != string(afterMetadata) {
				t.Errorf("resolution refusal changed metadata: %s -> %s (%v)", metadata, afterMetadata, err)
			}
			if _, err := os.Stat(filepath.Join(f.root, phase.PhaseLockRelPath)); !os.IsNotExist(err) {
				t.Errorf("resolution refusal reached migration lock: %v", err)
			}
			if before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") || f.exec.commits != 0 {
				t.Error("resolution refusal wrote the plan or made a commit")
			}
			if t.Failed() {
				return
			}

			// The control proves that each pending write has a live path.
			f.writePlan(before + "\nResolve plan-missing.\n")
			f.commit("resolve finding")
			f.exec.commits = 0
			if _, err := f.approve("plan_approve"); err != nil {
				t.Fatal(err)
			}
			if f.closes != 1 || f.creates != 1 || migrations == 0 || f.meta["panel_abandoned"] != true || f.meta["mindspec_migrated_at"] == nil {
				t.Fatalf("control did not reach pending writes: closes=%d creates=%d migrations=%d metadata=%v", f.closes, f.creates, migrations, f.meta)
			}
		})
	}
}

func TestS133Resolutions_TrackedRoundGate(t *testing.T) {
	f := s133Document(t, "enforce", "spec_approve", "unresolved")
	id := f.deferFinding(false)
	lines, err := panel.ReadLedger(filepath.Join(f.dir, "reviews/spec_approve"))
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			var records [][]byte
			for _, line := range lines {
				var record map[string]interface{}
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatal(err)
				}
				if record["record"] == panel.RecordRound {
					record["gate"] = gate
				}
				data, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				records = append(records, data)
			}
			ids := map[string]bool{}
			if err := collectPlanDeferralIDs(records, s133DocID, ids); err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{}
			if gate == "spec_approve" {
				want[id] = true
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("wrong obligations for round gate %s: got %v, want %v", gate, ids, want)
			}
		})
	}
}

func TestS133Resolutions_ReApprovalRetainsAcceptedIDs(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			f.epic = true
			f.meta["plan_deferral_ids"] = []string{"plan-older-deleted-ledger"}
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatal(err)
			}
			ids, err := recordedPlanDeferralIDs(f.meta)
			if err != nil || len(ids) != 2 || !resolutionNamesID(strings.Join(ids, " "), id) || !resolutionNamesID(strings.Join(ids, " "), "plan-older-deleted-ledger") {
				t.Fatalf("accepted obligation lost: %v %v", ids, err)
			}
		})
	}
}

func TestS133Resolutions_TrackedCommitUsesSpecCheckout(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
			preflightGit(t, f.root, "add", "-f", ledger)
			preflightGit(t, f.root, "commit", "-qm", "tracked ledger on spec only")
			primary := filepath.Join(t.TempDir(), "primary")
			preflightGit(t, f.root, "worktree", "add", "--detach", primary, "HEAD~1")
			if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "snapshot")); err != nil {
				t.Fatal(err)
			}
			exec := executor.NewMindspecExecutor(primary)
			err := checkPlanResolutions(f.root, s133DocID, "## Resolutions\nNone.\n", nil, exec)
			if err == nil || !strings.Contains(err.Error(), id) {
				t.Fatalf("read primary HEAD instead of spec commit: %v", err)
			}
			ids, err := specPlanDeferralIDs(f.root, f.root, s133DocID, exec)
			if err != nil || len(ids) != 1 || ids[0] != id {
				t.Fatalf("ledger-only obligation lost at spec approval: %v %v", ids, err)
			}
		})
	}
}

func TestS133Resolutions_QuoteFenceBlankControl(t *testing.T) {
	text := "> ```\n> code\n\n## Resolutions\nR1 resolved\n"
	if !resolutionNamesID(planResolutionsSection(text), "R1") {
		t.Fatal("blank line did not end quote fence")
	}
}

var resolutionInlineCorpus = []struct {
	name, body string
	ids        []string
	want       bool
}{
	{"after lazy paragraph", "> Context.\nlazy text\n\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"checkbox", "- [x] Resolve plan-one and plan-epic-only.\n", []string{"plan-one", "plan-epic-only"}, true},
	{"after link", "- See [the ADR](x.md).\n- plan-one: resolved.\n", []string{"plan-one"}, true},
	{"bracket code same line", "- plan-one: returns `[]string` now.\n- plan-two: fixed.\n", []string{"plan-one"}, true},
	{"bracket code later line", "- plan-one: returns `[]string` now.\n- plan-two: fixed.\n", []string{"plan-two"}, true},
	{"after apostrophe destination", "- See [decision](path'part). plan-one: fixed.\n", []string{"plan-one"}, true},
	{"angle destination", "- See [decision](<path).plan-one>).\n", []string{"plan-one"}, false},
	{"parenthesis title", "- See [decision](x.md (plan-one)).\n", []string{"plan-one"}, false},
	{"link destination", "- see [decision](plan-one)\n", []string{"plan-one"}, false},
	{"link title", "- see [decision](x.md \"plan-one\")\n", []string{"plan-one"}, false},
	{"plain", "- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"nested link label", "- [[plan-one]: fixed](x.md)\n", []string{"plan-one"}, true},
	{"link label suffix", "- [plan-one](x.md)-extra\n", []string{"plan-one"}, false},
	{"link label", "- [plan-one](x.md): fixed.\n", []string{"plan-one"}, true},
	{"after title", "- see [decision](x.md \"hidden\")\n- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference definition", "[decision]: x.md \"plan-one\"\n", []string{"plan-one"}, false},
	{"after reference definition", "[decision]: x.md\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference continuation title", "[decision]: x.md\n\"plan-one\"\n", []string{"plan-one"}, false},
	{"after reference title", "[decision]: x.md\n\"hidden title\"\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference chained destination", "[one]: x.md\n[two]: plan-one\n", []string{"plan-one"}, false},
	{"reference chained label", "[one]: x.md\n[plan-one]: y.md\n", []string{"plan-one"}, false},
	{"reference multiline title", "[decision]: x.md \"title\nplan-one\nend\"\n", []string{"plan-one"}, false},
	{"after reference multiline title", "[decision]: x.md \"title\nhidden\nend\"\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference suffix", "[decision][plan-one]\n", []string{"plan-one"}, false},
	{"multiline title", "[decision](x.md\n\"plan-one\")\n", []string{"plan-one"}, false},
	{"incomplete target next item", "- [decision](unfinished\n- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"incomplete bracket next line", "[unfinished\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"incomplete target next paragraph", "[decision](unfinished\n\nplan-one: fixed.\n", []string{"plan-one"}, true},
}

func TestS133Resolutions_InlineCorpus(t *testing.T) {
	for _, tc := range resolutionInlineCorpus {
		t.Run(tc.name, func(t *testing.T) {
			section := planResolutionsSection("# T\n\n## Resolutions\n\n" + tc.body + "\n## Next\n")
			for _, id := range tc.ids {
				if got := resolutionNamesID(section, id); got != tc.want {
					t.Errorf("id %s counts=%v, want %v; section=%q", id, got, tc.want, section)
				}
			}
		})
	}
}

func TestS133Resolutions_InlineApprovePlan(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range resolutionInlineCorpus {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				f := s133Document(t, policy, "plan_approve", "absent")
				f.meta["plan_deferral_ids"] = tc.ids
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				f.writePlan(content + "\n## Resolutions\n\n" + tc.body)
				f.commit("inline resolutions")
				f.seed("plan_approve", "pass")
				_, err := f.approve("plan_approve")
				if tc.want {
					if err != nil || f.creates != 1 {
						t.Fatalf("visible ids refused: %v; creates=%d", err, f.creates)
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), "hidden text") || f.creates != 0 {
						t.Fatalf("hidden ids did not refuse clearly: %v; creates=%d", err, f.creates)
					}
					for _, id := range tc.ids {
						if !strings.Contains(err.Error(), id) {
							t.Errorf("refusal omits %s: %v", id, err)
						}
					}
				}
			})
		}
	}
}
